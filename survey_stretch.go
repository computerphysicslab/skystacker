package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// The stacked mean preview and the synthetic DSS composite are produced by
// different detectors, passbands and display stretches. Matching the stacked
// histogram to the rendered survey composite gives a preview that sits next to
// the survey products with the same tonal balance. Only presentation pixels are
// touched: the scientific stack keeps its own files.

// surveyMatchedPreviewName is the histogram-matched copy of stacked_mean.jpg.
// It is written inside the survey product folder and never replaces the source.
const surveyMatchedPreviewName = "stacked_mean_survey_matched.jpg"

// surveyStretchPercentiles are the histogram anchors used for the match. They
// cover the black point, the bulk of the signal and the white point so the
// result is balanced without clipping real structure.
var surveyStretchPercentiles = []float64{0.5, 2, 10, 25, 50, 75, 90, 98, 99.5}

// surveyStretchMinRange is the smallest survey luminance span (0..255) that is
// considered usable. Below it the reference has no tone to copy and the run
// reports a skip instead of inventing contrast.
const surveyStretchMinRange = 8.0

// surveyStretchReport is stored inside the RGB provenance JSON so the applied
// stretch can be reproduced and audited.
type surveyStretchReport struct {
	Applied          bool      `json:"applied"`
	Source           string    `json:"source,omitempty"`
	Reference        string    `json:"reference,omitempty"`
	Output           string    `json:"output,omitempty"`
	Method           string    `json:"method,omitempty"`
	Percentiles      []float64 `json:"percentiles,omitempty"`
	StackedAnchors   []float64 `json:"stacked_anchors_0_255,omitempty"`
	ReferenceAnchors []float64 `json:"reference_anchors_0_255,omitempty"`
	Note             string    `json:"note,omitempty"`
}

type tonePoint struct {
	x float64 // stacked luminance, 0..255
	y float64 // target survey luminance, 0..255
}

// surveyLuminanceOf weights an 8-bit RGB triple with Rec. 709 coefficients.
// Both sides of the match are measured in the same display units, so the
// absolute scale of the reference does not need any assumption.
func surveyLuminanceOf(r, g, b float64) float64 {
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// sampleImageLuminance returns the 0..255 luminance of at most maxSamples pixels
// of a decoded image, using a fixed stride so the result is deterministic.
func sampleImageLuminance(img image.Image, maxSamples int) []float64 {
	b := img.Bounds()
	total := b.Dx() * b.Dy()
	if total == 0 {
		return nil
	}
	stride := 1
	if maxSamples > 0 && total > maxSamples {
		stride = (total + maxSamples - 1) / maxSamples
	}
	out := make([]float64, 0, total/stride+1)
	count := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if count%stride != 0 {
				count++
				continue
			}
			count++
			r, g, bl, _ := img.At(x, y).RGBA()
			out = append(out, surveyLuminanceOf(float64(r>>8), float64(g>>8), float64(bl>>8)))
		}
	}
	return out
}

// percentileOf linearly interpolates the p-th percentile of a sorted sample.
func percentileOf(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	pos := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	f := pos - float64(lo)
	return sorted[lo]*(1-f) + sorted[hi]*f
}

// interpolateTone evaluates the piecewise-linear tone curve, extrapolating with
// the end segments so 0 and 255 are always defined.
func interpolateTone(points []tonePoint, x float64) float64 {
	if len(points) == 0 {
		return x
	}
	if len(points) == 1 {
		return points[0].y
	}
	if x <= points[0].x {
		dx := points[1].x - points[0].x
		if dx <= 0 {
			return points[0].y
		}
		slope := (points[1].y - points[0].y) / dx
		return points[0].y + (x-points[0].x)*slope
	}
	for i := 1; i < len(points); i++ {
		if x <= points[i].x {
			dx := points[i].x - points[i-1].x
			if dx <= 0 {
				return points[i].y
			}
			f := (x - points[i-1].x) / dx
			return points[i-1].y + f*(points[i].y-points[i-1].y)
		}
	}
	last := len(points) - 1
	dx := points[last].x - points[last-1].x
	if dx <= 0 {
		return points[last].y
	}
	slope := (points[last].y - points[last-1].y) / dx
	return points[last].y + (x-points[last].x)*slope
}

// surveyToneCurve builds a monotone 8-bit LUT that maps the stacked luminance
// distribution onto the reference one at fixed percentiles. The targets are
// absolute 0..255 display values, so the reference's own stretch is preserved.
// ok is false when either side has no usable sample or dynamic range; callers
// then keep the original preview and report the reason.
func surveyToneCurve(stackedSorted, referenceSorted []float64) (lut [256]uint8, stackedAnchors, referenceAnchors []float64, ok bool) {
	for i := range lut {
		lut[i] = uint8(i)
	}
	if len(stackedSorted) < 64 || len(referenceSorted) < 64 {
		return lut, nil, nil, false
	}
	refLo := percentileOf(referenceSorted, surveyStretchPercentiles[0])
	refHi := percentileOf(referenceSorted, surveyStretchPercentiles[len(surveyStretchPercentiles)-1])
	if !(refHi-refLo >= surveyStretchMinRange) || math.IsNaN(refHi-refLo) {
		return lut, nil, nil, false
	}
	points := make([]tonePoint, 0, len(surveyStretchPercentiles))
	stackedAnchors = make([]float64, 0, len(surveyStretchPercentiles))
	referenceAnchors = make([]float64, 0, len(surveyStretchPercentiles))
	prevX := 0.0
	prevY := 0.0
	for i, p := range surveyStretchPercentiles {
		x := percentileOf(stackedSorted, p)
		if i > 0 && x < prevX {
			x = prevX
		}
		prevX = x
		y := percentileOf(referenceSorted, p)
		if i > 0 && y < prevY {
			y = prevY
		}
		prevY = y
		if y < 0 {
			y = 0
		}
		if y > 255 {
			y = 255
		}
		points = append(points, tonePoint{x: x, y: y})
		stackedAnchors = append(stackedAnchors, math.Round(x*100)/100)
		referenceAnchors = append(referenceAnchors, math.Round(y*100)/100)
	}
	for x := 0; x < 256; x++ {
		v := interpolateTone(points, float64(x))
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		lut[x] = uint8(math.Round(v))
	}
	return lut, stackedAnchors, referenceAnchors, true
}

// applySurveyToneCurve applies one LUT to all three channels of a preview. The
// curve comes from luminance, so hue ordering is preserved and no channel is
// balanced independently against a different DSS plate.
func applySurveyToneCurve(src image.Image, lut [256]uint8) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			out.SetRGBA(x, y, color.RGBA{
				R: lut[uint8(r>>8)],
				G: lut[uint8(g>>8)],
				B: lut[uint8(bl>>8)],
				A: 255,
			})
		}
	}
	return out
}

// stretchStackedMeanToSurvey matches stacked_mean.jpg to the rendered synthetic
// survey composite and writes the copy into the survey product folder. The
// source preview is never modified and every failure is reported, never faked.
func stretchStackedMeanToSurvey(outputDir, surveyDir, reference string, valid []bool) *surveyStretchReport {
	rep := &surveyStretchReport{
		Method:    "luminance histogram match at 0.5/2/10/25/50/75/90/98/99.5 percentiles with a monotone per-channel LUT",
		Source:    "stacked_mean.jpg",
		Reference: reference,
	}
	covered := 0
	for _, ok := range valid {
		if ok {
			covered++
		}
	}
	if len(valid) > 0 && covered < 64 {
		rep.Note = fmt.Sprintf("survey composite covers only %d pixels; stretch skipped", covered)
		return rep
	}
	srcPath := filepath.Join(outputDir, "stacked_mean.jpg")
	src, err := decodePreviewJPEG(srcPath)
	if err != nil {
		rep.Note = fmt.Sprintf("stacked_mean.jpg unavailable (%v); stretch skipped", err)
		return rep
	}
	if reference == "" {
		rep.Note = "survey composite not available; stretch skipped"
		return rep
	}
	ref, err := decodePreviewJPEG(filepath.Join(surveyDir, reference))
	if err != nil {
		rep.Note = fmt.Sprintf("%s unavailable (%v); stretch skipped", reference, err)
		return rep
	}
	if src.Bounds() != ref.Bounds() {
		rep.Note = fmt.Sprintf("stacked preview %v and reference %v differ in size; stretch skipped",
			src.Bounds(), ref.Bounds())
		return rep
	}
	stackedSamples := sampleImageLuminance(src, 250_000)
	referenceSamples := sampleImageLuminance(ref, 250_000)
	sort.Float64s(stackedSamples)
	sort.Float64s(referenceSamples)
	lut, stackedAnchors, referenceAnchors, ok := surveyToneCurve(stackedSamples, referenceSamples)
	if !ok {
		rep.Note = "survey reference has no usable dynamic range; stretch skipped"
		return rep
	}
	outPath := filepath.Join(surveyDir, surveyMatchedPreviewName)
	stretched := applySurveyToneCurve(src, lut)
	if err := writeFileAtomic(outPath, func(out io.Writer) error {
		return jpeg.Encode(out, stretched, &jpeg.Options{Quality: 92})
	}); err != nil {
		rep.Note = fmt.Sprintf("could not write stretched preview: %v", err)
		return rep
	}
	rep.Applied = true
	rep.Output = filepath.Base(outPath)
	rep.Percentiles = append([]float64(nil), surveyStretchPercentiles...)
	rep.StackedAnchors = stackedAnchors
	rep.ReferenceAnchors = referenceAnchors
	logf("Survey RGB: matched %s to %s -> %s/%s (%d anchors)",
		rep.Source, reference, surveyProductSubdir, rep.Output, len(surveyStretchPercentiles))
	return rep
}

// clampByte converts a 0..255 tone value to the nearest table index.
func clampByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// decodePreviewJPEG opens and decodes a JPEG preview for the stretch helpers.
func decodePreviewJPEG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return jpeg.Decode(f)
}

// ---------------------------------------------------------------------------
// Per-channel (independent RGB) stretch
// ---------------------------------------------------------------------------

// surveyRGBMatchedPreviewName is the per-channel histogram-matched copy.
const surveyRGBMatchedPreviewName = "stacked_mean_survey_RGB_matched.jpg"

// surveyRGBToneCurves builds three independent monotone 8-bit LUTs, one per
// channel, matching the stacked channel percentiles to the reference ones.
// It returns ok=false when any channel has no usable dynamic range.
func surveyRGBToneCurves(stacked, reference [][]float64) ([3][256]uint8, [3][]float64, [3][]float64, bool) {
	var luts [3][256]uint8
	var stackedAnchors [3][]float64
	var referenceAnchors [3][]float64
	for c := 0; c < 3; c++ {
		for i := range luts[c] {
			luts[c][i] = uint8(i)
		}
		ss := append([]float64(nil), stacked[c]...)
		rr := append([]float64(nil), reference[c]...)
		// reuse surveyToneCurve per channel by treating each channel as a
		// "luminance" distribution: the function is generic over sorted
		// samples, so it works for any per-channel values.
		lut, sa, ra, ok := surveyToneCurve(ss, rr)
		if !ok {
			return luts, stackedAnchors, referenceAnchors, false
		}
		luts[c] = lut
		stackedAnchors[c] = sa
		referenceAnchors[c] = ra
	}
	return luts, stackedAnchors, referenceAnchors, true
}

// applySurveyRGBToneCurves applies a separate LUT to each channel.
func applySurveyRGBToneCurves(src image.Image, luts [3][256]uint8) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA()
			out.SetRGBA(x, y, color.RGBA{
				R: luts[0][uint8(r>>8)],
				G: luts[1][uint8(g>>8)],
				B: luts[2][uint8(bl>>8)],
				A: 255,
			})
		}
	}
	return out
}

// sampleImageChannel returns the 0..255 values of channel c (0=R,1=G,2=B)
// for at most maxSamples pixels of a decoded image.
func sampleImageChannel(img image.Image, c, maxSamples int) []float64 {
	b := img.Bounds()
	total := b.Dx() * b.Dy()
	if total == 0 {
		return nil
	}
	stride := 1
	if maxSamples > 0 && total > maxSamples {
		stride = (total + maxSamples - 1) / maxSamples
	}
	out := make([]float64, 0, total/stride+1)
	count := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if count%stride != 0 {
				count++
				continue
			}
			count++
			r, g, bl, _ := img.At(x, y).RGBA()
			v := float64(r >> 8)
			if c == 1 {
				v = float64(g >> 8)
			} else if c == 2 {
				v = float64(bl >> 8)
			}
			out = append(out, v)
		}
	}
	return out
}

// stretchStackedMeanToSurveyRGB matches stacked_mean.jpg to the rendered
// survey composite independently per channel and writes the copy into the
// survey product folder. The source preview is never modified.
func stretchStackedMeanToSurveyRGB(outputDir, surveyDir, reference string, valid []bool) *surveyStretchReport {
	rep := &surveyStretchReport{
		Method:    "per-channel (independent R/G/B) percentile match at 0.5/2/10/25/50/75/90/98/99.5 percentiles with monotone per-channel LUTs",
		Source:    "stacked_mean.jpg",
		Reference: reference,
	}
	src, err := decodePreviewJPEG(filepath.Join(outputDir, "stacked_mean.jpg"))
	if err != nil {
		rep.Note = fmt.Sprintf("stacked_mean.jpg unavailable (%v); stretch skipped", err)
		return rep
	}
	ref, err := decodePreviewJPEG(filepath.Join(surveyDir, reference))
	if err != nil {
		rep.Note = fmt.Sprintf("%s unavailable (%v); stretch skipped", reference, err)
		return rep
	}
	if src.Bounds() != ref.Bounds() {
		rep.Note = fmt.Sprintf("stacked preview %v and reference %v differ in size; stretch skipped",
			src.Bounds(), ref.Bounds())
		return rep
	}
	covered := 0
	for _, ok := range valid {
		if ok {
			covered++
		}
	}
	if len(valid) > 0 && covered < 64 {
		rep.Note = fmt.Sprintf("survey composite covers only %d pixels; stretch skipped", covered)
		return rep
	}
	stacked := make([][]float64, 3)
	referenceSamples := make([][]float64, 3)
	for c := 0; c < 3; c++ {
		ss := sampleImageChannel(src, c, 250_000)
		stacked[c] = ss
		// sample reference from the decoded JPEG
		sr := sampleImageChannel(ref, c, 250_000)
		referenceSamples[c] = sr
		sort.Float64s(stacked[c])
		sort.Float64s(referenceSamples[c])
	}
	luts, stackedAnchors, referenceAnchors, ok := surveyRGBToneCurves(stacked, referenceSamples)
	if !ok {
		rep.Note = "survey reference has no usable dynamic range in one channel; stretch skipped"
		return rep
	}
	outPath := filepath.Join(surveyDir, surveyRGBMatchedPreviewName)
	stretched := applySurveyRGBToneCurves(src, luts)
	if err := writeFileAtomic(outPath, func(out io.Writer) error {
		return jpeg.Encode(out, stretched, &jpeg.Options{Quality: 92})
	}); err != nil {
		rep.Note = fmt.Sprintf("could not write stretched preview: %v", err)
		return rep
	}
	rep.Applied = true
	rep.Output = filepath.Base(outPath)
	rep.Percentiles = append([]float64(nil), surveyStretchPercentiles...)
	// record luminance anchors from channel 0 as a representative
	rep.StackedAnchors = stackedAnchors[0]
	rep.ReferenceAnchors = referenceAnchors[0]
	logf("Survey RGB: per-channel matched %s to %s -> %s/%s",
		rep.Source, "survey composite", surveyProductSubdir, rep.Output)
	return rep
}

package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// writeSelftestPreview renders a synthetic preview through the production JPEG
// writer so the selftests use the same display space as a real run.
func writeSelftestPreview(path string, w, h int, fn func(x, y int) (float32, float32, float32)) error {
	pix := make([]float32, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			pix[i], pix[i+1], pix[i+2] = fn(x, y)
		}
	}
	return writeFileAtomic(path, func(out io.Writer) error {
		return writePreviewJPEG(out, w, h, pix)
	})
}

// testSurveyToneCurve locks the histogram-matching maths: the LUT must be
// monotone, must move each percentile onto the reference value and must reject
// a reference with no usable dynamic range.
func testSurveyToneCurve() error {
	stacked := make([]float64, 0, 1000)
	reference := make([]float64, 0, 1000)
	for i := 0; i < 1000; i++ {
		f := float64(i) / 999
		stacked = append(stacked, 40+70*f)
		reference = append(reference, 45+190*f)
	}
	sort.Float64s(stacked)
	sort.Float64s(reference)
	lut, stackedAnchors, referenceAnchors, ok := surveyToneCurve(stacked, reference)
	if !ok || len(stackedAnchors) != len(surveyStretchPercentiles) || len(referenceAnchors) != len(surveyStretchPercentiles) {
		return fmt.Errorf("tone curve ok=%v anchors=%d/%d", ok, len(stackedAnchors), len(referenceAnchors))
	}
	for i := 1; i < len(lut); i++ {
		if lut[i] < lut[i-1] {
			return fmt.Errorf("tone curve not monotone at %d: %d < %d", i, lut[i], lut[i-1])
		}
	}
	for i, p := range surveyStretchPercentiles {
		src := stackedAnchors[i]
		got := float64(lut[clampByte(src)])
		if diff := got - referenceAnchors[i]; diff > 3 || diff < -3 {
			return fmt.Errorf("p%g: LUT(%.1f) = %.0f, want %.1f", p, src, got, referenceAnchors[i])
		}
	}
	flat := make([]float64, 500)
	for i := range flat {
		flat[i] = 77
	}
	if _, _, _, ok := surveyToneCurve(stacked, flat); ok {
		return fmt.Errorf("flat reference accepted")
	}
	if _, _, _, ok := surveyToneCurve(stacked[:20], stacked); ok {
		return fmt.Errorf("tiny sample accepted")
	}
	return nil
}

// testStackedMeanSurveyStretch covers the end-to-end preview stretch: it reads
// stacked_mean.jpg plus the rendered survey composite, writes only the survey
// folder copy and reports the matched anchors.
func testStackedMeanSurveyStretch() error {
	dir, err := os.MkdirTemp("", "skystacker-stretch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	surveyDir, err := ensureProductSubdir(dir, surveyProductSubdir)
	if err != nil {
		return err
	}
	const w, h = 48, 32
	if err := writeSelftestPreview(filepath.Join(dir, "stacked_mean.jpg"), w, h,
		func(x, y int) (float32, float32, float32) {
			v := float32(0.20 + 0.40*float64(x)/float64(w-1))
			return v, v, v
		}); err != nil {
		return err
	}
	if err := writeSelftestPreview(filepath.Join(surveyDir, surveyRGBStem+".jpg"), w, h,
		func(x, y int) (float32, float32, float32) {
			f := float64(x) / float64(w-1)
			return float32(0.50 + 0.40*f), float32(0.42 + 0.33*f), float32(0.38 + 0.28*f)
		}); err != nil {
		return err
	}
	valid := make([]bool, w*h)
	for i := range valid {
		valid[i] = true
	}
	rep := stretchStackedMeanToSurvey(dir, surveyDir, surveyRGBStem+".jpg", valid)
	if !rep.Applied || rep.Output != surveyMatchedPreviewName {
		return fmt.Errorf("stretch report = %+v", rep)
	}
	if len(rep.Percentiles) != len(surveyStretchPercentiles) || len(rep.StackedAnchors) != len(surveyStretchPercentiles) ||
		len(rep.ReferenceAnchors) != len(surveyStretchPercentiles) {
		return fmt.Errorf("stretch provenance incomplete: %+v", rep)
	}
	if _, err := os.Stat(filepath.Join(dir, surveyMatchedPreviewName)); err == nil {
		return fmt.Errorf("stretched preview leaked into the output root")
	}
	src, err := decodePreviewJPEG(filepath.Join(surveyDir, surveyMatchedPreviewName))
	if err != nil || src.Bounds().Dx() != w || src.Bounds().Dy() != h {
		return fmt.Errorf("stretched preview invalid: %v", err)
	}
	ref, err := decodePreviewJPEG(filepath.Join(surveyDir, surveyRGBStem+".jpg"))
	if err != nil {
		return err
	}
	outSamples := sampleImageLuminance(src, 0)
	refSamples := sampleImageLuminance(ref, 0)
	sort.Float64s(outSamples)
	sort.Float64s(refSamples)
	for _, p := range surveyStretchPercentiles {
		got := percentileOf(outSamples, p)
		want := percentileOf(refSamples, p)
		if diff := got - want; diff > 8 || diff < -8 {
			return fmt.Errorf("p%g: matched %.1f, reference %.1f", p, got, want)
		}
	}
	emptyDir, err := os.MkdirTemp("", "skystacker-empty-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(emptyDir)
	missing := stretchStackedMeanToSurvey(emptyDir, surveyDir, surveyRGBStem+".jpg", nil)
	if missing.Applied || missing.Note == "" {
		return fmt.Errorf("missing preview accepted: %+v", missing)
	}
	return nil
}

// testProductSubfolders locks the output layout: survey artefacts go to
// survey/, annotation products to maps/, and the helper is idempotent.
func testProductSubfolders() error {
	dir, err := os.MkdirTemp("", "skystacker-layout-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for i := 0; i < 2; i++ {
		if _, err := ensureProductSubdir(dir, surveyProductSubdir); err != nil {
			return err
		}
	}
	mapsDir, err := ensureProductSubdir(dir, mapProductSubdir)
	if err != nil || filepath.Base(mapsDir) != mapProductSubdir {
		return fmt.Errorf("maps subdir = %q err=%v", mapsDir, err)
	}
	surveyDir, _ := ensureProductSubdir(dir, surveyProductSubdir)
	deg := math.Pi / 180
	wc := &wcsTan{CRVAL1: 10 * deg, CRVAL2: 41 * deg, CRPIX1: 4, CRPIX2: 4,
		CD: [2][2]float64{{1e-5 * deg, 0}, {0, 1e-5 * deg}}}
	const w, h = 4, 4
	mosaics := map[string]*surveyMosaic{
		"red":   {Frame: &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}, Valid: make([]bool, w*h), Tiles: []surveyTileReport{{PlateID: "A1HR"}}},
		"green": {Frame: &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}, Valid: make([]bool, w*h), Tiles: []surveyTileReport{{PlateID: "01JV"}}},
		"blue":  {Frame: &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}, Valid: make([]bool, w*h), Tiles: []surveyTileReport{{PlateID: "A066"}}},
	}
	for _, m := range mosaics {
		for p := range m.Valid {
			m.Valid[p] = true
		}
	}
	rgb := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	if err := writeSurveyRGBProducts(dir, surveyDir, w, h, wc,
		[]string{"red", "green", "blue"}, nil, mosaics, rgb, make([]bool, w*h), 0, nil, nil); err != nil {
		return err
	}
	for _, name := range []string{surveyRGBStem + ".fits", surveyRGBStem + "_provenance.json"} {
		if _, err := os.Stat(filepath.Join(surveyDir, name)); err != nil {
			return fmt.Errorf("missing survey product %s: %w", name, err)
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return fmt.Errorf("%s leaked into the output root", name)
		}
	}
	return nil
}

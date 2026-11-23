package main

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeTestPreview renders a synthetic preview through the production JPEG
// writer so the tests exercise the same 0..255 display space as a real run.
func writeTestPreview(t *testing.T, path string, w, h int, fn func(x, y int) (float32, float32, float32)) {
	t.Helper()
	pix := make([]float32, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			pix[i], pix[i+1], pix[i+2] = fn(x, y)
		}
	}
	if err := writeFileAtomic(path, func(out io.Writer) error {
		return writePreviewJPEG(out, w, h, pix)
	}); err != nil {
		t.Fatal(err)
	}
}

// jpegLuminancePercentiles returns the sorted luminance sample of a preview.
func jpegLuminancePercentiles(t *testing.T, path string) []float64 {
	t.Helper()
	img, err := decodePreviewJPEG(path)
	if err != nil {
		t.Fatal(err)
	}
	samples := sampleImageLuminance(img, 0)
	sort.Float64s(samples)
	return samples
}

func TestSurveyToneCurveMonotone(t *testing.T) {
	stacked := make([]float64, 0, 2048)
	reference := make([]float64, 0, 2048)
	for i := 0; i < 2048; i++ {
		f := float64(i) / 2047
		stacked = append(stacked, 20+80*f)
		reference = append(reference, 60+180*f)
	}
	sort.Float64s(stacked)
	sort.Float64s(reference)
	lut, stackedAnchors, referenceAnchors, ok := surveyToneCurve(stacked, reference)
	if !ok {
		t.Fatal("valid distributions rejected")
	}
	if len(stackedAnchors) != len(surveyStretchPercentiles) || len(referenceAnchors) != len(stackedAnchors) {
		t.Fatalf("anchors = %d/%d", len(stackedAnchors), len(referenceAnchors))
	}
	for i := 1; i < len(lut); i++ {
		if lut[i] < lut[i-1] {
			t.Fatalf("LUT not monotone at %d: %d -> %d", i, lut[i-1], lut[i])
		}
	}
	// The reference is brighter and wider, so the curve must lift every anchor
	// onto it and tolerate out-of-range input by clamping.
	for i, p := range surveyStretchPercentiles {
		want := referenceAnchors[i]
		src := stackedAnchors[i]
		if got := float64(lut[int(clampByte(src))]); absFloat(got-want) > 3 {
			t.Fatalf("p%g: LUT(%.1f) = %.0f, want %.1f", p, src, got, want)
		}
	}
	// The reference black sits near 61 and its white near 240, so the curve
	// must keep pure black below the reference black point instead of forcing
	// it to zero: matching absolute reference levels is the whole point.
	if lut[0] > uint8(referenceAnchors[0])+3 || lut[0] < 8 {
		t.Fatalf("LUT[0] = %d, want just below the reference black %.1f", lut[0], referenceAnchors[0])
	}
	if lut[255] < uint8(referenceAnchors[len(referenceAnchors)-1])-3 {
		t.Fatalf("LUT[255] = %d, want the reference white %.1f", lut[255], referenceAnchors[len(referenceAnchors)-1])
	}
}

func TestSurveyToneCurveDegenerate(t *testing.T) {
	stacked := make([]float64, 200)
	for i := range stacked {
		stacked[i] = float64(i)
	}
	sort.Float64s(stacked)
	flat := make([]float64, 200)
	for i := range flat {
		flat[i] = 42
	}
	if _, _, _, ok := surveyToneCurve(stacked, flat); ok {
		t.Fatal("flat reference accepted")
	}
	narrow := make([]float64, 200)
	for i := range narrow {
		narrow[i] = 100 + float64(i%3)
	}
	sort.Float64s(narrow)
	if _, _, _, ok := surveyToneCurve(stacked, narrow); ok {
		t.Fatal("reference with no useful range accepted")
	}
	if _, _, _, ok := surveyToneCurve(stacked[:10], stacked); ok {
		t.Fatal("tiny sample accepted")
	}
}

func TestStretchStackedMeanToSurvey(t *testing.T) {
	dir := t.TempDir()
	surveyDir, err := ensureProductSubdir(dir, surveyProductSubdir)
	if err != nil {
		t.Fatal(err)
	}
	const w, h = 64, 40
	writeTestPreview(t, filepath.Join(dir, "stacked_mean.jpg"), w, h, func(x, y int) (float32, float32, float32) {
		v := float32(0.18 + 0.35*float64(x)/float64(w-1))
		return v, v, v
	})
	writeTestPreview(t, filepath.Join(surveyDir, surveyRGBStem+".jpg"), w, h, func(x, y int) (float32, float32, float32) {
		f := float64(x) / float64(w-1)
		return float32(0.55 + 0.35*f), float32(0.45 + 0.30*f), float32(0.40 + 0.25*f)
	})
	valid := make([]bool, w*h)
	for i := range valid {
		valid[i] = true
	}
	rep := stretchStackedMeanToSurvey(dir, surveyDir, surveyRGBStem+".jpg", valid)
	if !rep.Applied || rep.Output != surveyMatchedPreviewName {
		t.Fatalf("stretch report = %+v", rep)
	}
	if len(rep.StackedAnchors) != len(surveyStretchPercentiles) || len(rep.ReferenceAnchors) != len(surveyStretchPercentiles) {
		t.Fatalf("provenance anchors = %d/%d", len(rep.StackedAnchors), len(rep.ReferenceAnchors))
	}
	if _, err := os.Stat(filepath.Join(dir, surveyMatchedPreviewName)); err == nil {
		t.Fatal("matched preview leaked into the output root")
	}
	out := jpegLuminancePercentiles(t, filepath.Join(surveyDir, surveyMatchedPreviewName))
	ref := jpegLuminancePercentiles(t, filepath.Join(surveyDir, surveyRGBStem+".jpg"))
	for _, p := range surveyStretchPercentiles {
		if got, want := percentileOf(out, p), percentileOf(ref, p); absFloat(got-want) > 8 {
			t.Fatalf("p%g: matched %.1f, reference %.1f", p, got, want)
		}
	}
	// The source must stay untouched.
	if src := jpegLuminancePercentiles(t, filepath.Join(dir, "stacked_mean.jpg")); percentileOf(src, 50) > percentileOf(ref, 50) {
		t.Fatal("source preview was modified")
	}
}

func TestStretchStackedMeanToSurveySkips(t *testing.T) {
	dir := t.TempDir()
	surveyDir, err := ensureProductSubdir(dir, surveyProductSubdir)
	if err != nil {
		t.Fatal(err)
	}
	const w, h = 32, 16
	writeTestPreview(t, filepath.Join(dir, "stacked_mean.jpg"), w, h, func(x, y int) (float32, float32, float32) {
		v := float32(0.3 + 0.2*float64(x)/float64(w-1))
		return v, v, v
	})
	// Missing reference.
	if rep := stretchStackedMeanToSurvey(dir, surveyDir, surveyRGBStem+".jpg", nil); rep.Applied || rep.Note == "" {
		t.Fatalf("missing reference not reported: %+v", rep)
	}
	// Flat reference.
	writeTestPreview(t, filepath.Join(surveyDir, "flat.jpg"), w, h, func(x, y int) (float32, float32, float32) {
		return 0.5, 0.5, 0.5
	})
	if rep := stretchStackedMeanToSurvey(dir, surveyDir, "flat.jpg", nil); rep.Applied || rep.Note == "" {
		t.Fatalf("flat reference not reported: %+v", rep)
	}
	// Size mismatch.
	writeTestPreview(t, filepath.Join(surveyDir, "small.jpg"), w/2, h/2, func(x, y int) (float32, float32, float32) {
		v := float32(float64(x) / float64(w/2))
		return v, v, v
	})
	if rep := stretchStackedMeanToSurvey(dir, surveyDir, "small.jpg", nil); rep.Applied || rep.Note == "" {
		t.Fatalf("size mismatch not reported: %+v", rep)
	}
	// No covered survey pixels.
	all := make([]bool, w*h)
	if rep := stretchStackedMeanToSurvey(dir, surveyDir, surveyRGBStem+".jpg", all); rep.Applied || rep.Note == "" {
		t.Fatalf("empty coverage not reported: %+v", rep)
	}
}

func TestEnsureProductSubdir(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		got, err := ensureProductSubdir(dir, mapProductSubdir)
		if err != nil || filepath.Base(got) != mapProductSubdir {
			t.Fatalf("call %d: %q %v", i, got, err)
		}
	}
	if _, err := ensureProductSubdir("", mapProductSubdir); err == nil {
		t.Fatal("empty output directory accepted")
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

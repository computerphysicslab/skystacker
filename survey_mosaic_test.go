package main

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// Item 168: a rotated M31 field is ~44.8 arcmin in RA and ~65.6 in Dec. The
// old unrotated geometry requested 60 x 42.2 and lost a third of the frame.
func TestSurveyFootprintRotated(t *testing.T) {
	deg := math.Pi / 180
	w := &wcsTan{
		CRVAL1: 10.70791973605478 * deg,
		CRVAL2: 41.33741332462579 * deg,
		CD: [2][2]float64{
			{2.1948854849574314e-05 * deg, 0.0005332892984032778 * deg},
			{-0.0005331296744964473 * deg, 2.178615358846491e-05 * deg},
		},
	}
	raSpan, decSpan, err := surveyFootprintArcmin(w, 1997, 1318)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(raSpan-44.79) > 0.2 || math.Abs(decSpan-65.57) > 0.2 {
		t.Fatalf("rotated footprint = %.3f x %.3f arcmin, want ~44.79 x 65.57", raSpan, decSpan)
	}
	_, _, h, wd, err := surveyGeometry(w, 1997, 1318)
	if err != nil || math.Abs(h-60) > 1e-9 || math.Abs(wd-44.79) > 0.2 {
		t.Fatalf("clamped geometry = h %.3f w %.3f err=%v", h, wd, err)
	}
}

// Item 169: the 65.6 arcmin Dec span needs two overlapping 60 arcmin tiles.
func TestDSSMosaicTileGrid(t *testing.T) {
	tiles := dssTileGrid(10.7079, 41.3374, 44.79, 65.57)
	if len(tiles) != 2 {
		t.Fatalf("tile count = %d, want 2", len(tiles))
	}
	if math.Abs(tiles[0].Dec+tiles[1].Dec-2*41.3374) > 1e-9 {
		t.Fatalf("tile centres not symmetric: %.6f %.6f", tiles[0].Dec, tiles[1].Dec)
	}
	if tiles[0].H != 60 || tiles[0].W < 44 || tiles[0].W > 47.5 {
		t.Fatalf("tile size = %.3f x %.3f, want 60 x ~45", tiles[0].H, tiles[0].W)
	}
	if math.Abs(tiles[0].RA-tiles[1].RA) > 1e-9 {
		t.Fatalf("RA should not be tiled: %.8f %.8f", tiles[0].RA, tiles[1].RA)
	}
	one := dssTileGrid(10, 20, 10, 10)
	if len(one) != 1 || math.Abs(one[0].H-12.0) > 1e-9 || math.Abs(one[0].W-12.0) > 1e-9 {
		t.Fatalf("small field tile = %+v", one)
	}
}

func TestMergeSurveyTiles(t *testing.T) {
	const w, h = 3, 1
	a := &frame{Width: w, Height: h, Pix: []float32{1, 2, 3, 4, 5, 6, 7, 8, 9}}
	b := &frame{Width: w, Height: h, Pix: []float32{10, 20, 30, 40, 50, 60, 70, 80, 90}}
	merged, valid, invalid := mergeSurveyTiles(w, h, []tileFrame{
		{Frame: a, Valid: []bool{true, true, false}},
		{Frame: b, Valid: []bool{false, true, true}},
	})
	if invalid != 0 || len(valid) != w*h {
		t.Fatalf("merge invalid=%d valid=%d", invalid, len(valid))
	}
	want := []float32{1, 2, 3, 22, 27.5, 33, 70, 80, 90}
	for i, v := range want {
		if math.Abs(float64(merged.Pix[i]-v)) > 1e-5 {
			t.Fatalf("pixel %d = %g, want %g", i, merged.Pix[i], v)
		}
	}
	_, _, invalid = mergeSurveyTiles(w, h, []tileFrame{{Frame: a, Valid: []bool{true, false, false}}})
	if invalid != 2 {
		t.Fatalf("uncovered pixels = %d, want 2", invalid)
	}
}

func TestParseSurveyRGBSurveys(t *testing.T) {
	got, err := parseSurveyRGBSurveys(defaultSurveyRGB)
	if err != nil || len(got) != 3 || got[0] != "poss2ukstu_red" || got[1] != "quickv" || got[2] != "poss2ukstu_blue" {
		t.Fatalf("RGB surveys = %v err=%v", got, err)
	}
	if _, err := parseSurveyRGBSurveys("poss2ukstu_red,poss2ukstu_blue"); err == nil {
		t.Fatal("two-channel RGB accepted")
	}
	if _, err := parseSurveyRGBSurveys("poss2ukstu_red,bogus,poss2ukstu_blue"); err == nil {
		t.Fatal("unknown RGB survey accepted")
	}
	opts, err := parseArgs([]string{"--survey-cutout", "--survey-source", "dss", "--survey-rgb"})
	if err != nil || !opts.SurveyRGB || opts.SurveyRGBSurveys != defaultSurveyRGB {
		t.Fatalf("parseArgs = %+v err=%v", opts, err)
	}
}

func TestComposeSurveyRGB(t *testing.T) {
	const w, h = 1, 1
	mosaics := map[string]*surveyMosaic{
		"red":   {Frame: &frame{Width: w, Height: h, Pix: []float32{0.9, 0, 0}}, Valid: []bool{true}},
		"green": {Frame: &frame{Width: w, Height: h, Pix: []float32{0.5, 0, 0}}, Valid: []bool{true}},
		"blue":  {Frame: &frame{Width: w, Height: h, Pix: []float32{0.2, 0, 0}}, Valid: []bool{true}},
	}
	rgb, valid, invalid, fallbacks := composeSurveyRGB(w, h, []string{"red", "green", "blue"}, nil, mosaics)
	if invalid != 0 || len(fallbacks) != 0 || !valid[0] {
		t.Fatalf("invalid = %d valid = %v fallbacks = %v", invalid, valid, fallbacks)
	}
	want := []float32{0.9, 0.5, 0.2}
	for i, v := range want {
		if math.Abs(float64(rgb.Pix[i]-v)) > 1e-6 {
			t.Fatalf("channel %d = %g, want %g", i, rgb.Pix[i], v)
		}
	}
	mosaics["blue"].Valid[0] = false
	_, valid, invalid, _ = composeSurveyRGB(w, h, []string{"red", "green", "blue"}, nil, mosaics)
	if invalid != 1 || valid[0] {
		t.Fatalf("uncovered RGB pixel not reported")
	}
	mosaics["fallback"] = &surveyMosaic{Frame: &frame{Width: w, Height: h, Pix: []float32{0.7, 0, 0}}, Valid: []bool{true}}
	rgb, valid, invalid, fallbacks = composeSurveyRGB(w, h, []string{"red", "green", "blue"}, []string{"fallback"}, mosaics)
	if invalid != 0 || !valid[0] || fallbacks["blue"] != 1 || math.Abs(float64(rgb.Pix[2])-0.7) > 1e-6 {
		t.Fatalf("fallback invalid=%d counts=%v blue=%g", invalid, fallbacks, rgb.Pix[2])
	}
}

func TestWriteSurveyRGBFITS(t *testing.T) {
	deg := math.Pi / 180
	wc := &wcsTan{CRVAL1: 10 * deg, CRVAL2: 20 * deg, CRPIX1: 2, CRPIX2: 2,
		CD: [2][2]float64{{1e-5 * deg, 0}, {0, 1e-5 * deg}}}
	f := &frame{Width: 2, Height: 1, Pix: []float32{1, 2, 3, 4, 5, 6}}
	path := filepath.Join(t.TempDir(), "rgb.fits")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		return writeSurveyRGBFITS(out, f, wc, []string{"red", "quickv", "blue"})
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) != 2880*2 {
		t.Fatalf("FITS size = %d err=%v", len(data), err)
	}
	if string(data[:6]) != "SIMPLE" {
		t.Fatalf("missing SIMPLE card: %q", data[:6])
	}
}

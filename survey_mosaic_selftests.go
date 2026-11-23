package main

import (
	"fmt"
	"math"
)

// testSurveyRotatedFootprint locks the rotated-geometry correction: an M31
// plate solved frame is ~44.8 arcmin wide in RA and ~65.6 arcmin tall in Dec,
// not 60 x 42.2 as the old unrotated width*scale calculation reported.
func testSurveyRotatedFootprint() error {
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
		return err
	}
	if math.Abs(raSpan-44.79) > 0.2 || math.Abs(decSpan-65.57) > 0.2 {
		return fmt.Errorf("footprint %.2f x %.2f arcmin", raSpan, decSpan)
	}
	_, _, h, wd, err := surveyGeometry(w, 1997, 1318)
	if err != nil || math.Abs(h-60) > 1e-9 || math.Abs(wd-raSpan) > 1e-6 {
		return fmt.Errorf("clamped geometry h=%.2f w=%.2f err=%v", h, wd, err)
	}
	return nil
}

// testDSSMosaicTiling verifies that a 65.6 arcmin Dec span is split into two
// overlapping 60 arcmin tiles symmetric around the field centre, and that a
// small field still needs exactly one centred tile.
func testDSSMosaicTiling() error {
	tiles := dssTileGrid(10.7079, 41.3374, 44.79, 65.57)
	if len(tiles) != 2 || math.Abs(tiles[0].Dec+tiles[1].Dec-2*41.3374) > 1e-9 {
		return fmt.Errorf("two-tile grid: %+v", tiles)
	}
	if tiles[0].H != 60 || tiles[0].W < 44 || tiles[0].W > 47.5 {
		return fmt.Errorf("tile size %.2f x %.2f", tiles[0].H, tiles[0].W)
	}
	one := dssTileGrid(10, 20, 10, 10)
	if len(one) != 1 || math.Abs(one[0].H-12.0) > 1e-9 || math.Abs(one[0].W-12.0) > 1e-9 {
		return fmt.Errorf("small tile: %+v", one)
	}
	return nil
}

// testSurveyMosaicAndRGBComposition checks masked overlap averaging and that
// a documented fallback plate supplies a missing channel pixel.
func testSurveyMosaicAndRGBComposition() error {
	const w, h = 3, 1
	a := &frame{Width: w, Height: h, Pix: []float32{1, 2, 3, 4, 5, 6, 7, 8, 9}}
	b := &frame{Width: w, Height: h, Pix: []float32{10, 20, 30, 40, 50, 60, 70, 80, 90}}
	merged, valid, invalid := mergeSurveyTiles(w, h, []tileFrame{
		{Frame: a, Valid: []bool{true, true, false}},
		{Frame: b, Valid: []bool{false, true, true}},
	})
	if invalid != 0 || len(valid) != w*h || math.Abs(float64(merged.Pix[4])-27.5) > 1e-5 {
		return fmt.Errorf("merge invalid=%d middle=%g", invalid, merged.Pix[4])
	}
	mosaics := map[string]*surveyMosaic{
		"r": {Frame: &frame{Width: 1, Height: 1, Pix: []float32{0.9, 0, 0}}, Valid: []bool{true}},
		"g": {Frame: &frame{Width: 1, Height: 1, Pix: []float32{0.5, 0, 0}}, Valid: []bool{false}},
		"b": {Frame: &frame{Width: 1, Height: 1, Pix: []float32{0.2, 0, 0}}, Valid: []bool{true}},
		"f": {Frame: &frame{Width: 1, Height: 1, Pix: []float32{0.7, 0, 0}}, Valid: []bool{true}},
	}
	rgb, valid, invalid, counts := composeSurveyRGB(1, 1, []string{"r", "g", "b"}, []string{"f"}, mosaics)
	if invalid != 0 || !valid[0] || counts["g"] != 1 || math.Abs(float64(rgb.Pix[1])-0.7) > 1e-6 {
		return fmt.Errorf("RGB fallback invalid=%d counts=%v g=%g", invalid, counts, rgb.Pix[1])
	}
	return nil
}

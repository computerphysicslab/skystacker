package main

import (
	"math"
	"testing"
)

func TestBayesianOddsRealisticFOV(t *testing.T) {
	tol := 5 * math.Pi / (180 * 3600)
	realistic := bayesianLogOdds(10, 100, 1000, tol, 20)
	tooSmall := bayesianLogOdds(10, 100, 1000, tol, 0.008)
	if !(realistic > 10 && tooSmall < 0) {
		t.Fatalf("odds realistic=%.2f tiny-fov=%.2f", realistic, tooSmall)
	}
}

func TestBlindFOVAndTolerance(t *testing.T) {
	stars := []star{{X: 0, Y: 0}, {X: 1000, Y: 800}}
	fov := blindFOVSqDeg(stars, 16)
	if fov < 10 || fov > 20 {
		t.Fatalf("fov %.3f deg^2 out of range", fov)
	}
	if tol := blindTolArcsec(16); tol < 20 || tol > 30 {
		t.Fatalf("tolerance %.2f arcsec out of range", tol)
	}
	if blindTolArcsec(0) != 5 {
		t.Fatal("missing hint should keep 5 arcsec fallback")
	}
}

func TestEstimateArcsecPerPixel(t *testing.T) {
	frame := []star{{X: 0}, {X: 10}, {X: 20}, {X: 30}}
	arcsecPerPx := 2.0
	catalog := make([]gaiaStar, 4)
	for i := range catalog {
		catalog[i] = gaiaStar{RA: 1 + float64(i)*10*arcsecPerPx/206264.806, Dec: 0, MagG: 10}
	}
	got := estimateArcsecPerPixel(frame, catalog)
	if math.Abs(got-arcsecPerPx) > 0.2 {
		t.Fatalf("estimated %.3f arcsec/px, want %.3f", got, arcsecPerPx)
	}
}

func TestBlindRegionTileMargin(t *testing.T) {
	ra0, dec0 := healpixPix2AngNested(4, 0)
	region := &blindRegion{RA: ra0, Dec: dec0, Radius: 0}
	foundNear := blindRegionIncludesTile(region, 4, 0)
	foundFar := false
	for ipix := 0; ipix < 12*4*4; ipix++ {
		ra, dec := healpixPix2AngNested(4, ipix)
		sep := angularSeparation(region.RA, region.Dec, ra, dec)
		if blindRegionIncludesTile(region, 4, ipix) {
			foundNear = true
		}
		if sep > region.Radius+0.01 {
			foundFar = true
		}
	}
	if !foundNear || !foundFar {
		t.Fatalf("region margin test inconclusive: near=%v far=%v", foundNear, foundFar)
	}
	for ipix := 0; ipix < 12*4*4; ipix++ {
		ra, dec := healpixPix2AngNested(4, ipix)
		sep := angularSeparation(region.RA, region.Dec, ra, dec)
		if blindRegionIncludesTile(region, 4, ipix) && sep > region.Radius+0.02 {
			t.Fatalf("tile %d outside margin selected", ipix)
		}
	}
}

func TestBlindDownsampleScaleRule(t *testing.T) {
	cases := []struct {
		w, h int
		want int
	}{{600, 400, 1}, {1350, 900, 2}, {4000, 3000, 4}}
	for _, c := range cases {
		if got := blindDownsampleScale(c.w, c.h); got != c.want {
			t.Fatalf("%dx%d scale=%d want %d", c.w, c.h, got, c.want)
		}
	}
}

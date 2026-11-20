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

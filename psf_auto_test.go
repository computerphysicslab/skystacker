package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func makePSFFrame(w, h int, cx, cy float64, moffat bool) *frame {
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			r2 := dx*dx + dy*dy
			v := 0.02 + 0.8*math.Exp(-0.5*r2/(2.2*2.2))
			if moffat {
				v = 0.02 + 0.8*math.Pow(1+r2/(2.2*2.2), -2.5)
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	return f
}

func centroidError(f *frame, s star, moffat bool) float64 {
	samples := collectPSFSamples(f, s, 12)
	fit := fitPSFModel(samples, []float64{0.02, 0.8, s.X, s.Y, 2.2, 2.5}, moffat)
	if !fit.OK {
		return math.Inf(1)
	}
	return math.Hypot(fit.X-float64(f.Width)/2, fit.Y-float64(f.Height)/2)
}

func TestPSFAutoMoffatImprovesCentroid(t *testing.T) {
	const w, h = 64, 64
	cx, cy := 32.3, 31.7
	f := makePSFFrame(w, h, cx, cy, true)
	start := star{X: cx + 0.4, Y: cy - 0.35, HFR: 2.6, Flux: 1, Size: 40}
	gaussErr := centroidError(f, start, false)
	autoStars, report := refineCentroidsPSF(f, []star{start})
	if len(autoStars) != 1 || report == nil {
		t.Fatal("auto refinement did not run")
	}
	autoErr := math.Hypot(autoStars[0].X-cx, autoStars[0].Y-cy)
	if gaussErr <= 0 || !(autoErr < gaussErr*0.8) {
		t.Fatalf("auto error %.4f not >=20%% better than gaussian %.4f", autoErr, gaussErr)
	}
	if autoErr >= 0.05 {
		t.Fatalf("auto centroid error %.4f px too large", autoErr)
	}
	if report.Moffat != 1 {
		t.Fatalf("Moffat not selected: %+v", report)
	}
}

func TestPSFAutoGaussianSelectsGaussian(t *testing.T) {
	f := makePSFFrame(64, 64, 32.0, 32.0, false)
	start := star{X: 32.2, Y: 31.9, HFR: 2.6, Flux: 1, Size: 40}
	_, report := refineCentroidsPSF(f, []star{start})
	if report == nil || report.Gaussian == 0 {
		t.Fatalf("Gaussian not selected: %+v", report)
	}
}

func TestPSFAutoFlagAndReport(t *testing.T) {
	opts, err := parseArgs([]string{"--psf-auto"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.PSFAuto {
		t.Fatal("flag not parsed")
	}
	data, err := json.Marshal(&processingReport{PSFAutoReport: &psfAutoReport{Gaussian: 2, Moffat: 1, MedianBeta: 2.5}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"psf_auto"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

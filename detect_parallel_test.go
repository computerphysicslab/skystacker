package main

import (
	"math"
	"testing"
)

// TestDetectStarsWorkersMatchesSequential verifies that the worker-aware
// detection path produces the same star list as the sequential path when
// workers=1. This is the determinism guard for roadmap item 135.
func TestDetectStarsWorkersMatchesSequential(t *testing.T) {
	const w, h = 64, 64
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < 20; i++ {
		cx, cy := (i*17)%w, (i*23)%h
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				v := float32(0.8) * float32(1-(dx*dx+dy*dy)/6.0)
				if v < 0 {
					v = 0
				}
				idx := (y*w + x) * 3
				f.Pix[idx] += v
				f.Pix[idx+1] += v
				f.Pix[idx+2] += v
			}
		}
	}
	seq := detectStars(f, 50)
	par := detectStarsWorkers(f, 50, 1)
	if len(seq) != len(par) {
		t.Fatalf("star count: sequential %d, parallel(1) %d", len(seq), len(par))
	}
	for i := range seq {
		if math.Abs(seq[i].X-par[i].X) > 1e-9 || math.Abs(seq[i].Y-par[i].Y) > 1e-9 ||
			math.Abs(seq[i].Flux-par[i].Flux) > 1e-6 {
			t.Fatalf("star %d differs: seq=(%.3f,%.3f,%.3f) par=(%.3f,%.3f,%.3f)",
				i, seq[i].X, seq[i].Y, seq[i].Flux, par[i].X, par[i].Y, par[i].Flux)
		}
	}
	// Also verify all worker counts give identical results.
	for _, wc := range []int{2, 4, 8} {
		p := detectStarsWorkers(f, 50, wc)
		if len(p) != len(seq) {
			t.Fatalf("workers=%d: star count %d, want %d", wc, len(p), len(seq))
		}
		for i := range seq {
			if math.Abs(seq[i].X-p[i].X) > 1e-9 || math.Abs(seq[i].Y-p[i].Y) > 1e-9 {
				t.Fatalf("workers=%d: star %d position differs", wc, i)
			}
		}
	}
}

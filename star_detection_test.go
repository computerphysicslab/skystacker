package main

import (
	"math"
	"testing"
)

func findStarNear(stars []star, x, y, tol float64) (star, bool) {
	for _, s := range stars {
		if math.Abs(s.X-x) <= tol && math.Abs(s.Y-y) <= tol {
			return s, true
		}
	}
	return star{}, false
}

func TestSaturatedStarDetection(t *testing.T) {
	const w, h = 400, 400
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	const bx, by = 100.0, 100.0
	const fx, fy = 300.0, 300.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dbx := float64(x) - bx
			dby := float64(y) - by
			dfx := float64(x) - fx
			dfy := float64(y) - fy
			bright := 1.25 * math.Exp(-(dbx*dbx+dby*dby)/18)
			if bright > 1 {
				bright = 1
			}
			if dbx*dbx+dby*dby <= 121 {
				bright = 1
			}
			v := 0.05 + bright + 0.25*math.Exp(-(dfx*dfx+dfy*dfy)/8)
			if v > 1 {
				v = 1
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	stars := detectStarsSigma(f, 50, 5.0)
	bright, ok := findStarNear(stars, bx, by, 3)
	if !ok {
		t.Fatalf("saturated star not detected among %d stars", len(stars))
	}
	faint, ok := findStarNear(stars, fx, fy, 3)
	if !ok {
		t.Fatalf("faint control star not detected among %d stars", len(stars))
	}
	if bright.Flux <= faint.Flux {
		t.Fatalf("saturated flux %.3f <= faint flux %.3f", bright.Flux, faint.Flux)
	}
}

func TestSaturatedStarWithSpikesDetection(t *testing.T) {
	const w, h = 400, 400
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	const bx, by = 200.0, 200.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - bx
			dy := float64(y) - by
			d2 := dx*dx + dy*dy
			v := 0.05
			if d2 <= 121 {
				v += 1
			} else {
				v += 0.95 * math.Exp(-d2/18)
			}
			spike := math.Abs(dx)
			if math.Abs(dy) < spike {
				spike = math.Abs(dy)
			}
			if spike <= 25 && d2 > 121 {
				v += 0.18 * (1 - float64(spike)/26)
			}
			if v > 1 {
				v = 1
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	stars := detectStarsSigma(f, 50, 5.0)
	bright, ok := findStarNear(stars, bx, by, 2)
	if !ok {
		t.Fatalf("saturated star with spikes not detected among %d stars", len(stars))
	}
	if bright.Ecc > 0.6 || bright.HFR > 12 {
		t.Fatalf("core metrics displaced by spikes: ecc=%.3f hfr=%.3f", bright.Ecc, bright.HFR)
	}
	filtered := filterStarsForMatching([]star{bright})
	if len(filtered) != 1 {
		t.Fatalf("bright star rejected by blind-match filter: %+v", bright)
	}
}

func TestBrightStarSurvivesFluxTruncation(t *testing.T) {
	const w, h = 420, 420
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	const bx, by = 210.0, 210.0
	faint := make([]struct{ x, y float64 }, 0, 64)
	for gy := 30; gy < h-20; gy += 45 {
		for gx := 30; gx < w-20; gx += 45 {
			if math.Hypot(float64(gx)-bx, float64(gy)-by) < 40 {
				continue
			}
			faint = append(faint, struct{ x, y float64 }{float64(gx), float64(gy)})
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 0.05
			dbx := float64(x) - bx
			dby := float64(y) - by
			if dbx*dbx+dby*dby <= 121 {
				v += 1
			} else {
				v += 0.95 * math.Exp(-(dbx*dbx+dby*dby)/18)
			}
			for _, s := range faint {
				d2 := (float64(x)-s.x)*(float64(x)-s.x) + (float64(y)-s.y)*(float64(y)-s.y)
				v += 0.22 * math.Exp(-d2/4)
			}
			if v > 1 {
				v = 1
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	stars := detectStarsSigma(f, 20, 5.0)
	if len(stars) != 20 {
		t.Fatalf("expected 20 stars, got %d", len(stars))
	}
	if _, ok := findStarNear(stars, bx, by, 3); !ok {
		t.Fatalf("brightest star dropped by flux truncation: first=%+v", stars[0])
	}
}

package main

import (
	"math"
	"sort"
)

type psfFit struct {
	Model string
	X     float64
	Y     float64
	Sigma float64
	Beta  float64
	RMS   float64
	OK    bool
}

type psfAutoReport struct {
	Gaussian   int     `json:"gaussian"`
	Moffat     int     `json:"moffat"`
	MedianBeta float64 `json:"median_beta,omitempty"`
}

type psfSample struct {
	x, y float64
	v    float64
}

func psfModelValue(x, y float64, p []float64, moffat bool) float64 {
	dx, dy := x-p[2], y-p[3]
	r2 := dx*dx + dy*dy
	sigma := math.Max(0.3, p[4])
	if !moffat {
		return p[0] + p[1]*math.Exp(-0.5*r2/(sigma*sigma))
	}
	beta := math.Max(1, math.Min(8, p[5]))
	return p[0] + p[1]*math.Pow(1+r2/(sigma*sigma), -beta)
}

func fitPSFModel(samples []psfSample, start []float64, moffat bool) psfFit {
	n := 5
	if moffat {
		n = 6
	}
	p := append([]float64(nil), start...)
	if len(p) < n {
		return psfFit{}
	}
	p = p[:n]
	lambda := 1e-3
	for iter := 0; iter < 40; iter++ {
		jac := make([][]float64, len(samples))
		res := make([]float64, len(samples))
		for i, s := range samples {
			pred := psfModelValue(s.x, s.y, p, moffat)
			res[i] = pred - s.v
			jac[i] = make([]float64, n)
			for j := range n {
				step := 1e-4 * (1 + math.Abs(p[j]))
				q := append([]float64(nil), p...)
				q[j] += step
				jac[i][j] = (psfModelValue(s.x, s.y, q, moffat) - pred) / step
			}
		}
		ata := make([][]float64, n)
		atb := make([]float64, n)
		for i := range n {
			ata[i] = make([]float64, n)
			for j := range n {
				sum := 0.0
				for k := range samples {
					sum += jac[k][i] * jac[k][j]
				}
				ata[i][j] = sum
			}
			if i == i {
				ata[i][i] += lambda
			}
			sum := 0.0
			for k := range samples {
				sum += jac[k][i] * res[k]
			}
			atb[i] = -sum
		}
		delta := solveLinear(ata, atb, n)
		if len(delta) != n {
			break
		}
		oldRMS := psfFitRMS(samples, p, moffat)
		for j := range n {
			p[j] += delta[j]
		}
		p[1] = math.Max(0, p[1])
		p[2] = math.Max(0, p[2])
		p[3] = math.Max(0, p[3])
		p[4] = math.Max(0.3, math.Min(64, p[4]))
		if moffat {
			p[5] = math.Max(1, math.Min(8, p[5]))
		}
		newRMS := psfFitRMS(samples, p, moffat)
		if newRMS <= oldRMS {
			lambda = math.Max(1e-6, lambda*0.5)
		} else {
			for j := range n {
				p[j] -= delta[j]
			}
			lambda *= 4
			if lambda > 1e6 {
				break
			}
		}
	}
	fit := psfFit{Model: "gaussian", X: p[2], Y: p[3], Sigma: p[4], RMS: psfFitRMS(samples, p, moffat)}
	if moffat {
		fit.Model, fit.Beta = "moffat", p[5]
	}
	fit.OK = len(samples) >= 8 && fit.RMS >= 0 && !math.IsNaN(fit.RMS)
	return fit
}

func psfFitRMS(samples []psfSample, p []float64, moffat bool) float64 {
	if len(samples) == 0 {
		return math.Inf(1)
	}
	sum := 0.0
	for _, s := range samples {
		d := psfModelValue(s.x, s.y, p, moffat) - s.v
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func collectPSFSamples(f *frame, s star, radius int) []psfSample {
	samples := make([]psfSample, 0, (2*radius+1)*(2*radius+1))
	x0, y0 := int(math.Floor(s.X)), int(math.Floor(s.Y))
	for y := y0 - radius; y <= y0+radius; y++ {
		if y < 0 || y >= f.Height {
			continue
		}
		for x := x0 - radius; x <= x0+radius; x++ {
			if x < 0 || x >= f.Width {
				continue
			}
			v := float64(f.Pix[(y*f.Width+x)*3])
			if v < 0 {
				continue
			}
			samples = append(samples, psfSample{x: float64(x), y: float64(y), v: v})
		}
	}
	return samples
}

func refineCentroidsPSF(f *frame, stars []star) ([]star, *psfAutoReport) {
	report := &psfAutoReport{}
	betas := make([]float64, 0, len(stars))
	out := append([]star(nil), stars...)
	for i, s := range stars {
		radius := int(math.Ceil(2*s.HFR + 3))
		if radius < 5 {
			radius = 5
		}
		if radius > 14 {
			radius = 14
		}
		samples := collectPSFSamples(f, s, radius)
		if len(samples) < 12 {
			continue
		}
		bg := 0.0
		peak := 0.0
		for _, p := range samples {
			if p.v < peak {
				bg += p.v
			}
			if p.v > peak {
				peak = p.v
			}
		}
		bg /= float64(len(samples))
		sigma := math.Max(0.8, s.HFR/1.18)
		start := []float64{bg, peak - bg, s.X, s.Y, sigma, 2.5}
		g := fitPSFModel(samples, start, false)
		m := fitPSFModel(samples, start, true)
		if !g.OK && !m.OK {
			continue
		}
		best := g
		if m.OK && (!g.OK || m.RMS < g.RMS) {
			best = m
		}
		out[i].X, out[i].Y = best.X, best.Y
		if best.Model == "moffat" {
			report.Moffat++
			betas = append(betas, best.Beta)
		} else {
			report.Gaussian++
		}
	}
	if len(betas) > 0 {
		sort.Float64s(betas)
		report.MedianBeta = betas[len(betas)/2]
	}
	return out, report
}

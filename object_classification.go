package main

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// This file implements roadmap item 89: report-only detection and
// classification of diffuse sources. Stars are masked using the existing
// detector, then connected components of the multiscale background residual
// are measured with the standard non-parametric morphology descriptors
// (CAS, Gini, M20) and labelled with IVOA object-type terms.

type objectMorphology struct {
	ID            int     `json:"id"`
	X             float64 `json:"x_px"`
	Y             float64 `json:"y_px"`
	Area          int     `json:"area_px"`
	Flux          float64 `json:"flux"`
	Concentration float64 `json:"concentration"`
	Asymmetry     float64 `json:"asymmetry"`
	Smoothness    float64 `json:"smoothness"`
	Gini          float64 `json:"gini"`
	M20           float64 `json:"m20"`
	Ellipticity   float64 `json:"ellipticity"`
	ObjectType    string  `json:"object_type"`
	ClassUCD      string  `json:"class_ucd"`
}

type objectClassificationReport struct {
	Image          string             `json:"image"`
	Width          int                `json:"width"`
	Height         int                `json:"height"`
	Standards      map[string]string  `json:"standards"`
	ThresholdSigma float64            `json:"threshold_sigma"`
	StarMaskPixels int                `json:"star_mask_pixels"`
	Count          int                `json:"count"`
	Objects        []objectMorphology `json:"objects"`
}

type diffusePixel struct {
	x, y int
	flux float64
}

func connectedDiffusePixels(gray []float64, mask []bool, w, h int, threshold float64) [][]diffusePixel {
	visited := make([]bool, w*h)
	var out [][]diffusePixel
	for seed := 0; seed < w*h; seed++ {
		if visited[seed] || mask[seed] || gray[seed] < threshold {
			continue
		}
		component := make([]diffusePixel, 0, 64)
		stack := []int{seed}
		visited[seed] = true
		for len(stack) > 0 {
			idx := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, diffusePixel{idx % w, idx / w, gray[idx]})
			for _, d := range []int{-1, 1, -w, w, -w - 1, -w + 1, w - 1, w + 1} {
				n := idx + d
				if n < 0 || n >= w*h || visited[n] || mask[n] || gray[n] < threshold {
					continue
				}
				nx, ny := n%w, n/w
				x, y := idx%w, idx/w
				if math.Abs(float64(nx-x)) > 1 || math.Abs(float64(ny-y)) > 1 {
					continue
				}
				visited[n] = true
				stack = append(stack, n)
			}
		}
		if len(component) >= 32 {
			out = append(out, component)
		}
	}
	return out
}

func morphologyOf(component []diffusePixel, w, h int) objectMorphology {
	var total, cx, cy float64
	for _, p := range component {
		total += p.flux
		cx += float64(p.x) * p.flux
		cy += float64(p.y) * p.flux
	}
	if total <= 0 {
		return objectMorphology{}
	}
	cx, cy = cx/total, cy/total
	radii := make([]struct {
		r, flux float64
	}, len(component))
	var ixx, iyy, ixy, mTot float64
	for i, p := range component {
		dx, dy := float64(p.x)-cx, float64(p.y)-cy
		r := math.Hypot(dx, dy)
		radii[i] = struct{ r, flux float64 }{r, p.flux}
		ixx += p.flux * dx * dx
		iyy += p.flux * dy * dy
		ixy += p.flux * dx * dy
		mTot += p.flux * (dx*dx + dy*dy)
	}
	sort.Slice(radii, func(i, j int) bool { return radii[i].r < radii[j].r })
	r20, r80 := radii[len(radii)/5].r, radii[len(radii)*4/5].r
	concentration := 5 * math.Log10((r80+1)/(r20+1))
	// Asymmetry against the 180-degree-rotated component.
	fluxAt := make(map[[2]int]float64, len(component))
	for _, p := range component {
		fluxAt[[2]int{p.x, p.y}] += p.flux
	}
	var asym float64
	for _, p := range component {
		rx, ry := int(math.Round(2*cx))-p.x, int(math.Round(2*cy))-p.y
		other := fluxAt[[2]int{rx, ry}]
		asym += math.Abs(p.flux - other)
	}
	asym /= total
	// Smoothness against the local four-neighbour mean.
	var smooth float64
	for _, p := range component {
		var sum float64
		var n int
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if v, ok := fluxAt[[2]int{p.x + d[0], p.y + d[1]}]; ok {
				sum += v
				n++
			}
		}
		if n > 0 {
			smooth += math.Abs(p.flux - sum/float64(n))
		}
	}
	smooth /= total
	// Gini on ascending fluxes.
	fluxes := make([]float64, len(component))
	for i, p := range component {
		fluxes[i] = p.flux
	}
	sort.Float64s(fluxes)
	var giniNum float64
	for i, f := range fluxes {
		giniNum += float64(2*i-len(fluxes)+1) * f
	}
	gini := giniNum / (float64(len(fluxes)-1) * total)
	// M20: second moment of the brightest 20% of the flux.
	byFlux := append([]diffusePixel{}, component...)
	sort.Slice(byFlux, func(i, j int) bool { return byFlux[i].flux > byFlux[j].flux })
	var cumFlux, m20 float64
	for _, p := range byFlux {
		if cumFlux >= 0.2*total {
			break
		}
		cumFlux += p.flux
		dx, dy := float64(p.x)-cx, float64(p.y)-cy
		m20 += p.flux * (dx*dx + dy*dy)
	}
	m20Value := math.Log10((m20 + 1) / (mTot + 1))
	ell := math.Sqrt(math.Pow(ixx-iyy, 2)+4*ixy*ixy) / (ixx + iyy + 1e-9)
	return objectMorphology{X: cx, Y: cy, Area: len(component), Flux: total,
		Concentration: concentration, Asymmetry: asym, Smoothness: smooth,
		Gini: gini, M20: m20Value, Ellipticity: ell}
}

// classifyObjectType maps morphology onto IVOA object-type terms. The
// thresholds are deliberately conservative: ambiguous cases are reported as
// "unknown" rather than over-classified.
func classifyObjectType(m objectMorphology) string {
	switch {
	case m.Area < 24:
		return "unknown"
	case m.Asymmetry > 0.35 || m.Ellipticity > 0.45:
		// Likely a deblending residual or a masked-star wing, not a source.
		return "unknown"
	case m.Gini > 0.45 && m.Concentration > 0.8:
		return "galaxy"
	case m.Area >= 400:
		return "nebula"
	default:
		return "unknown"
	}
}

func classifyObjects(imagePath string, f *frame, thresholdSigma float64) (objectClassificationReport, error) {
	rep := objectClassificationReport{Image: filepath.Base(imagePath), Width: f.Width, Height: f.Height,
		Standards: map[string]string{
			"object_type": "IVOA object-type vocabulary (draft, SIMBAD/ObsCore)",
			"class_ucd":   "src.class",
			"morph_ucd":   "src.morph.param",
		}, ThresholdSigma: thresholdSigma, Objects: []objectMorphology{}}
	if thresholdSigma <= 0 {
		thresholdSigma = 3
	}
	residual := multiscaleBackground(f, 64)
	if residual == nil {
		return rep, errors.New("diffuse classification: background residual failed")
	}
	gray := grayscale(residual)
	vals := make([]float64, len(gray))
	for i, v := range gray {
		vals[i] = float64(v)
	}
	sigma := estimateResidualNoise(gray)
	if sigma <= 1e-9 {
		sigma = 1e-6
	}
	threshold := thresholdSigma * sigma
	mask := make([]bool, f.Width*f.Height)
	for _, s := range detectStars(f, 300) {
		radius := math.Max(6, 4*s.HFR+2)
		for y := max(0, int(s.Y-radius)); y <= min(f.Height-1, int(s.Y+radius)); y++ {
			for x := max(0, int(s.X-radius)); x <= min(f.Width-1, int(s.X+radius)); x++ {
				if math.Hypot(float64(x)-s.X, float64(y)-s.Y) <= radius {
					mask[y*f.Width+x] = true
				}
			}
		}
	}
	for _, m := range mask {
		if m {
			rep.StarMaskPixels++
		}
	}
	components := connectedDiffusePixels(vals, mask, f.Width, f.Height, threshold)
	for _, c := range components {
		m := morphologyOf(c, f.Width, f.Height)
		if m.Flux <= 0 {
			continue
		}
		m.ObjectType = classifyObjectType(m)
		m.ClassUCD = "src.class"
		rep.Objects = append(rep.Objects, m)
	}
	sort.Slice(rep.Objects, func(i, j int) bool { return rep.Objects[i].Flux > rep.Objects[j].Flux })
	for i := range rep.Objects {
		rep.Objects[i].ID = i + 1
	}
	if len(rep.Objects) > 200 {
		rep.Objects = rep.Objects[:200]
	}
	rep.Count = len(rep.Objects)
	return rep, nil
}

func writeObjectClassification(path string, rep objectClassificationReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, func(out io.Writer) error {
		_, err := out.Write(data)
		return err
	})
}

package main

import (
	"math"
	"sort"
)

// This file contains the detection and verification stages of the efficient
// native blind solver (roadmap items 78 and 80). The index format itself lives
// in blind_index.go. Everything here is stdlib-only and side-effect free apart
// from the package-level matcher configuration flags.

// filterStarsForMatching removes detections that cannot be reliable stellar
// centroids: single-pixel spikes (hot pixels, cosmic rays), severely bloated
// blobs and elongated nebular knots or trails. It is deliberately conservative
// so that a real star is never dropped; false detections are rejected later by
// the geometric verifier.
func filterStarsForMatching(stars []star) []star {
	out := make([]star, 0, len(stars))
	for _, s := range stars {
		if s.Size < 2 || s.HFR < 0.4 || s.HFR > 12 || s.Ecc > 0.85 {
			continue
		}
		if s.X < 0 || s.Y < 0 || math.IsNaN(s.X) || math.IsNaN(s.Y) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// detrendedBlindStars extracts clean, flux-ranked centroids for blind solving.
// Processed JPEG/PNG frames contain compression noise and nebular structure;
// detection therefore runs on a downsampled, background-equalised scratch copy
// and centroids are mapped back to the original pixel frame.
func blindDownsampleScale(width, height int) int {
	if max(width, height) > 3000 {
		return 4
	}
	if width > 1280 || height > 960 {
		return 2
	}
	return 1
}

func detrendedBlindStars(f *frame, maxStars int) []star {
	if f == nil || f.Width < 16 || f.Height < 16 || len(f.Pix) < f.Width*f.Height*3 {
		return nil
	}
	if maxStars <= 0 {
		maxStars = 200
	}
	// Downsampling suppresses JPEG noise and improves star SNR before
	// detection; large frames additionally bound peak memory.
	scale := blindDownsampleScale(f.Width, f.Height)
	work := f
	if scale > 1 {
		work = resizeFrame(f, f.Width/scale, f.Height/scale)
	}
	plane := asymmetricBackgroundPlane(work, 2, 2, 3.0, 1.5, false)
	scratch := &frame{Width: work.Width, Height: work.Height, Pix: append([]float32{}, work.Pix...)}
	if plane != nil {
		for p := 0; p < work.Width*work.Height; p++ {
			for c := 0; c < 3; c++ {
				scratch.Pix[p*3+c] = max(0, scratch.Pix[p*3+c]-plane[p])
			}
		}
	}
	stars := detectStarsSigma(scratch, maxStars*2, 4.0)
	if len(stars) < 10 {
		stars = detectStarsWithDetrend(scratch, maxStars*2)
	}
	stars = filterStarsForMatching(stars)
	if scale > 1 {
		for i := range stars {
			stars[i].X *= float64(scale)
			stars[i].Y *= float64(scale)
			stars[i].HFR *= float64(scale)
			stars[i].Flux *= float64(scale * scale)
		}
	}
	sort.SliceStable(stars, func(i, j int) bool { return stars[i].Flux > stars[j].Flux })
	if len(stars) > maxStars {
		stars = stars[:maxStars]
	}
	return stars
}

// bayesianLogOdds compares a foreground model (real star correspondences
// within tolerance) against a uniform random-background model. It is the
// acceptance statistic used by Astrometry.net: expected random matches are
// nFrame * nCatalog * pi * tol^2 / FOV_area, so a genuine alignment has a
// large positive log-odds while descriptor collisions stay near zero or below.
func bayesianLogOdds(matches, nFrame, nCatalog int, toleranceRad, fovSqDeg float64) float64 {
	if matches <= 0 || nFrame <= 0 || nCatalog <= 0 || toleranceRad <= 0 || fovSqDeg <= 0 {
		return 0
	}
	area := fovSqDeg * math.Pi / 180.0 * (math.Pi / 180.0) // steradians
	expected := float64(nFrame) * float64(nCatalog) * math.Pi * toleranceRad * toleranceRad / area
	if expected < 1e-9 {
		expected = 1e-9
	}
	// Poisson log-likelihood ratio plus a weak Occam penalty for the four
	// fitted similarity parameters.
	return float64(matches)*math.Log(float64(matches)/expected) - expected - 4*math.Log(10)
}

// scaleConsistentMatches keeps only quad correspondences whose implied pixel
// scale agrees with the median within maxDeviation. ASTAP uses the same gate
// to reject descriptor collisions that happen to share a quad shape but not a
// common scale.
func scaleConsistentMatches(matches []matchPair, frameStars []star, catalog []gaiaStar, maxDeviation float64) []matchPair {
	if len(matches) < 3 {
		return matches
	}
	if maxDeviation <= 0 || maxDeviation > 1 {
		maxDeviation = 0.35
	}
	// Every pair of correspondences implies a pixel scale: angular separation
	// of the catalogue stars divided by their pixel separation. The median of
	// those estimates is robust to a minority of false quads.
	scales := make([]float64, 0, len(matches)*len(matches)/2)
	for i := 0; i < len(matches); i++ {
		a := matches[i]
		_, ga := matchStar(a, frameStars, catalog)
		if ga == nil {
			continue
		}
		for j := i + 1; j < len(matches); j++ {
			b := matches[j]
			_, gb := matchStar(b, frameStars, catalog)
			if gb == nil {
				continue
			}
			dPix := math.Hypot(frameStars[a.FrameIdx].X-frameStars[b.FrameIdx].X,
				frameStars[a.FrameIdx].Y-frameStars[b.FrameIdx].Y)
			if dPix < 1 {
				continue
			}
			dSky := angularSeparation(ga.RA, ga.Dec, gb.RA, gb.Dec)
			if dSky > 0 {
				scales = append(scales, dSky/dPix)
			}
		}
	}
	if len(scales) < 3 {
		return matches
	}
	sort.Float64s(scales)
	median := scales[len(scales)/4]
	out := make([]matchPair, 0, len(matches))
	for _, m := range matches {
		_, ga := matchStar(m, frameStars, catalog)
		_, gb := matchStar(matches[0], frameStars, catalog)
		if ga == nil || gb == nil {
			continue
		}
		dPix := math.Hypot(frameStars[m.FrameIdx].X-frameStars[matches[0].FrameIdx].X,
			frameStars[m.FrameIdx].Y-frameStars[matches[0].FrameIdx].Y)
		if dPix < 1 {
			out = append(out, m)
			continue
		}
		dSky := angularSeparation(ga.RA, ga.Dec, gb.RA, gb.Dec)
		scale := dSky / dPix
		if median <= 0 || math.Abs(scale-median)/median <= maxDeviation {
			out = append(out, m)
		}
	}

	return out
}

// matchStar resolves a match pair into its frame star and catalogue star.
func matchStar(m matchPair, frameStars []star, catalog []gaiaStar) (*star, *gaiaStar) {
	if m.FrameIdx < 0 || m.FrameIdx >= len(frameStars) || m.GaiaIdx < 0 || m.GaiaIdx >= len(catalog) {
		return nil, nil
	}
	return &frameStars[m.FrameIdx], &catalog[m.GaiaIdx]
}

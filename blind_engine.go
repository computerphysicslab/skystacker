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
func estimateArcsecPerPixel(stars []star, catalog []gaiaStar) float64 {
	if len(stars) < 4 || len(catalog) < 4 {
		return 0
	}
	if len(catalog) > 200 {
		sample := append([]gaiaStar(nil), catalog...)
		sort.Slice(sample, func(i, j int) bool { return sample[i].MagG < sample[j].MagG })
		catalog = sample[:200]
	}
	frameSpacing := medianNeighbourSeparationStars(stars)
	if frameSpacing <= 0 {
		return 0
	}
	var ra, dec float64
	for _, g := range catalog {
		ra += g.RA
		dec += g.Dec
	}
	ra /= float64(len(catalog))
	dec /= float64(len(catalog))
	points := make([]point2D, 0, len(catalog))
	for _, g := range catalog {
		points = append(points, tangentXY(ra, dec, g.RA, g.Dec))
	}
	catalogSpacing := medianNeighbourSeparationPoints(points)
	if catalogSpacing <= 0 {
		return 0
	}
	arcsec := catalogSpacing / frameSpacing * 206264.806
	return math.Max(0.05, math.Min(60, arcsec))
}

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
// scale agrees with the dominant scale cluster within maxDeviation. ASTAP uses
// the same gate to reject descriptor collisions that happen to share a quad
// shape but not a common scale.
//
// The naive version used scales[len/4] (a quartile, not a median) and anchored
// every comparison to matches[0]. When the anchor itself was a false match,
// all genuine correspondences were rejected. Here each match gets a robust
// representative from its own pairwise scales, the largest tolerance cluster
// of representatives selects the consensus, and a member of that cluster
// becomes the anchor. It runs in O(n^2 log n) time with O(n) live scratch
// space; the number of input correspondences is small (one quad hit).
func scaleConsistentMatches(matches []matchPair, frameStars []star, catalog []gaiaStar, maxDeviation float64) []matchPair {
	if len(matches) < 3 {
		return matches
	}
	if maxDeviation <= 0 || maxDeviation > 1 {
		maxDeviation = 0.35
	}
	// Cache index lookups once; matchStar is a bounds check plus a slice index.
	gaia := make([]*gaiaStar, len(matches))
	px, py := make([]float64, len(matches)), make([]float64, len(matches))
	for i, m := range matches {
		_, g := matchStar(m, frameStars, catalog)
		gaia[i] = g
		px[i], py[i] = math.NaN(), math.NaN()
		if m.FrameIdx >= 0 && m.FrameIdx < len(frameStars) {
			px[i], py[i] = frameStars[m.FrameIdx].X, frameStars[m.FrameIdx].Y
		}
	}

	// A scale vote needs at least two other correspondences to be meaningful.
	// Count usable pairs without retaining the O(n^2) scale matrix: the robust
	// representatives below recompute the same separations with O(n) scratch.
	validPairs := 0
	for i := range matches {
		if gaia[i] == nil || math.IsNaN(px[i]) {
			continue
		}
		for j := i + 1; j < len(matches); j++ {
			if gaia[j] == nil || math.IsNaN(px[j]) {
				continue
			}
			if math.Hypot(px[i]-px[j], py[i]-py[j]) < 1 {
				continue
			}
			if angularSeparation(gaia[i].RA, gaia[i].Dec, gaia[j].RA, gaia[j].Dec) > 0 {
				validPairs++
			}
		}
	}
	if validPairs < 3 {
		return matches
	}

	// A representative per match: the median of the scales measured against
	// all other matches. With a single false match among genuine ones, every
	// genuine representative is unaffected while the false one is isolated.
	type scaleVote struct {
		scale float64
		index int
	}
	votes := make([]scaleVote, 0, len(matches))
	local := make([]float64, 0, len(matches)-1)
	for i := range matches {
		if gaia[i] == nil || math.IsNaN(px[i]) {
			continue
		}
		local = local[:0]
		for j := range matches {
			if i == j || gaia[j] == nil || math.IsNaN(px[j]) {
				continue
			}
			dPix := math.Hypot(px[i]-px[j], py[i]-py[j])
			if dPix < 1 {
				continue
			}
			dSky := angularSeparation(gaia[i].RA, gaia[i].Dec, gaia[j].RA, gaia[j].Dec)
			if dSky > 0 {
				local = append(local, dSky/dPix)
			}
		}
		if len(local) == 0 {
			continue
		}
		sort.Float64s(local)
		votes = append(votes, scaleVote{scale: local[len(local)/2], index: i})
	}
	if len(votes) == 0 {
		return matches
	}
	sort.Slice(votes, func(i, j int) bool {
		if votes[i].scale == votes[j].scale {
			return votes[i].index < votes[j].index
		}
		return votes[i].scale < votes[j].scale
	})

	// Choose the largest tolerance cluster. Ties prefer the cluster with the
	// lower first match index so the result stays deterministic.
	bestStart, bestEnd := 0, 1
	for start := range votes {
		if votes[start].scale <= 0 {
			continue
		}
		end := start + 1
		for end < len(votes) &&
			math.Abs(votes[end].scale-votes[start].scale)/votes[start].scale <= maxDeviation {
			end++
		}
		if end-start > bestEnd-bestStart ||
			(end-start == bestEnd-bestStart && votes[start].index < votes[bestStart].index) {
			bestStart, bestEnd = start, end
		}
	}
	consensus := votes[bestStart].scale

	// Anchor inside the consensus cluster, closest to its centre, so a false
	// first match can never define the gate.
	anchor := votes[bestStart].index
	bestDelta := math.Abs(votes[bestStart].scale - consensus)
	for _, v := range votes[bestStart:bestEnd] {
		if delta := math.Abs(v.scale - consensus); delta < bestDelta ||
			(delta == bestDelta && v.index < anchor) {
			anchor, bestDelta = v.index, delta
		}
	}
	if gaia[anchor] == nil {
		return matches
	}
	out := make([]matchPair, 0, len(matches))
	for i, m := range matches {
		if gaia[i] == nil || math.IsNaN(px[i]) {
			continue
		}
		dPix := math.Hypot(px[i]-px[anchor], py[i]-py[anchor])
		if dPix < 1 {
			out = append(out, m)
			continue
		}
		dSky := angularSeparation(gaia[i].RA, gaia[i].Dec, gaia[anchor].RA, gaia[anchor].Dec)
		scale := dSky / dPix
		if consensus <= 0 || math.Abs(scale-consensus)/consensus <= maxDeviation {
			out = append(out, m)
		}
	}
	return out
}

func matchStar(m matchPair, frameStars []star, catalog []gaiaStar) (*star, *gaiaStar) {
	if m.FrameIdx < 0 || m.FrameIdx >= len(frameStars) || m.GaiaIdx < 0 || m.GaiaIdx >= len(catalog) {
		return nil, nil
	}
	return &frameStars[m.FrameIdx], &catalog[m.GaiaIdx]
}

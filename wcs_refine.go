package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type wcsRefineReport struct {
	RMSBeforePx float64 `json:"rms_before_px"`
	RMSAfterPx  float64 `json:"rms_after_px"`
	NMatches    int     `json:"n_matches"`
	NOutliers   int     `json:"n_outliers"`
	NIterations int     `json:"n_iterations"`
}

type refinePair struct {
	frame star
	gaia  gaiaStar
	res   float64
}

func refinePairs(w *wcsTan, width, height int, stars []star, gaia []gaiaStar, tolPx float64) []refinePair {
	used := make([]bool, len(stars))
	out := make([]refinePair, 0, len(gaia))
	for _, g := range gaia {
		x, y := wcsTanForward(w, g.RA, g.Dec)
		if x < 0 || y < 0 || x >= float64(width) || y >= float64(height) {
			continue
		}
		best, idx, bestDist := tolPx, -1, tolPx
		for i, s := range stars {
			if used[i] {
				continue
			}
			if d := math.Hypot(s.X-x, s.Y-y); d < best {
				best, idx, bestDist = d, i, d
			}
		}
		if idx < 0 {
			continue
		}
		used[idx] = true
		out = append(out, refinePair{frame: stars[idx], gaia: g, res: bestDist})
	}
	return out
}

func refineRMS(w *wcsTan, pairs []refinePair) float64 {
	if len(pairs) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range pairs {
		x, y := wcsTanForward(w, p.gaia.RA, p.gaia.Dec)
		d := math.Hypot(x-p.frame.X, y-p.frame.Y)
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(pairs)))
}

func computeWCSRefine(w *wcsTan, width, height int, stars []star, gaia []gaiaStar, tolPx float64) (wcsTan, wcsRefineReport) {
	work := *w
	pairs := refinePairs(&work, width, height, stars, gaia, tolPx)
	report := wcsRefineReport{RMSBeforePx: refineRMS(&work, pairs), NMatches: len(pairs)}
	if len(pairs) < 4 {
		report.RMSAfterPx = report.RMSBeforePx
		return work, report
	}
	inliers := pairs
	for iter := 0; iter < 3; iter++ {
		report.NIterations++
		residuals := make([]float64, len(inliers))
		for i, p := range inliers {
			x, y := wcsTanForward(&work, p.gaia.RA, p.gaia.Dec)
			residuals[i] = math.Hypot(x-p.frame.X, y-p.frame.Y)
		}
		sorted := append([]float64(nil), residuals...)
		sort.Float64s(sorted)
		median := sorted[len(sorted)/2]
		dev := make([]float64, len(sorted))
		for i, v := range sorted {
			dev[i] = math.Abs(v - median)
		}
		sort.Float64s(dev)
		sigma := 1.4826 * dev[len(dev)/2]
		limit := math.Max(3*sigma, math.Max(1.25*median, 2.0))
		kept := make([]refinePair, 0, len(inliers))
		for i, p := range inliers {
			if residuals[i] <= limit {
				kept = append(kept, p)
			}
		}
		if len(kept) < 4 || len(kept) == len(inliers) {
			inliers = kept
			break
		}
		inliers = kept
	}
	report.NOutliers = len(pairs) - len(inliers)
	if len(inliers) < 4 {
		report.RMSAfterPx = report.RMSBeforePx
		return work, report
	}
	frameStars := make([]star, len(inliers))
	gaiaIn := make([]gaiaStar, len(inliers))
	for i, p := range inliers {
		frameStars[i], gaiaIn[i] = p.frame, p.gaia
	}
	refined, err := fitWCSFromMatches(frameStars, gaiaIn)
	if err != nil {
		logf("WCS refinement fit failed: %v", err)
	}
	if err == nil {
		refined.SIP = append([]float64(nil), w.SIP...)
		work = refined
	}
	report.NMatches = len(inliers)
	report.RMSAfterPx = refineRMS(&work, inliers)
	return work, report
}

func runWCSRefine(sol *plateSolution, outputDir string, opts runOptions) (*wcsRefineReport, error) {
	if sol == nil {
		return nil, nil
	}
	w := sol.WCS
	w.SIP = sol.SIP
	width, height, pix, err := readStackFloat32(outputDir)
	if err != nil {
		return nil, err
	}
	stars := detectStars(&frame{Width: width, Height: height, Pix: pix}, 300)
	cd := math.Hypot(w.CD[0][0], w.CD[1][0])
	if cd <= 0 {
		cd = 1e-5
	}
	radiusDeg := math.Hypot(float64(width), float64(height)) * cd * 180 / math.Pi * 0.6
	nside := opts.GaiaNside
	if nside <= 0 {
		nside = 64
	}
	gaia := cachedGaiaStarsAround(gaiaDefaultCacheDir(), nside, w.CRVAL1, w.CRVAL2, radiusDeg*math.Pi/180)
	refined, report := computeWCSRefine(&w, width, height, stars, gaia, 3)
	sol.WCS = refined
	sol.WCS.SIP = append([]float64(nil), sol.SIP...)
	sol.RMSBeforePx, sol.RMSAfterPx = report.RMSBeforePx, report.RMSAfterPx
	data, err := json.Marshal(sol)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "plate_solution.json"), data, 0o644); err != nil {
		return nil, err
	}
	return &report, nil
}

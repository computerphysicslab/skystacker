package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func verificationFixture() (*wcsTan, []star, []gaiaStar) {
	w := &wcsTan{
		CRVAL1: 10 * math.Pi / 180,
		CRVAL2: 20 * math.Pi / 180,
		CRPIX1: 256,
		CRPIX2: 256,
		CD:     [2][2]float64{{-1e-5, 0}, {0, 1e-5}},
	}
	var stars []star
	var gaia []gaiaStar
	for y := 32; y < 512; y += 48 {
		for x := 32; x < 512; x += 48 {
			ra, dec := wcsTanInverse(w, float64(x), float64(y))
			stars = append(stars, star{X: float64(x), Y: float64(y), Flux: 1, Size: 4, HFR: 2})
			gaia = append(gaia, gaiaStar{RA: ra, Dec: dec, MagG: 12})
		}
	}
	return w, stars, gaia
}

func TestVerifySolutionPerfectSynthetic(t *testing.T) {
	w, stars, gaia := verificationFixture()
	report := computeSolutionVerification(w, 512, 512, stars, gaia, 3)
	if report.GlobalRMSPx >= 0.05 {
		t.Fatalf("RMS %.6f >= 0.05", report.GlobalRMSPx)
	}
	if report.MatchFraction != 1 || report.NMatches != len(gaia) {
		t.Fatalf("matches %d/%d", report.NMatches, len(gaia))
	}
	if report.ZoneAnomaly {
		t.Fatal("perfect solution flagged zone anomaly")
	}
}

func TestVerifySolutionCornerDistortion(t *testing.T) {
	w, stars, gaia := verificationFixture()
	for i := range stars {
		if stars[i].X > 512*2/3 && stars[i].Y > 512*2/3 {
			stars[i].X += 2
			stars[i].Y += 2
		}
	}
	report := computeSolutionVerification(w, 512, 512, stars, gaia, 3)
	if !report.ZoneAnomaly {
		t.Fatalf("corner distortion not detected: %+v", report.Zones)
	}
	if report.Zones[8].RMSPx <= 1.5*report.GlobalRMSPx {
		t.Fatalf("zone RMS %.4f not > global %.4f", report.Zones[8].RMSPx, report.GlobalRMSPx)
	}
}

func TestVerifySolutionReportAndSVG(t *testing.T) {
	report := solutionVerificationReport{GlobalRMSPx: 0.02, NMatches: 4, NGaiaInFrame: 4, MatchFraction: 1, ZoneAnomaly: true}
	data, err := json.Marshal(&processingReport{SolutionVerification: &report})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"solution_verification", "global_rms_px", "zone_anomaly"} {
		if !strings.Contains(string(data), "\""+key+"\"") {
			t.Fatalf("missing %s", key)
		}
	}
	svg := buildSolutionResidualSVG(512, 512, report)
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "zone anomaly") {
		t.Fatalf("bad SVG: %s", svg[:200])
	}
	opts, err := parseArgs([]string{"--verify-solution"})
	if err != nil || !opts.VerifySolution {
		t.Fatalf("flag not parsed: %+v %v", opts, err)
	}
}

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWCSRefineShiftedMirrored(t *testing.T) {
	w, stars, gaia := verificationFixture()
	for i := range stars {
		stars[i].X += 1.5
		stars[i].Y -= 1.2
	}
	stars[0].X += 30
	stars[1].Y -= 30
	refined, report := computeWCSRefine(w, 512, 512, stars, gaia, 60)
	t.Logf("report=%+v", report)
	if report.RMSBeforePx <= 0 || report.RMSAfterPx >= report.RMSBeforePx*0.5 {
		t.Fatalf("refine did not improve RMS: before %.4f after %.4f", report.RMSBeforePx, report.RMSAfterPx)
	}
	if report.NOutliers < 2 {
		t.Fatalf("outliers not rejected: %+v", report)
	}
	if refined.CD[0][0] >= 0 {
		t.Fatal("mirror parity lost")
	}
}

func TestWCSRefineFlagAndJSON(t *testing.T) {
	opts, err := parseArgs([]string{"--wcs-refine"})
	if err != nil || !opts.WCSRefine {
		t.Fatalf("flag not parsed: %+v %v", opts, err)
	}
	data, err := json.Marshal(&processingReport{WCSRefineReport: &wcsRefineReport{RMSBeforePx: 1.5, RMSAfterPx: 0.02}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"wcs_refine"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

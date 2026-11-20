package main

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestRayleighCoefficientsScaleWithAirmass(t *testing.T) {
	a := rayleighCoefficients(1.1, 1013.25, 10)
	b := rayleighCoefficients(2.0, 1013.25, 10)
	for _, ch := range []string{"R", "G", "B"} {
		want := a[ch] * (2.0 / 1.1)
		if math.Abs(b[ch]-want)/want > 0.01 {
			t.Fatalf("%s: got %.6f want %.6f", ch, b[ch], want)
		}
	}
	if !(a["B"] > a["G"] && a["G"] > a["R"]) {
		t.Fatalf("lambda^-4 ordering broken: %+v", a)
	}
}

func TestExtinctionReportFromFITS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "light.fits")
	writeSurveyFITSHeader(t, path, []string{
		"NAXIS1   =                   64",
		"NAXIS2   =                   64",
		"AIRMASS  =                  2.0",
		"SITEELEV =                 2500.0",
		"PRESSURE =                  750.0",
	})
	report := computeExtinctionReport([]string{path})
	if report.Airmass != 2 || report.AirmassSource != "AIRMASS" {
		t.Fatalf("airmass not parsed: %+v", report)
	}
	if report.SiteElevM != 2500 || report.PressureHPa != 750 {
		t.Fatalf("site metadata not parsed: %+v", report)
	}
	if len(report.Rayleigh) != 3 {
		t.Fatalf("missing channels: %+v", report)
	}
}

func TestExtinctionReportFlagAndJSON(t *testing.T) {
	opts, err := parseArgs([]string{"--extinction-report"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.ExtinctionReport {
		t.Fatal("flag not parsed")
	}
	rep := &processingReport{ExtinctionReport: &extinctionReport{Airmass: 1.2, Rayleigh: map[string]float64{"R": 1, "G": 2, "B": 3}}}
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"extinction_report"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

package main

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestFitTransformationBand(t *testing.T) {
	pairs := make([]transformationPair, 0, 20)
	for i := 0; i < 20; i++ {
		x := 1.1 + 0.045*float64(i)
		color := 0.9 * math.Sin(1.7*float64(i))
		ref := 10 + 0.02*float64(i)
		inst := ref + 0.35 + 0.12*(x-1) + (-0.08)*color
		pairs = append(pairs, transformationPair{Band: "R", Instrumental: inst, Reference: ref, Airmass: x, Color: color})
	}
	coeff, rms, n := fitTransformationBand(pairs)
	if n != len(pairs) || rms > 1e-6 {
		t.Fatalf("fit rms %.6g n=%d", rms, n)
	}
	if math.Abs(coeff.ZP-0.35) > 1e-6 || math.Abs(coeff.K-0.12) > 1e-6 || math.Abs(coeff.ColorTerm+0.08) > 1e-6 {
		t.Fatalf("coefficients not recovered: %+v", coeff)
	}
}

func TestTransformationFromFITS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "light.fits")
	writeSurveyFITSHeader(t, path, []string{
		"NAXIS1   =                   64",
		"NAXIS2   =                   64",
		"OBSTYPE  = 'SCIENCE'",
		"FILTER   = 'R,B'",
		"TRANS    = 'STD'",
	})
	info := &extinctionReport{SiteElevM: 10, PressureHPa: 1013.25}
	report := computeTransformationReport([]string{path}, info)
	if report.OBSType != "SCIENCE" || report.Trans != "STD" || len(report.Filters) != 2 {
		t.Fatalf("cards not parsed: %+v", report)
	}
	for _, band := range []string{"R", "B"} {
		if report.Coefficients[band].K <= 0 {
			t.Fatalf("missing k for %s: %+v", band, report.Coefficients)
		}
	}
}

func TestTransformationFlagAndJSON(t *testing.T) {
	opts, err := parseArgs([]string{"--transformation-report"})
	if err != nil || !opts.TransformationReport {
		t.Fatalf("flag not parsed: %+v %v", opts, err)
	}
	data, err := json.Marshal(&processingReport{TransformationReport: &transformationReport{OBSType: "SCIENCE"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"transformation_report"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

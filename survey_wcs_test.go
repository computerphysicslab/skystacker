package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSurveyFITSHeader(t *testing.T, path string, cards []string) {
	t.Helper()
	var buf bytes.Buffer
	for _, card := range cards {
		buf.WriteString(fmt.Sprintf("%-80s", card))
	}
	buf.WriteString(fmt.Sprintf("%-80s", "END"))
	for buf.Len()%2880 != 0 {
		buf.WriteString(strings.Repeat(" ", 80))
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSurveyWCSFlags(t *testing.T) {
	dir := t.TempDir()
	base := []string{
		"NAXIS   =                    2",
		"NAXIS1  =                   64",
		"NAXIS2  =                   64",
		"RADESYS = 'FK5'",
		"TIMESYS = 'TAI'",
		"DEC     =                 20.0",
		"CD1_1   = -1.0E-5",
		"CD1_2   =  0.0E-5",
		"CD2_1   =  0.0E-5",
		"CD2_2   =  1.0E-5",
	}
	cases := []struct {
		name string
		add  []string
		drop []string
		want string
	}{
		{"legacy_pc", []string{"PC001001 = 1.0", "PC001002 = 0.0"}, nil, "legacy_pc_wcs"},
		{"missing_radesys", nil, []string{"RADESYS"}, "missing_radesys"},
		{"missing_timesys", nil, []string{"TIMESYS"}, "missing_timesys"},
		{"asinh", []string{"STRETCH = 'asinh'"}, nil, "asinh_scaling"},
		{"panstarrs_pole", []string{"DEC     =                 86.0", "TELESCOP= 'Pan-STARRS'"}, nil, "panstarrs_pole_risk"},
		{"clean", nil, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cards := make([]string, 0, len(base)+len(tc.add))
			for _, c := range base {
				key := strings.TrimSpace(strings.SplitN(c, "=", 2)[0])
				if containsString(tc.drop, key) {
					continue
				}
				cards = append(cards, c)
			}
			cards = append(cards, tc.add...)
			path := filepath.Join(dir, tc.name+".fits")
			writeSurveyFITSHeader(t, path, cards)
			report, err := checkSurveyFITS(path)
			if err != nil {
				t.Fatal(err)
			}
			flags := strings.Join(report.SurveyWCSFlags.Flags, ",")
			if tc.want == "" {
				if flags != "" || !report.OK {
					t.Fatalf("clean header flagged: %s %+v", flags, report)
				}
				return
			}
			if !containsString(report.SurveyWCSFlags.Flags, tc.want) {
				t.Fatalf("want flag %q, got %q", tc.want, flags)
			}
			if report.OK {
				t.Fatal("flagged header reported OK")
			}
		})
	}
}

func TestSurveyCheckFlagAndReports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cutout.fits")
	writeSurveyFITSHeader(t, path, []string{
		"NAXIS1  =                   32",
		"NAXIS2  =                   32",
		"PC001001 = 1.0",
		"DEC     =                 20.0",
	})
	opts, err := parseArgs([]string{"--check-survey-fits", path, "--output", dir})
	if err != nil {
		t.Fatal(err)
	}
	if opts.CheckSurveyFITS != path {
		t.Fatalf("flag not parsed: %+v", opts)
	}
	if err := runSurveyFITSCheck(opts.CheckSurveyFITS, opts.OutputDir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	flags, ok := parsed["survey_wcs_flags"].(map[string]any)
	if !ok {
		t.Fatalf("missing survey_wcs_flags: %s", data)
	}
	if flags["ok"] != false {
		t.Fatalf("bad header reported OK: %s", data)
	}
	text, err := os.ReadFile(filepath.Join(dir, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "survey_wcs_flags:") {
		t.Fatalf("report.txt missing flags: %s", text)
	}
}

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Item 119: oversized hips2fits frames must be rejected before any request.
func TestFetchHiPSCutoutPixelLimit(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("SIMPLE  = T"))
	}))
	defer server.Close()
	if _, err := fetchHiPSCutout(context.Background(), server.URL, "CDS/P/DSS2/color", 10000, 10000, 10, 20, 0.5, 0); err == nil {
		t.Fatal("100 Mpix cutout accepted")
	}
	if hits != 0 {
		t.Fatalf("oversized cutout reached the server: %d hits", hits)
	}
	if _, err := fetchHiPSCutout(context.Background(), server.URL, "CDS/P/DSS2/color", 0, 10, 10, 20, 0.5, 0); err == nil {
		t.Fatal("zero-width cutout accepted")
	}
	data, err := fetchHiPSCutout(context.Background(), server.URL, "CDS/P/DSS2/color", 64, 64, 10, 20, 0.5, 0)
	if err != nil || hits != 1 || string(data) != "SIMPLE  = T" {
		t.Fatalf("small cutout failed: %d bytes err=%v hits=%d", len(data), err, hits)
	}
}

// Items 165/166: multi-survey DSS lists and automatic HTTP timeout selection.
func TestParseDSSSurveys(t *testing.T) {
	got, err := parseDSSSurveys(" poss2ukstu_red, poss2ukstu_blue ,poss2ukstu_red ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "poss2ukstu_red" || got[1] != "poss2ukstu_blue" {
		t.Fatalf("surveys = %v", got)
	}
	got, err = parseDSSSurveys("")
	if err != nil || len(got) != 1 || got[0] != "poss2ukstu_red" {
		t.Fatalf("default surveys = %v, err=%v", got, err)
	}
	if _, err = parseDSSSurveys("poss2ukstu_red,bogus"); err == nil {
		t.Fatal("unknown survey accepted")
	}
}

func TestSuggestSurveyTimeout(t *testing.T) {
	if got := suggestSurveyTimeout("dss", 1997, 1318); got != 300 {
		t.Fatalf("large DSS timeout = %d, want 300", got)
	}
	if got := suggestSurveyTimeout("dss", 512, 512); got != 120 {
		t.Fatalf("small DSS timeout = %d, want 120", got)
	}
	if got := suggestSurveyTimeout("hips", 1997, 1318); got != 30 {
		t.Fatalf("HiPS timeout = %d, want 30", got)
	}
	opts, err := parseArgs([]string{"--survey-cutout", "--survey-source", "dss",
		"--dss-survey", "poss2ukstu_red,poss2ukstu_blue", "--survey-timeout", "0"})
	if err != nil || opts.SurveyTimeout != 0 || opts.SurveyDSSSurvey != "poss2ukstu_red,poss2ukstu_blue" {
		t.Fatalf("parseArgs = %+v, err=%v", opts, err)
	}
}

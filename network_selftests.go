package main

// Embedded coverage for the archive/network safety wave (roadmap items 118
// and 119): cross-host redirects must not receive a bearer token, cancelled
// queries must not dial, and oversized survey cutouts must be refused locally.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
)

func testArchiveAndSurveySafety() error {
	redirectAuth := "unset"
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`<VOTable><RESOURCE><TABLE><DATA><TABLEDATA><TR><TD>1</TD></TR></TABLEDATA></DATA></TABLE></RESOURCE></VOTable>`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "missing first-hop token", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, target.URL+"/target", http.StatusFound)
	}))
	defer origin.Close()
	result, err := queryArchiveAdapter(context.Background(), "mast", origin.URL, "secret", 10, 20, 60)
	if err != nil || result.Rows != 1 {
		return fmt.Errorf("archive redirect: rows=%d err=%v", result.Rows, err)
	}
	if redirectAuth != "" {
		return fmt.Errorf("bearer token forwarded across hosts: %q", redirectAuth)
	}

	hits := 0
	cancelled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`<VOTable/>`))
	}))
	defer cancelled.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queryArchiveAdapter(ctx, "mast", cancelled.URL, "secret", 10, 20, 60); err == nil || hits != 0 {
		return fmt.Errorf("cancelled query dialled: err=%v hits=%d", err, hits)
	}

	hits = 0
	cutout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("SIMPLE  = T"))
	}))
	defer cutout.Close()
	if _, err := fetchHiPSCutout(context.Background(), cutout.URL, "CDS/P/DSS2/color", 10000, 10000, 10, 20, 0.5, 0); err == nil || hits != 0 {
		return fmt.Errorf("oversized cutout not refused: err=%v hits=%d", err, hits)
	}
	if data, err := fetchHiPSCutout(context.Background(), cutout.URL, "CDS/P/DSS2/color", 64, 64, 10, 20, 0.5, 0); err != nil || string(data) != "SIMPLE  = T" {
		return fmt.Errorf("small cutout failed: %d bytes err=%v", len(data), err)
	}
	return nil
}

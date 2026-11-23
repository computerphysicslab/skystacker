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

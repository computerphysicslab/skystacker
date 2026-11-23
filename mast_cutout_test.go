package main

import (
	"os"
	"testing"
)

// TestMASTFITSFromZip verifies extraction of a science image with WCS from a
// MAST HAPCut ZIP using a captured real 100x100 M31 cutout.
func TestMASTFITSFromZip(t *testing.T) {
	data, err := os.ReadFile("/tmp/opencode/hapcut_m31.zip")
	if err != nil {
		t.Skipf("captured HAPCut zip not available: %v", err)
	}
	fitsData, name, err := mastFITSFromZip(data)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if fitsData == nil || len(fitsData) < 2880 {
		t.Fatalf("invalid FITS data (%d bytes)", len(fitsData))
	}
	if name == "" {
		t.Fatal("no FITS name")
	}
	wcs, _, err := parseAstrometryWCS(fitsData)
	if err != nil {
		t.Fatalf("parse WCS: %v", err)
	}
	if wcs.CRVAL1 <= 0 || wcs.CRVAL2 <= 0 {
		t.Fatalf("bad WCS center: %v %v", wcs.CRVAL1, wcs.CRVAL2)
	}
	// The synthesized FITS must be decodable
	tmp, err := os.CreateTemp("", "mast-test-*.fits")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Write(fitsData)
	tmp.Close()
	defer os.Remove(tmp.Name())
	frame, err := decodeFITSNative(tmp.Name(), 0, 0)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if frame.Width <= 0 || frame.Height <= 0 {
		t.Fatalf("bad frame geometry %dx%d", frame.Width, frame.Height)
	}
	t.Logf("extracted %s: %dx%d WCS center (%.3f, %.3f)", name, frame.Width, frame.Height,
		wcs.CRVAL1*180/3.14159265, wcs.CRVAL2*180/3.14159265)
}

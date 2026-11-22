package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotiff "github.com/computerphysicslab/skystacker/goTiff"
)

func TestTIFFInputIntegration(t *testing.T) {
	img := &gotiff.Image{
		Width: 4, Height: 2, Channels: 3, Bits: 32,
		Format: gotiff.SampleFloat, Photometric: gotiff.PhotoRGB,
		Pixels: []float32{
			0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.1, 1.2,
			1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 2.0, 2.1, 2.2, 2.3, 2.4,
		},
	}
	path := filepath.Join(t.TempDir(), "input.tif")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gotiff.Encode(f, img, &gotiff.Options{Compression: gotiff.CompressionDeflate, BigTIFF: true}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	frm, err := decodeTIFFInput(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if frm.Width != 4 || frm.Height != 2 || len(frm.Pix) != 4*2*3 {
		t.Fatalf("unexpected frame geometry %dx%d %d", frm.Width, frm.Height, len(frm.Pix))
	}
	for i := range img.Pixels {
		if frm.Pix[i] != img.Pixels[i] {
			t.Fatalf("pixel %d got %v want %v", i, frm.Pix[i], img.Pixels[i])
		}
	}
	cfg, err := decodeTIFFConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 4 || cfg.Height != 2 {
		t.Fatalf("unexpected config %+v", cfg)
	}
}

func TestTIFFInputAstroMetadataPropagation(t *testing.T) {
	wcs := &wcsTan{
		CRVAL1: 83.63 * math.Pi / 180,
		CRVAL2: 22.01 * math.Pi / 180,
		CRPIX1: 1500, CRPIX2: 1000,
		CD: [2][2]float64{{-1e-5, 0}, {0, 1e-5}},
	}
	img := &gotiff.Image{
		Width: 4, Height: 2, Channels: 3, Bits: 32,
		Format: gotiff.SampleFloat, Photometric: gotiff.PhotoRGB,
		Pixels:     make([]float32, 24),
		Metadata:   map[string]string{"ImageDescription": astroTIFFDescription(wcs, "RGGB")},
		CFAPattern: "RGGB", CFARepeatX: 2, CFARepeatY: 2,
	}
	path := filepath.Join(t.TempDir(), "astro.tif")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gotiff.Encode(f, img, &gotiff.Options{Compression: gotiff.CompressionDeflate}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	frm, err := decodeTIFFInput(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if frm.WCS == nil {
		t.Fatal("Astro-TIFF WCS not propagated to frame")
	}
	if math.Abs(frm.WCS.CRVAL1-wcs.CRVAL1) > 1e-9 || math.Abs(frm.WCS.CRVAL2-wcs.CRVAL2) > 1e-9 {
		t.Fatalf("WCS CRVAL mismatch: got %.9f/%.9f want %.9f/%.9f",
			frm.WCS.CRVAL1, frm.WCS.CRVAL2, wcs.CRVAL1, wcs.CRVAL2)
	}
	if frm.BayerPattern != "RGGB" {
		t.Fatalf("CFA pattern not propagated: %q", frm.BayerPattern)
	}
	if got := firstBayerPattern([]string{path}); got != "RGGB" {
		t.Fatalf("firstBayerPattern from Astro-TIFF = %q", got)
	}
	if got := astroTIFFDescription(wcs, "RGGB"); !strings.Contains(got, "BAYERPAT") {
		t.Fatalf("astro description missing BAYERPAT: %q", got)
	}
}

func TestAstroTIFFDescriptionRoundTrip(t *testing.T) {
	wcs := &wcsTan{
		CRVAL1: 1.234 * math.Pi / 180,
		CRVAL2: -2.345 * math.Pi / 180,
		CRPIX1: 100, CRPIX2: 200,
		CD: [2][2]float64{{1.5e-5, 2.5e-6}, {-3.5e-6, -4.5e-5}},
	}
	desc := astroTIFFDescription(wcs, "BGGR")
	got, bayer := parseAstroTIFFDescription(desc)
	if got == nil {
		t.Fatal("parseAstroTIFFDescription returned nil")
	}
	if math.Abs(got.CRVAL1-wcs.CRVAL1) > 1e-9 || math.Abs(got.CRVAL2-wcs.CRVAL2) > 1e-9 ||
		math.Abs(got.CRPIX1-100) > 1e-6 || math.Abs(got.CRPIX2-200) > 1e-6 {
		t.Fatalf("WCS round trip mismatch: %+v vs %+v", got, wcs)
	}
	if bayer != "BGGR" {
		t.Fatalf("bayer round trip = %q", bayer)
	}
}

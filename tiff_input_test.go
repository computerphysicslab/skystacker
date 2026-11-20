package main

import (
	"os"
	"path/filepath"
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

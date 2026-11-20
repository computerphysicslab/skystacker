package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotiff "github.com/computerphysicslab/skystacker/goTiff"
)

func TestWriteStackTIFFOptions(t *testing.T) {
	pix := make([]float32, 32*32*3)
	for i := range pix {
		pix[i] = float32(i%97) / 97
	}
	path := filepath.Join(t.TempDir(), "stacked_sum.tif")
	cfg := tiffWriteConfig{compression: "deflate", depth: "16", astro: true, tiled: true, predictor: true}
	if err := writeStackTIFF(path, 32, 32, pix, 1, true, cfg, &wcsTan{
		CRVAL1: 1.0, CRVAL2: 2.0, CRPIX1: 16, CRPIX2: 16,
		CD: [2][2]float64{{1e-5, 0}, {0, 1e-5}},
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := gotiff.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bits != 16 || img.Format != gotiff.SampleUint || !strings.Contains(img.Metadata["ImageDescription"], "CTYPE1") {
		t.Fatalf("unexpected image: bits=%d format=%d meta=%v", img.Bits, img.Format, img.Metadata)
	}
}

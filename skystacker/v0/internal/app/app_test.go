package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skystacker/skystacker/internal/output"
)

func writeSolidJPEG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
}

func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeSolidJPEG(t, filepath.Join(dir, "a.jpg"), 8, 6, color.RGBA{200, 100, 50, 255})
	writeSolidJPEG(t, filepath.Join(dir, "b.jpg"), 8, 6, color.RGBA{100, 200, 150, 255})
	writeSolidJPEG(t, filepath.Join(dir, "c_bad.jpg"), 4, 6, color.RGBA{0, 0, 0, 255})
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), Options{InputDir: dir, Workers: 2}); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "skystacker_results")
	expected := []string{
		"stacked_sum.tiff",
		"stacked_sum.jpg",
		"stacked_mean.tiff",
		"stacked_mean.jpg",
		"stacked_no_correction_mean.tiff",
		"stacked_no_correction_mean.jpg",
		"stacked_preview.jpg",
		"report.txt",
	}
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("missing output %s: %v", name, err)
		}
	}

	// In v0.1, no_correction_mean.tiff must equal mean.tiff byte-for-byte.
	meanBytes, err := os.ReadFile(filepath.Join(outDir, "stacked_mean.tiff"))
	if err != nil {
		t.Fatal(err)
	}
	noCorrBytes, err := os.ReadFile(filepath.Join(outDir, "stacked_no_correction_mean.tiff"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(meanBytes, noCorrBytes) {
		t.Fatalf("stacked_no_correction_mean.tiff differs from stacked_mean.tiff (must be identical in v0.1)")
	}

	// Verify the sum TIFF.
	sumFile, err := os.Open(filepath.Join(outDir, "stacked_sum.tiff"))
	if err != nil {
		t.Fatal(err)
	}
	w, h, sumPix, err := output.ReadFloat32RGB(sumFile)
	sumFile.Close()
	if err != nil {
		t.Fatalf("reading sum TIFF: %v", err)
	}
	if w != 8 || h != 6 {
		t.Fatalf("sum dims = %dx%d, want 8x6", w, h)
	}
	wantSum := [3]float32{300.0 / 255, 300.0 / 255, 200.0 / 255}
	for i := 0; i < w*h; i++ {
		for c := 0; c < 3; c++ {
			got := sumPix[i*3+c]
			if math.Abs(float64(got-wantSum[c])) > 0.03 {
				t.Fatalf("sum pixel %d ch %d = %f, want ~%f", i, c, got, wantSum[c])
			}
		}
	}
	if sumPix[0] <= 1.0 {
		t.Fatalf("expected sum to exceed 1.0 (got %f)", sumPix[0])
	}

	// Verify the mean TIFF.
	meanFile, err := os.Open(filepath.Join(outDir, "stacked_mean.tiff"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, meanPix, err := output.ReadFloat32RGB(meanFile)
	meanFile.Close()
	if err != nil {
		t.Fatalf("reading mean TIFF: %v", err)
	}
	wantMean := [3]float32{150.0 / 255, 150.0 / 255, 100.0 / 255}
	for i := 0; i < w*h; i++ {
		for c := 0; c < 3; c++ {
			got := meanPix[i*3+c]
			if math.Abs(float64(got-wantMean[c])) > 0.03 {
				t.Fatalf("mean pixel %d ch %d = %f, want ~%f", i, c, got, wantMean[c])
			}
		}
	}

	// Report must mention the rejected file and the registration caveat.
	reportBytes, err := os.ReadFile(filepath.Join(outDir, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	reportStr := string(reportBytes)
	for _, s := range []string{
		"c_bad.jpg",
		"No star registration",
		"Accepted:         2",
		"stacked_no_correction_mean.tiff",
	} {
		if !strings.Contains(reportStr, s) {
			t.Errorf("report missing %q\n--- report ---\n%s", s, reportStr)
		}
	}
}

func TestRunEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	err := Run(context.Background(), Options{InputDir: dir})
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}

func TestRunOutputExistsWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeSolidJPEG(t, filepath.Join(dir, "a.jpg"), 4, 4, color.RGBA{128, 128, 128, 255})
	out := filepath.Join(dir, "skystacker_results")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), Options{InputDir: dir}); err == nil {
		t.Fatal("expected error when output exists without --overwrite")
	}
	if err := Run(context.Background(), Options{InputDir: dir, Overwrite: true}); err != nil {
		t.Fatalf("overwrite run failed: %v", err)
	}
}

func TestRunAllFramesRejected(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not a jpeg"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Run(context.Background(), Options{InputDir: dir}); err == nil {
		t.Fatal("expected error when all candidates are rejected")
	}
}

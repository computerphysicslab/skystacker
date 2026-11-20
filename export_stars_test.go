package main

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestExportStarsFluxSorted(t *testing.T) {
	const w, h = 120, 90
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Three Gaussian stars with clearly different peak amplitudes.
	stars := []struct {
		x, y, amp float64
	}{{30, 30, 0.9}, {70, 50, 0.45}, {100, 70, 0.20}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 0.05
			for _, s := range stars {
				d2 := (float64(x)-s.x)*(float64(x)-s.x) + (float64(y)-s.y)*(float64(y)-s.y)
				v += s.amp * math.Exp(-d2/8)
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "stars.json")
	if err := writeStarCatalog(jsonPath, "synthetic.jpg", f, 50); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog starCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Count != len(catalog.Stars) || catalog.Count < 3 {
		t.Fatalf("catalog count=%d stars=%d", catalog.Count, len(catalog.Stars))
	}
	for i := 1; i < len(catalog.Stars); i++ {
		if catalog.Stars[i-1].Flux < catalog.Stars[i].Flux {
			t.Fatalf("stars not flux-sorted at %d: %.3f < %.3f", i,
				catalog.Stars[i-1].Flux, catalog.Stars[i].Flux)
		}
	}
	if catalog.Stars[0].X < 25 || catalog.Stars[0].X > 35 || catalog.Stars[0].Y < 25 || catalog.Stars[0].Y > 35 {
		t.Fatalf("brightest star centroid unexpected: %+v", catalog.Stars[0])
	}
	csvPath := filepath.Join(dir, "stars.csv")
	if err := writeStarCatalog(csvPath, "synthetic.jpg", f, 50); err != nil {
		t.Fatal(err)
	}
	cf, err := os.Open(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	defer cf.Close()
	rows, err := csv.NewReader(cf).ReadAll()
	if err != nil || len(rows) != catalog.Count+1 {
		t.Fatalf("csv rows=%d err=%v", len(rows), err)
	}
	if rows[0][0] != "x_px" || rows[0][5] != "hfr_px" {
		t.Fatalf("csv header: %v", rows[0])
	}
}

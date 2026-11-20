package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestDiffuseObjectClassification(t *testing.T) {
	const w, h = 200, 200
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	stars := []struct{ x, y, amp float64 }{{30, 30, 1.2}, {70, 150, 0.9}, {150, 60, 0.8}}
	extended := []struct{ x, y, amp, sigma float64 }{
		{100, 100, 0.35, 8},  // compact galaxy-like source
		{175, 175, 0.08, 25}, // diffuse nebula-like source
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 0.05 + 0.0002*float64(x)
			for _, s := range stars {
				d2 := (float64(x)-s.x)*(float64(x)-s.x) + (float64(y)-s.y)*(float64(y)-s.y)
				v += s.amp * math.Exp(-d2/4.5)
			}
			for _, s := range extended {
				d2 := (float64(x)-s.x)*(float64(x)-s.x) + (float64(y)-s.y)*(float64(y)-s.y)
				v += s.amp * math.Exp(-d2/(2*s.sigma*s.sigma))
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	rep, err := classifyObjects("synthetic.jpg", f, 3)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Count < 2 {
		t.Fatalf("expected at least two diffuse objects, got %d", rep.Count)
	}
	for _, s := range stars {
		for _, o := range rep.Objects {
			if math.Hypot(o.X-s.x, o.Y-s.y) < 4 {
				t.Fatalf("star at (%.0f,%.0f) misclassified as diffuse %s at (%.1f,%.1f)",
					s.x, s.y, o.ObjectType, o.X, o.Y)
			}
		}
	}
	types := map[string]bool{}
	for _, o := range rep.Objects {
		types[o.ObjectType] = true
		if o.ClassUCD != "src.class" {
			t.Fatalf("missing UCD tag: %+v", o)
		}
	}
	if !types["galaxy"] && !types["nebula"] {
		t.Fatalf("no extended object classified: %v", types)
	}
	path := filepath.Join(t.TempDir(), "objects.json")
	if err := writeObjectClassification(path, rep); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded objectClassificationReport
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.Count != rep.Count {
		t.Fatalf("classification JSON round-trip: %v count=%d", err, decoded.Count)
	}
}

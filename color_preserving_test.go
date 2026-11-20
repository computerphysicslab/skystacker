package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestColorPreservingStretchRatios(t *testing.T) {
	const w, h = 64, 48
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			base := 0.02 + 0.0008*float64(x+y)
			star := 0.6 * math.Exp(-(math.Pow(float64(x-32), 2)+math.Pow(float64(y-24), 2))/8)
			i := (y*w + x) * 3
			f.Pix[i] = float32(base + star)
			f.Pix[i+1] = float32(base*0.8 + star*0.7)
			f.Pix[i+2] = float32(base*0.6 + star*0.4)
		}
	}
	out := colorPreservingStretch(f, -2.8, 0.25)
	if !out.Nonlinear {
		t.Fatal("result not marked nonlinear")
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			rIn, gIn, bIn := float64(f.Pix[i]), float64(f.Pix[i+1]), float64(f.Pix[i+2])
			rOut, gOut, bOut := float64(out.Pix[i]), float64(out.Pix[i+1]), float64(out.Pix[i+2])
			if gIn <= 1e-6 || gOut <= 1e-6 {
				continue
			}
			rgIn, rgOut := rIn/gIn, rOut/gOut
			bgIn, bgOut := bIn/gIn, bOut/gOut
			if math.Abs(rgOut-rgIn)/rgIn > 0.02 || math.Abs(bgOut-bgIn)/bgIn > 0.02 {
				t.Fatalf("pixel %d,%d ratios changed: in %.4f/%.4f out %.4f/%.4f", x, y, rgIn, bgIn, rgOut, bgOut)
			}
		}
	}
}

func TestColorPreservingFlagAndReport(t *testing.T) {
	opts, err := parseArgs([]string{"--stretch-color-preserving"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.StretchColorPreserving {
		t.Fatal("flag not parsed")
	}
	data, err := json.Marshal(&processingReport{StretchColorPreserving: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"stretch_color_preserving"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

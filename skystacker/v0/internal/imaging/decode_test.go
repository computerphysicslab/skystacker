package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
)

func TestFromImageRGBA(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})

	f, err := FromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	if f.Width != 2 || f.Height != 1 {
		t.Fatalf("dims = %dx%d, want 2x1", f.Width, f.Height)
	}
	if !near(f.At(0, 0, 0), 1) || !near(f.At(0, 0, 1), 0) || !near(f.At(0, 0, 2), 0) {
		t.Errorf("pixel (0,0) = %v", f.Pix[0:3])
	}
	if !near(f.At(1, 0, 0), 0) || !near(f.At(1, 0, 1), 1) || !near(f.At(1, 0, 2), 0) {
		t.Errorf("pixel (1,0) = %v", f.Pix[3:6])
	}
}

func TestDecodeJPEG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			src.SetRGBA(x, y, color.RGBA{200, 100, 50, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	f, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if f.Width != 4 || f.Height != 3 {
		t.Fatalf("dims = %dx%d, want 4x3", f.Width, f.Height)
	}
	want := [3]float32{200.0 / 255.0, 100.0 / 255.0, 50.0 / 255.0}
	for i := 0; i < 4*3; i++ {
		for c := 0; c < 3; c++ {
			if math.Abs(float64(f.Pix[i*3+c]-want[c])) > 0.02 {
				t.Fatalf("pixel %d channel %d = %f, want ~%f", i, c, f.Pix[i*3+c], want[c])
			}
		}
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-6 }

package stack

import (
	"math"
	"testing"

	"github.com/skystacker/skystacker/internal/imaging"
)

func TestSumAndMean(t *testing.T) {
	a := New(2, 2)
	f1 := &imaging.Frame{Width: 2, Height: 2, Pix: []float32{
		0.1, 0.2, 0.3, 0.4, 0.5, 0.6,
		0.7, 0.8, 0.9, 1.0, 0.0, 0.5,
	}}
	f2 := &imaging.Frame{Width: 2, Height: 2, Pix: []float32{
		0.9, 0.8, 0.7, 0.6, 0.5, 0.4,
		0.3, 0.2, 0.1, 0.0, 1.0, 0.5,
	}}
	if err := a.AddFrame(f1); err != nil {
		t.Fatal(err)
	}
	if err := a.AddFrame(f2); err != nil {
		t.Fatal(err)
	}
	for i, v := range a.Pix {
		if math.Abs(float64(v-1.0)) > 1e-6 {
			t.Fatalf("sum[%d] = %f, want 1", i, v)
		}
	}
	a.Divide()
	for i, v := range a.Pix {
		if math.Abs(float64(v-0.5)) > 1e-6 {
			t.Fatalf("mean[%d] = %f, want 0.5", i, v)
		}
	}
}

func TestSumExceedsUnity(t *testing.T) {
	a := New(1, 1)
	f := &imaging.Frame{Width: 1, Height: 1, Pix: []float32{1, 1, 1}}
	for i := 0; i < 300; i++ {
		if err := a.AddFrame(f); err != nil {
			t.Fatal(err)
		}
	}
	if a.Pix[0] != 300 {
		t.Fatalf("sum = %f, want 300", a.Pix[0])
	}
	a.Divide()
	if math.Abs(float64(a.Pix[0]-1)) > 1e-6 {
		t.Fatalf("mean = %f, want 1", a.Pix[0])
	}
}

func TestDimensionMismatch(t *testing.T) {
	a := New(2, 2)
	bad := &imaging.Frame{Width: 3, Height: 2, Pix: make([]float32, 3*2*3)}
	if err := a.AddFrame(bad); err == nil {
		t.Fatal("expected dimension-mismatch error")
	}
}

func TestParallelMatchesSequential(t *testing.T) {
	a := New(64, 32)
	f := &imaging.Frame{Width: 64, Height: 32, Pix: make([]float32, 64*32*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i%97) / 97.0
	}
	if err := a.AddFrameParallel(f, 8); err != nil {
		t.Fatal(err)
	}
	b := New(64, 32)
	if err := b.AddFrame(f); err != nil {
		t.Fatal(err)
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("mismatch at %d: %f vs %f", i, a.Pix[i], b.Pix[i])
		}
	}
}

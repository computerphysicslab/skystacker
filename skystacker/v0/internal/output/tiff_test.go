package output

import (
	"bytes"
	"math"
	"testing"
)

func TestTIFFRoundTrip(t *testing.T) {
	const w, h = 3, 2
	pix := []float32{
		0, 0.25, 0.5,
		1, 2, 3,
		1e-6, 1e6, 255,
		0.1, 0.2, 0.3,
		-1, 0, 1,
		3.14159265, 2.7182818, 1.41421356,
	}
	var buf bytes.Buffer
	if err := WriteFloat32RGB(&buf, w, h, pix); err != nil {
		t.Fatal(err)
	}
	gotW, gotH, gotPix, err := ReadFloat32RGB(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if gotW != w || gotH != h {
		t.Fatalf("dims = %dx%d, want %dx%d", gotW, gotH, w, h)
	}
	if len(gotPix) != len(pix) {
		t.Fatalf("pixel count = %d, want %d", len(gotPix), len(pix))
	}
	for i := range pix {
		if gotPix[i] != pix[i] {
			t.Fatalf("pixel[%d] = %v, want %v", i, gotPix[i], pix[i])
		}
	}
}

func TestTIFFHeader(t *testing.T) {
	var buf bytes.Buffer
	pix := make([]float32, 2*2*3)
	if err := WriteFloat32RGB(&buf, 2, 2, pix); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if b[0] != 'I' || b[1] != 'I' {
		t.Errorf("expected little-endian byte order marker, got %v", b[0:2])
	}
	if len(b) < 8 {
		t.Fatalf("output too short: %d", len(b))
	}
	if _, _, _, err := ReadFloat32RGB(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

func TestTIFFRejectsBadInput(t *testing.T) {
	if err := WriteFloat32RGB(&bytes.Buffer{}, 0, 0, nil); err == nil {
		t.Fatal("expected error for zero dimensions")
	}
	if err := WriteFloat32RGB(&bytes.Buffer{}, 2, 2, make([]float32, 11)); err == nil {
		t.Fatal("expected error for mismatched buffer length")
	}
}

func TestTIFFPreservesLargeValues(t *testing.T) {
	var buf bytes.Buffer
	pix := []float32{1000000, 999999.5, 3.4e38}
	if err := WriteFloat32RGB(&buf, 1, 1, pix); err != nil {
		t.Fatal(err)
	}
	_, _, got, err := ReadFloat32RGB(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := range pix {
		if math.Abs(float64(got[i]-pix[i])) > 1e-3 {
			t.Fatalf("pixel[%d] = %v, want %v", i, got[i], pix[i])
		}
	}
}

package gotiff

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func roundTrip(t *testing.T, img *Image, opt *Options, tolerance float64) *Image {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, img, opt); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != img.Width || got.Height != img.Height || got.Channels != img.Channels {
		t.Fatalf("geometry got %dx%d/%d want %dx%d/%d", got.Width, got.Height, got.Channels, img.Width, img.Height, img.Channels)
	}
	if got.Bits != img.Bits || got.Format != img.Format || got.Photometric != img.Photometric {
		t.Fatalf("format got %d/%d/%d want %d/%d/%d", got.Bits, got.Format, got.Photometric, img.Bits, img.Format, img.Photometric)
	}
	for i := range img.Pixels {
		if math.Abs(float64(got.Pixels[i]-img.Pixels[i])) > tolerance {
			t.Fatalf("pixel %d got %.6f want %.6f", i, got.Pixels[i], img.Pixels[i])
		}
	}
	return got
}

func TestRoundTripGrayUint8(t *testing.T) {
	img := &Image{
		Width: 4, Height: 2, Channels: 1, Bits: 8,
		Format: SampleUint, Photometric: PhotoBlackIsZero,
		Pixels:   []float32{0, 0.25, 0.5, 1, 0.125, 0.375, 0.75, 0.875},
		Metadata: map[string]string{"ImageDescription": "WCS=test"},
	}
	got := roundTrip(t, img, nil, 1.0/255)
	if got.Metadata["ImageDescription"] != "WCS=test" {
		t.Fatalf("metadata lost: %v", got.Metadata)
	}
}

func TestRoundTripGrayUint16(t *testing.T) {
	img := &Image{
		Width: 3, Height: 3, Channels: 1, Bits: 16,
		Format: SampleUint, Photometric: PhotoBlackIsZero,
		Pixels: []float32{
			0, 1.0 / 3, 0.5,
			0.75, 1, 0.1,
			0.2, 0.3, 0.4,
		},
	}
	roundTrip(t, img, nil, 1.0/65535)
}

func TestRoundTripRGBUint8(t *testing.T) {
	img := &Image{
		Width: 2, Height: 2, Channels: 3, Bits: 8,
		Format: SampleUint, Photometric: PhotoRGB,
		Pixels: []float32{
			1, 0, 0, 0, 1, 0,
			0, 0, 1, 0.25, 0.5, 0.75,
		},
	}
	roundTrip(t, img, nil, 1.0/255)
}

func TestRoundTripGrayFloat32(t *testing.T) {
	img := &Image{
		Width: 4, Height: 1, Channels: 1, Bits: 32,
		Format: SampleFloat, Photometric: PhotoBlackIsZero,
		Pixels: []float32{-1.5, 0, 0.125, 1234.5},
	}
	roundTrip(t, img, nil, 0)
}

func TestRoundTripRGBFloat64BigEndian(t *testing.T) {
	img := &Image{
		Width: 2, Height: 1, Channels: 3, Bits: 64,
		Format: SampleFloat, Photometric: PhotoRGB,
		Pixels: []float32{1.25, -2.5, 3.75, 0.1, 0.2, 0.3},
	}
	roundTrip(t, img, &Options{ByteOrder: binary.BigEndian}, 0)
}

func TestRoundTripWhiteIsZero(t *testing.T) {
	img := &Image{
		Width: 3, Height: 1, Channels: 1, Bits: 8,
		Format: SampleUint, Photometric: PhotoWhiteIsZero,
		Pixels: []float32{0, 0.5, 1},
	}
	roundTrip(t, img, nil, 1.0/255)
}

func TestDecodeRejectsBigTIFF(t *testing.T) {
	magic := []byte{'I', 'I', 43, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if _, err := Decode(bytes.NewReader(magic)); err == nil {
		t.Fatal("expected BigTIFF rejection")
	}
}

func TestDecodeRejectsCompression(t *testing.T) {
	img := &Image{Width: 2, Height: 2, Channels: 1, Bits: 8, Format: SampleUint, Photometric: PhotoBlackIsZero, Pixels: make([]float32, 4)}
	var buf bytes.Buffer
	if err := Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	idx := bytes.Index(data, []byte{3, 1, 3, 0})
	if idx < 0 {
		t.Fatal("compression tag not found")
	}
	data[idx+8] = 5
	if _, err := Decode(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "compression") {
		t.Fatalf("expected compression error, got %v", err)
	}
}

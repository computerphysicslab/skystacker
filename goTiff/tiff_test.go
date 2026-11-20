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

func TestCompressionRoundTrips(t *testing.T) {
	img := &Image{
		Width: 32, Height: 16, Channels: 3, Bits: 8,
		Format: SampleUint, Photometric: PhotoRGB,
		Pixels: make([]float32, 32*16*3),
	}
	for i := range img.Pixels {
		img.Pixels[i] = float32(i%251) / 251
	}
	cases := []struct {
		name string
		opt  *Options
	}{
		{"packbits", &Options{Compression: CompressionPackBits}},
		{"deflate", &Options{Compression: CompressionDeflate}},
		{"deflate-predictor", &Options{Compression: CompressionDeflate, Predictor: true}},
		{"lzw", &Options{Compression: CompressionLZW}},
		{"lzw-predictor", &Options{Compression: CompressionLZW, Predictor: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Encode(&buf, img, tc.opt); err != nil {
				t.Fatal(err)
			}
			got, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			for i := range img.Pixels {
				if math.Abs(float64(got.Pixels[i]-img.Pixels[i])) > 1.0/255 {
					t.Fatalf("pixel %d got %.6f want %.6f", i, got.Pixels[i], img.Pixels[i])
				}
			}
		})
	}
}

func TestAstroTIFFAndCFAMetadata(t *testing.T) {
	wcs := "CTYPE1=RA---TAN\nCTYPE2=DEC--TAN\nCRVAL1=83.63\nCRVAL2=22.01\nBAYERPAT='RGGB'"
	img := &Image{
		Width: 4, Height: 4, Channels: 1, Bits: 16,
		Format: SampleUint, Photometric: PhotoBlackIsZero,
		Pixels:     make([]float32, 16),
		Metadata:   map[string]string{"ImageDescription": wcs, "Software": "SkyStacker"},
		CFAPattern: "RGGB", CFARepeatX: 2, CFARepeatY: 2,
	}
	var buf bytes.Buffer
	if err := Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata["ImageDescription"] != wcs || got.Metadata["Software"] != "SkyStacker" {
		t.Fatalf("astro metadata lost: %v", got.Metadata)
	}
	if got.CFAPattern != "RGGB" || got.CFARepeatX != 2 || got.CFARepeatY != 2 {
		t.Fatalf("CFA lost: %q %dx%d", got.CFAPattern, got.CFARepeatX, got.CFARepeatY)
	}
}

func TestBigTIFFRoundTrip(t *testing.T) {
	img := &Image{
		Width: 16, Height: 8, Channels: 3, Bits: 32,
		Format: SampleFloat, Photometric: PhotoRGB,
		Pixels: make([]float32, 16*8*3),
	}
	for i := range img.Pixels {
		img.Pixels[i] = float32(i) / 7
	}
	var buf bytes.Buffer
	if err := Encode(&buf, img, &Options{BigTIFF: true, Compression: CompressionDeflate}); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte{'I', 'I', 43, 0, 8, 0}) {
		t.Fatalf("missing BigTIFF header: % x", buf.Bytes()[:8])
	}
	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for i := range img.Pixels {
		if got.Pixels[i] != img.Pixels[i] {
			t.Fatalf("pixel %d got %v want %v", i, got.Pixels[i], img.Pixels[i])
		}
	}
}

func TestMultiPageRoundTrip(t *testing.T) {
	first := &Image{Width: 3, Height: 2, Channels: 1, Bits: 8, Format: SampleUint, Photometric: PhotoBlackIsZero, Pixels: []float32{0, 0.5, 1, 0.25, 0.75, 0.125}}
	second := &Image{Width: 2, Height: 2, Channels: 3, Bits: 16, Format: SampleUint, Photometric: PhotoRGB, Pixels: []float32{1, 0, 0, 0, 1, 0, 0, 0, 1, 0.5, 0.5, 0.5}}
	for _, big := range []bool{false, true} {
		var buf bytes.Buffer
		if err := EncodeAll(&buf, []*Image{first, second}, &Options{BigTIFF: big}); err != nil {
			t.Fatal(err)
		}
		pages, err := DecodeAll(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 2 {
			t.Fatalf("big=%v got %d pages", big, len(pages))
		}
		if pages[0].Width != 3 || pages[1].Channels != 3 {
			t.Fatalf("big=%v wrong page geometry: %+v %+v", big, pages[0], pages[1])
		}
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
	data[idx+8] = 7
	if _, err := Decode(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "compression") {
		t.Fatalf("expected compression error, got %v", err)
	}
}

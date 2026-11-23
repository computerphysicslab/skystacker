package ljpeg

import (
	"bytes"
	"errors"
	"testing"
)

// minimalStream builds a tiny lossless JPEG with one Huffman table whose
// only symbol is 0, so every decoded difference is zero.
func minimalStream() []byte {
	// SOI + DHT (one table, 4-bit code '0000' -> symbol 0) + SOF3 + SOS.
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xC4, 0x00, 0x14, 0x00,
		0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0,
		0xFF, 0xC3, 0x00, 0x0B, 0x08, 0x00, 0x02, 0x00, 0x02, 0x01, 0x01, 0x11, 0x00,
		0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00,
		0x00, 0x00,
		0xFF, 0xD9,
	}
}

func TestMinimalStream(t *testing.T) {
	img, err := Decode(minimalStream())
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != 2 || img.Height != 2 || img.Precision != 8 {
		t.Fatalf("geometry: %+v", img)
	}
	for p, v := range img.Planes[0] {
		if v != 128 { // initial predictor is 2^(P-1)
			t.Fatalf("sample %d = %d, want 128", p, v)
		}
	}
}

// minimalStreamWithRestart inserts an RST0 marker in the middle of the
// entropy data of a 4x1 stream whose symbols are all zero. The restart
// resets the bit reader; decoded samples must still be the initial
// predictor (128) everywhere.
func minimalStreamWithRestart() []byte {
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xC4, 0x00, 0x14, 0x00,
		0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0,
		0xFF, 0xC3, 0x00, 0x0B, 0x08, 0x00, 0x01, 0x00, 0x04, 0x01, 0x01, 0x11, 0x00,
		0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00,
		0x00, 0x00, 0x00, 0xFF, 0xD0, 0x00, 0x00,
		0xFF, 0xD9,
	}
}

func TestRestartMarkers(t *testing.T) {
	img, err := Decode(minimalStreamWithRestart())
	if err != nil {
		t.Fatalf("restart stream rejected: %v", err)
	}
	if len(img.Planes[0]) != 4 {
		t.Fatalf("plane length %d", len(img.Planes[0]))
	}
	for p, v := range img.Planes[0] {
		if v != 128 {
			t.Fatalf("sample %d = %d, want 128", p, v)
		}
	}
}

func TestBadStreams(t *testing.T) {
	for _, b := range [][]byte{nil, {0xFF, 0xD9}, {0xFF, 0xD8, 0xFF, 0xC3, 0x00, 0x01, 0x00}} {
		if _, err := Decode(b); err == nil {
			t.Fatal("bad stream accepted")
		}
	}
}

// streamWithSOF3 builds a structurally valid stream with arbitrary SOF3
// geometry. Entropy bytes are only emitted for small streams: oversized cases
// must be rejected from the header, before the test itself allocates data.
func streamWithSOF3(width, height uint16, precision byte, ncomp byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8})
	// DHT: one 4-bit code for symbol 0.
	b.Write([]byte{0xFF, 0xC4, 0x00, 0x14, 0x00, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	sofLen := 8 + 3*int(ncomp)
	b.Write([]byte{0xFF, 0xC3, byte(sofLen >> 8), byte(sofLen), precision,
		byte(height >> 8), byte(height), byte(width >> 8), byte(width), ncomp})
	for i := 0; i < int(ncomp); i++ {
		b.Write([]byte{byte(i + 1), 0x11, 0x00})
	}
	sosLen := 6 + 2*int(ncomp)
	b.Write([]byte{0xFF, 0xDA, byte(sosLen >> 8), byte(sosLen), ncomp})
	for i := 0; i < int(ncomp); i++ {
		b.Write([]byte{byte(i + 1), 0x00})
	}
	b.Write([]byte{0x00, 0x3F, 0x00})
	if int(width)*int(height)*int(ncomp) <= 1024 {
		b.Write(make([]byte, 4))
	}
	b.Write([]byte{0xFF, 0xD9})
	return b.Bytes()
}

func TestValidateGeometryLimits(t *testing.T) {
	cases := []struct {
		name                       string
		width, height, prec, ncomp int
		want                       error
	}{
		{"plane at limit", MaxPixels, 1, 14, 1, nil},
		{"plane above limit", MaxPixels + 1, 1, 14, 1, ErrTooLarge},
		{"samples at limit", maxSamples / 4, 1, 14, 4, nil},
		{"samples above limit", maxSamples/4 + 1, 1, 14, 4, ErrTooLarge},
		{"zero width", 0, 1, 8, 1, ErrCorrupt},
		{"zero precision", 2, 2, 0, 1, ErrCorrupt},
		{"precision above 16", 2, 2, 17, 1, ErrCorrupt},
		{"no components", 2, 2, 8, 0, ErrCorrupt},
		{"too many components", 2, 2, 8, 5, ErrCorrupt},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateGeometry(c.width, c.height, c.prec, c.ncomp)
			if !errors.Is(err, c.want) {
				t.Fatalf("ValidateGeometry(%d,%d,%d,%d) = %v, want %v",
					c.width, c.height, c.prec, c.ncomp, err, c.want)
			}
		})
	}
}

func TestOversizedSOF3RejectedBeforeAllocation(t *testing.T) {
	// 50,000 x 5,000 = 250 Mpix per plane, above the 200 Mpix cap. The
	// synthetic stream itself is tiny; a successful plane allocation here
	// would need 1 GiB and is exactly what the guard prevents.
	_, err := Decode(streamWithSOF3(50000, 5000, 14, 2))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized SOF3 error = %v, want ErrTooLarge", err)
	}
}

// FuzzLJPEG feeds arbitrary bytes to Decode and asserts it never panics.
func FuzzLJPEG(f *testing.F) {
	f.Add(minimalStream())
	f.Add(minimalStreamWithRestart())
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic: %v", r)
			}
		}()
		_, _ = Decode(data)
	})
}

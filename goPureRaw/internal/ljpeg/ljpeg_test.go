package ljpeg

import "testing"

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

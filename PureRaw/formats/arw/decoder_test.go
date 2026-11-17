package arw

import (
	"bytes"
	"testing"
)

func TestProbe(t *testing.T) {
	// Little-endian TIFF header with "SONY" in the first 64 KiB.
	le := append([]byte{'I', 'I', 0x2A, 0x00}, make([]byte, 64)...)
	copy(le[16:], []byte("SONY"))
	if !(Decoder{}).Probe(bytes.NewReader(le)) {
		t.Fatal("LE ARW signature not detected")
	}
	// Big-endian variant.
	be := append([]byte{'M', 'M', 0x00, 0x2A}, make([]byte, 64)...)
	copy(be[16:], []byte("SONY"))
	if !(Decoder{}).Probe(bytes.NewReader(be)) {
		t.Fatal("BE ARW signature not detected")
	}
	bad := [][]byte{
		nil,
		[]byte{'I', 'I', 0x2A, 0x00}, // TIFF but no SONY marker
		append([]byte{'I', 'I', 0x2B, 0x00}, []byte("SONY")...),
		bytes.Repeat([]byte{'X'}, 128),
	}
	for i, b := range bad {
		if (Decoder{}).Probe(bytes.NewReader(b)) {
			t.Fatalf("bad input %d accepted", i)
		}
	}
}

func TestDecodeStub(t *testing.T) {
	img, err := (Decoder{}).Decode(bytes.NewReader([]byte("II*\x00")), 4)
	if img != nil || err == nil {
		t.Fatalf("stub Decode returned img=%v err=%v", img, err)
	}
}

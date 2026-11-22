package gotiff

import (
	"encoding/binary"
	"testing"
)

// A single IFD entry with a large LONG array must be accepted as long as the
// total byte count stays within maxReadSize. The former arbitrary cap
// (maxIFDEntries*1024) rejected legitimate highly-striped or tiled files.
func TestReadIFDLargeValueArray(t *testing.T) {
	const n = 300000
	order := binary.LittleEndian
	entryCount := 1
	valuesOffset := 2 + entryCount*12 + 4
	data := make([]byte, valuesOffset+n*4)
	order.PutUint16(data[0:2], uint16(entryCount))
	p := data[2 : 2+12]
	order.PutUint16(p[0:2], tagStripOffsets)
	order.PutUint16(p[2:4], typeLong)
	order.PutUint32(p[4:8], uint32(n))
	order.PutUint32(p[8:12], uint32(valuesOffset))
	for i := 0; i < n; i++ {
		order.PutUint32(data[valuesOffset+i*4:], uint32(i))
	}
	entries, _, err := readIFD(data, order, 0, false)
	if err != nil {
		t.Fatalf("readIFD: %v", err)
	}
	vals, ok := values(entries, tagStripOffsets)
	if !ok || len(vals) != n {
		t.Fatalf("values len=%d ok=%v", len(vals), ok)
	}
	if vals[0] != 0 || vals[n-1] != uint64(n-1) {
		t.Fatalf("unexpected values: %d .. %d", vals[0], vals[n-1])
	}
}

// A crafted count that would overflow the uint32 entry count must be rejected
// instead of silently truncated.
func TestReadIFDRejectsHugeCount(t *testing.T) {
	order := binary.LittleEndian
	data := make([]byte, 8+2+12+4)
	data[0], data[1] = 'I', 'I'
	data[2], data[3] = 42, 0
	order.PutUint32(data[4:8], 8)
	order.PutUint16(data[8:10], 1)
	p := data[10:22]
	order.PutUint16(p[0:2], tagStripOffsets)
	order.PutUint16(p[2:4], typeLong8)
	order.PutUint32(p[4:8], 0xFFFFFFFF)
	if _, _, err := readIFD(data, order, 8, false); err == nil {
		t.Fatal("expected error for overflowing count")
	}
}

func TestDecodePackBitsRejectsHugeExpected(t *testing.T) {
	src := []byte{0x00, 0xAA}
	if _, err := decodePackBits(src, maxReadSize+1); err == nil {
		t.Fatal("expected error for expected > maxReadSize")
	}
}

func TestDecodePackBitsLiteralOverflow(t *testing.T) {
	// Literal run of 128 bytes but expected is only 4.
	src := []byte{127, 1, 2, 3, 4, 5}
	if _, err := decodePackBits(src, 4); err == nil {
		t.Fatal("expected error when literal run exceeds expected")
	}
}

func TestDecodePackBitsRunClamp(t *testing.T) {
	// Run of 10 bytes, expected 4: must clamp to 4, not error.
	src := []byte{257 - 10, 0x7F}
	out, err := decodePackBits(src, 4)
	if err != nil {
		t.Fatalf("decodePackBits: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("len=%d want 4", len(out))
	}
	for i := range out {
		if out[i] != 0x7F {
			t.Fatalf("byte %d = %#x", i, out[i])
		}
	}
}

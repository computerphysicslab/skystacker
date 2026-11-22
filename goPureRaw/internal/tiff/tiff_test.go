package tiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type testEntry struct {
	tag  uint16
	typ  uint16
	data []byte
}

// buildTIFF assembles a little- or big-endian TIFF with IFD0 and an Exif IFD.
func buildTIFF(t *testing.T, big bool, ifd0, exif []testEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	order := binary.ByteOrder(binary.LittleEndian)
	if big {
		order = binary.BigEndian
	}
	buf.Write([]byte{0, 0})
	_ = binary.Write(&buf, order, uint16(42))
	ifd0Off := uint32(8)
	_ = binary.Write(&buf, order, ifd0Off)
	// Reserve IFD0 space; data area follows both IFDs.
	ifd0Size := 2 + 12*len(ifd0) + 4
	exifOff := uint32(8 + ifd0Size)
	exifSize := 2 + 12*len(exif) + 4
	if len(exif) == 0 {
		exifSize = 0
	}
	dataOff := int(exifOff) + exifSize
	writeIFD := func(entries []testEntry, next uint32) []testEntry {
		_ = binary.Write(&buf, order, uint16(len(entries)))
		var overflow []testEntry
		for _, e := range entries {
			_ = binary.Write(&buf, order, e.tag)
			_ = binary.Write(&buf, order, e.typ)
			count := uint32(len(e.data))
			if e.typ == 2 || e.typ == 1 || e.typ == 7 {
				count = uint32(len(e.data))
			} else {
				size, _ := typeSize(e.typ)
				count = uint32(len(e.data) / size)
			}
			_ = binary.Write(&buf, order, count)
			if len(e.data) <= 4 {
				padded := make([]byte, 4)
				copy(padded, e.data)
				buf.Write(padded)
			} else {
				_ = binary.Write(&buf, order, uint32(dataOff))
				overflow = append(overflow, e)
				dataOff += len(e.data)
			}
		}
		_ = binary.Write(&buf, order, next)
		return overflow
	}
	nextIFD := exifOff
	if len(exif) == 0 {
		nextIFD = 0
	}
	overflow0 := writeIFD(ifd0, nextIFD)
	var overflowE []testEntry
	if len(exif) > 0 {
		overflowE = writeIFD(exif, 0)
	}
	for _, e := range append(overflow0, overflowE...) {
		buf.Write(e.data)
	}
	b := buf.Bytes()
	if big {
		b[0], b[1] = 'M', 'M'
	} else {
		b[0], b[1] = 'I', 'I'
	}
	return b
}

func shortVal(order binary.ByteOrder, v uint16) []byte {
	b := make([]byte, 2)
	order.PutUint16(b, v)
	return b
}

func longVal(order binary.ByteOrder, v uint32) []byte {
	b := make([]byte, 4)
	order.PutUint32(b, v)
	return b
}

func TestReadLEAndExif(t *testing.T) {
	order := binary.LittleEndian
	ifd0 := []testEntry{
		{tag: 0x010F, typ: 2, data: append([]byte("FUJIFILM"), 0)},
		{tag: 0x0110, typ: 2, data: append([]byte("X-T20"), 0)},
		{tag: 0x0112, typ: 3, data: shortVal(order, 6)},
		{tag: 0x8769, typ: 4, data: longVal(order, 0)}, // patched below
	}
	exif := []testEntry{
		{tag: 0x8827, typ: 3, data: shortVal(order, 800)},
		{tag: 0x829A, typ: 5, data: longVal(order, 1) /* overwritten below */},
	}
	// Exposure rational 1/250: build 8-byte value explicitly.
	rat := make([]byte, 8)
	order.PutUint32(rat[0:], 1)
	order.PutUint32(rat[4:], 250)
	exif[1].data = rat
	// ExifIFD offset is 8 + 2 + 12*len(ifd0) + 4.
	exifOff := uint32(8 + 2 + 12*len(ifd0) + 4)
	ifd0[3].data = longVal(order, exifOff)
	b := buildTIFF(t, false, ifd0, exif)
	doc, err := Read(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Exif == nil {
		t.Fatal("Exif IFD not followed")
	}
	if got := doc.IFD0.Entries[0x010F].String(); got != "FUJIFILM" {
		t.Fatalf("Make = %q", got)
	}
	v, err := doc.Exif.Entries[0x8827].Uint16(order, 0)
	if err != nil || v != 800 {
		t.Fatalf("ISO = %d (%v)", v, err)
	}
	r, err := doc.Exif.Entries[0x829A].Rational(order, 0)
	if err != nil || r > 0.0041 || r < 0.0039 {
		t.Fatalf("Exposure = %g (%v)", r, err)
	}
}

func TestReadBEAndBadHeader(t *testing.T) {
	order := binary.BigEndian
	ifd0 := []testEntry{{tag: 0x0110, typ: 2, data: append([]byte("CAMERA"), 0)}}
	b := buildTIFF(t, true, ifd0, nil)
	doc, err := Read(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.IFD0.Entries[0x0110].String(); got != "CAMERA" {
		t.Fatalf("Model = %q", got)
	}
	for _, bad := range [][]byte{nil, []byte("xx\x2a\x00\x08\x00\x00\x00"), {0, 0, 7, 0, 8, 0, 0, 0}} {
		if _, err := Read(bytes.NewReader(bad)); err == nil {
			t.Fatal("bad TIFF header accepted")
		}
	}
	_ = order
}

func TestTagName(t *testing.T) {
	if got := TagName(0x010F); got != "Make" {
		t.Fatalf("TagName = %q", got)
	}
	if got := TagName(0x9999); got != "tag_0x9999" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestReadIFDRejectsHugeEntryCount(t *testing.T) {
	// Two bytes 0xFFFF as the entry count must not allocate 12*65535 bytes.
	data := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 0xFF, 0xFF, 0, 0}
	_, err := readIFD(bytes.NewReader(data), binary.LittleEndian, 8)
	if err == nil {
		t.Fatal("huge IFD entry count accepted")
	}
}

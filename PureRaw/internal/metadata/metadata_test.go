package metadata

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/computerphysicslab/skystacker/PureRaw/internal/tiff"
)

type entry struct {
	tag  uint16
	typ  uint16
	data []byte
}

func buildTIFF(t *testing.T, ifd0, exif []entry) []byte {
	t.Helper()
	order := binary.LittleEndian
	var buf bytes.Buffer
	buf.Write([]byte{'I', 'I'})
	_ = binary.Write(&buf, order, uint16(42))
	_ = binary.Write(&buf, order, uint32(8))
	ifd0Size := 2 + 12*len(ifd0) + 4
	exifOff := uint32(8 + ifd0Size)
	exifSize := 2 + 12*len(exif) + 4
	if len(exif) == 0 {
		exifSize = 0
	}
	dataOff := int(exifOff) + exifSize
	writeIFD := func(entries []entry, next uint32) []entry {
		_ = binary.Write(&buf, order, uint16(len(entries)))
		var overflow []entry
		for _, e := range entries {
			_ = binary.Write(&buf, order, e.tag)
			_ = binary.Write(&buf, order, e.typ)
			count := uint32(len(e.data))
			switch e.typ {
			case 3:
				count = uint32(len(e.data) / 2)
			case 4:
				count = uint32(len(e.data) / 4)
			case 5:
				count = uint32(len(e.data) / 8)
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
	ov0 := writeIFD(ifd0, exifOff)
	ovE := []entry{}
	if len(exif) > 0 {
		ovE = writeIFD(exif, 0)
	}
	for _, e := range append(ov0, ovE...) {
		buf.Write(e.data)
	}
	return buf.Bytes()
}

func short(order binary.ByteOrder, v uint16) []byte {
	b := make([]byte, 2)
	order.PutUint16(b, v)
	return b
}

func rat(order binary.ByteOrder, n, d uint32) []byte {
	b := make([]byte, 8)
	order.PutUint32(b[0:], n)
	order.PutUint32(b[4:], d)
	return b
}

func TestFromTIFF(t *testing.T) {
	order := binary.LittleEndian
	sh := func(v uint16) []byte { return short(order, v) }
	ifd0 := []entry{
		{tag: 0x010F, typ: 2, data: append([]byte("FUJIFILM"), 0)},
		{tag: 0x0110, typ: 2, data: append([]byte("X-T20"), 0)},
		{tag: 0x0112, typ: 3, data: sh(6)},
		{tag: 0x0132, typ: 2, data: []byte("2017:11:18 16:30:00\x00")},
		{tag: 0x8769, typ: 4, data: sh(0)}, // overwritten below
	}
	exifOff := uint32(8 + 2 + 12*len(ifd0) + 4)
	ifd0[4].data = make([]byte, 4)
	order.PutUint32(ifd0[4].data, exifOff)
	exif := []entry{
		{tag: 0x8827, typ: 3, data: sh(400)},
		{tag: 0x829A, typ: 5, data: rat(order, 1, 125)},
		{tag: 0x829D, typ: 5, data: rat(order, 56, 10)},
		{tag: 0x920A, typ: 5, data: rat(order, 50, 1)},
		{tag: 0xA431, typ: 2, data: append([]byte("SN123"), 0)},
	}
	doc, err := tiff.Read(bytes.NewReader(buildTIFF(t, ifd0, exif)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := FromTIFF(doc)
	if err != nil {
		t.Fatal(err)
	}
	if m.Make != "FUJIFILM" || m.Model != "X-T20" || m.SerialNumber != "SN123" {
		t.Fatalf("identity: %+v", m)
	}
	if m.ISO != 400 || m.Orientation != 6 || m.Aperture != 5.6 || m.FocalLength != 50 {
		t.Fatalf("exposure/geometry: %+v", m)
	}
	if m.ExposureTime != time.Second/125 {
		t.Fatalf("exposure = %v", m.ExposureTime)
	}
	if !m.DateTime.Equal(time.Date(2017, 11, 18, 16, 30, 0, 0, time.UTC)) {
		t.Fatalf("DateTime = %v", m.DateTime)
	}
	if len(m.Vendor) != len(ifd0)+len(exif) {
		t.Fatalf("vendor tags = %d, want %d", len(m.Vendor), len(ifd0)+len(exif))
	}
	if _, err := FromTIFF(nil); err == nil {
		t.Fatal("nil document accepted")
	}
}

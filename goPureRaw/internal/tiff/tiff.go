// Package tiff implements a minimal, dependency-free TIFF IFD reader for
// the subset needed by camera RAW containers (RAF's TIFF block and ARW).
// It uses only io.ReaderAt and the Go standard library.
package tiff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	ErrBadHeader = errors.New("tiff: bad header")
	ErrTruncated = errors.New("tiff: truncated IFD")
	ErrBadType   = errors.New("tiff: unsupported field type")
)

// maxIFDEntries bounds the allocation derived from the on-disk entry count.
// A hostile file may claim up to 65535 entries; real RAW containers stay
// well below this cap.
const maxIFDEntries = 4096

// Entry is one IFD directory entry. Raw is the value bytes (inline or
// fetched from the referenced offset), already decoded in file byte order.
type Entry struct {
	Tag   uint16
	Type  uint16
	Count uint32
	Raw   []byte
}

// IFD is a decoded image file directory.
type IFD struct {
	Entries map[uint16]Entry
	Next    uint32 // byte offset of the next IFD, 0 if none
}

// ByteOrder records the byte order of the source file.
type ByteOrder binary.ByteOrder

// Document is a parsed TIFF header plus IFD0, the optional Exif IFD and
// any SubIFDs referenced from IFD0 (CR2 stores raw sensor data there).
type Document struct {
	Order   binary.ByteOrder
	IFD0    *IFD
	IFD1    *IFD
	Exif    *IFD
	SubIFDs []*IFD
}

func typeSize(t uint16) (int, error) {
	switch t {
	case 1, 2, 6, 7: // BYTE, ASCII, SBYTE, UNDEFINED
		return 1, nil
	case 3, 8: // SHORT, SSHORT
		return 2, nil
	case 4, 9, 11: // LONG, SLONG, FLOAT
		return 4, nil
	case 5, 10, 12: // RATIONAL, SRATIONAL, DOUBLE
		return 8, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrBadType, t)
	}
}

// Read parses the TIFF header at offset 0.
func Read(r io.ReaderAt) (*Document, error) {
	return ReadAt(r, 0)
}

// ReadAt parses a TIFF header at the given byte offset and follows the
// standard ExifIFD pointer (tag 0x8769) when present.
func ReadAt(r io.ReaderAt, offset int64) (*Document, error) {
	hdr := make([]byte, 8)
	if _, err := r.ReadAt(hdr, offset); err != nil {
		return nil, ErrBadHeader
	}
	var order binary.ByteOrder
	switch {
	case hdr[0] == 'I' && hdr[1] == 'I':
		order = binary.LittleEndian
	case hdr[0] == 'M' && hdr[1] == 'M':
		order = binary.BigEndian
	default:
		return nil, ErrBadHeader
	}
	if order.Uint16(hdr[2:]) != 42 {
		return nil, ErrBadHeader
	}
	off := offset + int64(order.Uint32(hdr[4:]))
	ifd0, err := readIFD(r, order, off)
	if err != nil {
		return nil, err
	}
	doc := &Document{Order: order, IFD0: ifd0}
	if ifd0.Next != 0 {
		if next, nerr := readIFD(r, order, offset+int64(ifd0.Next)); nerr == nil {
			doc.IFD1 = next
		}
	}
	if e, ok := ifd0.Entries[0x8769]; ok {
		p, err := e.Uint32(order, 0)
		if err == nil && p != 0 {
			doc.Exif, _ = readIFD(r, order, offset+int64(p))
		}
	}
	// SubIFDs: CR2 keeps the raw sensor strip metadata in IFD0's SubIFD.
	if e, ok := ifd0.Entries[0x014A]; ok {
		for i := 0; i < int(e.Count); i++ {
			p, err := e.Uint32(order, i)
			if err != nil || p == 0 {
				continue
			}
			sub, serr := readIFD(r, order, offset+int64(p))
			if serr == nil {
				doc.SubIFDs = append(doc.SubIFDs, sub)
			}
		}
	}
	return doc, nil
}

func readIFD(r io.ReaderAt, order binary.ByteOrder, off int64) (*IFD, error) {
	cntBytes := make([]byte, 2)
	if _, err := r.ReadAt(cntBytes, off); err != nil {
		return nil, ErrTruncated
	}
	n := int(order.Uint16(cntBytes))
	if n == 0 || n > maxIFDEntries {
		return nil, ErrTruncated
	}
	ifd := &IFD{Entries: make(map[uint16]Entry, n)}
	base := off + 2
	entryBytes := make([]byte, 12*n+4)
	if _, err := r.ReadAt(entryBytes, base); err != nil {
		return nil, ErrTruncated
	}
	for i := range n {
		e := entryBytes[i*12 : i*12+12]
		tag := order.Uint16(e[0:])
		typ := order.Uint16(e[2:])
		count := order.Uint32(e[4:])
		size, err := typeSize(typ)
		if err != nil {
			continue // unknown tag type: skip, do not fail the whole IFD
		}
		total := int(count) * size
		if total < 0 || total > 1<<24 {
			return nil, ErrTruncated
		}
		var raw []byte
		if total <= 4 {
			raw = append([]byte(nil), e[8:8+total]...)
		} else {
			voff := int64(order.Uint32(e[8:]))
			raw = make([]byte, total)
			if _, err := r.ReadAt(raw, voff); err != nil {
				return nil, ErrTruncated
			}
		}
		ifd.Entries[tag] = Entry{Tag: tag, Type: typ, Count: count, Raw: raw}
	}
	tail := entryBytes[12*n:]
	ifd.Next = order.Uint32(tail[:4])
	return ifd, nil
}

// Uint16 returns the idx-th SHORT value, decoded in file byte order.
func (e Entry) Uint16(order binary.ByteOrder, idx int) (uint16, error) {
	if e.Type != 3 || idx*2+2 > len(e.Raw) {
		return 0, ErrBadType
	}
	return order.Uint16(e.Raw[idx*2:]), nil
}

// Uint32 returns the idx-th LONG value, decoded in file byte order.
func (e Entry) Uint32(order binary.ByteOrder, idx int) (uint32, error) {
	if e.Type != 4 || idx*4+4 > len(e.Raw) {
		return 0, ErrBadType
	}
	return order.Uint32(e.Raw[idx*4:]), nil
}

// String returns the NUL-terminated ASCII value.
func (e Entry) String() string {
	if e.Type != 2 {
		return ""
	}
	b := e.Raw
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

func indexByte(b []byte, v byte) int {
	for i, x := range b {
		if x == v {
			return i
		}
	}
	return -1
}

// Rational returns the idx-th unsigned RATIONAL as a float64.
func (e Entry) Rational(order binary.ByteOrder, idx int) (float64, error) {
	if e.Type != 5 || idx*8+8 > len(e.Raw) {
		return 0, ErrBadType
	}
	num := order.Uint32(e.Raw[idx*8:])
	den := order.Uint32(e.Raw[idx*8+4:])
	if den == 0 {
		return 0, nil
	}
	return float64(num) / float64(den), nil
}

// TagName returns a human-readable name for common TIFF/Exif tags, or a
// hexadecimal fallback for unknown tags.
func TagName(tag uint16) string {
	if n, ok := knownTags[tag]; ok {
		return n
	}
	return fmt.Sprintf("tag_0x%04x", tag)
}

var knownTags = map[uint16]string{
	0x010F: "Make",
	0x0110: "Model",
	0x0112: "Orientation",
	0x0132: "DateTime",
	0x829A: "ExposureTime",
	0x829D: "FNumber",
	0x8769: "ExifIFD",
	0x8827: "ISO",
	0x920A: "FocalLength",
	0xA431: "BodySerialNumber",
	0xA434: "LensModel",
}

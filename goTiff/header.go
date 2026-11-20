package gotiff

import (
	"encoding/binary"
	"fmt"
)

const (
	maxReadSize   = int64(1) << 30
	maxPixels     = 200_000_000
	maxIFDEntries = 256
)

type ifdEntry struct {
	tag      uint16
	dataType uint16
	count    uint32
	values   []uint64
	ascii    []byte
}

func typeSize(t uint16) (int, error) {
	switch t {
	case typeByte, typeASCII, typeUndefined:
		return 1, nil
	case typeShort:
		return 2, nil
	case typeLong:
		return 4, nil
	case typeRational:
		return 8, nil
	default:
		return 0, UnsupportedError(fmt.Sprintf("IFD datatype %d", t))
	}
}

func readHeader(data []byte) (binary.ByteOrder, uint32, error) {
	if len(data) < 8 {
		return nil, 0, errInvalidHeader
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, 0, errInvalidHeader
	}
	if order.Uint16(data[2:4]) != classicMagic {
		return nil, 0, errInvalidHeader
	}
	return order, order.Uint32(data[4:8]), nil
}

func readIFD(data []byte, order binary.ByteOrder, offset uint32) (map[uint16]ifdEntry, error) {
	if int64(offset)+2 > int64(len(data)) {
		return nil, errBadIFD
	}
	n := int(order.Uint16(data[offset : offset+2]))
	if n == 0 || n > maxIFDEntries {
		return nil, errBadIFD
	}
	start := int64(offset) + 2
	if start+int64(n)*12+4 > int64(len(data)) {
		return nil, errBadIFD
	}
	entries := make(map[uint16]ifdEntry, n)
	for i := 0; i < n; i++ {
		p := data[start+int64(i)*12 : start+int64(i)*12+12]
		e := ifdEntry{
			tag:      order.Uint16(p[0:2]),
			dataType: order.Uint16(p[2:4]),
			count:    order.Uint32(p[4:8]),
		}
		size, err := typeSize(e.dataType)
		if err != nil {
			return nil, err
		}
		total := int64(e.count) * int64(size)
		if total < 0 || total > maxReadSize {
			return nil, errBadIFD
		}
		var raw []byte
		if total <= 4 {
			raw = p[8 : 8+total]
		} else {
			off := int64(order.Uint32(p[8:12]))
			if off < 0 || off+total > int64(len(data)) {
				return nil, errBadIFD
			}
			raw = data[off : off+total]
		}
		if e.dataType == typeASCII {
			e.ascii = append([]byte(nil), raw...)
			e.count = uint32(len(raw))
		} else {
			e.values = make([]uint64, 0, e.count)
			for j := int64(0); j < total; j += int64(size) {
				switch e.dataType {
				case typeByte, typeUndefined:
					e.values = append(e.values, uint64(raw[j]))
				case typeShort:
					e.values = append(e.values, uint64(order.Uint16(raw[j:j+2])))
				case typeLong:
					e.values = append(e.values, uint64(order.Uint32(raw[j:j+4])))
				case typeRational:
					num := order.Uint32(raw[j : j+4])
					den := order.Uint32(raw[j+4 : j+8])
					if den == 0 {
						return nil, errBadIFD
					}
					e.values = append(e.values, uint64(num), uint64(den))
				}
			}
		}
		entries[e.tag] = e
	}
	return entries, nil
}

func firstValue(entries map[uint16]ifdEntry, tag uint16) (uint64, bool) {
	e, ok := entries[tag]
	if !ok || len(e.values) == 0 {
		return 0, false
	}
	return e.values[0], true
}

func values(entries map[uint16]ifdEntry, tag uint16) ([]uint64, bool) {
	e, ok := entries[tag]
	if !ok || len(e.values) == 0 {
		return nil, false
	}
	return e.values, true
}

func metadataFromIFD(entries map[uint16]ifdEntry) map[string]string {
	out := make(map[string]string)
	named := map[uint16]string{
		tagImageDescription: "ImageDescription",
		tagSoftware:         "Software",
		tagDateTime:         "DateTime",
		tagArtist:           "Artist",
	}
	for tag, name := range named {
		if e, ok := entries[tag]; ok && len(e.ascii) > 0 {
			out[name] = string(trimNUL(e.ascii))
		}
	}
	return out
}

func trimNUL(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

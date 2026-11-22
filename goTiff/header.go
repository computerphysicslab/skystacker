package gotiff

import (
	"encoding/binary"
	"fmt"
	"math"
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
	case typeRational, typeLong8, typeSLong8, typeIFD8:
		return 8, nil
	default:
		return 0, UnsupportedError(fmt.Sprintf("IFD datatype %d", t))
	}
}

func readHeader(data []byte) (binary.ByteOrder, uint16, uint64, error) {
	if len(data) < 8 {
		return nil, 0, 0, errInvalidHeader
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, 0, 0, errInvalidHeader
	}
	magic := order.Uint16(data[2:4])
	switch magic {
	case classicMagic:
		return order, magic, uint64(order.Uint32(data[4:8])), nil
	case bigMagic:
		if len(data) < 16 || order.Uint16(data[4:6]) != 8 || order.Uint16(data[6:8]) != 0 {
			return nil, 0, 0, errInvalidHeader
		}
		return order, magic, order.Uint64(data[8:16]), nil
	default:
		return nil, 0, 0, errInvalidHeader
	}
}

func readIFD(data []byte, order binary.ByteOrder, offset uint64, big bool) (map[uint16]ifdEntry, uint64, error) {
	if offset > uint64(len(data)) {
		return nil, 0, errBadIFD
	}
	entrySize, countSize, countBytes := 12, 2, 2
	if big {
		entrySize, countSize, countBytes = 20, 8, 8
	}
	if offset+uint64(countSize) > uint64(len(data)) {
		return nil, 0, errBadIFD
	}
	o := int(offset)
	var n int
	if big {
		n = int(order.Uint64(data[o : o+8]))
	} else {
		n = int(order.Uint16(data[o : o+2]))
	}
	if n <= 0 || n > maxIFDEntries {
		return nil, 0, errBadIFD
	}
	start := o + countBytes
	if start+n*entrySize+countBytes > len(data) {
		return nil, 0, errBadIFD
	}
	entries := make(map[uint16]ifdEntry, n)
	for i := 0; i < n; i++ {
		p := data[start+i*entrySize : start+(i+1)*entrySize]
		e := ifdEntry{tag: order.Uint16(p[0:2]), dataType: order.Uint16(p[2:4])}
		size, err := typeSize(e.dataType)
		if err != nil {
			return nil, 0, err
		}
		var count uint64
		var valuePos int
		if big {
			count = order.Uint64(p[4:12])
			valuePos = 12
		} else {
			count = uint64(order.Uint32(p[4:8]))
			valuePos = 8
		}
		if count > math.MaxUint32 {
			return nil, 0, errBadIFD
		}
		e.count = uint32(count)
		if size <= 0 || count > uint64(maxReadSize)/uint64(size) {
			return nil, 0, errBadIFD
		}
		total := int64(count) * int64(size)
		if total < 0 || total > maxReadSize {
			return nil, 0, errBadIFD
		}
		var raw []byte
		if big {
			if total <= 8 {
				raw = p[valuePos : valuePos+int(total)]
			} else {
				off := order.Uint64(p[valuePos : valuePos+8])
				if off > uint64(len(data)) || off+uint64(total) > uint64(len(data)) {
					return nil, 0, errBadIFD
				}
				raw = data[off : off+uint64(total)]
			}
		} else {
			if total <= 4 {
				raw = p[valuePos : valuePos+int(total)]
			} else {
				off := uint64(order.Uint32(p[valuePos : valuePos+4]))
				if off > uint64(len(data)) || off+uint64(total) > uint64(len(data)) {
					return nil, 0, errBadIFD
				}
				raw = data[off : off+uint64(total)]
			}
		}
		if e.dataType == typeASCII {
			e.ascii = append([]byte(nil), raw...)
			e.count = uint32(len(raw))
		} else {
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
						return nil, 0, errBadIFD
					}
					e.values = append(e.values, uint64(num), uint64(den))
				case typeLong8, typeSLong8, typeIFD8:
					e.values = append(e.values, order.Uint64(raw[j:j+8]))
				}
			}
		}
		entries[e.tag] = e
	}
	nextPos := start + n*entrySize
	var next uint64
	if big {
		next = order.Uint64(data[nextPos : nextPos+8])
	} else {
		next = uint64(order.Uint32(data[nextPos : nextPos+4]))
	}
	return entries, next, nil
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

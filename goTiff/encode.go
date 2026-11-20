package gotiff

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

type writeEntry struct {
	tag      uint16
	dataType uint16
	count    uint32
	data     []byte
	inline   bool
	pointer  uint32
}

func encodeBytes(img *Image, opt *Options) ([]byte, error) {
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return nil, errBadIFD
	}
	if int64(img.Width)*int64(img.Height) > maxPixels {
		return nil, errBadIFD
	}
	if img.Channels != 1 && img.Channels != 3 {
		return nil, UnsupportedError(fmt.Sprintf("channels %d", img.Channels))
	}
	if img.Photometric == PhotoRGB && img.Channels != 3 {
		return nil, errBadIFD
	}
	if img.Photometric != PhotoRGB && img.Channels != 1 {
		return nil, errBadIFD
	}
	if img.Photometric != PhotoBlackIsZero && img.Photometric != PhotoWhiteIsZero && img.Photometric != PhotoRGB {
		return nil, UnsupportedError(fmt.Sprintf("photometric %d", img.Photometric))
	}
	bytesPerSample, err := sampleBytes(img.Bits, img.Format)
	if err != nil {
		return nil, err
	}
	if len(img.Pixels) != img.Width*img.Height*img.Channels {
		return nil, errNoPixels
	}
	if opt.Compression != CompressionNone && opt.Compression != 0 {
		return nil, UnsupportedError(fmt.Sprintf("compression %d", opt.Compression))
	}
	order := opt.ByteOrder
	if order == nil {
		order = binary.LittleEndian
	}
	pixels, err := encodePixels(img, order, bytesPerSample)
	if err != nil {
		return nil, err
	}
	entries := []writeEntry{
		{tag: tagImageWidth, dataType: typeLong, count: 1, data: uint32Bytes(order, uint32(img.Width)), inline: true},
		{tag: tagImageLength, dataType: typeLong, count: 1, data: uint32Bytes(order, uint32(img.Height)), inline: true},
		{tag: tagBitsPerSample, dataType: typeShort, count: uint32(img.Channels), data: repeatedShort(order, uint16(img.Bits), img.Channels)},
		{tag: tagCompression, dataType: typeShort, count: 1, data: shortBytes(order, uint16(CompressionNone)), inline: true},
		{tag: tagPhotometric, dataType: typeShort, count: 1, data: shortBytes(order, uint16(img.Photometric)), inline: true},
		{tag: tagSamplesPerPixel, dataType: typeShort, count: 1, data: shortBytes(order, uint16(img.Channels)), inline: true},
		{tag: tagRowsPerStrip, dataType: typeLong, count: 1, data: uint32Bytes(order, uint32(img.Height)), inline: true},
		{tag: tagPlanarConfiguration, dataType: typeShort, count: 1, data: shortBytes(order, 1), inline: true},
		{tag: tagResolutionUnit, dataType: typeShort, count: 1, data: shortBytes(order, 2), inline: true},
		{tag: tagSampleFormat, dataType: typeShort, count: uint32(img.Channels), data: repeatedShort(order, uint16(img.Format), img.Channels)},
	}
	resolution := append(uint32Bytes(order, 72), uint32Bytes(order, 1)...)
	entries = append(entries,
		writeEntry{tag: tagXResolution, dataType: typeRational, count: 1, data: resolution},
		writeEntry{tag: tagYResolution, dataType: typeRational, count: 1, data: append([]byte(nil), resolution...)},
	)
	stripOffset := uint32(8)
	entries = append(entries,
		writeEntry{tag: tagStripOffsets, dataType: typeLong, count: 1, data: uint32Bytes(order, stripOffset), inline: true},
		writeEntry{tag: tagStripByteCounts, dataType: typeLong, count: 1, data: uint32Bytes(order, uint32(len(pixels))), inline: true},
	)
	meta := make(map[string]string)
	for k, v := range opt.Metadata {
		meta[k] = v
	}
	for k, v := range img.Metadata {
		meta[k] = v
	}
	for _, m := range []struct {
		tag  uint16
		name string
	}{{tagImageDescription, "ImageDescription"}, {tagSoftware, "Software"}, {tagDateTime, "DateTime"}, {tagArtist, "Artist"}} {
		if value, ok := meta[m.name]; ok {
			data := append([]byte(value), 0)
			entries = append(entries, writeEntry{tag: m.tag, dataType: typeASCII, count: uint32(len(data)), data: data})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].tag < entries[j].tag })
	for i := range entries {
		if len(entries[i].data) <= 4 {
			entries[i].inline = true
		}
	}
	ifdOffset := 8 + len(pixels)
	pointerStart := int64(ifdOffset) + 2 + int64(len(entries))*12 + 4
	var pointer bytes.Buffer
	for i := range entries {
		if entries[i].inline {
			continue
		}
		if rem := len(entries[i].data) % 4; rem != 0 {
			entries[i].data = append(entries[i].data, make([]byte, 4-rem)...)
		}
		entries[i].pointer = uint32(pointerStart + int64(pointer.Len()))
		pointer.Write(entries[i].data)
	}
	buf := make([]byte, 0, ifdOffset+int(pointer.Len())+16)
	if order == binary.LittleEndian {
		buf = append(buf, 'I', 'I')
	} else {
		buf = append(buf, 'M', 'M')
	}
	buf = append(buf, shortBytes(order, classicMagic)...)
	buf = append(buf, uint32Bytes(order, uint32(ifdOffset))...)
	buf = append(buf, pixels...)
	buf = append(buf, shortBytes(order, uint16(len(entries)))...)
	for _, e := range entries {
		buf = append(buf, shortBytes(order, e.tag)...)
		buf = append(buf, shortBytes(order, e.dataType)...)
		buf = append(buf, uint32Bytes(order, e.count)...)
		if e.inline {
			value := make([]byte, 4)
			copy(value, e.data)
			buf = append(buf, value...)
		} else {
			buf = append(buf, uint32Bytes(order, e.pointer)...)
		}
	}
	buf = append(buf, uint32Bytes(order, 0)...)
	buf = append(buf, pointer.Bytes()...)
	return buf, nil
}

func encodePixels(img *Image, order binary.ByteOrder, bytesPerSample int) ([]byte, error) {
	out := make([]byte, img.Width*img.Height*img.Channels*bytesPerSample)
	pos := 0
	for _, v := range img.Pixels {
		sample := float64(v)
		if img.Photometric == PhotoWhiteIsZero && img.Format != SampleFloat {
			sample = 1 - sample
		}
		switch img.Format {
		case SampleUint:
			switch img.Bits {
			case 8:
				out[pos] = byte(clampUint(sample, 255))
				pos++
			case 16:
				order.PutUint16(out[pos:pos+2], uint16(clampUint(sample, 65535)))
				pos += 2
			case 32:
				order.PutUint32(out[pos:pos+4], uint32(clampUint64(sample, math.MaxUint32)))
				pos += 4
			case 64:
				order.PutUint64(out[pos:pos+8], clampUint64(sample, math.MaxUint64))
				pos += 8
			}
		case SampleInt:
			switch img.Bits {
			case 8:
				out[pos] = byte(clampInt(sample, math.MinInt8, math.MaxInt8))
				pos++
			case 16:
				order.PutUint16(out[pos:pos+2], uint16(clampInt(sample, math.MinInt16, math.MaxInt16)))
				pos += 2
			case 32:
				order.PutUint32(out[pos:pos+4], uint32(clampInt(sample, math.MinInt32, math.MaxInt32)))
				pos += 4
			case 64:
				order.PutUint64(out[pos:pos+8], uint64(clampInt64(sample)))
				pos += 8
			}
		case SampleFloat:
			switch img.Bits {
			case 32:
				order.PutUint32(out[pos:pos+4], math.Float32bits(float32(sample)))
				pos += 4
			case 64:
				order.PutUint64(out[pos:pos+8], math.Float64bits(sample))
				pos += 8
			}
		}
	}
	return out, nil
}

func clampUint(v float64, max uint64) uint64 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return max
	}
	return uint64(v * float64(max))
}

func clampUint64(v float64, max uint64) uint64 {
	if v <= 0 {
		return 0
	}
	if v >= float64(max) {
		return max
	}
	return uint64(v)
}

func clampInt(v float64, min, max int64) int64 {
	if v < float64(min) {
		return min
	}
	if v > float64(max) {
		return max
	}
	return int64(v)
}

func clampInt64(v float64) uint64 {
	if v < math.MinInt64 {
		return uint64(1) << 63
	}
	if v > math.MaxInt64 {
		return uint64(math.MaxInt64)
	}
	return uint64(int64(v))
}

func shortBytes(order binary.ByteOrder, v uint16) []byte {
	b := make([]byte, 2)
	order.PutUint16(b, v)
	return b
}

func uint32Bytes(order binary.ByteOrder, v uint32) []byte {
	b := make([]byte, 4)
	order.PutUint32(b, v)
	return b
}

func repeatedShort(order binary.ByteOrder, v uint16, n int) []byte {
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		order.PutUint16(out[i*2:i*2+2], v)
	}
	return out
}

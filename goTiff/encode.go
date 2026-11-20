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
	pointer  uint64
}

type encodePage struct {
	blocks  [][]byte
	offsets []uint64
	counts  []uint64
	entries []writeEntry
	ifdSize int64
	tiled   bool
	tileW   int
	tileH   int
}

func encodeAllBytes(images []*Image, opt *Options) ([]byte, error) {
	if len(images) == 0 {
		return nil, errBadIFD
	}
	order := opt.ByteOrder
	if order == nil {
		order = binary.LittleEndian
	}
	compression := opt.Compression
	if compression == 0 {
		compression = CompressionNone
	}
	switch compression {
	case CompressionNone, CompressionLZW, CompressionDeflate, CompressionDeflateOld, CompressionPackBits:
	default:
		return nil, UnsupportedError(fmt.Sprintf("compression %d", compression))
	}
	if opt.Predictor && compression != CompressionLZW && compression != CompressionDeflate && compression != CompressionDeflateOld {
		return nil, UnsupportedError("predictor requires LZW or Deflate")
	}
	big := opt.BigTIFF
	pages := make([]*encodePage, 0, len(images))
	for _, img := range images {
		page, err := preparePage(img, order, opt, compression, big)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	headerSize := 8
	if big {
		headerSize = 16
	}
	offset := uint64(headerSize)
	for _, page := range pages {
		page.offsets = make([]uint64, len(page.blocks))
		for i, block := range page.blocks {
			page.offsets[i] = offset
			offset += uint64(len(block))
		}
	}
	ifdOffsets := make([]uint64, len(pages))
	for i, page := range pages {
		ifdOffsets[i] = offset
		offset += uint64(page.ifdSize)
	}
	for i, page := range pages {
		setBlockEntries(page.entries, page.offsets, page.counts, big, order)
		assignPointers(page.entries, ifdOffsets[i], big)
	}
	buf := make([]byte, 0, offset)
	if order == binary.LittleEndian {
		buf = append(buf, 'I', 'I')
	} else {
		buf = append(buf, 'M', 'M')
	}
	if big {
		buf = append(buf, shortBytes(order, bigMagic)...)
		buf = append(buf, shortBytes(order, 8)...)
		buf = append(buf, shortBytes(order, 0)...)
		buf = append(buf, uint64Bytes(order, ifdOffsets[0])...)
	} else {
		buf = append(buf, shortBytes(order, classicMagic)...)
		buf = append(buf, uint32Bytes(order, uint32(ifdOffsets[0]))...)
	}
	for _, page := range pages {
		for _, block := range page.blocks {
			buf = append(buf, block...)
		}
	}
	for i, page := range pages {
		buf = append(buf, encodeIFD(page.entries, ifdOffsets[i], nextOffset(ifdOffsets, i, big), big, order)...)
	}
	return buf, nil
}

func nextOffset(offsets []uint64, i int, big bool) uint64 {
	if i+1 < len(offsets) {
		return offsets[i+1]
	}
	return 0
}

func preparePage(img *Image, order binary.ByteOrder, opt *Options, compression Compression, big bool) (*encodePage, error) {
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
	pixels, err := encodePixels(img, order, bytesPerSample)
	if err != nil {
		return nil, err
	}
	page := &encodePage{}
	if opt.Tiled {
		page.tiled = true
		page.tileW, page.tileH = 256, 256
		if opt.TileWidth > 0 {
			page.tileW = opt.TileWidth
		}
		if opt.TileHeight > 0 {
			page.tileH = opt.TileHeight
		}
		across := (img.Width + page.tileW - 1) / page.tileW
		down := (img.Height + page.tileH - 1) / page.tileH
		tileRowBytes := page.tileW * img.Channels * bytesPerSample
		for ty := 0; ty < down; ty++ {
			for tx := 0; tx < across; tx++ {
				tile := make([]byte, tileRowBytes*page.tileH)
				for y := 0; y < page.tileH; y++ {
					gy := ty*page.tileH + y
					if gy >= img.Height {
						break
					}
					src := (gy*img.Width + tx*page.tileW) * img.Channels * bytesPerSample
					copy(tile[y*tileRowBytes:], pixels[src:src+min(img.Width-tx*page.tileW, page.tileW)*img.Channels*bytesPerSample])
				}
				if opt.Predictor {
					if err := applyPredictor(tile, tileRowBytes, img.Channels*bytesPerSample, true); err != nil {
						return nil, err
					}
				}
				block, err := compressStrip(tile, compression)
				if err != nil {
					return nil, err
				}
				page.blocks = append(page.blocks, block)
				page.counts = append(page.counts, uint64(len(block)))
			}
		}
	} else {
		if opt.Predictor {
			if err := applyPredictor(pixels, img.Width*img.Channels*bytesPerSample, img.Channels*bytesPerSample, true); err != nil {
				return nil, err
			}
		}
		block, err := compressStrip(pixels, compression)
		if err != nil {
			return nil, err
		}
		page.blocks = [][]byte{block}
		page.counts = []uint64{uint64(len(block))}
	}
	entries := buildEntries(img, order, opt, compression, page, big)
	sort.Slice(entries, func(i, j int) bool { return entries[i].tag < entries[j].tag })
	limit := 4
	if big {
		limit = 8
	}
	for i := range entries {
		entries[i].inline = len(entries[i].data) <= limit
	}
	countSize, entrySize, nextSize := 2, 12, 4
	if big {
		countSize, entrySize, nextSize = 8, 20, 8
	}
	page.entries = entries
	page.ifdSize = int64(countSize + len(entries)*entrySize + nextSize + pointerAreaSize(entries, big))
	return page, nil
}

func buildEntries(img *Image, order binary.ByteOrder, opt *Options, compression Compression, page *encodePage, big bool) []writeEntry {
	dimData := func(v int) []byte {
		if big {
			return uint64Bytes(order, uint64(v))
		}
		return uint32Bytes(order, uint32(v))
	}
	dimType := uint16(typeLong)
	if big {
		dimType = typeLong8
	}
	entries := []writeEntry{
		{tag: tagImageWidth, dataType: dimType, count: 1, data: dimData(img.Width)},
		{tag: tagImageLength, dataType: dimType, count: 1, data: dimData(img.Height)},
		{tag: tagBitsPerSample, dataType: typeShort, count: uint32(img.Channels), data: repeatedShort(order, uint16(img.Bits), img.Channels)},
		{tag: tagCompression, dataType: typeShort, count: 1, data: shortBytes(order, uint16(compression))},
		{tag: tagPhotometric, dataType: typeShort, count: 1, data: shortBytes(order, uint16(img.Photometric))},
		{tag: tagSamplesPerPixel, dataType: typeShort, count: 1, data: shortBytes(order, uint16(img.Channels))},
		{tag: tagPlanarConfiguration, dataType: typeShort, count: 1, data: shortBytes(order, 1)},
		{tag: tagResolutionUnit, dataType: typeShort, count: 1, data: shortBytes(order, 2)},
		{tag: tagSampleFormat, dataType: typeShort, count: uint32(img.Channels), data: repeatedShort(order, uint16(img.Format), img.Channels)},
	}
	resolution := append(uint32Bytes(order, 72), uint32Bytes(order, 1)...)
	entries = append(entries,
		writeEntry{tag: tagXResolution, dataType: typeRational, count: 1, data: resolution},
		writeEntry{tag: tagYResolution, dataType: typeRational, count: 1, data: append([]byte(nil), resolution...)},
	)
	countType := uint16(typeLong)
	if big {
		countType = typeLong8
	}
	if page.tiled {
		entries = append(entries,
			writeEntry{tag: tagTileWidth, dataType: dimType, count: 1, data: dimData(page.tileW)},
			writeEntry{tag: tagTileLength, dataType: dimType, count: 1, data: dimData(page.tileH)},
			writeEntry{tag: tagTileOffsets, dataType: countType, count: uint32(len(page.counts)), data: make([]byte, len(page.counts)*typeSizeUnchecked(countType))},
			writeEntry{tag: tagTileByteCounts, dataType: countType, count: uint32(len(page.counts)), data: encodeCounts(order, big, page.counts)},
		)
	} else {
		entries = append(entries,
			writeEntry{tag: tagRowsPerStrip, dataType: dimType, count: 1, data: dimData(img.Height)},
			writeEntry{tag: tagStripOffsets, dataType: countType, count: 1, data: make([]byte, typeSizeUnchecked(countType))},
			writeEntry{tag: tagStripByteCounts, dataType: countType, count: 1, data: encodeCounts(order, big, page.counts)},
		)
	}
	if opt.Predictor {
		entries = append(entries, writeEntry{tag: tagPredictor, dataType: typeShort, count: 1, data: shortBytes(order, predictorHorizontal)})
	}
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
	if img.CFAPattern != "" {
		repeatX, repeatY := img.CFARepeatX, img.CFARepeatY
		if repeatX == 0 || repeatY == 0 {
			repeatX, repeatY = 2, 2
		}
		repeat := make([]byte, 4)
		order.PutUint16(repeat[0:2], uint16(repeatX))
		order.PutUint16(repeat[2:4], uint16(repeatY))
		entries = append(entries,
			writeEntry{tag: tagCFARepeatPatternDim, dataType: typeShort, count: 2, data: repeat},
			writeEntry{tag: tagCFAPattern, dataType: typeByte, count: uint32(len(img.CFAPattern)), data: []byte(img.CFAPattern)},
		)
	}
	return entries
}

func typeSizeUnchecked(t uint16) int {
	if t == typeLong8 {
		return 8
	}
	return 4
}

func encodeCounts(order binary.ByteOrder, big bool, values []uint64) []byte {
	width := 4
	if big {
		width = 8
	}
	out := make([]byte, len(values)*width)
	for i, v := range values {
		if big {
			order.PutUint64(out[i*8:], v)
		} else {
			order.PutUint32(out[i*4:], uint32(v))
		}
	}
	return out
}

func setBlockEntries(entries []writeEntry, offsets, counts []uint64, big bool, order binary.ByteOrder) {
	for i := range entries {
		switch entries[i].tag {
		case tagStripOffsets, tagTileOffsets:
			entries[i].data = encodeCounts(order, big, offsets)
		case tagStripByteCounts, tagTileByteCounts:
			entries[i].data = encodeCounts(order, big, counts)
		}
	}
}

func pointerAreaSize(entries []writeEntry, big bool) int {
	unit := 4
	if big {
		unit = 8
	}
	size := 0
	for _, e := range entries {
		if e.inline {
			continue
		}
		size += padLength(len(e.data), unit)
	}
	return size
}

func padLength(n, unit int) int {
	if rem := n % unit; rem != 0 {
		return n + unit - rem
	}
	return n
}

func assignPointers(entries []writeEntry, ifdOffset uint64, big bool) {
	countSize, entrySize, pointerWidth, nextSize := 2, 12, 4, 4
	if big {
		countSize, entrySize, pointerWidth, nextSize = 8, 20, 8, 8
	}
	pointerStart := ifdOffset + uint64(countSize+len(entries)*entrySize+nextSize)
	for i := range entries {
		if entries[i].inline {
			continue
		}
		entries[i].pointer = pointerStart
		pointerStart += uint64(padLength(len(entries[i].data), pointerWidth))
	}
}

func encodeIFD(entries []writeEntry, ifdOffset, next uint64, big bool, order binary.ByteOrder) []byte {
	countSize, entrySize, pointerWidth, nextSize := 2, 12, 4, 4
	if big {
		countSize, entrySize, pointerWidth, nextSize = 8, 20, 8, 8
	}
	buf := make([]byte, 0, countSize+len(entries)*entrySize+nextSize+pointerAreaSize(entries, big))
	if big {
		buf = append(buf, uint64Bytes(order, uint64(len(entries)))...)
	} else {
		buf = append(buf, shortBytes(order, uint16(len(entries)))...)
	}
	var pointer bytes.Buffer
	for _, e := range entries {
		buf = append(buf, shortBytes(order, e.tag)...)
		buf = append(buf, shortBytes(order, e.dataType)...)
		if big {
			buf = append(buf, uint64Bytes(order, uint64(e.count))...)
		} else {
			buf = append(buf, uint32Bytes(order, e.count)...)
		}
		if e.inline {
			value := make([]byte, pointerWidth)
			copy(value, e.data)
			buf = append(buf, value...)
		} else {
			if big {
				buf = append(buf, uint64Bytes(order, e.pointer)...)
			} else {
				buf = append(buf, uint32Bytes(order, uint32(e.pointer))...)
			}
			padded := append([]byte(nil), e.data...)
			if rem := len(padded) % pointerWidth; rem != 0 {
				padded = append(padded, make([]byte, pointerWidth-rem)...)
			}
			pointer.Write(padded)
		}
	}
	if big {
		buf = append(buf, uint64Bytes(order, next)...)
	} else {
		buf = append(buf, uint32Bytes(order, uint32(next))...)
	}
	buf = append(buf, pointer.Bytes()...)
	return buf
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

func uint64Bytes(order binary.ByteOrder, v uint64) []byte {
	b := make([]byte, 8)
	order.PutUint64(b, v)
	return b
}

func repeatedShort(order binary.ByteOrder, v uint16, n int) []byte {
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		order.PutUint16(out[i*2:i*2+2], v)
	}
	return out
}

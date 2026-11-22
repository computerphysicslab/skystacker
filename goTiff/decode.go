package gotiff

import (
	"encoding/binary"
	"fmt"
	"math"
)

func decodeBytes(data []byte) ([]*Image, error) {
	order, magic, ifdOffset, err := readHeader(data)
	if err != nil {
		return nil, err
	}
	big := magic == bigMagic
	var images []*Image
	for ifdOffset != 0 {
		entries, next, err := readIFD(data, order, ifdOffset, big)
		if err != nil {
			return nil, err
		}
		img, err := decodeIFD(data, order, entries)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
		ifdOffset = next
	}
	if len(images) == 0 {
		return nil, errBadIFD
	}
	return images, nil
}

func decodeIFD(data []byte, order binary.ByteOrder, entries map[uint16]ifdEntry) (*Image, error) {
	w64, ok := firstValue(entries, tagImageWidth)
	if !ok {
		return nil, errBadIFD
	}
	h64, ok := firstValue(entries, tagImageLength)
	if !ok {
		return nil, errBadIFD
	}
	w, h := int(w64), int(h64)
	if w <= 0 || h <= 0 || int64(w)*int64(h) > maxPixels {
		return nil, errBadIFD
	}
	photo64, _ := firstValue(entries, tagPhotometric)
	photo := Photometric(photo64)
	if photo != PhotoBlackIsZero && photo != PhotoWhiteIsZero && photo != PhotoRGB {
		return nil, UnsupportedError(fmt.Sprintf("photometric %d", photo64))
	}
	spp64, ok := firstValue(entries, tagSamplesPerPixel)
	if !ok {
		spp64 = 1
	}
	channels := int(spp64)
	wantChannels := 3
	if photo != PhotoRGB {
		wantChannels = 1
	}
	if channels != wantChannels {
		return nil, UnsupportedError(fmt.Sprintf("samples per pixel %d", channels))
	}
	planar, _ := firstValue(entries, tagPlanarConfiguration)
	if planar != 1 {
		return nil, UnsupportedError("planar configuration")
	}
	comp, _ := firstValue(entries, tagCompression)
	compression := Compression(comp)
	switch compression {
	case CompressionNone, CompressionLZW, CompressionDeflate, CompressionDeflateOld, CompressionPackBits, 0:
	default:
		return nil, UnsupportedError(fmt.Sprintf("compression %d", comp))
	}
	predictor, _ := firstValue(entries, tagPredictor)
	if predictor == 0 {
		predictor = predictorNone
	}
	if predictor != predictorNone && predictor != predictorHorizontal {
		return nil, UnsupportedError(fmt.Sprintf("predictor %d", predictor))
	}
	bitsVals, ok := values(entries, tagBitsPerSample)
	if !ok || len(bitsVals) != channels {
		return nil, errBadIFD
	}
	bits := int(bitsVals[0])
	for _, b := range bitsVals {
		if int(b) != bits {
			return nil, UnsupportedError("mixed BitsPerSample")
		}
	}
	formatVals, ok := values(entries, tagSampleFormat)
	if !ok {
		formatVals = make([]uint64, channels)
		for i := range formatVals {
			formatVals[i] = uint64(SampleUint)
		}
	}
	if len(formatVals) != channels {
		return nil, errBadIFD
	}
	format := SampleFormat(formatVals[0])
	for _, f := range formatVals {
		if SampleFormat(f) != format {
			return nil, UnsupportedError("mixed SampleFormat")
		}
	}
	bytesPerSample, err := sampleBytes(bits, format)
	if err != nil {
		return nil, err
	}
	img := &Image{
		Width:       w,
		Height:      h,
		Channels:    channels,
		Bits:        bits,
		Format:      format,
		Photometric: photo,
		Pixels:      make([]float32, w*h*channels),
		Metadata:    metadataFromIFD(entries),
	}
	if repeat, ok := values(entries, tagCFARepeatPatternDim); ok && len(repeat) >= 2 {
		img.CFARepeatX, img.CFARepeatY = int(repeat[0]), int(repeat[1])
	}
	if pattern, ok := values(entries, tagCFAPattern); ok {
		b := make([]byte, len(pattern))
		for i, v := range pattern {
			b[i] = byte(v)
		}
		img.CFAPattern = string(b)
	}
	bpp := channels * bytesPerSample
	tileW64, tileOKW := firstValue(entries, tagTileWidth)
	tileH64, tileOKH := firstValue(entries, tagTileLength)
	if tileOKW && tileOKH {
		tileW, tileH := int(tileW64), int(tileH64)
		if tileW < 8 || tileH < 8 || tileW > 8192 || tileH > 8192 {
			return nil, errBadIFD
		}
		offsets, ok := values(entries, tagTileOffsets)
		if !ok {
			return nil, errBadIFD
		}
		counts, ok := values(entries, tagTileByteCounts)
		if !ok {
			return nil, errBadIFD
		}
		across, down := (w+tileW-1)/tileW, (h+tileH-1)/tileH
		if len(offsets) < across*down || len(counts) < across*down {
			return nil, errBadIFD
		}
		blockRowBytes := int64(tileW) * int64(channels) * int64(bytesPerSample)
		need := blockRowBytes * int64(tileH)
		for ty := 0; ty < down; ty++ {
			for tx := 0; tx < across; tx++ {
				i := ty*across + tx
				off, cnt := int64(offsets[i]), int64(counts[i])
				if off < 0 || cnt <= 0 || off+cnt > int64(len(data)) {
					return nil, errNoPixels
				}
				block, err := decompressStrip(data[off:off+cnt], compression, need)
				if err != nil {
					return nil, err
				}
				if predictor == predictorHorizontal {
					if err := applyPredictor(block, int(blockRowBytes), bpp, false); err != nil {
						return nil, err
					}
				}
				x1, y1 := min((tx+1)*tileW, w), min((ty+1)*tileH, h)
				if err := decodeBlock(img, block, order, tx*tileW, ty*tileH, x1, y1, tileW, bpp, bytesPerSample, bits, format, photo); err != nil {
					return nil, err
				}
			}
		}
		return img, nil
	}
	rows64, ok := firstValue(entries, tagRowsPerStrip)
	if !ok || rows64 == 0 {
		rows64 = uint64(h)
	}
	rows := int(rows64)
	if rows > h {
		rows = h
	}
	offsets, ok := values(entries, tagStripOffsets)
	if !ok {
		return nil, errBadIFD
	}
	counts, ok := values(entries, tagStripByteCounts)
	if !ok {
		return nil, errBadIFD
	}
	strips := (h + rows - 1) / rows
	if len(offsets) < strips || len(counts) < strips {
		return nil, errBadIFD
	}
	rowBytes := int64(w) * int64(channels) * int64(bytesPerSample)
	for s := 0; s < strips; s++ {
		y0 := s * rows
		y1 := min(y0+rows, h)
		need := int64(y1-y0) * rowBytes
		off, cnt := int64(offsets[s]), int64(counts[s])
		if off < 0 || cnt <= 0 || off+cnt > int64(len(data)) {
			return nil, errNoPixels
		}
		strip, err := decompressStrip(data[off:off+cnt], compression, need)
		if err != nil {
			return nil, err
		}
		if predictor == predictorHorizontal {
			if err := applyPredictor(strip, int(rowBytes), bpp, false); err != nil {
				return nil, err
			}
		}
		if err := decodeBlock(img, strip, order, 0, y0, w, y1, w, bpp, bytesPerSample, bits, format, photo); err != nil {
			return nil, err
		}
	}
	return img, nil
}

func decodeBlock(img *Image, block []byte, order binary.ByteOrder, x0, y0, x1, y1, blockW, bpp, bytesPerSample, bits int, format SampleFormat, photo Photometric) error {
	blockRowBytes := int64(blockW) * int64(img.Channels) * int64(bytesPerSample)
	for y := y0; y < y1; y++ {
		row := block[int64(y-y0)*blockRowBytes : int64(y-y0+1)*blockRowBytes]
		for x := x0; x < x1; x++ {
			for c := 0; c < img.Channels; c++ {
				pos := (x-x0)*img.Channels*bytesPerSample + c*bytesPerSample
				v, err := readSample(row[pos:pos+bytesPerSample], order, bits, format)
				if err != nil {
					return err
				}
				if photo == PhotoWhiteIsZero && format != SampleFloat {
					v = 1 - v
				}
				img.Pixels[(y*img.Width+x)*img.Channels+c] = float32(v)
			}
		}
	}
	return nil
}

func readSample(p []byte, order binary.ByteOrder, bits int, format SampleFormat) (float64, error) {
	switch format {
	case SampleUint:
		switch bits {
		case 8:
			return float64(p[0]) / 255, nil
		case 16:
			return float64(order.Uint16(p)) / 65535, nil
		case 32:
			return float64(order.Uint32(p)) / math.MaxUint32, nil
		case 64:
			return float64(order.Uint64(p)) / math.MaxUint64, nil
		}
	case SampleFloat:
		switch bits {
		case 32:
			return float64(math.Float32frombits(order.Uint32(p))), nil
		case 64:
			return math.Float64frombits(order.Uint64(p)), nil
		}
	case SampleInt:
		switch bits {
		case 8:
			return float64(int8(p[0])), nil
		case 16:
			return float64(int16(order.Uint16(p))), nil
		case 32:
			return float64(int32(order.Uint32(p))), nil
		case 64:
			return float64(int64(order.Uint64(p))), nil
		}
	}
	return 0, UnsupportedError(fmt.Sprintf("%d-bit sample format %d", bits, format))
}

func sampleBytes(bits int, format SampleFormat) (int, error) {
	if bits <= 0 || bits%8 != 0 || bits > 64 {
		return 0, UnsupportedError(fmt.Sprintf("BitsPerSample %d", bits))
	}
	switch format {
	case SampleUint, SampleInt, SampleFloat:
		return bits / 8, nil
	default:
		return 0, UnsupportedError(fmt.Sprintf("SampleFormat %d", format))
	}
}

package gotiff

import (
	"encoding/binary"
	"fmt"
	"math"
)

func decodeBytes(data []byte) (*Image, error) {
	order, ifdOffset, err := readHeader(data)
	if err != nil {
		return nil, err
	}
	entries, err := readIFD(data, order, ifdOffset)
	if err != nil {
		return nil, err
	}
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
	if comp != uint64(CompressionNone) {
		return nil, UnsupportedError(fmt.Sprintf("compression %d", comp))
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
	rowBytes := int64(w) * int64(channels) * int64(bytesPerSample)
	for s := 0; s < strips; s++ {
		y0 := s * rows
		y1 := min(y0+rows, h)
		need := int64(y1-y0) * rowBytes
		off := int64(offsets[s])
		cnt := int64(counts[s])
		if off < 0 || cnt < need || off+cnt > int64(len(data)) {
			return nil, errNoPixels
		}
		strip := data[off : off+need]
		for y := y0; y < y1; y++ {
			row := strip[int64(y-y0)*rowBytes : int64(y-y0+1)*rowBytes]
			for x := 0; x < w; x++ {
				for c := 0; c < channels; c++ {
					pos := (x*channels + c) * bytesPerSample
					v, err := readSample(row[pos:pos+bytesPerSample], order, bits, format)
					if err != nil {
						return nil, err
					}
					if photo == PhotoWhiteIsZero && format != SampleFloat {
						v = 1 - v
					}
					img.Pixels[(y*w+x)*channels+c] = float32(v)
				}
			}
		}
	}
	return img, nil
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package output

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// ReadFloat32RGB reads a TIFF produced by WriteFloat32RGB. It supports only
// uncompressed, little-endian, interleaved RGB float32 TIFFs. It exists to
// support round-trip validation and diagnostics; it is not a general TIFF
// decoder.
func ReadFloat32RGB(r io.Reader) (width, height int, pix []float32, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(data) < 8 {
		return 0, 0, nil, errors.New("tiff: file too short")
	}
	if data[0] != 'I' || data[1] != 'I' {
		return 0, 0, nil, errors.New("tiff: not a little-endian TIFF")
	}
	if v := binary.LittleEndian.Uint16(data[2:]); v != 42 {
		return 0, 0, nil, fmt.Errorf("tiff: unexpected version %d", v)
	}
	ifdOff := int(binary.LittleEndian.Uint32(data[4:]))
	if ifdOff < 0 || ifdOff+2 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD offset out of range")
	}
	nEntries := int(binary.LittleEndian.Uint16(data[ifdOff:]))
	if ifdOff+2+nEntries*12+4 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD truncated")
	}

	var (
		w, h     uint32
		bps      []uint16
		sf       []uint16
		spp      uint16
		photo    uint16
		planar   uint16
		stripOff []uint32
		stripLen []uint32
	)

	for i := 0; i < nEntries; i++ {
		off := ifdOff + 2 + i*12
		tag := binary.LittleEndian.Uint16(data[off:])
		typ := binary.LittleEndian.Uint16(data[off+2:])
		count := int(binary.LittleEndian.Uint32(data[off+4:]))
		valField := off + 8

		readShorts := func() ([]uint16, error) {
			if typ != tiffTypeShort {
				return nil, fmt.Errorf("tiff: tag %d: expected SHORT, got type %d", tag, typ)
			}
			out := make([]uint16, count)
			if count*2 <= 4 {
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[valField+k*2:])
				}
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o < 0 || o+count*2 > len(data) {
					return nil, fmt.Errorf("tiff: tag %d: value out of range", tag)
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[o+k*2:])
				}
			}
			return out, nil
		}
		readLongs := func() ([]uint32, error) {
			if typ != tiffTypeLong {
				return nil, fmt.Errorf("tiff: tag %d: expected LONG, got type %d", tag, typ)
			}
			out := make([]uint32, count)
			if count*4 <= 4 {
				out[0] = binary.LittleEndian.Uint32(data[valField:])
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o < 0 || o+count*4 > len(data) {
					return nil, fmt.Errorf("tiff: tag %d: value out of range", tag)
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint32(data[o+k*4:])
				}
			}
			return out, nil
		}

		switch tag {
		case tiffTagImageWidth:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			w = v[0]
		case tiffTagImageLength:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			h = v[0]
		case tiffTagBitsPerSample:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			bps = v
		case tiffTagPhotometricInterp:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			photo = v[0]
		case tiffTagStripOffsets:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripOff = v
		case tiffTagSamplesPerPixel:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			spp = v[0]
		case tiffTagStripByteCounts:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripLen = v
		case tiffTagPlanarConfig:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			planar = v[0]
		case tiffTagSampleFormat:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			sf = v
		}
	}

	if w == 0 || h == 0 {
		return 0, 0, nil, errors.New("tiff: missing dimensions")
	}
	if spp != 3 {
		return 0, 0, nil, fmt.Errorf("tiff: expected 3 samples per pixel, got %d", spp)
	}
	if photo != tiffPhotometricRGB {
		return 0, 0, nil, fmt.Errorf("tiff: expected RGB photometric, got %d", photo)
	}
	if planar != tiffPlanarChunky {
		return 0, 0, nil, fmt.Errorf("tiff: expected chunky planar config, got %d", planar)
	}
	if len(bps) != 3 || bps[0] != 32 || bps[1] != 32 || bps[2] != 32 {
		return 0, 0, nil, fmt.Errorf("tiff: expected 32-bit samples, got %v", bps)
	}
	if len(sf) != 3 || sf[0] != tiffSampleFormatFloat || sf[1] != tiffSampleFormatFloat || sf[2] != tiffSampleFormatFloat {
		return 0, 0, nil, fmt.Errorf("tiff: expected IEEE float sample format, got %v", sf)
	}
	if len(stripOff) != 1 || len(stripLen) != 1 {
		return 0, 0, nil, errors.New("tiff: expected a single strip")
	}
	nPix := int(w) * int(h) * 3
	if int(stripLen[0]) != nPix*4 {
		return 0, 0, nil, fmt.Errorf("tiff: strip byte count %d, expected %d", stripLen[0], nPix*4)
	}
	off := int(stripOff[0])
	if off < 0 || off+nPix*4 > len(data) {
		return 0, 0, nil, errors.New("tiff: pixel data out of range")
	}
	pix = make([]float32, nPix)
	for i := range pix {
		pix[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*4:]))
	}
	return int(w), int(h), pix, nil
}

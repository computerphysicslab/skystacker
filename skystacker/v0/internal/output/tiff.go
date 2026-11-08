package output

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// TIFF tag identifiers used by this writer.
const (
	tiffTagImageWidth        = 256
	tiffTagImageLength       = 257
	tiffTagBitsPerSample     = 258
	tiffTagCompression       = 259
	tiffTagPhotometricInterp = 262
	tiffTagStripOffsets      = 273
	tiffTagSamplesPerPixel   = 277
	tiffTagRowsPerStrip      = 278
	tiffTagStripByteCounts   = 279
	tiffTagXResolution       = 282
	tiffTagYResolution       = 283
	tiffTagPlanarConfig      = 284
	tiffTagResolutionUnit    = 296
	tiffTagSampleFormat      = 339
)

// TIFF field types.
const (
	tiffTypeShort    = 3
	tiffTypeLong     = 4
	tiffTypeRational = 5
)

// TIFF field values.
const (
	tiffCompressionNone    = 1
	tiffPhotometricRGB     = 2
	tiffPlanarChunky       = 1
	tiffSampleFormatFloat  = 3
	tiffResolutionUnitNone = 1
)

// The number of IFD entries we always emit, including Tags for optional
// fields like XResolution/YResolution which we always populate.
const tiffIFDEntryCount = 14

// WriteFloat32RGB writes an uncompressed, little-endian, RGB float32 TIFF.
//
// pix must contain Width*Height*3 interleaved RGB samples. The output uses a
// single strip, chunky (interleaved) sample layout and IEEE float32 samples,
// so no precision is lost relative to the in-memory accumulator. Values are
// not scaled or clipped.
func WriteFloat32RGB(w io.Writer, width, height int, pix []float32) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("tiff: invalid dimensions %dx%d", width, height)
	}
	n := width * height * 3
	if len(pix) != n {
		return fmt.Errorf("tiff: pixel buffer length %d does not match %dx%dx3 = %d",
			len(pix), width, height, n)
	}
	stripBytes64 := int64(n) * 4
	if stripBytes64 > math.MaxUint32 {
		return fmt.Errorf("tiff: image too large for a single strip (%d bytes)", stripBytes64)
	}
	stripBytes := uint32(stripBytes64)

	ifdSize := uint32(2 + tiffIFDEntryCount*12 + 4)
	ifdOffset := uint32(8)
	auxOffset := ifdOffset + ifdSize

	bitsOffset := auxOffset
	sampleFormatOffset := bitsOffset + 6
	xResOffset := sampleFormatOffset + 6
	yResOffset := xResOffset + 8
	auxEnd := yResOffset + 8

	pixelOffset := auxEnd
	if rem := pixelOffset % 4; rem != 0 {
		pixelOffset += 4 - rem
	}

	// Header
	var header [8]byte
	header[0], header[1] = 'I', 'I'
	binary.LittleEndian.PutUint16(header[2:], 42)
	binary.LittleEndian.PutUint32(header[4:], ifdOffset)
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	// IFD
	ifd := make([]byte, ifdSize)
	binary.LittleEndian.PutUint16(ifd[0:], tiffIFDEntryCount)
	pos := 2

	putShort := func(tag, v uint16) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], tiffTypeShort)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint16(ifd[pos+8:], v)
		pos += 12
	}
	putLong := func(tag uint16, v uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], tiffTypeLong)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint32(ifd[pos+8:], v)
		pos += 12
	}
	putOffset := func(tag, typ uint16, count, offset uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typ)
		binary.LittleEndian.PutUint32(ifd[pos+4:], count)
		binary.LittleEndian.PutUint32(ifd[pos+8:], offset)
		pos += 12
	}

	putLong(tiffTagImageWidth, uint32(width))
	putLong(tiffTagImageLength, uint32(height))
	putOffset(tiffTagBitsPerSample, tiffTypeShort, 3, bitsOffset)
	putShort(tiffTagCompression, tiffCompressionNone)
	putShort(tiffTagPhotometricInterp, tiffPhotometricRGB)
	putLong(tiffTagStripOffsets, pixelOffset)
	putShort(tiffTagSamplesPerPixel, 3)
	putLong(tiffTagRowsPerStrip, uint32(height))
	putLong(tiffTagStripByteCounts, stripBytes)
	putOffset(tiffTagXResolution, tiffTypeRational, 1, xResOffset)
	putOffset(tiffTagYResolution, tiffTypeRational, 1, yResOffset)
	putShort(tiffTagPlanarConfig, tiffPlanarChunky)
	putShort(tiffTagResolutionUnit, tiffResolutionUnitNone)
	putOffset(tiffTagSampleFormat, tiffTypeShort, 3, sampleFormatOffset)

	// Next IFD = 0
	binary.LittleEndian.PutUint32(ifd[pos:], 0)
	pos += 4

	if _, err := w.Write(ifd); err != nil {
		return err
	}

	// Auxiliary data: BitsPerSample, SampleFormat, XResolution, YResolution.
	aux := make([]byte, int(auxEnd-auxOffset))
	bo := int(bitsOffset - auxOffset)
	sfo := int(sampleFormatOffset - auxOffset)
	xro := int(xResOffset - auxOffset)
	yro := int(yResOffset - auxOffset)
	binary.LittleEndian.PutUint16(aux[bo:], 32)
	binary.LittleEndian.PutUint16(aux[bo+2:], 32)
	binary.LittleEndian.PutUint16(aux[bo+4:], 32)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint16(aux[sfo+i*2:], tiffSampleFormatFloat)
	}
	binary.LittleEndian.PutUint32(aux[xro:], 1)
	binary.LittleEndian.PutUint32(aux[xro+4:], 1)
	binary.LittleEndian.PutUint32(aux[yro:], 1)
	binary.LittleEndian.PutUint32(aux[yro+4:], 1)
	if _, err := w.Write(aux); err != nil {
		return err
	}

	if pad := int(pixelOffset) - int(auxEnd); pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}

	// Pixel data
	const bufFloats = 4096
	scratch := make([]byte, bufFloats*4)
	for i := 0; i < n; {
		count := n - i
		if count > bufFloats {
			count = bufFloats
		}
		for j := 0; j < count; j++ {
			binary.LittleEndian.PutUint32(scratch[j*4:], math.Float32bits(pix[i+j]))
		}
		if _, err := w.Write(scratch[:count*4]); err != nil {
			return err
		}
		i += count
	}
	return nil
}

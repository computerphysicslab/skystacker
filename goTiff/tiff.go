package gotiff

import (
	"encoding/binary"
	"io"
)

type SampleFormat uint16

const (
	SampleUint  SampleFormat = 1
	SampleInt   SampleFormat = 2
	SampleFloat SampleFormat = 3
)

type Photometric uint16

const (
	PhotoWhiteIsZero Photometric = 0
	PhotoBlackIsZero Photometric = 1
	PhotoRGB         Photometric = 2
)

type Compression uint16

const (
	CompressionNone Compression = 1
)

type FormatError string

func (e FormatError) Error() string { return "gotiff: invalid format: " + string(e) }

type UnsupportedError string

func (e UnsupportedError) Error() string { return "gotiff: unsupported feature: " + string(e) }

const (
	tagImageDescription    = 270
	tagImageWidth          = 256
	tagImageLength         = 257
	tagBitsPerSample       = 258
	tagCompression         = 259
	tagPhotometric         = 262
	tagStripOffsets        = 273
	tagOrientation         = 274
	tagSamplesPerPixel     = 277
	tagRowsPerStrip        = 278
	tagStripByteCounts     = 279
	tagXResolution         = 282
	tagYResolution         = 283
	tagPlanarConfiguration = 284
	tagResolutionUnit      = 296
	tagSoftware            = 305
	tagDateTime            = 306
	tagArtist              = 315
	tagSampleFormat        = 339

	typeByte      = 1
	typeASCII     = 2
	typeShort     = 3
	typeLong      = 4
	typeRational  = 5
	typeUndefined = 7

	classicMagic = 42
)

var (
	errInvalidHeader = FormatError("bad TIFF header")
	errBadIFD        = FormatError("bad IFD")
	errNoPixels      = FormatError("not enough pixel data")
)

type Image struct {
	Width       int
	Height      int
	Channels    int
	Bits        int
	Format      SampleFormat
	Photometric Photometric
	Pixels      []float32
	Metadata    map[string]string
}

type Options struct {
	ByteOrder   binary.ByteOrder
	Compression Compression
	Metadata    map[string]string
}

func Decode(r io.Reader) (*Image, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxReadSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxReadSize {
		return nil, FormatError("file too large")
	}
	return decodeBytes(data)
}

func Encode(w io.Writer, img *Image, opt *Options) error {
	if opt == nil {
		opt = &Options{}
	}
	data, err := encodeBytes(img, opt)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

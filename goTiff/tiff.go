package gotiff

import (
	"encoding/binary"
	"image"
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
	CompressionNone       Compression = 1
	CompressionLZW        Compression = 5
	CompressionDeflate    Compression = 8
	CompressionPackBits   Compression = 32773
	CompressionDeflateOld Compression = 32946
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
	tagPredictor           = 317
	tagCFARepeatPatternDim = 33421
	tagCFAPattern          = 33422
	tagTileWidth           = 322
	tagTileLength          = 323
	tagTileOffsets         = 324
	tagTileByteCounts      = 325

	typeByte      = 1
	typeASCII     = 2
	typeShort     = 3
	typeLong      = 4
	typeRational  = 5
	typeUndefined = 7
	typeLong8     = 16
	typeSLong8    = 17
	typeIFD8      = 18

	classicMagic = 42
	bigMagic     = 43
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
	CFAPattern  string
	CFARepeatX  int
	CFARepeatY  int
}

type Options struct {
	ByteOrder   binary.ByteOrder
	Compression Compression
	Predictor   bool
	BigTIFF     bool
	Tiled       bool
	TileWidth   int
	TileHeight  int
	Metadata    map[string]string
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxReadSize+1))
	if err != nil {
		return image.Config{}, err
	}
	order, magic, offset, err := readHeader(data)
	if err != nil {
		return image.Config{}, err
	}
	entries, _, err := readIFD(data, order, offset, magic == bigMagic)
	if err != nil {
		return image.Config{}, err
	}
	w, okW := firstValue(entries, tagImageWidth)
	h, okH := firstValue(entries, tagImageLength)
	if !okW || !okH || w == 0 || h == 0 {
		return image.Config{}, errBadIFD
	}
	return image.Config{Width: int(w), Height: int(h)}, nil
}

func Decode(r io.Reader) (*Image, error) {
	images, err := DecodeAll(r)
	if err != nil {
		return nil, err
	}
	return images[0], nil
}

func DecodeAll(r io.Reader) ([]*Image, error) {
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
	return EncodeAll(w, []*Image{img}, opt)
}

func EncodeAll(w io.Writer, images []*Image, opt *Options) error {
	if opt == nil {
		opt = &Options{}
	}
	data, err := encodeAllBytes(images, opt)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

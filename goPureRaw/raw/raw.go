// Package raw defines the core types and interfaces for pure-Go decoding
// of camera RAW files.
package raw

import "io"

// Format identifies a camera RAW file format.
type Format uint32

const (
	FormatUnknown Format = iota
	FormatRAF            // Fujifilm RAF
	FormatARW            // Sony ARW
	FormatCR3            // Canon CR3
	FormatCR2            // Canon CR2
)

func (f Format) String() string {
	switch f {
	case FormatRAF:
		return "RAF"
	case FormatARW:
		return "ARW"
	case FormatCR3:
		return "CR3"
	case FormatCR2:
		return "CR2"
	default:
		return "unknown"
	}
}

// Decoder decodes a single RAW format.
//
// Implementations must be safe for concurrent use. Probe must not read
// beyond the first 4 KiB of r and must not panic on malformed input.
type Decoder interface {
	// Format returns the format this decoder handles.
	Format() Format

	// Probe reports whether r begins with this decoder's format
	// signature. It is a cheap, side-effect-free check.
	Probe(r io.ReaderAt) bool

	// Decode parses the full file. size is the total file size in
	// bytes. On success the returned Image is fully populated with
	// sensor data and metadata.
	Decode(r io.ReaderAt, size int64) (*Image, error)
}

// Image is a decoded RAW image in its native sensor layout.
type Image struct {
	Width    int
	Height   int
	BitDepth int // valid sensor bits per sample (10, 12, 14, 16)

	// SensorData holds Width*Height samples in row-major order. Each
	// sample is one CFA site, scaled so the value fits in BitDepth
	// bits. The low BitDepth bits are meaningful; the rest are zero.
	SensorData []uint16

	// CFAPattern is the 2x2 Bayer layout. For monochrome or exotic
	// sensors this may be a zero value; check BitDepth > 0 and
	// Metadata.Vendor["cfa"] before assuming Bayer.
	CFAPattern CFAPattern

	// BlackLevel is per-CFA-position: {top-left, top-right,
	// bottom-left, bottom-right}.
	BlackLevel [4]float64
	WhiteLevel float64

	// ColorMatrix maps linear camera RGB to CIE XYZ (D65).
	ColorMatrix [3][3]float64

	Metadata Metadata
}

// CFAPattern is a 2x2 Bayer layout, e.g. {'R','G','G','B'} for RGGB.
type CFAPattern [2][2]byte

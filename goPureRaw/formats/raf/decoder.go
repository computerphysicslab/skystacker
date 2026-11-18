// Package raf implements a pure-Go decoder for Fujifilm RAF files.
//
// An RAF file contains a custom header, a TIFF block with EXIF/IFD
// data, and a raw sensor data block whose format is described by a
// 16-byte "IS" header. Sensor data is stored in 768-pixel stripes,
// each entropy-coded with adaptive Rice coding.
//
// See docs/formats/raf.md for the detailed specification.
//
// Status: pre-alpha. Only Probe is implemented.
package raf

import (
	"bytes"
	"io"

	"github.com/computerphysicslab/skystacker/goPureRaw/raw"
)

const magic = "FUJIFILMCCD-RAW "

// Decoder decodes Fujifilm RAF files.
type Decoder struct{}

// Format implements raw.Decoder.
func (Decoder) Format() raw.Format { return raw.FormatRAF }

// Probe implements raw.Decoder.
func (Decoder) Probe(r io.ReaderAt) bool {
	buf := make([]byte, len(magic))
	if _, err := r.ReadAt(buf, 0); err != nil {
		return false
	}
	return bytes.Equal(buf, []byte(magic))
}

// Decode implements raw.Decoder.
func (Decoder) Decode(r io.ReaderAt, size int64) (*raw.Image, error) {
	// TODO: parse header, TIFF IFD, IS header, and decompress stripes.
	return nil, raw.ErrUnsupportedVariant
}

func init() { raw.Register(Decoder{}) }

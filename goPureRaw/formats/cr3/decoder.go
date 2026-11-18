// Package cr3 implements a pure-Go decoder for Canon CR3 files.
//
// CR3 is an ISOBMFF (MP4-style) container. Raw sensor data lives in a
// track compressed with Canon's proprietary CRX codec. Metadata is
// split between standard ISOBMFF boxes and Canon-specific "CTMD" maker
// note boxes.
//
// See docs/formats/cr3.md for the detailed specification.
//
// Status: pre-alpha. Only Probe is implemented.
package cr3

import (
	"bytes"
	"io"

	"github.com/computerphysicslab/skystacker/goPureRaw/raw"
)

// Decoder decodes Canon CR3 files.
type Decoder struct{}

// Format implements raw.Decoder.
func (Decoder) Format() raw.Format { return raw.FormatCR3 }

// Probe implements raw.Decoder.
func (Decoder) Probe(r io.ReaderAt) bool {
	// ISOBMFF: [size:4][ftyp][major_brand:4]. CR3 uses "crx ".
	buf := make([]byte, 12)
	if _, err := r.ReadAt(buf, 0); err != nil {
		return false
	}
	return bytes.Equal(buf[4:8], []byte("ftyp")) &&
		bytes.Equal(buf[8:12], []byte("crx "))
}

// Decode implements raw.Decoder.
func (Decoder) Decode(r io.ReaderAt, size int64) (*raw.Image, error) {
	// TODO: walk ISOBMFF boxes, locate CRX Bayer track, decode CRX.
	return nil, raw.ErrUnsupportedVariant
}

func init() { raw.Register(Decoder{}) }

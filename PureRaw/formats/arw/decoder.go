// Package arw implements a pure-Go decoder for Sony ARW files.
//
// ARW is a TIFF-based container. Modern models compress sensor data
// using one of several schemes; ARW6 uses SMPTE RDD 34 (LLVC), a
// low-latency video codec with separate green_lo, green_hi, chroma_r,
// and chroma_b tracks.
//
// See docs/formats/arw.md for the detailed specification.
//
// Status: pre-alpha. Only Probe is implemented.
package arw

import (
	"bytes"
	"io"

	"github.com/youruser/pureraw/raw"
)

// Decoder decodes Sony ARW files.
type Decoder struct{}

// Format implements raw.Decoder.
func (Decoder) Format() raw.Format { return raw.FormatARW }

// Probe implements raw.Decoder.
func (Decoder) Probe(r io.ReaderAt) bool {
	header := make([]byte, 4)
	if _, err := r.ReadAt(header, 0); err != nil {
		return false
	}
	isTIFF := bytes.Equal(header, []byte{'I', 'I', 0x2A, 0x00}) ||
		bytes.Equal(header, []byte{'M', 'M', 0x00, 0x2A})
	if !isTIFF {
		return false
	}
	// ARW files carry "SONY" in the TIFF header/maker area.
	tail := make([]byte, 64*1024)
	n, _ := r.ReadAt(tail, 0)
	return bytes.Contains(tail[:n], []byte("SONY"))
}

// Decode implements raw.Decoder.
func (Decoder) Decode(r io.ReaderAt, size int64) (*raw.Image, error) {
	// TODO: parse TIFF IFDs, locate raw image track, decode LLVC.
	return nil, raw.ErrUnsupportedVariant
}

func init() { raw.Register(Decoder{}) }

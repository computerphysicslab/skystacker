package raw

import (
	"io"
	"sync"
)

var (
	mu       sync.RWMutex
	decoders []Decoder
)

// Register adds a decoder to the global registry. It is intended to be
// called from an init function in each format package. Registering a
// second decoder for the same Format panics.
func Register(d Decoder) {
	mu.Lock()
	defer mu.Unlock()
	for _, existing := range decoders {
		if existing.Format() == d.Format() {
			panic("raw: duplicate decoder for format " + d.Format().String())
		}
	}
	decoders = append(decoders, d)
}

// Detect returns the first registered decoder whose Probe matches r.
// It returns ErrUnsupportedFormat if none match.
func Detect(r io.ReaderAt) (Decoder, error) {
	mu.RLock()
	defer mu.RUnlock()
	for _, d := range decoders {
		if d.Probe(r) {
			return d, nil
		}
	}
	return nil, ErrUnsupportedFormat
}

// Decode detects the format and decodes the file in one call.
func Decode(r io.ReaderAt, size int64) (*Image, error) {
	d, err := Detect(r)
	if err != nil {
		return nil, err
	}
	return d.Decode(r, size)
}

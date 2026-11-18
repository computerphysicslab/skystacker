package raw

import "errors"

var (
	// ErrUnsupportedFormat is returned by Detect when no registered
	// decoder's Probe matches.
	ErrUnsupportedFormat = errors.New("raw: unsupported format")

	// ErrCorrupt indicates that the file matched a format signature
	// but its contents are malformed or truncated.
	ErrCorrupt = errors.New("raw: corrupt file")

	// ErrUnsupportedVariant indicates a valid file of a known format
	// that uses a feature the decoder does not yet implement (e.g. a
	// new compression mode).
	ErrUnsupportedVariant = errors.New("raw: unsupported variant")
)

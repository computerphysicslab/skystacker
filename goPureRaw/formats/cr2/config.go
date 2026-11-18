package cr2

import (
	"errors"
	"fmt"
	"io"
)

// Config returns the sensor geometry of a CR2 lossless-JPEG stream without
// decoding its samples. Width is the full Bayer width (SOF3 width x 2).
func Config(r io.ReaderAt, size int64) (w, h, bits int, err error) {
	stream, err := findLosslessJPEG(r, size)
	if err != nil {
		return 0, 0, 0, err
	}
	i := indexSOF3(stream)
	if i < 0 || i+10 > len(stream) {
		return 0, 0, 0, errors.New("cr2: SOF3 not found")
	}
	p := stream[i+4 : i+10] // precision, height(2), width(2), ncomp
	bits = int(p[0])
	h = int(p[1])<<8 | int(p[2])
	w = (int(p[3])<<8 | int(p[4])) * 2
	if w <= 0 || h <= 0 || bits < 8 || bits > 16 {
		return 0, 0, 0, fmt.Errorf("cr2: invalid SOF3 geometry %dx%d @ %d bits", w, h, bits)
	}
	return w, h, bits, nil
}

func indexSOF3(b []byte) int {
	for i := 0; i+2 < len(b); i++ {
		if b[i] == 0xFF && b[i+1] == 0xC3 {
			return i
		}
	}
	return -1
}

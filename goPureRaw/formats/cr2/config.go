package cr2

import (
	"errors"
	"fmt"
	"io"

	"github.com/computerphysicslab/skystacker/goPureRaw/internal/ljpeg"
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
	sofw := int(p[3])<<8 | int(p[4])
	ncomp := int(p[5])
	if err := validateBayerGeometry(sofw, h, bits, ncomp); err != nil {
		return 0, 0, 0, fmt.Errorf("cr2: %w", err)
	}
	return sofw * 2, h, bits, nil
}

// validateBayerGeometry applies the shared ljpeg limits to an SOF3 header and
// also caps the interleaved Bayer raster. It is used before Config returns
// dimensions, before findLosslessJPEG reads the stream tail and before Decode
// allocates the output sample slice.
func validateBayerGeometry(sofWidth, height, precision, ncomp int) error {
	if err := ljpeg.ValidateGeometry(sofWidth, height, precision, ncomp); err != nil {
		return err
	}
	pixels := int64(sofWidth) * 2 * int64(height)
	if pixels > ljpeg.MaxPixels {
		return fmt.Errorf("%w: Bayer raster %dx%d exceeds %d pixels",
			ljpeg.ErrTooLarge, sofWidth*2, height, ljpeg.MaxPixels)
	}
	return nil
}

func indexSOF3(b []byte) int {
	for i := 0; i+2 < len(b); i++ {
		if b[i] == 0xFF && b[i+1] == 0xC3 {
			return i
		}
	}
	return -1
}

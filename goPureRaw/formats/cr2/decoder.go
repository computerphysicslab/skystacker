// Package cr2 implements a pure-Go decoder for Canon CR2 files.
//
// Status: container/metadata support plus lossless-JPEG sensor decoding for
// cameras that store raw data as SOF3 streams (e.g. EOS 40D). Geometric
// validation against dcraw is still preliminary; see cr2_test.go.
package cr2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/computerphysicslab/skystacker/goPureRaw/internal/ljpeg"
	"github.com/computerphysicslab/skystacker/goPureRaw/internal/metadata"
	"github.com/computerphysicslab/skystacker/goPureRaw/internal/tiff"
	"github.com/computerphysicslab/skystacker/goPureRaw/raw"
)

// Decoder decodes Canon CR2 files.
type Decoder struct{}

func (Decoder) Format() raw.Format { return raw.FormatCR2 }

// Probe implements raw.Decoder. A CR2 starts with "II*\0", the IFD0 offset
// at bytes 4..7, then "CR" and a version byte at offsets 8..11.
func (Decoder) Probe(r io.ReaderAt) bool {
	h := make([]byte, 10)
	if _, err := r.ReadAt(h, 0); err != nil {
		return false
	}
	return bytes.Equal(h[:4], []byte{'I', 'I', 0x2A, 0x00}) &&
		h[8] == 'C' && h[9] == 'R'
}

// ReadMetadata parses standard TIFF/Exif metadata from the CR2 header.
func ReadMetadata(r io.ReaderAt) (raw.Metadata, error) {
	doc, err := tiff.Read(r)
	if err != nil {
		return raw.Metadata{}, err
	}
	return metadata.FromTIFF(doc)
}

// findLosslessJPEG scans a ReaderAt for an SOI marker followed by a DHT or
// SOF3 marker, which identifies the CR2 raw lossless-JPEG stream.
func findLosslessJPEG(r io.ReaderAt, size int64) ([]byte, error) {
	const chunk = 1 << 20
	buf := make([]byte, chunk+2)
	var carry int
	for off := int64(0); off < size; off += chunk - 2 {
		n, err := r.ReadAt(buf[carry:carry+chunk], off)
		if n <= 0 {
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}
			break
		}
		data := buf[:carry+n]
		if os.Getenv("CR2_DEBUG") != "" {
			fmt.Printf("off=%d n=%d carry=%d first=%x\\n", off, n, carry, data[:4])
		}
		for i := 0; i+4 <= len(data); i++ {
			// Lossless streams carry SOF3 (FFC3); thumbnail JPEGs use SOF0.
			if data[i] == 0xFF && data[i+1] == 0xC3 {
				soi := -1
				for j := i; j >= 0 && j >= i-4096; j-- {
					if data[j] == 0xFF && j+1 < len(data) && data[j+1] == 0xD8 {
						soi = j
						break
					}
				}
				if soi < 0 {
					if os.Getenv("CR2_DEBUG") != "" {
						fmt.Printf("FFC3 at %d but no SOI nearby\\n", off+int64(i))
					}
					continue
				}
				start := off + int64(soi) - int64(carry)
				// Read only enough to identify a genuine SOF3 raw stream before
				// allocating the whole tail. Thumbnail SOF0 markers and stray
				// FFC3 byte pairs must not trigger a full-file read.
				probeLen := int64(4096)
				if remain := size - start; remain < probeLen {
					probeLen = remain
				}
				if probeLen <= 0 {
					continue
				}
				probe := make([]byte, probeLen)
				if _, err := r.ReadAt(probe, start); err != nil && !errors.Is(err, io.EOF) {
					return nil, err
				}
				idx := indexSOF3(probe)
				if idx < 0 || idx+10 > len(probe) {
					continue
				}
				p := probe[idx+4 : idx+10]
				precision, height := int(p[0]), int(p[1])<<8|int(p[2])
				width := int(p[3])<<8 | int(p[4])
				components := int(p[5])
				if precision < 8 || precision > 16 || height <= 0 || width <= 0 || components < 1 || components > 4 {
					continue
				}
				rest := make([]byte, size-start)
				if _, err := r.ReadAt(rest, start); err != nil && !errors.Is(err, io.EOF) {
					return nil, err
				}
				return rest, nil
			}
		}
		carry = 2
		copy(buf[:2], data[len(data)-2:])
	}
	return nil, errors.New("cr2: lossless JPEG stream not found")
}

// Decode implements raw.Decoder for lossless-JPEG CR2 files.
func (Decoder) Decode(r io.ReaderAt, size int64) (*raw.Image, error) {
	stream, err := findLosslessJPEG(r, size)
	if err != nil {
		return nil, err
	}
	lj, err := ljpeg.Decode(stream)
	if err != nil {
		return nil, fmt.Errorf("cr2: lossless JPEG: %w", err)
	}
	if lj.CompCount != 2 {
		return nil, fmt.Errorf("%w: %d components", raw.ErrUnsupportedVariant, lj.CompCount)
	}
	w, h := lj.Width*2, lj.Height
	pix := make([]uint16, w*h)
	for y := range h {
		for x := 0; x < lj.Width; x++ {
			p := y*lj.Width + x
			pix[y*w+2*x] = lj.Planes[0][p]
			pix[y*w+2*x+1] = lj.Planes[1][p]
		}
	}
	meta, _ := ReadMetadata(r)
	return &raw.Image{
		Width: w, Height: h, BitDepth: lj.Precision,
		SensorData: pix,
		CFAPattern: raw.CFAPattern{{'R', 'G'}, {'G', 'B'}},
		BlackLevel: [4]float64{1018, 1018, 1018, 1018},
		WhiteLevel: 16224,
		Metadata:   meta,
	}, nil
}

func init() { raw.Register(Decoder{}) }

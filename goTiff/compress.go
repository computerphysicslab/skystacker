package gotiff

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/computerphysicslab/skystacker/goTiff/internal/lzw"
)

const (
	predictorNone       = 1
	predictorHorizontal = 2
)

func compressStrip(src []byte, comp Compression) ([]byte, error) {
	switch comp {
	case CompressionNone, 0:
		return src, nil
	case CompressionDeflate, CompressionDeflateOld:
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)
		if _, err := w.Write(src); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case CompressionLZW:
		var buf bytes.Buffer
		w := lzw.NewWriter(&buf)
		if _, err := w.Write(src); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case CompressionPackBits:
		return encodePackBits(src), nil
	default:
		return nil, UnsupportedError(fmt.Sprintf("compression %d", comp))
	}
}

func decompressStrip(src []byte, comp Compression, expected int64) ([]byte, error) {
	limit := expected*4 + (1 << 20)
	if limit > maxReadSize {
		limit = maxReadSize
	}
	switch comp {
	case CompressionNone, 0:
		if int64(len(src)) < expected {
			return nil, errNoPixels
		}
		return src[:expected], nil
	case CompressionDeflate, CompressionDeflateOld:
		r, err := zlib.NewReader(bytes.NewReader(src))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return readLimited(r, limit, expected)
	case CompressionLZW:
		r := lzw.NewReader(bytes.NewReader(src), lzw.MSB, 8)
		defer r.Close()
		return readLimited(r, limit, expected)
	case CompressionPackBits:
		return decodePackBits(src, expected)
	default:
		return nil, UnsupportedError(fmt.Sprintf("compression %d", comp))
	}
}

func readLimited(r io.Reader, limit, expected int64) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) < expected {
		return nil, errNoPixels
	}
	return buf[:expected], nil
}

func encodePackBits(src []byte) []byte {
	out := make([]byte, 0, len(src)+len(src)/128+8)
	for i := 0; i < len(src); {
		run := 1
		for i+run < len(src) && src[i+run] == src[i] && run < 128 {
			run++
		}
		if run >= 3 {
			out = append(out, byte(257-run), src[i])
			i += run
			continue
		}
		start := i
		i += run
		for i < len(src) && i-start < 128 {
			repeats := 1
			for i+repeats < len(src) && src[i+repeats] == src[i] && repeats < 3 {
				repeats++
			}
			if repeats >= 3 {
				break
			}
			i++
		}
		n := i - start
		out = append(out, byte(n-1))
		out = append(out, src[start:i]...)
	}
	return out
}

func decodePackBits(src []byte, expected int64) ([]byte, error) {
	out := make([]byte, 0, expected)
	for i := 0; i < len(src); {
		ctrl := int8(src[i])
		i++
		if ctrl >= 0 {
			n := int(ctrl) + 1
			if i+n > len(src) {
				return nil, errNoPixels
			}
			out = append(out, src[i:i+n]...)
			i += n
			continue
		}
		if ctrl == -128 {
			continue
		}
		if i >= len(src) {
			return nil, errNoPixels
		}
		n := 257 - int(ctrl)
		for j := 0; j < n && int64(len(out)) < expected; j++ {
			out = append(out, src[i])
		}
		i++
	}
	if int64(len(out)) < expected {
		return nil, errNoPixels
	}
	return out[:expected], nil
}

func applyPredictor(data []byte, rowBytes, bpp int, encode bool) error {
	if bpp <= 0 || rowBytes <= 0 || len(data)%rowBytes != 0 {
		return errNoPixels
	}
	for row := 0; row < len(data); row += rowBytes {
		if !encode {
			for i := bpp; i < rowBytes; i++ {
				data[row+i] += data[row+i-bpp]
			}
			continue
		}
		prev := make([]byte, bpp)
		copy(prev, data[row:row+bpp])
		for i := bpp; i < rowBytes; i++ {
			orig := data[row+i]
			data[row+i] = orig - prev[i%bpp]
			prev[i%bpp] = orig
		}
	}
	return nil
}

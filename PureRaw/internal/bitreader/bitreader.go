// Package bitreader provides an MSB-first bit reader used by RAW
// decompressors. A LSB-first variant will be added when ARW/CR3 need it.
package bitreader

import "io"

// Reader reads bits from an io.ByteReader. Bits are returned MSB-first
// within each byte.
type Reader struct {
	r     io.ByteReader
	cur   byte
	nbits uint8 // bits remaining in cur
	err   error
}

// New returns a Reader wrapping r.
func New(r io.ByteReader) *Reader { return &Reader{r: r} }

// ReadBit returns the next bit (0 or 1).
func (br *Reader) ReadBit() (uint32, error) {
	if br.nbits == 0 {
		if br.err != nil {
			return 0, br.err
		}
		b, err := br.r.ReadByte()
		if err != nil {
			br.err = err
			return 0, err
		}
		br.cur = b
		br.nbits = 8
	}
	br.nbits--
	return uint32(br.cur>>br.nbits) & 1, nil
}

// ReadBits returns the next n bits as an unsigned integer, MSB-first.
// n must be in [0, 32].
func (br *Reader) ReadBits(n uint8) (uint32, error) {
	if n > 32 {
		panic("bitreader: ReadBits n > 32")
	}
	var v uint32
	for i := uint8(0); i < n; i++ {
		b, err := br.ReadBit()
		if err != nil {
			return 0, err
		}
		v = v<<1 | b
	}
	return v, nil
}

// ReadUnary returns the number of zero bits before the next 1 bit.
// The terminating 1 is consumed. Returns io.ErrUnexpectedEOF on EOF.
func (br *Reader) ReadUnary() (uint32, error) {
	var n uint32
	for {
		b, err := br.ReadBit()
		if err != nil {
			return 0, err
		}
		if b == 1 {
			return n, nil
		}
		n++
		if n > 1<<24 {
			return 0, io.ErrUnexpectedEOF
		}
	}
}

// Err returns the first error encountered by the reader, if any.
func (br *Reader) Err() error { return br.err }

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

type Order int

const (
	LSB Order = iota
	MSB
)

const (
	maxWidth           = 12
	decoderInvalidCode = 0xffff
	flushBuffer        = 1 << maxWidth
)

type decoder struct {
	r                              io.ByteReader
	bits                           uint32
	nBits                          uint
	width                          uint
	read                           func(*decoder) (uint16, error)
	litWidth                       int
	err                            error
	clear, eof, hi, overflow, last uint16
	suffix                         [1 << maxWidth]uint8
	prefix                         [1 << maxWidth]uint16
	output                         [2 * 1 << maxWidth]byte
	o                              int
	toRead                         []byte
}

func (d *decoder) readLSB() (uint16, error) {
	for d.nBits < d.width {
		x, err := d.r.ReadByte()
		if err != nil {
			return 0, err
		}
		d.bits |= uint32(x) << d.nBits
		d.nBits += 8
	}
	code := uint16(d.bits & (1<<d.width - 1))
	d.bits >>= d.width
	d.nBits -= d.width
	return code, nil
}

func (d *decoder) readMSB() (uint16, error) {
	for d.nBits < d.width {
		x, err := d.r.ReadByte()
		if err != nil {
			return 0, err
		}
		d.bits |= uint32(x) << (24 - d.nBits)
		d.nBits += 8
	}
	code := uint16(d.bits >> (32 - d.width))
	d.bits <<= d.width
	d.nBits -= d.width
	return code, nil
}

func (d *decoder) Read(b []byte) (int, error) {
	for {
		if len(d.toRead) > 0 {
			n := copy(b, d.toRead)
			d.toRead = d.toRead[n:]
			return n, nil
		}
		if d.err != nil {
			return 0, d.err
		}
		d.decode()
	}
}

func (d *decoder) decode() {
loop:
	for {
		code, err := d.read(d)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			d.err = err
			break
		}
		switch {
		case code < d.clear:
			d.output[d.o] = uint8(code)
			d.o++
			if d.last != decoderInvalidCode {
				d.suffix[d.hi] = uint8(code)
				d.prefix[d.hi] = d.last
			}
		case code == d.clear:
			d.width = 1 + uint(d.litWidth)
			d.hi = d.eof
			d.overflow = 1 << d.width
			d.last = decoderInvalidCode
			continue
		case code == d.eof:
			d.err = io.EOF
			break loop
		case code <= d.hi:
			c, i := code, len(d.output)-1
			if code == d.hi && d.last != decoderInvalidCode {
				c = d.last
				for c >= d.clear {
					c = d.prefix[c]
				}
				d.output[i] = uint8(c)
				i--
				c = d.last
			}
			for c >= d.clear {
				d.output[i] = d.suffix[c]
				i--
				c = d.prefix[c]
			}
			d.output[i] = uint8(c)
			d.o += copy(d.output[d.o:], d.output[i:])
			if d.last != decoderInvalidCode {
				d.suffix[d.hi] = uint8(c)
				d.prefix[d.hi] = d.last
			}
		default:
			d.err = errors.New("lzw: invalid code")
			break loop
		}
		d.last, d.hi = code, d.hi+1
		if d.hi+1 >= d.overflow {
			if d.width == maxWidth {
				d.last = decoderInvalidCode
				d.hi--
			} else {
				d.width++
				d.overflow <<= 1
			}
		}
		if d.o >= flushBuffer {
			break
		}
	}
	d.toRead = d.output[:d.o]
	d.o = 0
}

func (d *decoder) Close() error {
	d.err = errors.New("lzw: reader/writer is closed")
	return nil
}

func NewReader(r io.Reader, order Order, litWidth int) io.ReadCloser {
	d := new(decoder)
	switch order {
	case LSB:
		d.read = (*decoder).readLSB
	case MSB:
		d.read = (*decoder).readMSB
	default:
		d.err = errors.New("lzw: unknown order")
		return d
	}
	if litWidth < 2 || 8 < litWidth {
		d.err = fmt.Errorf("lzw: litWidth %d out of range", litWidth)
		return d
	}
	if br, ok := r.(io.ByteReader); ok {
		d.r = br
	} else {
		d.r = bufio.NewReader(r)
	}
	d.litWidth = litWidth
	d.width = 1 + uint(litWidth)
	d.clear = uint16(1) << uint(litWidth)
	d.eof, d.hi = d.clear+1, d.clear+1
	d.overflow = uint16(1) << d.width
	d.last = decoderInvalidCode
	return d
}

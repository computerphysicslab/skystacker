// Package ljpeg implements the small subset of lossless JPEG (ISO 10918-1
// Part 3, SOF3) needed by Canon CR2 files. No DCT, no color conversion:
// the output is the raw predictor-reconstructed sample planes.
package ljpeg

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupported = errors.New("ljpeg: unsupported feature")
	ErrCorrupt     = errors.New("ljpeg: corrupt stream")
)

// Image holds lossless JPEG sample planes (one []uint16 per component).
type Image struct {
	Width     int
	Height    int
	Precision int
	CompCount int
	Planes    [][]uint16
	Predictor int
}

type huffTable struct {
	counts [17]int
	syms   []byte
	lookup map[uint16]byte // (length<<8)|code -> symbol
}

func newHuffTable(counts [17]int, syms []byte) (*huffTable, error) {
	total := 0
	for _, c := range counts[:16] {
		total += c
	}
	if total == 0 || total != len(syms) {
		return nil, ErrCorrupt
	}
	t := &huffTable{counts: counts, syms: append([]byte(nil), syms...), lookup: map[uint16]byte{}}
	code := 0
	k := 0
	for length := 1; length <= 16; length++ {
		for i := 0; i < counts[length]; i++ {
			t.lookup[uint16(length)<<8|uint16(code)] = syms[k]
			k++
			code++
		}
		code <<= 1
	}
	return t, nil
}

type compInfo struct {
	id    byte
	table byte
	pred  []uint16
}

type bitReader struct {
	data []byte
	pos  int
	bits uint32
	n    int
}

func (br *bitReader) bit() (int, error) {
	for br.n == 0 {
		if br.pos >= len(br.data) {
			return 0, ErrCorrupt
		}
		b := br.data[br.pos]
		br.pos++
		if b == 0xFF {
			if br.pos >= len(br.data) {
				return 0, ErrCorrupt
			}
			next := br.data[br.pos]
			if next == 0x00 { // byte stuffing
				br.pos++
			} else {
				return 0, fmt.Errorf("%w: marker FF%02X in entropy data", ErrCorrupt, next)
			}
		}
		br.bits = uint32(b)
		br.n = 8
	}
	br.n--
	return int((br.bits >> br.n) & 1), nil
}

func (br *bitReader) readBits(n int) (int, error) {
	v := 0
	for i := 0; i < n; i++ {
		b, err := br.bit()
		if err != nil {
			return 0, err
		}
		v = v<<1 | b
	}
	return v, nil
}

func (br *bitReader) huff(t *huffTable) (byte, error) {
	code := 0
	for length := 1; length <= 16; length++ {
		b, err := br.bit()
		if err != nil {
			return 0, err
		}
		code = code<<1 | b
		if sym, ok := t.lookup[uint16(length)<<8|uint16(code)]; ok {
			return sym, nil
		}
	}
	return 0, ErrCorrupt
}

// Decode parses an SOI..EOI lossless JPEG stream and returns its sample
// planes. Restart markers reset the bit reader but preserve predictors.
func Decode(data []byte) (*Image, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, ErrCorrupt
	}
	tables := map[byte]*huffTable{}
	var width, height, precision, ncomp int
	var comps []compInfo
	var predSel int
	pos := 2
	var entropy []byte
	for pos < len(data) {
		for pos < len(data) && data[pos] != 0xFF {
			pos++
		}
		for pos < len(data) && data[pos] == 0xFF {
			pos++
		}
		if pos >= len(data) {
			break
		}
		marker := data[pos]
		pos++
		if marker == 0xD9 { // EOI
			break
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			entropy = append(entropy, 0xFF, marker)
			continue
		}
		if pos+2 > len(data) {
			return nil, ErrCorrupt
		}
		length := int(data[pos])<<8 | int(data[pos+1])
		if length < 2 || pos+length > len(data) {
			return nil, ErrCorrupt
		}
		payload := data[pos+2 : pos+length]
		switch marker {
		case 0xC4: // DHT: may contain multiple tables
			for k := 0; k < len(payload); {
				info := payload[k]
				k++
				var counts [17]int
				total := 0
				for i := 1; i <= 16; i++ {
					if k >= len(payload) {
						return nil, ErrCorrupt
					}
					counts[i] = int(payload[k])
					total += counts[i]
					k++
				}
				if k+total > len(payload) {
					return nil, ErrCorrupt
				}
				ht, err := newHuffTable(counts, payload[k:k+total])
				if err != nil {
					return nil, err
				}
				tables[info] = ht
				k += total
			}
		case 0xC3: // SOF3
			if len(payload) < 6 {
				return nil, ErrCorrupt
			}
			precision = int(payload[0])
			height = int(payload[1])<<8 | int(payload[2])
			width = int(payload[3])<<8 | int(payload[4])
			ncomp = int(payload[5])
			if len(payload) < 6+3*ncomp {
				return nil, ErrCorrupt
			}
			comps = nil
			for i := 0; i < ncomp; i++ {
				comps = append(comps, compInfo{id: payload[6+3*i], table: payload[8+3*i]})
			}
		case 0xDA: // SOS
			if len(payload) < 4 || int(payload[0]) != ncomp || len(payload) < 1+2*ncomp+3 {
				return nil, ErrUnsupported
			}
			for i := 0; i < ncomp; i++ {
				comps[i].table = payload[1+2*i+1] >> 4
			}
			predSel = int(payload[1+2*ncomp+2] & 0x0F)
			if predSel < 1 || predSel > 7 {
				predSel = 1
			}
			entropy = append([]byte(nil), data[pos+length:]...)
			pos = len(data)
		default:
			// APPn, COM, DQT, DRI are ignored.
		}
		pos += length
	}
	if width <= 0 || height <= 0 || ncomp <= 0 || len(comps) == 0 {
		return nil, ErrCorrupt
	}
	if len(entropy) == 0 {
		return nil, ErrCorrupt
	}
	for i := range comps {
		comps[i].pred = make([]uint16, 4)
	}
	img := &Image{Width: width, Height: height, Precision: precision, CompCount: ncomp,
		Planes: make([][]uint16, ncomp), Predictor: predSel}
	for c := range img.Planes {
		img.Planes[c] = make([]uint16, width*height)
	}
	br := &bitReader{data: entropy}
	mask := uint16(1<<precision - 1)
	initPred := uint16(1 << (precision - 1))
	prev := make([]uint16, ncomp)
	for i := range prev {
		prev[i] = initPred
	}
	for p := 0; p < width*height; p++ {
		left := p % width
		for c := 0; c < ncomp; c++ {
			ht := tables[comps[c].table]
			if ht == nil {
				return nil, ErrUnsupported
			}
			sym, err := br.huff(ht)
			if err != nil {
				return nil, err
			}
			diff := 0
			if sym > 0 {
				if sym > 16 {
					return nil, ErrCorrupt
				}
				v, err := br.readBits(int(sym))
				if err != nil {
					return nil, err
				}
				diff = v
				if v < (1 << (sym - 1)) {
					diff = v - (1 << sym) + 1
				}
			}
			var pr int
			switch predSel {
			case 1:
				if left == 0 && p >= width {
					pr = int(img.Planes[c][p-width])
				} else if left == 0 {
					pr = int(initPred)
				} else {
					pr = int(prev[c])
				}
			case 2:
				if p >= width {
					pr = int(img.Planes[c][p-width])
				}
			case 3:
				if p >= width && left > 0 {
					pr = int(img.Planes[c][p-width-1])
				}
			case 4:
				ul, up, lf := 0, 0, int(prev[c])
				if p >= width {
					up = int(img.Planes[c][p-width])
				}
				if p >= width && left > 0 {
					ul = int(img.Planes[c][p-width-1])
				}
				pr = lf + up - ul
			case 5:
				ul, up, lf := 0, 0, int(prev[c])
				if p >= width {
					up = int(img.Planes[c][p-width])
				}
				if p >= width && left > 0 {
					ul = int(img.Planes[c][p-width-1])
				}
				pr = lf + ((up - ul) >> 1)
			case 6:
				ul, up, lf := 0, 0, int(prev[c])
				if p >= width {
					up = int(img.Planes[c][p-width])
				}
				if p >= width && left > 0 {
					ul = int(img.Planes[c][p-width-1])
				}
				pr = up + ((lf - ul) >> 1)
			case 7:
				lf, up := int(prev[c]), 0
				if p >= width {
					up = int(img.Planes[c][p-width])
				}
				pr = (lf + up) / 2
			}
			v := uint16(int(uint16(pr))+diff) & mask
			img.Planes[c][p] = v
			prev[c] = v
		}
	}
	return img, nil
}

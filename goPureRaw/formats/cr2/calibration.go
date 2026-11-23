package cr2

import (
	"encoding/binary"
	"math"

	"github.com/computerphysicslab/skystacker/goPureRaw/internal/tiff"
	"github.com/computerphysicslab/skystacker/goPureRaw/raw"
)

// TIFF/DNG and Canon tags that carry sensor calibration values. CR2 files
// may publish these in any IFD in the chain; older cameras omit them, in
// which case Decode keeps its documented compatibility defaults.
const (
	tagCFARepeatPatternDim uint16 = 0x828D
	tagCFAPattern          uint16 = 0x828E
	tagCR2CFAPattern       uint16 = 0xC5E0
	tagBlackLevelRepeatDim uint16 = 0xC619
	tagBlackLevel          uint16 = 0xC61A
	tagWhiteLevel          uint16 = 0xC61D
)

// defaultBlackLevel14 and defaultWhiteLevel14 reproduce the historical EOS
// 40D constants used before container calibration tags were read.
const (
	defaultBlackLevel14 = 1018.0
	defaultWhiteLevel14 = 16224.0
)

// calibrationIFDs returns every directory worth searching for calibration
// tags. The Next-pointer chain is preferred; the legacy IFD fields and
// SubIFDs are included as fallbacks for minimal synthetic documents.
func calibrationIFDs(doc *tiff.Document) []*tiff.IFD {
	if doc == nil {
		return nil
	}
	seen := make(map[*tiff.IFD]bool)
	out := make([]*tiff.IFD, 0, len(doc.IFDs)+len(doc.SubIFDs)+2)
	add := func(ifd *tiff.IFD) {
		if ifd != nil && !seen[ifd] {
			seen[ifd] = true
			out = append(out, ifd)
		}
	}
	if len(doc.IFDs) > 0 {
		for _, ifd := range doc.IFDs {
			add(ifd)
		}
	} else {
		add(doc.IFD0)
		add(doc.IFD1)
	}
	for _, ifd := range doc.SubIFDs {
		add(ifd)
	}
	return out
}

// entryCode reads a small integer tag value whether it is stored as BYTE or
// SHORT, the two encodings used by the TIFF CFA tags.
func entryCode(e tiff.Entry, order binary.ByteOrder, idx int) (int, bool) {
	if idx < 0 {
		return 0, false
	}
	switch e.Type {
	case 1, 7: // BYTE, UNDEFINED
		if idx >= len(e.Raw) {
			return 0, false
		}
		return int(e.Raw[idx]), true
	case 3: // SHORT
		v, err := e.Uint16(order, idx)
		if err != nil {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

// bayerColor maps the standard TIFF CFA colour codes (0=R, 1=G, 2=B) to
// their byte representation. Non-Bayer codes are not accepted by raw.Image.
func bayerColor(code int) (byte, bool) {
	switch code {
	case 0:
		return 'R', true
	case 1:
		return 'G', true
	case 2:
		return 'B', true
	default:
		return 0, false
	}
}

// cfaFromTIFF looks first for the standard CFARepeatPatternDim/CFAPattern
// pair, then for Canon's compact CR2CFAPattern code. It returns false when
// no supported 2x2 Bayer pattern is present.
func cfaFromTIFF(doc *tiff.Document) (raw.CFAPattern, bool) {
	ifds := calibrationIFDs(doc)
	if len(ifds) == 0 {
		return raw.CFAPattern{}, false
	}
	order := doc.Order
	for _, ifd := range ifds {
		dim, hasDim := ifd.Entries[tagCFARepeatPatternDim]
		pat, hasPat := ifd.Entries[tagCFAPattern]
		if !hasDim || !hasPat || pat.Count < 4 {
			continue
		}
		rows, okR := entryCode(dim, order, 0)
		cols, okC := entryCode(dim, order, 1)
		if !okR || !okC || rows != 2 || cols != 2 {
			continue
		}
		var cfa raw.CFAPattern
		valid := true
		for i := range 4 {
			code, ok := entryCode(pat, order, i)
			if !ok {
				valid = false
				break
			}
			color, ok := bayerColor(code)
			if !ok {
				valid = false
				break
			}
			cfa[i/2][i%2] = color
		}
		if valid {
			return cfa, true
		}
	}
	for _, ifd := range ifds {
		e, ok := ifd.Entries[tagCR2CFAPattern]
		if !ok {
			continue
		}
		code, err := e.Float(order, 0)
		if err != nil {
			continue
		}
		switch code {
		case 1:
			return raw.CFAPattern{{'R', 'G'}, {'G', 'B'}}, true
		case 2:
			return raw.CFAPattern{{'B', 'G'}, {'G', 'R'}}, true
		case 3:
			return raw.CFAPattern{{'G', 'B'}, {'R', 'G'}}, true
		case 4:
			return raw.CFAPattern{{'G', 'R'}, {'B', 'G'}}, true
		}
	}
	return raw.CFAPattern{}, false
}

// blackLevelFromTIFF reads a scalar or 2x2 BlackLevel tag. Scalar values are
// applied to all four CFA positions; a repeat dimension other than 1x1 or
// 2x2 is rejected rather than guessed.
func blackLevelFromTIFF(doc *tiff.Document) ([4]float64, bool) {
	var black [4]float64
	for _, ifd := range calibrationIFDs(doc) {
		e, ok := ifd.Entries[tagBlackLevel]
		if !ok {
			continue
		}
		count := 0
		if dim, hasDim := ifd.Entries[tagBlackLevelRepeatDim]; hasDim {
			rows, okR := entryCode(dim, doc.Order, 0)
			cols, okC := entryCode(dim, doc.Order, 1)
			if okR && okC && rows > 0 && cols > 0 && rows <= 16 && cols <= 16 {
				count = rows * cols
			}
		}
		if count == 0 {
			count = int(e.Count)
		}
		if count == 1 {
			v, err := e.Float(doc.Order, 0)
			if err != nil {
				continue
			}
			black = [4]float64{v, v, v, v}
			return black, true
		}
		if count != 4 {
			continue
		}
		valid := true
		for i := range 4 {
			v, err := e.Float(doc.Order, i)
			if err != nil {
				valid = false
				break
			}
			black[i] = v
		}
		if valid {
			return black, true
		}
	}
	return black, false
}

// whiteLevelFromTIFF reads the first valid scalar DNG WhiteLevel tag.
func whiteLevelFromTIFF(doc *tiff.Document) (float64, bool) {
	for _, ifd := range calibrationIFDs(doc) {
		e, ok := ifd.Entries[tagWhiteLevel]
		if !ok {
			continue
		}
		v, err := e.Float(doc.Order, 0)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

// calibrationFromTIFF builds the CFA and level fields used by raw.Image.
// Values are validated against maxLevel before they are accepted; a missing
// or invalid tag falls back to the compatibility defaults for legacy CR2.
func calibrationFromTIFF(doc *tiff.Document, maxLevel float64) (raw.CFAPattern, [4]float64, float64) {
	cfa, cfaOK := cfaFromTIFF(doc)
	if !cfaOK {
		cfa = raw.CFAPattern{{'R', 'G'}, {'G', 'B'}}
	}
	black, blackOK := blackLevelFromTIFF(doc)
	maxBlack := 0.0
	for _, v := range black {
		if v < 0 || v > maxLevel || math.IsNaN(v) || math.IsInf(v, 0) {
			blackOK = false
			break
		}
		if v > maxBlack {
			maxBlack = v
		}
	}
	if !blackOK {
		black = [4]float64{math.Min(defaultBlackLevel14, maxLevel/4), math.Min(defaultBlackLevel14, maxLevel/4), math.Min(defaultBlackLevel14, maxLevel/4), math.Min(defaultBlackLevel14, maxLevel/4)}
		maxBlack = black[0]
	}
	white, whiteOK := whiteLevelFromTIFF(doc)
	if !whiteOK || white <= maxBlack || white > maxLevel {
		white = math.Min(defaultWhiteLevel14, maxLevel)
		if white <= maxBlack {
			white = maxLevel
		}
	}
	return cfa, black, white
}

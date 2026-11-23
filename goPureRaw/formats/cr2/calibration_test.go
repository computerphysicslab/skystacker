package cr2

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/computerphysicslab/skystacker/goPureRaw/internal/tiff"
	"github.com/computerphysicslab/skystacker/goPureRaw/raw"
)

// twoComponentSOF3Stream builds the smallest structural CR2-like lossless
// stream with two components. Every Huffman symbol is 0, so all samples decode
// to the initial predictor for their precision.
func twoComponentSOF3Stream(precision byte) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8}) // SOI
	for _, id := range []byte{0x00, 0x01} {
		b.Write([]byte{
			0xFF, 0xC4, 0x00, 0x14, id, // DHT, one 4-bit code for symbol 0
			0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0,
		})
	}
	b.Write([]byte{
		0xFF, 0xC3, 0x00, 0x0E, precision, // SOF3, 2x2, two components
		0x00, 0x02, 0x00, 0x02, 0x02,
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x01,
	})
	b.Write([]byte{
		0xFF, 0xDA, 0x00, 0x0A, // SOS, two components, predictor 1
		0x02,
		0x01, 0x00,
		0x02, 0x10,
		0x00, 0x3F, 0x00,
	})
	b.Write([]byte{0, 0, 0, 0}) // eight 4-bit zero differences
	b.Write([]byte{0xFF, 0xD9}) // EOI
	return b.Bytes()
}

// syntheticCR2WithCalibration writes a minimal CR2 header and IFD0 carrying
// DNG calibration tags, followed by a decodable two-component SOF3 stream.
func syntheticCR2WithCalibration(cfa, black []uint16, white uint16) []byte {
	var b bytes.Buffer
	b.Write([]byte{'I', 'I', 0x2A, 0x00, 0x10, 0x00, 0x00, 0x00, 'C', 'R', 0x02, 0x00, 0, 0, 0, 0})
	_ = binary.Write(&b, binary.LittleEndian, uint16(5))

	entry := func(tag uint16, count uint32, value uint32) {
		_ = binary.Write(&b, binary.LittleEndian, tag)
		_ = binary.Write(&b, binary.LittleEndian, uint16(3)) // SHORT
		_ = binary.Write(&b, binary.LittleEndian, count)
		_ = binary.Write(&b, binary.LittleEndian, value)
	}
	entry(tagCFARepeatPatternDim, 2, uint32(2)|uint32(2)<<16)
	cfaOffset := uint32(16 + 2 + 5*12 + 4)
	entry(tagCFAPattern, 4, cfaOffset)
	entry(tagBlackLevelRepeatDim, 2, uint32(2)|uint32(2)<<16)
	blackOffset := cfaOffset + uint32(len(cfa)*2)
	entry(tagBlackLevel, 4, blackOffset)
	entry(tagWhiteLevel, 1, uint32(white))
	b.Write([]byte{0, 0, 0, 0}) // no next IFD

	for _, v := range cfa {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	for _, v := range black {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.Write(twoComponentSOF3Stream(14))
	return b.Bytes()
}

func TestDecodeReadsContainerCalibration(t *testing.T) {
	data := syntheticCR2WithCalibration(
		[]uint16{2, 1, 1, 0},         // BGGR
		[]uint16{500, 600, 700, 800}, // non-1018 black levels
		16000,
	)
	img, err := (Decoder{}).Decode(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != 4 || img.Height != 2 || img.BitDepth != 14 {
		t.Fatalf("geometry = %dx%d @ %d bits, want 4x2 @ 14", img.Width, img.Height, img.BitDepth)
	}
	wantCFA := raw.CFAPattern{{'B', 'G'}, {'G', 'R'}}
	if img.CFAPattern != wantCFA {
		t.Fatalf("CFA = %v, want %v", img.CFAPattern, wantCFA)
	}
	wantBlack := [4]float64{500, 600, 700, 800}
	if img.BlackLevel != wantBlack {
		t.Fatalf("black = %v, want %v", img.BlackLevel, wantBlack)
	}
	if img.WhiteLevel != 16000 {
		t.Fatalf("white = %v, want 16000", img.WhiteLevel)
	}
}

func TestCR2CompactCFAPattern(t *testing.T) {
	doc := &tiff.Document{
		Order: binary.LittleEndian,
		IFD0: &tiff.IFD{Entries: map[uint16]tiff.Entry{
			tagCR2CFAPattern: {Tag: tagCR2CFAPattern, Type: 4, Count: 1, Raw: []byte{2, 0, 0, 0}},
		}},
	}
	cfa, ok := cfaFromTIFF(doc)
	if !ok {
		t.Fatal("CR2CFAPattern code 2 not accepted")
	}
	want := raw.CFAPattern{{'B', 'G'}, {'G', 'R'}}
	if cfa != want {
		t.Fatalf("CFA = %v, want %v", cfa, want)
	}
}

func TestCalibrationFallback(t *testing.T) {
	cfa, black, white := calibrationFromTIFF(nil, 16383)
	if cfa != (raw.CFAPattern{{'R', 'G'}, {'G', 'B'}}) {
		t.Fatalf("fallback CFA = %v", cfa)
	}
	if black != [4]float64{1018, 1018, 1018, 1018} {
		t.Fatalf("fallback black = %v", black)
	}
	if white != 16224 {
		t.Fatalf("fallback white = %v", white)
	}
}

func TestBlackLevelRejectsUnsupportedRepeatDim(t *testing.T) {
	doc := &tiff.Document{
		Order: binary.LittleEndian,
		IFD0: &tiff.IFD{Entries: map[uint16]tiff.Entry{
			tagBlackLevelRepeatDim: {Tag: tagBlackLevelRepeatDim, Type: 3, Count: 2, Raw: []byte{3, 0, 3, 0}},
			tagBlackLevel:          {Tag: tagBlackLevel, Type: 3, Count: 4, Raw: []byte{1, 0, 2, 0, 3, 0, 4, 0}},
		}},
	}
	if _, ok := blackLevelFromTIFF(doc); ok {
		t.Fatal("3x3 black-level repeat accepted")
	}
}

// TestCR2CFAInIFD3 mirrors the real EOS 40D layout where the compact CFA tag
// lives in IFD3, beyond IFD0 and IFD1. A BGGR code proves the value comes from
// the container rather than the RGGB compatibility fallback.
func TestCR2CFAInIFD3(t *testing.T) {
	doc := &tiff.Document{
		Order: binary.LittleEndian,
		IFD0:  &tiff.IFD{Entries: map[uint16]tiff.Entry{}},
		IFDs: []*tiff.IFD{
			{Entries: map[uint16]tiff.Entry{}},
			{Entries: map[uint16]tiff.Entry{}},
			{Entries: map[uint16]tiff.Entry{}},
			{Entries: map[uint16]tiff.Entry{
				tagCR2CFAPattern: {Tag: tagCR2CFAPattern, Type: 4, Count: 1, Raw: []byte{2, 0, 0, 0}},
			}},
		},
	}
	cfa, _, _ := calibrationFromTIFF(doc, 16383)
	want := raw.CFAPattern{{'B', 'G'}, {'G', 'R'}}
	if cfa != want {
		t.Fatalf("CFA from IFD3 = %v, want %v", cfa, want)
	}
}

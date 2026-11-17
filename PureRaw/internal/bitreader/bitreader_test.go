package bitreader

import (
	"bytes"
	"io"
	"testing"
)

func TestReadBitAndBits(t *testing.T) {
	// 0b10110010 = 0xB2; bits must come out MSB-first.
	br := New(bytes.NewReader([]byte{0xB2, 0x00}))
	wantBits := []uint32{1, 0, 1, 1, 0, 0, 1, 0}
	for i, want := range wantBits {
		got, err := br.ReadBit()
		if err != nil {
			t.Fatalf("bit %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("bit %d = %d, want %d", i, got, want)
		}
	}
	if err := br.Err(); err != nil {
		t.Fatalf("Err() after clean read: %v", err)
	}
}

func TestReadBitsWidths(t *testing.T) {
	cases := []struct {
		data []byte
		n    uint8
		want uint32
	}{
		{[]byte{0x80}, 1, 1},
		{[]byte{0x01}, 1, 0},
		{[]byte{0xFF}, 8, 0xFF},
		{[]byte{0xAB, 0xCD}, 16, 0xABCD},
		{[]byte{0x12, 0x34, 0x56, 0x78}, 32, 0x12345678},
		{[]byte{0xFF}, 0, 0},
	}
	for _, c := range cases {
		br := New(bytes.NewReader(c.data))
		got, err := br.ReadBits(c.n)
		if err != nil {
			t.Fatalf("ReadBits(%d): %v", c.n, err)
		}
		if got != c.want {
			t.Fatalf("ReadBits(%d) = %#x, want %#x", c.n, got, c.want)
		}
	}
}

func TestReadBitsAcrossBytesAndEOF(t *testing.T) {
	br := New(bytes.NewReader([]byte{0b10110010, 0b11001100}))
	v, err := br.ReadBits(16)
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(0xB2CC); v != want {
		t.Fatalf("16 bits = %#x, want %#x", v, want)
	}
	if _, err := br.ReadBit(); err != io.EOF {
		t.Fatalf("expected EOF after consuming two bytes, got %v", err)
	}
	if err := br.Err(); err != io.EOF {
		t.Fatalf("Err() = %v, want EOF", err)
	}
}

func TestReadUnary(t *testing.T) {
	// 0010 0001 -> two zeros then a one (2), then four zeros then a one (4).
	br := New(bytes.NewReader([]byte{0b00100001, 0x00}))
	for _, want := range []uint32{2, 4} {
		got, err := br.ReadUnary()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("ReadUnary = %d, want %d", got, want)
		}
	}
}

func TestReadUnaryEOF(t *testing.T) {
	br := New(bytes.NewReader([]byte{0x00}))
	if _, err := br.ReadUnary(); err != io.EOF {
		t.Fatalf("ReadUnary on all-zero byte: got %v, want EOF", err)
	}
}

func TestReadBitsPanicsAbove32(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("ReadBits(33) did not panic")
		}
	}()
	br := New(bytes.NewReader([]byte{0xFF}))
	_, _ = br.ReadBits(33)
}

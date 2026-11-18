package raf

import (
	"bytes"
	"testing"
)

func TestProbe(t *testing.T) {
	good := append([]byte(magic), make([]byte, 64)...)
	if !(Decoder{}).Probe(bytes.NewReader(good)) {
		t.Fatal("valid RAF signature not detected")
	}
	bad := [][]byte{
		nil,
		[]byte("FUJIFILM"),
		[]byte("FUJIFILMCCD-RAWX"),
		make([]byte, 64),
		bytes.Repeat([]byte{0xFF}, 16),
	}
	for i, b := range bad {
		if (Decoder{}).Probe(bytes.NewReader(b)) {
			t.Fatalf("bad input %d accepted", i)
		}
	}
}

func TestDecodeStub(t *testing.T) {
	img, err := (Decoder{}).Decode(bytes.NewReader([]byte(magic)), int64(len(magic)))
	if img != nil || err == nil {
		t.Fatalf("stub Decode returned img=%v err=%v", img, err)
	}
}

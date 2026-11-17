package cr3

import (
	"bytes"
	"testing"
)

func TestProbe(t *testing.T) {
	good := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'c', 'r', 'x', ' ', 0, 0, 0, 0}
	if !(Decoder{}).Probe(bytes.NewReader(good)) {
		t.Fatal("valid CR3 signature not detected")
	}
	bad := [][]byte{
		nil,
		good[:8], // truncated
		[]byte{0, 0, 0, 0, 'f', 't', 'y', 'p', 'm', 'p', '4', '1'},
		[]byte{0, 0, 0, 0, 'm', 'o', 'o', 'v', 'c', 'r', 'x', ' '},
		bytes.Repeat([]byte{0xFF}, 32),
	}
	for i, b := range bad {
		if (Decoder{}).Probe(bytes.NewReader(b)) {
			t.Fatalf("bad input %d accepted", i)
		}
	}
}

func TestDecodeStub(t *testing.T) {
	img, err := (Decoder{}).Decode(bytes.NewReader([]byte("xxxxftypcrx ")), 12)
	if img != nil || err == nil {
		t.Fatalf("stub Decode returned img=%v err=%v", img, err)
	}
}

package cr2

import (
	"bytes"
	"os"
	"testing"

	"github.com/computerphysicslab/skystacker/PureRaw/raw"
)

func TestProbe(t *testing.T) {
	good := []byte{'I', 'I', 0x2A, 0x00, 0x10, 0x00, 0x00, 0x00, 'C', 'R', 0x02, 0x00}
	if !(Decoder{}).Probe(bytes.NewReader(good)) {
		t.Fatal("valid CR2 header not detected")
	}
	bad := [][]byte{
		nil,
		{'I', 'I', 0x2B, 0x00, 0x10, 0x00, 0x00, 0x00, 'C', 'R', 0x02, 0x00},
		{'I', 'I', 0x2A, 0x00, 0x10, 0x00, 0x00, 0x00, 'C', 'S', 0x02, 0x00},
		bytes.Repeat([]byte{0x00}, 16),
	}
	for i, b := range bad {
		if (Decoder{}).Probe(bytes.NewReader(b)) {
			t.Fatalf("bad header %d accepted", i)
		}
	}
	if (Decoder{}).Format() != raw.FormatCR2 {
		t.Fatal("wrong format identifier")
	}
}

// TestDecodeOracle runs only when PURERAW_CR2 points at a real CR2 file.
// It checks dimensions and sample plausibility, not bit-exactness against
// dcraw (geometric mapping is still under validation).
func TestDecodeOracle(t *testing.T) {
	path := os.Getenv("PURERAW_CR2")
	if path == "" {
		t.Skip("set PURERAW_CR2 to a Canon CR2 file")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	img, err := (Decoder{}).Decode(f, fi.Size())
	if err != nil {
		t.Fatal(err)
	}
	if img.Width == 0 || img.Height == 0 || img.BitDepth < 10 || img.BitDepth > 16 {
		t.Fatalf("bad geometry: %dx%d @ %d bits", img.Width, img.Height, img.BitDepth)
	}
	if len(img.SensorData) != img.Width*img.Height {
		t.Fatalf("sensor data length %d, want %d", len(img.SensorData), img.Width*img.Height)
	}
	var sum uint64
	for _, v := range img.SensorData {
		sum += uint64(v)
	}
	mean := float64(sum) / float64(len(img.SensorData))
	if mean <= 0 || mean >= float64(uint64(1)<<img.BitDepth) {
		t.Fatalf("implausible mean %.1f", mean)
	}
	t.Logf("decoded %dx%d @ %d bits, mean %.1f", img.Width, img.Height, img.BitDepth, mean)
}

func TestConfigOracle(t *testing.T) {
	path := os.Getenv("PURERAW_CR2")
	if path == "" {
		t.Skip("set PURERAW_CR2 to a Canon CR2 file")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	w, h, bits, err := Config(f, fi.Size())
	if err != nil {
		t.Fatal(err)
	}
	if w != 3944 || h != 2622 || bits != 14 {
		t.Fatalf("Config = %dx%d @ %d bits, want 3944x2622 @ 14", w, h, bits)
	}
}

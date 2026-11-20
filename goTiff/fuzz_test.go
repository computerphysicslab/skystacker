package gotiff

import (
	"bytes"
	"testing"
)

func FuzzDecode(f *testing.F) {
	img := &Image{
		Width: 4, Height: 3, Channels: 3, Bits: 8,
		Format: SampleUint, Photometric: PhotoRGB,
		Pixels: make([]float32, 36),
	}
	var valid bytes.Buffer
	if err := Encode(&valid, img, &Options{Compression: CompressionDeflate}); err != nil {
		f.Fatal(err)
	}
	f.Add(valid.Bytes())
	f.Add([]byte("II*\x00"))
	f.Add([]byte("II+\x00\x08\x00\x00\x00\x08\x00\x00\x00\x00\x00\x00\x00"))
	f.Add([]byte("not a tiff"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Decode(bytes.NewReader(data))
		_, _ = DecodeConfig(bytes.NewReader(data))
	})
}

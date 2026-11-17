package ljpeg

import (
	"fmt"
	"os"
	"testing"
)

func TestRealCR2(t *testing.T) {
	f, err := os.Open("/home/jips/datos/datos-juan/proyectos/astronomia/SkyStacker/assets/OrionNebula/lights/IMG_7005.CR2")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	b := make([]byte, fi.Size())
	f.Read(b)
	off := 1366852
	end := len(b)
	img, err := Decode(b[off:end])
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("w=%d h=%d prec=%d comps=%d pred=%d\n", img.Width, img.Height, img.Precision, img.CompCount, img.Predictor)
	for c := range img.Planes {
		var mn, mx uint16 = 65535, 0
		var sum uint64
		for _, v := range img.Planes[c] {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
			sum += uint64(v)
		}
		fmt.Printf(" plane%d n=%d mn=%d mx=%d mean=%.1f first=%v\n", c, len(img.Planes[c]), mn, mx, float64(sum)/float64(len(img.Planes[c])), img.Planes[c][:8])
	}
}

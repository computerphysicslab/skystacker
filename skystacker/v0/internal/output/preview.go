package output

import (
	"image"
	"image/jpeg"
	"io"
	"math"
)

// previewGamma is the display gamma applied to all preview JPEGs. A value
// of 0.5 (square root) is a modest stretch suitable for typical
// astrophotography data whose background is far dimmer than its stars.
// Previews are not scientific outputs.
const previewGamma = 0.5

const previewLUTSize = 4096

var previewLUT = buildPreviewLUT()

func buildPreviewLUT() [previewLUTSize + 1]uint8 {
	var lut [previewLUTSize + 1]uint8
	for i := 0; i <= previewLUTSize; i++ {
		v := float64(i) / previewLUTSize
		s := math.Pow(v, previewGamma)
		x := int(s*255 + 0.5)
		if x < 0 {
			x = 0
		}
		if x > 255 {
			x = 255
		}
		lut[i] = uint8(x)
	}
	return lut
}

// WritePreviewJPEG writes a display-oriented JPEG preview of an image whose
// values are expected to lie in [0, 1]. Values outside that range are
// clamped by the display LUT.
func WritePreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	return writePreviewJPEG(w, width, height, pix, 1.0)
}

// WriteAutoNormalizedPreviewJPEG writes a display-oriented JPEG preview that
// divides every sample by the global maximum value before applying the
// display LUT. It is intended for outputs such as the sum image whose
// numeric range can exceed [0, 1]. Relative intensities and colour balance
// are preserved; the absolute scale is not.
func WriteAutoNormalizedPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	maxVal := float32(0)
	for _, v := range pix {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal <= 0 {
		maxVal = 1.0
	}
	return writePreviewJPEG(w, width, height, pix, maxVal)
}

func writePreviewJPEG(w io.Writer, width, height int, pix []float32, scale float32) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	inv := float32(1.0) / scale
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = previewLUT[stretchIndex(pix[i+0]*inv)]
			img.Pix[o+1] = previewLUT[stretchIndex(pix[i+1]*inv)]
			img.Pix[o+2] = previewLUT[stretchIndex(pix[i+2]*inv)]
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

func stretchIndex(v float32) int {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return previewLUTSize
	}
	return int(v * previewLUTSize)
}

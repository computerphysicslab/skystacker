package imaging

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // register JPEG decoder
	"io"
	"os"
)

// DecodeConfigFile reads only enough of the file to determine its dimensions.
func DecodeConfigFile(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Config{}, err
	}
	return cfg, nil
}

// DecodeFile fully decodes the file and returns a Frame.
func DecodeFile(path string) (*Frame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}

// Decode reads an image from r and returns a Frame.
func Decode(r io.Reader) (*Frame, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return FromImage(img)
}

// FromImage converts any image.Image into a Frame.
func FromImage(img image.Image) (*Frame, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", w, h)
	}
	f := &Frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	switch src := img.(type) {
	case *image.YCbCr:
		decodeYCbCr(src, f)
	default:
		decodeGeneric(img, f)
	}
	return f, nil
}

// decodeYCbCr converts a YCbCr image using direct plane access.
func decodeYCbCr(src *image.YCbCr, f *Frame) {
	w, h := f.Width, f.Height
	b := src.Bounds()
	const inv = 1.0 / 255.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			yi := src.YOffset(b.Min.X+x, b.Min.Y+y)
			ci := src.COffset(b.Min.X+x, b.Min.Y+y)
			r, g, bl := color.YCbCrToRGB(src.Y[yi], src.Cb[ci], src.Cr[ci])
			i := (y*w + x) * 3
			f.Pix[i+0] = float32(r) * inv
			f.Pix[i+1] = float32(g) * inv
			f.Pix[i+2] = float32(bl) * inv
		}
	}
}

// decodeGeneric handles any image.Image via color.Color. The RGBA() method
// returns 16-bit values in [0, 65535]; we normalise to [0, 1].
func decodeGeneric(img image.Image, f *Frame) {
	w, h := f.Width, f.Height
	b := img.Bounds()
	const inv = 1.0 / 65535.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := (y*w + x) * 3
			f.Pix[i+0] = float32(r) * inv
			f.Pix[i+1] = float32(g) * inv
			f.Pix[i+2] = float32(bl) * inv
		}
	}
}

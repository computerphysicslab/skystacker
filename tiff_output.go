package main

import (
	"fmt"
	"io"
	"math"
	"strings"

	gotiff "github.com/computerphysicslab/skystacker/goTiff"
)

type tiffWriteConfig struct {
	compression string
	depth       string
	astro       bool
	tiled       bool
	predictor   bool
	Bayer       string
}

func (c tiffWriteConfig) active() bool {
	return (c.compression != "" && c.compression != "none") ||
		(c.depth != "" && c.depth != "32f") || c.astro || c.tiled || c.predictor
}

func writeStackTIFF(path string, width, height int, pix []float32, divisor float32, big bool, cfg tiffWriteConfig, wcs *wcsTan) error {
	if !cfg.active() {
		return writeFileAtomic(path, func(w io.Writer) error {
			if big {
				return writeFloat32RGBBigTIFF(w, width, height, pix)
			}
			return writeFloat32RGBScaled(w, width, height, pix, divisor)
		})
	}
	compression := gotiff.CompressionNone
	switch cfg.compression {
	case "", "none":
	case "packbits":
		compression = gotiff.CompressionPackBits
	case "lzw":
		compression = gotiff.CompressionLZW
	case "deflate":
		compression = gotiff.CompressionDeflate
	default:
		return fmt.Errorf("unsupported --tiff-compression %q", cfg.compression)
	}
	bits, format := 32, gotiff.SampleFloat
	switch cfg.depth {
	case "", "32f":
	case "8":
		bits, format = 8, gotiff.SampleUint
	case "16":
		bits, format = 16, gotiff.SampleUint
	case "64f":
		bits, format = 64, gotiff.SampleFloat
	default:
		return fmt.Errorf("unsupported --tiff-depth %q", cfg.depth)
	}
	scaled := append([]float32(nil), pix...)
	if divisor != 0 && divisor != 1 {
		for i := range scaled {
			scaled[i] /= divisor
		}
	}
	img := &gotiff.Image{
		Width: width, Height: height, Channels: 3,
		Bits: bits, Format: format, Photometric: gotiff.PhotoRGB,
		Pixels: scaled,
	}
	if cfg.astro && wcs != nil {
		img.Metadata = map[string]string{"ImageDescription": astroTIFFDescription(wcs, cfg.Bayer)}
	}
	opt := &gotiff.Options{
		Compression: compression,
		BigTIFF:     big,
		Tiled:       cfg.tiled,
		Predictor:   cfg.predictor,
	}
	return writeFileAtomic(path, func(w io.Writer) error { return gotiff.Encode(w, img, opt) })
}

func astroTIFFDescription(w *wcsTan, bayer string) string {
	if w == nil {
		return ""
	}
	deg := 180 / math.Pi
	text := fmt.Sprintf("CTYPE1  = 'RA---TAN'\nCTYPE2  = 'DEC--TAN'\nCRVAL1  = %.12f\nCRVAL2  = %.12f\nCRPIX1  = %.8f\nCRPIX2  = %.8f\nCD1_1   = %.12e\nCD1_2   = %.12e\nCD2_1   = %.12e\nCD2_2   = %.12e\n",
		w.CRVAL1*deg, w.CRVAL2*deg, w.CRPIX1, w.CRPIX2,
		w.CD[0][0]*deg, w.CD[0][1]*deg, w.CD[1][0]*deg, w.CD[1][1]*deg)
	if bayer != "" {
		text += fmt.Sprintf("BAYERPAT= '%-8s'\n", bayer)
	}
	return text
}

func firstBayerPattern(paths []string) string {
	for _, path := range paths {
		cards, err := readFITSPrimaryHeader(path)
		if err != nil {
			continue
		}
		if v := strings.TrimSpace(cards["BAYERPAT"]); v != "" {
			return v
		}
	}
	return ""
}

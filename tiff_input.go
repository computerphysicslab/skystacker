package main

import (
	"fmt"
	"image"
	"os"

	gotiff "github.com/computerphysicslab/skystacker/goTiff"
)

func decodeTIFFConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	return gotiff.DecodeConfig(f)
}

func decodeTIFFInput(path string, targetW, targetH int) (*frame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	img, err := gotiff.Decode(f)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("decode TIFF %s: %w", path, err)
	}
	frm := &frame{Width: img.Width, Height: img.Height, Pix: make([]float32, img.Width*img.Height*3)}
	if wcs, bayer := parseAstroTIFFDescription(img.Metadata["ImageDescription"]); wcs != nil {
		frm.WCS = wcs
	} else if bayer != "" {
		frm.BayerPattern = bayer
	}
	if img.CFAPattern != "" {
		frm.BayerPattern = img.CFAPattern
	}
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			src := (y*img.Width + x) * img.Channels
			r := img.Pixels[src]
			g, b := r, r
			if img.Channels >= 3 {
				g, b = img.Pixels[src+1], img.Pixels[src+2]
			}
			dst := (y*img.Width + x) * 3
			frm.Pix[dst], frm.Pix[dst+1], frm.Pix[dst+2] = r, g, b
		}
	}
	if targetW > 0 && targetH > 0 && (frm.Width != targetW || frm.Height != targetH) {
		return resizeFrame(frm, targetW, targetH), nil
	}
	return frm, nil
}

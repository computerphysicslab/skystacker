package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
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

// tiffSampleConfig resolves the compression, bit depth and sample format of a
// goTiff write from the user-facing flags.
type tiffSampleConfig struct {
	compression gotiff.Compression
	bits        int
	format      gotiff.SampleFormat
}

func resolveTIFFSampleConfig(cfg tiffWriteConfig) (tiffSampleConfig, error) {
	c := tiffSampleConfig{compression: gotiff.CompressionNone, bits: 32, format: gotiff.SampleFloat}
	switch cfg.compression {
	case "", "none":
	case "packbits":
		c.compression = gotiff.CompressionPackBits
	case "lzw":
		c.compression = gotiff.CompressionLZW
	case "deflate":
		c.compression = gotiff.CompressionDeflate
	default:
		return c, fmt.Errorf("unsupported --tiff-compression %q", cfg.compression)
	}
	switch cfg.depth {
	case "", "32f":
	case "8":
		c.bits, c.format = 8, gotiff.SampleUint
	case "16":
		c.bits, c.format = 16, gotiff.SampleUint
	case "64f":
		c.bits, c.format = 64, gotiff.SampleFloat
	default:
		return c, fmt.Errorf("unsupported --tiff-depth %q", cfg.depth)
	}
	return c, nil
}

func scaleTIFFPixels(pix []float32, divisor float32) ([]float32, error) {
	if divisor == 0 {
		return nil, errors.New("tiff: divisor must be non-zero")
	}
	if divisor == 1 {
		return pix, nil
	}
	out := make([]float32, len(pix))
	inv := float32(1) / divisor
	for i := range pix {
		out[i] = pix[i] * inv
	}
	return out, nil
}

func newTIFFImage(width, height int, pix []float32, sc tiffSampleConfig) *gotiff.Image {
	return &gotiff.Image{
		Width: width, Height: height, Channels: 3,
		Bits: sc.bits, Format: sc.format, Photometric: gotiff.PhotoRGB,
		Pixels: pix,
	}
}

// writeStackTIFF writes a scientific stack through goTiff. Every path — the
// default uncompressed 32-bit float layout and the optional compression,
// depth, Astro-TIFF, tiled and predictor variants — is encoded by the same
// module; the legacy in-tree IFD writers were removed (roadmap item 110).
func writeStackTIFF(path string, width, height int, pix []float32, divisor float32, big bool, cfg tiffWriteConfig, wcs *wcsTan) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("tiff: invalid dimensions %dx%d", width, height)
	}
	if len(pix) != width*height*3 {
		return fmt.Errorf("tiff: pixel buffer length %d does not match %dx%dx3 = %d", len(pix), width, height, width*height*3)
	}
	sc, err := resolveTIFFSampleConfig(cfg)
	if err != nil {
		return err
	}
	scaled, err := scaleTIFFPixels(pix, divisor)
	if err != nil {
		return err
	}
	img := newTIFFImage(width, height, scaled, sc)
	if cfg.astro && wcs != nil {
		img.Metadata = map[string]string{"ImageDescription": astroTIFFDescription(wcs, cfg.Bayer)}
	}
	opt := &gotiff.Options{
		Compression: sc.compression,
		BigTIFF:     big,
		Tiled:       cfg.tiled,
		Predictor:   cfg.predictor,
	}
	return writeFileAtomic(path, func(w io.Writer) error { return gotiff.Encode(w, img, opt) })
}

// The functions below keep the historical names used by the embedded
// self-tests, but are now thin wrappers over goTiff. No TIFF IFD, strip or
// tag bytes are constructed in this package any more.

func writeFloat32RGB(w io.Writer, width, height int, pix []float32) error {
	return writeFloat32RGBScaled(w, width, height, pix, 1)
}

func writeFloat32RGBScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("tiff: invalid dimensions %dx%d", width, height)
	}
	if len(pix) != width*height*3 {
		return fmt.Errorf("tiff: pixel buffer length %d does not match %dx%dx3 = %d", len(pix), width, height, width*height*3)
	}
	scaled, err := scaleTIFFPixels(pix, divisor)
	if err != nil {
		return err
	}
	img := newTIFFImage(width, height, scaled, tiffSampleConfig{compression: gotiff.CompressionNone, bits: 32, format: gotiff.SampleFloat})
	return gotiff.Encode(w, img, &gotiff.Options{})
}

func writeFloat32RGBBigTIFF(w io.Writer, width, height int, pix []float32) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("bigtiff: invalid dimensions %dx%d", width, height)
	}
	if len(pix) != width*height*3 {
		return fmt.Errorf("bigtiff: pixel buffer length %d does not match %dx%dx3 = %d", len(pix), width, height, width*height*3)
	}
	img := newTIFFImage(width, height, pix, tiffSampleConfig{compression: gotiff.CompressionNone, bits: 32, format: gotiff.SampleFloat})
	return gotiff.Encode(w, img, &gotiff.Options{BigTIFF: true})
}

func decodeFloat32RGBImage(img *gotiff.Image) (width, height int, pix []float32, err error) {
	if img.Channels != 3 || img.Bits != 32 || img.Format != gotiff.SampleFloat || img.Photometric != gotiff.PhotoRGB {
		return 0, 0, nil, fmt.Errorf("tiff: unsupported float32 RGB layout (ch=%d bits=%d format=%d photo=%d)",
			img.Channels, img.Bits, img.Format, img.Photometric)
	}
	return img.Width, img.Height, img.Pixels, nil
}

func readFloat32RGB(r io.Reader) (width, height int, pix []float32, err error) {
	img, err := gotiff.Decode(r)
	if err != nil {
		return 0, 0, nil, err
	}
	return decodeFloat32RGBImage(img)
}

func readFloat32RGBBigTIFF(path string) (width, height int, pix []float32, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()
	hdr := make([]byte, 8)
	if _, err = io.ReadFull(f, hdr); err != nil {
		return 0, 0, nil, err
	}
	if string(hdr[:2]) != "II" || binary.LittleEndian.Uint16(hdr[2:4]) != 43 || binary.LittleEndian.Uint16(hdr[4:6]) != 8 {
		return 0, 0, nil, fmt.Errorf("bigtiff: %s: bad header", path)
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return 0, 0, nil, err
	}
	return readFloat32RGB(f)
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

// parseAstroTIFFDescription extracts a TAN WCS and the BAYERPAT from the
// FITS-style card text that astroTIFFDescription writes into TIFF tag 270.
func parseAstroTIFFDescription(desc string) (*wcsTan, string) {
	if desc == "" {
		return nil, ""
	}
	cards := make(map[string]string)
	for _, line := range strings.Split(desc, "\n") {
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), "' ")
		cards[key] = val
	}
	var wcs *wcsTan
	if strings.Contains(cards["CTYPE1"], "RA---TAN") {
		parseFloatCard := func(key string) (float64, bool) {
			v, ok := cards[key]
			if !ok {
				return 0, false
			}
			f, e := strconv.ParseFloat(strings.Trim(strings.TrimSpace(v), "' "), 64)
			return f, e == nil
		}
		vals := make([]float64, 8)
		keys := []string{"CRVAL1", "CRVAL2", "CRPIX1", "CRPIX2", "CD1_1", "CD1_2", "CD2_1", "CD2_2"}
		ok := true
		for i, k := range keys {
			vals[i], ok = parseFloatCard(k)
			if !ok {
				break
			}
		}
		if ok {
			wcs = &wcsTan{
				CRVAL1: vals[0] * math.Pi / 180,
				CRVAL2: vals[1] * math.Pi / 180,
				CRPIX1: vals[2],
				CRPIX2: vals[3],
				CD: [2][2]float64{
					{vals[4] * math.Pi / 180, vals[5] * math.Pi / 180},
					{vals[6] * math.Pi / 180, vals[7] * math.Pi / 180},
				},
			}
		}
	}
	return wcs, strings.TrimSpace(cards["BAYERPAT"])
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
		// Astro-TIFF stores BAYERPAT in tag 270; read it through goTiff.
		if ext := strings.ToLower(filepath.Ext(path)); ext == ".tif" || ext == ".tiff" || ext == ".btf" || ext == ".bigtiff" {
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			img, err := gotiff.Decode(f)
			_ = f.Close()
			if err != nil {
				continue
			}
			if v := strings.TrimSpace(img.CFAPattern); v != "" {
				return v
			}
			if _, bayer := parseAstroTIFFDescription(img.Metadata["ImageDescription"]); bayer != "" {
				return bayer
			}
		}
	}
	return ""
}

// SkyStacker v0.2.0 — single-file monolithic build.
//
// Combine JPG/JPEG astrophotography frames into sum, mean and
// no-correction-mean images with matching previews and a text report.
//
// Build:  go build -o skystacker main.go
// Test:   ./skystacker --selftest
// Run:    ./skystacker [flags] [input-directory]
//
// v0.2 change: stacked_sum.tiff is now normalised by its global
// maximum so ordinary image viewers display it (previously the raw sum
// values could exceed 1.0, which most viewers clamp to white). The
// scale factor is written to report.txt; raw_sum = tiff_value * max.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/jpeg"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const version = "0.2.0"

// =========================================================================
// CLI
// =========================================================================

type cliOptions struct {
	InputDir  string
	OutputDir string
	Workers   int
	Recursive bool
	Overwrite bool
	Version   bool
	Help      bool
	SelfTest  bool
}

func parseArgs(args []string) (*cliOptions, error) {
	fs := flag.NewFlagSet("skystacker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	o := &cliOptions{}
	fs.StringVar(&o.OutputDir, "output", "", "output directory (default: <input>/skystacker_results)")
	fs.IntVar(&o.Workers, "workers", runtime.NumCPU(), "maximum concurrent workers")
	fs.BoolVar(&o.Recursive, "recursive", false, "search subdirectories")
	fs.BoolVar(&o.Overwrite, "overwrite", false, "permit replacing files in the output directory")
	fs.BoolVar(&o.Version, "version", false, "print version and exit")
	fs.BoolVar(&o.SelfTest, "selftest", false, "run internal self-tests and exit")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			o.Help = true
			return o, nil
		}
		return nil, err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return nil, fmt.Errorf("unexpected arguments: %v", rest[1:])
	}
	if len(rest) == 1 {
		o.InputDir = rest[0]
	} else {
		o.InputDir = "."
	}
	if o.Workers < 1 {
		return nil, fmt.Errorf("--workers must be >= 1, got %d", o.Workers)
	}
	return o, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: skystacker [flags] [input-directory]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Combine JPG/JPEG astrophotography frames into sum, mean and")
	fmt.Fprintln(w, "no-correction-mean images. No star registration or calibration")
	fmt.Fprintln(w, "is performed.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  input-directory   directory containing JPG/JPEG files (default: .)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --output string   output directory (default: <input>/skystacker_results)")
	fmt.Fprintln(w, "  --workers int     maximum concurrent workers (default: NumCPU)")
	fmt.Fprintln(w, "  --recursive       search subdirectories")
	fmt.Fprintln(w, "  --overwrite       permit replacing files in the output directory")
	fmt.Fprintln(w, "  --selftest        run internal self-tests and exit")
	fmt.Fprintln(w, "  --version         print version and exit")
	fmt.Fprintln(w, "  --help            print help")
}

// =========================================================================
// Discovery
// =========================================================================

func discoverFiles(root string, recursive bool, exclude []string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	excludes := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		abs, err := filepath.Abs(e)
		if err != nil {
			return nil, err
		}
		excludes[abs] = true
	}

	var files []string
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if excludes[abs] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != absRoot && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" {
			files = append(files, abs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// =========================================================================
// Imaging
// =========================================================================

// frame holds interleaved float32 RGB samples in [0, 1].
type frame struct {
	Width  int
	Height int
	Pix    []float32
}

func decodeImageConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg, err
}

func decodeImageFile(path string) (*frame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return frameFromImage(img)
}

func frameFromImage(img image.Image) (*frame, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", w, h)
	}
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	switch src := img.(type) {
	case *image.YCbCr:
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
	default:
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
	return f, nil
}

// =========================================================================
// Stack
// =========================================================================

type accumulator struct {
	Width, Height int
	Pix           []float32
	Count         int
}

func newAccumulator(w, h int) *accumulator {
	return &accumulator{Width: w, Height: h, Pix: make([]float32, w*h*3)}
}

func (a *accumulator) addFrame(f *frame, workers int) error {
	if f.Width != a.Width || f.Height != a.Height {
		return fmt.Errorf("frame dimensions %dx%d do not match accumulator %dx%d",
			f.Width, f.Height, a.Width, a.Height)
	}
	if len(f.Pix) != len(a.Pix) {
		return fmt.Errorf("pixel buffer length %d does not match accumulator %d",
			len(f.Pix), len(a.Pix))
	}
	if workers <= 1 || a.Height < workers {
		for i, v := range f.Pix {
			a.Pix[i] += v
		}
		a.Count++
		return nil
	}
	rowsPerWorker := (a.Height + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		y0 := w * rowsPerWorker
		if y0 >= a.Height {
			break
		}
		y1 := y0 + rowsPerWorker
		if y1 > a.Height {
			y1 = a.Height
		}
		start := y0 * a.Width * 3
		end := y1 * a.Width * 3
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				a.Pix[i] += f.Pix[i]
			}
		}(start, end)
	}
	wg.Wait()
	a.Count++
	return nil
}

func (a *accumulator) divide() {
	if a.Count == 0 {
		return
	}
	inv := 1.0 / float32(a.Count)
	for i := range a.Pix {
		a.Pix[i] *= inv
	}
}

func (a *accumulator) maxValue() float32 {
	m := float32(0)
	for _, v := range a.Pix {
		if v > m {
			m = v
		}
	}
	return m
}

// =========================================================================
// Output — TIFF
// =========================================================================

const (
	tagImageWidth        = 256
	tagImageLength       = 257
	tagBitsPerSample     = 258
	tagCompression       = 259
	tagPhotometricInterp = 262
	tagStripOffsets      = 273
	tagSamplesPerPixel   = 277
	tagRowsPerStrip      = 278
	tagStripByteCounts   = 279
	tagXResolution       = 282
	tagYResolution       = 283
	tagPlanarConfig      = 284
	tagResolutionUnit    = 296
	tagSampleFormat      = 339

	typeShort    = 3
	typeLong     = 4
	typeRational = 5

	compressionNone    = 1
	photometricRGB     = 2
	planarChunky       = 1
	sampleFormatFloat  = 3
	resolutionUnitNone = 1

	ifdEntryCount = 14
)

// writeFloat32RGB writes pix unchanged.
func writeFloat32RGB(w io.Writer, width, height int, pix []float32) error {
	return writeFloat32RGBScaled(w, width, height, pix, 1.0)
}

// writeFloat32RGBScaled writes each sample divided by divisor. Values are
// not clipped; the caller is responsible for choosing a divisor that puts
// the output in [0, 1] for viewers.
func writeFloat32RGBScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("tiff: invalid dimensions %dx%d", width, height)
	}
	n := width * height * 3
	if len(pix) != n {
		return fmt.Errorf("tiff: pixel buffer length %d does not match %dx%dx3 = %d",
			len(pix), width, height, n)
	}
	if divisor == 0 {
		return errors.New("tiff: divisor must be non-zero")
	}
	stripBytes64 := int64(n) * 4
	if stripBytes64 > math.MaxUint32 {
		return fmt.Errorf("tiff: image too large for single strip (%d bytes)", stripBytes64)
	}
	stripBytes := uint32(stripBytes64)

	ifdSize := uint32(2 + ifdEntryCount*12 + 4)
	ifdOffset := uint32(8)
	auxOffset := ifdOffset + ifdSize

	bitsOffset := auxOffset
	sfOffset := bitsOffset + 6
	xResOffset := sfOffset + 6
	yResOffset := xResOffset + 8
	auxEnd := yResOffset + 8

	pixelOffset := auxEnd
	if rem := pixelOffset % 4; rem != 0 {
		pixelOffset += 4 - rem
	}

	var header [8]byte
	header[0], header[1] = 'I', 'I'
	binary.LittleEndian.PutUint16(header[2:], 42)
	binary.LittleEndian.PutUint32(header[4:], ifdOffset)
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	ifd := make([]byte, ifdSize)
	binary.LittleEndian.PutUint16(ifd[0:], ifdEntryCount)
	pos := 2
	putShort := func(tag, v uint16) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typeShort)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint16(ifd[pos+8:], v)
		pos += 12
	}
	putLong := func(tag uint16, v uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typeLong)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint32(ifd[pos+8:], v)
		pos += 12
	}
	putOffset := func(tag, typ uint16, count, offset uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typ)
		binary.LittleEndian.PutUint32(ifd[pos+4:], count)
		binary.LittleEndian.PutUint32(ifd[pos+8:], offset)
		pos += 12
	}

	putLong(tagImageWidth, uint32(width))
	putLong(tagImageLength, uint32(height))
	putOffset(tagBitsPerSample, typeShort, 3, bitsOffset)
	putShort(tagCompression, compressionNone)
	putShort(tagPhotometricInterp, photometricRGB)
	putLong(tagStripOffsets, pixelOffset)
	putShort(tagSamplesPerPixel, 3)
	putLong(tagRowsPerStrip, uint32(height))
	putLong(tagStripByteCounts, stripBytes)
	putOffset(tagXResolution, typeRational, 1, xResOffset)
	putOffset(tagYResolution, typeRational, 1, yResOffset)
	putShort(tagPlanarConfig, planarChunky)
	putShort(tagResolutionUnit, resolutionUnitNone)
	putOffset(tagSampleFormat, typeShort, 3, sfOffset)

	binary.LittleEndian.PutUint32(ifd[pos:], 0)
	if _, err := w.Write(ifd); err != nil {
		return err
	}

	aux := make([]byte, int(auxEnd-auxOffset))
	bo := int(bitsOffset - auxOffset)
	sfo := int(sfOffset - auxOffset)
	xro := int(xResOffset - auxOffset)
	yro := int(yResOffset - auxOffset)
	binary.LittleEndian.PutUint16(aux[bo:], 32)
	binary.LittleEndian.PutUint16(aux[bo+2:], 32)
	binary.LittleEndian.PutUint16(aux[bo+4:], 32)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint16(aux[sfo+i*2:], sampleFormatFloat)
	}
	binary.LittleEndian.PutUint32(aux[xro:], 1)
	binary.LittleEndian.PutUint32(aux[xro+4:], 1)
	binary.LittleEndian.PutUint32(aux[yro:], 1)
	binary.LittleEndian.PutUint32(aux[yro+4:], 1)
	if _, err := w.Write(aux); err != nil {
		return err
	}
	if pad := int(pixelOffset) - int(auxEnd); pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}

	const bufFloats = 4096
	scratch := make([]byte, bufFloats*4)
	inv := float32(1.0) / divisor
	for i := 0; i < n; {
		count := n - i
		if count > bufFloats {
			count = bufFloats
		}
		for j := 0; j < count; j++ {
			binary.LittleEndian.PutUint32(scratch[j*4:], math.Float32bits(pix[i+j]*inv))
		}
		if _, err := w.Write(scratch[:count*4]); err != nil {
			return err
		}
		i += count
	}
	return nil
}

// readFloat32RGB is a minimal TIFF reader for self-tests and diagnostics.
func readFloat32RGB(r io.Reader) (width, height int, pix []float32, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(data) < 8 || data[0] != 'I' || data[1] != 'I' {
		return 0, 0, nil, errors.New("tiff: not a little-endian TIFF")
	}
	if v := binary.LittleEndian.Uint16(data[2:]); v != 42 {
		return 0, 0, nil, fmt.Errorf("tiff: unexpected version %d", v)
	}
	ifdOff := int(binary.LittleEndian.Uint32(data[4:]))
	if ifdOff+2 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD offset out of range")
	}
	nEntries := int(binary.LittleEndian.Uint16(data[ifdOff:]))
	if ifdOff+2+nEntries*12+4 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD truncated")
	}

	var (
		w, h     uint32
		bps, sf  []uint16
		spp      uint16
		photo    uint16
		planar   uint16
		stripOff []uint32
		stripLen []uint32
	)

	for i := 0; i < nEntries; i++ {
		off := ifdOff + 2 + i*12
		tag := binary.LittleEndian.Uint16(data[off:])
		typ := binary.LittleEndian.Uint16(data[off+2:])
		count := int(binary.LittleEndian.Uint32(data[off+4:]))
		valField := off + 8

		readShorts := func() ([]uint16, error) {
			if typ != typeShort {
				return nil, fmt.Errorf("tiff: tag %d: expected SHORT, got type %d", tag, typ)
			}
			out := make([]uint16, count)
			if count*2 <= 4 {
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[valField+k*2:])
				}
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o+count*2 > len(data) {
					return nil, fmt.Errorf("tiff: tag %d: value out of range", tag)
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[o+k*2:])
				}
			}
			return out, nil
		}
		readLongs := func() ([]uint32, error) {
			if typ != typeLong {
				return nil, fmt.Errorf("tiff: tag %d: expected LONG, got type %d", tag, typ)
			}
			out := make([]uint32, count)
			if count*4 <= 4 {
				out[0] = binary.LittleEndian.Uint32(data[valField:])
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o+count*4 > len(data) {
					return nil, fmt.Errorf("tiff: tag %d: value out of range", tag)
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint32(data[o+k*4:])
				}
			}
			return out, nil
		}

		switch tag {
		case tagImageWidth:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			w = v[0]
		case tagImageLength:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			h = v[0]
		case tagBitsPerSample:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			bps = v
		case tagPhotometricInterp:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			photo = v[0]
		case tagStripOffsets:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripOff = v
		case tagSamplesPerPixel:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			spp = v[0]
		case tagStripByteCounts:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripLen = v
		case tagPlanarConfig:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			planar = v[0]
		case tagSampleFormat:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			sf = v
		}
	}

	if w == 0 || h == 0 || spp != 3 || photo != photometricRGB || planar != planarChunky {
		return 0, 0, nil, errors.New("tiff: unexpected header fields")
	}
	if len(bps) != 3 || bps[0] != 32 || len(sf) != 3 || sf[0] != sampleFormatFloat {
		return 0, 0, nil, errors.New("tiff: unexpected sample format")
	}
	if len(stripOff) != 1 || len(stripLen) != 1 {
		return 0, 0, nil, errors.New("tiff: expected a single strip")
	}
	nPix := int(w) * int(h) * 3
	if int(stripLen[0]) != nPix*4 {
		return 0, 0, nil, fmt.Errorf("tiff: strip byte count %d, expected %d", stripLen[0], nPix*4)
	}
	off := int(stripOff[0])
	if off+nPix*4 > len(data) {
		return 0, 0, nil, errors.New("tiff: pixel data out of range")
	}
	pix = make([]float32, nPix)
	for i := range pix {
		pix[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*4:]))
	}
	return int(w), int(h), pix, nil
}

// =========================================================================
// Output — preview JPEG
// =========================================================================

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
		} else if x > 255 {
			x = 255
		}
		lut[i] = uint8(x)
	}
	return lut
}

func writePreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	return writePreviewJPEGScaled(w, width, height, pix, 1.0)
}

func writePreviewJPEGScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if divisor == 0 {
		divisor = 1.0
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	inv := float32(1.0) / divisor
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

func writeAutoNormalizedPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	maxVal := float32(0)
	for _, v := range pix {
		if v > maxVal {
			maxVal = v
		}
	}
	if math.IsNaN(float64(maxVal)) || math.IsInf(float64(maxVal), 0) || maxVal <= 0 {
		maxVal = 1.0
	}
	return writePreviewJPEGScaled(w, width, height, pix, maxVal)
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

// =========================================================================
// Report
// =========================================================================

type rejection struct {
	Path   string
	Reason string
}

type processingReport struct {
	Version         string
	Timestamp       time.Time
	InputDir        string
	OutputDir       string
	Discovered      int
	Accepted        []string
	Rejected        []rejection
	Width           int
	Height          int
	Channels        int
	Method          string
	AccumulatorType string
	OutputFormat    string
	OutputScaling   string
	SumNormalization float32
	OutputFiles     []string
	Workers         int
	Duration        time.Duration
	Notes           []string
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

func (r *processingReport) writeTo(w io.Writer) error {
	ew := &errWriter{w: w}
	ew.printf("SkyStacker %s\n", r.Version)
	ew.printf("Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))
	ew.printf("Input directory:  %s\n", r.InputDir)
	ew.printf("Output directory: %s\n\n", r.OutputDir)
	ew.printf("Discovered files: %d\n", r.Discovered)
	ew.printf("Accepted:         %d\n", len(r.Accepted))
	ew.printf("Rejected:         %d\n\n", len(r.Rejected))
	ew.printf("Image dimensions: %d x %d\n", r.Width, r.Height)
	ew.printf("Channels:         %d\n\n", r.Channels)
	ew.printf("Method:           %s\n", r.Method)
	ew.printf("Accumulator:      %s\n", r.AccumulatorType)
	ew.printf("Output format:    %s\n", r.OutputFormat)
	ew.printf("Output scaling:   %s\n", r.OutputScaling)
	ew.printf("Workers:          %d\n", r.Workers)
	ew.printf("Duration:         %s\n\n", r.Duration.Round(time.Millisecond))

	ew.printf("Sum TIFF normalisation: %g\n\n", r.SumNormalization)

	ew.printf("Output files:\n")
	for _, f := range r.OutputFiles {
		ew.printf("  %s\n", f)
	}
	ew.printf("\n")

	ew.printf("Accepted files:\n")
	for _, p := range r.Accepted {
		ew.printf("  %s\n", p)
	}
	ew.printf("\n")
	if len(r.Rejected) > 0 {
		ew.printf("Rejected files:\n")
		for _, rj := range r.Rejected {
			ew.printf("  %s: %s\n", rj.Path, rj.Reason)
		}
		ew.printf("\n")
	}
	ew.printf("Notes:\n")
	for _, n := range r.Notes {
		ew.printf("  - %s\n", n)
	}
	return ew.err
}

// =========================================================================
// Atomic file write
// =========================================================================

func writeFileAtomic(path string, fn func(io.Writer) error) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	if err := fn(bw); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// =========================================================================
// Pipeline
// =========================================================================

type runOptions struct {
	InputDir  string
	OutputDir string
	Workers   int
	Recursive bool
	Overwrite bool
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func runPipeline(ctx context.Context, opts runOptions) error {
	start := time.Now()

	inputDir, err := filepath.Abs(opts.InputDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(inputDir)
	if err != nil {
		return fmt.Errorf("input directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", inputDir)
	}

	var outputDir string
	if opts.OutputDir == "" {
		outputDir = filepath.Join(inputDir, "skystacker_results")
	} else {
		outputDir, err = filepath.Abs(opts.OutputDir)
		if err != nil {
			return err
		}
	}
	if filepath.Clean(outputDir) == filepath.Clean(inputDir) {
		return fmt.Errorf("output directory must differ from input directory")
	}

	if info, err := os.Stat(outputDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output path exists and is not a directory: %s", outputDir)
		}
		if !opts.Overwrite {
			return fmt.Errorf("output directory already exists: %s (use --overwrite to replace)", outputDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("output directory: %w", err)
	}

	files, err := discoverFiles(inputDir, opts.Recursive, []string{outputDir})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no JPG/JPEG images found in %s", inputDir)
	}

	type candidate struct{ path string }
	var (
		candidates []candidate
		rejected   []rejection
		refW, refH int
	)
	for _, path := range files {
		cfg, err := decodeImageConfig(path)
		if err != nil {
			rejected = append(rejected, rejection{Path: path, Reason: err.Error()})
			continue
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			rejected = append(rejected, rejection{
				Path:   path,
				Reason: fmt.Sprintf("invalid dimensions %dx%d", cfg.Width, cfg.Height),
			})
			continue
		}
		if refW == 0 {
			refW, refH = cfg.Width, cfg.Height
		} else if cfg.Width != refW || cfg.Height != refH {
			rejected = append(rejected, rejection{
				Path: path,
				Reason: fmt.Sprintf("dimensions %dx%d differ from reference %dx%d",
					cfg.Width, cfg.Height, refW, refH),
			})
			continue
		}
		candidates = append(candidates, candidate{path: path})
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no valid images (all %d candidates rejected)", len(files))
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(candidates) {
		workers = len(candidates)
	}
	if workers < 1 {
		workers = 1
	}

	logf("Stacking %d frame(s) at %dx%d using up to %d worker(s)",
		len(candidates), refW, refH, workers)

	acc := newAccumulator(refW, refH)
	var accepted []string
	for i, c := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		logf("  [%d/%d] %s", i+1, len(candidates), filepath.Base(c.path))
		f, err := decodeImageFile(c.path)
		if err != nil {
			rejected = append(rejected, rejection{
				Path:   c.path,
				Reason: "decode failed after validation: " + err.Error(),
			})
			continue
		}
		if err := acc.addFrame(f, workers); err != nil {
			return fmt.Errorf("accumulate %s: %w", c.path, err)
		}
		accepted = append(accepted, c.path)
	}
	if acc.Count == 0 {
		return fmt.Errorf("no frames accumulated")
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	sumTiff := filepath.Join(outputDir, "stacked_sum.tiff")
	sumJPG := filepath.Join(outputDir, "stacked_sum.jpg")
	meanTiff := filepath.Join(outputDir, "stacked_mean.tiff")
	meanJPG := filepath.Join(outputDir, "stacked_mean.jpg")
	noCorrTiff := filepath.Join(outputDir, "stacked_no_correction_mean.tiff")
	noCorrJPG := filepath.Join(outputDir, "stacked_no_correction_mean.jpg")
	previewJPG := filepath.Join(outputDir, "stacked_preview.jpg")
	reportTxt := filepath.Join(outputDir, "report.txt")

	// Compute the sum's global max once. This is used to normalise the
	// sum TIFF so that ordinary viewers display it (values <= 1.0).
	sumMax := acc.maxValue()
	if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) {
		sumMax = 1.0
	}
	divisor := float32(1.0)
	if sumMax > 1.0 {
		divisor = sumMax
	}
	logf("Sum max = %g; writing sum TIFF normalised by %g", sumMax, divisor)

	// --- Sum outputs (buffer still holds the raw sum) ---
	logf("Writing sum image (TIFF)")
	if err := writeFileAtomic(sumTiff, func(w io.Writer) error {
		return writeFloat32RGBScaled(w, refW, refH, acc.Pix, divisor)
	}); err != nil {
		return fmt.Errorf("write sum tiff: %w", err)
	}
	logf("Writing sum preview (JPG)")
	if err := writeFileAtomic(sumJPG, func(w io.Writer) error {
		return writeAutoNormalizedPreviewJPEG(w, refW, refH, acc.Pix)
	}); err != nil {
		return fmt.Errorf("write sum jpg: %w", err)
	}

	// --- Convert sum to mean in place ---
	acc.divide()

	// --- Mean outputs ---
	logf("Writing mean image (TIFF)")
	if err := writeFileAtomic(meanTiff, func(w io.Writer) error {
		return writeFloat32RGB(w, refW, refH, acc.Pix)
	}); err != nil {
		return fmt.Errorf("write mean tiff: %w", err)
	}
	logf("Writing mean preview (JPG)")
	if err := writeFileAtomic(meanJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, acc.Pix)
	}); err != nil {
		return fmt.Errorf("write mean jpg: %w", err)
	}

	// --- No-correction mean (identical to mean in v0.2) ---
	logf("Writing no-correction mean image (TIFF)")
	if err := writeFileAtomic(noCorrTiff, func(w io.Writer) error {
		return writeFloat32RGB(w, refW, refH, acc.Pix)
	}); err != nil {
		return fmt.Errorf("write no_correction_mean tiff: %w", err)
	}
	logf("Writing no-correction mean preview (JPG)")
	if err := writeFileAtomic(noCorrJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, acc.Pix)
	}); err != nil {
		return fmt.Errorf("write no_correction_mean jpg: %w", err)
	}

	// --- Legacy stacked_preview.jpg ---
	logf("Writing legacy preview (JPG)")
	if err := writeFileAtomic(previewJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, acc.Pix)
	}); err != nil {
		return fmt.Errorf("write preview: %w", err)
	}

	rep := &processingReport{
		Version:          version,
		Timestamp:        start,
		InputDir:         inputDir,
		OutputDir:        outputDir,
		Discovered:       len(files),
		Accepted:         accepted,
		Rejected:         rejected,
		Width:            refW,
		Height:           refH,
		Channels:         3,
		Method:           "sum and arithmetic mean",
		AccumulatorType:  "float32 (interleaved RGB)",
		OutputFormat:     "TIFF: uncompressed, little-endian, RGB, IEEE float32, single strip. JPG: 8-bit, gamma 0.5 display preview.",
		OutputScaling:    "sum.tiff: raw sum divided by the reported normalisation factor so values lie in [0,1] for viewers; mean.tiff / no_correction_mean.tiff: sum / N, range [0,1]; sum.jpg: auto-normalised by global max; mean.jpg / no_correction_mean.jpg / preview.jpg: gamma 0.5 of values in [0,1]. No numeric clipping in TIFF outputs.",
		SumNormalization: divisor,
		OutputFiles: []string{
			sumTiff, sumJPG,
			meanTiff, meanJPG,
			noCorrTiff, noCorrJPG,
			previewJPG,
			reportTxt,
		},
		Workers:  workers,
		Duration: time.Since(start),
		Notes: []string{
			"No star registration or calibration was performed (v0.2).",
			"Frames were combined in their existing pixel coordinate system.",
			fmt.Sprintf("stacked_sum.tiff was divided by %g (its global maximum) so that viewers display it; to recover the raw sum, multiply pixel values by %g.", divisor, divisor),
			"stacked_no_correction_mean.tiff is byte-for-byte identical to stacked_mean.tiff in v0.2 because no registration is performed. It is produced so that future registered means can be compared directly against the uncorrected stack.",
			"stacked_mean.jpg, stacked_no_correction_mean.jpg and stacked_preview.jpg are identical in v0.2.",
			"JPEG inputs are gamma-encoded (sRGB-like); samples were averaged in encoded space, not linear light.",
			"All JPG outputs are display previews, not scientific outputs.",
		},
	}
	if err := writeFileAtomic(reportTxt, func(w io.Writer) error {
		return rep.writeTo(w)
	}); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	logf("Done in %s -> %s", rep.Duration.Round(time.Millisecond), outputDir)
	return nil
}

// =========================================================================
// Self-tests (invoked via --selftest)
// =========================================================================

type testCase struct {
	name string
	fn   func() error
}

func runSelfTests() int {
	cases := []testCase{
		{"accumulator sum and mean", testAccumulatorSumMean},
		{"accumulator overflow", testAccumulatorOverflow},
		{"accumulator dimension mismatch", testAccumulatorDimensionMismatch},
		{"accumulator parallel vs sequential", testAccumulatorParallel},
		{"accumulator max", testAccumulatorMax},
		{"TIFF round trip", testTIFFRoundTrip},
		{"TIFF large values", testTIFFLargeValues},
		{"TIFF scaled round trip", testTIFFScaledRoundTrip},
		{"TIFF bad input", testTIFFBadInput},
		{"decode RGBA", testDecodeRGBA},
		{"decode JPEG", testDecodeJPEG},
		{"discovery", testDiscovery},
	}
	failures := 0
	for _, c := range cases {
		if err := c.fn(); err != nil {
			fmt.Printf("FAIL %s: %v\n", c.name, err)
			failures++
		} else {
			fmt.Printf("ok   %s\n", c.name)
		}
	}
	if failures > 0 {
		fmt.Printf("\n%d of %d self-tests failed\n", failures, len(cases))
	} else {
		fmt.Printf("\nall %d self-tests passed\n", len(cases))
	}
	return failures
}

func testAccumulatorSumMean() error {
	a := newAccumulator(2, 2)
	f1 := &frame{Width: 2, Height: 2, Pix: []float32{
		0.1, 0.2, 0.3, 0.4, 0.5, 0.6,
		0.7, 0.8, 0.9, 1.0, 0.0, 0.5,
	}}
	f2 := &frame{Width: 2, Height: 2, Pix: []float32{
		0.9, 0.8, 0.7, 0.6, 0.5, 0.4,
		0.3, 0.2, 0.1, 0.0, 1.0, 0.5,
	}}
	if err := a.addFrame(f1, 1); err != nil {
		return err
	}
	if err := a.addFrame(f2, 1); err != nil {
		return err
	}
	for i, v := range a.Pix {
		if math.Abs(float64(v-1.0)) > 1e-6 {
			return fmt.Errorf("sum[%d] = %v, want 1", i, v)
		}
	}
	a.divide()
	for i, v := range a.Pix {
		if math.Abs(float64(v-0.5)) > 1e-6 {
			return fmt.Errorf("mean[%d] = %v, want 0.5", i, v)
		}
	}
	return nil
}

func testAccumulatorOverflow() error {
	a := newAccumulator(1, 1)
	f := &frame{Width: 1, Height: 1, Pix: []float32{1, 1, 1}}
	for i := 0; i < 300; i++ {
		if err := a.addFrame(f, 1); err != nil {
			return err
		}
	}
	if a.Pix[0] != 300 {
		return fmt.Errorf("sum = %v, want 300", a.Pix[0])
	}
	a.divide()
	if math.Abs(float64(a.Pix[0]-1)) > 1e-6 {
		return fmt.Errorf("mean = %v, want 1", a.Pix[0])
	}
	return nil
}

func testAccumulatorDimensionMismatch() error {
	a := newAccumulator(2, 2)
	bad := &frame{Width: 3, Height: 2, Pix: make([]float32, 3*2*3)}
	if err := a.addFrame(bad, 1); err == nil {
		return errors.New("expected dimension-mismatch error")
	}
	return nil
}

func testAccumulatorParallel() error {
	a := newAccumulator(64, 32)
	f := &frame{Width: 64, Height: 32, Pix: make([]float32, 64*32*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i%97) / 97.0
	}
	if err := a.addFrame(f, 8); err != nil {
		return err
	}
	b := newAccumulator(64, 32)
	if err := b.addFrame(f, 1); err != nil {
		return err
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			return fmt.Errorf("mismatch at %d: %v vs %v", i, a.Pix[i], b.Pix[i])
		}
	}
	return nil
}

func testAccumulatorMax() error {
	a := newAccumulator(2, 1)
	a.Pix = []float32{0.1, 0.5, 3.2, 1.1, 0.9, 2.7}
	got := a.maxValue()
	if got != 3.2 {
		return fmt.Errorf("max = %v, want 3.2", got)
	}
	return nil
}

func testTIFFRoundTrip() error {
	const w, h = 3, 2
	pix := []float32{
		0, 0.25, 0.5,
		1, 0.5, 0.75,
		0.125, 0.375, 0.625,
		0.1, 0.2, 0.3,
		0, 0, 1,
		0.99, 0.5, 0.01,
	}
	var buf bytes.Buffer
	if err := writeFloat32RGB(&buf, w, h, pix); err != nil {
		return err
	}
	gotW, gotH, gotPix, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	if gotW != w || gotH != h {
		return fmt.Errorf("dims = %dx%d, want %dx%d", gotW, gotH, w, h)
	}
	for i := range pix {
		if gotPix[i] != pix[i] {
			return fmt.Errorf("pixel[%d] = %v, want %v", i, gotPix[i], pix[i])
		}
	}
	return nil
}

func testTIFFLargeValues() error {
	var buf bytes.Buffer
	pix := []float32{1000000, 999999.5, 3.4e38}
	if err := writeFloat32RGB(&buf, 1, 1, pix); err != nil {
		return err
	}
	_, _, got, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	for i := range pix {
		if math.Abs(float64(got[i]-pix[i]))/math.Abs(float64(pix[i])) > 1e-6 {
			return fmt.Errorf("pixel[%d] = %v, want %v", i, got[i], pix[i])
		}
	}
	return nil
}

func testTIFFScaledRoundTrip() error {
	var buf bytes.Buffer
	pix := []float32{10, 20, 30, 40, 50, 60}
	if err := writeFloat32RGBScaled(&buf, 2, 1, pix, 10); err != nil {
		return err
	}
	_, _, got, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	want := []float32{1, 2, 3, 4, 5, 6}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1e-6 {
			return fmt.Errorf("scaled pixel[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	return nil
}

func testTIFFBadInput() error {
	if err := writeFloat32RGB(&bytes.Buffer{}, 0, 0, nil); err == nil {
		return errors.New("expected error for zero dimensions")
	}
	if err := writeFloat32RGB(&bytes.Buffer{}, 2, 2, make([]float32, 11)); err == nil {
		return errors.New("expected error for mismatched buffer length")
	}
	if err := writeFloat32RGBScaled(&bytes.Buffer{}, 2, 2, make([]float32, 12), 0); err == nil {
		return errors.New("expected error for zero divisor")
	}
	return nil
}

func testDecodeRGBA() error {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	f, err := frameFromImage(img)
	if err != nil {
		return err
	}
	checks := []struct {
		x, y, c int
		want    float32
	}{
		{0, 0, 0, 1}, {0, 0, 1, 0}, {0, 0, 2, 0},
		{1, 0, 0, 0}, {1, 0, 1, 1}, {1, 0, 2, 0},
	}
	for _, ck := range checks {
		got := f.Pix[(ck.y*f.Width+ck.x)*3+ck.c]
		if math.Abs(float64(got-ck.want)) > 1e-6 {
			return fmt.Errorf("pixel (%d,%d,%d) = %v, want %v", ck.x, ck.y, ck.c, got, ck.want)
		}
	}
	return nil
}

func testDecodeJPEG() error {
	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			src.SetRGBA(x, y, color.RGBA{200, 100, 50, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 95}); err != nil {
		return err
	}
	img, _, err := image.Decode(&buf)
	if err != nil {
		return err
	}
	f, err := frameFromImage(img)
	if err != nil {
		return err
	}
	want := [3]float32{200.0 / 255.0, 100.0 / 255.0, 50.0 / 255.0}
	for i := 0; i < 4*3; i++ {
		for c := 0; c < 3; c++ {
			if math.Abs(float64(f.Pix[i*3+c]-want[c])) > 0.02 {
				return fmt.Errorf("pixel %d channel %d = %v, want ~%v", i, c, f.Pix[i*3+c], want[c])
			}
		}
	}
	return nil
}

func testDiscovery() error {
	dir, err := os.MkdirTemp("", "skystacker-discovery-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	must := func(path string) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("x"), 0o644)
	}
	for _, p := range []string{
		filepath.Join(dir, "b.JPG"),
		filepath.Join(dir, "a.jpeg"),
		filepath.Join(dir, "c.png"),
		filepath.Join(dir, "sub", "d.jpg"),
	} {
		if err := must(p); err != nil {
			return err
		}
	}

	got, err := discoverFiles(dir, false, nil)
	if err != nil {
		return err
	}
	want := []string{filepath.Join(dir, "a.jpeg"), filepath.Join(dir, "b.JPG")}
	if len(got) != len(want) {
		return fmt.Errorf("non-recursive: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Errorf("non-recursive[%d]: got %v, want %v", i, got[i], want[i])
		}
	}

	got, err = discoverFiles(dir, true, []string{filepath.Join(dir, "sub")})
	if err != nil {
		return err
	}
	if len(got) != 2 {
		return fmt.Errorf("recursive+exclude: got %v, want 2 entries", got)
	}
	return nil
}

// =========================================================================
// main
// =========================================================================

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		fmt.Fprintln(os.Stderr, "Run 'skystacker --help' for usage.")
		os.Exit(2)
	}
	if opts.Help {
		printUsage(os.Stdout)
		return
	}
	if opts.Version {
		fmt.Println(version)
		return
	}
	if opts.SelfTest {
		if runSelfTests() > 0 {
			os.Exit(1)
		}
		return
	}

	err = runPipeline(context.Background(), runOptions{
		InputDir:  opts.InputDir,
		OutputDir: opts.OutputDir,
		Workers:   opts.Workers,
		Recursive: opts.Recursive,
		Overwrite: opts.Overwrite,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		os.Exit(1)
	}
}


package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var imageOperations = []string{
	"resize", "blur", "sharpen", "background", "stretch", "denoise", "deconvolve", "starless",
	"sum", "subtract", "average", "median", "min", "max",
	"measure-transform", "measure-quality", "sky-map", "survey-cutout", "plate-solve",
}

func validImageOperation(op string) bool { return slices.Contains(imageOperations, op) }

func imagePaths(spec string) ([]string, error) {
	parts := strings.Split(spec, ",")
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("--process-image has no paths")
	}
	return paths, nil
}

func loadImageFrames(paths []string) ([]*frame, error) {
	frames := make([]*frame, 0, len(paths))
	for _, path := range paths {
		f, err := decodeImageFile(path, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		frames = append(frames, f)
	}
	for i := 1; i < len(frames); i++ {
		if frames[i].Width != frames[0].Width || frames[i].Height != frames[0].Height {
			return nil, fmt.Errorf("%s is %dx%d but %s is %dx%d; equal sizes required",
				filepath.Base(paths[0]), frames[0].Width, frames[0].Height,
				filepath.Base(paths[i]), frames[i].Width, frames[i].Height)
		}
	}
	return frames, nil
}

// expandImageInputs expands directory arguments into their discovered image
// files, keeping explicit file paths and comma-separated lists intact. It is
// used by --image-op resize so a whole folder can be processed in one call.
func expandImageInputs(spec string, recursive bool) ([]string, error) {
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("image input %s: %w", p, err)
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		files, err := discoverFiles(p, recursive, nil)
		if err != nil {
			return nil, fmt.Errorf("scan image directory %s: %w", p, err)
		}
		out = append(out, files...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--process-image has no image files")
	}
	return out, nil
}

// resizeFrameArea downsamples by averaging every source pixel that falls in
// each destination cell. Compared with nearest/bilinear sampling this avoids
// aliasing when shrinking star fields by 2x or more.
func resizeFrameArea(f *frame, w, h int) *frame {
	if w <= 0 || h <= 0 {
		return nil
	}
	if f.Width == w && f.Height == h {
		return f
	}
	if w > f.Width || h > f.Height {
		return resizeFrame(f, w, h)
	}
	out := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3), WCS: f.WCS,
		Nonlinear: f.Nonlinear, BayerPattern: f.BayerPattern}
	for y := range h {
		y0 := y * f.Height / h
		y1 := (y + 1) * f.Height / h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := range w {
			x0 := x * f.Width / w
			x1 := (x + 1) * f.Width / w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum [3]float64
			for sy := y0; sy < y1 && sy < f.Height; sy++ {
				for sx := x0; sx < x1 && sx < f.Width; sx++ {
					i := (sy*f.Width + sx) * 3
					sum[0] += float64(f.Pix[i])
					sum[1] += float64(f.Pix[i+1])
					sum[2] += float64(f.Pix[i+2])
				}
			}
			count := float64((min(y1, f.Height) - y0) * (min(x1, f.Width) - x0))
			if count <= 0 {
				count = 1
			}
			o := (y*w + x) * 3
			for c := range 3 {
				out.Pix[o+c] = float32(sum[c] / count)
			}
		}
	}
	return out
}

// jpegMetadataSegments returns the original APP1 Exif/XMP segments so a
// resized JPEG keeps its capture time, GPS and camera metadata.
func jpegMetadataSegments(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, 256*1024)
	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]
	if len(buf) < 4 || buf[0] != 0xff || buf[1] != 0xd8 {
		return nil
	}
	var out []byte
	for pos := 2; pos+4 <= len(buf); {
		if buf[pos] != 0xff {
			break
		}
		marker := buf[pos+1]
		if marker == 0xda || marker == 0xd9 {
			break
		}
		length := int(buf[pos+2])<<8 | int(buf[pos+3])
		end := pos + 2 + length
		if length < 2 || end > len(buf) {
			break
		}
		if marker == 0xe1 {
			payload := buf[pos+4 : end]
			if (len(payload) >= 6 && string(payload[:6]) == "Exif\x00\x00") ||
				(len(payload) >= 29 && strings.HasPrefix(string(payload[:28]), "http://ns.adobe.com/xap/1.0/")) {
				out = append(out, buf[pos:end]...)
			}
		}
		pos = end
	}
	return out
}

// encodeResizedImage writes a display-faithful raster (no preview gamma lift)
// in the same format as the source path.
func encodeResizedImage(w io.Writer, srcPath, ext string, f *frame) error {
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	for y := range f.Height {
		row := y * img.Stride
		for x := range f.Width {
			i := (y*f.Width + x) * 3
			o := row + x*4
			for c := range 3 {
				v := f.Pix[i+c]
				if v < 0 {
					v = 0
				} else if v > 1 {
					v = 1
				}
				img.Pix[o+c] = uint8(v*255 + 0.5)
			}
			img.Pix[o+3] = 255
		}
	}
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		var body bytes.Buffer
		if err := jpeg.Encode(&body, img, &jpeg.Options{Quality: 95}); err != nil {
			return err
		}
		data := body.Bytes()
		meta := jpegMetadataSegments(srcPath)
		if len(meta) > 0 && len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8 {
			if _, err := w.Write(data[:2]); err != nil {
				return err
			}
			if _, err := w.Write(meta); err != nil {
				return err
			}
			_, err := w.Write(data[2:])
			return err
		}
		_, err := w.Write(data)
		return err
	case ".png":
		return png.Encode(w, img)
	default:
		return fmt.Errorf("resize output format %q is not supported (use jpg, jpeg or png)", ext)
	}
}

// runResizeImages implements --image-op resize: it shrinks every input by
// --scale without stacking and either writes <name>_resize.<ext> into
// --image-output or, with --overwrite, replaces each original atomically.
func runResizeImages(o *cliOptions) error {
	paths, err := expandImageInputs(o.ProcessImage, o.Recursive)
	if err != nil {
		return err
	}
	scale := o.Scale
	if scale < 1 {
		return fmt.Errorf("--scale must be >= 1 for resize, got %d", scale)
	}
	dir := o.ImageOutput
	if dir == "" {
		dir = "image_processing"
	}
	if !o.Overwrite {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png":
		default:
			return fmt.Errorf("resize does not support %s: convert it to jpg or png first", ext)
		}
		f, err := decodeImageFile(path, 0, 0)
		if err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		w := max(1, int(math.Round(float64(f.Width)/float64(scale))))
		h := max(1, int(math.Round(float64(f.Height)/float64(scale))))
		small := resizeFrameArea(f, w, h)
		dst := path
		if !o.Overwrite {
			stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			dst = filepath.Join(dir, stem+"_resize"+ext)
			if _, statErr := os.Stat(dst); statErr == nil {
				return fmt.Errorf("output %s already exists (use --overwrite to replace it)", dst)
			}
		}
		if err := writeFileAtomic(dst, func(wr io.Writer) error {
			return encodeResizedImage(wr, path, ext, small)
		}); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		logf("Image resize: %s -> %s (%dx%d -> %dx%d, scale 1/%d)",
			filepath.Base(path), dst, f.Width, f.Height, w, h, scale)
	}
	return nil
}

func writeImageProduct(dir, base, op string, f *frame, meta any) error {
	stem := strings.TrimSuffix(base, filepath.Ext(base)) + "_" + op
	raw := filepath.Join(dir, stem+".f32raw")
	if err := writeFileAtomic(raw, func(w io.Writer) error {
		return writeRawFloat32RGB(w, f.Width, f.Height, f.Pix)
	}); err != nil {
		return err
	}
	jpg := filepath.Join(dir, stem+".jpg")
	if err := writeFileAtomic(jpg, func(w io.Writer) error {
		return writePreviewJPEG(w, f.Width, f.Height, f.Pix)
	}); err != nil {
		return err
	}
	if meta != nil {
		if err := writeJSON(filepath.Join(dir, stem+".json"), meta); err != nil {
			return err
		}
	}
	logf("Image operation %s: wrote %s (%dx%d)", op, stem, f.Width, f.Height)
	return nil
}

type imageMeasure struct {
	File      string  `json:"file"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Stars     int     `json:"stars"`
	MedianHFR float64 `json:"median_hfr_px"`
	MedianEcc float64 `json:"median_eccentricity"`
	BgSigma   float64 `json:"bg_sigma"`
	SNR       float64 `json:"snr"`
	Quality   float64 `json:"quality"`
}

type imageMeasureReport struct {
	Operation string         `json:"operation"`
	Images    []imageMeasure `json:"images"`
	Transform *struct {
		Dx       float64 `json:"dx_px"`
		Dy       float64 `json:"dy_px"`
		Rotation float64 `json:"rotation_deg"`
		Scale    float64 `json:"scale"`
		Inliers  int     `json:"inliers"`
		RMS      float64 `json:"rms_px"`
	} `json:"transform,omitempty"`
}

func measureImageFrame(path string, f *frame) imageMeasure {
	stars := detectStars(f, 300)
	bg := estimateFrameBgNoise(f)
	hfr := medianStarHFR(stars)
	ecc := medianStarEcc(stars)
	snr := computeFrameSNR(stars, bg)
	q := computeAdvancedQuality(frameImageStats{
		StarCount: len(stars), SNR: snr, FWHM: 1.995 * hfr, Ecc: ecc, BgNoise: bg,
	})
	return imageMeasure{
		File: filepath.Base(path), Width: f.Width, Height: f.Height,
		Stars: len(stars), MedianHFR: hfr, MedianEcc: ecc,
		BgSigma: bg, SNR: snr, Quality: q,
	}
}

func writeImageMeasurements(dir string, rep imageMeasureReport) error {
	jsonPath := filepath.Join(dir, "image_measurements.json")
	if err := writeJSON(jsonPath, rep); err != nil {
		return err
	}
	csvPath := filepath.Join(dir, "image_measurements.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{"file", "width", "height", "stars", "median_hfr_px", "median_ecc", "bg_sigma", "snr", "quality"}); err != nil {
		f.Close()
		return err
	}
	for _, m := range rep.Images {
		if err := w.Write([]string{m.File, fmt.Sprint(m.Width), fmt.Sprint(m.Height), fmt.Sprint(m.Stars),
			fmt.Sprintf("%.4f", m.MedianHFR), fmt.Sprintf("%.4f", m.MedianEcc),
			fmt.Sprintf("%.6f", m.BgSigma), fmt.Sprintf("%.3f", m.SNR), fmt.Sprintf("%.2f", m.Quality)}); err != nil {
			f.Close()
			return err
		}
	}
	w.Flush()
	err = w.Error()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

type exportedStar struct {
	X    float64 `json:"x_px"`
	Y    float64 `json:"y_px"`
	Flux float64 `json:"flux"`
	Size int     `json:"size_px"`
	Ecc  float64 `json:"eccentricity"`
	HFR  float64 `json:"hfr_px"`
}

type starCatalog struct {
	Image     string         `json:"image"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Detection string         `json:"detection"`
	Count     int            `json:"count"`
	Stars     []exportedStar `json:"stars"`
}

// writeStarCatalog exports the detected star list of a single image, sorted
// from brightest to faintest, as JSON (default) or CSV (path ending in .csv).
// Coordinates are 0-based pixels and Flux is the background-subtracted
// integrated intensity from the high-pass detector.
func writeStarCatalog(path, imagePath string, f *frame, maxStars int) error {
	if f == nil || path == "" {
		return errors.New("star catalog: empty frame or path")
	}
	if maxStars <= 0 {
		maxStars = 500
	}
	stars := detectStars(f, maxStars)
	sort.SliceStable(stars, func(i, j int) bool { return stars[i].Flux > stars[j].Flux })
	catalog := starCatalog{Image: filepath.Base(imagePath), Width: f.Width, Height: f.Height,
		Detection: "high-pass connected components", Count: len(stars), Stars: make([]exportedStar, 0, len(stars))}
	for _, s := range stars {
		catalog.Stars = append(catalog.Stars, exportedStar{s.X, s.Y, s.Flux, s.Size, s.Ecc, s.HFR})
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(path), ".csv") {
		out, err := os.Create(path)
		if err != nil {
			return err
		}
		w := csv.NewWriter(out)
		if err := w.Write([]string{"x_px", "y_px", "flux", "size_px", "eccentricity", "hfr_px"}); err != nil {
			out.Close()
			return err
		}
		for _, s := range catalog.Stars {
			if err := w.Write([]string{
				strconv.FormatFloat(s.X, 'f', 3, 64), strconv.FormatFloat(s.Y, 'f', 3, 64),
				strconv.FormatFloat(s.Flux, 'f', 4, 64), strconv.Itoa(s.Size),
				strconv.FormatFloat(s.Ecc, 'f', 4, 64), strconv.FormatFloat(s.HFR, 'f', 4, 64),
			}); err != nil {
				out.Close()
				return err
			}
		}
		w.Flush()
		err = w.Error()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		return err
	}
	return writeJSON(path, catalog)
}

func imageArithmetic(op string, frames []*frame) (*frame, error) {
	w, h := frames[0].Width, frames[0].Height
	out := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	n := w * h * 3
	vals := make([]float32, len(frames))
	for i := range n {
		switch op {
		case "average":
			var s float64
			for _, f := range frames {
				s += float64(f.Pix[i])
			}
			out.Pix[i] = float32(s / float64(len(frames)))
		case "median":
			for j, f := range frames {
				vals[j] = f.Pix[i]
			}
			work := append([]float32(nil), vals...)
			out.Pix[i] = medianInPlace(work)
		case "min", "max":
			v := frames[0].Pix[i]
			for _, f := range frames[1:] {
				if op == "min" {
					v = min(v, f.Pix[i])
				} else {
					v = max(v, f.Pix[i])
				}
			}
			out.Pix[i] = v
		case "sum", "subtract":
			v := float64(frames[0].Pix[i])
			if op == "sum" {
				for _, f := range frames[1:] {
					v += float64(f.Pix[i])
				}
			} else {
				for _, f := range frames[1:] {
					v -= float64(f.Pix[i])
				}
			}
			out.Pix[i] = float32(v)
		default:
			return nil, fmt.Errorf("unsupported arithmetic op %q", op)
		}
	}
	return out, nil
}

func imageWCS(o *cliOptions, f *frame, path, dir, base string) (*plateSolution, error) {
	for _, candidate := range []string{
		filepath.Join(dir, "plate_solution.json"),
		filepath.Join(filepath.Dir(path), "plate_solution.json"),
	} {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		var sol plateSolution
		if err := json.Unmarshal(data, &sol); err == nil {
			return &sol, nil
		}
	}
	if !o.PlateSolve {
		return nil, fmt.Errorf("image needs a plate solution: pass --plate-solve or place plate_solution.json beside it")
	}
	stars := detectStars(f, 200)
	if len(stars) < 3 {
		return nil, fmt.Errorf("only %d stars detected; cannot plate-solve", len(stars))
	}
	seedRA, seedDec := 0.0, 0.0
	meta := readJPEGExif(path)
	if o.PlateCenter != "" {
		var err error
		seedRA, seedDec, err = parseRADecArg(o.PlateCenter, "--plate-center")
		if err != nil {
			return nil, err
		}
	} else if ra, dec, ok := referenceFITSSeed(path); ok {
		seedRA, seedDec = ra, dec
	} else if ra, dec, ok := meta.RADec(); ok {
		seedRA, seedDec = ra, dec
	}
	oldOffline, oldEpoch := plateGaiaOffline, plateEpochYear
	plateGaiaOffline = o.GaiaOffline
	plateEpochYear = meta.ObservationYear()
	defer func() { plateGaiaOffline, plateEpochYear = oldOffline, oldEpoch }()
	sol, err := solvePlateWithFallback(stars, f.Width, f.Height, seedRA, seedDec, path, solverOptionsFromCLI(o))
	if err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(dir, strings.TrimSuffix(base, filepath.Ext(base))+"_plate_solution.json")
	if err := writeJSON(jsonPath, sol); err != nil {
		logf("WARNING: could not write %s: %v", filepath.Base(jsonPath), err)
	}
	return sol, nil
}

func runImageCommand(o *cliOptions) error {
	oldMatched := matchedFilterEnabled
	matchedFilterEnabled = o.MatchedFilter
	defer func() { matchedFilterEnabled = oldMatched }()
	paths, err := imagePaths(o.ProcessImage)
	if err != nil {
		return err
	}
	if o.ExportStars != "" {
		f, err := decodeImageFile(paths[0], 0, 0)
		if err != nil {
			return err
		}
		return writeStarCatalog(o.ExportStars, paths[0], f, 500)
	}
	if o.ImageOp == "resize" {
		return runResizeImages(o)
	}
	if o.ClassifyObjects != "" {
		f, err := decodeImageFile(paths[0], 0, 0)
		if err != nil {
			return err
		}
		rep, err := classifyObjects(paths[0], f, 3)
		if err != nil {
			return err
		}
		return writeObjectClassification(o.ClassifyObjects, rep)
	}
	dir := o.ImageOutput
	if dir == "" {
		dir = "image_processing"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	op := o.ImageOp
	switch op {
	case "plate-solve":
		o.PlateSolve = true
		f, err := decodeImageFile(paths[0], 0, 0)
		if err != nil {
			return err
		}
		sol, err := imageWCS(o, f, paths[0], dir, filepath.Base(paths[0]))
		if err != nil {
			return err
		}
		logf("Plate solved: CRVAL=(%.6f, %.6f) RMS=%.2f arcsec method=%s",
			sol.WCS.CRVAL1*180/math.Pi, sol.WCS.CRVAL2*180/math.Pi, sol.RMSArcsec, sol.Method)
		return nil
	case "sky-map":
		f, err := decodeImageFile(paths[0], 0, 0)
		if err != nil {
			return err
		}
		sol, err := imageWCS(o, f, paths[0], dir, filepath.Base(paths[0]))
		if err != nil {
			return err
		}
		return runSkyMapForImage(f, sol, dir, filepath.Base(paths[0]), runOptions{
			AnnotationCatalogs: o.AnnotationCatalogs,
			SsoEphemeris:       o.SsoEphemeris,
		})
	case "survey-cutout":
		f, err := decodeImageFile(paths[0], 0, 0)
		if err != nil {
			return err
		}
		sol, err := imageWCS(o, f, paths[0], dir, filepath.Base(paths[0]))
		if err != nil {
			return err
		}
		return runSurveyForImage(f, sol, dir, filepath.Base(paths[0]), runOptions{
			SurveyHIPS: o.SurveyHIPS, SurveyTimeout: o.SurveyTimeout,
		})
	}

	frames, err := loadImageFrames(paths)
	if err != nil {
		return err
	}
	switch op {
	case "measure-transform", "measure-quality":
		rep := imageMeasureReport{Operation: op, Images: make([]imageMeasure, 0, len(frames))}
		for i, f := range frames {
			rep.Images = append(rep.Images, measureImageFrame(paths[i], f))
		}
		if op == "measure-transform" && len(frames) >= 2 {
			starsA := detectStars(frames[0], 300)
			starsB := detectStars(frames[1], 300)
			sim, inliers, ok := matchStarsToReference(starsA, starsB)
			if ok {
				p := sim.apply(point2D{})
				rep.Transform = &struct {
					Dx       float64 `json:"dx_px"`
					Dy       float64 `json:"dy_px"`
					Rotation float64 `json:"rotation_deg"`
					Scale    float64 `json:"scale"`
					Inliers  int     `json:"inliers"`
					RMS      float64 `json:"rms_px"`
				}{Dx: p.X, Dy: p.Y, Rotation: sim.rotation() * 180 / math.Pi, Scale: sim.scale(), Inliers: inliers}
			}
		}
		return writeImageMeasurements(dir, rep)
	case "sum", "subtract", "average", "median", "min", "max":
		out, err := imageArithmetic(op, frames)
		if err != nil {
			return err
		}
		return writeImageProduct(dir, filepath.Base(paths[0]), op, out, map[string]any{
			"operation": op, "inputs": paths, "width": out.Width, "height": out.Height,
		})
	}

	result := frames[0]
	switch op {
	case "blur":
		result = gaussianBlur(result, float64(max(2, o.DenoiseRadius)))
	case "sharpen":
		if o.WaveSharpen {
			if gains, gerr := parseWaveGains(o.WaveGains); gerr == nil {
				result = multiScaleSharpen(result, gains)
			}
		} else {
			result = unsharpMask(result, o.SharpenAmount, o.SharpenRadius)
		}
	case "background":
		if o.BGMultiscale {
			result = multiscaleBackground(result, 256)
		} else {
			result = removeBackgroundWithOptions(result, o.BGDegree, o.MaskBG, runOptions{
				BgAsymmetric: o.BgAsymmetric, BgClipLow: o.BgClipLow, BgClipHigh: o.BgClipHigh,
				BgHarmonicScale: o.BgHarmonicScale, BgPolyDegree: o.BgPolyDegree, BgRBF: o.BgRBF,
			})
		}
	case "stretch":
		if o.StretchColorPreserving {
			result = colorPreservingStretch(result, o.STFShadowsClip, o.STFTargetBG)
		} else {
			result = autoStretch(result, o.STFShadowsClip, o.STFTargetBG, o.STFLinked)
		}
	case "denoise":
		if o.Wavelets {
			if o.WaveletAdaptive {
				result = waveletDenoiseAdaptive(result, o.WaveletScales, o.WaveletSigma, 32)
			} else {
				result = waveletDenoise(result, o.WaveletScales, o.WaveletSigma)
			}
		} else {
			result = denoiseFrame(result, o.DenoiseRadius, 0.01, o.DenoiseStrength, o.DenoiseChroma)
		}
	case "deconvolve":
		if o.DeconvolveWiener {
			result = deconvolveWiener(result, o.DeconvSigma)
		} else if o.DeconvolveGrid {
			result = richardsonLucyGrid(result, o.DeconvSigma, o.DeconvIters, o.DeconvGridSize)
		} else {
			result = richardsonLucy(result, o.DeconvSigma, o.DeconvIters)
		}
	case "starless":
		result = removeStarsForOptions(result, runOptions{
			StarlessPSF: o.StarlessPSF, StarSigma: o.StarSigma, StarRadius: o.StarRadius,
		})
	default:
		return fmt.Errorf("unsupported image operation %q", op)
	}
	return writeImageProduct(dir, filepath.Base(paths[0]), op, result, map[string]any{
		"operation": op, "input": paths[0],
	})
}

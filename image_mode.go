package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var imageOperations = []string{
	"blur", "sharpen", "background", "stretch", "denoise", "deconvolve", "starless",
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
	paths, err := imagePaths(o.ProcessImage)
	if err != nil {
		return err
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
		return runSkyMapForImage(f, sol, dir, filepath.Base(paths[0]), runOptions{AnnotationCatalogs: o.AnnotationCatalogs})
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
			result = removeBackground(result, o.BGDegree, o.MaskBG)
		}
	case "stretch":
		result = autoStretch(result, o.STFShadowsClip, o.STFTargetBG, o.STFLinked)
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

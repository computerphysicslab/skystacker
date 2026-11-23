package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// This file closes the DSS survey gap: a rotated plate-solved frame can be
// wider in sky declination than the 60 arcmin GetImage limit, and a single
// photographic plate is monochrome. The helpers below tile the requested sky
// area, reproject every tile onto the stacked grid and combine three plates
// into a documented RGB/synthetic colour image.

const (
	dssMaxArcmin         = 60.0
	dssTilePadArcmin     = 2.0
	surveyRGBStem        = "survey_rgb_synthetic"
	defaultSurveyRGB     = "poss2ukstu_red,quickv,poss2ukstu_blue"
	dssMosaicWorkerCount = 4
)

// dssTile is one STScI GetImage request. H/W are already clamped to the
// service maximum; RA/Dec are the centres of this tile in degrees.
type dssTile struct {
	RA  float64
	Dec float64
	H   float64
	W   float64
}

// dssTileGrid lays out centred tiles over the sky footprint. When an axis is
// smaller than 60 arcmin one tile is enough. Otherwise the tile centres are
// symmetric around the field centre and separated by exactly (span - 60) /
// (n - 1), so adjacent 60 arcmin tiles overlap and the union covers the whole
// frame without extrapolating beyond the requested sky.
func dssTileGrid(ra, dec, raSpan, decSpan float64) []dssTile {
	paddedRA := math.Min(raSpan+dssTilePadArcmin, dssMaxArcmin*float64(int(math.Ceil(raSpan/dssMaxArcmin))+1))
	paddedDec := math.Min(decSpan+dssTilePadArcmin, dssMaxArcmin*float64(int(math.Ceil(decSpan/dssMaxArcmin))+1))
	raOffsets := dssAxisOffsets(paddedRA)
	decOffsets := dssAxisOffsets(paddedDec)
	tiles := make([]dssTile, 0, len(raOffsets)*len(decOffsets))
	cosDec := math.Cos(dec * math.Pi / 180)
	if math.Abs(cosDec) < 1e-9 {
		cosDec = 1e-9
	}
	for _, dy := range decOffsets {
		for _, dx := range raOffsets {
			tiles = append(tiles, dssTile{
				RA:  ra + dx/60/cosDec,
				Dec: dec + dy/60,
				H:   math.Min(dssMaxArcmin, paddedDec),
				W:   math.Min(dssMaxArcmin, paddedRA),
			})
		}
	}
	return tiles
}

func dssAxisOffsets(span float64) []float64 {
	if span <= dssMaxArcmin {
		return []float64{0}
	}
	n := int(math.Ceil(span / dssMaxArcmin))
	if n < 2 {
		n = 2
	}
	step := (span - dssMaxArcmin) / float64(n-1)
	out := make([]float64, n)
	for i := range out {
		out[i] = (float64(i) - float64(n-1)/2) * step
	}
	return out
}

// tileFrame is a reprojected tile with an explicit validity mask.
type tileFrame struct {
	Frame *frame
	Valid []bool
}

// mergeSurveyTiles averages all valid reprojected tiles onto the target grid.
// A pixel covered by two tiles is averaged, not summed, and pixels with no
// source coverage remain zero and are counted as invalid for the report.
func mergeSurveyTiles(width, height int, tiles []tileFrame) (*frame, []bool, int) {
	sum := make([]float32, width*height*3)
	weight := make([]float32, width*height)
	for _, tile := range tiles {
		if tile.Frame == nil || len(tile.Valid) < width*height || len(tile.Frame.Pix) < width*height*3 {
			continue
		}
		for p := 0; p < width*height; p++ {
			if !tile.Valid[p] {
				continue
			}
			weight[p]++
			i := p * 3
			sum[i] += tile.Frame.Pix[i]
			sum[i+1] += tile.Frame.Pix[i+1]
			sum[i+2] += tile.Frame.Pix[i+2]
		}
	}
	valid := make([]bool, width*height)
	invalid := 0
	for p := 0; p < width*height; p++ {
		if weight[p] <= 0 {
			invalid++
			continue
		}
		valid[p] = true
		i := p * 3
		inv := 1 / weight[p]
		sum[i] *= inv
		sum[i+1] *= inv
		sum[i+2] *= inv
	}
	return &frame{Width: width, Height: height, Pix: sum}, valid, invalid
}

// surveyMosaic is the in-memory result for one DSS survey: an aligned frame,
// its validity mask and the per-tile provenance needed for attribution.
type surveyMosaic struct {
	Frame       *frame
	Valid       []bool
	Invalid     int
	Tiles       []surveyTileReport
	SourceWCS   *wcsTan
	SourceCards map[string]string
	SourceFITS  []byte
}

// fetchDSSSurveyMosaic downloads every DSS tile needed to cover the target
// field, reprojects them onto the stacked grid and writes native and aligned
// products when requested. The original FITS bytes are never modified.
func fetchDSSSurveyMosaic(width, height int, targetWCS *wcsTan, opts runOptions,
	dir, productStem, survey string, timeout int, writeProducts bool) (*surveyMosaic, error) {
	ra, dec, _, _, err := surveyGeometry(targetWCS, width, height)
	if err != nil {
		return nil, err
	}
	raSpan, decSpan, err := surveyFootprintArcmin(targetWCS, width, height)
	if err != nil {
		return nil, err
	}
	tiles := dssTileGrid(ra, dec, raSpan, decSpan)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("survey mosaic: no tiles for %s", survey)
	}
	logf("Survey cutout: %s footprint %.2f x %.2f arcmin -> %d DSS tile(s)",
		survey, raSpan, decSpan, len(tiles))
	reprojected := make([]tileFrame, 0, len(tiles))
	result := &surveyMosaic{}
	for i, tile := range tiles {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		sourceURL, urlErr := dssCutoutURL("https://archive.stsci.edu/cgi-bin/dss_search",
			survey, tile.RA, tile.Dec, tile.H, tile.W)
		if urlErr != nil {
			cancel()
			return nil, urlErr
		}
		fitsData, fetchErr := fetchDSSCutout(ctx, "https://archive.stsci.edu/cgi-bin/dss_search",
			survey, tile.RA, tile.Dec, tile.H, tile.W)
		cancel()
		if fetchErr != nil {
			return nil, fmt.Errorf("survey mosaic: %s tile %d: %w", survey, i+1, fetchErr)
		}
		nativeStem := productStem
		if len(tiles) > 1 {
			nativeStem = fmt.Sprintf("%s_tile%d", productStem, i+1)
		}
		if writeProducts {
			if err := writeSurveyNativeFITS(dir, nativeStem, fitsData); err != nil {
				return nil, err
			}
		}
		sourceWCS, _, wcsErr := parseAstrometryWCS(fitsData)
		if wcsErr != nil {
			return nil, fmt.Errorf("survey mosaic: %s tile %d WCS: %w", survey, i+1, wcsErr)
		}
		tmp, tmpErr := os.CreateTemp("", "skystacker-mosaic-*.fits")
		if tmpErr != nil {
			return nil, tmpErr
		}
		tmpPath := tmp.Name()
		cleanup := func() { _ = os.Remove(tmpPath) }
		if _, err := tmp.Write(fitsData); err != nil {
			tmp.Close()
			cleanup()
			return nil, err
		}
		if err := tmp.Close(); err != nil {
			cleanup()
			return nil, err
		}
		sourceFrame, decodeErr := decodeFITSNative(tmpPath, 0, 0)
		cleanup()
		if decodeErr != nil {
			return nil, fmt.Errorf("survey mosaic: %s tile %d pixels: %w", survey, i+1, decodeErr)
		}
		aligned, valid, _ := reprojectSurveyMasked(sourceFrame, sourceWCS, targetWCS,
			width, height, dssMosaicWorkerCount)
		if aligned == nil {
			return nil, fmt.Errorf("survey mosaic: %s tile %d reprojection failed", survey, i+1)
		}
		reprojected = append(reprojected, tileFrame{Frame: aligned, Valid: valid})
		cards := parseFITSPrimaryCards(fitsData)
		result.Tiles = append(result.Tiles, surveyTileReport{
			Index: i + 1, RA: tile.RA, Dec: tile.Dec, H: tile.H, W: tile.W,
			PlateID:   strings.TrimSpace(cards["PLATEID"]),
			Copyright: strings.TrimSpace(cards["COPYRGHT"]),
			URL:       sourceURL, FITS: nativeStem + ".fits",
		})
		if i == 0 {
			result.SourceWCS = sourceWCS
			result.SourceCards = cards
			result.SourceFITS = fitsData
		}
	}
	frame, valid, invalid := mergeSurveyTiles(width, height, reprojected)
	result.Frame, result.Valid, result.Invalid = frame, valid, invalid
	if writeProducts && opts.SurveyReproject {
		if err := writeAlignedSurveyProducts(dir, productStem, frame, targetWCS, result.SourceWCS,
			result.SourceFITS, "dss", survey, result.Tiles[0].URL, invalid, result.Tiles); err != nil {
			return nil, err
		}
	}
	covered := width*height - invalid
	logf("Survey align: %s mosaic %dx%d, %.1f%% covered, %d tile(s)",
		survey, width, height, 100*float64(covered)/float64(width*height), len(tiles))
	return result, nil
}

func writeSurveyNativeFITS(dir, stem string, fitsData []byte) error {
	path := filepath.Join(dir, stem+".fits")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := out.Write(fitsData)
		return err
	}); err != nil {
		return err
	}
	sourceFrame, err := decodeFITSNative(path, 0, 0)
	if err == nil {
		jpg := filepath.Join(dir, stem+".jpg")
		if jerr := writeFileAtomic(jpg, func(out io.Writer) error {
			return writePreviewJPEG(out, sourceFrame.Width, sourceFrame.Height, sourceFrame.Pix)
		}); jerr != nil {
			logf("Survey cutout: native preview: %v", jerr)
		}
	}
	logf("Survey cutout: %s (%d bytes)", filepath.Base(path), len(fitsData))
	return nil
}

// parseSurveyRGBSurveys validates the three channel surveys. DSS has no green
// plate, so the default middle channel is Quick-V (V band, plate 01JV); users
// can substitute poss2ukstu_ir for a near-IR-into-green false colour mapping.
func parseSurveyRGBSurveys(value string) ([]string, error) {
	channels, err := parseDSSSurveys(value)
	if err != nil {
		return nil, err
	}
	if len(channels) != 3 {
		return nil, fmt.Errorf("RGB needs exactly 3 DSS surveys (R,G,B), got %d", len(channels))
	}
	return channels, nil
}

// parseSurveyRGBFallbacks accepts an ordered list of DSS surveys used to fill
// primary-channel gaps. An empty value disables the fallback entirely.
func parseSurveyRGBFallbacks(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	return parseDSSSurveys(value)
}

// composeSurveyRGB copies one monochrome DSS mosaic into each output channel.
// If a primary plate does not cover a pixel, the first fallback with valid data
// supplies that pixel; every substitution is counted for provenance. A pixel
// is invalid only when neither the primary nor any fallback covers it.
func composeSurveyRGB(width, height int, channels, fallbacks []string,
	mosaics map[string]*surveyMosaic) (*frame, []bool, int, map[string]int) {
	pix := make([]float32, width*height*3)
	valid := make([]bool, width*height)
	fallbackCounts := make(map[string]int, len(channels))
	invalid := 0
	for p := 0; p < width*height; p++ {
		i := p * 3
		complete := true
		for c, survey := range channels {
			m := mosaics[survey]
			if m != nil && m.Frame != nil && len(m.Valid) > p && m.Valid[p] {
				pix[i+c] = m.Frame.Pix[i]
				continue
			}
			filled := false
			for _, fallback := range fallbacks {
				fm := mosaics[fallback]
				if fm != nil && fm.Frame != nil && len(fm.Valid) > p && fm.Valid[p] {
					pix[i+c] = fm.Frame.Pix[i]
					fallbackCounts[survey]++
					filled = true
					break
				}
			}
			if !filled {
				complete = false
			}
		}
		if !complete {
			invalid++
			continue
		}
		valid[p] = true
	}
	return &frame{Width: width, Height: height, Pix: pix}, valid, invalid, fallbackCounts
}

type surveyRGBChannelReport struct {
	Role           string `json:"role"`
	Survey         string `json:"survey"`
	PlateID        string `json:"plate_id,omitempty"`
	Copyright      string `json:"copyright,omitempty"`
	URL            string `json:"url"`
	Licence        string `json:"licence"`
	Tiles          int    `json:"tiles"`
	InvalidPixels  int    `json:"invalid_pixels"`
	FallbackPixels int    `json:"fallback_pixels,omitempty"`
}

type surveyRGBReport struct {
	Generated         string                      `json:"generated"`
	Width             int                         `json:"width"`
	Height            int                         `json:"height"`
	Channels          []surveyRGBChannelReport    `json:"channels"`
	InvalidPixels     int                         `json:"invalid_pixels"`
	Mapping           string                      `json:"mapping"`
	Fallbacks         []string                    `json:"fallbacks,omitempty"`
	JPEG              string                      `json:"jpeg"`
	PNG               string                      `json:"png"`
	FITS              string                      `json:"fits"`
	StackedStretch    *surveyStretchReport        `json:"stacked_stretch,omitempty"`
	StackedRGBStretch *surveyStretchReport        `json:"stacked_rgb_stretch,omitempty"`
	AlignmentShift    *surveyAlignmentShiftReport `json:"alignment_shift,omitempty"`
}

// writeSurveyRGBFITS writes a three-plane BITPIX=-32 FITS with the stacked WCS
// so the colour image can be consumed by FITS-aware tools, not only viewers.
func writeSurveyRGBFITS(w io.Writer, f *frame, wc *wcsTan, channels []string) error {
	if f == nil || wc == nil || len(f.Pix) < f.Width*f.Height*3 {
		return fmt.Errorf("RGB FITS: invalid frame or WCS")
	}
	deg := 180 / math.Pi
	header := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                  -32",
		"NAXIS   =                    3",
		fmt.Sprintf("NAXIS1  = %20d", f.Width),
		fmt.Sprintf("NAXIS2  = %20d", f.Height),
		"NAXIS3  =                    3",
		"EXTEND  =                    T",
		"CTYPE1  = 'RA---TAN'",
		"CTYPE2  = 'DEC--TAN'",
		"RADESYS = 'ICRS'",
		fmt.Sprintf("CRVAL1  = %20.12f", wc.CRVAL1*deg),
		fmt.Sprintf("CRVAL2  = %20.12f", wc.CRVAL2*deg),
		fmt.Sprintf("CRPIX1  = %20.8f", wc.CRPIX1+1),
		fmt.Sprintf("CRPIX2  = %20.8f", wc.CRPIX2+1),
		fmt.Sprintf("CD1_1   = %20.12e", wc.CD[0][0]*deg),
		fmt.Sprintf("CD1_2   = %20.12e", wc.CD[0][1]*deg),
		fmt.Sprintf("CD2_1   = %20.12e", wc.CD[1][0]*deg),
		fmt.Sprintf("CD2_2   = %20.12e", wc.CD[1][1]*deg),
		fitsStringCard("RGBMAP", strings.Join(channels, ",")),
		fitsStringCard("LICENCE", "CC BY-NC-SA 4.0 (MAST DSS); plate rights vary"),
		fitsStringCard("ALIGNED", "DSS mosaic reprojected to target plate solution"),
	}
	if len(wc.SIP) >= 6 {
		header = append(header, "A_ORDER =                    2", "B_ORDER =                    2")
		for i, key := range []string{"A_2_0", "A_1_1", "A_0_2", "B_2_0", "B_1_1", "B_0_2"} {
			header = append(header, fmt.Sprintf("%-8s= %20.12e", key, wc.SIP[i]))
		}
	}
	header = append(header, "END")
	var hdr bytes.Buffer
	for _, card := range header {
		if len(card) > 80 {
			return fmt.Errorf("RGB FITS: header card too long: %q", card)
		}
		hdr.WriteString(fmt.Sprintf("%-80s", card))
	}
	for hdr.Len()%2880 != 0 {
		hdr.WriteByte(' ')
	}
	if _, err := w.Write(hdr.Bytes()); err != nil {
		return err
	}
	data := make([]byte, f.Width*f.Height*4)
	for c := 0; c < 3; c++ {
		for p := 0; p < f.Width*f.Height; p++ {
			binary.BigEndian.PutUint32(data[p*4:], math.Float32bits(f.Pix[p*3+c]))
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	pad := (2880 - (3*len(data))%2880) % 2880
	if pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}
	return nil
}

// writeSurveyRGBProducts writes the colour image, its FITS cube and the
// per-channel provenance. Every DSS plate keeps its own licence and plate ID.
func writeSurveyRGBProducts(outputDir, dir string, width, height int, wc *wcsTan, channels, fallbacks []string,
	mosaics map[string]*surveyMosaic, rgb *frame, valid []bool, invalid int, fallbackCounts map[string]int,
	shift *surveyAlignmentShiftReport) error {
	jpg := filepath.Join(dir, surveyRGBStem+".jpg")
	if err := writeFileAtomic(jpg, func(out io.Writer) error {
		return writePreviewJPEG(out, width, height, rgb.Pix)
	}); err != nil {
		return err
	}
	png := filepath.Join(dir, surveyRGBStem+".png")
	if err := writeFileAtomic(png, func(out io.Writer) error {
		return writePNGPreview(out, width, height, rgb.Pix)
	}); err != nil {
		return err
	}
	fits := filepath.Join(dir, surveyRGBStem+".fits")
	if err := writeFileAtomic(fits, func(out io.Writer) error {
		return writeSurveyRGBFITS(out, rgb, wc, channels)
	}); err != nil {
		return err
	}
	roles := []string{"R", "G", "B"}
	report := surveyRGBReport{
		Generated: time.Now().UTC().Format(time.RFC3339),
		Width:     width, Height: height,
		InvalidPixels: invalid,
		Mapping:       "R=red plate, G=Quick-V (V band; DSS has no green plate), B=blue plate",
		Fallbacks:     fallbacks,
		JPEG:          filepath.Base(jpg), PNG: filepath.Base(png), FITS: filepath.Base(fits),
	}
	for i, survey := range channels {
		m := mosaics[survey]
		ch := surveyRGBChannelReport{Role: roles[i], Survey: survey, Tiles: len(m.Tiles),
			InvalidPixels: m.Invalid, FallbackPixels: fallbackCounts[survey],
			Licence: "CC BY-NC-SA 4.0 (MAST DSS data use policy); plate rights vary"}
		if len(m.Tiles) > 0 {
			ch.PlateID = m.Tiles[0].PlateID
			ch.Copyright = m.Tiles[0].Copyright
			ch.URL = m.Tiles[0].URL
		}
		report.Channels = append(report.Channels, ch)
	}
	report.AlignmentShift = shift
	report.StackedStretch = stretchStackedMeanToSurvey(outputDir, dir, surveyRGBStem+".jpg", valid)
	report.StackedRGBStretch = stretchStackedMeanToSurveyRGB(outputDir, dir, surveyRGBStem+".jpg", valid)
	if err := writeJSON(filepath.Join(dir, surveyRGBStem+"_provenance.json"), report); err != nil {
		return err
	}
	logf("Survey RGB: wrote %s / %s / %s (%dx%d, %d uncovered pixels)",
		filepath.Base(jpg), filepath.Base(png), filepath.Base(fits), width, height, invalid)
	return nil
}

// writeDSSSurveyCutouts is the DSS branch of writeSurveyCutout. It fetches the
// requested plates plus any RGB channels, builds full-coverage mosaics and
// optionally writes the synthetic colour composite.
func writeDSSSurveyCutouts(width, height int, wcs *wcsTan, opts runOptions,
	outputDir, dir, stem string, refFrame *frame, timeout int) error {
	requested, err := parseDSSSurveys(opts.SurveyDSSSurvey)
	if err != nil {
		return err
	}
	rgbChannels, err := parseSurveyRGBSurveys(opts.SurveyRGBSurveys)
	if err != nil {
		return err
	}
	fallbacks, err := parseSurveyRGBFallbacks(opts.SurveyRGBFallback)
	if err != nil {
		return err
	}
	needed := append([]string{}, requested...)
	if opts.SurveyRGB {
		needed = append(needed, rgbChannels...)
		needed = append(needed, fallbacks...)
	}
	seen := map[string]bool{}
	surveys := make([]string, 0, len(needed))
	for _, survey := range needed {
		if !seen[survey] {
			seen[survey] = true
			surveys = append(surveys, survey)
		}
	}
	mosaics := make(map[string]*surveyMosaic, len(surveys))
	writeProducts := opts.SurveyReproject || opts.SurveyRGB
	for _, survey := range surveys {
		productStem := stem
		if len(surveys) > 1 {
			productStem += "_" + survey
		}
		mosaic, err := fetchDSSSurveyMosaic(width, height, wcs, opts, dir, productStem,
			survey, timeout, writeProducts)
		if err != nil {
			return err
		}
		mosaics[survey] = mosaic
	}
	if opts.SurveyRGB {
		rgb, valid, invalid, fallbackCounts := composeSurveyRGB(width, height, rgbChannels, fallbacks, mosaics)
		var shiftReport *surveyAlignmentShiftReport
		if refFrame != nil {
			dx, dy, ncc := surveyAlignmentShift(refFrame, rgb)
			if dx != 0 || dy != 0 {
				rgb = applySurveyAlignmentShift(rgb, dx, dy)
				logf("Survey RGB: alignment shift corrected by (%d, %d) px (NCC %.3f)", dx, dy, ncc)
			} else {
				logf("Survey RGB: no alignment shift needed (NCC %.3f)", ncc)
			}
			r := formatShiftReport(dx, dy, ncc, dx != 0 || dy != 0, "")
			shiftReport = &r
		}
		if err := writeSurveyRGBProducts(outputDir, dir, width, height, wcs, rgbChannels, fallbacks,
			mosaics, rgb, valid, invalid, fallbackCounts, shiftReport); err != nil {
			return err
		}
	}
	_ = refFrame
	return nil
}

// sortedSurveyNames gives deterministic ordering to provenance/map helpers.
func sortedSurveyNames(m map[string]*surveyMosaic) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

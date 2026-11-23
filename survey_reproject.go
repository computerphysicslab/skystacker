package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DSS surveys accepted by the STScI/MAST GetImage endpoint. The list is
// documented at https://archive.stsci.edu/dss/getting_started.html.
var dssSurveys = []string{
	"poss2ukstu_red", "poss2ukstu_blue", "poss2ukstu_ir",
	"poss1_red", "poss1_blue", "quickv", "phase2_gsc2", "phase2_gsc1",
}

func validDSSSurvey(name string) bool {
	for _, s := range dssSurveys {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

// dssCutoutURL builds the read-only STScI DSS GetImage query. h and w are in
// arcminutes (the service accepts at most 60); v3 is intentionally empty
// because it only rotates GIF overlays, not the FITS pixel grid.
func dssCutoutURL(baseURL, survey string, ra, dec, h, w float64) (string, error) {
	if !validDSSSurvey(survey) {
		return "", fmt.Errorf("unknown DSS survey %q", survey)
	}
	if h <= 0 || w <= 0 || h > 60 || w > 60 {
		return "", fmt.Errorf("DSS cutout size %.3f x %.3f arcmin outside (0, 60]", h, w)
	}
	if dec < -90 || dec > 90 {
		return "", fmt.Errorf("DSS declination %.6f outside [-90, 90]", dec)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("v", survey)
	q.Set("r", strconv.FormatFloat(ra, 'f', 8, 64))
	q.Set("d", strconv.FormatFloat(dec, 'f', 8, 64))
	q.Set("e", "J2000")
	q.Set("h", strconv.FormatFloat(h, 'f', 6, 64))
	q.Set("w", strconv.FormatFloat(w, 'f', 6, 64))
	q.Set("f", "fits")
	q.Set("c", "none")
	q.Set("fov", "NONE")
	q.Set("v3", "")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// fetchDSSCutout downloads a FITS cutout from the STScI/MAST DSS service.
func fetchDSSCutout(ctx context.Context, baseURL, survey string, ra, dec, h, w float64) ([]byte, error) {
	target, err := dssCutoutURL(baseURL, survey, ra, dec, h, w)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DSS GetImage HTTP %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, 512<<20)); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) < 2880 || string(data[:6]) != "SIMPLE" {
		return nil, fmt.Errorf("DSS GetImage returned non-FITS data (%d bytes)", len(data))
	}
	return data, nil
}

// parseFITSPrimaryCards reads the primary FITS header cards up to END. It is
// used only for provenance; geometry is handled by parseAstrometryWCS so the
// same CD/CDELT/SIP rules apply to every survey and plate solution.
func parseFITSPrimaryCards(data []byte) map[string]string {
	cards := make(map[string]string)
	limit := len(data)
	if limit > 2880*64 {
		limit = 2880 * 64
	}
	for off := 0; off+80 <= limit; off += 80 {
		card := strings.TrimRight(string(data[off:off+80]), " \x00")
		if strings.HasPrefix(card, "END") {
			break
		}
		eq := strings.IndexByte(card, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(card[:eq])
		val := strings.TrimSpace(card[eq+1:])
		if i := strings.Index(val, " /"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		cards[key] = strings.Trim(val, "' ")
	}
	return cards
}

// surveyAlignmentReport is written next to the reprojected FITS so a consumer
// can audit which plate, service and licence produced the aligned pixels.
type surveyAlignmentReport struct {
	Source        string          `json:"source"`
	Survey        string          `json:"survey"`
	URL           string          `json:"url,omitempty"`
	PlateID       string          `json:"plate_id,omitempty"`
	Copyright     string          `json:"copyright,omitempty"`
	Licence       string          `json:"licence"`
	Attribution   string          `json:"attribution"`
	Generated     string          `json:"generated"`
	Width         int             `json:"width"`
	Height        int             `json:"height"`
	InvalidPixels int             `json:"invalid_pixels"`
	SourceWCS     surveyWCSReport `json:"source_wcs"`
	TargetWCS     surveyWCSReport `json:"target_wcs"`
	FITS          string          `json:"fits"`
	Preview       string          `json:"preview"`
}

type surveyWCSReport struct {
	CRVAL1 float64       `json:"crval1_deg"`
	CRVAL2 float64       `json:"crval2_deg"`
	CRPIX1 float64       `json:"crpix1_0based"`
	CRPIX2 float64       `json:"crpix2_0based"`
	CD     [2][2]float64 `json:"cd_deg_per_pixel"`
}

func surveyWCSReportFrom(w *wcsTan) surveyWCSReport {
	if w == nil {
		return surveyWCSReport{}
	}
	deg := 180 / math.Pi
	return surveyWCSReport{
		CRVAL1: w.CRVAL1 * deg, CRVAL2: w.CRVAL2 * deg,
		CRPIX1: w.CRPIX1, CRPIX2: w.CRPIX2,
		CD: [2][2]float64{
			{w.CD[0][0] * deg, w.CD[0][1] * deg},
			{w.CD[1][0] * deg, w.CD[1][1] * deg},
		},
	}
}

// reprojectSurvey resamples the source survey onto the target WCS pixel grid.
// Target pixel centres are mapped to sky coordinates with the target TAN/SIP
// inverse, then to source pixels with the source TAN/SIP forward transform and
// bilinearly sampled. Rows are split across workers; results are independent
// of the worker count.
func reprojectSurvey(src *frame, srcWCS, dstWCS *wcsTan, width, height, workers int) (*frame, int) {
	if src == nil || srcWCS == nil || dstWCS == nil || width <= 0 || height <= 0 {
		return nil, 0
	}
	if workers < 1 {
		workers = 1
	}
	if workers > height {
		workers = height
	}
	pix := make([]float32, width*height*3)
	invalid := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for y := id; y < height; y += workers {
				for x := 0; x < width; x++ {
					ra, dec := wcsTanInverseSIP(dstWCS, float64(x), float64(y))
					sx, sy := wcsTanForward(srcWCS, ra, dec)
					r, g, b, ok := sampleBilinear(src, sx, sy)
					if !ok {
						mu.Lock()
						invalid++
						mu.Unlock()
						continue
					}
					i := (y*width + x) * 3
					pix[i], pix[i+1], pix[i+2] = r, g, b
				}
			}
		}(worker)
	}
	wg.Wait()
	return &frame{Width: width, Height: height, Pix: pix}, invalid
}

// fitsStringCard formats a string header card with doubled quotes and a
// bounded length; FITS cards are exactly 80 bytes and written padded.
func fitsStringCard(key, value string) string {
	value = strings.ReplaceAll(value, "'", "''")
	if len(value) > 68 {
		value = value[:68]
	}
	return fmt.Sprintf("%-8s= '%s'", key, value)
}

// writeReprojectedSurveyFITS writes a two-dimensional BITPIX=-32 FITS image
// on the target WCS. CRPIX is converted back to the FITS 1-based convention
// and quadratic SIP cards are included when the plate solution has them.
func writeReprojectedSurveyFITS(w io.Writer, f *frame, wc *wcsTan, cards []string) error {
	if f == nil || len(f.Pix) < f.Width*f.Height*3 || wc == nil {
		return fmt.Errorf("reprojected survey: invalid frame or WCS")
	}
	deg := 180 / math.Pi
	out := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                  -32",
		"NAXIS   =                    2",
		fmt.Sprintf("NAXIS1  = %20d", f.Width),
		fmt.Sprintf("NAXIS2  = %20d", f.Height),
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
	}
	if len(wc.SIP) >= 6 {
		out = append(out, "A_ORDER =                    2", "B_ORDER =                    2")
		for i, key := range []string{"A_2_0", "A_1_1", "A_0_2", "B_2_0", "B_1_1", "B_0_2"} {
			out = append(out, fmt.Sprintf("%-8s= %20.12e", key, wc.SIP[i]))
		}
	}
	out = append(out, cards...)
	out = append(out, "END")
	var hdr bytes.Buffer
	for _, c := range out {
		if len(c) > 80 {
			return fmt.Errorf("reprojected survey: header card too long: %q", c)
		}
		hdr.WriteString(fmt.Sprintf("%-80s", c))
	}
	for hdr.Len()%2880 != 0 {
		hdr.WriteByte(' ')
	}
	if _, err := w.Write(hdr.Bytes()); err != nil {
		return err
	}
	data := make([]byte, f.Width*f.Height*4)
	for i := 0; i < f.Width*f.Height; i++ {
		binary.BigEndian.PutUint32(data[i*4:], math.Float32bits(f.Pix[i*3]))
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	pad := (2880 - len(data)%2880) % 2880
	if pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}
	return nil
}

// surveyCutoutCards builds the provenance cards shared by the FITS header and
// the JSON sidecar. DSS plate copyright is preserved verbatim; the licence is
// stated explicitly instead of claiming the plate is public domain.
func surveyCutoutCards(rep surveyAlignmentReport) []string {
	return []string{
		fitsStringCard("SURVEY", rep.Survey),
		fitsStringCard("SERVING", rep.Source),
		fitsStringCard("PLATEID", rep.PlateID),
		fitsStringCard("COPYRGHT", rep.Copyright),
		fitsStringCard("LICENCE", rep.Licence),
		fitsStringCard("SRCURL", rep.URL),
		fitsStringCard("ALIGNED", "WCS reprojection to target plate solution"),
	}
}

// writeAlignedSurveyProducts writes the aligned FITS, a JPEG preview and a
// provenance report. It never modifies the downloaded source cutout.
func writeAlignedSurveyProducts(dir, stem string, aligned *frame, dstWCS, srcWCS *wcsTan,
	fitsData []byte, source, survey, sourceURL string, invalid int) error {
	cards := parseFITSPrimaryCards(fitsData)
	rep := surveyAlignmentReport{
		Source: source, Survey: survey, URL: sourceURL,
		PlateID: cards["PLATEID"], Copyright: cards["COPYRGHT"],
		Licence:     "CC BY-NC-SA 4.0 (MAST DSS data use policy); plate rights vary",
		Attribution: "Digitized Sky Survey, STScI/MAST; original plate observatory per COPYRGHT",
		Generated:   time.Now().UTC().Format(time.RFC3339),
		Width:       aligned.Width, Height: aligned.Height, InvalidPixels: invalid,
		SourceWCS: surveyWCSReportFrom(srcWCS), TargetWCS: surveyWCSReportFrom(dstWCS),
		FITS: stem + "_aligned.fits", Preview: stem + "_aligned.jpg",
	}
	if source == "hips" {
		rep.Licence = "Survey-dependent HiPS licence; consult the CDS HiPS collection metadata"
		rep.Attribution = "CDS hips2fits HiPS survey (plethoric survey collection)"
	}
	fitsPath := filepath.Join(dir, rep.FITS)
	if err := writeFileAtomic(fitsPath, func(out io.Writer) error {
		return writeReprojectedSurveyFITS(out, aligned, dstWCS, surveyCutoutCards(rep))
	}); err != nil {
		return err
	}
	jpgPath := filepath.Join(dir, rep.Preview)
	if err := writeFileAtomic(jpgPath, func(out io.Writer) error {
		return writePreviewJPEG(out, aligned.Width, aligned.Height, aligned.Pix)
	}); err != nil {
		return err
	}
	jsonPath := filepath.Join(dir, stem+"_alignment.json")
	if err := writeJSON(jsonPath, rep); err != nil {
		return err
	}
	logf("Survey align: %s (%dx%d, %d pixels outside source, %s/%s)", filepath.Base(fitsPath),
		aligned.Width, aligned.Height, invalid, source, survey)
	return nil
}

// surveyGeometry computes the centre and the DSS half-widths in arcminutes
// from a target WCS. It is pure so it can be unit-tested without a network.
func surveyGeometry(w *wcsTan, width, height int) (ra, dec, hArcmin, wArcmin float64, err error) {
	if w == nil || width <= 0 || height <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("survey geometry: invalid WCS or dimensions")
	}
	scale := math.Hypot(w.CD[0][0], w.CD[0][1]) * 180 / math.Pi
	if scale <= 0 || !isFiniteFloat(scale) {
		return 0, 0, 0, 0, fmt.Errorf("survey geometry: invalid plate scale")
	}
	ra = w.CRVAL1 * 180 / math.Pi
	dec = w.CRVAL2 * 180 / math.Pi
	hArcmin = float64(height) * scale * 60
	wArcmin = float64(width) * scale * 60
	if hArcmin > 60 {
		hArcmin = 60
	}
	if wArcmin > 60 {
		wArcmin = 60
	}
	return ra, dec, hArcmin, wArcmin, nil
}

// isFiniteFloat rejects NaN and infinity from hostile WCS headers.
func isFiniteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// surveyAlignmentRun ties the fetch, WCS parse, resampling and provenance
// together so offline tests can exercise the full path with a local FITS.
func surveyAlignmentRun(dir, stem string, fitsData []byte, source, survey, sourceURL string,
	targetWCS *wcsTan, width, height int) error {
	srcWCS, _, err := parseAstrometryWCS(fitsData)
	if err != nil {
		return fmt.Errorf("survey align: source WCS: %w", err)
	}
	tmp, err := os.CreateTemp("", "skystacker-survey-*.fits")
	if err != nil {
		return fmt.Errorf("survey align: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(fitsData); err != nil {
		tmp.Close()
		return fmt.Errorf("survey align: temp write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("survey align: temp close: %w", err)
	}
	sourceFrame, err := decodeFITSNative(tmpPath, 0, 0)
	if err != nil {
		return fmt.Errorf("survey align: source pixels: %w", err)
	}
	aligned, invalid := reprojectSurvey(sourceFrame, srcWCS, targetWCS, width, height, 4)
	if aligned == nil {
		return fmt.Errorf("survey align: reprojection failed")
	}
	return writeAlignedSurveyProducts(dir, stem, aligned, targetWCS, srcWCS,
		fitsData, source, survey, sourceURL, invalid)
}

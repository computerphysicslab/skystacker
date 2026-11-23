package main

import (
	"bytes"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func testSurveyWCS(raDeg, decDeg, scaleArcsec, crpix1, crpix2 float64) *wcsTan {
	deg := math.Pi / 180
	s := scaleArcsec / 3600 * deg
	return &wcsTan{
		CRVAL1: raDeg * deg, CRVAL2: decDeg * deg,
		CRPIX1: crpix1, CRPIX2: crpix2,
		CD: [2][2]float64{{s, 0}, {0, s}},
	}
}

func linearSurveyFrame(w, h int) *frame {
	pix := make([]float32, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := float32(x + 100*y)
			i := (y*w + x) * 3
			pix[i], pix[i+1], pix[i+2] = v, v, v
		}
	}
	return &frame{Width: w, Height: h, Pix: pix}
}

// testDSSCutoutURL verifies the documented GetImage parameters without a
// network request, including the FITS-only and overlay-roll decisions.
func testDSSCutoutURL() error {
	raw, err := dssCutoutURL("https://archive.stsci.edu/cgi-bin/dss_search",
		"poss2ukstu_red", 10.68458, 41.26917, 0.2, 0.3)
	if err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	q := u.Query()
	want := map[string]string{"v": "poss2ukstu_red", "r": "10.68458000", "d": "41.26917000",
		"e": "J2000", "h": "0.200000", "w": "0.300000", "f": "fits", "c": "none", "fov": "NONE"}
	for k, v := range want {
		if q.Get(k) != v {
			return fmt.Errorf("DSS param %s: got %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Get("v3") != "" {
		return fmt.Errorf("DSS v3 roll must stay empty for FITS: %q", q.Get("v3"))
	}
	if _, err := dssCutoutURL("https://example.test", "unknown", 0, 0, 1, 1); err == nil {
		return fmt.Errorf("unknown DSS survey accepted")
	}
	if _, err := dssCutoutURL("https://example.test", "quickv", 0, 0, 61, 1); err == nil {
		return fmt.Errorf("DSS field larger than 60 arcmin accepted")
	}
	return nil
}

func testSurveyGeometry() error {
	w := testSurveyWCS(10, 20, 1, 50, 50)
	ra, dec, h, wd, err := surveyGeometry(w, 100, 120)
	if err != nil || math.Abs(ra-10) > 1e-9 || math.Abs(dec-20) > 1e-9 {
		return fmt.Errorf("survey geometry centre: %.6f %.6f err=%v", ra, dec, err)
	}
	if math.Abs(h-2.0) > 1e-6 || math.Abs(wd-100.0/60) > 1e-6 {
		return fmt.Errorf("survey geometry arcmin: h=%.6f w=%.6f", h, wd)
	}
	_, _, h, wd, err = surveyGeometry(w, 10000, 10000)
	if err != nil || h != 60 || wd != 60 {
		return fmt.Errorf("survey geometry cap: %.3f %.3f err=%v", h, wd, err)
	}
	if _, _, _, _, err := surveyGeometry(nil, 1, 1); err == nil {
		return fmt.Errorf("nil WCS accepted")
	}
	return nil
}

// testSurveyReprojection verifies a pure pixel translation against the
// analytic linear pattern, then checks rotation/scale wiring and the count of
// target pixels with no source coverage.
func testSurveyReprojection() error {
	dst := testSurveyWCS(10, 20, 1, 50, 50)
	src := testSurveyWCS(10, 20, 1, 60, 60) // same sky, +10 px in source
	frameIn := linearSurveyFrame(120, 120)
	out, invalid := reprojectSurvey(frameIn, src, dst, 100, 100, 1)
	if out == nil || invalid != 0 {
		return fmt.Errorf("translation reprojection: invalid=%d", invalid)
	}
	for _, p := range [][2]int{{0, 0}, {17, 42}, {99, 99}} {
		got := out.Pix[(p[1]*100+p[0])*3]
		want := float32(p[0] + 10 + 100*(p[1]+10))
		if math.Abs(float64(got-want)) > 1e-3 {
			return fmt.Errorf("pixel %v: got %.3f want %.3f", p, got, want)
		}
	}
	// A rotated source still maps through TAN; compare with the analytic
	// forward transform at four sample points.
	deg := math.Pi / 180
	s := 1.0 / 3600 * deg
	rot := testSurveyWCS(10, 20, 1, 50, 50)
	rot.CD = [2][2]float64{{0, -s}, {s, 0}}
	out, _ = reprojectSurvey(frameIn, rot, dst, 100, 100, 3)
	for _, p := range [][2]int{{10, 10}, {80, 20}} {
		ra, dec := wcsTanInverseSIP(dst, float64(p[0]), float64(p[1]))
		sx, sy := wcsTanForward(rot, ra, dec)
		if sx < 0 || sy < 0 || sx >= 120 || sy >= 120 {
			return fmt.Errorf("rotated source pixel outside frame: %.2f %.2f", sx, sy)
		}
		want := float32(sx + 100*sy)
		got := out.Pix[(p[1]*100+p[0])*3]
		if math.Abs(float64(got-want)) > 0.05 {
			return fmt.Errorf("rotated pixel %v: got %.3f want %.3f", p, got, want)
		}
	}
	_, invalid = reprojectSurvey(linearSurveyFrame(10, 10), src, dst, 100, 100, 2)
	if invalid == 0 {
		return fmt.Errorf("small source frame reported full coverage")
	}
	return nil
}

// testReprojectedFITSHeader checks that the aligned FITS carries a FITS
// 1-based CRPIX, quadratic SIP and provenance cards readable by our own
// parser, so downstream tools can register the aligned product directly.
func testReprojectedFITSHeader() error {
	w := testSurveyWCS(10, 20, 1, 50, 50)
	w.SIP = []float64{1e-5, -2e-5, 3e-5, -4e-5, 5e-5, -6e-5}
	f := linearSurveyFrame(8, 6)
	cards := []string{
		fitsStringCard("SURVEY", "POSSII-F"),
		fitsStringCard("COPYRGHT", "Caltech/Palomar"),
		fitsStringCard("LICENCE", "CC BY-NC-SA 4.0"),
	}
	var buf bytes.Buffer
	if err := writeReprojectedSurveyFITS(&buf, f, w, cards); err != nil {
		return err
	}
	if buf.Len()%2880 != 0 {
		return fmt.Errorf("FITS length %d not padded", buf.Len())
	}
	parsed, sip, err := parseAstrometryWCS(buf.Bytes())
	if err != nil {
		return err
	}
	if math.Abs(parsed.CRVAL1-w.CRVAL1) > 1e-12 || math.Abs(parsed.CRPIX1-w.CRPIX1) > 1e-6 {
		return fmt.Errorf("WCS round trip: CRVAL1 %.12f CRPIX1 %.6f", parsed.CRVAL1, parsed.CRPIX1)
	}
	if len(sip) != 6 || math.Abs(sip[2]-w.SIP[2]) > 1e-12 {
		return fmt.Errorf("SIP round trip: %v", sip)
	}
	got := parseFITSPrimaryCards(buf.Bytes())
	if got["SURVEY"] != "POSSII-F" || got["COPYRGHT"] != "Caltech/Palomar" ||
		!strings.Contains(got["LICENCE"], "BY-NC-SA") {
		return fmt.Errorf("provenance cards: %v", got)
	}
	if v := strconv.Itoa(f.Width); got["NAXIS1"] != v {
		return fmt.Errorf("NAXIS1 %q", got["NAXIS1"])
	}
	return nil
}

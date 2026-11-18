package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type skyMapSource struct {
	X        float64
	Y        float64
	Mag      float64
	Name     string
	Kind     string
	Major    float64
	Minor    float64
	PA       float64
	Extended bool
}

func buildSkyMapSVG(width, height int, sources []skyMapSource) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", width, height, width, height)
	fmt.Fprintf(&b, "<rect width=\"%d\" height=\"%d\" fill=\"#05070d\"/>\n", width, height)
	for _, s := range sources {
		r := 1.5 + (18-s.Mag)*0.35
		if r < 1.2 {
			r = 1.2
		}
		if r > 10 {
			r = 10
		}
		label := html.EscapeString(s.Name)
		if s.Extended && s.Major > 0 && s.Minor > 0 {
			fmt.Fprintf(&b, "<ellipse cx=\"%.2f\" cy=\"%.2f\" rx=\"%.2f\" ry=\"%.2f\" transform=\"rotate(%.1f %.2f %.2f)\" fill=\"none\" stroke=\"#55ccff\" stroke-width=\"1.2\"><title>%s (%s)</title></ellipse>\n",
				s.X, s.Y, s.Major/2, s.Minor/2, s.PA, s.X, s.Y, label, html.EscapeString(s.Kind))
		} else {
			fmt.Fprintf(&b, "<circle cx=\"%.2f\" cy=\"%.2f\" r=\"%.2f\" fill=\"#ffe9b0\" opacity=\"0.85\"><title>%s (%s, G=%.2f)</title></circle>\n",
				s.X, s.Y, r, label, html.EscapeString(s.Kind), s.Mag)
		}
	}
	fmt.Fprintf(&b, "<text x=\"8\" y=\"%d\" fill=\"#8899aa\" font-family=\"monospace\" font-size=\"11\">SkyStacker synthetic sky map — %d catalogued sources</text>\n</svg>\n", height-8, len(sources))
	return b.String()
}

func readStackDimensions(outputDir string) (int, int, error) {
	f, err := os.Open(filepath.Join(outputDir, "_work", "stacked_mean.f32raw"))
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return 0, 0, err
	}
	return int(uint32(hdr[0]) | uint32(hdr[1])<<8 | uint32(hdr[2])<<16 | uint32(hdr[3])<<24),
		int(uint32(hdr[4]) | uint32(hdr[5])<<8 | uint32(hdr[6])<<16 | uint32(hdr[7])<<24), nil
}

func runSkyMapSVG(outputDir string, opts runOptions) error {
	data, err := os.ReadFile(filepath.Join(outputDir, "plate_solution.json"))
	if err != nil {
		logf("Sky map: no plate solution, skipping")
		return nil
	}
	var sol plateSolution
	if err := json.Unmarshal(data, &sol); err != nil {
		return err
	}
	w, h, err := readStackDimensions(outputDir)
	if err != nil {
		return err
	}
	ww := sol.WCS
	ww.SIP = sol.SIP
	nside := gaiaDefaultNside
	ipix := healpixAng2PixNested(nside, ww.CRVAL1, ww.CRVAL2)
	tile, ok := gaiaLoadTileCache(gaiaDefaultCacheDir(), nside, ipix)
	if !ok {
		logf("Sky map: no cached Gaia tile for nside=%d ipix=%d, skipping", nside, ipix)
		return nil
	}
	sources := make([]skyMapSource, 0, len(tile.Stars))
	for _, st := range tile.Stars {
		x, y := wcsTanForward(&ww, st.RA, st.Dec)
		if x >= 0 && y >= 0 && x < float64(w) && y < float64(h) {
			sources = append(sources, skyMapSource{X: x, Y: y, Mag: st.MagG, Kind: "Gaia"})
		}
	}
	extra, err := loadAnnotationCatalogs(opts.AnnotationCatalogs)
	if err != nil {
		return err
	}
	for _, e := range extra {
		x, y := wcsTanForward(&ww, e.RA*math.Pi/180, e.Dec*math.Pi/180)
		if x >= 0 && y >= 0 && x < float64(w) && y < float64(h) {
			sources = append(sources, skyMapSource{X: x, Y: y, Mag: e.Mag, Name: e.Name, Kind: e.Type, Major: e.Major, Minor: e.Minor, PA: e.PA, Extended: e.Major > 0})
		}
	}
	path := filepath.Join(outputDir, "stacked_sky_map.svg")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := io.WriteString(out, buildSkyMapSVG(w, h, sources))
		return err
	}); err != nil {
		return err
	}
	logf("Sky map: wrote %s with %d sources (%dx%d)", filepath.Base(path), len(sources), w, h)
	return nil
}

func fetchHiPSCutout(ctx context.Context, baseURL, hips string, width, height int, ra, dec, fov, rotation float64) ([]byte, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("hips", hips)
	q.Set("width", strconv.Itoa(width))
	q.Set("height", strconv.Itoa(height))
	q.Set("projection", "TAN")
	q.Set("ra", strconv.FormatFloat(ra, 'f', 8, 64))
	q.Set("dec", strconv.FormatFloat(dec, 'f', 8, 64))
	q.Set("fov", strconv.FormatFloat(fov, 'f', 8, 64))
	q.Set("coordsys", "icrs")
	q.Set("rotation_angle", strconv.FormatFloat(rotation, 'f', 8, 64))
	q.Set("format", "fits")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hips2fits HTTP %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, 512<<20)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func runSurveyCutout(outputDir string, opts runOptions) error {
	data, err := os.ReadFile(filepath.Join(outputDir, "plate_solution.json"))
	if err != nil {
		logf("Survey cutout: no plate solution, skipping")
		return nil
	}
	var sol plateSolution
	if err := json.Unmarshal(data, &sol); err != nil {
		return err
	}
	w, h, err := readStackDimensions(outputDir)
	if err != nil {
		return err
	}
	scale := math.Hypot(sol.WCS.CD[0][0], sol.WCS.CD[0][1]) * 180 / math.Pi
	if scale <= 0 {
		return fmt.Errorf("survey cutout: invalid plate scale")
	}
	fov := float64(max(w, h)) * scale
	ra, dec := sol.WCS.CRVAL1*180/math.Pi, sol.WCS.CRVAL2*180/math.Pi
	rotation := math.Atan2(sol.WCS.CD[0][1], sol.WCS.CD[1][1]) * 180 / math.Pi
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.SurveyTimeout)*time.Second)
	defer cancel()
	fitsData, err := fetchHiPSCutout(ctx, "https://alasky.cds.unistra.fr/hips-image-services/hips2fits",
		opts.SurveyHIPS, w, h, ra, dec, fov, rotation)
	if err != nil {
		return err
	}
	if len(fitsData) < 2880 || string(fitsData[:6]) != "SIMPLE" {
		return fmt.Errorf("survey cutout: service returned non-FITS data")
	}
	path := filepath.Join(outputDir, "survey_reference.fits")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := out.Write(fitsData)
		return err
	}); err != nil {
		return err
	}
	survey, err := decodeFITSNative(path, 0, 0)
	if err != nil {
		logf("Survey cutout: FITS saved but preview failed: %v", err)
		return nil
	}
	jpg := filepath.Join(outputDir, "survey_reference.jpg")
	if err := writeFileAtomic(jpg, func(out io.Writer) error {
		return writePreviewJPEG(out, survey.Width, survey.Height, survey.Pix)
	}); err != nil {
		logf("Survey cutout: preview: %v", err)
	}
	iw, ih, ipix, ierr := readRawFloat32RGB(filepath.Join(outputDir, "_work", "stacked_mean.f32raw"))
	if ierr == nil && iw == survey.Width && ih == survey.Height {
		refStars := detectStars(&frame{Width: iw, Height: ih, Pix: ipix}, 300)
		surveyStars := detectStars(survey, 300)
		sim, inliers, ok := matchStarsToReference(refStars, surveyStars)
		if ok {
			logf("Survey cutout: star-match %d inliers, scale %.4f, rotation %.2f deg", inliers, sim.scale(), sim.rotation()*180/math.Pi)
		} else {
			logf("Survey cutout: star matching failed (%d ref, %d survey stars); inspect %s", len(refStars), len(surveyStars), filepath.Base(path))
		}
	}
	logf("Survey cutout: %s (%dx%d, FOV %.4f deg, rotation %.2f deg)", filepath.Base(path), w, h, fov, rotation)
	return nil
}

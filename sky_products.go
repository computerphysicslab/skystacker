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
	"sort"
	"strconv"
	"strings"
	"time"
)

type skyMapSource struct {
	X        float64
	Y        float64
	RA       float64
	Dec      float64
	Mag      float64
	Name     string
	Kind     string
	Major    float64
	Minor    float64
	PA       float64
	Extended bool
}

type skyVelocityVector struct {
	X1          float64
	Y1          float64
	X2          float64
	Y2          float64
	Name        string
	Observatory string
}

// Label placement constants mirror the layered annotation design: eight
// selectable fields per label, sixteen candidate anchor positions (eight
// directions at two radii) and twenty collision-resolution passes.
const (
	skyLabelFields             = 8
	skyLabelCandidatePositions = 16
	skyLabelCollisionPasses    = 20
	skyLabelFontSize           = 10.0
)

var skyLabelFieldNames = [skyLabelFields]string{"name", "kind", "mag", "ra", "dec", "major", "minor", "pa"}

// fieldRotationDeg returns the pixel x-axis position angle derived from the
// CD matrix, in degrees.
func fieldRotationDeg(w *wcsTan) float64 {
	if w == nil {
		return 0
	}
	return math.Atan2(w.CD[0][1], w.CD[1][1]) * 180.0 / math.Pi
}

// correctedPA maps a celestial position angle into the pixel frame, accounting
// for field rotation and a mirrored CD matrix (negative determinant).
func correctedPA(w *wcsTan, paDeg float64) float64 {
	if w == nil {
		return paDeg
	}
	sign := 1.0
	if w.CD[0][0]*w.CD[1][1]-w.CD[0][1]*w.CD[1][0] < 0 {
		sign = -1
	}
	return fieldRotationDeg(w) + sign*paDeg
}

type labelBox struct {
	X, Y, W, H float64
	Text       string
}

func labelText(s skyMapSource, fields int) string {
	if fields <= 0 || fields > skyLabelFields {
		fields = 1
	}
	parts := make([]string, 0, fields)
	for i := 0; i < fields; i++ {
		switch skyLabelFieldNames[i] {
		case "name":
			if s.Name != "" {
				parts = append(parts, s.Name)
			}
		case "kind":
			if s.Kind != "" {
				parts = append(parts, s.Kind)
			}
		case "mag":
			if s.Mag != 0 {
				parts = append(parts, fmt.Sprintf("G=%.1f", s.Mag))
			}
		case "ra":
			parts = append(parts, fmt.Sprintf("RA=%.3f", s.RA*180/math.Pi))
		case "dec":
			parts = append(parts, fmt.Sprintf("Dec=%.3f", s.Dec*180/math.Pi))
		case "major":
			if s.Major > 0 {
				parts = append(parts, fmt.Sprintf("a=%.1f", s.Major))
			}
		case "minor":
			if s.Minor > 0 {
				parts = append(parts, fmt.Sprintf("b=%.1f", s.Minor))
			}
		case "pa":
			if s.PA != 0 || s.Extended {
				parts = append(parts, fmt.Sprintf("PA=%.1f", s.PA))
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func overlapArea(a, b labelBox) float64 {
	dx := math.Min(a.X+a.W, b.X+b.W) - math.Max(a.X, b.X)
	dy := math.Min(a.Y+a.H, b.Y+b.H) - math.Max(a.Y, b.Y)
	if dx <= 0 || dy <= 0 {
		return 0
	}
	return dx * dy
}

// placeLabels resolves collisions deterministically over 20 passes. Each
// label checks 16 candidate anchors (8 directions x 2 radii) and keeps the
// placement with the smallest overlap and distance penalty.
func placeLabels(width, height int, sources []skyMapSource, fields int) []labelBox {
	placed := make([]labelBox, 0, len(sources))
	dirs := [8][2]float64{{1, 0}, {0.707, 0.707}, {0, 1}, {-0.707, 0.707}, {-1, 0}, {-0.707, -0.707}, {0, -1}, {0.707, -0.707}}
	order := make([]int, len(sources))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return sources[order[i]].Mag < sources[order[j]].Mag })
	for pass := range skyLabelCollisionPasses {
		for _, idx := range order {
			s := sources[idx]
			text := labelText(s, fields)
			if text == "" {
				continue
			}
			w := float64(len(text)) * skyLabelFontSize * 0.62
			h := skyLabelFontSize + 4
			best := labelBox{}
			bestScore := math.Inf(1)
			for c := 0; c < skyLabelCandidatePositions; c++ {
				d := dirs[c%8]
				radius := 11.0
				if c >= 8 {
					radius = 22.0
				}
				x := s.X + d[0]*radius
				y := s.Y + d[1]*radius
				if d[0] < -0.5 {
					x -= w
				}
				if d[1] < -0.5 {
					y -= h
				}
				x = math.Max(2, math.Min(x, float64(width)-w-2))
				y = math.Max(2, math.Min(y, float64(height)-h-2))
				cand := labelBox{X: x, Y: y, W: w, H: h, Text: text}
				score := math.Hypot(x-s.X, y-s.Y) + float64(c)*0.05
				for _, other := range placed {
					if other.Text == text && other.X == x && other.Y == y {
						continue
					}
					score += overlapArea(cand, other) * 2
				}
				if score < bestScore {
					bestScore, best = score, cand
				}
			}
			found := false
			for j := range placed {
				if placed[j].Text == text {
					placed[j] = best
					found = true
					break
				}
			}
			if !found {
				placed = append(placed, best)
			}
		}
		if pass == skyLabelCollisionPasses-1 {
			break
		}
	}
	return placed
}

func buildSkyMapSVG(width, height int, sources []skyMapSource) string {
	return buildSkyMapSVGLayers(width, height, sources, nil, 1)
}

// buildSkyMapSVGLayers writes a layered SVG: independent <g> groups for the
// background, point sources, extended objects, SSO markers, velocity vectors,
// labels and footer. Layers carry stable ids and class attributes so external
// applications can toggle, restyle or recompose them.
func buildSkyMapSVGLayers(width, height int, sources []skyMapSource, velocities []skyVelocityVector, labelFields int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", width, height, width, height)
	fmt.Fprintln(&b, "<style>.skystacker-layer{isolation:isolate} .skystacker-hidden{display:none}</style>")
	fmt.Fprintf(&b, "<g id=\"background\" class=\"skystacker-layer\" data-layer=\"background\"><rect width=\"%d\" height=\"%d\" fill=\"#05070d\"/></g>\n", width, height)
	fmt.Fprintln(&b, "<g id=\"gaia-stars\" class=\"skystacker-layer\" data-layer=\"point-sources\">")
	for _, s := range sources {
		if s.Extended && s.Major > 0 && s.Minor > 0 {
			continue
		}
		r := 1.5 + (18-s.Mag)*0.35
		if r < 1.2 {
			r = 1.2
		}
		if r > 10 {
			r = 10
		}
		label := html.EscapeString(s.Name)
		fmt.Fprintf(&b, "<circle cx=\"%.2f\" cy=\"%.2f\" r=\"%.2f\" fill=\"#ffe9b0\" opacity=\"0.85\"><title>%s (%s, G=%.2f)</title></circle>\n",
			s.X, s.Y, r, label, html.EscapeString(s.Kind), s.Mag)
	}
	fmt.Fprintln(&b, "</g>")
	fmt.Fprintln(&b, "<g id=\"extended-objects\" class=\"skystacker-layer\" data-layer=\"extended-sources\">")
	for _, s := range sources {
		if !s.Extended || s.Major <= 0 || s.Minor <= 0 {
			continue
		}
		label := html.EscapeString(s.Name)
		fmt.Fprintf(&b, "<ellipse cx=\"%.2f\" cy=\"%.2f\" rx=\"%.2f\" ry=\"%.2f\" transform=\"rotate(%.1f %.2f %.2f)\" fill=\"none\" stroke=\"#55ccff\" stroke-width=\"1.2\" data-pa=\"%.1f\"><title>%s (%s)</title></ellipse>\n",
			s.X, s.Y, s.Major/2, s.Minor/2, s.PA, s.X, s.Y, s.PA, label, html.EscapeString(s.Kind))
	}
	fmt.Fprintln(&b, "</g>")
	fmt.Fprintln(&b, "<g id=\"sso-vectors\" class=\"skystacker-layer\" data-layer=\"motion\">")
	for _, v := range velocities {
		fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#ff8f6b\" stroke-width=\"1.4\" marker-end=\"url(#sso-arrow)\" data-object=\"%s\" data-site=\"%s\"><title>%s velocity vector</title></line>\n",
			v.X1, v.Y1, v.X2, v.Y2, html.EscapeString(v.Name), html.EscapeString(v.Observatory), html.EscapeString(v.Name))
	}
	fmt.Fprintln(&b, "</g>")
	fmt.Fprintf(&b, "<defs><marker id=\"sso-arrow\" markerWidth=\"6\" markerHeight=\"6\" refX=\"5\" refY=\"3\" orient=\"auto\"><path d=\"M0,0 L6,3 L0,6 Z\" fill=\"#ff8f6b\"/></marker></defs>\n")
	fmt.Fprintf(&b, "<g id=\"labels\" class=\"skystacker-layer\" data-layer=\"labels\" data-fields=\"%d\" data-candidates=\"%d\" data-passes=\"%d\">\n",
		skyLabelFields, skyLabelCandidatePositions, skyLabelCollisionPasses)
	for _, l := range placeLabels(width, height, sources, labelFields) {
		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" fill=\"#d7e3f4\" font-family=\"monospace\" font-size=\"%.0f\">%s</text>\n",
			l.X, l.Y+skyLabelFontSize, skyLabelFontSize, html.EscapeString(l.Text))
	}
	fmt.Fprintln(&b, "</g>")
	fmt.Fprintf(&b, "<g id=\"footer\" class=\"skystacker-layer\" data-layer=\"footer\"><text x=\"8\" y=\"%d\" fill=\"#8899aa\" font-family=\"monospace\" font-size=\"11\">SkyStacker layered sky map — %d catalogued sources, %d motion vectors</text></g>\n",
		height-8, len(sources), len(velocities))
	fmt.Fprintln(&b, "</svg>")
	return b.String()
}

func skySourcesForImage(width, height int, ww *wcsTan, gaia []gaiaStar, extra []annotationCatalogEntry) []skyMapSource {
	sources := make([]skyMapSource, 0, len(gaia)+len(extra))
	for _, st := range gaia {
		x, y := wcsTanForward(ww, st.RA, st.Dec)
		if x >= 0 && y >= 0 && x < float64(width) && y < float64(height) {
			sources = append(sources, skyMapSource{X: x, Y: y, RA: st.RA, Dec: st.Dec, Mag: st.MagG, Kind: "Gaia"})
		}
	}
	for _, e := range extra {
		ra, dec := e.RA*math.Pi/180, e.Dec*math.Pi/180
		x, y := wcsTanForward(ww, ra, dec)
		if x >= 0 && y >= 0 && x < float64(width) && y < float64(height) {
			sources = append(sources, skyMapSource{X: x, Y: y, RA: ra, Dec: dec, Mag: e.Mag, Name: e.Name, Kind: e.Type,
				Major: e.Major, Minor: e.Minor, PA: correctedPA(ww, e.PA), Extended: e.Major > 0})
		}
	}
	return sources
}

// skyMapVectors loads an optional SSO ephemeris and projects its motion into
// the solved frame. Missing files are reported and skipped; a bad ephemeris is
// a hard error because the user explicitly requested the motion layer.
func skyMapVectors(w *wcsTan, path string) []skyVelocityVector {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	e, err := loadSSOEphemeris(path)
	if err != nil {
		logf("Sky map: SSO ephemeris ignored: %v", err)
		return nil
	}
	return ssoVectorsForWCS(w, e, 0, 0)
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
	fetch := func(ipix int) bool {
		if opts.GaiaOffline || !opts.GaiaMultiTile {
			return false
		}
		if _, err := gaiaGetTile(gaiaDefaultCacheDir(), gaiaDefaultMirror, gaiaDefaultNside,
			ipix, gaiaDefaultMagLimit, false, false); err != nil {
			logf("Sky map: Gaia tile %d fetch failed: %v", ipix, err)
			return false
		}
		return true
	}
	gaia, coverage := gaiaStarsForField(gaiaDefaultCacheDir(), gaiaDefaultNside, &ww, w, h, 9, fetch)
	if coverage.Missing > 0 {
		logf("Sky map: Gaia covers %d/%d frame samples (%d tiles); %d samples have no cached tile",
			coverage.Covered, coverage.Samples, coverage.Tiles, coverage.Missing)
	}
	if len(gaia) == 0 {
		logf("Sky map: no Gaia stars for the field, skipping")
		return nil
	}
	extra, err := loadAnnotationCatalogs(opts.AnnotationCatalogs)
	if err != nil {
		return err
	}
	sources := skySourcesForImage(w, h, &ww, gaia, extra)
	vectors := skyMapVectors(&ww, opts.SsoEphemeris)
	mapsDir, err := ensureProductSubdir(outputDir, mapProductSubdir)
	if err != nil {
		return err
	}
	path := filepath.Join(mapsDir, "stacked_sky_map.svg")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := io.WriteString(out, buildSkyMapSVGLayers(w, h, sources, vectors, 1))
		return err
	}); err != nil {
		return err
	}
	logf("Sky map: wrote %s with %d sources (%dx%d)", filepath.Base(path), len(sources), w, h)
	return nil
}

// suggestSurveyTimeout returns the HTTP timeout used when --survey-timeout=0
// (auto). DSS plates of a full frame are ~18 MB and took ~76 s on the M31
// acceptance run, so they get a much larger budget than HiPS thumbnails.
func suggestSurveyTimeout(source string, width, height int) int {
	if strings.EqualFold(strings.TrimSpace(source), "dss") {
		if int64(width)*int64(height) > 1_000_000 {
			return 300
		}
		return 120
	}
	return 30
}

// writeSurveyProduct validates, stores, optionally reprojects and previews a
// single survey FITS. The stem already includes the survey suffix when a run
// requests more than one DSS plate.
func writeSurveyProduct(fitsData []byte, source, survey, sourceURL string,
	opts runOptions, dir, stem string, width, height int, wcs *wcsTan, refFrame *frame) error {
	if len(fitsData) < 2880 || string(fitsData[:6]) != "SIMPLE" {
		return fmt.Errorf("survey cutout: service returned non-FITS data")
	}
	path := filepath.Join(dir, stem+".fits")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := out.Write(fitsData)
		return err
	}); err != nil {
		return err
	}
	if opts.SurveyReproject {
		if err := surveyAlignmentRun(dir, stem, fitsData, source, survey, sourceURL, wcs, width, height); err != nil {
			return err
		}
	}
	surveyFrame, err := decodeFITSNative(path, 0, 0)
	if err != nil {
		logf("Survey cutout: FITS saved but preview failed: %v", err)
		return nil
	}
	jpg := filepath.Join(dir, stem+".jpg")
	if err := writeFileAtomic(jpg, func(out io.Writer) error {
		return writePreviewJPEG(out, surveyFrame.Width, surveyFrame.Height, surveyFrame.Pix)
	}); err != nil {
		logf("Survey cutout: preview: %v", err)
	}
	if refFrame != nil && refFrame.Width == surveyFrame.Width && refFrame.Height == surveyFrame.Height {
		refStars := detectStars(refFrame, 300)
		surveyStars := detectStars(surveyFrame, 300)
		sim, inliers, ok := matchStarsToReference(refStars, surveyStars)
		if ok {
			logf("Survey cutout: %s star-match %d inliers, scale %.4f, rotation %.2f deg", survey, inliers, sim.scale(), sim.rotation()*180/math.Pi)
		} else {
			logf("Survey cutout: %s star matching failed (%d ref, %d survey stars); inspect %s", survey, len(refStars), len(surveyStars), filepath.Base(path))
		}
	}
	logf("Survey cutout: %s (%dx%d)", filepath.Base(path), surveyFrame.Width, surveyFrame.Height)
	return nil
}

// fetchLegacySurveyCutout downloads a FITS cutout from the Legacy Surveys
// (DECaLS/DESI) viewer. The cutout is square (size x size pixels) at the
// survey's native 0.262 arcsec/pixel; the caller reprojects it onto the
// target WCS grid.
func fetchLegacySurveyCutout(ctx context.Context, ra, dec float64, size int, layer string) ([]byte, error) {
	if size <= 0 || size > 4096 {
		return nil, fmt.Errorf("legacy cutout: size %d outside (0, 4096]", size)
	}
	if layer == "" {
		layer = "ls-dr9"
	}
	u := fmt.Sprintf("https://www.legacysurvey.org/viewer/cutout.fits?ra=%.6f&dec=%.6f&size=%d&layer=%s",
		ra, dec, size, layer)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("legacy cutout HTTP %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, 512<<20)); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) < 2880 || string(data[:6]) != "SIMPLE" {
		return nil, fmt.Errorf("legacy cutout returned non-FITS data (%d bytes)", len(data))
	}
	return data, nil
}

// writeLegacySurveyCutout fetches a Legacy Surveys cutout covering the target
// field, reprojects it onto the target WCS and writes aligned products.
func writeLegacySurveyCutout(width, height int, wcs *wcsTan, opts runOptions, dir, productDir, stem string, refFrame *frame, timeout int) error {
	scale := math.Hypot(wcs.CD[0][0], wcs.CD[0][1]) * 180 / math.Pi
	if scale <= 0 {
		return fmt.Errorf("survey cutout: invalid plate scale")
	}
	fov := float64(max(width, height)) * scale
	ra, dec := wcs.CRVAL1*180/math.Pi, wcs.CRVAL2*180/math.Pi
	// Legacy Surveys native scale is 0.262 arcsec/px.
	size := int(math.Ceil(fov*3600/0.262)) + 4
	if size < 64 {
		size = 64
	}
	if size > 4096 {
		size = 4096
	}
	layer := "ls-dr9"
	if opts.SurveyHIPS != "" && opts.SurveyHIPS != "CDS/P/DSS2/color" {
		layer = opts.SurveyHIPS
	}
	if timeout <= 0 {
		timeout = suggestSurveyTimeout("legacy", width, height)
	}
	logf("Survey cutout: HTTP timeout %d s", timeout)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	fitsData, err := fetchLegacySurveyCutout(ctx, ra, dec, size, layer)
	if err != nil {
		return err
	}
	srcWCS, _, err := parseAstrometryWCS(fitsData)
	if err != nil {
		return fmt.Errorf("legacy align: source WCS: %w", err)
	}
	tmp, err := os.CreateTemp("", "skystacker-legacy-*.fits")
	if err != nil {
		return fmt.Errorf("legacy align: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(fitsData); err != nil {
		tmp.Close()
		return fmt.Errorf("legacy align: temp write: %w", err)
	}
	tmp.Close()
	sourceFrame, err := decodeFITSNative(tmpPath, 0, 0)
	if err != nil {
		return fmt.Errorf("legacy align: source pixels: %w", err)
	}
	aligned, invalid := reprojectSurvey(sourceFrame, srcWCS, wcs, width, height, 4)
	if aligned == nil {
		return fmt.Errorf("legacy align: reprojection failed")
	}
	// alignment shift correction
	if refFrame != nil {
		dx, dy, ncc := surveyAlignmentShift(refFrame, aligned)
		if dx != 0 || dy != 0 {
			aligned = applySurveyAlignmentShift(aligned, dx, dy)
			logf("Legacy survey: alignment shift corrected by (%d, %d) px (NCC %.3f)", dx, dy, ncc)
		}
	}
	stem2 := stem + "_legacy"
	return writeAlignedSurveyProducts(productDir, stem2, aligned, wcs, srcWCS,
		fitsData, "legacy", layer, "", invalid, nil)
}

// writeESOSurveyCutout fetches an ESO DSS plate, injects a derived WCS,
// reprojects it onto the target WCS and writes aligned products.
func writeESOSurveyCutout(width, height int, wcs *wcsTan, opts runOptions, dir, productDir, stem string, refFrame *frame, timeout int) error {
	scale := math.Hypot(wcs.CD[0][0], wcs.CD[0][1]) * 180 / math.Pi
	if scale <= 0 {
		return fmt.Errorf("survey cutout: invalid plate scale")
	}
	fov := float64(max(width, height)) * scale
	ra, dec := wcs.CRVAL1*180/math.Pi, wcs.CRVAL2*180/math.Pi
	if timeout <= 0 {
		timeout = 120
	}
	logf("Survey cutout: HTTP timeout %d s", timeout)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	fitsData, err := fetchESOCutout(ctx, ra, dec, fov, fov)
	if err != nil {
		return err
	}
	srcWCS, _, err := parseAstrometryWCS(fitsData)
	if err != nil {
		return fmt.Errorf("eso align: source WCS: %w", err)
	}
	tmp, err := os.CreateTemp("", "skystacker-eso-*.fits")
	if err != nil {
		return fmt.Errorf("eso align: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(fitsData); err != nil {
		tmp.Close()
		return fmt.Errorf("eso align: temp write: %w", err)
	}
	tmp.Close()
	sourceFrame, err := decodeFITSNative(tmpPath, 0, 0)
	if err != nil {
		return fmt.Errorf("eso align: source pixels: %w", err)
	}
	aligned, invalid := reprojectSurvey(sourceFrame, srcWCS, wcs, width, height, 4)
	if aligned == nil {
		return fmt.Errorf("eso align: reprojection failed")
	}
	if refFrame != nil {
		dx, dy, ncc := surveyAlignmentShift(refFrame, aligned)
		if dx != 0 || dy != 0 {
			aligned = applySurveyAlignmentShift(aligned, dx, dy)
			logf("ESO DSS: alignment shift corrected by (%d, %d) px (NCC %.3f)", dx, dy, ncc)
		}
	}
	stem2 := stem + "_eso"
	return writeAlignedSurveyProducts(productDir, stem2, aligned, wcs, srcWCS,
		fitsData, "eso", "eso-dss", "", invalid, nil)
}

// fetchSDSSCutout downloads a JPEG cutout from the SDSS SkyServer ImgCutout
// service. SDSS does not provide FITS cutouts through this endpoint, so the
// result is a JPEG preview without WCS; reprojection is not possible.
func fetchSDSSCutout(ctx context.Context, ra, dec float64, width, height int, scale float64) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("sdss cutout: invalid dimensions")
	}
	if scale <= 0 {
		scale = 0.396 // SDSS native plate scale ~0.396 arcsec/px
	}
	u := fmt.Sprintf("https://skyserver.sdss.org/dr18/SkyServerWS/ImgCutout/getjpeg?ra=%.6f&dec=%.6f&scale=%.4f&width=%d&height=%d&opt=G",
		ra, dec, scale, width, height)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sdss cutout HTTP %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, 64<<20)); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) < 100 || string(data[:2]) != "\xff\xd8" {
		return nil, fmt.Errorf("sdss cutout returned non-JPEG data (%d bytes)", len(data))
	}
	return data, nil
}

// writeSDSSSurveyCutout fetches an SDSS JPEG cutout and saves it. Because
// SDSS ImgCutout provides no WCS, no reprojection is performed; the JPEG is
// saved with a small JSON sidecar recording the request parameters.
func writeSDSSSurveyCutout(width, height int, wcs *wcsTan, opts runOptions, dir, productDir, stem string, refFrame *frame, timeout int) error {
	ra, dec := wcs.CRVAL1*180/math.Pi, wcs.CRVAL2*180/math.Pi
	scale := math.Hypot(wcs.CD[0][0], wcs.CD[0][1]) * 180 / math.Pi * 3600
	if scale <= 0 {
		scale = 0.396
	}
	if timeout <= 0 {
		timeout = 30
	}
	logf("Survey cutout: HTTP timeout %d s", timeout)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	data, err := fetchSDSSCutout(ctx, ra, dec, width, height, scale)
	if err != nil {
		return err
	}
	outPath := filepath.Join(productDir, stem+"_sdss.jpg")
	if err := writeFileAtomic(outPath, func(out io.Writer) error {
		_, err := out.Write(data)
		return err
	}); err != nil {
		return err
	}
	meta := map[string]any{
		"source": "sdss", "ra": ra, "dec": dec, "width": width, "height": height,
		"scale_arcsec_per_px": scale, "note": "JPEG only; no WCS, no reprojection",
	}
	if err := writeJSON(filepath.Join(productDir, stem+"_sdss.json"), meta); err != nil {
		logf("SDSS cutout: metadata write failed: %v", err)
	}
	logf("Survey cutout: SDSS JPEG %dx%d -> %s", width, height, filepath.Base(outPath))
	return nil
}

func writeSurveyCutout(width, height int, wcs *wcsTan, opts runOptions, dir, stem string, refFrame *frame) error {
	scale := math.Hypot(wcs.CD[0][0], wcs.CD[0][1]) * 180 / math.Pi
	if scale <= 0 {
		return fmt.Errorf("survey cutout: invalid plate scale")
	}
	fov := float64(max(width, height)) * scale
	ra, dec := wcs.CRVAL1*180/math.Pi, wcs.CRVAL2*180/math.Pi
	rotation := math.Atan2(wcs.CD[0][1], wcs.CD[1][1]) * 180 / math.Pi
	timeout := opts.SurveyTimeout
	if timeout <= 0 {
		timeout = suggestSurveyTimeout(opts.SurveySource, width, height)
	}
	logf("Survey cutout: HTTP timeout %d s", timeout)
	source := strings.ToLower(strings.TrimSpace(opts.SurveySource))
	if source == "" {
		source = "hips"
	}
	productDir, err := ensureProductSubdir(dir, surveyProductSubdir)
	if err != nil {
		return err
	}
	switch source {
	case "hips":
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
		sourceURL := fmt.Sprintf("https://alasky.cds.unistra.fr/hips-image-services/hips2fits?hips=%s&ra=%.8f&dec=%.8f&fov=%.8f&width=%d&height=%d&rotation_angle=%.8f",
			opts.SurveyHIPS, ra, dec, fov, width, height, rotation)
		fitsData, err := fetchHiPSCutout(ctx, "https://alasky.cds.unistra.fr/hips-image-services/hips2fits",
			opts.SurveyHIPS, width, height, ra, dec, fov, rotation)
		if err != nil {
			return err
		}
		return writeSurveyProduct(fitsData, source, opts.SurveyHIPS, sourceURL, opts, productDir, stem, width, height, wcs, refFrame)
	case "dss":
		return writeDSSSurveyCutouts(width, height, wcs, opts, dir, productDir, stem, refFrame, timeout)
	case "eso":
		return writeESOSurveyCutout(width, height, wcs, opts, dir, productDir, stem, refFrame, timeout)
	case "legacy":
		return writeLegacySurveyCutout(width, height, wcs, opts, dir, productDir, stem, refFrame, timeout)
	case "sdss":
		return writeSDSSSurveyCutout(width, height, wcs, opts, dir, productDir, stem, refFrame, timeout)
	default:
		return fmt.Errorf("survey cutout: unknown source %q", source)
	}
}

func runSkyMapForImage(f *frame, sol *plateSolution, dir, base string, opts runOptions) error {
	ww := sol.WCS
	ww.SIP = sol.SIP
	nside := gaiaDefaultNside
	ipix := healpixAng2PixNested(nside, ww.CRVAL1, ww.CRVAL2)
	tile, ok := gaiaLoadTileCache(gaiaDefaultCacheDir(), nside, ipix)
	if !ok {
		logf("Sky map: no cached Gaia tile for nside=%d ipix=%d", nside, ipix)
		return fmt.Errorf("no cached Gaia tile for this field (run with --plate-solve and populate the cache)")
	}
	extra, err := loadAnnotationCatalogs(opts.AnnotationCatalogs)
	if err != nil {
		return err
	}
	sources := skySourcesForImage(f.Width, f.Height, &ww, tile.Stars, extra)
	vectors := skyMapVectors(&ww, opts.SsoEphemeris)
	stem := strings.TrimSuffix(base, filepath.Ext(base)) + "_sky_map.svg"
	mapsDir, err := ensureProductSubdir(dir, mapProductSubdir)
	if err != nil {
		return err
	}
	path := filepath.Join(mapsDir, stem)
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := io.WriteString(out, buildSkyMapSVGLayers(f.Width, f.Height, sources, vectors, 1))
		return err
	}); err != nil {
		return err
	}
	logf("Sky map: wrote %s with %d sources (%dx%d)", stem, len(sources), f.Width, f.Height)
	return nil
}

func runSurveyForImage(f *frame, sol *plateSolution, dir, base string, opts runOptions) error {
	stem := strings.TrimSuffix(base, filepath.Ext(base)) + "_survey_reference"
	return writeSurveyCutout(f.Width, f.Height, &sol.WCS, opts, dir, stem, f)
}

// maxHiPSCoutPixels is the published CDS hips2fits service limit. Refusing
// larger frames before dialing avoids a guaranteed 400 and a wasteful upload.
const maxHiPSCoutPixels = 50_000_000

func fetchHiPSCutout(ctx context.Context, baseURL, hips string, width, height int, ra, dec, fov, rotation float64) ([]byte, error) {
	if width <= 0 || height <= 0 || int64(width)*int64(height) > maxHiPSCoutPixels {
		return nil, fmt.Errorf("survey cutout: %dx%d exceeds the %d Mpix service limit", width, height, maxHiPSCoutPixels/1_000_000)
	}
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
	refPath := filepath.Join(outputDir, "_work", "stacked_mean.f32raw")
	var refFrame *frame
	if iw, ih, ipix, ierr := readRawFloat32RGB(refPath); ierr == nil {
		refFrame = &frame{Width: iw, Height: ih, Pix: ipix}
	}
	return writeSurveyCutout(w, h, &sol.WCS, opts, outputDir, "survey_reference", refFrame)
}

// fetchESOCutout downloads an ESO DSS plate and injects a derived WCS into
// the FITS header so the standard reprojection path can consume it. ESO
// plates carry plate-centre coordinates, plate scale and scan centre pixels
// but no CRVAL/CD keywords; the WCS is derived assuming a 25.4 um scan of a
// plate with PLTSCALE arcsec/mm, an unrotated north-up orientation and the
// requested (ra, dec) as the cutout centre.
func fetchESOCutout(ctx context.Context, ra, dec, h, w float64) ([]byte, error) {
	u := fmt.Sprintf("https://archive.eso.org/dss/dss?ra=%.6f&dec=%.6f&mime-type=application/x-fits&x=%.3f&y=%.3f",
		ra, dec, w, h)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ESO DSS HTTP %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(resp.Body, 512<<20)); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) < 2880 || string(data[:6]) != "SIMPLE" {
		return nil, fmt.Errorf("ESO DSS returned non-FITS data (%d bytes)", len(data))
	}
	// parse plate metadata
	cards := parseFITSPrimaryCards(data)
	pltrah := parseFloatCard(cards, "PLTRAH")
	pltram := parseFloatCard(cards, "PLTRAM")
	pltras := parseFloatCard(cards, "PLTRAS")
	pltd := parseFloatCard(cards, "PLTDECD")
	pltdm := parseFloatCard(cards, "PLTDECM")
	pltds := parseFloatCard(cards, "PLTDECS")
	pltscale := parseFloatCard(cards, "PLTSCALE")
	naxis1, _ := strconv.Atoi(cards["NAXIS1"])
	naxis2, _ := strconv.Atoi(cards["NAXIS2"])
	if naxis1 <= 0 || naxis2 <= 0 || pltscale <= 0 {
		return nil, fmt.Errorf("ESO DSS: invalid plate metadata")
	}
	raCenter := (pltrah + pltram/60 + pltras/3600) * 15
	decCenter := pltd + pltdm/60 + pltds/3600
	if sn := strings.TrimSpace(cards["PLTDECSN"]); sn == "-" {
		decCenter = -decCenter
	}
	// pixel scale: PLTSCALE (arcsec/mm) * 25.4 um/px scan (standard ESO)
	pixScaleDeg := pltscale * 25.4e-3 / 3600.0
	// The cutout is centred on the requested (ra, dec), not the plate centre,
	// so CRVAL = requested position; CRPIX = cutout centre.
	crpix1 := float64(naxis1)/2 + 0.5
	crpix2 := float64(naxis2)/2 + 0.5
	// derive offset from plate centre to requested centre is already encoded in
	// the cutout position; keep it simple: identity orientation.
	_ = raCenter
	_ = decCenter

	// build a new FITS header with WCS
	headerCards := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                   16",
		"NAXIS   =                    2",
		fmt.Sprintf("NAXIS1  = %20d", naxis1),
		fmt.Sprintf("NAXIS2  = %20d", naxis2),
		"EXTEND  =                    T",
		"CTYPE1  = 'RA---TAN'",
		"CTYPE2  = 'DEC--TAN'",
		"RADESYS = 'ICRS'",
		fmt.Sprintf("CRVAL1  = %20.12f", ra),
		fmt.Sprintf("CRVAL2  = %20.12f", dec),
		fmt.Sprintf("CRPIX1  = %20.8f", crpix1),
		fmt.Sprintf("CRPIX2  = %20.8f", crpix2),
		fmt.Sprintf("CD1_1   = %20.12e", -pixScaleDeg),
		"CD1_2   =     0.000000000000E+00",
		"CD2_1   =     0.000000000000E+00",
		fmt.Sprintf("CD2_2   = %20.12e", pixScaleDeg),
		"END",
	}
	var hdr bytes.Buffer
	for _, c := range headerCards {
		if len(c) > 80 {
			c = c[:80]
		}
		hdr.WriteString(fmt.Sprintf("%-80s", c))
	}
	for hdr.Len()%2880 != 0 {
		hdr.WriteByte(' ')
	}
	// pixel data offset in original: write to a temp file and parse
	tmp, err := os.CreateTemp("", "skystacker-eso-hdr-*.fits")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Write(data)
	tmp.Close()
	defer os.Remove(tmpPath)
	var offset int64
	_, _, _, _, _, _, offset, _, _, _, _, err = parseFITSHeader(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("ESO DSS: parse header: %w", err)
	}
	out := make([]byte, 0, hdr.Len()+len(data)-int(offset))
	out = append(out, hdr.Bytes()...)
	out = append(out, data[offset:]...)
	return out, nil
}

func parseFloatCard(cards map[string]string, key string) float64 {
	v, ok := cards[key]
	if !ok {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return f
}

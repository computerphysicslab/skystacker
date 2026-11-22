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
	path := filepath.Join(outputDir, "stacked_sky_map.svg")
	if err := writeFileAtomic(path, func(out io.Writer) error {
		_, err := io.WriteString(out, buildSkyMapSVGLayers(w, h, sources, vectors, 1))
		return err
	}); err != nil {
		return err
	}
	logf("Sky map: wrote %s with %d sources (%dx%d)", filepath.Base(path), len(sources), w, h)
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
		timeout = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	fitsData, err := fetchHiPSCutout(ctx, "https://alasky.cds.unistra.fr/hips-image-services/hips2fits",
		opts.SurveyHIPS, width, height, ra, dec, fov, rotation)
	if err != nil {
		return err
	}
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
	survey, err := decodeFITSNative(path, 0, 0)
	if err != nil {
		logf("Survey cutout: FITS saved but preview failed: %v", err)
		return nil
	}
	jpg := filepath.Join(dir, stem+".jpg")
	if err := writeFileAtomic(jpg, func(out io.Writer) error {
		return writePreviewJPEG(out, survey.Width, survey.Height, survey.Pix)
	}); err != nil {
		logf("Survey cutout: preview: %v", err)
	}
	if refFrame != nil && refFrame.Width == survey.Width && refFrame.Height == survey.Height {
		refStars := detectStars(refFrame, 300)
		surveyStars := detectStars(survey, 300)
		sim, inliers, ok := matchStarsToReference(refStars, surveyStars)
		if ok {
			logf("Survey cutout: star-match %d inliers, scale %.4f, rotation %.2f deg", inliers, sim.scale(), sim.rotation()*180/math.Pi)
		} else {
			logf("Survey cutout: star matching failed (%d ref, %d survey stars); inspect %s", len(refStars), len(surveyStars), filepath.Base(path))
		}
	}
	logf("Survey cutout: %s (%dx%d, FOV %.4f deg, rotation %.2f deg)", filepath.Base(path), width, height, fov, rotation)
	return nil
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
	path := filepath.Join(dir, stem)
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
	refPath := filepath.Join(outputDir, "_work", "stacked_mean.f32raw")
	var refFrame *frame
	if iw, ih, ipix, ierr := readRawFloat32RGB(refPath); ierr == nil {
		refFrame = &frame{Width: iw, Height: ih, Pix: ipix}
	}
	return writeSurveyCutout(w, h, &sol.WCS, opts, outputDir, "survey_reference", refFrame)
}

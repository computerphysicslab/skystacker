package main

// Layered annotation subsystem (roadmap goal 144, items 137-141).
//
// Standard-library only. This file owns a single AnnotationSource data model
// shared by the SVG 2 renderer and the raster renderer, a small offline catalog
// bundle, VO acquisition with provenance, a deterministic label placement
// engine and parity-oriented drawing primitives. Existing sky-map and
// annotation entry points keep working as thin wrappers around these renderers.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Unified annotation model (item 137)
// ---------------------------------------------------------------------------

// annotationProvenance records where an annotation source came from. Online
// entries carry the service and query time; offline entries are marked so a
// report can be audited without network access.
type annotationProvenance struct {
	Catalog   string `json:"catalog"`
	Service   string `json:"service,omitempty"`
	URL       string `json:"url,omitempty"`
	Retrieved string `json:"retrieved,omitempty"`
	Offline   bool   `json:"offline"`
	Truncated bool   `json:"truncated,omitempty"`
}

// annotationSource is the single interchange type consumed by every renderer.
// Sky positions are radians, errors and axes are arcseconds, epoch is a decimal
// year and proper motions are mas/year. Pixel fields are never serialised.
type annotationSource struct {
	ID         string               `json:"id"`
	Name       string               `json:"name,omitempty"`
	Type       string               `json:"type"`
	RA         float64              `json:"ra"`
	Dec        float64              `json:"dec"`
	RAError    float64              `json:"ra_error_arcsec,omitempty"`
	DecError   float64              `json:"dec_error_arcsec,omitempty"`
	Band       string               `json:"band,omitempty"`
	Mag        float64              `json:"mag,omitempty"`
	Major      float64              `json:"major_arcsec,omitempty"`
	Minor      float64              `json:"minor_arcsec,omitempty"`
	PA         float64              `json:"pa_deg,omitempty"`
	Epoch      float64              `json:"epoch,omitempty"`
	PMRA       float64              `json:"pmra_mas_yr,omitempty"`
	PMDec      float64              `json:"pmdec_mas_yr,omitempty"`
	Priority   int                  `json:"priority"`
	Provenance annotationProvenance `json:"provenance"`
	Metadata   map[string]string    `json:"metadata,omitempty"`
	Extended   bool                 `json:"extended,omitempty"`
	X          float64              `json:"-"`
	Y          float64              `json:"-"`
	PixelA     float64              `json:"-"`
	PixelB     float64              `json:"-"`
	PixelPA    float64              `json:"-"`
}

// constellationSegment joins two named stars of the embedded bundle.
type constellationSegment struct {
	Constellation string
	From          string
	To            string
}

// annotationBundle is the complete offline catalog bundle, built once from
// compact string tables so the process never loads a heavy catalog file.
type annotationBundle struct {
	Version        string
	Sources        []annotationSource
	Constellations []constellationSegment
}

var (
	bundleOnce sync.Once
	bundleData annotationBundle
	bundleByID map[string]annotationSource
)

// Compact tables: name|RA(deg)|Dec(deg)|mag|type. J2000 values rounded to
// 0.001 deg, well below one pixel for the wide fields they target.
const offlineBrightStars = `Sirius|101.287|-16.716|-1.46|star
Canopus|95.988|-52.696|-0.72|star
Arcturus|213.915|19.182|-0.05|star
Vega|279.235|38.784|0.03|star
Capella|79.172|45.998|0.08|star
Rigel|78.634|-8.202|0.13|star
Procyon|114.825|5.225|0.34|star
Betelgeuse|88.793|7.407|0.50|star
Achernar|24.429|-57.237|0.46|star
Altair|297.696|8.868|0.77|star
Aldebaran|68.980|16.509|0.85|star
Spica|201.298|-11.161|0.97|star
Antares|247.352|-26.432|1.06|star
Pollux|116.329|28.026|1.14|star
Fomalhaut|344.413|-29.622|1.16|star
Deneb|310.358|45.280|1.25|star
Regulus|152.093|11.967|1.35|star
Castor|113.650|31.888|1.58|star
Alnitak|85.190|-1.943|1.77|star
Alnilam|84.053|-1.202|1.69|star
Mintaka|83.002|-0.299|2.23|star
Saiph|86.940|-9.670|2.06|star
Bellatrix|81.283|6.350|1.64|star
Sadr|305.557|40.257|2.23|star
Gienah Cygni|303.407|33.970|2.48|star
Delta Cygni|296.244|45.131|2.87|star
Albireo|292.680|27.960|3.05|star
Schedar|10.127|56.537|2.24|star
Caph|2.294|59.150|2.27|star
Gamma Cassiopeiae|14.177|60.717|2.15|star
Ruchbah|21.454|60.235|2.68|star
Segin|27.036|63.670|3.35|star
Dubhe|165.932|61.751|1.79|star
Merak|165.460|56.382|2.37|star
Phecda|178.458|53.695|2.44|star
Megrez|183.857|57.033|3.31|star
Alioth|193.507|55.960|1.77|star
Mizar|200.981|54.925|2.23|star
Alkaid|206.885|49.313|1.86|star
Shaula|263.402|-37.104|1.62|star
Sargas|263.695|-42.998|1.87|star
Graffias|241.360|-25.592|2.62|star
Dschubba|240.083|-22.622|2.29|star
Elnath|81.573|28.608|1.65|star
Hyadum I|65.384|15.628|3.76|star
Hyadum II|66.948|15.631|3.79|star
Polaris|37.955|89.264|1.98|star`

// name|RA|Dec|mag|major(')|minor(')|PA(deg)|type
const offlineDeepSky = `M31|10.685|41.269|3.4|178|63|35|galaxy
M33|23.462|30.660|5.7|73|45|23|galaxy
M42|83.822|-5.391|4.0|85|60|30|nebula
M43|83.818|-5.267|9.0|20|15|0|nebula
M45|56.750|24.117|1.6|110|70|0|cluster
M78|86.752|0.075|8.3|8|6|0|nebula
M8|270.904|-24.387|6.0|90|40|0|nebula
M13|250.423|36.461|5.8|20|20|0|cluster
M16|274.700|-13.807|6.4|35|28|0|nebula
M17|275.196|-16.172|6.0|46|37|0|nebula
M20|270.025|-23.011|6.3|28|28|0|nebula
M27|299.901|22.721|7.5|8|6|135|planetary
M51|202.470|47.195|8.4|11|7|170|galaxy
M57|283.396|33.029|8.8|2|1|60|planetary
NGC 7000|314.750|44.330|4.0|120|100|0|nebula
NGC 2024|85.752|-1.850|10.0|30|20|0|nebula
IC 2118|84.500|-7.150|13.0|180|60|0|nebula
NGC 6960|311.600|30.720|7.0|70|6|35|remnant
NGC 6974|311.400|31.100|9.0|20|10|45|remnant
NGC 6992|312.800|31.720|7.0|60|8|155|remnant
NGC 6888|303.050|38.020|7.4|20|15|0|nebula
NGC 5078|199.877|-25.513|11.2|4|2|35|galaxy`

const offlineDarkNebulae = `LDN 846|311.000|31.500|0|40|20|0|dark-nebula
LDN 864|312.200|31.900|0|35|18|0|dark-nebula
LDN 868|313.500|32.000|0|30|15|0|dark-nebula
Sh2-103|311.200|31.700|0|50|30|0|dark-nebula
Sh2-293|109.600|-9.200|0|45|25|0|dark-nebula
Sh2-294|110.100|-9.000|0|30|20|0|dark-nebula
Barnard 33|85.200|-2.450|0|8|5|0|dark-nebula`

const offlineConstellations = `Orion|Betelgeuse|Bellatrix
Orion|Bellatrix|Mintaka
Orion|Mintaka|Alnilam
Orion|Alnilam|Alnitak
Orion|Alnitak|Saiph
Orion|Saiph|Rigel
Orion|Rigel|Mintaka
Orion|Alnitak|Betelgeuse
Cygnus|Deneb|Sadr
Cygnus|Sadr|Albireo
Cygnus|Sadr|Gienah Cygni
Cygnus|Sadr|Delta Cygni
Cassiopeia|Caph|Schedar
Cassiopeia|Schedar|Gamma Cassiopeiae
Cassiopeia|Gamma Cassiopeiae|Ruchbah
Cassiopeia|Ruchbah|Segin
Ursa Major|Dubhe|Merak
Ursa Major|Merak|Phecda
Ursa Major|Phecda|Megrez
Ursa Major|Megrez|Alioth
Ursa Major|Alioth|Mizar
Ursa Major|Mizar|Alkaid
Ursa Major|Megrez|Dubhe
Scorpius|Graffias|Dschubba
Scorpius|Dschubba|Antares
Scorpius|Antares|Sargas
Scorpius|Sargas|Shaula
Taurus|Elnath|Aldebaran
Taurus|Aldebaran|Hyadum I
Taurus|Hyadum I|Hyadum II`

const offlineBundleVersion = "2026-11-22-1"

func parseBrightStarTable(table string, provenance annotationProvenance) []annotationSource {
	out := make([]annotationSource, 0, 64)
	for _, line := range strings.Split(strings.TrimSpace(table), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) != 5 {
			continue
		}
		ra, e1 := strconv.ParseFloat(fields[1], 64)
		dec, e2 := strconv.ParseFloat(fields[2], 64)
		mag, e3 := strconv.ParseFloat(fields[3], 64)
		if e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		out = append(out, annotationSource{
			ID: "bundle-star-" + fields[0], Name: fields[0], Type: fields[4],
			RA: ra * math.Pi / 180, Dec: dec * math.Pi / 180, Mag: mag,
			Priority: 80, Provenance: provenance,
		})
	}
	return out
}

func parseDeepSkyTable(table string, provenance annotationProvenance, basePriority int) []annotationSource {
	out := make([]annotationSource, 0, 64)
	for _, line := range strings.Split(strings.TrimSpace(table), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) != 8 {
			continue
		}
		vals := make([]float64, 6)
		bad := false
		for i := 1; i <= 6; i++ {
			v, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				bad = true
				break
			}
			vals[i-1] = v
		}
		if bad {
			continue
		}
		out = append(out, annotationSource{
			ID: "bundle-dso-" + fields[0], Name: fields[0], Type: fields[7],
			RA: vals[0] * math.Pi / 180, Dec: vals[1] * math.Pi / 180, Mag: vals[2],
			Major: vals[3] * 60, Minor: vals[4] * 60, PA: vals[5],
			Priority: basePriority, Provenance: provenance, Extended: isExtendedAnnotationType(fields[7]),
		})
	}
	return out
}

// isExtendedAnnotationType reports whether a catalog type is drawn as an
// ellipse rather than a point.
func isExtendedAnnotationType(kind string) bool {
	switch strings.ToLower(kind) {
	case "star", "gaia", "cone", "sso", "":
		return false
	default:
		return true
	}
}

func parseConstellationTable(table string) []constellationSegment {
	out := make([]constellationSegment, 0, 64)
	for _, line := range strings.Split(strings.TrimSpace(table), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) == 3 {
			out = append(out, constellationSegment{Constellation: fields[0], From: fields[1], To: fields[2]})
		}
	}
	return out
}

// offlineAnnotationBundle returns the embedded catalog bundle, built once.
func offlineAnnotationBundle() annotationBundle {
	bundleOnce.Do(func() {
		prov := annotationProvenance{Catalog: offlineBundleVersion, Service: "offline-bundle", Offline: true}
		sources := make([]annotationSource, 0, 100)
		sources = append(sources, parseBrightStarTable(offlineBrightStars, prov)...)
		sources = append(sources, parseDeepSkyTable(offlineDeepSky, prov, 90)...)
		sources = append(sources, parseDeepSkyTable(offlineDarkNebulae, prov, 60)...)
		bundleData = annotationBundle{
			Version:        offlineBundleVersion,
			Sources:        sources,
			Constellations: parseConstellationTable(offlineConstellations),
		}
		bundleByID = make(map[string]annotationSource, len(bundleData.Sources))
		for _, s := range bundleData.Sources {
			bundleByID[s.Name] = s
		}
	})
	return bundleData
}

// annotationFromCatalogEntry converts an external JSON catalog entry into the
// unified model, preserving its type and axis information.
func annotationFromCatalogEntry(e annotationCatalogEntry, source string) annotationSource {
	return annotationSource{
		ID: "catalog-" + e.Name, Name: e.Name, Type: e.Type,
		RA: e.RA * math.Pi / 180, Dec: e.Dec * math.Pi / 180, Mag: e.Mag,
		Major: e.Major * 60, Minor: e.Minor * 60, PA: e.PA,
		Priority: 70, Extended: e.Major > 0,
		Provenance: annotationProvenance{Catalog: source, Offline: true},
	}
}

// annotationFromSkyMapSource converts a legacy sky-map source into the model.
func annotationFromSkyMapSource(s skyMapSource) annotationSource {
	return annotationSource{
		ID: fmt.Sprintf("sky-%.3f-%.3f", s.X, s.Y), Name: s.Name, Type: s.Kind,
		RA: s.RA, Dec: s.Dec, Mag: s.Mag, Major: s.Major, Minor: s.Minor, PA: s.PA,
		Priority: 30, Extended: s.Extended,
		Provenance: annotationProvenance{Catalog: "gaia-cache", Offline: true},
		X:          s.X, Y: s.Y,
	}
}

// annotationArcsecPerPixel returns the local plate scale from the CD matrix.
func annotationArcsecPerPixel(w *wcsTan) float64 {
	if w == nil {
		return 1
	}
	det := math.Abs(w.CD[0][0]*w.CD[1][1] - w.CD[0][1]*w.CD[1][0])
	if det <= 0 {
		return 1
	}
	// CD entries are radians/pixel: rad -> deg -> arcsec.
	return math.Sqrt(det) * (180 / math.Pi) * 3600
}

// applyProperMotion propagates a source from its catalog epoch to 2026.0.
func applyProperMotion(s annotationSource, targetEpoch float64) annotationSource {
	if s.Epoch == 0 || (s.PMRA == 0 && s.PMDec == 0) {
		return s
	}
	dtYears := targetEpoch - s.Epoch
	cosDec := math.Max(1e-6, math.Cos(s.Dec))
	s.RA += (s.PMRA * dtYears / 1000 / cosDec) * math.Pi / 180
	s.Dec += (s.PMDec * dtYears / 1000) * math.Pi / 180
	return s
}

// projectAnnotationSources filters sources to the frame and fills pixel
// geometry: the single place where sky coordinates become pixels.
func projectAnnotationSources(w *wcsTan, width, height int, in []annotationSource) []annotationSource {
	if w == nil || width <= 0 || height <= 0 {
		return nil
	}
	scale := annotationArcsecPerPixel(w)
	out := make([]annotationSource, 0, len(in))
	for _, src := range in {
		s := applyProperMotion(src, 2026.0)
		x, y := wcsTanForward(w, s.RA, s.Dec)
		if x < 0 || y < 0 || x >= float64(width) || y >= float64(height) {
			continue
		}
		s.X, s.Y = x, y
		s.PixelA, s.PixelB = s.Major/scale, s.Minor/scale
		s.PixelPA = correctedPA(w, s.PA)
		out = append(out, s)
	}
	return out
}

// cachedGaiaStarsForField unions every cached Gaia tile that intersects the
// solved frame. Reading only the tile that contains CRVAL truncates wide
// fields: a single HEALPix tile is smaller than a typical APS-C frame, so
// stars and extended objects near the far corners were silently missing from
// both sky maps (demo-test.md, "Distribucion geometrica truncada").
//
// The frame is sampled on a 9x9 grid (including points just outside the
// borders) and each sample is mapped to its nested HEALPix tile. Tiles are
// visited in sorted order and stars are de-duplicated, so the result is
// deterministic and safe to feed into the stable renderers.
func cachedGaiaStarsForField(cacheDir string, nside int, w *wcsTan, width, height int) []gaiaStar {
	if w == nil || nside < 1 || width <= 0 || height <= 0 {
		return nil
	}
	const samples = 9
	seen := make(map[int]bool)
	for gy := 0; gy < samples; gy++ {
		for gx := 0; gx < samples; gx++ {
			x := (float64(gx) - 0.5) * float64(width) / float64(samples-1)
			y := (float64(gy) - 0.5) * float64(height) / float64(samples-1)
			ra, dec := wcsTanInverse(w, x, y)
			seen[healpixAng2PixNested(nside, ra, dec)] = true
		}
	}
	ipixs := make([]int, 0, len(seen))
	for ipix := range seen {
		ipixs = append(ipixs, ipix)
	}
	sort.Ints(ipixs)
	var out []gaiaStar
	dedup := make(map[string]bool)
	for _, ipix := range ipixs {
		tile, ok := gaiaLoadTileCache(cacheDir, nside, ipix)
		if !ok {
			continue
		}
		for _, st := range tile.Stars {
			key := fmt.Sprintf("%.9f:%.9f", st.RA, st.Dec)
			if dedup[key] {
				continue
			}
			dedup[key] = true
			out = append(out, st)
		}
	}
	return out
}

// annotationLabelFields returns the label string of a source using the
// canonical sky-map field ordering.
func annotationLabelFields(s annotationSource, fields int) string {
	return labelText(skyMapSource{
		X: s.X, Y: s.Y, RA: s.RA, Dec: s.Dec, Mag: s.Mag, Name: s.Name,
		Kind: s.Type, Major: s.Major / 60, Minor: s.Minor / 60, PA: s.PA, Extended: s.Extended,
	}, fields)
}

// ---------------------------------------------------------------------------
// VO catalog acquisition with cache and provenance (item 138)
// ---------------------------------------------------------------------------

type annotationAcquisitionOptions struct {
	Offline      bool
	Services     []string
	CacheDir     string
	Timeout      time.Duration
	RADeg        float64
	DecDeg       float64
	RadiusArcsec float64
	SIMBADURL    string
	VizierURL    string
}

type annotationAcquisition struct {
	Sources    []annotationSource     `json:"sources"`
	Provenance []annotationProvenance `json:"provenance"`
	Warnings   []string               `json:"warnings,omitempty"`
}

var annotationCredentialPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|authorization|bearer|password|secret|passwd)([=: ]+)[^&\s"',]+`)

// redactAnnotationError removes credentials from service error messages before
// they reach a report, log or SVG description.
func redactAnnotationError(message string) string {
	return annotationCredentialPattern.ReplaceAllString(message, "$1$2[redacted]")
}

func annotationCacheFile(cacheDir, key string) string {
	if cacheDir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(cacheDir, "annvo_"+hex.EncodeToString(sum[:16])+".json")
}

func annotationReadCache(path string, dst any) bool {
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	return json.Unmarshal(data, dst) == nil
}

func annotationWriteCache(path string, value any) {
	if path == "" {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
}

// simbadTAPCone queries a SIMBAD TAP sync endpoint and parses either the TAP
// JSON envelope or a bare row array.
func simbadTAPCone(ctx context.Context, baseURL string, raDeg, decDeg, radiusArcsec float64, timeout time.Duration) ([]annotationSource, bool, error) {
	if baseURL == "" {
		baseURL = "https://simbad.u-strasbg.fr/simbad/sim-tap/sync"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	query := fmt.Sprintf("SELECT TOP 1000 basic.ra AS ra, basic.dec AS dec, basic.main_id AS name, "+
		"basic.otype AS otype, basic.V AS mag FROM basic WHERE CONTAINS(POINT('ICRS',basic.ra,basic.dec),"+
		"CIRCLE('ICRS',%g,%g,%g))=1", raDeg, decDeg, radiusArcsec/3600)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, false, err
	}
	q := u.Query()
	q.Set("REQUEST", "doQuery")
	q.Set("LANG", "ADQL")
	q.Set("FORMAT", "json")
	q.Set("QUERY", query)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, false, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, false, fmt.Errorf("SIMBAD TAP HTTP %d: %s", resp.StatusCode, redactAnnotationError(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, false, err
	}
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Data != nil {
		rows = envelope.Data
	} else if err := json.Unmarshal(body, &rows); err != nil {
		return nil, false, fmt.Errorf("SIMBAD TAP JSON: %w", err)
	}
	truncated := len(rows) >= 1000
	prov := annotationProvenance{Catalog: "SIMBAD", Service: "simbad-tap", URL: baseURL,
		Retrieved: time.Now().UTC().Format(time.RFC3339), Truncated: truncated}
	sources := make([]annotationSource, 0, len(rows))
	for _, row := range rows {
		ra, raOK := numericField(row, "ra")
		dec, decOK := numericField(row, "dec")
		if !raOK || !decOK {
			continue
		}
		name := stringField(row, "name", "main_id", "id")
		kind := stringField(row, "otype", "type")
		if kind == "" {
			kind = "cone"
		}
		mag, _ := numericField(row, "mag", "V", "flux")
		sources = append(sources, annotationSource{
			ID: "simbad-" + name, Name: name, Type: kind, RA: ra * math.Pi / 180,
			Dec: dec * math.Pi / 180, Mag: mag, Priority: 75, Provenance: prov,
			Extended: isExtendedAnnotationType(kind),
		})
	}
	return sources, truncated, nil
}

func numericField(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok || v == nil {
			continue
		}
		switch n := v.(type) {
		case float64:
			return n, true
		case json.Number:
			f, err := n.Float64()
			return f, err == nil
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
			if err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func stringField(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// acquireAnnotationSources assembles offline and optional online sources with
// per-source provenance. Network failures degrade to the offline bundle with a
// redacted warning; they never abort an offline run.
func acquireAnnotationSources(ctx context.Context, o annotationAcquisitionOptions) (annotationAcquisition, error) {
	bundle := offlineAnnotationBundle()
	result := annotationAcquisition{
		Sources: append([]annotationSource(nil), bundle.Sources...),
		Provenance: []annotationProvenance{{
			Catalog: bundle.Version, Service: "offline-bundle", Offline: true,
		}},
	}
	if o.Offline {
		return result, nil
	}
	for _, service := range o.Services {
		switch strings.ToLower(strings.TrimSpace(service)) {
		case "":
			continue
		case "simbad":
			key := fmt.Sprintf("simbad|%.6f|%.6f|%.1f", o.RADeg, o.DecDeg, o.RadiusArcsec)
			path := annotationCacheFile(o.CacheDir, key)
			var sources []annotationSource
			var truncated bool
			if annotationReadCache(path, &sources) {
				result.Provenance = append(result.Provenance, annotationProvenance{Catalog: "SIMBAD", Service: "cache", Offline: true, URL: path})
			} else {
				var err error
				sources, truncated, err = simbadTAPCone(ctx, o.SIMBADURL, o.RADeg, o.DecDeg, o.RadiusArcsec, o.Timeout)
				if err != nil {
					result.Warnings = append(result.Warnings, redactAnnotationError("SIMBAD: "+err.Error()))
					continue
				}
				annotationWriteCache(path, sources)
				result.Provenance = append(result.Provenance, annotationProvenance{Catalog: "SIMBAD", Service: "simbad-tap", URL: o.SIMBADURL, Retrieved: time.Now().UTC().Format(time.RFC3339), Truncated: truncated})
			}
			result.Sources = append(result.Sources, sources...)
		case "vizier":
			svc := coneServices["vizier"]
			key := fmt.Sprintf("vizier|%.6f|%.6f|%.1f", o.RADeg, o.DecDeg, o.RadiusArcsec)
			path := annotationCacheFile(o.CacheDir, key)
			var cached, res coneResult
			if annotationReadCache(path, &cached) {
				res = cached
				result.Provenance = append(result.Provenance, annotationProvenance{Catalog: "VizieR", Service: "cache", Offline: true, URL: path})
			} else {
				var err error
				res, err = coneSearchDetailed(svc, o.VizierURL, o.RADeg, o.DecDeg, o.RadiusArcsec, o.Timeout)
				if err != nil {
					result.Warnings = append(result.Warnings, redactAnnotationError("VizieR: "+err.Error()))
					continue
				}
				annotationWriteCache(path, res)
				result.Provenance = append(result.Provenance, annotationProvenance{Catalog: "VizieR", Service: "vizier-cone", URL: o.VizierURL, Retrieved: time.Now().UTC().Format(time.RFC3339), Truncated: res.Truncated})
			}
			prov := result.Provenance[len(result.Provenance)-1]
			for _, s := range res.Sources {
				result.Sources = append(result.Sources, annotationSource{
					ID: "vizier-" + s.Name, Name: s.Name, Type: "cone",
					// coneSearchDetailed already stores RA/Dec in radians.
					RA: s.RA, Dec: s.Dec, Mag: s.Mag,
					Priority: 50, Provenance: prov,
				})
			}
		default:
			return annotationAcquisition{}, fmt.Errorf("unknown annotation VO service %q", service)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Deterministic label placement (item 141)
// ---------------------------------------------------------------------------

const (
	annotationLabelCell        = 64
	annotationLabelRelaxPasses = 8
)

type placedLabel struct {
	box      labelBox
	priority int
	id       string
}

// annotationLabelPlacer is a dense uniform spatial grid. It avoids both the
// quadratic all-pairs scan and per-query map allocations, while keeping
// deterministic ordering (priority, mag, ID). Scratch buffers are reused
// across queries because callers consume them immediately.
type annotationLabelPlacer struct {
	width, height int
	fields        int
	cell          int
	cols, rows    int
	buckets       [][]int
	placed        []placedLabel
	scratch       []int
	neighboursBuf []int
	seen          []int
	seenGen       int
}

func newAnnotationLabelPlacer(width, height, fields int) *annotationLabelPlacer {
	p := &annotationLabelPlacer{
		width: width, height: height, fields: fields, cell: annotationLabelCell,
	}
	p.cols = width/p.cell + 2
	p.rows = height/p.cell + 2
	p.buckets = make([][]int, p.cols*p.rows)
	return p
}

func (p *annotationLabelPlacer) cellIndex(x, y int) int {
	if x < 0 {
		x = 0
	} else if x >= p.cols {
		x = p.cols - 1
	}
	if y < 0 {
		y = 0
	} else if y >= p.rows {
		y = p.rows - 1
	}
	return y*p.cols + x
}

// keys fills the reusable scratch slice with the flat indices of every cell
// touched by b. Labels are smaller than one cell in practice, so this is
// normally one to four entries.
func (p *annotationLabelPlacer) keys(b labelBox) []int {
	x0 := int(math.Floor(b.X / float64(p.cell)))
	y0 := int(math.Floor(b.Y / float64(p.cell)))
	x1 := int(math.Floor((b.X + b.W) / float64(p.cell)))
	y1 := int(math.Floor((b.Y + b.H) / float64(p.cell)))
	p.scratch = p.scratch[:0]
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			p.scratch = append(p.scratch, p.cellIndex(x, y))
		}
	}
	return p.scratch
}

func (p *annotationLabelPlacer) add(item placedLabel) int {
	idx := len(p.placed)
	p.placed = append(p.placed, item)
	if idx >= len(p.seen) {
		p.seen = append(p.seen, make([]int, idx-len(p.seen)+1)...)
	}
	for _, key := range p.keys(item.box) {
		p.buckets[key] = append(p.buckets[key], idx)
	}
	return idx
}

func (p *annotationLabelPlacer) neighbours(b labelBox) []int {
	p.seenGen++
	out := p.neighboursBuf[:0]
	// Every placed box is indexed in all cells it touches. Two overlapping
	// boxes necessarily share at least one cell, so the box's own cells are
	// sufficient; scanning the eight surrounding cells is redundant.
	for _, key := range p.keys(b) {
		for _, idx := range p.buckets[key] {
			if p.seen[idx] != p.seenGen {
				p.seen[idx] = p.seenGen
				out = append(out, idx)
			}
		}
	}
	p.neighboursBuf = out
	return out
}

func (p *annotationLabelPlacer) totalOverlap(b labelBox, skip int) float64 {
	total := 0.0
	for _, idx := range p.neighbours(b) {
		if idx != skip {
			total += overlapArea(b, p.placed[idx].box)
		}
	}
	return total
}

// collides reports the first overlapping placed label without summing all
// overlaps. Candidate scoring only needs a boolean, so this short-circuits in
// dense fields.
func (p *annotationLabelPlacer) collides(b labelBox, skip int) bool {
	for _, idx := range p.neighbours(b) {
		if idx != skip && overlapArea(b, p.placed[idx].box) > 0 {
			return true
		}
	}
	return false
}

func (p *annotationLabelPlacer) reindex() {
	for i := range p.buckets {
		p.buckets[i] = p.buckets[i][:0]
	}
	for i, item := range p.placed {
		for _, key := range p.keys(item.box) {
			p.buckets[key] = append(p.buckets[key], i)
		}
	}
}

// placeAnnotationLabels places labels by priority with the spatial hash, then
// relaxes overlapping pairs with deterministic minimum-translation moves.
// A final priority filter guarantees zero overlaps: a label that cannot be
// placed without collision is suppressed rather than drawn over a more
// important one.
func placeAnnotationLabels(width, height int, sources []annotationSource, fields int) []labelBox {
	p := newAnnotationLabelPlacer(width, height, fields)
	order := make([]int, len(sources))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := sources[order[i]], sources[order[j]]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if a.Mag != b.Mag {
			return a.Mag < b.Mag
		}
		return a.ID < b.ID
	})
	dirs := [8][2]float64{{1, 0}, {0.707, 0.707}, {0, 1}, {-0.707, 0.707}, {-1, 0}, {-0.707, -0.707}, {0, -1}, {0.707, -0.707}}
	for _, idx := range order {
		s := sources[idx]
		text := annotationLabelFields(s, fields)
		if text == "" {
			continue
		}
		w := float64(len(text)) * skyLabelFontSize * 0.62
		h := skyLabelFontSize + 4
		best, bestScore := labelBox{}, math.Inf(1)
		for c := 0; c < skyLabelCandidatePositions; c++ {
			d := dirs[c%8]
			radius := 11.0
			if c >= 8 {
				radius = 22.0
			}
			x, y := s.X+d[0]*radius, s.Y+d[1]*radius
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
			// Overlap is non-negative: a candidate already worse than the
			// incumbent cannot improve by checking collisions.
			if score >= bestScore {
				continue
			}
			if p.collides(cand, -1) {
				score += 2
			}
			if score < bestScore {
				bestScore, best = score, cand
			}
		}
		p.add(placedLabel{box: best, priority: s.Priority, id: s.ID})
	}
	for range annotationLabelRelaxPasses {
		moved := false
		for i := range p.placed {
			for _, j := range p.neighbours(p.placed[i].box) {
				if j <= i {
					continue
				}
				a, b := p.placed[i].box, p.placed[j].box
				dx := math.Min(a.X+a.W, b.X+b.W) - math.Max(a.X, b.X)
				dy := math.Min(a.Y+a.H, b.Y+b.H) - math.Max(a.Y, b.Y)
				if dx <= 0 || dy <= 0 {
					continue
				}
				low, high := i, j
				if p.placed[i].priority < p.placed[j].priority {
					low, high = j, i
				}
				box := p.placed[low].box
				if dx < dy {
					if box.X < p.placed[high].box.X {
						box.X -= dx
					} else {
						box.X += dx
					}
				} else if box.Y < p.placed[high].box.Y {
					box.Y -= dy
				} else {
					box.Y += dy
				}
				box.X = math.Max(2, math.Min(box.X, float64(width)-box.W-2))
				box.Y = math.Max(2, math.Min(box.Y, float64(height)-box.H-2))
				p.placed[low].box = box
				moved = true
			}
		}
		if moved {
			p.reindex()
		} else {
			break
		}
	}
	// Final priority filter reuses a spatial hash instead of an all-pairs scan,
	// keeping the 10k-label acceptance criterion below 200 ms.
	accepted := make([]labelBox, 0, len(p.placed))
	filter := newAnnotationLabelPlacer(width, height, fields)
	for _, item := range p.placed {
		if filter.totalOverlap(item.box, -1) > 0 {
			continue
		}
		accepted = append(accepted, item.box)
		filter.add(placedLabel{box: item.box, priority: item.priority, id: item.id})
	}
	return accepted
}

// ---------------------------------------------------------------------------
// SVG 2 layered renderer (item 139)
// ---------------------------------------------------------------------------

// annotationRenderOptions selects layers and carries the WCS needed for grids,
// scale bar and finding chart. An empty Layers slice enables all base layers;
// optional layers require ShowGrid/ShowConstellations/ShowFinderChart.
type annotationRenderOptions struct {
	WCS                *wcsTan
	Layers             []string
	LabelFields        int
	Provenance         []annotationProvenance
	ShowGrid           bool
	ShowConstellations bool
	ShowFinderChart    bool
}

func (o annotationRenderOptions) enabled(name string) bool {
	if len(o.Layers) == 0 {
		switch name {
		case "grid-radec", "grid-galactic", "grid-ecliptic", "constellations", "finder-chart":
			return o.ShowGrid || o.ShowConstellations || o.ShowFinderChart
		default:
			return true
		}
	}
	for _, l := range o.Layers {
		if strings.TrimSpace(l) == name {
			return true
		}
	}
	return false
}

// galacticToEquatorial converts J2000 galactic longitude/latitude (degrees)
// into equatorial RA/Dec (radians).
func galacticToEquatorial(lDeg, bDeg float64) (ra, dec float64) {
	l, b := lDeg*math.Pi/180, bDeg*math.Pi/180
	const (
		raPole  = 192.85948 * math.Pi / 180
		decPole = 27.12825 * math.Pi / 180
		lNorth  = 122.93192 * math.Pi / 180
	)
	sinB, cosB := math.Sin(b), math.Cos(b)
	dl := l - lNorth
	dec = math.Asin(sinB*math.Sin(decPole) + cosB*math.Cos(decPole)*math.Cos(dl))
	y := cosB * math.Sin(dl)
	x := sinB*math.Cos(decPole) - cosB*math.Sin(decPole)*math.Cos(dl)
	ra = math.Mod(raPole+math.Atan2(y, x), 2*math.Pi)
	if ra < 0 {
		ra += 2 * math.Pi
	}
	return ra, dec
}

// eclipticToEquatorial converts J2000 ecliptic longitude/latitude (degrees)
// into equatorial RA/Dec (radians).
func eclipticToEquatorial(lonDeg, latDeg float64) (ra, dec float64) {
	lon, lat := lonDeg*math.Pi/180, latDeg*math.Pi/180
	const eps = 23.4392911 * math.Pi / 180
	sinLat, cosLat := math.Sin(lat), math.Cos(lat)
	dec = math.Asin(sinLat*math.Cos(eps) + cosLat*math.Sin(eps)*math.Sin(lon))
	y := math.Sin(lon)*math.Cos(eps) - math.Tan(lat)*math.Sin(eps)
	ra = math.Mod(math.Atan2(y, math.Cos(lon)), 2*math.Pi)
	if ra < 0 {
		ra += 2 * math.Pi
	}
	return ra, dec
}

// annotationGridStep chooses a graticule spacing showing ~6-12 lines.
func annotationGridStep(w *wcsTan, width, height int) float64 {
	scale := annotationArcsecPerPixel(w)
	spanDeg := math.Hypot(float64(width), float64(height)) * scale / 3600
	// Walk from coarse to fine and keep the largest spacing that still draws
	// at least six lines across the frame. The reverse order is essential:
	// starting at 0.02 deg generated millions of redundant path points.
	for _, s := range []float64{60, 45, 30, 15, 10, 5, 2, 1, 0.5, 0.2, 0.1, 0.05, 0.02} {
		if spanDeg/s >= 6 {
			return s
		}
	}
	return 0.02
}

// annotationGridPath projects one graticule family into SVG path commands.
// Sampling on the sky keeps the code WCS-agnostic apart from the TAN forward.
func annotationGridPath(w *wcsTan, width, height int, kind string, step float64) string {
	var b strings.Builder
	addSegment := func(points [][2]float64) {
		run := make([]string, 0, 32)
		flush := func() {
			if len(run) > 1 {
				b.WriteString(strings.Join(run, " "))
				b.WriteString(" ")
			}
			run = run[:0]
		}
		for _, p := range points {
			if len(run) > 0 {
				var px, py float64
				fmt.Sscanf(run[len(run)-1], "%g,%g", &px, &py)
				if math.Abs(p[0]-px) > float64(width)/2 || math.Abs(p[1]-py) > float64(height)/2 {
					flush()
				}
			}
			run = append(run, fmt.Sprintf("%.1f,%.1f", p[0], p[1]))
		}
		flush()
	}
	project := func(ra, dec float64) [2]float64 {
		x, y := wcsTanForward(w, ra, dec)
		return [2]float64{x, y}
	}
	inside := func(p [2]float64) bool {
		return p[0] >= -20 && p[1] >= -20 && p[0] <= float64(width)+20 && p[1] <= float64(height)+20
	}
	collect := func(toSky func(float64, float64) (float64, float64), firstRange, secondRange [2]float64, secondStep float64) {
		for a := firstRange[0]; a < firstRange[1]; a += step {
			pts := make([][2]float64, 0, 361)
			for b := secondRange[0]; b <= secondRange[1]; b += secondStep {
				ra, dec := toSky(a, b)
				p := project(ra, dec)
				if inside(p) {
					pts = append(pts, p)
				} else if len(pts) > 1 {
					addSegment(pts)
					pts = pts[:0]
				} else {
					pts = pts[:0]
				}
			}
			addSegment(pts)
		}
	}
	switch kind {
	case "grid-radec":
		collect(func(ra, dec float64) (float64, float64) { return ra * math.Pi / 180, dec * math.Pi / 180 },
			[2]float64{0, 360}, [2]float64{-88, 88}, 1)
		collect(func(dec, ra float64) (float64, float64) { return ra * math.Pi / 180, dec * math.Pi / 180 },
			[2]float64{-80, 80}, [2]float64{0, 360}, 1)
	case "grid-galactic":
		collect(func(l, b float64) (float64, float64) { return galacticToEquatorial(l, b) },
			[2]float64{0, 360}, [2]float64{-88, 88}, 1)
		collect(func(b, l float64) (float64, float64) { return galacticToEquatorial(l, b) },
			[2]float64{-80, 80}, [2]float64{0, 360}, 1)
	case "grid-ecliptic":
		collect(func(lon, lat float64) (float64, float64) { return eclipticToEquatorial(lon, lat) },
			[2]float64{0, 360}, [2]float64{-80, 80}, 1)
		collect(func(lat, lon float64) (float64, float64) { return eclipticToEquatorial(lon, lat) },
			[2]float64{-60, 60}, [2]float64{0, 360}, 1)
	}
	return strings.TrimSpace(b.String())
}

// annotationScaleBar returns the bar length in pixels and its angular label.
func annotationScaleBar(w *wcsTan, width int) (float64, string) {
	scale := annotationArcsecPerPixel(w)
	target := float64(width) * 0.18 * scale
	if target <= 0 {
		return 10, "?"
	}
	exp := math.Floor(math.Log10(target))
	base := math.Pow(10, exp)
	nice := base
	for _, m := range []float64{1, 2, 5, 10} {
		if m*base >= target {
			nice = m * base
			break
		}
	}
	if nice >= 3600 {
		return nice / scale, fmt.Sprintf("%.1f deg", nice/3600)
	}
	if nice >= 60 {
		return nice / scale, fmt.Sprintf("%.0f arcmin", nice/60)
	}
	return nice / scale, fmt.Sprintf("%.0f arcsec", nice)
}

// annotationStarStyle maps magnitude to a renderer-safe literal colour and
// opacity. Presentation attributes cannot use CSS var() in non-browser SVG
// viewers, and a single flat yellow dot hid all photometric information
// (demo-test.md, "Perdida de informacion fotografica"). Literal colours keep
// the map visible everywhere while still encoding relative brightness.
func annotationStarStyle(mag float64) (string, float64) {
	switch {
	case mag <= 4:
		return "#ffffff", 0.98
	case mag <= 7:
		return "#dbeaff", 0.94
	case mag <= 10:
		return "#ffe9b0", 0.88
	case mag <= 13:
		return "#ffcf8f", 0.78
	case mag <= 16:
		return "#e6a878", 0.66
	default:
		return "#b98a68", 0.55
	}
}

// buildAnnotationSVG renders a complete SVG 2 document. Legacy layer ids are
// preserved so existing consumers and selftests keep working.
func buildAnnotationSVG(width, height int, sources []annotationSource, velocities []skyVelocityVector, opts annotationRenderOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\" version=\"2\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", width, height, width, height)
	b.WriteString("<title>SkyStacker annotated sky map</title>\n")
	fmt.Fprintf(&b, "<desc>Layered astronomical annotation: %d catalogued sources, %d motion vectors. Layers are independently toggleable.</desc>\n", len(sources), len(velocities))
	b.WriteString("<style>:root{--skystacker-star:#ffe9b0;--skystacker-extended:#55ccff;--skystacker-motion:#ff8f6b;--skystacker-label:#d7e3f4;--skystacker-grid:#3f5976} .skystacker-layer{isolation:isolate} .skystacker-hidden{display:none}</style>\n")
	fmt.Fprintf(&b, "<g id=\"background\" class=\"skystacker-layer\" data-layer=\"background\"><rect width=\"%d\" height=\"%d\" fill=\"#05070d\"/></g>\n", width, height)
	if opts.WCS != nil {
		step := annotationGridStep(opts.WCS, width, height)
		for _, grid := range []string{"grid-radec", "grid-galactic", "grid-ecliptic"} {
			if !opts.enabled(grid) {
				continue
			}
			d := annotationGridPath(opts.WCS, width, height, grid, step)
			fmt.Fprintf(&b, "<g id=\"%s\" class=\"skystacker-layer\" data-layer=\"grid\" data-step-deg=\"%g\"><title>%s graticule</title><path d=\"%s\" fill=\"none\" stroke=\"#3f5976\" stroke-width=\"0.6\" opacity=\"0.55\"/></g>\n",
				grid, step, html.EscapeString(grid), html.EscapeString(d))
		}
		if opts.enabled("constellations") {
			b.WriteString("<g id=\"constellations\" class=\"skystacker-layer\" data-layer=\"constellations\"><title>IAU constellation lines</title>\n")
			bundle := offlineAnnotationBundle()
			for _, seg := range bundle.Constellations {
				a, okA := bundleByID[seg.From]
				c, okC := bundleByID[seg.To]
				if !okA || !okC {
					continue
				}
				x1, y1 := wcsTanForward(opts.WCS, a.RA, a.Dec)
				x2, y2 := wcsTanForward(opts.WCS, c.RA, c.Dec)
				if x1 < 0 || y1 < 0 || x2 < 0 || y2 < 0 || x1 > float64(width) || y1 > float64(height) || x2 > float64(width) || y2 > float64(height) {
					continue
				}
				fmt.Fprintf(&b, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#8fb4d8\" stroke-width=\"0.8\" opacity=\"0.6\" data-constellation=\"%s\"><title>%s: %s - %s</title></line>\n",
					x1, y1, x2, y2, html.EscapeString(seg.Constellation), html.EscapeString(seg.Constellation), html.EscapeString(seg.From), html.EscapeString(seg.To))
			}
			b.WriteString("</g>\n")
		}
	}
	b.WriteString("<g id=\"gaia-stars\" class=\"skystacker-layer\" data-layer=\"point-sources\">\n")
	titles := true
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
		fill, opacity := annotationStarStyle(s.Mag)
		fmt.Fprintf(&b, "<circle cx=\"%.2f\" cy=\"%.2f\" r=\"%.2f\" fill=\"%s\" opacity=\"%.2f\"", s.X, s.Y, r, fill, opacity)
		if titles && label != "" {
			fmt.Fprintf(&b, "><title>%s (%s, G=%.2f)</title></circle>\n", label, html.EscapeString(s.Type), s.Mag)
		} else {
			b.WriteString("/>\n")
		}
		if titles && b.Len() > 1<<20 {
			titles = false
		}
	}
	b.WriteString("</g>\n")
	b.WriteString("<g id=\"extended-objects\" class=\"skystacker-layer\" data-layer=\"extended-sources\">\n")
	titles = true
	for _, s := range sources {
		if !s.Extended || s.Major <= 0 || s.Minor <= 0 {
			continue
		}
		a, bb := s.Major/2, s.Minor/2
		if s.PixelA > 0 {
			a, bb = s.PixelA/2, s.PixelB/2
		}
		label := html.EscapeString(s.Name)
		fmt.Fprintf(&b, "<ellipse cx=\"%.2f\" cy=\"%.2f\" rx=\"%.2f\" ry=\"%.2f\" transform=\"rotate(%.1f %.2f %.2f)\" fill=\"none\" stroke=\"#55ccff\" stroke-width=\"1.2\" data-pa=\"%.1f\"", s.X, s.Y, a, bb, s.PixelPA, s.X, s.Y, s.PixelPA)
		if titles && label != "" {
			fmt.Fprintf(&b, "><title>%s (%s)</title></ellipse>\n", label, html.EscapeString(s.Type))
		} else {
			b.WriteString("/>\n")
		}
		if titles && b.Len() > 1<<20 {
			titles = false
		}
	}
	b.WriteString("</g>\n")
	b.WriteString("<g id=\"sso-vectors\" class=\"skystacker-layer\" data-layer=\"motion\">\n")
	for _, v := range velocities {
		fmt.Fprintf(&b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#ff8f6b\" stroke-width=\"1.4\" marker-end=\"url(#sso-arrow)\" data-object=\"%s\" data-site=\"%s\"><title>%s velocity vector</title></line>\n",
			v.X1, v.Y1, v.X2, v.Y2, html.EscapeString(v.Name), html.EscapeString(v.Observatory), html.EscapeString(v.Name))
	}
	b.WriteString("</g>\n")
	b.WriteString("<defs><marker id=\"sso-arrow\" markerWidth=\"6\" markerHeight=\"6\" refX=\"5\" refY=\"3\" orient=\"auto\"><path d=\"M0,0 L6,3 L0,6 Z\" fill=\"#ff8f6b\"/></marker></defs>\n")
	if opts.WCS != nil && opts.enabled("scale-bar") {
		px, label := annotationScaleBar(opts.WCS, width)
		x0, y0 := 14.0, float64(height)-34.0
		fmt.Fprintf(&b, "<g id=\"scale-bar\" class=\"skystacker-layer\" data-layer=\"scale-bar\" data-pixels=\"%.2f\"><title>Scale bar %s</title><path d=\"M%.1f,%.1f L%.1f,%.1f M%.1f,%.1f L%.1f,%.1f M%.1f,%.1f L%.1f,%.1f\" fill=\"none\" stroke=\"#d7e3f4\" stroke-width=\"1.4\"/><text x=\"%.1f\" y=\"%.1f\" fill=\"#d7e3f4\" font-family=\"monospace\" font-size=\"10\">%s</text></g>\n",
			px, html.EscapeString(label), x0, y0, x0+px, y0, x0, y0-4, x0, y0+4, x0+px, y0-4, x0+px, y0+4, x0, y0-8, html.EscapeString(label))
	}
	if opts.WCS != nil && opts.enabled("finder-chart") {
		size := 120.0
		x0, y0 := float64(width)-size-14, 14.0
		fmt.Fprintf(&b, "<g id=\"finder-chart\" class=\"skystacker-layer\" data-layer=\"finder-chart\"><title>Finding chart inset</title><rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"#080d18\" stroke=\"#8fb4d8\" stroke-width=\"1\"/>", x0, y0, size, size)
		fmt.Fprintf(&b, "<path d=\"M%.1f,%.1f L%.1f,%.1f M%.1f,%.1f L%.1f,%.1f\" stroke=\"#3f5976\" stroke-width=\"0.6\"/>", x0+size/2, y0, x0+size/2, y0+size, x0, y0+size/2, x0+size, y0+size/2)
		limit := len(sources)
		if limit > 250 {
			limit = 250
		}
		for _, s := range sources[:limit] {
			ix := x0 + 8 + (s.X/float64(width))*(size-16)
			iy := y0 + 8 + (s.Y/float64(height))*(size-16)
			fmt.Fprintf(&b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"0.8\" fill=\"#ffe9b0\"/>", ix, iy)
		}
		fmt.Fprintf(&b, "<text x=\"%.1f\" y=\"%.1f\" fill=\"#d7e3f4\" font-family=\"monospace\" font-size=\"9\">N</text></g>\n", x0+size-10, y0+11)
	}
	fields := opts.LabelFields
	if fields <= 0 || fields > skyLabelFields {
		fields = 1
	}
	fmt.Fprintf(&b, "<g id=\"labels\" class=\"skystacker-layer\" data-layer=\"labels\" data-fields=\"%d\" data-candidates=\"%d\" data-passes=\"%d\" data-relax=\"%d\">\n",
		skyLabelFields, skyLabelCandidatePositions, skyLabelCollisionPasses, annotationLabelRelaxPasses)
	for _, l := range placeAnnotationLabels(width, height, sources, fields) {
		fmt.Fprintf(&b, "<text x=\"%.2f\" y=\"%.2f\" fill=\"#d7e3f4\" font-family=\"monospace\" font-size=\"%.0f\">%s</text>\n",
			l.X, l.Y+skyLabelFontSize, skyLabelFontSize, html.EscapeString(l.Text))
	}
	b.WriteString("</g>\n")
	prov := "offline-bundle"
	if len(opts.Provenance) > 0 {
		names := make([]string, 0, len(opts.Provenance))
		for _, p := range opts.Provenance {
			names = append(names, p.Catalog+"/"+p.Service)
		}
		prov = strings.Join(names, ",")
	}
	fmt.Fprintf(&b, "<g id=\"footer\" class=\"skystacker-layer\" data-layer=\"footer\" data-provenance=\"%s\"><text x=\"8\" y=\"%d\" fill=\"#8899aa\" font-family=\"monospace\" font-size=\"11\">SkyStacker layered sky map — %d catalogued sources, %d motion vectors</text></g>\n",
		html.EscapeString(prov), height-8, len(sources), len(velocities))
	b.WriteString("</svg>\n")
	return b.String()
}

// ---------------------------------------------------------------------------
// Raster renderer with SVG parity (item 140)
// ---------------------------------------------------------------------------

// Okabe-Ito palette: distinguishable for the most common colour-vision types.
var annotationPalette = map[string]color.RGBA{
	"star":        {230, 159, 0, 255},
	"gaia":        {230, 159, 0, 255},
	"galaxy":      {86, 180, 233, 255},
	"nebula":      {0, 158, 115, 255},
	"cluster":     {0, 114, 178, 255},
	"planetary":   {204, 121, 167, 255},
	"remnant":     {213, 94, 0, 255},
	"dark-nebula": {100, 100, 110, 255},
	"sso":         {213, 94, 0, 255},
	"cone":        {240, 228, 66, 255},
}

func annotationColour(kind string) color.RGBA {
	if c, ok := annotationPalette[strings.ToLower(kind)]; ok {
		return c
	}
	return color.RGBA{215, 227, 244, 255}
}

func blendPixel(img *image.RGBA, x, y int, col color.RGBA, alpha float64) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() || alpha <= 0 {
		return
	}
	if alpha > 1 {
		alpha = 1
	}
	off := img.PixOffset(x, y)
	src := [3]float64{float64(col.R), float64(col.G), float64(col.B)}
	for c := 0; c < 3; c++ {
		dst := float64(img.Pix[off+c])
		img.Pix[off+c] = uint8(dst*(1-alpha) + src[c]*alpha + 0.5)
	}
}

// drawAAEllipse paints a rotated ellipse with 3x3 supersampled coverage. The
// SVG renderer uses the same centre, axes and position angle.
func drawAAEllipse(img *image.RGBA, cx, cy, a, b, paDeg float64, col color.RGBA) {
	if a <= 0 || b <= 0 {
		return
	}
	pa := paDeg * math.Pi / 180
	cp, sp := math.Cos(pa), math.Sin(pa)
	r := math.Max(a, b) + 2
	for y := int(math.Floor(cy - r)); y <= int(math.Ceil(cy+r)); y++ {
		for x := int(math.Floor(cx - r)); x <= int(math.Ceil(cx+r)); x++ {
			hits := 0
			for sy := 0; sy < 3; sy++ {
				for sx := 0; sx < 3; sx++ {
					dx, dy := float64(x)+float64(sx)/3+1/6-cx, float64(y)+float64(sy)/3+1/6-cy
					u := dx*cp + dy*sp
					v := -dx*sp + dy*cp
					if (u*u)/(a*a)+(v*v)/(b*b) <= 1 {
						hits++
					}
				}
			}
			if hits > 0 {
				blendPixel(img, x, y, col, float64(hits)/9)
			}
		}
	}
}

// drawAALine paints a segment using per-pixel distance to the segment.
func drawAALine(img *image.RGBA, x1, y1, x2, y2, width float64, col color.RGBA) {
	minX, maxX := math.Min(x1, x2)-width-1, math.Max(x1, x2)+width+1
	minY, maxY := math.Min(y1, y2)-width-1, math.Max(y1, y2)+width+1
	dx, dy := x2-x1, y2-y1
	len2 := dx*dx + dy*dy
	for y := int(math.Floor(minY)); y <= int(math.Ceil(maxY)); y++ {
		for x := int(math.Floor(minX)); x <= int(math.Ceil(maxX)); x++ {
			t := 0.0
			if len2 > 0 {
				t = math.Max(0, math.Min(1, ((float64(x)+0.5-x1)*dx+(float64(y)+0.5-y1)*dy)/len2))
			}
			d := math.Hypot(float64(x)+0.5-(x1+t*dx), float64(y)+0.5-(y1+t*dy))
			blendPixel(img, x, y, col, math.Max(0, math.Min(1, width/2+0.5-d)))
		}
	}
}

// Font 5x7, uppercase ASCII plus common punctuation. Lowercase is folded to
// uppercase so catalog names remain legible at small sizes.
var annotationFont = map[rune][7]uint8{
	' ': {0, 0, 0, 0, 0, 0, 0},
	'A': {14, 17, 17, 31, 17, 17, 17}, 'B': {30, 17, 17, 30, 17, 17, 30},
	'C': {14, 17, 16, 16, 16, 17, 14}, 'D': {30, 17, 17, 17, 17, 17, 30},
	'E': {31, 16, 16, 30, 16, 16, 31}, 'F': {31, 16, 16, 30, 16, 16, 16},
	'G': {14, 17, 16, 23, 17, 17, 15}, 'H': {17, 17, 17, 31, 17, 17, 17},
	'I': {14, 4, 4, 4, 4, 4, 14}, 'J': {7, 2, 2, 2, 2, 18, 12},
	'K': {17, 18, 20, 24, 20, 18, 17}, 'L': {16, 16, 16, 16, 16, 16, 31},
	'M': {17, 27, 21, 21, 17, 17, 17}, 'N': {17, 25, 21, 19, 17, 17, 17},
	'O': {14, 17, 17, 17, 17, 17, 14}, 'P': {30, 17, 17, 30, 16, 16, 16},
	'Q': {14, 17, 17, 17, 21, 18, 13}, 'R': {30, 17, 17, 30, 20, 18, 17},
	'S': {15, 16, 16, 14, 1, 1, 30}, 'T': {31, 4, 4, 4, 4, 4, 4},
	'U': {17, 17, 17, 17, 17, 17, 14}, 'V': {17, 17, 17, 17, 17, 10, 4},
	'W': {17, 17, 17, 21, 21, 27, 17}, 'X': {17, 17, 10, 4, 10, 17, 17},
	'Y': {17, 17, 10, 4, 4, 4, 4}, 'Z': {31, 1, 2, 4, 8, 16, 31},
	'0': {14, 17, 19, 21, 25, 17, 14}, '1': {4, 12, 4, 4, 4, 4, 14},
	'2': {14, 17, 1, 2, 4, 8, 31}, '3': {30, 1, 1, 14, 1, 1, 30},
	'4': {2, 6, 10, 18, 31, 2, 2}, '5': {31, 16, 16, 30, 1, 1, 30},
	'6': {14, 16, 16, 30, 17, 17, 14}, '7': {31, 1, 2, 4, 8, 8, 8},
	'8': {14, 17, 17, 14, 17, 17, 14}, '9': {14, 17, 17, 15, 1, 1, 14},
	'.': {0, 0, 0, 0, 0, 12, 12}, '-': {0, 0, 0, 31, 0, 0, 0},
	':': {0, 12, 12, 0, 12, 12, 0}, '/': {1, 1, 2, 4, 8, 16, 16},
	'(': {2, 4, 8, 8, 8, 4, 2}, ')': {8, 4, 2, 2, 2, 4, 8},
	'=': {0, 0, 31, 0, 31, 0, 0}, '+': {0, 4, 4, 31, 4, 4, 0},
	'?': {14, 17, 1, 2, 4, 0, 4},
}

func drawBitmapText(img *image.RGBA, text string, x, y float64, col color.RGBA) {
	halo := color.RGBA{5, 7, 13, 255}
	cursor := int(math.Round(x))
	top := int(math.Round(y))
	for _, r := range text {
		if r >= 'a' && r <= 'z' {
			r = r - 'a' + 'A'
		}
		glyph, ok := annotationFont[r]
		if !ok {
			glyph = annotationFont['?']
		}
		for row, bits := range glyph {
			for colIdx := 0; colIdx < 5; colIdx++ {
				if bits&(1<<uint(4-colIdx)) == 0 {
					continue
				}
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						blendPixel(img, cursor+colIdx+dx, top+row+dy, halo, 0.85)
					}
				}
				blendPixel(img, cursor+colIdx, top+row, col, 1)
			}
		}
		cursor += 6
	}
}

// renderAnnotationRaster draws the same source list and placement decisions as
// buildAnnotationSVG. The raster top is offset from the SVG text baseline by
// exactly the bitmap font height, so glyph centres agree within one pixel.
//
// annotationGlyphHeight is the bitmap font cell height used for that offset.
const (
	annotationGlyphHeight    = 7
	annotationGlyphTopOffset = skyLabelFontSize - annotationGlyphHeight
)

func renderAnnotationRaster(base *image.RGBA, sources []annotationSource, velocities []skyVelocityVector, labels []labelBox) *image.RGBA {
	if base == nil {
		return nil
	}
	out := image.NewRGBA(base.Bounds())
	copy(out.Pix, base.Pix)
	for _, s := range sources {
		col := annotationColour(s.Type)
		if s.Extended && s.PixelA > 0 && s.PixelB > 0 {
			drawAAEllipse(out, s.X, s.Y, s.PixelA/2, s.PixelB/2, s.PixelPA, col)
		} else {
			r := 1.5 + (18-s.Mag)*0.35
			if r < 1.2 {
				r = 1.2
			}
			if r > 10 {
				r = 10
			}
			drawAAEllipse(out, s.X, s.Y, r, r, 0, col)
		}
	}
	for _, v := range velocities {
		drawAALine(out, v.X1, v.Y1, v.X2, v.Y2, 1.2, annotationColour("sso"))
	}
	textCol := color.RGBA{215, 227, 244, 255}
	for _, l := range labels {
		drawBitmapText(out, l.Text, l.X, l.Y+annotationGlyphTopOffset, textCol)
	}
	return out
}

// annotationSSIM computes a global SSIM score on luminance for parity tests.
func annotationSSIM(a, b *image.RGBA) float64 {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
		return 0
	}
	const (
		c1 = 6.5025
		c2 = 58.5225
	)
	var sumA, sumB float64
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			_, _, _, aa := a.At(x, y).RGBA()
			_, _, _, bb := b.At(x, y).RGBA()
			sumA += float64(aa >> 8)
			sumB += float64(bb >> 8)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	ma, mb := sumA/float64(n), sumB/float64(n)
	var va, vb, cov float64
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			_, _, _, aa := a.At(x, y).RGBA()
			_, _, _, bb := b.At(x, y).RGBA()
			da, db := float64(aa>>8)-ma, float64(bb>>8)-mb
			va += da * da
			vb += db * db
			cov += da * db
		}
	}
	va, vb, cov = va/float64(n), vb/float64(n), cov/float64(n)
	return ((2*ma*mb + c1) * (2*cov + c2)) / ((ma*ma + mb*mb + c1) * (va + vb + c2))
}

// ---------------------------------------------------------------------------
// End-to-end orchestration (item 143)
// ---------------------------------------------------------------------------

// annotationAllLayers is the default layer set: every stable <g data-layer>
// group emitted by buildAnnotationSVG, including all three graticules,
// constellation lines, scale bar and finding-chart inset.
var annotationAllLayers = []string{
	"background", "grid-radec", "grid-galactic", "grid-ecliptic", "constellations",
	"point-sources", "extended-sources", "motion", "scale-bar", "finder-chart",
	"labels", "footer",
}

// parseAnnotationLayers maps user-facing layer aliases to the names consumed
// by annotationRenderOptions.enabled. An empty spec selects all layers.
func parseAnnotationLayers(spec string) []string {
	if strings.TrimSpace(spec) == "" {
		return append([]string(nil), annotationAllLayers...)
	}
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		switch name {
		case "gaia-stars", "stars", "points":
			name = "point-sources"
		case "extended-objects", "dso":
			name = "extended-sources"
		case "sso-vectors", "vectors":
			name = "motion"
		case "grid":
			out = append(out, "grid-radec", "grid-galactic", "grid-ecliptic")
			continue
		}
		out = append(out, name)
	}
	return out
}

// layeredAnnotationReport is the provenance report written next to the SVG and
// raster products. It records the exact model, layers and sources used so a
// downstream consumer can audit or reproduce the annotation.
type layeredAnnotationReport struct {
	Version    string                 `json:"version"`
	Width      int                    `json:"width"`
	Height     int                    `json:"height"`
	Sources    int                    `json:"sources"`
	Labels     int                    `json:"labels"`
	Layers     []string               `json:"layers"`
	SVG        string                 `json:"svg"`
	Raster     string                 `json:"raster"`
	Provenance []annotationProvenance `json:"provenance"`
	Warnings   []string               `json:"warnings,omitempty"`
}

// annotationBaseImage loads the stacked JPEG as RGBA. A missing or invalid
// preview yields a dark canvas so annotation products are still emitted for a
// plate-solved field whose stack preview was not written.
func annotationBaseImage(dir string, width, height int) *image.RGBA {
	base := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range base.Pix {
		base.Pix[i] = 0
	}
	for i := 0; i < len(base.Pix); i += 4 {
		base.Pix[i] = 5
		base.Pix[i+1] = 7
		base.Pix[i+2] = 13
		base.Pix[i+3] = 255
	}
	f, err := os.Open(filepath.Join(dir, "stacked_mean.jpg"))
	if err != nil {
		return base
	}
	defer f.Close()
	src, err := jpeg.Decode(f)
	if err != nil {
		return base
	}
	for y := 0; y < height && y < src.Bounds().Dy(); y++ {
		for x := 0; x < width && x < src.Bounds().Dx(); x++ {
			base.Set(x, y, src.At(src.Bounds().Min.X+x, src.Bounds().Min.Y+y))
		}
	}
	return base
}

// runLayeredAnnotation builds the state-of-the-art annotation products from a
// single unified source model: stacked_annotated.svg (SVG 2, layered),
// stacked_annotated.png (same model and placement) and
// annotation_provenance.json. It is a no-op without a plate solution and
// degrades to the offline bundle when no Gaia tile or VO service is reachable.
func runLayeredAnnotation(outputDir string, opts runOptions) error {
	data, err := os.ReadFile(filepath.Join(outputDir, "plate_solution.json"))
	if err != nil {
		logf("Layered annotation: no plate solution, skipping")
		return nil
	}
	var sol plateSolution
	if err := json.Unmarshal(data, &sol); err != nil {
		return fmt.Errorf("layered annotation: plate solution: %w", err)
	}
	w, h, err := readStackDimensions(outputDir)
	if err != nil {
		return fmt.Errorf("layered annotation: stack dimensions: %w", err)
	}
	ww := sol.WCS
	ww.SIP = sol.SIP

	// Unified source model: offline bundle + cached Gaia + user catalogs + VO.
	acqOpts := annotationAcquisitionOptions{
		Offline:      opts.GaiaOffline || strings.TrimSpace(opts.AnnotationVO) == "",
		CacheDir:     opts.AnnotationCache,
		RADeg:        ww.CRVAL1 * 180 / math.Pi,
		DecDeg:       ww.CRVAL2 * 180 / math.Pi,
		RadiusArcsec: opts.ConeRadiusArcsec,
		Timeout:      30 * time.Second,
	}
	if acqOpts.CacheDir == "" {
		acqOpts.CacheDir = gaiaDefaultCacheDir()
	}
	for _, svc := range strings.Split(opts.AnnotationVO, ",") {
		if name := strings.ToLower(strings.TrimSpace(svc)); name != "" {
			acqOpts.Services = append(acqOpts.Services, name)
		}
	}
	acq, err := acquireAnnotationSources(context.Background(), acqOpts)
	if err != nil {
		return fmt.Errorf("layered annotation: acquisition: %w", err)
	}
	bundle := offlineAnnotationBundle()
	sources := append([]annotationSource(nil), acq.Sources...)

	// Cached Gaia stars carry epoch and proper motion, so they enrich the
	// unified model rather than bypassing it. Union all cached tiles crossing
	// the frame so wide fields are not truncated at one HEALPix tile.
	for _, st := range cachedGaiaStarsForField(gaiaDefaultCacheDir(), gaiaDefaultNside, &ww, w, h) {
		sources = append(sources, annotationSource{
			ID: fmt.Sprintf("gaia-%.6f-%.6f", st.RA, st.Dec), Name: "Gaia", Type: "gaia",
			RA: st.RA, Dec: st.Dec, Band: "G", Mag: st.MagG, Epoch: gaiaReferenceYear,
			PMRA: st.PMRA, PMDec: st.PMDec, Priority: 40,
			Provenance: annotationProvenance{Catalog: "Gaia DR3", Service: "cache", Offline: true},
		})
	}
	extra, err := loadAnnotationCatalogs(opts.AnnotationCatalogs)
	if err != nil {
		return err
	}
	for _, e := range extra {
		sources = append(sources, annotationFromCatalogEntry(e, "annotation-catalog"))
	}
	sources = projectAnnotationSources(&ww, w, h, sources)

	var vectors []skyVelocityVector
	if strings.TrimSpace(opts.SsoEphemeris) != "" {
		if ephem, serr := loadSSOEphemeris(opts.SsoEphemeris); serr != nil {
			acq.Warnings = append(acq.Warnings, redactAnnotationError("SSO: "+serr.Error()))
		} else {
			vectors = skyMapVectors(&ww, opts.SsoEphemeris)
			_ = ephem
		}
	}

	layers := parseAnnotationLayers(opts.AnnotationLayers)
	renderOpts := annotationRenderOptions{
		WCS: &ww, Layers: layers, LabelFields: 1, Provenance: acq.Provenance,
	}
	svg := buildAnnotationSVG(w, h, sources, vectors, renderOpts)
	labels := placeAnnotationLabels(w, h, sources, 1)
	raster := renderAnnotationRaster(annotationBaseImage(outputDir, w, h), sources, vectors, labels)

	svgPath := filepath.Join(outputDir, "stacked_annotated.svg")
	if err := writeFileAtomic(svgPath, func(out io.Writer) error {
		_, err := io.WriteString(out, svg)
		return err
	}); err != nil {
		return err
	}
	pngPath := filepath.Join(outputDir, "stacked_annotated.png")
	if err := writeFileAtomic(pngPath, func(out io.Writer) error {
		return png.Encode(out, raster)
	}); err != nil {
		return err
	}
	report := layeredAnnotationReport{
		Version:    bundle.Version,
		Width:      w,
		Height:     h,
		Sources:    len(sources),
		Labels:     len(labels),
		Layers:     layers,
		SVG:        filepath.Base(svgPath),
		Raster:     filepath.Base(pngPath),
		Provenance: acq.Provenance,
		Warnings:   acq.Warnings,
	}
	if err := writeJSON(filepath.Join(outputDir, "annotation_provenance.json"), report); err != nil {
		return err
	}
	logf("Layered annotation: wrote %s + %s (%d sources, %d labels, %d layers)",
		filepath.Base(svgPath), filepath.Base(pngPath), len(sources), len(labels), len(layers))
	return nil
}

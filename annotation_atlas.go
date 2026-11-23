package main

// Atlas-style annotation symbols (roadmap items 146-149).
//
// The procedural shapes are deterministic and derived only from the unified
// annotationSource model. SVG and raster renderers share the same helpers so a
// galaxy outline, nebula contour or variable-star ring cannot drift apart.

import (
	"fmt"
	"hash/fnv"
	"html"
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	atlasBlobVertexCount = 16
	atlasLegendWidth     = 168.0
	atlasLegendHeight    = 118.0
	atlasLegendMargin    = 14.0
)

// atlasBool parses the truthy values used by catalog exports and metadata.
func atlasBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "v", "var", "variable", "d", "ds", "double":
		return true
	default:
		return false
	}
}

// atlasMetadataFlag looks up a flag case-insensitively in source metadata.
func atlasMetadataFlag(s annotationSource, keys ...string) bool {
	for key, value := range s.Metadata {
		for _, want := range keys {
			if strings.EqualFold(key, want) && atlasBool(value) {
				return true
			}
		}
	}
	return false
}

func atlasVariable(s annotationSource) bool {
	return s.Variable || atlasMetadataFlag(s, "variable", "var", "is_variable", "variability")
}

func atlasDouble(s annotationSource) bool {
	return s.Double || atlasMetadataFlag(s, "double", "double_star", "is_double", "ds")
}

const atlasMotionVectorLimit = 240

// atlasMotionOverlay is a deterministic, renderer-independent overlay record.
type atlasMotionOverlay struct {
	ID           string
	X1, Y1       float64
	X2, Y2       float64
	ProperMotion bool
	Variability  float64
}

// atlasVariabilityAmplitude reads an amplitude from metadata or from min/max
// magnitude fields. Missing values are not invented.
func atlasVariabilityAmplitude(s annotationSource) (float64, bool) {
	if v, ok := atlasMetadataNumber(s, "amplitude", "var_amp", "variability_amplitude", "amplitude_mag", "v_amp"); ok && v > 0 {
		return math.Min(v, 2.5), true
	}
	lo, loOK := atlasMetadataNumber(s, "min_mag", "magnitude_min", "mag_min")
	hi, hiOK := atlasMetadataNumber(s, "max_mag", "magnitude_max", "mag_max")
	if loOK && hiOK && hi > lo {
		return math.Min((hi-lo)/2, 2.5), true
	}
	return 0, false
}

// atlasProperMotionVector projects a fixed 50-year baseline, then shifts the
// arrow clear of the marker so it never overlaps the symbol itself.
func atlasProperMotionVector(s annotationSource, w *wcsTan) (float64, float64, float64, float64, bool) {
	if w == nil || s.Epoch <= 0 || (s.PMRA == 0 && s.PMDec == 0) {
		return 0, 0, 0, 0, false
	}
	current := s
	current.Epoch = 2026.0 // source coordinates are already propagated to 2026
	future := applyProperMotion(current, 2076.0)
	x, y := wcsTanForward(w, future.RA, future.Dec)
	dx, dy := x-s.X, y-s.Y
	length := math.Hypot(dx, dy)
	if !(length > 0.1) || math.IsNaN(length) || math.IsInf(length, 0) {
		return 0, 0, 0, 0, false
	}
	clamped := math.Min(34, math.Max(6, length))
	ux, uy := dx/length, dy/length
	start := annotationStarRadius(s.Mag) + 4
	return s.X + ux*start, s.Y + uy*start, s.X + ux*(start+clamped), s.Y + uy*(start+clamped), true
}

// atlasMotionOverlays builds all proper-motion and variability overlays in a
// stable ID order. The cap keeps large Gaia fields inside the SVG size guard.
func atlasMotionOverlays(sources []annotationSource, w *wcsTan) []atlasMotionOverlay {
	candidates := append([]annotationSource(nil), sources...)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	out := make([]atlasMotionOverlay, 0, len(candidates))
	pmDrawn := 0
	for _, s := range candidates {
		if s.Extended {
			continue
		}
		o := atlasMotionOverlay{ID: s.ID}
		if x1, y1, x2, y2, ok := atlasProperMotionVector(s, w); ok && pmDrawn < atlasMotionVectorLimit {
			o.X1, o.Y1, o.X2, o.Y2, o.ProperMotion = x1, y1, x2, y2, true
			pmDrawn++
		}
		if amp, ok := atlasVariabilityAmplitude(s); ok {
			o.Variability = 4 + amp*8
		}
		if o.ProperMotion || o.Variability > 0 {
			out = append(out, o)
		}
	}
	return out
}

// atlasGeometrySource records whether a footprint came from catalog axes or
// from the deterministic procedural fallback.
func atlasGeometrySource(s annotationSource) string {
	for key, value := range s.Metadata {
		if !strings.EqualFold(key, "geometry_source") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(value), "catalog") {
			return "catalog"
		}
	}
	return "fallback"
}

// atlasGeometryMetadata attaches the geometry provenance flag used by the
// report and by consumers of annotation_provenance.json.
func atlasGeometryMetadata(major, minor float64) map[string]string {
	source := "fallback"
	if major > 0 && minor > 0 {
		source = "catalog"
	}
	return map[string]string{"geometry_source": source}
}

// atlasExtendedSymbol maps catalog object types to a human-readable atlas
// contour family. Unknown extended types use the generic nebula contour.
func atlasExtendedSymbol(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "star", "gaia", "cone", "":
		return "star"
	case "sso", "asteroid", "comet":
		return "sso"
	case "galaxy", "gal", "gxy", "spiral", "elliptical", "d galaxy", "giant galaxy":
		return "galaxy"
	case "dark-nebula", "dark_nebula", "dark", "ldn", "dark cloud":
		return "dark-nebula"
	case "planetary", "planetary-nebula", "planetary nebula", "pn":
		return "planetary"
	case "cluster", "globular", "globular cluster", "open cluster", "open":
		return "cluster"
	case "remnant", "snr", "supernova remnant", "supernova-remnant":
		return "remnant"
	default:
		return "nebula"
	}
}

// atlasSymbol is the single symbol vocabulary shared by both renderers.
func atlasSymbol(s annotationSource) string {
	if s.Extended && (s.PixelA > 0 || s.Major > 0) {
		return atlasExtendedSymbol(s.Type)
	}
	if atlasDouble(s) {
		return "double"
	}
	if atlasVariable(s) {
		return "variable"
	}
	return atlasExtendedSymbol(s.Type)
}

// atlasStroke returns the literal SVG colour for a symbol. The values match
// annotationPalette exactly so raster overlays stay colour-compatible.
func atlasStroke(symbol string) string {
	switch symbol {
	case "galaxy":
		return "#56b4e9"
	case "nebula":
		return "#009e73"
	case "dark-nebula":
		return "#64646e"
	case "planetary":
		return "#cc79a7"
	case "cluster":
		return "#0072b2"
	case "remnant", "sso":
		return "#d55e00"
	case "variable", "double":
		return "#ffffff"
	default:
		return "#d7e3f4"
	}
}

// atlasDash returns the SVG dash pattern used to distinguish extended-object
// families without relying on colour alone.
func atlasDash(symbol string) string {
	switch symbol {
	case "dark-nebula", "remnant":
		return "4 3"
	case "cluster":
		return "2 2"
	default:
		return ""
	}
}

// atlasDashed reports whether the raster renderer should break a contour.
func atlasDashed(symbol string) bool {
	return atlasDash(symbol) != ""
}

// atlasMetadataNumber reads a numeric colour/amplitude field from metadata.
func atlasMetadataNumber(s annotationSource, keys ...string) (float64, bool) {
	for _, want := range keys {
		for key, value := range s.Metadata {
			if !strings.EqualFold(key, want) {
				continue
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil && !math.IsNaN(v) {
				return v, true
			}
		}
	}
	return 0, false
}

func stringFieldFromMetadata(metadata map[string]string, keys ...string) string {
	for _, want := range keys {
		for key, value := range metadata {
			if strings.EqualFold(key, want) && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

// atlasColourIndex returns a colour index from metadata or a Band expression
// such as "B-V=0.65". Gaia G alone carries no temperature information.
func atlasColourIndex(s annotationSource) (float64, bool) {
	if v, ok := atlasMetadataNumber(s, "b-v", "bv", "b_v", "colour-index", "color-index", "ci", "bp-rp", "bprp", "g-r"); ok {
		return v, true
	}
	if i := strings.Index(strings.ToUpper(s.Band), "B-V="); i >= 0 {
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.ToUpper(s.Band)[i+4:]), 64)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

// atlasSpectralClass bins a B-V-like index into the classical temperature
// sequence. It is a display class, not a spectral classification.
func atlasSpectralClass(index float64) string {
	switch {
	case index <= -0.30:
		return "O/B"
	case index <= 0.00:
		return "B"
	case index <= 0.30:
		return "A"
	case index <= 0.58:
		return "F"
	case index <= 0.85:
		return "G"
	case index <= 1.40:
		return "K"
	default:
		return "M"
	}
}

// atlasStarTint returns a colour-blind-safe Okabe-Ito fill for stars with a
// usable index. The boolean is false for the magnitude-only fallback, so both
// SVG and raster can advertise which colour mode produced the marker.
func atlasStarTint(s annotationSource) (string, bool) {
	if index, ok := atlasColourIndex(s); ok {
		switch atlasSpectralClass(index) {
		case "O/B", "B":
			return "#0072b2", true
		case "A":
			return "#56b4e9", true
		case "F":
			return "#f0e442", true
		case "G":
			return "#ffffff", true
		case "K":
			return "#e69f00", true
		default:
			return "#d55e00", true
		}
	}
	if sptype := strings.ToUpper(strings.TrimSpace(stringFieldFromMetadata(s.Metadata, "spectral_class", "sptype", "spectral_type"))); sptype != "" {
		switch sptype[0] {
		case 'O', 'B':
			return "#0072b2", true
		case 'A':
			return "#56b4e9", true
		case 'F':
			return "#f0e442", true
		case 'G':
			return "#ffffff", true
		case 'K':
			return "#e69f00", true
		case 'M':
			return "#d55e00", true
		}
	}
	// Gaia G/BP/RP are passbands, not temperatures: leave them on the
	// magnitude-only fallback instead of inventing a spectral class.
	switch strings.ToUpper(strings.TrimSpace(s.Band)) {
	case "U", "B":
		return "#0072b2", true
	case "V":
		return "#f0e442", true
	case "R":
		return "#e69f00", true
	case "I":
		return "#d55e00", true
	}
	f, _ := annotationStarStyle(s.Mag)
	return f, false
}

// atlasHexColour converts the literal SVG palette to an RGBA colour.
func atlasHexColour(hex string) color.RGBA {
	var r, g, b int
	if n, _ := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); n != 3 {
		return color.RGBA{215, 227, 244, 255}
	}
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

// annotationStarColour mirrors annotationStarStyle for the raster renderer.
func annotationStarColour(mag float64) color.RGBA {
	switch {
	case mag <= 4:
		return color.RGBA{255, 255, 255, 255}
	case mag <= 7:
		return color.RGBA{219, 234, 255, 255}
	case mag <= 10:
		return color.RGBA{255, 233, 176, 255}
	case mag <= 13:
		return color.RGBA{255, 207, 143, 255}
	case mag <= 16:
		return color.RGBA{230, 168, 120, 255}
	default:
		return color.RGBA{185, 138, 104, 255}
	}
}

// annotationStarOpacity mirrors annotationStarStyle for the raster renderer.
func annotationStarOpacity(mag float64) float64 {
	switch {
	case mag <= 4:
		return 0.98
	case mag <= 7:
		return 0.94
	case mag <= 10:
		return 0.88
	case mag <= 13:
		return 0.78
	case mag <= 16:
		return 0.66
	default:
		return 0.55
	}
}

// annotationStarRadius is the shared marker radius for point sources.
func annotationStarRadius(mag float64) float64 {
	r := 1.5 + (18-mag)*0.35
	if r < 1.2 {
		return 1.2
	}
	if r > 10 {
		return 10
	}
	return r
}

// annotationBlobPoints returns a deterministic irregular contour around an
// extended object. It is intentionally smooth and low-frequency: atlas maps
// communicate footprint, not a pixel-level segmentation of the photograph.
func annotationBlobPoints(cx, cy, a, b, paDeg float64, id string) [][2]float64 {
	points := make([][2]float64, 0, atlasBlobVertexCount)
	if a <= 0 || b <= 0 {
		return points
	}
	var seed uint32
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	seed = h.Sum32()
	pa := paDeg * math.Pi / 180
	cp, sp := math.Cos(pa), math.Sin(pa)
	for i := 0; i < atlasBlobVertexCount; i++ {
		t := 2 * math.Pi * float64(i) / atlasBlobVertexCount
		phase := float64(seed%23) / 23 * 2 * math.Pi
		wobble := 1 + 0.11*math.Sin(3*t+phase) + 0.06*math.Cos(5*t-phase*0.5)
		radial := 1 + 0.08*math.Cos(2*t-phase)
		u := a * math.Cos(t) * wobble
		v := b * math.Sin(t) * radial
		points = append(points, [2]float64{cx + u*cp - v*sp, cy + u*sp + v*cp})
	}
	return points
}

// annotationBlobPath renders the shared contour as an SVG path.
func annotationBlobPath(cx, cy, a, b, paDeg float64, id string) string {
	points := annotationBlobPoints(cx, cy, a, b, paDeg, id)
	if len(points) == 0 {
		return ""
	}
	var bld strings.Builder
	for i, p := range points {
		if i == 0 {
			fmt.Fprintf(&bld, "M%.2f,%.2f", p[0], p[1])
		} else {
			fmt.Fprintf(&bld, "L%.2f,%.2f", p[0], p[1])
		}
	}
	bld.WriteString("Z")
	return bld.String()
}

// appendAtlasOverlaysSVG emits proper-motion arrows and variability brackets
// into the shared motion layer.
func appendAtlasOverlaysSVG(b *strings.Builder, sources []annotationSource, w *wcsTan) {
	for _, o := range atlasMotionOverlays(sources, w) {
		if o.ProperMotion {
			angle := math.Atan2(o.Y2-o.Y1, o.X2-o.X1)
			fmt.Fprintf(b, "<line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#ff8f6b\" stroke-width=\"1\" marker-end=\"url(#skystacker-pm-arrow)\" data-overlay=\"proper-motion\" data-source-id=\"%s\"/>\n",
				o.X1, o.Y1, o.X2, o.Y2, html.EscapeString(o.ID))
			ax, ay := 5*math.Cos(angle-0.5), 5*math.Sin(angle-0.5)
			bx, by := 5*math.Cos(angle+0.5), 5*math.Sin(angle+0.5)
			fmt.Fprintf(b, "<path d=\"M%.2f %.2f L%.2f %.2f L%.2f %.2f\" fill=\"none\" stroke=\"#ff8f6b\" stroke-width=\"1\" data-overlay=\"proper-motion-arrow\"/>\n",
				o.X2, o.Y2, o.X2-ax, o.Y2-ay, o.X2-bx, o.Y2-by)
		}
	}
	for _, s := range sources {
		if s.Extended {
			continue
		}
		amp, ok := atlasVariabilityAmplitude(s)
		if !ok {
			continue
		}
		top := s.Y - annotationStarRadius(s.Mag) - 4
		fmt.Fprintf(b, "<path d=\"M%.2f %.2f L%.2f %.2f L%.2f %.2f\" fill=\"none\" stroke=\"#d55e00\" stroke-width=\"1\" stroke-dasharray=\"2 1\" data-overlay=\"variability\" data-amplitude=\"%.2f\" data-source-id=\"%s\"/>\n",
			s.X, top, s.X, top-(4+amp*8), s.X+4, top-(4+amp*8), amp, html.EscapeString(s.ID))
	}
}

// drawAtlasOverlaysRaster mirrors appendAtlasOverlaysSVG on the raster map.
func drawAtlasOverlaysRaster(img *image.RGBA, sources []annotationSource, w *wcsTan) {
	col := color.RGBA{255, 143, 107, 255}
	for _, o := range atlasMotionOverlays(sources, w) {
		if !o.ProperMotion {
			continue
		}
		drawAALine(img, o.X1, o.Y1, o.X2, o.Y2, 1, col)
		angle := math.Atan2(o.Y2-o.Y1, o.X2-o.X1)
		for _, delta := range []float64{-0.5, 0.5} {
			x := o.X2 - 5*math.Cos(angle+delta)
			y := o.Y2 - 5*math.Sin(angle+delta)
			drawAALine(img, o.X2, o.Y2, x, y, 1, col)
		}
	}
	varCol := color.RGBA{213, 94, 0, 255}
	for _, s := range sources {
		if s.Extended {
			continue
		}
		amp, ok := atlasVariabilityAmplitude(s)
		if !ok {
			continue
		}
		top := s.Y - annotationStarRadius(s.Mag) - 4
		end := top - (4 + amp*8)
		drawAALine(img, s.X, top, s.X, end, 1, varCol)
		drawAALine(img, s.X, end, s.X+4, end, 1, varCol)
	}
}

// atlasLegendOrigin keeps the key clear of the scale bar and finding chart.
func atlasLegendOrigin(width, height int) (float64, float64) {
	x := float64(width) - atlasLegendWidth - atlasLegendMargin
	if x < atlasLegendMargin {
		x = atlasLegendMargin
	}
	y := float64(height) - atlasLegendHeight - atlasLegendMargin - 38
	if y < atlasLegendMargin {
		y = atlasLegendMargin
	}
	return x, y
}

// atlasSymbolFamilies is the stable vocabulary exposed to SVG consumers.
// A frontend toggles a family by adding skystacker-hide-<family> to the root
// <svg> element; no re-render or JavaScript is required for the common case.
var atlasSymbolFamilies = []string{
	"galaxy", "nebula", "dark-nebula", "planetary", "cluster",
	"remnant", "sso", "star", "variable", "double",
}

// atlasLegendClass returns the stable CSS classes attached to a legend swatch.
// Unknown symbols fall back to the plain star family so a frontend can never
// encounter a classless key entry. Map markers use the stable data-symbol
// attribute instead, which keeps the 10k-source SVG well below its size cap.
func atlasLegendClass(symbol string) string {
	name := strings.ToLower(strings.TrimSpace(symbol))
	known := false
	for _, family := range atlasSymbolFamilies {
		if family == name {
			known = true
			break
		}
	}
	if !known {
		name = "star"
	}
	return "skystacker-key skystacker-key-" + name
}

// atlasLegendTitle is the accessible name of a legend swatch.
func atlasLegendTitle(symbol string) string {
	switch symbol {
	case "galaxy":
		return "Galaxy: solid rotated ellipse"
	case "nebula":
		return "Diffuse nebula: irregular solid contour"
	case "dark-nebula":
		return "Dark nebula: dashed irregular contour"
	case "planetary":
		return "Planetary nebula: ring with a bright core"
	case "cluster":
		return "Star cluster: dashed circle"
	case "remnant":
		return "Supernova remnant: dashed irregular contour"
	case "sso":
		return "Solar system object: motion vector"
	case "variable":
		return "Variable star: dashed surrounding ring"
	case "double":
		return "Double star: two resolved components"
	default:
		return "Star: magnitude-sized disc"
	}
}

// atlasToggleCSS emits one pure-CSS rule per symbol family. Rules are inert
// unless the matching skystacker-hide-* class is present on the root SVG, so
// librsvg and ImageMagick keep rendering the literal attributes unchanged.
func atlasToggleCSS() string {
	var b strings.Builder
	for _, family := range atlasSymbolFamilies {
		fmt.Fprintf(&b, "svg.skystacker-hide-%s [data-symbol=\"%s\"],svg.skystacker-hide-%s .skystacker-key-%s{display:none}",
			family, family, family, family)
	}
	return b.String()
}

// atlasLegendSVG emits a compact, viewer-safe atlas key. Literal colours and
// shape descriptors make the map understandable without a browser or CSS.
func atlasLegendSVG(width, height int) string {
	x, y := atlasLegendOrigin(width, height)
	var b strings.Builder
	fmt.Fprintf(&b, "<g id=\"legend\" class=\"skystacker-layer\" data-layer=\"legend\"><title>Atlas symbol key</title>")
	fmt.Fprintf(&b, "<rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" fill=\"#080d18\" stroke=\"#3f5976\" stroke-width=\"0.8\" opacity=\"0.92\"/>",
		x, y, atlasLegendWidth, atlasLegendHeight)
	fmt.Fprintf(&b, "<text x=\"%.1f\" y=\"%.1f\" fill=\"#d7e3f4\" font-family=\"monospace\" font-size=\"10\">ATLAS KEY</text>", x+8, y+15)
	rows := []struct {
		symbol string
		label  string
	}{
		{"galaxy", "GALAXY"},
		{"nebula", "NEBULA"},
		{"planetary", "PLANETARY"},
		{"cluster", "CLUSTER"},
		{"star", "STAR"},
		{"variable", "VARIABLE"},
		{"double", "DOUBLE"},
	}
	cx := x + 18
	for i, row := range rows {
		cy := y + 30 + float64(i)*12
		col := atlasStroke(row.symbol)
		fmt.Fprintf(&b, "<g class=\"%s\" data-key-symbol=\"%s\"><title>%s</title>",
			atlasLegendClass(row.symbol), row.symbol, atlasLegendTitle(row.symbol))
		switch row.symbol {
		case "galaxy":
			fmt.Fprintf(&b, "<ellipse cx=\"%.1f\" cy=\"%.1f\" rx=\"7\" ry=\"3.6\" fill=\"none\" stroke=\"%s\" stroke-width=\"1\"/>", cx, cy, col)
		case "nebula":
			fmt.Fprintf(&b, "<path d=\"M%.1f,%.1f q5,-5 10,0 q4,5 -2,7 q-7,2 -8,-7\" fill=\"none\" stroke=\"%s\" stroke-width=\"1\"/>", cx-7, cy, col)
		case "planetary":
			fmt.Fprintf(&b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"4\" fill=\"none\" stroke=\"%s\" stroke-width=\"1\"/><circle cx=\"%.1f\" cy=\"%.1f\" r=\"1\" fill=\"%s\"/>", cx, cy, col, cx, cy, col)
		case "cluster":
			fmt.Fprintf(&b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"4.5\" fill=\"none\" stroke=\"%s\" stroke-width=\"1\" stroke-dasharray=\"2 2\"/>", cx, cy, col)
		case "star":
			fmt.Fprintf(&b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"2\" fill=\"#ffe9b0\"/>", cx, cy)
		case "variable":
			fmt.Fprintf(&b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"2\" fill=\"%s\"/><circle cx=\"%.1f\" cy=\"%.1f\" r=\"4.5\" fill=\"none\" stroke=\"%s\" stroke-width=\"0.8\" stroke-dasharray=\"2 2\"/>", cx, cy, col, cx, cy, col)
		case "double":
			fmt.Fprintf(&b, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"2\" fill=\"%s\"/><circle cx=\"%.1f\" cy=\"%.1f\" r=\"2\" fill=\"%s\"/>", cx-2.5, cy, col, cx+2.5, cy, col)
		}
		fmt.Fprintf(&b, "<text x=\"%.1f\" y=\"%.1f\" fill=\"#b9c8dc\" font-family=\"monospace\" font-size=\"8\">%s</text>", x+34, cy+3, row.label)
		b.WriteString("</g>")
	}
	b.WriteString("</g>\n")
	return b.String()
}

// atlasRasterColour is the raster twin of atlasStroke.
func atlasRasterColour(symbol string) color.RGBA {
	switch symbol {
	case "galaxy":
		return color.RGBA{86, 180, 233, 255}
	case "nebula":
		return color.RGBA{0, 158, 115, 255}
	case "dark-nebula":
		return color.RGBA{100, 100, 110, 255}
	case "planetary":
		return color.RGBA{204, 121, 167, 255}
	case "cluster":
		return color.RGBA{0, 114, 178, 255}
	case "remnant", "sso":
		return color.RGBA{213, 94, 0, 255}
	case "variable", "double":
		return color.RGBA{255, 255, 255, 255}
	default:
		return color.RGBA{215, 227, 244, 255}
	}
}

// drawAAFilledEllipse paints a rotated filled ellipse with 3x3 supersampling
// and an explicit alpha, matching the opacity of SVG point markers.
func drawAAFilledEllipse(img *image.RGBA, cx, cy, a, b, paDeg float64, col color.RGBA, alpha float64) {
	if img == nil || a <= 0 || b <= 0 || alpha <= 0 {
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
				blendPixel(img, x, y, col, alpha*float64(hits)/9)
			}
		}
	}
}

// drawAAPolyline paints a closed polyline, optionally breaking alternate
// segments for the dashed atlas families.
func drawAAPolyline(img *image.RGBA, pts [][2]float64, width float64, col color.RGBA, dashed bool) {
	if len(pts) < 2 {
		return
	}
	closed := append(append([][2]float64(nil), pts...), pts[0])
	for i := 1; i < len(closed); i++ {
		if dashed && i%2 == 0 {
			continue
		}
		drawAALine(img, closed[i-1][0], closed[i-1][1], closed[i][0], closed[i][1], width, col)
	}
}

// drawAAEllipseOutline samples an ellipse boundary and paints it as a closed
// polyline. The sample count is capped to keep 10k-source maps responsive.
func drawAAEllipseOutline(img *image.RGBA, cx, cy, a, b, paDeg, width float64, col color.RGBA, dashed bool) {
	if a <= 0 || b <= 0 {
		return
	}
	steps := int(2 * math.Pi * math.Max(a, b) / 1.5)
	if steps < 24 {
		steps = 24
	}
	if steps > 2048 {
		steps = 2048
	}
	pa := paDeg * math.Pi / 180
	cp, sp := math.Cos(pa), math.Sin(pa)
	pts := make([][2]float64, 0, steps)
	for i := 0; i < steps; i++ {
		t := 2 * math.Pi * float64(i) / float64(steps)
		u, v := a*math.Cos(t), b*math.Sin(t)
		pts = append(pts, [2]float64{cx + u*cp - v*sp, cy + u*sp + v*cp})
	}
	drawAAPolyline(img, pts, width, col, dashed)
}

// renderAtlasLegendRaster paints the same key as atlasLegendSVG so the PNG
// overlay is self-explanatory when viewed without the SVG document.
func renderAtlasLegendRaster(img *image.RGBA) {
	if img == nil {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x, y := atlasLegendOrigin(w, h)
	boxCol := color.RGBA{8, 13, 24, 255}
	borderCol := color.RGBA{63, 89, 118, 255}
	for yy := int(y); yy < int(y+atlasLegendHeight); yy++ {
		for xx := int(x); xx < int(x+atlasLegendWidth); xx++ {
			blendPixel(img, xx, yy, boxCol, 0.92)
		}
	}
	for xx := int(x); xx < int(x+atlasLegendWidth); xx++ {
		blendPixel(img, xx, int(y), borderCol, 0.9)
		blendPixel(img, xx, int(y+atlasLegendHeight-1), borderCol, 0.9)
	}
	for yy := int(y); yy < int(y+atlasLegendHeight); yy++ {
		blendPixel(img, int(x), yy, borderCol, 0.9)
		blendPixel(img, int(x+atlasLegendWidth-1), yy, borderCol, 0.9)
	}
	drawBitmapText(img, "ATLAS KEY", x+8, y+4, color.RGBA{215, 227, 244, 255})
	rows := []struct {
		symbol string
		label  string
	}{
		{"galaxy", "GALAXY"},
		{"nebula", "NEBULA"},
		{"planetary", "PLANETARY"},
		{"cluster", "CLUSTER"},
		{"star", "STAR"},
		{"variable", "VARIABLE"},
		{"double", "DOUBLE"},
	}
	cx := x + 18
	for i, row := range rows {
		cy := y + 30 + float64(i)*12
		col := atlasRasterColour(row.symbol)
		switch row.symbol {
		case "galaxy":
			drawAAEllipseOutline(img, cx, cy, 7, 3.6, 0, 1, col, false)
		case "nebula":
			drawAAPolyline(img, annotationBlobPoints(cx, cy, 8, 4, 0, "legend-nebula"), 1, col, false)
		case "planetary":
			drawAAEllipseOutline(img, cx, cy, 4, 4, 0, 1, col, false)
			drawAAFilledEllipse(img, cx, cy, 1, 1, 0, col, 1)
		case "cluster":
			drawAAEllipseOutline(img, cx, cy, 4.5, 4.5, 0, 1, col, true)
		case "star":
			drawAAFilledEllipse(img, cx, cy, 2, 2, 0, color.RGBA{255, 233, 176, 255}, 1)
		case "variable":
			drawAAFilledEllipse(img, cx, cy, 2, 2, 0, col, 1)
			drawAAEllipseOutline(img, cx, cy, 4.5, 4.5, 0, 0.8, col, true)
		case "double":
			drawAAFilledEllipse(img, cx-2.5, cy, 2, 2, 0, col, 1)
			drawAAFilledEllipse(img, cx+2.5, cy, 2, 2, 0, col, 1)
		}
		drawBitmapText(img, row.label, x+34, cy-3, color.RGBA{185, 200, 220, 255})
	}
}

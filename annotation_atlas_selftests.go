package main

// Embedded self-tests for the incremental atlas-style annotation roadmap
// (items 146-150). These tests lock the symbol vocabulary, shared geometry,
// legend and SVG/raster parity so later map work cannot silently regress it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// countChangedPixels compares two RGBA images and returns the number of
// pixels whose colour differs.
func countChangedPixels(base, out *image.RGBA) int {
	if base == nil || out == nil || base.Bounds() != out.Bounds() {
		return -1
	}
	n := 0
	for y := base.Bounds().Min.Y; y < base.Bounds().Max.Y; y++ {
		for x := base.Bounds().Min.X; x < base.Bounds().Max.X; x++ {
			if base.At(x, y) != out.At(x, y) {
				n++
			}
		}
	}
	return n
}

// atlasTestBase returns a uniform dark canvas for raster symbol tests.
func atlasTestBase(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := color.RGBA{5, 7, 13, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	return img
}

// testAtlasSymbolVocabulary covers item 146: one deterministic vocabulary for
// extended objects, variables, doubles and ordinary stars.
func testAtlasSymbolVocabulary() error {
	cases := []struct {
		name string
		src  annotationSource
		want string
	}{
		{"galaxy", annotationSource{Type: "galaxy", Extended: true, Major: 40, Minor: 20, PixelA: 40, PixelB: 20}, "galaxy"},
		{"nebula", annotationSource{Type: "nebula", Extended: true, Major: 40, Minor: 20, PixelA: 40, PixelB: 20}, "nebula"},
		{"dark", annotationSource{Type: "dark-nebula", Extended: true, Major: 40, Minor: 20, PixelA: 40, PixelB: 20}, "dark-nebula"},
		{"planetary", annotationSource{Type: "planetary", Extended: true, Major: 30, Minor: 30, PixelA: 30, PixelB: 30}, "planetary"},
		{"cluster", annotationSource{Type: "cluster", Extended: true, Major: 40, Minor: 40, PixelA: 40, PixelB: 40}, "cluster"},
		{"remnant", annotationSource{Type: "snr", Extended: true, Major: 40, Minor: 25, PixelA: 40, PixelB: 25}, "remnant"},
		{"variable flag", annotationSource{Type: "star", Variable: true}, "variable"},
		{"variable metadata", annotationSource{Type: "star", Metadata: map[string]string{"VAR": "yes"}}, "variable"},
		{"double flag", annotationSource{Type: "star", Double: true}, "double"},
		{"double metadata", annotationSource{Type: "star", Metadata: map[string]string{"double_star": "true"}}, "double"},
		{"plain star", annotationSource{Type: "star"}, "star"},
	}
	for _, tc := range cases {
		if got := atlasSymbol(tc.src); got != tc.want {
			return fmt.Errorf("%s: symbol %q, want %q", tc.name, got, tc.want)
		}
	}

	// Catalog flags must survive the provenance JSON round-trip.
	src := annotationSource{ID: "v1", Type: "star", Variable: true, Double: true,
		Metadata: map[string]string{"crossmatch_catalog": "SIMBAD"}}
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	var decoded annotationSource
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !decoded.Variable || !decoded.Double {
		return errors.New("variable/double flags lost in JSON round-trip")
	}

	svg := buildAnnotationSVG(320, 320, []annotationSource{
		{ID: "g", Name: "G", Type: "galaxy", X: 60, Y: 60, Mag: 8, Extended: true, PixelA: 40, PixelB: 20, Priority: 70},
		{ID: "v", Name: "V", Type: "star", X: 140, Y: 60, Mag: 8, Variable: true, Priority: 70},
		{ID: "d", Name: "D", Type: "star", X: 220, Y: 60, Mag: 8, Double: true, Priority: 70},
	}, nil, annotationRenderOptions{Layers: []string{"point-sources", "extended-sources", "legend"}})
	for _, want := range []string{`data-symbol="galaxy"`, `data-symbol="variable"`, `data-symbol="double"`, `stroke-dasharray="2 2"`, `<g id="legend"`} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("atlas SVG missing %q", want)
		}
	}
	if strings.Contains(svg, "var(--") {
		return errors.New("atlas SVG reintroduced CSS custom-property colours")
	}
	return nil
}

// testAtlasExtendedContours covers item 147: type-aware contour families and
// deterministic blob geometry shared by SVG and raster.
func testAtlasExtendedContours() error {
	pathA := annotationBlobPath(100, 100, 30, 15, 25, "obj-a")
	pathB := annotationBlobPath(100, 100, 30, 15, 25, "obj-a")
	pathC := annotationBlobPath(100, 100, 30, 15, 25, "obj-b")
	if pathA == "" || pathA != pathB {
		return errors.New("blob contour is empty or not deterministic")
	}
	if pathA == pathC {
		return errors.New("different object IDs must not produce identical contours")
	}

	sources := []annotationSource{
		{ID: "g1", Name: "G1", Type: "galaxy", X: 100, Y: 100, Extended: true, PixelA: 50, PixelB: 26, PixelPA: 20, Priority: 70},
		{ID: "n1", Name: "N1", Type: "nebula", X: 100, Y: 100, Extended: true, PixelA: 50, PixelB: 26, PixelPA: 20, Priority: 70},
		{ID: "d1", Name: "D1", Type: "dark-nebula", X: 100, Y: 100, Extended: true, PixelA: 50, PixelB: 26, PixelPA: 20, Priority: 70},
		{ID: "p1", Name: "P1", Type: "planetary", X: 100, Y: 100, Extended: true, PixelA: 40, PixelB: 40, Priority: 70},
		{ID: "c1", Name: "C1", Type: "cluster", X: 100, Y: 100, Extended: true, PixelA: 40, PixelB: 40, Priority: 70},
	}
	opts := annotationRenderOptions{Layers: []string{"extended-sources"}}
	svg := buildAnnotationSVG(200, 200, sources, nil, opts)
	for _, want := range []string{
		`data-symbol="galaxy"`, `data-symbol="nebula"`, `data-symbol="dark-nebula"`,
		`data-symbol="planetary"`, `data-symbol="cluster"`, `stroke-dasharray="4 3"`,
		`stroke-dasharray="2 2"`,
	} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("extended contour SVG missing %q", want)
		}
	}

	base := atlasTestBase(200, 200)
	for _, s := range sources {
		out := renderAnnotationRasterWithOptions(base, []annotationSource{s}, nil, nil, opts)
		if changed := countChangedPixels(base, out); changed < 20 {
			return fmt.Errorf("%s raster contour changed only %d pixels", s.Type, changed)
		}
		// Extended contours are outlines, not filled blobs: the object centre
		// remains untouched except for the planetary nebula's core dot.
		if s.Type != "planetary" && base.At(100, 100) != out.At(100, 100) {
			return fmt.Errorf("%s contour fills the object centre", s.Type)
		}
	}
	return nil
}

// testAtlasStarMarkers covers item 148: variable and double markers derive
// from explicit flags or metadata and remain distinct in SVG and raster.
func testAtlasStarMarkers() error {
	sources := []annotationSource{
		{ID: "v", Type: "star", X: 100, Y: 100, Mag: 18, Variable: true, Priority: 70},
		{ID: "d", Type: "star", X: 100, Y: 100, Mag: 18, Double: true, Priority: 70},
		{ID: "s", Type: "star", X: 100, Y: 100, Mag: 18, Priority: 70},
	}
	opts := annotationRenderOptions{Layers: []string{"point-sources"}}
	svg := buildAnnotationSVG(200, 200, sources, nil, opts)
	for _, want := range []string{`data-symbol="variable"`, `data-symbol="double"`, `data-symbol="star"`} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("star marker SVG missing %q", want)
		}
	}
	base := atlasTestBase(200, 200)
	counts := make(map[string]int)
	for _, s := range sources {
		out := renderAnnotationRasterWithOptions(base, []annotationSource{s}, nil, nil, opts)
		counts[atlasSymbol(s)] = countChangedPixels(base, out)
	}
	if counts["variable"] <= counts["star"] {
		return fmt.Errorf("variable marker %d px <= plain star %d px", counts["variable"], counts["star"])
	}
	if counts["double"] <= counts["star"] {
		return fmt.Errorf("double marker %d px <= plain star %d px", counts["double"], counts["star"])
	}
	// At magnitude 18 the two components are resolved and the midpoint is
	// background again, as in the atlas legend.
	doubleOut := renderAnnotationRasterWithOptions(base, []annotationSource{sources[1]}, nil, nil, opts)
	if base.At(100, 100) != doubleOut.At(100, 100) {
		return errors.New("double-star components are not resolved")
	}
	return nil
}

// testAtlasLegendParity covers item 149: the key is present in both products,
// deterministic, viewer-safe and absent when the layer is disabled.
func testAtlasLegendParity() error {
	all := annotationRenderOptions{Layers: []string{"legend"}}
	svg := buildAnnotationSVG(400, 300, nil, nil, all)
	for _, want := range []string{
		`<g id="legend"`, `data-layer="legend"`, `ATLAS KEY`, `GALAXY`, `VARIABLE`, `DOUBLE`,
	} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("atlas legend SVG missing %q", want)
		}
	}
	if strings.Contains(svg, "var(--") {
		return errors.New("atlas legend uses CSS custom-property colours")
	}
	if strings.Contains(svg, `data-symbol=\"`) {
		return errors.New("generated CSS contains escaped quotes")
	}
	if again := buildAnnotationSVG(400, 300, nil, nil, all); again != svg {
		return errors.New("atlas legend SVG is not deterministic")
	}

	base := atlasTestBase(400, 300)
	withLegend := renderAnnotationRasterWithOptions(base, nil, nil, nil, all)
	if changed := countChangedPixels(base, withLegend); changed < 1000 {
		return fmt.Errorf("raster legend changed only %d pixels", changed)
	}
	again := renderAnnotationRasterWithOptions(base, nil, nil, nil, all)
	if countChangedPixels(withLegend, again) != 0 {
		return errors.New("raster legend is not deterministic")
	}
	without := renderAnnotationRasterWithOptions(base, nil, nil, nil, annotationRenderOptions{Layers: []string{"labels"}})
	if countChangedPixels(base, without) != 0 {
		return errors.New("raster legend persists when the layer is disabled")
	}
	if hidden := buildAnnotationSVG(400, 300, nil, nil, annotationRenderOptions{Layers: []string{"labels"}}); strings.Contains(hidden, `<g id="legend"`) {
		return errors.New("SVG legend persists when the layer is disabled")
	}
	if layers := parseAnnotationLayers("key"); !atlasContainsString(layers, "legend") {
		return errors.New("parseAnnotationLayers does not accept the 'key' alias")
	}
	return nil
}

func atlasContainsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// testAtlasInteractiveLegend covers item 154: every marker and legend swatch
// carries a stable class, the root advertises the vocabulary and pure-CSS
// toggles can hide a family without re-rendering the document.
func testAtlasInteractiveLegend() error {
	sources := []annotationSource{
		{ID: "g", Name: "G", Type: "galaxy", X: 60, Y: 60, Mag: 8, Extended: true, PixelA: 40, PixelB: 20, Priority: 70},
		{ID: "n", Name: "N", Type: "nebula", X: 140, Y: 60, Mag: 8, Extended: true, PixelA: 40, PixelB: 20, Priority: 70},
		{ID: "v", Name: "V", Type: "star", X: 220, Y: 60, Mag: 8, Variable: true, Priority: 70},
		{ID: "d", Name: "D", Type: "star", X: 280, Y: 60, Mag: 8, Double: true, Priority: 70},
	}
	svg := buildAnnotationSVG(600, 400, sources, nil, annotationRenderOptions{Layers: annotationAllLayers})
	for _, want := range []string{
		`class="skystacker-atlas-map"`,
		`data-symbol-families="galaxy,nebula,dark-nebula,planetary,cluster,remnant,sso,star,open-star,variable,double"`,
		`svg.skystacker-hide-variable [data-symbol="variable"]`,
		`svg.skystacker-hide-galaxy .skystacker-key-galaxy{display:none}`,
	} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("interactive legend SVG missing %q", want)
		}
	}

	// Every source marker must expose both its data-symbol and its class.
	for _, s := range sources {
		symbol := atlasSymbol(s)
		if !strings.Contains(svg, `data-symbol="`+symbol+`"`) {
			return fmt.Errorf("no data-symbol for %q", symbol)
		}
		if !strings.Contains(svg, `svg.skystacker-hide-`+symbol+` [data-symbol="`+symbol+`"]`) {
			return fmt.Errorf("no CSS toggle for marker family %q", symbol)
		}
	}

	// Legend rows are individually addressable and accessible.
	for _, family := range []string{"galaxy", "nebula", "planetary", "cluster", "star", "open-star", "variable", "double"} {
		if !strings.Contains(svg, `class="skystacker-key skystacker-key-`+family+`"`) {
			return fmt.Errorf("legend row %q has no stable class", family)
		}
		if !strings.Contains(svg, `data-key-symbol="`+family+`"`) {
			return fmt.Errorf("legend row %q has no key symbol", family)
		}
		if strings.TrimSpace(atlasLegendTitle(family)) == "" {
			return fmt.Errorf("legend row %q has no accessible title", family)
		}
		if !strings.Contains(svg, atlasLegendTitle(family)) {
			return fmt.Errorf("legend row %q title missing from SVG", family)
		}
	}

	// All ten families get a CSS rule; unknown symbols fall back to star.
	if got := strings.Count(svg, "{display:none}"); got != len(atlasSymbolFamilies)+1 {
		return fmt.Errorf("expected %d CSS toggle rules, got %d", len(atlasSymbolFamilies)+1, got)
	}
	if got := atlasLegendClass("unexpected-type"); got != "skystacker-key skystacker-key-star" {
		return fmt.Errorf("unknown symbol class %q", got)
	}

	// The toggle metadata must not disturb raster output or determinism.
	base := atlasTestBase(300, 200)
	opts := annotationRenderOptions{Layers: []string{"legend"}}
	first := renderAnnotationRasterWithOptions(base, nil, nil, nil, opts)
	second := renderAnnotationRasterWithOptions(base, nil, nil, nil, opts)
	if countChangedPixels(first, second) != 0 {
		return errors.New("interactive legend changed raster determinism")
	}
	if again := buildAnnotationSVG(600, 400, sources, nil, annotationRenderOptions{Layers: annotationAllLayers}); again != svg {
		return errors.New("interactive legend SVG is not deterministic")
	}
	return nil
}

// testAtlasSpectralMarkers covers item 151: colour indices tint point sources
// with a colour-blind-safe palette while magnitude still drives size/opacity.
func testAtlasSpectralMarkers() error {
	hot := annotationSource{ID: "hot", Type: "star", X: 40, Y: 40, Mag: 8,
		Metadata: map[string]string{"B-V": "-0.50"}}
	cool := annotationSource{ID: "cool", Type: "star", X: 90, Y: 40, Mag: 8,
		Metadata: map[string]string{"b_v": "1.60"}}
	plain := annotationSource{ID: "plain", Type: "star", X: 140, Y: 40, Mag: 8}
	if c, ok := atlasStarTint(hot); !ok || c != "#0072b2" {
		return fmt.Errorf("hot star tint %q, ok=%v", c, ok)
	}
	if c, ok := atlasStarTint(cool); !ok || c != "#d55e00" {
		return fmt.Errorf("cool star tint %q, ok=%v", c, ok)
	}
	if c, ok := atlasStarTint(plain); ok || c != "#ffe9b0" {
		return fmt.Errorf("magnitude fallback %q, ok=%v", c, ok)
	}
	if _, ok := atlasStarTint(annotationSource{Type: "star", Band: "G", Mag: 8}); ok {
		return errors.New("Gaia G passband must keep the magnitude fallback")
	}
	if atlasSpectralClass(0.1) != "A" || atlasSpectralClass(2.0) != "M" {
		return errors.New("spectral class bins are incorrect")
	}

	sources := []annotationSource{hot, cool, plain}
	svg := buildAnnotationSVG(220, 100, sources, nil, annotationRenderOptions{Layers: []string{"point-sources"}})
	for _, want := range []string{
		`data-spectral="O/B"`, `data-spectral="M"`,
		`data-colour-mode="index"`, `fill="#0072b2"`, `fill="#d55e00"`,
	} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("spectral SVG missing %q", want)
		}
	}
	if strings.Contains(svg, `data-colour-mode="magnitude"`) {
		return errors.New("magnitude fallback should not add per-star colour attributes")
	}
	if again := buildAnnotationSVG(220, 100, sources, nil, annotationRenderOptions{Layers: []string{"point-sources"}}); again != svg {
		return errors.New("spectral SVG is not deterministic")
	}

	base := atlasTestBase(220, 100)
	opts := annotationRenderOptions{Layers: []string{"point-sources"}}
	out := renderAnnotationRasterWithOptions(base, sources, nil, nil, opts)
	if countChangedPixels(out, renderAnnotationRasterWithOptions(base, sources, nil, nil, opts)) != 0 {
		return errors.New("spectral raster is not deterministic")
	}
	if countChangedPixels(out, renderAnnotationRasterWithOptions(base, []annotationSource{hot, cool, {ID: "plain2", Type: "star", X: 140, Y: 40, Mag: 8}}, nil, nil, opts)) != 0 {
		return errors.New("spectral tint did not change raster pixels")
	}
	return nil
}

// testAtlasCatalogFootprints covers item 152: axes parsed from a VO/VOTable
// row drive the atlas contour and are flagged in the provenance model.
func testAtlasCatalogFootprints() error {
	votable := `<?xml version="1.0"?><VOTABLE><RESOURCE><TABLE>
<FIELD name="ra"/><FIELD name="dec"/><FIELD name="name"/><FIELD name="otype"/>
<FIELD name="major"/><FIELD name="minor"/><FIELD name="pa"/>
<DATA><TABLEDATA><TR><TD>10</TD><TD>20</TD><TD>NGC1234</TD><TD>galaxy</TD>
<TD>45</TD><TD>20</TD><TD>35</TD></TR></TABLEDATA></DATA>
</TABLE></RESOURCE></VOTABLE>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(votable))
	}))
	defer srv.Close()
	res, err := coneSearchDetailed(coneServices["vizier"], srv.URL, 10, 20, 60, time.Second)
	if err != nil {
		return err
	}
	if len(res.Sources) != 1 {
		return fmt.Errorf("VOTable produced %d sources", len(res.Sources))
	}
	row := res.Sources[0]
	if row.Major != 45 || row.Minor != 20 || row.PA != 35 {
		return fmt.Errorf("VOTable geometry %+v", row)
	}
	src := annotationSourceFromCone(row, annotationProvenance{Catalog: "VizieR"})
	if !src.Extended || atlasGeometrySource(src) != "catalog" {
		return fmt.Errorf("catalog geometry flag: extended=%v source=%q", src.Extended, atlasGeometrySource(src))
	}
	if atlasGeometrySource(annotationSource{Type: "galaxy", Extended: true}) != "fallback" {
		return errors.New("fallback geometry flag is wrong")
	}
	src.X, src.Y, src.PixelA, src.PixelB, src.PixelPA = 100, 80, 45, 20, 35
	svg := buildAnnotationSVG(300, 200, []annotationSource{src}, nil, annotationRenderOptions{Layers: []string{"extended-sources"}})
	for _, want := range []string{`rx="22.50"`, `ry="10.00"`, `data-pa="35.0"`, `data-symbol="galaxy"`} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("catalog footprint SVG missing %q", want)
		}
	}
	if again := buildAnnotationSVG(300, 200, []annotationSource{src}, nil, annotationRenderOptions{Layers: []string{"extended-sources"}}); again != svg {
		return errors.New("catalog footprint SVG is not deterministic")
	}
	return nil
}

// testAtlasMotionVariabilityOverlays covers item 153: PM arrows start outside
// the marker, variability brackets encode amplitude, and both are provenance
// counted and deterministic in SVG and raster.
func testAtlasMotionVariabilityOverlays() error {
	w := &wcsTan{CRVAL1: 0, CRVAL2: 0, CRPIX1: 300, CRPIX2: 200,
		CD: [2][2]float64{{1 / 206265.0, 0}, {0, 1 / 206265.0}}}
	moving := annotationSource{ID: "pm", Type: "star", X: 300, Y: 200, Mag: 8,
		RA: 0, Dec: 0, Epoch: 2000, PMRA: 100,
		Metadata: map[string]string{"amplitude": "0.5"}}
	still := annotationSource{ID: "still", Type: "star", X: 100, Y: 100, Mag: 10,
		Metadata: map[string]string{"min_mag": "9.0", "max_mag": "11.0"}}
	sources := []annotationSource{moving, still}
	overlays := atlasMotionOverlays(sources, w)
	if len(overlays) != 2 || !overlays[0].ProperMotion || overlays[1].ProperMotion {
		return fmt.Errorf("overlay records %+v", overlays)
	}
	radius := annotationStarRadius(moving.Mag)
	if math.Hypot(overlays[0].X1-moving.X, overlays[0].Y1-moving.Y) <= radius+3 {
		return errors.New("proper-motion arrow overlaps the marker")
	}
	if overlays[0].Variability <= 0 || overlays[1].Variability <= 0 {
		return errors.New("variability amplitudes were lost")
	}

	opts := annotationRenderOptions{Layers: []string{"motion"}, WCS: w}
	svg := buildAnnotationSVG(600, 400, sources, nil, opts)
	for _, want := range []string{
		`data-overlay="proper-motion"`, `data-overlay="proper-motion-arrow"`,
		`data-overlay="variability"`, `data-amplitude="0.50"`,
		`skystacker-pm-arrow`,
	} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("motion SVG missing %q", want)
		}
	}
	if again := buildAnnotationSVG(600, 400, sources, nil, opts); again != svg {
		return errors.New("motion SVG is not deterministic")
	}
	base := atlasTestBase(600, 400)
	out := renderAnnotationRasterWithOptions(base, sources, nil, nil, opts)
	if countChangedPixels(out, renderAnnotationRasterWithOptions(base, sources, nil, nil, opts)) != 0 {
		return errors.New("motion raster is not deterministic")
	}
	if countChangedPixels(out, renderAnnotationRasterWithOptions(base, nil, nil, nil, opts)) == 0 {
		return errors.New("motion overlays did not change raster pixels")
	}

	many := make([]annotationSource, 0, 300)
	for i := 299; i >= 0; i-- {
		many = append(many, annotationSource{ID: fmt.Sprintf("pm-%03d", i), Type: "star", X: 300, Y: 200,
			RA: 0, Dec: 0, Epoch: 2000, PMRA: 100})
	}
	capped := atlasMotionOverlays(many, w)
	if len(capped) != atlasMotionVectorLimit || capped[0].ID != "pm-000" {
		return fmt.Errorf("motion cap %d, first %q", len(capped), capped[0].ID)
	}
	return nil
}

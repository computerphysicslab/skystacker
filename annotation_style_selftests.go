package main

// Self-tests for roadmap item 157: SVG label collars and the optional
// open-circle point-source style. Both renderers must express the same style
// state without moving labels or collapsing scientifically distinct markers.

import (
	"errors"
	"fmt"
	"image"
	"strings"
)

// testAnnotationLabelCollar checks the SVG paint-order stroke, the raster
// twin and, critically, that label placement is independent of visual style.
func testAnnotationLabelCollar() error {
	const w, h = 520, 360
	sources := annotationFixtureSources(40, 40, 3)
	labels := placeAnnotationLabels(w, h, sources, 1)
	if len(labels) == 0 {
		return errors.New("label collar test has no labels")
	}
	plainOpts := annotationRenderOptions{Layers: []string{"labels"}, LabelCollar: false}
	collarOpts := annotationRenderOptions{Layers: []string{"labels"}, LabelCollar: true}
	plainSVG := buildAnnotationSVG(w, h, sources, nil, plainOpts)
	collarSVG := buildAnnotationSVG(w, h, sources, nil, collarOpts)
	if !strings.Contains(plainSVG, `data-collar="none"`) ||
		!strings.Contains(collarSVG, `data-collar="dark"`) {
		return errors.New("SVG label collar state is missing")
	}
	if strings.Contains(plainSVG, `paint-order="stroke"`) ||
		!strings.Contains(collarSVG, `paint-order="stroke" stroke="#05070d" stroke-width="2.6"`) {
		return errors.New("SVG collar stroke does not match the selected style")
	}
	for _, l := range labels {
		pos := fmt.Sprintf(`x="%.2f" y="%.2f"`, l.X, l.Y+skyLabelFontSize)
		if !strings.Contains(plainSVG, pos) || !strings.Contains(collarSVG, pos) {
			return fmt.Errorf("label collar moved label %q", l.Text)
		}
	}
	if again := buildAnnotationSVG(w, h, sources, nil, collarOpts); again != collarSVG {
		return errors.New("collared SVG is not deterministic")
	}
	base := image.NewRGBA(image.Rect(0, 0, w, h))
	plain := renderAnnotationRasterWithOptions(base, sources, nil, labels, plainOpts)
	collar := renderAnnotationRasterWithOptions(base, sources, nil, labels, collarOpts)
	if countChangedPixels(plain, collar) == 0 {
		return errors.New("raster collar produced no visible change")
	}
	if again := renderAnnotationRasterWithOptions(base, sources, nil, labels, collarOpts); countChangedPixels(collar, again) != 0 {
		return errors.New("collared raster is not deterministic")
	}
	return nil
}

// testOpenStarStyle verifies that only ordinary point sources adopt the open
// circle, that the legend/CSS vocabulary follows and that the raster twin
// changes in exactly the same style state.
func testOpenStarStyle() error {
	sources := []annotationSource{
		{ID: "star", Name: "STAR", Type: "star", X: 60, Y: 60, Mag: 6, Priority: 70},
		{ID: "var", Name: "VAR", Type: "star", X: 130, Y: 60, Mag: 6, Variable: true, Priority: 70},
		{ID: "double", Name: "DOUBLE", Type: "star", X: 200, Y: 60, Mag: 6, Double: true, Priority: 70},
		{ID: "galaxy", Name: "GAL", Type: "galaxy", X: 270, Y: 60, Mag: 10, Extended: true, PixelA: 40, PixelB: 20, Priority: 70},
	}
	filledOpts := annotationRenderOptions{Layers: annotationAllLayers, StarStyle: "filled"}
	openOpts := annotationRenderOptions{Layers: annotationAllLayers, StarStyle: "open"}
	filledSVG := buildAnnotationSVG(380, 220, sources, nil, filledOpts)
	openSVG := buildAnnotationSVG(380, 220, sources, nil, openOpts)
	if !strings.Contains(filledSVG, `data-symbol="star"`) ||
		!strings.Contains(openSVG, `data-symbol="open-star"`) {
		return errors.New("open-star style did not change the point-source family")
	}
	for _, want := range []string{
		`data-star-style="open"`, `OPEN STAR`,
		`svg.skystacker-hide-open-star [data-symbol="open-star"]`,
		`fill="none" stroke="#ffe9b0"`,
	} {
		if !strings.Contains(openSVG, want) {
			return fmt.Errorf("open-star SVG missing %q", want)
		}
	}
	// The CSS toggle rule is present in every document; only the open style
	// may additionally emit an actual marker with that data-symbol.
	if strings.Count(filledSVG, `data-symbol="open-star"`) != 1 ||
		strings.Count(openSVG, `data-symbol="open-star"`) != 2 {
		return errors.New("open-star marker family leaked into the filled style")
	}
	for _, symbol := range []string{"variable", "double", "galaxy"} {
		if !strings.Contains(filledSVG, `data-symbol="`+symbol+`"`) ||
			!strings.Contains(openSVG, `data-symbol="`+symbol+`"`) {
			return fmt.Errorf("open-star style altered %s marker family", symbol)
		}
	}
	if got := atlasLegendClass("open-star"); !strings.HasSuffix(got, "skystacker-key-open-star") {
		return fmt.Errorf("open-star legend class %q", got)
	}
	if strings.TrimSpace(atlasLegendTitle("open-star")) == "" {
		return errors.New("open-star legend swatch has no accessible title")
	}
	base := atlasTestBase(380, 220)
	filled := renderAnnotationRasterWithOptions(base, sources, nil, nil, filledOpts)
	open := renderAnnotationRasterWithOptions(base, sources, nil, nil, openOpts)
	if countChangedPixels(filled, open) == 0 {
		return errors.New("open-star raster produced no visible change")
	}
	if again := renderAnnotationRasterWithOptions(base, sources, nil, nil, openOpts); countChangedPixels(open, again) != 0 {
		return errors.New("open-star raster is not deterministic")
	}
	return nil
}

// testAnnotationStyleCLI locks the two new flags through parseArgs, the
// positional-input preprocessor and the reproducible command-line report.
func testAnnotationStyleCLI() error {
	opts, err := parseArgs([]string{
		"assets/annotation-examples", "--annotation-star-style", "open",
		"--annotation-label-collar=false", "--annotation-names", "off",
	})
	if err != nil || opts.InputDir != "assets/annotation-examples" ||
		opts.AnnotationStarStyle != "open" || opts.AnnotationLabelCollar {
		return fmt.Errorf("annotation style flag parsing: style=%q collar=%v err=%v",
			opts.AnnotationStarStyle, opts.AnnotationLabelCollar, err)
	}
	if _, err := parseArgs([]string{"--annotation-star-style", "hollow"}); err == nil {
		return errors.New("invalid annotation star style must be rejected")
	}
	lines := resolvedFlagLines(runOptions{
		Annotate:              true,
		AnnotationLabelCollar: false,
		AnnotationStarStyle:   "open",
	})
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"--annotation-label-collar=false", "--annotation-star-style open"} {
		if !strings.Contains(joined, want) {
			return fmt.Errorf("resolved flag line missing %q", want)
		}
	}
	defaults := strings.Join(resolvedFlagLines(runOptions{Annotate: true, AnnotationLabelCollar: true}), "\n")
	if !strings.Contains(defaults, "--annotation-label-collar") ||
		strings.Contains(defaults, "--annotation-star-style") {
		return errors.New("default annotation style flags are not reproducible")
	}
	return nil
}

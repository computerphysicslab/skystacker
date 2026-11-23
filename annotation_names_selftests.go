package main

// Self-tests for roadmap items 155 (named-star catalogue and cross-match
// priority) and 158 (vendor-style M31 example corpus acceptance).

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

func namedBundleSource(id string) (annotationSource, bool) {
	for _, s := range offlineAnnotationBundle().Sources {
		if s.ID == id {
			return s, true
		}
	}
	return annotationSource{}, false
}

// testNamedStarCatalogue covers item 155: the offline bundle carries proper
// names, Bayer and Flamsteed designations for the Andromeda field and keeps
// a deterministic, compact representation.
func testNamedStarCatalogue() error {
	type want struct {
		id, name, bayer, flam, constellation string
		rank                                 int
	}
	cases := []want{
		{"hip-677", "Alpheratz / Sirrah", "alpha And", "21 And", "And", 3},
		{"hip-5447", "Mirach", "beta And", "43 And", "And", 3},
		{"hip-5550", "45 And", "", "45 And", "And", 1},
		{"hip-11313", "65 And", "", "65 And", "And", 1},
		{"hip-145", "29 Psc", "", "29 Psc", "Psc", 1},
	}
	for _, c := range cases {
		s, ok := namedBundleSource(c.id)
		if !ok {
			return fmt.Errorf("named star %s missing from offline bundle", c.id)
		}
		if s.Name != c.name || s.Type != "star" || !s.Provenance.Offline {
			return fmt.Errorf("%s: name=%q type=%q offline=%v", c.id, s.Name, s.Type, s.Provenance.Offline)
		}
		if s.Metadata["bayer"] != c.bayer || s.Metadata["flamsteed"] != c.flam ||
			s.Metadata["constellation"] != c.constellation || annotationNameRank(s) != c.rank {
			return fmt.Errorf("%s: metadata=%+v rank=%d", c.id, s.Metadata, annotationNameRank(s))
		}
		if s.RA < 0 || s.RA > 2*math.Pi || s.Dec < -math.Pi/2 || s.Dec > math.Pi/2 {
			return fmt.Errorf("%s: invalid coordinates %f %f", c.id, s.RA, s.Dec)
		}
	}
	parsed := parseNamedStarTable(offlineNamedStars, annotationProvenance{Catalog: "test", Offline: true})
	if len(parsed) != 3241 {
		return fmt.Errorf("named star table: got %d entries, want 3241", len(parsed))
	}
	if again := parseNamedStarTable(offlineNamedStars, annotationProvenance{Catalog: "test", Offline: true}); len(again) != len(parsed) ||
		again[0].Name != parsed[0].Name || again[len(again)-1].ID != parsed[len(parsed)-1].ID {
		return errors.New("named star table parse is not deterministic")
	}
	seen := make(map[string]bool, len(parsed))
	for _, s := range parsed {
		if seen[s.ID] || seen[s.Name] {
			return fmt.Errorf("duplicate named star id/name %q / %q", s.ID, s.Name)
		}
		seen[s.ID], seen[s.Name] = true, true
	}
	return nil
}

// testNamedStarCrossmatchPriority covers item 155: a readable proper name wins
// over Flamsteed or anonymous candidates at essentially the same position, but
// a clearly closer anonymous source is never displaced.
func testNamedStarCrossmatchPriority() error {
	gaia := annotationSource{ID: "gaia-1", Name: "Gaia", Type: "gaia",
		RA: 17.433 * math.Pi / 180, Dec: 35.6206 * math.Pi / 180, Mag: 2.07}
	named := annotationSource{ID: "hip-5447", Name: "Mirach", Type: "star",
		RA: gaia.RA, Dec: gaia.Dec, Metadata: map[string]string{"name_rank": "3"}}
	flam := annotationSource{ID: "bundle-43-And", Name: "43 And", Type: "star",
		RA: gaia.RA + (1.0/3600)*math.Pi/180, Dec: gaia.Dec,
		Metadata: map[string]string{"name_rank": "1"}}
	anon := annotationSource{ID: "vizier-1", Name: "VizieR 1", Type: "star",
		RA: gaia.RA + (2.0/3600)*math.Pi/180, Dec: gaia.Dec}

	out, report := crossMatchGaiaNames([]annotationSource{gaia}, []annotationSource{flam, anon, named}, 10)
	if len(out) != 1 || out[0].Name != "Mirach" || out[0].Metadata["crossmatch_id"] != "hip-5447" {
		return fmt.Errorf("proper-name priority failed: %+v", out)
	}
	if report.Matched != 1 || report.Designated != 1 || len(report.Examples) == 0 {
		return fmt.Errorf("cross-match report: %+v", report)
	}
	out, _ = crossMatchGaiaNames([]annotationSource{gaia}, []annotationSource{flam, anon}, 10)
	if len(out) != 1 || out[0].Name != "43 And" {
		return fmt.Errorf("Flamsteed fallback failed: %+v", out)
	}
	near := annotationSource{ID: "anon-near", Name: "Near", Type: "star",
		RA: gaia.RA + (0.1/3600)*math.Pi/180, Dec: gaia.Dec}
	farNamed := annotationSource{ID: "named-far", Name: "Far Name", Type: "star",
		RA: gaia.RA + (6.0/3600)*math.Pi/180, Dec: gaia.Dec,
		Metadata: map[string]string{"name_rank": "3"}}
	out, _ = crossMatchGaiaNames([]annotationSource{gaia}, []annotationSource{farNamed, near}, 10)
	if len(out) != 1 || out[0].Name != "Near" {
		return fmt.Errorf("rank bonus displaced a genuinely closer source: %+v", out)
	}
	return nil
}

// testVendorExampleCorpus covers item 158: the two new M31 references are
// documented, valid WebP files when present, and the bundled catalogue can
// reproduce the Andromeda-field labels of the wide-field example.
func testVendorExampleCorpus() error {
	readme, err := os.ReadFile(filepath.Join("assets", "annotation-examples", "README.md"))
	if err != nil {
		return err
	}
	for _, name := range []string{
		"m31camera-annotated.webp",
		"bc9374_cacccbac9cb2432e9061aae64f812dfd.webp",
	} {
		if !strings.Contains(string(readme), name) {
			return fmt.Errorf("vendor example %s is not documented", name)
		}
		path := filepath.Join("assets", "annotation-examples", name)
		data, statErr := os.ReadFile(path)
		if os.IsNotExist(statErr) {
			continue // gitignored reference material, optional in a checkout
		}
		if statErr != nil {
			return statErr
		}
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
			return fmt.Errorf("%s is not a WebP file", name)
		}
	}
	w := rotatedWCS(20*math.Pi/180, 40*math.Pi/180, 240, 0, false)
	ids := map[string]bool{
		"hip-677": true, "hip-5447": true, "hip-5550": true, "hip-11313": true,
	}
	field := make([]annotationSource, 0, len(ids))
	for _, s := range offlineAnnotationBundle().Sources {
		if ids[s.ID] {
			field = append(field, s)
		}
	}
	projected := projectAnnotationSources(w, 600, 600, 2024.6, field)
	if len(projected) != len(ids) {
		return fmt.Errorf("Andromeda field projection: %d/%d sources", len(projected), len(ids))
	}
	svg := buildAnnotationSVG(600, 600, projected, nil, annotationRenderOptions{Layers: annotationAllLayers})
	for _, label := range []string{"Alpheratz / Sirrah", "Mirach", "45 And", "65 And"} {
		if !strings.Contains(svg, label) {
			return fmt.Errorf("vendor-style label %q missing from SVG", label)
		}
	}
	if again := buildAnnotationSVG(600, 600, projected, nil, annotationRenderOptions{Layers: annotationAllLayers}); again != svg {
		return errors.New("vendor-style SVG is not deterministic")
	}
	// Optional visual-review artifact. Normal self-test runs never write files;
	// the atlas review workflow sets SKYSTACKER_REVIEW_DIR explicitly.
	if dir := strings.TrimSpace(os.Getenv("SKYSTACKER_REVIEW_DIR")); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		showcase := buildAnnotationSVG(600, 600, projected, nil,
			annotationRenderOptions{WCS: w, Layers: annotationAllLayers})
		if err := os.WriteFile(filepath.Join(dir, "named-stars-wide-field.svg"), []byte(showcase), 0o644); err != nil {
			return err
		}
		manifest := fmt.Sprintf("{\"item\":155,\"sources\":%d,\"labels\":[\"Alpheratz / Sirrah\",\"Mirach\",\"45 And\",\"65 And\"],\"catalog\":\"2026-11-23-1\"}\n", len(projected))
		if err := os.WriteFile(filepath.Join(dir, "named-stars-wide-field.json"), []byte(manifest), 0o644); err != nil {
			return err
		}
	}
	return nil
}

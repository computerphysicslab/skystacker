package main

// Self-tests for roadmap item 156: offline PGC designations, normalisation of
// LEDA aliases and deterministic cross-matching of anonymous extended objects.

import (
	"errors"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// testPGCDestinationNormalisation locks the accepted spellings and rejects
// values that could otherwise create unstable or misleading labels.
func testPGCDestinationNormalisation() error {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"PGC 21991", "PGC 21991", true},
		{"pgc 021991", "PGC 21991", true},
		{"LEDA 21991", "PGC 21991", true},
		{"Leda021991", "PGC 21991", true},
		{" PGC\t2167368 ", "PGC 2167368", true},
		{"PGC", "", false},
		{"PGC 0", "", false},
		{"PGC -1", "", false},
		{"NGC 21991", "", false},
		{"PGC 12A", "", false},
	}
	for _, tc := range cases {
		got, ok := pgcDesignation(tc.in)
		if ok != tc.ok || got != tc.want {
			return fmt.Errorf("pgcDesignation(%q) = %q,%v; want %q,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	if annotationExtendedNameRank(annotationSource{Name: "M31"}) != 3 {
		return errors.New("Messier name must outrank PGC")
	}
	if annotationExtendedNameRank(annotationSource{Name: "LEDA 21991"}) != 2 {
		return errors.New("LEDA alias must rank as a PGC designation")
	}
	if annotationExtendedNameRank(annotationSource{Name: "WISEA J0039+4135"}) != 1 {
		return errors.New("survey identifier must rank below PGC")
	}
	return nil
}

// testPGCOfflineTable checks the compact NED-derived reference table without
// relying on a network connection at test time.
func testPGCOfflineTable() error {
	want := map[string][2]float64{
		"PGC 21991":   {117.824542, -25.134556},
		"PGC 2167368": {11.829012, 40.542609},
		"PGC 2183568": {138.937416, 41.593928},
		"PGC 3087883": {9.853164, 41.598406},
	}
	srcs := offlinePGCSources(annotationProvenance{Catalog: "test", Offline: true})
	if len(srcs) != len(want) {
		return fmt.Errorf("offline PGC table: %d entries, want %d", len(srcs), len(want))
	}
	for _, s := range srcs {
		pos, ok := want[s.Name]
		if !ok {
			return fmt.Errorf("unexpected offline PGC entry %q", s.Name)
		}
		if math.Abs(s.RA*180/math.Pi-pos[0]) > 1e-5 || math.Abs(s.Dec*180/math.Pi-pos[1]) > 1e-5 {
			return fmt.Errorf("%s position mismatch", s.Name)
		}
		if !s.Extended || s.Priority >= 90 {
			return fmt.Errorf("%s must be an extended object below M31 priority", s.Name)
		}
		delete(want, s.Name)
	}
	if len(want) != 0 {
		return fmt.Errorf("offline PGC table missing %d entries", len(want))
	}
	m31 := bundleSourceByName("M31")
	if !m31.Extended || m31.Priority <= 80 {
		return errors.New("M31 priority must remain above PGC entries")
	}
	return nil
}

// bundleSourceByName finds a source in the offline bundle by exact name.
func bundleSourceByName(name string) annotationSource {
	for _, s := range offlineAnnotationBundle().Sources {
		if s.Name == name {
			return s
		}
	}
	return annotationSource{}
}

// testPGCCrossmatchPriority verifies that anonymous galaxies adopt PGC labels,
// named objects keep theirs, stars are untouched and repeated runs are stable.
// PGC 21991 is supplied only as a VO LEDA alias, proving that designation
// aliases and their online provenance survive normalisation.
func testPGCCrossmatchPriority() error {
	deg := math.Pi / 180
	candidates := make([]annotationSource, 0, 4)
	for _, c := range offlinePGCSources(annotationProvenance{Catalog: "NED", Offline: true}) {
		if c.Name != "PGC 21991" {
			candidates = append(candidates, c)
		}
	}
	candidates = append(candidates, annotationSource{
		ID: "vizier-leda", Name: "LEDA 021991", Type: "galaxy", Extended: true,
		RA: 117.824542 * deg, Dec: -25.134556 * deg,
		Provenance: annotationProvenance{Catalog: "VizieR", Offline: false},
	})
	sources := []annotationSource{
		{ID: "unknown-1", Name: "WISEA J0039+4135", Type: "galaxy", Extended: true,
			RA: 9.853164 * deg, Dec: 41.598406 * deg, Priority: 50},
		{ID: "unknown-2", Name: "", Type: "galaxy", Extended: true,
			RA: 11.829012 * deg, Dec: 40.542609 * deg, Major: 30, Minor: 20, Priority: 50},
		{ID: "unknown-3", Name: "SDSS J0915+4135", Type: "galaxy", Extended: true,
			RA: 138.937416 * deg, Dec: 41.593928 * deg, Priority: 50},
		{ID: "unknown-4", Name: "2MASX J07511789-2508044", Type: "galaxy", Extended: true,
			RA: 117.824542 * deg, Dec: -25.134556 * deg, Priority: 50},
		{ID: "named", Name: "M31", Type: "galaxy", Extended: true,
			RA: 10.685 * deg, Dec: 41.269 * deg, Priority: 90},
		{ID: "star", Name: "Gaia", Type: "gaia", RA: 9.853164 * deg, Dec: 41.598406 * deg, Priority: 40},
		{ID: "far", Name: "", Type: "galaxy", Extended: true, RA: 200 * deg, Dec: 10 * deg, Priority: 50},
	}
	out, report := crossMatchPGCNames(sources, candidates, 10)
	if report.Candidates != 4 || report.Matched != 4 {
		return fmt.Errorf("PGC report candidates=%d matched=%d; want 4/4", report.Candidates, report.Matched)
	}
	got := make(map[string]string)
	for _, s := range out {
		got[s.ID] = s.Name
	}
	for id, want := range map[string]string{
		"unknown-1": "PGC 3087883", "unknown-2": "PGC 2167368",
		"unknown-3": "PGC 2183568", "unknown-4": "PGC 21991",
		"named": "M31", "star": "Gaia", "far": "",
	} {
		if got[id] != want {
			return fmt.Errorf("%s label %q, want %q", id, got[id], want)
		}
	}
	for _, s := range out {
		switch s.ID {
		case "unknown-1":
			if s.Priority != 80 || s.Metadata["pgc_id"] != "3087883" || !s.Provenance.Offline {
				return errors.New("offline PGC cross-match metadata/provenance missing")
			}
		case "unknown-4":
			if s.Provenance.Offline || s.Metadata["crossmatch_catalog"] != "VizieR" {
				return errors.New("VO-sourced PGC alias must keep online provenance")
			}
		case "named":
			if s.Priority != 90 {
				return errors.New("Messier label priority was overwritten by PGC")
			}
		}
	}
	again, againReport := crossMatchPGCNames(sources, candidates, 10)
	if againReport.Candidates != report.Candidates || againReport.Matched != report.Matched ||
		strings.Join(againReport.Examples, ",") != strings.Join(report.Examples, ",") ||
		len(again) != len(out) {
		return errors.New("PGC cross-match report is not deterministic")
	}
	for i := range out {
		if again[i].Name != out[i].Name || again[i].Priority != out[i].Priority {
			return errors.New("PGC cross-match order is not deterministic")
		}
	}
	return nil
}

// testPGCCandidateDedup checks that a candidate present as both a rendered
// source and a VO alias does not produce two labels for the same galaxy.
func testPGCCandidateDedup() error {
	deg := math.Pi / 180
	candidate := annotationSource{ID: "pgc", Name: "PGC 21991", Type: "galaxy", Extended: true,
		RA: 117.824542 * deg, Dec: -25.134556 * deg, Priority: 80,
		Provenance: annotationProvenance{Catalog: "NED", Offline: true}}
	sources := []annotationSource{
		candidate,
		{ID: "anonymous", Name: "", Type: "galaxy", Extended: true,
			RA: 117.824542 * deg, Dec: -25.134556 * deg, Priority: 50},
	}
	out, report := crossMatchPGCNames(sources, []annotationSource{candidate}, 10)
	if report.Matched != 0 || len(out) != 1 || out[0].Name != "PGC 21991" {
		return fmt.Errorf("dedup produced %d sources, %+v", len(out), report)
	}
	return nil
}

// testPGCVisualFixture emits a compact review artifact with all four vendor
// reference labels. Normal selftest runs never write files; the atlas review
// workflow sets SKYSTACKER_REVIEW_DIR explicitly.
func testPGCVisualFixture() error {
	srcs := offlinePGCSources(annotationProvenance{Catalog: "NED", Offline: true})
	positions := [][2]float64{{40, 90}, {160, 130}, {280, 80}, {400, 140}}
	for i := range srcs {
		srcs[i].X, srcs[i].Y = positions[i][0], positions[i][1]
		srcs[i].PixelA, srcs[i].PixelB = 24, 12
	}
	opts := annotationRenderOptions{Layers: []string{"point-sources", "extended-sources", "labels", "legend"}}
	svg := buildAnnotationSVG(480, 240, srcs, nil, opts)
	for _, name := range []string{"PGC 21991", "PGC 2167368", "PGC 2183568", "PGC 3087883"} {
		if !strings.Contains(svg, name) {
			return fmt.Errorf("PGC visual fixture missing %s", name)
		}
	}
	if again := buildAnnotationSVG(480, 240, srcs, nil, opts); again != svg {
		return errors.New("PGC visual fixture is not deterministic")
	}
	dir := strings.TrimSpace(os.Getenv("SKYSTACKER_REVIEW_DIR"))
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "pgc-catalogue-fixture.svg"), []byte(svg), 0o644); err != nil {
		return err
	}
	base := atlasTestBase(480, 240)
	raster := renderAnnotationRasterWithOptions(base, srcs, nil, placeAnnotationLabels(480, 240, srcs, 1), opts)
	f, err := os.Create(filepath.Join(dir, "pgc-catalogue-fixture.png"))
	if err != nil {
		return err
	}
	if err := png.Encode(f, raster); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

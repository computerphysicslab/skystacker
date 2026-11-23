package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// testExtendedDeepSkyCatalogue locks the generated catalogue: row count,
// ordering, valid geometry and the classification of well-known objects.
// A truncated embed or a mislabelled Messier object is a release blocker.
func testExtendedDeepSkyCatalogue() error {
	rows := parseExtendedDSOTable()
	if len(rows) != 19292 {
		return fmt.Errorf("extended deep-sky rows: got %d, want 19292", len(rows))
	}
	seen := make(map[string]bool, len(rows))
	prevRA := -1.0
	for i, row := range rows {
		if row.RA < prevRA {
			return fmt.Errorf("row %d not ordered by RA: %.5f after %.5f", i, row.RA, prevRA)
		}
		prevRA = row.RA
		key := annotationIdentity(row.Name)
		if seen[key] {
			return fmt.Errorf("duplicate designation %q at row %d", row.Name, i)
		}
		seen[key] = true
		if row.RA < 0 || row.RA >= 360 || row.Dec < -90 || row.Dec > 90 ||
			row.Major < 0 || row.Minor < 0 || row.Minor > row.Major ||
			row.Priority < 1 || row.Priority > 100 || row.Type == "" {
			return fmt.Errorf("invalid row %d: %+v", i, row)
		}
	}
	want := map[string]string{
		"m31": "galaxy", "m42": "nebula", "m13": "cluster",
		"m57": "planetary", "ngc6960": "remnant", "ldn846": "dark-nebula",
	}
	for key, kind := range want {
		found := false
		for _, row := range rows {
			if annotationIdentity(row.Name) == key {
				found = row.Type == kind
				break
			}
		}
		if !found {
			return fmt.Errorf("catalogue object %s missing or misclassified as %s", key, kind)
		}
	}
	for _, row := range rows {
		if annotationIdentity(row.Name) == "m31" {
			joined := strings.Join(row.Aliases, ",")
			if !strings.Contains(joined, "NGC 224") || !strings.Contains(joined, "Andromeda Galaxy") {
				return fmt.Errorf("M 31 lost its cross-identifiers: %q", joined)
			}
			break
		}
	}
	return nil
}

// testExtendedDeepSkyFieldFilter verifies that the declination index returns
// the expected M 31 neighbourhood and that duplicate bundle names are skipped.
func testExtendedDeepSkyFieldFilter() error {
	const ra, dec = 10.685, 41.269
	all := extendedDSOForField(ra, dec, 3600)
	if len(all) != 4 {
		return fmt.Errorf("M 31 field rows: got %d, want 4", len(all))
	}
	names := make(map[string]bool, len(all))
	for _, row := range all {
		names[annotationIdentity(row.Name)] = true
		sep := math.Hypot((row.RA-ra)*math.Cos(dec*math.Pi/180), row.Dec-dec)
		if sep > 1.0001 {
			return fmt.Errorf("field row %s outside cone: %.6f deg", row.Name, sep)
		}
	}
	for _, key := range []string{"m31", "m32", "m110", "ngc0206"} {
		if !names[key] {
			return fmt.Errorf("field missing %s", key)
		}
	}
	known := map[string]bool{"m31": true}
	sources := extendedDSOSources(ra, dec, 3600, extendedDSOProvenance(), known)
	if len(sources) != 3 {
		return fmt.Errorf("deduplicated field sources: got %d, want 3", len(sources))
	}
	for _, src := range sources {
		if annotationIdentity(src.Name) == "m31" {
			return errors.New("bundle duplicate M 31 was not suppressed")
		}
		if !src.Extended || !src.Provenance.Offline {
			return fmt.Errorf("deep-sky source %s not marked extended/offline", src.Name)
		}
	}
	if got := extendedDSOForField(180, -60, 600); len(got) != 0 {
		return fmt.Errorf("empty field returned %d rows", len(got))
	}
	return nil
}

// testExtendedDeepSkyAcquisition checks the offline acquisition path: the
// small bundle keeps its hand-curated M 31, the extended catalogue adds the
// surrounding objects and provenance is reported once.
func testExtendedDeepSkyAcquisition() error {
	opts := annotationAcquisitionOptions{
		Offline: true, DeepSky: true, RADeg: 10.685, DecDeg: 41.269,
		RadiusArcsec: 600, DeepSkyRadiusArcsec: 3600,
	}
	base, err := acquireAnnotationSources(context.Background(), annotationAcquisitionOptions{
		Offline: true, RADeg: opts.RADeg, DecDeg: opts.DecDeg, RadiusArcsec: 600,
	})
	if err != nil {
		return err
	}
	acq, err := acquireAnnotationSources(context.Background(), opts)
	if err != nil {
		return err
	}
	if len(acq.Sources) <= len(base.Sources) || len(acq.Sources) != len(base.Sources)+3 {
		return fmt.Errorf("acquisition sizes: base %d, extended %d", len(base.Sources), len(acq.Sources))
	}
	counts := map[string]int{}
	for _, src := range acq.Sources {
		counts[annotationIdentity(src.Name)]++
	}
	if counts["m31"] != 1 {
		return fmt.Errorf("M 31 appears %d times", counts["m31"])
	}
	provs := 0
	for _, prov := range acq.Provenance {
		if strings.Contains(prov.Catalog, "OpenNGC") && prov.Offline {
			provs++
		}
	}
	if provs != 1 {
		return fmt.Errorf("deep-sky provenance entries: %d", provs)
	}
	return nil
}

// testExtendedDeepSkyLayeredOutput covers the CLI path and the provenance
// report produced by a real layered annotation run with the catalogue on.
func testExtendedDeepSkyLayeredOutput() error {
	dir, err := os.MkdirTemp("", "skystacker-deepsky-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(filepath.Join(dir, "_work"), 0o755); err != nil {
		return err
	}
	const w, h = 512, 512
	raw := make([]byte, 8+w*h*3*4)
	binary.LittleEndian.PutUint32(raw[0:4], uint32(w))
	binary.LittleEndian.PutUint32(raw[4:8], uint32(h))
	if err := os.WriteFile(filepath.Join(dir, "_work", "stacked_mean.f32raw"), raw, 0o644); err != nil {
		return err
	}
	sol := plateSolution{WCS: *rotatedWCS(10.0*math.Pi/180, 41.0*math.Pi/180, 120, 0, false)}
	if err := writeJSON(filepath.Join(dir, "plate_solution.json"), &sol); err != nil {
		return err
	}
	if err := runLayeredAnnotation(dir, runOptions{GaiaOffline: true, AnnotationDeepSky: true}); err != nil {
		return err
	}
	var report layeredAnnotationReport
	if err := json.Unmarshal(mustReadFile(filepath.Join(dir, mapProductSubdir, "annotation_provenance.json")), &report); err != nil {
		return err
	}
	found := false
	for _, prov := range report.Provenance {
		if strings.Contains(prov.Catalog, "OpenNGC") {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("layered report lacks deep-sky provenance: %+v", report.Provenance)
	}
	return nil
}

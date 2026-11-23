package main

// PGC/LEDA designation support for dense galaxy fields (roadmap item 156).
//
// A compact offline table provides a few well-identified reference galaxies.
// Any PGC designation arriving from a VO cone or a user catalog is normalised
// and used as a cross-match candidate. Anonymous extended objects are renamed
// to the closest PGC identifier; a Messier/NGC/IC name already outranks PGC,
// so M31 is never replaced by a catalogue number. Matching is spatial and
// indexed, so it stays linear in the number of sources even when a VO cone
// returns many candidate galaxies.

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// offlinePGC holds name|RA(deg)|Dec(deg)|mag|major(')|minor(')|PA(deg)|type.
// Positions, magnitudes and diameters are from NASA/IPAC NED object queries
// performed on 2026-11-23 (see docs/catalog-attribution.md). PGC 3087883 has
// no published diameter in the NED basic-data record, so it keeps the point
// marker rather than an invented ellipse.
const offlinePGC = `PGC 21991|117.824542|-25.134556|16.3|0.630|0.220|0|galaxy
PGC 2167368|11.829012|40.542609|16.0|0.458|0.312|120|galaxy
PGC 2183568|138.937416|41.593928|18.3|0.278|0.222|25|galaxy
PGC 3087883|9.853164|41.598406|17.6|0|0|0|galaxy`

// pgcDesignation normalises a PGC or LEDA identifier. Leading zeroes and a
// missing space are accepted, so "PGC 021991", "LEDA 21991" and "Leda021991"
// all produce the same stable label.
func pgcDesignation(value string) (string, bool) {
	text := strings.ToUpper(strings.TrimSpace(value))
	if text == "" {
		return "", false
	}
	for _, prefix := range []string{"PGC", "LEDA"} {
		if !strings.HasPrefix(text, prefix) {
			continue
		}
		digits := strings.TrimSpace(text[len(prefix):])
		if digits == "" {
			return "", false
		}
		id, err := strconv.ParseUint(digits, 10, 64)
		if err != nil || id == 0 {
			return "", false
		}
		return fmt.Sprintf("PGC %d", id), true
	}
	return "", false
}

// pgcCandidateID is a stable key for de-duplicating catalogue entries.
func pgcCandidateID(name, id string) string {
	if canonical, ok := pgcDesignation(name); ok {
		return canonical
	}
	if id != "" {
		return id
	}
	return strings.TrimSpace(name)
}

// surveyDesignationPrefixes lists identifier styles that are less readable
// than a PGC number. Messier/NGC/IC names keep their higher rank.
var surveyDesignationPrefixes = []string{
	"WISEA ", "WISE ", "2MASS ", "2MASX ", "SDSS ", "GALEX ", "SSTSL2 ",
	"NPM1G ", "ASK ", "NSA ", "LEDA ", "PGC ", "[",
}

// annotationExtendedNameRank orders extended-object labels: named objects
// (3) beat PGC identifiers (2), which beat anonymous/survey identifiers (1).
func annotationExtendedNameRank(s annotationSource) int {
	name := strings.TrimSpace(s.Name)
	if name == "" || strings.EqualFold(name, "source") ||
		strings.EqualFold(name, "catalog") || strings.EqualFold(name, "gaia") ||
		strings.EqualFold(name, "cone") || strings.EqualFold(name, "unknown") {
		return 1
	}
	upper := strings.ToUpper(name)
	for _, prefix := range surveyDesignationPrefixes {
		if strings.HasPrefix(upper, prefix) {
			if _, ok := pgcDesignation(name); ok {
				return 2
			}
			return 1
		}
	}
	return 3
}

// pgcSourceKey returns the canonical key only for a source that already
// carries a readable PGC designation. Such a source must not be renamed and
// must suppress a duplicate anonymous source at the same position.
func pgcSourceKey(s annotationSource) (string, bool) {
	if annotationExtendedNameRank(s) != 2 {
		return "", false
	}
	return pgcDesignation(s.Name)
}

// annotationPGCMatchReport is written to annotation_provenance.json so an
// offline run can be audited without querying an external catalogue.
type annotationPGCMatchReport struct {
	Candidates int      `json:"candidates"`
	Matched    int      `json:"matched"`
	Examples   []string `json:"examples,omitempty"`
}

type pgcMatchCandidate struct {
	key, name, catalog string
	ra, dec            float64
	offline            bool
}

// pgcGridCell is a 0.1 degree spatial bucket. The largest accepted separation
// is 120 arcsec, smaller than one cell, so searching the 3x3 neighbourhood is
// sufficient and avoids an all-pairs scan.
type pgcGridCell struct{ ra, dec int }

const (
	pgcGridCellDeg         = 0.1
	pgcGridRACells         = int(360 / pgcGridCellDeg)
	pgcMaxSeparationArcsec = 120
)

func pgcCellFor(raRad, decRad float64) pgcGridCell {
	ra := math.Mod(raRad*180/math.Pi, 360)
	if ra < 0 {
		ra += 360
	}
	dec := decRad * 180 / math.Pi
	return pgcGridCell{
		ra:  int(math.Floor(ra / pgcGridCellDeg)),
		dec: int(math.Floor(dec / pgcGridCellDeg)),
	}
}

// pgcNeighbourCells returns the cell itself and its eight neighbours, with
// right ascension wrapped at 360 degrees.
func pgcNeighbourCells(cell pgcGridCell) [9]pgcGridCell {
	var out [9]pgcGridCell
	i := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			out[i] = pgcGridCell{
				ra:  ((cell.ra+dx)%pgcGridRACells + pgcGridRACells) % pgcGridRACells,
				dec: cell.dec + dy,
			}
			i++
		}
	}
	return out
}

// pgcMatchLimit derives the positional tolerance from the object's catalogued
// size. Point sources use the default tolerance; resolved galaxies allow up to
// half their major axis, capped to avoid swallowing a neighbouring galaxy.
func pgcMatchLimit(s annotationSource, tolArcsec float64) float64 {
	limit := tolArcsec
	if axis := math.Max(s.Major, s.Minor); axis > 2*tolArcsec {
		limit = axis / 2
	}
	if limit > pgcMaxSeparationArcsec {
		limit = pgcMaxSeparationArcsec
	}
	return limit
}

// crossMatchPGCNames renames anonymous extended sources to a nearby PGC
// designation. The matcher is deterministic: candidate keys are sorted before
// tie-breaking, each PGC identifier is used at most once, and an existing PGC
// label suppresses a duplicate anonymous source at the same position.
func crossMatchPGCNames(sources, candidatesIn []annotationSource, tolArcsec float64) ([]annotationSource, annotationPGCMatchReport) {
	if tolArcsec <= 0 {
		tolArcsec = 20
	}
	byKey := make(map[string]pgcMatchCandidate)
	for _, s := range candidatesIn {
		name, ok := pgcDesignation(s.Name)
		if !ok || !(s.Extended || isExtendedAnnotationType(s.Type)) {
			continue
		}
		key := pgcCandidateID(s.Name, s.ID)
		if _, exists := byKey[key]; exists {
			continue
		}
		byKey[key] = pgcMatchCandidate{
			key: key, name: name, catalog: s.Provenance.Catalog,
			ra: s.RA, dec: s.Dec, offline: s.Provenance.Offline,
		}
	}
	keys := make([]string, 0, len(byKey))
	grid := make(map[pgcGridCell][]string)
	for key, c := range byKey {
		keys = append(keys, key)
		cell := pgcCellFor(c.ra, c.dec)
		grid[cell] = append(grid[cell], key)
	}
	sort.Strings(keys)
	for _, bucket := range grid {
		sort.Strings(bucket)
	}

	// Source keys that already carry a PGC label are reserved: they keep their
	// name and any anonymous source within a few arcsec is a duplicate.
	reserved := make(map[string]bool)
	for _, s := range sources {
		if key, ok := pgcSourceKey(s); ok {
			if _, known := byKey[key]; known {
				reserved[key] = true
			}
		}
	}

	report := annotationPGCMatchReport{Candidates: len(keys)}
	matched := make(map[string]bool)
	out := make([]annotationSource, 0, len(sources))
	for _, s := range sources {
		if s.Type == "gaia" || !(s.Extended || isExtendedAnnotationType(s.Type)) {
			out = append(out, s)
			continue
		}
		if _, alreadyNamed := pgcSourceKey(s); alreadyNamed {
			out = append(out, s)
			continue
		}
		limit := pgcMatchLimit(s, tolArcsec)
		var found []string
		seen := make(map[string]bool)
		for _, cell := range pgcNeighbourCells(pgcCellFor(s.RA, s.Dec)) {
			for _, key := range grid[cell] {
				if !seen[key] {
					seen[key] = true
					found = append(found, key)
				}
			}
		}
		sort.Strings(found)

		// A nearby reserved label is a duplicate, not a new cross-match.
		duplicate := false
		for _, key := range found {
			if !reserved[key] {
				continue
			}
			c := byKey[key]
			sep := angularSeparation(s.RA, s.Dec, c.ra, c.dec) * (180 / math.Pi) * 3600
			if sep <= math.Min(limit, 5) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		best := -1
		bestSep := math.Inf(1)
		for i, key := range found {
			if matched[key] {
				continue
			}
			c := byKey[key]
			sep := angularSeparation(s.RA, s.Dec, c.ra, c.dec) * (180 / math.Pi) * 3600
			if sep > limit {
				continue
			}
			if sep < bestSep || (sep == bestSep && (best < 0 || key < found[best])) {
				best, bestSep = i, sep
			}
		}
		if best < 0 {
			out = append(out, s)
			continue
		}
		key := found[best]
		c := byKey[key]
		if s.Metadata == nil {
			s.Metadata = make(map[string]string)
		}
		catalog := c.catalog
		if catalog == "" {
			catalog = "PGC"
		}
		s.Metadata["pgc_id"] = strings.TrimPrefix(c.name, "PGC ")
		s.Metadata["crossmatch_catalog"] = catalog
		s.Metadata["crossmatch_sep_arcsec"] = strconv.FormatFloat(bestSep, 'f', 2, 64)
		s.Name = c.name
		s.Priority = 80
		s.Provenance = annotationProvenance{
			Catalog: "PGC", Service: "positional-crossmatch", Offline: c.offline,
			URL: c.key,
		}
		matched[key] = true
		report.Matched++
		if len(report.Examples) < 12 {
			report.Examples = append(report.Examples, c.name)
		}
		out = append(out, s)
	}
	return out, report
}

// offlinePGCSources parses the compact reference table with the same geometry
// parser as the rest of the offline bundle.
func offlinePGCSources(prov annotationProvenance) []annotationSource {
	return parseDeepSkyTable(offlinePGC, prov, 80)
}

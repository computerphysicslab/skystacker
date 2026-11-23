package main

import (
	"math"
	"strconv"
	"strings"
)

// namedStarAliases records the few IAU stars commonly labelled with two
// proper names in vendor sky charts. The alias is appended to the primary name
// so a cross-matched Gaia source gets the same readable label.
var namedStarAliases = map[string]string{
	"Alpheratz": "Sirrah",
}

// parseNamedStarTable converts the compact offline bright-star designation
// table into the unified annotation model. Greek Bayer letters are already
// transliterated in the table so the raster bitmap font can render every label.
func parseNamedStarTable(table string, provenance annotationProvenance) []annotationSource {
	out := make([]annotationSource, 0, 2048)
	seenNames := make(map[string]bool, 2048)
	for _, line := range strings.Split(strings.TrimSpace(table), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) != 10 {
			continue
		}
		ra, e1 := strconv.ParseFloat(fields[4], 64)
		dec, e2 := strconv.ParseFloat(fields[5], 64)
		mag, e3 := strconv.ParseFloat(fields[6], 64)
		rank, e4 := strconv.Atoi(fields[9])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			continue
		}
		name := fields[0]
		if alias := namedStarAliases[name]; alias != "" {
			name += " / " + alias
		}
		// A few proper names are shared by unrelated Hipparcos entries in the
		// upstream data. Keep labels unique and stable by qualifying repeats.
		if seenNames[name] {
			name += " HIP " + fields[7]
		}
		seenNames[name] = true
		metadata := map[string]string{"name_rank": strconv.Itoa(rank)}
		if fields[1] != "" {
			metadata["bayer"] = fields[1]
		}
		if fields[2] != "" {
			metadata["flamsteed"] = fields[2]
		}
		if fields[3] != "" {
			metadata["constellation"] = fields[3]
		}
		if fields[7] != "" {
			metadata["hip"] = fields[7]
		}
		if fields[8] != "" {
			metadata["hd"] = fields[8]
		}
		// d3-celestial stores RA in [-180, 180]; the unified model uses [0, 360).
		ra = math.Mod(ra+360, 360)
		out = append(out, annotationSource{
			ID:         "hip-" + fields[7],
			Name:       name,
			Type:       "star",
			RA:         ra * math.Pi / 180,
			Dec:        dec * math.Pi / 180,
			Mag:        mag,
			Priority:   75 + rank,
			Provenance: provenance,
			Metadata:   metadata,
		})
	}
	return out
}

// annotationNameRank returns the label quality of a name candidate:
// 3 = proper name, 2 = Bayer, 1 = Flamsteed, 0 = variable or unknown.
func annotationNameRank(s annotationSource) int {
	if s.Metadata == nil {
		return 0
	}
	rank, err := strconv.Atoi(s.Metadata["name_rank"])
	if err != nil || rank < 0 || rank > 3 {
		return 0
	}
	return rank
}

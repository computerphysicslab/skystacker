package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// testSAOCatalogue checks the generated HEASARC SAO table for completeness,
// ordering and valid cross-identifiers. A truncated or corrupted embed is a
// release blocker because downstream names would silently disappear.
func testSAOCatalogue() error {
	rows := parseSAOTable()
	if len(rows) != 46136 {
		return fmt.Errorf("SAO rows: got %d, want 46136", len(rows))
	}
	seen := make(map[int]bool, len(rows))
	prev := 0
	for i, e := range rows {
		if e.Number <= prev && i > 0 {
			return fmt.Errorf("SAO row %d not ordered: %d after %d", i, e.Number, prev)
		}
		prev = e.Number
		if seen[e.Number] {
			return fmt.Errorf("duplicate SAO number %d", e.Number)
		}
		seen[e.Number] = true
		if e.RA < 0 || e.RA >= 360 || e.Dec < -90 || e.Dec > 90 || e.Mag > 8.0 {
			return fmt.Errorf("SAO %d outside valid range: %.5f %.5f %.1f", e.Number, e.RA, e.Dec, e.Mag)
		}
	}
	if rows[0].Number != 1 || rows[len(rows)-1].Number != 258996 {
		return fmt.Errorf("SAO bounds: %d..%d", rows[0].Number, rows[len(rows)-1].Number)
	}
	// A bright northern star with all cross-identifiers present.
	found := false
	for _, e := range rows {
		if e.HD == "110994" {
			found = e.GC == "17838" && strings.Contains(e.DM, "89")
			break
		}
	}
	if !found {
		return fmt.Errorf("SAO 17838 cross-identifiers missing")
	}
	cands := saoNameCandidates()
	if len(cands) != len(rows) || cands[0].Name != "SAO 1" {
		return fmt.Errorf("SAO candidates: %d, first %q", len(cands), cands[0].Name)
	}
	return nil
}

// testSAOCrossmatch verifies that an SAO designation labels an otherwise
// anonymous Gaia source, that a proper name still wins at comparable distance,
// and that the compact declination index does not miss a near-pole match.
func testSAOCrossmatch() error {
	target := saoEntry{}
	for _, e := range parseSAOTable() {
		if e.Number == 1 {
			target = e
			break
		}
	}
	ra, dec := target.RA*math.Pi/180, target.Dec*math.Pi/180
	gaia := annotationSource{
		ID: "gaia-test", Name: "Gaia", Type: "gaia", RA: ra, Dec: dec,
		Epoch: 2000, Priority: 40,
	}
	sao := saoNameCandidates()[:1]
	out, report := crossMatchGaiaNames([]annotationSource{gaia}, sao, 10)
	if report.Matched != 1 || out[0].Name != "SAO 1" {
		return fmt.Errorf("SAO match: %+v name=%q", report, out[0].Name)
	}
	if out[0].Metadata["crossmatch_id"] != "sao-1" || countSAOMatches(out) != 1 {
		return fmt.Errorf("SAO metadata missing: %+v", out[0].Metadata)
	}
	// A proper name closer to the Gaia position must beat the SAO identifier:
	// GA offset 4" from SAO, named candidate at the GA position itself.
	shifted := gaia
	shifted.RA = ra + 4*math.Pi/(180*3600)
	named := annotationSource{
		ID: "named-1", Name: "Alpheratz", Type: "star",
		RA: shifted.RA, Dec: dec, Priority: 75,
	}
	out, _ = crossMatchGaiaNames([]annotationSource{shifted}, append([]annotationSource{named}, sao...), 10)
	if out[0].Name != "Alpheratz" || countSAOMatches(out) != 0 {
		return fmt.Errorf("proper name lost to SAO: %q", out[0].Name)
	}
	// Near the pole RA cells are useless; the declination-band index must
	// still find a star a few arcseconds away.
	poleRA := 123.0 * math.Pi / 180
	poleDec := (89.99 * math.Pi) / 180
	poleGaia := annotationSource{ID: "pole", Name: "Gaia", Type: "gaia", RA: poleRA, Dec: poleDec, Epoch: 2000}
	poleCand := annotationSource{ID: "pole-sao", Name: "SAO 999", Type: "star",
		RA: poleRA + 3*math.Pi/180, Dec: poleDec, Provenance: annotationProvenance{Catalog: "SAO"}}
	out, report = crossMatchGaiaNames([]annotationSource{poleGaia}, []annotationSource{poleCand}, 10)
	if report.Matched != 1 {
		return fmt.Errorf("near-pole SAO match failed: %+v", report)
	}
	if got := strconv.Itoa(report.Matched); got != "1" {
		return fmt.Errorf("unexpected report count %s", got)
	}
	return nil
}

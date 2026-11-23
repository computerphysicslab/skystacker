package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

// saoEntry is one row of the compact SAO table generated from the HEASARC
// dump. Positions are J2000 degrees; cross-identifiers use "-" when absent.
type saoEntry struct {
	Number   int
	RA       float64
	Dec      float64
	Mag      float64
	Spectral string
	DM       string
	HD       string
	GC       string
}

var (
	saoOnce sync.Once
	saoData []saoEntry
)

// parseSAOTable parses the embedded SAO table once. A bad row is skipped
// rather than aborting the whole annotation run; the self-test checks the
// expected row count so silent truncation is detected.
func parseSAOTable() []saoEntry {
	saoOnce.Do(func() {
		saoData = make([]saoEntry, 0, 46136)
		for _, line := range strings.Split(strings.TrimSpace(offlineSAOStars), "\n") {
			f := strings.Split(strings.TrimSpace(line), "|")
			if len(f) != 8 {
				continue
			}
			num, e0 := strconv.Atoi(f[0])
			ra, e1 := strconv.ParseFloat(f[1], 64)
			dec, e2 := strconv.ParseFloat(f[2], 64)
			mag, e3 := strconv.ParseFloat(f[3], 64)
			if e0 != nil || e1 != nil || e2 != nil || e3 != nil {
				continue
			}
			if num <= 0 || ra < 0 || ra >= 360 || dec < -90 || dec > 90 || mag < -2 || mag > 20 {
				continue
			}
			saoData = append(saoData, saoEntry{
				Number: num, RA: ra, Dec: dec, Mag: mag,
				Spectral: f[4], DM: f[5], HD: f[6], GC: f[7],
			})
		}
	})
	return saoData
}

// saoNameCandidates converts the SAO table into unified annotation candidates.
// The SAO number is the label; HD/DM/GC identifiers stay in metadata so a
// single star never receives two competing labels. SAO carries no proper name,
// so annotationNameRank returns 0 and a Bayer/Flamsteed/proper name always wins
// a positional tie.
func saoNameCandidates() []annotationSource {
	rows := parseSAOTable()
	out := make([]annotationSource, 0, len(rows))
	prov := annotationProvenance{
		Catalog: "SAO (HEASARC I/131A, V<=8.0)",
		Service: "offline-bundle",
		URL:     "https://heasarc.gsfc.nasa.gov/W3Browse/star-catalog/sao.html",
		Offline: true,
	}
	for _, e := range rows {
		meta := map[string]string{
			"sao_number": strconv.Itoa(e.Number),
			"vmag":       strconv.FormatFloat(e.Mag, 'f', 1, 64),
			"spectral":   e.Spectral,
		}
		if e.DM != "-" && e.DM != "" {
			meta["dm"] = e.DM
		}
		if e.HD != "-" && e.HD != "" {
			meta["hd"] = e.HD
		}
		if e.GC != "-" && e.GC != "" {
			meta["gc"] = e.GC
		}
		out = append(out, annotationSource{
			ID:         fmt.Sprintf("sao-%d", e.Number),
			Name:       fmt.Sprintf("SAO %d", e.Number),
			Type:       "star",
			RA:         e.RA * math.Pi / 180,
			Dec:        e.Dec * math.Pi / 180,
			Band:       "V",
			Mag:        e.Mag,
			Priority:   20,
			Provenance: prov,
			Metadata:   meta,
		})
	}
	return out
}

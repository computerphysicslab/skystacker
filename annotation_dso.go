package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

// extendedDSOEntry is one row of the generated deep-sky catalogue. All
// positions are J2000 degrees and all axes are arcseconds, matching the
// unified annotationSource geometry conventions.
type extendedDSOEntry struct {
	Name     string
	RA       float64
	Dec      float64
	Mag      float64
	Major    float64
	Minor    float64
	PA       float64
	Type     string
	Priority int
	Aliases  []string
}

var (
	extendedDSOOnce  sync.Once
	extendedDSORows  []extendedDSOEntry
	extendedDSOBands map[int][]int
)

const extendedDSOBandDeg = 0.25

// parseExtendedDSOTable parses the generated table once. A malformed row is
// skipped instead of aborting an annotation run; the selftest locks the row
// count so silent truncation is detected.
func parseExtendedDSOTable() []extendedDSOEntry {
	extendedDSOOnce.Do(func() {
		rows := make([]extendedDSOEntry, 0, 20000)
		for _, line := range strings.Split(strings.TrimSpace(offlineExtendedDeepSky), "\n") {
			f := strings.Split(strings.TrimSpace(line), "|")
			if len(f) != 10 {
				continue
			}
			ra, e1 := strconv.ParseFloat(f[1], 64)
			dec, e2 := strconv.ParseFloat(f[2], 64)
			mag, e3 := strconv.ParseFloat(f[3], 64)
			major, e4 := strconv.ParseFloat(f[4], 64)
			minor, e5 := strconv.ParseFloat(f[5], 64)
			pa, e6 := strconv.ParseFloat(f[6], 64)
			priority, e7 := strconv.Atoi(f[8])
			if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil || e7 != nil {
				continue
			}
			if f[0] == "" || !(0 <= ra && ra < 360) || !(dec >= -90 && dec <= 90) ||
				priority < 1 || priority > 100 {
				continue
			}
			aliases := []string{}
			if strings.TrimSpace(f[9]) != "" {
				for _, alias := range strings.Split(f[9], ",") {
					if alias = strings.TrimSpace(alias); alias != "" {
						aliases = append(aliases, alias)
					}
				}
			}
			rows = append(rows, extendedDSOEntry{
				Name: f[0], RA: ra, Dec: dec, Mag: mag, Major: major, Minor: minor,
				PA: pa, Type: f[7], Priority: priority, Aliases: aliases,
			})
		}
		extendedDSORows = rows
		extendedDSOBands = make(map[int][]int, 1024)
		for i, row := range rows {
			band := extendedDSOBand(row.Dec)
			extendedDSOBands[band] = append(extendedDSOBands[band], i)
		}
	})
	return extendedDSORows
}

// extendedDSOBand maps a declination in degrees to the shared index band.
func extendedDSOBand(decDeg float64) int {
	return int(math.Floor((decDeg + 90) / extendedDSOBandDeg))
}

// extendedDSOForField returns the catalogue rows inside a cone. The
// declination-band index keeps the scan proportional to the field size
// instead of walking all 19k rows for every annotation render.
func extendedDSOForField(raDeg, decDeg, radiusArcsec float64) []extendedDSOEntry {
	rows := parseExtendedDSOTable()
	if len(rows) == 0 || radiusArcsec <= 0 {
		return nil
	}
	radiusDeg := radiusArcsec / 3600
	if radiusDeg > 180 {
		radiusDeg = 180
	}
	bandSpan := int(math.Ceil(radiusDeg/extendedDSOBandDeg)) + 1
	centre := extendedDSOBand(decDeg)
	out := make([]extendedDSOEntry, 0, 64)
	for b := centre - bandSpan; b <= centre+bandSpan; b++ {
		for _, idx := range extendedDSOBands[b] {
			row := rows[idx]
			dra := (row.RA - raDeg) * math.Cos(decDeg*math.Pi/180)
			if dra > 180 {
				dra -= 360
			}
			if dra < -180 {
				dra += 360
			}
			sep := math.Hypot(dra, row.Dec-decDeg)
			if sep <= radiusDeg {
				out = append(out, row)
			}
		}
	}
	return out
}

// annotationIdentity normalises a designation for duplicate suppression so
// "M31", "M 31" and "m-31" are recognised as the same catalogue object.
func annotationIdentity(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(name)
	return name
}

// extendedDSOSources converts the field rows into unified annotation sources.
// Metadata preserves the merged cross-identifiers without creating competing
// labels, and the priority comes from the generator (Messier > NGC/IC > ...).
func extendedDSOSources(raDeg, decDeg, radiusArcsec float64, prov annotationProvenance, known map[string]bool) []annotationSource {
	rows := extendedDSOForField(raDeg, decDeg, radiusArcsec)
	out := make([]annotationSource, 0, len(rows))
	for _, row := range rows {
		identity := annotationIdentity(row.Name)
		if known[identity] {
			continue
		}
		aliases := make([]string, 0, len(row.Aliases)+1)
		aliases = append(aliases, row.Name)
		aliases = append(aliases, row.Aliases...)
		meta := map[string]string{
			"catalogue": "OpenNGC/HEASARC deep-sky",
			"aliases":   strings.Join(aliases, ","),
		}
		out = append(out, annotationSource{
			ID:         "deepsky-" + identity,
			Name:       row.Name,
			Type:       row.Type,
			RA:         row.RA * math.Pi / 180,
			Dec:        row.Dec * math.Pi / 180,
			Band:       "V",
			Mag:        row.Mag,
			Major:      row.Major,
			Minor:      row.Minor,
			PA:         row.PA,
			Priority:   row.Priority,
			Provenance: prov,
			Metadata:   meta,
			Extended:   isExtendedAnnotationType(row.Type),
		})
		known[identity] = true
	}
	return out
}

// extendedDSOProvenance describes the offline catalogue in the report.
func extendedDSOProvenance() annotationProvenance {
	return annotationProvenance{
		Catalog: "OpenNGC + HEASARC deep-sky catalogues",
		Service: "offline-bundle",
		URL:     "https://github.com/mattiaverga/OpenNGC",
		Offline: true,
	}
}

// extendedDSOCatalogueSummary reports the row count for logs and tests.
func extendedDSOCatalogueSummary() string {
	return fmt.Sprintf("%d", len(parseExtendedDSOTable()))
}

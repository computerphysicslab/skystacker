package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

// Binary native index format (SBI1).
//
// Layout (all integers little-endian, strings length-prefixed with uint16):
//
//	header        magic[4], version u16, nside u16, bin f64, brightest u32,
//	              bandCount u32, tileCount u32, quadCount u64
//	band table    per band: id, maxStars u32, minArcsec f64, maxArcsec f64
//	star table    starCount u64, then ra f64, dec f64, mag f32, pmra f32, pmdec f32
//	tile table    per tile: ipix i32, starCount u32, starIDs u32[]
//	quad table    per key: key, hitCount u32, then ipix i32, bandIdx u16,
//	              longest f64, starIDs [4]u32
//
// Stars are stored once and referenced by index from quad hits, which keeps a
// 300k-star index far smaller than the equivalent JSON representation. The
// reader rebuilds the in-memory nativeBlindIndex used by the solver, so both
// formats are interchangeable at the call site.

var nativeBinaryMagic = [4]byte{'S', 'B', 'I', '1'}

const nativeBinaryVersion = 1

// SBI1 parser bounds. They are generous enough for the published Gaia quad
// indexes but keep a hostile header from driving unbounded allocations. The
// star cap was already enforced in the reader; the other limits are the new
// item-121 gate applied before any band/tile/quad loop.
const (
	maxNativeIndexBands     = uint64(1) << 10
	maxNativeIndexTiles     = uint64(1) << 20
	maxNativeIndexQuads     = uint64(1) << 24
	maxNativeIndexStars     = uint64(1) << 24
	maxNativeIndexTileStars = uint64(1) << 20
	maxNativeIndexQuadHits  = uint64(1) << 20
)

// nativeIndexHeader is the fixed SBI1 header. Keeping one definition for the
// writer and reader prevents a silent layout drift.
type nativeIndexHeader struct {
	Magic     [4]byte
	Version   uint16
	Nside     uint16
	Bin       float64
	Brightest uint32
	Bands     uint32
	Tiles     uint32
	Quads     uint64
}

// validateNativeIndexCounts rejects implausible table sizes with int64-safe
// values, before callers create slices or range over attacker-controlled
// uint32/uint64 counts.
func validateNativeIndexCounts(bands, tiles, quads, stars uint64) error {
	switch {
	case bands > maxNativeIndexBands:
		return fmt.Errorf("native index: %d bands exceeds limit %d", bands, maxNativeIndexBands)
	case tiles > maxNativeIndexTiles:
		return fmt.Errorf("native index: %d tiles exceeds limit %d", tiles, maxNativeIndexTiles)
	case quads > maxNativeIndexQuads:
		return fmt.Errorf("native index: %d quads exceeds limit %d", quads, maxNativeIndexQuads)
	case stars > maxNativeIndexStars:
		return fmt.Errorf("native index: %d stars exceeds limit %d", stars, maxNativeIndexStars)
	}
	return nil
}

// validateNativeIndexSectionCount bounds inner loops and their allocations.
func validateNativeIndexSectionCount(name string, count uint32, limit uint64) error {
	if uint64(count) > limit {
		return fmt.Errorf("native index: %s count %d exceeds limit %d", name, count, limit)
	}
	return nil
}

func writeString(w io.Writer, s string) error {
	if len(s) > 65535 {
		return fmt.Errorf("index string too long: %d", len(s))
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

func readString(r io.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func writeNativeIndexBinary(w io.Writer, idx *nativeBlindIndex) error {
	tileIDs := make([]int, 0, len(idx.Tiles))
	for ipix := range idx.Tiles {
		tileIDs = append(tileIDs, ipix)
	}
	sort.Ints(tileIDs)
	starIDs := make([]int, 0)
	starOffset := make(map[int]uint32, len(idx.Tiles))
	for _, ipix := range tileIDs {
		starOffset[ipix] = uint32(len(starIDs))
		for range idx.Tiles[ipix] {
			starIDs = append(starIDs, 0)
		}
	}
	// Flatten unique catalogue stars in tile order; quads reference these IDs.
	flat := make([]gaiaStar, 0, len(starIDs))
	flatIndex := make(map[int]uint32)
	for _, ipix := range tileIDs {
		for si, s := range idx.Tiles[ipix] {
			key := int(starOffset[ipix]) + si
			flatIndex[key] = uint32(len(flat))
			flat = append(flat, s)
		}
	}
	keys := make([]string, 0, len(idx.Quads))
	for k := range idx.Quads {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if err := validateNativeIndexCounts(uint64(len(idx.Bands)), uint64(len(tileIDs)),
		uint64(len(keys)), uint64(len(flat))); err != nil {
		return err
	}
	header := nativeIndexHeader{nativeBinaryMagic, nativeBinaryVersion, uint16(idx.Nside), idx.Bin,
		uint32(idx.Brightest), uint32(len(idx.Bands)), uint32(len(tileIDs)), uint64(len(keys))}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}
	for _, b := range idx.Bands {
		if err := writeString(w, b.ID); err != nil {
			return err
		}
		rec := struct {
			MaxStars  uint32
			MinArcsec float64
			MaxArcsec float64
		}{uint32(b.MaxStars), b.MinArcsec, b.MaxArcsec}
		if err := binary.Write(w, binary.LittleEndian, rec); err != nil {
			return err
		}
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(flat))); err != nil {
		return err
	}
	for _, s := range flat {
		rec := struct {
			RA, Dec     float64
			Mag         float32
			PMRA, PMDec float32
		}{s.RA, s.Dec, float32(s.MagG), float32(s.PMRA), float32(s.PMDec)}
		if err := binary.Write(w, binary.LittleEndian, rec); err != nil {
			return err
		}
	}
	for _, ipix := range tileIDs {
		if err := binary.Write(w, binary.LittleEndian, int32(ipix)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.Tiles[ipix]))); err != nil {
			return err
		}
		for si := range idx.Tiles[ipix] {
			id := flatIndex[int(starOffset[ipix])+si]
			if err := binary.Write(w, binary.LittleEndian, id); err != nil {
				return err
			}
		}
	}
	bandID := make(map[string]uint16, len(idx.Bands))
	for i, b := range idx.Bands {
		bandID[b.ID] = uint16(i)
	}
	// Resolve quad star coordinates to shared IDs in O(1). A linear scan per
	// hit would make index writing quadratic in the star/quad count.
	type coordKey struct{ ra, dec uint64 }
	idByCoord := make(map[coordKey]uint32, len(flat))
	for i, s := range flat {
		idByCoord[coordKey{math.Float64bits(s.RA), math.Float64bits(s.Dec)}] = uint32(i)
	}
	for _, key := range keys {
		if err := writeString(w, key); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.Quads[key]))); err != nil {
			return err
		}
		for _, hit := range idx.Quads[key] {
			bi, ok := bandID[hit.Band]
			if !ok {
				bi = 0
			}
			if err := binary.Write(w, binary.LittleEndian, int32(hit.Ipix)); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, bi); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, hit.Longest); err != nil {
				return err
			}
			for _, sc := range hit.Stars {
				key := coordKey{math.Float64bits(sc[0]), math.Float64bits(sc[1])}
				id, found := idByCoord[key]
				if !found {
					id = uint32(0xffffffff)
				}
				if err := binary.Write(w, binary.LittleEndian, id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func readNativeIndexBinary(r io.Reader) (*nativeBlindIndex, error) {
	var header nativeIndexHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	if header.Magic != nativeBinaryMagic || header.Version != nativeBinaryVersion {
		return nil, fmt.Errorf("unsupported native index header %q v%d", header.Magic, header.Version)
	}
	if err := validateNativeIndexCounts(uint64(header.Bands), uint64(header.Tiles),
		header.Quads, 0); err != nil {
		return nil, err
	}
	idx := &nativeBlindIndex{Version: nativeIndexVersion, Nside: int(header.Nside),
		Bin: header.Bin, Brightest: int(header.Brightest),
		Tiles: make(map[int][]gaiaStar), Quads: make(map[string][]nativeIndexEntry)}
	for range header.Bands {
		id, err := readString(r)
		if err != nil {
			return nil, err
		}
		rec := struct {
			MaxStars  uint32
			MinArcsec float64
			MaxArcsec float64
		}{}
		if err := binary.Read(r, binary.LittleEndian, &rec); err != nil {
			return nil, err
		}
		idx.Bands = append(idx.Bands, nativeBandInfo{id, int(rec.MaxStars), rec.MinArcsec, rec.MaxArcsec})
	}
	var starCount uint64
	if err := binary.Read(r, binary.LittleEndian, &starCount); err != nil {
		return nil, err
	}
	if err := validateNativeIndexCounts(0, 0, 0, starCount); err != nil {
		return nil, err
	}
	stars := make([]gaiaStar, starCount)
	for i := range stars {
		rec := struct {
			RA, Dec     float64
			Mag         float32
			PMRA, PMDec float32
		}{}
		if err := binary.Read(r, binary.LittleEndian, &rec); err != nil {
			return nil, err
		}
		stars[i] = gaiaStar{RA: rec.RA, Dec: rec.Dec, MagG: float64(rec.Mag),
			PMRA: float64(rec.PMRA), PMDec: float64(rec.PMDec)}
	}
	lookupStar := func(id uint32) (gaiaStar, bool) {
		if id == uint32(0xffffffff) || int(id) >= len(stars) {
			return gaiaStar{}, false
		}
		return stars[id], true
	}
	for range header.Tiles {
		var ipix int32
		var count uint32
		if err := binary.Read(r, binary.LittleEndian, &ipix); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
			return nil, err
		}
		if err := validateNativeIndexSectionCount("tile stars", count, maxNativeIndexTileStars); err != nil {
			return nil, err
		}
		list := make([]gaiaStar, 0, count)
		for range count {
			var id uint32
			if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
				return nil, err
			}
			if s, ok := lookupStar(id); ok {
				list = append(list, s)
			}
		}
		idx.Tiles[int(ipix)] = list
	}
	for range header.Quads {
		key, err := readString(r)
		if err != nil {
			return nil, err
		}
		var count uint32
		if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
			return nil, err
		}
		if err := validateNativeIndexSectionCount("quad hits", count, maxNativeIndexQuadHits); err != nil {
			return nil, err
		}
		hits := make([]nativeIndexEntry, 0, count)
		for range count {
			var ipix int32
			var band uint16
			var longest float64
			if err := binary.Read(r, binary.LittleEndian, &ipix); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &band); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &longest); err != nil {
				return nil, err
			}
			hit := nativeIndexEntry{Ipix: int(ipix), Longest: longest}
			if int(band) < len(idx.Bands) {
				hit.Band = idx.Bands[band].ID
			}
			for i := range hit.Stars {
				var id uint32
				if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
					return nil, err
				}
				if s, ok := lookupStar(id); ok {
					hit.Stars[i] = [2]float64{s.RA, s.Dec}
				}
			}
			hits = append(hits, hit)
		}
		idx.Quads[key] = hits
	}
	return idx, nil
}

func readNativeIndexBinaryFile(path string) (*nativeBlindIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readNativeIndexBinary(f)
}

func isNativeBinaryIndex(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	return magic == nativeBinaryMagic
}

// selectDensityUniformStars caps stars per sub-cell so that sparse high-latitude
// fields and dense Galactic tiles contribute comparable numbers of reference
// stars. The quota follows the surface-density limit used by ASTAP D50/D80.
func selectDensityUniformStars(stars []gaiaStar, nside int, maxPerSqDeg float64) []gaiaStar {
	if nside <= 0 || maxPerSqDeg <= 0 || len(stars) < 8 {
		return stars
	}
	centerRA, centerDec := 0.0, 0.0
	for _, s := range stars {
		centerRA += s.RA
		centerDec += s.Dec
	}
	centerRA /= float64(len(stars))
	centerDec /= float64(len(stars))
	tileRad := math.Sqrt(4 * math.Pi / float64(12*nside*nside))
	cellRad := tileRad / 4
	cellDeg := cellRad * 180 / math.Pi
	quota := int(math.Ceil(maxPerSqDeg * cellDeg * cellDeg))
	if quota < 2 {
		quota = 2
	}
	groups := make(map[[2]int][]gaiaStar)
	for _, s := range stars {
		dRA := normalizedAngle(s.RA - centerRA)
		key := [2]int{int(dRA / cellRad), int((s.Dec - centerDec) / cellRad)}
		groups[key] = append(groups[key], s)
	}
	out := make([]gaiaStar, 0, len(stars))
	for _, group := range groups {
		sort.SliceStable(group, func(i, j int) bool { return group[i].MagG < group[j].MagG })
		if len(group) > quota {
			group = group[:quota]
		}
		out = append(out, group...)
	}
	return out
}

func normalizedAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

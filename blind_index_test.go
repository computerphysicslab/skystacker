package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryIndexRoundTrip(t *testing.T) {
	idx := &nativeBlindIndex{
		Version: nativeIndexVersion, Nside: 4, Bin: nativeQuadBin, Brightest: 2,
		Bands: []nativeBandInfo{{"wide", 25, 5, 60}, {"medium", 50, 0.8, 6}},
		Tiles: map[int][]gaiaStar{
			7: {{RA: 1.0, Dec: 0.2, MagG: 8, PMRA: 3, PMDec: -2}, {RA: 1.01, Dec: 0.21, MagG: 9}},
			9: {{RA: 2.0, Dec: 0.3, MagG: 7}},
		},
		Quads: map[string][]nativeIndexEntry{
			"wide|1:2:3:4": {{Ipix: 7, Band: "wide", Longest: 0.01,
				Stars: [4][2]float64{{1.0, 0.2}, {1.01, 0.21}, {1.0, 0.2}, {2.0, 0.3}}}},
			"medium|5:6:7:8": {{Ipix: 9, Band: "medium", Longest: 0.02,
				Stars: [4][2]float64{{2.0, 0.3}, {2.0, 0.3}, {2.0, 0.3}, {2.0, 0.3}}}},
		},
	}
	var buf bytes.Buffer
	if err := writeNativeIndexBinary(&buf, idx); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readNativeIndexBinary(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Nside != idx.Nside || len(got.Tiles) != len(idx.Tiles) || len(got.Quads) != len(idx.Quads) {
		t.Fatalf("metadata mismatch: %+v", got)
	}
	if len(got.Bands) != 2 || got.Bands[1].ID != "medium" {
		t.Fatalf("bands mismatch: %+v", got.Bands)
	}
	for ipix, stars := range idx.Tiles {
		if len(got.Tiles[ipix]) != len(stars) {
			t.Fatalf("tile %d star count: %d vs %d", ipix, len(got.Tiles[ipix]), len(stars))
		}
		for i := range stars {
			if math.Abs(got.Tiles[ipix][i].RA-stars[i].RA) > 1e-12 {
				t.Fatalf("tile %d star %d RA mismatch", ipix, i)
			}
			if math.Abs(got.Tiles[ipix][i].PMRA-stars[i].PMRA) > 1e-5 {
				t.Fatalf("tile %d star %d PM mismatch", ipix, i)
			}
		}
	}
	for key, hits := range idx.Quads {
		gotHits := got.Quads[key]
		if len(gotHits) != len(hits) {
			t.Fatalf("quad %s hit count: %d vs %d", key, len(gotHits), len(hits))
		}
		for i := range hits {
			if gotHits[i].Band != hits[i].Band || gotHits[i].Ipix != hits[i].Ipix ||
				math.Abs(gotHits[i].Longest-hits[i].Longest) > 1e-12 {
				t.Fatalf("quad %s hit %d mismatch: %+v vs %+v", key, i, gotHits[i], hits[i])
			}
			for j := range hits[i].Stars {
				if math.Abs(gotHits[i].Stars[j][0]-hits[i].Stars[j][0]) > 1e-12 ||
					math.Abs(gotHits[i].Stars[j][1]-hits[i].Stars[j][1]) > 1e-12 {
					t.Fatalf("quad %s hit %d star %d RA/Dec mismatch: %+v vs %+v",
						key, i, j, gotHits[i].Stars[j], hits[i].Stars[j])
				}
			}
		}
	}
}

func TestDensityUniformSelection(t *testing.T) {
	stars := make([]gaiaStar, 400)
	for i := range stars {
		stars[i] = gaiaStar{RA: 1.0 + float64(i%20)*0.0005, Dec: 0.2 + float64(i/20)*0.0005, MagG: 8 + float64(i)*0.001}
	}
	got := selectDensityUniformStars(stars, 64, 100)
	if len(got) == 0 || len(got) >= len(stars) {
		t.Fatalf("density cap did not select a subset: %d of %d", len(got), len(stars))
	}
	for _, s := range got {
		if s.RA < 1.0 || s.RA > 1.01 || s.Dec < 0.2 || s.Dec > 0.22 {
			t.Fatalf("selected star outside the tile: %+v", s)
		}
	}
}

func TestBayesianLogOdds(t *testing.T) {
	real := bayesianLogOdds(20, 100, 100, 2.4e-5, 1.0)
	chance := bayesianLogOdds(2, 100, 100, 2.4e-5, 1.0)
	if real <= chance || chance > 0 {
		t.Fatalf("log-odds: real %.1f chance %.1f", real, chance)
	}
	if real < 10 {
		t.Fatalf("true alignment should exceed threshold: %.1f", real)
	}
}

func TestDetrendedBlindStars(t *testing.T) {
	const w, h = 160, 120
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	truth := []point2D{{30, 30}, {70, 50}, {120, 40}, {50, 90}, {130, 100}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 0.08 + 0.0015*float64(x) + 0.001*float64(y)
			for _, s := range truth {
				d2 := (float64(x)-s.X)*(float64(x)-s.X) + (float64(y)-s.Y)*(float64(y)-s.Y)
				v += 0.6 * math.Exp(-d2/8)
			}
			i := (y*w + x) * 3
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = float32(v), float32(v), float32(v)
		}
	}
	detected := detrendedBlindStars(f, 50)
	if len(detected) < 3 {
		t.Fatalf("detrended extraction found %d stars", len(detected))
	}
	for _, s := range truth {
		best := 1e9
		for _, d := range detected {
			if dist := math.Hypot(d.X-s.X, d.Y-s.Y); dist < best {
				best = dist
			}
		}
		if best > 3 {
			t.Fatalf("star at (%g,%g) not recovered (best %.1f px)", s.X, s.Y, best)
		}
	}
}

func TestScaleConsistentMatches(t *testing.T) {
	frameStars := []star{{X: 10, Y: 10, HFR: 2}, {X: 20, Y: 20, HFR: 2}, {X: 30, Y: 30, HFR: 2}, {X: 40, Y: 40, HFR: 2}}
	catalog := []gaiaStar{
		{RA: 1.000, Dec: 0.500}, {RA: 1.001, Dec: 0.501},
		{RA: 1.002, Dec: 0.502}, {RA: 1.050, Dec: 0.550},
	}
	matches := []matchPair{{0, 0}, {1, 1}, {2, 2}, {3, 3}}
	pruned := scaleConsistentMatches(matches, frameStars, catalog, 0.2)
	if len(pruned) != 3 {
		t.Fatalf("expected 3 consistent matches, got %d: %+v", len(pruned), pruned)
	}
}

func TestScaleConsistentMatchesFalseAnchorFirst(t *testing.T) {
	frameStars := []star{
		{X: 10, Y: 10, HFR: 2},
		{X: 20, Y: 20, HFR: 2},
		{X: 30, Y: 30, HFR: 2},
		{X: 40, Y: 40, HFR: 2},
	}
	catalog := []gaiaStar{
		{RA: 1.050, Dec: 0.550}, // false correspondence at index 0
		{RA: 1.001, Dec: 0.501},
		{RA: 1.002, Dec: 0.502},
		{RA: 1.003, Dec: 0.503},
	}
	matches := []matchPair{{0, 0}, {1, 1}, {2, 2}, {3, 3}}
	pruned := scaleConsistentMatches(matches, frameStars, catalog, 0.2)
	if len(pruned) != 3 {
		t.Fatalf("false first anchor: got %d matches %+v, want 3", len(pruned), pruned)
	}
	for _, m := range pruned {
		if m.FrameIdx == 0 || m.GaiaIdx == 0 {
			t.Fatalf("false correspondence survived: %+v", m)
		}
	}
}

func TestExponentialDescriptorKey(t *testing.T) {
	a := nativeExpKeyFromFine("b20|10:2:8:3")
	b := nativeExpKeyFromFine("b20|10:2:8:3")
	if a == "" || a != b {
		t.Fatalf("unstable exponential key: %q vs %q", a, b)
	}
	if nativeExpKeyFromFine("b20|bad") != "" {
		t.Fatal("malformed fine key should be rejected")
	}
}

func TestSubDegreeIndexWithProperMotion(t *testing.T) {
	dir := t.TempDir()
	ra, dec := 1.0, 0.3
	nside := 32
	ipix := healpixAng2PixNested(nside, ra, dec)
	tile := gaiaTile{Version: gaiaCacheVersion, Nside: nside, Ipix: ipix, Stars: []gaiaStar{
		{RA: ra, Dec: dec, MagG: 9, PMRA: 120, PMDec: -45},
		{RA: ra + 0.0002, Dec: dec + 0.0001, MagG: 10, PMRA: 80, PMDec: 20},
		{RA: ra + 0.0004, Dec: dec, MagG: 11},
		{RA: ra, Dec: dec + 0.0003, MagG: 12},
	}}
	data, err := json.Marshal(tile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("healpix_n%d_p%d.json", nside, ipix))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := buildNativeIndexFromCache(dir, nside, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Tiles) != 1 {
		t.Fatalf("expected one cached tile, got %d", len(idx.Tiles))
	}
	out := filepath.Join(dir, "subdegree.sbi")
	if err := runBuildBlindIndex(out, dir, nside, 100); err != nil {
		t.Fatal(err)
	}
	got, err := readNativeIndexBinaryFile(out)
	if err != nil {
		t.Fatal(err)
	}
	stars := got.Tiles[ipix]
	if len(stars) < 4 {
		t.Fatalf("expected 4 stars in tile, got %d", len(stars))
	}
	if math.Abs(stars[0].PMRA-120) > 1e-6 || math.Abs(stars[0].PMDec+45) > 1e-6 {
		t.Fatalf("proper motion not preserved: %+v", stars[0])
	}
}

func TestValidateNativeIndexCounts(t *testing.T) {
	cases := []struct {
		name                       string
		bands, tiles, quads, stars uint64
		wantErr                    bool
	}{
		{"all at limit", maxNativeIndexBands, maxNativeIndexTiles, maxNativeIndexQuads, maxNativeIndexStars, false},
		{"bands above limit", maxNativeIndexBands + 1, 0, 0, 0, true},
		{"tiles above limit", 0, maxNativeIndexTiles + 1, 0, 0, true},
		{"quads above limit", 0, 0, maxNativeIndexQuads + 1, 0, true},
		{"stars above limit", 0, 0, 0, maxNativeIndexStars + 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateNativeIndexCounts(c.bands, c.tiles, c.quads, c.stars)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateNativeIndexCounts(%d,%d,%d,%d) = %v, wantErr=%v",
					c.bands, c.tiles, c.quads, c.stars, err, c.wantErr)
			}
		})
	}
	if err := validateNativeIndexSectionCount("tile stars", uint32(maxNativeIndexTileStars), maxNativeIndexTileStars); err != nil {
		t.Fatalf("tile star count at limit rejected: %v", err)
	}
	if err := validateNativeIndexSectionCount("tile stars", uint32(maxNativeIndexTileStars+1), maxNativeIndexTileStars); err == nil {
		t.Fatal("tile star count above limit accepted")
	}
}

func sbiHeaderBytes(bands, tiles uint32, quads uint64) []byte {
	var b bytes.Buffer
	h := nativeIndexHeader{Magic: nativeBinaryMagic, Version: nativeBinaryVersion,
		Nside: 8, Bin: nativeQuadBin, Brightest: 4, Bands: bands, Tiles: tiles, Quads: quads}
	if err := binary.Write(&b, binary.LittleEndian, h); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func TestReadNativeIndexBinaryRejectsHostileCounts(t *testing.T) {
	cases := []struct {
		name         string
		bands, tiles uint32
		quads        uint64
		tail         func() []byte
	}{
		{"bands", uint32(maxNativeIndexBands + 1), 0, 0, func() []byte { return nil }},
		{"tiles", 0, uint32(maxNativeIndexTiles + 1), 0, func() []byte { return nil }},
		{"quads", 0, 0, maxNativeIndexQuads + 1, func() []byte { return nil }},
		{"stars", 0, 0, 0, func() []byte {
			var b bytes.Buffer
			_ = binary.Write(&b, binary.LittleEndian, maxNativeIndexStars+1)
			return b.Bytes()
		}},
		{"tile stars", 0, 1, 0, func() []byte {
			var b bytes.Buffer
			_ = binary.Write(&b, binary.LittleEndian, uint64(0)) // star table
			_ = binary.Write(&b, binary.LittleEndian, int32(7))  // ipix
			_ = binary.Write(&b, binary.LittleEndian, uint32(maxNativeIndexTileStars+1))
			return b.Bytes()
		}},
		{"quad hits", 0, 0, 1, func() []byte {
			var b bytes.Buffer
			_ = binary.Write(&b, binary.LittleEndian, uint64(0)) // star table
			_ = writeString(&b, "wide|1:2:3:4")
			_ = binary.Write(&b, binary.LittleEndian, uint32(maxNativeIndexQuadHits+1))
			return b.Bytes()
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := append(sbiHeaderBytes(c.bands, c.tiles, c.quads), c.tail()...)
			if _, err := readNativeIndexBinary(bytes.NewReader(data)); err == nil {
				t.Fatalf("hostile %s count accepted", c.name)
			}
		})
	}
}

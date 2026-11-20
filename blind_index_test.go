package main

import (
	"bytes"
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
		if len(got.Quads[key]) != len(hits) || got.Quads[key][0].Band != hits[0].Band {
			t.Fatalf("quad %s mismatch: %+v", key, got.Quads[key])
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

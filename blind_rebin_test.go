package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRebinGaiaCacheOffline verifies the offline fine-nside rebin: build a
// small coarse nside=4 cache with known stars, rebin to nside=16, confirm every
// star lands in the correct fine tile and a native index can be built from it.
func TestRebinGaiaCacheOffline(t *testing.T) {
	dir := t.TempDir()
	// two coarse tiles with a handful of stars
	clusterA := []gaiaStar{}
	for i := 0; i < 30; i++ {
		clusterA = append(clusterA, gaiaStar{RA: 0.10 + 0.0005*float64(i), Dec: 0.20 + 0.0003*float64(i), MagG: 7 + 0.02*float64(i)})
	}
	clusterB := []gaiaStar{}
	for i := 0; i < 30; i++ {
		clusterB = append(clusterB, gaiaStar{RA: 2.0 + 0.0005*float64(i), Dec: 0.5 + 0.0003*float64(i), MagG: 7 + 0.02*float64(i)})
	}
	coarse := map[int][]gaiaStar{10: clusterA, 20: clusterB}
	for ipix, stars := range coarse {
		tile := gaiaTile{Version: gaiaCacheVersion, Nside: 4, Ipix: ipix, Stars: stars}
		if err := gaiaSaveTileCache(dir, tile); err != nil {
			t.Fatal(err)
		}
	}
	written, nstars, err := rebinGaiaCache(dir, 4, 16)
	if err != nil {
		t.Fatal(err)
	}
	if nstars != 60 || written == 0 {
		t.Fatalf("rebin: %d stars -> %d tiles, want 60 stars", nstars, written)
	}
	// all fine tiles must be nside=16 and every star must be in its own pixel
	pattern := filepath.Join(dir, "healpix_n16_p*.json")
	files, _ := filepath.Glob(pattern)
	gotStars := 0
	for _, f := range files {
		data, _ := os.ReadFile(f)
		tile, err := unmarshalGaiaTile(data)
		if err != nil || tile.Nside != 16 {
			t.Fatalf("bad fine tile %s: %v", f, err)
		}
		for _, s := range tile.Stars {
			ipix := healpixAng2PixNested(16, s.RA, s.Dec)
			if ipix != tile.Ipix {
				t.Fatalf("star in tile %d actually belongs to %d", tile.Ipix, ipix)
			}
			gotStars++
		}
	}
	if gotStars != 60 {
		t.Fatalf("fine tiles contain %d stars, want 60", gotStars)
	}
	// build a native index from the rebinned cache; it must have tiles+quads
	idx, err := buildNativeIndexFromCache(dir, 16, 100)
	if err != nil {
		t.Fatalf("build index from rebinned cache: %v", err)
	}
	if len(idx.Tiles) == 0 || len(idx.Quads) == 0 {
		t.Fatalf("empty index: %d tiles, %d quads", len(idx.Tiles), len(idx.Quads))
	}
}

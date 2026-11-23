package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// rebinGaiaCache expands coarse cached HEALPix tiles into a finer nside using
// only local data: every star carries RA/Dec, so it can be reassigned to its
// fine-nside pixel with healpixAng2PixNested. This avoids refetching Gaia TAP
// (which is often unavailable) and closes the "full-sky generation at fine
// nside" R&D gate when a coarse all-sky cache already exists.
//
// It returns the number of fine tiles written and the number of stars copied.
func rebinGaiaCache(cacheDir string, srcNside, dstNside int) (int, int, error) {
	if srcNside < 1 || dstNside < 1 || dstNside <= srcNside {
		return 0, 0, fmt.Errorf("rebin: need dst nside > src nside (%d -> %d)", srcNside, dstNside)
	}
	// power-of-two check
	for n := dstNside; n > 1; n >>= 1 {
		if n%2 != 0 {
			return 0, 0, fmt.Errorf("rebin: nside %d not a power of two", dstNside)
		}
	}
	pattern := filepath.Join(cacheDir, fmt.Sprintf("healpix_n%d_p*.json", srcNside))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return 0, 0, err
	}
	bins := make(map[int][]gaiaStar)
	totalStars := 0
	for _, path := range files {
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		t, uerr := unmarshalGaiaTile(data)
		if uerr != nil || t.Nside != srcNside {
			continue
		}
		for _, s := range t.Stars {
			ipix := healpixAng2PixNested(dstNside, s.RA, s.Dec)
			bins[ipix] = append(bins[ipix], s)
			totalStars++
		}
	}
	if len(bins) == 0 {
		return 0, 0, fmt.Errorf("rebin: no nside=%d tiles in %s", srcNside, cacheDir)
	}
	written := 0
	ipixList := make([]int, 0, len(bins))
	for ipix := range bins {
		ipixList = append(ipixList, ipix)
	}
	sort.Ints(ipixList)
	for _, ipix := range ipixList {
		stars := bins[ipix]
		sort.SliceStable(stars, func(i, j int) bool { return stars[i].MagG < stars[j].MagG })
		t := gaiaTile{
			Version: gaiaCacheVersion, Nside: dstNside, Ipix: ipix,
			QueriedAt: "rebin", Mirror: "local", MagLimit: 10, Stars: stars,
		}
		if err := gaiaSaveTileCache(cacheDir, t); err != nil {
			return written, totalStars, err
		}
		written++
	}
	return written, totalStars, nil
}

// unmarshalGaiaTile decodes a cached tile from raw bytes.
func unmarshalGaiaTile(data []byte) (gaiaTile, error) {
	var t gaiaTile
	err := json.Unmarshal(data, &t)
	return t, err
}

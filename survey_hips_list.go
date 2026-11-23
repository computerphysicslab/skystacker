package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// popularHiPS is a curated, always-available list of commonly used HiPS
// survey IDs. It is used when the CDS registry cannot be reached so that
// `--list-survey-hips` remains useful offline.
var popularHiPS = []string{
	"CDS/P/DSS2/color",
	"CDS/P/DSS2/red",
	"CDS/P/DSS2/blue",
	"CDS/P/DSS1/red",
	"CDS/P/DSS1/blue",
	"CDS/P/2MASS/color",
	"CDS/P/2MASS/J",
	"CDS/P/2MASS/H",
	"CDS/P/2MASS/K",
	"CDS/P/SDSS9/color",
	"CDS/P/PanSTARRS/DR1/color",
	"CDS/P/PanSTARRS/DR1/r",
	"CDS/P/AllWISE/color",
	"CDS/P/AllWISE/W1",
	"CDS/P/AllWISE/W2",
	"CDS/P/Planck/HFI/857",
	"CDS/P/IRAS/100",
	"CDS/P/Fermi/color",
	"ESO/P/2MASS/color",
	"CDS/P/GALEXGR6/AIS/color",
}

// hipsRegistryCachePath returns the on-disk cache for the CDS HiPS list.
func hipsRegistryCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "skystacker", "hipslist.txt")
}

// fetchHiPSList returns the list of HiPS IDs from the CDS registry, using a
// cached copy when available and refreshing it when online. On total failure
// it returns the built-in popular list.
func fetchHiPSList() []string {
	cache := hipsRegistryCachePath()
	// try network refresh
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://alasky.cds.unistra.fr/hipslist")
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var ids []string
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// registry lines are "id title..." or just "id"
			fields := strings.Fields(line)
			if len(fields) > 0 {
				ids = append(ids, fields[0])
			}
		}
		if len(ids) > 0 {
			// cache
			_ = os.MkdirAll(filepath.Dir(cache), 0o755)
			_ = os.WriteFile(cache, []byte(strings.Join(ids, "\n")), 0o644)
			return ids
		}
	}
	// try cache
	if data, err := os.ReadFile(cache); err == nil {
		var ids []string
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				ids = append(ids, line)
			}
		}
		if len(ids) > 0 {
			return ids
		}
	}
	return append([]string(nil), popularHiPS...)
}

// runListSurveyHiPS implements --list-survey-hips [KEYWORD].
func runListSurveyHiPS(keyword string) error {
	ids := fetchHiPSList()
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "all" {
		kw = ""
	}
	var filtered []string
	for _, id := range ids {
		if kw == "" || strings.Contains(strings.ToLower(id), kw) {
			filtered = append(filtered, id)
		}
	}
	sort.Strings(filtered)
	fmt.Printf("Available HiPS surveys (%d):\n", len(filtered))
	for _, id := range filtered {
		fmt.Println("  " + id)
	}
	if len(filtered) == 0 {
		fmt.Println("  (none match; try a broader keyword or run without a filter)")
	}
	return nil
}

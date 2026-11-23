package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// runSurveyServiceHealth probes the HiPS and DSS endpoints and prints
// latency/availability. It never fetches a cutout.
func runSurveyServiceHealth() error {
	type probe struct {
		name string
		url  string
	}
	probes := []probe{
		{"hips (CDS registry)", "https://alasky.cds.unistra.fr/hipslist"},
		{"dss (STScI/MAST)", "https://archive.stsci.edu/cgi-bin/dss_search"},
		{"eso (ESO DSS)", "https://archive.eso.org/dss/dss"},
	}
	fmt.Println("Survey service health:")
	for _, p := range probes {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
		if err != nil {
			cancel()
			fmt.Printf("  %-22s ERROR %v\n", p.name, err)
			continue
		}
		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			fmt.Printf("  %-22s DOWN   (%v)  %s\n", p.name, err, elapsed.Round(time.Millisecond))
			continue
		}
		resp.Body.Close()
		status := "OK"
		if resp.StatusCode >= 400 {
			status = "HTTP " + resp.Status
		}
		fmt.Printf("  %-22s %-8s %s  %s\n", p.name, status, resp.Status, elapsed.Round(time.Millisecond))
	}
	return nil
}

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type archiveAdapter struct {
	URL         string
	MaxRows     int
	TokenHeader bool
}

var archiveAdapters = map[string]archiveAdapter{
	"mast":      {URL: "https://mast.stsci.edu/api/v0.1/", MaxRows: 1000, TokenHeader: true},
	"esasky":    {URL: "https://skyserver.esac.esa.int/skyview/", MaxRows: 10000},
	"irsa-most": {URL: "https://irsa.ipac.caltech.edu/MOST/", MaxRows: 20000},
}

type archiveAdapterResult struct {
	Adapter   string `json:"adapter"`
	Rows      int    `json:"rows"`
	Sources   int    `json:"sources"`
	Truncated bool   `json:"truncated,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Error     string `json:"error,omitempty"`
}

type archiveAdaptersReport struct {
	Results []archiveAdapterResult `json:"results"`
}

func archiveToken() string {
	if v := strings.TrimSpace(os.Getenv("MAST_TOKEN")); v != "" {
		return v
	}
	return ""
}

func queryArchiveAdapter(name, baseURL, token string, raDeg, decDeg, radiusArcsec float64) (archiveAdapterResult, error) {
	adapter, ok := archiveAdapters[name]
	if !ok {
		return archiveAdapterResult{}, fmt.Errorf("unknown archive adapter %q", name)
	}
	if baseURL == "" {
		baseURL = adapter.URL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return archiveAdapterResult{}, err
	}
	q := u.Query()
	q.Set("RA", strconv.FormatFloat(raDeg, 'g', -1, 64))
	q.Set("DEC", strconv.FormatFloat(decDeg, 'g', -1, 64))
	q.Set("RADIUS", strconv.FormatFloat(radiusArcsec, 'g', -1, 64))
	q.Set("MAXREC", strconv.Itoa(adapter.MaxRows))
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return archiveAdapterResult{}, err
	}
	if adapter.TokenHeader && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return archiveAdapterResult{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return archiveAdapterResult{}, err
	}
	if resp.StatusCode >= 400 {
		return archiveAdapterResult{}, fmt.Errorf("archive %s: HTTP %d", name, resp.StatusCode)
	}
	var table coneVOTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return archiveAdapterResult{}, err
	}
	result := archiveAdapterResult{Adapter: name, Rows: len(table.Rows), Sources: len(table.Rows)}
	for _, info := range append(table.Infos, table.ResourceInfos...) {
		v := strings.ToUpper(info.Value)
		n := strings.ToUpper(info.Name)
		if strings.Contains(n, "TRUNC") || strings.Contains(n, "OVERFLOW") || strings.Contains(v, "TRUNCATED") || strings.Contains(v, "OVERFLOW") {
			result.Truncated = true
			result.Reason = "service_reported_truncation"
		}
	}
	if result.Rows >= adapter.MaxRows {
		result.Truncated, result.Reason = true, "service_row_limit"
	}
	return result, nil
}

func computeArchiveAdaptersReport(opts *runOptions, sol *plateSolution, baseURLs map[string]string) *archiveAdaptersReport {
	if opts == nil || strings.TrimSpace(opts.ArchiveAdapters) == "" {
		return nil
	}
	report := &archiveAdaptersReport{}
	if sol == nil {
		for _, name := range strings.Split(opts.ArchiveAdapters, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				report.Results = append(report.Results, archiveAdapterResult{Adapter: name, Error: "no plate solution"})
			}
		}
		return report
	}
	token := opts.MASTToken
	if token == "" {
		token = archiveToken()
	}
	ra, dec := sol.WCS.CRVAL1*180/3.141592653589793, sol.WCS.CRVAL2*180/3.141592653589793
	for _, name := range strings.Split(opts.ArchiveAdapters, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		result, err := queryArchiveAdapter(name, baseURLs[name], token, ra, dec, opts.ConeRadiusArcsec)
		if err != nil {
			result = archiveAdapterResult{Adapter: name, Error: err.Error()}
		}
		report.Results = append(report.Results, result)
	}
	return report
}

func formatArchiveAdaptersReport(r *archiveAdaptersReport) string {
	if r == nil {
		return ""
	}
	parts := make([]string, 0, len(r.Results))
	for _, result := range r.Results {
		part := fmt.Sprintf("%s rows=%d truncated=%v", result.Adapter, result.Rows, result.Truncated)
		if result.Error != "" {
			part += " error=" + result.Error
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

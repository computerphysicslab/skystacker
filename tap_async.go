package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	tapAsyncPollInterval = 50 * time.Millisecond
	tapAsyncTimeout      = 10 * time.Second
)

type tapAsyncConfig struct {
	Mirror    string
	MaxRec    int
	Verbosity int
	Registry  string
	Client    *http.Client
}

type tapAsyncJob struct {
	JobURL    string `json:"job_url"`
	Status    string `json:"status"`
	Rows      int    `json:"rows"`
	Truncated bool   `json:"truncated"`
	Reason    string `json:"reason,omitempty"`
	Polls     int    `json:"polls"`
}

type tapAsyncReport struct {
	Mode           string `json:"mode"`
	JobURL         string `json:"job_url,omitempty"`
	Status         string `json:"status,omitempty"`
	Rows           int    `json:"rows,omitempty"`
	Truncated      bool   `json:"truncated,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Polls          int    `json:"polls,omitempty"`
	FallbackUsed   bool   `json:"fallback_used,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

func tapHTTPClient(cfg tapAsyncConfig) *http.Client {
	base := cfg.Client
	if base == nil {
		base = &http.Client{}
	}
	client := *base
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func tapEndpoint(mirror, suffix string) string {
	mirror = strings.TrimRight(mirror, "/")
	if strings.HasSuffix(mirror, "/sync") || strings.HasSuffix(mirror, "/async") {
		mirror = mirror[:strings.LastIndex(mirror, "/")]
	}
	return mirror + "/" + suffix
}

func tapStatusFromVOTable(data []byte) (string, bool, string) {
	var table coneVOTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return "", false, ""
	}
	status, reason := "", ""
	check := func(name, value string) {
		n, v := strings.ToUpper(name), strings.ToUpper(value)
		if strings.Contains(n, "TRUNC") || strings.Contains(n, "OVERFLOW") || strings.Contains(v, "TRUNCATED") || strings.Contains(v, "OVERFLOW") {
			reason = "service_reported_truncation"
		}
		if strings.Contains(n, "QUERY_STATUS") || strings.Contains(n, "STATUS") {
			status = v
		}
	}
	for _, i := range table.Infos {
		check(i.Name, i.Value)
	}
	for _, i := range table.ResourceInfos {
		check(i.Name, i.Value)
	}
	return status, reason != "", reason
}

func tapSubmitForm(ctx context.Context, cfg tapAsyncConfig, endpoint, adql string) (*http.Response, error) {
	form := url.Values{}
	form.Set("REQUEST", "doQuery")
	form.Set("LANG", "ADQL")
	form.Set("FORMAT", "votable")
	form.Set("QUERY", adql)
	if cfg.MaxRec > 0 {
		form.Set("MAXREC", strconv.Itoa(cfg.MaxRec))
	}
	if cfg.Verbosity > 0 {
		form.Set("VERBOSITY", strconv.Itoa(cfg.Verbosity))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "SkyStacker/0.6.1")
	return tapHTTPClient(cfg).Do(req)
}

func submitTAPAsyncJob(ctx context.Context, cfg tapAsyncConfig, adql string) (*tapAsyncJob, error) {
	resp, err := tapSubmitForm(ctx, cfg, tapEndpoint(cfg.Mirror, "async"), adql)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("TAP async submit: HTTP %d: %s", resp.StatusCode, gaiaExtractTAPError(body))
	}
	job := &tapAsyncJob{}
	if loc := resp.Header.Get("Location"); loc != "" {
		job.JobURL = loc
	} else {
		job.Status, job.Truncated, job.Reason = tapStatusFromVOTable(body)
		job.Rows = tapCountRows(body)
		if job.Status == "" && job.Rows > 0 {
			job.Status = "COMPLETED"
		}
		if job.Status == "ERROR" || job.Status == "ABORTED" {
			return job, fmt.Errorf("TAP async job %s", job.Status)
		}
		return job, nil
	}
	for {
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		default:
		}
		job.Polls++
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.JobURL, nil)
		if err != nil {
			return job, err
		}
		poll, err := tapHTTPClient(cfg).Do(req)
		if err != nil {
			return job, err
		}
		data, readErr := io.ReadAll(io.LimitReader(poll.Body, 8<<20))
		poll.Body.Close()
		if readErr != nil {
			return job, readErr
		}
		if poll.StatusCode >= 400 {
			return job, fmt.Errorf("TAP async poll: HTTP %d", poll.StatusCode)
		}
		status, truncated, reason := tapStatusFromVOTable(data)
		job.Status, job.Truncated, job.Reason = strings.ToUpper(status), truncated, reason
		job.Rows = tapCountRows(data)
		if job.Status == "COMPLETED" || job.Status == "ERROR" || job.Status == "ABORTED" {
			if job.Status != "COMPLETED" {
				return job, fmt.Errorf("TAP async job %s", job.Status)
			}
			return job, nil
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-time.After(tapAsyncPollInterval):
		}
	}
}

func tapCountRows(data []byte) int {
	var table coneVOTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return 0
	}
	return len(table.Rows)
}

func tapDiscoverRegistry(ctx context.Context, cfg tapAsyncConfig) (string, error) {
	if cfg.Registry == "" {
		return "", fmt.Errorf("no VO registry configured")
	}
	adql := "SELECT TOP 1 access_url FROM rr.capability WHERE standardID LIKE 'ivoa.net/std/TAP%'"
	resp, err := tapSubmitForm(ctx, cfg, tapEndpoint(cfg.Registry, "sync"), adql)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("registry query: HTTP %d", resp.StatusCode)
	}
	var table coneVOTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return "", err
	}
	if len(table.Rows) == 0 || len(table.Rows[0].Cells) == 0 {
		return "", fmt.Errorf("registry returned no TAP service")
	}
	return strings.TrimSpace(table.Rows[0].Cells[0]), nil
}

func tapSyncFallback(ctx context.Context, cfg tapAsyncConfig, adql string) (int, error) {
	resp, err := tapSubmitForm(ctx, cfg, tapEndpoint(cfg.Mirror, "sync"), adql)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("TAP sync fallback: HTTP %d", resp.StatusCode)
	}
	return tapCountRows(data), nil
}

func computeTAPAsyncReport(opts *runOptions, sol *plateSolution) *tapAsyncReport {
	if opts == nil || !opts.GaiaAsync || sol == nil {
		return nil
	}
	cfg := tapAsyncConfig{Mirror: opts.GaiaMirror, MaxRec: opts.GaiaMaxRec, Verbosity: opts.GaiaVerbosity, Registry: opts.VORegistry}
	if cfg.MaxRec <= 0 {
		cfg.MaxRec = 2000
	}
	if cfg.Mirror == "" {
		cfg.Mirror = gaiaDefaultMirror
	}
	if cfg.Registry != "" {
		if discovered, err := tapDiscoverRegistry(context.Background(), cfg); err == nil && discovered != "" {
			cfg.Mirror = discovered
		}
	}
	nside := opts.GaiaNside
	if nside <= 0 {
		nside = 64
	}
	ipix := healpixAng2PixNested(nside, sol.WCS.CRVAL1, sol.WCS.CRVAL2)
	adql := gaiaBuildADQL(nside, ipix, opts.GaiaMagLimit)
	ctx, cancel := context.WithTimeout(context.Background(), tapAsyncTimeout)
	defer cancel()
	job, err := submitTAPAsyncJob(ctx, cfg, adql)
	report := &tapAsyncReport{Mode: "async"}
	if err != nil {
		rows, _ := tapSyncFallback(ctx, cfg, adql)
		report.Mode, report.FallbackUsed, report.FallbackReason = "sync", true, err.Error()
		report.Rows = rows
		return report
	}
	report.JobURL, report.Status, report.Rows = job.JobURL, job.Status, job.Rows
	report.Truncated, report.Reason, report.Polls = job.Truncated, job.Reason, job.Polls
	return report
}

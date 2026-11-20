package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func tapVOTable(status string, rows int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><VOTable xmlns="http://www.ivoa.net/xml/VOTable/v1.3">`)
	if status != "" {
		b.WriteString(`<INFO name="QUERY_STATUS" value="` + status + `"/>`)
	}
	b.WriteString(`<RESOURCE><TABLE><FIELD name="ra"/><FIELD name="dec"/><DATA><TABLEDATA>`)
	for i := 0; i < rows; i++ {
		b.WriteString(`<TR><TD>10.0</TD><TD>20.0</TD></TR>`)
	}
	b.WriteString(`</TABLEDATA></DATA></TABLE></RESOURCE></VOTable>`)
	return b.String()
}

func TestTAPAsyncQueuedToCompleted(t *testing.T) {
	old := tapAsyncPollInterval
	tapAsyncPollInterval = time.Millisecond
	defer func() { tapAsyncPollInterval = old }()
	polls := 0
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/async", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", server.URL+"/job/1")
		w.WriteHeader(http.StatusSeeOther)
	})
	mux.HandleFunc("/job/1", func(w http.ResponseWriter, r *http.Request) {
		polls++
		if polls == 1 {
			_, _ = w.Write([]byte(tapVOTable("QUEUED", 0)))
			return
		}
		_, _ = w.Write([]byte(tapVOTable("COMPLETED", 3)))
	})
	cfg := tapAsyncConfig{Mirror: server.URL, MaxRec: 2000}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	job, err := submitTAPAsyncJob(ctx, cfg, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(job.JobURL, "/job/1") || job.Status != "COMPLETED" || job.Rows != 3 || job.Polls < 2 {
		t.Fatalf("unexpected job: %+v", job)
	}
}

func TestTAPAsyncTruncatedAndFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/async") {
			_, _ = w.Write([]byte(tapVOTable("OVERFLOW", 2)))
			return
		}
		_, _ = w.Write([]byte(tapVOTable("COMPLETED", 7)))
	}))
	defer server.Close()
	cfg := tapAsyncConfig{Mirror: server.URL, MaxRec: 5, Verbosity: 2}
	job, err := submitTAPAsyncJob(context.Background(), cfg, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if !job.Truncated || job.Reason != "service_reported_truncation" || job.Rows != 2 {
		t.Fatalf("truncation not detected: %+v", job)
	}
	rows, err := tapSyncFallback(context.Background(), cfg, "SELECT 1")
	if err != nil || rows != 7 {
		t.Fatalf("fallback rows=%d err=%v", rows, err)
	}
}

func TestTAPAsyncTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", r.Host)
		w.WriteHeader(http.StatusSeeOther)
	}))
	defer server.Close()
	cfg := tapAsyncConfig{Mirror: server.URL, Client: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := submitTAPAsyncJob(ctx, cfg, "SELECT 1"); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestTAPAsyncFlagAndJSON(t *testing.T) {
	opts, err := parseArgs([]string{"--gaia-async", "--gaia-maxrec", "5000", "--gaia-verbosity", "2", "--vo-registry", "http://registry"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.GaiaAsync || opts.GaiaMaxRec != 5000 || opts.GaiaVerbosity != 2 || opts.VORegistry != "http://registry" {
		t.Fatalf("flags not parsed: %+v", opts)
	}
	data, err := json.Marshal(&processingReport{TAPAsyncReport: &tapAsyncReport{Mode: "async", Status: "COMPLETED"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tap_async"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

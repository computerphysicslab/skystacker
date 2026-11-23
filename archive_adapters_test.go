package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestArchiveAdapterQuery(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><VOTable xmlns="http://www.ivoa.net/xml/VOTable/v1.3"><INFO name="QUERY_STATUS" value="OVERFLOW"/><RESOURCE><TABLE><FIELD name="ra"/><FIELD name="dec"/><DATA><TABLEDATA><TR><TD>10</TD><TD>20</TD></TR><TR><TD>10.1</TD><TD>20.1</TD></TR></TABLEDATA></DATA></TABLE></RESOURCE></VOTable>`))
	}))
	defer server.Close()
	result, err := queryArchiveAdapter(context.Background(), "mast", server.URL, "secret", 10, 20, 60)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 2 || result.Sources != 2 || !result.Truncated || result.Reason != "service_reported_truncation" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("token header missing: %q", gotAuth)
	}
}

func TestArchiveAdaptersReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<VOTable><RESOURCE><TABLE><DATA><TABLEDATA><TR><TD>1</TD></TR></TABLEDATA></DATA></TABLE></RESOURCE></VOTable>`))
	}))
	defer server.Close()
	opts := &runOptions{ArchiveAdapters: "mast,esasky", MASTToken: "secret", ConeRadiusArcsec: 60}
	sol := &plateSolution{}
	report := computeArchiveAdaptersReport(opts, sol, map[string]string{"mast": server.URL, "esasky": server.URL})
	if len(report.Results) != 2 || report.Results[0].Rows != 1 || report.Results[1].Rows != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestArchiveAdaptersFlagAndJSON(t *testing.T) {
	opts, err := parseArgs([]string{"--archive-adapters", "mast,esasky", "--mast-token", "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ArchiveAdapters != "mast,esasky" || opts.MASTToken != "secret" {
		t.Fatalf("flags not parsed: %+v", opts)
	}
	data, err := json.Marshal(&processingReport{ArchiveAdaptersReport: &archiveAdaptersReport{Results: []archiveAdapterResult{{Adapter: "mast", Rows: 2}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"archive_adapters"`) {
		t.Fatalf("missing json tag: %s", data)
	}
}

func TestArchiveAdapterRedirectDropsAuth(t *testing.T) {
	var firstAuth, redirectAuth string
	var redirectHits int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectAuth = r.Header.Get("Authorization")
		redirectHits++
		_, _ = w.Write([]byte(`<VOTable><RESOURCE><TABLE><DATA><TABLEDATA><TR><TD>1</TD></TR></TABLEDATA></DATA></TABLE></RESOURCE></VOTable>`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstAuth = r.Header.Get("Authorization")
		http.Redirect(w, r, target.URL+"/target", http.StatusFound)
	}))
	defer origin.Close()

	result, err := queryArchiveAdapter(context.Background(), "mast", origin.URL, "secret", 10, 20, 60)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows != 1 || redirectHits != 1 {
		t.Fatalf("redirect not followed: %+v hits=%d", result, redirectHits)
	}
	if firstAuth != "Bearer secret" {
		t.Fatalf("first hop lost its token: %q", firstAuth)
	}
	if redirectAuth != "" {
		t.Fatalf("token leaked to cross-host redirect: %q", redirectAuth)
	}
}

func TestArchiveAdapterCancelledBeforeDial(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`<VOTable/>`))
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queryArchiveAdapter(ctx, "mast", server.URL, "secret", 10, 20, 60); err == nil {
		t.Fatal("cancelled archive query succeeded")
	}
	if hits != 0 {
		t.Fatalf("handler was reached with a cancelled context: %d hits", hits)
	}
}

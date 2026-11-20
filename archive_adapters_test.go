package main

import (
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
	result, err := queryArchiveAdapter("mast", server.URL, "secret", 10, 20, 60)
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

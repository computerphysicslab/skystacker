package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const coneVOTableComplete = `<?xml version="1.0"?>
<VOTable xmlns="http://www.ivoa.net/xml/VOTable/v1.3">
<RESOURCE><TABLE>
<FIELD name="ra"/><FIELD name="dec"/>
<DATA><TABLEDATA>
<TR><TD>10.0</TD><TD>20.0</TD></TR>
<TR><TD>10.001</TD><TD>20.001</TD></TR>
</TABLEDATA></DATA>
</TABLE></RESOURCE></VOTable>`

const coneVOTableTruncated = `<?xml version="1.0"?>
<VOTable xmlns="http://www.ivoa.net/xml/VOTable/v1.3">
<INFO name="QUERY_STATUS" value="OVERFLOW"/>
<RESOURCE><TABLE>
<FIELD name="ra"/><FIELD name="dec"/>
<DATA><TABLEDATA>
<TR><TD>10.0</TD><TD>20.0</TD></TR>
</TABLEDATA></DATA>
</TABLE></RESOURCE></VOTable>`

func coneTestService(baseURL string) coneService {
	return coneService{
		URL:       baseURL,
		MaxArcsec: 180,
		Build: func(ra, dec, radius float64) url.Values {
			return url.Values{}
		},
	}
}

func TestConeTruncationDetection(t *testing.T) {
	cases := []struct {
		name      string
		votable   string
		truncated bool
	}{
		{"complete", coneVOTableComplete, false},
		{"overflow", coneVOTableTruncated, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/x-votable+xml")
				_, _ = w.Write([]byte(tc.votable))
			}))
			defer server.Close()
			result, err := coneSearchDetailed(coneTestService(server.URL), "", 10, 20, 60, 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if result.Truncated != tc.truncated {
				t.Fatalf("truncated=%v want %v", result.Truncated, tc.truncated)
			}
			if tc.truncated && result.LimitReason != "service_reported_truncation" {
				t.Fatalf("reason=%q", result.LimitReason)
			}
			if result.Rows < 1 || len(result.Sources) < 1 {
				t.Fatalf("rows=%d sources=%d", result.Rows, len(result.Sources))
			}
			if result.CoveredRadiusArcsec <= 0 || result.CoveredAreaDeg2 <= 0 {
				t.Fatalf("coverage not computed: %+v", result)
			}
		})
	}
}

func TestConeReportJSONTags(t *testing.T) {
	rep := &processingReport{ConeTruncated: true, ConeRows: 42, ConeCoveredRadiusArcsec: 30, ConeCoveredAreaDeg2: 0.1, ConeLimitReason: "service_row_limit"}
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, key := range []string{"cone_truncated", "cone_rows", "cone_covered_radius_arcsec", "cone_covered_area_deg2", "cone_limit_reason"} {
		if !strings.Contains(text, "\""+key+"\"") {
			t.Fatalf("missing %s in %s", key, text)
		}
	}
}

func TestConeCoverageGuard(t *testing.T) {
	centerRA, centerDec := 10*math.Pi/180, 20*math.Pi/180
	insideRA, insideDec := 10*math.Pi/180, 20.001*math.Pi/180
	outsideRA, outsideDec := 12*math.Pi/180, 20*math.Pi/180
	covered := 30 * math.Pi / (180 * 3600)
	if !coneCoverageAllows(false, centerRA, centerDec, covered, outsideRA, outsideDec) {
		t.Fatal("non-truncated cone must allow all positions")
	}
	if !coneCoverageAllows(true, centerRA, centerDec, covered, insideRA, insideDec) {
		t.Fatal("inside coverage rejected")
	}
	if coneCoverageAllows(true, centerRA, centerDec, covered, outsideRA, outsideDec) {
		t.Fatal("outside coverage accepted")
	}
}

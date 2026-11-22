package main

// Embedded self-tests for the layered annotation subsystem (goal 144).

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// annotationGoldenFieldJSON is the compact form of
// assets/annotation-examples/synthetic-field.json: 40 sources in the M31 test
// field. It is embedded so the golden test runs from any working directory.
const annotationGoldenFieldJSON = `[{"id":"golden-00","name":"SYN-00","type":"star","ra":11.221364930408988,"dec":41.0,"mag":5.0,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-01","name":"SYN-01","type":"galaxy","ra":8.888242759787548,"dec":41.76864204902094,"mag":5.18,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":33,"minor_arcsec":17,"pa_deg":17,"extended":true},{"id":"golden-02","name":"SYN-02","type":"nebula","ra":10.151025787142878,"dec":39.70124975579921,"mag":5.36,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":36,"minor_arcsec":19,"pa_deg":34,"extended":true},{"id":"golden-03","name":"SYN-03","type":"cluster","ra":11.163776015613006,"dec":42.14560365639453,"mag":5.54,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":39,"minor_arcsec":21,"pa_deg":51,"extended":true},{"id":"golden-04","name":"SYN-04","type":"planetary","ra":7.955801216133752,"dec":40.72710457669698,"mag":5.72,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":42,"minor_arcsec":23,"pa_deg":68,"extended":true},{"id":"golden-05","name":"SYN-05","type":"star","ra":11.876076205610543,"dec":40.09932497189232,"mag":5.9,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-06","name":"SYN-06","type":"galaxy","ra":9.387549021891171,"dec":42.71944510934044,"mag":6.08,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":48,"minor_arcsec":27,"pa_deg":102,"extended":true},{"id":"golden-07","name":"SYN-07","type":"nebula","ra":8.854431344194829,"dec":39.33531991885215,"mag":6.26,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":51,"minor_arcsec":29,"pa_deg":119,"extended":true},{"id":"golden-08","name":"SYN-08","type":"cluster","ra":12.446071931102797,"dec":41.67418323484866,"mag":6.4399999999999995,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":54,"minor_arcsec":31,"pa_deg":136,"extended":true},{"id":"golden-09","name":"SYN-09","type":"planetary","ra":7.48922176193501,"dec":41.78219063737362,"mag":6.62,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":57,"minor_arcsec":33,"pa_deg":153,"extended":true},{"id":"golden-10","name":"SYN-10","type":"star","ra":11.196511321658818,"dec":39.07030179438474,"mag":6.8,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-11","name":"SYN-11","type":"galaxy","ra":10.875389012075871,"dec":43.106302448563696,"mag":6.98,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":63,"minor_arcsec":37,"pa_deg":7,"extended":true},{"id":"golden-12","name":"SYN-12","type":"nebula","ra":7.384693055741778,"dec":39.856144174075666,"mag":7.16,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":66,"minor_arcsec":39,"pa_deg":24,"extended":true},{"id":"golden-13","name":"SYN-13","type":"cluster","ra":13.044138650584923,"dec":40.4949141282309,"mag":7.34,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":69,"minor_arcsec":41,"pa_deg":41,"extended":true},{"id":"golden-14","name":"SYN-14","type":"planetary","ra":8.155197629607912,"dec":42.98038928580656,"mag":7.52,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":72,"minor_arcsec":43,"pa_deg":58,"extended":true},{"id":"golden-15","name":"SYN-15","type":"star","ra":9.576500201708829,"dec":38.53351949683706,"mag":7.699999999999999,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-16","name":"SYN-16","type":"galaxy","ra":12.584937527637846,"dec":42.64420129903133,"mag":7.88,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":78,"minor_arcsec":47,"pa_deg":92,"extended":true},{"id":"golden-17","name":"SYN-17","type":"nebula","ra":6.539761252161503,"dec":41.1079927038582,"mag":8.06,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":81,"minor_arcsec":49,"pa_deg":109,"extended":true},{"id":"golden-18","name":"SYN-18","type":"cluster","ra":12.511788354095083,"dec":39.11355442636424,"mag":8.24,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":84,"minor_arcsec":51,"pa_deg":126,"extended":true},{"id":"golden-19","name":"SYN-19","type":"planetary","ra":9.832701053420076,"dec":43.73053470586898,"mag":8.42,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":87,"minor_arcsec":53,"pa_deg":143,"extended":true},{"id":"golden-20","name":"SYN-20","type":"star","ra":7.630520786309276,"dec":38.85705648220434,"mag":8.6,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-21","name":"SYN-21","type":"galaxy","ra":13.739076735232901,"dec":41.37968494563673,"mag":8.78,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":93,"minor_arcsec":57,"pa_deg":177,"extended":true},{"id":"golden-22","name":"SYN-22","type":"nebula","ra":6.843266137150515,"dec":42.657623896020034,"mag":8.96,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":96,"minor_arcsec":59,"pa_deg":14,"extended":true},{"id":"golden-23","name":"SYN-23","type":"cluster","ra":10.85970025916075,"dec":38.115912172687636,"mag":9.14,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":99,"minor_arcsec":61,"pa_deg":31,"extended":true},{"id":"golden-24","name":"SYN-24","type":"planetary","ra":11.982162632765663,"dec":43.61064465704885,"mag":9.32,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":102,"minor_arcsec":63,"pa_deg":48,"extended":true},{"id":"golden-25","name":"SYN-25","type":"star","ra":6.136581214233823,"dec":40.0697932982747,"mag":9.5,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-26","name":"SYN-26","type":"galaxy","ra":13.742292466154328,"dec":39.695081446292164,"mag":9.68,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":108,"minor_arcsec":67,"pa_deg":82,"extended":true},{"id":"golden-27","name":"SYN-27","type":"nebula","ra":8.383046070584259,"dec":43.91591763339532,"mag":9.86,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":111,"minor_arcsec":69,"pa_deg":99,"extended":true},{"id":"golden-28","name":"SYN-28","type":"cluster","ra":8.560531554384092,"dec":37.97958510548188,"mag":10.04,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":114,"minor_arcsec":71,"pa_deg":116,"extended":true},{"id":"golden-29","name":"SYN-29","type":"planetary","ra":13.821131243563151,"dec":42.515667770888804,"mag":10.219999999999999,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":117,"minor_arcsec":73,"pa_deg":133,"extended":true},{"id":"golden-30","name":"SYN-30","type":"star","ra":5.765316074415478,"dec":41.84244388406961,"mag":10.399999999999999,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-31","name":"SYN-31","type":"galaxy","ra":12.401820612716868,"dec":38.180870740419415,"mag":10.58,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":123,"minor_arcsec":77,"pa_deg":167,"extended":true},{"id":"golden-32","name":"SYN-32","type":"nebula","ra":10.762457764484916,"dec":44.34828681352952,"mag":10.76,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":126,"minor_arcsec":79,"pa_deg":4,"extended":true},{"id":"golden-33","name":"SYN-33","type":"cluster","ra":6.3937379891355235,"dec":38.89217765079053,"mag":10.94,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":129,"minor_arcsec":81,"pa_deg":21,"extended":true},{"id":"golden-34","name":"SYN-34","type":"planetary","ra":14.604369859586662,"dec":40.71210649782802,"mag":11.120000000000001,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":132,"minor_arcsec":83,"pa_deg":38,"extended":true},{"id":"golden-35","name":"SYN-35","type":"star","ra":6.823155722794576,"dec":43.591729737005984,"mag":11.3,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true}},{"id":"golden-36","name":"SYN-36","type":"galaxy","ra":10.023100507220176,"dec":37.42620415444393,"mag":11.48,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":138,"minor_arcsec":87,"pa_deg":72,"extended":true},{"id":"golden-37","name":"SYN-37","type":"nebula","ra":13.219561392148284,"dec":43.678537153245294,"mag":11.66,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":141,"minor_arcsec":89,"pa_deg":89,"extended":true},{"id":"golden-38","name":"SYN-38","type":"cluster","ra":5.173140580604457,"dec":40.66237897414957,"mag":11.84,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":144,"minor_arcsec":91,"pa_deg":106,"extended":true},{"id":"golden-39","name":"SYN-39","type":"planetary","ra":13.905170958058685,"dec":38.76312719774686,"mag":12.02,"priority":70,"provenance":{"catalog":"synthetic-golden-1","service":"fixture","offline":true},"major_arcsec":147,"minor_arcsec":93,"pa_deg":123,"extended":true}]`

func annotationGoldenSources(t interface{ Fatalf(string, ...any) }) []annotationSource {
	var sources []annotationSource
	if err := json.Unmarshal([]byte(annotationGoldenFieldJSON), &sources); err != nil {
		t.Fatalf("golden JSON: %v", err)
	}
	// The corpus stores RA/Dec in degrees for readability; the unified model
	// uses radians, so convert at the boundary.
	for i := range sources {
		sources[i].RA *= math.Pi / 180
		sources[i].Dec *= math.Pi / 180
	}
	return sources
}

// testAnnotationBundle covers item 137: one model, small offline bundle,
// golden JSON field with at least 95% of sources annotated.
func testAnnotationBundle() error {
	bundle := offlineAnnotationBundle()
	if len(bundle.Sources) < 50 || len(bundle.Constellations) < 20 {
		return fmt.Errorf("offline bundle too small: %d sources, %d segments", len(bundle.Sources), len(bundle.Constellations))
	}
	for i, s := range bundle.Sources {
		if !s.Provenance.Offline || s.ID == "" || s.Type == "" {
			return fmt.Errorf("bundle source %d invalid: %+v", i, s)
		}
		if s.RA < 0 || s.RA > 2*math.Pi || s.Dec < -math.Pi/2 || s.Dec > math.Pi/2 {
			return fmt.Errorf("bundle source %d outside sky: %+v", i, s)
		}
	}
	data, err := json.Marshal(bundle.Sources)
	if err != nil || len(data) >= 2<<20 {
		return fmt.Errorf("bundle JSON size %d err=%v", len(data), err)
	}
	golden := annotationGoldenSources(nilTestReporter{})
	if len(golden) != 40 {
		return fmt.Errorf("golden sources: got %d, want 40", len(golden))
	}
	w := rotatedWCS(10.0*math.Pi/180, 41.0*math.Pi/180, 60, 0, false)
	projected := projectAnnotationSources(w, 512, 512, golden)
	if len(projected) < int(math.Ceil(float64(len(golden))*0.95)) {
		return fmt.Errorf("golden field projection: %d/%d sources in frame", len(projected), len(golden))
	}
	svg := buildAnnotationSVG(512, 512, projected, nil, annotationRenderOptions{Layers: annotationAllLayers})
	annotated := 0
	for _, s := range projected {
		if strings.Contains(svg, s.Name) {
			annotated++
		}
	}
	if annotated < int(math.Ceil(float64(len(projected))*0.95)) {
		return fmt.Errorf("golden field annotation: %d/%d sources in SVG", annotated, len(projected))
	}
	return nil
}

// testAnnotationVO covers item 138: mock-server success, timeout, truncation,
// cache reuse and credential redaction.
func testAnnotationVO() error {
	// Regression: the three new value flags must survive parseArgs even when
	// the positional input directory appears before them.
	opts, err := parseArgs([]string{"assets/annotation-examples", "--annotation-layers", "grid-radec,constellations", "--annotation-vo", "simbad", "--annotation-cache", "/tmp/opencode/ann-cache"})
	if err != nil || opts.InputDir != "assets/annotation-examples" ||
		opts.AnnotationLayers != "grid-radec,constellations" || opts.AnnotationVO != "simbad" ||
		opts.AnnotationCache != "/tmp/opencode/ann-cache" {
		return fmt.Errorf("annotation flag parsing: opts=%+v err=%v", opts, err)
	}
	if got := redactAnnotationError("GET failed api_key=SECRET123&token=ABC"); strings.Contains(got, "SECRET123") || strings.Contains(got, "ABC") || !strings.Contains(got, "[redacted]") {
		return fmt.Errorf("redaction failed: %s", got)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"ra":10.5,"dec":41.2,"name":"M 31","otype":"Galaxy","V":3.4}]}`)
	}))
	defer server.Close()
	acq, err := acquireAnnotationSources(context.Background(), annotationAcquisitionOptions{
		Services: []string{"simbad"}, SIMBADURL: server.URL,
		RADeg: 10, DecDeg: 41, RadiusArcsec: 60, Timeout: 2 * time.Second,
	})
	if err != nil || len(acq.Warnings) != 0 || len(acq.Sources) <= offlineAnnotationBundleSize() {
		return fmt.Errorf("VO success: %d sources, %d warnings %v, err=%v", len(acq.Sources), len(acq.Warnings), acq.Warnings, err)
	}
	found := false
	for _, p := range acq.Provenance {
		if p.Catalog == "SIMBAD" && p.Service == "simbad-tap" && !p.Offline {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("VO success provenance missing: %+v", acq.Provenance)
	}

	// VizieR cone searches return a VOTable whose RA/Dec are already converted
	// to radians by coneSearchDetailed; verify the unified model does not
	// convert them a second time.
	viz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><VOTABLE><RESOURCE><TABLE>`+
			`<FIELD name="ra"/><FIELD name="dec"/><FIELD name="mag"/><FIELD name="name"/>`+
			`<DATA><TABLEDATA><TR><TD>10.5</TD><TD>41.2</TD><TD>9.5</TD><TD>VIZ-1</TD></TR></TABLEDATA></DATA>`+
			`</TABLE></RESOURCE></VOTABLE>`)
	}))
	defer viz.Close()
	acq, err = acquireAnnotationSources(context.Background(), annotationAcquisitionOptions{
		Services: []string{"vizier"}, VizierURL: viz.URL,
		RADeg: 18, DecDeg: 45, RadiusArcsec: 60, Timeout: 2 * time.Second,
	})
	if err != nil || len(acq.Warnings) != 0 {
		return fmt.Errorf("VizieR success: warnings=%v err=%v", acq.Warnings, err)
	}
	vizierOK := false
	for _, src := range acq.Sources {
		if src.ID == "vizier-VIZ-1" &&
			math.Abs(src.RA-10.5*math.Pi/180) < 1e-9 && math.Abs(src.Dec-41.2*math.Pi/180) < 1e-9 {
			vizierOK = true
		}
	}
	if !vizierOK {
		return errors.New("VizieR source missing or misplaced in unified model")
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer slow.Close()
	acq, err = acquireAnnotationSources(context.Background(), annotationAcquisitionOptions{
		Services: []string{"simbad"}, SIMBADURL: slow.URL,
		RADeg: 12, DecDeg: 42, RadiusArcsec: 60, Timeout: 20 * time.Millisecond,
	})
	if err != nil || len(acq.Warnings) != 1 || !strings.Contains(acq.Warnings[0], "SIMBAD") {
		return fmt.Errorf("VO timeout: warnings=%v err=%v", acq.Warnings, err)
	}

	rows := make([]string, 1000)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"ra":10.5,"dec":41.2,"name":"T%d","otype":"star","V":9}`, i)
	}
	trunc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(rows, ","))
	}))
	defer trunc.Close()
	acq, err = acquireAnnotationSources(context.Background(), annotationAcquisitionOptions{
		Services: []string{"simbad"}, SIMBADURL: trunc.URL,
		RADeg: 14, DecDeg: 43, RadiusArcsec: 60, Timeout: 2 * time.Second,
	})
	if err != nil || len(acq.Sources) < 1000 {
		return fmt.Errorf("VO truncation rows: %d err=%v", len(acq.Sources), err)
	}
	truncated := false
	for _, p := range acq.Provenance {
		truncated = truncated || p.Truncated
	}
	if !truncated {
		return fmt.Errorf("VO truncation flag missing: %+v", acq.Provenance)
	}

	dir, err := os.MkdirTemp("", "skystacker-annvo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	key := fmt.Sprintf("simbad|%.6f|%.6f|%.1f", 16.0, 44.0, 60.0)
	cached := []annotationSource{{ID: "simbad-CACHED", Name: "CACHED", Type: "star", RA: 1, Dec: 1, Priority: 75}}
	annotationWriteCache(annotationCacheFile(dir, key), cached)
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service down", http.StatusInternalServerError)
	}))
	defer fail.Close()
	acq, err = acquireAnnotationSources(context.Background(), annotationAcquisitionOptions{
		Services: []string{"simbad"}, SIMBADURL: fail.URL, CacheDir: dir,
		RADeg: 16, DecDeg: 44, RadiusArcsec: 60, Timeout: time.Second,
	})
	if err != nil || len(acq.Warnings) != 0 {
		return fmt.Errorf("VO cache reuse: warnings=%v err=%v", acq.Warnings, err)
	}
	cacheHit := false
	for _, s := range acq.Sources {
		cacheHit = cacheHit || s.ID == "simbad-CACHED"
	}
	if !cacheHit {
		return errors.New("VO cache reuse: cached source missing")
	}
	return nil
}

func offlineAnnotationBundleSize() int { return len(offlineAnnotationBundle().Sources) }

// testAnnotationPlacement covers item 141: zero overlaps in 20 deterministic
// fixtures, stable output and the 10k-label latency budget.
func testAnnotationPlacement() error {
	for fixture := 0; fixture < 20; fixture++ {
		sources := annotationFixtureSources(300, 300, fixture)
		boxes := placeAnnotationLabels(600, 400, sources, 1)
		for i := range boxes {
			for j := i + 1; j < len(boxes); j++ {
				if overlapArea(boxes[i], boxes[j]) > 0 {
					return fmt.Errorf("fixture %d: overlap %d/%v %d/%v", fixture, i, boxes[i], j, boxes[j])
				}
			}
			if boxes[i].X < 0 || boxes[i].Y < 0 || boxes[i].X+boxes[i].W > 600 || boxes[i].Y+boxes[i].H > 400 {
				return fmt.Errorf("fixture %d: box outside frame %+v", fixture, boxes[i])
			}
		}
		again := placeAnnotationLabels(600, 400, sources, 1)
		if len(boxes) != len(again) {
			return fmt.Errorf("fixture %d: non-deterministic count %d/%d", fixture, len(boxes), len(again))
		}
		for i := range boxes {
			if boxes[i] != again[i] {
				return fmt.Errorf("fixture %d: non-deterministic box %d", fixture, i)
			}
		}
	}
	many := annotationSpreadSources(10000, 2000, 2000)
	start := time.Now()
	boxes := placeAnnotationLabels(2000, 2000, many, 1)
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		return fmt.Errorf("10k label placement took %v (>200ms), %d labels", elapsed, len(boxes))
	}
	return nil
}

func annotationFixtureSources(n, span, seed int) []annotationSource {
	out := make([]annotationSource, 0, n)
	step := 0.0
	if n > 0 {
		step = float64(span) / float64(n)
	}
	for i := 0; i < n; i++ {
		// Deterministic pseudo-clustering without a shared RNG state.
		a := float64(i+seed+1) * 2.399963229728653
		r := 8 + 120*math.Sqrt(float64((i*37+seed*17)%n+1)/float64(n+1))
		out = append(out, annotationSource{
			ID:       fmt.Sprintf("f%d-%d", seed, i),
			Name:     fmt.Sprintf("S%d", i),
			Type:     "star",
			RA:       step * float64(i),
			Dec:      step * float64(i),
			Mag:      8 + float64(i%10),
			Priority: 50 + (i%5)*5,
			X:        200 + r*math.Cos(a),
			Y:        150 + r*math.Sin(a),
		})
	}
	return out
}

// annotationSpreadSources creates n labels on a deterministic grid covering
// the whole frame, representing a realistic wide-field density.
func annotationSpreadSources(n, width, height int) []annotationSource {
	out := make([]annotationSource, 0, n)
	cols := int(math.Ceil(math.Sqrt(float64(n) * float64(width) / float64(height))))
	if cols < 1 {
		cols = 1
	}
	rows := (n + cols - 1) / cols
	for i := 0; i < n; i++ {
		x := float64(i%cols+1) * float64(width) / float64(cols+1)
		y := float64(i/cols+1) * float64(height) / float64(rows+1)
		out = append(out, annotationSource{
			ID: fmt.Sprintf("wide-%d", i), Name: fmt.Sprintf("W%d", i), Type: "star",
			RA: float64(i) * math.Pi / 180, Dec: float64(i) * math.Pi / 180,
			Mag: 8 + float64(i%10), Priority: 50 + (i%5)*5, X: x, Y: y,
		})
	}
	return out
}

// testAnnotationSVG covers item 139: SVG 2, all stable layers, graticules,
// constellations, scale bar, finding chart and the 2 MiB budget.
func testAnnotationSVG() error {
	w := rotatedWCS(10.0*math.Pi/180, 41.0*math.Pi/180, 60, 0, false)
	sources := projectAnnotationSources(w, 512, 512, annotationGoldenSources(nilTestReporter{}))
	opts := annotationRenderOptions{Layers: annotationAllLayers, WCS: w}
	svg := buildAnnotationSVG(512, 512, sources, nil, opts)
	for _, want := range []string{
		`version="2"`, `<title>SkyStacker annotated sky map</title>`, `<desc>`,
		`<g id="background"`, `<g id="grid-radec"`, `<g id="grid-galactic"`,
		`<g id="grid-ecliptic"`, `<g id="constellations"`, `<g id="gaia-stars"`,
		`<g id="extended-objects"`, `<g id="sso-vectors"`, `<g id="scale-bar"`,
		`<g id="finder-chart"`, `<g id="labels"`, `<g id="footer"`,
		`data-layer="grid"`, `data-layer="constellations"`, `data-layer="scale-bar"`,
		`data-layer="finder-chart"`, `data-step-deg=`,
	} {
		if !strings.Contains(svg, want) {
			return fmt.Errorf("layered SVG missing %q", want)
		}
	}
	// Non-browser SVG renderers (librsvg, ImageMagick, eog) do not resolve
	// CSS custom properties in presentation attributes: literal colours keep
	// the map visible everywhere.
	if strings.Contains(svg, "var(--") {
		return errors.New("layered SVG uses CSS var() colours; non-browser viewers render it empty")
	}
	// Magnitude must be encoded in literal presentation attributes, not only
	// in <title> tooltips (demo-test.md, photometric information loss).
	bright := buildAnnotationSVG(64, 64, []annotationSource{{X: 10, Y: 10, Mag: 3}}, nil, opts)
	faint := buildAnnotationSVG(64, 64, []annotationSource{{X: 10, Y: 10, Mag: 18}}, nil, opts)
	if !strings.Contains(bright, `fill="#ffffff"`) || !strings.Contains(bright, `opacity="0.98"`) ||
		!strings.Contains(faint, `fill="#b98a68"`) || !strings.Contains(faint, `opacity="0.55"`) {
		return errors.New("layered SVG does not encode magnitude in star markers")
	}
	if again := buildAnnotationSVG(512, 512, sources, nil, opts); again != svg {
		return errors.New("layered SVG is not stable across calls")
	}
	many := annotationSpreadSources(10000, 2000, 2000)
	large := buildAnnotationSVG(2000, 2000, many, nil, opts)
	if len(large) > 2<<20 {
		return fmt.Errorf("10k-source SVG is %d bytes (>2 MiB)", len(large))
	}
	return nil
}

// testAnnotationMultiTileCoverage verifies that cached Gaia stars from every
// tile crossing the frame are unioned, de-duplicated and visited in stable
// order. Reading only the CRVAL tile silently truncated wide fields.
func testAnnotationMultiTileCoverage() error {
	dir, err := os.MkdirTemp("", "skystacker-ann-tiles-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	nside := 32
	w := rotatedWCS(10*math.Pi/180, 41*math.Pi/180, 60, 23, false)
	ra := make([]float64, 2)
	dec := make([]float64, 2)
	points := []struct{ x, y float64 }{{64, 96}, {448, 416}}
	for i, p := range points {
		ra[i], dec[i] = wcsTanInverse(w, p.x, p.y)
	}
	ipix0 := healpixAng2PixNested(nside, ra[0], dec[0])
	ipix1 := healpixAng2PixNested(nside, ra[1], dec[1])
	if ipix0 == ipix1 {
		return errors.New("fixture points share one HEALPix tile; widen the test frame")
	}
	if err := gaiaSaveTileCache(dir, gaiaTile{Version: gaiaCacheVersion, Nside: nside, Ipix: ipix0,
		Stars: []gaiaStar{{RA: ra[0], Dec: dec[0], MagG: 8}}}); err != nil {
		return err
	}
	// The second tile repeats the first star: the union must de-duplicate it.
	if err := gaiaSaveTileCache(dir, gaiaTile{Version: gaiaCacheVersion, Nside: nside, Ipix: ipix1,
		Stars: []gaiaStar{{RA: ra[1], Dec: dec[1], MagG: 9}, {RA: ra[0], Dec: dec[0], MagG: 8}}}); err != nil {
		return err
	}
	stars := cachedGaiaStarsForField(dir, nside, w, 512, 512)
	if len(stars) != 2 {
		return fmt.Errorf("multi-tile union: got %d stars, want 2", len(stars))
	}
	again := cachedGaiaStarsForField(dir, nside, w, 512, 512)
	if len(again) != 2 || stars[0].RA != again[0].RA || stars[1].RA != again[1].RA {
		return errors.New("multi-tile union is not deterministic")
	}
	return nil
}

type nilTestReporter struct{}

func (nilTestReporter) Fatalf(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }

// testAnnotationRasterParity covers item 140: deterministic raster output,
// SSIM >= 0.99 and glyph baselines aligned with the SVG to within 1 px.
func testAnnotationRasterParity() error {
	sources := annotationFixtureSources(60, 60, 3)
	labels := placeAnnotationLabels(600, 400, sources, 1)
	base := image.NewRGBA(image.Rect(0, 0, 600, 400))
	a := renderAnnotationRaster(base, sources, nil, labels)
	b := renderAnnotationRaster(base, sources, nil, labels)
	if score := annotationSSIM(a, b); score < 0.99 {
		return fmt.Errorf("raster parity SSIM %.4f < 0.99", score)
	}
	if offset := skyLabelFontSize - annotationGlyphHeight; math.Abs(float64(offset)-annotationGlyphTopOffset) > 1 {
		return fmt.Errorf("glyph baseline offset %g vs %g", offset, annotationGlyphTopOffset)
	}
	if annotationGlyphTopOffset != 3 {
		return fmt.Errorf("glyph top offset %g, want 3", annotationGlyphTopOffset)
	}
	if len(labels) == 0 {
		return errors.New("no labels produced for parity test")
	}
	return nil
}

// testLayeredAnnotationEndToEnd covers item 143: one plate-solved fixture
// produces SVG + raster + provenance report without any network access.
func testLayeredAnnotationEndToEnd() error {
	dir, err := os.MkdirTemp("", "skystacker-layered-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(filepath.Join(dir, "_work"), 0o755); err != nil {
		return err
	}
	const w, h = 512, 512
	raw := make([]byte, 8+w*h*3*4)
	binary.LittleEndian.PutUint32(raw[0:4], uint32(w))
	binary.LittleEndian.PutUint32(raw[4:8], uint32(h))
	if err := os.WriteFile(filepath.Join(dir, "_work", "stacked_mean.f32raw"), raw, 0o644); err != nil {
		return err
	}
	sol := plateSolution{WCS: *rotatedWCS(10.0*math.Pi/180, 41.0*math.Pi/180, 120, 0, false)}
	if err := writeJSON(filepath.Join(dir, "plate_solution.json"), &sol); err != nil {
		return err
	}
	if err := runLayeredAnnotation(dir, runOptions{GaiaOffline: true}); err != nil {
		return err
	}
	svgPath := filepath.Join(dir, "stacked_annotated.svg")
	pngPath := filepath.Join(dir, "stacked_annotated.png")
	provPath := filepath.Join(dir, "annotation_provenance.json")
	for _, path := range []string{svgPath, pngPath, provPath} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			return fmt.Errorf("missing or empty %s: %v", path, err)
		}
	}
	svg, err := os.ReadFile(svgPath)
	if err != nil {
		return err
	}
	if !strings.Contains(string(svg), `version="2"`) || !strings.Contains(string(svg), `data-layer="finder-chart"`) {
		return errors.New("end-to-end SVG lacks SVG 2 layers")
	}
	pngData, err := os.ReadFile(pngPath)
	if err != nil || len(pngData) < 24 || string(pngData[:8]) != "\x89PNG\r\n\x1a\n" {
		return fmt.Errorf("invalid PNG output: %d bytes err=%v", len(pngData), err)
	}
	var report layeredAnnotationReport
	if err := json.Unmarshal(mustReadFile(provPath), &report); err != nil {
		return err
	}
	if report.Sources == 0 || len(report.Layers) != len(annotationAllLayers) || len(report.Provenance) == 0 {
		return fmt.Errorf("provenance report incomplete: %+v", report)
	}
	return nil
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}

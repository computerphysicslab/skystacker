package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestVOTableBINARY2RoundTrip(t *testing.T) {
	catalog := votableCatalog{
		Info:   map[string]string{"QUERY_STATUS": "TRUNCATED", "ROW_COUNT": "3"},
		Params: map[string]string{"ConeCatalog": "vizier"},
		Columns: []votableColumn{
			{Name: "RA", UCD: "pos.eq.ra", Unit: "deg", Datatype: "float", Values: []float64{10.5, 11.5, 12.5}},
			{Name: "Mag", UCD: "phot.mag", Datatype: "float", Values: []float64{9.5, 0, 11.5}, Null: []bool{false, true, false}},
			{Name: "IDX", Datatype: "int", Ints: []int32{0, 1, 2}},
		},
	}
	path := filepath.Join(t.TempDir(), "catalog.vot")
	if err := writeVOTable(path, catalog); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseVOTableBinary2(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Info["QUERY_STATUS"] != "TRUNCATED" || got.Params["ConeCatalog"] != "vizier" {
		t.Fatalf("metadata lost: %+v %+v", got.Info, got.Params)
	}
	if len(got.Columns) != 3 {
		t.Fatalf("columns=%d", len(got.Columns))
	}
	ra := got.Columns[0]
	if len(ra.Values) != 3 || math.Abs(ra.Values[0]-10.5) > 1e-9 {
		t.Fatalf("RA values: %+v", ra.Values)
	}
	mag := got.Columns[1]
	if len(mag.Null) != 3 || !mag.Null[1] || mag.Null[0] || mag.Null[2] {
		t.Fatalf("mag masks: %+v", mag.Null)
	}
	if len(mag.Values) != 2 || math.Abs(mag.Values[1]-11.5) > 1e-9 {
		t.Fatalf("mag values: %+v", mag.Values)
	}
	idx := got.Columns[2]
	if len(idx.Ints) != 3 || idx.Ints[2] != 2 {
		t.Fatalf("int column: %+v", idx.Ints)
	}
}

func TestExportVotableFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.vot")
	opts, err := parseArgs([]string{"--export-votable", path})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ExportVotable != path {
		t.Fatalf("flag not parsed: %+v", opts)
	}
}

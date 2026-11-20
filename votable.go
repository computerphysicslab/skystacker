package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
)

type votableColumn struct {
	Name     string
	UCD      string
	Unit     string
	Datatype string
	Values   []float64
	Ints     []int32
	Null     []bool
}

type votableCatalog struct {
	Info    map[string]string
	Params  map[string]string
	Columns []votableColumn
}

type votableKV struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type votableNull struct {
	Value string `xml:"value,attr"`
}

type votableValues struct {
	Null votableNull `xml:"NULL"`
}

type votableField struct {
	Name      string         `xml:"name,attr"`
	UCD       string         `xml:"ucd,attr,omitempty"`
	Unit      string         `xml:"unit,attr,omitempty"`
	Datatype  string         `xml:"datatype,attr"`
	Arraysize string         `xml:"arraysize,attr"`
	Values    *votableValues `xml:"VALUES,omitempty"`
}

type votableStream struct {
	Encoding string `xml:"encoding,attr"`
	Data     string `xml:",chardata"`
}

type votableBinary2 struct {
	Stream votableStream `xml:"STREAM"`
}

type votableData struct {
	Binary2 votableBinary2 `xml:"BINARY2"`
}

type votableTable struct {
	Fields []votableField `xml:"FIELD"`
	Data   votableData    `xml:"DATA"`
}

type votableResource struct {
	Table votableTable `xml:"TABLE"`
}

type votableDocument struct {
	XMLName  xml.Name        `xml:"VOTABLE"`
	Version  string          `xml:"version,attr"`
	Xmlns    string          `xml:"xmlns,attr"`
	Info     []votableKV     `xml:"INFO"`
	Params   []votableKV     `xml:"PARAM"`
	Resource votableResource `xml:"RESOURCE"`
}

func sortedKV(m map[string]string) []votableKV {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	out := make([]votableKV, 0, len(keys))
	for _, k := range keys {
		out = append(out, votableKV{Name: k, Value: m[k]})
	}
	return out
}

func votableRowCount(cols []votableColumn) int {
	n := 0
	for _, c := range cols {
		length := len(c.Values)
		if c.Datatype == "int" {
			length = len(c.Ints)
		}
		if length > n {
			n = length
		}
	}
	return n
}

func encodeVOTableBinary2(cols []votableColumn) []byte {
	rows := votableRowCount(cols)
	maskBytes := (len(cols) + 7) / 8
	var buf bytes.Buffer
	for row := 0; row < rows; row++ {
		mask := make([]byte, maskBytes)
		for i, c := range cols {
			isNull := row < len(c.Null) && c.Null[row]
			if !isNull && c.Datatype == "float" && row >= len(c.Values) {
				isNull = true
			}
			if !isNull && c.Datatype == "int" && row >= len(c.Ints) {
				isNull = true
			}
			if isNull {
				mask[i/8] |= 1 << uint(i%8)
			}
		}
		buf.Write(mask)
		for i, c := range cols {
			if mask[i/8]&(1<<uint(i%8)) != 0 {
				continue
			}
			if c.Datatype == "int" {
				_ = binary.Write(&buf, binary.LittleEndian, c.Ints[row])
			} else {
				_ = binary.Write(&buf, binary.LittleEndian, c.Values[row])
			}
		}
	}
	return buf.Bytes()
}

func writeVOTable(path string, catalog votableCatalog) error {
	payload := encodeVOTableBinary2(catalog.Columns)
	doc := votableDocument{
		Version:  "1.3",
		Xmlns:    "http://www.ivoa.net/xml/VOTable/v1.3",
		Info:     sortedKV(catalog.Info),
		Params:   sortedKV(catalog.Params),
		Resource: votableResource{Table: votableTable{Data: votableData{Binary2: votableBinary2{Stream: votableStream{Encoding: "base64", Data: base64.StdEncoding.EncodeToString(payload)}}}}},
	}
	for _, c := range catalog.Columns {
		datatype := c.Datatype
		if datatype == "" {
			datatype = "float"
		}
		field := votableField{Name: c.Name, UCD: c.UCD, Unit: c.Unit, Datatype: datatype, Arraysize: "1"}
		hasNull := false
		for _, n := range c.Null {
			if n {
				hasNull = true
				break
			}
		}
		if hasNull {
			field.Values = &votableValues{Null: votableNull{Value: "null"}}
		}
		doc.Resource.Table.Fields = append(doc.Resource.Table.Fields, field)
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return writeFileAtomic(path, func(w io.Writer) error {
		_, err := w.Write(buf.Bytes())
		return err
	})
}

func parseVOTableBinary2(data []byte) (votableCatalog, error) {
	var doc votableDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return votableCatalog{}, err
	}
	catalog := votableCatalog{Info: map[string]string{}, Params: map[string]string{}}
	for _, kv := range doc.Info {
		catalog.Info[kv.Name] = kv.Value
	}
	for _, kv := range doc.Params {
		catalog.Params[kv.Name] = kv.Value
	}
	raw, err := base64.StdEncoding.DecodeString(doc.Resource.Table.Data.Binary2.Stream.Data)
	if err != nil {
		return votableCatalog{}, err
	}
	cols := doc.Resource.Table.Fields
	maskBytes := (len(cols) + 7) / 8
	rows := 0
	for pos := 0; pos+maskBytes <= len(raw); {
		mask := raw[pos : pos+maskBytes]
		pos += maskBytes
		for i, f := range cols {
			if mask[i/8]&(1<<uint(i%8)) != 0 {
				continue
			}
			if f.Datatype == "int" {
				pos += 4
			} else {
				pos += 8
			}
		}
		rows++
	}
	for _, f := range cols {
		c := votableColumn{Name: f.Name, UCD: f.UCD, Unit: f.Unit, Datatype: f.Datatype, Null: make([]bool, rows)}
		catalog.Columns = append(catalog.Columns, c)
	}
	pos := 0
	for row := 0; row < rows; row++ {
		mask := raw[pos : pos+maskBytes]
		pos += maskBytes
		for i := range catalog.Columns {
			isNull := mask[i/8]&(1<<uint(i%8)) != 0
			catalog.Columns[i].Null[row] = isNull
			if isNull {
				continue
			}
			if catalog.Columns[i].Datatype == "int" {
				v := binary.LittleEndian.Uint32(raw[pos : pos+4])
				catalog.Columns[i].Ints = append(catalog.Columns[i].Ints, int32(v))
				pos += 4
			} else {
				bits := binary.LittleEndian.Uint64(raw[pos : pos+8])
				catalog.Columns[i].Values = append(catalog.Columns[i].Values, math.Float64frombits(bits))
				pos += 8
			}
		}
	}
	return catalog, nil
}

func collectVOTableCatalog(opts *runOptions, sol *plateSolution) votableCatalog {
	catalog := votableCatalog{
		Info:   map[string]string{},
		Params: map[string]string{},
	}
	var ra, dec, mag []float64
	var ids []int32
	var nullMag []bool
	add := func(raDeg, decDeg float64, magnitude float64, hasMag bool) {
		ra = append(ra, raDeg)
		dec = append(dec, decDeg)
		mag = append(mag, magnitude)
		nullMag = append(nullMag, !hasMag)
		ids = append(ids, int32(len(ids)))
	}
	if entries, err := loadAnnotationCatalogs(opts.AnnotationCatalogs); err == nil {
		for _, e := range entries {
			add(e.RA, e.Dec, e.Mag, e.Mag > -90)
		}
	}
	if opts.ConeCatalog != "" && sol != nil {
		if service, ok := coneServices[opts.ConeCatalog]; ok {
			if result, err := coneSearchCachedDetailed(gaiaDefaultCacheDir(), opts.ConeCatalog, service, "",
				sol.WCS.CRVAL1*180/math.Pi, sol.WCS.CRVAL2*180/math.Pi, opts.ConeRadiusArcsec, 60*time.Second); err == nil {
				status := "COMPLETE"
				if result.Truncated {
					status = "TRUNCATED"
				}
				catalog.Info["QUERY_STATUS"] = status
				catalog.Info["ROW_COUNT"] = fmt.Sprintf("%d", result.Rows)
				for _, s := range result.Sources {
					add(s.RA*180/math.Pi, s.Dec*180/math.Pi, s.Mag, s.Mag > -90)
				}
			}
		}
	}
	catalog.Params["ConeCatalog"] = opts.ConeCatalog
	catalog.Columns = []votableColumn{
		{Name: "RA", UCD: "pos.eq.ra;meta.main", Unit: "deg", Datatype: "float", Values: ra},
		{Name: "DEC", UCD: "pos.eq.dec;meta.main", Unit: "deg", Datatype: "float", Values: dec},
		{Name: "Mag", UCD: "phot.mag", Unit: "mag", Datatype: "float", Values: mag, Null: nullMag},
		{Name: "IDX", UCD: "meta.number", Datatype: "int", Ints: ids},
	}
	return catalog
}

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeMinimalFITSWithBlackLevel writes a tiny FITS primary header with the
// given BLACK_LVL value (0 means absent).
func writeMinimalFITSWithBlackLevel(t *testing.T, path string, black float64, has bool) {
	t.Helper()
	cards := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                   16",
		"NAXIS   =                    2",
		"NAXIS1  =                   10",
		"NAXIS2  =                   10",
	}
	if has {
		cards = append(cards, "BLACK_LVL= "+strconvFloat(black))
	}
	cards = append(cards, "END")
	var buf []byte
	for _, c := range cards {
		if len(c) > 80 {
			c = c[:80]
		}
		pad := 80 - len(c)
		buf = append(buf, []byte(c)...)
		buf = append(buf, make([]byte, pad)...)
	}
	for len(buf)%2880 != 0 {
		buf = append(buf, ' ')
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseFITSBlackLevelOrderIndependent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.fits")
	b := filepath.Join(dir, "b.fits")
	writeMinimalFITSWithBlackLevel2(t, a, 1018.5, true)
	writeMinimalFITSWithBlackLevel2(t, b, 2048.0, true)
	// order: read a then b
	la, ha := parseFITSBlackLevel(a)
	t.Logf("a: level=%v has=%v", la, ha)
	lb, hb := parseFITSBlackLevel(b)
	if !ha || !hb {
		t.Fatal("expected both to have BLACK_LVL")
	}
	if la != 1018.5 || lb != 2048.0 {
		t.Fatalf("levels = %v, %v", la, lb)
	}
	// read b then a (order independence)
	lb2, _ := parseFITSBlackLevel(b)
	la2, _ := parseFITSBlackLevel(a)
	if lb2 != 2048.0 || la2 != 1018.5 {
		t.Fatalf("order-dependent: %v, %v", la2, lb2)
	}
	// absent
	c := filepath.Join(dir, "c.fits")
	writeMinimalFITSWithBlackLevel2(t, c, 0, false)
	lc, hc := parseFITSBlackLevel(c)
	if hc || lc != 0 {
		t.Fatalf("absent BLACK_LVL reported present: %v %v", lc, hc)
	}
}

func writeMinimalFITSWithBlackLevel2(t *testing.T, path string, black float64, has bool) {
	t.Helper()
	cards := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                   16",
		"NAXIS   =                    2",
		"NAXIS1  =                   10",
		"NAXIS2  =                   10",
	}
	if has {
		cards = append(cards, "BLACK_LVL= "+strconvFloat(black))
	}
	cards = append(cards, "END")
	var buf []byte
	for _, c := range cards {
		if len(c) > 80 {
			c = c[:80]
		}
		buf = append(buf, []byte(c)...)
		buf = append(buf, []byte(strings.Repeat(" ", 80-len(c)))...)
	}
	for len(buf)%2880 != 0 {
		buf = append(buf, ' ')
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func strconvFloat(v float64) string {
	// use fmt-like formatting
	s := ""
	// simple: use strconv via importing? avoid import cycle by manual
	if v == float64(int64(v)) {
		s = itoa(int64(v)) + ".0"
	} else {
		s = ftoa(v)
	}
	return s
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if neg {
		return "-" + digits
	}
	return digits
}

func ftoa(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64)
}

package main

import (
	"fmt"
	"math"
	"strings"
)

type transformationCoefficients struct {
	ZP        float64 `json:"zp"`
	K         float64 `json:"k"`
	ColorTerm float64 `json:"color_term"`
}

type transformationReport struct {
	OBSType      string                                `json:"obstype"`
	Filters      []string                              `json:"filters"`
	Trans        string                                `json:"trans"`
	Coefficients map[string]transformationCoefficients `json:"coefficients"`
	NStars       int                                   `json:"n_stars"`
	RMS          float64                               `json:"rms"`
	Notes        []string                              `json:"notes,omitempty"`
}

type transformationPair struct {
	Band         string
	Instrumental float64
	Reference    float64
	Airmass      float64
	Color        float64
}

func fitTransformationBand(pairs []transformationPair) (transformationCoefficients, float64, int) {
	n := 3
	a := make([][]float64, n)
	b := make([]float64, n)
	for i := range a {
		a[i] = make([]float64, n)
	}
	count := 0
	for _, p := range pairs {
		y := p.Instrumental - p.Reference
		x := []float64{1, p.Airmass - 1, p.Color}
		for i := range n {
			b[i] += x[i] * y
			for j := range n {
				a[i][j] += x[i] * x[j]
			}
		}
		count++
	}
	if count < n {
		return transformationCoefficients{}, 0, count
	}
	for i := range n {
		a[i][i] += 1e-9
	}
	sol := solveLinear(a, b, n)
	if len(sol) != n {
		return transformationCoefficients{}, 0, count
	}
	coeff := transformationCoefficients{ZP: sol[0], K: sol[1], ColorTerm: sol[2]}
	rms := 0.0
	for _, p := range pairs {
		pred := coeff.ZP + coeff.K*(p.Airmass-1) + coeff.ColorTerm*p.Color
		d := (p.Instrumental - p.Reference) - pred
		rms += d * d
	}
	return coeff, math.Sqrt(rms / float64(count)), count
}

func computeTransformationFromExtinction(info *extinctionReport, cards map[string]string) *transformationReport {
	if info == nil {
		return nil
	}
	obstype := strings.TrimSpace(cards["OBSTYPE"])
	trans := strings.TrimSpace(cards["TRANS"])
	filters := []string{"R", "G", "B"}
	if raw := strings.TrimSpace(cards["FILTER"]); raw != "" {
		parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '|' || r == ' ' })
		if len(parts) > 0 {
			filters = parts
		}
	}
	report := &transformationReport{
		OBSType: obstype, Filters: filters, Trans: trans,
		Coefficients: map[string]transformationCoefficients{},
		Notes:        []string{"coefficients derived from the Rayleigh model; no paired multi-band catalog photometry was available"},
	}
	pressureFactor := info.PressureHPa * math.Exp(-info.SiteElevM/8500) / 1013.25
	for _, band := range filters {
		lambda, ok := extinctionWavelengths[band]
		if !ok {
			lambda = 0.55
		}
		report.Coefficients[band] = transformationCoefficients{K: rayleighCoefficient(lambda) * pressureFactor}
	}
	return report
}

func computeTransformationReport(paths []string, info *extinctionReport) *transformationReport {
	for _, path := range paths {
		cards, err := readFITSPrimaryHeader(path)
		if err != nil || len(cards) == 0 {
			continue
		}
		return computeTransformationFromExtinction(info, cards)
	}
	return computeTransformationFromExtinction(info, map[string]string{})
}

func formatTransformationReport(r *transformationReport) string {
	if r == nil {
		return ""
	}
	text := fmt.Sprintf("Photometric transformation (report only):\n  OBSTYPE=%q FILTER=%v TRANS=%q\n", r.OBSType, r.Filters, r.Trans)
	for _, band := range r.Filters {
		c := r.Coefficients[band]
		text += fmt.Sprintf("  %s: ZP=%.4f k=%.6f color=%.4f\n", band, c.ZP, c.K, c.ColorTerm)
	}
	for _, note := range r.Notes {
		text += "  note: " + note + "\n"
	}
	return text
}

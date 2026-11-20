package main

import (
	"fmt"
	"math"
)

type extinctionReport struct {
	Airmass       float64            `json:"airmass"`
	AirmassSource string             `json:"airmass_source"`
	ZenithDeg     float64            `json:"zenith_deg,omitempty"`
	SiteElevM     float64            `json:"site_elev_m"`
	PressureHPa   float64            `json:"pressure_hpa"`
	Rayleigh      map[string]float64 `json:"rayleigh"`
	Notes         []string           `json:"notes,omitempty"`
}

var extinctionWavelengths = map[string]float64{
	"R": 0.65,
	"G": 0.55,
	"B": 0.45,
}

func rayleighCoefficient(lambdaUm float64) float64 {
	inv2 := 1 / (lambdaUm * lambdaUm)
	return 0.008569 * inv2 * inv2 * (1 + 0.0113*inv2 + 0.00013*inv2*inv2)
}

func rayleighCoefficients(airmass, pressureHPa, siteElevM float64) map[string]float64 {
	if airmass <= 0 {
		airmass = 1
	}
	if pressureHPa <= 0 {
		pressureHPa = 1013.25
	}
	effectivePressure := pressureHPa * math.Exp(-siteElevM/8500)
	factor := airmass * effectivePressure / 1013.25
	out := make(map[string]float64, len(extinctionWavelengths))
	for channel, lambda := range extinctionWavelengths {
		out[channel] = rayleighCoefficient(lambda) * factor
	}
	return out
}

func computeExtinctionReport(acceptedPaths []string) *extinctionReport {
	report := &extinctionReport{
		Airmass:       1,
		AirmassSource: "default",
		SiteElevM:     10,
		PressureHPa:   1013.25,
	}
	for _, path := range acceptedPaths {
		cards, err := readFITSPrimaryHeader(path)
		if err != nil || len(cards) == 0 {
			continue
		}
		if v, ok := fitsCardFloat(cards, "AIRMASS"); ok && v > 0 {
			report.Airmass = v
			report.AirmassSource = "AIRMASS"
		} else if alt, ok := fitsCardFloat(cards, "CENTALT"); ok && alt > 0 && alt < 90 {
			zenith := 90 - alt
			report.ZenithDeg = zenith
			report.Airmass = 1 / math.Cos(zenith*math.Pi/180)
			report.AirmassSource = "CENTALT"
		}
		if v, ok := fitsCardFloat(cards, "SITEELEV"); ok {
			report.SiteElevM = v
		}
		if v, ok := fitsCardFloat(cards, "PRESSURE"); ok && v > 0 {
			report.PressureHPa = v
		}
		break
	}
	if report.AirmassSource == "default" {
		report.Notes = append(report.Notes, "no AIRMASS/CENTALT in accepted frames; assumed airmass 1.0")
	}
	report.Rayleigh = rayleighCoefficients(report.Airmass, report.PressureHPa, report.SiteElevM)
	return report
}

func formatExtinctionReport(r *extinctionReport) string {
	if r == nil {
		return ""
	}
	text := fmt.Sprintf("Extinction / Rayleigh (report only):\n  airmass: %.3f (%s)\n  site elevation: %.1f m\n  pressure: %.2f hPa\n",
		r.Airmass, r.AirmassSource, r.SiteElevM, r.PressureHPa)
	for _, channel := range []string{"R", "G", "B"} {
		text += fmt.Sprintf("  %s tau: %.6f\n", channel, r.Rayleigh[channel])
	}
	for _, note := range r.Notes {
		text += "  note: " + note + "\n"
	}
	return text
}

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type residualPoint struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	DX   float64 `json:"dx"`
	DY   float64 `json:"dy"`
	Zone int     `json:"zone"`
}

type verificationZone struct {
	Zone    int     `json:"zone"`
	N       int     `json:"n"`
	RMSPx   float64 `json:"rms_px"`
	Anomaly bool    `json:"anomaly,omitempty"`
}

type solutionVerificationReport struct {
	GlobalRMSPx   float64            `json:"global_rms_px"`
	MatchFraction float64            `json:"match_fraction"`
	NMatches      int                `json:"n_matches"`
	NGaiaInFrame  int                `json:"n_gaia_in_frame"`
	ZoneAnomaly   bool               `json:"zone_anomaly"`
	Zones         []verificationZone `json:"zones"`
	Residuals     []residualPoint    `json:"residual_map,omitempty"`
}

func computeSolutionVerification(w *wcsTan, width, height int, stars []star, gaia []gaiaStar, tolPx float64) solutionVerificationReport {
	report := solutionVerificationReport{Zones: make([]verificationZone, 9)}
	for i := range report.Zones {
		report.Zones[i].Zone = i
	}
	used := make([]bool, len(stars))
	zoneSum := make([]float64, 9)
	var total float64
	for _, g := range gaia {
		x, y := wcsTanForward(w, g.RA, g.Dec)
		if x < 0 || y < 0 || x >= float64(width) || y >= float64(height) {
			continue
		}
		report.NGaiaInFrame++
		best, bestIdx, bestDist := tolPx, -1, tolPx
		for i, s := range stars {
			if used[i] {
				continue
			}
			d := math.Hypot(s.X-x, s.Y-y)
			if d < best {
				best, bestIdx, bestDist = d, i, d
			}
		}
		if bestIdx < 0 {
			continue
		}
		used[bestIdx] = true
		s := stars[bestIdx]
		dx, dy := x-s.X, y-s.Y
		zx := int(x / (float64(width) / 3))
		zy := int(y / (float64(height) / 3))
		if zx < 0 {
			zx = 0
		}
		if zx > 2 {
			zx = 2
		}
		if zy < 0 {
			zy = 0
		}
		if zy > 2 {
			zy = 2
		}
		zone := zy*3 + zx
		r2 := dx*dx + dy*dy
		total += r2
		zoneSum[zone] += r2
		report.Zones[zone].N++
		report.Residuals = append(report.Residuals, residualPoint{X: x, Y: y, DX: dx, DY: dy, Zone: zone})
		_ = bestDist
	}
	report.NMatches = len(report.Residuals)
	if report.NGaiaInFrame > 0 {
		report.MatchFraction = float64(report.NMatches) / float64(report.NGaiaInFrame)
	}
	if report.NMatches > 0 {
		report.GlobalRMSPx = math.Sqrt(total / float64(report.NMatches))
	}
	for i := range report.Zones {
		if report.Zones[i].N > 0 {
			report.Zones[i].RMSPx = math.Sqrt(zoneSum[i] / float64(report.Zones[i].N))
		}
		if report.Zones[i].N >= 3 && report.GlobalRMSPx > 1e-6 && report.Zones[i].RMSPx > 1.5*report.GlobalRMSPx {
			report.Zones[i].Anomaly = true
			report.ZoneAnomaly = true
		}
	}
	return report
}

func buildSolutionResidualSVG(width, height int, report solutionVerificationReport) string {
	scale := 12.0
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, width+40, height+40)
	b.WriteString(`<rect width="100%" height="100%" fill="#101216"/>`)
	fmt.Fprintf(&b, `<g transform="translate(20,20)"><rect width="%d" height="%d" fill="#1b1f24" stroke="#4a5560"/>`, width, height)
	for i := 1; i < 3; i++ {
		x := i * width / 3
		y := i * height / 3
		fmt.Fprintf(&b, `<line x1="%d" y1="0" x2="%d" y2="%d" stroke="#37404a"/>`, x, x, height)
		fmt.Fprintf(&b, `<line x1="0" y1="%d" x2="%d" y2="%d" stroke="#37404a"/>`, y, width, y)
	}
	for _, p := range report.Residuals {
		fmt.Fprintf(&b, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#76d7c4" stroke-width="1.2"/>`, p.X, p.Y, p.X+p.DX*scale, p.Y+p.DY*scale)
	}
	for _, z := range report.Zones {
		zx, zy := z.Zone%3, z.Zone/3
		label := fmt.Sprintf("Z%d %.3fpx", z.Zone+1, z.RMSPx)
		color := "#94a3b8"
		if z.Anomaly {
			color = "#ef4444"
		}
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-size="12">%s</text>`, zx*width/3+6, zy*height/3+16, color, html.EscapeString(label))
	}
	fmt.Fprintf(&b, `<text x="0" y="%d" fill="#e2e8f0" font-size="12">global RMS %.4f px | matches %d/%d | zone anomaly %v</text>`, height+20, report.GlobalRMSPx, report.NMatches, report.NGaiaInFrame, report.ZoneAnomaly)
	b.WriteString(`</g></svg>`)
	return b.String()
}

func readStackFloat32(outputDir string) (int, int, []float32, error) {
	width, height, err := readStackDimensions(outputDir)
	if err != nil {
		return 0, 0, nil, err
	}
	data, err := os.ReadFile(filepath.Join(outputDir, "_work", "stacked_mean.f32raw"))
	if err != nil {
		return 0, 0, nil, err
	}
	if len(data)%4 != 0 {
		return 0, 0, nil, fmt.Errorf("stacked_mean.f32raw: odd byte length")
	}
	pix := make([]float32, len(data)/4)
	for i := range pix {
		pix[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4 : i*4+4]))
	}
	if len(pix) < width*height*3 {
		return 0, 0, nil, fmt.Errorf("stacked_mean.f32raw: buffer %d < %d", len(pix), width*height*3)
	}
	return width, height, pix, nil
}

func runSolutionVerification(outputDir string, opts runOptions) (*solutionVerificationReport, error) {
	data, err := os.ReadFile(filepath.Join(outputDir, "plate_solution.json"))
	if err != nil {
		return nil, err
	}
	var sol plateSolution
	if err := json.Unmarshal(data, &sol); err != nil {
		return nil, err
	}
	w := sol.WCS
	w.SIP = sol.SIP
	width, height, pix, err := readStackFloat32(outputDir)
	if err != nil {
		return nil, err
	}
	stars := detectStars(&frame{Width: width, Height: height, Pix: pix}, 300)
	cd := math.Hypot(w.CD[0][0], w.CD[1][0])
	if cd <= 0 {
		cd = 1e-5
	}
	radiusDeg := math.Hypot(float64(width), float64(height)) * cd * 180 / math.Pi * 0.6
	nside := opts.GaiaNside
	if nside <= 0 {
		nside = 64
	}
	gaia := cachedGaiaStarsAround(gaiaDefaultCacheDir(), nside, w.CRVAL1, w.CRVAL2, radiusDeg*math.Pi/180)
	report := computeSolutionVerification(&w, width, height, stars, gaia, 3)
	if err := writeJSON(filepath.Join(outputDir, "solution_verification.json"), report); err != nil {
		return nil, err
	}
	svg := buildSolutionResidualSVG(width, height, report)
	if err := writeFileAtomic(filepath.Join(outputDir, "stacked_solution_residuals.svg"), func(out io.Writer) error {
		_, err := io.WriteString(out, svg)
		return err
	}); err != nil {
		return nil, err
	}
	return &report, nil
}

func sortedZoneReports(z []verificationZone) []verificationZone {
	out := append([]verificationZone(nil), z...)
	sort.Slice(out, func(i, j int) bool { return out[i].Zone < out[j].Zone })
	return out
}

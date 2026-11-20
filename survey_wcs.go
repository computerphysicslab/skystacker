package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type surveyWCSFlagsReport struct {
	File     string   `json:"file"`
	Width    int      `json:"width,omitempty"`
	Height   int      `json:"height,omitempty"`
	Flags    []string `json:"flags"`
	RADESYS  string   `json:"radesys"`
	TIMESYS  string   `json:"timesys"`
	DecDeg   float64  `json:"dec_deg,omitempty"`
	OK       bool     `json:"ok"`
	Messages []string `json:"messages,omitempty"`
}

type surveyCheckReport struct {
	SurveyWCSFlags surveyWCSFlagsReport `json:"survey_wcs_flags"`
	OK             bool                 `json:"ok"`
}

func readFITSPrimaryHeader(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cards := make(map[string]string)
	block := make([]byte, 2880)
	for {
		n, err := io.ReadFull(f, block)
		if n > 0 {
			for i := 0; i+80 <= n; i += 80 {
				card := string(block[i : i+80])
				key := strings.TrimSpace(card[:8])
				if key == "END" {
					return cards, nil
				}
				eq := strings.IndexByte(card, '=')
				if eq < 0 || eq > 9 {
					continue
				}
				value := strings.TrimSpace(card[eq+1:])
				if slash := strings.IndexByte(value, '/'); slash >= 0 {
					value = value[:slash]
				}
				value = strings.TrimSpace(strings.Trim(value, "'"))
				cards[strings.TrimSpace(card[:eq])] = value
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return cards, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func fitsCardFloat(cards map[string]string, key string) (float64, bool) {
	raw, ok := cards[key]
	if !ok {
		return 0, false
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	return v, err == nil
}

func checkSurveyFITS(path string) (surveyCheckReport, error) {
	cards, err := readFITSPrimaryHeader(path)
	if err != nil {
		return surveyCheckReport{}, err
	}
	flags := make([]string, 0, 5)
	messages := make([]string, 0, 5)
	if _, bad := cards["PC001001"]; bad {
		flags = append(flags, "legacy_pc_wcs")
		messages = append(messages, "legacy PC001001/PC001002 WCS keywords present; prefer CD matrix")
	}
	if _, bad := cards["PC001002"]; bad {
		if !containsString(flags, "legacy_pc_wcs") {
			flags = append(flags, "legacy_pc_wcs")
			messages = append(messages, "legacy PC001001/PC001002 WCS keywords present; prefer CD matrix")
		}
	}
	radesys := cards["RADESYS"]
	if strings.TrimSpace(radesys) == "" {
		flags = append(flags, "missing_radesys")
		messages = append(messages, "RADESYS missing; some viewers may assume B1950")
	}
	timesys := cards["TIMESYS"]
	if strings.TrimSpace(timesys) == "" {
		flags = append(flags, "missing_timesys")
		messages = append(messages, "TIMESYS missing; sub-30 s astrometry may be ambiguous")
	}
	for key, value := range cards {
		if strings.Contains(strings.ToLower(key+" "+value), "asinh") {
			flags = append(flags, "asinh_scaling")
			messages = append(messages, "non-linear asinh scaling detected; use the FITS plane for measurements")
			break
		}
	}
	dec, hasDec := fitsCardFloat(cards, "DEC")
	provenance := strings.ToLower(cards["TELESCOP"] + " " + cards["INSTRUME"] + " " + cards["SURVEY"] + " " + cards["ORIGIN"])
	if hasDec && dec > 85 && (strings.Contains(provenance, "panstarrs") || strings.Contains(provenance, "pan-starrs") || strings.Contains(provenance, "ps1")) {
		flags = append(flags, "panstarrs_pole_risk")
		messages = append(messages, "Pan-STARRS field above Dec 85 deg may have ~0.5 arcsec position errors")
	}
	sort.Strings(flags)
	width, _ := fitsCardFloat(cards, "NAXIS1")
	height, _ := fitsCardFloat(cards, "NAXIS2")
	report := surveyCheckReport{
		SurveyWCSFlags: surveyWCSFlagsReport{
			File:     path,
			Width:    int(width),
			Height:   int(height),
			Flags:    flags,
			RADESYS:  radesys,
			TIMESYS:  timesys,
			DecDeg:   dec,
			OK:       len(flags) == 0,
			Messages: messages,
		},
		OK: len(flags) == 0,
	}
	return report, nil
}

func containsString(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func formatSurveyCheckReport(report surveyCheckReport) string {
	f := report.SurveyWCSFlags
	var b strings.Builder
	fmt.Fprintf(&b, "Survey FITS check: %s\n", f.File)
	if f.Width > 0 && f.Height > 0 {
		fmt.Fprintf(&b, "Dimensions: %dx%d\n", f.Width, f.Height)
	}
	fmt.Fprintf(&b, "survey_wcs_flags: %s\n", strings.Join(f.Flags, ", "))
	fmt.Fprintf(&b, "RADESYS: %q  TIMESYS: %q  DEC: %.4f\n", f.RADESYS, f.TIMESYS, f.DecDeg)
	for _, m := range f.Messages {
		b.WriteString("- ")
		b.WriteString(m)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "OK: %v\n", f.OK)
	return b.String()
}

func runSurveyFITSCheck(path, outputDir string) error {
	report, err := checkSurveyFITS(path)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	if outputDir == "" {
		outputDir = filepath.Dir(path)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outputDir, "report.json"), report); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(outputDir, "report.txt"), func(w io.Writer) error {
		_, err := io.WriteString(w, formatSurveyCheckReport(report))
		return err
	}); err != nil {
		return err
	}
	return nil
}

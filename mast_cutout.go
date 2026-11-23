package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// fetchMASTCutoutZip downloads a MAST HAPCut (HST) or ZCut (deep fields)
// positional cutout. The response is a ZIP archive of FITS cutouts.
func fetchMASTCutoutZip(ctx context.Context, service string, ra, dec float64, width, height int, survey string) ([]byte, error) {
	base := "https://mast.stsci.edu/hapcut/api/v0.1/astrocut"
	if service == "zcut" {
		base = "https://mast.stsci.edu/zcut/api/v0.1/astrocut"
	}
	u := fmt.Sprintf("%s?ra=%.6f&dec=%.6f&x=%d&y=%d&units=px", base, ra, dec, width, height)
	// Only ZCut accepts a survey filter; HAPCut covers all HAP imaging at the
	// position. The default --survey-hips value is a HiPS ID and must never be
	// forwarded as a ZCut survey name.
	if service == "zcut" && survey != "" && !strings.HasPrefix(survey, "CDS/") {
		u += "&survey=" + survey
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("MAST %s cutout HTTP %d: %s", service, resp.StatusCode, string(data))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	if len(data) < 4 || string(data[:2]) != "PK" {
		return nil, fmt.Errorf("MAST %s cutout returned non-ZIP (%d bytes)", service, len(data))
	}
	return data, nil
}

// mastFITSFromZip extracts the first FITS file from a MAST ZIP containing a
// science image extension with a usable WCS. The primary HDU of HAPCut
// products has no pixel data; the image lives in an XTENSION (SCI/CUTOUT).
func mastFITSFromZip(zipData []byte) ([]byte, string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, "", err
	}
	for _, f := range zr.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".fits") && !strings.HasSuffix(strings.ToLower(f.Name), ".fit") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, 256<<20))
		rc.Close()
		if err != nil || len(data) < 2880 {
			continue
		}
		if ext := mastScienceExtension(data); ext != nil {
			return ext, f.Name, nil
		}
	}
	return nil, "", fmt.Errorf("MAST ZIP contains no image extension with WCS")
}

// mastScienceExtension walks FITS HDUs and returns a synthesized single-HDU
// FITS byte slice for the first image extension with CRVAL/CD and pixel data.
func mastScienceExtension(data []byte) []byte {
	for off := 0; off+2880 <= len(data); {
		// read header cards for this HDU until END
		var cards []string
		end := off
		for p := off; p+80 <= len(data); p += 80 {
			card := strings.TrimRight(string(data[p:p+80]), " ")
			if strings.HasPrefix(card, "END") {
				end = p + 80
				break
			}
			cards = append(cards, card)
		}
		if len(cards) == 0 {
			break
		}
		cardMap := map[string]string{}
		for _, c := range cards {
			if idx := strings.IndexByte(c, '='); idx >= 0 {
				key := strings.TrimSpace(c[:idx])
				val := strings.TrimSpace(c[idx+1:])
				if i := strings.Index(val, " /"); i >= 0 {
					val = strings.TrimSpace(val[:i])
				}
				cardMap[key] = strings.Trim(val, "' ")
			}
		}
		// pad header to 2880 boundary
		headerEnd := end
		for headerEnd%2880 != 0 {
			headerEnd++
		}
		naxis1, _ := strconv.Atoi(cardMap["NAXIS1"])
		naxis2, _ := strconv.Atoi(cardMap["NAXIS2"])
		bitpix, _ := strconv.Atoi(cardMap["BITPIX"])
		_, hasCRVAL1 := cardMap["CRVAL1"]
		_, hasCD := cardMap["CD1_1"]
		if naxis1 > 0 && naxis2 > 0 && hasCRVAL1 && hasCD && (bitpix == -32 || bitpix == 16 || bitpix == 8) {
			isImage := cardMap["XTENSION"] == "IMAGE" || cardMap["EXTNAME"] == "SCI" ||
				cardMap["EXTNAME"] == "CUTOUT" || len(cardMap["XTENSION"]) == 0
			if isImage {
				bytesPer := 4
				if bitpix == 16 {
					bytesPer = 2
				} else if bitpix == 8 {
					bytesPer = 1
				}
				dataLen := naxis1 * naxis2 * bytesPer
				dataEnd := headerEnd + dataLen
				if dataEnd <= len(data) {
					// synthesize a primary-HDU FITS from this extension
					out := make([]byte, 0, 2880+dataLen+2880)
					newCards := []string{
						"SIMPLE  =                    T",
						fmt.Sprintf("BITPIX  = %20d", bitpix),
						"NAXIS   =                    2",
						fmt.Sprintf("NAXIS1  = %20d", naxis1),
						fmt.Sprintf("NAXIS2  = %20d", naxis2),
					}
					for _, k := range []string{"CRVAL1", "CRVAL2", "CRPIX1", "CRPIX2", "CD1_1", "CD1_2", "CD2_1", "CD2_2", "CDELT1", "CDELT2", "CTYPE1", "CTYPE2", "EQUINOX", "BSCALE", "BZERO"} {
						if v, ok := cardMap[k]; ok {
							newCards = append(newCards, fmt.Sprintf("%-8s= %s", k, v))
						}
					}
					newCards = append(newCards, "END")
					var hdr bytes.Buffer
					for _, c := range newCards {
						if len(c) > 80 {
							c = c[:80]
						}
						hdr.WriteString(fmt.Sprintf("%-80s", c))
					}
					for hdr.Len()%2880 != 0 {
						hdr.WriteByte(' ')
					}
					out = append(out, hdr.Bytes()...)
					out = append(out, data[headerEnd:dataEnd]...)
					for (len(out))%2880 != 0 {
						out = append(out, ' ')
					}
					return out
				}
			}
		}
		// advance to next HDU
		off = headerEnd
		if naxis1 > 0 && naxis2 > 0 {
			bp := 4
			if bitpix == 16 {
				bp = 2
			} else if bitpix == 8 {
				bp = 1
			}
			off += naxis1 * naxis2 * bp
			for off%2880 != 0 {
				off++
			}
		}
	}
	return nil
}

// writeMASTSurveyCutout fetches a MAST HAPCut/ZCut cutout, picks the first
// science image with a WCS, reprojects it onto the target WCS and writes
// aligned products. No authentication is needed for public HAP/ZCut data.
func writeMASTSurveyCutout(width, height int, wcs *wcsTan, opts runOptions, dir, productDir, stem string, refFrame *frame, timeout int) error {
	service := opts.SurveySource // "hapcut" or "zcut"
	ra, dec := wcs.CRVAL1*180/math.Pi, wcs.CRVAL2*180/math.Pi
	if timeout <= 0 {
		timeout = 120
	}
	logf("Survey cutout: HTTP timeout %d s", timeout)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	zipData, err := fetchMASTCutoutZip(ctx, service, ra, dec, width, height, opts.SurveyHIPS)
	if err != nil {
		return err
	}
	fitsData, fitsName, err := mastFITSFromZip(zipData)
	if err != nil {
		return err
	}
	srcWCS, _, err := parseAstrometryWCS(fitsData)
	if err != nil {
		return fmt.Errorf("%s align: source WCS: %w", service, err)
	}
	tmp, err := os.CreateTemp("", "skystacker-mast-*.fits")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Write(fitsData)
	tmp.Close()
	defer os.Remove(tmpPath)
	sourceFrame, err := decodeFITSNative(tmpPath, 0, 0)
	if err != nil {
		return fmt.Errorf("%s align: source pixels: %w", service, err)
	}
	aligned, invalid := reprojectSurvey(sourceFrame, srcWCS, wcs, width, height, 4)
	if aligned == nil {
		return fmt.Errorf("%s align: reprojection failed", service)
	}
	if refFrame != nil {
		dx, dy, ncc := surveyAlignmentShift(refFrame, aligned)
		if dx != 0 || dy != 0 {
			aligned = applySurveyAlignmentShift(aligned, dx, dy)
			logf("%s: alignment shift corrected by (%d, %d) px (NCC %.3f)", service, dx, dy, ncc)
		}
	}
	stem2 := stem + "_" + service
	return writeAlignedSurveyProducts(productDir, stem2, aligned, wcs, srcWCS,
		fitsData, service, fitsName, "", invalid, nil)
}

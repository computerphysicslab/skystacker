package main

import (
	"math"
)

// surveyAlignmentShiftMax is the largest integer shift (in pixels) searched in
// each direction. The DSS reprojection is expected to be sub-pixel accurate;
// this window only corrects small residual mis-registrations.
const surveyAlignmentShiftMax = 16

// surveyAlignmentShift finds the integer (dx, dy) shift that best aligns
// `aligned` to `ref` using a high-pass (box-blur-subtracted) normalised
// cross-correlation. The returned shift is applied to `aligned` (i.e.
// corrected[y, x] = aligned[y-dy, x-dx]) to produce the aligned result.
// ncc is the correlation at the best shift (higher is better). When either
// frame is nil or too small, or the best correlation is not clearly better
// than (0, 0), (0, 0) is returned so the caller never invents a correction.
// subPlane extracts a rectangular region from a plane.
func subPlane(p [][]float64, x0, y0, x1, y1 int) [][]float64 {
	out := make([][]float64, y1-y0)
	for i := 0; i < y1-y0; i++ {
		row := make([]float64, x1-x0)
		copy(row, p[y0+i][x0:x1])
		out[i] = row
	}
	return out
}

func surveyAlignmentShift(ref, aligned *frame) (dx, dy int, ncc float64) {
	if ref == nil || aligned == nil || ref.Width != aligned.Width || ref.Height != aligned.Height {
		return 0, 0, 0
	}
	if ref.Width < 32 || ref.Height < 32 {
		return 0, 0, 0
	}
	w, h := ref.Width, ref.Height
	ra := luminancePlane(ref)
	rb := luminancePlane(aligned)
	// High-pass: remove the low-frequency brightness gradient (vignetting,
	// passband differences) so the correlation locks onto structure, not
	// overall illumination.
	raHP := highPassPlane(ra, w, h, 15)
	rbHP := highPassPlane(rb, w, h, 15)

	bestNCC := 0.0
	bestDx, bestDy := 0, 0
	haveBest := false
	for dy := -surveyAlignmentShiftMax; dy <= surveyAlignmentShiftMax; dy++ {
		for dx := -surveyAlignmentShiftMax; dx <= surveyAlignmentShiftMax; dx++ {
			y0 := maxInt(0, dy)
			y1 := minInt(h, h+dy)
			x0 := maxInt(0, dx)
			x1 := minInt(w, w+dx)
			if y1-y0 < 16 || x1-x0 < 16 {
				continue
			}
			aa := subPlane(raHP, x0, y0, x1, y1)
			bb := subPlane(rbHP, x0-dx, y0-dy, x1-dx, y1-dy)
			am := meanPlane(aa)
			bm := meanPlane(bb)
			aa = subScalar(aa, am)
			bb = subScalar(bb, bm)
			denom := math.Sqrt(dotPlane(aa, aa) * dotPlane(bb, bb))
			if denom <= 0 {
				continue
			}
			cur := dotPlane(aa, bb) / denom
			if !haveBest || cur > bestNCC {
				bestNCC = cur
				bestDx, bestDy = dx, dy
				haveBest = true
			}
		}
	}
	// Only accept the correction when it is meaningfully better than (0, 0).
	// A tiny improvement is just noise and must not shift pixels.
	zeroNCC := planeNCC(raHP, rbHP)
	if bestNCC < zeroNCC+0.02 {
		return 0, 0, zeroNCC
	}
	return bestDx, bestDy, bestNCC
}

// luminancePlane extracts a float64 luminance plane from a frame using Rec.709.
func luminancePlane(f *frame) [][]float64 {
	w, h := f.Width, f.Height
	out := make([][]float64, h)
	for y := 0; y < h; y++ {
		row := make([]float64, w)
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			row[x] = 0.2126*float64(f.Pix[i]) + 0.7152*float64(f.Pix[i+1]) + 0.0722*float64(f.Pix[i+2])
		}
		out[y] = row
	}
	return out
}

// highPassPlane subtracts a (2k+1)x(2k+1) box-blur from p.
func highPassPlane(p [][]float64, w, h, k int) [][]float64 {
	if k < 1 {
		k = 1
	}
	size := 2*k + 1
	// integral image
	ii := make([][]float64, h+1)
	for i := range ii {
		ii[i] = make([]float64, w+1)
	}
	for y := 0; y < h; y++ {
		rowSum := 0.0
		for x := 0; x < w; x++ {
			rowSum += p[y][x]
			ii[y+1][x+1] = ii[y][x+1] + rowSum
		}
	}
	out := make([][]float64, h)
	for y := 0; y < h; y++ {
		row := make([]float64, w)
		for x := 0; x < w; x++ {
			y0 := y - k
			if y0 < 0 {
				y0 = 0
			}
			y1 := y + k + 1
			if y1 > h {
				y1 = h
			}
			x0 := x - k
			if x0 < 0 {
				x0 = 0
			}
			x1 := x + k + 1
			if x1 > w {
				x1 = w
			}
			area := float64((y1 - y0) * (x1 - x0))
			sum := ii[y1][x1] - ii[y0][x1] - ii[y1][x0] + ii[y0][x0]
			row[x] = p[y][x] - sum/area
		}
		out[y] = row
	}
	_ = size
	return out
}

func meanPlane(p [][]float64) float64 {
	var s float64
	n := 0
	for _, row := range p {
		for _, v := range row {
			s += v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return s / float64(n)
}

func subScalar(p [][]float64, v float64) [][]float64 {
	out := make([][]float64, len(p))
	for i, row := range p {
		r := make([]float64, len(row))
		for j := range row {
			r[j] = row[j] - v
		}
		out[i] = r
	}
	return out
}

func dotPlane(a, b [][]float64) float64 {
	var s float64
	for i := range a {
		for j := range a[i] {
			s += a[i][j] * b[i][j]
		}
	}
	return s
}

func planeNCC(a, b [][]float64) float64 {
	am, bm := meanPlane(a), meanPlane(b)
	aa := subScalar(a, am)
	bb := subScalar(b, bm)
	denom := math.Sqrt(dotPlane(aa, aa) * dotPlane(bb, bb))
	if denom <= 0 {
		return 0
	}
	return dotPlane(aa, bb) / denom
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// applySurveyAlignmentShift translates an aligned frame by (dx, dy) using
// bilinear resampling. Pixels with no source coverage become invalid (NaN in
// the float plane, reported as invalid by the caller if needed).
func applySurveyAlignmentShift(f *frame, dx, dy int) *frame {
	if f == nil || (dx == 0 && dy == 0) {
		return f
	}
	w, h := f.Width, f.Height
	out := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := float64(x - dx)
			sy := float64(y - dy)
			r, g, b, ok := sampleBilinearFloat(f.Pix, w, h, sx, sy)
			if !ok {
				continue
			}
			i := (y*w + x) * 3
			out.Pix[i], out.Pix[i+1], out.Pix[i+2] = r, g, b
		}
	}
	return out
}

// sampleBilinearFloat is the float32 counterpart of sampleBilinear for frames.
func sampleBilinearFloat(pix []float32, w, h int, x, y float64) (float32, float32, float32, bool) {
	if x < 0 || y < 0 || x >= float64(w-1) || y >= float64(h-1) {
		return 0, 0, 0, false
	}
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)
	if x0+1 >= w || y0+1 >= h {
		return 0, 0, 0, false
	}
	i00 := (y0*w + x0) * 3
	i10 := (y0*w + x0 + 1) * 3
	i01 := ((y0+1)*w + x0) * 3
	i11 := ((y0+1)*w + x0 + 1) * 3
	r := (1-fx)*(1-fy)*float64(pix[i00]) + fx*(1-fy)*float64(pix[i10]) + (1-fx)*fy*float64(pix[i01]) + fx*fy*float64(pix[i11])
	g := (1-fx)*(1-fy)*float64(pix[i00+1]) + fx*(1-fy)*float64(pix[i10+1]) + (1-fx)*fy*float64(pix[i01+1]) + fx*fy*float64(pix[i11+1])
	b := (1-fx)*(1-fy)*float64(pix[i00+2]) + fx*(1-fy)*float64(pix[i10+2]) + (1-fx)*fy*float64(pix[i01+2]) + fx*fy*float64(pix[i11+2])
	return float32(r), float32(g), float32(b), true
}

// surveyAlignmentShiftReport records the residual correction applied to a
// survey product so the alignment can be audited.
type surveyAlignmentShiftReport struct {
	Applied bool    `json:"applied"`
	DX      int     `json:"dx"`
	DY      int     `json:"dy"`
	NCC     float64 `json:"ncc_before_correction,omitempty"`
	Note    string  `json:"note,omitempty"`
}

func formatShiftReport(dx, dy int, ncc float64, applied bool, note string) surveyAlignmentShiftReport {
	return surveyAlignmentShiftReport{Applied: applied, DX: dx, DY: dy, NCC: ncc, Note: note}
}

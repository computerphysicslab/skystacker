package main

import (
	"image"
	"testing"
)

// makeShiftedFrame builds a frame with a deterministic pattern and returns
// (ref, shifted) where shifted is ref translated by (dx,dy).
func makeShiftedFrame(w, h, dx, dy int) (*frame, *frame) {
	ref := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			v := float32(0)
			// a few bright blobs so structure exists
			for _, b := range []struct{ cx, cy int }{{w / 4, h / 4}, {3 * w / 4, h / 3}, {w / 2, 2 * h / 3}} {
				d := float64((x-b.cx)*(x-b.cx) + (y-b.cy)*(y-b.cy))
				if d < 400 {
					v = 200 * float32(1-d/400)
				}
			}
			// gradient for low-frequency content
			v += float32(0.1 * float64(x+y))
			ref.Pix[i], ref.Pix[i+1], ref.Pix[i+2] = v, v, v
		}
	}
	// shifted: content of ref moved by (dx,dy)
	shifted := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x-dx, y-dy
			if sx < 0 || sy < 0 || sx >= w || sy >= h {
				continue
			}
			i := (y*w + x) * 3
			j := (sy*w + sx) * 3
			shifted.Pix[i], shifted.Pix[i+1], shifted.Pix[i+2] = ref.Pix[j], ref.Pix[j+1], ref.Pix[j+2]
		}
	}
	return ref, shifted
}

func TestSurveyAlignmentShiftRecoversKnownShift(t *testing.T) {
	for _, tc := range []struct{ dx, dy int }{{3, -2}, {-4, 5}, {0, 0}, {7, -6}, {-1, 8}} {
		ref, shifted := makeShiftedFrame(128, 96, tc.dx, tc.dy)
		gotDx, gotDy, ncc := surveyAlignmentShift(ref, shifted)
		if gotDx != -tc.dx || gotDy != -tc.dy {
			t.Fatalf("shift (%d,%d): got (%d,%d) ncc=%.3f", tc.dx, tc.dy, gotDx, gotDy, ncc)
		}
		if ncc < 0.5 {
			t.Fatalf("shift (%d,%d): ncc too low %.3f", tc.dx, tc.dy, ncc)
		}
	}
}

func TestSurveyAlignmentShiftNoShiftOnIdentical(t *testing.T) {
	ref, _ := makeShiftedFrame(128, 96, 0, 0)
	dx, dy, _ := surveyAlignmentShift(ref, ref)
	if dx != 0 || dy != 0 {
		t.Fatalf("identical frames: got shift (%d,%d), want (0,0)", dx, dy)
	}
}

func TestSurveyAlignmentShiftRejectsMismatched(t *testing.T) {
	ref, _ := makeShiftedFrame(128, 96, 3, -2)
	shifted2 := &frame{Width: 64, Height: 48, Pix: make([]float32, 64*48*3)}
	dx, dy, ncc := surveyAlignmentShift(ref, shifted2)
	if dx != 0 || dy != 0 || ncc != 0 {
		t.Fatalf("mismatched sizes accepted: (%d,%d) ncc=%.3f", dx, dy, ncc)
	}
}

func TestSurveyAlignmentShiftRejectsNil(t *testing.T) {
	ref, _ := makeShiftedFrame(64, 48, 1, 1)
	dx, dy, ncc := surveyAlignmentShift(nil, ref)
	if dx != 0 || dy != 0 || ncc != 0 {
		t.Fatal("nil ref accepted")
	}
	dx, dy, ncc = surveyAlignmentShift(ref, nil)
	if dx != 0 || dy != 0 || ncc != 0 {
		t.Fatal("nil aligned accepted")
	}
}

func TestApplySurveyAlignmentShiftRoundTrip(t *testing.T) {
	ref, shifted := makeShiftedFrame(64, 48, 4, -3)
	dx, dy, _ := surveyAlignmentShift(ref, shifted)
	corrected := applySurveyAlignmentShift(shifted, dx, dy)
	// corrected should be close to ref
	var mae float64
	n := 0
	for y := 0; y < ref.Height; y++ {
		for x := 0; x < ref.Width; x++ {
			i := (y*ref.Width + x) * 3
			if corrected.Pix[i] == 0 && corrected.Pix[i+1] == 0 && corrected.Pix[i+2] == 0 {
				continue
			}
			mae += float64(absFloat32(corrected.Pix[i] - ref.Pix[i]))
			n++
		}
	}
	if n == 0 || mae/float64(n) > 5 {
		t.Fatalf("corrected MAE %.2f (n=%d), want ~0", mae/float64(n), n)
	}
}

func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

var _ = image.NewRGBA

func TestApplySurveyAlignmentShiftReal(t *testing.T) {
	// Load the actual frames via the test helper? Use makeShiftedFrame instead.
	ref, shifted := makeShiftedFrame(64, 48, 4, -3)
	dx, dy, _ := surveyAlignmentShift(ref, shifted)
	corrected := applySurveyAlignmentShift(shifted, dx, dy)
	// compute MAE between corrected and ref
	var mae float64
	n := 0
	for y := 0; y < ref.Height; y++ {
		for x := 0; x < ref.Width; x++ {
			i := (y*ref.Width + x) * 3
			if corrected.Pix[i] == 0 && corrected.Pix[i+1] == 0 && corrected.Pix[i+2] == 0 {
				continue
			}
			d := corrected.Pix[i] - ref.Pix[i]
			if d < 0 {
				d = -d
			}
			mae += float64(d)
			n++
		}
	}
	t.Logf("dx=%d dy=%d corrected MAE=%.4f n=%d", dx, dy, mae/float64(n), n)
	if mae/float64(n) > 1 {
		t.Fatalf("corrected MAE too high: %.4f", mae/float64(n))
	}
}

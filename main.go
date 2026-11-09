// SkyStacker v0.3.0 — single-file build with star registration.
//
// Build:  go build -o skystacker main.go
// Test:   ./skystacker --selftest
// Run:    ./skystacker [flags] [input-directory]
//
// v0.3.0 adds the 0.2 milestone (registration):
//   - Star detection with sub-pixel centroiding
//   - Reference-frame selection by star count
//   - Triangle-invariant star matching with RANSAC similarity estimation
//   - Bilinear sub-pixel resampling
//   - Safe crop to the largest fully-valid axis-aligned rectangle
//
// Frames that fail registration are reported and excluded from the
// registered stack (they remain in the no-correction mean).

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/jpeg"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const version = "0.4.1"

// =========================================================================
// CLI
// =========================================================================

type cliOptions struct {
	InputDir      string
	OutputDir     string
	Workers       int
	Recursive     bool
	Overwrite     bool
	Version       bool
	Help          bool
	SelfTest      bool
	Register      bool
	PostProcess   bool
	StackOnly     bool
	PostOnly      bool
	NoColorCalib  bool
	NoSharpen     bool
	BGDegree      int
	StretchFactor float64
	SharpenAmount float64
	SharpenRadius float64
}

func parseArgs(args []string) (*cliOptions, error) {
	fs := flag.NewFlagSet("skystacker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	o := &cliOptions{}
	var noRegister bool
	fs.StringVar(&o.OutputDir, "output", "", "output directory (default: <input>/skystacker_results)")
	fs.IntVar(&o.Workers, "workers", runtime.NumCPU(), "maximum concurrent workers")
	fs.BoolVar(&o.Recursive, "recursive", false, "search subdirectories")
	fs.BoolVar(&o.Overwrite, "overwrite", false, "permit replacing files in the output directory")
	fs.BoolVar(&o.Version, "version", false, "print version and exit")
	fs.BoolVar(&o.SelfTest, "selftest", false, "run internal self-tests and exit")
	fs.BoolVar(&noRegister, "no-register", false, "disable star-based registration")
	var noPostProcess bool
	var stackOnly, postOnly bool
	fs.BoolVar(&noPostProcess, "no-postprocess", false, "alias for --stack-only")
	fs.BoolVar(&stackOnly, "stack-only", false, "run stacking only, skip post-processing")
	fs.BoolVar(&postOnly, "post-only", false, "run post-processing only using existing TIFFs")
	fs.BoolVar(&o.NoColorCalib, "no-color-calib", false, "skip color calibration step")
	fs.BoolVar(&o.NoSharpen, "no-sharpen", false, "skip sharpening step")
	fs.IntVar(&o.BGDegree, "bg-degree", 2, "background polynomial degree (1-3)")
	fs.Float64Var(&o.StretchFactor, "stretch", 10.0, "asinh stretch factor")
	fs.Float64Var(&o.SharpenAmount, "sharpen-amount", 0.8, "unsharp mask amount")
	fs.Float64Var(&o.SharpenRadius, "sharpen-radius", 1.5, "unsharp mask radius in pixels")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			o.Help = true
			return o, nil
		}
		return nil, err
	}
	o.Register = !noRegister
	o.PostProcess = !noPostProcess && !stackOnly
	if stackOnly && postOnly {
		return nil, fmt.Errorf("--stack-only and --post-only are mutually exclusive")
	}
	o.StackOnly = stackOnly
	o.PostOnly = postOnly
	if o.BGDegree < 1 || o.BGDegree > 3 {
		return nil, fmt.Errorf("--bg-degree must be 1, 2 or 3, got %d", o.BGDegree)
	}
	if o.StretchFactor < 0 {
		return nil, fmt.Errorf("--stretch must be >= 0, got %g", o.StretchFactor)
	}
	if o.SharpenAmount < 0 || o.SharpenAmount > 5 {
		return nil, fmt.Errorf("--sharpen-amount must be in [0, 5], got %g", o.SharpenAmount)
	}
	if o.SharpenRadius <= 0 || o.SharpenRadius > 20 {
		return nil, fmt.Errorf("--sharpen-radius must be in (0, 20], got %g", o.SharpenRadius)
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return nil, fmt.Errorf("unexpected arguments: %v", rest[1:])
	}
	if len(rest) == 1 {
		o.InputDir = rest[0]
	} else {
		o.InputDir = "."
	}
	if o.Workers < 1 {
		return nil, fmt.Errorf("--workers must be >= 1, got %d", o.Workers)
	}
	return o, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: skystacker [flags] [input-directory]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Combine JPG/JPEG astrophotography frames with optional star-based")
	fmt.Fprintln(w, "registration into sum, mean and no-correction-mean images.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  input-directory   directory containing JPG/JPEG files (default: .)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --output string   output directory (default: <input>/skystacker_results)")
	fmt.Fprintln(w, "  --workers int     maximum concurrent workers (default: NumCPU)")
	fmt.Fprintln(w, "  --recursive       search subdirectories")
	fmt.Fprintln(w, "  --overwrite       permit replacing files in the output directory")
	fmt.Fprintln(w, "  --no-register     disable star registration (v0.2 behaviour)")
	fmt.Fprintln(w, "  --stack-only      run stacking only, skip post-processing")
	fmt.Fprintln(w, "  --post-only       run post-processing only using existing TIFFs")
	fmt.Fprintln(w, "  --no-postprocess  alias for --stack-only")
	fmt.Fprintln(w, "  --no-color-calib  skip color calibration step")
	fmt.Fprintln(w, "  --no-sharpen      skip sharpening step")
	fmt.Fprintln(w, "  --bg-degree N     background polynomial degree (1-3, default 2)")
	fmt.Fprintln(w, "  --stretch F       asinh stretch factor (default 10)")
	fmt.Fprintln(w, "  --sharpen-amount F  unsharp mask amount (default 0.8)")
	fmt.Fprintln(w, "  --sharpen-radius F  unsharp mask radius (default 1.5)")
	fmt.Fprintln(w, "  --selftest        run internal self-tests and exit")
	fmt.Fprintln(w, "  --version         print version and exit")
	fmt.Fprintln(w, "  --help            print help")
}

// =========================================================================
// Discovery
// =========================================================================

func discoverFiles(root string, recursive bool, exclude []string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	excludes := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		abs, err := filepath.Abs(e)
		if err != nil {
			return nil, err
		}
		excludes[abs] = true
	}
	var files []string
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if excludes[abs] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != absRoot && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" {
			files = append(files, abs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// =========================================================================
// Imaging
// =========================================================================

type frame struct {
	Width  int
	Height int
	Pix    []float32 // interleaved RGB in [0, 1], unless a pixel was marked invalid (-1)
}

func decodeImageConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg, err
}

func decodeImageFile(path string) (*frame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return frameFromImage(img)
}

func frameFromImage(img image.Image) (*frame, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", w, h)
	}
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	switch src := img.(type) {
	case *image.YCbCr:
		const inv = 1.0 / 255.0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				yi := src.YOffset(b.Min.X+x, b.Min.Y+y)
				ci := src.COffset(b.Min.X+x, b.Min.Y+y)
				r, g, bl := color.YCbCrToRGB(src.Y[yi], src.Cb[ci], src.Cr[ci])
				i := (y*w + x) * 3
				f.Pix[i+0] = float32(r) * inv
				f.Pix[i+1] = float32(g) * inv
				f.Pix[i+2] = float32(bl) * inv
			}
		}
	default:
		const inv = 1.0 / 65535.0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				i := (y*w + x) * 3
				f.Pix[i+0] = float32(r) * inv
				f.Pix[i+1] = float32(g) * inv
				f.Pix[i+2] = float32(bl) * inv
			}
		}
	}
	return f, nil
}

// grayscale returns a luminance image in [0, 1].
func grayscale(f *frame) []float32 {
	n := f.Width * f.Height
	g := make([]float32, n)
	for i := 0; i < n; i++ {
		g[i] = 0.299*f.Pix[i*3] + 0.587*f.Pix[i*3+1] + 0.114*f.Pix[i*3+2]
	}
	return g
}

// =========================================================================
// Star detection
// =========================================================================

type star struct {
	X, Y float64 // sub-pixel centroid
	Flux float64 // background-subtracted integrated intensity
}

// estimateBackgroundNoise uses a 256-bin histogram over a subsample to
// return the median (background level) and an IQR-based Gaussian sigma.
func estimateBackgroundNoise(gray []float32) (bg, noise float64) {
	const numBins = 256
	var hist [numBins]int64
	total := int64(0)
	step := 4
	for i := 0; i < len(gray); i += step {
		v := gray[i]
		if v < 0 {
			v = 0
		} else if v >= 1 {
			v = 0.999999
		}
		hist[int(v*numBins)]++
		total++
	}
	if total == 0 {
		return 0, 1e-6
	}
	p25, p50, p75 := int64(0), int64(0), int64(0)
	want := []int64{total * 25 / 100, total * 50 / 100, total * 75 / 100}
	cum := int64(0)
	idx := 0
	for i := 0; i < numBins && idx < 3; i++ {
		cum += hist[i]
		for idx < 3 && cum >= want[idx] {
			switch idx {
			case 0:
				p25 = int64(i)
			case 1:
				p50 = int64(i)
			case 2:
				p75 = int64(i)
			}
			idx++
		}
	}
	bg = float64(p50) / numBins
	iqr := float64(p75-p25) / numBins
	noise = iqr / 1.349
	if noise < 1e-6 {
		noise = 1e-6
	}
	return bg, noise
}

// detectStars returns up to maxStars stars sorted by decreasing flux.
func detectStars(f *frame, maxStars int) []star {
	w, h := f.Width, f.Height
	gray := grayscale(f)
	bg, noise := estimateBackgroundNoise(gray)
	// The histogram returns the lower edge of the median bin. Add one
	// bin width (1/256) so that background pixels — which may lie
	// anywhere within the median bin — fall below the threshold rather
	// than leak through as one giant connected component.
	bg += 1.0 / 256.0
	threshold := float32(bg + 5*noise)
	bg32 := float32(bg)

	visited := make([]bool, w*h)
	type comp struct {
		pixels []int
	}
	var comps []comp

	// Iterative flood fill for 8-connectivity
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if visited[i] || gray[i] <= threshold {
				continue
			}
			// BFS
			stack := []int{i}
			visited[i] = true
			var pixels []int
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				pixels = append(pixels, p)
				py, px := p/w, p%w
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := px+dx, py+dy
						if nx < 0 || ny < 0 || nx >= w || ny >= h {
							continue
						}
						ni := ny*w + nx
						if visited[ni] || gray[ni] <= threshold {
							continue
						}
						visited[ni] = true
						stack = append(stack, ni)
					}
				}
			}
			if len(pixels) >= 2 && len(pixels) <= 200 {
				comps = append(comps, comp{pixels: pixels})
			}
		}
	}

	stars := make([]star, 0, len(comps))
	const margin = 3
	for _, c := range comps {
		var sumW, sumX, sumY, sumFlux float64
		minX, minY, maxX, maxY := w, h, -1, -1
		for _, p := range c.pixels {
			py, px := p/w, p%w
			val := float64(gray[p] - bg32)
			if val < 0 {
				val = 0
			}
			sumW += val
			sumX += float64(px) * val
			sumY += float64(py) * val
			sumFlux += val
			if px < minX {
				minX = px
			}
			if px > maxX {
				maxX = px
			}
			if py < minY {
				minY = py
			}
			if py > maxY {
				maxY = py
			}
		}
		if sumW == 0 {
			continue
		}
		cx := sumX / sumW
		cy := sumY / sumW
		if cx < margin || cy < margin || cx > float64(w-margin) || cy > float64(h-margin) {
			continue
		}
		// Shape filter: reject very elongated blobs
		bw, bh := maxX-minX+1, maxY-minY+1
		aspect := float64(bw) / float64(bh)
		if aspect < 0.33 || aspect > 3.0 {
			continue
		}
		stars = append(stars, star{X: cx, Y: cy, Flux: sumFlux})
	}
	sort.Slice(stars, func(i, j int) bool { return stars[i].Flux > stars[j].Flux })
	if len(stars) > maxStars {
		stars = stars[:maxStars]
	}
	return stars
}

// =========================================================================
// Similarity transform
// =========================================================================

type point2D struct {
	X, Y float64
}

// similarity represents p' = s·R·p + t, encoded as:
//
//	p'.X = SC·p.X - SS·p.Y + Tx
//	p'.Y = SS·p.X + SC·p.Y + Ty
//
// with s = sqrt(SC² + SS²), θ = atan2(SS, SC).
type similarity struct {
	SC, SS, Tx, Ty float64
}

var identitySimilarity = similarity{SC: 1, SS: 0, Tx: 0, Ty: 0}

func (s similarity) apply(p point2D) point2D {
	return point2D{
		X: s.SC*p.X - s.SS*p.Y + s.Tx,
		Y: s.SS*p.X + s.SC*p.Y + s.Ty,
	}
}

func (s similarity) inverse() similarity {
	det := s.SC*s.SC + s.SS*s.SS
	if det < 1e-12 {
		return identitySimilarity
	}
	invSC := s.SC / det
	invSS := -s.SS / det
	return similarity{
		SC: invSC,
		SS: invSS,
		Tx: -(invSC*s.Tx - invSS*s.Ty),
		Ty: -(invSS*s.Tx + invSC*s.Ty),
	}
}

func (s similarity) scale() float64 {
	return math.Sqrt(s.SC*s.SC + s.SS*s.SS)
}

func (s similarity) rotation() float64 {
	return math.Atan2(s.SS, s.SC)
}

// estimateSimilarity returns the least-squares similarity mapping src→dst.
func estimateSimilarity(src, dst []point2D) similarity {
	n := len(src)
	if n < 2 || n != len(dst) {
		return identitySimilarity
	}
	var scX, scY, dcX, dcY float64
	for i := 0; i < n; i++ {
		scX += src[i].X
		scY += src[i].Y
		dcX += dst[i].X
		dcY += dst[i].Y
	}
	scX /= float64(n)
	scY /= float64(n)
	dcX /= float64(n)
	dcY /= float64(n)
	var num1, num2, den float64
	for i := 0; i < n; i++ {
		sx := src[i].X - scX
		sy := src[i].Y - scY
		dx := dst[i].X - dcX
		dy := dst[i].Y - dcY
		num1 += sx*dx + sy*dy
		num2 += sx*dy - sy*dx
		den += sx*sx + sy*sy
	}
	if den < 1e-12 {
		return identitySimilarity
	}
	sc := num1 / den
	ss := num2 / den
	return similarity{
		SC: sc,
		SS: ss,
		Tx: dcX - (sc*scX - ss*scY),
		Ty: dcY - (ss*scX + sc*scY),
	}
}

// =========================================================================
// Triangle-based star matching
// =========================================================================

type triangle struct {
	A, B, C int     // canonical vertex indices: A opposite shortest side, C opposite longest
	R1, R2  float64 // side ratios (short/long, mid/long)
}

func dist2(a, b star) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

// buildTriangles forms canonical triangles from the top-N brightest stars.
func buildTriangles(stars []star) []triangle {
	n := len(stars)
	if n > 30 {
		n = 30
	}
	if n < 3 {
		return nil
	}
	var out []triangle
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dij2 := dist2(stars[i], stars[j])
			if dij2 < 25 { // reject very short sides
				continue
			}
			for k := j + 1; k < n; k++ {
				dik2 := dist2(stars[i], stars[k])
				djk2 := dist2(stars[j], stars[k])
				if dik2 < 25 || djk2 < 25 {
					continue
				}
				// sides[] = (length, opposite-vertex-index)
				sides := [3][2]float64{
					{math.Sqrt(djk2), float64(i)}, // opposite i
					{math.Sqrt(dik2), float64(j)}, // opposite j
					{math.Sqrt(dij2), float64(k)}, // opposite k
				}
				// Sort ascending by length
				if sides[1][0] < sides[0][0] {
					sides[0], sides[1] = sides[1], sides[0]
				}
				if sides[2][0] < sides[1][0] {
					sides[1], sides[2] = sides[2], sides[1]
				}
				if sides[1][0] < sides[0][0] {
					sides[0], sides[1] = sides[1], sides[0]
				}
				// Reject near-isosceles top two (ambiguous ordering)
				if sides[2][0]-sides[1][0] < 0.05*sides[2][0] {
					continue
				}
				// Reject elongated triangles
				if sides[0][0] < 0.15*sides[2][0] {
					continue
				}
				out = append(out, triangle{
					A:  int(sides[0][1]),
					B:  int(sides[1][1]),
					C:  int(sides[2][1]),
					R1: sides[0][0] / sides[2][0],
					R2: sides[1][0] / sides[2][0],
				})
			}
		}
	}
	return out
}

type triangleMatch struct {
	Ref, Tgt [3]int // star indices in ref / target frame
}

// chirality returns the sign of (B-A) × (C-A).
func chirality(stars []star, a, b, c int) float64 {
	ax, ay := stars[a].X, stars[a].Y
	bx, by := stars[b].X, stars[b].Y
	cx, cy := stars[c].X, stars[c].Y
	return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
}

func matchTriangles(refStars, tgtStars []star) []triangleMatch {
	refTris := buildTriangles(refStars)
	tgtTris := buildTriangles(tgtStars)
	if len(refTris) == 0 || len(tgtTris) == 0 {
		return nil
	}
	// Bucket reference triangles by descriptor into a 0.01 grid.
	const grid = 100
	bucket := make(map[[2]int][]int, len(refTris))
	for i, rt := range refTris {
		k := [2]int{int(rt.R1 * grid), int(rt.R2 * grid)}
		bucket[k] = append(bucket[k], i)
	}
	const descTol = 0.02
	var out []triangleMatch
	for _, tt := range tgtTris {
		key := [2]int{int(tt.R1 * grid), int(tt.R2 * grid)}
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for _, ri := range bucket[[2]int{key[0] + dx, key[1] + dy}] {
					rt := refTris[ri]
					d1 := rt.R1 - tt.R1
					d2 := rt.R2 - tt.R2
					if d1*d1+d2*d2 > descTol*descTol {
						continue
					}
					// Chirality must agree for a proper similarity.
					refChir := chirality(refStars, rt.A, rt.B, rt.C)
					tgtChir := chirality(tgtStars, tt.A, tt.B, tt.C)
					if (refChir > 0) != (tgtChir > 0) {
						continue
					}
					out = append(out, triangleMatch{
						Ref: [3]int{rt.A, rt.B, rt.C},
						Tgt: [3]int{tt.A, tt.B, tt.C},
					})
				}
			}
		}
	}
	return out
}

// countInliers returns the number of target stars whose transformed position
// lies within tol of a reference star. Uses a simple linear scan; fine for
// ~100 stars.
func countInliers(sim similarity, tgtStars, refStars []star, tol float64) int {
	tol2 := tol * tol
	n := 0
	for _, ts := range tgtStars {
		p := sim.apply(point2D{ts.X, ts.Y})
		for _, rs := range refStars {
			dx := p.X - rs.X
			dy := p.Y - rs.Y
			if dx*dx+dy*dy <= tol2 {
				n++
				break
			}
		}
	}
	return n
}

// collectInliers returns matched (src, dst) point pairs under sim within tol.
func collectInliers(sim similarity, tgtStars, refStars []star, tol float64) ([]point2D, []point2D) {
	tol2 := tol * tol
	var src, dst []point2D
	used := make([]bool, len(refStars))
	for _, ts := range tgtStars {
		p := sim.apply(point2D{ts.X, ts.Y})
		bestJ, bestD := -1, tol2
		for j, rs := range refStars {
			if used[j] {
				continue
			}
			dx := p.X - rs.X
			dy := p.Y - rs.Y
			d := dx*dx + dy*dy
			if d <= bestD {
				bestJ = j
				bestD = d
			}
		}
		if bestJ >= 0 {
			used[bestJ] = true
			src = append(src, point2D{ts.X, ts.Y})
			dst = append(dst, point2D{refStars[bestJ].X, refStars[bestJ].Y})
		}
	}
	return src, dst
}

// matchStarsToReference returns the similarity that maps target stars onto
// reference stars, plus the inlier count. Returns ok=false if the match is
// not confident enough.
func matchStarsToReference(refStars, tgtStars []star) (similarity, int, bool) {
	const tol = 3.0
	matches := matchTriangles(refStars, tgtStars)
	if len(matches) == 0 {
		return identitySimilarity, 0, false
	}
	bestInliers := 0
	bestSim := identitySimilarity
	for _, m := range matches {
		src := []point2D{
			{tgtStars[m.Tgt[0]].X, tgtStars[m.Tgt[0]].Y},
			{tgtStars[m.Tgt[1]].X, tgtStars[m.Tgt[1]].Y},
			{tgtStars[m.Tgt[2]].X, tgtStars[m.Tgt[2]].Y},
		}
		dst := []point2D{
			{refStars[m.Ref[0]].X, refStars[m.Ref[0]].Y},
			{refStars[m.Ref[1]].X, refStars[m.Ref[1]].Y},
			{refStars[m.Ref[2]].X, refStars[m.Ref[2]].Y},
		}
		sim := estimateSimilarity(src, dst)
		if sim.scale() < 0.5 || sim.scale() > 2.0 {
			continue
		}
		ni := countInliers(sim, tgtStars, refStars, tol)
		if ni > bestInliers {
			bestInliers = ni
			bestSim = sim
		}
	}
	if bestInliers < 4 {
		return identitySimilarity, bestInliers, false
	}
	// Refine with all inliers
	src, dst := collectInliers(bestSim, tgtStars, refStars, tol)
	if len(src) >= 2 {
		bestSim = estimateSimilarity(src, dst)
		bestInliers = len(src)
	}
	return bestSim, bestInliers, true
}

// =========================================================================
// Resampling and cropping
// =========================================================================

// sampleBilinear samples f at sub-pixel (x, y). Returns ok=false if (x, y)
// lies outside the source bounds.
func sampleBilinear(f *frame, x, y float64) (r, g, b float32, ok bool) {
	if x < 0 || x > float64(f.Width-1) || y < 0 || y > float64(f.Height-1) {
		return 0, 0, 0, false
	}
	x0 := int(x)
	y0 := int(y)
	fx := float32(x - float64(x0))
	fy := float32(y - float64(y0))
	x1 := x0 + 1
	if x1 >= f.Width {
		x1 = f.Width - 1
	}
	y1 := y0 + 1
	if y1 >= f.Height {
		y1 = f.Height - 1
	}
	i00 := (y0*f.Width + x0) * 3
	i10 := (y0*f.Width + x1) * 3
	i01 := (y1*f.Width + x0) * 3
	i11 := (y1*f.Width + x1) * 3
	w00 := (1 - fx) * (1 - fy)
	w10 := fx * (1 - fy)
	w01 := (1 - fx) * fy
	w11 := fx * fy
	r = w00*f.Pix[i00] + w10*f.Pix[i10] + w01*f.Pix[i01] + w11*f.Pix[i11]
	g = w00*f.Pix[i00+1] + w10*f.Pix[i10+1] + w01*f.Pix[i01+1] + w11*f.Pix[i11+1]
	b = w00*f.Pix[i00+2] + w10*f.Pix[i10+2] + w01*f.Pix[i01+2] + w11*f.Pix[i11+2]
	return r, g, b, true
}

// resampleToReference produces a refW×refH frame in the reference coordinate
// system. Pixels with no source are marked -1.
func resampleToReference(f *frame, sim similarity, refW, refH, workers int) *frame {
	out := &frame{Width: refW, Height: refH, Pix: make([]float32, refW*refH*3)}
	inv := sim.inverse()
	rowBand := func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			fy := float64(y)
			rowBase := y * refW * 3
			for x := 0; x < refW; x++ {
				fx := float64(x)
				sx := inv.SC*fx - inv.SS*fy + inv.Tx
				sy := inv.SS*fx + inv.SC*fy + inv.Ty
				i := rowBase + x*3
				r, g, b, ok := sampleBilinear(f, sx, sy)
				if ok {
					out.Pix[i] = r
					out.Pix[i+1] = g
					out.Pix[i+2] = b
				} else {
					out.Pix[i] = -1
					out.Pix[i+1] = -1
					out.Pix[i+2] = -1
				}
			}
		}
	}
	if workers <= 1 || refH < workers {
		rowBand(0, refH)
		return out
	}
	var wg sync.WaitGroup
	rowsPerWorker := (refH + workers - 1) / workers
	for w := 0; w < workers; w++ {
		y0 := w * rowsPerWorker
		if y0 >= refH {
			break
		}
		y1 := y0 + rowsPerWorker
		if y1 > refH {
			y1 = refH
		}
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			rowBand(y0, y1)
		}(y0, y1)
	}
	wg.Wait()
	return out
}

// findSafeCrop returns the largest axis-aligned rectangle in which every
// pixel has validCount == requiredCount. Uses maximal-rectangle-in-histogram.
func findSafeCrop(validCount []int32, w, h int, requiredCount int32) image.Rectangle {
	heights := make([]int, w)
	bestArea := 0
	best := image.Rect(0, 0, 0, 0)
	for y := 0; y < h; y++ {
		rowBase := y * w
		for x := 0; x < w; x++ {
			if validCount[rowBase+x] == requiredCount {
				heights[x]++
			} else {
				heights[x] = 0
			}
		}
		stack := []int{-1}
		for x := 0; x <= w; x++ {
			hx := 0
			if x < w {
				hx = heights[x]
			}
			for len(stack) > 1 && heights[stack[len(stack)-1]] > hx {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				left := stack[len(stack)-1] + 1
				hgt := heights[top]
				area := hgt * (x - left)
				if area > bestArea {
					bestArea = area
					best = image.Rect(left, y-hgt+1, x, y+1)
				}
			}
			stack = append(stack, x)
		}
	}
	if bestArea == 0 {
		return image.Rect(0, 0, w, h)
	}
	return best
}

// =========================================================================
// Accumulator
// =========================================================================

type accumulator struct {
	Width, Height int
	Pix           []float32
	Count         int
}

func newAccumulator(w, h int) *accumulator {
	return &accumulator{Width: w, Height: h, Pix: make([]float32, w*h*3)}
}

func (a *accumulator) addFrame(f *frame, workers int) error {
	if f.Width != a.Width || f.Height != a.Height {
		return fmt.Errorf("frame dimensions %dx%d do not match accumulator %dx%d",
			f.Width, f.Height, a.Width, a.Height)
	}
	if len(f.Pix) != len(a.Pix) {
		return fmt.Errorf("pixel buffer length mismatch")
	}
	addRows := func(start, end int) {
		for i := start; i < end; i++ {
			a.Pix[i] += f.Pix[i]
		}
	}
	if workers <= 1 || a.Height < workers {
		addRows(0, len(a.Pix))
	} else {
		var wg sync.WaitGroup
		rowsPerWorker := (a.Height + workers - 1) / workers
		for w := 0; w < workers; w++ {
			y0 := w * rowsPerWorker
			if y0 >= a.Height {
				break
			}
			y1 := y0 + rowsPerWorker
			if y1 > a.Height {
				y1 = a.Height
			}
			wg.Add(1)
			go func(y0, y1 int) {
				defer wg.Done()
				addRows(y0*a.Width*3, y1*a.Width*3)
			}(y0, y1)
		}
		wg.Wait()
	}
	a.Count++
	return nil
}

// addFrameMasked skips pixels marked -1 and increments validCount per pixel.
func (a *accumulator) addFrameMasked(f *frame, workers int, validCount []int32) error {
	if f.Width != a.Width || f.Height != a.Height {
		return fmt.Errorf("frame dimensions mismatch")
	}
	addRows := func(startPix, endPix int) {
		for p := startPix; p < endPix; p++ {
			r := f.Pix[p*3]
			g := f.Pix[p*3+1]
			b := f.Pix[p*3+2]
			if r >= 0 && g >= 0 && b >= 0 {
				a.Pix[p*3] += r
				a.Pix[p*3+1] += g
				a.Pix[p*3+2] += b
				validCount[p]++
			}
		}
	}
	nPix := a.Width * a.Height
	if workers <= 1 || a.Height < workers {
		addRows(0, nPix)
	} else {
		var wg sync.WaitGroup
		rowsPerWorker := (a.Height + workers - 1) / workers
		for w := 0; w < workers; w++ {
			y0 := w * rowsPerWorker
			if y0 >= a.Height {
				break
			}
			y1 := y0 + rowsPerWorker
			if y1 > a.Height {
				y1 = a.Height
			}
			wg.Add(1)
			go func(y0, y1 int) {
				defer wg.Done()
				addRows(y0*a.Width, y1*a.Width)
			}(y0, y1)
		}
		wg.Wait()
	}
	a.Count++
	return nil
}

func (a *accumulator) divide() {
	if a.Count == 0 {
		return
	}
	inv := 1.0 / float32(a.Count)
	for i := range a.Pix {
		a.Pix[i] *= inv
	}
}

func (a *accumulator) divideByCount(counts []int32) {
	for i := 0; i < len(a.Pix); i++ {
		c := counts[i/3]
		if c > 0 {
			a.Pix[i] /= float32(c)
		} else {
			a.Pix[i] = 0
		}
	}
}

func (a *accumulator) maxValue() float32 {
	m := float32(0)
	for _, v := range a.Pix {
		if v > m {
			m = v
		}
	}
	return m
}

func cropAccumulator(a *accumulator, r image.Rectangle) *accumulator {
	w, h := r.Dx(), r.Dy()
	out := newAccumulator(w, h)
	for y := 0; y < h; y++ {
		srcOff := ((r.Min.Y+y)*a.Width + r.Min.X) * 3
		dstOff := y * w * 3
		copy(out.Pix[dstOff:dstOff+w*3], a.Pix[srcOff:srcOff+w*3])
	}
	return out
}

func cropCounts(c []int32, w int, r image.Rectangle) []int32 {
	cw, ch := r.Dx(), r.Dy()
	out := make([]int32, cw*ch)
	for y := 0; y < ch; y++ {
		srcOff := (r.Min.Y+y)*w + r.Min.X
		dstOff := y * cw
		copy(out[dstOff:dstOff+cw], c[srcOff:srcOff+cw])
	}
	return out
}

// =========================================================================
// Output — TIFF
// =========================================================================

const (
	tagImageWidth        = 256
	tagImageLength       = 257
	tagBitsPerSample     = 258
	tagCompression       = 259
	tagPhotometricInterp = 262
	tagStripOffsets      = 273
	tagSamplesPerPixel   = 277
	tagRowsPerStrip      = 278
	tagStripByteCounts   = 279
	tagXResolution       = 282
	tagYResolution       = 283
	tagPlanarConfig      = 284
	tagResolutionUnit    = 296
	tagSampleFormat      = 339

	typeShort    = 3
	typeLong     = 4
	typeRational = 5

	compressionNone    = 1
	photometricRGB     = 2
	planarChunky       = 1
	sampleFormatFloat  = 3
	resolutionUnitNone = 1

	ifdEntryCount = 14
)

func writeFloat32RGB(w io.Writer, width, height int, pix []float32) error {
	return writeFloat32RGBScaled(w, width, height, pix, 1.0)
}

func writeFloat32RGBScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("tiff: invalid dimensions %dx%d", width, height)
	}
	n := width * height * 3
	if len(pix) != n {
		return fmt.Errorf("tiff: pixel buffer length %d does not match %dx%dx3 = %d", len(pix), width, height, n)
	}
	if divisor == 0 {
		return errors.New("tiff: divisor must be non-zero")
	}
	stripBytes64 := int64(n) * 4
	if stripBytes64 > math.MaxUint32 {
		return fmt.Errorf("tiff: image too large for single strip (%d bytes)", stripBytes64)
	}
	stripBytes := uint32(stripBytes64)

	ifdSize := uint32(2 + ifdEntryCount*12 + 4)
	ifdOffset := uint32(8)
	auxOffset := ifdOffset + ifdSize
	bitsOffset := auxOffset
	sfOffset := bitsOffset + 6
	xResOffset := sfOffset + 6
	yResOffset := xResOffset + 8
	auxEnd := yResOffset + 8
	pixelOffset := auxEnd
	if rem := pixelOffset % 4; rem != 0 {
		pixelOffset += 4 - rem
	}

	var header [8]byte
	header[0], header[1] = 'I', 'I'
	binary.LittleEndian.PutUint16(header[2:], 42)
	binary.LittleEndian.PutUint32(header[4:], ifdOffset)
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	ifd := make([]byte, ifdSize)
	binary.LittleEndian.PutUint16(ifd[0:], ifdEntryCount)
	pos := 2
	putShort := func(tag, v uint16) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typeShort)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint16(ifd[pos+8:], v)
		pos += 12
	}
	putLong := func(tag uint16, v uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typeLong)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint32(ifd[pos+8:], v)
		pos += 12
	}
	putOffset := func(tag, typ uint16, count, offset uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typ)
		binary.LittleEndian.PutUint32(ifd[pos+4:], count)
		binary.LittleEndian.PutUint32(ifd[pos+8:], offset)
		pos += 12
	}
	putLong(tagImageWidth, uint32(width))
	putLong(tagImageLength, uint32(height))
	putOffset(tagBitsPerSample, typeShort, 3, bitsOffset)
	putShort(tagCompression, compressionNone)
	putShort(tagPhotometricInterp, photometricRGB)
	putLong(tagStripOffsets, pixelOffset)
	putShort(tagSamplesPerPixel, 3)
	putLong(tagRowsPerStrip, uint32(height))
	putLong(tagStripByteCounts, stripBytes)
	putOffset(tagXResolution, typeRational, 1, xResOffset)
	putOffset(tagYResolution, typeRational, 1, yResOffset)
	putShort(tagPlanarConfig, planarChunky)
	putShort(tagResolutionUnit, resolutionUnitNone)
	putOffset(tagSampleFormat, typeShort, 3, sfOffset)
	binary.LittleEndian.PutUint32(ifd[pos:], 0)
	if _, err := w.Write(ifd); err != nil {
		return err
	}

	aux := make([]byte, int(auxEnd-auxOffset))
	bo := int(bitsOffset - auxOffset)
	sfo := int(sfOffset - auxOffset)
	xro := int(xResOffset - auxOffset)
	yro := int(yResOffset - auxOffset)
	binary.LittleEndian.PutUint16(aux[bo:], 32)
	binary.LittleEndian.PutUint16(aux[bo+2:], 32)
	binary.LittleEndian.PutUint16(aux[bo+4:], 32)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint16(aux[sfo+i*2:], sampleFormatFloat)
	}
	binary.LittleEndian.PutUint32(aux[xro:], 1)
	binary.LittleEndian.PutUint32(aux[xro+4:], 1)
	binary.LittleEndian.PutUint32(aux[yro:], 1)
	binary.LittleEndian.PutUint32(aux[yro+4:], 1)
	if _, err := w.Write(aux); err != nil {
		return err
	}
	if pad := int(pixelOffset) - int(auxEnd); pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}

	const bufFloats = 4096
	scratch := make([]byte, bufFloats*4)
	inv := float32(1.0) / divisor
	for i := 0; i < n; {
		count := n - i
		if count > bufFloats {
			count = bufFloats
		}
		for j := 0; j < count; j++ {
			binary.LittleEndian.PutUint32(scratch[j*4:], math.Float32bits(pix[i+j]*inv))
		}
		if _, err := w.Write(scratch[:count*4]); err != nil {
			return err
		}
		i += count
	}
	return nil
}

func readFloat32RGB(r io.Reader) (width, height int, pix []float32, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(data) < 8 || data[0] != 'I' || data[1] != 'I' {
		return 0, 0, nil, errors.New("tiff: not little-endian")
	}
	if v := binary.LittleEndian.Uint16(data[2:]); v != 42 {
		return 0, 0, nil, fmt.Errorf("tiff: version %d", v)
	}
	ifdOff := int(binary.LittleEndian.Uint32(data[4:]))
	if ifdOff+2 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD out of range")
	}
	nEntries := int(binary.LittleEndian.Uint16(data[ifdOff:]))
	if ifdOff+2+nEntries*12+4 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD truncated")
	}
	var (
		w, h     uint32
		bps, sf  []uint16
		spp      uint16
		photo    uint16
		planar   uint16
		stripOff []uint32
		stripLen []uint32
	)
	for i := 0; i < nEntries; i++ {
		off := ifdOff + 2 + i*12
		tag := binary.LittleEndian.Uint16(data[off:])
		typ := binary.LittleEndian.Uint16(data[off+2:])
		count := int(binary.LittleEndian.Uint32(data[off+4:]))
		valField := off + 8
		readShorts := func() ([]uint16, error) {
			if typ != typeShort {
				return nil, fmt.Errorf("tiff: tag %d type %d", tag, typ)
			}
			out := make([]uint16, count)
			if count*2 <= 4 {
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[valField+k*2:])
				}
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o+count*2 > len(data) {
					return nil, errors.New("tiff: offset out of range")
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[o+k*2:])
				}
			}
			return out, nil
		}
		readLongs := func() ([]uint32, error) {
			if typ != typeLong {
				return nil, fmt.Errorf("tiff: tag %d type %d", tag, typ)
			}
			out := make([]uint32, count)
			if count*4 <= 4 {
				out[0] = binary.LittleEndian.Uint32(data[valField:])
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o+count*4 > len(data) {
					return nil, errors.New("tiff: offset out of range")
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint32(data[o+k*4:])
				}
			}
			return out, nil
		}
		switch tag {
		case tagImageWidth:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			w = v[0]
		case tagImageLength:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			h = v[0]
		case tagBitsPerSample:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			bps = v
		case tagPhotometricInterp:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			photo = v[0]
		case tagStripOffsets:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripOff = v
		case tagSamplesPerPixel:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			spp = v[0]
		case tagStripByteCounts:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripLen = v
		case tagPlanarConfig:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			planar = v[0]
		case tagSampleFormat:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			sf = v
		}
	}
	if w == 0 || h == 0 || spp != 3 || photo != photometricRGB || planar != planarChunky {
		return 0, 0, nil, errors.New("tiff: unexpected header fields")
	}
	if len(bps) != 3 || bps[0] != 32 || len(sf) != 3 || sf[0] != sampleFormatFloat {
		return 0, 0, nil, errors.New("tiff: unexpected sample format")
	}
	if len(stripOff) != 1 || len(stripLen) != 1 {
		return 0, 0, nil, errors.New("tiff: expected single strip")
	}
	nPix := int(w) * int(h) * 3
	if int(stripLen[0]) != nPix*4 {
		return 0, 0, nil, fmt.Errorf("tiff: strip byte count %d, expected %d", stripLen[0], nPix*4)
	}
	off := int(stripOff[0])
	if off+nPix*4 > len(data) {
		return 0, 0, nil, errors.New("tiff: pixel data out of range")
	}
	pix = make([]float32, nPix)
	for i := range pix {
		pix[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*4:]))
	}
	return int(w), int(h), pix, nil
}

// =========================================================================
// Output — preview JPEG
// =========================================================================

const previewGamma = 0.5
const previewLUTSize = 4096

var previewLUT = buildPreviewLUT()

func buildPreviewLUT() [previewLUTSize + 1]uint8 {
	var lut [previewLUTSize + 1]uint8
	for i := 0; i <= previewLUTSize; i++ {
		v := float64(i) / previewLUTSize
		s := math.Pow(v, previewGamma)
		x := int(s*255 + 0.5)
		if x < 0 {
			x = 0
		} else if x > 255 {
			x = 255
		}
		lut[i] = uint8(x)
	}
	return lut
}

func writePreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	return writePreviewJPEGScaled(w, width, height, pix, 1.0)
}

func writePreviewJPEGScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if divisor == 0 {
		divisor = 1.0
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	inv := float32(1.0) / divisor
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = previewLUT[stretchIndex(pix[i+0]*inv)]
			img.Pix[o+1] = previewLUT[stretchIndex(pix[i+1]*inv)]
			img.Pix[o+2] = previewLUT[stretchIndex(pix[i+2]*inv)]
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

func writeAutoNormalizedPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	maxVal := float32(0)
	for _, v := range pix {
		if v > maxVal {
			maxVal = v
		}
	}
	if math.IsNaN(float64(maxVal)) || math.IsInf(float64(maxVal), 0) || maxVal <= 0 {
		maxVal = 1.0
	}
	return writePreviewJPEGScaled(w, width, height, pix, maxVal)
}

func stretchIndex(v float32) int {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return previewLUTSize
	}
	return int(v * previewLUTSize)
}

// =========================================================================
// Report
// =========================================================================

type rejection struct {
	Path   string
	Reason string
}

type processingReport struct {
	Version            string
	Timestamp          time.Time
	InputDir           string
	OutputDir          string
	Discovered         int
	Accepted           []string
	Rejected           []rejection
	Width              int
	Height             int
	Channels           int
	Method             string
	AccumulatorType    string
	OutputFormat       string
	OutputScaling      string
	RegistrationOn     bool
	RegisteredCount    int
	RegistrationFails  []string
	CropRect           image.Rectangle
	SumNormalization   float32
	PostProcessApplied bool
	BGDegree           int
	StretchFactor      float64
	SharpenApplied     bool
	SharpenAmount      float64
	SharpenRadius      float64
	OutputFiles        []string
	Workers            int
	Duration           time.Duration
	Notes              []string
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

func (r *processingReport) writeTo(w io.Writer) error {
	ew := &errWriter{w: w}
	ew.printf("SkyStacker %s\n", r.Version)
	ew.printf("Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))
	ew.printf("Input directory:  %s\n", r.InputDir)
	ew.printf("Output directory: %s\n\n", r.OutputDir)
	ew.printf("Discovered files: %d\n", r.Discovered)
	ew.printf("Accepted:         %d\n", len(r.Accepted))
	ew.printf("Rejected:         %d\n\n", len(r.Rejected))
	ew.printf("Image dimensions: %d x %d\n", r.Width, r.Height)
	ew.printf("Channels:         %d\n\n", r.Channels)
	ew.printf("Method:           %s\n", r.Method)
	ew.printf("Accumulator:      %s\n", r.AccumulatorType)
	ew.printf("Output format:    %s\n", r.OutputFormat)
	ew.printf("Output scaling:   %s\n", r.OutputScaling)
	ew.printf("Workers:          %d\n", r.Workers)
	ew.printf("Duration:         %s\n\n", r.Duration.Round(time.Millisecond))

	ew.printf("Registration:        %v\n", r.RegistrationOn)
	ew.printf("Registered frames:   %d / %d\n", r.RegisteredCount, len(r.Accepted))
	if r.RegistrationOn && r.RegisteredCount > 0 {
		ew.printf("Crop rectangle:      x=%d..%d, y=%d..%d (%dx%d)\n",
			r.CropRect.Min.X, r.CropRect.Max.X-1,
			r.CropRect.Min.Y, r.CropRect.Max.Y-1,
			r.CropRect.Dx(), r.CropRect.Dy())
	}
	if len(r.RegistrationFails) > 0 {
		ew.printf("Registration failures:\n")
		for _, f := range r.RegistrationFails {
			ew.printf("  - %s\n", f)
		}
	}
	ew.printf("\n")

	ew.printf("Sum TIFF normalisation: %g\n", r.SumNormalization)
	if r.PostProcessApplied {
		ew.printf("Post-processing:    ON\n")
		ew.printf("  Background removal: polynomial degree %d\n", r.BGDegree)
		ew.printf("  Contrast stretch:   asinh f=%g\n", r.StretchFactor)
		ew.printf("  Sharpening:         %v (amount=%g, radius=%g)\n",
			r.SharpenApplied, r.SharpenAmount, r.SharpenRadius)
	} else {
		ew.printf("Post-processing:    OFF\n")
	}
	ew.printf("\n")

	ew.printf("Output files:\n")
	for _, f := range r.OutputFiles {
		ew.printf("  %s\n", f)
	}
	ew.printf("\n")

	ew.printf("Accepted files:\n")
	for _, p := range r.Accepted {
		ew.printf("  %s\n", p)
	}
	ew.printf("\n")
	if len(r.Rejected) > 0 {
		ew.printf("Rejected files:\n")
		for _, rj := range r.Rejected {
			ew.printf("  %s: %s\n", rj.Path, rj.Reason)
		}
		ew.printf("\n")
	}
	ew.printf("Notes:\n")
	for _, n := range r.Notes {
		ew.printf("  - %s\n", n)
	}
	return ew.err
}

// =========================================================================
// Atomic file write
// =========================================================================

func writeFileAtomic(path string, fn func(io.Writer) error) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	if err := fn(bw); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// =========================================================================
// Pipeline
// =========================================================================

type runOptions struct {
	InputDir      string
	OutputDir     string
	Workers       int
	Recursive     bool
	Overwrite     bool
	Register      bool
	PostProcess   bool
	StackOnly     bool
	PostOnly      bool
	NoColorCalib  bool
	NoSharpen     bool
	BGDegree      int
	StretchFactor float64
	SharpenAmount float64
	SharpenRadius float64
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

type frameTransform struct {
	sim   similarity
	valid bool
}

func runPipeline(ctx context.Context, opts runOptions) error {
	start := time.Now()

	inputDir, err := filepath.Abs(opts.InputDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(inputDir)
	if err != nil {
		return fmt.Errorf("input directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", inputDir)
	}

	var outputDir string
	if opts.OutputDir == "" {
		outputDir = filepath.Join(inputDir, "skystacker_results")
	} else {
		outputDir, err = filepath.Abs(opts.OutputDir)
		if err != nil {
			return err
		}
	}
	if filepath.Clean(outputDir) == filepath.Clean(inputDir) {
		return fmt.Errorf("output directory must differ from input directory")
	}
	if opts.PostOnly {
		if _, err := os.Stat(outputDir); err != nil {
			return fmt.Errorf("--post-only: output directory does not exist: %w", err)
		}
		meanPath := filepath.Join(outputDir, "stacked_mean.tiff")
		if _, err := os.Stat(meanPath); err != nil {
			return fmt.Errorf("--post-only: %s not found: %w", meanPath, err)
		}
		logf("Post-only mode: reading %s", meanPath)
		postOutputs, err := runPostProcessing(outputDir, opts)
		if err != nil {
			return fmt.Errorf("post-processing: %w", err)
		}
		logf("Post-processing complete: %d files", len(postOutputs))
		return nil
	}

	if info, err := os.Stat(outputDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output path exists and is not a directory: %s", outputDir)
		}
		if !opts.Overwrite {
			return fmt.Errorf("output directory already exists: %s (use --overwrite to replace)", outputDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("output directory: %w", err)
	}

	files, err := discoverFiles(inputDir, opts.Recursive, []string{outputDir})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no JPG/JPEG images found in %s", inputDir)
	}

	type candidate struct{ path string }
	var (
		candidates []candidate
		rejected   []rejection
		refW, refH int
	)
	for _, path := range files {
		cfg, err := decodeImageConfig(path)
		if err != nil {
			rejected = append(rejected, rejection{Path: path, Reason: err.Error()})
			continue
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			rejected = append(rejected, rejection{Path: path, Reason: fmt.Sprintf("invalid dimensions %dx%d", cfg.Width, cfg.Height)})
			continue
		}
		if refW == 0 {
			refW, refH = cfg.Width, cfg.Height
		} else if cfg.Width != refW || cfg.Height != refH {
			rejected = append(rejected, rejection{
				Path:   path,
				Reason: fmt.Sprintf("dimensions %dx%d differ from reference %dx%d", cfg.Width, cfg.Height, refW, refH),
			})
			continue
		}
		candidates = append(candidates, candidate{path: path})
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no valid images (all %d candidates rejected)", len(files))
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(candidates) {
		workers = len(candidates)
	}
	if workers < 1 {
		workers = 1
	}

	logf("Stacking %d frame(s) at %dx%d using up to %d worker(s)",
		len(candidates), refW, refH, workers)
	if opts.Register {
		logf("Registration: ON (use --no-register to disable)")
	} else {
		logf("Registration: OFF")
	}

	// ============== Pass 1: decode, detect stars, accumulate unregistered
	unregAcc := newAccumulator(refW, refH)
	starLists := make([][]star, len(candidates))
	var accepted []candidate
	for i, c := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		logf("  [pass1 %d/%d] %s", i+1, len(candidates), filepath.Base(c.path))
		f, err := decodeImageFile(c.path)
		if err != nil {
			rejected = append(rejected, rejection{
				Path:   c.path,
				Reason: "decode failed after validation: " + err.Error(),
			})
			continue
		}
		if opts.Register {
			starLists[len(accepted)] = detectStars(f, 100)
		}
		if err := unregAcc.addFrame(f, workers); err != nil {
			return fmt.Errorf("accumulate %s: %w", c.path, err)
		}
		accepted = append(accepted, c)
	}
	if unregAcc.Count == 0 {
		return fmt.Errorf("no frames accumulated")
	}
	starLists = starLists[:len(accepted)]

	// ============== Registration: pick reference, compute transforms
	transforms := make([]frameTransform, len(accepted))
	for i := range transforms {
		transforms[i] = frameTransform{sim: identitySimilarity, valid: true}
	}
	var registeredCount int
	var regFailures []string
	registrationOn := opts.Register

	if registrationOn {
		// Reference = frame with the most stars.
		refIdx := 0
		for i := 1; i < len(starLists); i++ {
			if len(starLists[i]) > len(starLists[refIdx]) {
				refIdx = i
			}
		}
		refStars := starLists[refIdx]
		logf("Reference frame: %s (%d stars)", filepath.Base(accepted[refIdx].path), len(refStars))

		if len(refStars) < 5 {
			logf("WARNING: reference frame has only %d stars; disabling registration", len(refStars))
			registrationOn = false
		} else {
			registeredCount = 1
			for i := range accepted {
				if i == refIdx {
					continue
				}
				if len(starLists[i]) < 3 {
					regFailures = append(regFailures, fmt.Sprintf("%s: only %d stars detected",
						filepath.Base(accepted[i].path), len(starLists[i])))
					transforms[i].valid = false
					continue
				}
				sim, inliers, ok := matchStarsToReference(refStars, starLists[i])
				if !ok {
					regFailures = append(regFailures, fmt.Sprintf("%s: no consistent transform (%d inliers)",
						filepath.Base(accepted[i].path), inliers))
					transforms[i].valid = false
					continue
				}
				scale := sim.scale()
				rotDeg := sim.rotation() * 180 / math.Pi
				logf("  register %s: scale=%.4f rot=%.2f° inliers=%d",
					filepath.Base(accepted[i].path), scale, rotDeg, inliers)
				transforms[i] = frameTransform{sim: sim, valid: true}
				registeredCount++
			}
			if registeredCount < 2 {
				logf("WARNING: only %d frame(s) registered; disabling registration", registeredCount)
				registrationOn = false
			}
		}
	}
	if !registrationOn {
		registeredCount = 0
		for i := range transforms {
			transforms[i] = frameTransform{sim: identitySimilarity, valid: true}
		}
	}

	// ============== Pass 2: resample + accumulate registered
	var regAcc *accumulator
	var validCount []int32
	var cropRect image.Rectangle
	if registrationOn {
		logf("Resampling %d registered frame(s)", registeredCount)
		regAcc = newAccumulator(refW, refH)
		validCount = make([]int32, refW*refH)
		for i, c := range accepted {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !transforms[i].valid {
				continue
			}
			logf("  [pass2 %d/%d] %s", i+1, len(accepted), filepath.Base(c.path))
			f, err := decodeImageFile(c.path)
			if err != nil {
				// Unlikely (already decoded) but be safe.
				return fmt.Errorf("re-decode %s: %w", c.path, err)
			}
			resampled := resampleToReference(f, transforms[i].sim, refW, refH, workers)
			if err := regAcc.addFrameMasked(resampled, workers, validCount); err != nil {
				return fmt.Errorf("accumulate %s: %w", c.path, err)
			}
		}
		cropRect = findSafeCrop(validCount, refW, refH, int32(registeredCount))
		logf("Crop: x=%d..%d, y=%d..%d (%dx%d)", cropRect.Min.X, cropRect.Max.X-1,
			cropRect.Min.Y, cropRect.Max.Y-1, cropRect.Dx(), cropRect.Dy())
	}

	// ============== Write outputs
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	sumTiff := filepath.Join(outputDir, "stacked_sum.tiff")
	sumJPG := filepath.Join(outputDir, "stacked_sum.jpg")
	meanTiff := filepath.Join(outputDir, "stacked_mean.tiff")
	meanJPG := filepath.Join(outputDir, "stacked_mean.jpg")
	noCorrTiff := filepath.Join(outputDir, "stacked_no_correction_mean.tiff")
	noCorrJPG := filepath.Join(outputDir, "stacked_no_correction_mean.jpg")
	previewJPG := filepath.Join(outputDir, "stacked_preview.jpg")
	reportTxt := filepath.Join(outputDir, "report.txt")

	var sumMax float32
	var outW, outH int

	if registrationOn {
		// Registered output: crop then divide the crop.
		croppedSum := cropAccumulator(regAcc, cropRect)
		croppedCounts := cropCounts(validCount, refW, cropRect)
		outW, outH = cropRect.Dx(), cropRect.Dy()

		sumMax = croppedSum.maxValue()
		if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) {
			sumMax = 1.0
		}
		sumDivisor := float32(1.0)
		if sumMax > 1.0 {
			sumDivisor = sumMax
		}
		logf("Registered sum max = %g (normalising sum TIFF by %g)", sumMax, sumDivisor)

		logf("Writing registered sum TIFF")
		if err := writeFileAtomic(sumTiff, func(w io.Writer) error {
			return writeFloat32RGBScaled(w, outW, outH, croppedSum.Pix, sumDivisor)
		}); err != nil {
			return fmt.Errorf("write sum tiff: %w", err)
		}
		logf("Writing registered sum preview")
		if err := writeFileAtomic(sumJPG, func(w io.Writer) error {
			return writeAutoNormalizedPreviewJPEG(w, outW, outH, croppedSum.Pix)
		}); err != nil {
			return fmt.Errorf("write sum jpg: %w", err)
		}

		croppedSum.divideByCount(croppedCounts)
		logf("Writing registered mean TIFF")
		if err := writeFileAtomic(meanTiff, func(w io.Writer) error {
			return writeFloat32RGB(w, outW, outH, croppedSum.Pix)
		}); err != nil {
			return fmt.Errorf("write mean tiff: %w", err)
		}
		logf("Writing registered mean preview")
		if err := writeFileAtomic(meanJPG, func(w io.Writer) error {
			return writePreviewJPEG(w, outW, outH, croppedSum.Pix)
		}); err != nil {
			return fmt.Errorf("write mean jpg: %w", err)
		}
		if err := writeFileAtomic(previewJPG, func(w io.Writer) error {
			return writePreviewJPEG(w, outW, outH, croppedSum.Pix)
		}); err != nil {
			return fmt.Errorf("write preview: %w", err)
		}
	} else {
		// Unregistered output (v0.2 behaviour).
		outW, outH = refW, refH
		sumMax = unregAcc.maxValue()
		if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) {
			sumMax = 1.0
		}
		sumDivisor := float32(1.0)
		if sumMax > 1.0 {
			sumDivisor = sumMax
		}
		logf("Sum max = %g (normalising sum TIFF by %g)", sumMax, sumDivisor)

		logf("Writing sum TIFF")
		if err := writeFileAtomic(sumTiff, func(w io.Writer) error {
			return writeFloat32RGBScaled(w, outW, outH, unregAcc.Pix, sumDivisor)
		}); err != nil {
			return fmt.Errorf("write sum tiff: %w", err)
		}
		logf("Writing sum preview")
		if err := writeFileAtomic(sumJPG, func(w io.Writer) error {
			return writeAutoNormalizedPreviewJPEG(w, outW, outH, unregAcc.Pix)
		}); err != nil {
			return fmt.Errorf("write sum jpg: %w", err)
		}

		unregAcc.divide()
		logf("Writing mean TIFF")
		if err := writeFileAtomic(meanTiff, func(w io.Writer) error {
			return writeFloat32RGB(w, outW, outH, unregAcc.Pix)
		}); err != nil {
			return fmt.Errorf("write mean tiff: %w", err)
		}
		logf("Writing mean preview")
		if err := writeFileAtomic(meanJPG, func(w io.Writer) error {
			return writePreviewJPEG(w, outW, outH, unregAcc.Pix)
		}); err != nil {
			return fmt.Errorf("write mean jpg: %w", err)
		}
		if err := writeFileAtomic(previewJPG, func(w io.Writer) error {
			return writePreviewJPEG(w, outW, outH, unregAcc.Pix)
		}); err != nil {
			return fmt.Errorf("write preview: %w", err)
		}
	}

	// No-correction mean (always from unregistered accumulator)
	if registrationOn {
		unregAcc.divide()
	}
	logf("Writing no-correction mean TIFF")
	if err := writeFileAtomic(noCorrTiff, func(w io.Writer) error {
		return writeFloat32RGB(w, refW, refH, unregAcc.Pix)
	}); err != nil {
		return fmt.Errorf("write no_correction_mean tiff: %w", err)
	}
	logf("Writing no-correction mean preview")
	if err := writeFileAtomic(noCorrJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, unregAcc.Pix)
	}); err != nil {
		return fmt.Errorf("write no_correction_mean jpg: %w", err)
	}

	// Post-processing pipeline
	var postOutputs []string
	if opts.PostProcess && !opts.StackOnly {
		logf("Post-processing pipeline: bg-degree=%d stretch=%g colorCalib=%v sharpen=%v",
			opts.BGDegree, opts.StretchFactor, !opts.NoColorCalib, !opts.NoSharpen)
		outs, err := runPostProcessing(outputDir, opts)
		if err != nil {
			logf("WARNING: post-processing failed: %v", err)
		} else {
			postOutputs = outs
			logf("Post-processing complete: %d files", len(outs))
		}
	} else if opts.StackOnly {
		logf("Skipping post-processing (--stack-only)")
	}

	// Report
	notes := []string{
		"No calibration (dark, flat, bias) was performed.",
		"JPEG inputs are gamma-encoded (sRGB-like); samples were averaged in encoded space, not linear light.",
		"All JPG outputs are display previews, not scientific outputs.",
	}
	if registrationOn {
		notes = append(notes,
			fmt.Sprintf("Registration: %d of %d frames registered using a similarity model (rotation + uniform scale + translation).", registeredCount, len(accepted)),
			fmt.Sprintf("Registered outputs (stacked_sum/mean) are cropped to the largest fully-valid region: %dx%d.", outW, outH),
			"stacked_no_correction_mean is the unregistered full-frame mean.",
			fmt.Sprintf("Registered sum TIFF was divided by %g (global max of registered sum); multiply by %g to recover raw float32 sum.", sumMax, sumMax),
		)
		if len(regFailures) > 0 {
			notes = append(notes, fmt.Sprintf("%d frame(s) failed registration and were excluded from the registered stack; see 'Registration failures' above.", len(regFailures)))
		}
	} else {
		notes = append(notes,
			"Registration: disabled. Frames were combined in their existing pixel coordinate system.",
			"stacked_mean.tiff and stacked_no_correction_mean.tiff are identical in this run.",
			fmt.Sprintf("Sum TIFF was divided by %g (global max); multiply by %g to recover raw float32 sum.", sumMax, sumMax),
		)
	}

	acceptedPaths := make([]string, len(accepted))
	for i, c := range accepted {
		acceptedPaths[i] = c.path
	}

	rep := &processingReport{
		Version:           version,
		Timestamp:         start,
		InputDir:          inputDir,
		OutputDir:         outputDir,
		Discovered:        len(files),
		Accepted:          acceptedPaths,
		Rejected:          rejected,
		Width:             refW,
		Height:            refH,
		Channels:          3,
		Method:            methodString(registrationOn),
		AccumulatorType:   "float32 (interleaved RGB)",
		OutputFormat:      "TIFF: uncompressed, little-endian, RGB, IEEE float32, single strip. JPG: 8-bit, gamma 0.5 display preview.",
		OutputScaling:     scalingString(registrationOn),
		RegistrationOn:    registrationOn,
		RegisteredCount:   registeredCount,
		RegistrationFails: regFailures,
		CropRect:          cropRect,
		SumNormalization:  sumMax,
		OutputFiles: append([]string{
			sumTiff, sumJPG,
			meanTiff, meanJPG,
			noCorrTiff, noCorrJPG,
			previewJPG,
		}, append(postOutputs, reportTxt)...),
		Workers:            workers,
		Duration:           time.Since(start),
		PostProcessApplied: opts.PostProcess && !opts.StackOnly,
		BGDegree:           opts.BGDegree,
		StretchFactor:      opts.StretchFactor,
		SharpenApplied:     opts.PostProcess && !opts.StackOnly && !opts.NoSharpen,
		SharpenAmount:      opts.SharpenAmount,
		SharpenRadius:      opts.SharpenRadius,
		Notes:              notes,
	}
	if err := writeFileAtomic(reportTxt, func(w io.Writer) error {
		return rep.writeTo(w)
	}); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	logf("Done in %s -> %s", rep.Duration.Round(time.Millisecond), outputDir)
	return nil
}

func methodString(registered bool) string {
	if registered {
		return "sum and arithmetic mean over registered frames; no-correction mean over all frames"
	}
	return "sum and arithmetic mean (no registration)"
}

func scalingString(registered bool) string {
	if registered {
		return "registered sum.tiff divided by its global max (recoverable via reported factor); mean.tiff divided by per-pixel valid count; no_correction_mean.tiff divided by total frames. Preview JPGs use gamma 0.5."
	}
	return "sum.tiff divided by its global max (recoverable via reported factor); mean.tiff divided by total frames. Preview JPGs use gamma 0.5."
}

// =========================================================================
// Self-tests
// =========================================================================

type testCase struct {
	name string
	fn   func() error
}

func runSelfTests() int {
	cases := []testCase{
		{"accumulator sum and mean", testAccumulatorSumMean},
		{"accumulator overflow", testAccumulatorOverflow},
		{"accumulator dimension mismatch", testAccumulatorDimensionMismatch},
		{"accumulator parallel vs sequential", testAccumulatorParallel},
		{"accumulator max", testAccumulatorMax},
		{"accumulator masked", testAccumulatorMasked},
		{"TIFF round trip", testTIFFRoundTrip},
		{"TIFF large values", testTIFFLargeValues},
		{"TIFF scaled round trip", testTIFFScaledRoundTrip},
		{"TIFF bad input", testTIFFBadInput},
		{"decode RGBA", testDecodeRGBA},
		{"decode JPEG", testDecodeJPEG},
		{"discovery", testDiscovery},
		{"similarity estimate", testSimilarityEstimate},
		{"similarity inverse", testSimilarityInverse},
		{"star detection synthetic", testStarDetectionSynthetic},
		{"star registration synthetic", testStarRegistrationSynthetic},
		{"bilinear sampling", testBilinearSample},
		{"resample identity", testResampleIdentity},
		{"safe crop", testSafeCrop},
		{"postprocess polynomial basis", testPolyBasis},
		{"postprocess linear solve", testSolveLinear},
		{"postprocess background removal", testBackgroundRemoval},
		{"postprocess contrast stretch", testContrastStretch},
		{"postprocess color calibration", testColorCalibration},
		{"postprocess gaussian blur", testGaussianBlur},
		{"postprocess unsharp mask", testUnsharpMask},
		{"resample translation", testResampleTranslation},
		{"resample round trip", testResampleRoundTrip},
		{"registration with outliers", testRegistrationWithOutliers},
		{"star detection empty", testStarDetectionEmpty},
		{"star detection edge stars", testStarDetectionEdgeStars},
		{"parse args", testParseArgs},
		{"atomic write cleanup", testAtomicWriteCleanup},
		{"pipeline stack-only", testPipelineStackOnly},
		{"pipeline post-only", testPipelinePostOnly},
		{"report contains", testReportContains},
	}
	failures := 0
	for _, c := range cases {
		if err := c.fn(); err != nil {
			fmt.Printf("FAIL %s: %v\n", c.name, err)
			failures++
		} else {
			fmt.Printf("ok   %s\n", c.name)
		}
	}
	if failures > 0 {
		fmt.Printf("\n%d of %d self-tests failed\n", failures, len(cases))
	} else {
		fmt.Printf("\nall %d self-tests passed\n", len(cases))
	}
	return failures
}

func testAccumulatorSumMean() error {
	a := newAccumulator(2, 2)
	f1 := &frame{Width: 2, Height: 2, Pix: []float32{
		0.1, 0.2, 0.3, 0.4, 0.5, 0.6,
		0.7, 0.8, 0.9, 1.0, 0.0, 0.5,
	}}
	f2 := &frame{Width: 2, Height: 2, Pix: []float32{
		0.9, 0.8, 0.7, 0.6, 0.5, 0.4,
		0.3, 0.2, 0.1, 0.0, 1.0, 0.5,
	}}
	if err := a.addFrame(f1, 1); err != nil {
		return err
	}
	if err := a.addFrame(f2, 1); err != nil {
		return err
	}
	for i, v := range a.Pix {
		if math.Abs(float64(v-1.0)) > 1e-6 {
			return fmt.Errorf("sum[%d] = %v, want 1", i, v)
		}
	}
	a.divide()
	for i, v := range a.Pix {
		if math.Abs(float64(v-0.5)) > 1e-6 {
			return fmt.Errorf("mean[%d] = %v, want 0.5", i, v)
		}
	}
	return nil
}

func testAccumulatorOverflow() error {
	a := newAccumulator(1, 1)
	f := &frame{Width: 1, Height: 1, Pix: []float32{1, 1, 1}}
	for i := 0; i < 300; i++ {
		if err := a.addFrame(f, 1); err != nil {
			return err
		}
	}
	if a.Pix[0] != 300 {
		return fmt.Errorf("sum = %v, want 300", a.Pix[0])
	}
	a.divide()
	if math.Abs(float64(a.Pix[0]-1)) > 1e-6 {
		return fmt.Errorf("mean = %v, want 1", a.Pix[0])
	}
	return nil
}

func testAccumulatorDimensionMismatch() error {
	a := newAccumulator(2, 2)
	bad := &frame{Width: 3, Height: 2, Pix: make([]float32, 3*2*3)}
	if err := a.addFrame(bad, 1); err == nil {
		return errors.New("expected dimension-mismatch error")
	}
	return nil
}

func testAccumulatorParallel() error {
	a := newAccumulator(64, 32)
	f := &frame{Width: 64, Height: 32, Pix: make([]float32, 64*32*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i%97) / 97.0
	}
	if err := a.addFrame(f, 8); err != nil {
		return err
	}
	b := newAccumulator(64, 32)
	if err := b.addFrame(f, 1); err != nil {
		return err
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			return fmt.Errorf("mismatch at %d", i)
		}
	}
	return nil
}

func testAccumulatorMax() error {
	a := newAccumulator(2, 1)
	a.Pix = []float32{0.1, 0.5, 3.2, 1.1, 0.9, 2.7}
	if got := a.maxValue(); got != 3.2 {
		return fmt.Errorf("max = %v, want 3.2", got)
	}
	return nil
}

func testAccumulatorMasked() error {
	a := newAccumulator(2, 1)
	valid := make([]int32, 2)
	f1 := &frame{Width: 2, Height: 1, Pix: []float32{0.5, 0.6, 0.7, -1, -1, -1}}
	f2 := &frame{Width: 2, Height: 1, Pix: []float32{-1, -1, -1, 0.9, 0.8, 0.7}}
	if err := a.addFrameMasked(f1, 1, valid); err != nil {
		return err
	}
	if err := a.addFrameMasked(f2, 1, valid); err != nil {
		return err
	}
	if valid[0] != 1 || valid[1] != 1 {
		return fmt.Errorf("validCounts = %v, want [1 1]", valid)
	}
	a.divideByCount(valid)
	for i, v := range a.Pix {
		want := []float32{0.5, 0.6, 0.7, 0.9, 0.8, 0.7}[i]
		if math.Abs(float64(v-want)) > 1e-6 {
			return fmt.Errorf("masked[%d] = %v, want %v", i, v, want)
		}
	}
	return nil
}

func testTIFFRoundTrip() error {
	const w, h = 3, 2
	pix := []float32{
		0, 0.25, 0.5,
		1, 0.5, 0.75,
		0.125, 0.375, 0.625,
		0.1, 0.2, 0.3,
		0, 0, 1,
		0.99, 0.5, 0.01,
	}
	var buf bytes.Buffer
	if err := writeFloat32RGB(&buf, w, h, pix); err != nil {
		return err
	}
	gotW, gotH, gotPix, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	if gotW != w || gotH != h {
		return fmt.Errorf("dims")
	}
	for i := range pix {
		if gotPix[i] != pix[i] {
			return fmt.Errorf("pixel[%d]", i)
		}
	}
	return nil
}

func testTIFFLargeValues() error {
	var buf bytes.Buffer
	pix := []float32{1000000, 999999.5, 3.4e38}
	if err := writeFloat32RGB(&buf, 1, 1, pix); err != nil {
		return err
	}
	_, _, got, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	for i := range pix {
		if math.Abs(float64(got[i]-pix[i]))/math.Abs(float64(pix[i])) > 1e-6 {
			return fmt.Errorf("pixel[%d]", i)
		}
	}
	return nil
}

func testTIFFScaledRoundTrip() error {
	var buf bytes.Buffer
	pix := []float32{10, 20, 30, 40, 50, 60}
	if err := writeFloat32RGBScaled(&buf, 2, 1, pix, 10); err != nil {
		return err
	}
	_, _, got, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	want := []float32{1, 2, 3, 4, 5, 6}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1e-6 {
			return fmt.Errorf("scaled[%d]", i)
		}
	}
	return nil
}

func testTIFFBadInput() error {
	if err := writeFloat32RGB(&bytes.Buffer{}, 0, 0, nil); err == nil {
		return errors.New("expected error for zero dimensions")
	}
	if err := writeFloat32RGB(&bytes.Buffer{}, 2, 2, make([]float32, 11)); err == nil {
		return errors.New("expected error for bad buffer")
	}
	if err := writeFloat32RGBScaled(&bytes.Buffer{}, 2, 2, make([]float32, 12), 0); err == nil {
		return errors.New("expected error for zero divisor")
	}
	return nil
}

func testDecodeRGBA() error {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	f, err := frameFromImage(img)
	if err != nil {
		return err
	}
	checks := []struct {
		x, y, c int
		want    float32
	}{
		{0, 0, 0, 1}, {0, 0, 1, 0}, {0, 0, 2, 0},
		{1, 0, 0, 0}, {1, 0, 1, 1}, {1, 0, 2, 0},
	}
	for _, ck := range checks {
		got := f.Pix[(ck.y*f.Width+ck.x)*3+ck.c]
		if math.Abs(float64(got-ck.want)) > 1e-6 {
			return fmt.Errorf("pixel")
		}
	}
	return nil
}

func testDecodeJPEG() error {
	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			src.SetRGBA(x, y, color.RGBA{200, 100, 50, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 95}); err != nil {
		return err
	}
	img, _, err := image.Decode(&buf)
	if err != nil {
		return err
	}
	f, err := frameFromImage(img)
	if err != nil {
		return err
	}
	want := [3]float32{200.0 / 255.0, 100.0 / 255.0, 50.0 / 255.0}
	for i := 0; i < 4*3; i++ {
		for c := 0; c < 3; c++ {
			if math.Abs(float64(f.Pix[i*3+c]-want[c])) > 0.02 {
				return fmt.Errorf("pixel %d ch %d = %v, want ~%v", i, c, f.Pix[i*3+c], want[c])
			}
		}
	}
	return nil
}

func testDiscovery() error {
	dir, err := os.MkdirTemp("", "skystacker-discovery-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	must := func(p string) error {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte("x"), 0o644)
	}
	for _, p := range []string{
		filepath.Join(dir, "b.JPG"),
		filepath.Join(dir, "a.jpeg"),
		filepath.Join(dir, "c.png"),
		filepath.Join(dir, "sub", "d.jpg"),
	} {
		if err := must(p); err != nil {
			return err
		}
	}
	got, err := discoverFiles(dir, false, nil)
	if err != nil {
		return err
	}
	want := []string{filepath.Join(dir, "a.jpeg"), filepath.Join(dir, "b.JPG")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		return fmt.Errorf("discovery: got %v, want %v", got, want)
	}
	return nil
}

func testSimilarityEstimate() error {
	// 3 point pairs under known similarity
	src := []point2D{{0, 0}, {10, 0}, {0, 10}}
	dst := []point2D{{5, 3}, {15, 3}, {5, 13}} // pure translation (5, 3)
	sim := estimateSimilarity(src, dst)
	if math.Abs(sim.SC-1) > 1e-9 || math.Abs(sim.SS) > 1e-9 {
		return fmt.Errorf("SC=%v SS=%v, want 1, 0", sim.SC, sim.SS)
	}
	if math.Abs(sim.Tx-5) > 1e-9 || math.Abs(sim.Ty-3) > 1e-9 {
		return fmt.Errorf("T=(%v,%v), want (5,3)", sim.Tx, sim.Ty)
	}
	return nil
}

func testSimilarityInverse() error {
	// Rotation 90°, translation (1, 2), unit scale
	sim := similarity{SC: 0, SS: 1, Tx: 1, Ty: 2}
	inv := sim.inverse()
	p := point2D{3, 4}
	q := sim.apply(p)
	back := inv.apply(q)
	if math.Abs(back.X-p.X) > 1e-9 || math.Abs(back.Y-p.Y) > 1e-9 {
		return fmt.Errorf("inverse roundtrip: %v -> %v -> %v", p, q, back)
	}
	return nil
}

func testStarDetectionSynthetic() error {
	// Create a 100×100 frame with 5 Gaussian-like blobs.
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	stars := []point2D{{20, 20}, {50, 50}, {80, 30}, {30, 80}, {70, 70}}
	for _, s := range stars {
		for dy := -3; dy <= 3; dy++ {
			for dx := -3; dx <= 3; dx++ {
				x, y := int(s.X)+dx, int(s.Y)+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				d2 := float64(dx*dx + dy*dy)
				v := float32(0.9 * math.Exp(-d2/4))
				i := (y*w + x) * 3
				f.Pix[i+0] += v
				f.Pix[i+1] += v
				f.Pix[i+2] += v
			}
		}
	}
	detected := detectStars(f, 20)
	if len(detected) < 5 {
		return fmt.Errorf("detected %d stars, expected at least 5", len(detected))
	}
	// Each detected star should be close to one of the 5 known positions.
	for _, ds := range detected[:5] {
		best := 1e9
		for _, s := range stars {
			d := math.Hypot(ds.X-s.X, ds.Y-s.Y)
			if d < best {
				best = d
			}
		}
		if best > 2.0 {
			return fmt.Errorf("detected star at (%v,%v) is %v px from nearest true star", ds.X, ds.Y, best)
		}
	}
	return nil
}

func testStarRegistrationSynthetic() error {
	// Reference stars
	refStars := []star{
		{X: 100, Y: 100, Flux: 1000}, {X: 200, Y: 150, Flux: 800}, {X: 300, Y: 200, Flux: 600},
		{X: 150, Y: 300, Flux: 700}, {X: 250, Y: 350, Flux: 500}, {X: 400, Y: 250, Flux: 400},
		{X: 180, Y: 220, Flux: 300}, {X: 280, Y: 180, Flux: 250}, {X: 350, Y: 300, Flux: 200},
	}
	// Known T: rotation 2°, translation (5, 3), unit scale
	theta := 2.0 * math.Pi / 180
	T := similarity{
		SC: math.Cos(theta),
		SS: math.Sin(theta),
		Tx: 5,
		Ty: 3,
	}
	Tinv := T.inverse()
	var tgtStars []star
	for _, s := range refStars {
		p := Tinv.apply(point2D{s.X, s.Y})
		tgtStars = append(tgtStars, star{X: p.X, Y: p.Y, Flux: s.Flux})
	}
	recovered, inliers, ok := matchStarsToReference(refStars, tgtStars)
	if !ok {
		return errors.New("matchStarsToReference returned ok=false")
	}
	if inliers < len(refStars)-1 {
		return fmt.Errorf("inliers = %d, want ~%d", inliers, len(refStars))
	}
	for i := range tgtStars {
		expected := point2D{refStars[i].X, refStars[i].Y}
		actual := recovered.apply(point2D{tgtStars[i].X, tgtStars[i].Y})
		if math.Hypot(actual.X-expected.X, actual.Y-expected.Y) > 1.0 {
			return fmt.Errorf("star %d: recovered (%v) != expected (%v)", i, actual, expected)
		}
	}
	return nil
}

func testBilinearSample() error {
	f := &frame{Width: 2, Height: 2, Pix: []float32{
		0, 0, 0, 1, 0, 0, // (0,0)=(0,0,0), (1,0)=(1,0,0)
		0, 1, 0, 1, 1, 0, // (0,1)=(0,1,0), (1,1)=(1,1,0)
	}}
	// At (0.5, 0.5) bilinear = average of 4 = (0.5, 0.5, 0)
	r, g, b, ok := sampleBilinear(f, 0.5, 0.5)
	if !ok {
		return errors.New("sampleBilinear(0.5, 0.5) returned ok=false")
	}
	if math.Abs(float64(r)-0.5) > 1e-6 || math.Abs(float64(g)-0.5) > 1e-6 || math.Abs(float64(b)) > 1e-6 {
		return fmt.Errorf("(0.5,0.5) = (%v,%v,%v), want (0.5,0.5,0)", r, g, b)
	}
	// Out of bounds
	if _, _, _, ok := sampleBilinear(f, -0.1, 0); ok {
		return errors.New("expected out-of-bounds")
	}
	return nil
}

func testResampleIdentity() error {
	f := &frame{Width: 5, Height: 5, Pix: make([]float32, 5*5*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i) / float32(len(f.Pix))
	}
	out := resampleToReference(f, identitySimilarity, 5, 5, 1)
	for i := range f.Pix {
		if math.Abs(float64(out.Pix[i]-f.Pix[i])) > 1e-5 {
			return fmt.Errorf("identity resample: pixel %d = %v, want %v", i, out.Pix[i], f.Pix[i])
		}
	}
	return nil
}

func testSafeCrop() error {
	const w, h = 10, 10
	valid := make([]int32, w*h)
	for i := range valid {
		valid[i] = 3 // all valid
	}
	// Carve out an invalid region: row 0
	for x := 0; x < w; x++ {
		valid[x] = 2
	}
	crop := findSafeCrop(valid, w, h, 3)
	if crop.Min.Y != 1 || crop.Max.Y != 10 {
		return fmt.Errorf("crop y = %d..%d, want 1..10", crop.Min.Y, crop.Max.Y)
	}
	if crop.Min.X != 0 || crop.Max.X != 10 {
		return fmt.Errorf("crop x = %d..%d, want 0..10", crop.Min.X, crop.Max.X)
	}
	return nil
}

// =========================================================================
// v0.4.3 TEST ADDITIONS — resampling, registration robustness, CLI, pipeline
// =========================================================================

// testResampleTranslation checks that a non-identity similarity actually
// moves samples. This is the single most important test missing from v0.4.2:
// every existing resampling test uses identitySimilarity, so a sign flip in
// sampleBilinear or a direction error in similarity would go unnoticed.
func testResampleTranslation() error {
	const w, h = 5, 5
	src := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Bright pixel at (1, 1)
	idx := (1*w + 1) * 3
	src.Pix[idx], src.Pix[idx+1], src.Pix[idx+2] = 1, 1, 1

	// sim maps source → reference by translating (+2, 0).
	// So reference pixel (3, 1) samples source (1, 1) → should be bright.
	sim := similarity{SC: 1, SS: 0, Tx: 2, Ty: 0}
	out := resampleToReference(src, sim, w, h, 1)

	if got := out.Pix[(1*w+3)*3]; got < 0.9 {
		return fmt.Errorf("expected bright at (3,1), got %v", got)
	}
	if got := out.Pix[(1*w+1)*3]; got > 0.1 {
		return fmt.Errorf("expected dark at (1,1), got %v", got)
	}
	return nil
}

// testResampleRoundTrip: forward then inverse transform must recover the
// source for integer translations (bilinear at integer offsets is exact).
// This exercises similarity.inverse(), which was previously tested only
// for a single point pair, not through the whole resampler.
func testResampleRoundTrip() error {
	const w, h = 8, 8
	src := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range src.Pix {
		src.Pix[i] = float32((i*37)%100) / 100
	}

	sim := similarity{SC: 1, SS: 0, Tx: -2, Ty: 3}
	inv := sim.inverse()

	fwd := resampleToReference(src, sim, w, h, 1)
	// Resample fwd back through inv: net transform is identity, so pixels
	// that remained in bounds should match src.
	back := resampleToReference(fwd, inv, w, h, 1)

	diff := 0
	for y := 2; y < h-3; y++ {
		for x := 2; x < w-3; x++ {
			i := (y*w + x) * 3
			if math.Abs(float64(back.Pix[i]-src.Pix[i])) > 1e-4 {
				diff++
			}
		}
	}
	if diff > 0 {
		return fmt.Errorf("%d interior pixels differ after round-trip", diff)
	}
	return nil
}

// testRegistrationWithOutliers: RANSAC must survive 10 spurious "stars" in
// the target frame. Without this, a session with satellite trails or hot
// pixels could cause the transform to be fitted to garbage.
func testRegistrationWithOutliers() error {
	refStars := []star{
		{X: 100, Y: 100, Flux: 1000}, {X: 200, Y: 150, Flux: 800},
		{X: 300, Y: 200, Flux: 600}, {X: 150, Y: 300, Flux: 700},
		{X: 250, Y: 350, Flux: 500}, {X: 400, Y: 250, Flux: 400},
		{X: 180, Y: 220, Flux: 300}, {X: 280, Y: 180, Flux: 250},
		{X: 350, Y: 300, Flux: 200},
	}
	theta := 2.0 * math.Pi / 180
	T := similarity{SC: math.Cos(theta), SS: math.Sin(theta), Tx: 5, Ty: 3}
	Tinv := T.inverse()

	var tgtStars []star
	for _, s := range refStars {
		p := Tinv.apply(point2D{s.X, s.Y})
		tgtStars = append(tgtStars, star{X: p.X, Y: p.Y, Flux: s.Flux})
	}
	// 10 spurious detections scattered across the field
	for i := 0; i < 10; i++ {
		tgtStars = append(tgtStars, star{
			X: float64(500 + i*15), Y: float64(500 + i*11), Flux: 50,
		})
	}

	_, inliers, ok := matchStarsToReference(refStars, tgtStars)
	if !ok {
		return errors.New("matchStarsToReference failed with outliers present")
	}
	if inliers < 8 {
		return fmt.Errorf("only %d inliers, want ≥8 of 9 true stars", inliers)
	}
	return nil
}

// testStarDetectionEmpty: a perfectly uniform frame must return zero stars
// and must not panic. Current detectStars has a "no comps" fast path that
// is untested.
func testStarDetectionEmpty() error {
	const w, h = 64, 64
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	got := detectStars(f, 100)
	if len(got) != 0 {
		return fmt.Errorf("expected 0 stars, got %d", len(got))
	}
	return nil
}

// testStarDetectionEdgeStars: stars touching the margin filter must be
// rejected. Without this, registration can match on partially-clipped
// stars whose centroids are wrong by several pixels.
func testStarDetectionEdgeStars() error {
	const w, h = 40, 40
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	// Place stars at (1,1) — inside margin — and (20,20) — well inside
	place := func(cx, cy int) {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				i := (y*w + x) * 3
				f.Pix[i] += 0.5
				f.Pix[i+1] += 0.5
				f.Pix[i+2] += 0.5
			}
		}
	}
	place(1, 1)
	place(20, 20)
	got := detectStars(f, 100)
	for _, s := range got {
		if s.X < 3 || s.Y < 3 {
			return fmt.Errorf("edge star not rejected: (%v, %v)", s.X, s.Y)
		}
	}
	return nil
}

// testParseArgs: invalid CLI configurations must be rejected at parse time.
// Right now any --bg-degree, including "0" or "99", is accepted and silently
// clamped inside removeBackground.
func testParseArgs() error {
	cases := []struct {
		args    []string
		wantErr bool
	}{
		{[]string{"--version"}, false},
		{[]string{"--bg-degree", "5"}, true},
		{[]string{"--bg-degree", "0"}, true},
		{[]string{"--stretch", "-1"}, true},
		{[]string{"--workers", "0"}, true},
		{[]string{"--sharpen-amount", "-1"}, true},
		{[]string{"--sharpen-radius", "0"}, true},
		{[]string{"--stack-only", "--post-only"}, true},
		{[]string{"a", "b"}, true}, // two positional args
		{nil, false},
	}
	for i, c := range cases {
		_, err := parseArgs(c.args)
		if (err != nil) != c.wantErr {
			return fmt.Errorf("case %d (%v): wantErr=%v, got err=%v",
				i, c.args, c.wantErr, err)
		}
	}
	return nil
}

// testAtomicWriteCleanup: a failed write must not leave a .tmp file behind.
// The atomic write is the only safety net against partially-written outputs;
// a .tmp leak there would silently fill disks on every failed run.
func testAtomicWriteCleanup() error {
	dir, err := os.MkdirTemp("", "skystacker-atomic-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "out.bin")
	writeErr := writeFileAtomic(path, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return errors.New("simulated encoder failure")
	})
	if writeErr == nil {
		return errors.New("expected error from failing writer")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("output file exists after failure")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		return fmt.Errorf("orphan .tmp file after failure")
	}
	return nil
}

// testPipelineStackOnly: end-to-end through runPipeline with --stack-only.
// This is the highest-level test the project has and catches regressions
// in output naming, report writing, and directory creation.
func testPipelineStackOnly() error {
	dir, err := os.MkdirTemp("", "skystacker-e2e-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	// Write 3 synthetic 32×32 JPEGs
	const w, h = 32, 32
	for i := 0; i < 3; i++ {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		// put a bright "star" at the same spot in every frame
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				img.SetRGBA(16+dx, 16+dy, color.RGBA{255, 255, 255, 255})
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("frame_%02d.jpg", i))
		fp, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := jpeg.Encode(fp, img, &jpeg.Options{Quality: 95}); err != nil {
			fp.Close()
			return err
		}
		fp.Close()
	}

	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 2, Register: true, StackOnly: true,
	}); err != nil {
		return fmt.Errorf("runPipeline: %w", err)
	}

	outDir := filepath.Join(dir, "skystacker_results")
	for _, name := range []string{
		"stacked_mean.tiff", "stacked_mean.jpg",
		"stacked_no_correction_mean.tiff", "stacked_no_correction_mean.jpg",
		"stacked_sum.tiff", "stacked_sum.jpg", "report.txt",
	} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("missing output %s: %w", name, err)
		}
	}
	// --stack-only must NOT create post-processing outputs
	if _, err := os.Stat(filepath.Join(outDir, "post_step4_sharpened.jpg")); err == nil {
		return errors.New("post-processing ran during --stack-only")
	}
	return nil
}

// testPipelinePostOnly: --post-only must consume an existing stacked_mean.tiff
// and produce the four step files. This path is completely untested today.
func testPipelinePostOnly() error {
	dir, err := os.MkdirTemp("", "skystacker-postonly-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	outDir := filepath.Join(dir, "skystacker_results")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Write a synthetic stacked_mean.tiff directly
	const w, h = 64, 64
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		// gradient background + a bright spot
		x, y := i%w, i/w
		bg := float32(0.1 + 0.3*float64(x)/float64(w) + 0.1*float64(y)/float64(h))
		f.Pix[i*3] = bg
		f.Pix[i*3+1] = bg
		f.Pix[i*3+2] = bg
	}
	// bright spot
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((30+dy)*w + 32 + dx) * 3
			f.Pix[i] += 0.5
			f.Pix[i+1] += 0.5
			f.Pix[i+2] += 0.5
		}
	}
	if err := writeFileAtomic(filepath.Join(outDir, "stacked_mean.tiff"),
		func(out io.Writer) error { return writeFloat32RGB(out, w, h, f.Pix) }); err != nil {
		return err
	}

	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, PostOnly: true, BGDegree: 2, StretchFactor: 10,
		SharpenAmount: 0.8, SharpenRadius: 1.5,
	}); err != nil {
		return fmt.Errorf("runPipeline --post-only: %w", err)
	}

	for _, name := range []string{
		"post_step1_background_removed.jpg",
		"post_step2_contrast_stretched.jpg",
		"post_step3_color_calibrated.jpg",
		"post_step4_sharpened.jpg",
		"stacked_processed.tiff",
	} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("missing post output %s: %w", name, err)
		}
	}
	return nil
}

// testReportContains: report.txt must contain the exact caveats the project
// promises in its documentation. A silent drop of the "no registration"
// caveat would ship a scientifically misleading report.
func testReportContains() error {
	dir, err := os.MkdirTemp("", "skystacker-report-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	outDir := filepath.Join(dir, "results")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	f := &frame{Width: 4, Height: 4, Pix: make([]float32, 4*4*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.5
	}
	if err := writeFileAtomic(filepath.Join(outDir, "stacked_mean.tiff"),
		func(w io.Writer) error { return writeFloat32RGB(w, 4, 4, f.Pix) }); err != nil {
		return err
	}

	rep := &processingReport{
		Version: version, Timestamp: time.Now(),
		InputDir: "in", OutputDir: "out",
		Width: 4, Height: 4, Channels: 3,
		Method: "test", AccumulatorType: "float32",
		OutputFormat: "TIFF", OutputScaling: "none",
		RegistrationOn: false,
		Notes: []string{
			"No star registration or calibration was performed.",
			"JPEG inputs are gamma-encoded.",
		},
	}
	var buf bytes.Buffer
	if err := rep.writeTo(&buf); err != nil {
		return err
	}
	out := buf.String()
	for _, want := range []string{
		"SkyStacker", "Registration:", "Notes:",
		"No star registration or calibration",
	} {
		if !strings.Contains(out, want) {
			return fmt.Errorf("report missing %q", want)
		}
	}
	return nil
}

// =========================================================================
// POST-PROCESSING PIPELINE (v0.4.0)
// =========================================================================
//
// Reads stacked_mean.tiff and produces:
//   post_step1_background_removed.jpg
//   post_step2_contrast_stretched.jpg
//   post_step3_color_calibrated.jpg
//   post_step4_sharpened.jpg
//   stacked_processed.tiff
//
// Each step is independent and reversible in isolation — you can inspect
// the intermediate JPEGs to see exactly what each stage contributes.

// --- Polynomial helpers (used by background removal) ---

func nCoeff(degree int) int { return (degree + 1) * (degree + 2) / 2 }

// polyBasis returns the monomial basis vector for a 2D polynomial of the
// given degree. Ordering: 1, x, y, x², xy, y², x³, x²y, xy², y³.
func polyBasis(x, y float64, degree int) []float64 {
	switch degree {
	case 1:
		return []float64{1, x, y}
	case 2:
		return []float64{1, x, y, x * x, x * y, y * y}
	case 3:
		return []float64{1, x, y, x * x, x * y, y * y, x * x * x, x * x * y, x * y * y, y * y * y}
	}
	return []float64{1}
}

func evalPoly(coeffs []float64, x, y float64, degree int) float64 {
	b := polyBasis(x, y, degree)
	var sum float64
	for i := range b {
		if i < len(coeffs) {
			sum += coeffs[i] * b[i]
		}
	}
	return sum
}

// solveLinear solves A·x = b for x via Gauss-Jordan with partial pivoting.
// Returns zeros if the matrix is singular.
func solveLinear(A [][]float64, b []float64, n int) []float64 {
	M := make([][]float64, n)
	for i := 0; i < n; i++ {
		M[i] = make([]float64, n+1)
		copy(M[i], A[i])
		M[i][n] = b[i]
	}
	for col := 0; col < n; col++ {
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(M[row][col]) > math.Abs(M[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(M[pivot][col]) < 1e-12 {
			return make([]float64, n)
		}
		M[col], M[pivot] = M[pivot], M[col]
		pv := M[col][col]
		for j := col; j <= n; j++ {
			M[col][j] /= pv
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			f := M[row][col]
			if f == 0 {
				continue
			}
			for j := col; j <= n; j++ {
				M[row][j] -= f * M[col][j]
			}
		}
	}
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = M[i][n]
	}
	return x
}

func percentile(sorted []float32, pct float64) float32 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(pct * float64(len(sorted)-1) / 100)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// --- Step 1: background gradient removal ---

type bgSample struct {
	x, y float64    // normalised [-1, 1]
	v    [3]float64 // per-channel background estimate
}

func removeBackground(f *frame, degree int) *frame {
	if degree < 1 || degree > 3 {
		degree = 2
	}
	blockSize := 32
	if f.Width/8 < blockSize {
		blockSize = f.Width / 8
	}
	if f.Height/8 < blockSize {
		blockSize = f.Height / 8
	}
	if blockSize < 8 {
		blockSize = 8
	}
	nbx := (f.Width + blockSize - 1) / blockSize
	nby := (f.Height + blockSize - 1) / blockSize

	var samples []bgSample
	const subsample = 4
	for by := 0; by < nby; by++ {
		for bx := 0; bx < nbx; bx++ {
			x0 := bx * blockSize
			y0 := by * blockSize
			x1 := x0 + blockSize
			y1 := y0 + blockSize
			if x1 > f.Width {
				x1 = f.Width
			}
			if y1 > f.Height {
				y1 = f.Height
			}
			var rV, gV, bV []float32
			for y := y0; y < y1; y += subsample {
				for x := x0; x < x1; x += subsample {
					i := (y*f.Width + x) * 3
					rV = append(rV, f.Pix[i])
					gV = append(gV, f.Pix[i+1])
					bV = append(bV, f.Pix[i+2])
				}
			}
			if len(rV) < 8 {
				continue
			}
			sort.Slice(rV, func(i, j int) bool { return rV[i] < rV[j] })
			sort.Slice(gV, func(i, j int) bool { return gV[i] < gV[j] })
			sort.Slice(bV, func(i, j int) bool { return bV[i] < bV[j] })
			cx := float64(x0+x1)/2.0/float64(f.Width)*2.0 - 1.0
			cy := float64(y0+y1)/2.0/float64(f.Height)*2.0 - 1.0
			samples = append(samples, bgSample{
				x: cx,
				y: cy,
				v: [3]float64{
					float64(percentile(rV, 20)),
					float64(percentile(gV, 20)),
					float64(percentile(bV, 20)),
				},
			})
		}
	}
	if len(samples) < nCoeff(degree)*2 {
		return f // not enough blocks — skip step
	}

	// Robust polynomial fit per channel
	coeffs := make([][]float64, 3)
	for c := 0; c < 3; c++ {
		active := make([]bool, len(samples))
		for i := range active {
			active[i] = true
		}
		n := nCoeff(degree)
		const iters = 3
		for iter := 0; iter < iters; iter++ {
			A := make([][]float64, n)
			for i := range A {
				A[i] = make([]float64, n)
			}
			b := make([]float64, n)
			count := 0
			for i := range samples {
				if !active[i] {
					continue
				}
				basis := polyBasis(samples[i].x, samples[i].y, degree)
				for j := 0; j < n; j++ {
					for k := 0; k < n; k++ {
						A[j][k] += basis[j] * basis[k]
					}
					b[j] += basis[j] * samples[i].v[c]
				}
				count++
			}
			if count < n {
				coeffs[c] = make([]float64, n)
				break
			}
			coeffs[c] = solveLinear(A, b, n)
			if iter < iters-1 {
				var residuals []float64
				for i := range samples {
					if !active[i] {
						continue
					}
					pred := evalPoly(coeffs[c], samples[i].x, samples[i].y, degree)
					residuals = append(residuals, samples[i].v[c]-pred)
				}
				var mean, sq float64
				for _, r := range residuals {
					mean += r
				}
				mean /= float64(len(residuals))
				for _, r := range residuals {
					sq += (r - mean) * (r - mean)
				}
				sigma := math.Sqrt(sq / float64(len(residuals)))
				if sigma < 1e-9 {
					sigma = 1e-9
				}
				// Reject samples that are significantly BRIGHTER than the
				// fit (those are nebulae/stars, not background).
				kept := 0
				for i := range samples {
					if !active[i] {
						continue
					}
					pred := evalPoly(coeffs[c], samples[i].x, samples[i].y, degree)
					if samples[i].v[c]-pred > 2.0*sigma {
						active[i] = false
					} else {
						kept++
					}
				}
				if kept < n {
					break
				}
			}
		}
	}

	// Apply: subtract the fitted surface
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	invW := 2.0 / float64(f.Width-1)
	invH := 2.0 / float64(f.Height-1)
	for y := 0; y < f.Height; y++ {
		ny := float64(y)*invH - 1.0
		for x := 0; x < f.Width; x++ {
			nx := float64(x)*invW - 1.0
			i := (y*f.Width + x) * 3
			for c := 0; c < 3; c++ {
				bg := evalPoly(coeffs[c], nx, ny, degree)
				result.Pix[i+c] = f.Pix[i+c] - float32(bg)
			}
		}
	}
	return result
}

// --- Step 2: contrast stretch (asinh) ---

func stretchContrast(f *frame, stretch float64) *frame {
	// Compute luminance percentiles and robust statistics on a subsample.
	var lums []float32
	const step = 4
	for y := 0; y < f.Height; y += step {
		for x := 0; x < f.Width; x += step {
			i := (y*f.Width + x) * 3
			lums = append(lums, 0.299*f.Pix[i]+0.587*f.Pix[i+1]+0.114*f.Pix[i+2])
		}
	}
	if len(lums) == 0 {
		return f
	}
	sort.Slice(lums, func(i, j int) bool { return lums[i] < lums[j] })

	n := len(lums)
	median := float64(lums[n/2])

	// Robust sigma from median absolute deviation.
	devs := make([]float64, n)
	for i, l := range lums {
		devs[i] = math.Abs(float64(l) - median)
	}
	sort.Float64s(devs)
	mad := devs[n/2]
	sigma := 1.4826 * mad
	if sigma < 1e-6 {
		sigma = 1e-6
	}

	// Shadow clip: 2.8 sigma below median (clips ~0.25% of background).
	black := median - 2.8*sigma
	if black < 0 {
		black = 0
	}

	// White point: P99.9 (top 0.1% of pixels — actual stars, not noise).
	pWhite := float64(lums[int(float64(n-1)*0.999)])
	if pWhite <= black {
		pWhite = black + 1e-4
	}

	// Target background brightness. Higher --stretch → darker background.
	targetBG := 1.5 / stretch
	if stretch <= 0 {
		targetBG = 0.15
	}
	if targetBG < 0.03 {
		targetBG = 0.03
	}
	if targetBG > 0.35 {
		targetBG = 0.35
	}

	// Normalised median (in [0, 1] within the [black, white] range).
	mNorm := (median - black) / (pWhite - black)
	if mNorm < 0.001 {
		mNorm = 0.001
	}
	if mNorm > 0.999 {
		mNorm = 0.999
	}

	// Solve mtf(mNorm, m) = targetBG for the MTF midtone parameter m.
	// MTF(x, m) = ((m-1)*x) / ((2m-1)*x - m)
	// Inverse:  m = x*(1-t) / (x*(1-2t) + t)  where t = target output.
	m := mNorm * (1 - targetBG) / (mNorm*(1-2*targetBG) + targetBG)

	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	invRange := 1.0 / (pWhite - black)
	for i := range f.Pix {
		v := (float64(f.Pix[i]) - black) * invRange
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		num := (m - 1) * v
		den := (2*m-1)*v - m
		var out float64
		if math.Abs(den) < 1e-10 {
			out = 0
		} else {
			out = num / den
		}
		if out < 0 {
			out = 0
		} else if out > 1 {
			out = 1
		}
		result.Pix[i] = float32(out)
	}
	return result
}

// --- Step 3: color calibration / background neutralisation ---

func calibrateColor(f *frame) *frame {
	// Find darkest 20% by luminance
	var lums []float32
	const step = 4
	for y := 0; y < f.Height; y += step {
		for x := 0; x < f.Width; x += step {
			i := (y*f.Width + x) * 3
			lums = append(lums, 0.299*f.Pix[i]+0.587*f.Pix[i+1]+0.114*f.Pix[i+2])
		}
	}
	sort.Slice(lums, func(i, j int) bool { return lums[i] < lums[j] })
	thresh := percentile(lums, 20)

	var sumR, sumG, sumB float64
	var n int
	for i := 0; i < len(f.Pix); i += 3 {
		l := 0.299*f.Pix[i] + 0.587*f.Pix[i+1] + 0.114*f.Pix[i+2]
		if l <= thresh {
			sumR += float64(f.Pix[i])
			sumG += float64(f.Pix[i+1])
			sumB += float64(f.Pix[i+2])
			n++
		}
	}
	if n == 0 {
		return f
	}
	meanR := sumR / float64(n)
	meanG := sumG / float64(n)
	meanB := sumB / float64(n)
	const eps = 1e-4
	if meanR < eps {
		meanR = eps
	}
	if meanG < eps {
		meanG = eps
	}
	if meanB < eps {
		meanB = eps
	}
	target := (meanR + meanG + meanB) / 3.0
	scaleR := clampFloat(target/meanR, 0.5, 2.0)
	scaleG := clampFloat(target/meanG, 0.5, 2.0)
	scaleB := clampFloat(target/meanB, 0.5, 2.0)
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := 0; i < len(f.Pix); i += 3 {
		r := float64(f.Pix[i]) * scaleR
		g := float64(f.Pix[i+1]) * scaleG
		b := float64(f.Pix[i+2]) * scaleB
		if r > 1 {
			r = 1
		}
		if g > 1 {
			g = 1
		}
		if b > 1 {
			b = 1
		}
		if r < 0 {
			r = 0
		}
		if g < 0 {
			g = 0
		}
		if b < 0 {
			b = 0
		}
		result.Pix[i] = float32(r)
		result.Pix[i+1] = float32(g)
		result.Pix[i+2] = float32(b)
	}
	return result
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- Step 4: unsharp mask ---

func gaussianBlur(f *frame, sigma float64) *frame {
	if sigma <= 0 {
		sigma = 1.5
	}
	radius := int(math.Ceil(sigma * 3))
	if radius < 1 {
		radius = 1
	}
	kernelSize := radius*2 + 1
	kernel := make([]float32, kernelSize)
	var sum float32
	for i := 0; i < kernelSize; i++ {
		off := float64(i - radius)
		kernel[i] = float32(math.Exp(-off * off / (2 * sigma * sigma)))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	// Horizontal pass
	tmp := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			for c := 0; c < 3; c++ {
				var acc float32
				for k := 0; k < kernelSize; k++ {
					sx := x + k - radius
					if sx < 0 {
						sx = 0
					} else if sx >= f.Width {
						sx = f.Width - 1
					}
					acc += kernel[k] * f.Pix[(y*f.Width+sx)*3+c]
				}
				tmp.Pix[(y*f.Width+x)*3+c] = acc
			}
		}
	}
	// Vertical pass
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			for c := 0; c < 3; c++ {
				var acc float32
				for k := 0; k < kernelSize; k++ {
					sy := y + k - radius
					if sy < 0 {
						sy = 0
					} else if sy >= f.Height {
						sy = f.Height - 1
					}
					acc += kernel[k] * tmp.Pix[(sy*f.Width+x)*3+c]
				}
				result.Pix[(y*f.Width+x)*3+c] = acc
			}
		}
	}
	return result
}

func unsharpMask(f *frame, amount, radius float64) *frame {
	blurred := gaussianBlur(f, radius)
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	amt := float32(amount)
	for i := range f.Pix {
		o := f.Pix[i]
		b := blurred.Pix[i]
		v := o + amt*(o-b)
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		result.Pix[i] = v
	}
	return result
}

// --- Pipeline driver ---

func floatToByte(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// writeLinearPreviewJPEG maps [0,1] values directly to [0,255] with no
// gamma. Use for post-processing stages whose outputs are already stretched.
func writeLinearPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = floatToByte(pix[i+0])
			img.Pix[o+1] = floatToByte(pix[i+1])
			img.Pix[o+2] = floatToByte(pix[i+2])
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

// writeAutoNormalizedLinearPreviewJPEG divides by the global max then maps
// [0,1] to [0,255] linearly. Use when values may be slightly outside [0,1]
// (step 1) but gamma lifting would wash out the background.
func writeAutoNormalizedLinearPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	maxVal := float32(0)
	for _, v := range pix {
		if v > maxVal {
			maxVal = v
		}
	}
	if math.IsNaN(float64(maxVal)) || math.IsInf(float64(maxVal), 0) || maxVal <= 0 {
		maxVal = 1.0
	}
	inv := float32(1.0) / maxVal
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = floatToByte(pix[i+0] * inv)
			img.Pix[o+1] = floatToByte(pix[i+1] * inv)
			img.Pix[o+2] = floatToByte(pix[i+2] * inv)
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

func writePostPreview(dir, name string, f *frame) error {
	return writeFileAtomic(filepath.Join(dir, name), func(w io.Writer) error {
		return writeAutoNormalizedLinearPreviewJPEG(w, f.Width, f.Height, f.Pix)
	})
}

func runPostProcessing(outputDir string, opts runOptions) ([]string, error) {
	meanPath := filepath.Join(outputDir, "stacked_mean.tiff")
	fh, err := os.Open(meanPath)
	if err != nil {
		return nil, fmt.Errorf("post-process: open mean tiff: %w", err)
	}
	w, h, pix, err := readFloat32RGB(fh)
	fh.Close()
	if err != nil {
		return nil, fmt.Errorf("post-process: read mean tiff: %w", err)
	}
	src := &frame{Width: w, Height: h, Pix: pix}

	var outputs []string

	// Step 1: background removal
	logf("  post step 1: background removal (degree %d)", opts.BGDegree)
	s1 := removeBackground(src, opts.BGDegree)
	p1 := filepath.Join(outputDir, "post_step1_background_removed.jpg")
	if err := writePostPreview(outputDir, "post_step1_background_removed.jpg", s1); err != nil {
		logf("  WARNING: step1 preview: %v", err)
	}
	outputs = append(outputs, p1)

	// Step 2: contrast stretch
	logf("  post step 2: contrast stretch (asinh f=%g)", opts.StretchFactor)
	s2 := stretchContrast(s1, opts.StretchFactor)
	p2 := filepath.Join(outputDir, "post_step2_contrast_stretched.jpg")
	if err := writePostPreview(outputDir, "post_step2_contrast_stretched.jpg", s2); err != nil {
		logf("  WARNING: step2 preview: %v", err)
	}
	outputs = append(outputs, p2)

	// Step 3: color calibration
	var s3 *frame
	if opts.NoColorCalib {
		logf("  post step 3: skipped (--no-color-calib)")
		s3 = s2
	} else {
		logf("  post step 3: color calibration")
		s3 = calibrateColor(s2)
	}
	p3 := filepath.Join(outputDir, "post_step3_color_calibrated.jpg")
	if err := writePostPreview(outputDir, "post_step3_color_calibrated.jpg", s3); err != nil {
		logf("  WARNING: step3 preview: %v", err)
	}
	outputs = append(outputs, p3)

	// Step 4: sharpening
	var s4 *frame
	if opts.NoSharpen {
		logf("  post step 4: skipped (--no-sharpen)")
		s4 = s3
	} else {
		logf("  post step 4: unsharp mask (amount=%g radius=%g)", opts.SharpenAmount, opts.SharpenRadius)
		s4 = unsharpMask(s3, opts.SharpenAmount, opts.SharpenRadius)
	}
	p4 := filepath.Join(outputDir, "post_step4_sharpened.jpg")
	if err := writePostPreview(outputDir, "post_step4_sharpened.jpg", s4); err != nil {
		logf("  WARNING: step4 preview: %v", err)
	}
	outputs = append(outputs, p4)

	// Final TIFF
	tiffPath := filepath.Join(outputDir, "stacked_processed.tiff")
	if err := writeFileAtomic(tiffPath, func(w io.Writer) error {
		return writeFloat32RGB(w, s4.Width, s4.Height, s4.Pix)
	}); err != nil {
		return outputs, fmt.Errorf("post-process: write final tiff: %w", err)
	}
	outputs = append(outputs, tiffPath)

	return outputs, nil
}

// =========================================================================
// POST-PROCESSING SELF-TESTS
// =========================================================================

func testPolyBasis() error {
	b := polyBasis(2, 3, 2)
	want := []float64{1, 2, 3, 4, 6, 9}
	if len(b) != len(want) {
		return fmt.Errorf("len=%d want %d", len(b), len(want))
	}
	for i := range want {
		if math.Abs(b[i]-want[i]) > 1e-9 {
			return fmt.Errorf("b[%d]=%v want %v", i, b[i], want[i])
		}
	}
	b3 := polyBasis(2, 3, 3)
	if len(b3) != 10 {
		return fmt.Errorf("degree 3 len=%d want 10", len(b3))
	}
	if nCoeff(1) != 3 || nCoeff(2) != 6 || nCoeff(3) != 10 {
		return fmt.Errorf("nCoeff wrong")
	}
	return nil
}

func testSolveLinear() error {
	// 2×2 system: 2x + y = 5; x + 3y = 10  → x=1, y=3
	A := [][]float64{{2, 1}, {1, 3}}
	b := []float64{5, 10}
	x := solveLinear(A, b, 2)
	if math.Abs(x[0]-1) > 1e-9 || math.Abs(x[1]-3) > 1e-9 {
		return fmt.Errorf("got %v, want [1 3]", x)
	}
	return nil
}

func testBackgroundRemoval() error {
	const w, h = 128, 128
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Linear gradient background: 0.10 → 0.60
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bg := float32(0.10 + 0.30*float64(x)/float64(w) + 0.20*float64(y)/float64(h))
			i := (y*w + x) * 3
			f.Pix[i] = bg
			f.Pix[i+1] = bg
			f.Pix[i+2] = bg
		}
	}
	// A few synthetic stars
	stars := []point2D{{20, 20}, {100, 30}, {60, 90}, {30, 100}, {110, 110}}
	for _, s := range stars {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				x, y := int(s.X)+dx, int(s.Y)+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				d2 := float64(dx*dx + dy*dy)
				v := float32(0.5 * math.Exp(-d2))
				i := (y*w + x) * 3
				f.Pix[i] += v
				f.Pix[i+1] += v
				f.Pix[i+2] += v
			}
		}
	}
	result := removeBackground(f, 2)
	// Residuals should be small away from stars
	var sumRes float64
	var nRes int
	for y := 8; y < h-8; y += 8 {
		for x := 8; x < w-8; x += 8 {
			nearStar := false
			for _, s := range stars {
				if math.Hypot(float64(x)-s.X, float64(y)-s.Y) < 5 {
					nearStar = true
					break
				}
			}
			if nearStar {
				continue
			}
			i := (y*w + x) * 3
			sumRes += math.Abs(float64(result.Pix[i]))
			nRes++
		}
	}
	if nRes == 0 {
		return errors.New("no residual samples")
	}
	avg := sumRes / float64(nRes)
	if avg > 0.02 {
		return fmt.Errorf("avg residual %.4f too high (want < 0.02)", avg)
	}
	return nil
}

func testContrastStretch() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		var v float32
		switch {
		case i%100 == 0:
			v = 0.5
		case i%10 == 0:
			v = 0.2
		default:
			v = 0.1
		}
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	result := stretchContrast(f, 10)
	var bgSum, midSum, brSum float32
	var nBg, nMid, nBr int
	for i := 0; i < w*h; i++ {
		switch {
		case i%100 == 0:
			brSum += result.Pix[i*3]
			nBr++
		case i%10 == 0:
			midSum += result.Pix[i*3]
			nMid++
		default:
			bgSum += result.Pix[i*3]
			nBg++
		}
	}
	bg := bgSum / float32(nBg)
	mid := midSum / float32(nMid)
	br := brSum / float32(nBr)
	if mid < 0.3 {
		return fmt.Errorf("mid not lifted: %v (want > 0.3)", mid)
	}
	if br < 0.85 {
		return fmt.Errorf("bright not stretched: %v (want > 0.85)", br)
	}
	if bg > 0.1 {
		return fmt.Errorf("background leaked: %v (want < 0.1)", bg)
	}
	return nil
}

func testColorCalibration() error {
	const w, h = 32, 32
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		// Red cast: R mean 0.2, G mean 0.1, B mean 0.1
		f.Pix[i*3] = 0.2
		f.Pix[i*3+1] = 0.1
		f.Pix[i*3+2] = 0.1
	}
	result := calibrateColor(f)
	// Compute means after
	var sr, sg, sb float64
	for i := 0; i < len(result.Pix); i += 3 {
		sr += float64(result.Pix[i])
		sg += float64(result.Pix[i+1])
		sb += float64(result.Pix[i+2])
	}
	n := float64(len(result.Pix) / 3)
	mr, mg, mb := sr/n, sg/n, sb/n
	// After calibration, the three channel means should be closer together
	maxDiff := math.Max(math.Abs(mr-mg), math.Max(math.Abs(mg-mb), math.Abs(mr-mb)))
	if maxDiff > 0.02 {
		return fmt.Errorf("channels not neutralised: means %.3f %.3f %.3f", mr, mg, mb)
	}
	return nil
}

func testGaussianBlur() error {
	const w, h = 9, 9
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	f.Pix[(4*w+4)*3] = 1.0 // single bright pixel
	result := gaussianBlur(f, 1.5)
	// Neighbours should be positive, sum should be ~1
	var total float32
	for i := 0; i < len(result.Pix); i += 3 {
		total += result.Pix[i]
	}
	if math.Abs(float64(total-1)) > 0.01 {
		return fmt.Errorf("total energy %.4f want ~1", total)
	}
	neighbour := result.Pix[(4*w+5)*3]
	if neighbour <= 0 {
		return errors.New("neighbour not blurred")
	}
	if neighbour >= 0.5 {
		return fmt.Errorf("neighbour too bright: %v", neighbour)
	}
	return nil
}

func testUnsharpMask() error {
	const w, h = 16, 1
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Step edge at x=8
	for x := 0; x < w; x++ {
		v := float32(0.2)
		if x >= 8 {
			v = 0.6
		}
		f.Pix[x*3] = v
		f.Pix[x*3+1] = v
		f.Pix[x*3+2] = v
	}
	result := unsharpMask(f, 0.8, 1.5)
	// Edge contrast should be enhanced: value just after edge higher, value just before lower
	beforeEdge := result.Pix[7*3]
	afterEdge := result.Pix[8*3]
	if afterEdge-beforeEdge < 0.4 {
		return fmt.Errorf("edge contrast not enhanced: before=%v after=%v", beforeEdge, afterEdge)
	}
	// Values should be clipped to [0, 1]
	for i, v := range result.Pix {
		if v < 0 || v > 1.0001 {
			return fmt.Errorf("out-of-range value at %d: %v", i, v)
		}
	}
	return nil
}

// =========================================================================
// main
// =========================================================================

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		fmt.Fprintln(os.Stderr, "Run 'skystacker --help' for usage.")
		os.Exit(2)
	}
	if opts.Help {
		printUsage(os.Stdout)
		return
	}
	if opts.Version {
		fmt.Println(version)
		return
	}
	if opts.SelfTest {
		if runSelfTests() > 0 {
			os.Exit(1)
		}
		return
	}
	err = runPipeline(context.Background(), runOptions{
		InputDir:      opts.InputDir,
		OutputDir:     opts.OutputDir,
		Workers:       opts.Workers,
		Recursive:     opts.Recursive,
		Overwrite:     opts.Overwrite,
		Register:      opts.Register,
		PostProcess:   opts.PostProcess,
		StackOnly:     opts.StackOnly,
		PostOnly:      opts.PostOnly,
		NoColorCalib:  opts.NoColorCalib,
		NoSharpen:     opts.NoSharpen,
		BGDegree:      opts.BGDegree,
		StretchFactor: opts.StretchFactor,
		SharpenAmount: opts.SharpenAmount,
		SharpenRadius: opts.SharpenRadius,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		os.Exit(1)
	}
}

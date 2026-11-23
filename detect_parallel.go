package main

import (
	"math"
	"sort"
	"sync"
)

// detectWorkerCount is set from --parallel-detect at pipeline start.
var detectWorkerCount int = 1

// parallelGrayscale computes luminance with a worker pool. Element-wise and
// deterministic: each worker writes a disjoint row range.
func parallelGrayscale(f *frame, workers int) []float32 {
	n := f.Width * f.Height
	g := make([]float32, n)
	if workers < 1 {
		workers = 1
	}
	if workers > f.Height {
		workers = f.Height
	}
	if workers <= 0 {
		return g
	}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			y0 := id * f.Height / workers
			y1 := (id + 1) * f.Height / workers
			for y := y0; y < y1; y++ {
				base := y * f.Width
				for x := 0; x < f.Width; x++ {
					i := base + x
					g[i] = 0.299*f.Pix[i*3] + 0.587*f.Pix[i*3+1] + 0.114*f.Pix[i*3+2]
				}
			}
		}(w)
	}
	wg.Wait()
	return g
}

// parallelBoxBlurLuminance is a separable box blur with a worker pool. The
// horizontal pass is parallelised by rows; the vertical pass is parallelised
// by columns. Both passes are exact (integer prefix sums) and independent of
// worker count, so results are byte-identical to the sequential version.
func parallelBoxBlurLuminance(gray []float32, w, h, radius int, workers int) []float32 {
	if radius < 1 {
		radius = 1
	}
	if workers < 1 {
		workers = 1
	}
	tmp := make([]float32, len(gray))
	out := make([]float32, len(gray))
	// horizontal pass: each worker handles a disjoint row range
	wp := min(workers, h)
	if wp < 1 {
		wp = 1
	}
	var wg sync.WaitGroup
	for id := 0; id < wp; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			y0 := id * h / wp
			y1 := (id + 1) * h / wp
			prefixX := make([]float64, w+1)
			for y := y0; y < y1; y++ {
				rowOff := y * w
				prefixX[0] = 0
				for x := 0; x < w; x++ {
					prefixX[x+1] = prefixX[x] + float64(gray[rowOff+x])
				}
				for x := 0; x < w; x++ {
					x0 := max(x-radius, 0)
					x1 := min(x+radius+1, w)
					tmp[rowOff+x] = float32((prefixX[x1] - prefixX[x0]) / float64(x1-x0))
				}
			}
		}(id)
	}
	wg.Wait()
	// vertical pass: each worker handles a disjoint column range
	wc := min(workers, w)
	if wc < 1 {
		wc = 1
	}
	for id := 0; id < wc; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			x0 := id * w / wc
			x1 := (id + 1) * w / wc
			prefixY := make([]float64, h+1)
			for x := x0; x < x1; x++ {
				prefixY[0] = 0
				for y := 0; y < h; y++ {
					prefixY[y+1] = prefixY[y] + float64(tmp[y*w+x])
				}
				for y := 0; y < h; y++ {
					ya := max(y-radius, 0)
					yb := min(y+radius+1, h)
					out[y*w+x] = float32((prefixY[yb] - prefixY[ya]) / float64(yb-ya))
				}
			}
		}(id)
	}
	wg.Wait()
	return out
}

// parallelResidual computes the high-pass residual with a worker pool.
func parallelResidual(gray, blurred []float32, workers int) []float32 {
	residual := make([]float32, len(gray))
	if workers < 1 {
		workers = 1
	}
	chunk := (len(gray) + workers - 1) / workers
	var wg sync.WaitGroup
	for id := 0; id < workers; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			start := id * chunk
			end := min(start+chunk, len(gray))
			for i := start; i < end; i++ {
				residual[i] = gray[i] - blurred[i]
			}
		}(id)
	}
	wg.Wait()
	return residual
}

// detectStarsWorkers is a worker-aware variant of detectStars: the
// element-wise high-pass stages (grayscale, separable box blur, residual) use
// a worker pool. Connected-component labeling and centroiding stay sequential
// because they are global operations; changing them would alter results.
func detectStarsSigmaWorkers(f *frame, maxStars int, sigma float64, workers int) []star {
	gray := parallelGrayscale(f, workers)
	hpRadius := estimateHighPassRadius(f.Width, f.Height)
	blurred := parallelBoxBlurLuminance(gray, f.Width, f.Height, hpRadius, workers)
	residual := parallelResidual(gray, blurred, workers)
	noise := estimateResidualNoise(residual)
	threshold := float32(sigma * noise)
	if threshold < 1.0/512.0 {
		threshold = 1.0 / 512.0
	}
	w, h := f.Width, f.Height
	visited := make([]bool, w*h)
	type comp struct {
		pixels []int
		peak   float32
		bright bool
	}
	var comps []comp
	const maxNormalComponentPixels = 200
	const maxBrightComponentPixels = 2000
	const maxBrightCorePixels = 2000
	for y := range h {
		for x := range w {
			i := y*w + x
			if visited[i] || residual[i] <= threshold {
				continue
			}
			stack := []int{i}
			visited[i] = true
			var pixels []int
			peak := float32(0)
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				pixels = append(pixels, p)
				if residual[p] > peak {
					peak = residual[p]
				}
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
						if visited[ni] || residual[ni] <= threshold {
							continue
						}
						visited[ni] = true
						stack = append(stack, ni)
					}
				}
			}
			accept := len(pixels) >= 1 && len(pixels) <= maxNormalComponentPixels
			bright := false
			if !accept && len(pixels) <= maxBrightComponentPixels && peak >= 4*threshold {
				core := 0
				var coreFlux, totalFlux float64
				for _, p := range pixels {
					totalFlux += float64(residual[p])
					if residual[p] >= 0.5*peak {
						core++
						coreFlux += float64(residual[p])
					}
				}
				accept = core >= 4 && core <= maxBrightCorePixels && len(pixels) <= 6*core && coreFlux >= 0.5*totalFlux
				bright = accept
			}
			if accept {
				comps = append(comps, comp{pixels: pixels, peak: peak, bright: bright})
			}
		}
	}
	stars := make([]star, 0, len(comps))
	const margin = 3
	for _, c := range comps {
		core := c.pixels
		if c.bright {
			core = make([]int, 0, len(c.pixels))
			for _, p := range c.pixels {
				if residual[p] >= 0.5*c.peak {
					core = append(core, p)
				}
			}
			if len(core) < 4 {
				core = c.pixels
			}
		}
		var sumW, sumX, sumY, sumFlux float64
		minX, minY, maxX, maxY := w, h, -1, -1
		for _, p := range c.pixels {
			val := float64(residual[p])
			if val < 0 {
				val = 0
			}
			sumFlux += val
		}
		for _, p := range core {
			py, px := p/w, p%w
			val := float64(residual[p])
			if val < 0 {
				val = 0
			}
			sumW += val
			sumX += float64(px) * val
			sumY += float64(py) * val
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
		bw, bh := maxX-minX+1, maxY-minY+1
		aspect := float64(bw) / float64(bh)
		if aspect < 0.33 || aspect > 3.0 {
			continue
		}
		var cxx, cyy, cxy float64
		for _, p := range core {
			py, px := p/w, p%w
			val := float64(residual[p])
			if val < 0 {
				val = 0
			}
			dx := float64(px) - cx
			dy := float64(py) - cy
			cxx += dx * dx * val
			cyy += dy * dy * val
			cxy += dx * dy * val
		}
		ecc := 0.0
		if sumW > 1e-12 {
			cxx /= sumW
			cyy /= sumW
			cxy /= sumW
			if denom := cxx + cyy; denom > 1e-12 {
				ecc = math.Sqrt((cxx-cyy)*(cxx-cyy)+4*cxy*cxy) / denom
				if ecc > 1 {
					ecc = 1
				}
			}
		}
		stars = append(stars, star{X: cx, Y: cy, Flux: sumFlux, Size: len(c.pixels), Ecc: ecc,
			HFR: starHFR(core, w, cx, cy, residual)})
	}
	sort.Slice(stars, func(i, j int) bool { return stars[i].Flux > stars[j].Flux })
	if len(stars) > maxStars {
		stars = stars[:maxStars]
	}
	return stars
}

// detectStarsWorkers mirrors detectStars' multi-threshold fallback using the
// parallel high-pass core, guaranteeing identical results to detectStars.
func detectStarsWorkers(f *frame, maxStars int, workers int) []star {
	stars := detectStarsSigmaWorkers(f, maxStars, 5.0, workers)
	if len(stars) >= 20 {
		return stars
	}
	for _, sig := range []float64{4.0, 3.5, 3.0} {
		s := detectStarsSigmaWorkers(f, maxStars, sig, workers)
		if len(s) > len(stars) {
			stars = s
		}
		if len(stars) >= 20 {
			break
		}
	}
	return stars
}

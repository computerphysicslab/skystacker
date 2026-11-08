package stack

import (
	"fmt"
	"sync"

	"github.com/skystacker/skystacker/internal/imaging"
)

// Accumulator holds a per-pixel RGB float32 sum.
type Accumulator struct {
	Width  int
	Height int
	Pix    []float32
	Count  int
}

// New allocates an accumulator for the given dimensions.
func New(width, height int) *Accumulator {
	return &Accumulator{
		Width:  width,
		Height: height,
		Pix:    make([]float32, width*height*3),
	}
}

// AddFrame adds f to the sum using a single goroutine.
func (a *Accumulator) AddFrame(f *imaging.Frame) error {
	return a.AddFrameParallel(f, 1)
}

// AddFrameParallel adds f to the sum, distributing rows across up to workers
// goroutines. Each worker owns a disjoint row band, so no synchronisation is
// required on the accumulator buffer.
func (a *Accumulator) AddFrameParallel(f *imaging.Frame, workers int) error {
	if f.Width != a.Width || f.Height != a.Height {
		return fmt.Errorf("frame dimensions %dx%d do not match accumulator %dx%d",
			f.Width, f.Height, a.Width, a.Height)
	}
	if len(f.Pix) != len(a.Pix) {
		return fmt.Errorf("frame pixel buffer length %d does not match accumulator %d",
			len(f.Pix), len(a.Pix))
	}
	if workers <= 1 || a.Height < workers {
		for i, v := range f.Pix {
			a.Pix[i] += v
		}
		a.Count++
		return nil
	}

	rowsPerWorker := (a.Height + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		y0 := w * rowsPerWorker
		if y0 >= a.Height {
			break
		}
		y1 := y0 + rowsPerWorker
		if y1 > a.Height {
			y1 = a.Height
		}
		start := y0 * a.Width * 3
		end := y1 * a.Width * 3
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				a.Pix[i] += f.Pix[i]
			}
		}(start, end)
	}
	wg.Wait()
	a.Count++
	return nil
}

// Divide converts the accumulated sum into an arithmetic mean in place.
func (a *Accumulator) Divide() {
	if a.Count == 0 {
		return
	}
	inv := 1.0 / float32(a.Count)
	for i := range a.Pix {
		a.Pix[i] *= inv
	}
}

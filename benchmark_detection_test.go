package main

import (
	"fmt"
	"testing"
)

// BenchmarkDetectStars measures star detection throughput on a synthetic
// 30 MP (roughly 5472x5520) frame at increasing worker counts. It is the
// evidence base for roadmap item 135. Run with:
//   go test -run XXX -bench BenchmarkDetectStars -benchmem ./...
func BenchmarkDetectStars(b *testing.B) {
	const w, h = 5472, 5520
	frame := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Synthetic star field: a few hundred point sources on a dark background.
	for i := 0; i < 400; i++ {
		cx := (i * 997) % w
		cy := (i * 1301) % h
		for dy := -3; dy <= 3; dy++ {
			for dx := -3; dx <= 3; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				v := float32(0.9) * float32(1 - (dx*dx+dy*dy)/10.0)
				if v < 0 {
					v = 0
				}
				idx := (y*w + x) * 3
				frame.Pix[idx] += v
				frame.Pix[idx+1] += v
				frame.Pix[idx+2] += v
			}
		}
	}
	for _, workers := range []int{1, 2, 4, 8} {
		workers := workers
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = detectStars(frame, 300)
			}
		})
	}
}

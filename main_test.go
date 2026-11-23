package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"
)

func sigmaClipBenchData() (vals, weights []float32) {
	rng := rand.New(rand.NewSource(20261117))
	vals = make([]float32, 16)
	weights = make([]float32, 16)
	for i := range vals {
		vals[i] = float32(100 + rng.NormFloat64()*3)
		weights[i] = float32(0.2 + rng.Float64()*2)
	}
	vals[0] = 1000
	return vals, weights
}

func BenchmarkSigmaClipFused(b *testing.B) {
	vals, weights := sigmaClipBenchData()
	b.Run("unweighted", func(b *testing.B) {
		for range b.N {
			sigmaClipReduce(vals, 3, 2)
		}
	})
	b.Run("weighted", func(b *testing.B) {
		for range b.N {
			sigmaClipReduceWeighted(vals, weights, 3, 2)
		}
	})
}

func BenchmarkSigmaClipTwoPassReference(b *testing.B) {
	vals, weights := sigmaClipBenchData()
	b.Run("unweighted", func(b *testing.B) {
		for range b.N {
			twoPassSigmaClipReduceReference(vals, 3, 2)
		}
	})
	b.Run("weighted", func(b *testing.B) {
		for range b.N {
			twoPassSigmaClipReduceWeightedReference(vals, weights, 3, 2)
		}
	})
}

func benchmarkSpillFrames(b *testing.B, n, w, h int) []string {
	b.Helper()
	dir := b.TempDir()
	paths := make([]string, 0, n)
	for i := range n {
		f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
		for j := range f.Pix {
			f.Pix[j] = float32(100+i) / 1000
		}
		path := filepath.Join(dir, "frame_"+string(rune('a'+i))+".f32raw")
		if err := writeSpillFrame(path, f); err != nil {
			b.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func BenchmarkReduceTilesStack(b *testing.B) {
	const w, h = 64, 64
	paths := benchmarkSpillFrames(b, 4, w, h)
	crop := image.Rect(0, 0, w, h)
	for _, method := range []string{"sigma", "median"} {
		b.Run(method, func(b *testing.B) {
			for range b.N {
				_, _, _, _, err := reduceTilesStack(paths, w, h, crop, method, 3, 2, 64, 1, nil, nil, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkNativeCR2Decode(b *testing.B) {
	path := filepath.Join("assets", "OrionNebula", "lights", "IMG_7005.CR2")
	if _, err := os.Stat(path); err != nil {
		b.Skipf("CR2 fixture unavailable: %v", err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := decodeCR2Native(path); err != nil {
			b.Fatal(err)
		}
	}
}

// TestGaiaRateLimitSynctest uses Go 1.26's synctest bubble to verify the
// 1 req/s Gaia TAP pacing deterministically, without sleeping in real time.
func TestGaiaRateLimitSynctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gaiaRateMu.Lock()
		gaiaLastQuery = time.Now()
		gaiaRateMu.Unlock()

		done := make(chan struct{})
		go func() {
			gaiaRateLimit()
			close(done)
		}()

		synctest.Wait()
		select {
		case <-done:
			t.Fatal("rate limiter returned before the 1s interval")
		default:
		}

		time.Sleep(999 * time.Millisecond)
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("rate limiter returned at 999ms")
		default:
		}

		time.Sleep(time.Millisecond)
		synctest.Wait()
		select {
		case <-done:
		default:
			t.Fatal("rate limiter did not return after 1s")
		}
	})
}

func TestTileBufferBudgetAndEstimate(t *testing.T) {
	if got := tileBufferBudgetMB(0); got != stackTileBufferMB {
		t.Fatalf("unknown availability budget = %.0f, want %.0f", got, stackTileBufferMB)
	}
	if got := tileBufferBudgetMB(64); got != 32 {
		t.Fatalf("low-memory budget = %.0f, want 32", got)
	}
	if got := tileBufferBudgetMB(512); got != 512.0/10 {
		t.Fatalf("512 MB budget = %.1f, want %.1f", got, 512.0/10)
	}
	if got := tileBufferBudgetMB(8192); got != stackTileBufferMB {
		t.Fatalf("high-memory budget = %.0f, want %.0f", got, stackTileBufferMB)
	}
	full := estimateStackMemoryWithBudget(6000, 4000, 5, "sigma", true, stackTileBufferMB)
	small := estimateStackMemoryWithBudget(6000, 4000, 5, "sigma", true, 32)
	if full.NaiveMB-small.NaiveMB != 224 || full.RequiredMB-small.RequiredMB < 300 {
		t.Fatalf("budget delta not reflected: full=%+v small=%+v", full, small)
	}
}

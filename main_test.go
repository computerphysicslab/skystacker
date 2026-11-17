package main

import (
	"math/rand"
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

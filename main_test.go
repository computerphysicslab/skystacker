package main

import (
	"testing"
	"testing/synctest"
	"time"
)

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

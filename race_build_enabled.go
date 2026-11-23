//go:build race

package main

// raceBuild reports whether the race detector instrumentation is active.
// Latency assertions in the embedded self-tests use it to relax budgets that
// cannot be met while every memory access is instrumented.
const raceBuild = true

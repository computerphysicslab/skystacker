//go:build !race

package main

// raceBuild is false in normal builds, so embedded latency budgets stay strict.
const raceBuild = false

package main

import (
	"os"
	"testing"
)

// TestMain makes the Go test binary answer the three internal mock flags used
// by the embedded self-tests. Four of those cases spawn os.Executable() as a
// subprocess; under `go test` that is this test binary rather than the CLI, so
// without the dispatch below the subprocess exits with status 2. Production
// main() performs the same dispatch at startup; only the entry point differs.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case internalAISidecarFlag:
			os.Exit(runInternalAISidecarMock(os.Args[2:]))
		case internalAstrometryFlag:
			os.Exit(runInternalAstrometryMock(os.Args[2:]))
		case internalASTAPFlag:
			os.Exit(runInternalASTAPMock(os.Args[2:]))
		}
	}
	os.Exit(m.Run())
}

// TestEmbeddedSelftest exposes the binary's embedded self-test suite through
// the standard Go test runner. It deliberately calls runSelfTests instead of
// copying or rewriting its cases, so `go test` and `skystacker --selftest`
// exercise exactly the same suite. Extracting the cases into individual Go
// tests (P4-1) remains a separate refactor.
func TestEmbeddedSelftest(t *testing.T) {
	if failures := runSelfTests(); failures != 0 {
		t.Fatalf("embedded self-test suite reported %d failures", failures)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSolveFieldArgsTokenization(t *testing.T) {
	dir := t.TempDir()
	argvOut := filepath.Join(dir, "argv.txt")
	script := filepath.Join(dir, "mock-solve-field")
	body := `#!/usr/bin/env python3
import os, sys
open(os.environ["ARGV_OUT"], "w").write("\n".join(sys.argv[1:]))
for i, a in enumerate(sys.argv):
    if a == "--wcs" and i + 1 < len(sys.argv):
        open(sys.argv[i + 1], "wb").write(b"")
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGV_OUT", argvOut)
	_, _ = solveAstrometryLocal(filepath.Join(dir, "input.jpg"), script,
		"--odds-to-solve 1e3 --cpulimit '60 5' --guess-scale", 1.5, 5*time.Second)
	data, err := os.ReadFile(argvOut)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(string(data), "\n")
	joined := strings.Join(args, "|")
	for _, want := range []string{"--odds-to-solve|1e3", "--cpulimit|60 5", "--guess-scale", "--scale-low|1.2", "--no-plots", "--wcs"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q missing %q", joined, want)
		}
	}
}

func TestSolveFieldArgsRejectsUnterminatedQuote(t *testing.T) {
	_, err := solveAstrometryLocal("input.jpg", "solve-field", "--odds-to-solve 'unterminated", 0, time.Second)
	if err == nil || !strings.Contains(err.Error(), "solve-field-args") {
		t.Fatalf("expected solve-field-args error, got %v", err)
	}
}

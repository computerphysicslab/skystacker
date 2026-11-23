package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Product subfolders keep the output root readable when a run produces many
// survey or map artefacts. Survey cutouts, mosaics and the histogram-matched
// stacked preview live in survey/; every annotation or sky map lives in maps/.
const (
	surveyProductSubdir = "survey"
	mapProductSubdir    = "maps"
)

// ensureProductSubdir creates dir/<sub> when needed and returns its path.
func ensureProductSubdir(dir, sub string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("output layout: empty output directory")
	}
	path := filepath.Join(dir, sub)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("output layout: create %s: %w", path, err)
	}
	return path, nil
}

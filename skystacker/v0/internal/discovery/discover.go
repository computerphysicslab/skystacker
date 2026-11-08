package discovery

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Options controls file discovery.
type Options struct {
	Root      string
	Recursive bool
	Exclude   []string
}

// Discover returns absolute paths of JPG/JPEG files under Root, sorted
// lexicographically. Paths matching an entry of Exclude (and their
// descendants) are skipped.
func Discover(opts Options) ([]string, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	excludes := make(map[string]bool, len(opts.Exclude))
	for _, e := range opts.Exclude {
		abs, err := filepath.Abs(e)
		if err != nil {
			return nil, err
		}
		excludes[abs] = true
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if excludes[abs] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && !opts.Recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" {
			files = append(files, abs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

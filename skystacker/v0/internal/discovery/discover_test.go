package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverNonRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.JPG"))
	writeFile(t, filepath.Join(dir, "a.jpeg"))
	writeFile(t, filepath.Join(dir, "c.png"))
	writeFile(t, filepath.Join(dir, "sub", "d.jpg"))

	got, err := Discover(Options{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "a.jpeg"),
		filepath.Join(dir, "b.JPG"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiscoverRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.JPG"))
	writeFile(t, filepath.Join(dir, "sub", "d.jpg"))

	got, err := Discover(Options{Root: dir, Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "b.JPG"),
		filepath.Join(dir, "sub", "d.jpg"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDiscoverExclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.jpg"))
	writeFile(t, filepath.Join(dir, "out", "b.jpg"))

	got, err := Discover(Options{
		Root:      dir,
		Recursive: true,
		Exclude:   []string{filepath.Join(dir, "out")},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "a.jpg")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

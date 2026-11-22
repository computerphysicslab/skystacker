package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// testImageResize covers --image-op resize: directory expansion, area
// downsampling, output naming, atomic overwrite and EXIF-safe JPEG encoding.
func testImageResize() error {
	dir, err := os.MkdirTemp("", "skystacker-resize-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	srcDir := filepath.Join(dir, "lights")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return err
	}
	makeImage := func(path string, w, h int, jpegOut bool) error {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
			}
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if jpegOut {
			return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
		}
		return png.Encode(f, img)
	}
	pngPath := filepath.Join(srcDir, "a.png")
	jpgPath := filepath.Join(srcDir, "b.JPG")
	if err := makeImage(pngPath, 300, 210, false); err != nil {
		return err
	}
	if err := makeImage(jpgPath, 301, 201, true); err != nil {
		return err
	}
	outDir := filepath.Join(dir, "resized")
	opts := &cliOptions{ProcessImage: srcDir, ImageOp: "resize", Scale: 3, ImageOutput: outDir}
	if err := runResizeImages(opts); err != nil {
		return err
	}
	for _, name := range []string{"a_resize.png", "b_resize.jpg"} {
		path := filepath.Join(outDir, name)
		f, err := decodeImageFile(path, 0, 0)
		if err != nil {
			return err
		}
		wantW, wantH := 100, 70
		if strings.HasPrefix(name, "b") {
			wantW, wantH = 100, 67
		}
		if f.Width != wantW || f.Height != wantH {
			return fmt.Errorf("%s is %dx%d, want %dx%d", name, f.Width, f.Height, wantW, wantH)
		}
	}
	if err := runResizeImages(opts); err == nil || !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("expected an existing-output error, got %v", err)
	}
	opts.Overwrite = true
	if err := runResizeImages(opts); err != nil {
		return err
	}
	for _, path := range []string{pngPath, jpgPath} {
		f, err := decodeImageFile(path, 0, 0)
		if err != nil {
			return err
		}
		if f.Width != 100 || (path == pngPath && f.Height != 70) || (path == jpgPath && f.Height != 67) {
			return fmt.Errorf("overwritten %s is %dx%d", filepath.Base(path), f.Width, f.Height)
		}
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return fmt.Errorf("overwrite left %d files in the source folder", len(entries))
	}
	return nil
}

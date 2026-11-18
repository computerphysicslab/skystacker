package raw

import "time"

// Metadata holds EXIF and vendor-specific metadata common to all
// formats. Fields that a format does not provide remain zero-valued.
type Metadata struct {
	Make         string
	Model        string
	SerialNumber string
	LensModel    string

	ISO          int
	ExposureTime time.Duration // e.g. 1/250s = 4ms
	Aperture     float64       // f-number
	FocalLength  float64       // mm

	Orientation int // EXIF orientation 1–8
	DateTime    time.Time

	// Vendor holds format- or vendor-specific tags not covered above.
	// Keys are format-defined strings, e.g. "fuji.dynamic_range".
	Vendor map[string]any
}

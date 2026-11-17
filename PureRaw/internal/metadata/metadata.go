// Package metadata maps TIFF/Exif IFD entries into raw.Metadata.
// It deliberately covers only the small tag set needed by camera RAW
// containers; unknown tags remain the caller's responsibility.
package metadata

import (
	"errors"
	"time"

	"github.com/computerphysicslab/skystacker/PureRaw/internal/tiff"
	"github.com/computerphysicslab/skystacker/PureRaw/raw"
)

// FromTIFF builds raw.Metadata from a parsed TIFF document. A nil document
// or missing IFD0 is an error; individual missing tags are simply skipped.
func FromTIFF(doc *tiff.Document) (raw.Metadata, error) {
	if doc == nil || doc.IFD0 == nil {
		return raw.Metadata{}, errors.New("metadata: nil TIFF document")
	}
	order := doc.Order
	m := raw.Metadata{Vendor: map[string]any{}}
	all := []*tiff.IFD{doc.IFD0}
	if doc.Exif != nil {
		all = append(all, doc.Exif)
	}
	for _, ifd := range all {
		for tag, e := range ifd.Entries {
			switch tag {
			case 0x010F: // Make
				m.Make = e.String()
			case 0x0110: // Model
				m.Model = e.String()
			case 0x0112: // Orientation
				if v, err := e.Uint16(order, 0); err == nil {
					m.Orientation = int(v)
				}
			case 0x829A: // ExposureTime (seconds)
				if v, err := e.Rational(order, 0); err == nil && v > 0 {
					m.ExposureTime = time.Duration(v * float64(time.Second))
				}
			case 0x829D: // FNumber
				if v, err := e.Rational(order, 0); err == nil {
					m.Aperture = v
				}
			case 0x8827: // ISOSpeedRatings
				if v, err := e.Uint16(order, 0); err == nil {
					m.ISO = int(v)
				}
			case 0x920A: // FocalLength (mm)
				if v, err := e.Rational(order, 0); err == nil {
					m.FocalLength = v
				}
			case 0x0132: // DateTime
				if t, err := time.Parse("2006:01:02 15:04:05", e.String()); err == nil {
					m.DateTime = t
				}
			case 0xA431: // Exif.Photo.BodySerialNumber
				m.SerialNumber = e.String()
			case 0xA434: // Exif.Photo.LensModel
				m.LensModel = e.String()
			}
			// Keep all raw tags under a stable key for vendor-specific use.
			m.Vendor[tiff.TagName(tag)] = e
		}
	}
	return m, nil
}

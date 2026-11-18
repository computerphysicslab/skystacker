package arw

import (
	"io"

	"github.com/computerphysicslab/skystacker/goPureRaw/internal/metadata"
	"github.com/computerphysicslab/skystacker/goPureRaw/internal/tiff"
	"github.com/computerphysicslab/skystacker/goPureRaw/raw"
)

// ReadMetadata parses the standard TIFF/Exif metadata of an ARW file.
// It does not attempt to locate or decode the sensor track.
func ReadMetadata(r io.ReaderAt) (raw.Metadata, error) {
	doc, err := tiff.Read(r)
	if err != nil {
		return raw.Metadata{}, err
	}
	return metadata.FromTIFF(doc)
}

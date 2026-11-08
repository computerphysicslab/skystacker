package imaging

// Frame is a decoded image expressed as interleaved float32 RGB samples in [0,1].
//
// Pix holds Width*Height*3 values, laid out as:
//
//	Pix[(y*Width+x)*3 + channel]
//
// For grayscale sources the R, G and B channels carry identical values.
type Frame struct {
	Width  int
	Height int
	Pix    []float32
}

// At returns the value of channel c at pixel (x, y).
func (f *Frame) At(x, y, c int) float32 {
	return f.Pix[(y*f.Width+x)*3+c]
}

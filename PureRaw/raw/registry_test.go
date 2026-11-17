package raw

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type fakeDecoder struct {
	format Format
	probe  bool
	err    error
}

func (f fakeDecoder) Format() Format { return f.format }
func (f fakeDecoder) Probe(io.ReaderAt) bool {
	return f.probe
}
func (f fakeDecoder) Decode(io.ReaderAt, int64) (*Image, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &Image{Width: 1, Height: 1, BitDepth: 16}, nil
}

func TestFormatString(t *testing.T) {
	cases := map[Format]string{
		FormatUnknown: "unknown",
		FormatRAF:     "RAF",
		FormatARW:     "ARW",
		FormatCR3:     "CR3",
		Format(99):    "unknown",
	}
	for f, want := range cases {
		if got := f.String(); got != want {
			t.Errorf("Format(%d).String() = %q, want %q", f, got, want)
		}
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrUnsupportedFormat, ErrCorrupt) ||
		errors.Is(ErrCorrupt, ErrUnsupportedVariant) {
		t.Fatal("typed errors must be distinct")
	}
}

func TestRegistryDetectDecode(t *testing.T) {
	// The registry is global; tests must not assume a fixed length.
	before := len(decoders)
	Register(fakeDecoder{format: FormatRAF, probe: true})
	Register(fakeDecoder{format: FormatARW, probe: false})
	if len(decoders) != before+2 {
		t.Fatalf("registry grew by %d, want 2", len(decoders)-before)
	}
	d, err := Detect(bytes.NewReader([]byte{0}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Format() != FormatRAF {
		t.Fatalf("detected %s, want RAF", d.Format())
	}
	img, err := Decode(bytes.NewReader([]byte{0}), 1)
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != 1 || img.BitDepth != 16 {
		t.Fatalf("unexpected image: %+v", img)
	}
	// Restore registry state for other tests.
	mu.Lock()
	decoders = decoders[:before]
	mu.Unlock()
}

func TestRegisterDuplicatePanics(t *testing.T) {
	before := len(decoders)
	Register(fakeDecoder{format: FormatRAF, probe: true})
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate Register did not panic")
		}
		mu.Lock()
		decoders = decoders[:before]
		mu.Unlock()
	}()
	Register(fakeDecoder{format: FormatRAF, probe: false})
}

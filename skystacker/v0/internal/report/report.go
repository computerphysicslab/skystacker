package report

import (
	"fmt"
	"io"
	"time"
)

// Rejection records a file that could not be included in the stack.
type Rejection struct {
	Path   string
	Reason string
}

// Report summarises a processing run.
type Report struct {
	Version         string
	Timestamp       time.Time
	InputDir        string
	OutputDir       string
	Discovered      int
	Accepted        []string
	Rejected        []Rejection
	Width           int
	Height          int
	Channels        int
	Method          string
	AccumulatorType string
	OutputFormat    string
	OutputScaling   string
	OutputFiles     []string
	Workers         int
	Duration        time.Duration
	Notes           []string
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// WriteTo writes the report as plain text.
func (r *Report) WriteTo(w io.Writer) error {
	ew := &errWriter{w: w}
	ew.printf("SkyStacker %s\n", r.Version)
	ew.printf("Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))
	ew.printf("Input directory:  %s\n", r.InputDir)
	ew.printf("Output directory: %s\n\n", r.OutputDir)
	ew.printf("Discovered files: %d\n", r.Discovered)
	ew.printf("Accepted:         %d\n", len(r.Accepted))
	ew.printf("Rejected:         %d\n\n", len(r.Rejected))
	ew.printf("Image dimensions: %d x %d\n", r.Width, r.Height)
	ew.printf("Channels:         %d\n\n", r.Channels)
	ew.printf("Method:           %s\n", r.Method)
	ew.printf("Accumulator:      %s\n", r.AccumulatorType)
	ew.printf("Output format:    %s\n", r.OutputFormat)
	ew.printf("Output scaling:   %s\n", r.OutputScaling)
	ew.printf("Workers:          %d\n", r.Workers)
	ew.printf("Duration:         %s\n\n", r.Duration.Round(time.Millisecond))

	ew.printf("Output files:\n")
	for _, f := range r.OutputFiles {
		ew.printf("  %s\n", f)
	}
	ew.printf("\n")

	ew.printf("Accepted files:\n")
	for _, p := range r.Accepted {
		ew.printf("  %s\n", p)
	}
	ew.printf("\n")
	if len(r.Rejected) > 0 {
		ew.printf("Rejected files:\n")
		for _, rj := range r.Rejected {
			ew.printf("  %s: %s\n", rj.Path, rj.Reason)
		}
		ew.printf("\n")
	}
	ew.printf("Notes:\n")
	for _, n := range r.Notes {
		ew.printf("  - %s\n", n)
	}
	return ew.err
}

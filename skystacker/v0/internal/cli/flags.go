package cli

import (
	"flag"
	"fmt"
	"io"
	"runtime"
)

// Parsed holds the result of command-line parsing.
type Parsed struct {
	InputDir  string
	OutputDir string
	Workers   int
	Recursive bool
	Overwrite bool
	Version   bool
	Help      bool
}

// Parse parses command-line arguments. Standard flag semantics apply:
// flags must precede the optional positional directory argument.
func Parse(args []string) (*Parsed, error) {
	fs := flag.NewFlagSet("skystacker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	p := &Parsed{}
	fs.StringVar(&p.OutputDir, "output", "", "output directory (default: <input>/skystacker_results)")
	fs.IntVar(&p.Workers, "workers", runtime.NumCPU(), "maximum concurrent workers")
	fs.BoolVar(&p.Recursive, "recursive", false, "search subdirectories")
	fs.BoolVar(&p.Overwrite, "overwrite", false, "permit replacing files in the output directory")
	fs.BoolVar(&p.Version, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			p.Help = true
			return p, nil
		}
		return nil, err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return nil, fmt.Errorf("unexpected arguments: %v", rest[1:])
	}
	if len(rest) == 1 {
		p.InputDir = rest[0]
	} else {
		p.InputDir = "."
	}
	if p.Workers < 1 {
		return nil, fmt.Errorf("--workers must be >= 1, got %d", p.Workers)
	}
	return p, nil
}

func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: skystacker [flags] [input-directory]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Combine JPG/JPEG astrophotography frames into sum and mean images.")
	fmt.Fprintln(w, "Frames are combined in their existing pixel coordinate system; no")
	fmt.Fprintln(w, "star registration or calibration is performed in v0.1.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  input-directory   directory containing JPG/JPEG files (default: .)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --output string   output directory (default: <input>/skystacker_results)")
	fmt.Fprintln(w, "  --workers int     maximum concurrent workers (default: NumCPU)")
	fmt.Fprintln(w, "  --recursive       search subdirectories")
	fmt.Fprintln(w, "  --overwrite       permit replacing files in the output directory")
	fmt.Fprintln(w, "  --version         print version and exit")
	fmt.Fprintln(w, "  --help            print help")
}

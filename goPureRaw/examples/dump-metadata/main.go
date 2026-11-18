// Command dump-metadata prints the detected format and standard
// metadata for a RAW file. It exercises the registry and the format
// decoders, and is a useful smoke test as decoders land.
package main

import (
	"fmt"
	"os"

	"github.com/computerphysicslab/skystacker/goPureRaw/raw"

	_ "github.com/computerphysicslab/skystacker/goPureRaw/formats/arw"
	_ "github.com/computerphysicslab/skystacker/goPureRaw/formats/cr3"
	_ "github.com/computerphysicslab/skystacker/goPureRaw/formats/raf"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: dump-metadata <file.raw>")
		os.Exit(2)
	}
	path := os.Args[1]

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	d, err := raw.Detect(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("format: %s\n", d.Format())

	img, err := d.Decode(f, st.Size())
	if err != nil {
		// Stub decoders return ErrUnsupportedVariant until implemented.
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	md := img.Metadata
	fmt.Printf("make:       %s\n", md.Make)
	fmt.Printf("model:      %s\n", md.Model)
	fmt.Printf("dimensions: %dx%d @ %d bits\n", img.Width, img.Height, img.BitDepth)
	fmt.Printf("cfa:        %c%c/%c%c\n",
		img.CFAPattern[0][0], img.CFAPattern[0][1],
		img.CFAPattern[1][0], img.CFAPattern[1][1])
}

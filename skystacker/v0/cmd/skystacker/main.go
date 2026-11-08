package main

import (
	"context"
	"fmt"
	"os"

	"github.com/skystacker/skystacker/internal/app"
	"github.com/skystacker/skystacker/internal/cli"
)

func main() {
	parsed, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		fmt.Fprintln(os.Stderr, "Run 'skystacker --help' for usage.")
		os.Exit(2)
	}
	if parsed.Help {
		cli.PrintUsage(os.Stdout)
		return
	}
	if parsed.Version {
		fmt.Println(app.Version)
		return
	}

	opts := app.Options{
		InputDir:  parsed.InputDir,
		OutputDir: parsed.OutputDir,
		Workers:   parsed.Workers,
		Recursive: parsed.Recursive,
		Overwrite: parsed.Overwrite,
	}
	if err := app.Run(context.Background(), opts); err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		os.Exit(1)
	}
}

package main

import (
	"flag"
	"slices"
	"testing"
)

// Item 109: the positional preprocessor must derive value-taking flags from
// the registered FlagSet, including custom flag.Value implementations.
func TestRegisteredValueFlagNames(t *testing.T) {
	fs := flag.NewFlagSet("flags", flag.ContinueOnError)
	fs.Bool("switch", false, "")
	fs.String("text", "", "")
	fs.Int("count", 0, "")
	fs.Func("custom", "", func(string) error { return nil })
	fs.BoolFunc("trigger", "", func(string) error { return nil })

	got := registeredValueFlagNames(fs)
	for _, name := range []string{"text", "count", "custom"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("value flag %q missing from %v", name, got)
		}
	}
	for _, name := range []string{"switch", "trigger"} {
		if _, ok := got[name]; ok {
			t.Fatalf("bool flag %q misclassified in %v", name, got)
		}
	}
}

func TestReorderArgsForParse(t *testing.T) {
	valueFlags := map[string]struct{}{
		"workers": {}, "output": {}, "method": {}, "negative": {},
	}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"flag after positional", []string{"lights", "--method", "mean"}, []string{"--method", "mean", "lights"}},
		{"attached value", []string{"--workers=2", "lights"}, []string{"--workers=2", "lights"}},
		{"single dash", []string{"-workers", "2", "lights"}, []string{"-workers", "2", "lights"}},
		{"negative value", []string{"--negative", "-2.5", "lights"}, []string{"--negative", "-2.5", "lights"}},
		{"terminator", []string{"lights", "--", "--workers", "2"}, []string{"--", "lights", "--workers", "2"}},
		{"bool before value flag", []string{"lights", "--switch", "--output", "out"}, []string{"--switch", "--output", "out", "lights"}},
		{"flag-looking value", []string{"--output", "--switch", "lights"}, []string{"--output", "--switch", "lights"}},
		{"bare dash", []string{"-"}, []string{"-"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := reorderArgsForParse(c.in, valueFlags)
			if !slices.Equal(got, c.want) {
				t.Fatalf("reorder(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

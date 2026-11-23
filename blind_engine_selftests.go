package main

// Embedded regression coverage for the blind-solver scale gate (roadmap item
// 115). The Go unit tests live in blind_index_test.go; this copy runs inside
// skystacker --selftest so the shipped binary carries the same guarantee.

import (
	"errors"
	"fmt"
)

// testScaleConsistentMatchesRobust places the false correspondence at index 0.
// The old implementation anchored on matches[0] and discarded the three valid
// pairs; the robust consensus must keep exactly those three.
func testScaleConsistentMatchesRobust() error {
	frame := []star{
		{X: 10, Y: 10, HFR: 2},
		{X: 20, Y: 20, HFR: 2},
		{X: 30, Y: 30, HFR: 2},
		{X: 40, Y: 40, HFR: 2},
	}
	catalog := []gaiaStar{
		{RA: 1.050, Dec: 0.550}, // false match, deliberately first
		{RA: 1.001, Dec: 0.501},
		{RA: 1.002, Dec: 0.502},
		{RA: 1.003, Dec: 0.503},
	}
	matches := []matchPair{{0, 0}, {1, 1}, {2, 2}, {3, 3}}
	pruned := scaleConsistentMatches(matches, frame, catalog, 0.2)
	if len(pruned) != 3 {
		return fmt.Errorf("false first anchor: got %d matches %+v, want 3", len(pruned), pruned)
	}
	for _, m := range pruned {
		if m.FrameIdx == 0 || m.GaiaIdx == 0 {
			return fmt.Errorf("false correspondence survived: %+v", m)
		}
	}
	// Out-of-range indices must be skipped rather than panicking, and the gate
	// must remain deterministic under repeated calls.
	broken := []matchPair{{99, 0}, {1, 99}, {1, 1}, {2, 2}, {3, 3}}
	first := scaleConsistentMatches(broken, frame, catalog, 0.35)
	second := scaleConsistentMatches(broken, frame, catalog, 0.35)
	if len(first) < 3 || len(first) != len(second) {
		return fmt.Errorf("invalid-index gate: %+v vs %+v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			return errors.New("scale gate is not deterministic")
		}
		if first[i].FrameIdx == 99 || first[i].GaiaIdx == 99 {
			return fmt.Errorf("invalid correspondence survived: %+v", first[i])
		}
	}
	// A field with a genuinely uniform scale must keep every correspondence.
	for i := range catalog {
		catalog[i].RA = 1.0 + 0.001*float64(i)
		catalog[i].Dec = 0.5 + 0.001*float64(i)
	}
	all := scaleConsistentMatches(matches, frame, catalog, 0.2)
	if len(all) != len(matches) {
		return fmt.Errorf("uniform scale: kept %d/%d", len(all), len(matches))
	}
	for i, m := range all {
		if m != matches[i] {
			return fmt.Errorf("uniform scale changed order at %d: %+v", i, m)
		}
	}
	return nil
}

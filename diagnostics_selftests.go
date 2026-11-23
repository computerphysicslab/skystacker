package main

// Embedded regression for the goDiagnostics redaction and lifetime-CPU
// labelling contract (roadmap items 116 and 117). The formula and parser
// unit tests live in the module itself; this copy locks the public report
// contract in the shipped binary.

import (
	"encoding/json"
	"errors"
	"strings"

	diagnostics "github.com/computerphysicslab/skystacker/goDiagnostics"
)

func testDiagnosticsReportRedaction() error {
	r := diagnostics.Report{Processes: []diagnostics.ProcessInfo{{
		PID: 7, Cmd: "skystacker --mast-token hunter2 --selftest",
		RSSMB: 42, LifetimeCPUSharePct: 12.5,
	}}}
	// Lock the public report shapes produced by the module's Collect path.
	text := r.Text()
	if !strings.Contains(text, "lifetime CPU share 12.5% (since boot)") {
		return errors.New("diagnostics text lost the explicit lifetime CPU label")
	}
	b, err := r.JSON()
	if err != nil {
		return err
	}
	var decoded struct {
		Processes []struct {
			LifetimeCPUSharePct float64 `json:"lifetime_cpu_share_pct"`
		} `json:"processes"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		return err
	}
	if len(decoded.Processes) != 1 || decoded.Processes[0].LifetimeCPUSharePct != 12.5 {
		return errors.New("diagnostics JSON lost lifetime_cpu_share_pct")
	}
	// Lock the actual redaction policy exposed by the diagnostics module:
	// credentials disappear, ordinary flags stay readable.
	cmd := diagnostics.RedactProcessCmd("skystacker --mast-token hunter2 --selftest", diagnostics.RedactValue)
	if strings.Contains(cmd, "hunter2") || !strings.Contains(cmd, "***REDACTED***") ||
		!strings.Contains(cmd, "--selftest") {
		return errors.New("diagnostics process cmd redaction failed: " + cmd)
	}
	return nil
}

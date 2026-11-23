package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakePlatform struct {
	failMemory bool
	procs      []ProcessInfo
}

func (fakePlatform) Host(context.Context) (int64, string, error) {
	return 3600, "2026-11-18T00:00:00Z", nil
}
func (fakePlatform) OS(context.Context) (OSInfo, error) {
	return OSInfo{Name: "FakeOS", Version: "1.0", GOOS: "test", GOARCH: "amd64"}, nil
}
func (fakePlatform) CPU(context.Context) (CPUInfo, error) {
	return CPUInfo{Model: "Fake CPU", Logical: 8, Physical: 4, Load1: 0.5}, nil
}
func (f fakePlatform) Memory(context.Context) (MemoryInfo, error) {
	if f.failMemory {
		return MemoryInfo{}, errors.New("memory unavailable")
	}
	return MemoryInfo{TotalMB: 1024, AvailableMB: 512, UsedMB: 512}, nil
}
func (fakePlatform) Disks(context.Context) ([]DiskInfo, error) {
	return []DiskInfo{{Mount: "/b", TotalBytes: 200, FreeBytes: 100}, {Mount: "/a", TotalBytes: 100, FreeBytes: 50}}, nil
}
func (f fakePlatform) Processes(context.Context, int) ([]ProcessInfo, error) {
	if f.procs != nil {
		return f.procs, nil
	}
	return []ProcessInfo{{PID: 1, RSSMB: 50}, {PID: 2, RSSMB: 100, Self: true}}, nil
}
func (fakePlatform) Packages(context.Context) ([]PackageInfo, error) {
	return []PackageInfo{{Name: "pkg", Version: "1", Source: "fake"}}, nil
}

func TestRedactValue(t *testing.T) {
	if got := RedactValue("skystacker-token-abcdefghijklmnopqrstuvwxyz"); got != "***REDACTED***" {
		t.Fatalf("token not redacted: %q", got)
	}
	if got := RedactValue("https://user:password@example.com/path"); strings.Contains(got, "password") {
		t.Fatalf("URL credentials not redacted: %q", got)
	}
	if got := RedactValue("en_US.UTF-8"); got != "en_US.UTF-8" {
		t.Fatalf("ordinary value changed: %q", got)
	}
}

func TestCollectWithFakePlatform(t *testing.T) {
	old := platformFactory
	defer func() { platformFactory = old }()
	platformFactory = func() platform { return fakePlatform{failMemory: true} }

	o := DefaultOptions()
	o.ProgramDir = t.TempDir()
	o.IncludePackages = true
	o.Timeout = 2 * time.Second
	r := Collect(context.Background(), o)
	if r.Host.Hostname == "" || r.OS.Name != "FakeOS" || r.CPU.Model != "Fake CPU" {
		t.Fatalf("unexpected core report: %+v", r)
	}
	if len(r.Disks) != 2 || r.Disks[0].Mount != "/a" {
		t.Fatalf("disks not sorted: %+v", r.Disks)
	}
	if len(r.Errors) == 0 || r.Errors[0].Collector != "memory" {
		t.Fatalf("partial collector error missing: %+v", r.Errors)
	}
	if len(r.Packages) != 1 || len(r.Processes) != 2 {
		t.Fatalf("packages/processes missing: %+v", r)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.OS.Name != "FakeOS" || len(back.Errors) != 1 {
		t.Fatalf("JSON round trip failed: %+v", back)
	}
	text := r.Text()
	for _, want := range []string{"SkyStacker diagnostics", "FakeOS", "memory"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text report missing %q:\n%s", want, text)
		}
	}
}

func TestOptionsNormalize(t *testing.T) {
	o := Options{}
	o.normalize()
	if o.Timeout <= 0 || o.CmdTimeout <= 0 || o.TopProcesses <= 0 || o.MaxWalkFiles <= 0 || o.Redact == nil || o.ProgramDir == "" {
		t.Fatalf("defaults not applied: %+v", o)
	}
}

func TestCollectNeverPanics(t *testing.T) {
	old := platformFactory
	defer func() { platformFactory = old }()
	platformFactory = func() platform { return nil }
	_ = Collect(context.Background(), DefaultOptions())
}

// Item 116: process command lines must not leak credentials into diagnostics.
func TestRedactProcessCmd(t *testing.T) {
	cmd := "skystacker --api_key=secret --mast-token hunter2 --config /tmp/a.json"
	got := RedactProcessCmd(cmd, RedactValue)
	for _, forbidden := range []string{"secret", "hunter2"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("credential leaked into %q", got)
		}
	}
	for _, want := range []string{"skystacker", "--api_key=***REDACTED***", "--mast-token", "--config /tmp/a.json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted cmd %q is missing %q", got, want)
		}
	}
	if strings.Count(got, redactedMarker) != 2 {
		t.Fatalf("expected two masked values: %q", got)
	}
	// The caller-supplied policy must see ordinary tokens too.
	called := false
	custom := RedactProcessCmd("plain --flag value", func(v string) string {
		called = true
		if v == "value" {
			return "X"
		}
		return v
	})
	if !called || custom != "plain --flag X" {
		t.Fatalf("custom redact policy ignored: %q", custom)
	}
	if RedactProcessCmd("", nil) != "" || RedactProcessCmd("skystacker --selftest", nil) != "skystacker --selftest" {
		t.Fatal("empty/nil-redact behaviour changed")
	}
	// URL credentials embedded in an argument are masked by RedactValue.
	urlCmd := RedactProcessCmd("curl https://user:password@example.com/x", RedactValue)
	if strings.Contains(urlCmd, "password") || !strings.Contains(urlCmd, redactedMarker) {
		t.Fatalf("URL credentials not redacted: %q", urlCmd)
	}
}

func TestCollectRedactsProcessCmds(t *testing.T) {
	old := platformFactory
	defer func() { platformFactory = old }()
	platformFactory = func() platform {
		return fakePlatform{procs: []ProcessInfo{{PID: 7, Cmd: "skystacker --mast-token hunter2 --selftest", Self: true}}}
	}
	o := DefaultOptions()
	o.ProgramDir = t.TempDir()
	r := Collect(context.Background(), o)
	if len(r.Processes) != 1 || strings.Contains(r.Processes[0].Cmd, "hunter2") {
		t.Fatalf("process cmd not redacted: %+v", r.Processes)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	text := r.Text()
	for _, blob := range []string{string(b), text} {
		if strings.Contains(blob, "hunter2") {
			t.Fatalf("credential leaked in report:\n%s", blob)
		}
		if !strings.Contains(blob, "--selftest") {
			t.Fatalf("ordinary flag removed from report:\n%s", blob)
		}
	}
}

// Item 117: swap parsing and the lifetime CPU label must be stable.
func TestParseDarwinSwapUsage(t *testing.T) {
	total, used, ok := parseDarwinSwapUsage("total = 1024.00M  used = 512.00M  free = 512.00M")
	if !ok || total != 1024 || used != 512 {
		t.Fatalf("swap usage: total=%.1f used=%.1f ok=%v", total, used, ok)
	}
	total, used, ok = parseDarwinSwapUsage("total = 2.00G used = 1024K free = 1.99G")
	if !ok || total != 2048 || used != 1 {
		t.Fatalf("swap suffixes: total=%.1f used=%.1f ok=%v", total, used, ok)
	}
	if _, _, ok := parseDarwinSwapUsage("vm_stat: no swap data"); ok {
		t.Fatal("garbage swap output accepted")
	}
	if got := lifetimeCPUSharePct(25, 100); got != 25 {
		t.Fatalf("lifetime share formula: %.2f", got)
	}
	if got := lifetimeCPUSharePct(300, 100); got != 100 {
		t.Fatalf("lifetime share not clamped: %.2f", got)
	}
	if got := lifetimeCPUSharePct(10, 0); got != 0 {
		t.Fatalf("zero total ticks: %.2f", got)
	}
}

func TestLifetimeCPUShareLabelledInText(t *testing.T) {
	r := Report{Processes: []ProcessInfo{{
		PID: 1, Cmd: "skystacker --selftest", RSSMB: 42,
		CPUPct: 3, LifetimeCPUSharePct: 12.5,
	}}}
	text := r.Text()
	if !strings.Contains(text, "lifetime CPU share 12.5% (since boot)") {
		t.Fatalf("explicit lifetime label missing:\n%s", text)
	}
	if strings.Contains(text, "CPU 12.5%") {
		t.Fatalf("lifetime share printed as current CPU:\n%s", text)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Processes) != 1 || back.Processes[0].LifetimeCPUSharePct != 12.5 {
		t.Fatalf("JSON round trip lost lifetime share: %s", b)
	}
	if !strings.Contains(string(b), `"lifetime_cpu_share_pct": 12.5`) ||
		strings.Contains(string(b), `"cpu_pct": 12.5`) {
		t.Fatalf("lifetime share must have its own JSON field: %s", b)
	}
}

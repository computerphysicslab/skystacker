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
func (fakePlatform) Processes(context.Context, int) ([]ProcessInfo, error) {
	return []ProcessInfo{{PID: 1, RSSMB: 50}, {PID: 2, RSSMB: 100, Self: true}}, nil
}
func (fakePlatform) Updates(context.Context) ([]PackageInfo, error) {
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
	o.IncludeUpdates = true
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
	if len(r.Updates) != 1 || len(r.Processes) != 2 {
		t.Fatalf("updates/processes missing: %+v", r)
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

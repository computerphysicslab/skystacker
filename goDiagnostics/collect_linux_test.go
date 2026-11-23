//go:build linux

package diagnostics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessesFromProcSyntheticTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "stat"), []byte("cpu  10 0 10 0 0 0 0 0 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		pid   string
		rss   string
		ticks string
		cmd   string
	}{
		{"100", "2048 kB", "5 2", "skystacker --selftest"},
		{"101", "1024 kB", "1 1", "other --flag"},
	} {
		dir := filepath.Join(root, tc.pid)
		if err := os.MkdirAll(filepath.Join(dir, "fd"), 0o700); err != nil {
			t.Fatal(err)
		}
		status := "Name:\tproc\nVmRSS:\t" + tc.rss + "\nThreads:\t3\n"
		if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o600); err != nil {
			t.Fatal(err)
		}
		stat := "1 (proc) S 9 0 0 0 0 0 0 0 0 0 " + tc.ticks + " 0 0 20 0\n"
		if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(tc.cmd+"\x00"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, fd := range []string{"0", "1", "2"} {
			if err := os.WriteFile(filepath.Join(dir, "fd", fd), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	procs, err := processesFromProc(context.Background(), root, 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 2 || procs[0].PID != 100 || procs[0].RSSMB != 2 || procs[0].Threads != 3 {
		t.Fatalf("unexpected processes: %+v", procs)
	}
	if procs[0].PPID != 9 || procs[0].FDs != 3 || !procs[0].Self || procs[1].Self {
		t.Fatalf("process metadata lost: %+v", procs)
	}
	if procs[0].Cmd != "skystacker --selftest" || procs[0].LifetimeCPUSharePct != 35 {
		t.Fatalf("cmd/share wrong: %+v", procs[0])
	}
	if _, err := processesFromProc(context.Background(), filepath.Join(root, "missing"), 100, 10); err == nil {
		t.Fatal("missing proc root accepted")
	}
}

func TestProcessesFromProcHonoursCancellation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	procs, err := processesFromProc(ctx, root, 1, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan error: %v", err)
	}
	if len(procs) != 0 {
		t.Fatalf("cancelled scan returned processes: %+v", procs)
	}
	if got := countProcFDs(ctx, filepath.Join(root, "missing")); got != 0 {
		t.Fatalf("missing fd directory counted %d", got)
	}
}

// Package diagnostics collects a read-only snapshot of the host and of the
// running Go program. It has no third-party dependencies and is safe to copy
// into another Go project: add the folder and, in that project's go.mod,
// require the module plus a local replace directive.
package diagnostics

import (
	"context"
	"errors"
	"sort"
	"time"
)

// ErrUnsupported is returned by platform collectors that have no
// implementation for the current operating system.
var ErrUnsupported = errors.New("diagnostics: platform collector not implemented")

// Options controls how much information Collect gathers and how long it may
// take. Zero values are replaced by the defaults from DefaultOptions.
type Options struct {
	Timeout        time.Duration
	CmdTimeout     time.Duration
	ProgramDir     string
	TopProcesses   int
	MaxWalkFiles   int
	IncludeUpdates bool
	IncludeEnv     bool
	IncludeProfile bool
	Redact         func(string) string
}

// DefaultOptions returns conservative values suitable for a production run.
func DefaultOptions() Options {
	return Options{
		Timeout:      5 * time.Second,
		CmdTimeout:   2 * time.Second,
		TopProcesses: 10,
		MaxWalkFiles: 50_000,
		IncludeEnv:   true,
		Redact:       RedactValue,
	}
}

func (o *Options) normalize() {
	d := DefaultOptions()
	if o.Timeout <= 0 {
		o.Timeout = d.Timeout
	}
	if o.CmdTimeout <= 0 {
		o.CmdTimeout = d.CmdTimeout
	}
	if o.TopProcesses <= 0 {
		o.TopProcesses = d.TopProcesses
	}
	if o.MaxWalkFiles <= 0 {
		o.MaxWalkFiles = d.MaxWalkFiles
	}
	if o.Redact == nil {
		o.Redact = d.Redact
	}
	if o.ProgramDir == "" {
		o.ProgramDir = "."
	}
}

// CollectorError records a partial failure. A failing collector never aborts
// the whole report.
type CollectorError struct {
	Collector string `json:"collector"`
	Reason    string `json:"reason"`
}

type HostInfo struct {
	Hostname  string `json:"hostname"`
	User      string `json:"user"`
	Home      string `json:"home"`
	TempDir   string `json:"temp_dir"`
	UptimeSec int64  `json:"uptime_sec"`
	BootTime  string `json:"boot_time"`
}

type OSInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Kernel    string `json:"kernel"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	Timezone  string `json:"timezone"`
	Container bool   `json:"container"`
}

type RuntimeInfo struct {
	GoVersion    string    `json:"go_version"`
	Module       string    `json:"module"`
	VCSRevision  string    `json:"vcs_revision"`
	VCSTime      string    `json:"vcs_time"`
	VCSModified  bool      `json:"vcs_modified"`
	CGOEnabled   string    `json:"cgo_enabled"`
	GOGC         string    `json:"gogc"`
	GOMAXPROCS   int       `json:"gomaxprocs"`
	NumCPU       int       `json:"num_cpu"`
	Goroutines   uint64    `json:"goroutines"`
	NumGC        uint64    `json:"num_gc"`
	GCPausesMS   []float64 `json:"gc_pauses_ms"`
	HeapMB       float64   `json:"heap_mb"`
	SysMB        float64   `json:"sys_mb"`
	TotalAllocMB float64   `json:"total_alloc_mb"`
}

type CPUInfo struct {
	Model    string  `json:"model"`
	Logical  int     `json:"logical"`
	Physical int     `json:"physical"`
	MHz      float64 `json:"mhz"`
	Load1    float64 `json:"load1"`
	Load5    float64 `json:"load5"`
	Load15   float64 `json:"load15"`
}

type MemoryInfo struct {
	TotalMB     float64  `json:"total_mb"`
	AvailableMB float64  `json:"available_mb"`
	UsedMB      float64  `json:"used_mb"`
	CachedMB    float64  `json:"cached_mb"`
	SwapTotalMB float64  `json:"swap_total_mb"`
	SwapUsedMB  float64  `json:"swap_used_mb"`
	CgroupLimit *float64 `json:"cgroup_limit_mb"`
}

type DiskInfo struct {
	Mount      string `json:"mount"`
	FSType     string `json:"fs_type"`
	TotalBytes uint64 `json:"total_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
	Path       string `json:"path,omitempty"`
	Files      int    `json:"files,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
	Skipped    int    `json:"skipped,omitempty"`
}

type ProcessInfo struct {
	PID   int     `json:"pid"`
	PPID  int     `json:"ppid"`
	User  string  `json:"user"`
	Cmd   string  `json:"cmd"`
	RSSMB float64 `json:"rss_mb"`
	// CPUPct is the platform-provided instantaneous/average CPU percentage.
	// Linux does not populate it: the kernel exposes only cumulative ticks.
	CPUPct float64 `json:"cpu_pct,omitempty"`
	// LifetimeCPUSharePct is the share of all CPU time since boot (Linux only).
	// It is never presented as an instantaneous utilisation in Text().
	LifetimeCPUSharePct float64 `json:"lifetime_cpu_share_pct,omitempty"`
	Threads             int     `json:"threads,omitempty"`
	FDs                 int     `json:"fds,omitempty"`
	Self                bool    `json:"self,omitempty"`
}

type EnvInfo struct {
	Vars     map[string]string `json:"vars"`
	ProxySet bool              `json:"proxy_set"`
}

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

type ToolInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Found   bool   `json:"found"`
}

type ProgramInfo struct {
	Executable string `json:"executable"`
	SHA256     string `json:"sha256"`
	SizeBytes  int64  `json:"size_bytes"`
	WorkingDir string `json:"working_dir"`
	DirBytes   int64  `json:"dir_bytes"`
	DirFiles   int    `json:"dir_files"`
	DirSkipped int    `json:"dir_skipped"`
}

type NetInfo struct {
	Name      string   `json:"name"`
	MAC       string   `json:"mac"`
	MTU       int      `json:"mtu"`
	Addresses []string `json:"addresses"`
}

// Report is the complete diagnostic snapshot.
type Report struct {
	GeneratedAt time.Time        `json:"generated_at"`
	DurationMS  int64            `json:"duration_ms"`
	Host        HostInfo         `json:"host"`
	OS          OSInfo           `json:"os"`
	Runtime     RuntimeInfo      `json:"runtime"`
	CPU         CPUInfo          `json:"cpu"`
	Memory      MemoryInfo       `json:"memory"`
	Disks       []DiskInfo       `json:"disks"`
	Processes   []ProcessInfo    `json:"processes"`
	Environment EnvInfo          `json:"environment"`
	Updates     []PackageInfo    `json:"updates"`
	Tools       []ToolInfo       `json:"tools"`
	Program     ProgramInfo      `json:"program"`
	Network     []NetInfo        `json:"network"`
	Errors      []CollectorError `json:"errors"`
}

// Collect gathers the snapshot. It always returns a Report; individual
// collector failures are recorded in Report.Errors.
func Collect(ctx context.Context, o Options) Report {
	o.normalize()
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	r := Report{
		GeneratedAt: start.UTC(),
		Disks:       []DiskInfo{},
		Processes:   []ProcessInfo{},
		Updates:     []PackageInfo{},
		Tools:       []ToolInfo{},
		Network:     []NetInfo{},
		Errors:      []CollectorError{},
	}
	r.Runtime.GCPausesMS = []float64{}
	p := newPlatform()
	run := func(name string, fn func()) {
		defer func() {
			if v := recover(); v != nil {
				r.Errors = append(r.Errors, CollectorError{Collector: name, Reason: "panic: " + shortError(v)})
			}
		}()
		fn()
	}

	run("host", func() { r.Host = collectHost(p, ctx) })
	run("os", func() {
		var err error
		r.OS, err = p.OS(ctx)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			r.Errors = append(r.Errors, CollectorError{Collector: "os", Reason: err.Error()})
		}
	})
	run("runtime", func() { r.Runtime = collectRuntime() })
	run("cpu", func() {
		var err error
		r.CPU, err = p.CPU(ctx)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			r.Errors = append(r.Errors, CollectorError{Collector: "cpu", Reason: err.Error()})
		}
	})
	run("memory", func() {
		var err error
		r.Memory, err = p.Memory(ctx)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			r.Errors = append(r.Errors, CollectorError{Collector: "memory", Reason: err.Error()})
		}
	})
	run("disks", func() {
		var err error
		r.Disks, err = p.Disks(ctx)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			r.Errors = append(r.Errors, CollectorError{Collector: "disks", Reason: err.Error()})
		}
	})
	run("processes", func() {
		var err error
		r.Processes, err = p.Processes(ctx, o.TopProcesses)
		if err != nil && !errors.Is(err, ErrUnsupported) {
			r.Errors = append(r.Errors, CollectorError{Collector: "processes", Reason: err.Error()})
		}
		redactProcessCmds(r.Processes, o.Redact)
	})
	if o.IncludeUpdates {
		run("updates", func() {
			var err error
			r.Updates, err = p.Updates(ctx)
			if err != nil && !errors.Is(err, ErrUnsupported) {
				r.Errors = append(r.Errors, CollectorError{Collector: "updates", Reason: err.Error()})
			}
		})
	}
	run("tools", func() { r.Tools = collectTools(ctx, o.CmdTimeout) })
	if o.IncludeEnv {
		run("environment", func() { r.Environment = collectEnv(o.Redact) })
	}
	run("program", func() { r.Program = collectProgram(ctx, o) })
	run("network", func() { r.Network = collectNetwork() })
	sortReport(&r)
	r.DurationMS = time.Since(start).Milliseconds()
	return r
}

func sortReport(r *Report) {
	sort.Slice(r.Disks, func(i, j int) bool { return r.Disks[i].Mount < r.Disks[j].Mount })
	sort.Slice(r.Processes, func(i, j int) bool {
		if r.Processes[i].RSSMB == r.Processes[j].RSSMB {
			return r.Processes[i].PID < r.Processes[j].PID
		}
		return r.Processes[i].RSSMB > r.Processes[j].RSSMB
	})
	sort.Slice(r.Updates, func(i, j int) bool { return r.Updates[i].Name < r.Updates[j].Name })
	sort.Slice(r.Tools, func(i, j int) bool { return r.Tools[i].Name < r.Tools[j].Name })
	sort.Slice(r.Network, func(i, j int) bool { return r.Network[i].Name < r.Network[j].Name })
	sort.Slice(r.Errors, func(i, j int) bool { return r.Errors[i].Collector < r.Errors[j].Collector })
}

func shortError(v any) string {
	s := ""
	switch t := v.(type) {
	case error:
		s = t.Error()
	case string:
		s = t
	default:
		s = "unknown"
	}
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

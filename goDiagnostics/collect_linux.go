//go:build linux

package diagnostics

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type linuxPlatform struct{}

func newPlatformImpl() platform { return linuxPlatform{} }

func readFileFields(path string) map[string]string {
	out := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(line, ':'); i > 0 {
			out[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		} else if i := strings.IndexByte(line, '='); i > 0 {
			out[strings.TrimSpace(line[:i])] = strings.Trim(strings.TrimSpace(line[i+1:]), `"`)
		}
	}
	return out
}

func (linuxPlatform) Host(ctx context.Context) (int64, string, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, "", os.ErrInvalid
	}
	sec := int64(parseFloat(fields[0]))
	boot := time.Now().Add(-time.Duration(sec) * time.Second).UTC().Format(time.RFC3339)
	return sec, boot, nil
}

func (linuxPlatform) OS(ctx context.Context) (OSInfo, error) {
	o := OSInfo{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Timezone: timezoneName()}
	release := readFileFields("/etc/os-release")
	o.Name = release["NAME"]
	o.Version = release["VERSION"]
	if o.Name == "" {
		o.Name = release["PRETTY_NAME"]
	}
	if kernel, err := runCommand(ctx, time.Second, "uname", "-r"); err == nil {
		o.Kernel = kernel
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		o.Container = true
	}
	if cgroup, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		low := strings.ToLower(string(cgroup))
		o.Container = o.Container || strings.Contains(low, "docker") || strings.Contains(low, "containerd") || strings.Contains(low, "kubepods")
	}
	return o, nil
}

func (linuxPlatform) CPU(ctx context.Context) (CPUInfo, error) {
	c := CPUInfo{Logical: runtime.NumCPU()}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return c, err
	}
	pairs := make(map[string]bool)
	for _, block := range strings.Split(string(data), "\n\n") {
		fields := readFileFieldsFrom(block)
		if c.Model == "" {
			c.Model = fields["model name"]
			c.MHz = parseFloat(fields["cpu MHz"])
		}
		phys, core := fields["physical id"], fields["core id"]
		if phys != "" || core != "" {
			pairs[phys+":"+core] = true
		}
	}
	if len(pairs) > 0 {
		c.Physical = len(pairs)
	} else {
		c.Physical = c.Logical
	}
	if load, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(load))
		if len(fields) >= 3 {
			c.Load1, c.Load5, c.Load15 = parseFloat(fields[0]), parseFloat(fields[1]), parseFloat(fields[2])
		}
	}
	return c, nil
}

func readFileFieldsFrom(block string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(block, "\n") {
		if i := strings.IndexByte(line, ':'); i > 0 {
			out[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	return out
}

func cgroupMemoryLimit() *float64 {
	for _, path := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(data))
		if s == "max" {
			continue
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil || v >= 1<<62 {
			continue
		}
		mb := float64(v) / (1024 * 1024)
		return &mb
	}
	return nil
}

func (linuxPlatform) Memory(ctx context.Context) (MemoryInfo, error) {
	f := readFileFields("/proc/meminfo")
	kb := func(key string) float64 {
		fields := strings.Fields(f[key])
		if len(fields) == 0 {
			return 0
		}
		return parseFloat(fields[0]) / 1024
	}
	m := MemoryInfo{
		TotalMB: kb("MemTotal"), AvailableMB: kb("MemAvailable"),
		CachedMB: kb("Cached"), SwapTotalMB: kb("SwapTotal"),
		SwapUsedMB:  kb("SwapTotal") - kb("SwapFree"),
		CgroupLimit: cgroupMemoryLimit(),
	}
	if m.AvailableMB == 0 {
		m.AvailableMB = kb("MemFree") + kb("Buffers") + kb("Cached")
	}
	if m.TotalMB > 0 {
		m.UsedMB = m.TotalMB - m.AvailableMB
	}
	return m, nil
}

func (linuxPlatform) Disks(ctx context.Context) ([]DiskInfo, error) {
	out, err := runCommand(ctx, 2*time.Second, "df", "-P", "-k")
	if err != nil {
		return nil, err
	}
	var disks []DiskInfo
	for _, line := range strings.Split(out, "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		disks = append(disks, DiskInfo{
			Mount: fields[5], FSType: "-",
			TotalBytes: parseUInt64(fields[1]) * 1024,
			UsedBytes:  parseUInt64(fields[2]) * 1024,
			FreeBytes:  parseUInt64(fields[3]) * 1024,
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Mount < disks[j].Mount })
	return disks, nil
}

// Bounds for the /proc walk. A machine with thousands of short-lived PIDs
// must not stall a diagnostic snapshot, and each /proc/<pid>/fd listing is
// read in small batches so one unreadable directory cannot block forever.
const (
	maxProcScanPIDs   = 8192
	procCtxCheckEvery = 32
	maxProcFDs        = 4096
)

func (linuxPlatform) Processes(ctx context.Context, n int) ([]ProcessInfo, error) {
	return processesFromProc(ctx, "/proc", os.Getpid(), n)
}

// processesFromProc is the testable core of the Linux process collector.
// procRoot is injectable so cancellation, capping and fd counting can be
// exercised with a synthetic /proc tree on any machine.
func processesFromProc(ctx context.Context, procRoot string, selfPID, n int) ([]ProcessInfo, error) {
	if n <= 0 {
		return []ProcessInfo{}, nil
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	totalTicks := uint64(0)
	if boot, err := os.ReadFile(filepath.Join(procRoot, "stat")); err == nil {
		fields := strings.Fields(strings.Split(string(boot), "\n")[0])
		if len(fields) > 1 {
			for _, f := range fields[1:] {
				totalTicks += parseUInt64(f)
			}
		}
	}
	procs := make([]ProcessInfo, 0, min(n, maxProcScanPIDs))
	scanned, seen := 0, 0
	for _, e := range entries {
		// Check the deadline every N entries, including non-PID entries.
		if seen%procCtxCheckEvery == 0 {
			if err := ctx.Err(); err != nil {
				return procs, err
			}
		}
		seen++
		if scanned >= maxProcScanPIDs {
			break
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		scanned++
		dir := filepath.Join(procRoot, e.Name())
		status := readFileFields(filepath.Join(dir, "status"))
		fields := strings.Fields(status["VmRSS"])
		rss := 0.0
		if len(fields) > 0 {
			rss = parseFloat(fields[0]) / 1024
		}
		threads, _ := strconv.Atoi(status["Threads"])
		ppid := 0
		lifetimeShare := 0.0
		if stat, err := os.ReadFile(filepath.Join(dir, "stat")); err == nil {
			parts := strings.Fields(string(stat))
			if len(parts) > 3 {
				ppid, _ = strconv.Atoi(parts[3])
			}
			if len(parts) > 14 {
				if ticks := parseUInt64(parts[13]) + parseUInt64(parts[14]); ticks > 0 {
					lifetimeShare = lifetimeCPUSharePct(ticks, totalTicks)
				}
			}
		}
		cmd := status["Name"]
		if data, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
			parts := strings.FieldsFunc(string(data), func(r rune) bool { return r == 0 })
			if len(parts) > 0 {
				cmd = strings.Join(parts, " ")
			}
		}
		procs = append(procs, ProcessInfo{
			PID: pid, PPID: ppid, User: "-", Cmd: cmd, RSSMB: rss,
			LifetimeCPUSharePct: lifetimeShare,
			Threads:             threads, FDs: countProcFDs(ctx, filepath.Join(dir, "fd")),
			Self: pid == selfPID,
		})
	}
	if err := ctx.Err(); err != nil {
		return procs, err
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].RSSMB > procs[j].RSSMB })
	if len(procs) > n {
		procs = procs[:n]
	}
	return procs, nil
}

// countProcFDs counts directory entries in bounded batches so a single
// unreadable or very large fd directory cannot stall the collector.
func countProcFDs(ctx context.Context, dir string) int {
	d, err := os.Open(dir)
	if err != nil {
		return 0
	}
	defer d.Close()
	count := 0
	for {
		if count%128 == 0 {
			if err := ctx.Err(); err != nil {
				return count
			}
		}
		names, err := d.Readdirnames(128)
		count += len(names)
		if err != nil || count >= maxProcFDs {
			return count
		}
	}
}

func (linuxPlatform) Packages(ctx context.Context) ([]PackageInfo, error) {
	specs := []struct {
		cmd    string
		args   []string
		source string
	}{
		{"dpkg-query", []string{"-W", "-f=${Package} ${Version}\n"}, "dpkg"},
		{"rpm", []string{"-qa", "--qf", "%{NAME} %{VERSION}\n"}, "rpm"},
		{"apk", []string{"info", "-v"}, "apk"},
		{"pacman", []string{"-Q"}, "pacman"},
	}
	for _, spec := range specs {
		out, err := runCommand(ctx, 2*time.Second, spec.cmd, spec.args...)
		if err != nil || out == "" {
			continue
		}
		var pkgs []PackageInfo
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pkgs = append(pkgs, PackageInfo{Name: fields[0], Version: fields[1], Source: spec.source})
			}
			if len(pkgs) >= 50 {
				break
			}
		}
		sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
		return pkgs, nil
	}
	return nil, ErrUnsupported
}

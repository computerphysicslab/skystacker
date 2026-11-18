//go:build darwin

package diagnostics

import (
	"context"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type darwinPlatform struct{}

func newPlatformImpl() platform { return darwinPlatform{} }

func sysctl(ctx context.Context, key string) string {
	v, _ := runCommand(ctx, time.Second, "sysctl", "-n", key)
	return strings.TrimSpace(v)
}

func (darwinPlatform) Host(ctx context.Context) (int64, string, error) {
	raw := sysctl(ctx, "kern.boottime")
	re := regexp.MustCompile(`sec\s*=\s*(\d+)`)
	m := re.FindStringSubmatch(raw)
	if len(m) < 2 {
		return 0, "", os.ErrInvalid
	}
	bootSec, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, "", err
	}
	now := time.Now().Unix()
	sec := now - bootSec
	if sec < 0 {
		sec = 0
	}
	return sec, time.Unix(bootSec, 0).UTC().Format(time.RFC3339), nil
}

func (darwinPlatform) OS(ctx context.Context) (OSInfo, error) {
	o := OSInfo{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Timezone: timezoneName()}
	o.Name, _ = runCommand(ctx, time.Second, "sw_vers", "-productName")
	o.Version, _ = runCommand(ctx, time.Second, "sw_vers", "-productVersion")
	o.Kernel, _ = runCommand(ctx, time.Second, "uname", "-r")
	return o, nil
}

func (darwinPlatform) CPU(ctx context.Context) (CPUInfo, error) {
	c := CPUInfo{Logical: runtime.NumCPU(), Physical: runtime.NumCPU(),
		Model: sysctl(ctx, "hw.model")}
	mhz := sysctl(ctx, "hw.cpufrequency_max")
	if v, err := strconv.ParseFloat(mhz, 64); err == nil {
		c.MHz = v / 1e6
	}
	if n := sysctl(ctx, "hw.physicalcpu"); n != "" {
		if v, err := strconv.Atoi(n); err == nil {
			c.Physical = v
		}
	}
	load := sysctl(ctx, "vm.loadavg")
	fields := strings.Fields(load)
	if len(fields) >= 3 {
		c.Load1, c.Load5, c.Load15 = parseFloat(fields[0]), parseFloat(fields[1]), parseFloat(fields[2])
	}
	return c, nil
}

func (darwinPlatform) Memory(ctx context.Context) (MemoryInfo, error) {
	m := MemoryInfo{}
	total := sysctl(ctx, "hw.memsize")
	if v, err := strconv.ParseUint(total, 10, 64); err == nil {
		m.TotalMB = float64(v) / (1024 * 1024)
	}
	out, err := runCommand(ctx, time.Second, "vm_stat")
	if err != nil {
		return m, err
	}
	pages := func(key string) float64 {
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, key+":") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					return parseFloat(strings.TrimRight(fields[len(fields)-1], ".")) * 4096 / (1024 * 1024)
				}
			}
		}
		return 0
	}
	free := pages("Pages free")
	inactive := pages("Pages inactive")
	m.AvailableMB = free + inactive
	m.CachedMB = pages("Pages purgeable") + inactive
	m.SwapTotalMB = pages("Swapused") // best-effort; many macOS builds omit swap stats
	if m.TotalMB > 0 {
		m.UsedMB = m.TotalMB - m.AvailableMB
	}
	return m, nil
}

func (darwinPlatform) Disks(ctx context.Context) ([]DiskInfo, error) {
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
			Mount: fields[5], FSType: fields[0],
			TotalBytes: parseUInt64(fields[1]) * 1024,
			UsedBytes:  parseUInt64(fields[2]) * 1024,
			FreeBytes:  parseUInt64(fields[3]) * 1024,
		})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Mount < disks[j].Mount })
	return disks, nil
}

func (darwinPlatform) Processes(ctx context.Context, n int) ([]ProcessInfo, error) {
	self := os.Getpid()
	out, err := runCommand(ctx, 2*time.Second, "ps", "-axo", "pid=,ppid=,rss=,%cpu=,user=,command=")
	if err != nil {
		return nil, err
	}
	var procs []ProcessInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		rssKB, _ := strconv.ParseFloat(fields[2], 64)
		cpu, _ := strconv.ParseFloat(fields[3], 64)
		procs = append(procs, ProcessInfo{
			PID: pid, PPID: ppid, User: fields[4], Cmd: strings.Join(fields[5:], " "),
			RSSMB: rssKB / 1024, CPUPct: cpu, Self: pid == self,
		})
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].RSSMB > procs[j].RSSMB })
	if len(procs) > n {
		procs = procs[:n]
	}
	return procs, nil
}

func (darwinPlatform) Updates(ctx context.Context) ([]PackageInfo, error) {
	out, err := runCommand(ctx, 2*time.Second, "brew", "list", "--formula", "--versions")
	if err != nil || out == "" {
		return nil, ErrUnsupported
	}
	var pkgs []PackageInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pkgs = append(pkgs, PackageInfo{Name: fields[0], Version: fields[1], Source: "brew"})
		}
		if len(pkgs) >= 50 {
			break
		}
	}
	return pkgs, nil
}

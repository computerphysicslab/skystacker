//go:build windows

package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

type windowsPlatform struct{}

func newPlatformImpl() platform { return windowsPlatform{} }

func psJSON(ctx context.Context, script string, dst any) error {
	out, err := runCommand(ctx, 2*time.Second, "powershell", "-NoProfile", "-Command", script)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(out), dst)
}

func (windowsPlatform) Host(ctx context.Context) (int64, string, error) {
	var v struct {
		Boot string `json:"boot"`
	}
	if err := psJSON(ctx, `Get-CimInstance Win32_OperatingSystem | Select-Object @{n='boot';e={$_.ConvertToDateTime($_.LastBootUpTime).ToString('o')}} | ConvertTo-Json -Compress`, &v); err != nil {
		return 0, "", err
	}
	t, err := time.Parse(time.RFC3339Nano, v.Boot)
	if err != nil {
		return 0, "", err
	}
	sec := int64(time.Since(t).Seconds())
	if sec < 0 {
		sec = 0
	}
	return sec, t.UTC().Format(time.RFC3339), nil
}

func (windowsPlatform) OS(ctx context.Context) (OSInfo, error) {
	o := OSInfo{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Timezone: timezoneName()}
	var v struct {
		Caption        string `json:"Caption"`
		Version        string `json:"Version"`
		OSArchitecture string `json:"OSArchitecture"`
	}
	if err := psJSON(ctx, `Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,OSArchitecture | ConvertTo-Json -Compress`, &v); err == nil {
		o.Name, o.Version = v.Caption, v.Version+" "+v.OSArchitecture
	}
	o.Kernel, _ = runCommand(ctx, time.Second, "powershell", "-NoProfile", "-Command", `[Environment]::OSVersion.VersionString`)
	return o, nil
}

func (windowsPlatform) CPU(ctx context.Context) (CPUInfo, error) {
	c := CPUInfo{Logical: runtime.NumCPU(), Physical: runtime.NumCPU()}
	var v []struct {
		Name                      string  `json:"Name"`
		NumberOfCores             int     `json:"NumberOfCores"`
		NumberOfLogicalProcessors int     `json:"NumberOfLogicalProcessors"`
		MaxClockSpeed             float64 `json:"MaxClockSpeed"`
	}
	if err := psJSON(ctx, `Get-CimInstance Win32_Processor | Select-Object Name,NumberOfCores,NumberOfLogicalProcessors,MaxClockSpeed | ConvertTo-Json -Compress`, &v); err == nil && len(v) > 0 {
		c.Model = strings.TrimSpace(v[0].Name)
		c.Physical, c.Logical, c.MHz = v[0].NumberOfCores, v[0].NumberOfLogicalProcessors, v[0].MaxClockSpeed
	}
	return c, nil
}

func (windowsPlatform) Memory(ctx context.Context) (MemoryInfo, error) {
	var v struct {
		Total uint64 `json:"TotalVisibleMemorySize"`
		Free  uint64 `json:"FreePhysicalMemory"`
	}
	if err := psJSON(ctx, `Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory | ConvertTo-Json -Compress`, &v); err != nil {
		return MemoryInfo{}, err
	}
	total := float64(v.Total) / 1024
	free := float64(v.Free) / 1024
	return MemoryInfo{TotalMB: total, AvailableMB: free, UsedMB: total - free}, nil
}

func (windowsPlatform) Disks(ctx context.Context) ([]DiskInfo, error) {
	var v []struct {
		DeviceID string `json:"DeviceID"`
		FS       string `json:"FileSystem"`
		Size     uint64 `json:"Size"`
		Free     uint64 `json:"FreeSpace"`
	}
	if err := psJSON(ctx, `Get-CimInstance Win32_LogicalDisk | Select-Object DeviceID,FileSystem,Size,FreeSpace | ConvertTo-Json -Compress`, &v); err != nil {
		return nil, err
	}
	out := make([]DiskInfo, 0, len(v))
	for _, d := range v {
		out = append(out, DiskInfo{Mount: d.DeviceID, FSType: d.FS, TotalBytes: d.Size, FreeBytes: d.Free, UsedBytes: d.Size - d.Free})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	return out, nil
}

func (windowsPlatform) Processes(ctx context.Context, n int) ([]ProcessInfo, error) {
	self := os.Getpid()
	script := fmt.Sprintf(`Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First %d Id,ParentProcessId,ProcessName,WorkingSet64,CPU | ConvertTo-Json -Compress`, n)
	var v []struct {
		ID      int     `json:"Id"`
		PPID    int     `json:"ParentProcessId"`
		Name    string  `json:"ProcessName"`
		Working float64 `json:"WorkingSet64"`
		CPU     float64 `json:"CPU"`
	}
	if err := psJSON(ctx, script, &v); err != nil {
		return nil, err
	}
	out := make([]ProcessInfo, 0, len(v))
	for _, p := range v {
		out = append(out, ProcessInfo{
			PID: p.ID, PPID: p.PPID, User: "-", Cmd: p.Name,
			RSSMB: p.Working / (1024 * 1024), CPUPct: p.CPU, Self: p.ID == self,
		})
	}
	return out, nil
}

func (windowsPlatform) Updates(ctx context.Context) ([]PackageInfo, error) {
	var v []struct {
		ID        string `json:"HotFixID"`
		Installed string `json:"InstalledOn"`
	}
	if err := psJSON(ctx, `Get-HotFix | Select-Object HotFixID,InstalledOn | ConvertTo-Json -Compress`, &v); err != nil {
		return nil, ErrUnsupported
	}
	var out []PackageInfo
	for _, h := range v {
		out = append(out, PackageInfo{Name: h.ID, Version: h.Installed, Source: "hotfix"})
		if len(out) >= 50 {
			break
		}
	}
	return out, nil
}

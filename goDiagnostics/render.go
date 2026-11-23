package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// JSON returns the report as indented UTF-8 JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// WriteJSON writes the report as JSON to w.
func (r Report) WriteJSON(w io.Writer) error {
	b, err := r.JSON()
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// Text returns a human-readable, stable rendering of the report.
func (r Report) Text() string {
	var b strings.Builder
	line := func(label string, v any) { fmt.Fprintf(&b, "%-22s %v\n", label+":", v) }
	b.WriteString("SkyStacker diagnostics\n")
	b.WriteString("======================\n")
	line("Generated", r.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
	line("Duration", fmt.Sprintf("%d ms", r.DurationMS))
	b.WriteString("\n[host]\n")
	line("Hostname", r.Host.Hostname)
	line("User", r.Host.User)
	line("Uptime (s)", r.Host.UptimeSec)
	line("Boot", r.Host.BootTime)
	b.WriteString("\n[os]\n")
	line("Name", strings.TrimSpace(r.OS.Name+" "+r.OS.Version))
	line("Kernel", r.OS.Kernel)
	line("GOOS/GOARCH", r.OS.GOOS+"/"+r.OS.GOARCH)
	line("Container", r.OS.Container)
	b.WriteString("\n[runtime]\n")
	line("Go", r.Runtime.GoVersion)
	line("Module", r.Runtime.Module)
	line("VCS", strings.TrimSpace(r.Runtime.VCSRevision+" "+r.Runtime.VCSTime))
	line("Goroutines", r.Runtime.Goroutines)
	line("GC cycles", r.Runtime.NumGC)
	line("Heap/Sys MB", fmt.Sprintf("%.1f / %.1f", r.Runtime.HeapMB, r.Runtime.SysMB))
	b.WriteString("\n[cpu]\n")
	line("Model", r.CPU.Model)
	line("Cores L/P", fmt.Sprintf("%d/%d", r.CPU.Logical, r.CPU.Physical))
	line("Load 1/5/15", fmt.Sprintf("%.2f %.2f %.2f", r.CPU.Load1, r.CPU.Load5, r.CPU.Load15))
	b.WriteString("\n[memory]\n")
	line("Total MB", fmt.Sprintf("%.1f", r.Memory.TotalMB))
	line("Available MB", fmt.Sprintf("%.1f", r.Memory.AvailableMB))
	line("Used MB", fmt.Sprintf("%.1f", r.Memory.UsedMB))
	if r.Memory.CgroupLimit != nil {
		line("Cgroup limit MB", fmt.Sprintf("%.1f", *r.Memory.CgroupLimit))
	}
	b.WriteString("\n[disks]\n")
	for _, d := range r.Disks {
		label := d.Mount
		if d.Path != "" {
			label += " (" + d.Path + ")"
		}
		line(label, fmt.Sprintf("total=%d free=%d files=%d bytes=%d", d.TotalBytes, d.FreeBytes, d.Files, d.Bytes))
	}
	if len(r.Processes) > 0 {
		b.WriteString("\n[processes]\n")
		for _, p := range r.Processes {
			mark := ""
			if p.Self {
				mark = " *"
			}
			detail := fmt.Sprintf("%.1f MB", p.RSSMB)
			if p.LifetimeCPUSharePct > 0 {
				detail += fmt.Sprintf(", lifetime CPU share %.1f%% (since boot)", p.LifetimeCPUSharePct)
			}
			line(fmt.Sprintf("pid %d%s", p.PID, mark), detail+" "+shorten(p.Cmd, 90))
		}
	}
	if len(r.Tools) > 0 {
		b.WriteString("\n[tools]\n")
		for _, t := range r.Tools {
			line(t.Name, strings.TrimSpace(t.Version))
		}
	}
	if len(r.Updates) > 0 {
		b.WriteString("\n[updates]\n")
		for _, u := range r.Updates {
			line(u.Name, u.Version+" ("+u.Source+")")
		}
	}
	if len(r.Errors) > 0 {
		b.WriteString("\n[errors]\n")
		for _, e := range r.Errors {
			line(e.Collector, e.Reason)
		}
	}
	return b.String()
}

func shorten(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

package diagnostics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

type cappedBuffer struct {
	bytes.Buffer
	max int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	room := b.max - b.Len()
	if room <= 0 {
		return len(p), nil
	}
	if len(p) > room {
		p = p[:room]
	}
	return b.Buffer.Write(p)
}

func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	if timeout <= 0 {
		timeout = time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	var out cappedBuffer
	out.max = 64 * 1024
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// RedactValue masks values that look like credentials or URLs with embedded
// credentials. It is intentionally conservative: false positives only hide
// diagnostic detail, false negatives would leak secrets.
func RedactValue(v string) string {
	if v == "" {
		return ""
	}
	lower := strings.ToLower(v)
	for _, needle := range []string{"token", "secret", "password", "api_key", "apikey", "bearer "} {
		if strings.Contains(lower, needle) {
			return "***REDACTED***"
		}
	}
	if strings.Contains(v, "://") {
		if u, err := url.Parse(v); err == nil && u != nil {
			return u.Redacted()
		}
		return "***REDACTED***"
	}
	if len(v) > 40 && !strings.ContainsAny(v, " /\\") {
		return "***REDACTED***"
	}
	return v
}

func collectHost(p platform, ctx context.Context) HostInfo {
	h := HostInfo{
		Hostname: os.Getenv("HOSTNAME"),
		Home:     os.Getenv("HOME"),
		TempDir:  os.TempDir(),
	}
	if name, err := os.Hostname(); err == nil {
		h.Hostname = name
	}
	if u, err := user.Current(); err == nil {
		h.User = u.Username
		if h.Home == "" {
			h.Home = u.HomeDir
		}
	} else {
		h.User = os.Getenv("USER")
		if h.User == "" {
			h.User = os.Getenv("USERNAME")
		}
	}
	if sec, boot, err := p.Host(ctx); err == nil {
		h.UptimeSec, h.BootTime = sec, boot
	}
	return h
}

func collectRuntime() RuntimeInfo {
	r := RuntimeInfo{
		GoVersion:  runtime.Version(),
		GOGC:       os.Getenv("GOGC"),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		Goroutines: uint64(runtime.NumGoroutine()),
	}
	bi, ok := debug.ReadBuildInfo()
	if ok {
		r.Module = bi.Path
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				r.VCSRevision = s.Value
			case "vcs.time":
				r.VCSTime = s.Value
			case "vcs.modified":
				r.VCSModified, _ = strconv.ParseBool(s.Value)
			case "CGO_ENABLED":
				r.CGOEnabled = s.Value
			}
		}
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	r.NumGC = uint64(ms.NumGC)
	r.HeapMB = float64(ms.HeapAlloc) / (1024 * 1024)
	r.SysMB = float64(ms.Sys) / (1024 * 1024)
	r.TotalAllocMB = float64(ms.TotalAlloc) / (1024 * 1024)
	start := int(ms.NumGC) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < int(ms.NumGC); i++ {
		r.GCPausesMS = append(r.GCPausesMS, float64(ms.PauseNs[i%len(ms.PauseNs)])/float64(time.Millisecond))
	}
	return r
}

var envWhitelist = []string{
	"HOME", "USER", "USERNAME", "SHELL", "LANG", "LC_ALL", "TZ",
	"TMPDIR", "TEMP", "TMP", "PATH", "GOGC", "GOMAXPROCS",
	"GOPATH", "GOMODCACHE", "GOCACHE", "GOFLAGS",
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "all_proxy", "no_proxy",
}

func collectEnv(redact func(string) string) EnvInfo {
	e := EnvInfo{Vars: make(map[string]string)}
	for _, key := range envWhitelist {
		v := os.Getenv(key)
		if v == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(key), "_proxy") {
			e.ProxySet = true
			e.Vars[key] = redact(v)
			continue
		}
		e.Vars[key] = redact(v)
	}
	return e
}

func collectTools(ctx context.Context, timeout time.Duration) []ToolInfo {
	specs := []struct {
		name string
		args []string
	}{
		{"go", []string{"version"}},
		{"git", []string{"version"}},
		{"dcraw", []string{"-v"}},
		{"magick", []string{"-version"}},
		{"convert", []string{"-version"}},
	}
	if runtime.GOOS == "windows" {
		specs = append(specs, struct {
			name string
			args []string
		}{"powershell", []string{"-NoProfile", "-Command", "$PSVersionTable.PSVersion.ToString()"}})
	}
	var out []ToolInfo
	for _, spec := range specs {
		t := ToolInfo{Name: spec.name}
		path, err := exec.LookPath(spec.name)
		if err != nil {
			out = append(out, t)
			continue
		}
		t.Found, t.Path = true, path
		ver, err := runCommand(ctx, timeout, spec.name, spec.args...)
		if err == nil {
			lines := strings.Split(ver, "\n")
			if len(lines) > 0 {
				t.Version = strings.TrimSpace(lines[0])
			}
			if len(t.Version) > 200 {
				t.Version = t.Version[:200]
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func collectProgram(ctx context.Context, o Options) ProgramInfo {
	p := ProgramInfo{WorkingDir: o.ProgramDir}
	if exe, err := os.Executable(); err == nil {
		p.Executable = exe
		if st, err := os.Stat(exe); err == nil {
			p.SizeBytes = st.Size()
			if f, err := os.Open(exe); err == nil {
				h := sha256.New()
				if _, err := io.Copy(h, f); err == nil {
					p.SHA256 = hex.EncodeToString(h.Sum(nil))
				}
				f.Close()
			}
		}
	}
	root, err := filepath.Abs(o.ProgramDir)
	if err != nil {
		return p
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			p.DirSkipped++
			return nil
		}
		if ctx.Err() != nil || p.DirFiles >= o.MaxWalkFiles {
			p.DirSkipped++
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return filepath.SkipAll
		}
		if d.IsDir() {
			if d.Type()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			p.DirSkipped++
			return nil
		}
		p.DirFiles++
		p.DirBytes += info.Size()
		return nil
	})
	if err != nil && ctx.Err() == nil {
		p.DirSkipped++
	}
	return p
}

func collectNetwork() []NetInfo {
	faces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]NetInfo, 0, len(faces))
	for _, f := range faces {
		n := NetInfo{Name: f.Name, MAC: f.HardwareAddr.String(), MTU: f.MTU}
		addrs, aerr := f.Addrs()
		if aerr == nil {
			for _, a := range addrs {
				n.Addresses = append(n.Addresses, a.String())
			}
		}
		sort.Strings(n.Addresses)
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func parseUInt64(s string) uint64 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return v
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func parseMB(s string) float64 { return parseFloat(s) / (1024 * 1024) }

func timezoneName() string {
	name := time.Now().Format("MST")
	if name == "" {
		return "UTC"
	}
	return name
}

# Installation

SkyStacker builds with the Go standard library only. Runtime helpers
(`dcraw`/ImageMagick for legacy RAW/FITS paths, `solve-field`, `astap`) are
optional and selected by flags.

## Requirements

- **Go 1.27.4** (pinned by `toolchain go1.27.4` in `go.mod` and CI).
- Linux, macOS or Windows. The canonical binary is static and `CGO_ENABLED=0`
  by default.
- Optional: `astrometry.net` with indexes, ASTAP with a D05–D80 database, or a
  `nova.astrometry.net` API key for blind plate solving.

## Build and install

```bash
cd SkyStacker
gofmt -w *.go
go vet ./...
go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o /home/jips/.local/bin/skystacker .
cp -f /home/jips/.local/bin/skystacker ./skystacker
ln -sf /home/jips/.local/bin/skystacker /home/jips/go/bin/skystacker
which skystacker && skystacker --version
skystacker --selftest        # expect: all 264 self-tests passed
go test ./...
```

Replace the absolute paths with your own `~/.local/bin` if needed. The
repository convention is to keep `./skystacker` in sync with the installed
binary and to verify `which skystacker` plus `skystacker --version` before
declaring a build done.

## Modules

The root module `github.com/computerphysicslab/skystacker` imports two local
sibling modules through `replace` directives:

| Module | Purpose |
|---|---|
| `goPureRaw/` | Pure-Go CR2 decoding and RAW metadata (`github.com/computerphysicslab/skystacker/goPureRaw` v0.1.1). |
| `goDiagnostics/` | Portable host/runtime collectors used by `--diagnose` (`.../goDiagnostics` v0.0.0). |

`go.work` links `.`, `./goPureRaw` and `./goDiagnostics` and is gitignored.
New modules must follow the `go<Name>` naming convention.

## Native decoding

Pure-Go CR2 decoding is enabled by default. Use `--no-native-cr2` to fall back
to `dcraw`/ImageMagick. CR3 still requires an external decoder.

## Verification checklist

1. `gofmt -l *.go` prints nothing.
2. `go vet ./...` is clean.
3. `go test ./...` passes (Go `testing/synctest` rate-limiter test included).
4. `skystacker --selftest` prints `all 264 self-tests passed`.
5. `skystacker --version` shows v0.6.1 and the current build date.

See [quickstart.md](quickstart.md) for the first real run and
[integration.md](integration.md) for process-level usage.

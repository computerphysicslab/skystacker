# goDiagnostics

A standalone, read-only diagnostic collector for Go programs. It reports the
host operating system, Go runtime, CPU, memory, disks, processes, installed
packages, external tools, network interfaces and the size/hash of the running
program, as text or JSON.

## Design rules

* Standard library only. No cgo, no third-party imports.
* `go 1.22` baseline: reusable in older Go projects than SkyStacker.
* Every collector is bounded by a context timeout and a 64 KiB output cap;
  the Linux `/proc` walk is capped, context-aware in bounded batches and never
  performs an unbounded `ReadDir` on a process descriptor directory.
* Collector failures are recorded in `Report.Errors`; they never abort a run.
* Environment values are whitelisted and redacted; process command lines are
  masked by `RedactProcessCmd` on every platform, so no tokens are emitted.
* OS-specific code lives in files with build tags behind the `platform`
  interface, so unknown systems degrade gracefully with `ErrUnsupported`.

## Use in another project

Copy this folder into the target repository, then add to that project's
`go.mod`:

```go
require github.com/computerphysicslab/skystacker/goDiagnostics v0.0.0
replace github.com/computerphysicslab/skystacker/goDiagnostics => ./goDiagnostics
```

If `go.work` is used, add `./goDiagnostics` to its `use` block.

```go
import diagnostics "github.com/computerphysicslab/skystacker/goDiagnostics"

o := diagnostics.DefaultOptions()
o.ProgramDir = "."
o.IncludePackages = true
rep := diagnostics.Collect(ctx, o)
fmt.Print(rep.Text())
_ = rep.WriteJSON(os.Stdout)
```

To consume it as a published module instead, tag it `goDiagnostics/v0.1.0`,
remove the `replace` line and use `go get ...@v0.1.0`.

## API

`Options` controls timeouts, program directory, process count, directory-walk
budget, package inventory and environment redaction. The installed-package
inventory is exposed as `Options.IncludePackages`, `Report.Packages` and the
JSON key `packages` (it is not a list of pending updates). `Collect` always
returns a `Report`; `Report.JSON()`, `Report.WriteJSON()` and `Report.Text()`
render it deterministically (all slices are sorted).

## Tests

```bash
cd goDiagnostics
go vet ./...
go test ./...
```

Tests use a fake `platform`, so they never depend on the host machine. CI also
cross-builds the module for linux, darwin and windows.

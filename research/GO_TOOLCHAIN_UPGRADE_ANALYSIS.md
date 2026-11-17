# Go toolchain upgrade analysis (2026-11-17)

## Current state

- SkyStacker root `go.mod`: `go 1.20`; installed toolchain go1.20.3 linux/amd64.
- PureRaw subproject `go.mod`: `go 1.22`.
- CI: gofmt / go vet / self-test / `-race` / cross-platform build / `go install`.
- Constraints: single-file `main.go`, standard library only, `CGO_ENABLED=0`,
  static cross-platform binaries (Linux/macOS/Windows, amd64 + arm64).

## Latest stable (verified against go.dev, 2026-11-17)

- **go1.27.4** (2026-11-10) is the current stable release; go1.26.11 is the
  previous maintained branch. Both received 12 security fixes on 2026-11-10
  (cgo, `go` command, compiler/linker, `net/http`, crypto packages).
- Go 1.20 is far outside the support window (Go supports the last two major
  releases), so it no longer receives security fixes.

## Advantages of upgrading to Go 1.27.4

1. **Security** — strongest argument. SkyStacker uses `net/http` (Gaia TAP
   client), `encoding/json`, `os/exec`, `crypto/sha256`; those areas received
   fixes in 1.27.2/1.27.3/1.27.4.
2. **Runtime performance relevant to our workload**:
   - Go 1.24 Swiss Table maps (~20–30% faster map insert/lookup in Go blog
     benchmarks) — our star matcher uses `map[[2]int32][]int32` spatial
     hashes, `seen` maps in BPM/QA, and the decoder registry.
   - Go 1.26 Green Tea GC enabled by default (~7–11% GC throughput gain,
     18–20% faster GC with many small objects) — we explicitly set
     `debug.SetGCPercent(50)` for large sessions, so GC behaviour matters.
   - Go 1.27 size-specialized allocator (<80 B objects up to 30% cheaper,
     ~1% overall) and faster `encoding/binary` / large `io.ReadAll` — relevant
     to the FITS/XISF/`.f32raw` writers and the native CR2 reader (12 MB reads).
   - Slice backing-store stack allocation improvements reduce heap pressure
     in tiled reducers.
3. **Standard library / tooling**:
   - `encoding/json/v2` with stricter defaults; existing `encoding/json`
     becomes faster and v2-backed — benefits `report.json`, cache JSON,
     `plate_solution.json`, `anomalies.json`.
   - `testing/synctest` (Go 1.25 GA, 1.27 `httptest.NewTestServer`) enables
     deterministic concurrency tests for the Gaia TAP client and rate limiter.
   - `runtime/pprof` goroutine-leak profile GA — useful for our worker pools.
   - `go fix` modernizers and improved `go vet` can modernize safely.
   - `uuid` in stdlib would help provenance IDs if adopted later.
4. **Language quality-of-life**: generic methods, embedded field keys in
   struct literals, generalized function type inference — could simplify
   small helpers, but are not required.

## Disadvantages / risks

1. **Behavioural drift**: the JSON implementation change and GC change can
   alter serialized output and peak RSS. We have exact-string assertions in
   self-tests (`report.txt`, `report.json`) and calibrated pre-flight memory
   estimates in `TECHNICAL_SPEC.md` §5 / `BENCHMARKS.md`; all need re-baseline.
2. **Toolchain requirement for users**: README currently says "Go 1.20 or
   later"; bumping the `go` directive to 1.27 means source builders need
   Go >= 1.27. CI must use `go-version-file: go.mod`.
3. **Binary reproducibility**: v0.6.1 tag was built with 1.20.3; a 1.27 build
   may differ in float formatting/JSON output. Document the toolchain version
   in the banner/`report.json` if strict reproducibility matters.
4. **Temptation to use `simd/archsimd`** — experimental, opt-in, amd64-only.
   Using it would break arm64/macOS parity and the single-file portability
   promise. Do not adopt.
5. **Migration effort**: low (no external modules), but must install the new
   toolchain, run the full test matrix (gofmt, vet, race, cross-build,
   `go install`), repeat BENCHMARKS rows on the i7-8565U, and update docs
   (README, TECHNICAL_SPEC, prompt-ai, PureRaw go.mod, CI).
6. **PureRaw subproject** declares `go 1.22`; it can be left as-is (newer
   toolchains build older directives), or bumped for consistency.

## Recommendation

Upgrade, in two low-risk steps:

1. Install **go1.27.4**, bump root `go.mod` to `go 1.27`, update CI to use
   `go-version-file`, run the full test matrix and re-measure the
   `BENCHMARKS.md` baseline rows. No code changes required for compilation.
2. If all green, optionally run `go fix` modernizers and adopt
   `testing/synctest` for the Gaia mock tests. Keep `CGO_ENABLED=0`, avoid
   `simd/archsimd`, and document the toolchain in README/report.

If a conservative path is preferred, **go1.26.11** is still security-patched
and brings the Green Tea GC and cgo improvements with one less major-version
jump. Do not remain on 1.20.3: it is unsupported and unpatched.

## Sources

- <https://go.dev/dl/> — stable versions go1.27.4, go1.26.11 (verified 2026-11-17).
- <https://go.dev/doc/devel/release> — release dates and security revisions.
- <https://go.dev/blog/go1.27> — Go 1.27 language/tooling/runtime highlights.
- <https://go.dev/blog/go1.26> — Go 1.26 Green Tea GC, cgo, `go fix`.
- <https://www.openwall.com/lists/oss-security/2026/11/10/28> — 12 security fixes
  in go1.27.4 / go1.26.11.

## Decision implemented (2026-11-17)

Adopted the conservative option: **go1.26.11** installed at `/home/jips/.local/go1.26.11`, `go 1.26` + `toolchain go1.26.11` in both modules, CI/release pinned to 1.26.11. Full matrix green (gofmt, vet, build, 208 self-tests, PureRaw CGO=0 + race). No `simd/archsimd` adoption.
### Current-state correction (2026-11-17, post-migration)

The "Current state" section above describes the pre-migration baseline (go1.20.3, `go 1.20`). The repository now uses **go1.26.11** with `go 1.26` + `toolchain go1.26.11` in both modules, CI/release pinned to 1.26.11, and all matrices green. Sections on Go 1.27 remain a forward-looking option, not the adopted toolchain.

### Code improvements adopted (2026-11-17)

- Applied `go fix` (Go 1.26 modernizers: `newexpr`, `minmax`, `rangeint`, `mapsloop`, `slicescontains`, `slicessort`, `fmtappendf`, `omitzero`, `forvar`) to both modules; all tests remained green.
- Added `main_test.go` using Go 1.26 `testing/synctest` to verify the 1 req/s Gaia TAP rate limiter deterministically (no wall-clock sleeps in CI).
- No `simd/archsimd`, no `encoding/json/v2` (not available in 1.26 without an experiment).

### Go 1.27.4 adopted (2026-11-17, append-only status)

- The forward-looking option above was executed: go1.27.4 installed at `/home/jips/.local/go1.27.4/bin` (module-cache toolchain symlinked), `go 1.27` + `toolchain go1.27.4` in both `go.mod` files, CI/release pinned to 1.27.4, local `go.work` bumped to 1.27.4.
- Full matrix green on go1.27.4: `gofmt -l .` clean, `go vet ./...`, build, 209 self-tests, `go test ./...`, PureRaw `CGO_ENABLED=0` + `-race`, race build + 209 self-tests, and static cross-builds for linux/darwin/windows amd64+arm64. No source changes were needed and no JSON/report drift was observed.
- Re-baselined BENCHMARKS.md: 4-frame native-CR2 sigma stack 39.031 s (was 59.937 s on 1.26.11), `--resume --starless` 21.11 s (was 23.00 s). Not a controlled A/B; thermal variance applies.
- `go fix` modernizers were already applied under 1.26; no additional 1.27 modernization was applied. `simd/archsimd` remains rejected.

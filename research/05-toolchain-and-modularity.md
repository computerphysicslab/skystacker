# 05 — Go toolchain, modularization, and engineering baseline (canonical research record)

**CURRENT STATUS (canonical, 2026-11-17):**

- **Toolchain:** Go **1.27.4** at `/home/jips/.local/go1.27.4/bin`; `go 1.27` + `toolchain go1.27.4` in **both** modules; CI/release pinned to 1.27.4; local `go.work` (gitignored) bumped to 1.27.4.
- **Self-tests:** **262 registered** (`skystacker --selftest`); a Go 1.26 `testing/synctest` rate-limiter test also runs via `go test ./...`. Lower counts (203 → 208 → 209) in the absorbed records are historical snapshots, preserved once in §1.
- **Full matrix green** on go1.27.4: `gofmt -l .` clean, `go vet ./...`, build, `go test ./...`, goPureRaw `CGO_ENABLED=0` + `-race`, race build, and six static cross-build targets (linux/darwin/windows × amd64/arm64). No source changes were needed and no JSON/report drift was observed.
- **Module layout:** a single stdlib-only executable plus **one first-party module**, `github.com/computerphysicslab/skystacker/goPureRaw v0.1.1` — chosen **Option A (monorepo + subdirectory modules)**.
- **Benchmarks re-baselined:** 4-frame native-CR2 sigma stack **39.031 s** (was 59.937 s on 1.26.11); `--resume --starless` **21.11 s** (was 23.00 s). Not a controlled A/B; thermal variance applies.
- **Architecture constraints held throughout:** `CGO_ENABLED=0`, standard library only at build time, static cross-platform CLI (Linux/macOS/Windows, amd64 + arm64), float32 RGB end-to-end, spill files + tiled out-of-core reduction.

This is the single canonical, deduplicated English record for the Go toolchain upgrade and the module-layout work. It **absorbs** `research/GO_TOOLCHAIN_UPGRADE_ANALYSIS.md`, `research/MODULARIZATION_ANALYSIS.md`, the toolchain / engineering / file-format portions of `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md`, and the ground-truth and benchmark context of `research/GEMINI_DEEP_RESEARCH_PROMPT.md`. Every unique fact, number, date, URL, risk, decision, status layer and open item from those sources appears exactly once below; this is a rewrite, not an append-only log.

**Cross-links (canonical files — do not duplicate their content here):**

- [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) — stacking engine, SoA/Swiss internals, memory model, full benchmark tables, file-format layouts, provenance, observatory automation.
- [`02-astrometry-and-identification.md`](02-astrometry-and-identification.md) — Gaia TAP/DR4 quotas, plate solving, WCS/SIP, anomaly detection.
- [`03-reference-and-literature.md`](03-reference-and-literature.md) — master bibliography and comprehensive technical glossary.
- [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) — calibration, master frames, grid self-flat.
- [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) — RC-Astro/GraXpert/StarNet2 CLI sidecar implementation (only the *rejection of embedded ML* is recorded here).
- [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) — CR2/CHDK bit-exact decoding internals (only the goPureRaw module packaging is recorded here).
- [`08-crawler-source-discovery.md`](08-crawler-source-discovery.md) — crawler and source-discovery runs.
- [`README.md`](README.md) — project overview and flag reference.

---

## 1. Status layers (canonical CURRENT vs historical snapshots)

The absorbed files were written at different moments. The layers below fix each status in time; do not read an earlier layer as the current state.

| Layer | Date | Toolchain / layout | Self-tests | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Survey baseline** (Gemini prompt ground truth) | pre-2026-11-17 | go1.20.3 linux/amd64; root `go 1.20`; goPureRaw `go 1.22`; single `main.go` | **203** | Source version 0.6.1 (tag exists, main advanced beyond it); `dcraw`/`dcraw_emu`/ImageMagick allowed only for RAW decoding. |
| **Baseline (analysis "Current state")** | 2026-11-17 pre-migration | go1.20.3; `go 1.20`; goPureRaw `go 1.22` | — | README said "Go 1.20 or later"; `v0.6.1` was built with 1.20.3. |
| **Conservative adoption (historical)** | 2026-11-17 | go1.26.11 at `/home/jips/.local/go1.26.11`; `go 1.26` + `toolchain go1.26.11` in both modules; CI/release pinned to 1.26.11 | **208** | First executed upgrade; `go fix` modernizers + `testing/synctest` added. |
| **Final adoption (append-only status)** | 2026-11-17 | go1.27.4 at `/home/jips/.local/go1.27.4/bin`; `go 1.27` + `toolchain go1.27.4` in both modules; CI/release pinned to 1.27.4; `go.work` bumped | **209** (recorded at the time) | Full matrix green; benchmarks re-baselined. |
| **Modularization progress snapshot** | 2026-11-17 | goPureRaw v0.1.1 imported; `pr*` code deleted | (same as final-adoption row) | `GOWORK=off` green; 1-light CR2 E2E 7.5 s. |
| **CURRENT (canonical)** | 2026-11-18 | Go 1.27.4, both modules `go 1.27` + `toolchain go1.27.4`, CI/release pinned, `go.work` gitignored | **262** | Single stdlib-only executable + one first-party module; full matrix green. |

### 1.1 Adopted state detail (final)

- go1.27.4 installed at `/home/jips/.local/go1.27.4/bin` (module-cache toolchain symlinked).
- Both `go.mod` files declare `go 1.27` and `toolchain go1.27.4`; CI and release pinned to 1.27.4; local `go.work` bumped to 1.27.4.
- CI must derive its Go version from the module (`go-version-file: go.mod`); the older `README` claim "Go 1.20 or later" was superseded — source builders now need Go ≥ 1.27.
- No source changes were required for compilation under 1.27.4, and no JSON/report drift was observed.
- `go fix` modernizers had already been applied under 1.26; **no additional 1.27 modernization was applied**.

### 1.2 Pre-migration baseline detail (historical)

- SkyStacker root `go.mod`: `go 1.20`; installed toolchain go1.20.3 linux/amd64.
- goPureRaw subproject `go.mod`: `go 1.22` (newer toolchains build older directives, so it could have been left as-is or bumped for consistency).
- CI stages at baseline: gofmt, go vet, self-test, `-race`, cross-platform build, `go install`.
- Constraints: single-file `main.go`, standard library only, `CGO_ENABLED=0`, static cross-platform binaries; no external Python/ML runtime, model weights, GPU/CUDA/OpenCL, or third-party Go modules; external runtime tools allowed only for RAW decoding.
- Go 1.20 is far outside the support window (Go supports the last two major releases) and no longer receives security fixes.

### 1.3 Intermediate conservative state detail (historical)

- The recommendation's conservative option was executed first: go1.26.11 (still security-patched, with Green Tea GC and cgo improvements, one less major-version jump) before the 1.27.4 step.
- Full matrix green at that point: gofmt, vet, build, 208 self-tests, goPureRaw `CGO_ENABLED=0` + race.
- **Code improvements adopted at that step:**
  - `go fix` (Go 1.26 modernizers: `newexpr`, `minmax`, `rangeint`, `mapsloop`, `slicescontains`, `slicessort`, `fmtappendf`, `omitzero`, `forvar`) applied to both modules; all tests remained green.
  - Added `main_test.go` using Go 1.26 `testing/synctest` to verify the **1 req/s** Gaia TAP rate limiter deterministically (no wall-clock sleeps in CI). (TAP client design and quota handling → [02](02-astrometry-and-identification.md).)
  - No `simd/archsimd`; no `encoding/json/v2` (not available in 1.26 without an experiment).
- The forward-looking Go 1.27 section of the source analysis was explicitly marked a status option, not the adopted toolchain — until it was executed later the same day.

---

## 2. Security and support chronology

- **2026-11-10:** **12 security fixes** shipped in **go1.27.4** and **go1.26.11**, covering cgo, the `go` command, compiler/linker, `net/http`, and crypto packages ([openwall oss-security 2026/11/10/28](https://www.openwall.com/lists/oss-security/2026/11/10/28)).
- **go1.27.4** (2026-11-10) is the **current stable** release; **go1.26.11** is the **previous maintained branch** (verified against go.dev 2026-11-17).
- Security was the **strongest argument** for upgrading: SkyStacker uses `net/http` (Gaia TAP client), `encoding/json`, `os/exec`, and `crypto/sha256`; those areas received fixes in 1.27.2 / 1.27.3 / 1.27.4.
- **Do not remain on 1.20.3** — unsupported and unpatched.

---

## 3. Go 1.24–1.27 gains relevant to SkyStacker

All gains below are stdlib/runtime/toolchain-level: **zero dependency changes**, preserving `CGO_ENABLED=0` and single-binary portability.

### 3.1 Swiss Tables (Go 1.24, Feb 2025)

- Built-in Go `map` switched to Swiss Tables: **8-slot groups with 64-bit control words**, extendible hashing up to **128 groups / 1024 slots per sub-table**, **87.5% max average load factor**, eliminating overflow buckets; reduced lock contention and faster disjoint-key `sync.Map`.
- Go 1.24 RC2 microbenchmark blog: **31.65% faster inserts (137.10 ns → 93.70 ns)** and **21.40% faster lookups (41.17 ns → 32.36 ns)**. The toolchain analysis summarizes this as "**~20–30% faster map insert/lookup** in Go blog benchmarks".
- Datadog engineering case study: **~500–510 MiB live-heap savings on a ~726 MiB map** (~70% memory reduction combined with struct trimming); Reddit commenters noted memory tradeoffs in low-traffic workloads.
- Relevance: the star matcher's `map[[2]int32][]int32` spatial hashes, the `seen` maps in BPM/QA, and the decoder registry. Go 1.24 also introduced `testing/synctest` (GA in Go 1.25). (SoA/cache-line internals → [01](01-architecture-and-core-engine.md).)

### 3.2 Green Tea GC (Go 1.26 default)

- The new GC (experimental in Go 1.25) became **default in Go 1.26**: **7–11% overall GC throughput gain**, **18–20% faster GC with many small objects** (goperf.dev tracking), with sub-32-byte allocation specialization, **8–16% faster `sync.Map`**, and an experimental `RuntimeFreegc` flag for immediate memory reuse.
- Relevance: SkyStacker explicitly sets `debug.SetGCPercent(50)` for large sessions, so GC behaviour directly affects peak RSS and throughput.

### 3.3 Go 1.27 allocator, binary I/O, and allocation improvements

- **Size-specialized allocator:** objects **< 80 B up to 30% cheaper**, ~1% overall.
- **Faster `encoding/binary`** and **large `io.ReadAll`**, and **slice backing-store stack allocation** improvements that reduce heap pressure in tiled reducers.
- Relevance: the FITS/XISF/`.f32raw` writers and the native CR2 reader (12 MB reads).

### 3.4 `encoding/json` / `encoding/json/v2`

- `encoding/json/v2` brings stricter defaults; the existing `encoding/json` becomes faster and v2-backed.
- Relevance: `report.json`, cache JSON, `plate_solution.json`, `anomalies.json`. **Not adopted**: `encoding/json/v2` was unavailable in 1.26 without an experiment, and its serialization change is a drift risk (§4).

### 3.5 Tooling: `testing/synctest`, pprof, `go fix`, vet, `uuid`

- `testing/synctest` (Go 1.25 GA; Go 1.27 adds `httptest.NewTestServer`) enables deterministic concurrency tests for the Gaia TAP client and rate limiter — applied in `main_test.go` (§1.3).
- `runtime/pprof` goroutine-leak profile GA — useful for the worker pools.
- `go fix` modernizers and improved `go vet` enable safe modernization (§1.3).
- A future stdlib `uuid` would help provenance IDs **if adopted later** (not implemented).

### 3.6 Language quality-of-life (optional, not required)

- Generic methods, embedded field keys in struct literals, and generalized function type inference could simplify small helpers but are not required for the upgrade.

---

## 4. Risks, disadvantages, and mitigations

| Risk | Detail | Mitigation / disposition |
| :--- | :--- | :--- |
| **Behavioural drift** | JSON implementation and GC changes can alter serialized output and peak RSS; self-tests have exact-string assertions for `report.txt` / `report.json`, and `TECHNICAL_SPEC.md` §5 / `BENCHMARKS.md` hold calibrated pre-flight memory estimates. | Re-baseline all of the above. Final 1.27.4 matrix observed **no JSON/report drift**. |
| **Toolchain requirement for users** | Bumping the `go` directive to 1.27 means source builders need Go ≥ 1.27 (README previously said "Go 1.20 or later"). | CI uses `go-version-file: go.mod`; document in README. |
| **Binary reproducibility** | `v0.6.1` was built with 1.20.3; a 1.27 build may differ in float formatting / JSON output. | Document the toolchain version in the banner / `report.json` if strict reproducibility matters. |
| **`simd/archsimd` temptation** | Experimental, opt-in, amd64-only; would break arm64/macOS parity and the single-file portability promise. | **Do not adopt** (§7). |
| **Migration effort** | Low (no external modules), but concrete work remains: install toolchain, run full matrix (gofmt, vet, race, cross-build, `go install`), repeat `BENCHMARKS.md` rows on the i7-8565U, and update README / TECHNICAL_SPEC / prompt-ai / goPureRaw go.mod / CI. | Executed across the 1.26.11 and 1.27.4 steps (§5). |
| **goPureRaw `go 1.22` directive** | Left as-is or bumped for consistency; newer toolchains build older directives. | Ultimately bumped with the rest to `go 1.27` + `toolchain go1.27.4`. |

---

## 5. Adoption decision and executed chronology

**Original recommendation (two low-risk steps):**

1. Install **go1.27.4**, bump root `go.mod` to `go 1.27`, update CI to `go-version-file`, run the full test matrix, and re-measure the `BENCHMARKS.md` baseline rows. **No code changes required for compilation.**
2. If all green, optionally run `go fix` modernizers and adopt `testing/synctest` for Gaia mock tests. Keep `CGO_ENABLED=0`, avoid `simd/archsimd`, document the toolchain in README/report.

**What was actually executed (consolidated):**

- **Step 1 — conservative first:** go1.26.11 adopted with `go 1.26` + `toolchain go1.26.11`, CI/release pinned, matrix green, `go fix` modernizers and the `testing/synctest` rate-limiter test added (§1.3).
- **Step 2 — final:** the forward-looking option was then executed: go1.27.4 with `go 1.27` + `toolchain go1.27.4` in both modules, CI/release pinned to 1.27.4, `go.work` bumped; full matrix green; benchmarks re-baselined (§1.1 and §6).
- The interim "current-state correction" in the source analysis is itself superseded: the Go 1.27 section is no longer merely forward-looking.

---

## 6. Reproducible benchmark methodology and re-baseline

- **Host:** Intel i7-8565U (4c/8t, 15 W ULV, ~2–4.6 GHz), **15 GiB RAM**, Ubuntu 20.04; original toolchain Go 1.20.3. (Full OrionNebula baseline table → [01](01-architecture-and-core-engine.md) and `BENCHMARKS.md`.)
- **No standardized 2024–2026 astro-stacking benchmark suite or cross-tool reproducibility protocol** was found in the gathered material.
- Adopt **goperf.dev's Go 1.24+ methodology** as a stdlib template: `testing.B.Loop()`, and export per-benchmark **mean, standard deviation, coefficient of variation (CV)**, and **reliability classification** pinned to specific kernel/hardware versions.
- Pair with **fixed CR2/FITS test sets**, **pinned CPU frequencies/governors** on the thermally throttling 15 W ULV laptop, and **identical debayering/rejection parameters**.
- Proposal-1 acceptance target: 5 repeated OrionNebula 4-CR2 and 16-CR2 passes at fixed governor, logging wall time, peak RSS, and GC pause totals, with all self-tests/vet/`-race`/`CGO_ENABLED=0` cross-builds green. External comparisons (DSS on i5-6200U; Siril/PI/APP on FX-8320/i7-9700K) are documented but **not apples-to-apples**.
- **Re-baseline result on go1.27.4:** 4-frame native-CR2 sigma stack **39.031 s** (wall 39.25 s; was **59.937 s** on 1.26.11); `--resume --starless` **21.11 s** wall (was **23.00 s**). **Not a controlled A/B — thermal variance applies.** (Sigma-clip kernel microbenchmarks → [01](01-architecture-and-core-engine.md).)

---

## 7. Explicitly rejected engineering paths (toolchain scope)

- **`simd/archsimd` intrinsics:** an **off-by-default `GOEXPERIMENT` restricted to amd64**, introduced in Go 1.26; unsuitable for reproducible cross-platform static builds (windows/macos/linux, including Apple Silicon arm64) and leaves arm64 unaccelerated. **Do not adopt**; revisit only if cross-architecture SIMD graduates to Go's stable stdlib.
- **Embedded neural-network inference (ONNX / GGUF / WASM / pure-Go CNNs):** infeasible — GraXpert's linear ONNX denoise weights alone are **284 MB (v2) to 456 MB (v3)**, deconvolution heavier still, and multi-layer U-Nets over 2622×3908 float32 stacks without GPU/CUDA/DirectML/CoreML or CGO bindings incur prohibitive CPU latency on 15 W ULV laptops. External CLI orchestration delivers the same capability while preserving the `CGO_ENABLED=0` static binary — implementation belongs to [06](06-external-ai-sidecars.md).
- **Scale note:** Seti Astro Suite Pro requires **32 GiB RAM** for full-memory stacking, underscoring the advantage of SkyStacker's tiled spill-file reducers on 15 GiB hardware.
- (Other non-goals — 2× mono/CFA upscale drizzle, GZIP-compressed FITS, restoration on unstacked/stretched data — are recorded in [01](01-architecture-and-core-engine.md); not repeated here.)

---

## 8. Modularization: module layout decision

### 8.1 Situation at analysis time

- `github.com/computerphysicslab/skystacker` is a Go module whose CLI lived in a single `main.go`.
- `goPureRaw/` was a **nested module** with its own `go.mod`, originally declaring the placeholder path `github.com/youruser/pureraw`. It was **not imported** by the parent: SkyStacker embedded an equivalent CR2 decoder as `pr*` functions in `main.go`.
- Both modules are standard-library-only and build with `CGO_ENABLED=0`.

### 8.2 Option A — one repo, several subdirectory Go modules (chosen)

A module at `goPureRaw/` must declare `module github.com/computerphysicslab/skystacker/goPureRaw` and be tagged `goPureRaw/v0.1.0`, `goPureRaw/v0.2.0`, etc.; consumers use:

```bash
go get github.com/computerphysicslab/skystacker/goPureRaw@v0.1.0
```

- **Benefits:** one CI, one issue tracker, one LICENSE, one release workflow (per-module tag patterns), one place for research/docs; lockstep evolution and easier version-skew control during rapid development; no extra repo secrets/permissions; Go module zip serving already handles subdirectory modules (consumers need not clone the whole repo).
- **Costs:** lower discoverability than a dedicated repo named `pureraw`; per-module semver requires subdirectory tags (`goPureRaw/vX.Y.Z`) and release-workflow discipline; local development needs `go.work` (or a temporary `replace` that **must NOT be committed** if `go install ...@latest` support is desired — replace directives in a dependency module are ignored by the Go proxy).

### 8.3 Option B — one independent GitHub repo per library (not chosen)

- **Benefits:** clean import path `github.com/computerphysicslab/pureraw`, maximum discoverability, independent license/version/release cadence; each library can attract its own contributors, simpler per-repo CI; smaller clones for library-only users.
- **Costs:** N repos → N CI configs, N issue trackers, N release processes, cross-repo version bumps and higher coordination cost; local cross-repo development still needs `go.work` or `replace`; higher version-skew risk while code changes quickly.

### 8.4 Decision

**Start with Option A (monorepo + subdirectory modules).** It matches the existing single-repo workflow, keeps co-evolution cheap, and still makes `goPureRaw` `go get`-able. Revisit Option B only if a library gains an external contributor base or needs a different release cadence or license.

---

## 9. Migration plan (ordered by effort) and progress

| # | Step | Effort | Status |
| :--- | :--- | :--- | :--- |
| 1 | Fix module path in `goPureRaw/go.mod` → `module github.com/computerphysicslab/skystacker/goPureRaw` (before any tag; changing a published path is breaking). | S, ~15 min | **Done** — path fixed, all imports updated. |
| 2 | Add root `go.work` listing `.` and `./goPureRaw`; add `go.work` to `.gitignore` (or commit it — ignored by `go install` either way). | S, ~10 min | **Done** — gitignored `go.work`; both modules build/test clean. Later bumped to 1.27.4 (§1.1). |
| 3 | Publish `v0.1.0`: tag `goPureRaw/v0.1.0`, update `goPureRaw/README.md` with `go get` usage, add CI job testing the nested module alone. | S, ~20 min | **Done** — v0.1.0 tagged; a CR2 `Probe` offset bug was found and fixed, leading to v0.1.1 (§10). |
| 4 | Use the library from SkyStacker: add `require ...goPureRaw v0.1.1` (no committed `replace`), blank-import `formats/cr2`, call `raw.Decode`, delete duplicated `pr*` decoder from `main.go`; keep `--no-native-cr2` fallback until the oracle test passes. | M, ~1–2 h | **Done** — see §10. |
| 5 | Document the change: README/TECHNICAL_SPEC say "single executable, stdlib-only" rather than "single source file"; CI builds from a clean checkout without `go.work`. | S, ~30 min | Reflected in the canonical cross-links and `GOWORK=off` verification. |
| 6 | Future sibling modules: `fitsio/` (FITS read/write + Rice), `photometry/` (HFR/PSF helpers), each with its own `go.mod` and `fitsio/vX.Y.Z`-style tags, following the same pattern. | M–L | **Planned / open** (§12 for file-format work). |

---

## 10. Modularization risks, caveats, and final state

### 10.1 Risks / caveats

- **`internal/` packages** inside goPureRaw stay private to goPureRaw by Go design; SkyStacker consumes only the exported packages **`raw`** and **`formats/cr2`**.
- **`go install github.com/computerphysicslab/skystacker@latest`** needs the parent `go.mod` to resolve the nested module from the proxy: use a real **tagged version**, not a local `replace`.
- **First-party dependency honesty:** adding a dependency (even our own module) changes the "zero external dependencies" claim. It remains stdlib-only at the code level, but source builders now need module resolution — **document it honestly**.
- **API stability:** do not split until a library's API is stable — a published module path plus semver is a compatibility promise.

### 10.2 Final modularized state

- Tagged `goPureRaw/v0.1.0`, then found and fixed a CR2 `Probe` offset bug — **signature is at bytes 8..9, not 4..5** — and published **`goPureRaw/v0.1.1`**. (Decoder internals → [07](07-native-cr2-decoding.md).)
- SkyStacker declares `require github.com/computerphysicslab/skystacker/goPureRaw v0.1.1`, imports **`raw` + `formats/cr2`**, and the duplicated **`pr*` CR2 decoder was deleted from `main.go`**.
- `GOWORK=off` build/test and E2E (**1-light CR2 stack, 7.5 s**) are green; the self-test count at this snapshot and the canonical current count are fixed in §1.
- The binary is **no longer "single source file"** but remains a **single stdlib-only executable plus one first-party module**.
- Future modules planned: `fitsio/`, `photometry/` (§9 step 6).

---

## 11. Open file-format / interoperability items (remain open)

These came out of the external survey as adjacent work and are **still open** — they are not implemented. Format layouts and writer internals → [01](01-architecture-and-core-engine.md).

- **Pure-Go FITS Rice tile compression — `--fits-rice`:** implement a pure-Go Golomb-Rice codec for **`.fits.fz`**; acceptance: integer/quantized tiles round-trip bit-exactly. ASTAP's **July 2026** changelog added native FITS Rice tile compression (`.fits.fz`) and explicitly **excluded GZIP FITS**; PixInsight 1.9.6 added tile-compressed FITS, BigTIFF, JPEG XL, and WebP. **Open.**
- **BigTIFF float32 export — `--bigtiff`:** support files **> 4 GiB** via 64-bit offsets (large wide/drizzled mosaics). **Implemented 2026-11-18** (`stacked_sum.bigtif`/`stacked_mean.bigtif`, `testBigTIFFRoundTrip`); the writer is 64-bit ready but has not been exercised on a >4 GiB file.
- **XISF 1.0 Revision 2 conformance — `--xisf-rev2`:** strict sample-mapping bounds checking and corrupt-block rejection, with **SHA-256/512 natively** and **SHA-3 only if verified in the target Go stdlib**; PixInsight 1.9.6 conforms to XISF Specification Revision 2 with parallel compressed I/O (**1.5–4.6× faster writes, 1.8–4× faster reads, 3.3–4× faster checksummed I/O, ~1.8× faster uncompressed float reads**) and **SHA3-256 / SHA3-512** block checksums plus conformance repair/warning flags; adjacent readers (openastro-ara, sergio-dr/xisf) validate sample mappings and reject corrupt blocks. Acceptance: malformed XISF sample headers fail gracefully without panics or integer overflow. **Implemented 2026-11-18** as `--xisf-rev2` with SHA3-256 block checksum, strict planar/Float32/bounds/size validation and corrupt-block rejection (`testXISFRev2RoundTrip`); parallel compressed I/O from PixInsight 1.9.6 is not reproduced.
- No **2024–2026** updates to FITS 4.0, WCS Paper II/III, or stdlib-Go **lz4/zstd vs `compress/zlib`** benchmarks were found in the gathered material.

---

## 12. Primary source URLs (each listed once)

Toolchain / runtime:

- <https://go.dev/dl/> — stable versions go1.27.4, go1.26.11 (verified 2026-11-17).
- <https://go.dev/doc/devel/release> — release dates and security revisions.
- <https://go.dev/blog/go1.27> — Go 1.27 language/tooling/runtime highlights.
- <https://go.dev/blog/go1.26> — Go 1.26 Green Tea GC, cgo, `go fix`.
- <https://www.openwall.com/lists/oss-security/2026/11/10/28> — 12 security fixes in go1.27.4 / go1.26.11.
- <https://go.dev/blog/swisstable> — "Faster Go maps with Swiss Tables" (Feb 2025).
- <https://www.datadoghq.com/blog/engineering/go-swiss-tables/> — "How Go 1.24's Swiss Tables saved hundreds of gigabytes" (2025).
- <https://goperf.dev/03-version-tracking/> — GoPerf Go 1.24–1.26 performance tracking (2025–2026).
- <https://www.alexedwards.net/blog/go-experiments-explained> — Go experiments explained (`simd/archsimd`, Go 1.26 GC) (2026).
- <https://ajitem.com/blog/performance-optimizations-in-go-1-24-swiss-table-maps-and-more/> — Sahasrabuddhe, "Performance Optimizations in Go 1.24" (Feb 2025).

Modularization:

- <https://go.dev/ref/mod> — module paths and subdirectory modules.
- <https://go.dev/doc/workspaces> — Go workspaces.
- <https://go.dev/ref/mod#go-install> — `go install pkg@version` and replace-directive behaviour.

File formats / rejection rationale:

- <https://pixinsight.net/dev/index.php?articles/pixinsight-1-9-6-lockhart-released.28/> — PixInsight 1.9.6 Lockhart release notes (date uncertain / 2025–2026).
- <https://pixinsight.net/dev/index.php?articles/pixinsight-1-9-6-lockhart-build-1718-released.30/> — Build 1718 / XISF 1.0 Specification Revision 1–2 notes.
- <https://github.com/open-astro/openastro-ara/blob/master/CHANGELOG.md> — openastro-ara changelog (2025–2026).
- <https://github.com/sergio-dr/xisf> — sergio-dr/xisf XISF 1.0 Revision 2 implementation.
- <https://www.hnsky.org/history_astap.htm> — ASTAP version history (Jan 2025 – Oct 2026).
- <https://www.hnsky.org/astap.htm> — ASTAP homepage.
- <https://polaris-astro.app.br/handbook/23-ai-cleanup.html> — Polaris Astro Handbook, AI Cleanup (284/456 MB ONNX weights).

---

## Sources absorbed

- `research/GO_TOOLCHAIN_UPGRADE_ANALYSIS.md` — baseline, verified stable versions and 2026-11-10 security fixes, advantages/disadvantages, two-step recommendation, "Decision implemented" (go1.26.11), "Current-state correction", "Code improvements adopted" (`go fix` modernizers + `testing/synctest`), and "Go 1.27.4 adopted" append-only status (toolchain scope only).
- `research/MODULARIZATION_ANALYSIS.md` — current situation, Option A vs Option B with costs/benefits, decision, migration steps 1–6 with effort estimates, risks/caveats, "Progress" (steps 1–2) and "Progress update" (steps 3–4, goPureRaw v0.1.1).
- `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — Go 1.24–1.26 stdlib/runtime gains (Swiss Tables numbers, Green Tea GC numbers, `encoding/binary`/`io.ReadAll`, `testing/synctest`, `go fix`), Go 1.26 `simd/archsimd` limits, Seti Astro 32 GiB RAM note, reproducible benchmark methodology, feasibility matrix rows for toolchain/file formats, FITS Rice / BigTIFF / XISF Rev 2 proposals, embedded-ML infeasibility (284–456 MB) and the FITS/XISF still-open status (toolchain/engineering/file-format portions only).
- `research/GEMINI_DEEP_RESEARCH_PROMPT.md` — §1 ground truth (0.6.1, single `main.go`, 203 self-tests, `CGO_ENABLED=0`, build/test constraints), §2 benchmark context (i7-8565U host), §4 F/G research questions (Go performance, reproducibility, file formats), §5 feasibility/roadmap guardrails.
- Cross-document status framing: `research/01-architecture-and-core-engine.md` §18 (canonical 262 self-tests, current toolchain and open-items snapshots) — used to fix status layers; its engine/format content is not duplicated here.

A cycle-7 curated corpus with Siril/APP/RAW-format facts is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §10.

A full goCrawler export with CFITSIO, astrometry index sizes and N.I.N.A. keyword vocabulary is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §11.

# SkyStacker — Deep Audit

This audit evaluates SkyStacker v0.6.1. The codebase is a sophisticated, high-performance astrophotography pipeline. The architecture successfully balances the "monolith" requirement with clean, first-party module boundaries.

---

### 1. Executive summary
SkyStacker is a mature, high-quality CLI tool. The transition to `goTiff` for all TIFF I/O and the integration of Gaia DR3 plate solving are significant architectural wins.

**Top 5 Findings:**
1. **TIFF I/O Consolidation:** The migration of all TIFF I/O to the `goTiff` module (item 110) has eliminated significant technical debt and improved metadata propagation (Astro-TIFF/CFA).
2. **Robustness of `goDiagnostics`:** The diagnostic module is well-isolated, but the Linux collector (`collect_linux.go`) lacks proper context handling during `/proc` walks, posing a risk of hanging on high-load systems.
3. **Flag Parsing Fragility:** The `parseArgs` function (main.go:456) relies on a hardcoded list of flags that consume values. This is a maintenance bottleneck and a source of potential CLI drift.
4. **Performance Bottleneck:** `runPostProcessing` (main.go:31707) is strictly sequential. Given the CPU-intensive nature of guided-filter denoising and deconvolution, this is the primary scalability limit for 60MP+ sessions.
5. **Test Sustainability:** The split between embedded `--selftest` and `go test` is becoming redundant. While the embedded suite provides a quick "sanity check" for users, it complicates maintenance by duplicating logic.

---

### 2. Architecture & module boundaries
The `go<Name>` rule is strictly followed.
*   **Coherence:** The boundaries are clean. `goPureRaw` handles ingestion, `goTiff` handles output, and `goDiagnostics` handles system introspection.
*   **Misplaced Responsibilities:** `main.go` remains a monolith (~34k lines). While the project constraints forbid splitting the root package, the logic for `runPostProcessing` and `accumulator` should be moved to internal files (e.g., `pipeline.go`, `stacking.go`) to improve navigability.
*   **Circular Concerns:** None detected. The dependency graph is strictly acyclic: `main` → `goTiff`, `goPureRaw`, `goDiagnostics`.

---

### 3. Correctness & robustness
*   **CONFIRMED:** `collect_linux.go` (line 179) performs a `ReadDir` on `/proc` without checking `ctx.Err()`. On a system with many zombie processes or high churn, this can block indefinitely.
*   **CONFIRMED:** `parseArgs` (main.go:456) is a fragile manual parser. Adding a new flag that requires a value necessitates updating this function, which is prone to human error.
*   **HYPOTHESIS:** The `runPostProcessing` sequential execution (P2-2) may lead to thermal throttling on high-core-count machines during long sessions, as it fails to saturate available workers.

---

### 4. Performance & scalability
*   **Hot Loops:** The per-pixel min/max loop in Pass 1 (main.go:11562) is a serial bottleneck. This should be parallelized using a tiled approach similar to the `reduceTilesStack` logic.
*   **Memory:** The 256 MB tile buffer cap is appropriate for 15 GiB hosts, but on 64 GiB+ machines, this is overly conservative and increases disk spill I/O. Consider making this a dynamic percentage of `MemAvailable`.

---

### 5. Testing gaps
*   **Sustainability:** The embedded `--selftest` (274 cases) is a "black box" for CI.
*   **Recommendation:** Migrate all `--selftest` logic into standard `*_test.go` files. Use `go test -tags=selftest` to allow users to run the full suite, while keeping the binary size small for production builds.

---

### 6. CLI, docs & UX consistency
*   **Flag Drift:** The `main-function-map.txt` shows a massive surface area. The help text is generally consistent, but the "hidden" flags (e.g., `SKYSTACKER_DEBUG_MATCH`) are not documented in `README.md`.

---

### 7. Security & privacy
*   **Path Traversal:** File discovery uses `filepath.WalkDir`. Ensure that input paths are sanitized to prevent accessing files outside the session directory.
*   **External Execs:** The use of `exec.CommandContext` for `dcraw` and `magick` is safe, but the `sidecar` logic (main.go:4452) should be audited to ensure that `{input}` and `{output}` placeholders do not allow shell injection via crafted filenames.

---

### 8. Proposed NEW roadmap items

| ID | Title | Effort | Status | Owner | Acceptance Criterion |
|---|---|---|---|---|---|
| 111 | **Context-aware /proc walk** | S | Open | `goDiagnostics` | `collect_linux.go` checks `ctx.Err()` every 100 PIDs. |
| 112 | **Dynamic tile buffer sizing** | M | Open | `main.go` | Tile buffer scales to 10% of `MemAvailable` (min 256MB). |
| 113 | **Flag-value parsing registry** | M | Open | `main.go` | `parseArgs` uses `flag.Visit` to detect value-consuming flags. |
| 114 | **Parallel post-processing** | L | Open | `main.go` | `runPostProcessing` uses a worker pool for per-channel ops. |
| 115 | **FITS Rice (.fits.fz) support** | L | Open | `goFitsio` | Bit-exact round-trip of Rice-compressed FITS tiles. |
| 116 | **SIMD-accelerated accumulation** | XL | Open | `main.go` | `addFrame` uses `golang.org/x/sys/cpu` to detect AVX2/NEON. |
| 117 | **Memory-mapped tile I/O** | XL | Open | `main.go` | `reduceTilesStack` uses `mmap` for spill files. |
| 118 | **Native CR3 CRX decoder** | XL | Open | `goPureRaw` | Decode CRX track without `dcraw` dependency. |
| 119 | **Bayer Drizzle implementation** | XL | Open | `main.go` | Per-frame drizzle resampling with weight accumulation. |
| 120 | **PSF-based deconvolution** | XL | Open | `main.go` | Richardson-Lucy with spatially varying PSF. |
| 121 | **Multi-night mosaic engine** | XL | Open | `main.go` | Overlap-based photometric normalization for mosaics. |
| 122 | **Full-frame blind Gaia solver** | XL | Open | `main.go` | Native all-sky index solves without external backends. |

---

### 9. Prioritised top-10 action list
1. **Fix `collect_linux.go` (111):** Immediate stability fix for high-load systems.
2. **Implement Flag Registry (113):** Removes the hardcoded flag list, reducing maintenance burden.
3. **Parallelize Post-Processing (114):** Highest impact on user-perceived performance for 60MP+ frames.
4. **Migrate `--selftest` to `go test`:** Improves CI/CD integration and code maintainability.
5. **Dynamic Tile Sizing (112):** Improves performance on high-RAM machines.
6. **Parallelize Pass 1 Min/Max (P2-1):** Removes a major serial bottleneck.
7. **Audit Sidecar Execs (main.go:4452):** Security hardening for external tool invocation.
8. **Implement `goFitsio` (115):** Reduces reliance on ImageMagick.
9. **Refactor `main.go` into internal files:** Improves navigability without violating the "no splitting root package" rule.
10. **Implement SIMD Accumulation (116):** Long-term performance scaling.

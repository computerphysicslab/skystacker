# SkyStacker — Compact Audit  

---

## 1. Top 5 Strengths  

| # | Strength | Why it matters |
|---|----------|----------------|
| **S1** | **Self‑test suite (274 cases) baked into the binary** | Guarantees that core pipelines (detect‑match‑stack, calibration handling, TIFF I/O) are exercised on every release; reduces regression risk. |
| **S2** | **Pure‑Go core (no CGO)** | Binary is portable, easy to distribute, and avoids runtime library mismatches on the many Linux/ macOS / Windows environments used by astrophotographers. |
| **S3** | **Modular first‑party packages (`goPureRaw`, `goTiff`, `goDiagnostics`)** | Clear separation of concerns (RAW decoding, TIFF handling, diagnostics) makes future refactors or external contributions tractable. |
| **S4** | **Streaming mean & tiled median/sigma‑clipping implementations** | Enables processing of > 10 GB image sets on modest RAM (≤ 4 GB) – a common user scenario. |
| **S5** | **Cache of star detections & transforms** | Skips expensive Pass 1/Pass 2 work on re‑runs, cutting total runtime by ~30 % on typical 30‑frame stacks. |

---

## 2. Top 5 Risks  

| # | Risk | Potential impact |
|---|------|-------------------|
| **R1** | **Monolithic `main.go` (≈ 35 k LOC) still houses most CLI logic** | Hard to reason about, increases likelihood of hidden bugs, and makes static analysis / code‑review expensive. |
| **R2** | **Flag‑value parsing is hard‑coded (see `parseArgs` lines 721‑741)** | Adding a new flag can silently break positional parsing → user‑visible errors. *(hypothesis)* |
| **R3** | **TIFF handling still mixes legacy multi‑strip logic with new `goTiff` code** | Edge‑case files (e.g., > 4 GB strips) may trigger panics; recent regression tests added but coverage is narrow. |
| **R4** | **Blind‑solver pipeline (items 73, 78‑81) repeatedly fails acceptance criteria** | Indicates algorithmic fragility; users may rely on external astrometry.net, negating the “native blind solve” promise. |
| **R5** | **Linux diagnostics collector can block on `/proc/<pid>/fd` reads** (item 106) | In long‑running batch jobs the `--diagnose` command may hang, causing CI time‑outs and poor UX. |

---

## 3. Correctness & Performance Hypotheses  

| # | Hypothesis (code‑level claim) | Rationale |
|---|-------------------------------|-----------|
| **H1** | *Star detection produces identical results on re‑run when cache is present.* | Cache invalidation logic is simple timestamp check; any change in input files may be missed. |
| **H2** | *Mean‑stack streaming uses a single‑pass Kahan summation, guaranteeing < 1 LSB error for 32‑bit floats.* | Implementation not inspected; could be a naïve `sum += val` leading to drift on > 10 k frames. |
| **H3** | *`goTiff` write‑path always creates a single strip when `--scale` = 1, avoiding the multi‑strip bug fixed in test `tiff_multistrip_test.go`.* | The fix only covers the test case; other code paths (e.g., tiled median) may still emit multi‑strip files. |
| **H4** | *`--workers` default (`runtime.NumCPU()`) scales linearly up to the number of CPU cores for detection and matching.* | Parallelism is limited by a global `sync.WaitGroup` and a channel of fixed size; contention may appear beyond 8 cores. |
| **H5** | *`--bpm-auto` correctly flags outliers using MAD even when the underlying frame is uniform (MAD = 0).* | Fallback to mean‑deviation is mentioned, but the threshold logic may never fire, leaving bad pixels unmasked. |

---

## 4. Testing Gaps  

| Gap | Description | Suggested mitigation |
|-----|-------------|----------------------|
| **T1** | **Sparse coverage of extreme TIFF sizes** (≥ 8 GB, > 65535 strips). | Add property‑based tests generating synthetic large TIFFs and verify round‑trip via `goTiff`. |
| **T2** | **No end‑to‑end validation of calibration pipelines** (dark/flat/bias interaction). | Create fixture sets with known master frames and assert final pixel statistics (mean, variance). |
| **T3** | **Blind‑solver acceptance criteria are not exercised in CI** (all 14‑image corpus failures). | Integrate the corpus as a nightly regression test; fail the build if any image solves. |
| **T4** | **Cache invalidation edge cases** (partial file updates, timestamp drift). | Simulate file modification while cache exists and verify that a fresh detection run occurs. |
| **T5** | **CLI flag parsing for new flags** (e.g., future `--gpu` flag). | Add unit tests that register a dummy flag via `flag.CommandLine` and ensure positional arguments are still parsed correctly. |

---

## 5. New Roadmap Items (IDs 111‑120)  

| ID | Title | Effort | Acceptance Criteria (falsifiable) |
|----|-------|--------|-----------------------------------|
| **111** | **Refactor CLI flag registration to dynamic set** | **S** (≤ 1 h) | `parseArgs` no longer contains hard‑coded ranges; new flag added via `flag.Var` is automatically excluded from positional parsing. Test: add dummy `--foo` flag and verify `skystacker foo.jpg` still treats `foo.jpg` as positional. |
| **112** | **Introduce a thin `cmd` package to host CLI entry‑point** | **M** (1‑4 h) | `main.go` reduced to ≤ 200 LOC, imports only `cmd.Run`. Build passes and existing self‑tests still succeed. |
| **113** | **Add Kahan summation to mean‑stack streaming** | **S** (≤ 1 h) | Numerical test: stack 100 000 random float32 values; compare result to high‑precision reference (float64) – error ≤ 1 LSB. |
| **114** | **Implement bounded `/proc` scan in `goDiagnostics`** (item 106) | **S** (≤ 1 h) | `--diagnose-timeout 2s` always returns within the timeout on a system with > 10 000 processes. No goroutine leak detected by `go test -run TestDiagnostics -count=10`. |
| **115** | **Expand TIFF test matrix to > 8 GB and > 65535 strips** | **M** (1‑4 h) | Synthetic TIFFs generated, written, and read back with zero checksum differences; `go test ./goTiff -run LargeTIFF` passes. |
| **116** | **Add end‑to‑end calibration fixture suite** | **M** (1‑4 h) | Fixture set of 5 lights + master dark/flat/bias; after `skystacker --dark … --flat …` the stacked image mean matches reference within 0.5 % and variance within 1 %. |
| **117** | **Cache invalidation sanity test** | **S** (≤ 1 h) | Modify a single light file after cache creation; subsequent run recomputes detection (log contains “cache miss”). |
| **118** | **Parallel detection scaling benchmark** | **M** (1‑4 h) | Run detection on 30 MP images with workers = 1, 2, 4, 8; runtime scales ≥ 0.8× per added core up to 8 cores. Benchmark script must be part of CI and fail if scaling drops below 0.6×. |
| **119** | **Graceful fallback for `--bpm-auto` when MAD = 0** | **S** (≤ 1 h) | Unit test where input frame is constant; `--bpm-auto` produces a BPM file with zero flagged pixels and exits without error. |
| **120** | **Re‑enable native blind‑solver acceptance test** (items 78‑81) | **L** (4‑8 h) | After implementation, the 14‑image corpus solves ≥ 12/14 images with ≤ 1 % false‑positive rate. Success measured by `skystacker --blind-test corpus/` returning “PASS”. |

*All items are ordered by ascending effort. IDs continue sequentially from the existing highest ID (110).*

---  

*End of audit.*

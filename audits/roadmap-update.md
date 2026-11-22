# Integrated roadmap update — multi-provider audit (2026-11-22)

**Sources:** `gemini-skystacker-audit.md`, `groq-skystacker-audit.md`,
`grok-skystacker-audit.md`, `openrouter-skystacker-audit.md`,
`ollama-skystacker-audit.md` (see `benchmarks.md` for method, latency and
quality scoring). Existing tracker: `ROADMAP.md` (91 Done, 2 Open at time of
writing; highest assigned ID = 110).

> **Status of these proposals:** they are **audit claims, not verified
> defects**. Each item must be reproduced/validated before implementation.
> Items already present in `ROADMAP.md`, in the P0–P4 backlog, or in the
> Deferred section were removed during integration.

## 1. What the audits agree on

| Theme | gemini | groq | grok | openrouter | ollama | Action |
|---|---|---|---|---|---|---|
| Hardcoded flag-value list in `parseArgs` | ✓ | ✓ (111) | — | — | ✓ | Promote as new item (existing 109 already covers it — **skip**, keep 109) |
| `goDiagnostics` `/proc` scan ignores context | ✓ | ✓ (114) | — | — | — | Already item 106 — **skip** |
| `runPostProcessing` ignores `--workers` | ✓ | ✓ | — | ✓ (generic) | ✓ | Promote (new) |
| Embedded `--selftest` vs `go test` duplication | ✓ | — | ✓ (123) | ✓ | ✓ | Promote (new, non-destructive version) |
| Docs/help/report drift (`--method` default, PROJECT_CONTEXT counts, README contradictions, troubleshooting blind solvers) | — | — | ✓ (111–114) | — | — | Promote (new) |
| Blind-solver verifier/index hardening (`scaleConsistentMatches`, SBI1 caps, circular mean, fuzz) | — | — | ✓ (115,121,122,130) | — | ✓ (generic) | Promote (new) |
| Security/privacy (diagnostics cmdline redaction, archive redirect auth, survey 50 Mpix cap, offline switch) | — | ✓ (generic) | ✓ (116,118,119,127) | ✓ (generic) | — | Promote (new) |
| CR2 robustness (allocation caps, container metadata instead of EOS 40D constants) | — | — | ✓ (120,124) | — | — | Promote (new) |
| Numeric stability (Kahan sum for streaming mean) | — | ✓ (113) | — | — | — | Promote (new) |
| E2E calibration / cache / scaling tests | — | ✓ (116–118) | — | — | ✓ | Promote (new) |

## 2. Proposed new roadmap items (IDs 111–136, ascending effort)

### S (≤ 1 h)

| ID | Title | Source | Falsifiable acceptance |
|---|---|---|---|
| 111 | Reconcile `--method` default across `parseArgs`, `--help`, `TECHNICAL_SPEC.md`, `README.md`, `docs/cli-reference.md` | grok 111 | One default string remains; `testParseArgs` asserts it; grep finds no other value. |
| 112 | Fix current-section drift in `PROJECT_CONTEXT.md` (274 self-tests, spike defaults 10/0.3, not "single-file") | grok 112 | Recovery line says 274; spike table uses length 10/intensity 0.3; session logs untouched. |
| 113 | Remove `README.md` self-contradictions (grid default, Go 1.20 badge, "no PSF deconvolution / no drizzle") | grok 113 | No stale claim found by grep; badge says 1.27.4; shipped `--deconvolve`/`--drizzle-upsample` acknowledged. |
| 114 | Fix troubleshooting blind-solver paragraph and `TECHNICAL_SPEC.md` "no native RAW" exclusion | grok 114 | Docs name native/astrometry-local/astap/web; spec no longer denies native CR2 decode. |
| 115 | Fix `scaleConsistentMatches` quartile index and `matches[0]` anchor | grok 115 | Test with an outlier at index 0 drops it and keeps the other three; existing test passes. |
| 116 | Redact process command lines in `goDiagnostics` output | grok 116 | Fixture cmdline with `api_key=secret` and `--mast-token hunter2` produces redacted JSON/text. |
| 117 | Fix Darwin swap field and Linux lifetime-CPU-share label in diagnostics | grok 117 | `Swapused` not stored in `SwapTotalMB`; text no longer presents the ratio as current CPU %. |
| 118 | Give archive HTTP calls a context and a redirect policy that cannot forward `Authorization` | grok 118 | Cross-host redirect receives no `Authorization`; cancelled context returns before handler. |
| 119 | Refuse survey cutouts above the documented 50 Mpix limit before dialing | grok 119 | `width*height > 50_000_000` errors with zero requests to the test server; 64×64 still works. |
| 120 | Cap `ljpeg.Decode` pixel count and CR2 decode allocation | grok 120 | Oversized SOF3 returns an error without allocating planes; `TestMinimalStream`, `TestRestartMarkers`, CR2 oracle still pass. |
| 121 | Cap SBI1 tile/quad counts before allocation | grok 121 | Oversized header rejected; small-fixture round-trip still passes and compares star coords. |
| 122 | Use a circular mean in `selectDensityUniformStars` | grok 122 | Stars at RA 0.01/6.27 rad are not split across a tile centre near π; non-wrapping tile unaffected. |
| 123 | Expose the embedded suite through `go test` without splitting `main.go` | grok 123 | `go test -run TestEmbeddedSelftest` calls `runSelfTests` and fails on any failure; 274 cases untouched. |
| 124 | Kahan summation for the streaming mean stack | groq 113 | 100k float32 values stacked; result within 1 LSB of a float64 reference. |
| 125 | Cache-invalidation sanity regression test | groq 117 | Modify one light after cache creation; next run logs a cache miss and re-detects that frame. |

### M (1–4 h)

| ID | Title | Source | Falsifiable acceptance |
|---|---|---|---|
| 126 | Read CR2 black level, white level and CFA from the container instead of EOS 40D constants | grok 124 | Synthetic CR2/TIFF header with non-1018 black and non-RGGB CFA yields those values; 40D oracle unchanged. |
| 127 | Delete the `fitsBlackLevelFor` side channel (pure function) | grok 125 | Function gone from the map; two FITS files with different `BLACK_LVL` return their own levels, order-independent. |
| 128 | Prove EXIF GPS is not consumed as ICRS RA/Dec | grok 126 | JPEG with only GPS (40 N, 3 W) does not seed RA=3°, Dec=40°; rejected with `geographic` or tested conversion. |
| 129 | One offline switch; no nova upload unless `astrometry-web` was selected explicitly | grok 127 | With `AN_API_KEY` set, `--plate-solver auto` makes zero POSTs; explicit web still uploads; offline fails before dial. |
| 130 | Replace quadratic sky-map label placement with a spatial hash | grok 128 | 1,000-source fixture keeps label strings, no overlapping boxes, neighbour lookup only. |
| 131 | Surface cone-search failures in the processing report | grok 129 | Mock HTTP 500 sets an error field; complete VOTable still sets `cone_truncated=false`. |
| 132 | Fuzz `ljpeg.Decode` and CR2 `findLosslessJPEG` as `goTiff` is fuzzed | grok 130 | `go test -fuzz=FuzzLJPEG -fuzztime=10s` passes with the two minimal streams as seeds; second target covers stray `FFC3`. |
| 133 | Dynamic tile-buffer sizing from available memory | gemini 112 | Buffer scales with `MemAvailable` (floor 256 MB); 60 MP test does not exceed 10 % of available RAM. |
| 134 | End-to-end calibration fixture suite (lights + dark/flat/bias) | groq 116 | 5-light fixture; stacked mean within 0.5 % and variance within 1 % of a stored reference. |
| 135 | Parallel-detection scaling benchmark (`--workers` 1/2/4/8) | groq 118 | ≥0.8× scaling per added core up to 8 on 30 MP images; report committed to `BENCHMARKS.md`. |

### L (4–8 h)

| ID | Title | Source | Falsifiable acceptance |
|---|---|---|---|
| 136 | Parallelise `runPostProcessing` with a worker pool (per-row/per-channel bands) | gemini 114, groq, openrouter | 60 MP post-processing time drops ≥1.7× with 4 workers; JPEG outputs byte-identical to the sequential path. |

## 3. Proposals explicitly rejected during integration

| Proposal | From | Reason |
|---|---|---|
| SIMD/AVX2/NEON accumulation, `mmap` spill I/O | gemini 116/117 | Explicitly rejected project constraint (no new deps, portable static CLI). |
| Native CR3 CRX decoder | gemini 118 | Would add a heavy dependency/algorithm outside scope; CR3 stays external. |
| Bayer drizzle, multi-night mosaic | gemini 119/121 | Already deferred in `ROADMAP.md` / `TECHNICAL_SPEC.md`. |
| Full-frame blind Gaia solver, FITS Rice | gemini 122/115 | Duplicates of item 73 and blocked item 9. |
| `cmd/` package to shrink `main.go` | groq 112 | Would split the root `package main` — forbidden. |
| "Improve error handling / add UI / optimize algorithms" | openrouter, ollama | Generic, no file references, no falsifiable acceptance criteria. |
| Retrying `--method` flag registry, `/proc` context fix | gemini/groq/ollama | Already tracked as roadmap items 109 and 106. |

## 4. Recommended integration order

1. **115** (verifier bug) — highest-confidence correctness claim in the deepest audit.
2. **111–114** (doc drift) — cheap, no code risk, prevents user confusion.
3. **120–121** (allocation caps) — cheap robustness wins for untrusted files.
4. **116–119, 129** (security/privacy) — before any new network feature.
5. **123–125, 132, 134–135** (test coverage) — makes later refactors safe.
6. **136** (parallel post-processing) — only after the test items land.
7. **126–128, 130–131, 133** — domain-specific work.

`ROADMAP.md` has not been modified by this integration pass: promoting these
items is a separate decision and should follow the canonical gap-analysis
routine (research owner + effort + acceptance criteria).

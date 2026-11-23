# 09 — Parallel post-processing

**Owner:** roadmap item 136 (`runPostProcessing` worker pool).
**Status:** research only; no code changes yet.
**Date:** 2026-11-23.

## 1. Pipeline shape

`runPostProcessing` is a **single linear chain**:

```
src → (narrowband) → (external BGE) → (multiscale bg) → (wavelet denoise) →
removeBackground (step1, saved) → (external denoise/deconv) → deconvolve →
wave-sharpen → chromatic-aberration → colour-calib → vignette → unsharp →
HDR local-contrast → star-removal (step2) → black-point shift → auto-stretch (step4) →
guided-denoise (step5) → curves (step6) → HDR-blend (step7, uses step1) →
HDR-mix (step8 ×4) → final stretches
```

Only two branches survive to the end:

- **HDR-mix**: four independent weighted blends of `step1` + `step7`.
- **Stretch variants**: three calls to `stretchContrast(cur, level)`.

Everything else is strictly ordered because each step consumes the exact
floating-point output of the previous one (the JPEG previews are a side effect).

## 2. Provider comparison (myTools/AI-API)

Both `groq` (openai/gpt-oss-120b) and `openrouter` (meta-llama/llama-3.3-70b)
were queried with `--strict` on the serialized `runPostProcessing`.

| Provider | Verdict | Useful? |
|---|---|---|
| **groq** | Accurate: identified the linear chain, the two real parallel branches, tile-level parallelism for heavy transforms, data-race risks (shared `frame.Pix`, `logf`, sidecars) and memory (12 B/px). | Yes — used as the basis for §3. |
| **openrouter** | Less accurate: claimed background-removal and star-removal can run in parallel (false — star removal consumes background-removed output); suggested parallelizing "auto-stretch variants" that do not exist as variants. | No — discarded. |

**Tool note:** `ai-api.py --strict` works well for code audit prompts up to ~16 KB.
For larger codebases, use the tiered-bundle recipe (A/B/C/mini) from the
SkyStacker multi-provider audit (2026-11-22). `--check` reports availability only,
not TPM/context caps (groq 8 000 TPM, gemini 250 k input/min).

## 3. Realistic parallelization strategy

### 3.1 Step-level parallelism (low gain)

Only three places benefit:

1. **HDR-mix (step 8)**: four blends read the same `step1` + `step7` and write
   four distinct files → run on 4 workers.
2. **Stretch variants** (`gentle`/`moderate`/`full`): three independent calls
   once `cur` is final → 3 workers.
3. **JPEG preview writes**: `writePostPreview` is pure and can be queued to a
   separate I/O pool while the CPU-bound next step runs.

Expected speedup from step-level alone: ~1.2–1.3× (only 4 of ~10 steps are
independent). Not enough for the 1.7× target.

### 3.2 Tile-level parallelism (the real lever)

Every heavy transform (`multiscaleBackground`, `waveletDenoise*`,
`deconvolve*`, `autoStretch`, `denoiseFrame`, `applyCurves`, `blendHDR`) is an
element-wise or local-neighbourhood operation. Split the frame into horizontal
tiles, process each tile on a worker, then combine. This is where the 1.7×+
speedup comes from.

Requirements:

- **Global statistics first**: compute percentiles, background polynomial
  coefficients, wavelet sigma, etc. in the main goroutine *before* tiling.
  Store them in immutable variables.
- **In-place, tile-local writes**: each worker writes only its tile slice;
  no cross-tile reads except for the (pre-computed) global stats and a halo
  of `radius` pixels at tile boundaries (copy the halo in, process, discard).
- **No shared `frame.Pix` mutation across workers**: pass each worker a
  sub-slice (`pix[tileStart:tileEnd]`) or a copy.
- **Determinism**: process tiles in index order when combining results; use a
  fixed worker count (`runtime.GOMAXPROCS` or `--workers`).

### 3.3 Data-race guards

| Risk | Guard |
|---|---|
| Shared `frame.Pix` | Tile-local slices; read-only reference for global stats. |
| `logf` interleaving | `sync.Mutex` or `log.Logger` (already safe). |
| Random spike angle | Generate once in main goroutine; pass as parameter. |
| External sidecars | Wait for completion before mutating the buffer, or copy to a temp file. |
| File writes | Unique paths per worker; serialize same-path writes. |

### 3.4 Memory budget

12 bytes/pixel (3 floats × 4 B). For a 60 MP frame: 720 MB per frame.
Worst case (worst-case simultaneous frames): ~3–4 frames → 2–3 GB.
Tile-level processing reduces peak by freeing tiles as they complete; keep at
most `workers+1` tile buffers alive.

## 4. Minimal, low-risk first increment (proposed)

Do **not** refactor the whole pipeline at once. Instead:

1. Pick **one** heavy transform (e.g. `denoiseFrame` or `applyCurves`) and add
   a `--workers`-aware tile path behind a flag (`--parallel-post`).
2. Keep the sequential path as the default and as the oracle.
3. Add a byte-identical JPEG comparison test (sequential vs parallel).
4. Measure speedup on the 30 MP benchmark from item 135.
5. If >=1.7×, promote tile parallelism to the other transforms one at a time.

This mirrors the audit-wave strategy (small, tested, reversible increments).

## 5. Acceptance criteria feasibility

- **>=1.7× with 4 workers**: realistic *only* with tile-level parallelism.
  Step-level alone gives ~1.2×. The 30 MP benchmark (item 135) shows detection
  is currently single-threaded (705 ms → 576 ms at 2 workers, no gain beyond),
  confirming that tile-level work is where the speedup must come from.
- **Byte-identical JPEGs**: achievable if the tile combination is deterministic
  (fixed tile order, no floating-point reordering) and the JPEG encoder uses
  the same quality/settings. Tile-level summation may introduce tiny float
  differences in the last ULP; use a tolerance or ensure identical reduction
  order.

## 6. Open risks

- **Halo handling**: transforms with a radius (guided filter, wavelets) need
  a border of `radius` pixels from neighbouring tiles; forgetting the halo
  produces seam artifacts.
- **Global stats**: some transforms (auto-stretch, black-point) compute
  percentiles over the whole frame; these must be computed before tiling.
- **Determinism**: tile count must be fixed (not based on runtime scheduling)
  to keep outputs byte-identical across runs.

## 7. myTools findings (2026-11-23)

### 7.1 `AI-API` (AI-API/ai-api.py)

- `--check` correctly reported availability: **groq** and **openrouter** operative;
  gemini disconnected ("model currently expired"), grok/deepseek/openai/mistral/sambanova
  no balance, ollama timeout, github disconnected.
- `--strict` with a 16 KB serialized function worked well on both groq and openrouter.
- **Finding (non-blocking):** the prompt file was passed as a positional argument, but
  the tool accepts `prompt` as a file path only — documented in README; no bug found.
- **Quality:** groq gave an accurate dependency graph and correct tile-level strategy;
  openrouter gave a more generic/partly wrong analysis (claimed dependent steps are
  independent). For code audits, groq with `--strict` is currently the better free option.

### 7.2 `GoCrawler`

- Running `go run crawler.go <profile>` from any directory other than the module
  root fails with "cannot find package goCrawler/iolib" (module paths not resolvable
  from outside). **Improvement:** resolve the module root automatically or document
  that the crawler must be run from its own directory.
- Flags use single-dash (`-max-links=N`), but `--help` shows them with single dash —
  no bug; `--max-links=N` fails with "unknown flag".
- With a custom profile (seeds go.dev/blog/pipelines, go.dev/blog/color, Wikipedia),
  the crawl processed 7 links and accepted **0**:
  `low_relevancy 1, stopword_ratio 321, too_short 3`.
  The default `regexBannedDomains` includes `(^en)\.wikipedia\.org`, so Wikipedia
  seeds were rejected before download; `go.dev` seeds returned HTTP 403/404.
- **Finding (real usability issue):** the built-in `engStopWords`/`engLowRelevancyWords`
  lists are tuned for natural-language articles and reject almost all technical
  content (`stopword_ratio 321` on Go documentation). For technical/coding research,
  the stopword lists need a "technical" profile variant or a configurable
  `stopwordRatioThreshold`.

### 7.3 Websearch tool

- `websearch` returned HTTP 402 (no balance) for a generic query. Not usable in this
  session; research relied on AI-API instead.

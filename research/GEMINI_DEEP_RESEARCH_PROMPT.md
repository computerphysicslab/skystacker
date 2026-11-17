# Gemini Deep Research prompt — SkyStacker 2026 knowledge-base expansion

Copy everything below the marker into Google Gemini in **Deep Research** mode.
It is intentionally self-contained: it describes the program, its constraints,
what is already in the knowledge base, and exactly what new information is
wanted.

---

## PROMPT (copy from here)

You are performing a deep research task for an astrophotography software project called **SkyStacker**. I need a state-of-the-art survey (priority: 2024–2026, with older foundational papers where needed) of techniques and implementation advances that are **not already covered** by the project's knowledge base, followed by a concrete feasibility assessment for this specific codebase. Do not write code; produce a structured research report with citations.

### 1. The program (ground truth — assume this is accurate)

- **Repository:** `https://github.com/computerphysicslab/skystacker` (public). Current source version **0.6.1**, tag `v0.6.1` exists but main has advanced beyond it.
- **Architecture:** a **single Go source file** (`main.go`), standard library only at build time, `CGO_ENABLED=0`, static cross-platform CLI binary (Windows/macOS/Linux). No external Python/ML runtime, no model weights, no GPU/CUDA/OpenCL, no third-party Go modules. External runtime tools are allowed only for RAW decoding (`dcraw`/`dcraw_emu`/ImageMagick). Data path is float32 RGB end-to-end; large sessions use spill files and tiled out-of-core reduction.
- **Tests:** 203 embedded self-tests via `skystacker --selftest`; CI runs gofmt, go vet, race detector, and cross-platform builds.
- **Documentation in repo:** `README.md`, `TECHNICAL_SPEC.md`, `COMPARISON.md`, `BENCHMARKS.md`, `CHANGELOG.md`, `PROJECT_CONTEXT.md`, `prompt-ai.txt`, and the `research/` tree (details in §3).

**Implemented pipeline:** discovery of JPG/JPEG/PNG/FITS/SER/CR2/CR3; optional dark/flat/bias calibration with case-insensitive `--calib-root` discovery and master export; optional metadata-pedestal bias; **sensor-space grid self-flat** (local-contrast source mask, SExtractor-style cell mode with low-quantile consistency, 5x5 median/MAD rejection, degree-4 Chebyshev + asymmetric Huber IRLS, radial-model comparison, extrapolation/positivity guards); star detection (high-pass residual, connected components, multi-sigma retry), triangle matching + RANSAC similarity, optional full affine refinement, sub-pixel bilinear resampling, safe crop, optional global/local/per-channel normalization; `mean`/`median`/MAD sigma-clipped weighted stacking with per-pixel rejection/weight/coverage maps, max/min stacks; reference-frame crop; comet stacking (pixel rate, RA/Dec rate, ephemeris file with EXIF-time interpolation); TAN WCS plate solve against Gaia DR3 with HEALPix nested tiles, TAP client, persistent cache, SIP order-2 fit.

**Post-processing:** polynomial background, `--bg-multiscale` coarse percentile background, optional Richardson–Lucy `--deconvolve`, optional `--wavesharp` multi-scale Gaussian detail enhancement, optional CA/color/flatten/unsharp/HDR stages, always-on star removal, black-point shift, STF/MTF auto-stretch, guided filter + two-scale chroma denoise, curves, HDR blend + 4 weighted mixes, diffraction spikes; `--starless` layers; `--narrowband HOO/SHO`; offline Gaia catalog annotation with `stacked_annotated.jpg` + `anomalies.json`.

**Outputs:** JPEG/PNG/BMP previews, TIFF float32, FITS (`BITPIX=-32`, TAN WCS keywords), XISF 1.0 (monolithic planar Float32), `.f32raw`, `plate_solution.json`, `report.txt`, `report.json`, `--config` JSON flag files, `--resume`.

**Explicit non-goals today:** GUI, native neural-network inference (BXT/NXT/SXT equivalents), true multi-frame drizzle resampling, mosaics, multi-night integration, PSF-fitting photometry, PCC/SPCC calibration, SIMD/GPU paths, plugin architecture, REST API. SIP is applied in WCS helpers but **not** in the stacking resampler.

### 2. Benchmark context (so you can reason about performance realistically)

Developer machine: Intel i7-8565U (4c/8t, 15 W ULV, ~2–4.6 GHz), 15 GiB RAM, Ubuntu 20.04, Go 1.20.3. Measured: OrionNebula 16 CR2 lights at 2602x3908: 6m05s (grid self-flat + dark/bias) and 7m03s (real flat); 4 lights: ~2m02s; post-processing of a 2599x3894 stack: 22.4 s default, 38.3 s with deconvolution + multi-scale sharpening. Stacking is CPU-bound and scales with cores/clock; external published comparisons (DSS on i5-6200U, Siril/PI/APP on FX-8320/i7-9700K) are documented but not apples-to-apples.

### 3. What the knowledge base ALREADY contains (do not merely repeat it)

Read/summarise these repo files if accessible; treat them as the current state of knowledge:

- `research/RESEARCH_SYNTHESIS.md` — master synthesis + prioritized engineering roadmap.
- `research/05-research-deep-dive-2026-11-16.md` — English hub with 13 sections: detector/calibration physics (sCMOS RTN, multi-Gaussian dark modelling, CDS, BFE, Ubercal/FGCM/HSC sky models, NoiseChisel); artifact rejection (percentile/Winsorized/GESDT clipping, MTF/asinh/GHS stretch equations); HEALPix/TAP client details; ASTAP `-analyse`/`-focus`, error-level scale, 32-bit HFD ceiling; memory/Swiss-table/SoA/SIMD notes; TAN–SIP references; Gaia ESA Archive v3.15 changes (DATAMODEL_* DataLink renames, 25 Nov–2 Dec 2026 DR4 blackout window, quotas); ASCOM Alpaca/WebSocket telemetry sidecar pattern; IVOA ProvenanceDM; **§13 BXT/NXT/SXT analysis** (AI deconvolution, poor-seeing wall, classical alternatives) and **§13.4 default-pipeline review**.
- `research/04-calibration-and-self-flat.md` — §17–§22: grid self-flat phases I–IV, IRLS details, Orion validation, gamma-aware note, OrionNebula grid-vs-real-flat protocol and results, SIP-in-resampler limitation note.
- `research/01-architecture-and-core-engine/core-engine-technical-specification.md` — pipeline architecture, `sync.Pool`, Swiss Tables, Roofline model, formats (TIFF/BigTIFF/FITS/XISF), optical diagnostics (HFD/tilt/SQM), comet dual-stack, ensemble photometry.
- `research/01-architecture-and-core-engine/cli-integration-and-observatory-automation.md` — ASTAP CLI flags, headless automation, output schemas.
- `research/02-astrometry-and-identification/astrometric-catalog-annotation-and-anomalies.md` — WCS/SIP projection, HEALPix bounding queries, k-d tree cross-match, anomaly manifest design.
- `research/02-astrometry-and-identification/astronomical-identification-and-sky-charts.md` — blind solving, multi-catalogue cross-match, ZOGY difference imaging, moving objects, 6-phase roadmap.
- `research/03-reference-and-literature/source-catalog-and-microarchitecture.md` and `comprehensive-technical-glossary.md` — 50+ primary references, 100-term glossary, Go runtime notes.
- `BENCHMARKS.md`, `COMPARISON.md`, `TECHNICAL_SPEC.md` for current features/performance.

### 4. Your mission: find what is MISSING and what is NEW

Focus on advances published or substantially adopted in **2024–2026**, and on classical techniques the project has not yet evaluated. For each topic, state explicitly whether it is (a) already in the KB, (b) adjacent but not implemented, or (c) genuinely new to this project.

Research questions, grouped by domain (answer all that yield useful material):

**A. Calibration and noise models**
- Recent work on dark/bias/flat modelling from lights alone; temperature-dependent dark-current modelling; sCMOS/CIS noise characterization; FPN/PRNU correction without physical flats.
- Robust illumination/vignetting estimation that does not absorb nebulosity (beyond SExtractor cell medians + low-quantile consistency): gradient-preserving masks, morphological approaches, low-rank matrix factorization, robust PCA.
- Modern detrending/systematics removal ideas (Ubercal, FGCM, EAWF, sky-background matching) applicable to small amateur datasets.

**B. Registration, astrometry and distortion**
- New open-source plate solvers / WCS fitters (ASTAP updates, Astrometry.net successors, SCAMP alternatives), Gaia DR4 implications for TAP clients and catalog queries.
- Practical algorithms for applying SIP/distortion during resampling without external libraries; TAN-SIP vs TPV vs TNX/GNOM; quad-tree/HEALPix-based distortion lookups.
- Sub-pixel registration robustness: phase correlation, Fourier-Mellon, feature descriptors beyond triangle matching; handling field rotation and scale in wide-field amateur data.

**C. Stacking statistics and outliers**
- State of the art in robust pixel rejection for small N (3–30 frames): MAD variants, Winsorized sigma, linear-fit clipping, GESDT, quantile-based methods, and how they compare for faint extended objects.
- Weighted integration: PSF/SNR-based weights, quality criteria that avoid bias toward bright stars; variance/weight propagation in float32 pipelines.
- Drizzle and multi-frame reconstruction advances; CFA/Bayer drizzle; when drizzle actually helps with modern small-pixel sensors.

**D. Deconvolution, restoration and denoising (classical, stdlib-feasible)**
- Modern classical (non-neural) deconvolution: Richardson–Lucy variants, Wiener, regularized/edge-preserving, spatially-varying PSF estimation from stars, multi-scale wavelets, and how they compare qualitatively to AI tools like BlurXTerminator on real amateur data.
- Noise estimation and denoising: noise models for stacked float32 data, guided/bilateral variants, non-local means, wavelets, and safe parameter ranges that avoid detail destruction.
- The "poor seeing / lost contrast" limit: what can and cannot be recovered; cite concrete literature or practitioner benchmarks.

**E. AI/ML advances relevant to a non-ML binary**
- Current landscape (2025–2026) of AI tools (BlurXTerminator, NoiseXTerminator, StarXTerminator, StarNet, GraXpert/ACR, etc.): what they do, documented failure modes, licensing/API availability, and whether any have batch/CLI interfaces a stdlib Go binary could orchestrate as an external sidecar.
- Lightweight on-device inference formats (ONNX runtime, GGUF, WASM) and whether a single-file Go binary could realistically embed a tiny model **without** dependencies; be explicit about feasibility limits.

**F. Computation, memory and engineering**
- Go-specific performance techniques for image pipelines available in stdlib only: concurrency patterns, cache-friendly layouts (SoA), `sync.Pool` tradeoffs, allocator pressure, bounds-check elimination, generic intrinsics status, and Rust/C++ comparisons that suggest transferable algorithms.
- Memory-mapped I/O, tiled out-of-core reducers, checksum/provenance formats, reproducible pipelines, and structured telemetry standards (IVOA ProvenanceDM updates).
- Reproducible benchmark methodology for astro stacking: what variables must be controlled for a fair comparison.

**G. File formats and interoperability**
- FITS 4.0/WCS Paper II/III updates, XISF spec revision 2 (2026) details, BigTIFF vs FITS vs XISF tradeoffs, compression codecs implementable in stdlib Go (zlib/lz4/zstd status), and metadata/provenance preservation.

### 5. Required output format

Produce a report with these sections:

1. **Executive summary** — top 10 findings, each tagged `[KB-covered]`, `[adjacent]`, or `[new]`.
2. **Domain-by-domain findings** — for each topic in §4: what is new, key papers/tools with dates, and one or two sentences on relevance to SkyStacker.
3. **Feasibility matrix** — table: technique | expected benefit | implementation effort (S/M/L/XL) | stdlib-Go feasibility (Yes/Partial/No) | external-tool/license dependency | suggested flag name.
4. **Prioritized roadmap proposals** — 5–10 concrete items that could be added to `research/` and/or the roadmap, with acceptance criteria and a small test plan.
5. **Things explicitly NOT worth pursuing** for this project, with reasons.
6. **Sources** — numbered list of primary sources (papers, official docs, release notes, maintainer posts) with publication dates and URLs. Prefer primary sources; mark anything uncertain as uncertain. Do not invent citations or URLs.

### 6. Guardrails

- Be honest about uncertainty and about the limits of a stdlib-only binary; do not suggest magic gains from AI upscaling.
- Distinguish "peer-reviewed/authoritative" from "forum practitioner report" and label them.
- Do not assume I have installed any other stacking application; comparisons must be literature-based.
- The goal is to **extend** our knowledge base with genuinely new, actionable material, not to restate what is in §3.
- Final report language: English. (I will translate summaries into Spanish for the user.)

## PROMPT (copy until here)

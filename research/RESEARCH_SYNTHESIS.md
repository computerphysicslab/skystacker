# SkyStacker Research Synthesis

**Status:** Active development (single-file Go CLI, version reported as `0.4.1` / moving toward `0.5.0`)  
**Last synthesized:** 2026-11-11  
**Purpose:** Consolidate the research folder into one clear reference that describes what the software currently does, the mathematical and architectural foundations it is built on, known limitations, and a prioritized roadmap for improvement.

This document draws from `01-architecture-and-core-engine/current-implementation-v0.4.1.md`, the technical specifications, the astrometric research, and the source compendia. All detailed documents remain available in their respective domain folders for deeper reading.

---

## 1. Current Implementation Status

SkyStacker is a **local-first, single-file Go CLI** that automates the core deep-sky stacking workflow:

1. **Discover frames:** JPG/JPEG, CR2/CR3, FIT/FITS (optional recursive).
2. **Decode and prepare float32 pixel data:** Native JPEG; RAW/FITS via external ImageMagick / `dcraw`.
3. **Optional photometric calibration:** Master dark / flat / bias.
4. **Star detection & reference-frame selection.**
5. **Triangle-based pattern matching:** Similarity transform (scale + rotation + translation) refined by inliers.
6. **Bilinear resampling & safe crop:** Restricted to the fully valid overlap region.
7. **Statistical stacking:** Mean / median / sigma-clipping, plus diagnostic max/min stacks.
8. **Multi-stage post-processing:** Background removal, optional CA reduction, color calibration, stretch, sharpening, star control, etc.
9. **Atomic outputs & telemetry:** Detailed `report.txt`.

### Key Design Choices Already Present
* Sub-pixel centroids and bilinear resampling.
* Robust noise estimation (MAD/IQR) and iterative outlier rejection.
* Registration quality gates ($\ge 8$ inliers, inlier ratio $\ge 0.20$).
* Memory estimator with `GOGC` tuning and `--force` override.
* Extensive internal self-tests (`--selftest`).
* Multi-OS CI (build, self-test, `gofmt`, race detector, `go install`).

### Post-Processing Pipeline (Recent Context)
Background removal $\rightarrow$ star removal $\rightarrow$ black-point $\rightarrow$ STF/MTF stretch $\rightarrow$ guided-filter denoise $\rightarrow$ curves $\rightarrow$ HDR blend $\rightarrow$ weighted mixes. Individual stages produce preview JPEGs.

---

## 2. Core Mathematical Foundations

### 2.1 Photometric Calibration

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

Master darks remove bias + thermal dark current; master flats correct vignetting and dust; the mean flat scalar ($\bar{I}_{\text{flat}}$) preserves dynamic range.

### 2.2 Signal-to-Noise Improvement
Photon shot noise follows Poisson statistics. Stacking $N$ frames yields:

$$\text{SNR}_{\text{stack}} \approx \sqrt{N} \cdot \text{SNR}_{\text{sub}}$$

* **Sigma-clipping:** Approaches $\sim 0.95\sqrt{N}$ while rejecting outliers.
* **Median:** More robust to extreme outliers, but $\sim 20\%$ less statistically efficient ($\approx 0.80\sqrt{N}$).
* **Pure mean:** Optimal ($1.00\sqrt{N}$) only for pure Gaussian noise without transient outliers.

### 2.3 Star Registration
* **Detection:** 8-connected components above a dynamic $\sim 5\sigma$ threshold after high-pass residual.
* **Sub-pixel centroids (intensity-weighted moments):**

  $$x_c = \frac{\sum x \cdot I}{\sum I}, \quad y_c = \frac{\sum y \cdot I}{\sum I}$$

* **Descriptors:** Scale- and rotation-invariant triangle descriptors from the brightest stars (side-length ratios).
* **Transform & Crop:** Closed-form similarity transform + RANSAC-style inlier filtering, followed by a safe crop to the largest axis-aligned rectangle valid in every registered frame.

### 2.4 Quality Scoring (Planned / Partial)

$$Q_i = w_1 \cdot \frac{1}{\text{FWHM}_i} + w_2 \cdot (1 - e_i) + w_3 \cdot \text{SNR}_i + \ldots$$

Used to prefer sharper, rounder, higher-SNR frames as the geometric reference or for weighted integration.

### 2.5 Generalized Hyperbolic Stretch (GHS)

$$f(x) = \frac{\sinh^{-1}\bigl(\frac{x-D}{b}\bigr) - \sinh^{-1}\bigl(\frac{-D}{b}\bigr)}{\sinh^{-1}\bigl(\frac{1-D}{b}\bigr) - \sinh^{-1}\bigl(\frac{-D}{b}\bigr)}$$

Preserves stellar color better than simple gamma or arcsinh stretches while lifting faint nebulosity.

---

## 3. Architecture Principles

* **Local-first:** Everything runs on the user's machine; zero cloud dependency.
* **Resource-conscious:** Memory estimation before processing; tile-oriented design for future large datasets; `sync.Pool`-style buffer reuse (`*TileBuffer`) for zero-allocation hot paths.
* **Transparent:** Per-frame diagnostics in `report.txt`; intermediate preview JPEGs.
* **Numerically careful:** `float32` accumulation; atomic file writes.
* **Concurrent:** Worker pools; `context.Context` cancellation patterns for clean shutdown and leak prevention.
* **Static binary friendly:** `CGO_ENABLED=0` target for frictionless cross-platform distribution.

Advanced notes in the source compendia also cover Swiss Tables (Go 1.24+), Structure-of-Arrays (SoA) layouts, SIMD inspection, and an optional GPU Hardware Abstraction Layer (CUDA/Vulkan/Metal) for future pixel-heavy stages.

---

## 4. Strengths & Known Limitations

### Strengths
* Practical end-to-end workflow covering the most common astrophotography pain points (alignment, gradients, stretch, star control).
* Carefully engineered registration (triangle descriptors + chirality + inlier refinement + safe crop).
* Comprehensive diagnostic output and internal self-tests (`--selftest`).
* Memory safety checks and atomic outputs.
* Clean, predictable CLI contract.

### Known Limitations
* Registration is similarity-only (no optical distortion model, no full affine). Adequate for short-to-medium focal lengths, less ideal for very wide fields or untracked mounts with significant field rotation.
* Star detection uses a fixed high-pass radius; dense star fields or very large saturated stars can still cause merges or misses.
* Post-processing is heuristic (no photometric/SPCC calibration, no deconvolution, no noise-aware stretch).
* Heavy reliance on external tools (ImageMagick / `dcraw`) for RAW and FITS decoding.
* Single-file implementation makes large-scale refactoring harder.
* Median/sigma paths and full accumulation may still materialize more data than a pure tile-streaming design would.
* No drizzle, local normalization, plate solving, or multi-night intelligence yet.

---

## 5. Prioritized Engineering Roadmap

Ordered by practical impact for typical users:

1. **Frame quality weighting & better selection** — Incorporate FWHM, eccentricity, and SNR into reference choice and optional weighted stacking.
2. **Stronger background modeling** — Multiscale or sample-based extraction; optional local normalization (LNC).
3. **Improved rejection & maps** — Better sigma-clipping, rejection maps, and per-pixel weight maps.
4. **Modularization** — Extract registration, stacking, post-processing, and I/O into clean Go packages while keeping the CLI thin. This unlocks the rest of the roadmap.
5. **Native format improvements** — Reduce external-tool dependency; better native FITS/XISF/metadata handling.
6. **Advanced registration & astrometry** — Optional higher-order transforms or field-rotation handling; WCS / blind plate solving (Astrometry.net/ASTAP adapters + Gaia DR3 cross-matching).
7. **Photometric color calibration (PCC/SPCC)** — Gaia-based stellar color correction.
8. **Optional advanced modes** — Multi-night integration, mosaics, comet/moving-object dual registration, drizzle reconstruction.
9. **Performance** — Tile streaming, pointer-wrapped `sync.Pool` memory pools, optional SIMD, and later GPU HAL backends.
10. **Optional AI modules** — Denoising, star removal, defect detection (kept strictly optional and local).

---

## 6. Guiding Principles for Implementation

* Treat [`01-architecture-and-core-engine/current-implementation-v0.4.1.md`](01-architecture-and-core-engine/current-implementation-v0.4.1.md) and the live `--help` / `--selftest` suite as the source of truth for what is implemented today.
* Prefer incremental, testable improvements that preserve the transparent, local-first character of the tool.
* When implementing new algorithms (quality weighting, local normalization, better rejection, WCS overlays), add corresponding self-tests and `report.txt` diagnostics.
* Keep external-tool dependencies explicit and documented; prefer pure-Go paths when they do not sacrifice correctness or simplicity.
* Modularize the codebase before attempting large architectural changes (tile streaming, GPU HAL, plugin system).

---

*This synthesis is intended to be the living entry point for the research folder. Update it when major implementation milestones are reached.*

## Transfer status at v0.6.0 (2026-11-15)

In production in `main.go`: native FITS read/debayer, Gaia TAP + HEALPix
nested queries, TAN WCS plate solving (`plate_solution.json`), star/transform
cache version 2, calibration masters, self-flat v1 and grid Phase 1, SER
input, pixel-mode comet stacking. SIP is fit and stored, not applied.
Catalog annotation, drizzle, XISF, and RA/Dec comet ephemeris are still
research. Registered self-tests: 177. Persistent fixtures live in
`assets/testdata/`.

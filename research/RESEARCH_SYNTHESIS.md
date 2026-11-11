# SkyStacker Research Synthesis

**Status:** Active development (single-file Go CLI, version reported as 0.4.1 / moving toward 0.5.0)  
**Last synthesized:** 2026-11-11  
**Purpose:** Consolidate the research folder into one clear reference that describes what the software currently does, the mathematical and architectural foundations it is built on, known limitations, and a prioritized roadmap for improvement.

This document draws from `analysis.md`, the technical specifications, the advanced processing guide, the source catalogs, and the project context files. The original documents remain in this folder for deeper reading.

---

## 1. Current Implementation Status

SkyStacker is a **local-first, single-file Go CLI** that automates the core deep-sky stacking workflow:

1. Discover frames (JPG/JPEG, CR2/CR3, FIT/FITS; optional recursive).
2. Decode and prepare float32 pixel data (native JPEG; RAW/FITS via external ImageMagick / dcraw).
3. Optional photometric calibration (master dark / flat / bias).
4. Star detection + reference-frame selection.
5. Triangle-based pattern matching → similarity transform (scale + rotation + translation) refined by inliers.
6. Bilinear resampling + safe crop to the fully valid overlap region.
7. Statistical stacking (mean / median / sigma-clipping) plus diagnostic max/min stacks.
8. Multi-stage post-processing (background removal, optional CA reduction, color calibration, stretch, sharpening, star control, etc.).
9. Atomic outputs + detailed `report.txt`.

**Key design choices already present:**
- Sub-pixel centroids and bilinear resampling.
- Robust noise estimation (MAD/IQR) and iterative outlier rejection.
- Registration quality gates (≥ 8 inliers, inlier ratio ≥ 0.20).
- Memory estimator with GOGC tuning and `--force` override.
- Extensive internal self-tests (`--selftest`).
- Multi-OS CI (build, self-test, gofmt, race detector, `go install`).

**Post-processing pipeline (as of recent context):** background removal → star removal → black-point → STF/MTF stretch → guided-filter denoise → curves → HDR blend → weighted mixes. Individual stages produce preview JPEGs.

---

## 2. Core Mathematical Foundations

### 2.1 Photometric Calibration

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

Master darks remove bias + thermal dark current; master flats correct vignetting and dust; the mean flat scalar preserves dynamic range.

### 2.2 Signal-to-Noise Improvement

Photon shot noise follows Poisson statistics. Stacking $N$ frames yields:

$$\text{SNR}_{\text{stack}} \approx \sqrt{N} \cdot \text{SNR}_{\text{sub}}$$

Sigma-clipping approaches ~0.95√N while rejecting outliers; median is more robust but ~20% less efficient; pure mean is optimal only for pure Gaussian noise.

### 2.3 Star Registration

- Detection: 8-connected components above a dynamic ~5σ threshold after high-pass residual.
- Sub-pixel centroids (intensity-weighted moments):

  $$x_c = \frac{\sum x \cdot I}{\sum I}, \quad y_c = \frac{\sum y \cdot I}{\sum I}$$

- Scale- and rotation-invariant triangle descriptors from the brightest stars (side-length ratios).
- Closed-form similarity transform + RANSAC-style inlier filtering.
- Safe crop to the largest axis-aligned rectangle valid in every registered frame.

### 2.4 Quality Scoring (planned / partial)

$$Q_i = w_1 \cdot \frac{1}{\text{FWHM}_i} + w_2 \cdot (1 - e_i) + w_3 \cdot \text{SNR}_i + \ldots$$

Used to prefer sharper, rounder, higher-SNR frames as reference or for weighting.

### 2.5 Generalized Hyperbolic Stretch (GHS)

$$f(x) = \frac{\sinh^{-1}\bigl(\frac{x-D}{b}\bigr) - \sinh^{-1}\bigl(\frac{-D}{b}\bigr)}{\sinh^{-1}\bigl(\frac{1-D}{b}\bigr) - \sinh^{-1}\bigl(\frac{-D}{b}\bigr)}$$

Preserves stellar color better than simple gamma or arcsinh while lifting faint nebulosity.

---

## 3. Architecture Principles

- **Local-first:** Everything runs on the user’s machine; no cloud dependency.
- **Resource-conscious:** Memory estimation before processing; tile-oriented thinking for future large datasets; `sync.Pool`-style buffer reuse discussed for zero-allocation hot paths.
- **Transparent:** Per-frame diagnostics in `report.txt`; intermediate preview JPEGs.
- **Numerically careful:** Float32 accumulation; atomic writes.
- **Concurrent:** Worker pools; `context.Context` cancellation patterns described for clean shutdown.
- **Static binary friendly:** CGO_ENABLED=0 target for easy distribution.

Advanced notes in the source catalogs discuss Swiss Tables (Go 1.24+), SoA layouts, SIMD inspection, and optional GPU HAL (CUDA/Vulkan) for future pixel-heavy stages.

---

## 4. Strengths

- Practical end-to-end workflow covering the most common pain points (alignment, gradients, stretch, star control).
- Carefully engineered registration (triangle descriptors + chirality + inlier refinement + safe crop).
- Good diagnostic output and self-tests.
- Memory safety checks and atomic outputs.
- Clean CLI contract.

## 5. Known Limitations

- Registration is similarity-only (no distortion, no full affine). Adequate for most short-to-medium focal lengths; less ideal for very wide fields or untracked mounts with significant field rotation.
- Star detection uses a fixed high-pass radius; dense fields or very large stars can still cause merges or misses.
- Post-processing is heuristic (no photometric/SPCC calibration, no deconvolution, no noise-aware stretch).
- Heavy reliance on external tools (ImageMagick / dcraw) for RAW and FITS.
- Single-file implementation makes large-scale refactoring harder.
- Median/sigma paths and full accumulation may still materialize more data than a pure tile-streaming design would.
- No drizzle, local normalization, plate solving, or multi-night intelligence yet.

---

## 6. Prioritized Roadmap

Aligned with the planned-features section in the main README and the research notes. Ordered by practical impact for typical users:

1. **Frame quality weighting & better selection** — Incorporate FWHM, eccentricity, and SNR into reference choice and optional weighted stacking.
2. **Stronger background modeling** — Multiscale or sample-based extraction; optional local normalization.
3. **Improved rejection & maps** — Better sigma-clipping, rejection maps, and per-pixel weight maps.
4. **Modularization** — Extract registration, stacking, post-processing, and I/O into packages while keeping the CLI thin. This unlocks the rest of the roadmap.
5. **Native format improvements** — Reduce external-tool dependency; better FITS/XISF/metadata handling.
6. **Advanced registration** — Optional higher-order transforms or simple field-rotation handling; later WCS / plate solving (Astrometry.net + Gaia).
7. **Photometric color calibration (PCC/SPCC)** — Gaia-based stellar color correction.
8. **Optional advanced modes** — Multi-night integration, mosaics, comet/moving-object dual registration, drizzle.
9. **Performance** — Tile streaming, better memory pools, optional SIMD, and later GPU backends.
10. **Optional AI modules** — Denoising, star removal, defect detection (kept clearly optional and local).

---

## 7. How This Research Should Guide the Code

- Treat the current `analysis.md` snapshot and the live `--help` / self-tests as the source of truth for what is implemented today.
- Prefer incremental, testable improvements that preserve the transparent, local-first character of the tool.
- When implementing new algorithms (quality weighting, local normalization, better rejection), add corresponding self-tests and report.txt diagnostics.
- Keep external-tool dependencies explicit and documented; prefer pure-Go paths when they do not sacrifice correctness or simplicity.
- Modularize before attempting large architectural changes (tile streaming, GPU, plugin system).

---

## 8. Source Documents in This Folder

- `analysis.md` — Concrete analysis of the v0.4.1 single-file code.
- `skystacker_advanced_processing_and_stacking_guide.md` — Pipeline stages, calibration equation, stacking comparison, GHS.
- `skystacker_technical_specification.md` and related files — Extended architecture, tile streaming, concurrency, and format notes.
- Source catalogs (`skystacker_*compendium*.md`) — References to ASTAP, Siril, PixInsight, Astropy, Swiss Tables, WCS/SIP, GraXpert, wavelets, CUDA/Vulkan, etc.
- Cross-platform automation specification — Ideas for observatory integration and pipeline control.

The catalogs are useful for inspiration and citations; the synthesis above extracts the parts most relevant to SkyStacker’s near-term evolution.

---

*This synthesis is intended to be the living entry point for the research folder. Update it when major implementation milestones are reached.*

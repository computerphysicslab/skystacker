# Current Implementation Snapshot: SkyStacker v0.4.1

**Scope:** Concrete architectural and functional audit of the `v0.4.1` single-file Go CLI implementation.

---

## 1. Overview

**SkyStacker v0.4.1** is a single-file Go CLI tool for stacking and post-processing astrophotography frames (`JPG`/`JPEG`, `CR2`/`CR3`, `FITS`). It performs optional star-based registration, photometric calibration, multi-method stacking, and a multi-stage post-processing pipeline aimed at producing display-ready images.

---

## 2. Core Pipeline Stages

### 2.1 Discovery & Validation
* Walks the input directory (optionally recursive), accepting `.jpg`, `.jpeg`, `.cr2`, `.cr3`, `.fits`, and `.fit`.
* Soft-rejects individual decode failures without aborting the entire batch.
* **Fails hard** if the folder mixes image dimensions (evaluated after optional `--scale`).
* Output defaults to `<input>/0_skystacker_results`.

### 2.2 Photometric Calibration (Optional)
* Supports master dark, flat, and bias frames (either a single master file or a folder of calibration frames that are mean-stacked).
* Applied as dark subtraction followed by flat division (with bias correction applied to the flat).

### 2.3 Star Detection & Registration (Default: On)
* **High-pass residual:** Box blur radius `12` combined with a robust MAD noise estimate.
* **Connected-component detection:** 8-connectivity, component size `1–200 px`, with an aspect-ratio filter to reject elongated artifacts.
* **Centroiding:** Sub-pixel flux-weighted centroids.
* **Reference frame selection:** Selects the frame with the highest detected star count.
* **Pattern matching:** Triangle-invariant matching (canonical side ratios, chirality check, `0.01` grid bucketing) paired with a similarity transform (scale + rotation + translation) iteratively refined by inliers.
* **Resampling & safe crop:** Bilinear resampling into the reference coordinate system; pixels mapping outside the source frame are marked invalid (`-1`). Performs a safe crop to the largest axis-aligned rectangle where every frame contributed a valid pixel (maximal-rectangle-in-histogram algorithm).

### 2.4 Statistical Stacking
* Accumulator supports streaming **mean**, plus **median** and **sigma-clipping** (via CLI flags).
* Registered frames use a masked addition pass that tracks per-pixel valid frame counts.
* Concurrently generates diagnostic no-correction mean, sum, max, and min stacks.
* Includes a pre-flight memory estimator paired with `GOGC=50` tuning and a `--force` override safety check.

### 2.5 Post-Processing (Default: On; Skippable via `--stack-only` / `--post-only`)
Reads the `float32` raw mean stack and generates a sequence of intermediate preview JPEGs:
1. **Polynomial background removal:** Degree `1–3`, robust iterative fit on block medians, with an optional bright-object mask.
2. **Optional corrective & enhancement filters:** Chromatic aberration (CA) reduction, color calibration (mid-tone neutralization), vignette flatten, unsharp mask, and HDR local contrast.
3. **Star removal:** Annulus median replacement using a size-adaptive mask radius.
4. **Black-point shift:** Percentile-based luminance floor adjustment.
5. **Auto-stretch:** PixInsight-style STF/MTF auto-stretch (linked or unlinked channels, shadow clip parameterized in $\sigma$, configurable target background).
6. **Denoising:** Guided-filter denoise with multi-pass strength control.
7. **HDR & Weighted Mixes:** HDR blend combining stretched and linear detail, plus weighted mixes of the background-removed image vs. the HDR result.
8. **Optional multi-level stretch variants.**

*(Optional `--tiff` flag writes an uncompressed `float32` RGB TIFF of the raw sum.)*

---

## 3. Notable Design Choices

* **Single-file architecture** embedding extensive internal self-tests (`--selftest`).
* **RAW/FITS decoding:** Handled via external `dcraw` / `dcraw_emu` + ImageMagick `convert` (or a pure ImageMagick fallback).
* **Sub-pixel precision throughout:** Centroiding, bilinear resampling, and similarity matrix estimation all operate in continuous floating-point coordinates.
* **Statistical robustness:** IQR/MAD noise estimation, iterative outlier rejection in background fitting, inlier-ratio + absolute-count gates for registration, and quality scoring that penalizes large scale or rotation deviations.
* **Rich diagnostics:** Writes a detailed `report.txt` containing per-frame star counts, inlier counts, scale/rotation/offset parameters, quality scores, and transformed bounding boxes.
* **Memory hygiene:** Atomic file writes, `GOGC` tuning, and an explicit peak-memory estimator that accounts for Garbage Collector lag.

---

## 4. Strengths & Observed Limitations

### Strengths
* Practical, self-contained astrophotography workflow addressing the most common deep-sky pain points (alignment, gradients, star control, stretching).
* Carefully engineered registration engine (triangle descriptors + chirality + RANSAC-style refinement + safe crop).
* Individually previewable post-processing stages make visual debugging straightforward.
* Clear, actionable error messages (mixed dimensions, OOM prevention, registration failures).
* Solid unit-test coverage of math-heavy routines (similarity transform, MTF, guided filter, background fit, star removal, etc.).

### Limitations & Observations
* **Median/sigma stacking** and parts of the main accumulation loop are truncated in the reviewed source; the memory estimator conservatively assumes they materialize all frames in RAM.
* Registration is similarity-only (no optical distortion modeling, no full affine transform)—well suited for short-focal-length imaging, less ideal for very wide fields or untracked mounts experiencing field rotation.
* Star detection relies on a fixed high-pass radius; very large saturated stars or extremely dense star fields can still merge or be missed.
* Post-processing is entirely heuristic (no spectrophotometric color calibration, no PSF deconvolution, no noise-aware stretch).
* Relies on external `dcraw` + ImageMagick binaries for RAW and FITS ingestion.
* Lacks GPU acceleration, multi-scale registration, drizzle reconstruction, and local normalization.
* The post-processing chain has numerous sequential stages that can over-process an image if every optional flag is enabled simultaneously.

---

## 5. Overall Assessment

`v0.4.1` is a capable, well-structured single-binary stacker featuring a thoughtfully designed registration engine and a transparent multi-step post-processing chain. It is clearly geared toward practical, reproducible deep-sky imaging rather than scientific photometry.

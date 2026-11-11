**SkyStacker v0.4.1** is a single-file Go CLI tool for stacking and post-processing astrophotography frames (JPG/JPEG, CR2/CR3, FITS). It performs optional star-based registration, calibration, multi-method stacking, and a multi-stage post-processing pipeline aimed at producing display-ready images.

### Core Pipeline
1. **Discovery & validation**  
   Walks the input directory (optionally recursive), accepts `.jpg`/`.jpeg`/`.cr2`/`.cr3`/`.fits`/`.fit`, soft-rejects decode failures, and **fails hard** if the folder mixes image dimensions (after optional `--scale`). Output defaults to `<input>/0_skystacker_results`.

2. **Calibration** (optional)  
   Supports master dark / flat / bias (single file or folder of frames that are mean-stacked). Applied as dark subtraction then flat division (with bias correction on the flat).

3. **Star detection & registration** (default on)  
   - High-pass residual (box blur radius 12) + robust MAD noise estimate.  
   - Connected-component detection (8-connectivity, size 1–200 px, aspect-ratio filter).  
   - Sub-pixel flux-weighted centroids.  
   - Reference frame = the one with the most stars.  
   - Triangle-invariant matching (canonical side ratios, chirality check, 0.01 grid bucketing) + similarity transform (scale + rotation + translation) refined by inliers.  
   - Bilinear resampling into the reference coordinate system; pixels outside the source are marked invalid (`-1`).  
   - Safe crop to the largest axis-aligned rectangle where every frame contributed a valid pixel (maximal-rectangle-in-histogram).

4. **Stacking**  
   Accumulator supports mean (streaming), and (from flags) median / sigma. Registered frames use a masked add that tracks per-pixel valid counts. Also produces no-correction mean, sum, max, and min. Memory estimator + GOGC=50 + `--force` safety check.

5. **Post-processing** (default on, skippable with `--stack-only` / `--post-only`)  
   Reads the float32 raw mean and produces a sequence of preview JPEGs:
   - Polynomial background removal (degree 1–3, robust iterative fit on block medians, optional bright-object mask).
   - Optional CA reduction, color calibration (mid-tone neutralization), vignette flatten, unsharp mask, HDR local contrast.
   - Star removal (annulus median replacement, size-adaptive radius).
   - Black-point percentile shift.
   - PixInsight-style STF/MTF auto-stretch (linked or unlinked, shadow clip in σ, target background).
   - Guided-filter denoise (multi-pass strength).
   - HDR blend (stretched + linear detail).
   - Weighted mixes of background-removed vs HDR result.
   - Optional multi-level stretch variants.

   Optional `--tiff` writes a float32 RGB TIFF of the sum.

### Notable Design Choices
- **Single-file** with extensive internal self-tests (`--selftest`).
- **CR2/CR3** via external `dcraw`/`dcraw_emu` + ImageMagick `convert` (or pure ImageMagick fallback).
- **Sub-pixel** everything: centroiding, bilinear resampling, similarity estimation.
- **Robustness**: IQR/MAD noise, iterative outlier rejection in background fit, inlier-ratio + absolute-count gates for registration, quality scoring that penalizes large scale/rotation deviations.
- **Diagnostics**: detailed `report.txt` with per-frame star counts, inliers, scale/rotation/offset, quality score, and transformed bounding boxes.
- **Memory hygiene**: atomic writes, GOGC tuning, explicit peak-memory estimate that accounts for GC lag.

### Strengths
- Practical, self-contained astrophotography workflow that covers the most common pain points (alignment, gradients, star control, stretch).
- Registration is carefully engineered (triangle descriptors + chirality + RANSAC-style refinement + safe crop).
- Post-processing steps are individually previewable, which makes debugging easy.
- Good error messages (mixed dimensions, OOM prevention, registration failures).
- Solid unit-test coverage of the math-heavy pieces (similarity, MTF, guided filter, background fit, star removal, etc.).

### Limitations & Observations
- **Median/sigma stacking** and parts of the main accumulation loop are truncated in the provided source; the memory estimator assumes they materialize all frames.
- Registration is similarity-only (no distortion, no full affine). Suitable for most short-focal-length work, less ideal for very wide fields or untracked mounts with field rotation.
- Star detection uses a fixed high-pass radius; very large stars or extremely dense fields can still merge or be missed.
- Post-processing is entirely heuristic (no photometric calibration, no deconvolution, no proper noise-aware stretch).
- External dependency on `dcraw` + ImageMagick for raw/FITS.
- No GPU, no multi-scale registration, no drizzle, no local normalization.
- The post-processing pipeline has many sequential stages that can over-process if the user enables every optional flag.

Overall this is a capable, well-structured single-binary stacker with a thoughtfully designed registration engine and a transparent multi-step post-processing chain. It is clearly aimed at practical deep-sky imaging rather than scientific photometry.


# SkyStacker — Technical Specification

**Target release:** 0.5.0
**Implementation language:** Go
**Status:** Active Development

## 1. Purpose
SkyStacker is a local command-line program that discovers, aligns, calibrates, and stacks astronomical images. It produces high-dynamic-range float32 TIFFs and a fully post-processed display image, utilizing advanced statistical stacking and luminance-based stretching.

## 2. Scope
### Included
- CLI executable named `skystacker`.
- Input formats: JPG/JPEG (native), CR2/CR3/FITS (via ImageMagick).
- Star detection, triangle matching, and RANSAC similarity transforms.
- Calibration frames (dark, flat, bias).
- Stacking methods: Mean, Median, Sigma-Clipping (default).
- Max/Min stacks (brightest/darkest pixels).
- 6-step post-processing pipeline.
- Memory and time estimation reporting.
- 37 embedded self-tests.

### Excluded
- Native RAW/FITS decoding (delegated to ImageMagick).
- Drizzle or affine registration.
- GUI.

## 3. CLI Contract
```text
skystacker [flags] [input-directory]
```

**Flags:**
- `--method` (default `sigma`): `mean`, `median`, or `sigma`.
- `--dark`, `--flat`, `--bias`: Paths to calibration frames.
- `--mask-bg`: Enables object masking during background extraction (default false).
- `--stack-only`, `--post-only`: Pipeline control.
- `--no-register`: Skips star alignment.
- `--workers`: Concurrent processing limit.

## 4. Numeric Model & Algorithms

### 4.1 Stacking
- **Mean**: Simple arithmetic average.
- **Median**: Sorts pixel values across all frames and takes the middle.
- **Sigma-Clipping**: Calculates the median and standard deviation per pixel; rejects values `> 2.5 sigma` from the median before averaging.
- **Max/Min**: Tracks the highest and lowest value per pixel across all frames.

### 4.2 Star Registration
- **Detection**: 8-connected components above a 5-sigma threshold. Centroids calculated via intensity-weighted average.
- **Matching**: Triangles built from the 30 brightest stars. Descriptor: `(short/long, mid/long)`.
- **Transform**: Closed-form similarity transform (rotation, scale, translation) via least squares.

### 4.3 Post-Processing
1. **Background Removal**: Fits a 2nd-degree 2D polynomial to the image, rejecting pixels `> 2 sigma` above the fit. If `--mask-bg` is passed, pixels `> 0.1` luminance are masked out entirely.
2. **Chromatic Aberration**: Suppresses magenta/violet halos by detecting pixels where both R and B are significantly higher than G.
3. **Color Calibration**: Computes channel medians from the 10th-50th percentile of luminance. Applies a scale factor clamped to `[0.5, 1.5]`. This runs *before* stretching to prevent color casts from being amplified.
4. **Luminance GHS Stretch**: Applies Generalized Hyperbolic Stretch to the luminance channel only, then scales RGB proportionally. This preserves the color of the stars and prevents independent channel clipping. 3 stages: Gentle, Moderate, Full.
5. **Vignette Flattening**: Re-applies background removal after stretching to remove residual localized gradients.
6. **Unsharp Mask**: Standard high-pass sharpening filter.

## 5. Memory & Time Estimation
Before processing begins, the software estimates:
- **Time**: `frames * megapixels * 0.35 seconds + 10 seconds (post-processing)`.
- **Memory**: `width * height * 3 * 4 bytes * 5 (buffers) + (frames * frameMB if median/sigma)`.
It reads `/proc/meminfo` to compare against available RAM and prints a warning if memory is insufficient.

## 6. Output Format
- **TIFF**: Uncompressed, little-endian, RGB, IEEE float32. Stored in `TIFF/` subdirectory.
- **JPG**: 8-bit previews, gamma 0.5 encoded. Stored in the root output directory.

## 7. Error Handling
- Corrupt files are skipped and logged to `report.txt`.
- Output uses atomic writes (`.tmp` file renamed on success).
- If `--post-only` is specified, it verifies `stacked_mean.tiff` exists before starting.

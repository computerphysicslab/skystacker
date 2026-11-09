# SkyStacker

**Fast, lightweight, local-first Go CLI for astrophotography image stacking.**

> Drop a folder of JPG/RAW exposures. Get registered, high-dynamic-range TIFFs, max/min stacks, plus a fully post-processed preview and a detailed report.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-active%20development-orange)]()

SkyStacker is an open-source command-line tool written in Go that discovers, aligns, and stacks astronomical images. It emphasizes numerical precision, bounded memory use, reproducibility, and a minimal user experience. It features a robust star-registration engine and a 6-stage post-processing pipeline.

**Current status:** v0.5.0 Monolith. Implementation is in `main.go` at the repo root.

## Why SkyStacker?

- **Lightweight & pure Go**: Single static binary, no heavy dependencies.
- **Memory & Time Conscious**: Estimates memory and runtime *before* starting, and warns if RAM is insufficient.
- **Precision-focused**: Float32 accumulation (Sum, Mean, Median, Sigma-Clipping).
- **Max/Min Stacks**: Automatically generates a "brightest pixel" stack (to reveal satellite trails) and a "darkest pixel" stack (to isolate sky glow gradients).
- **Advanced Processing**: Luminance-based Generalized Hyperbolic Stretch (GHS) preserves star color while lifting faint dust.
- **Transparent**: Clear terminal output showing per-frame registration and a complete `report.txt`.

## Quick Start

### Prerequisites
- Go 1.20 or later
- ImageMagick (for RAW/FITS decoding: `sudo apt install imagemagick`)
- A directory of compatible frames (JPG/JPEG, CR2, CR3, FITS)

### Build
```bash
go build -o skystacker main.go
```

### Run
```bash
# Default: Sigma-Clipping stack + full post-processing
./skystacker ./night-session

# Just stack, skip post-processing
./skystacker ./night-session --stack-only

# Apply post-processing to an existing stack
./skystacker ./night-session --post-only
```

## Outputs

SkyStacker creates a `skystacker_results/` directory with the following structure:

**JPG Previews (Root Directory):**
- `stacked_max.jpg` — brightest pixels across all frames (shows satellite trails/trails)
- `stacked_min.jpg` — darkest pixels across all frames (shows pure sky gradient/vignette)
- `post_step1_background_removed.jpg`
- `post_step2_chromatic_aberration_reduced.jpg`
- `post_step2b_color_calibrated.jpg` (pre-stretch neutralization)
- `post_step3a_gentle_stretch.jpg` / `3b_moderate` / `3c_full`
- `post_step4_vignette_removed.jpg`
- `post_step6_sharpened.jpg` — final fully processed image
- `report.txt` — full processing report

**TIFF Scientific Outputs (`TIFF/` Subdirectory):**
- `stacked_sum.tiff` — raw float32 sum
- `stacked_mean.tiff` — float32 arithmetic mean of registered frames
- `stacked_no_correction_mean.tiff` — mean without registration
- `stacked_max.tiff` / `stacked_min.tiff`
- `stacked_processed.tiff` — final post-processed float32 output

## 🚀 Usage & Command Line Interface (CLI)

### Basic Syntax
```bash
skystacker [flags] [input-directory]
```
*(The `input-directory` defaults to the current directory `.`)*

### Available CLI Flags

| Flag | Description | Default |
| --- | --- | --- |
| `--output string` | Path to the output directory | `<input>/skystacker_results` |
| `--method string` | Stacking method: `mean`, `median`, `sigma` | `sigma` |
| `--workers int` | Maximum concurrent workers | `NumCPU` |
| `--recursive` | Search subdirectories for input files | `false` |
| `--overwrite` | Permit replacing files in the output directory | `false` |
| `--no-register` | Disable star-based registration | `false` |
| `--stack-only` | Run stacking only, skip post-processing | `false` |
| `--post-only` | Run post-processing only using existing TIFFs | `false` |
| `--dark string` | Path to dark frame for calibration | |
| `--flat string` | Path to flat frame for calibration | |
| `--bias string` | Path to bias frame for calibration | |
| `--mask-bg` | Mask bright objects during background extraction | `false` |
| `--bg-degree int` | Background polynomial degree (1-3) | `2` |
| `--sharpen-amount float`| Unsharp mask amount | `0.8` |
| `--sharpen-radius float`| Unsharp mask radius in pixels | `1.5` |
| `--no-color-calib` | Skip color calibration step | `false` |
| `--no-sharpen` | Skip sharpening step | `false` |
| `--selftest` | Run internal self-tests and exit | `false` |
| `--version` | Print version and exit | `false` |

## Features

### Implemented
- **Input Formats**: Native JPG/JPEG. CR2, CR3, FITS (via ImageMagick).
- **Calibration**: Dark, Flat, and Bias frame subtraction.
- **Stacking Methods**:
  - `mean`: Standard average.
  - `median`: Robust against satellite trails.
  - `sigma`: Sigma-clipping (default) for rejecting cosmic rays and hot pixels.
- **Star Registration**: Triangle-invariant star matching with RANSAC similarity estimation.
- **Post-Processing Pipeline**:
  1. Linear background gradient removal (optional `--mask-bg`).
  2. Chromatic aberration reduction (violet/magenta halo suppression).
  3. Color calibration (pre-stretch mid-tone neutralization).
  4. Progressive Luminance GHS stretching (Gentle, Moderate, Full) — preserves star color.
  5. Post-stretch vignette flattening.
  6. Unsharp masking (sharpening).

### 🎯 Planned & Next Improvements
- [ ] Autodetection of input data routing (distinct pipelines for RAW vs JPG).
- [ ] Weighted stacking based on star count or FWHM.
- [ ] Deconvolution.

## ⚙️ Architecture & Processing Pipeline

```text
[ Image Set (JPG/RAW/FITS) ]
           │
           ▼
  ┌──────────────────┐
  │   Calibration    │ ──> Dark/Flat/Bias subtraction
  └──────────────────┘
           │
           ▼
  ┌──────────────────┐
  │   Registration   │ ──> Triangle matching + RANSAC transform
  └──────────────────┘
           │
           ▼
  ┌──────────────────┐
  │   Accumulation   │ ──> Sum/Mean/Median/Sigma + Max/Min stacks
  └──────────────────┘
           │
           ▼ (Outputs high-dynamic-range TIFF)
  ┌────────────────────────────────────────────────────────┐
  │              Post-Processing Pipeline                  │
  │  ┌──────────────────────────────────────────────────┐  │
  │  │ 1. Background Gradient Removal                   │  │
  │  │ 2. Chromatic Aberration Reduction                │  │
  │  │ 3. Color Calibration (Pre-Stretch)               │  │
  │  │ 4. Luminance GHS Stretching (3 Stages)           │  │
  │  │ 5. Vignette Flattening                           │  │
  │  │ 6. Unsharp Masking                               │  │
  │  └──────────────────────────────────────────────────┘  │
  └────────────────────────────────────────────────────────┘
```

## License
MIT License. See [LICENSE](LICENSE).

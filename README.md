# SkyStacker

**Fast, lightweight, local-first Go CLI for astrophotography image stacking.**

> Drop a folder of JPG exposures. Get registered, high-dynamic-range TIFF sum and mean images, plus a fully post-processed preview and a detailed report.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-active%20development-orange)]()

SkyStacker is an open-source command-line tool written in Go that discovers, aligns, and stacks astronomical images (currently JPG/JPEG) into integrated outputs. It emphasizes numerical precision, bounded memory use, reproducibility, and a minimal user experience.

**Current status:** v0.4.x Monolith. Implementation is in `main.go` at the repo root. It includes robust star registration and an advanced post-processing pipeline.

## Why SkyStacker?

- **Lightweight & pure Go**: Single static binary, no heavy dependencies, easy cross-compilation.
- **Local-first & offline**: No accounts, cloud, or network required.
- **Precision-focused**: Floating-point (float32) accumulation to avoid 8-bit overflow; explicit TIFF float32 outputs.
- **Memory-conscious**: Streaming/tile-friendly design; does not load all frames at once.
- **Advanced Processing**: Includes triangle-based star registration, background gradient removal, chromatic aberration reduction, and multi-stage stretching.
- **Transparent**: Clear reports and explicit notes about what was (and was not) done.

## 🌟 Visual Results & Processing Showcase

SkyStacker automates frame discovery, alignment, stacking, and an advanced post-processing pipeline to transform raw astrophotography data into clean, high-dynamic-range images.

| Raw Stacked Image (Before Gradient Removal) | Final Post-Processed Output (After Pipeline) |
| :---: | :---: |
| ![Stacked Mean with Gradient](assets/stacked_mean.jpg) | ![Processed Background Removed](assets/post_step1_background_removed.jpg) |
| *Heavy background light pollution, magenta cast, and washed-out contrast.* | *Cleaned background, neutralized sky, and enhanced stellar details.* |

## Quick Start

### Prerequisites
- Go 1.20 or later
- A directory of compatible JPG/JPEG frames (same dimensions)

### Build
```bash
go build -o skystacker main.go
```

### Run
```bash
# Process current directory
./skystacker

# Process a specific session with custom output
./skystacker ./night-session --output ./results

# Skip post-processing and just stack
./skystacker ./night-session --stack-only

# Apply post-processing to an existing stack without re-stacking
./skystacker ./night-session --post-only
```

## Outputs

SkyStacker creates a `skystacker_results/` directory (or your chosen output directory) with the following structure:

**JPG Previews (Root Directory):**
- `stacked_sum.jpg` / `stacked_mean.jpg` — normalized display previews
- `stacked_no_correction_mean.jpg` — unregistered mean preview
- `post_step1_background_removed.jpg`
- `post_step2_chromatic_aberration_reduced.jpg`
- `post_step3a_gentle_stretch.jpg` / `3b_moderate` / `3c_full`
- `post_step4_vignette_removed.jpg`
- `post_step5_color_calibrated.jpg`
- `post_step6_sharpened.jpg` — final fully processed image
- `report.txt` — full processing report

**TIFF Scientific Outputs (`TIFF/` Subdirectory):**
- `stacked_sum.tiff` — raw float32 sum
- `stacked_mean.tiff` — float32 arithmetic mean of registered frames
- `stacked_no_correction_mean.tiff` — mean without registration
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
| `--workers int` | Maximum concurrent workers | `NumCPU` |
| `--recursive` | Search subdirectories for input files | `false` |
| `--overwrite` | Permit replacing files in the output directory | `false` |
| `--no-register` | Disable star-based registration (v0.2 behavior) | `false` |
| `--stack-only` | Run stacking only, skip post-processing | `false` |
| `--post-only` | Run post-processing only using existing TIFFs | `false` |
| `--bg-degree int` | Background polynomial degree (1-3) | `2` |
| `--stretch float` | Asinh stretch factor | `10.0` |
| `--sharpen-amount float` | Unsharp mask amount | `0.8` |
| `--sharpen-radius float` | Unsharp mask radius in pixels | `1.5` |
| `--no-color-calib` | Skip color calibration step | `false` |
| `--no-sharpen` | Skip sharpening step | `false` |
| `--selftest` | Run internal self-tests and exit | `false` |
| `--version` | Print version and exit | `false` |

## Features

### Implemented
- Automatic discovery of `.jpg` / `.jpeg`
- Input validation and graceful rejection of corrupt/incompatible files
- **Star Registration**: Triangle-invariant star matching with RANSAC similarity estimation (rotation + scale + translation).
- **Float32 Accumulation**: High-precision float32 TIFF outputs.
- **Advanced Post-Processing Pipeline**:
  - Linear background gradient removal (polynomial fitting).
  - Chromatic aberration reduction (violet/magenta halo suppression).
  - 3-stage progressive contrast stretching (Gentle, Moderate, Full).
  - Post-stretch vignette flattening (re-applies background removal to fix localized gradients).
  - Color calibration & background neutralization.
  - Unsharp masking (sharpening).
- Concurrent processing with worker limit.
- Atomic writes, overwrite control, recursive option.
- 37 internal self-tests covering math, TIFF encoding, registration, and pipelines.

### 🎯 Planned & Next Improvements
- **[NEXT] Autodetection of Input Data**: Manage data automatically depending on whether input files are JPG images or **Canon RAW (CR2/CR3)** files, routing to the correct decoder pipeline.
- Median, sigma-clipping, and weighted stacking algorithms to reject satellite trails and cosmic rays.
- FITS support and dark/flat/bias calibration frames.
- Generalized Hyperbolic Stretch (GHS) and object-masked background extraction.

## ⚙️ Architecture & Processing Pipeline

SkyStacker is designed as a high-performance, memory-conscious processing pipeline.

```text
[ JPG/JPEG Image Set ]
           │
           ▼
  ┌──────────────────┐
  │  Frame Discovery │ ──> Scans directory, validates headers
  └──────────────────┘
           │
           ▼
  ┌──────────────────┐
  │   Registration   │ ──> Detects stars, triangle matching, RANSAC similarity transform
  └──────────────────┘
           │
           ▼
  ┌──────────────────┐
  │   Accumulation   │ ──> Merges frames via Sum/Mean with bounding box cropping
  └──────────────────┘
           │
           ▼ (Outputs high-dynamic-range TIFF)
  ┌────────────────────────────────────────────────────────┐
  │              Post-Processing Pipeline                  │
  │  ┌──────────────────────────────────────────────────┐  │
  │  │ 1. Linear Background Gradient Removal            │  │
  │  │ 2. Chromatic Aberration Reduction                │  │
  │  │ 3. Progressive Histogram Stretching (3 Stages)   │  │
  │  │ 4. Post-Stretch Vignette Flattening              │  │
  │  │ 5. Color Calibration & Neutralization            │  │
  │  │ 6. Unsharp Masking (Sharpening)                  │  │
  │  └──────────────────────────────────────────────────┘  │
  └────────────────────────────────────────────────────────┘
           │
           ├───────────────────────────────┬───────────────────────────────┐
           ▼                               ▼                               ▼
 [ Intermediate JPEGs ]          [ Intermediate JPEGs ]          [ Final Processed TIFF & JPEG ]
```

## Comparative Feature Table

| Feature                        | SkyStacker (v0.4)          | DeepSkyStacker          | Siril                          | Sequator                  |
|--------------------------------|----------------------------|-------------------------|--------------------------------|---------------------------|
| **License**                    | MIT                        | BSD-3                   | GPL                            | Freeware                  |
| **Interface**                  | CLI only                   | GUI                     | GUI + CLI/scripts              | GUI                       |
| **Registration / Alignment**   | Yes (Similarity)           | Yes                     | Yes                            | Yes (specialized)         |
| **Calibration (dark/flat)**    | No (planned)               | Yes                     | Yes                            | Limited                   |
| **Stacking methods**           | Sum, Mean                  | Multiple                | Multiple + advanced            | Specialized nightscape    |
| **Input formats**              | JPG/JPEG                   | RAW, FITS, TIFF, etc.   | RAW, FITS, TIFF, SER, etc.     | JPG, RAW                  |
| **Post-processing**            | Built-in (6 steps)         | Limited                 | Advanced (GHS, Deconvolution)  | Built-in                  |
| **Single binary / pure Go**    | Yes                        | No                      | No                             | No                        |

## Building and Development

```bash
# Run the robust self-test suite
./skystacker --selftest

# Build the binary
go build -o skystacker main.go
```

Module path: `github.com/computerphysicslab/skystacker`

## License

MIT License. See [LICENSE](LICENSE).

## Contributing

Contributions welcome! Priority areas:
- Canon RAW autodetection and decoding.
- Advanced stacking algorithms (Median, Sigma Clipping).
- Benchmarks and performance.
- Documentation and test datasets.

Please open an issue or PR. 


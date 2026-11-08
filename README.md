# SkyStacker

**Fast, lightweight, local-first Go CLI for astrophotography image stacking.**

> Drop a folder of (pre-aligned) JPG exposures. Get high-dynamic-range TIFF sum and mean images, plus previews and a detailed report.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)](https://go.dev/)
[![Status](https://img.shields.io/badge/status-early%20MVP-orange)]()

SkyStacker is an open-source command-line tool written in Go that discovers and stacks astronomical images (currently JPG/JPEG) into integrated outputs. It emphasizes numerical precision, bounded memory use, reproducibility, and a minimal user experience. Version 0.1 performs pixel-wise sum and arithmetic mean in the existing coordinate system (no automatic registration or calibration yet).

**Current status:** Early MVP implementation available under `skystacker/v0/`. Suitable for testing with pre-aligned frames. Not a full replacement for mature suites.

## Why SkyStacker?

- **Lightweight & pure Go**: Single static binary, no heavy dependencies, easy cross-compilation.
- **Local-first & offline**: No accounts, cloud, or network required.
- **Precision-focused**: Floating-point (float32) accumulation to avoid 8-bit overflow; explicit TIFF float32 outputs.
- **Memory-conscious**: Streaming/tile-friendly design; does not load all frames at once.
- **Transparent**: Clear reports and explicit notes about what was (and was not) done.
- **Extensible foundation**: Modular architecture prepared for future registration, calibration, and advanced stacking.

Ideal for simple stacking of already-aligned sessions, batch automation, learning, or as a building block.

## 🌟 Visual Results & Processing Showcase

SkyStacker automates frame discovery, alignment, stacking, and an advanced post-processing pipeline (background gradient removal and contrast stretch) to transform raw astrophotography data into clean, high-dynamic-range images.

| Raw Stacked Image (Before Gradient Removal) | Final Post-Processed Output (After Pipeline) |
| :---: | :---: |
| ![Stacked Mean with Gradient](assets/stacked_mean.jpg) | ![Processed Background Removed](assets/post_step1_background_removed.jpg) |
| *Heavy background light pollution, magenta cast, and washed-out contrast.* | *Cleaned background, neutralized sky, and enhanced stellar details.* |

## Quick Start

### Prerequisites
- Go 1.20 or later
- A directory of compatible JPG/JPEG frames (same dimensions recommended)

### Build
```bash
cd skystacker/v0
go build -o skystacker ./cmd/skystacker
```

### Run
```bash
# Process current directory
./skystacker

# Process a specific session
./skystacker ./night-session --workers 4 --output ./results
```

Outputs appear in `skystacker_results/` (or your chosen directory):
- `stacked_sum.tiff` / `stacked_sum.jpg` — raw sum (float32 TIFF + normalized preview)
- `stacked_mean.tiff` / `stacked_mean.jpg` — arithmetic mean
- `stacked_no_correction_mean.*` — identical to mean in v0.1 (placeholder for future registered comparison)
- `stacked_preview.jpg` — display preview
- `report.txt` — full processing report

**Important:** v0.1 does **not** align or calibrate frames. Use pre-registered images or expect blur if frames are misaligned.

## Features

### Implemented (v0.1)
- Automatic discovery of `.jpg` / `.jpeg` (case-insensitive, non-recursive by default)
- Input validation and graceful rejection of corrupt/incompatible files
- Float32 interleaved RGB accumulation (sum + mean)
- Concurrent processing with worker limit
- High-precision float32 TIFF outputs (uncompressed, little-endian)
- JPG display previews (with gamma stretch)
- Detailed `report.txt`
- Atomic writes, overwrite control, recursive option

### Planned
- Star detection and geometric registration (v0.2)
- Median, sigma-clipping, weighted stacking (v0.3)
- RAW / FITS support, dark/flat/bias calibration, background correction (v0.4+)
- More advanced quality metrics and rejection

## Comparative Feature Table

| Feature                        | SkyStacker (v0.1)          | DeepSkyStacker          | Siril                          | OpenSkyStacker             | Sequator                  |
|--------------------------------|----------------------------|-------------------------|--------------------------------|----------------------------|---------------------------|
| **License**                    | MIT                        | BSD-3                   | GPL                            | MIT                        | Freeware                  |
| **Platform**                   | Linux, Windows, macOS     | Windows (Mac experimental) | Win/Mac/Linux                 | Win/Mac/Linux             | Windows                   |
| **Interface**                  | CLI only                   | GUI                     | GUI + CLI/scripts              | GUI + CLI                 | GUI                       |
| **Registration / Alignment**   | No (planned)               | Yes                     | Yes                            | Yes                       | Yes (specialized)         |
| **Calibration (dark/flat/bias)** | No (planned)             | Yes                     | Yes                            | Partial                   | Limited                   |
| **Stacking methods**           | Sum, arithmetic mean       | Multiple (incl. median, sigma) | Multiple + advanced          | Mean/median etc.          | Specialized nightscape    |
| **Input formats**              | JPG/JPEG                   | RAW, FITS, TIFF, etc.   | RAW, FITS, TIFF, SER, etc.     | Various                   | JPG, RAW                  |
| **Output**                     | Float32 TIFF + JPG preview | 16/32-bit TIFF         | FITS, TIFF, etc.               | 32-bit TIFF               | JPG/TIFF                  |
| **Memory efficiency**          | High (streaming design)    | Good                    | Good                           | Good                      | Good                      |
| **Single binary / pure Go**    | Yes                        | No                      | No                             | No                        | No                        |
| **Best for**                   | Lightweight pre-aligned stacking, automation, foundation | Beginner deep-sky     | Full free workflow             | Cross-platform deep-sky   | Milky Way / nightscapes   |

SkyStacker deliberately starts simpler and more focused. It is not trying to replace Siril or DeepSkyStacker today; it aims to provide a clean, modern, portable core that can grow.

## Related Projects

Learn from and coexist with established tools:

- **[Siril](https://siril.org/)** ([GitHub](https://github.com/Astro-Siril/Siril)) — Mature open-source full pipeline (registration, calibration, stacking, post-processing, scripting). Cross-platform.
- **[DeepSkyStacker](https://github.com/deepskystacker/DSS)** — Classic, beginner-friendly deep-sky stacker (primarily Windows; open source since 2018).
- **[OpenSkyStacker](https://github.com/BenJuan26/OpenSkyStacker)** — Multi-platform deep-sky stacker with GUI and CLI.
- **[Sequator](https://sites.google.com/view/sequator/)** — Specialized for nightscape / Milky Way stacking (Windows).
- **[ASTAP](https://www.hnsky.org/astap.htm)** — Plate solving, stacking, and analysis tools.

## Building and Development

```bash
cd skystacker/v0
go test ./...
go build -o skystacker ./cmd/skystacker
```

Module path: `github.com/skystacker/skystacker` (adjust if needed for your fork).

## Design Principles

1. Correctness before speed.
2. Explicit behaviour (never claim alignment or calibration that did not happen).
3. Local-first.
4. Reproducibility.
5. Modularity.
6. Measured optimisation.

See `TECHNICAL_SPEC.md` for the full design document.

## License

MIT License. See [LICENSE](LICENSE).

## Contributing

Contributions welcome! Priority areas:

- Improve registration and alignment
- Add more input formats and calibration
- Benchmarks and performance
- Documentation and test datasets
- Cross-platform CI

Please open an issue or PR. Review the technical specification first.

---

**Working tagline:** Fast. Local. Astronomical.  
**Status:** Early MVP — usable for basic stacking of compatible frames.

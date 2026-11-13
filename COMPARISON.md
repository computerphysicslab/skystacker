# SkyStacker — feature comparison

High-level orientation, not a benchmark. Feature sets change between
releases. Legend: `Yes` = supported; `–` = not supported; `Partial` =
limited or workflow-dependent.

## Overview

| Tool | License | Platforms | Interface |
|---|---|---|---|
| **SkyStacker** | MIT | Win / Mac / Linux | CLI only |
| **OriginStack** | GPLv3 | Win / Mac / Linux | GUI + CLI |
| **DeepSkyStacker** | Free | Win, Mac (6.1.3+), Linux (Wine) | GUI + CLI |
| **Siril** | GPLv3 | Win / Mac / Linux | GUI + CLI |
| **Astro Pixel Processor** | Paid | Win / Mac / Linux | GUI |
| **PixInsight** | Paid | Win / Mac / Linux | GUI + CLI |
| **Sequator** | Freeware | Win (Mac via CrossOver) | GUI |
| **ASTAP** | MPL 2.0 | Win / Linux | GUI + CLI |
| **IRIS** | Free | Win (Linux via Wine) | GUI + CLI |
| **Nebulosity** | Paid | Win / Mac | GUI |
| **MaxIm DL** | Paid | Windows | GUI + COM |
| **AstroImageJ** | GPL | Java (all OS) | GUI + macros |
| **Starry Landscape Stacker** | Paid | macOS | GUI |
| **AutoStakkert!** | Free | Windows | GUI + CLI |

## Calibration and registration

| Tool | Dark / Flat / Bias | Bad-pixel map | Distortion corr. | Plate solving |
|---|---|---|---|---|
| **SkyStacker** | Yes | – | – | – |
| **OriginStack** | Yes | – | – | Yes |
| **DeepSkyStacker** | Yes | – | – | – |
| **Siril** | Yes | Yes | Yes | Partial |
| **Astro Pixel Processor** | Yes | Yes | Yes | – |
| **PixInsight** | Yes | Yes | Yes | Yes |
| **Sequator** | Dark, Flat | – | Yes | – |
| **ASTAP** | Yes | – | – | Yes |
| **IRIS** | Yes | – | – | – |
| **Nebulosity** | Yes | Yes | – | – |
| **MaxIm DL** | Yes | Yes | – | – |
| **AstroImageJ** | Partial | – | – | – |
| **Starry Landscape Stacker** | Dark, Flat | – | Yes | – |
| **AutoStakkert!** | – | – | – | – |

## Stacking methods

| Tool | Mean / Med / Sigma | Weighted | Drizzle | Comet |
|---|---|---|---|---|
| **SkyStacker** | Yes | Yes | – | – |
| **OriginStack** | Yes | Yes | Yes | Yes |
| **DeepSkyStacker** | Yes | Yes | x2 / x3 | Yes |
| **Siril** | Yes | Yes | HST | Yes |
| **Astro Pixel Processor** | Yes | Yes | Bayer | Partial |
| **PixInsight** | Yes | Yes | Yes | Yes |
| **Sequator** | Yes | – | – | – |
| **ASTAP** | Yes | – | – | – |
| **IRIS** | Yes | – | – | – |
| **Nebulosity** | Yes | – | – | – |
| **MaxIm DL** | Yes | – | – | – |
| **AstroImageJ** | Yes | – | – | – |
| **Starry Landscape Stacker** | Yes | – | – | – |
| **AutoStakkert!** | Lucky-imaging | – | – | – |

## Post-processing and automation

| Tool | Built-in post | Scripting / CLI | Output formats | GPU |
|---|---|---|---|---|
| **SkyStacker** | Extensive | CLI only | TIFF / JPEG / raw | – |
| **OriginStack** | Extensive | CLI (Python) | FITS / JPG | Yes (CUDA) |
| **DeepSkyStacker** | Minimal | CLI | TIFF / FITS | – |
| **Siril** | Extensive | Scripts + CLI | FITS / TIFF / JPEG / SER | – |
| **Astro Pixel Processor** | Moderate | Limited | FITS / TIFF / JPEG | – |
| **PixInsight** | Exhaustive | PJSR / Python / CLI | XISF / FITS / TIFF / JPEG | Partial |
| **Sequator** | Moderate | – | TIFF / JPEG | – |
| **ASTAP** | Moderate | CLI | FITS / TIFF / JPEG | – |
| **IRIS** | Moderate | Editor + CLI | FITS / PIC / BMP | – |
| **Nebulosity** | Moderate | – | FITS / TIFF | – |
| **MaxIm DL** | Moderate | COM | FITS / TIFF / JPEG | – |
| **AstroImageJ** | Photometry | Macros | FITS / TIFF / PNG | – |
| **Starry Landscape Stacker** | Minimal | – | TIFF / JPEG | – |
| **AutoStakkert!** | Minimal | CLI | TIFF / PNG | – |

## Where SkyStacker sits

**Strengths**

- Only single cross-platform binary in this list — no GUI, no external runtime.
- Fully reproducible CLI workflow; every option is a flag.
- Float32 end-to-end; per-pixel rejection and weight maps.
- Weighted sigma combining registration and intrinsic frame quality.
- Global + local per-tile normalization built in.
- Unusually complete post-processing for a stacker: background removal,
  star removal, STF auto-stretch, guided denoise (incl. chroma), curves,
  HDR blend, four weighted mixes, diffraction spikes.
- Transparent `report.txt` with per-frame diagnostics.

**Gaps vs mature tools**

- No GUI.
- No mosaic, no drizzle, no plate solving / WCS / SIP — Gaia DR3 data-access scaffolding (HEALPix + TAP client + on-disk tile cache) is in place; pipeline consumer pending.
- No deconvolution or PSF modelling.
- No dedicated comet / moving-object stacking.
- No scripting beyond flags.
- No GPU acceleration.
- No XISF or SER input.
- No multi-session / multi-filter automation.
- Planetary / lunar / solar lucky imaging out of scope.

## Sources

- DeepSkyStacker — <https://github.com/deepskystacker/DSS/releases>, <http://deepskystacker.free.fr>
- Siril — <https://siril.org>, <https://siril.pixls.us>
- Astro Pixel Processor — <https://www.astropixelprocessor.com>
- PixInsight — <https://pixinsight.com>
- ASTAP — <https://www.hnsky.org/astap.htm>
- IRIS — <http://www.astrosurf.com/buil/us/iris/iris.htm>
- Nebulosity — <https://www.stark-labs.com/nebulosity.html>
- MaxIm DL — <https://diffractionlimited.com/product/maxim-dl/>
- AstroImageJ — <https://www.astro.louisville.edu/software/astroimagej/>
- Starry Landscape Stacker — <https://apps.apple.com/app/starry-landscape-stacker/id1446056095>
- AutoStakkert! — <https://www.autostakkert.com>

Feature summaries reflect public documentation as of late 2026. If a cell
disagrees with your installed version, treat the vendor docs as authoritative.

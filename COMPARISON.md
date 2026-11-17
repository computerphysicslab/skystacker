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
| **Fitswork** | Freeware | Windows | GUI |

## Calibration and registration

| Tool | Dark / Flat / Bias | Bad-pixel map | Distortion corr. | Plate solving |
|---|---|---|---|---|
| **SkyStacker** | Yes | Yes (`--bpm`, `--bpm-auto`) | Partial (SIP in WCS helpers and `--resample-sip`) | Partial (wired, seeded; also in `--post-only`) |
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
| **Fitswork** | Yes | – | Partial (manual) | – |

## Stacking methods

| Tool | Mean / Med / Sigma | Weighted | Drizzle | Comet |
|---|---|---|---|---|
| **SkyStacker** | Yes | Yes | Partial (diagnostic `--drizzle-upsample`) | Yes (pixel rate, RA/Dec rate, ephemeris) |
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
| **Fitswork** | Yes | Partial | – | Partial (2-star) |

## Post-processing and automation

| Tool | Built-in post | Scripting / CLI | Output formats | GPU |
|---|---|---|---|---|
| **SkyStacker** | Extensive | CLI + JSON config (`--config`) | TIFF / JPEG / PNG / BMP / FITS / XISF / raw; native pure-Go CR2 input (CR3 via dcraw) | – |
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
| **Fitswork** | Extensive | – | FITS / TIFF / JPEG / BMP / PNG | – |

## Post-processing software comparison

This table focuses on what happens after the frames are stacked. It is
included because Fitswork is a long-standing reference for interactive
astronomical post-processing, and because its reference-frame crop
(two manually picked stars) is the direct precedent for SkyStacker's
automatic `--reference-crop`.

**A. Interface, calibration and registration**

| Tool | License | Platforms | Interface | Calibration | Star alignment | Crop to reference frame |
|---|---|---|---|---|---|---|
| **SkyStacker** | MIT | Win / Mac / Linux | CLI | Yes | Automatic star-pattern | **Yes, automatic (`--reference-crop`)** |
| **Fitswork** | Freeware | Windows | GUI | Yes | Manual 2-star + pattern | **Yes (manual 2-star)** |
| **PixInsight** | Paid | Win / Mac / Linux | GUI + CLI | Yes | Yes | Partial (StarAlignment / DynamicCrop) |
| **Siril** | GPLv3 | Win / Mac / Linux | GUI + CLI | Yes | Yes | Partial (registration + crop) |
| **IRIS** | Free | Windows | GUI + CLI | Yes | Yes | Yes (manual) |
| **Nebulosity** | Paid | Win / Mac | GUI | Yes | Yes | Partial |
| **MaxIm DL** | Paid | Windows | GUI + COM | Yes | Yes | Yes |
| **ASTAP** | MPL 2.0 | Win / Linux | GUI + CLI | Yes | Yes | Partial |
| **AstroImageJ** | GPL | Java (all OS) | GUI + macros | Yes | Yes | Partial |
| **AutoStakkert!** | Free | Windows | GUI + CLI | – | Yes (lucky imaging) | Partial |

**B. Post-processing and automation**

| Tool | Deconvolution | Starless layers | Multiscale background | Mosaic | Scripting |
|---|---|---|---|---|---|
| **SkyStacker** | Yes (`--deconvolve` RL, `--wavesharp` classical) | Yes (basic, `--starless`) | Yes (`--bg-multiscale`, partial) | – | CLI flags + `--config` |
| **Fitswork** | Yes (Richardson–Lucy, Wiener) | – | – | Yes | – |
| **PixInsight** | Yes | Yes | Yes | Yes | PJSR / Python |
| **Siril** | Yes | Yes | Partial | Partial | Scripts + CLI |
| **IRIS** | Yes (RL) | Partial | – | Yes | Editor + CLI |
| **Nebulosity** | Partial | – | – | Partial | – |
| **MaxIm DL** | Yes | Yes | Partial | Yes | COM |
| **ASTAP** | – | – | – | Yes | CLI |
| **AstroImageJ** | – | – | – | – | Macros |
| **AutoStakkert!** | – | – | – | – | CLI |

The key difference: Fitswork requires the user to mark the same two stars
in both images; SkyStacker detects the star field in both images, finds the
similarity transform by triangle matching, and warps every light into the
reference frame in memory. Source files are never modified.

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
- Scientific/metadata outputs: TIFF, FITS (`BITPIX=-32`, TAN WCS), XISF 1.0, `report.json`, structured bad-pixel maps, catalog annotation (`--annotate`) and anomaly manifests.
- Post-processing extras: Richardson–Lucy deconvolution (`--deconvolve`), multi-scale detail enhancement (`--wavesharp`), narrowband HOO/SHO mapping, star/starless layers, and config/checkpoint-style control (`--config`, `--resume`).
- Automatic reference-frame crop (`--reference-crop`): every light is
  star-matched to an external reference image and warped into its pixel
  frame in memory. Unlike Fitswork's manual two-star workflow, no points
  are picked by hand, and single-image runs produce the cropped frame
  directly.

**Gaps vs mature tools**

- No GUI.
- No mosaic. True multi-frame drizzle resampling is not wired; `--drizzle-upsample` is a diagnostic upsampler of the final stack.
- TAN WCS plate solving is wired (`--plate-solve`, seeded via `--plate-center` or the reference frame's FITS `RA`/`DEC`, also usable in `--post-only`; output in `plate_solution.json`). SIP order-2 is applied in WCS helpers and, with `--resample-sip`, in the pixel-space stacking resampler using the stored `plate_solution.json`; it remains `Partial` because there is no per-frame astrometric reprojection or drizzle.
- Deconvolution is classical only (`--deconvolve` Richardson–Lucy, `--wavesharp`); no neural BXT/NXT/SXT-equivalent inference under the stdlib-only constraint (see `research/05-research-deep-dive-2026-11-16.md` §13). PSF modelling is limited to star HFR/eccentricity.
- Comet stacking supports pixel rate (`--comet-center`/`--comet-rate`), RA/Dec rate (`--comet-radec`) and ephemeris files (`--comet-target`); no combined comet+stars frame.
- No scripting language beyond flags plus JSON config files (`--config`).
- No GPU acceleration.
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
- Fitswork — <https://www.fitswork.de/software/softw_en.shtml>

Feature summaries reflect public documentation as of late 2026. If a cell
disagrees with your installed version, treat the vendor docs as authoritative.

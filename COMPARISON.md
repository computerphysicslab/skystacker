# SkyStacker — feature comparison

High-level orientation, not a benchmark. Feature sets change between
releases. Legend: `Yes` = supported; `–` = not supported; `Partial` =
limited or workflow-dependent.

## Overview

| Tool | License | Platforms | Interface |
|---|---|---|---|
| **SkyStacker** | Open Source (MIT) | Win / Mac / Linux | CLI only |
| **OriginStack** | Open Source (GPLv3) | Win / Mac / Linux | GUI + CLI |
| **DeepSkyStacker** | Open Source (BSD-3-Clause) | Win, macOS (6.1.3+), Linux x64 native (Qt 6) | GUI + CLI |
| **Siril** | Open Source (GPLv3) | Win / Mac / Linux | GUI + CLI |
| **Astro Pixel Processor** | Paid | Win / Mac / Linux | GUI |
| **PixInsight** | Paid | Win / Mac / Linux | GUI + CLI |
| **Sequator** | Freeware (closed) | Win (Mac via CrossOver) | GUI |
| **ASTAP** | Open Source (MPL 2.0) | Win / Linux / macOS (Intel + Apple Silicon) / Linux ARM | GUI + CLI |
| **IRIS** | Freeware (closed) | Windows | GUI + CLI |
| **Nebulosity** | Paid | Win / Mac | GUI |
| **MaxIm DL** | Paid | Windows | GUI + COM |
| **AstroImageJ** | Open Source (GPL) | Java (all OS) | GUI + macros |
| **Starry Landscape Stacker** | Paid | macOS | GUI |
| **AutoStakkert!** | Freeware (closed) | Windows | GUI + CLI |
| **Fitswork** | Freeware (closed) | Windows | GUI |

## Calibration and registration

| Tool | Dark / Flat / Bias | Bad-pixel map | Distortion corr. | Plate solving |
|---|---|---|---|---|
| **SkyStacker** | Yes | Yes (`--bpm`, `--bpm-auto`) | Partial (SIP in WCS helpers and `--resample-sip`) | Partial (seeded Gaia + blind via astrometry.net/ASTAP/native; also in `--post-only`) |
| **OriginStack** | Yes | – | – | Yes |
| **DeepSkyStacker** | Yes | – | – | – |
| **Siril** | Yes | Yes | Yes | Yes |
| **Astro Pixel Processor** | Yes | Yes | Yes | – |
| **PixInsight** | Yes | Yes | Yes | Yes |
| **Sequator** | Dark, Flat | – | Yes | – |
| **ASTAP** | Yes | Yes | Yes (order-3 SIP) | Yes |
| **IRIS** | Yes | – | – | – |
| **Nebulosity** | Yes | Yes | – | – |
| **MaxIm DL** | Yes | Yes | – | – |
| **AstroImageJ** | Partial | – | – | – |
| **Starry Landscape Stacker** | Dark, Flat | – | Yes | – |
| **AutoStakkert!** | Dark, Flat | – | – | – |
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
| **SkyStacker** | Extensive | CLI + JSON config (`--config`), batch queue (`--batch`) | TIFF / BigTIFF / JPEG / PNG / BMP / FITS / XISF / raw; native pure-Go CR2 input (CR3 via dcraw) | – |
| **OriginStack** | Extensive | CLI (Python) | FITS / JPG | Yes (CUDA) |
| **DeepSkyStacker** | Minimal | CLI | TIFF / FITS | – |
| **Siril** | Extensive | Scripts + CLI | FITS / TIFF / JPEG / SER / XISF / PNG / BMP | – |
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
| **SkyStacker** | Open Source (MIT) | Win / Mac / Linux | CLI | Yes | Automatic star-pattern | **Yes, automatic (`--reference-crop`)** |
| **Fitswork** | Freeware (closed) | Windows | GUI | Yes | Manual 2-star + pattern | **Yes (manual 2-star)** |
| **PixInsight** | Paid | Win / Mac / Linux | GUI + CLI | Yes | Yes | Yes (StarAlignment / DynamicCrop) |
| **Siril** | Open Source (GPLv3) | Win / Mac / Linux | GUI + CLI | Yes | Yes | Yes (registration + crop) |
| **IRIS** | Freeware (closed) | Windows | GUI + CLI | Yes | Yes | Yes (manual) |
| **Nebulosity** | Paid | Win / Mac | GUI | Yes | Yes | Partial |
| **MaxIm DL** | Paid | Windows | GUI + COM | Yes | Yes | Yes |
| **ASTAP** | MPL 2.0 | Win / Linux | GUI + CLI | Yes | Yes | Partial |
| **AstroImageJ** | Open Source (GPL) | Java (all OS) | GUI + macros | Yes | Yes | Partial |
| **AutoStakkert!** | Freeware (closed) | Windows | GUI + CLI | – | Yes (lucky imaging) | Partial |

**B. Post-processing and automation**

| Tool | Deconvolution | Starless layers | Multiscale background | Mosaic | Scripting |
|---|---|---|---|---|---|
| **SkyStacker** | Yes (`--deconvolve` RL, `--deconvolve-wiener`, `--wavesharp` classical, `--deconvolve-grid`) | Yes (basic, `--starless`) | Yes (`--bg-multiscale`, partial) | – | CLI flags + `--config` + `--batch` |
| **Fitswork** | Yes (Richardson–Lucy, Wiener) | – | – | Yes | – |
| **PixInsight** | Yes | Yes | Yes | Yes | PJSR / Python |
| **Siril** | Yes | Yes | Partial | Yes | Scripts + CLI |
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
- Scientific/metadata outputs: TIFF/BigTIFF (via the unified `goTiff` engine, Astro-TIFF WCS/CFA propagated), FITS (`BITPIX=-32`, TAN WCS), XISF 1.0, `report.json`, VOTable `BINARY2`, structured bad-pixel maps, catalog annotation (`--annotate`) and anomaly manifests, plus layered SVG 2 + anti-aliased raster annotation with provenance, grids, constellations, scale bar, finder chart, magnitude-encoded renderer-safe colours and multi-tile Gaia coverage (`--annotate --sky-map`).
- Post-processing extras: Richardson–Lucy and Wiener deconvolution (`--deconvolve`, `--deconvolve-wiener`, grid PSF variant), multi-scale detail enhancement (`--wavesharp`), narrowband HOO/SHO mapping, star/starless layers, and config/checkpoint-style control (`--config`, `--resume`).
- Automatic reference-frame crop (`--reference-crop`): every light is
  star-matched to an external reference image and warped into its pixel
  frame in memory. Unlike Fitswork's manual two-star workflow, no points
  are picked by hand, and single-image runs produce the cropped frame
  directly.

**Gaps vs mature tools**

- No GUI.
- No mosaic. True multi-frame drizzle resampling is not wired; `--drizzle-upsample` is a diagnostic upsampler of the final stack.
- TAN WCS plate solving is wired (`--plate-solve`, seeded via `--plate-center` or the reference frame's FITS `RA`/`DEC`, also usable in `--post-only`; output in `plate_solution.json`). SIP order-2 is applied in WCS helpers and, with `--resample-sip`, in the pixel-space stacking resampler using the stored `plate_solution.json`; it remains `Partial` because there is no per-frame astrometric reprojection or drizzle.
- Deconvolution is classical only (`--deconvolve` Richardson–Lucy, `--deconvolve-wiener`, `--deconvolve-grid`, `--wavesharp`); no neural BXT/NXT/SXT-equivalent inference under the stdlib-only constraint (see `research/03-reference-and-literature.md` §5). PSF auto-selection fits Gaussian/Moffat (`--psf-auto`) but there is no spatially varying PSF model.
- Comet stacking supports pixel rate (`--comet-center`/`--comet-rate`), RA/Dec rate (`--comet-radec`) and ephemeris files (`--comet-target`), plus dual-stack recombination into a comet+stars frame (`--comet-combine`).
- No scripting language beyond flags, JSON config (`--config`) and the shell-free JSON run queue (`--batch`).
- No GPU acceleration.
- No multi-session / multi-filter automation.
- Planetary / lunar / solar lucky imaging out of scope.

## Sources

- DeepSkyStacker — <https://github.com/deepskystacker/DSS/releases>, <http://deepskystacker.free.fr>
- OriginStack — <https://github.com/hd152/originstack>, <https://originstack.site>
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

## Candidate features promoted to the roadmap (2026-11-18)

Features that are both present in mature tools and feasible under the
stdlib-only, static, CLI-only constraints. Full acceptance criteria live in
`ROADMAP.md` items 19–28; this table records only the comparative rationale.

| Comparative feature | Seen in | Roadmap item |
|---|---|---|
| SubframeSelector-style metrics + frame culling | PixInsight, DeepSkyStacker, Siril | 19 |
| Sharpest-N% / lucky-imaging selection | AutoStakkert! | 20 |
| Calibration library matched by temperature/exposure/gain | PixInsight, MaxIm DL, DeepSkyStacker | 21 |
| Comet + stars dual-stack recombination | PixInsight, DeepSkyStacker | 22 |
| Scripting / batch automation (shell-free JSON queue) | Siril scripts, PixInsight PJSR, MaxIm COM | 23 |
| PSF-based starless masks | PixInsight, Siril | 24 |
| À trous wavelets | PixInsight, Fitswork | 25 |
| Wiener deconvolution | Fitswork | 26 |
| Mosaics from plate-solved panels | PixInsight, Siril, APP, IRIS, MaxIm DL, ASTAP, Fitswork | 27 (blocked by item 13) |
| Ensemble photometry / light curves | AstroImageJ, MaxIm DL | 28 |

Not promoted: GUI, GPU/CUDA acceleration, embedded neural inference
(BXT/NXT/SXT), `simd/archsimd`, 2× drizzle as a general mode, GZIP FITS and
synthetic calibration-frame generation — all already rejected or out of scope
in `ROADMAP.md`. Multi-session/multi-filter automation is covered by item 23
rather than as a separate flag.

## 2026-11-18 corrections and ecosystem additions

The tables above were corrected in place for the highest-impact factual errors
(DSS native Linux and BSD-3 license, Siril complete plate solving and XISF/PNG/BMP
output, ASTAP macOS/ARM, order-3 SIP and bad-pixel maps/weighting, AutoStakkert!
dark/flat calibration, PixInsight and Siril reference-frame crop, Siril mosaic).
The notes below add the structural context that flat `Yes`/`–` matrices cannot express.

### License taxonomy

Use **Open Source (license)** for OSI-approved projects (SkyStacker MIT,
DeepSkyStacker BSD-3-Clause, Siril GPLv3, ASTAP MPL-2.0, Seti Astro Suite Pro
GPLv3, AstroImageJ GPL, THELI/Sirilic/ALS/PSS GPLv3) and **Freeware (closed)**
for distributed-at-no-cost binaries with no source offer (IRIS, AutoStakkert!,
Sequator, Fitswork, StarStaX, ASIStudio). **Freemium** is reserved for SharpCap.
This replaces the previously mixed `Free`/`Freeware` labels.

### Primary domain (recommended extra column)

| Tool | Primary domain |
|---|---|
| SkyStacker, DeepSkyStacker, Siril, PixInsight, APP, ASTAP, IRIS, Nebulosity, MaxIm DL, Fitswork | Deep-sky stacking and reduction |
| OriginStack | Deep-sky stacking (GPLv3, GUI + CLI) |
| AutoStakkert!, PlanetarySystemStacker, Lynkeos, AstroSurface | Planetary / lunar / solar lucky imaging |
| Starry Landscape Stacker, Sequator, Hugin `align_image_stack` | Nightscape / wide-field / star trails |
| AstroImageJ, C-Munipack / Munipack, Seti Astro Suite Pro | Scientific photometry / variable-star reduction |
| SharpCap, Astro Live Stacker, ASILive | Live stacking (EAA) |
| Sirilic, THELI | Multi-session / pipeline orchestration |
| Astropy (`ccdproc`/`reproject`/`astroalign`), AstrOmatic (`SWarp`/`SCAMP`/`SExtractor`) | Library / professional pipeline building |

### Modern stacking metrics (recommended extra columns)

| Tool | Local background normalisation | Bayer/CFA drizzle | Native multi-session / multi-filter |
|---|---|---|---|
| SkyStacker | Yes (`--local-normalize`, per-tile) | Open (item 14) | Partial (`--batch`, item 23) |
| PixInsight | Yes (`LocalNormalization`) | Yes (`DrizzleIntegration` CFA) | Yes (WBPP) |
| Astro Pixel Processor | Yes (LNC) | Yes (Bayer Drizzle) | Yes |
| Siril | Yes (local normalisation) | Yes (Bayer Drizzle) | Via Sirilic |
| Seti Astro Suite Pro | Yes (ABE/DBE) | Yes (CFA Drizzle) | Yes |
| THELI / AstrOmatic | Yes | Partial (SWarp reprojection) | Yes |

### Missing open-source tools

| Tool | License / platforms | Role |
|---|---|---|
| Seti Astro Suite Pro | GPLv3, Win/macOS/Linux, GUI + headless CLI/Python | Full reduction suite; advanced rejection (Winsorized, Kappa-Sigma, Rosner ESD), CFA drizzle, comet stacking, Multi-Frame Deconvolution Stacking, ABE/DBE, cross-platform GPU |
| AstrOmatic Suite (`SWarp`+`SCAMP`+`SExtractor`) | GPLv3, Linux/macOS/Win (WSL), CLI | Professional astrometry, high-order distortion, weighted FITS mosaics |
| Astropy (`ccdproc`+`reproject`+`astroalign`) | BSD-3, multi-platform, Python library/CLI | Scientific CCD reduction, triangle registration, flux-conserving reprojection |
| THELI | GPLv3, Linux/macOS, GUI + CLI | Automated wide-field pipeline with SCAMP/SWarp |
| Sirilic | GPLv3, Win/macOS/Linux, GUI + CLI | Multi-session/multi-filter Siril orchestrator |
| Astro Live Stacker | GPLv3, Win/macOS/Linux/Raspberry Pi, GUI + web | Live deep-sky stacking (EAA) |
| PlanetarySystemStacker | GPLv3, Win/macOS/Linux, GUI + CLI | FOSS alternative to AutoStakkert! with local-sharpness selection and multi-point alignment |
| Lynkeos | GPLv2, macOS, GUI | Native macOS planetary/lunar stacking with wavelets |
| C-Munipack / Munipack | GPLv2/GPLv3, Win/Linux, GUI + CLI | Differential photometry and FITS time-series reduction |
| Hugin `align_image_stack` | GPLv2, Win/macOS/Linux, CLI | Wide-field/landscape alignment with lens-distortion correction |

### Missing freeware / proprietary tools

| Tool | Model / platforms | Role |
|---|---|---|
| ASIStudio (`ASIDeepStack`/`ASILive`) | Freeware, Win/macOS/Linux | ZWO-oriented fast deep-sky and live stacking |
| SharpCap | Freemium, Windows | EAA reference with live calibration, plate solving and FWHM/background weighting |
| AstroSurface | Freeware, Windows | Modern planetary/lucky-imaging stacker with wavelets and deconvolution |
| StarStaX | Freeware, Win/macOS/Linux | Cross-platform star-trail stacking with gap filling |

URLs for the new tools will be added when verified; only OriginStack URLs were
supplied with this correction set and are listed in `## Sources`.

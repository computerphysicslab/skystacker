# SkyStacker — feature comparison

This table compares SkyStacker with the most common astrophotography
stacking and pre-processing tools. It is a **high-level orientation**, not a
benchmark: feature sets change between releases, and some tools offer more
than a single cell can capture.

Legend: `Y` = supported; `-` = not supported / not applicable;
`Partial` = limited or workflow-dependent.

## A. Calibration, registration and stacking

| Software | License / price | Platforms | Calibration | Registration | Stacking methods | Normalization / weighting |
|---|---|---|---|---|---|---|
| **SkyStacker** | MIT, free | Windows, macOS, Linux (Go single binary) | Dark, flat, bias; `--calib-root` auto-detect; master stacking from folders | Triangle RANSAC similarity; optional affine refinement; sub-pixel bilinear; safe crop | Mean, median, sigma (MAD); max/min diagnostics | Global + local per-tile normalization; weighted sigma by frame quality x intrinsic quality |
| **DeepSkyStacker** | Free (source on GitHub) | Windows; macOS native since 6.1.3 (Jan 2026); Linux via Wine | Dark, flat, bias, dark-flat | Star-pattern translation/rotation; manual star/comet editing | Average, median, kappa-sigma, auto-adaptive weighted, entropy-weighted, max | Limited; calibration frames handle most background correction |
| **Siril** | GPLv3, free | Windows, macOS, Linux | Master bias/dark/flat/dark-flat; cosmetic correction; bad-pixel map | Global star alignment, one-star, 2-pass, distortion correction; SER video | Mean, median, sigma-clip, weighted; rejection maps; HST drizzle | RGB equalization, lightweight normalization, star-count/wFWHM weighting |
| **Astro Pixel Processor** | Paid (EUR 125/year rental) | Windows, macOS, Linux | Full calibration; bad-pixel map; master frames | Quadrilateral/triangle matching; distortion; mosaic registration | Average, median, sigma, weighted; drizzle incl. Bayer drizzle | Local Normalization Correction (LNC); multi-band blending; multi-session normalization |
| **PixInsight** | Paid (EUR 300 non-EU; 45-day trial) | Windows, macOS, Linux | WBPP/FBPP; cosmetic correction; master frames | Star alignment; distortion correction; plate solving; drizzle; local normalization | Mean, median, sigma, weighted, pixel rejection; drizzle integration | Local normalization; SSWEIGHT; per-frame weighting |
| **Sequator** | Freeware | Windows (macOS via CrossOver) | Dark, flat (basic) | Star alignment; freeze-ground; lens-distortion correction | Average, median, HDR, star-trail modes | Limited |
| **ASTAP** | MPL 2.0, free | Windows, Linux | Dark, flat; HFD filtering | Star alignment; native plate solver | Average, median, sigma/rejection | Limited |
| **IRIS** | Free for non-commercial use | Windows (Linux via Wine) | Bias, dark, flat; DSLR control | Star alignment; affine transformation; astrometry | Add, mean, median, sigma | Limited |
| **Nebulosity** | Paid | Windows, macOS | Dark, flat, bias; bad-pixel map | Translation; translation+rotation; +scale | Average, median, SD-mask, sigma | Limited |
| **MaxIm DL** | Paid | Windows | Full calibration; bad-pixel map | Star alignment; LRGB groups; on-the-fly calibration | Mean, median, sigma-clip, SD-mask | Limited |
| **AstroImageJ** | GPL, free | Java (Windows, macOS, Linux) | Basic image calibration | Alignment; stack editing; normalize stack | Average, median, time-series stacks | Basic normalization |
| **Starry Landscape Stacker** | Paid | macOS | Dark, flat | Star alignment + landscape freeze; lens-distortion correction | Averaging/compositing | - |
| **AutoStakkert!** | Free for private use | Windows | - (planetary/lunar/solar) | Planet alignment; local alignment points | Lucky-imaging quality selection | - |

## B. Processing, automation and output

| Software | Drizzle | Mosaic | Comet stacking | Built-in post-processing | Scripting / CLI | Output formats | Acceleration |
|---|---|---|---|---|---|---|---|
| **SkyStacker** | - | - | - | Background removal, star removal, STF/MTF stretch, guided denoise incl. chroma, curves, HDR blend, 4 weighted mixes, diffraction spikes | CLI flags only | Float32 TIFF, raw float32 work file, JPEG previews, `report.txt`, rejection/weight maps | CPU multi-core |
| **DeepSkyStacker** | Y x2/x3 | - | Y | Minimal; outputs TIFF/FITS | DSS command-line tool | TIFF, FITS | CPU |
| **Siril** | Y HST algorithm | Partial (registration-based) | Y | Background extraction, colour calibration, deconvolution, histogram/curves, star removal via StarNet | Python/Siril scripts; `siril-cli` | FITS, TIFF, JPEG, SER | CPU multi-core |
| **Astro Pixel Processor** | Y incl. Bayer | Y automatic multi-panel | Limited | Light-pollution removal, background calibration, colour combination, stretch, saturation, sharpening | GUI; limited CLI | FITS, TIFF, JPEG | CPU, memory-heavy |
| **PixInsight** | Y | Y | Y | Exhaustive: deconvolution, noise reduction, star removal (plugins), histogram, curves, SPCC, etc. | PJSR JavaScript, Python, CLI | XISF, FITS, TIFF, JPEG | CPU (some GPU-assisted modules) |
| **Sequator** | - | - | - | Light-pollution reduction, star enhancement, HDR | GUI | TIFF, JPEG | CPU |
| **ASTAP** | - | - | - | Photometry, CCD inspector, annotation | CLI (PlateSolve2-compatible) | FITS, TIFF, JPEG | CPU |
| **IRIS** | - | - | - | Unsharp mask, Gaussian/adaptive filters, photometry, astrometry, spectra | Command-line editor + GUI | FITS, PIC, BMP, raw | CPU |
| **Nebulosity** | - | - | - | Digital development, curves, colour, noise reduction | GUI | FITS, TIFF | CPU |
| **MaxIm DL** | - | Limited | - | Deconvolution, colour, stretching, photometry | COM automation/scripting | FITS, TIFF, JPEG | CPU |
| **AstroImageJ** | - | - | - | Photometry, astrometry, plotting | ImageJ macros, Java | FITS, TIFF, PNG | CPU |
| **Starry Landscape Stacker** | - | - | - | Minimal; 32-bit float output | GUI | TIFF, JPEG | CPU |
| **AutoStakkert!** | - | - | - | Sharpening usually via RegiStax | CLI | TIFF, PNG | CPU |

## Where SkyStacker sits

**Strengths**

- Only tool in this list that is a single cross-platform binary with no GUI and no external runtime.
- Fully reproducible CLI workflow; every option is a flag, every run is scriptable.
- Float32 end-to-end accumulation, plus per-pixel rejection and weight maps.
- Weighted sigma combines registration quality and intrinsic frame quality.
- Global + local normalization without a separate pre-processing tool.
- Unusually complete built-in post-processing for a stacker: background removal, star removal, STF auto-stretch, guided denoise (including chroma), curves, HDR blend, four weighted mixes and diffraction spikes.
- Transparent `report.txt` with per-frame diagnostics (stars, inliers, rotation, drift, quality, weight).

**Gaps versus mature tools**

- No GUI (CLI only).
- No mosaic construction.
- No drizzle (standard or Bayer).
- No plate solving / WCS / SIP distortion model.
- No deconvolution or PSF modelling.
- No dedicated comet or moving-object stacking.
- No scripting beyond command-line flags.
- No GPU acceleration.
- Input formats limited to JPG/JPEG, CR2/CR3, FIT/FITS; no XISF or SER.
- No multi-session / multi-filter automation.
- Planetary, lunar and solar lucky imaging are out of scope.

## Sources and version notes

Feature summaries were cross-checked against public documentation and reviews
available in late 2026, including:

- DeepSkyStacker - <https://github.com/deepskystacker/DSS/releases> and <http://deepskystacker.free.fr>
- Siril - <https://siril.org> and <https://siril.pixls.us>
- Astro Pixel Processor - <https://www.astropixelprocessor.com>
- PixInsight - <https://pixinsight.com>
- ASTAP - <https://www.hnsky.org/astap.htm>
- IRIS - <http://www.astrosurf.com/buil/us/iris/iris.htm>
- Nebulosity - <https://www.stark-labs.com/nebulosity.html>
- MaxIm DL - <https://diffractionlimited.com/product/maxim-dl/>
- AstroImageJ - <https://www.astro.louisville.edu/software/astroimagej/>
- Starry Landscape Stacker - <https://apps.apple.com/app/starry-landscape-stacker/id1446056095>
- AutoStakkert! - <https://www.autostakkert.com>

If a cell disagrees with the version you use, treat the vendor documentation
as authoritative and update this file.

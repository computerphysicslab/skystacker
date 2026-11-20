# CLI reference

Usage: `skystacker [flags] [input-directory]`. Flags may appear before or after
the positional input directory. The default input directory is `.`.

This page mirrors `skystacker --help`; use that command for the authoritative,
build-specific text. See the linked pages for the semantics of each subsystem.

Discovered inputs include `.tif/.tiff/.btf/.bigtiff` files, decoded by the
first-party `goTiff` module: 8/16-bit uint or 32/64-bit float samples,
grayscale/RGB, classic and BigTIFF, uncompressed/PackBits/LZW/Deflate, plus
Astro-TIFF tag 270 and CFA metadata.

## Input and output

| Flag | Default | Description |
|---|---|---|
| `--output DIR` | `<input>/0_skystacker_results` | Product directory. |
| `--workers N` | `NumCPU` | Maximum concurrent workers. |
| `--recursive` | off | Search subdirectories. |
| `--overwrite` | off | Permit replacing files in the output directory. |
| `--limit N` | `0` | Process at most N discovered frames. |
| `--include-list FILE` | — | Keep only paths/names listed in FILE (`#` comments allowed). |
| `--reference-crop IMAGE` | — | Warp every light into this image's pixel frame. |
| `--no-cache` | off | Disable the star/transform cache. |
| `--cache-dir DIR` | `<output>/_work/cache` | Cache directory. |
| `--keep-cache` | off | Keep the cache after a successful run. |

## Registration and stacking

| Flag | Default | Description |
|---|---|---|
| `--method mean\|median\|sigma` | `mean` | Stacking reducer. |
| `--no-register` | off | Disable star-based registration. |
| `--no-affine` | off | Similarity-only registration. |
| `--sigma-clip F` | `3.0` | MAD rejection threshold for median/sigma. |
| `--sigma-clip-iters N` | `2` | Rejection iterations. |
| `--sigma-clip-low F` / `--sigma-clip-high F` | symmetric | Asymmetric thresholds. |
| `--tile-size N` | auto | Reduction tile side in pixels. |
| `--rejection MODE` | — | `auto/mean/median/sigma` stack-size heuristic. |
| `--keep-sharpest N` | `0` | Keep sharpest N% by median HFR. |
| `--frame-expr EXPR` | — | Keep frames whose expression value > 0.5; emit `SSWEIGHT` (`@file` supported). |
| `--hfr-quality` | off | Use star HFR instead of blob size as the FWHM proxy. |
| `--psf-scale-snr` | off | Multiply PSF weights by XNML relative scale. |
| `--xnml-dir DIR` | light folder | Fallback folder for `.xnml` sidecars. |
| `--frame-metrics FILE` | — | Write per-frame CSV. |

## Calibration

| Flag | Default | Description |
|---|---|---|
| `--dark FILE` / `--flat FILE` / `--bias FILE` | auto-detect | Explicit masters. |
| `--calib-root DIR` | — | Auto-detect Bias/Darks/Flats subfolders. |
| `--calib-library DIR` | — | Match masters by exposure/temperature/ISO/gain. |
| `--export-masters` | off | Write `master_*.f32raw` + JPEG for inspection. |
| `--black-level auto\|off` | `off` | Subtract EXIF/FITS black pedestal. |
| `--bpm FILE` / `--bpm-auto` / `--bpm-auto-sigma F` | off | Bad-pixel map. |
| `--self-flat` / `--no-self-flat` | auto | Grid self-flat from lights when no flat is found. |
| `--self-flat-model grid\|registered` | `grid` | Self-flat estimator. |
| `--self-flat-linear-srgb` | off | Experimental linear-light division. |
| `--normalize` / `--local-normalize` | off | Background equalisation (requires registration). |
| `--normalize-mode MODE` | `scale-offset` | `additive`, `multiplicative` or `scale-offset`. |
| `--normalize-per-channel` | off | Per-channel global normalisation. |
| `--ln-tile-size N` | `256` | Local-normalisation tile size. |
| `--synthetic-calibration*` | — | Report/recoverability guardrails; synthesis is refused. |

## Plate solving and astrometry

See [plate-solving.md](plate-solving.md) and [astrometry-products.md](astrometry-products.md).

| Flag | Default | Description |
|---|---|---|
| `--plate-solve` | off | Solve after stacking or in `--post-only`. |
| `--plate-solver MODE` | `auto` | `auto`, `gaia`, `astrometry-local`, `astap`, `astrometry-web`. |
| `--plate-center R,D` | — | Explicit RA/Dec seed in decimal degrees. |
| `--test-plate R,D` | — | One-shot synthetic solve against Gaia, then exit. |
| `--test-gaia R,D` | — | One-shot Gaia tile query, then exit. |
| `--astrometry-cmd CMD` | `solve-field` | Local astrometry.net command. |
| `--astap-cmd CMD` | `astap` | Local ASTAP command. |
| `--astrometry-api-key KEY` | env | nova API key (`AN_API_KEY`, `ASTROMETRY_API_KEY`). |
| `--astrometry-url URL` | nova | API base URL. |
| `--astrometry-timeout N` | `180` | Blind-solve timeout in seconds. |
| `--solver-focal-length MM` / `--solver-pixel-size UM` | — | Scale hint and sanity check. |
| `--gaia-mirror URL` / `--gaia-cache-dir DIR` | ESA / OS cache | Gaia TAP endpoint and cache. |
| `--gaia-nside N` / `--gaia-mag-limit M` | `64` / `14.0` | Tile resolution and magnitude limit. |
| `--gaia-offline` / `--gaia-cache-refresh` | off | Offline mode / forced refresh. |
| `--gaia-multitile` (default on) / `--no-gaia-multitile` | on | Grow the seeded Gaia catalogue from all cached tiles around the pointing. |
| `--blind-detrend` | off | Detrended, HFD-filtered star extraction for blind solves on processed frames. |
| `--gaia-loose-match` | off | Relax matcher thresholds for processed images (8 arcsec, 1/8 of top stars). |
| `--resample-sip` | off | Apply stored SIP in the stacking resampler. |
| `--annotate` | off | Gaia projection + anomaly flags. |
| `--annotation-catalogs LIST` | — | Extra JSON catalogues. |
| `--sso-ephemeris FILE` | — | MPC-style SSO ephemeris with site header and mandatory `DATE-OBS`; velocity-vector layer. |
| `--cone-catalog NAME` / `--cone-radius N` | — / `60` | SDSS/IRSA/VizieR cone search (arcsec). |
| `--sky-map` / `--survey-cutout` | off | Gaia SVG map / HiPS FITS cutout. |
| `--survey-hips ID` / `--survey-timeout N` | `CDS/P/DSS2/color` / `30` | Survey selection and timeout. |
| `--photometry` | off | Ensemble Gaia aperture photometry. |
| `--aperture-photometry` | off | Local annulus sky, Kron/FLUX_RADIUS and neighbour masking in `--photometry`. |
| `--matched-filter` | off | Matched-filter star detection (`T = αC⁻¹S`) instead of the high-pass threshold. |
| `--blind-benchmark DIR` / `--benchmark-solvers LIST` | — | Head-to-head solve corpus CSV (solvers include `native`). |
| `--blind-index FILE` | `<gaia-cache-dir>/blind_index.json` | Gaia quad index for `--plate-solver native`. |
| `--build-blind-index FILE` | — | Build that index from cached Gaia tiles and exit. |
| `--blind-allsky` / `--blind-max-tiles N` | off / `0` | Sweep Gaia sky tiles before building; bound the sweep to N tiles. |
| `--blind-tile-offset N` / `--blind-refresh` | `0` / off | Chunked/resumable catalog builds. |
| `--blind-region RA,DEC,RADIUS` / `--blind-index-info FILE` | — | Cone-limited regional index / print index metadata. |
| `--blind-density-cap N` | `0` | Stars/deg² per sub-cell when building an index (0 = brightest-star cut); `.sbi` paths use the compact SBI1 binary format. |
| `--blind-catalog ID` | — | Use a published magnitude-limited catalog by id. |
| `--list-blind-catalogs` | off | Print the catalog registry and exit. |
| `--download-blind-catalog ID` | — | Download and verify a catalog into the local cache. |
| `--blind-mag-limit M` / `--blind-brightest N` | `14` / `100` | Gaia magnitude limit and stars per tile for catalog builds. |

## Scientific outputs

| Flag | Default | Description |
|---|---|---|
| `--fits` / `--xisf` / `--bigtiff` | off | Additional scientific writers. |
| `--xisf-rev2` | off | XISF checksum + strict sample validation. |
| `--png` / `--bmp` | off | Extra display previews. |
| `--json-report` / `--provenance` | off | Structured report / IVOA-style provenance. |
| `--starless` | off | Starless and star-layer products. |

## Post-processing

See [post-processing.md](post-processing.md).

| Flag group | Examples |
|---|---|
| Background | `--bg-degree N`, `--bg-multiscale`, `--bg-asymmetric` (with `--bg-clip-low/-high`, `--bg-harmonic-scale`, `--bg-poly-degree`, `--bg-rbf`) |
| Stretch | `--stretch F`, `--no-auto-stretch`, `--stf-shadows F`, `--stf-target F`, `--stf-linked` |
| Denoise | `--no-denoise`, `--denoise-strength F`, `--denoise-radius N`, `--denoise-chroma F`, `--wavelets`, `--wavelet-scales N`, `--wavelet-sigma F`, `--wavelet-adaptive` |
| Sharpen | `--no-sharpen`, `--sharpen-amount F`, `--sharpen-radius F`, `--wavesharp`, `--wave-gains LIST` |
| Deconvolve | `--deconvolve`, `--deconvolve-wiener`, `--deconvolve-grid`, `--deconv-sigma F`, `--deconv-iters N`, `--deconv-grid-size N` |
| Curves/HDR | `--no-curves`, `--curves-black F`, `--curves-gamma F`, `--no-hdr-blend`, `--hdr-blend-factor F`, `--no-hdr-mix` |
| Spikes | `--spikes`, `--no-spikes`, `--spikes-*` family |
| Order | `--post-preset default\|detail-last`, `--no-color-calib` |
| Narrowband | `--narrowband HOO\|SHO`, `--nb-a/--nb-b/--nb-c` |
| Drizzle | `--drizzle-upsample N`, `--drizzle-pixfrac F`, `--wht-ctx` (WHT/CTX sidecars) |
| Cosmetic | `--cosmic-reject`, `--streak-detect`, `--starless-psf` |
| External AI | `--external-bge/--external-denoise/--external-deconv/--external-starless CMD`, `--external-timeout N`, `--external-allow-nonlinear` |
| Comet | `--comet-center`, `--comet-rate`, `--comet-radec`, `--comet-sky-rate-arcsec`, `--comet-target`, `--comet-combine`, `--comet-ephem-strict-exif` |

## Standalone image mode

| Flag | Default | Description |
|---|---|---|
| `--process-image PATH[,PATH...]` | — | Process existing images without stacking. |
| `--image-op OP` | — | `blur`, `sharpen`, `background`, `stretch`, `denoise`, `deconvolve`, `starless`, `sum`, `subtract`, `average`, `median`, `min`, `max`, `measure-transform`, `measure-quality`, `sky-map`, `survey-cutout`, `plate-solve`. |
| `--image-output DIR` | `image_processing` | Output directory. |
| `--export-stars FILE` | — | Export detected stars of `--process-image` as flux-sorted JSON/CSV; no `--image-op` required. |
| `--classify-objects FILE` | — | Report-only diffuse-object morphology (CAS/Gini/M20) and IVOA object-type JSON; no `--image-op` required. |

## Diagnostics and control

| Flag | Default | Description |
|---|---|---|
| `--config FILE` | — | JSON config expanded before CLI flags (CLI wins). |
| `--batch FILE` | — | JSON queue of CLI invocations. |
| `--resume` | off | Continue with post-processing using an existing stack. |
| `--stack-only` / `--post-only` / `--no-postprocess` | off | Pipeline stage selection. |
| `--no-native-cr2` | off | Use dcraw/ImageMagick for CR2. |
| `--test-debayer IMAGE` | — | FITS debayer diagnostic. |
| `--check-acquisition-keywords FILE` | — | Validate N.I.N.A. RAW/FITS acquisition keywords; JSON output, exit 1 on invalid values. |
| `--diagnose` / `--diagnose-json` / `--diagnose-quick` / `--diagnose-integrated` | off | Host diagnostics. |
| `--diagnose-timeout N` / `--diagnose-top N` / `--diagnose-updates` | `5` / `10` / off | Diagnostic collectors. |
| `--donation` / `--no-donation` / `--no-donation-qr` | — | Donation footer control. |
| `--force` | off | Bypass the OOM safety check. |
| `--clean` | off | Remove `_work` after success. |
| `--selftest` / `--version` / `--help` | — | Control flags. |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success (including `--help`, `--version`, a completed `--selftest`). |
| `1` | Runtime failure: invalid data, failed solve, I/O error, failed self-test. |
| `2` | Flag parsing error. |

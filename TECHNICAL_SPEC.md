# SkyStacker — Technical Specification

**Current version:** 0.6.1
**Implementation language:** Go 1.27.4 (single executable, standard library only; one first-party dependency `github.com/computerphysicslab/skystacker/PureRaw v0.1.1`, tag `PureRaw/v0.1.1`)
**Status:** Active Development. Registered `--selftest` cases: 209. Star/transform cache version: 2. Persistent fixtures: `assets/testdata/`.

## 1. Purpose
SkyStacker is a local command-line program that discovers, aligns, normalizes, calibrates, and stacks astronomical images. It produces high-dynamic-range float32 TIFFs and a fully post-processed display image, using robust statistical stacking and PixInsight-style luminance stretching.

## 2. Scope

### Included
- CLI executable `skystacker`, single Go file, standard library only.
- Input formats: JPG/JPEG/PNG (native), FITS (native, with ImageMagick fallback), SER (8/16-bit mono, Bayer-as-gray, RGB/BGR), CR2 native pure-Go lossless JPEG; CR3 via dcraw/ImageMagick at runtime.
- Star detection, triangle matching, RANSAC similarity transforms, optional 6-DOF affine refinement.
- Calibration frames (dark, flat, bias; individual files or auto-discovered folder trees via `--calib-root`).
- Stacking methods: mean (streaming), median (tiled), sigma-clipping (tiled, MAD-based, weighted).
- Optional per-frame normalization (global or per-tile local).
- Max/Min stacks (brightest/darkest pixels), optional TIFF sum output.
- 8-step post-processing pipeline (background, star removal, black point, STF/MTF auto-stretch, denoise, curves, HDR blend, weighted mix + optional diffraction spikes).
- Star/transform cache (`_work/cache/`) skipping Pass 1 detection and Pass 2 registration on re-runs.
- Memory and disk pre-flight checks; calibrated time estimates.
- 208 embedded self-tests (the count printed by `./skystacker --selftest` is authoritative; `runSelfTests` in `main.go` is the source of truth).

### Excluded
- Native RAW decoding (delegated to ImageMagick / dcraw at runtime). FITS decoding is native with ImageMagick fallback.
- Drizzle and Bayer drizzle not wired. SIP order-2 is supported in `wcsTanForward`/`wcsTanInverseSIP` (used by WCS/plate-solve paths); the image resampler used for stacking still applies linear TAN/affine transforms unless explicitly extended.
- Higher-order distortion models beyond SIP order 2.
- Whole-sky blind plate solving. TAN WCS plate solving is included and wired via `--plate-solve`; blind mode is a 3x3 nside=64 local scan around (0,0) and will fail on a real field, so a pointing seed (`--plate-center` or the reference frame's FITS `RA`/`DEC`) is required in practice.
- GUI.

## 3. CLI Contract

```text
skystacker [flags] [input-directory]
```

Flags are accepted before or after the positional input directory. `--config FILE` loads a JSON object of flag names (without `--`) with string/number/bool/array values; config flags are applied first so explicit CLI flags override them. Default input directory: `.`. Default output directory: `<input>/0_skystacker_results`.

### Core flags
- `--method` (default `sigma`): `mean`, `median`, `sigma`.
- `--output` (default `<input>/0_skystacker_results`).
- `--workers` (default NumCPU).
- `--recursive`, `--overwrite`, `--force`.
- `--scale` (default 1; downscale factor).
- `--no-register`, `--stack-only`, `--post-only`.
- `--reference-crop PATH`: detect stars in `PATH`, match each light by
  triangle pattern, and warp it in memory into the reference frame. The
  output size is exactly the reference image size, independent of the
  light dimensions. Requires registration. `--limit 1` produces a single
  cropped frame. Source files are never written.

### Calibration
- `--dark`, `--flat`, `--bias` (individual files or folders).
- `--bpm PATH` loads a user bad-pixel map; `--bpm-auto [--bpm-auto-sigma N]` derives one from the master dark (or bias) using median + N*1.4826*MAD on a strided sample and flags outliers over the full frame (mean-deviation fallback when MAD=0).
- `--export-masters` writes each loaded master as `master_<kind>.f32raw`
  plus a `.jpg` preview.
- `--self-flat` derives a relative illumination map from ≥15 registered
  dithered lights and applies it after registration; a loaded flat wins.
  This is the v1 reference-coordinate model; it can absorb extended
  emission and only corrects the registered stack. The specified successor
  is a full-frame sensor-space estimator (unregistered frames, source mask,
  coarse grid, robust rejection, IRLS polynomial) applied before
  resampling; see `research/04-calibration-and-self-flat.md` §9-14 and
  `ROADMAP_SYNTHETIC_CALIBRATION.md` §2.3.
- `--self-flat-model` is `grid` (default, Phase-3
  sensor-space model: unregistered frames, local-contrast source mask,
  SExtractor cell mode with low-quantile consistency, 5x5 median/MAD cell
  rejection, 64 px cells, degree-4 Chebyshev fit refined by asymmetric
  Huber IRLS, radial-model comparison, and positivity/extrapolation
  diagnostics, applied before resampling). If extrapolation is flagged and
  `min < 0.45`, the model is rejected with a warning. `report.txt` records
  fit RMS, OLS RMS, radial RMS, IRLS iterations and the Chebyshev
  coefficients.
- `--calib-root` (auto-detect `Bias/Darks/Flats/<filter>` subfolders).

### Normalization (requires registration)
- `--normalize`: per-frame global scale+offset to the reference frame.
- `--local-normalize`: per-tile, bilinear-interpolated correction; implies `--normalize`. `--normalize-mode` selects scale-offset (default), additive (median-only) or multiplicative (clamped median ratio) correction.
- `--ln-tile-size` (default 256).

### Cache
- `--no-cache`: disable the star/transform cache (reads and writes).
- `--cache-dir` (default `<output>/_work/cache`).

### Stacking
- `--sigma-clip` (default 3.0), `--sigma-clip-iters` (default 2), optional `--sigma-clip-low` / `--sigma-clip-high` for asymmetric MAD rejection on each side (0 = symmetric).
- `--tile-size` (default 0 = auto, capped at ~256 MB tile buffer).
- `--tiff` writes `TIFF/stacked_sum.tif`; `--fits` writes `stacked_sum.fits` (big-endian IEEE float32, BITPIX=-32, NAXIS=3, 2880-byte blocks, TAN WCS keywords when a plate solution exists); `--xisf` writes `stacked_sum.xisf` (monolithic XISF 1.0, planar little-endian Float32, no compression). Both divide by the same sum normalisation factor as the TIFF. `--no-exif`.

### Post-processing
- `--mask-bg`, `--bg-degree` (1-3, default 2), `--bg-percentile` (default 10), `--stretch` (default 10).
- `--star-radius` (0 = auto), `--star-sigma` (default 4.0).
- `--no-auto-stretch`, `--stf-shadows` (default -2.8), `--stf-target` (default 0.25), `--stf-linked`.
- `--no-hdr-blend`, `--hdr-blend-factor` (default 1.0), `--no-hdr-mix`.
- `--no-denoise`, `--denoise-strength` (default 4.0), `--denoise-radius` (default 3), `--denoise-chroma` (default 1.0).
- `--no-curves`, `--curves-black` (default 0.03), `--curves-gamma` (default 1.20).
- `--post-ca`, `--post-color`, `--post-flatten`, `--post-sharpen`, `--post-hdr`, `--post-stretch`, `--no-color-calib`, `--no-sharpen`.

### Diffraction spikes (step 8 overlay, ON by default)
- `--spikes` (default true), `--no-spikes`.
- `--spikes-percent` (0.2), `--spikes-length` (10), `--spikes-intensity` (0.3), `--spikes-arms` (4).
- `--spikes-softness` (0.5), `--spikes-min-ratio` (0.2).
- `--spikes-length-softness` (1.0), `--spikes-length-min-ratio` (0.03), `--spikes-min-count` (5).

### Diagnostics
- `--selftest`, `--version`, `--help`, `--json-report` (writes `report.json`).
- `--test-gaia RA,DEC` — one-shot Gaia DR3 tile query; prints nside, ipix, first 5 stars, exits.
- `--test-debayer PATH` — one-shot FITS debayer diagnostic; writes `<base>.jpg` and `<base>_raw.jpg`.
- `--plate-solve` — run the TAN WCS plate solver after stacking, or on the existing `_work/stacked_mean.f32raw` in `--post-only` (stars re-detected from the stored stack, seeded by `--plate-center`); writes `plate_solution.json`.
- `--plate-center RA,DEC` — seed for `--plate-solve` in decimal degrees. Falls back to the reference frame's FITS-header `RA`/`DEC` when unset; blind mode scans only the 3×3 nside=64 neighbourhood around (0,0) and will fail on a real field.
- `--annotate` — offline catalog annotation: projects cached Gaia stars through `plate_solution.json` (SIP included) into `stacked_annotated.jpg` (green crosses) and writes `anomalies.json` for blind detections farther than 8 px from any catalog source (red crosses). Skips cleanly without plate solution or cached tile.
- `--png` — write `stacked_mean.png` alongside the JPEGs (same gamma 0.5 LUT as the JPEGs, so the two are directly comparable; lossless).
- `--bmp` — write `stacked_mean.bmp` alongside the JPEGs (hand-written uncompressed 24-bit BMP, BGR, bottom-up, 4-byte row padding).

### Gaia DR3 data-access layer
- `--gaia-mirror URL` (default `https://gea.esac.esa.int/tap-server/tap`).
- `--gaia-cache-dir D` (default: OS user cache dir, e.g. `~/.cache/skystacker/gaia_cache` on Linux).
- `--gaia-nside N` (default 64; power of 2, 2-8192).
- `--gaia-mag-limit M` (default 14.0; 0-20).
- `--gaia-offline` (cache miss = hard error; no network access).
- `--gaia-cache-refresh` (re-download even if cached; mutually exclusive with `--gaia-offline`).

## 4. Numeric Model & Algorithms

### 4.1 Stacking
- **Mean**: streaming accumulate (one buffer per frame), divided by per-pixel valid count. Out-of-core not required.
- **Median**: tiled, out-of-core. Frames are spilled to `_work/_spill1/` as raw float32 in pass 1; register/resample produces `_work/_spill2/`; `reduceTilesStack` reduces tile-by-tile, capped at ~256 MB tile buffer to bound peak RSS.
- **Sigma-clipping**: MAD-based rejection. Per pixel per channel: compute median and `sigma = 1.4826 * MAD`; reject values with `|v - median| > threshold * sigma`; iterate `sigma-clip-iters` times. Default threshold 3.0 (not 2.5). Weighted variant uses per-frame quality weights.
- **Max/Min**: tracked in pass 1 across all accepted frames.

### 4.2 Star detection and registration
- **Detection**: 8-connected components above `sigma * noise` where `noise` is the MAD of a high-pass residual (`box blur` of radius `clamp(min(W,H)/200, 8, 24)`). Default sigma 5.0, with multi-sigma retry at 4.0/3.5/3.0 if fewer than 20 stars are found.
- **Reference frame**: frame with the most detected stars.
- **Matching**: triangles built from the top **40** brightest stars (not 30). Descriptor `(short/long, mid/long)`, bucketed on a 0.01 grid with 0.02 tolerance.
- **Transform**: closed-form similarity (rotation, scale, translation) via least squares; inlier refinement; optional full 6-DOF affine refinement kept only when residual drops ≥ 10 % and inliers ≥ 15.
- **Resampling**: bilinear sub-pixel. **Crop**: largest fully-valid rectangle (`findSafeCrop`, maximal-rectangle-in-histogram).
- **Frame diagnostics**: per-frame stars, inliers, scale, rotation, offset, bounding box, `BoxPct`, `IntrQ`, `Weight`. Each detected `star` records flux, connected-component size, eccentricity and flux-weighted half-flux radius (HFR; Gaussian ≈ 1.18σ). Crop limiter flags: `LOW QUALITY` (< 30), `LOW BOX%` (< 50 %), `LARGE OFFSET` (`|drift| > 10 % refW/H`, where drift subtracts the rotation-induced expected offset so legitimately-rotated frames are not falsely flagged).

### 4.3 Per-frame normalization
Robust statistics per frame: `(median, MAD)` of luminance, computed on a 1024-bin histogram with 4× subsampling. **Scale** = `ref.MAD / frame.MAD` clamped to `[0.5, 2.0]`. **Offset** = `ref.median - scale * frame.median`. **Global**: single scale+offset per frame. **Local**: same correction per `ln-tile-size` tile, bilinearly interpolated between tile centers. Reference frame is never normalized. Both modes require registration.

### 4.4 Post-processing pipeline (8 steps)
1. **Background removal**: robust 2nd-degree 2D polynomial fit to 32×32 block samples (20th percentile per channel), 3 iterations of outlier rejection at `> 2 sigma` above the fit. If `--mask-bg`, pixels above 0.1 luminance are excluded.
2. **Star removal**: high-pass detection, replaced by median of an annulus (radius scaled by star size).
3. **Black-point shift**: subtract a `--bg-percentile` percentile of luminance; not written to disk (consumed by step 4).
4. **STF/MTF auto-stretch**: per-channel shadow clip (`c0 = median + shadowsClip * sigma`) and MTF midtone balance. `--stf-linked` shares one transform across channels.
5. **Guided-filter denoise**: per-channel self-guided filter (He et al. 2010), `strength` passes, followed by a two-scale chroma-only pass (radii 24 and 96 with eps 0.005) for low-frequency colour blotch.
6. **Curves**: `x' = clamp((x - black) / (1 - black), 0, 1); out = x'^gamma`. Applied identically per channel.
7. **HDR blend**: `out = clamp(step6 + hdrBlendFactor * step1, 0, 1)`.
8. **Weighted mix**: 4 variants `w*step1 + (1-w)*step7`, `w ∈ {0.90, 0.80, 0.70, 0.60}`. Optional diffraction-spike overlay per star, `intensityWeight = (flux/maxFlux)^spikeSoftness`, `lengthWeight = (flux/maxFlux)^lengthSoftness`, floored at `spikeMinRatio` / `spikeLengthMinRatio`, same base angle for all stars, additive only.

Optional steps gated by flags: `--post-ca`, `--post-color`, `--post-flatten`, `--post-sharpen`, `--post-hdr`, `--post-stretch`.

### 4.5 Gaia DR3 data-access layer

Consumed by `--plate-solve` (seeded via `--plate-center` or the reference frame's FITS RA/DEC; blind mode scans only the 3x3 nside=64 neighbourhood around (0,0)). Still reachable standalone via `--test-gaia`. The solved TAN WCS is written to `plate_solution.json` and the SIP order-2 coefficients are stored alongside it; applying SIP to the resampler is not yet wired (see §2 Excluded).

- **HEALPix nested scheme** — `healpixAng2PixNested`, `healpixPix2AngNested`, `healpixTileCorners`. `ang2pix`/`pix2ang` match healpy 1.20.1 exactly (nside 1/16/64/256, ~3,600 reference points; exact pixel-corner points are float-noise ties). `healpixTileBoundary` ports `Healpix_Base::boundaries(step)` and matches `hp.boundaries` exactly; `healpixTileCorners` returns a 16-vertex polygon following the curved pixel edge, padded 1 % outward, RA in [0, 2π). Default query nside is 64 (~0.84 deg² per tile); override with `--gaia-nside 256` (~0.052 deg²) when finer tiles are needed.
- **TAP client** — `gaiaQueryTile` posts an ADQL `POLYGON` query to `/sync` on the configured mirror. Per-attempt `context.WithTimeout(180s)`; 5 s heartbeat via `gaiaWaitWithProgress`; 3 attempts with 2 s / 4 s backoff on 5xx / connection error. Timeouts are fatal and not retried. Custom User-Agent; `MAXREC=2000`.
- **Error extraction** — `gaiaExtractTAPError` pulls VOTable `<INFO>` text out of a non-JSON body; JSON with no `data` array is an error, not an empty star list.
- **Disk cache** — `os.UserCacheDir()/skystacker/gaia_cache/healpix_n<nside>_p<ipix>.json` (on Linux `$XDG_CACHE_HOME` or `~/.cache`), atomic writes, `gaiaCacheVersion = 1` (distinct from `cacheVersion`). Overridable with `--gaia-cache-dir`. Gaia DR3 is frozen, so tiles never expire; the OS user cache is the right lifetime.
- **Rate limit** — 1 req/sec between TAP calls within a run.

## 5. Memory, Disk, and Time Estimation

Before processing:
- **Time**: `sigma: 1.03 * N^1.20 * MP`; `median: 0.88 * N^1.18 * MP`; `mean: 0.55 * N * MP`. CR2/CR3 inputs add ~12 s/frame for dcraw+ImageMagick. Post-processing adds a base of 15 s plus `5 × denoise-strength` and 3 s for spike overlay. Calibrated against the Orion (70 frames, 9 m 58 s) and M42 (152 frames, 25 m 22 s) reference runs.
- **Memory**: mean = `frameMB * 5`; median/sigma = `frameMB * 4 + tileBufMB` (tile buffer capped at 256 MB). Real peak = `1.5 × naive + 2000 MB` safety margin. Compared against `/proc/meminfo` `MemAvailable`.
- **Disk** (median/sigma only): `int64(frames) * frameBytes * 2 + 500 MB` headroom, compared against `syscall.Statfs(outputDir).Bavail * Bsize × 1.20`. Mean has no spill, no disk check. `--force` bypasses both guards.

## 6. Output Format
- **TIFF**: uncompressed, little-endian, RGB, IEEE float32, single strip, in `TIFF/`.
- **JPG**: 8-bit display previews, gamma 0.5 encoded, in the root output directory.
- **Raw work file**: `_work/stacked_mean.f32raw` (uint32 w, uint32 h, float32 RGB), consumed by `--post-only`.
- **Report**: `report.txt` with the original command line, resolved flags, per-frame diagnostics, crop explanation, exposure summary, and normalization/limiter notes.
- **Maps**: `stacked_rejection_map.jpg`, `stacked_weight_map.jpg` (registered run, single-channel, gamma 0.5); `stacked_unregistered_rejection_map.jpg`, `stacked_unregistered_weight_map.jpg` (full-frame, when registration succeeded); `stacked_coverage_map.jpg` (full-frame per-pixel registered-frame coverage fraction, linear-mapped). The rejection map is the cosmic-ray / hot-pixel candidate map: sigma clipping rejects extreme outliers.

## 7. Error Handling
- Corrupt files are soft-rejected during discovery and logged to `report.txt`; the run continues unless every file fails.
- Dimension mismatches fail fast with the conflicting groups named in the error.
- Output uses atomic writes (`.tmp` + `os.Rename`); failed writes never leave a partial output behind.
- `--post-only` verifies `_work/stacked_mean.f32raw` exists before starting.
- Memory and disk pre-flight checks abort before Pass 1; `--force` bypasses both.

## 8. Self-tests
`./skystacker --selftest` runs 209 embedded tests (plus a Go 1.26 `testing/synctest` rate-limiter test via `go test ./...`) covering accumulators, TIFF round-trip, decoding, discovery, similarity/affine transforms, star detection (uniform / noisy / nebula / edge / single-pixel), registration robustness with outliers, bilinear resampling round-trip, safe crop, weighted sigma clipping, tile reduction, chroma denoise, curves, diffraction spikes, HDR blend, guided filter, per-frame quality, norm stats (uniform / offset / identity / gradient / masked), normalization helpers, disk pre-flight, intrinsic-quality edge cases, frame limiter flags, report contents, intermediate map file naming (suffix + coverage map), star/transform cache round-trip and invalidation, HEALPix nested math (exact vs healpy at nside 1/16/64/256, round-trip), HEALPix tile corners vs healpy and polygon-covers-pixel (incl. polar caps), WCS fit with rotated/mirrored CD and RA wrap, Gaia star matcher stress (rotation, both parities, noise, distractors, RA wrap, unrelated-list rejection), plate solve end-to-end from a seeded cache (both parities), Gaia TAP retry + cache + ADQL + CLI parsing, and end-to-end pipeline (stack-only, post-only), plus FITS/XISF writers, config/resume, narrowband, deconvolution, annotation, native CR2 decoding and detector QA.

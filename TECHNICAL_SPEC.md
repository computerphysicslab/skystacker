# SkyStacker — Technical Specification

**Current version:** 0.5.0
**Implementation language:** Go (single-file, standard library only)
**Status:** Active Development

## 1. Purpose
SkyStacker is a local command-line program that discovers, aligns, normalizes, calibrates, and stacks astronomical images. It produces high-dynamic-range float32 TIFFs and a fully post-processed display image, using robust statistical stacking and PixInsight-style luminance stretching.

## 2. Scope

### Included
- CLI executable `skystacker`, single Go file, standard library only.
- Input formats: JPG/JPEG (native), CR2/CR3/FITS (via ImageMagick and dcraw at runtime).
- Star detection, triangle matching, RANSAC similarity transforms, optional 6-DOF affine refinement.
- Calibration frames (dark, flat, bias; individual files or auto-discovered folder trees via `--calib-root`).
- Stacking methods: mean (streaming), median (tiled), sigma-clipping (tiled, MAD-based, weighted).
- Optional per-frame normalization (global or per-tile local).
- Max/Min stacks (brightest/darkest pixels), optional TIFF sum output.
- 8-step post-processing pipeline (background, star removal, black point, STF/MTF auto-stretch, denoise, curves, HDR blend, weighted mix + optional diffraction spikes).
- Star/transform cache (`_work/cache/`) skipping Pass 1 detection and Pass 2 registration on re-runs.
- Memory and disk pre-flight checks; calibrated time estimates.
- 111 embedded self-tests.

### Excluded
- Native RAW decoding (delegated to ImageMagick / dcraw at runtime). FITS decoding is native with ImageMagick fallback.
- Drizzle, Bayer drizzle, plate-solving, SIP distortion models.
- GUI.

## 3. CLI Contract

```text
skystacker [flags] [input-directory]
```

Flags are accepted before or after the positional input directory. Default input directory: `.`. Default output directory: `<input>/0_skystacker_results`.

### Core flags
- `--method` (default `sigma`): `mean`, `median`, `sigma`.
- `--output` (default `<input>/0_skystacker_results`).
- `--workers` (default NumCPU).
- `--recursive`, `--overwrite`, `--force`.
- `--scale` (default 1; downscale factor).
- `--no-register`, `--stack-only`, `--post-only`.

### Calibration
- `--dark`, `--flat`, `--bias` (individual files or folders).
- `--calib-root` (auto-detect `Bias/Darks/Flats/<filter>` subfolders).

### Normalization (requires registration)
- `--normalize`: per-frame global scale+offset to the reference frame.
- `--local-normalize`: per-tile, bilinear-interpolated correction; implies `--normalize`.
- `--ln-tile-size` (default 256).

### Cache
- `--no-cache`: disable the star/transform cache (reads and writes).
- `--cache-dir` (default `<output>/_work/cache`).

### Stacking
- `--sigma-clip` (default 3.0), `--sigma-clip-iters` (default 2).
- `--tile-size` (default 0 = auto, capped at ~256 MB tile buffer).
- `--tiff`, `--no-exif`.

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
- `--selftest`, `--version`, `--help`.
- `--test-gaia RA,DEC` — one-shot Gaia DR3 tile query; prints nside, ipix, first 5 stars, exits.
- `--test-debayer PATH` — one-shot FITS debayer diagnostic; writes `<base>.jpg` and `<base>_raw.jpg`.

### Gaia DR3 data-access layer (no pipeline consumer yet)
- `--gaia-mirror URL` (default `https://gea.esac.esa.int/tap-server/tap`).
- `--gaia-cache-dir D` (default `<output>/_work/gaia_cache`).
- `--gaia-nside N` (default 256; power of 2, 2-8192).
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
- **Frame diagnostics**: per-frame stars, inliers, scale, rotation, offset, bounding box, `BoxPct`, `IntrQ`, `Weight`. Crop limiter flags: `LOW QUALITY` (< 30), `LOW BOX%` (< 50 %), `LARGE OFFSET` (`|drift| > 10 % refW/H`, where drift subtracts the rotation-induced expected offset so legitimately-rotated frames are not falsely flagged).

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

### 4.5 Gaia DR3 data-access layer (scaffolding)

No pipeline consumer yet; reachable only via `--test-gaia`.

- **HEALPix nested scheme** — `healpixAng2PixNested`, `healpixPix2AngNested`, `healpixTileCorners`. nside=1 matches healpy 1.16.5 exactly; nside>1 is round-trip-consistent but tile geometry is approximate (follow-up A1a.5).
- **TAP client** — `gaiaQueryTile` posts an ADQL `POLYGON` query to `/sync` on the configured mirror. Per-attempt `context.WithTimeout(180s)`; 5 s heartbeat via `gaiaWaitWithProgress`; 3 attempts with 2 s / 4 s backoff on 5xx / connection error. Timeouts are fatal and not retried. Custom User-Agent; `MAXREC=2000`.
- **Error extraction** — `gaiaExtractTAPError` pulls VOTable `<INFO>` text out of a non-JSON body; JSON with no `data` array is an error, not an empty star list.
- **Disk cache** — `<output>/_work/gaia_cache/healpix_n<nside>_p<ipix>.json`, atomic writes, `gaiaCacheVersion = 1` (distinct from `cacheVersion`). Gaia DR3 is frozen, so tiles never expire.
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
- **Report**: `report.txt` with per-frame diagnostics, crop explanation, exposure summary, and normalization/limiter notes.
- **Maps**: `stacked_rejection_map.jpg`, `stacked_weight_map.jpg` (registered run, single-channel, gamma 0.5); `stacked_unregistered_rejection_map.jpg`, `stacked_unregistered_weight_map.jpg` (full-frame, when registration succeeded); `stacked_coverage_map.jpg` (full-frame per-pixel registered-frame coverage fraction, linear-mapped). The rejection map is the cosmic-ray / hot-pixel candidate map: sigma clipping rejects extreme outliers.

## 7. Error Handling
- Corrupt files are soft-rejected during discovery and logged to `report.txt`; the run continues unless every file fails.
- Dimension mismatches fail fast with the conflicting groups named in the error.
- Output uses atomic writes (`.tmp` + `os.Rename`); failed writes never leave a partial output behind.
- `--post-only` verifies `_work/stacked_mean.f32raw` exists before starting.
- Memory and disk pre-flight checks abort before Pass 1; `--force` bypasses both.

## 8. Self-tests
`./skystacker --selftest` runs 111 tests covering accumulators, TIFF round-trip, decoding, discovery, similarity/affine transforms, star detection (uniform / noisy / nebula / edge / single-pixel), registration robustness with outliers, bilinear resampling round-trip, safe crop, weighted sigma clipping, tile reduction, chroma denoise, curves, diffraction spikes, HDR blend, guided filter, per-frame quality, norm stats (uniform / offset / identity / gradient / masked), normalization helpers, disk pre-flight, intrinsic-quality edge cases, frame limiter flags, report contents, intermediate map file naming (suffix + coverage map), star/transform cache round-trip and invalidation, HEALPix nested math (nside=1 exact vs healpy, nside=256 round-trip), Gaia TAP retry + cache + ADQL + CLI parsing, and end-to-end pipeline (stack-only, post-only).

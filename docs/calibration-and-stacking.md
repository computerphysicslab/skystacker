# Calibration and stacking

## Discovery

SkyStacker walks the input directory and accepts `jpg, jpeg, png, cr2, cr3,
fits, fit, ser, pgm, ppm, xisf`. Lights are grouped by scaled dimensions; a
folder mixing sizes fails fast with an actionable error. `--calib-root` detects
`Bias`, `Darks`, `Flats` (case-insensitive, plural accepted), otherwise
`--dark/--flat/--bias` are explicit.

## Calibration chain

1. Master bias/dark/flat construction with optional QA rejection.
2. `--black-level auto` subtracts EXIF `BlackLevel` / FITS `BLACK_LVL`.
3. `--bpm` or `--bpm-auto` masks bad pixels.
4. `--dark-scale-entropy` rescales the master dark per light.
5. `--self-flat` (grid estimator, default when no flats exist) or a real flat.
6. `--normalize` / `--local-normalize` background equalisation.

Master dimensions are checked against the lights before stacking: a master
with different native width/height is rejected instead of being resized.
`CALSTAT` (D/F/B/S) and dark `D_TEMP`/`D_EXP`/`D_GAIN` provenance are written to
`report.txt`, `report.json` and the stacked FITS headers.

`--export-masters` writes `master_bias.f32raw`, `master_dark.f32raw` and
`master_flat.f32raw` with JPEG previews. Synthetic bias/dark/flat generation is
explicitly refused; `--synthetic-calibration-report` explains what can and
cannot be recovered from lights alone.

## Registration and rejection

Star detection uses a high-pass/connected-component extractor with sub-pixel
centroids. Registration uses triangle descriptors and RANSAC similarity, with
optional affine refinement. `--keep-sharpest N` filters before accumulation.

| Method | Reducer |
|---|---|
| `mean` | Streaming weighted mean, lowest memory. |
| `median` | Out-of-core per-pixel median. |
| `sigma` | MAD-based sigma clipping with configurable asymmetry. |

`--rejection auto` maps stack size to Min/Max (≤4), Percentile (5–9), Averaged
Sigma (9–15), Winsorized (15–25) and Linear Fit (>25) where available.
`--tile-size` and `--workers` bound RAM and CPU.

## Frame weights

With registration enabled, each frame gets a PSF-based quality score combining
SNR, FWHM/HFR, eccentricity and background noise. `--psf-scale-snr` multiplies
that score by the relative scale from a PixInsight LocalNormalization
`<basename>.xnml` sidecar; missing sidecars produce zero weight. `--xnml-dir`
adds a fallback folder for sidecars written elsewhere. The per-frame scale is
reported as `psf_scale` in the CSV and `PSFScale` in `report.txt`/`report.json`.

## Self-flat and normalisation interaction

The grid self-flat and `--local-normalize` both touch the low-frequency
background. The report emits an explicit overlap warning with the measured
tile-median ratio when both are active. Validation on 16 OrionNebula lights
showed a +1.6% shift, not a runaway double division; compare against
`--no-local-normalize` before publishing a master.

## Real example

```bash
skystacker --method sigma --workers 8 --black-level auto \
  --frame-metrics metrics.csv --json-report \
  ./assets/M42-2017-11-18-Leciñena-50mm
```

Observed report values are quoted in [quickstart.md](quickstart.md). The CSV
header is:

```text
frame,stars,inliers,hfr_px,fwhm_px,eccentricity,snr,bg_sigma,
intrinsic_quality,weight,psf_scale,note
```

## Calibration status keywords

For compatibility with PixInsight/DeepSkyStacker, masters can be described with
D/F/B/S-style `CALSTAT` metadata and `D_temperature`, `D_exposure`, `D_gain`
keywords; width and height are never overridden. See
[research/04](../research/04-calibration-and-self-flat.md) §16.

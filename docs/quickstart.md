# Quick start

This page walks through the three most common ways to run SkyStacker. All paths
point to real datasets in this repository.

## 1. Stack a folder of lights

```bash
skystacker --method sigma --workers 8 \
  ./assets/M42-2017-11-18-Leciñena-50mm
```

The run discovers 11 JPEG lights, registers them, rejects outliers with MAD
sigma clipping and writes the products into
`assets/M42-2017-11-18-Leciñena-50mm/0_skystacker_results/`. A real report from
this dataset records:

```text
Discovered files: 11
Accepted:         11
Image dimensions: 2256 x 1504
Method:           sum and MAD-based sigma-clipped mean over registered frames
Registered frames: 11 / 11
Crop rectangle:   x=16..2248, y=33..1502 (2233x1470)
Total exposure:   330.000 s (5.50 min)
Camera:           Canon Canon EOS 450D
Aperture:         f/2.2
ISO:              800
Duration:         39.599s
```

Useful additions:

```bash
--json-report                 # machine-readable report.json
--frame-metrics metrics.csv   # per-frame HFR/FWHM/SNR/weight CSV
--include-list keep.txt       # keep only selected frames
--keep-sharpest 50            # keep sharpest 50% by median HFR
--calib-root ./calibration    # auto-detect darks/flats/biases
```

## 2. Solve the plate of one image

```bash
skystacker --process-image \
  "assets/M42-2017-11-18-Leciñena-50mm/0_skystacker_results/stacked_mean.jpg" \
  --image-op plate-solve --image-output /tmp/quick-plate \
  --plate-solver auto
```

The result is `/tmp/quick-plate/stacked_mean_plate_solution.json`. See
[plate-solving.md](plate-solving.md) for seeded, local, ASTAP and web options
with real observed outputs.

## 3. Re-run post-processing on an existing stack

```bash
skystacker --post-only --resume \
  --no-denoise --post-preset detail-last \
  ./assets/OrionNebula/0_skystacker_results
```

`--post-only` reads `_work/stacked_mean.f32raw` and regenerates the display
JPEGs without re-stacking.

## Where outputs go

| Product | Location |
|---|---|
| Scientific stack | `<output>/stacked_*.f32raw`, `.tif`, `.fits`, `.xisf` when requested |
| Display previews | `<output>/*.jpg` |
| Reports | `<output>/report.txt`, `report.json` with `--json-report` |
| Plate solution | `<output>/plate_solution.json` |
| Standalone image mode | `<image-output>/<basename>_<op>.<ext>` |
| Scratch/cache | `<output>/_work/`, `~/.cache/skystacker/` |

## Next steps

- [cli-reference.md](cli-reference.md) — all flags.
- [calibration-and-stacking.md](calibration-and-stacking.md) — masters, weights, rejection.
- [output-formats.md](output-formats.md) — exact file schemas.

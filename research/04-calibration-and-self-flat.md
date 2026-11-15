# 04 — Calibration master export and self-flat

**Date:** 2026-11-15
**Status:** implemented (Tier 2.2) and validated on the Orion-135mm JPEG set
**Code:** `main.go` — `exportCalibrationMasters`, `writeMasterPreviewJPEG`,
`selfFlatEligible`, `buildSelfFlat`, `normalizeSelfFlat`, `centerEdgeRatio`,
`flatResidualRMS`, `applyFlatToFrame`, `reduceTile`.

This note is append-only. New results go in new sections; earlier statements
are not rewritten.

## 1. Question

Two practical gaps remained after Tier 2.1 (`--bpm`):

1. There was no way to **inspect** the calibration products the pipeline
   actually loaded. A user who points `--dark/--flat/--bias` at a folder
   cannot see the master that was built, so a bad flat or a mis-scaled dark
   is invisible until the final image looks wrong.
2. The Orion-135mm dataset (66 lights) has **no flat frame**, yet the lights
   show a repeatable center-to-border illumination ratio of 1.24–1.25. The
   question was whether a usable *relative* flat can be recovered from the
   dithered lights themselves, without pretending that a synthetic flat is a
   real sensor flat.

## 2. Calibration master export (`--export-masters`)

For every non-nil master (bias, dark, flat, and the derived self-flat) the
pipeline writes two files:

| File | Content | Role |
| --- | --- | --- |
| `master_<kind>.f32raw` | 8-byte LE header (`width`, `height` uint32) + interleaved RGB float32 | scientific artifact |
| `master_<kind>.jpg` | 8-bit linear map of the 1st..99.5th luminance percentile to 0..255 | preview only |

The percentile window is computed over the whole master; if the window
collapses (hi ≤ lo) it falls back to min/max, and then to 0..1. The JPG is
never used for measurement. Per-master statistics (min, max, channel means)
are appended to `report.txt`.

The `.f32raw` format is deliberately the same as `_work/*.f32raw`, so the
existing `readRawFloat32RGB` and inspection scripts work unchanged. The
bias/dark/flat masters are full-frame; the self-flat is **crop-sized** (its
dimensions equal the safe crop), and the `report.txt` crop rectangle gives
its origin in the reference frame.

## 3. Self-flat derivation (`--self-flat`)

### 3.1 Why after registration

A sensor flat corrects pixel-to-pixel response and dust in *detector*
coordinates. A flat built from lights cannot do that: each light is
resampled into the reference frame, so fixed-pattern defects are smeared.
What survives alignment is the **relative illumination field** — vignetting
and large-scale throughput variation — expressed in reference coordinates.
The self-flat is therefore applied **after** registration, never through
`calibrate()`. Only the registered stack is corrected; the unregistered stack
is reported for reference and remains uncorrected.

### 3.2 Algorithm

Let \(L_i(x)\) be the aligned, calibrated light from frame \(i\) at reference
pixel \(x\), and \(Y = (R+G+B)/3\).

1. **Sky-level normalisation.** Compute each frame's median luminance
   \(m_i = \operatorname{median}_x Y(L_i(x))\) and the target
   \(\tilde m = \operatorname{median}_i m_i\). Scale
   \(s_i = \tilde m / m_i\).
2. **Reduction.** Reduce the scaled aligned frames with a one-iteration
   sigma-clipped mean. This rejects moving stars and cosmic rays. The result
   is \(F_0(x)\).
3. **Normalisation.** For each channel \(c\), divide by the channel median so
   the flat has median 1. Clamp to \([0.05, 20]\); NaN/Inf/non-positive
   values become 1.
4. **Sanity gate.** Compute
   \(R = \overline{F}_{\text{center}} / \overline{F}_{\text{edge}}\)
   (outer 10 % border vs a central 10 % box). Reject the flat if
   \(R < 0.5\) or \(R > 5\).
5. **Application.** Divide each aligned frame by \(F\) inside the safe crop
   before accumulation (`applyFlatToFrame` for `mean`, the `flat` argument of
   `reduceTile` for `median`/`sigma`).

Gate: at least **15 registered frames** (`selfFlatMinFrames = 15`) with spill
data. An explicit `--flat` (or a flat discovered through `--calib-root`)
always wins. Without registration `--self-flat` is refused.

### 3.3 Diagnostics

- `center/edge` — the ratio \(R\) above.
- `residual RMS` — \(\sqrt{\operatorname{mean}((F-1)^2)}\) over pixels with
  \(0.2 \le F \le 5\). It measures departure from unity, not goodness of fit;
  a large value means the map contains more than a smooth vignette.

## 4. Results — Orion-135mm

Command:

```
./skystacker assets/Orion-135mm --self-flat --export-masters \
  --stack-only --method mean --no-cache --output /tmp/orion_selfflat
```

66 lights, 2400×1600, crop 2395×1583.

| Quantity | Value |
| --- | --- |
| Registered frames | 66 |
| Self-flat center/edge | 1.2512 |
| Self-flat residual RMS | 0.19360 |
| Mean preview center/border, uncorrected | 1.134 |
| Mean preview center/border, self-flat | 1.000 |

The recovered center/edge (1.2512) agrees with the measured vignetting in the
lights (1.240–1.253), and the corrected mean is flat at the 1 % level. The
`--method sigma --limit 20` run reproduced both numbers (1.2497 and 1.000),
which exercises the `reduceTile` path rather than the `mean` re-accumulation
path.

## 5. Limits and interpretation

1. **Astrophysical bias is real.** The self-flat is built from the same
   photons as the science image. Extended structure common to every frame
   (M42 in this dataset) is not rejected by sigma clipping because it is not
   an outlier. The master preview consequently shows a center/border near
   1.9, well above the 1.25 vignetting ratio. The `.f32raw` is the honest
   artifact; the preview is not a flat-field quality metric.
2. **Not a sensor flat.** Dust shadows and pixel response do not align, so
   the self-flat cannot replace a real flat. It only removes the
   illumination gradient that survives registration.
3. **Gamma-encoded inputs.** The Orion lights are 8-bit sRGB JPEGs. Averaging
   and dividing in encoded space is not photometrically linear; the recovered
   ratios are internally consistent but not absolute throughput.
4. **`--local-normalize` overlap.** Local normalisation also removes
   large-scale background structure. Running it together with `--self-flat`
   can partially remove the same signal twice. Prefer one or the other until
   the interaction is measured.
5. **Pedestal.** The JPEG pipeline may add a display pedestal; that
   compresses the recovered ratio. The raw `.f32raw` and the report numbers
   avoid the pedestal.

## 6. Relation to real calibration data

Real calibration exists outside the repository and should be used to test the
*absolute* path (`--bias/--dark/--flat`), not the self-flat:

- `/home/jips/datos/Astro/datasets/OrionNebula/` — 134 CR2 lights, 22 darks,
  17 flats, 5 biases (Canon 40D). CR2 needs `dcraw -4 -T`.
- `/home/jips/datos/Astro/datasets/04_emission_nebula_Flame/` — 45 FITS
  lights, 2 darks, 1 flat, 1 bias. FITS is read natively.

The self-flat is the fallback for sessions with **no** flat; it is not a
substitute when flats exist.

## 7. Reproducibility checklist

- `gofmt -l main.go` → empty.
- `go vet ./...` → clean.
- `./skystacker --selftest` → `all 169 self-tests passed`.
- New tests: `testSelfFlatEligible`, `testSelfFlatRecovery` (synthetic
  vignetting + moving outliers, RMS < 0.03), `testExportCalibrationMasters`
  (6 files, 3 notes, vignetting/dust/hot pixel visible in previews).
- E2E outputs: `/tmp/orion_selfflat/` (mean) and
  `/tmp/orion_selfflat_sigma/` (sigma, limit 20). Never write into
  `assets/Orion-135mm/0_skystacker_results/`.

## 8. Open questions

- Does `--self-flat` interact measurably with `--local-normalize` on a
  gradient-free field? Needs a controlled synthetic run.
- Can the astrophysical bias be reduced by masking detected stars and
  high-surface-brightness regions before the reduction? This is the natural
  next experiment, not yet implemented.
- The residual RMS is currently a departure-from-unity metric. A radial
  goodness-of-fit metric would separate vignetting from astrophysical
  structure more cleanly.

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

## 9. Revision (2026-11-15): why the registered-space model overcorrects

The v1 implementation is documented in sections 1-8. It is useful as a
baseline but it is not a safe default on fields with extended emission.

The v1 flat is built from `spillPaths2`, i.e. from frames that have already
been resampled into the reference coordinate system. Aligned stars and
aligned nebulosity therefore occupy the same pixels in every frame. The
sigma-clipped mean sees them as a *common* component, exactly like
vignetting, and the normalised result is a map of "illumination plus the
static sky". Dividing the registered stack by that map suppresses real
structure and can produce dark halos around bright objects.

The effect is not literally "dividing the stack by itself": the flat is a
one-iteration sigma-clipped mean of the aligned frames, while the final
stack is a weighted/rejected combination, and the flat is normalised to
median 1. The correlation is nevertheless the problem. On Orion-135mm the
map reaches a centre/edge ratio of 1.2512 and a departure-from-unity RMS of
0.1936; around M42 it reaches values near 1.9 (`max 9.20992`). Most of that
is scene, not vignetting. The corrected stack therefore over-subtracts the
nebula.

Sigma clipping does not rescue the method. With the small dithers of this
dataset (~13 px) stars remain inside the same rejection neighbourhood; a
large nebula is present in every frame and cannot be rejected as a temporal
outlier at all. A Gaussian blur or a low-order polynomial applied to the
aligned mean has the same defect: it preserves the low-frequency power of
the scene. The estimator must see the scene *move* relative to the
illumination pattern, and it must explicitly exclude the scene.

The v1 map is also crop-sized and expressed in reference coordinates. That
is adequate for the current registered output but it is not a sensor model
and cannot be applied before resampling.

## 10. Proposed sensor-space robust estimator

Work on the calibrated but **unregistered** frames, i.e. the same data that
Pass 1 already has before `resampleToReference*`. Let `I_k(x,y,c)` be the
calibrated frame for exposure `k`, channel `c`, in sensor coordinates. The
observable is approximately

```text
I_k(x,y,c) = F(x,y,c) * S_k(x,y,c) + noise
```

where `F` is the multiplicative illumination (vignetting, transmission,
microlens response) and `S_k` is the astrophysical scene plus sky. The
scene moves with dithering and tracking; `F` is fixed to the sensor. The
goal is a smooth, positive, full-frame estimate of `F`, up to an arbitrary
multiplicative constant.

### 10.1 Per-frame normalisation

Normalise each frame per channel by a robust sky level, e.g. the median of
the unmasked pixels:

```text
I'_k(x,y,c) = I_k(x,y,c) / median_unmasked(I_k(.,.,c))
```

This removes exposure/transparency differences without removing the
spatial illumination pattern. Per-channel normalisation keeps a separate
scale per channel, so the relative flat is not forced to be achromatic. If
a frame is mostly masked, fall back to its global median and mark the frame
as low-quality for the flat.

### 10.2 Source mask

For each frame build a luminance mask and dilate it:

- luminance `L = 0.2126 R + 0.7152 G + 0.0722 B`;
- threshold `L > median(L) + 2.5 * MAD(L)` (the usual robust sigma);
- mask saturated pixels and known bad pixels as well;
- dilate the mask by 10-15 px so that PSF wings and blooming do not leak
  into the flat.

The mask is shared by the three channels. A large nebula will mask a large
fraction of some cells; cells with too few unmasked pixels are invalid, not
silently filled.

### 10.3 Coarse grid

Use square cells of 64 or 96 px. For a 2400x1600 frame, 64 px gives about
38x25 cells; 96 px gives about 25x17. Sixty-four pixels is a good default:
it resolves the vignetting scale while still giving hundreds of cells for a
degree-3/4 fit. A cell is valid only if it has enough unmasked samples
(e.g. at least 30% of its pixels in at least half of the frames).

For each cell, combine the unmasked normalised pixels from all frames (or,
better, average per-frame histograms with equal frame weights) and compute
a robust background. There are two useful statistics:

- the **SExtractor mode** `2.5 * median - 1.5 * mean` after asymmetric
  sigma clipping, or a smoothed histogram peak; this is the least biased
  estimate of the sky level when the sky dominates the cell;
- a **low quantile** (20-25%) of the pooled values; this is conservative
  against scene contamination but biased low by noise and by the scene's
  lower tail.

Recommendation: use the mode as the cell value and the low quantile as a
consistency diagnostic. If mode and low quantile disagree by more than a
few percent, the cell probably contains a second population (nebula, bright
star, dust) and should be rejected. A pure low-quantile estimator is
acceptable for a first prototype, but it will slightly overcorrect the flat
(i.e. over-brighten the corners) because noise makes the low tail too dark.

Alternative implementation, easier to reason about: compute the 20-25%
quantile *per frame per cell*, then take the median or 25% quantile across
frames. This is cheap (one float per frame per cell) and naturally handles
different masks per frame. It is a good Phase-1 prototype. A per-frame
histogram averaged with equal frame weights is the better production
estimator: it preserves the within-cell distribution without letting a
frame with a larger valid area dominate.

### 10.4 Grid outlier rejection

Apply a 5x5 median filter to the valid cell grid. Reject a cell when

```text
|B(x,y) - median_5x5(B)| > 2.5 * MAD_5x5(B)
```

where `B` is the cell background. This removes residual stars, nebula
cores, satellite trails and dust shadows that survived the pixel mask. Do
not replace rejected cells by interpolation before the fit; give them zero
weight and let the polynomial bridge the gap. Record the number of valid
and rejected cells in the report.

### 10.5 IRLS polynomial surface

Fit a low-degree 2D polynomial per channel. Chebyshev or Legendre
polynomials on `u,v in [-1,1]` are numerically stable:

```text
F(x,y,c) = sum_{p=0..d} sum_{q=0..d} a_pq,c * T_p(u) * T_q(v)
```

with degree `d = 3` or `4` (16 or 25 coefficients). Degree 4 is a good
starting point for a 38x25 grid; degree 5 starts to have enough freedom to
chase scene residuals. A radial polynomial in `r` with a centre and
ellipticity is even more constrained and is the preferred model when real
flats show a mostly radial vignetting pattern.

Fit with iteratively reweighted least squares:

```text
minimise sum_j w_j * rho( (B_j - F(u_j,v_j)) / sigma )
sigma = 1.4826 * MAD(residuals)
```

Use a Huber loss (or Tukey biweight) with an asymmetric scale if desired:
positive residuals are more likely to be scene contamination, negative
residuals more likely to be noise. Initialize with a degree-2 fit or a
radial model, then iterate 10-20 times. Use the cell's valid sample count
as an additional weight. Fit only the valid cell centres; evaluate the
polynomial on the full frame, but check the extrapolation at the corners
separately (a low-degree polynomial is usually well behaved, but a large
masked region can still produce an implausible corner).

After the fit:

- normalise per channel by the mean or the centre value (relative flat);
- clamp to a physically plausible range, e.g. `[0.1, 10]`;
- reject the model if the minimum/maximum are implausible (for example
  `min < 0.5` or `max > 2.0` on a normal lens) or if the fit RMS is large;
- record the coefficients, degree, cell size, valid/rejected counts, mask
  fraction, fit RMS, min/max and centre/edge ratio.

The polynomial is a strong prior: it cannot reproduce stars, dust or a
small nebula. That is exactly why it is preferable to a Gaussian blur or a
per-pixel aligned mean. It can, however, absorb a smooth sky gradient; see
section 11.

### 10.6 Full-frame output and application order

Write `master_self_flat.f32raw` at full sensor resolution, with the same
header as the other masters, and a preview `master_self_flat.jpg`. The
master is **not** crop-sized and is not tied to the reference frame.

Apply the flat to each calibrated frame *before* resampling in Pass 2:

```text
f.Pix /= F(x,y,c)   // sensor coordinates, full frame
resample(f)
```

This is the physically correct order: the illumination is fixed to the
sensor, and resampling the corrected frame preserves the correction. The
master is in the same encoding as the working frames, so this is the
encoded-space division; see the gamma bullet in section 11. The registered
stack then needs no flat-aware reduction code. For the
unregistered outputs, because `F` is constant across frames, dividing the
reduced sum/mean by `F` after reduction is mathematically equivalent and
much cheaper than correcting every frame again. The same applies to the
median and to the per-pixel max/min maps.

`stacked_no_correction_mean` remains intentionally uncorrected. The
unregistered mean/median/sigma outputs and the diagnostic max/min maps
should either be corrected consistently or be explicitly labelled as
uncorrected; the former is less surprising.

### 10.7 Streaming integration

The best production integration does not need `spillPaths1` at all:

1. Before Pass 1, allocate a `selfFlatAccumulator` (per-cell histograms or
   per-frame cell quantiles) when `--self-flat` is active.
2. In the Pass 1 loop, after `calibrate`, black-level subtraction and BPM,
   build the source mask and add the frame's unmasked normalised pixels to
   the accumulator. This is a single streaming pass and needs a few MB of
   state, not a second full-frame copy.
3. After Pass 1 (or after registration, since registration is independent),
   finalise the cell grid, fit the polynomial and validate it.
4. In Pass 2, apply the full-frame flat after calibration/BPM and before
   resampling.

This removes the current mean-path re-accumulation, the `flatCorrection`
type and the flat parameters in `reduceTilesStack`/`reduceTile`. It also
avoids forcing `spillPaths1` for the `mean` method. A Phase-1 prototype may
still read `spillPaths1` for convenience, but the production design should
stream in Pass 1.

## 11. Identifiability, degeneracies and physical limits

These limits are not implementation bugs; they are properties of the
single-session self-flat problem and must be documented for users.

- **Static scene vs. flat.** In sensor coordinates the scene moves only by
  the dither. A nebula hundreds of pixels wide is effectively static when
  the dither is ~13 px. The estimator cannot separate its smooth envelope
  from `F` without masking it. The mask, the small number of polynomial
  coefficients and the cell rejection are mitigations, not a proof.
- **Sky gradient vs. flat.** A stable spatial sky gradient (light
  pollution, Moon, twilight) is multiplicative with `F` in the model above.
  A 2D polynomial will absorb it. No single-night method can separate the
  two without a prior. A radial physical model is more identifiable than a
  general 2D polynomial because it cannot absorb an arbitrary linear
  gradient; consider fitting radial first and a mild 2D residual second.
  Report this caveat in every self-flat run.
- **Dust shadows.** Dust is high-frequency multiplicative structure. A
  degree-3/4 polynomial cannot represent it, and the 5x5 cell rejection
  will remove it rather than correct it. Real flats are required. Large
  dithers plus a min/max rejection can reduce dust in the stack, but the
  current ~13 px dithers are not enough. Out of scope for the self-flat.
- **Gamma.** The current pipeline operates on sRGB-encoded values. The
  flat is multiplicative in linear light. If the flat is estimated and
  applied consistently in encoded space, the first-order multiplicative
  factor cancels, but the sRGB transfer function is nonlinear (and has a
  linear toe), so the correction is not exact; the residual depends on the
  sky level and on the vignetting strength. For mild vignetting
  (`F ~ 0.8`) the error is small; for strong vignetting it grows. Options:
  (a) gamma-aware division using the sRGB EOTF, or (b) a later
  linear-light pipeline. Option (a) is local and testable; option (b) is a
  larger change to normalisation, sigma clipping and outputs. Do not
  silently mix the two.
- **Colour.** Vignetting and transmission are wavelength-dependent. Fit
  and normalise per channel, with a shared mask. A luminance-only flat is
  cheaper but can leave colour-dependent corners.
- **Noise.** The low quantile is biased low by noise. More frames and
  larger cells reduce the bias; the robust mode is less biased than a pure
  quantile. Report the fit RMS and the number of samples per cell.
- **Dithering.** The method needs relative motion between scene and
  sensor. A few pixels of dithering plus a source mask can still work for
  stars, but not for a large nebula. Compute the median dither translation
  and warn when it is very small; do not make it a hard gate without
  evidence.
- **Registration requirement.** The flat itself does not need
  registration: it is built in sensor coordinates. The v1 gate that
  requires registration is a property of the v1 application order, not of
  the estimator. After the integration work, `--self-flat` should also
  correct the unregistered stack; keep the gate only until that path is
  tested.

## 12. Integration map (v1 code to the sensor-space pipeline)

Line numbers are approximate; function names are the stable reference.

1. **Pass 1 spill / accumulation** (`main.go` around 5878 and 5951).
   v1 spills `spillPaths1` only for `median`/`sigma`. The sensor-space
   estimator can either stream in Pass 1 (preferred, section 10.7) or force
   `spillDir1` when `--self-flat` is active so the `mean` path also has
   unregistered frames. Do not use `spillPaths2` for the flat.
2. **Build the flat before Pass 2** (replace `buildSelfFlat`, around
   5113). The flat must be known before the resampling loop so it can be
   applied to each frame. v1 builds it after Pass 2 because it needs
   `spillPaths2`; that ordering is incompatible with pre-resampling
   correction. Use `refW x refH`, no `cropRect`.
3. **Apply before resampling** (Pass 2, after `calibrate`/BPM, around
   6326). Replace the current post-registration `applyFlatToFrame` with a
   full-frame sensor correction, then call `resampleToReference*`.
4. **Remove the self-flat special case in the registered mean path**
   (around 6298, 6351 and 6590). With the flat known before Pass 2 there
   is no need to spill `spillPaths2`, collect `spillMedians`, derive the
   flat after accumulation, or re-accumulate the mean. Accumulate the
   corrected resampled frames directly.
5. **Unregistered outputs** (around 6620). Correct the unregistered mean
   after `divideByCount`, and the median/sigma `unregMeanPix`/`unregSumPix`
   after `reduceTilesStack`, by dividing by the full-frame flat. Correct
   the max/min maps too, or label them explicitly.
6. **Reduce the reduction API** (around 4621, 4718, 4826). After the flat
   is applied before resampling, `flatCorrection` and the `flat` parameter
   of `reduceTilesStack`/`reduceTile` are unused; remove them. The
   `frameScales` parameter was added for the v1 self-flat reduction and is
   unused by the main calls; remove it as well, or document why it stays.
7. **Result and report** (`selfFlatResult` around 5067, report around
   3074/3150/7071). Add `Model`, `Degree`, `CellSize`, `ValidCells`,
   `RejectedCells`, `MaskedFraction`, `FitRMS`, `Min`, `Max` and
   `DitherMedian`. Keep `CenterEdge` and `ResidualRMS`, but rename the
   meaning of `ResidualRMS` to "fit residual" rather than "departure from
   unity". Emit the model and coefficients in `report.txt`.
8. **Flags.** Keep `--self-flat`. Add a temporary `--self-flat-model`
   (`grid` or `registered`) for A/B testing, defaulting to `registered`
   until the new model passes the acceptance tests, then to `grid`.
   `--self-flat-degree` (default 4) and `--self-flat-cell` (default 64)
   are useful for validation; quantile/loss can stay internal in v2.
   Document planned flags only after they exist.
9. **Eligibility** (`selfFlatEligible`, around 5078). Keep the real-flat
   precedence and the 15-frame minimum. Remove the registration gate only
   when the unregistered application path is implemented and tested. Add a
   dithering diagnostic.
10. **Master and docs.** The master becomes full-frame; update README,
    TECHNICAL_SPEC, prompt-ai and the v1 note in this document. The
    `--export-masters` path is unchanged.

## 13. Validation and acceptance criteria

Synthetic tests are the only way to separate "looks flatter" from
"recovered the illumination". Each test should generate a known flat
`F_true` and a scene with moving stars and a large static nebula, then
check the recovered flat, not only the output image.

Required synthetic cases:

- **Known vignetting.** `F_true = 1 - a r^2 - b r^4` with a realistic
  centre/edge ratio (1.2-1.4). Recovered flat RMS < 0.02 in the valid
  region, and centre/edge within 0.02 of truth.
- **Moving stars.** Stars at different sensor positions in every frame.
  The recovered flat must not contain star-shaped residuals; compare with
  and without the source mask.
- **Large nebula.** A Gaussian or M42-like extended source that is
  effectively static (dither << source size). The recovered flat at the
  nebula location must stay within 0.02 of `F_true`; the test fails if the
  fit absorbs the nebula. This is the regression test for the v1 failure.
- **Sky gradient.** A smooth additive/multiplicative gradient. Assert that
  the radial model does not absorb it; for the 2D polynomial, assert that
  the warning/report is emitted. This test documents the degeneracy rather
  than pretending to solve it.
- **Grid rejection.** A bright patch and a dust shadow; assert that the
  affected cells are rejected and the surrounding fit is unbiased.
- **IRLS.** Add outliers to the cell grid; compare Huber/Tukey with
  ordinary least squares; assert the robust fit is closer to truth.
- **Application order.** A synthetic translation/rotation: applying the
  flat before resampling must recover the same illumination as applying it
  in sensor space; applying it after resampling must be measurably worse.
- **Methods and registration.** Run `mean`, `median` and `sigma`, with and
  without `--no-register`; assert the corrected outputs are consistent and
  the unregistered stack is corrected.
- **Gamma.** If gamma-aware division is implemented, a synthetic sRGB
  vignette must flatten within 1%; naive division must fail the tighter
  bound. If not implemented, assert the report warns about encoded space.
- **Master round-trip.** Full-frame dimensions, header, preview, and
  `report.txt` diagnostics; `--export-masters` still writes three notes.

Real-data acceptance (outside the repo):

- Compare the derived flat with the real flat from `OrionNebula` (17 flats)
  or `04_emission_nebula_Flame` (1 flat) after resampling to the same
  sensor geometry. Expect agreement of the smooth component within 5-10%
  in the valid region; differences at dust scales are expected and should
  be reported, not hidden.
- Orion-135mm (no real flat): the M42-region ratio must no longer reach
  ~1.9; the map should be smooth, monotonic-ish and within the physically
  plausible range. The corrected stack must not show a dark halo around
  M42. This is a visual regression in addition to the numeric tests.
- Re-run the section-E calibration rejection tests unchanged; do not
  weaken their thresholds to make the new self-flat pass.

## 14. Roadmap

The work is deliberately staged so that each phase is independently
testable and the v1 path remains available for A/B.

- **Phase 0 (done in this revision): analysis and guardrails.**
  Document the v1 failure, the sensor-space design, the degeneracies and
  this roadmap. No code change; test count stays 169.
- **Phase 1: sensor-space prototype.** Build a full-frame flat from
  unregistered frames (streaming or `spillPaths1`), per-frame median
  normalisation, coarse cells and a per-frame/per-cell low quantile; fit a
  plain least-squares polynomial; apply before resampling; correct the
  registered stack. Keep `--self-flat-model=registered` for comparison.
  Tests: known vignetting + moving stars; recovery within 0.05; no
  registration requirement for the build. Acceptance: the M42 dome is
  gone, the map is full-frame, and the output is at least as flat as v1.
- **Phase 2: robust masking and cell rejection.** Add the source mask and
  dilation, the SExtractor mode, the 5x5 median/MAD cell rejection and the
  mode/quantile consistency check. Tests: large nebula, bright patch, dust
  shadow; the flat at the nebula location stays within 0.02 of truth.
- **Phase 3: IRLS polynomial.** Chebyshev/Legendre degree 3-4 with
  Huber/Tukey and asymmetric weighting; validity weights; positivity and
  extrapolation checks; fit diagnostics. Compare with a radial model.
  Tests: outliers, irregular masks, edge extrapolation; fit RMS < 0.02;
  robust fit beats OLS.
- **Phase 4: integration and cleanup.** Remove `flatCorrection`, the
  `flat`/`frameScales` reduction parameters and the mean re-accumulation;
  apply the flat before resampling; correct the unregistered mean/median/
  sigma and max/min outputs; relax the registration gate and test
  `--no-register`; update `report.txt`, README, TECHNICAL_SPEC and
  prompt-ai. E2E Orion mean/sigma. Switch the default to `grid` only after
  the acceptance tests pass.
- **Phase 5: gamma-aware correction.** Add an sRGB EOTF LUT and correct
  the flat division in linear light, initially behind a flag, then decide
  the default from the synthetic and real comparisons. Do not mix this
  with Phase 1-4; it is an independent axis.
- **Phase 6: real-flat validation.** Compare with the OrionNebula and
  Flame flats; quantify the smooth agreement and the dust residual; write
  the results into this document. This is the scientific acceptance gate
  for calling the estimator "validated".
- **Phase 7 (optional): dust/defect handling.** Out of scope for a
  polynomial self-flat. Requires real flats or a dedicated high-frequency
  residual model. Document as future work, do not promise it in the flag
  description.

Priority: Phases 1-4 replace the unsafe default; Phases 5-6 establish
scientific quality; Phase 7 is explicitly out of scope. Each phase must
keep `gofmt`/`go vet` clean, keep the section-E tests intact, and update
the self-test count and `PROJECT_CONTEXT.md` when tests are added.

## 15. Phase 1 implementation (2026-11-15)

`--self-flat-model=grid` is implemented. The default remains `registered`.

What shipped:

- A streaming `selfFlatGridAccumulator` filled in Pass 1 after calibration,
  black-level subtraction and BPM. It does not read `spillPaths2` and does
  not force an extra spill for `mean`.
- Per frame: per-channel median of unmasked pixels, luminance mask at
  `median + 2.5*MAD`, saturation mask, and a 12 px box dilation (scaled
  down on small frames).
- 64 px cells, shrunk only when a frame would otherwise have fewer than
  six cells on a side. Per cell: 25th percentile of unmasked normalised
  pixels, requiring 30% of the cell. Across frames: median of those
  quantiles, requiring 15 valid frames.
- Degree-4 Chebyshev least squares per channel, weighted by valid sample
  counts, evaluated on the full sensor grid. Normalised per channel,
  clamped to `[0.1, 10]`.
- Applied after calibration/BPM and before resampling. Unregistered
  median/sigma results are post-divided. The unregistered mean output is
  post-divided on a copy so `stacked_no_correction_mean` stays uncorrected.
- v1 is unchanged behind `--self-flat-model=registered`.

Tests added: `testSelfFlatGridRecovery` (RMS < 0.05), 
`testSelfFlatGridApplication`, `testSelfFlatGridNoRegistration`,
`testSelfFlatModelDefault`. Self-test count: **173**.

Orion-135mm smoke run (`--method mean --stack-only --limit 20
--self-flat-model=grid`, `/tmp/orion_grid`, not the repo output):

- full-frame master 2400x1600
- center/edge 1.2449 (matches the known ~1.24 vignetting)
- fit RMS 0.01337
- min 0.4855, max 1.2081 (the v1 M42 peak near 1.9 / raw max 9.2 is gone)
- 391/950 valid cells, mask fraction 67.6%

The mask fraction is too high for a finished estimator. Phase 2 should
relax the source mask on extended emission and add the 5x5 cell rejection
before this model becomes the default. The low corner (0.49) is also a
reminder that degree-4 extrapolation over masked regions needs the Phase 3
robust fit and the radial-model comparison.

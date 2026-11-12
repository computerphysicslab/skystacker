# SkyStacker — project context (last updated 2026-11-11)

## Version
v0.5.0 (single-file Go program, `main.go`)

## Build, test, install
```
gofmt -w main.go && go build -o skystacker main.go && ./skystacker --selftest
go install .
```

**Self-test count: 91 passing.** Update this number after any test change.

## Post-processing pipeline (8 steps)

1. Background removal (polynomial degree 1-3)
2. Star removal (high-pass + median annulus replacement)
3. Black-point shift (intermediate; not written to disk)
4. STF/MTF auto-stretch (PixInsight-style)
5. Guided-filter denoise (iterative, 1-10 passes, chroma-selective)
6. **Curves** — `x' = clamp((x-black)/(1-black), 0, 1)`, `out = x'^gamma`
7. HDR blend (step 6 base + linear step 1 detail)
8. Weighted mix — 4 variants of `w*step1 + (1-w)*step7`, `w in {0.90, 0.80, 0.70, 0.60}`
   Optional diffraction-spike overlay (ON by default)

Outputs: `post_step1_background_removed.jpg` … `post_step8_4.jpg`.
`post_step3` intentionally not written (visually identical to step 2).

## Registration / stacking pipeline

- Discovery + decode (JPG, CR2/CR3 via dcraw+ImageMagick, FITS via ImageMagick)
- Optional calibration: `--dark`, `--flat`, `--bias`, or `--calib-root` auto-detect
- Reference frame = frame with the most detected stars
- **Adaptive high-pass radius**: `clamp(min(W,H)/200, 8, 24)`. Was fixed 12;
  scales with image size so the same threshold works on 500 px previews and
  6000 px full-frame stacks.
- **Multi-sigma retry**: 5σ first; if < 20 stars detected, retry at 4.0, 3.5,
  3.0 sigma and take the best result.
- Triangle-invariant star matching using top **40** brightest stars (was 30;
  40^3/30^3 ≈ 2.4x more triangle descriptors; RANSAC downstream unaffected).
- RANSAC similarity estimation (rotation + uniform scale + translation).
- **Optional full 6-DOF affine refinement** — kept only when residual drops
  ≥ 10%; requires ≥ 15 inliers. Handles field rotation, anisotropic scale,
  and shear that similarity cannot represent. Frame table in `report.txt`
  shows `[affine]` for refined frames.
- Bilinear sub-pixel resampling; safe crop to the largest fully-valid region.
- Stacking methods: mean (streaming), median, sigma (tiled, MAD rejection,
  out-of-core via disk spill). `--scale N` downsamples by N.

- **Progress heartbeat**: `startProgress(label, interval)` spawns a
  goroutine logging `[label] still running, Ns elapsed` every 15 s.
  Wraps the tiled reduction. **Per-tile progress**: `reduceTilesStack`
  logs `reduce: N/M tiles (P%, Ss elapsed)` every `max(M/20, 5)` tiles.
  Together these eliminate the "is it stuck?" question on long sigmas.

- **Frame quality metrics** (computed once per frame in pass 1):
  - `medianBlob` — median `Size` of detected stars; proxy for FWHM.
  - `bgNoise` — MAD background sigma of the luminance channel.
  - `intrinsicQ` — 0-100 score: star count (0-40) + blob size (0-30)
    + background noise (0-30). Independent of registration, so it also
    scores frames that failed to register.

- **Weighted sigma-clipped mean** (`sigmaClipReduceWeighted`):
  - Registered stack: `weight = computeFrameQuality × intrinsicQ / 100`.
  - Unregistered stack: `weight = intrinsicQ / 100`.
  - `--no-register` and `--method median` still use equal weights
    (median is not yet weighted — see Pending).
  - Zero-weight survivors → fallback to unweighted mean (avoids divide-by-zero).
  - `report.txt` frame-diagnostics table shows `IntrQ` (0-100) and
    `Weight` (0-1) columns.

## CLI flags relevant to current work

### Registration (implicit, no new flags)
Automatic. Disable with `--no-register`.

### Curves (step 6)
- `--curves-black F` (default 0.03) — shadow crush point, [0, 0.5)
- `--curves-gamma F` (default 1.20) — midtone darkening, [0.1, 4.0]
- `--no-curves` — skip step entirely

### Diffraction spikes (step 8 overlay, ON by default)
- `--spikes` (default **true**)
- `--no-spikes` — explicit opt-out
- `--spikes-percent F` (default **0.2**)
- `--spikes-length F` (default **10**) — brightest star's arm half-length in px
- `--spikes-intensity F` (default **0.3**) — brightest star's peak additive brightness
- `--spikes-arms N` (default **4**; 4=cross, 6=hexagonal)
- `--spikes-softness F` (default **0.5**) — intensity falloff exponent
- `--spikes-min-ratio F` (default **0.2**) — intensity floor for faintest spiked star
- `--spikes-length-softness F` (default **1.0**) — length falloff exponent
- `--spikes-length-min-ratio F` (default **0.03**) — length floor for faintest spiked star

Two independent power laws:

```
intensityWeight = (flux/maxFlux)^intensitySoftness, floored at intensityMinRatio
lengthWeight    = (flux/maxFlux)^lengthSoftness,    floored at lengthMinRatio
```

`lengthSoftness=1.0` is strictly proportional. `intensitySoftness=0.5` compresses
the intensity range so faint spikes remain visible rather than cutting off hard.

At defaults (baseLength=100, baseIntensity=0.7):

| relFlux | length  | intensity |
|---------|---------|-----------|
| 100%    | 100 px  | 0.70      |
| 50%     |  50 px  | 0.49      |
| 25%     |  25 px  | 0.35      |
| 10%     |  10 px  | 0.22      |
| 1%      |   1 px  | 0.06      |

(The 1% intensity is 0.14, not 0.20: `intensityWeight` is floored at 0.2,
then multiplied by `baseIntensity=0.7`, so 0.7 × 0.2 = 0.14.)

Base angle: one random angle per image (seeded from `time.Now().UnixNano()`),
same for all stars. Arms at `baseAngle + k*2*pi/numArms`.

Additive only (never darkens); Gaussian cross-section σ=1.5 px along arms.
Stars detected **once** on `step7Frame`, reused for all 4 variants, so
`post_step8_1..4` spike the same stars at the same angles. `post_step1` …
`post_step7` are untouched.

## EXIF metadata (total exposure in report)

Enabled by default. Reads EXIF from every accepted JPG frame during
pass 1 and:

- Sums `ExposureTime` across all frames that have EXIF metadata → total
  exposure in seconds.
- Adds an `Exposure summary:` block to `report.txt`:
  - frames with EXIF (e.g. `70 / 70`)
  - total exposure (`1050.000 s (17.50 min)`)
  - average per frame
  - camera make + model
  - aperture (f-number)
  - ISO
  - reference frame's `DateTimeOriginal`

The block only appears when the run actually found EXIF data. Runs on
files without EXIF metadata produce the same report as before this
feature.

**Disable with `--no-exif`** — skips extraction entirely; the report
simply omits the exposure summary.

**Scope limitations** (documented for future work):

- **Only JPG inputs are parsed.** RAW (CR2/CR3) and FITS would need
  dedicated decoders. The current pipeline's JPG path covers the common
  case where total-exposure metadata matters.
- **No EXIF is embedded into the output JPEGs.** A future patch could
  inject an aggregate `ExposureTime` tag into each written JPEG, but
  that requires building a full APP1 payload and would need a
  dedicated self-test to verify all three output JPEG helpers
  (`writePreviewJPEGScaled`, `writeLinearPreviewJPEG`,
  `writeAutoNormalizedLinearPreviewJPEG`) produce valid output.
  Deferred to keep this patch small and focused.

## Known constraints

- Registration requires ≥ 8 inliers and ratio ≥ 0.20.
- Crop is the largest fully-valid rectangle (every pixel valid in ≥ N frames).
- Affine refinement requires ≥ 15 inliers (won't fire on sparse fields).
- Weighted sigma needs >= 3 surviving values per pixel per channel; if
  every surviving weight is zero it falls back to the unweighted mean of
  the survivors.
- Median method ignores the per-frame weights (see Pending: Weighted median).
- **Field distortion (radial/tangential) is NOT corrected.** Affine handles
  translation, rotation, uniform + anisotropic scale, shear — but not true
  optical distortion. SIP / plate-solving is future work.
- Multi-σ retry may run up to 4× detection cost on frames that never reach
  20 stars at 5σ; typical frames pay 1×.

- Disk pre-flight only checks the output filesystem. If --output points to a different filesystem than the spill dirs (currently they share one), the check would be wrong — but they always share, so this is currently correct.

- Normalization requires registration and the reference frame's background to be measurable. Both are opt-in (`--normalize` / `--local-normalize`). Global scale is clamped to [0.5, 2.0] to prevent runaway corrections on nearly-uniform tiles.

## Recent decisions / caveats

- **Tolerance-based float comparison APPLIED**: Tests that compare floating-point
  values (notably `testChromaDenoiseMultiScale` and `testApplyCurves`) now use
  `math.Abs(float64(got-want)) > 1e-4` instead of exact equality. This fixes
  CI failures caused by last-digit variations across platforms (e.g.,
  `0.41581752` vs `0.41581762`). The epsilon `1e-4` absorbs hardware/compiler
  differences without masking real regressions.
- **Time-estimate calibration APPLIED** (was pending): the ETA at run
  start now uses `sigma: 1.03 * N^1.20 * MP`, `median: 0.88 * N^1.18 * MP`,
  `mean: 0.55 * N^1.00 * MP`. Both real runs (Orion 70 fr, M42 152 fr)
  predicted within 2.5% of actual. Confirmed present in `main.go`.
- Curves applied identically per channel to preserve hue.
- Step 7 uses the curves output (step 6) as its display base.
- Affine refinement per-frame optional: similarity first, affine only when
  residual drops ≥ 10%. `[affine]` tag in `report.txt` for refined frames.
- Adaptive HP radius helps star detection across image scales without
  regression on either small previews or full-frame stacks.
- Triangle pool 30 → 40 costs ~2.4× more triangle descriptors in pass 1;
  RANSAC downstream unaffected.
- **Spikes ON by default** since v0.5.x. `--no-spikes` to disable.
- Chroma denoise multi-scale test uses tolerance-based float comparison (exact `>=` on stddev could flip on other hardware).
- **Progress heartbeat APPLIED**: `startProgress` helper + per-tile
  progress logging inside `reduceTilesStack`. Long sigmas no longer
  look hung.
- **Weighted quality stacking APPLIED**: registered frames weighted by
  `computeFrameQuality × computeIntrinsicQuality / 100`, unregistered
  by `computeIntrinsicQuality`. `report.txt` surfaces `IntrQ` and
  `Weight`. Unweighted path still reachable via `--no-register` or
  `--method median`. Frame weights are 0..1, so a perfect frame from
  a perfect fit contributes 1.0 and a bad frame contributes ~0.
- Length/intensity falloffs are **decoupled**: a single softness compressed
  both, making lengths look fixed. Now intensity uses soft falloff (0.5)
  and length uses proportional (1.0). This is what makes the visual
  hierarchy of the frame read correctly.
- Backups: check with `ls main.go.before_*.bak` (created by each patch).
  Expect to see files like `main.go.before_curves_step.bak`,
  `main.go.before_affine_step.bak`, `main.go.before_spikes_step.bak`,
  `main.go.before_spike_length.bak`, etc. — depending on which patches
  were actually applied.

- **Disk-space pre-flight APPLIED**: runPipeline checks syscall.Statfs on the output filesystem before Pass 1; aborts with INSUFFICIENT DISK SPACE if free < estimate × 1.20; --force overrides. Estimate: `int64(frames) × frameBytes × 2 + 500 MB` for median/sigma, 0 for mean.

- **Normalization APPLIED**: opt-in `--normalize` (global scale+offset per frame) and `--local-normalize` (per-tile, bilinear interp, `--ln-tile-size 256` default). Both require registration; reference frame is not normalized. Uses robust (median, MAD) statistics: scale = ref.MAD / frame.MAD (clamped to [0.5, 2.0]), offset = ref.median - scale*frame.median.

## Real-world validation

### Run 1 (2026-11-11): Orion-135mm, 70 × JPG, 2400×1600

- Registration: 70/70 matched, 72–94 inliers, scale ~1.001, rot ≤ 0.07°
- Affine refinement: **0 frames** (similarity was already sufficient)
- Crop: 2397×1355 (99.87% W, 84.7% H) — loss is real Y drift (~245 px),
  not registration error
- Auto-stretch: aggressive (m ≈ 0.015–0.021, c0 = 0); unlinked per-channel
- Runtime: **9 m 58 s**, peak memory 646 MB, 6.1 GB disk spill (sigma)
- ETA printed at start: 4 m 52 s → **2.05× low** (see Pending below)
- Post-processing: all 8 steps produced outputs
- Spikes: n/a (spikes not enabled at time of run)

### Run 2 (2026-11-11): M42, 152 × JPG, 2400×1600

- Registration: 152/152 matched, 45–72 inliers, rot drift 0.00 → 0.56°
- Affine refinement: **13 frames** used it (10.4–17.8% residual improvement).
  First real-data validation of the affine feature.
- Crop: 1723×1375 (62% of full frame) — **suspicious**. Rotation ≤ 0.56° can
  only explain ~14 px of corner displacement; the 677 px left-side loss is
  pure translation drift. `report.txt` frame table identifies the culprit
  via the `LOW BOX%` row. Two paths: remove the offending frame and re-run,
  or accept `--no-register` (full 2400×1600, slight edge smear).
- Auto-stretch: very aggressive (m ≈ 0.005–0.011, c0 = 0) — M42 is bright
- Runtime: **25 m 22 s**, peak memory 646 MB, 13.4 GB disk spill
- ETA printed at start: 10 m 23 s → **2.44× low**
- **Spikes: 1 star selected** at `--spikes-percent 0.2`. Math: 626 stars
  detected × 0.002 = 1.25 → 1. This motivated `--spikes-min-count` (Pending).

## Pending patches

Designed and validated but **not yet applied to the repo** as of the last
commit. A future session should either apply them or document why abandoned.

### Weighted median (future work)

**Problem**: `--method median` still produces a plain (unweighted) median.
That is a documented limitation of the weighting patch, not a bug — weighted
medians need a different algorithm (sort by value, walk CDF until cumulative
weight >= 0.5) and different rejection criteria, and are not yet worth the
complexity.

**Fix** (future work): implement a weighted median and wire it into the
`case "median":` branch of `reduceTile`.

### `--spikes-min-count` (fixes sparse-field collapse)

**Problem**: `--spikes-percent 0.2` selects 0 or 1 stars on fields with few
detected stars. M42 got 1 star (626 × 0.002 = 1.25 → 1). Sparse fields
end up with almost no spiked output.

**Fix**: `--spikes-min-count N` (default 5), a hard floor on stars selected.
Floors only when `int(percent × detected) < N`; dense fields unaffected.

Patch is written and self-tested but **failed to apply** on
`\tSpikesLengthMinRatio  float64` — anchor has two spaces, but the actual
file has one (gofmt collapsed it in an earlier run). To apply: re-anchor on
`\tSpikesLengthMinRatio float64` (single space) or add the field manually.

### Crop diagnostic for pure-translation drift

**Problem**: `LOW BOX%` fires only when BoxPct < 50%. A frame with 30%
translation but a mostly-full bounding box passes and goes unnamed, even
when it's dragging the crop (M42 case).

**Fix** (future work): add `LARGE OFFSET` flag when `|Offset| > 10% of
frame dimension`, independent of BoxPct.

## Future work (not implemented)

- **Field distortion correction** (SIP model or plate-solve fallback).
  Affine cannot correct radial or tangential distortion.
- **`--print-estimate-only` flag** that prints the ETA without running
  any pipeline stages; useful for validating estimate formulas.
- **Higher-order transformations** (polynomial, thin-plate spline) as
  alternatives to affine for extreme wide-field.
- **Spatial stratification** of the star-selection pool (brightest per
  grid cell) to prevent bright nebulosity from monopolising top-N.
- **Per-method calibrated coefficients** from a real benchmark, not
  real user runs (which mix in disk speed, thermal throttling, noise).

## Recovery / one-shot context

- Repo: https://github.com/computerphysicslab/skystacker
- Build: `gofmt -w main.go && go build -o skystacker main.go`
- Test: `./skystacker --selftest` (expect: **all 91 self-tests passed**)
- Post-only re-run: `skystacker --post-only --overwrite <dir>`
- Full re-run: `skystacker --overwrite <dir>`
- Sample data layouts (from validated runs):
  - `/media/jips/Elements/big/fotosAstro/2019-01-25-SanJuan/Orion-135mm`
  - `/media/jips/Elements/big/fotosAstro/2023-04-01-riofrio/m42/JPG`

# ROADMAP — Synthetic calibration for SkyStacker

**Input for the next implementation session.** Derived from
`SYNTHETIC_CALIBRATION_RESEARCH.md` section D. Do not implement items
out of order; each tier assumes the previous one is green in
`--selftest` and in CI.

**The honest answer to "should SkyStacker generate synthetic calibration
frames?" is no, not in this codebase.** The Tier 4 item is recorded only
to document the rejection. Everything user-visible in Tiers 1–3 is
either empirical reconstruction (defensible) or reporting (harmless).

## Tier 1 — Small, self-contained (`S`)

Both items are independently shippable in one commit.

### 1.1 `--black-level auto`

- **Tier:** S
- **User-visible change:** `--black-level auto` reads EXIF `BlackLevel`
  (JPEG/RAW) or FITS `BLACK_LVL` (FITS) and subtracts the scalar
  pedestal in `calibrate()`. Off by default.
- **Falsifiable acceptance test:** `testSubtractBlackLevel` builds a
  synthetic frame with a known pedestal, runs `calibrate()` with
  `--black-level auto`, and asserts the resulting histogram's 1st
  percentile is within 1 ADU of zero. Run with `--black-level off` and
  assert it is *not* within 1 ADU (the negative control).
- **Main risk:** a wrong EXIF tag biases every frame. Mitigation: the
  flag is off by default and `report.txt` names the tag and value used.

### 1.2 Report per-frame read-noise sigma

- **Tier:** S
- **User-visible change:** `report.txt` gains a `BgSigma` column in the
  frame-diagnostics table, from the existing `estimateFrameBgNoise`.
- **Falsifiable acceptance test:** `testFrameBgNoiseReported` builds a
  frame with known Gaussian noise (σ = 0.02), runs the pipeline, and
  asserts the reported value is within 10% of 0.02. A second frame with
  σ = 0.04 must report a larger value.
- **Main risk:** none; report-only.

## Tier 2 — Medium, a day each (`M`)

### 2.1 `--bpm PATH` bad-pixel map

- **Tier:** M (already named in `README.md` Tier 2)
- **User-visible change:** `--bpm PATH` loads a FITS or text map of
  `x,y` bad pixels, marks them `Pix = -1` before calibration, and
  `addFrameMasked` / `validCount` skip them as today.
- **Falsifiable acceptance test:** `testBPMMasksPixels` builds a frame
  with a bright pixel, supplies a BPM naming it, and asserts the
  resulting stack at that pixel equals the mean of the *other* frames,
  not the bright value. Without `--bpm`, the same fixture shows the
  bright value leaking through.
- **Main risk:** a wrong BPM masks real stars. Mitigation: explicit
  flag, no auto-detection in this tier.

### 2.2 `--self-flat` from dithered lights

- **Tier:** M
- **User-visible change:** `--self-flat` builds a master flat from ≥15
  registered, dithered lights by taking the per-pixel median across
  frames after rejecting star pixels via `sigmaClipReduceWeighted`.
  Writes `master_self_flat.f32raw` and reports the residual against
  `--local-normalize`.
- **Falsifiable acceptance test:** `testSelfFlatMedian` builds 20
  synthetic dithered lights with a known vignetting profile (radial
  polynomial), runs `--self-flat`, and asserts the recovered profile
  matches the truth within 1% RMS. A second test with 5 frames must
  *fail* the gate (fewer than 15) and fall back to `--local-normalize`
  with a WARNING.
- **Main risk:** removes real nebulosity on a field with extended
  emission. Mitigation: `--self-flat` is opt-in; the report names the
  frames it used and the residual it measured, so the user can compare
  to a real flat if one exists.
- **Shipped (2026-11-15):** implemented as a one-iteration sigma-clipped
  **mean** of per-frame sky-normalised aligned lights (not a per-pixel
  median), normalised per channel to median 1, gated at 15 registered
  frames, and applied after registration. A loaded flat wins.
  Tests: `testSelfFlatEligible`, `testSelfFlatRecovery` (20 synthetic
  frames with moving outliers, RMS < 0.03), and
  `testExportCalibrationMasters`. Orion-135mm: center/edge 1.2512,
  residual RMS 0.1936, corrected mean center/border 1.000. The
  extended-nebulosity risk was observed (M42 biases the map); see
  `research/04-calibration-and-self-flat.md`.
- **Status: superseded for scientific use by 2.3.** Keep v1 behind
  `--self-flat-model=registered` for A/B until the sensor-space model
  passes the acceptance tests; do not remove the v1 code before then.

### 2.3 Sensor-space robust self-flat (revision)

- **Tier:** L (multi-session; phases 1-4 are the minimum viable
  replacement).
- **User-visible change:** `--self-flat` builds a full-frame illumination
  model from calibrated, *unregistered* lights. It normalises each frame
  per channel, masks stars and bright extended structure, estimates a
  coarse cell grid, rejects cell outliers, and fits a low-degree
  Chebyshev/Legendre polynomial with IRLS (Huber/Tukey). The flat is
  applied to each frame **before** resampling, so it corrects both the
  registered and unregistered stacks. `master_self_flat.f32raw` is
  full-frame instead of crop-sized. A loaded flat still wins.
- **Why:** v1 is built in reference coordinates, so aligned stars and
  aligned nebulosity are treated as illumination. On Orion-135mm this
  produces a ~1.9 ratio around M42 and dark halos. The new estimator uses
  relative scene motion plus explicit masking, and a low-dimensional
  model that cannot reproduce stars or dust.
- **Phased acceptance tests:**
  - Phase 1 (prototype, shipped 2026-11-15): synthetic known vignetting +
    moving stars; recovered flat RMS < 0.05, full-frame master, applied
    before resampling, no registration needed for the build. Orion-135mm
    `--limit 20`: center/edge 1.2449, max 1.208 (v1 M42 dome absent),
    mask fraction 67.6% (too high; Phase 2). Default remains `registered`.
  - Phase 2 (masking): large static nebula; recovered flat at the nebula
    within 0.02 of truth; bright-patch and dust-shadow cells rejected.
  - Phase 3 (IRLS): fit RMS < 0.02; robust fit beats OLS with injected
    cell outliers; degree/cell-size diagnostics reported.
  - Phase 4 (integration): `mean`, `median`, `sigma`, registered and
    `--no-register` outputs all corrected; section-E tests unchanged;
    Orion E2E no longer shows the M42 dome.
- **Main risks:** (1) a stable sky gradient is degenerate with the flat
  and will be absorbed; document it and prefer a radial model when
  possible. (2) A large nebula with small dithers remains partially
  degenerate; masking and the polynomial reduce but do not eliminate it.
  (3) Dust is out of scope; a real flat is required. (4) The current
  sRGB-encoded pipeline makes the division approximate; gamma-aware
  correction is a separate phase.
- **Flags:** temporary `--self-flat-model=grid|registered` (default
  `registered` until Phase 4 passes, then `grid`); optional
  `--self-flat-degree` (default 4) and `--self-flat-cell` (default 64).
  Do not document a flag before it exists.
- **Implementation map:** see `research/04-calibration-and-self-flat.md`
  sections 10-14. Build the flat before Pass 2; force/stream the
  unregistered data in Pass 1; apply after calibration/BPM and before
  `resampleToReference*`; post-divide the unregistered mean/median/sigma;
  remove `flatCorrection` and the flat/`frameScales` parameters from
  `reduceTilesStack`/`reduceTile` once unused.

## Tier 3 — Large, multi-day (`L`)

### 3.1 Per-component confidence map in `report.txt`

- **Tier:** L
- **User-visible change:** `report.txt` gains a "Calibration
  confidence" block that reproduces the section-B table per run, with
  the actual measured residual for each component the run touched
  (e.g. `--local-normalize` residual, `--bg-degree` residual).
- **Falsifiable acceptance test:** `testConfidenceMapContains` runs the
  pipeline on a synthetic fixture with a known gradient and asserts the
  block names every one of the ten components and reports a
  non-`[assumed]` residual for at least the three the run touched.
- **Main risk:** none; report-only.

## Tier 4 — Architectural / out of scope (`XL`)

### 4.1 Synthetic bias/dark/flat generation

- **Tier:** XL — **refused**
- **User-visible change:** none. The refusal guards
  (`--synthetic-dark`, `--synthetic-flat`, `--synthetic-bias`) shipped
  in commit `5be6865`. This item records that a synthetic generator was
  considered and rejected, so the next session does not re-open it.
- **Falsifiable acceptance test:** the existing
  `testSyntheticCalibGuardrailFullSet` and
  `testSyntheticCalibRefusalMessage` already assert the refusal fires
  and names all three flags. A new attempt to add a generator must fail
  these tests.
- **Main risk:** silent synthesis. The guardrails exist precisely to
  make this failure mode impossible to ship.

## How to use this roadmap

Start at Tier 1.1. Apply the one-liner triple-quote patch scheme from
the session log in `PROJECT_CONTEXT.md`. Each item is one commit. After
each commit, update `PROJECT_CONTEXT.md`'s "Still open" list so the next
session starts from the current state, not from this file's initial
state.

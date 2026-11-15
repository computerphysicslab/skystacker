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

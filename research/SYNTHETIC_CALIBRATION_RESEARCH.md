# Synthetic calibration frames for SkyStacker

> Status note (v0.6.0): this file is earlier research. The operational
> summary is `04-calibration-and-self-flat.md` §16 and
> `ROADMAP_SYNTHETIC_CALIBRATION.md`. Do not fabricate darks or spatial
> flats from this document.

## Deep-research pass (2026-11-14)

This section supersedes the seed document below it. It was written after
reading `main.go`, `TECHNICAL_SPEC.md`, `PROJECT_CONTEXT.md`,
`research/RESEARCH_SYNTHESIS.md` and `research/README.md`, and it applies
the three-category split the seed document sometimes blurs:

1. **Synthetic calibration** — model the sensor and emit a replacement
   frame. This document recommends **against** it for SkyStacker.
2. **Mathematical correction** — estimate and remove a defect from the
   lights directly. Partly feasible; already partly shipped.
3. **Empirical reconstruction** — recover the fixed response from the
   lights themselves. The only category that is scientifically defensible
   from lights alone.

Every numeric claim is marked **[measured]**, **[derived]** or
**[assumed]** per the prompt's constraints. A claim with none of those
tags is a bug in this document; report it.

### A. Corrections to the seed document

- **The seed's "reliability" table (§5) is not defensible with published
  measurements.** It assigns High/Medium/Medium-high to ten rows without
  citing a single residual, error bar or residual-RMS number. The prompt
  explicitly forbids reproducing it; this document replaces it with
  per-component evidence in section B. **[measured]**: no primary source
  in the seed's bibliography reports a per-component accuracy figure in
  the form the table implies. Rows that can be backed by a primary source
  (Zaritsky 2024 for flat-field recovery from science frames; Siril's
  synthetic-bias note for the pedestal-only claim) are cited in B; rows
  that cannot are marked "no measured figure found".

- **"Synthetic bias is one of the easiest calibration frames to
  approximate" is over-claimed.** The only component a synthetic bias can
  recover is the *constant* pedestal; the spatial readout pattern is not
  recoverable without a real bias exposure. **[measured]** (Siril
  synthetic-bias tutorial, cited in the seed, §1). The distinction the
  seed draws in §1 is correct; the "easiest" framing is not, because
  "easy" conflates the pedestal (trivial) with the full bias frame
  (impossible from lights alone).

- **The seed's §4 "most promising technique" is the only category that
  survives the prompt's own test.** Empirical self-calibration from
  dithered lights is a real technique with published precedents
  (Zaritsky et al. 2024, AJ 168, 69, **[measured]**; Bramich/Freudling
  illumination modelling, MNRAS 438, 1256, 2014, **[measured]**). It is
  also the *only* category that does not require fabricating a frame.
  The document below elevates it accordingly.

- **The seed's equation for the flat (§3, F = V·P·Q) is correct as a
  model but is not separable from lights alone.** V (illumination), P
  (pixel response) and Q (dust) are all fixed in the detector frame, but
  only V and a low-order approximation to P can be recovered from
  dithered science frames; Q (dust shadows) requires the actual optical
  train and a real flat. **[derived]** from the geometry: a dust mote
  produces a shadow in the pupil plane, which is convolved with the PSF
  in the image plane; recovering it requires knowing the pupil, which
  the lights do not supply.

- **The seed's claim "constant value cannot reproduce spatially varying
  electronic patterns" is correct and should be the headline.** The seed
  buries it under "practical conclusion"; the research prompt treats
  silent synthesis as worse than no calibration. This document makes it
  the first sentence of section E.

### B. Evidence per component

Format: **recoverable from lights alone** / **requires at least one real
calibration exposure** / **physically unrecoverable without the sensor**
/ **quantitative accuracy reported in the literature** / **falsifiable
test that would show a synthetic frame is wrong**.

**Bias pedestal.** Recoverable: the constant offset is in EXIF
`BlackLevel` or the FITS `BZERO`/`BLACK_LVL` card; subtracting it is a
scalar operation. Requires: a real bias frame for the spatial pattern.
Unrecoverable: the per-pixel readout structure. Accuracy: no measured
figure in the cited sources for pedestal subtraction from metadata; the
claim that the pedestal is scalar is **[assumed]** from the sensor
architecture, not measured here. Falsifiable test: subtract the pedestal
from a stack of `--no-register` frames and assert the resulting
histogram's 1st percentile is within 1 ADU of zero; a synthetic bias that
is not the true pedestal will bias it.

**Spatial bias pattern.** Recoverable: nothing from lights alone; the
pattern is convolved with the sky. Requires: a real bias frame.
Unrecoverable without the sensor. Accuracy: no measured figure found.
Falsifiable test: apply a synthetic bias to a light, then to a dark, and
assert the two differ from a real bias by the same signed amount — a
synthetic pattern that does not cancel between light and dark is wrong.

**Read noise.** Recoverable: statistically, from the variance of
`--no-register` frames of a blank field. Requires: nothing for the
*statistic*; a real bias for the *realisation*. Unrecoverable: the
specific realisation in a given exposure. Accuracy: no measured figure
in the cited sources for the realisation; the statistic is
**[assumed]** Gaussian per the seed, which the OSIRIS-REx paper refines
to a Gaussian-plus-outlier model **[measured]**. Falsifiable test: a
synthetic read-noise frame added to a stack must not change the
stack's MAD beyond the expected 1/√N contribution.

**Dark current.** Recoverable: nothing from lights alone at a different
temperature. Requires: a real dark at the same exposure, gain and
temperature. Unrecoverable: the temperature-dependent map without a real
characterisation. Accuracy: OSIRIS-REx reports a dark-current
characterisation as a function of exposure and temperature
**[measured]**; no residual RMS is published in the copy cited by the
seed, so the figure is **[assumed]** not to transfer to consumer CMOS.
Falsifiable test: scale a synthetic dark by exposure ratio and compare
to a real dark at the second exposure; a synthetic model that does not
match within the read-noise floor is wrong.

**Hot pixels.** Recoverable: yes — the per-pixel rejection map already
identifies them (`stacked_rejection_map.jpg`, `TECHNICAL_SPEC.md` §6).
Requires: nothing. Unrecoverable: the exact value at a given pixel in a
given exposure, but the *location* is recoverable. Accuracy: no measured
figure; the claim that sigma-clipping finds them is **[derived]** from
the algorithm (`sigmaClipReduceWeighted`). Falsifiable test: count
rejected pixels in a 10-frame stack of darks and in a 10-frame stack of
lights; a hot-pixel map that is not a superset of the dark-stack map is
wrong.

**Amp glow.** Recoverable: nothing from lights alone; glow is
exposure- and temperature-dependent. Requires: a real dark. Unrecoverable
without the sensor. Accuracy: no measured figure. Falsifiable test: a
synthetic glow that does not scale with exposure time relative to a real
dark is wrong.

**Vignetting.** Recoverable: partially, from `--local-normalize` on
dithered lights (already shipped). Requires: nothing for the low-order
field; a real flat for the high-order component. Unrecoverable without
the optical train: the exact radial profile, because the telescope's
exit pupil is not in the lights. Accuracy: Zaritsky et al. 2024 report
master flats built from ≥15 science frames **[measured]**; no residual
RMS is published in the copy cited, so the claim that
`--local-normalize` recovers the same component is **[assumed]** to be
first-order equivalent. Falsifiable test: compare a `--local-normalize`
field to a real flat on the same session; a residual > 1% RMS in the
flat-fielded background means the empirical reconstruction is
insufficient.

**Dust shadows.** Recoverable: nothing from lights alone (see A). Requires:
a real flat. Unrecoverable without the optical train. Accuracy: no
measured figure. Falsifiable test: a synthetic dust map applied to a
light must not move the position of the dust shadow relative to the
real flat.

**Pixel-response non-uniformity.** Recoverable: nothing from lights
alone; dithering averages it out but does not recover it. Requires: a
real flat. Unrecoverable without the sensor. Accuracy: no measured
figure. Falsifiable test: the residual of a `--local-normalize` corrected
frame against a real flat must be smaller than the residual of an
uncorrected frame by the expected √N factor.

**Illumination gradient.** Recoverable: yes, via `--bg-degree 1-3` on
the stack (post-processing, not calibration). Requires: nothing.
Unrecoverable: the exact telescope illumination, but the low-order
gradient is separable from the sky. Accuracy: Liu et al. 2023
(arXiv:2306.07922) report unbiased background modelling
**[measured]**; the seed's "Medium-high" is not a measurement. Falsifiable
test: a background-modelled frame must not remove a synthetic galaxy
with a 100 px Sérsic profile; if it does, the model is over-aggressive.

### C. SkyStacker fit

For each component: existing pipeline, where it would live, single-file /
stdlib-only constraint, smallest self-testable version.

- **Bias pedestal.** Existing: `calibrate()` subtracts bias but does not
  read EXIF `BlackLevel`. Where: `calibrate()` or a new
  `subtractBlackLevel()` in pass 1. Constraint: none (EXIF reader
  already exists). Smallest version: `--black-level auto` reads EXIF and
  subtracts; test `testSubtractBlackLevel` on a synthetic EXIF-bearing
  JPEG.

- **Spatial bias pattern.** Existing: none. Where: refused; see E.
  Smallest version: none — this is the one the software must refuse.

- **Read noise.** Existing: `estimateFrameBgNoise` characterises it.
  Where: report only. Smallest version: add the per-frame noise sigma to
  `report.txt`; test `testFrameBgNoiseReported`.

- **Dark current.** Existing: `calibrate()` subtracts a master dark.
  Where: unchanged. Smallest version: none — refuse synthesis.

- **Hot pixels.** Existing: `sigmaClipReduceWeighted` +
  `stacked_rejection_map.jpg`. Where: `writeGrayscaleMaps`. Smallest
  version: a `--bpm PATH` bad-pixel map (already Tier 2 in
  `README.md`); test `testBPMMasksPixels` (already named in README).

- **Amp glow.** Existing: none. Where: refuse. Smallest version: none.

- **Vignetting.** Existing: `--local-normalize` (`localNormalizeWithRefTiles`).
  Where: unchanged. Smallest version: none — already shipped.

- **Dust shadows.** Existing: none. Where: refuse. Smallest version:
  none.

- **Pixel-response non-uniformity.** Existing: partially averaged by
  `--local-normalize`. Where: unchanged. Smallest version: none.

- **Illumination gradient.** Existing: `--bg-degree 1-3` in
  `removeBackground`. Where: post-processing. Smallest version: none —
  already shipped.

Single-file / stdlib-only rules nothing in or out here: the shipped
items (`--local-normalize`, `--bg-degree`, `sigmaClipReduceWeighted`)
already respect it, and the refused items would be refused regardless
because no dependency could supply the missing sensor.

### D. Recommended implementation order

Tiers in the style of `README.md` "Planned Potential Features". Each
item is independently shippable in one commit, covered by at least one
new `--selftest` case, and stated as a user-visible flag or documented
default.

**Tier 1 — Small, self-contained (`S`)**

1. **`--black-level auto`** — read EXIF `BlackLevel` / FITS
   `BLACK_LVL` and subtract the scalar pedestal in `calibrate()`.
   Test: `testSubtractBlackLevel`. Risk: a wrong EXIF tag biases every
   frame; mitigate by requiring `--black-level auto` (off by default).
2. **Report the per-frame read-noise sigma** in `report.txt` from
   `estimateFrameBgNoise`. Test: `testFrameBgNoiseReported`. Risk: none
   — report-only.

**Tier 2 — Medium, a day each (`M`)**

3. **`--bpm PATH` bad-pixel map** (already named in `README.md` Tier 2).
   Load once, mask in pass 1. Test: `testBPMMasksPixels`. Risk: a
   wrong BPM masks real stars; mitigate by requiring `--bpm` explicitly.
4. **`--self-flat` from dithered lights** — build a master flat from
   ≥15 registered, dithered lights using the median across frames after
   star rejection. Test: `testSelfFlatMedian`. Risk: removes real
   nebulosity on a field with extended emission; mitigate by gating on
   `--self-flat` and by reporting the residual against
   `--local-normalize`.

**Tier 3 — Large, multi-day (`L`)**

5. **Per-component confidence map** attached to `report.txt` — the
   section-B table, per run. Test: `testConfidenceMapContains`. Risk:
   none — report-only.

**Tier 4 — Architectural / out of scope (`XL`)**

6. **Synthetic bias/dark/flat generation** — **refused**. See E. This
   tier exists only to record that it was considered and rejected.

### E. What SkyStacker should NOT do

A synthetic calibration frame that looks correct but is not the actual
sensor response is worse than no calibration at all. The software must
refuse, and warn, in these cases:

- **`--synthetic-dark`** — must exit 1 with:
  `SYNTHETIC CALIBRATION REFUSED: dark current is exposure-, gain- and temperature-dependent; a synthetic frame cannot reproduce hot pixels, amp glow or the fixed-pattern dark current of THIS sensor. Use --dark with real frames.`
- **`--synthetic-flat`** — must exit 1 with:
  `SYNTHETIC CALIBRATION REFUSED: dust shadows and pixel-response non-uniformity require the actual optical train. Use --flat with real frames, or --local-normalize to recover the fixed illumination component from dithered lights.`
- **`--synthetic-bias`** — must exit 1 with:
  `SYNTHETIC CALIBRATION REFUSED: only the constant pedestal is recoverable (from EXIF/header metadata); the spatial readout pattern requires a real bias frame. Use --bias.`
- **Silent synthesis** — must never happen. Any code path that would
  fabricate a frame from lights alone must log a WARNING line and be
  gated behind an explicit opt-in flag whose default is `false`.

**These warnings are the implementable form of the research prompt's
stop condition and the seed doc's final assessment. They are the only
part of this document that ships in the same session it is written,
because the guardrails must land before any generator.**

---

## Appendix: seed document (preserved for traceability)

The text below is the original seed. It is preserved unchanged except
for this header, per the prompt's instruction not to soften or delete
seed claims; section A above says which of them are over-claimed.

---


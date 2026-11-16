# Changelog

## 0.6.1 (2026-11-16)
- Added `--comet-target PATH` ephemeris mode (ISO8601 `RA_deg Dec_deg`, linear interpolation by EXIF time; requires `--plate-solve`). Validation of RA/Dec ranges and clearer errors.
- Added `testCometEphemerisParse`; self-tests now 182.
- SIP order-2: applied in WCS helpers (`wcsTanForward`, `wcsTanInverseSIP`); stacking resampler remains affine/similarity-based (documented).
- Version bumped to 0.6.1; docs updated (README, TECHNICAL_SPEC, prompt-ai).
- Added `--fits` writer (`stacked_sum.fits`, big-endian BITPIX=-32, NAXIS=3) and round-trip selftest; self-tests 186.
- Added `--comet-ephem-strict-exif` to fail instead of falling back when a light lacks EXIF time; comet-target examples in README.
- Documented SIP stacking-resampler limitation and a reproducible OrionNebula grid-vs-real-flat validation protocol.
- Added `--bpm-auto` / `--bpm-auto-sigma` (bad-pixel map from master dark/bias) with `testBPMAuto`; self-tests 187.
- Wired `--plate-solve` into `--post-only` (re-detect stars from the stored float32 stack); `testPlateSolvePostOnlyNoStars` added; self-tests 188.
- Detected stars now carry a flux-weighted half-flux radius (HFR); `testStarHFR` added; self-tests 189.
- Added `--xisf` writer (monolithic XISF 1.0, planar Float32) with `testXISFWrite`; self-tests 190.

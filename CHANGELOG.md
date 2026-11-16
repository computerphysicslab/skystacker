# Changelog

## 0.6.1 (2026-11-16)
- Added `--comet-target PATH` ephemeris mode (ISO8601 `RA_deg Dec_deg`, linear interpolation by EXIF time; requires `--plate-solve`). Validation of RA/Dec ranges and clearer errors.
- Added `testCometEphemerisParse`; self-tests now 182.
- SIP order-2: applied in WCS helpers (`wcsTanForward`, `wcsTanInverseSIP`); stacking resampler remains affine/similarity-based (documented).
- Version bumped to 0.6.1; docs updated (README, TECHNICAL_SPEC, prompt-ai).

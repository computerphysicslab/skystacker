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
- Added `--hfr-quality` opt-in: median star HFR replaces blob size as FWHM proxy for frame quality; `testHFRQuality`; self-tests 191.
- Added `--sigma-clip-low` / `--sigma-clip-high` asymmetric MAD rejection; `testSigmaClipAsymmetric`; self-tests 192.
- Added `--annotate`: offline Gaia catalog projection onto the plate-solved stack, annotated JPEG, and `anomalies.json`; `testCatalogAnnotationSkip`; self-tests 193.
- Added `--starless`: starless + star-layer float32/JPEG outputs from the post-processing star-removal step; `testPipelineStarless`; self-tests 194.
- `--fits` now writes TAN WCS keywords (CRVAL/CRPIX/CD, degrees) when a plate solution is available.
- Hardened `--bpm-auto`: refuse maps flagging >0.1% of the sensor (avoids pathological thresholds on low-noise masters).
- Added `--json-report` (structured `report.json` alongside `report.txt`).
- `--fits`/`--xisf` now also write `stacked_mean.fits`/`stacked_mean.xisf` (linear mean stack, WCS in FITS).
- Added `--normalize-mode` (scale-offset/additive/multiplicative) with `testNormalizeModes`; self-tests 195.
- Added `--bg-multiscale` coarse percentile background before the polynomial fit; `testMultiscaleBackground`; self-tests 196.
- Added `--config FILE` JSON flag files (CLI overrides config); `testConfigFile`; self-tests 197.
- Added `--deconvolve` (Gaussian-PSF Richardson-Lucy) with `--deconv-sigma/-iters`; `testRichardsonLucy`; self-tests 198.
- Added `--narrowband HOO|SHO` with `--nb-a/--nb-b/--nb-c` mono stacks; `testNarrowband`; self-tests 199.

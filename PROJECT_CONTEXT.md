# SkyStacker — project context (last updated 2026-11-22)

## Version
v0.6.1 (Go module: `main.go` plus first-party files such as `annotation_layers.go` and the `goTiff/` module)

## Build, test, install
```
gofmt -w main.go && go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o skystacker . && ./skystacker --selftest
go install .
```

**Self-test count: 281 passing.** Update this number after any test change.
Earlier counts in the history sections below are historical and must not be edited.
Research paths below mentioning former partial files (for example
`SYNTHETIC_CALIBRATION_RESEARCH.md`, `RESEARCH_SYNTHESIS.md`, `GEMINI_*`,
`05-research-deep-dive-*.md`, `06/07/08-*-2026-11-17.md`) are historical
names: those files were consolidated on 2026-11-18 into
`research/01-...08-*.md` plus `research/README.md`. Use the canonical files
for current content.

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
  - Registered stack: `weight = computeFrameQuality × computeAdvancedQuality / 100`.
  - Unregistered stack: `weight = computeAdvancedQuality / 100`.
  - `computeAdvancedQuality` (0-100) implements the advanced-stacking
    formula `Q = w1*SNR - w2*FWHM - w3*E - w4*B`. Each metric is
    normalized to a "higher is better" `[0,1]` term
    (`SNR/(SNR+50)`, `1-(FWHM-3)/7`, `1-Ecc`, `1-(Bg-0.01)/0.14`)
    then weighted `0.40/0.30/0.15/0.15`. SNR and FWHM dominate.
  - **Eccentricity** is computed per star from second central moments
    of the intensity-weighted pixel distribution. Star cache bumped to
    `cacheVersion = 2`; `star.Ecc` and `starCache.MedianEcc` fields added.
  - `--no-register` and `--method median` still use equal weights
    (median is not yet weighted — see Pending).
  - Zero-weight survivors → fallback to unweighted mean (avoids divide-by-zero).
  - `report.txt` frame-diagnostics table shows `IntrQ` (0-100, now the
    advanced score) and `Weight` (0-1) columns.

## CLI flags relevant to current work

The Gaia DR3 flags (`--test-gaia`, `--gaia-mirror`, `--gaia-cache-dir`, `--gaia-nside`, `--gaia-mag-limit`, `--gaia-offline`, `--gaia-cache-refresh`) are documented in the "Gaia DR3 on-demand queries" section below.

### Registration (implicit, no new flags)
Automatic. Disable with `--no-register`.

### Affine refinement
- `--no-affine` — diagnostic only: forces similarity-only registration.
  The transform cache is bypassed entirely in this mode so a default-mode
  cached affine cannot leak into the diagnostic run.

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

## Gaia DR3 on-demand queries (data-access layer only)

The plate solver (a future session) will consume these helpers. This
session lands only the data access layer; nothing in the stacking
pipeline calls it yet.

**A1a.5 diagnosis (Phase 2, 2026-11-13)**: the `--test-gaia` 13-vs-4-star
discrepancy is a **face-assignment bug**, not a within-face interleave
bug. Empirical healpy 1.16.5 data: `pix2ang(256, 0) = (ra=45°, dec=0.15°)`,
i.e. face 0 extends to the equator at ra≈45°, and the boundary with face 4
at dec=32.6° is at ra=0° but with face 5 it is at ra=85°. The current
`za <= 2/3` branch is therefore wrong for equatorial vertices of polar
faces. The interleave was always correct. The root cause was two independent
bugs in the pair: (a) `healpixAng2PixNested` used `face = 4 + faceQ`
for equatorial points and a linear `(1-|z|)*3` polar scaling, when
healpy derives `face` from `(ifp, ifm) = (jp/nside, jm/nside)` and uses
`sqrt(3*(1-|z|))` scaling; (b) `healpixPix2AngNested` used the
equatorial `ra` formula for polar faces and did not invert the
`ifp`/`ifm` rotation. A previously-undiscovered *third* difference was
the `z`-reconstruction normalisation: healpy uses
`fact1 = 2/(3*nside)` and `fact2 = 1/(3*nside^2)`, and the old code's
matching `(v-0.5)*4/3` and `(1∓z)*3` reconstructions broke round-trip
as soon as only the face assignment was corrected — which is why the
earlier one-sided fixes each failed. Fixed at commit
`e47c3fd`: both functions rewritten as a matched pair, ported
line-for-line from healpy 1.16.5's
`healpix_base.cc` (`loc2pix` / `pix2loc` + `nest2xyf` / `xyf2nest`).

**HEALPix nested scheme** — `healpixAng2PixNested`,
`healpixPix2AngNested`, `healpixTileCorners`, `healpixNsideValid`,
`healpixTileSizeDeg2`. Implemented fresh from Gorski et al. 2005,
ApJ 622, 759, section 5. No external dependency; the algorithm is
~200 lines and covered by `testHealpixNested`. All nside=1, 16 and 256 cases match healpy 1.16.5 exactly in
both directions (`ang2pix` and `pix2ang`); fixtures live in
`testdata/healpix_reference.json` and are asserted by
`testHealpixNside16MatchAng2Pix`, `testHealpixNside256MatchAng2Pix`,
`testHealpixPix2AngMatchesHealpy` and `testHealpixAngPixRoundTrip`.

**TAP client** — `gaiaQueryTile` posts an ADQL `POLYGON` query to a
configurable TAP mirror, retries 3x with 1s/2s/4s backoff on
5xx/timeout, fails immediately on 4xx or malformed JSON. Rate
limited to 1 req/sec between calls within a single run.
`gaiaBuildADQL` emits a single-line query safe for
`application/x-www-form-urlencoded` POST.

**Disk cache** — `$XDG_CACHE_HOME/skystacker/gaia_cache/healpix_n<nside>_p<ipix>.json` on Linux (usually `~/.cache/skystacker/gaia_cache/`), `~/Library/Caches/skystacker/gaia_cache/` on macOS, `%LocalAppData%\skystacker\gaia_cache\` on Windows. Overridable with `--gaia-cache-dir`. The previous per-output-directory default was tied to a single stacking session, which is the wrong lifetime for a frozen catalog.
`gaiaCacheVersion = 1` (distinct from `cacheVersion`). Atomic writes
via the existing `writeFileAtomic(path, func(io.Writer) error)`
helper. Empty star lists (barren tiles) are valid and cached.
Version mismatches are rejected with a WARNING log line.

**CLI** — `--test-gaia RA,DEC` runs a one-shot diagnostic.
`--gaia-mirror`, `--gaia-cache-dir`, `--gaia-nside` (default 64 =
bit-depth 6, approx **0.84 deg²**/tile; `--gaia-nside 256` = bit-depth 8, approx 0.052 deg²/tile), `--gaia-mag-limit` (default
14.0), `--gaia-offline`, `--gaia-cache-refresh`. `--gaia-offline`
and `--gaia-cache-refresh` are mutually exclusive.

**Tile size correction.** The session prompt that scoped this work
stated "nside=256 (~13.75 deg² per tile)". That number is wrong by a
factor of nside²: `healpixTileSizeDeg2(256) = 41252.96 / (12 * 256²)
= 0.0524 deg²`. The prompt's 13.75 deg² corresponds to nside=16 (the
4-bit, not 8-bit, resolution). At nside=256 a 1-5 deg² image spans
**20-100 tiles**, not the 1-5 the prompt estimated. A 1000-target
imaging run therefore caches ~2 GB of JSON at nside=256 (~60 000
tiles at ~30 KB each), not the ~100 MB the prompt estimated. At
nside=64 (0.84 deg²/tile) the same run caches ~20-40 MB. The
plate-solver session should reconsider `gaiaDefaultNside` before
building on top of this layer.

**Cache size in practice** (real numbers, not the prompt's estimate)
— At mag_G < 14, Gaia source density is roughly 10^2-10^3 stars per
deg² depending on galactic latitude (higher toward the plane). A
1-5 deg² image contains ~10^2-10^4 stars. At nside=256 with the
cached fields (ra, dec, G, plx, pmra, pmdec) each tile is ~10-100 KB
of JSON. At nside=64 each tile is ~30-300 KB but there are 25-50x
fewer tiles per image.

### Gaia DR3 flags (added 2026-11-13)

| Flag | Default | Purpose |
|---|---|---|
| `--test-gaia RA,DEC` | (unset) | One-shot diagnostic: query one tile, print first 5 stars, exit. |
| `--gaia-mirror URL` | `https://gea.esac.esa.int/tap-server/tap` | Gaia TAP mirror. |
| `--gaia-cache-dir D` | OS user cache dir | Tile cache directory (see "Disk cache" above). |
| `--gaia-nside N` | `64` | HEALPix nside (power of 2, 2-8192). |
| `--gaia-mag-limit M` | `14.0` | Gaia G-band magnitude limit (0-20). |
| `--gaia-offline` | `false` | Cache misses are a hard error; no network access. |
| `--gaia-cache-refresh` | `false` | Re-download even if cached. Mutually exclusive with `--gaia-offline`. |


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

- **Gaia TAP client validated against ESA**
- **A1a.5 geometry discrepancy confirmed live** (2026-11-13): `--test-gaia 87.5,-45.0` (ipix=589120) returns 13 stars, not the 4 the earlier `curl` returned. Star values/columns are correct; the polygon emitted by `healpixTileCorners` is centred ~0.15 deg west of the true healpy tile at nside=256. Round-trip consistency holds but tile geometry is approximate. Not a parser bug. Fix in A1a.5 before the plate solver relies on tile centres.
 (2026-11-13): live
  `--test-gaia 87.5,-45.0` returns the 4 stars that a direct
  `curl` to `/tap/sync` returns. Fixes: `/sync` endpoint suffix,
  project User-Agent, context-cancel ordering (cancel after
  `io.ReadAll` + body close), mid-body timeout is fatal and not
  retried, empty-body retry, per-attempt 180 s context timeout
  with 5 s heartbeat, and VOTable `<INFO>` error extraction.
  No self-test count change.
- **Gaia DR3 on-demand HEALPix queries APPLIED** (2026-11-13): the
  data-access layer for the future plate solver. HEALPix nested
  math + TAP client + disk cache + `--test-gaia` diagnostic flag.
  No consumer in the pipeline yet. Tests: testHealpixNested,
  testGaiaCache, testGaiaADQL, testGaiaQueryTileRetry,
  testGaiaCLIParsing. Test count 106 -> 111. Commits bc09c85
  (HEALPix), 5038592 (TAP + cache), 402063c (--test-gaia).

- **Affine refinement guards APPLIED**: Affine was overfitting to noise.
  128 of 134 frames "used" affine with residual drops of 55-85%, impossible
  for a stack with scale ~1.000 and rotation <=0.6deg. Bounding boxes
  stayed ~100% wide so no flag fired, but the actual parallelograms
  covered only a thin wedge. `findSafeCrop` correctly intersected them,
  collapsing the crop to 169x136 (2% of the 867x1302 reference). Fix:
  (1) inline gate in the affine acceptance condition rejects shear > 0.10
  or anisotropy > 1.10 (real optics are <5%); (2) residual threshold
  tightened from 10% to 25%; (3) `computeFootprintPct` returns the true
  parallelogram area via the shoelace formula; (4) `frameDiag.FootprintPct`
  is surfaced as a `Foot%` column in `report.txt`; (5) `frameLimiterFlags`
  fires `LOW FOOTPRINT` when the actual footprint < 50%, so this failure
  mode is named directly instead of silently shrinking the crop. New test:
  `testAffineQuality`. Test count 104 -> 105.


- **Calibration-subdir detection plural- and case-insensitive APPLIED**:
  `detectCalibrationSubdirs` previously listed only specific case
  variants of each folder name ("Light", "Lights", "LIGHT", "LIGHTS")
  even though its doc comment promised case-insensitivity. A dataset
  with the standard Linux all-lowercase layout (`lights/`, `darks/`,
  `flats/`, `biases/`) returned an empty `calibLayout`, discovery fell
  back to the parent directory (which contains only subfolders), and
  the run died with `no image files (JPG/PNG/RAW/FITS) found` on a
  directory full of `.CR2` subs. Confirmed against
  `/home/jips/datos/Astro/datasets/OrionNebula`. Fix reads the
  directory once into a case-insensitive map of lowercased-name ->
  actual absolute path, then resolves each alias by lowercasing it
  first. Both sides are lowercased, so `pick("Dark", "Darks")` matches
  any case of either singular or plural (`dark`, `DARK`, `Dark`,
  `darks`, `DARKS`, `Darks`). Adding a new convention (e.g.
  `"Dark_frames"`) is a one-word edit to the `pick` argument list.
  Singular wins over plural by argument order, preserving the Case 4
  contract. `testDetectCalibrationSubdirs` extended in place with
  sub-cases 6-9 (plural-lower, singular-lower, mixed-case,
  singular+plural coexistence). Top-level test count unchanged
  (extended an existing test function).


- **post_step_0-stretched.jpg APPLIED**: alongside every `stacked_sum.jpg`,
  the pipeline now also writes `post_step_0-stretched.jpg`, a per-channel
  min-max normalized preview of the raw float32 sum. Each channel is
  mapped independently so its minimum becomes 0 (pure black) and its
  maximum becomes 255 (pure white), with no gamma lift. This is the
  gamma-free complement to `stacked_sum.jpg` (which applies a gamma 0.5
  LUT via `writeAutoNormalizedPreviewJPEG`). Written by
  `writeMinMaxStretchedPreviewJPEG` right after the sum preview in both
  the registered and unregistered branches of `runPipeline`. Reported in
  `report.txt` under "Output files". `stacked_sum.jpg` itself is
  unchanged. New self-test: `testMinMaxStretchedPreviewJPEG`. Test
  count 102 -> 103.

- **go vet: report header printf FIXED** (2026-11-13): the frame-diagnostics
  header string had 14 format verbs but 15 arguments — the `Foot%` column
  was added by the affine-footprint patch without updating the format
  string. `go build` and `--selftest` passed (runtime `printf` silently
  appends `%!(EXTRA string=Foot%)`), but `go vet`, which CI runs before
  tests, rejected it, turning all three CI jobs red since that patch.
  Fixed by inserting the missing `%7s` before the final `%s`. Also adds
  `--no-affine` for diagnostic runs.

- **Advanced frame weighting APPLIED** (2026-11-13): frame weights now use
  `computeAdvancedQuality` (0-100), implementing
  `Q = w1*SNR - w2*FWHM - w3*E - w4*B`. Per-metric normalization:
  `SNR/(SNR+50)` (saturating, higher better), `1-(FWHM-3)/7`,
  `1-Ecc`, `1-(Bg-0.01)/0.14`. Weights `0.40/0.30/0.15/0.15`.
  Eccentricity from second central moments; `star.Ecc` added;
  `starCache.MedianEcc` added; `cacheVersion` bumped to 2 to invalidate
  pre-eccentricity star caches. New test `testComputeAdvancedQuality`
  (asserts exact Q at SNR=200 and >=99 at saturated SNR). Test count
  105 → 106.

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

- **`--spikes-min-count` APPLIED** (default 5): floors `selectSpikeStars` at `max(percent-floor, minCount)`, capped at available star count. M42 previously spiked 1 star (626 × 0.2% = 1.25 → 1); with the default floor it now spikes 5. `--spikes-min-count 1` restores the old behaviour. Signature of `selectSpikeStars` gained a trailing `minCount int` parameter (all call sites updated).

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

## Post-v0.5.0 session updates (APPLIED)

### Star / transform cache (APPLIED)

Re-runs skip Pass 1 detection and Pass 2 registration via JSON files
under `<output>/_work/cache/`:

  - `stars_s<N>_<hash>.json` — star list + medianBlob + bgNoise
  - `xf_s<N>_<refHash>_<frameHash>.json` — similarity (+affine), inliers, or fail reason

`<hash>` = `sha256(absPath + "|" + mtime_nano + "|" + size)`; any file change invalidates its entry. `cacheVersion = 1` must be bumped when detection or registration algorithms change. Cache filename embeds `starDetectionSig(scale)` so `--scale 4` and `--scale 1` never collide.

New flags: `--no-cache` (disable reads+writes), `--cache-dir P` (override location; default `<output>/_work/cache`). Write failures log a warning, never abort. Second run logs `Cache: N/M star hits, K/L transform hits` after registration.

New self-tests: `testStarCacheRoundTrip`, `testStarCacheInvalidation`, `testXfCacheInvalidation`.

**Default cache removal (2026-11-12)**: `_work/cache/` is now deleted on successful exit by default. Rationale: on datasets with hundreds of subs the cache accumulates one small JSON per frame under every `0_skystacker_results/_work/cache/` on disk, which is invisible to the user and adds up across sessions. The `_work/stacked_mean.f32raw` work file is intentionally preserved so `--post-only` still works. Users who want fast re-runs pass `--keep-cache`. `--no-cache` (unchanged) skips cache writes and reads entirely. `--clean` (unchanged) is still the way to remove `_work/stacked_mean.f32raw` too.

### Release workflow (APPLIED)

`.github/workflows/release.yml` triggers on `v*` tag push. Matrix: linux/darwin/windows × amd64/arm64. Builds with `-trimpath -ldflags '-s -w -X main.version=${TAG} -X main.buildDate=...'`, uploads artifacts + `SHA256SUMS.txt` to a GitHub Release.

`version` is now `var` (was `const`) so `-ldflags -X main.version=v0.5.1` overrides it. `go.mod` declares `module github.com/computerphysicslab/skystacker`, so `go install github.com/computerphysicslab/skystacker@latest` works once a tag exists.

### Native PNG input (APPLIED)

`.png` accepted in `discoverFiles`; `image/png` was already imported via `_ "image/png"`. The `testDiscovery` fixture was updated: `c.png` → `c.tif` (still excluded, still proves unsupported extensions are skipped) and a positive assertion that `starfield.png` is discovered was added. Test count unchanged (97).

### Intermediate map export (APPLIED)

The registered sigma-clipped run always produced a rejection map and a
weight map, but the *unregistered* sigma reduction's maps were computed
and silently discarded when registration succeeded. Both sets are now
written, distinguished by filename:

- `stacked_rejection_map.jpg`, `stacked_weight_map.jpg` — registered run,
  cropped to the safe-crop region.
- `stacked_unregistered_rejection_map.jpg`,
  `stacked_unregistered_weight_map.jpg` — full-frame unregistered run
  (only emitted when registration also succeeded; otherwise the
  unsuffixed names carry the unregistered maps for backwards
  compatibility).
- `stacked_coverage_map.jpg` — full-frame, per-pixel fraction of
  registered frames with a valid sample. Linear-mapped (0 = black,
  1 = white); every pixel inside the safe crop is 1.0 by construction.

`report.txt` now names these files and states explicitly that the
rejection map is the cosmic-ray / hot-pixel candidate map: sigma clipping
rejects extreme outliers, which is exactly what cosmic rays are. A
dedicated spatial CR filter (L.A.Cosmic-style) and drizzle integration
remain future work; drizzle is a multi-session feature, and full
photometric calibration requires an external catalog (waiving the
stdlib-only rule).

### Auto-detect calibration layout (APPLIED)

`discoverFiles` was previously called on the raw input directory, so
a N.I.N.A./APT-style layout like
`04_emission_nebula_Flame/{Light,Dark,Flat,Bias}/*.fits` produced
`no image files found` because the top-level directory had no
images. `--calib-root` covers a *different* naming convention
(`Darks`, `Flats/<filter>`) and does not help.

`detectCalibrationSubdirs(inputDir)` (new) recognises singular and
plural forms of `Light`, `Dark`, `Flat`, `Bias` case-insensitively,
by exact folder name (so `Lighter/` is not matched). When a folder
is found and `--calib-root` is unset:

- `Light/` (or `Lights/`) replaces the input directory for discovery
- `Dark/`, `Flat/`, `Bias/` become calibration sources unless an
  explicit `--dark`/`--flat`/`--bias` was already given
- calibration directories are appended to the discovery exclude
  list, so `--recursive` cannot stack dark frames as lights

`--calib-root` keeps precedence over auto-detection. The
"no images" error now names the directory that was actually
scanned and lists the accepted formats (JPG/PNG/RAW/FITS).

New self-test: `testDetectCalibrationSubdirs`. Test count
100.

### Native FITS reader (APPLIED)

**FITS string padding bug (fixed 2026-11-12)**: FITS headers pad string values with spaces inside the quotes (e.g., `'RGGB    '`). `parseFITSHeader` originally used `strings.Trim(val, "'")`, which removed the quotes but left the trailing spaces. This caused `baypat` to be `"RGGB    "` instead of `"RGGB"`, so the `switch` in `debayerFITS` fell through to `default` and returned a grayscale frame. Fixed by wrapping in `strings.TrimSpace`. The `--test-debayer` CLI flag was added to diagnose this.

ImageMagick was returning incorrect (thumbnail-sized) dimensions for some FITS files. `parseFITSHeader()` and `decodeFITSNative()` now read the header text directly (NAXIS1/2/3, BITPIX, BSCALE, BZERO) and process the raw data array. Supports 8/16/32-bit integers, -32/-64 floats, and mono/planar RGB.

### `--clean` flag (APPLIED)

By default, `_work` is preserved for fast re-runs. The `--clean` flag removes `_work` (including the cache) after a successful run.

### Deferred to a future session

- **Parallel registration** — needs a producer/worker/collector rewrite with `transforms[]`, `registeredCount`, `regFailures`, cache writes, and per-frame log ordering all synchronized. Requires `testParallelRegistrationMatches` for bit-exact verification.
- **Native FITS reader (`decodeFITSNative`)** — ~150-line header/bitpix parser with BSCALE/BZERO application; fall back to ImageMagick on unrecognised variants.

## Pending patches

Designed and validated but **not yet applied to the repo** as of the last
commit. A future session should either apply them or document why abandoned.

### Plate solver — consumer of gaiaGetTile

**Status**: pending. Depends on A1a/A1b/A1c/A2 (all committed as of
2026-11-13: bc09c85, 5038592, 402063c).

**Design sketch**: after Pass 1 detects stars, take the brightest
N ~ 50 and try to match them against a broad-grid Gaia fetch
(nside=64, all tiles overlapping the frame's probable FOV). Then
refine the transform with the finer tiles (nside=256) and emit a
TAN WCS with SIP distortion coefficients. Requires knowing the
approximate pointing — either from FITS headers (`RA`, `DEC` in the
reference frame) or from a brute-force all-sky scan (expensive).

**nside default APPLIED** (2026-11-13): `gaiaDefaultNside` is now 64
(0.84 deg²/tile), so a 1-5 deg² image needs 1-3 tiles, not 20-100.
`parseArgs` and `printUsage` now derive from the const so the two
cannot drift; `testParseArgs` asserts the default reaches the parsed
struct. `--gaia-nside 256` still works as an override. The plate
solver session inherits nside=64 as its working resolution.

**Do not start** until the A1a/A1b/A1c/A2 sequence is green in CI.

### Weighted median (future work)

**Problem**: `--method median` still produces a plain (unweighted) median.
That is a documented limitation of the weighting patch, not a bug — weighted
medians need a different algorithm (sort by value, walk CDF until cumulative
weight >= 0.5) and different rejection criteria, and are not yet worth the
complexity.

**Fix** (future work): implement a weighted median and wire it into the
`case "median":` branch of `reduceTile`.

### `--spikes-min-count` — APPLIED

**Problem**: `--spikes-percent 0.2` selects 0 or 1 stars on fields with few
detected stars. M42 got 1 star (626 × 0.002 = 1.25 → 1). Sparse fields
end up with almost no spiked output.

**Fix**: `--spikes-min-count N` (default 5), a hard floor on stars selected.
Floors only when `int(percent × detected) < N`; dense fields unaffected.

**Applied.** `selectSpikeStars` gained a trailing `minCount int`.

### Crop diagnostic for pure-translation drift — APPLIED

**Problem**: `LOW BOX%` fires only when BoxPct < 50%. A frame with 30%
translation but a mostly-full bounding box passes and goes unnamed, even
when it's dragging the crop (M42 case).

**Fix** (APPLIED): `LARGE OFFSET` flag fires when `|drift| > 10% of frame
dimension`, independent of BoxPct. Drift is computed as
`Offset - expected_rotation_offset(Scale, RotationDeg)`, so legitimately
rotated frames (e.g. the Polaris run at 25°+) are not falsely flagged.
The crop-explanation template now distinguishes translation drift from
field rotation (|rotation| > 5°). Self-tests: 93 → 94.

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
- Build: `gofmt -w main.go && go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o skystacker .`
- Test: `./skystacker --selftest` (expect: **all 169 self-tests passed**)
- Post-only re-run: `skystacker --post-only --overwrite <dir>`
- Full re-run: `skystacker --overwrite <dir>`
- Sample data layouts (from validated runs):
  - `/media/jips/Elements/big/fotosAstro/2019-01-25-SanJuan/Orion-135mm`
  - `/media/jips/Elements/big/fotosAstro/2023-04-01-riofrio/m42/JPG`


## Session log: plate-solve integration fixes (2026-11-14)

- **Build**: `runOptions.PlateSolve` / `GaiaOffline` added and wired from `cliOptions`.
- **`fitWCSFromMatches`**: returned the TRANSPOSE of CD (invisible for diagonal CD); fixed, plus CRVAL re-centring iterations so the fit is exact. Test: `wcs fit rotated both parities`.
- **`matchStarsToGaia`**: rewritten. Tangent-plane projection about a robust centre; brightness-sorted pools; both parities (mirrored fields are common); spatial-hash pre-screen over all triangle candidates; descriptor tolerance 0.015 then 0.05; significance floor `max(6, frameStars/4)` inliers so unrelated lists return ok=false.
- **HEALPix tile polygon (A1a.5 RESOLVED)**: `ang2pix`/`pix2ang` were already correct vs healpy; the "0.15 deg west" discrepancy came from the old centre+/-half-side box in `healpixTileCorners`. Replaced with the true boundary (16 vertices).
- **`solvePlate`**: cache-first tile loading, then network for misses; gives up after 2 consecutive failures; honours `--gaia-offline`.
- **KNOWN LIMITS (not fixed)**: (1) pipeline calls `solvePlate(..., 0, 0, ...)` so it is always BLIND around RA/Dec 0,0 and cannot solve real fields: needs a pointing seed (e.g. `--plate-center RA,DEC` or FITS header RA/DEC). (2) Matcher assumes the frame covers a large fraction of the Gaia pool (top-40 by brightness each side); a small field inside a ~0.9 deg tile returns "no consistent transform". Needs local-neighbourhood triangles or a pointing+scale seed.

- **2026-11-14 (later)**: `plate solve blind` self-test fixed. It was not hermetic (unseeded scan tiles were fetched live from Gaia, adding unrelated real stars) and, with a fixture away from RA/Dec 0,0, could never succeed because blind mode only scans the 8 nside=64 tiles around (0,0) (RA 359.3-0.7 deg, Dec +/-1.2 deg). The test now forces `plateGaiaOffline` and moves the fixture's sky position (CRVAL) into that patch, keeping pixel geometry. NOTE: "blind" mode is therefore a local scan, not a whole-sky solve.

## Session log: --plate-center, crop-explanation validity, --png/--bmp (2026-11-14)

- **`--plate-center RA,DEC` APPLIED**: pipeline no longer calls
  `solvePlate(..., 0, 0, ...)` unconditionally. Priority: `--plate-center`
  flag > reference frame's FITS-header `RA`/`DEC` > blind (with a loud
  warning, because blind mode only scans the 3x3 nside=64 neighbourhood
  around (0,0) and will fail on a real field). `parseFITSHeader` now
  returns `(raRad, decRad)`; the three call sites (`decodeImageConfig`,
  `decodeFITSNative`, `testFITSFileDebayer`) discard them with `_, _`.
- **Crop explanation validity override APPLIED**: `runPipeline` counts
  pixels where `validCount[p] < registeredCount` right after
  `findSafeCrop`. If >0.5% of the frame is excluded and no frame is a
  geometric limiter, the report says "Crop limited by N pixels (P%) ...
  likely calibration-negative or out-of-bounds resampling" instead of
  "All registered frames cover". Precedence: explicit limiter frames
  win; otherwise the validity message wins over the "all cover" line.
- **`--png` / `--bmp` APPLIED**: both write alongside `stacked_mean.jpg`
  in the registered and unregistered branches. `writePNGPreview` reuses
  `image/png` and the same gamma-0.5 LUT as the JPEGs so the two are
  directly comparable. `writeBMP24` is a hand-written uncompressed
  24-bit BMP (BGR, bottom-up, 4-byte row padding).
- **Stale `--help` spike defaults FIXED**: length 100→10, intensity
  0.7→0.3, arms 6→4. `parseArgs` was already correct; only `printUsage`
  lied. `testParseArgs` now asserts the parser defaults (10/0.3/4) so
  the two cannot drift again.
- Tests: `testPlateCenterFlagReachesSolver`, `testCropExplanationMentionsValidity`,
  `testPNGBMPSnapshot`. `testParseArgs` extended in place (no count
  change). Test count 151 -> 154.

### Run 4 (2026-11-14): Horsehead (Barnard 33, ~1200mm, narrow field), 5 × JPG (from 46, via `--limit 5`), 2400×1600

- Registration: 5/5 frames, reference frame IMG_9158.JPG (100 stars).
- Runtime: 1 m 08 s — a `--limit 5` smoke test on the 46-frame session;
  the `--limit N` flag (added same session) cut it from ~30 min.
- Post-processing star-removal count: 583 detected stars.
- **Plate solve with `--plate-center 85.24785,-2.45811` (Barnard 33)
  SUCCEEDED.** Log lines:
  - `Plate solve seed: flag (--plate-center)`
  - `gaia: tile nside=64 ipix=22854 [fetched 231 stars]`
  - `Plate solved: CRVAL=(85.077297, -2.369846) RMS=1.30 arcsec N=71/100 (header)`
  - `plate_solution.json` written (439 bytes).
  - True center `85.0773, -2.3698` is 0.19° from the Barnard 33 seed —
    the target sits near (but not at) the frame center, exactly as
    expected for a 1200mm APS-C frame (~1.1° × 0.75° FOV). 71/100
    matches is a strong, unambiguous solve.
- **This closes done criterion 1 from the 2026-11-14 session.**
  The contrast with the ON run is instructive:
  - **ON** is a 5.4° × 3.6° (~19 deg²) field. One nside=64 tile covers
    ~4% of it. The matcher (`minInliers = max(6, frameStars/4) = 25`)
    cannot overlap the top-40 bright stars of a 19-deg² field with the
    245 Gaia stars of one 0.84-deg² tile. It failed with `0 matched
    pairs`, exactly as documented in `PROJECT_CONTEXT.md`'s
    "KNOWN LIMITS" from the 2026-11-14 session. This is the Tier 2
    single-tile matcher limitation, not a bug.
  - **Horsehead** is ~1.1° × 0.75° (~0.83 deg²). One nside=64 tile
    covers the entire frame; the top-40 bright stars and the tile's
    231 Gaia stars overlap heavily. It solved with 71/100 matches,
    RMS 1.30 arcsec.
  - `--plate-center` itself was correct in both cases (both runs log
    `Plate solve seed: flag (--plate-center)` and fetch a single
    correct tile). The difference is purely the frame FOV vs the
    tile's 0.84 deg² coverage.
- `--png` / `--bmp` outputs also written cleanly (no `WARNING: png:` /
  `WARNING: bmp:` lines).

- **Re-confirmed on a 3-frame subset** (same reference IMG_9158.JPG,
  `--limit 3`): identical `CRVAL=(85.077297, -2.369846) RMS=1.30 arcsec
  N=71/100`, and `plate_solution.json` written again. Two independent
  observations that the solution depends only on the reference frame's
  star list, not on the accumulated stack. This run also exercised the
  Gaia disk cache (`gaia: tile nside=64 ipix=22854 [cache hit]`),
  dropping end-to-end wall time from 1 m 08 s → 30 s on identical
  input. `--png` / `--bmp` outputs also written cleanly.

## Session log: Tier 0 documentation and hygiene (2026-11-14)

**Commit:** `1f1f875` — `tier0: docs — plate solving is wired; guard spike defaults in printUsage`

- **Tier 0 (XS) CLOSED.** Documentation-only patch; no code behaviour change, self-test count unchanged at **154** (`all 154 self-tests passed` printed twice during the two-step apply). `go vet` exited 0 both times. `git push` accepted `d6b1b46..1f1f875  HEAD -> main`.

- **Stale `--help` spike defaults — already correct in source; closed without a numeric edit.** `parseArgs` already set `--spikes-length 10.0`, `--spikes-intensity 0.3`, `--spikes-arms 4`; `printUsage` already printed those values; `testParseArgs` already asserted all three. This session added a comment above the `--spikes-*` block in `printUsage` making the contract explicit ("NOTE: the numeric defaults below MUST match parseArgs; testParseArgs asserts (SpikesLength=10.0, SpikesIntensity=0.3, SpikesArms=4) so any future drift fails the self-test suite."). Verified by `grep -n "MUST match parseArgs; testParseArgs" main.go` → line 506.

- **TECHNICAL_SPEC.md §2 `Excluded` — corrected.** Plate solving removed from the exclusion list. New exclusion text names what is actually still open: *applying* SIP to the resampler, higher-order distortion beyond SIP order 2, and whole-sky blind solving. Verified by `grep -n "blind mode is a 3x3 nside=64 local scan" TECHNICAL_SPEC.md` → line 29.

- **TECHNICAL_SPEC.md §4.5 — corrected.** Dropped "no pipeline consumer yet". Now states `--plate-solve` consumes the layer, `plate_solution.json` is written, SIP order-2 coefficients are stored alongside the WCS; *applying* SIP to the resampler is the remaining gap. Verified by `grep -n "applying SIP to the resampler is not yet wired" TECHNICAL_SPEC.md` → line 131.

- **TECHNICAL_SPEC.md §2 — self-test count source-of-truth note.** The "154 embedded self-tests" line now says the count printed by `./skystacker --selftest` (i.e. `runSelfTests` in `main.go`) is authoritative.

- **README.md — two corrections.** (1) "Recently implemented" Gaia-layer bullet: "No pipeline consumer yet" removed; now says the layer is consumed by `--plate-solve`, seeded with `--plate-center RA,DEC` or a reference frame with FITS `RA`/`DEC`, and that `plate_solution.json` is written. (2) Tier 2 plate-solving bullet reworded: was "wired via `--plate-solve` but always blind around (0,0) until Tier 1's `--plate-center` lands" (stale — `--plate-center` landed 2026-11-14); now: solver + data layer committed and tested, `--plate-center` and FITS-header auto-seed wired, `plate_solution.json` in the output bundle, remaining work is *applying* SIP and wiring the solver into `--post-only`.

- **COMPARISON.md — three corrections.** (1) "Calibration and registration" table, `SkyStacker` row, `Plate solving` column: `–` → `Partial (wired, seeded)` (line 30). `Distortion corr.` stays `–` because SIP is *fit* but not *applied*. (2) "Where SkyStacker sits" → `Gaps` bullet (line 100): "no plate solving / WCS / SIP — … pipeline consumer pending" replaced with the wired + SIP-fit-only text. (3) "Post-processing and automation" table, `SkyStacker` row, `Output formats` cell: `TIFF / JPEG / raw` → `TIFF / JPEG / PNG / BMP / raw` (line 68; `--png` / `--bmp` landed 2026-11-14; `--fits` is Tier 2 and intentionally not listed). Note: the first patch attempt used a regex keyed on the row label containing "plate solv"/"astrometric"/"WCS" and missed because the label is just `**SkyStacker**`; the corrected patch matches the row verbatim.

- **Backups.** Two rounds created `main.go.before_tier0_docs.bak`, `TECHNICAL_SPEC.md.before_tier0_docs.bak`, `README.md.before_tier0_docs.bak`, `PROJECT_CONTEXT.md.before_tier0_docs.bak`, then `COMPARISON.md.before_tier0_docs2.bak` and `PROJECT_CONTEXT.md.before_tier0_docs2.bak`. All untracked; the commit used explicit `git add main.go TECHNICAL_SPEC.md README.md COMPARISON.md`, so they stay out of history.

- **Test count:** 154 → 154. No new tests; `testParseArgs` already covered the spike defaults, and the doc-only patch adds no code paths.

### Still open after Tier 0

Tier 0 is closed. The next actionable roadmap items are **Tier 1 weighted median** or **Tier 1 SER video input**. Two follow-up hygiene items surfaced during this pass and are recorded here so the next session does not rediscover them:

1. **README "Planned Potential Features" Tier 1 list still contains `--plate-center RA,DEC`** (and its `Test: testSolvePlateSeededCLI` line), even though both landed 2026-11-14. Remove the bullet in a future hygiene pass; the test is actually named `testPlateCenterFlagReachesSolver`, not `testSolvePlateSeededCLI`, so the stale bullet is doubly wrong.

2. **`.gitignore` does not exclude `*.bak`.** Every patch by this project leaves a `main.go.before_*.bak` (and now `.md.before_*.bak`) next to the source. A single `*.bak` line in `.gitignore` prevents accidental `git add -A` commits of these backups. Not user-visible; cheap and self-contained.

### "Plate solve blind" note (carried forward, unchanged by Tier 0)

The `plate solve blind` self-test remains hermetic (forces `plateGaiaOffline = true`, fixture's sky position moved into the blind-scan patch, cache pre-seeded). Tier 0 changed no code in that path. The bigger caveat — that "blind" mode is a 3x3 nside=64 local scan, not a whole-sky solve — is now stated in `TECHNICAL_SPEC §2 Excluded` and in `COMPARISON.md` Gaps; it was previously only in the 2026-11-14 session log and in `printUsage`'s `--plate-center` description.

<!-- SYNTHCAL_V1_MARKER -->

## Session log: synthetic-calibration guardrails (2026-11-14)

**Commit:** `5be6865` — `synthcalib: refuse synthetic bias/dark/flat; add recovery report`

**Scope:** the guardrail half of the synthetic-calibration research
pass. No synthetic frame is generated; this patch only makes the
software *refuse* to generate one and *report* what is recoverable
from the lights alone. The research prompt's section E is mandatory
because silent synthetic calibration is worse than no calibration —
this is that section, landed.

**New flags (`parseArgs` + `printUsage` + `runOptions` + `cliOptions`):**
- `--synthetic-calibration` — logs the per-component recovery report
  at pipeline start (reaches `report.txt` via the normal Notes path).
- `--synthetic-calibration-report` — prints the report to stdout and
  exits, before any discovery or decode (see `main()` order: it runs
  before `--selftest`'s post-processing branch and before
  `runPipeline`).
- `--synthetic-dark`, `--synthetic-flat`, `--synthetic-bias` — each
  triggers `checkSyntheticCalibRefusal()` in `main()` and exits 1
  with a `SYNTHETIC CALIBRATION REFUSED` error naming the missing
  real exposure and the correct `--dark`/`--flat`/`--bias` flag.
  The refusal is the **first** thing the user sees, not a stack
  that silently produced a wrong master.

**Guardrail table (`syntheticCalibComponents`):** 10 rows, one per
component in `SYNTHETIC_CALIBRATION_RESEARCH.md` section B. Each row
has `Name`, `Recoverable` ∈ {yes, partial, no}, `Requires`, `Path`.
`testSyntheticCalibRefusalMessage` asserts every field is non-empty
and every `Recoverable` is one of the three values, so a half-filled
row fails `--selftest` rather than shipping a vague "medium".

Recoverability verdicts (from the research pass; the table in the
seed doc's section 5 is qualitative and is **not** reproduced as-is):
- bias pedestal: partial (EXIF/header metadata only)
- spatial bias pattern: no
- read noise: no (not correctable, only characterisable)
- dark current: no
- hot pixels: yes (sigma-clipping + rejection map already finds them)
- amp glow: no
- vignetting: partial (`--local-normalize` recovers the fixed component)
- dust shadows: no
- pixel-response non-uniformity: no
- illumination gradient: partial (`--bg-degree` on the stack)

**Tests:** 154 → 163 (the `--selftest` count is authoritative per
`TECHNICAL_SPEC.md` §2; the commit message says 161, which was the
projected count before the tree drifted by two; the real number is
163). New: `testSyntheticCalibGuardrailEmpty`,
`testSyntheticCalibGuardrailFullSet`,
`testSyntheticCalibGuardrailDarkOnly`,
`testSyntheticCalibGuardrailFlatOnly`,
`testSyntheticCalibGuardrailBiasOnly`,
`testSyntheticCalibRefusalMessage`, `testSyntheticCalibParseArgs`.
The last three are the `--plate-center` / crop-validity / PNG-BMP
tests, which were already in the tree before this patch.

**No change to** `TECHNICAL_SPEC.md` §4.1 (median description stays
correct — the unweighted algorithm text is untouched), `go.mod` (no
new deps), `testdata/` (no additions).

### Still open after this session

The implementation half of the research pass — `--synthetic-calibration`
*reporting* is live, but the *empirical reconstruction* roadmap is not
started:

- **`ROADMAP_SYNTHETIC_CALIBRATION.md` does not exist in the repo.**
  The research prompt's section D ("Recommended implementation order",
  Tier 1-4) is the input for the next implementation session, but it
  lives only in the prompt. Create it from section D before the next
  implementation session, with each Tier 1 item stated as a
  user-visible flag, a falsifiable acceptance test, and a
  `test…` self-test name. The section-D items that the guardrail
  table already points at are `--bpm` (Tier 2), `--self-flat` from
  dithered lights (Tier 3), and the confidence map (Tier 3).
- **`SYNTHETIC_CALIBRATION_RESEARCH.md` has not received the
  "Deep-research pass (YYYY-MM-DD)" section** required by the prompt.
  That section (A-E) is the *output* of the research session, not the
  input; this session only consumed section E's conclusion. The
  research session's own deliverable is still owed.
- The `.gitignore` `.bak` item from the Tier 0 cleanup is still open
  (this session added two more `.bak` files:
  `main.go.before_synth_calib.bak` and
  `PROJECT_CONTEXT.md.before_synth_calib.bak`; both are untracked
  and will stay untracked because the commits use explicit
  `git add <file>`, but a single `*.bak` line would prevent an
  accidental `git add -A`).

<!-- RESEARCH_DOCS_V1_MARKER -->

## Session log: research and roadmap documents (2026-11-14, later)

Two docs, no code change. Both close items the previous synthcalib
session left open.

- **`SYNTHETIC_CALIBRATION_RESEARCH.md` created** with the required
  `## Deep-research pass (2026-11-14)` section on top (A corrections,
  B ten-component evidence, C SkyStacker fit, D Tier 1-4, E refusal
  contract). The seed doc is preserved below the section for
  traceability; section A says which seed claims are over-claimed
  (the §5 reliability table is the main one).
- **`ROADMAP_SYNTHETIC_CALIBRATION.md` created** from section D. Tier 1
  is `--black-level auto` + read-noise sigma in `report.txt`; Tier 2
  is `--bpm PATH` + `--self-flat`; Tier 3 is the confidence map;
  Tier 4 is the refused synthetic generator, recorded only so the next
  session does not re-open it.

**Self-test count unchanged at 163.** No `main.go` change. No new
`go.mod` deps. No `testdata/` additions.

### Corrected divergence vs. `research/RESEARCH_SYNTHESIS.md`

`research/RESEARCH_SYNTHESIS.md` is **stale** on two points:
- It says "version reported as `0.4.1`"; `main.go` says `0.5.1`.
- Its §4 "Known Limitations" says "No drizzle, local normalization,
  plate solving, or multi-night intelligence yet"; three of those four
  are shipped (`--local-normalize`, `--plate-solve`, and the
  normalization path). Only drizzle is genuinely missing.

The next session that touches the research folder should re-anchor
`RESEARCH_SYNTHESIS.md` against `main.go`, not treat it as
authoritative. This is recorded here so the next session does not
re-discover it.

### Still open after this session

- `.gitignore` still does not exclude `*.bak` (this session added two
  more: `PROJECT_CONTEXT.md.before_research_docs.bak` and, if you ran
  Step 1's `cp`, `PROJECT_CONTEXT.md.before_synth_calib.bak` from last
  session — both untracked).
- `research/RESEARCH_SYNTHESIS.md` version and limitations are stale
  (see above).
- The **implementation** half of the synthetic-calibration roadmap is
  not started. Tier 1 of `ROADMAP_SYNTHETIC_CALIBRATION.md` is the
  next actionable item; it is two small commits, each with a named
  `test…` case.



## Session log: Tier 0 documentation and hygiene (2026-11-14)

**Commit:** `1f1f875` — `tier0: docs — plate solving is wired; guard spike defaults in printUsage`

- **Tier 0 (XS) CLOSED.** Documentation-only patch; no code behaviour change, self-test count unchanged at **154** (`all 154 self-tests passed` printed twice during the two-step apply). `go vet` exited 0 both times. `git push` accepted `d6b1b46..1f1f875  HEAD -> main`.

- **Stale `--help` spike defaults — already correct in source; closed without a numeric edit.** `parseArgs` already set `--spikes-length 10.0`, `--spikes-intensity 0.3`, `--spikes-arms 4`; `printUsage` already printed those values; `testParseArgs` already asserted all three. This session added a comment above the `--spikes-*` block in `printUsage` making the contract explicit ("NOTE: the numeric defaults below MUST match parseArgs; testParseArgs asserts (SpikesLength=10.0, SpikesIntensity=0.3, SpikesArms=4) so any future drift fails the self-test suite."). Verified by `grep -n "MUST match parseArgs; testParseArgs" main.go` → line 506.

- **TECHNICAL_SPEC.md §2 `Excluded` — corrected.** Plate solving removed from the exclusion list. New exclusion text names what is actually still open: *applying* SIP to the resampler, higher-order distortion beyond SIP order 2, and whole-sky blind solving. Verified by `grep -n "blind mode is a 3x3 nside=64 local scan" TECHNICAL_SPEC.md` → line 29.

- **TECHNICAL_SPEC.md §4.5 — corrected.** Dropped "no pipeline consumer yet". Now states `--plate-solve` consumes the layer, `plate_solution.json` is written, SIP order-2 coefficients are stored alongside the WCS; *applying* SIP to the resampler is the remaining gap. Verified by `grep -n "applying SIP to the resampler is not yet wired" TECHNICAL_SPEC.md` → line 131.

- **TECHNICAL_SPEC.md §2 — self-test count source-of-truth note.** The "154 embedded self-tests" line now says the count printed by `./skystacker --selftest` (i.e. `runSelfTests` in `main.go`) is authoritative.

- **README.md — two corrections.** (1) "Recently implemented" Gaia-layer bullet: "No pipeline consumer yet" removed; now says the layer is consumed by `--plate-solve`, seeded with `--plate-center RA,DEC` or a reference frame with FITS `RA`/`DEC`, and that `plate_solution.json` is written. (2) Tier 2 plate-solving bullet reworded: was "wired via `--plate-solve` but always blind around (0,0) until Tier 1's `--plate-center` lands" (stale — `--plate-center` landed 2026-11-14); now: solver + data layer committed and tested, `--plate-center` and FITS-header auto-seed wired, `plate_solution.json` in the output bundle, remaining work is *applying* SIP and wiring the solver into `--post-only`.

- **COMPARISON.md — three corrections.** (1) "Calibration and registration" table, `SkyStacker` row, `Plate solving` column: `–` → `Partial (wired, seeded)` (line 30). `Distortion corr.` stays `–` because SIP is *fit* but not *applied*. (2) "Where SkyStacker sits" → `Gaps` bullet (line 100): "no plate solving / WCS / SIP — … pipeline consumer pending" replaced with the wired + SIP-fit-only text. (3) "Post-processing and automation" table, `SkyStacker` row, `Output formats` cell: `TIFF / JPEG / raw` → `TIFF / JPEG / PNG / BMP / raw` (line 68; `--png` / `--bmp` landed 2026-11-14; `--fits` is Tier 2 and intentionally not listed). Note: the first patch attempt used a regex keyed on the row label containing "plate solv"/"astrometric"/"WCS" and missed because the label is just `**SkyStacker**`; the corrected patch matches the row verbatim.

- **Backups.** Two rounds created `main.go.before_tier0_docs.bak`, `TECHNICAL_SPEC.md.before_tier0_docs.bak`, `README.md.before_tier0_docs.bak`, `PROJECT_CONTEXT.md.before_tier0_docs.bak`, then `COMPARISON.md.before_tier0_docs2.bak` and `PROJECT_CONTEXT.md.before_tier0_docs2.bak`. All untracked; the commit used explicit `git add main.go TECHNICAL_SPEC.md README.md COMPARISON.md`, so they stay out of history.

- **Test count:** 154 → 154. No new tests; `testParseArgs` already covered the spike defaults, and the doc-only patch adds no code paths.

### Tier 0 roadmap-list cleanup (2026-11-14, later)

- **README "Planned Potential Features" → Tier 0 block removed.** The list still advertised the two Tier 0 (XS) items as open, even though commit `1f1f875` had closed both. Removed `### Tier 0 — Documentation and hygiene (XS)` and its two bullets; `### Tier 1 — Small, self-contained (S)` is now the first roadmap subsection.

- **README Tier 1 `--plate-center RA,DEC` bullet removed.** The flag landed 2026-11-14; the bullet's `Test: testSolvePlateSeededCLI` was also wrong — the actual test is `testPlateCenterFlagReachesSolver`.

- **Commit:** committed in the same change set as this log (the doc-only `git add` first listed only `main.go`/`TECHNICAL_SPEC.md`/`README.md`/`COMPARISON.md`, so `PROJECT_CONTEXT.md` was appended and committed separately in a follow-up).

- **Test count:** 154 → 154 (docs only).

### Still open after Tier 0

Tier 0 is closed. The next actionable roadmap items are **Tier 1 weighted median** or **Tier 1 SER video input**. Two follow-up hygiene items surfaced during this pass and are recorded here so the next session does not rediscover them:

1. **`.gitignore` does not exclude `*.bak`.** Every patch by this project leaves a `main.go.before_*.bak` (and now `.md.before_*.bak`) next to the source. A single `*.bak` line in `.gitignore` prevents accidental `git add -A` commits of these backups. Not user-visible; cheap and self-contained.

2. **README "Planned Potential Features" Tier 1 list may still contain other already-applied bullets** — a re-audit against the current `main.go` is worth doing in the next hygiene pass.

### "Plate solve blind" note (carried forward, unchanged by Tier 0)

The `plate solve blind` self-test remains hermetic (forces `plateGaiaOffline = true`, fixture's sky position moved into the blind-scan patch, cache pre-seeded). Tier 0 changed no code in that path. The bigger caveat — that "blind" mode is a 3x3 nside=64 local scan, not a whole-sky solve — is now stated in `TECHNICAL_SPEC §2 Excluded` and in `COMPARISON.md` Gaps; it was previously only in the 2026-11-14 session log and in `printUsage`'s `--plate-center` description.
## Session log: v0.5.1 docs cleanup (2026-11-14)

- **Version bump 0.5.0 -> 0.5.1** in `main.go` (`var version`) and the
  Gaia `User-Agent` string. The release workflow passes
  `-X main.version=${TAG}`, so tagged builds override this; the bump
  is for the `--version` / banner output of untagged builds only.
  `cacheVersion`, `gaiaCacheVersion` and `buildDate` are unchanged.
- **README Tier 1 roadmap cleanup.** Removed the shipped
  `--png`/`--bmp` and weighted-median bullets from
  `### Tier 1 — Small, self-contained (S)`; both items now live
  under "Recently implemented". This is the second half of the
  Tier 0 follow-up item #2 ("re-audit Tier 1 against `main.go`").
- **Recently implemented**: added an explicit
  "Extra output formats — PNG and BMP (v0.5.1)" bullet; the
  weighted-median bullet is unchanged.
- No code behaviour change beyond the version string. Self-test
  count remains **156**; gofmt and go vet stay clean.

## 2026-11-14 — Tier 1 of synthetic-calibration roadmap

Implements `ROADMAP_SYNTHETIC_CALIBRATION.md` §Tier 1 in one commit
(no code change to the guardrails, no `TECHNICAL_SPEC.md` change).

### 1.1 `--black-level auto`

- New `--black-level` string flag. Default `""` (off). Only accepted
  non-empty value is `auto` (case-insensitive; `parseArgs` normalizes
  to `"auto"`). Anything else is a parse error:
  `--black-level must be "auto" or empty (off), got %q`.
- `auto` reads the black-level pedestal from file metadata:
  - JPEG/RAW: EXIF tag `0x14A` via new helper
    `readExifBlackLevel` (SHORT / LONG / RATIONAL, averaged across
    CFA channels, normalized to [0,1]).
  - FITS: `BLACK_LVL` card via `fitsBlackLevelFor(path)`, which reads
    the same header string `parseFITSHeader` already scans.
- `subtractBlackLevel(f, level)` subtracts the normalized pedestal
  from every pixel, clamped to [0,1]. No-op for level <= 0 or NaN/Inf.
- Called in Pass 1 (after `calibrate`) and Pass 2 (after
  `calibrate`, before resample), so unregistered, registered, and
  spill paths all see the corrected pixel values.
- `report.txt` gains a `Black level: <tag> = <value>` line under the
  `Sum TIFF normalisation:` block. Tag is `EXIF BlackLevel` or
  `FITS BLACK_LVL`.
- New self-test: `testSubtractBlackLevel`. Covers:
  - Positive case: subtract a known 0.05 pedestal, assert 1st
    luminance percentile is within 1/65535 of 0.
  - Negative control: same frame without subtraction is NOT within
    1/65535 of 0.
  - `level = 0` is a no-op; over-subtraction clamps to 0.
  - EXIF tag decode: 26-byte TIFF with tag 0x14A (SHORT, count=1,
    value 0x0800) → returns 2048/65535.

**Design decision on `parseFITSHeader`.** Its signature returns a
12-tuple used by three callers (`decodeImageConfig`,
`decodeFITSNative`, `referenceFITSSeed`) and changing it would touch
all three. Instead the `BLACK_LVL` value is published through
package-level vars `lastFITSBlackLevel` / `lastFITSHasBlackLvl`,
guarded by `fitsBlackLevelMu`, and read back via `fitsBlackLevelFor`.
A follow-up should change `parseFITSHeader` to return a struct.

### 1.2 Report per-frame read-noise sigma

- `report.txt` gains a `BgSigma` column in the frame-diagnostics
  table. Value comes from `frameDiag.BgSigma`, which is set from
  `accepted[i].bgNoise` in Pass 2 (that is the same value
  `estimateFrameBgNoise` computes in Pass 1 and stores in the star
  cache, so no new computation).
- `frameDiag.BgNoise` was renamed to `BgSigma`. The
  `starCache.BgNoise` field is a separate type and is untouched.
- Column order: `… Foot%  BgSigma  Note`. Header is `%-9s` for
  `BgSigma`, row is `%9.5f`.
- New self-test: `testFrameBgNoiseReported`. Covers:
  - Two 256×256 frames with Gaussian σ = 0.02 and σ = 0.04.
  - Estimated σ(0.02) is within 10% of 0.02.
  - Estimated σ(0.04) is within 10% of 0.04 AND strictly greater
    than σ(0.02).
  - A `processingReport` with one `frameDiag{BgSigma: 0.02000}`
    written via `writeTo` contains both the string `BgSigma`
    (header) and `0.02000` (value).

### Constraints check

- Stdlib only. `go.mod` unchanged. `testdata/` unchanged.
- No network at `--selftest` time.
- `TECHNICAL_SPEC.md` §4.1 untouched (median description unchanged).
- The three `--synthetic-*` refusal flags and the
  `testSyntheticCalib*` guardrails are unchanged and still pass.

### Self-test count

**163 → 165.** Exactly two new test cases in `runSelfTests`,
matching the two named in the roadmap (`testSubtractBlackLevel`,
`testFrameBgNoiseReported`). `gofmt -l main.go` and `go vet ./...`
clean at commit time.

### Handoff to next session

- **Tier 2.1 `--bpm PATH`** (bad-pixel map) is the next actionable
  item in `ROADMAP_SYNTHETIC_CALIBRATION.md`.
- Follow-up on 1.1: change `parseFITSHeader` to return a struct and
  drop the `fitsBlackLevelFor` side channel.
- Follow-up on 1.2: nothing outstanding; the column is report-only.

<!-- BL_BGSIG_V1_MARKER -->

## Session log: doc reconciliation + next-session prompt (2026-11-15)

Documentation-only pass. No `main.go` change; `gofmt -l` empty and
`go vet ./...` clean. Self-test count verified by running
`./skystacker --selftest` → **`all 165 self-tests passed`**.

- **Stale header counts FIXED.** The document header still claimed
  "last updated 2026-11-11" and "Self-test count: 154 passing", and the
  Recovery section still said "all 154 self-tests passed". Tier 1
  (`--black-level auto`, `BgSigma`) landed after those lines were last
  touched, so the authoritative count is **165**. Updated line 1, the
  header count, and the Recovery expectation to 165.
- **`NEXT_SESSION_PROMPT.md` refreshed for Tier 2.1.** The local
  (`.gitignore`d) prompt still described Tier 1, which is shipped. It
  now scopes **Tier 2.1 `--bpm PATH`**, states the expected pre-patch
  count (165 → 166), the idempotent-patch protocol, and the stop
  conditions.
- **Known doc hygiene (not fixed here).** `PROJECT_CONTEXT.md` contains
  two "Session log: Tier 0 documentation and hygiene (2026-11-14)"
  sections; the earlier one is superseded by the later one (which adds
  the "Tier 0 roadmap-list cleanup" subsection). Remove the earlier
  duplicate in a separate docs commit; recorded in
  `NEXT_SESSION_PROMPT.md` under "Housekeeping".

<!-- DOC_RECON_V1_MARKER -->

## Session log: Tier 2.1 `--bpm PATH` (2026-11-15)

New flag `--bpm PATH` applies a bad-pixel map to every input frame.
Mapped pixels are marked invalid (`-1` in all three channels) after
calibration and black-level subtraction, then skipped by the same
valid-count machinery that already handles registration borders.

### Behaviour

- **Formats.** `.fits`/`.fit` files are decoded as an image and every
  non-zero pixel is treated as bad. Any other extension is parsed as
  text: one `x,y` pair per line, with `,`, `;`, space, or tab as the
  separator. Blank lines, `#` comment lines, and inline `#` comments are
  ignored. Coordinates are 0-based native-resolution pixels.
- **Scale.** With `--scale N`, native coordinates are divided by `N` to
  land on the decoded frame grid. Out-of-frame points are ignored.
- **Malformed input is fatal.** Missing files, lines that are not
  `x,y`, non-integers, negative coordinates, and empty maps all return a
  `bad-pixel map:` error before any frame is stacked.
- **`report.txt`** gains `Bad pixel map: <path> (<N> pixels masked)` when
  the flag is active, where `<N>` is the number of distinct in-frame
  pixels masked on the first processed frame.

### Pipeline details

- Both Pass 1 and Pass 2 apply the map after `calibrate()` and after
  `--black-level auto`. This order matters: `calibrate()` clamps to
  `[0,1]`, which would turn the `-1` sentinel into `0`.
- `sampleBilinear` now returns `ok=false` when any of its four source
  pixels is negative, so registration cannot interpolate the sentinel
  into neighbouring pixels.
- The unregistered accumulator now uses `addFrameMasked` with its own
  per-pixel count array and `divideByCount`, so masked samples are
  skipped there too. With no BPM every pixel is valid, so this is
  bit-identical to the previous `addFrame`/`divide` behaviour.
- `reduceTile` already skipped negative samples; the registered mean
  already used `addFrameMasked`; no changes were needed in either path.

### Self-test count

**165 → 166.** One new case, `testBPMMasksPixels`, covers text and FITS
loading, malformed-input rejection, the 3-frame masking scenarios
(no BPM → `0.383333`; mask the bright frame only → `0.10`; mask all →
`0`), neighbour preservation, `--scale` coordinate mapping,
out-of-frame points, CLI wiring, and the `report.txt` line.
`gofmt -l main.go` empty, `go vet ./...` clean, and
`./skystacker --selftest` prints **`all 166 self-tests passed`**.

### Manual verification

- PNG mean path, no registration: hot pixel `0.384314` without BPM,
  `0.0` with BPM; neighbours unchanged at `0.101961`.
- Registered path with synthetic stars: hot pixel `0.0`, stars intact.
- All-FITS workflow: the BPM coordinate follows the same native
  orientation as the input frames (ImageMagick's FITS writer is
  bottom-up, so both the frame and the map agree).
- Missing and malformed maps exit non-zero with a `bad-pixel map:`
  message.

<!-- BPM_V1_MARKER -->

## Session log — 2026-11-15 — calibration master export and self-flat

### `--export-masters`

- Writes `master_<kind>.f32raw` (8-byte LE width/height header + interleaved
  RGB float32, same format as `_work/*.f32raw`) and `master_<kind>.jpg` for
  every non-nil calibration master (bias, dark, flat, self_flat).
- The JPG is a preview only: the shared 1st..99.5th luminance percentile is
  mapped linearly to 0..255, falling back to min/max and then 0..1 when the
  percentiles collapse. The `.f32raw` is the scientific artifact.
- Per-master notes (min, max, channel means) are added to `report.txt`.

### `--self-flat`

- Derives a relative illumination map from >=15 registered dithered lights
  (`selfFlatMinFrames = 15`) in reference coordinates. It is applied after
  registration, so it corrects mosaic-level vignetting, not sensor-level
  fixed-pattern noise.
- Per-frame sky level is normalised with the median luminance
  (`frameLuminanceMedian`); the aligned frames are reduced with a
  one-iteration sigma-clipped mean; the result is normalised per channel to
  mean 1, clamped to [0.05, 20], and rejected if the center/edge ratio is
  outside [0.5, 5.0].
- An explicit `--flat` always wins; `--self-flat` requires registration.
- With `--method mean`, Pass 2 spills the frames instead of accumulating, the
  flat is built, and the frames are re-accumulated with the flat. With
  `--method median|sigma`, the flat is passed to `reduceTile` and applied
  inside the reduction.
- `master_self_flat.f32raw` is always written when the flat is applied;
  `master_self_flat.jpg` is written only with `--export-masters`.

### Orion-135mm verification

`./skystacker assets/Orion-135mm --self-flat --export-masters --stack-only
--method mean --no-cache` on 66 frames (2395x1583 crop):

- Self-flat: 66 frames, center/edge=1.2512, residual RMS=0.19360.
- The recovered center/edge matches the measured vignetting in the lights
  (1.240..1.253).
- The corrected mean preview center/border goes from 1.134 (uncorrected) to
  1.000, confirming the relative-illumination correction.
- The master preview center/border is ~1.9 because the same map also contains
  astrophysical structure (M42), which is the expected limitation of a flat
  built from lights. The `.f32raw` is the scientific artifact.
- `--method sigma --limit 20` was also verified end-to-end (self-flat 1.2497,
  corrected mean center/border 1.000), exercising the `reduceTile` path.

### Self-tests

**166 → 169.** New cases: `testSelfFlatEligible`, `testSelfFlatRecovery`
(synthetic vignetting + moving outliers, RMS < 0.03), and
`testExportCalibrationMasters` (6 files, 3 notes, vignetting/dust/hot pixel
visible in the previews). `gofmt -l main.go` empty, `go vet ./...` clean, and
`./skystacker --selftest` prints **`all 169 self-tests passed`**.

<!-- SELF_FLAT_EXPORT_V1_MARKER -->

### Handoff to next session (2026-11-15, supersedes the block above)

- **Tier 2.1 (`--bpm`) and Tier 2.2 (`--self-flat` + `--export-masters`)
  are shipped.** Self-test count is **169**.
- **Next actionable item:** validate the absolute calibration path
  (`--bias/--dark/--flat` + `--export-masters`) on real data outside the
  repo (OrionNebula CR2 via `dcraw -4 -T`; Flame FITS). See the refreshed
  `NEXT_SESSION_PROMPT.md`.
- **Then:** Tier 3.1 per-component confidence map in `report.txt`.
- Scientific caveat: a self-flat built from lights can absorb extended
  nebulosity (observed on M42); see
  `research/04-calibration-and-self-flat.md`.

### Self-flat revision analysis (2026-11-15, no code change)

- **v1 is unsafe on extended emission.** The current `--self-flat` is built
  from `spillPaths2` (registered/reference coordinates), so aligned stars
  and nebulosity look like illumination. Orion-135mm reaches ~1.9 around
  M42 and can produce dark halos. The effect is not literally dividing the
  stack by itself, but the scene is a common component and is suppressed.
- **Specified successor:** sensor-space, full-frame estimator from
  calibrated *unregistered* frames. Per-frame robust normalisation; source
  mask + 10-15 px dilation; 64/96 px cells; robust cell background
  (SExtractor mode, low quantile as diagnostic); 5x5 median/MAD cell
  rejection; degree-3/4 Chebyshev/Legendre fit with IRLS Huber/Tukey;
  normalise and validate; apply before resampling. Correct the
  unregistered mean/median/sigma by post-division. This removes the v1
  mean re-accumulation and the `flatCorrection`/`frameScales` reduction
  parameters.
- **Documented limits:** a stable sky gradient is degenerate with the flat
  and will be absorbed; a large nebula with ~13 px dithers remains
  partially degenerate; dust is out of scope; the sRGB-encoded pipeline
  makes the division approximate (gamma-aware correction is Phase 5).
- **Roadmap:** `ROADMAP_SYNTHETIC_CALIBRATION.md` §2.3, phases 0-7.
  Phase 0 is this analysis; Phase 1 is the sensor-space prototype with
  `--self-flat-model=registered|grid` for A/B. Default stays `registered`
  until the Phase 4 acceptance tests pass, then `grid`.
- **Self-test count unchanged: 169.** No implementation or test changes in
  this revision. `gofmt -l main.go` is empty, `go vet ./...` is clean, and
  `/home/jips/.local/bin/skystacker --selftest` prints
  `all 169 self-tests passed`.
- **Docs updated:** `research/04-calibration-and-self-flat.md` §9-14,
  `ROADMAP_SYNTHETIC_CALIBRATION.md` §2.3, `README.md`,
  `TECHNICAL_SPEC.md`, `prompt-ai.txt`, and this context block.
- **Next session:** implement Phase 1 only after user approval; do not
  commit these docs unless asked.

## Consolidated context snapshot (2026-11-15)

This is the current handoff. Earlier session logs are retained as history;
where they disagree with this snapshot, this snapshot wins. Nothing in this
document authorizes a commit or push.

### Project and environment

- SkyStacker is a local-first astrophotography engine: a single Go file
  (`main.go`, ~18k lines), standard library only, no external Go modules.
- Repository: `~/datos/datos-juan/proyectos/SkyStacker/`
  (`https://github.com/computerphysicslab/skystacker`).
- Version in the header: v0.5.1. Linux, bash, Go toolchain installed.
- The repo is usually clean; generated binaries, `assets/Orion-135mm/`, and
  `NEXT_SESSION_PROMPT.md` are gitignored. `prompt-ai.txt` is tracked.
- `research/` and `PROJECT_CONTEXT.md` are append-only: add sections, never
  delete or shorten prior findings.

### Canonical build, test, install

```bash
gofmt -w main.go
go vet ./...
go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o /home/jips/.local/bin/skystacker .
/home/jips/.local/bin/skystacker --selftest
```

- Canonical binary: `/home/jips/.local/bin/skystacker`.
- `~/.bashrc` aliases `skystacker` to that path. An old interactive shell
  may need `unalias skystacker` or `source ~/.bashrc`.
- Do not leave `./skystacker` or `~/go/bin/skystacker` behind; remove stale
  binaries after installing.
- `/usr/local/bin` requires sudo; not installed there.
- Tests are embedded self-tests, not `go test` files. `go test ./...` says
  “no test files”; use `--selftest`.
- Baseline: **169 self-tests passing**. `gofmt -l main.go` empty;
  `go vet ./...` clean.

### Non-negotiable constraints

- Never weaken the section-E calibration refusal contract
  (`testSyntheticCalib*`).
- Do not touch `computeFrameQuality` or the `--sigma-clip` defaults without
  an explicit request.
- Never delete or overwrite original photo files.
- Do not write into `assets/Orion-135mm/0_skystacker_results/`; use `/tmp`
  for E2E runs.
- Do not commit, amend, push, or open PRs unless explicitly asked.
- Code, comments, and docs are in English; final user-facing reports are in
  Spanish.
- Do not add dependencies and do not split `main.go` unless asked.

### Pipeline summary

1. Discover and decode inputs (JPG native; CR2 via dcraw; FITS via
   ImageMagick/native paths).
2. Optional calibration with `--dark`, `--flat`, `--bias`, or
   `--calib-root`; `--export-masters` writes scientific `.f32raw` plus a
   `.jpg` preview.
3. Pass 1: calibrate, auto black level, BPM, accumulate unregistered mean,
   detect stars/register, optionally spill frames for tiled reduction.
4. Pass 2: resample registered frames, optional normalization, accumulate
   or spill; tiled median/sigma reduction; safe crop.
5. Post-processing: 8 steps, background removal through curves/HDR/mix and
   optional diffraction spikes.
6. `report.txt` records settings, frame diagnostics, crop, warnings and
   calibration/self-flat diagnostics.

Key registration facts: adaptive high-pass radius, multi-sigma star retry,
40-star triangle matching, RANSAC similarity plus optional 6-DOF affine,
bilinear resampling. Stacking: streaming mean, tiled median/sigma with MAD
rejection and disk spill. `--scale N` downsamples.

### Current `--self-flat` v1 (shipped, limited)

- `selfFlatEligible` requires registration, at least 15 registered frames,
  and no loaded real flat (a real flat wins).
- Built from `spillPaths2`, i.e. frames already resampled into reference
  coordinates, after Pass 2. `buildSelfFlat` runs a one-iteration
  sigma-clipped mean with per-frame median normalization.
- `normalizeSelfFlat` normalizes per channel to median 1, clamps to
  `[0.05, 20]`; center/edge must be in `[0.5, 5]`.
- The v1 master is crop-sized (Orion: 2395x1583), not full-frame. It is
  applied after registration; only the registered stack is corrected.
- The arithmetic-mean path spills registered frames to `spillPaths2`,
  derives the flat, then re-accumulates the mean with `applyFlatToFrame`.
- `reduceTilesStack`/`reduceTile` carry `frameScales` and a
  `flat *flatCorrection` parameter; the main reductions pass `nil`.
- `selfFlatResult` currently stores `Frame`, `Frames`, `CenterEdge`,
  `ResidualRMS`, `Medians`; `report.txt` exposes `SelfFlatFrames`,
  `SelfFlatCenterEdge`, `SelfFlatResidual`.
- Orion-135mm (66 JPG, 2400x1600, no calibration): center/edge 1.2512,
  residual RMS 0.19360, corrected mean center/border 1.000 vs 1.134
  uncorrected. The flat itself reaches ~1.9 around M42 and can create dark
  halos.
- Why v1 fails: aligned stars and nebulosity are common to every
  registered frame, so sigma clipping treats them as illumination. With
  ~13 px dithers and a large nebula, temporal rejection cannot separate
  them. This is not literally “dividing the stack by itself”, but the
  correlation causes over-subtraction.
- E2E sigma run (`--method sigma --limit 20`): center/edge 1.2497,
  corrected center/border 1.000. Outputs were in `/tmp`, not the repo.

### Scientific limits to keep visible

- A stable sky gradient is degenerate with the multiplicative flat and
  will be absorbed by a 2D polynomial. A radial model is more identifiable.
- A large nebula with small dithers remains partially degenerate; masking
  and a low-degree model reduce but do not eliminate the bias.
- Dust shadows are high-frequency and out of scope for a polynomial
  self-flat; a real flat is required.
- The pipeline works on sRGB-encoded values. If the flat is estimated and
  applied consistently in encoded space, the first-order factor cancels,
  but the transfer function is nonlinear; gamma-aware correction is a
  separate phase.
- The self-flat is a fallback for sessions without flats, never a
  substitute when a real flat exists.

### Specified sensor-space revision (not yet implemented)

- Build from calibrated **unregistered** frames in sensor coordinates.
- Per-frame per-channel robust normalization; luminance source mask
  (`median + 2.5*MAD`), saturation/BPM mask, 10-15 px dilation.
- Coarse grid (64 or 96 px). Per cell: robust background (SExtractor mode
  `2.5*median - 1.5*mean`, low quantile as diagnostic), 5x5 median/MAD
  cell rejection.
- Fit degree-3/4 Chebyshev/Legendre per channel with IRLS
  Huber/Tukey; optional radial model; normalize and validate min/max,
  center/edge, fit RMS.
- Full-frame master. Apply after calibration/BPM and **before**
  `resampleToReference*`; post-divide unregistered mean/median/sigma
  outputs (equivalent because the flat is constant per pixel).
- Temporary flag `--self-flat-model=registered|grid`; default remains
  `registered` until Phase 4 acceptance tests pass, then `grid`.
- Eventually remove `flatCorrection`, the `flat`/`frameScales` reduction
  parameters, and the v1 mean re-accumulation.
- Detailed algorithm, integration map, validation plan and phases:
  `research/04-calibration-and-self-flat.md` §9-14 and
  `ROADMAP_SYNTHETIC_CALIBRATION.md` §2.3.

### Phase 1 implementation contract (next code session)

- Implement only the sensor-space prototype; keep v1 behind
  `--self-flat-model=registered`.
- For `grid`, use a streaming `selfFlatAccumulator` in the Pass 1 loop
  after `calibrate`/black-level/BPM. Reading `spillPaths1` is acceptable
  only if it does not force an extra spill for `mean`.
- Per frame/channel: robust median normalization; luminance mask and
  dilation; 64 px cells; 25th percentile of unmasked normalized pixels;
  require enough unmasked samples per cell. Across frames: median of the
  per-frame quantiles, at least 15 valid frames.
- Degree-4 Chebyshev/Legendre least-squares fit per channel, weighted by
  valid sample counts; solve normal equations in stdlib. Evaluate on the
  full `refW x refH` grid.
- Normalize per channel, clamp `[0.1, 10]`, reject implausible
  center/edge or min/max. Write a full-frame master and `report.txt`
  diagnostics (`Model=grid`, degree, cell, valid cells, mask fraction,
  fit RMS, min/max, center/edge).
- Apply before resampling in Pass 2; correct the registered stack and
  post-divide the unregistered mean (median/sigma if straightforward).
- Tests: synthetic known vignetting + moving stars, recovery within 0.05
  RMS; application-order test; build without registration; flag default
  and unknown-value rejection; keep the three existing self-flat tests.
- Acceptance: `gofmt`/`go vet` clean, self-test total updated, E2E Orion
  A/B in `/tmp`, M42 dome strongly reduced, no commit unless asked.

### Roadmap

- Phase 0: analysis and guardrails (done, docs only).
- Phase 1: sensor-space prototype, basic mask, cell quantile, OLS,
  full-frame application. **Next.**
- Phase 2: SExtractor mode, 5x5 rejection, mode/quantile consistency.
- Phase 3: IRLS polynomial, radial/2D comparison, fit diagnostics.
- Phase 4: integration/cleanup, all methods, `--no-register`, default
  `grid`, docs.
- Phase 5: gamma-aware correction.
- Phase 6: real-flat validation with OrionNebula/Flame.
- Phase 7: dust/defect handling, optional and out of scope.

### Current repository and docs state

- `HEAD` and `origin/main` are `a457a58`; no commit has been made since.
- Working tree currently has documentation edits only:
  `PROJECT_CONTEXT.md`, `README.md`, `ROADMAP_SYNTHETIC_CALIBRATION.md`,
  `TECHNICAL_SPEC.md`, `prompt-ai.txt`,
  `research/04-calibration-and-self-flat.md`.
- `NEXT_SESSION_PROMPT.md` is gitignored and was rewritten for Phase 1.
- `main.go` has **not** been changed for the sensor-space model yet.
- Recent commits: `a457a58` (`prompt-ai.txt`), `9a43a54` (remove backups
  and patch), `b003fa7` (user commit, included the then-current work).
- `assets/Orion-135mm/` contains the 66 JPG test set and an existing
  `0_skystacker_results/` that must not be overwritten.

### External datasets (outside the repo, do not commit)

- `/home/jips/datos/Astro/datasets/OrionNebula/`: 134 CR2 lights, 22 darks,
  17 flats, 5 biases, Canon 40D; CR2 needs `dcraw -4 -T`.
- `/home/jips/datos/Astro/datasets/04_emission_nebula_Flame/`: 45 FITS
  lights, 2 darks, 1 flat, 1 bias, 3056x2048; FITS is faster for E2E.
- Use `/tmp` for converted data and outputs.

### Key files

- `main.go`: flags, pipeline, calibration, registration, reduction,
  `buildSelfFlat`, `normalizeSelfFlat`, `reduceTilesStack`, report,
  self-tests.
- `research/04-calibration-and-self-flat.md`: science, v1 results,
  sensor-space design, integration map, validation, roadmap.
- `ROADMAP_SYNTHETIC_CALIBRATION.md`: Tier 2.2 shipped, Tier 2.3 revision.
- `PROJECT_CONTEXT.md`: this snapshot and session history.
- `README.md`, `TECHNICAL_SPEC.md`: user-facing caveats and planned work.
- `prompt-ai.txt`: persistent build/install/hygiene instructions.
- `NEXT_SESSION_PROMPT.md`: gitignored Phase 1 implementation prompt.

### Open decisions before/while implementing

- Prototype source: streaming accumulator vs. `spillPaths1`.
- Basic mask in Phase 1 vs. deferring all masking to Phase 2.
- Exact cell validity thresholds and across-frame quantile.
- Whether to correct unregistered median/sigma in Phase 1 or defer.
- Whether `--self-flat-model` unknown values fail hard (preferred) or
  fall back to `registered`.
- Keep the default `registered` until Phase 4; do not switch early.

### Phase 1 implemented (2026-11-15)

- `--self-flat-model=grid` builds a full-frame sensor-space flat in Pass 1
  (`selfFlatGridAccumulator`) and applies it before resampling. Default is
  still `registered`. Unknown model values are rejected.
- Estimator: per-channel median normalisation, luminance mask
  `median+2.5*MAD` plus saturation, 12 px dilation, 64 px cells, 25th
  percentile, across-frame median, degree-4 Chebyshev least squares.
  Not IRLS and not 5x5 rejection.
- Unregistered median/sigma outputs are post-divided. The no-register mean
  path copies the accumulator before dividing, so
  `stacked_no_correction_mean` stays uncorrected.
- Tests: 169 → **173**. New: `testSelfFlatGridRecovery`,
  `testSelfFlatGridApplication`, `testSelfFlatGridNoRegistration`,
  `testSelfFlatModelDefault`. Section-E tests unchanged.
  `gofmt`/`go vet` clean. Canonical binary rebuilt at
  `/home/jips/.local/bin/skystacker`; `--selftest` prints
  `all 173 self-tests passed`.
- Orion smoke (`/tmp/orion_grid`, `--method mean --stack-only --limit 20
  --self-flat-model=grid`): master 2400x1600, center/edge 1.2449, fit RMS
  0.01337, min 0.4855, max 1.2081, 391/950 cells, mask 67.6%. The v1 M42
  peak is gone. The mask fraction is too high for a default; Phase 2 next.
- Not committed.

### Report command line (2026-11-15)

- `report.txt` now starts with `Command line:` (the original `os.Args`,
  shell-quoted) and `Resolved flags:` (effective settings that differ from
  the built-in defaults, plus `--method` and `--workers`).
- `--calib-root` was parsed into `cliOptions` but never copied into
  `runOptions`. That copy is now made, so the flag both works and appears
  in the report.
- Test: `testReportRecordsFlags`. Self-test count was 174; see the 0.6.0 note.
- Coverage: do not add a `go test -cover` harness. The suite is `--selftest`
  inside `main`, so `go test` reports no tests. More tests are worthwhile
  only for wiring and scientific gates (grid Pass 1/2, mask fraction,
  report fields), not for line coverage.

### v0.6.0 default recoverable calibration and static release

- If no bias frames are present, the metadata pedestal is subtracted
  (`--black-level auto`). `--black-level off` opts out. Spatial bias is
  not synthesized.
- If no dark frames are present, no dark is fabricated. The report says so.
- If no flat frames are present and there are at least 15 lights,
  `--self-flat --self-flat-model=grid` is applied. `--no-self-flat` opts out.
  Dust is not corrected.
- `--synthetic-dark/flat/bias` remain refused. Test:
  `testDefaultSyntheticCalib`. Self-test count **175**.
- Release binaries are static (`CGO_ENABLED=0`) for linux/darwin/windows
  amd64 and arm64, attached to the GitHub release by `.github/workflows/release.yml`.

### Test fixtures live under assets (2026-11-15)

- Persistent fixtures are in `assets/testdata/`, not a top-level `testdata/`
  or `test_data/`. Current files: `healpix_reference.json`,
  `gaia_plate_fixture.json`. Self-tests read them as
  `assets/testdata/<file>` from the repository root.
- Any new committed file needed by a test (JSON, FITS, SER, image) goes in
  `assets/testdata/`. Do not recreate `testdata/` at the repo root.
- Generated fixtures may still use `os.MkdirTemp`. Large local datasets such
  as `assets/Orion-135mm/` stay gitignored and are not required by `--selftest`.
- Do not add a `go test -cover` harness. Add a self-test only when a behavior
  or a fixture path can regress. Current gaps that do not need new assets:
  comet output wiring in Pass 2, SER `#frame=` through `decodeImageFile`,
  and the grid self-flat mask fraction. `testSERDecode` now also checks that
  `discoverFiles` accepts `.ser`.

### Documentation sync (2026-11-15)

- An external review counted 180 `func test*` names and treated that as the
  self-test total. That is wrong. `runSelfTests` registers **177** cases.
  Phase 2 of the grid self-flat is not implemented; the extra cases are
  report flags, default synthetic calibration, SER, and comet translation.
- Docs updated to that fact: `prompt-ai.txt` (historical prompts removed;
  `./skystacker` may stay, gitignored), `NEXT_SESSION_PROMPT.md` (Phase 2
  still next), `COMPARISON.md`, `README.md`, `TECHNICAL_SPEC.md`, and
  append-only status notes in `research/`. History in this file was not
  deleted.

### Reference-frame crop (2026-11-15)

- Added `--reference-crop PATH`. The reference image defines the output
  frame. Each light is decoded at its own working size (`inW`/`inH`), stars
  are matched with `matchStarsToReference`, and the frame is warped with
  `resampleToReference` into the reference size (`refW`/`refH`).
- Pass 1 and the unregistered stack stay in input coordinates; registered
  accumulators, self-flat v1, coverage, and reduced stacks use reference
  coordinates. Do not change the spill-1 dimension back to `refW`/`refH`.
- `--limit 1` produces a single cropped frame. `--reference-crop` requires
  registration and disables `--normalize`/`--local-normalize` for now.
- Verified on Orion: 70% reference recovered scale 0.6999; 80% + 3°
  reference recovered scale 0.7999 and rotation 3.00°. Single- and
  three-frame runs output the exact reference dimensions.
- Registered self-tests: 178 (`testReferenceCropAlignment`).
- `COMPARISON.md` now has a post-processing software table that includes
  Fitswork and contrasts its manual two-star crop with the automatic
  star-pattern crop.
- Follow-up fixes: the mean fast path re-decoded at `refW`/`refH`, which
  double-scaled the light in crop mode (about half the frame uncovered);
  it now decodes at `inW`/`inH`. `computeFootprintPct` now takes separate
  source and reference dimensions, and the frame-limiter offset check uses
  the reference center, so crop mode no longer reports a false
  `LARGE OFFSET`. A regression check uses a 0.7-scaled source against a
  smaller reference.
- Time estimate now includes calibration masters. It previously counted only
  lights, so a run with 41 CR2 darks/flats/biases estimated ~1 min but took
  ~13.5 min. `calibrationFrameCount` resolves the same folders as
  `stackCalibrationFolder` without decoding and adds ~20 s per CR2
  (empirical) plus a small per-pixel cost for non-RAW frames. The
  `--calib-root` path resolution moved before the estimate for this reason.
  Verified on a 1-light/3-calibration subset: estimated 2m0s, actual 1m32s.
- Self-flat Phase 2 shipped behind `--self-flat-model=grid` (default still
  `registered`). The source mask is now local-contrast based (box radius 8,
  residual > 5*MAD, floor 0.008, 12 px dilation) so extended emission is
  not masked like a star. Cells store the SExtractor mode from a 10-90%
  trimmed mean and reject a heavy upper tail (`upper > 1.6*lower`). A 5x5
  median/MAD filter (`|B-med| > 2.5*MAD`) removes residual outliers with
  zero weight. `RejectedCells` is reported in the log, `report.txt`, and
  the processing report.
- Orion-135mm `--limit 20` grid result in `/tmp/orion_p2_grid`: mask
  31.7% (was 67.6%), cells 851/950 (66 rejected), fit RMS 0.01606,
  center/edge 1.2226, min/max 0.5316/1.1315. Corrected mean
  center/border 1.019. New `testSelfFlatGridNebula`; registered self-tests
  now 179. Phase 3 (IRLS) is the next calibration task.
- Self-flat Phase 3 shipped behind `--self-flat-model=grid` (default still
  `registered`). The single OLS solve is now a degree-4 Chebyshev fit
  refined by asymmetric Huber IRLS (c=1.5 positive, c=3.0 negative,
  1.4826*MAD scale, 15 iterations max, 1e-7 tolerance). `finalize` also
  reports OLS RMS, radial-model RMS (`1, r^2, r^4, r^6`) and IRLS
  iteration count; the 25 coefficients per channel are written to
  `report.txt`. The grid gate now rejects `center/edge` outside [0.5, 5],
  `min < 0.25`, or `max > 4.0`, and a note flags edge-cell extrapolation.
- Orion-135mm `--limit 20` grid result in `/tmp/orion_p3_grid`: mask
  31.7%, cells 851/950 (66 rejected), fit RMS 0.01634, OLS RMS 0.01608,
  radial RMS 0.07496, IRLS 11, center/edge 1.2180, min/max 0.5312/1.1300.
  Corrected mean center/border 1.023. New `testSelfFlatIRLSRobust` and
  `testSelfFlatRadialModel`; registered self-tests now 181. Phase 4
  (default switch, gamma-aware correction, real-flat validation) remains.
- Phase 4 validation (2026-11-16): with `--self-flat-model=grid`, `--method mean|median|sigma --limit 20 --export-masters` each produce identical self-flat diagnostics in `/tmp/orion_p4_*_grid` to Phase 3 (mask 31.7%, 851/950, 66 rejected, fit RMS ~0.0163, OLS 0.01608, radial 0.075, IRLS 11, center/edge ~1.218, min/max ~0.531/1.13). `--no-register --self-flat-model=grid --limit 20` also applies the grid model without errors in `/tmp/orion_p4_noreg_grid`. A soft guard rejects the grid model if extrapolated at an edge and `min < 0.45` (current Orion still passes with extrapolation note). All 181 self-tests unchanged/passing.
- Documented gamma-aware (EOTF) correction as out of scope for the current sRGB/encoded pipeline (division remains approximate); note added to grid self-flat report note. No implementation in this session.
- Point 2 (validate grid vs real flat): `assets/OrionNebula` has real flats/bias/darks but only 3 lights (<15) so grid cannot be built there; `assets/Orion-135mm` has 67 lights with no real flats. No single dataset allows a direct grid-vs-real-flat comparison; validation deferred. Ran `OrionNebula/lights` with `--calib-root` style flags (`--dark/--flat/--bias`) and `--self-flat-model=grid` (flat real present => grid not used, master_flat exported) as sanity check: exit 0, calibration applied (bias/dark/flat).
- Research reorganised (2026-11-16): Spanish `research/last-research.md` replaced by English master `research/05-research-deep-dive-2026-11-16.md` with pillars (calibration/detectors, astrometry/catalogues/HEALPix/cross-match, architecture/provenance/automation), backlog hooks, and OrionNebula notes; appendices added to relevant `research/0*/*.md`, `research/04-*.md`, and `ROADMAP_SYNTHETIC_CALIBRATION.md`. Do not duplicate this narrative elsewhere; extend the master file.
- `assets/OrionNebula/` now has 16 lights; running with `--calib-root assets/OrionNebula` while pointing input at the root failed due to lowercase subfolders (`darks/flats/biases/lights`) not matching auto-detection; running against `assets/OrionNebula/lights` succeeded (grid self-flat path exercised) — code fix candidate in `detectCalibrationSubdirs`/`--calib-root` resolution.
- Calibration auto-detect: `detectCalibrationSubdirs` is case-insensitive (already). `--calib-root` now uses detected `Bias/Darks/Flats` subfolders when present and falls back to common names; this fixes running with `--calib-root <root>` against a `lights/` input when calib folders are lowercase (`darks/flats/biases`). Verified on `assets/OrionNebula` (16 lights): bias/dark/flat masters exported.
- SIP in resampler: SIP order-2 coefficients are fitted/stored from plate solving (`plate_solution.json`) but are not applied in the pixel-space star-registration resampler (which uses similarity/affine). Applying them would require a WCS/astrometric resampling path distinct from current registration; left as a documented gap (see README/TECHNICAL_SPEC). No code change in this step.
- Research deep dive extended (2026-11-16): added ASTAP `-analyse`/`-focus` audit (errorlevel scale, 32-bit HFD ceiling), tile-vs-temporal rejection memory and Swiss Tables/Go notes, TAN–SIP + rejection/stretch references, Gaia v3.15/DR4+telemetry+ProvenanceDM notes to `research/05-research-deep-dive-2026-11-16.md` and appendices in core/CLI research docs.
- Comet RA/Dec mode added: `--comet-radec RA,DEC` (decimal degrees) + `--comet-sky-rate-arcsec dRA*,dDec` (arcsec/sec) with `--plate-solve`; per-frame tangent-plane offsets computed from WCS. Mutually exclusive with pixel `--comet-center`/`--comet-rate`. Self-tests still 181.
- Gamma/EOTF option (experimental): `--self-flat-linear-srgb` applies grid self-flat division in linear light after sRGB EOTF decode of frame samples and re-encodes with sRGB OETF. Default off (approximate encoded-space division preserved). No new deps; needs validation on real data.
- OrionNebula (16 lights): compared grid self-flat (`/tmp/onb_grid_only`, dark+bias, grid default) vs real flat (`/tmp/onb_realflat_run`, dark+flat+bias). Grid report: fit RMS 0.112, min/max 0.874–1.098, center/edge 1.11, extrapolated, masked 4.8%. Approx gamma-space center/border from `stacked_mean.jpg`: grid_only gamma 0.995 (linear ~0.990), realflat gamma 1.016 (linear ~1.033). Note: flat present disables grid; comparison is run-to-run in these temp dirs, not a single-pass ablation in-place except by separate outputs.

## 2026-11-16 — v0.6.1, comet ephemeris, SIP in WCS helpers

- **v0.6.1:** bumped from 0.6.0; self-tests 182 (added `testCometEphemerisParse`, extended `testCometTranslation`).
- **Comet ephemeris:** `--comet-target PATH` (`ISO8601 RA_deg Dec_deg`, `#` comments, ≥2 points, RA 0–360°, |Dec|≤90°), requires `--plate-solve`; interpolated linearly by EXIF `DateTimeOriginal` (fallback to reference time + warning). Docs in README/prompt-ai; help updated.
- **SIP order-2 (WCS helpers):** `wcsTan.SIP` stored; `wcsTanForward` applies SIP; `wcsTanInverseSIP` added (iterative). Stacking resampler (affine/similarity) still does **not** apply SIP (documented). `testWCSSIP` extended with round-trip sanity.
- **CI:** pushes `2c7e031`, `c84221c` verified locally green; check remote runs as needed.
- Added selftests `pipeline grid self-flat {mean,median,sigma} stack-only` (E2E runPipeline with `--self-flat-model=grid`, registered, stack-only); self-tests 185.

- Added `--fits` writer (`stacked_sum.fits`, big-endian BITPIX=-32, NAXIS=3, 2880-byte blocks), verified with `testFITSWriterRoundTrip` and ImageMagick `identify` (3 planes). selftests 186; README/TECH/prompt/CHANGELOG updated.

- Added `--bpm-auto` / `--bpm-auto-sigma`: builds a bad-pixel map from the master dark (or bias) via median + N*1.4826*MAD on a strided sample (mean-deviation fallback when MAD=0), flags full-frame outliers, merges with explicit `--bpm`. `testBPMAuto` added; selftests 187.

- Wired `--plate-solve` into `--post-only`: re-detects stars from `_work/stacked_mean.f32raw` and writes `plate_solution.json` (no FITS header in post-only, so `--plate-center` is the practical seed). `testPlateSolvePostOnlyNoStars` added; selftests 188.

- Added flux-weighted half-flux radius (HFR) to detected stars (`starHFR`, `testStarHFR`), a primitive for future PSF-based quality scoring. selftests 189.

- Added `--xisf`: minimal monolithic XISF 1.0 writer (`XISF0100`, XML Metadata, one RGB Float32 Image with planar attached block, no compression) verified by `testXISFWrite`. selftests 190.

- Added `--hfr-quality` (opt-in): advanced frame-quality FWHM proxy uses median star HFR (`1.995*HFR`) instead of connected-component blob size. `testHFRQuality`; selftests 191.

- Added asymmetric sigma clipping (`--sigma-clip-low`, `--sigma-clip-high`, 0 = symmetric); threaded through `reduceTilesStack`/`reduceTile` via optional params so all existing call sites keep symmetric behavior. `testSigmaClipAsymmetric`; selftests 192.

- Added `--annotate`: offline catalog annotation projects cached Gaia stars through the stored plate solution (SIP applied via `wcsTanForward`), draws `stacked_annotated.jpg`, and writes unmatched detections to `anomalies.json` (8 px tolerance). `testCatalogAnnotationSkip`; selftests 193.

- Added `--starless`: writes `_work/stacked_starless.f32raw`, `_work/stacked_stars.f32raw` and JPEG previews from the existing post-processing star-removal step (crude annulus-median baseline). `testPipelineStarless`; selftests 194.

- `--fits` writes TAN WCS keywords (CRVAL/CRPIX/CD in degrees) from the plate solution when available; `testFITSWriterRoundTrip` covers both plain and WCS headers.

- Hardened `--bpm-auto`: if >0.1% of pixels would be flagged, the map is rejected (previously a low-MAD master could flag ~10% of the sensor). E2E `--fits --xisf --bpm-auto` run on OrionNebula limit 4 verified output files.

- Added `--json-report`: writes the existing processing report as `report.json`; covered by `testPipelineStackOnly`.

- `--fits`/`--xisf` now emit the linear mean stack as `stacked_mean.fits`/`stacked_mean.xisf` in addition to the sum; e2e verified on OrionNebula limit 4 for `--fits --xisf --bpm-auto`.

- Added `--normalize-mode` (scale-offset default, additive, multiplicative) applied consistently to global and tile-local normalization; `testNormalizeModes`; selftests 195.

- Added `--bg-multiscale`: coarse 20th-percentile background grid (block >=64px, bilinear interpolation) applied before the polynomial fit; `testMultiscaleBackground`; selftests 196.

- Added `--config FILE`: JSON object of flag names/values expanded into CLI args before parsing (CLI wins); supports strings, numbers, booleans and arrays. `testConfigFile`; selftests 197.

- Added `--deconvolve` (Gaussian-PSF Richardson-Lucy, `--deconv-sigma` default 1.5, `--deconv-iters` default 3) in post-processing; `testRichardsonLucy`; selftests 198.

- Added `--narrowband HOO|SHO` + `--nb-a/--nb-b/--nb-c`: combines three mono `.f32raw` channels into an RGB stack before post-processing; `testNarrowband`; selftests 199.

- Added `--resume`: if `<output>/_work/stacked_mean.f32raw` exists, switches to post-only automatically instead of re-stacking; `testResumeFromStack`; selftests 200.

- Added diagnostic drizzle upsampling (`--drizzle-upsample N`, `--drizzle-pixfrac F`) writing `stacked_drizzled.f32raw/.jpg`; `testDrizzleUpsample`; selftests 201.

- Added `--normalize-per-channel`: global normalization with independent per-channel median/MAD (`channelNormStats`); overrides local normalization; `testNormalizePerChannel`; selftests 202.

- E2E smoke (OrionNebula limit 4, `/tmp/onb_combo`): combined `--fits --xisf --json-report --drizzle-upsample 2 --normalize --normalize-mode additive --sigma-clip-low 2 --dark --bias` produced all scientific outputs including `stacked_drizzled.*`; a follow-up `--resume --deconvolve --deconv-iters 1 --starless --bg-multiscale` run correctly switched to post-only and produced starless/star layers. All outputs present; no crashes.

- Added `--wavesharp`/`--wave-gains`: coarse-to-fine multi-scale Gaussian detail enhancement as a classical, stdlib-only analogue to AI deconvolution (research §13). `testWaveSharpen`; selftests 203.

- Default post pipeline review (research §13.4): kept sharpening/deconvolution opt-in; added reference-frame HFR seeing-proxy note to log/report to guide their use.

- Added `testReferenceFITSSeed`: validates FITS-header RA/DEC seed parsing (degrees to radians) and rejection of non-FITS/no-keyword files. selftests 204.

- Implemented proposal from Gemini outcome: `detectStarsWithDetrend` retries detection on a scratch `--bg-multiscale`-style background-removed copy when fewer than 20 stars are found (gradient-heavy wide fields / plate solving). Input buffer untouched; `testDetrendStarDetection`; selftests 205.

- Implemented Gemini proposal 6 (dark-series QA): dark folders with >=3 frames get per-frame median/MAD/hot-pixel QA and >3-sigma outliers are excluded from the master (mean-absolute-deviation fallback when MAD=0). `testCalibrationQA`; selftests 206.

- Gaia TAP hardening (Gemini survey proposal 3, partial): HTTP 429 is now retryable with `Retry-After` capped at 30 s; existing 1 req/s rate limit and 2000-row MAXREC already satisfy DR4 quota guidance. `testGaiaQueryTile429Retry`; selftests 207.

- SkyStacker now decodes CR2 natively (pure Go, stdlib only): locates the SOF3 stream, Huffman/entropy decodes two 14-bit planes, interleaves RGGB Bayer, applies black/white levels and bilinear demosaic. `--no-native-cr2` restores dcraw fallback. E2E: 1 CR2 stack in 6.5 s, 100 stars detected; selftests 208. Built-in implementation mirrors the goPureRaw subproject decoder.

- Native CR2 validation status: decoded EOS 40D CR2 (3944x2742, 14-bit, mean 1181.8) matches dcraw -D -4 mean (1181.0) and supports E2E stacking (100 stars, 6.5 s). Per-pixel/geometric agreement with dcraw is NOT yet established (downsampled cross-correlation across all 8 dihedral+crop variants < 0.1); flagged as open validation item in goPureRaw ROADMAP Phase 1b. Treat native output as usable but not yet bit-exact-equivalent.

- Finite-sample MAD correction: sigma-clip now uses calibrated small-N factors (Monte Carlo, 40k trials/N, seed 20261117) instead of asymptotic 1.4826 for N<30; unweighted and weighted reducers updated; `testFiniteSampleMAD`; selftests 209.
- Roadmap reordered by ascending effort in NEXT_SESSION_PROMPT.md; first item (finite-sample MAD) completed.
- myTools/AI folder names standardised to `AI-Detect`, `Gemini-Deep-Research`, `Gemini-Transcribe`, `Voice-Typing`; invocation paths recorded in prompt-ai.txt.

- Modularisation step 1–2 done: goPureRaw module path is now `github.com/computerphysicslab/skystacker/goPureRaw` (all imports updated, README includes `go get`); root `go.work` links both modules and is gitignored. SkyStacker still embeds its own `pr*` CR2 copy; importing the library and adding subdir tag remain.

- Modularisation steps 3–4 done: `goPureRaw/v0.1.1` tagged (fix: CR2 Probe signature at offset 8), root `go.mod` requires goPureRaw v0.1.1, SkyStacker imports `raw` + `formats/cr2` and the duplicated `pr*` decoder was deleted from main.go. E2E 1-light CR2 stack 7.5 s; `GOWORK=off` build/test green; selftests 209.

### Crawler-based source discovery (2026-11-17)

- `research/08-crawler-source-discovery.md` consolidates the `goCrawler` runs (21 seeds, 18 curated English documents kept, registration/stacking/calibration/drizzle sources: Siril, CFA PixInsight, APP, STScI).
- `robots.txt` policy blocked 8 URLs (IVOA MOC/VOTable/ConeSearch, Astrometry.net nova UI, astropy docs root, WCStools, Flickr); blocked sources need a manual/alternative route, the crawler does not bypass them.
- Crawler is sequential and network-bound (24% of one core on 8 cores); parallel worker pool + per-domain rate limits remain a research-tool improvement, not a SkyStacker runtime change.
- `.gitignore` now excludes the large local dataset `assets/M42-2017-11-18-Leciñena-50mm/`.

### Go 1.27.4 toolchain upgrade (2026-11-17)

- Toolchain installed at `/home/jips/.local/go1.27.4/bin` (symlink to the module-cache download); both `go.mod` files now declare `go 1.27` + `toolchain go1.27.4`; local `go.work` bumped to 1.27.4; CI and release workflows pinned to 1.27.4.
- All checks green on 1.27.4: gofmt, vet, build, 209 self-tests, `go test ./...`, goPureRaw CGO=0 + race, race build + selftests, six static cross-build targets. No source changes or exact-string report drift.
- Canonical binary and `./skystacker` rebuilt with the new toolchain; `/home/jips/go/bin/skystacker` symlink points at the repo copy.
- BENCHMARKS.md has a new Go 1.27.4 re-baseline section: 4-frame native-CR2 sigma stack 39.031 s, `--resume --starless` 21.11 s (not a controlled A/B).
- Docs synced: README, TECHNICAL_SPEC, CHANGELOG, prompt-ai.txt, NEXT_SESSION_PROMPT; research/ updated by appending status sections only.

### Single-pass sigma-clip accumulation (2026-11-17)

- Implemented the safe half of the ASTAP proposal in `sigmaClipReduce` and `sigmaClipReduceWeighted`: the final mean (weighted/unweighted, plus zero-weight fallback sum) is accumulated inside the last clipping pass; the surviving values are carried into the next iteration instead of being re-gathered from the boolean mask; `reduceTile` accumulates `sumPix` while gathering valid values instead of in a separate pass.
- Kept median/MAD rejection unchanged: replacing it with mean/variance would change scientific semantics and the bit-exact output guarantee. The two original reducers are preserved verbatim as `twoPassSigmaClipReduceReference`/`twoPassSigmaClipReduceWeightedReference` for tests only.
- `testSinglePassSigmaClipEquivalence` (selftest 210) compares exact float32 mean, sigma, accepted and rejected counts across N=4..40, 1–3 iterations, asymmetric thresholds, the <3-survivor branch and the zero-weight fallback, and checks tile sums pixel-by-pixel. Production vs reference microbenchmark (16 values, 2 iterations): ~763 vs ~1048 ns unweighted, ~833 vs ~1033 ns weighted.
- Real-data equivalence: 4-frame OrionNebula sigma stack, old and new binaries produced byte-identical `_work/stacked_mean.f32raw` (SHA-256 `d138ef5cc6b50c567fa1fe33aa478d901d8d12d4b6e3d0eeb031f15fedcfcbef`).
- Added `BenchmarkSigmaClipFused` and `BenchmarkSigmaClipTwoPassReference` to `main_test.go`; `go test -bench` does not affect the registered self-test count.

### External AI CLI sidecar runner (2026-11-17)

- Implemented `--external-bge`, `--external-denoise`, `--external-deconv`, `--external-starless`, `--external-allow-nonlinear` and `--external-timeout`. `CMD` is a whitespace-separated argv with `{input}`/`{output}` placeholders; `os/exec` is used directly (no shell).
- `runExternalSidecar` writes a private big-endian `BITPIX=-32`, `NAXIS=3` FITS with the plate-solution WCS, runs the tool under a context timeout, reads back only same-size 3-plane float32 FITS, restores WCS if the tool drops/changes it, and removes the temp dir on all paths.
- `frame` gained `WCS *wcsTan` and `Nonlinear bool`. `runPostProcessing` loads `plate_solution.json` into `src.WCS`, runs external BGE before step 1, external denoise/deconv after step 1, external starless in place of built-in star removal, and marks stretched frames nonlinear.
- `testExternalAISidecar` uses the binary itself via hidden `--internal-ai-sidecar-mock` to verify pixel/WCS round-trip, temp cleanup, missing-binary diagnostics, the linear guard and its override, CLI parsing and placeholder validation. `testExternalAISidecar` raises selftests to 211.
- E2E `--post-only --external-bge --external-denoise` on the OrionNebula 4-frame stack completed and wrote `_work/external_bge.f32raw` / `_work/external_denoise.f32raw`.
- Design and limits recorded in `research/06-external-ai-sidecars.md`. No ML model or runtime is embedded; the CLI tools remain user-supplied and licensed.

### SIP order-2 in the stacking resampler (2026-11-17)

- Added `--resample-sip`: `runPipeline` reads order-2 SIP from `<output>/plate_solution.json` (warning + plain warp if absent); `sipDistortPixel` / `sipUndistortPixel` (iterative inverse, 30 iterations) and `sipSampleRef` wrap the existing resamplers. The reference pixel is undistorted, the similarity/affine transform is applied in undistorted space, then the source pixel is redistorted before bilinear sampling. Comet-shift variants are also covered.
- Assumption: one shared distortion model from the stored plate solution for all lights; no per-frame astrometric reprojection. Identity transforms remain effectively identity; with zero SIP the warp is unchanged.
- `testResampleSIP` verifies distort/undistort round-trip, the composed mapping, a synthetic star relocation versus the plain warp, no-op behavior with nil WCS, and flag parsing. Selftests 212.
- E2E smoke: OrionNebula 4-frame sigma stack with `--resample-sip` and a seeded SIP plate solution completed (~38 s) and logged use of the stored SIP.
- Living docs updated (README, TECHNICAL_SPEC, COMPARISON, prompt-ai); research/ received only appended status sections.

### CR2 bit-exact investigation status (2026-11-17)

- `cr2_slice[]` hypothesis refuted: dcraw v9.28 only resets it to zero; no maker-note tag assigns it. The EOS 40D `0x0009` maker tag is an all-zero ASCII artist field, not a slice table.
- Next experiment is a raw-Bayer-plane comparison against `dcraw -D -4 -t 1` with margins cropped and black/white normalised; `curve[]` is identity for this camera, so linearization is not the cause. See `research/07-native-cr2-decoding.md`.

### Portable diagnostic module (2026-11-18)

- Added root `goDiagnostics/` as an independent stdlib-only Go module (`github.com/computerphysicslab/skystacker/goDiagnostics`, `go 1.22`, no toolchain pin, no dependencies). SkyStacker imports it through a `require v0.0.0` + local `replace`; `go.work` includes the folder.
- Public API: `Options`, `Collect(ctx, Options) Report`, `Report.JSON()/WriteJSON()/Text()`. OS-specific collectors sit behind a `platform` interface with `//go:build` files for linux (`/proc`, cgroup, dpkg/rpm/apk/pacman), darwin (`sysctl`, `vm_stat`, `sw_vers`, brew) and windows (PowerShell/CIM, Get-HotFix); unknown OS degrades with `ErrUnsupported`.
- Collectors cover host/OS, Go build info and `runtime/metrics`, CPU/load, memory/swap/cgroup, filesystems, top processes, tools, whitelisted+redacted env, program dir walk with file/byte counts and binary SHA-256, network interfaces and local package inventory. Every collector has a timeout and a 64 KiB command-output cap; failures go to `Report.Errors`.
- CLI: `--diagnose`, `--diagnose-json`, `--diagnose-quick`, `--diagnose-integrated`, `--diagnose-timeout`, `--diagnose-top`, `--diagnose-updates`. Standalone mode exits early in `main()`; integrated mode writes `diagnostics.txt`/`.json` next to a normal report. E2E `--diagnose --diagnose-json --diagnose-updates` produced a 17 kB JSON and a readable text report on this host.
- CI has a `goDiagnostics` job (vet, CGO=0 test, cross-build linux/darwin/windows); `goDiagnostics/README.md` documents the copy-paste `require`+`replace` protocol for other projects. Selftests 274.

### Module naming convention (2026-11-18, mandatory)

- Both nested modules were renamed to Go style: `PureRaw/` → `goPureRaw/` (`github.com/computerphysicslab/skystacker/goPureRaw` v0.1.1) and `diagnostics/` → `goDiagnostics/` (`github.com/computerphysicslab/skystacker/goDiagnostics` v0.0.0). The old paths are intentionally retired; nobody consumed them externally, so the former compatibility promise was explicitly lifted.
- Root `go.mod` now carries a local `replace` for **both** modules, so a clean checkout builds without published tags. `go.work` lists `.`, `./goPureRaw`, `./goDiagnostics`.
- Rule for every future module: directory and module path use `go<Name>` (e.g. future `goFitsio/`, `goPhotometry/` → `github.com/computerphysicslab/skystacker/go<Name>`). Never reintroduce `PureRaw/`, `diagnostics/` or hyphenated `go-*` module names. Once a module is published with a semver tag, its path becomes a compatibility promise and is frozen.

### Roadmap items 19–28 (2026-11-18)

- Delivered: frame metrics CSV + include-list culling, `--keep-sharpest` HFR selection (applied to pass-2 inputs), metadata-matched calibration library, comet+stars recombination, `--batch` JSON queue (self-exec, no shell), PSF-based starless radii, stdlib à trous wavelet denoise, FFT Wiener deconvolution with tiled overlap-add, and Gaia ensemble photometry (`photometry.csv/.json`).
- Each feature has a registered selftest; total **274**. E2E checks covered the batch queue, include-list + sharpest selection + metrics CSV, and post-only wavelets/Wiener/photometry.
- Item 27 (mosaic) remains blocked by item 13 (per-frame WCS). All other former *Open* items are now *Done*; `ROADMAP.md` is the authoritative tracker.
- Item 34 validated 2026-11-18: three 16-light OrionNebula runs gave center/border 1.00972 (self-flat only), 1.00972 (plus global normalize) and 1.02599 (plus local normalize). Local normalization touches the same low-frequency field (tile-median ratio 1.2667) but the final shift is ~1.6%, not a runaway double division. The report emits an overlap warning with the measured ratio; `testSelfFlatNormalizeInteraction` covers the synthetic gradient-only case. Self-tests 246.
- Item 47 delivered 2026-11-18: `donations.go` holds the public ETH/USDT-USDC and BTC addresses as constants, 110 rotating English templates with address placeholders, and terminal QR codes via MIT-licensed `github.com/skip2/go-qrcode`. The footer is printed only after a fully successful stack+postprocess run (verified: full run shows it, `--stack-only` and `--no-donation` do not); `--no-donation-qr` omits QR, `--donation` prints it standalone without processing images, and `--batch` children are suppressed. This is the first third-party build dependency; the binary remains pure Go and `CGO_ENABLED=0`.

### Research-to-roadmap routine (mandatory)

- After every change to the research knowledge base (ingestion, appended section or consolidated contribution) and after every `COMPARISON.md` update, scan canonical files `research/01-...08-*.md` for *open / not implemented / future / gap* statements and quantitative findings that imply work.
- Compare against `ROADMAP.md`, skip existing or explicitly rejected items, then add genuinely new challenges with ascending-effort order, S/M/L/XL estimate, status (Open/Blocked/Deferred), canonical research owner and a falsifiable acceptance scope; update the summary counts.
- Repeat the routine after comparison-matrix changes, promoting only features feasible under the stdlib-only static CLI constraints. Keep the comparative rationale in `COMPARISON.md`, not duplicated in `ROADMAP.md`.
- The habit is recorded in `prompt-ai.txt` and `NEXT_SESSION_PROMPT.md` so future sessions apply it automatically.

### Roadmap Open items delivered (2026-11-18)

- Items 29-44 were implemented in one pass: dither diagnostic and small-dither warning, `--rejection` stack-size heuristic, quoted sidecar argv tokenizer, `--post-preset detail-last`, GHS parameter-mapping test, independent WCS/SIP fixture, Gaia DR4 TAP adaptation (<=4 concurrent, DataLink IDs, blackout guard), pooled rejection buffers with 64px tiles, XISF/PGM/PPM input, 16-bit/mono sidecar FITS, `--wavelet-adaptive`, `--dark-scale-entropy`, `--cosmic-reject`, 9-zone tilt diagnostics and `--streak-detect`.
- All 274 selftests pass after items 58-62, 52, 68, solver-chain items 67/69/72/74/75/76/77, annotation items 50/55 and the native quad-index prototype for item 73; `ROADMAP.md` now shows 62 Done, 1 Open, 6 Blocked, 1 Executed/not accepted and 7 Deferred.
- Items 48-49 were then delivered: `sky_products.go` adds `--sky-map` (Gaia-based `stacked_sky_map.svg` at exact frame dimensions, with optional extended-object ellipses) and `--survey-cutout` (CDS hips2fits FITS at the solved frame's width/height/FOV/rotation, saved as `survey_reference.fits/.jpg` with a star-match diagnostic). Both are opt-in and never block offline runs.
- Items 58-62 deliver the standalone image mode in `image_mode.go`: `--process-image PATH[,PATH...]` with `--image-op` for blur/sharpen/background/stretch/denoise/deconvolve/starless, sum/subtract/average/median/min/max, measure-transform/measure-quality (CSV+JSON), plus per-image sky-map and survey-cutout using the existing `sky_products.go` helpers. Inputs are never modified; outputs go to `--image-output`.

### Roadmap item 52 — XNML / PSF Scale SNR ingestion (2026-11-18)

- `--psf-scale-snr` (opt-in, requires star detection) multiplies each frame's PSF quality weight by the relative scale from its PixInsight LocalNormalization `.xnml` sidecar; `--xnml-dir PATH` adds a fallback folder when sidecars are not next to the lights. Default weights are unchanged.
- `readXNMLPSFScale` parses XNML 1.0 `<RelativeScaleFactors>` (fallback `<GlobalNormalization><Scale>`), honours `valid="false"` and returns 0 when the sidecar is absent or malformed, matching PixInsight where PSF Scale SNR is zero without normalization data. Frames with zero scale get zero weight and a warning is logged.
- The per-frame scale is recorded as `psf_scale` in the `--frame-metrics` CSV, as `PSFScale` in `report.txt`/`report.json`, and covered by `testXNMLPSFScaleSNR` with synthetic XML (local, fallback folder, global-only, invalid, malformed, weight arithmetic and flag parsing).

### Roadmap item 68 — blind plate-solving fallback chain (2026-11-18)

- `--plate-solver auto` (default) keeps the existing seeded Gaia solve when `--plate-center` or FITS `RA`/`DEC` is available, then falls back to a **genuine blind solve** through astrometry.net: local `solve-field` first (`--astrometry-cmd`), then the nova web API (`--astrometry-api-key` or `AN_API_KEY`/`ASTROMETRY_API_KEY`). No coordinates from the user are required.
- Backends are opt-in by availability and never block offline runs: a missing CLI or key is logged and the chain continues; with neither backend configured the error explains how to enable one. `--plate-solver gaia|astrometry-local|astrometry-web` pins a single path.
- `parseAstrometryWCS` converts the returned FITS WCS (CD/CDELT+PC, 1-based CRPIX, quadratic SIP) into the internal 0-based `wcsTan`; `refineAstrometrySolution` then queries Gaia at the solved centre and reuses `plateSolutionFromGaia`, so external solves carry the same `plate_solution.json` schema, RMS and SIP as internal ones.
- Optional `--solver-focal-length MM`/`--solver-pixel-size UM` provide an arcsec/px hint (`f[mm]=(pixel[µm]/scale[″/px])×206.26`) and a post-solve sanity warning. API keys are redacted from the report command line.
- Wired into the stacking pipeline, `--post-only` (temporary JPEG from the stored stack) and standalone image mode; `--process-image … --image-op plate-solve` now solves a single JPEG/FITS with no metadata. Four new selftests cover WCS parsing, the local CLI mock, a mock nova API (login/upload/poll/WCS download) and the fallback chain.

### Layered annotation subsystem — roadmap items 137–141 and 143 (2026-11-22)

- New `annotation_layers.go` introduces one `annotationSource` model shared by the SVG 2 and raster renderers. It carries RA/Dec in radians, band/magnitude, axes/PA, epoch/proper motion, priority, label text and per-source provenance. An offline bundle (<2 MB) supplies bright stars, Messier/NGC/IC objects, LDN/Sh2 regions and constellation lines.
- `annotation_selftests.go` embeds the 40-source `synthetic-field.json` golden fixture and covers the bundle, mock SIMBAD/VizieR acquisition (success, timeout, truncation, cache reuse, redaction), deterministic placement, SVG layers/size, raster parity and an end-to-end plate-solved fixture. Total registered selftests: **281**.
- `--annotation-layers`, `--annotation-vo` and `--annotation-cache` were added to `cliOptions`, `runOptions`, `parseArgs`, `resolvedFlagLines` and the main `runOptions` literal. With `--annotate --sky-map` the pipeline now also writes `stacked_annotated.svg`, `stacked_annotated.png` and `annotation_provenance.json`.
- Label placement uses a dense spatial grid with cell-local collision queries and a deterministic priority/magnitude/ID order; 10 000 labels are placed in ~45 ms and 20 fixtures have zero overlaps. A bug in `annotationGridStep` (ascending step order) and the rad→arcsec conversion in `annotationArcsecPerPixel` were fixed, reducing a 10k-source SVG from 11 MB to ~1.8 MB (<2 MiB budget).
- Roadmap: 137–141 and 143 are Done; item 142 (per-tile WCS + distortion-aware projection) remains Blocked by item 13, so goal 144 stays Open. Examples and visual references are documented in `assets/annotation-examples/README.md`.

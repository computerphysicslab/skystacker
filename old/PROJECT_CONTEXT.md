# SkyStacker — project context (last updated 2026-11-11)

## Version
v0.5.0 (single-file Go program, `main.go`)

## Build, test, install
```
gofmt -w main.go && go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o skystacker . && ./skystacker --selftest
go install .
```

**Self-test count: 116 passing.** Update this number after any test change.

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
- Test: `./skystacker --selftest` (expect: **all 116 self-tests passed**)
- Post-only re-run: `skystacker --post-only --overwrite <dir>`
- Full re-run: `skystacker --overwrite <dir>`
- Sample data layouts (from validated runs):
  - `/media/jips/Elements/big/fotosAstro/2019-01-25-SanJuan/Orion-135mm`
  - `/media/jips/Elements/big/fotosAstro/2023-04-01-riofrio/m42/JPG`

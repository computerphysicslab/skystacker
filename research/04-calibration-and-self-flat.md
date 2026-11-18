# 04 — Calibration, master export, and self-flat (canonical research record)

**CURRENT STATUS (canonical, 2026-11-17):**

- Source version **v0.6.1**; **231 registered self-tests** (`skystacker --selftest`). Historical counts in the implementation record below (169 → 208) are snapshots, not the current state.
- **Grid self-flat is the default** model (`--self-flat-model=grid`); the v1 `registered` model and the Phase-1 least-squares path remain available for A/B.
- **Darks are not fabricated.** Master darks come only from real dark exposures.
- **`--synthetic-dark`, `--synthetic-flat`, `--synthetic-bias` are refused (exit 1)** with the exact strings in §13. Silent synthesis is forbidden.
- Architecture constraints: single Go source file (`main.go`), standard library only at build time, `CGO_ENABLED=0`, static cross-platform CLI, float32 RGB end-to-end, spill files + tiled out-of-core reduction.

This is the single canonical calibration document. It **absorbs the three former calibration research files** — the append-only v1 `04-calibration-and-self-flat.md` record, `research/SYNTHETIC_CALIBRATION_RESEARCH.md` (deep-research pass + preserved seed), and `research/Synthetic generation of calibration frames for astrophotography.md` — together with the calibration-relevant excerpts of the 2026-11-16 deep dive, the GEMINI deep-research outcome, and the cycle-5 curated corpus. Every unique fact, formula, parameter, validation number, date, tool, status, gap, flag and code symbol from those sources appears exactly once below; this file is a rewrite, not an append-only log.

**Canonical cross-links** (do not duplicate those topics here): [01-architecture-and-core-engine.md](01-architecture-and-core-engine.md) (core engine, resampling, `--resample-sip`, Go/toolchain), [02-astrometry-and-identification.md](02-astrometry-and-identification.md) (plate solving, SIP/WCS, Gaia), [03-reference-and-literature.md](03-reference-and-literature.md) (bibliography catalog), [04-calibration-and-self-flat.md](04-calibration-and-self-flat.md) (this file), [05-toolchain-and-modularity.md](05-toolchain-and-modularity.md), [06-external-ai-sidecars.md](06-external-ai-sidecars.md), [07-native-cr2-decoding.md](07-native-cr2-decoding.md), [08-crawler-source-discovery.md](08-crawler-source-discovery.md), [README.md](README.md). Astrometry, toolchain/Go-runtime, external-AI-sidecar and CR2-decoding detail is cross-linked, not repeated.

## Contents

1. Scope and three-category taxonomy
2. Master export (`--export-masters`)
3. Physical model and calibration literature (tagged)
4. Canonical 10-component recoverability table
5. Self-flat v1 — registered-space derivation (baseline)
6. Self-flat v2 — sensor-space grid estimator
7. Identifiability, degeneracies and physical limits
8. Integration map (v1 code → sensor-space pipeline)
9. Validation and acceptance criteria
10. Merged roadmap — Phases 0–7 and Tiers 1–4
11. Implementation history, self-test counts and full validation record
12. Refusal policy — exact strings and silent-synthesis rule
13. Detector-physics research survey (not implemented)
14. Master-frame practice from curated sources
15. Consolidated open gaps
16. Sources absorbed

---

## 1. Scope and three-category taxonomy

Calibration work splits into three categories that must never be conflated. A synthetic frame can reproduce a known systematic pattern without reproducing the actual random noise or time-dependent behaviour of the original exposure.

| Category | Definition | SkyStacker position |
| --- | --- | --- |
| **Mathematical correction** | Estimate and remove a specific defect directly from the light frames (post-processing, not calibration). | Partly shipped: `--bg-degree 1-3` illumination-gradient removal in `removeBackground`, `--local-normalize` (`localNormalizeWithRefTiles`) vignetting/low-order response removal, `sigmaClipReduceWeighted` hot-pixel/outlier rejection. |
| **Empirical reconstruction** | Recover the fixed detector response from the lights themselves (dithered self-calibration). The only scientifically defensible category from lights alone. | Shipped as the sensor-space **grid self-flat** (§6). Highest-priority category. |
| **Synthetic generation** | Model sensor/electronics and emit a replacement calibration frame. | **Refused** for dark/flat/bias (§12). A Tier-4 (XL) item recorded only to show it was considered and rejected. |

Consequences of the split (from the deep-research pass, 2026-11-14):

- The seed document's "reliability" table (its §5, assigning High/Medium/Medium-high to ten rows) is **not defensible**: no primary source in its bibliography reports a per-component accuracy figure in that form. It is replaced by the per-component evidence in §4; rows backed by a primary source (Zaritsky 2024 for flat recovery from science frames; Siril's synthetic-bias note for the pedestal-only claim) are cited there, all others are marked "no measured figure found".
- "Synthetic bias is one of the easiest calibration frames to approximate" is **over-claimed**: only the *constant pedestal* is recoverable; the spatial readout pattern is not recoverable from lights alone. The seed's §1 distinction is correct; "easiest" conflates a trivial scalar with an impossible full frame.
- The seed's §4 "most promising technique" — empirical self-calibration from dithered lights — is the only category that survives the project's own test and the only one that does not fabricate a frame (Zaritsky et al. 2024, AJ 168, 69; Bramich/Freudling illumination modelling, MNRAS 438, 1256, 2014).
- The seed's flat model `F = V·P·Q` is correct as a model but **not separable from lights alone**: V (illumination) and a low-order approximation to P (pixel response) are recoverable from dithered science frames, but Q (dust shadows) requires the actual optical train and a real flat — a dust mote casts a pupil-plane shadow convolved with the PSF, and the pupil is not present in the lights.
- "A constant value cannot reproduce spatially varying electronic patterns" is the headline: silent synthesis is worse than no calibration.

**Evidence-tag convention** (inherited from the deep-research pass): every quantitative or model claim in §3 is marked **[measured]**, **[derived]** or **[assumed]**; a claim with no tag is a documentation bug. The tags distinguish literature measurements, geometric/algorithmic derivations, and unmeasured architectural assumptions.

---

## 2. Master export (`--export-masters`)

Code: `exportCalibrationMasters`, `writeMasterPreviewJPEG` in `main.go`.

For every non-nil master (bias, dark, flat, and the derived self-flat) the pipeline writes two files:

| File | Content | Role |
| --- | --- | --- |
| `master_<kind>.f32raw` | 8-byte little-endian header (`width`, `height` uint32) + interleaved RGB float32 | scientific artifact |
| `master_<kind>.jpg` | 8-bit linear map of the 1st..99.5th luminance percentile to 0..255 | preview only |

- The percentile window is computed over the whole master; if it collapses (`hi <= lo`) it falls back to min/max, then to 0..1. The JPG is **never** used for measurement.
- Per-master statistics (min, max, channel means) are appended to `report.txt`.
- `.f32raw` is deliberately the same format as `_work/*.f32raw`, so `readRawFloat32RGB` and existing inspection scripts work unchanged.
- Bias/dark/flat masters are full-frame. In **v1** the self-flat is crop-sized (dimensions equal the safe crop) and `report.txt` gives the crop origin in the reference frame; in the **sensor-space grid model** the self-flat master is full-frame (`refW × refH`) and not tied to the reference crop (§6).
- The export path is unchanged across all self-flat phases.

---

## 3. Physical model and calibration literature (tagged)

### 3.1 Dark-current model

A synthetic dark would require separate estimation of physically distinct phenomena: dark current accumulated during exposure, hot/warm pixels, fixed-pattern dark-current variation, amp glow, thermal noise, readout offset and read noise. The physical model is

```
D(x,y) = B(x,y) + t * I_d(x,y,T) + N(x,y)
```

- `B(x,y)` bias pattern; `t` exposure duration; `I_d(x,y,T)` pixel- and temperature-dependent dark-current map; `N(x,y)` stochastic noise. **[derived]** from the detector model in the seed document.
- Dark current is strongly temperature dependent; a single-nominal-temperature model is inadequate if sensor temperature varies. **[assumed]** from consumer CMOS architecture, not measured here.
- OSIRIS-REx camera-suite calibration characterises dark current as a function of exposure and temperature and builds master darks from multiple observations **[measured]**; no residual RMS is published in the cited copy, so its transfer to consumer CMOS is **[assumed]**.
- Fossum & Shade, JATIS 2025: photon-counting CMOS characterisation and dark-current distributions, plus time-domain oversampled CMOS noise analysis **[measured]** (literature; not implemented in SkyStacker).
- No 2024–2026 peer-reviewed study or verified algorithm in the gathered material models dark/bias/FPN/PRNU frames purely from light exposures, or regresses temperature-dependent dark current from uncooled DSLR EXIF metadata **[measured: none found]**.

### 3.2 Flat model and illumination

```
F(x,y) = V(x,y) * P(x,y) * Q(x,y)
```

`V` optical illumination/vignetting, `P` pixel-response non-uniformity, `Q` dust shadows/obstructions. **[derived]** model; the three terms are fixed in the detector frame but only `V` and low-order `P` are recoverable from dithered lights; `Q` needs the optical train.

Radial vignetting approximation:

```
V(r) = 1 + a2*r^2 + a4*r^4 + a6*r^6
```

Approximates smooth vignetting but cannot reconstruct pixel-level sensitivity **[derived]**.

- Zaritsky et al. (2024), AJ 168, 69: master flats built from science images from the same night, using **≥15 images** and rejecting unsuitable illumination patterns **[measured]**. This is the origin of the `selfFlatMinFrames = 15` gate.
- Liu et al. (2023), *A Recipe for Unbiased Background Modeling in Deep Wide-Field Astronomical Images* (arXiv:2306.07922): unbiased background modelling **[measured]**; the seed's "Medium-high" reliability label is not a measurement. An aggressive synthetic flat can remove real extended structure (galaxies, nebulae, cirrus) **[derived]**.
- Bramich/Freudling illumination modelling, MNRAS 438, 1256 (2014), and the 2016 auroral EMCCD synthesis paper (uniform-illumination vs dithering flat construction) **[measured]** precedents for empirical self-calibration.
- Siril synthetic-bias tutorial **[measured]**: subtracting a bias is not equivalent to removing read noise; only the constant pedestal is well approximated. Astropy CCD Reduction Guide gives the calibration mathematics and simulated detector sensitivity **[measured]**.
- PixInsight 1.9.6 "Lockhart" finite-sample corrections: the standard 1.4826 MAD→Gaussian-sigma factor **underestimates scale at N = 3–30**, causing over-rejection of genuine peaks/faint nebulosity; exact consistency constants are now documented platform-wide **[measured]** (release notes). Applicable to SkyStacker's small amateur sessions (4–16 frames) but not yet wired in **[derived]**.

### 3.3 Sensor-space observation model (basis of the grid self-flat)

For calibrated but **unregistered** frames, with `I_k(x,y,c)` the calibrated frame for exposure `k`, channel `c`, in sensor coordinates:

```
I_k(x,y,c) = F(x,y,c) * S_k(x,y,c) + noise
```

`F` is the multiplicative illumination fixed to the sensor (vignetting, transmission, microlens response); `S_k` is the astrophysical scene plus sky, which moves with dithering/tracking. Goal: a smooth, positive, full-frame estimate of `F` up to an arbitrary multiplicative constant. **[derived]** from the geometry; validated by recovery experiments in §11.

Pixel-response non-uniformity is averaged out, not recovered, by dithering **[derived]**; the exact telescope illumination profile is unrecoverable because the exit pupil is absent from the lights **[derived]**.

---

## 4. Canonical 10-component recoverability table

This is the single merged replacement for the seed's qualitative reliability table and the per-component evidence/SkyStacker-fit sections of the deep-research pass. "Verdict" uses: *recoverable*, *partially*, *statistic only*, *requires real exposure*, *unrecoverable*, *refused*.

| # | Component | Verdict | Needed input | SkyStacker path / status | Test / falsifiable check |
| --- | --- | --- | --- | --- | --- |
| 1 | **Bias pedestal** (constant) | Recoverable (scalar) | EXIF `BlackLevel` or FITS `BZERO`/`BLACK_LVL` | `calibrate()` subtracts bias but does **not** read EXIF today; planned `--black-level auto` / `subtractBlackLevel()` (Tier 1, not shipped). Pedestal scalarity is **[assumed]** from sensor architecture. | `testSubtractBlackLevel`; subtract pedestal from `--no-register` stack, assert 1st-percentile histogram within 1 ADU of zero. No measured figure found for metadata pedestal subtraction. |
| 2 | **Spatial bias pattern** | Requires real bias; unrecoverable from lights | Real bias exposure | Refused for synthesis. No path. | Synthetic bias applied to a light and a dark must differ from a real bias by the same signed amount. No measured figure found. |
| 3 | **Read noise** | Statistic only | Nothing for the statistic; real bias for the realisation | `estimateFrameBgNoise` characterises it; report-only plan to add per-frame sigma to `report.txt`. Gaussian **[assumed]**, refined to Gaussian-plus-outlier by OSIRIS-REx **[measured]**. | `testFrameBgNoiseReported`; a synthetic read-noise frame added to a stack must not change MAD beyond the expected 1/√N contribution. |
| 4 | **Dark current** | Requires real dark at same exposure/gain/temperature | Real dark; temperature characterisation for transfer | `calibrate()` subtracts master dark; synthesis refused; darks never fabricated. Dark-series QA implemented (§14). | Scale synthetic dark by exposure ratio vs a real dark at second exposure; must match within read-noise floor. OSIRIS-REx **[measured]**, transfer **[assumed]**. |
| 5 | **Hot pixels** | Recoverable (location) | Nothing | `sigmaClipReduceWeighted` + `stacked_rejection_map.jpg`; `--bpm PATH` bad-pixel map (Tier 2, shipped). Detection of location is **[derived]** from the algorithm. | `testBPMMasksPixels`; rejected pixels in a 10-frame light stack must be a superset of a 10-frame dark-stack map. No measured figure. |
| 6 | **Amp glow** | Requires real dark; unrecoverable from lights | Real dark (exposure-/temperature-dependent) | Refused for synthesis. No path. | Synthetic glow must scale with exposure relative to a real dark. No measured figure. |
| 7 | **Vignetting** | Partially recoverable (low order) | Dithered lights; real flat for high order | `--local-normalize` (shipped, low order) and grid self-flat `V`/`F` field (§6). Zaritsky ≥15 **[measured]**; equivalence of `--local-normalize` to the same component **[assumed]**. | Compare `--local-normalize` field to real flat for the same session; residual > 1% RMS means reconstruction insufficient. |
| 8 | **Dust shadows** | Requires real flat; unrecoverable from lights | Real flat + unchanged optical train | Refused for synthesis; out of scope for polynomial self-flat (§7). Q shadow is pupil-plane/PSF-convolved **[derived]**. | Synthetic dust map must not move the dust shadow's position relative to the real flat. No measured figure. |
| 9 | **Pixel-response non-uniformity** (PRNU) | Not recoverable from lights; averaged by dithering | Real flat | Partially averaged by `--local-normalize`; synthesis refused. No measured figure. | Residual of `--local-normalize` frame vs real flat must be smaller than uncorrected by the expected √N factor. |
| 10 | **Illumination gradient** | Recoverable (low order, post-processing) | Nothing | `--bg-degree 1-3` in `removeBackground` (shipped). Liu et al. 2023 unbiased modelling **[measured]**; "Medium-high" seed label is not a measurement. | Background-modelled frame must not remove a synthetic 100 px Sérsic galaxy (over-aggressive model). |

The single-file/stdlib-only constraint rules nothing in or out here: shipped items (`--local-normalize`, `--bg-degree`, `sigmaClipReduceWeighted`) already respect it, and refused items would be refused regardless because no dependency can supply the missing sensor.

---

## 5. Self-flat v1 — registered-space derivation (baseline)

Code symbols: `selfFlatEligible`, `buildSelfFlat`, `normalizeSelfFlat`, `centerEdgeRatio`, `flatResidualRMS`, `applyFlatToFrame`, `reduceTile`, `spillPaths2`, `spillMedians`, `flatCorrection`, `frameScales`.

**Why after registration (v1):** a sensor flat corrects pixel response and dust in *detector* coordinates. A flat built from lights cannot do that — each light is resampled into the reference frame, smearing fixed-pattern defects. What survives alignment is the relative illumination field (vignetting + large-scale throughput) in reference coordinates. v1 therefore applies the flat **after** registration, never through `calibrate()`; only the registered stack is corrected, the unregistered stack remains uncorrected.

Let `L_i(x)` be the aligned, calibrated light `i` at reference pixel `x`, `Y = (R+G+B)/3`.

1. **Sky-level normalisation:** `m_i = median_x Y(L_i(x))`, target `m̃ = median_i m_i`, scale `s_i = m̃ / m_i`.
2. **Reduction:** one-iteration sigma-clipped mean of scaled aligned frames (rejects moving stars and cosmic rays) → `F_0(x)`.
3. **Normalisation:** per channel divide by channel median (flat median 1); clamp to `[0.05, 20]`; NaN/Inf/non-positive → 1.
4. **Sanity gate:** `R = F̄_center / F̄_edge` (outer 10% border vs central 10% box); reject if `R < 0.5` or `R > 5`.
5. **Application:** divide each aligned frame by `F` inside the safe crop before accumulation (`applyFlatToFrame` for `mean`; the `flat` argument of `reduceTile` for `median`/`sigma`).

Gate: at least **15 registered frames** (`selfFlatMinFrames = 15`) with spill data. An explicit `--flat` (or one discovered via `--calib-root`) always wins. Without registration `--self-flat` is refused.

Diagnostics: `center/edge` = `R`; `residual RMS` = `sqrt(mean((F-1)^2))` over pixels with `0.2 ≤ F ≤ 5` — a departure-from-unity metric, **not** a goodness-of-fit metric.

**Why the registered-space model overcorrects (2026-11-15 revision):** v1 builds the flat from `spillPaths2` (already resampled). Aligned stars and nebulosity occupy the same pixels in every frame, so the sigma-clipped mean sees them as a *common* component like vignetting; the normalised result is "illumination + static sky". Dividing the registered stack by it suppresses real structure and can create dark halos around bright objects. It is not literally "dividing the stack by itself" (one-iteration sigma-clipped mean vs weighted/rejected stack, normalised to median 1), but the correlation is the problem. On Orion-135mm the map reached center/edge 1.2512 and RMS 0.1936, near 1.9 around M42 (`max 9.20992`) — mostly scene, not vignetting. Sigma clipping does not help: with ~13 px dithers stars stay inside the rejection neighbourhood and a large nebula is present in every frame; a Gaussian blur or low-order polynomial on the aligned mean preserves the scene's low-frequency power. The estimator must see the scene *move* and explicitly exclude it. v1 is also crop-sized and reference-frame-bound, hence not a sensor model and not applicable before resampling. It is useful only as an A/B baseline behind `--self-flat-model=registered`.

---

## 6. Self-flat v2 — sensor-space grid estimator

This is the current default (`--self-flat-model=grid`). All formulas and parameters below are the shipped design (Phases 1–3; §10–§11 of the former record).

### 6.1 Observable and per-frame normalisation

Work on calibrated but **unregistered** frames (the data Pass 1 already has before `resampleToReference*`), using the model of §3.3: `I_k = F · S_k + noise`.

```
I'_k(x,y,c) = I_k(x,y,c) / median_unmasked(I_k(.,.,c))
```

Per-channel median normalisation removes exposure/transparency differences without removing the spatial illumination pattern; separate channel scales keep the relative flat non-achromatic. If a frame is mostly masked, fall back to its global median and mark it low-quality for the flat.

### 6.2 Source mask

The **shipped** (Phase 2) mask works on local contrast, not a global threshold:

- luminance `L = 0.2126 R + 0.7152 G + 0.0722 B`;
- subtract a separable `(2r+1)^2` box mean with `r = 8`, clipped at the border;
- residual scale from `1.4826 * MAD(L - box)` on a strided sample, floored at `0.008` so a noiseless gradient is not masked by numerical structure;
- mask residual `> median(residual) + 5*scale`, plus saturated and invalid (`-1`) pixels;
- dilate by **12 px** for PSF wings and blooming (Phase 1 used a global `median + 2.5*MAD` threshold and a 12 px box dilation scaled down on small frames; this over-masked extended emission).

Shared mask across channels. Extended emission raises local background and pixel nearly equally, so its residual stays below threshold; stars keep full contrast. Cells with too few unmasked pixels are invalid, not silently filled. The earlier design described dilating by 10–15 px; the implemented value is 12 px.

### 6.3 Coarse grid and cell estimator

- Square cells of **64 px** (default; shrunk only if a frame would have fewer than six cells on a side). A 2400×1600 frame gives ~38×25 cells at 64 px, ~25×17 at 96 px. Degree-3/4 fitting wants hundreds of cells.
- A cell is valid only with enough unmasked samples (Phase 1: 25th percentile of unmasked normalised pixels requiring 30% of the cell; across frames the median of those quantiles requiring 15 valid frames).
- **Shipped cell value (Phase 2)** — SExtractor-style mode, replacing the 25th percentile:

  ```
  mode = 2.5*median - 1.5*trimmed_mean(10%..90%)
  ```

  Lower/upper quartiles are kept as a contamination check: reject the cell before the fit if the upper quartile is farther from the median than `1.6x` the lower distance (symmetric sky ≈1; nebula/star wing inflates the upper tail). Clamp `mode` to the low quantile if non-positive. Across frames: median of per-frame cell values with frame sample counts as fit weights.

- Prototype alternatives (design record): per-frame-per-cell 20–25% quantile then median/25% across frames (cheap Phase-1 prototype); low quantile (20–25%) as a conservative but noise-low-biased diagnostic; smoothed histogram peak as the least-biased sky estimate. Mode and low quantile disagreeing by more than a few percent implies a second population (nebula, bright star, dust) → reject cell. A pure low-quantile estimator slightly over-brightens corners because noise darkens the low tail.

### 6.4 Grid outlier rejection (5×5)

After building the cell grid, per channel collect valid cells in the 5×5 neighbourhood, compute median and MAD, and reject when

```
|B - median_5x5(B)| > 2.5 * MAD_5x5(B)
```

Rejected cells get **zero weight and are not interpolated**; the polynomial bridges the gap. Report `cells=valid/total (rejected=N)`; `RejectedCells` counts unique cells rejected in at least one channel. This removes residual stars, nebula cores, satellite trails and dust shadows that survived the pixel mask.

### 6.5 IRLS polynomial surface (Phase 3)

Per channel, fit a degree-4 Chebyshev tensor surface on `u,v ∈ [-1,1]`:

```
F(x,y,c) = sum_{p=0..d} sum_{q=0..d} a_pq,c * T_p(u) * T_q(v)     (d = 3 or 4 → 16 or 25 coefficients)
```

- **Base fit:** weighted least squares using cell sample counts — also the IRLS initialisation and the fallback if a robust iteration fails.
- **Robust refinement:** residual scale `1.4826*MAD(|r|)`, floored at `1e-6`. Huber weights `min(1, c*sigma/|r|)` with `c = 1.5` for positive residuals (brighter-than-surface: residual star/nebula tails) and `c = 3.0` for negative ones. At most **15 iterations**, stopping when the largest coefficient change is `< 1e-7`. (Earlier design: 10–20 iterations, Huber or Tukey biweight, degree-2/radial initialization.)
- **Diagnostics:** `fit RMS` (robust residual over valid cells), `OLS RMS` (same basis, no IRLS), `radial RMS` (fit on `1, r^2, r^4, r^6`); all normalised by fitted channel means. `report.txt` prints the 25 Chebyshev coefficients per channel.
- **Extrapolation/positivity gate:** reject if `center/edge < 0.5 || > 5.0`, `min < 0.25`, or `max > 4.0`; emit a note when an edge cell was dropped so the model had to extrapolate to the border. Earlier design clamp range `[0.1, 10]`; normalise per channel by mean or centre value. Degree 5 starts chasing scene residuals; a radial model is more constrained and preferred when real flats look radial.
- The polynomial is a strong prior: it cannot reproduce stars, dust or a small nebula — which is why it is preferable to a Gaussian blur or a per-pixel aligned mean — but it can absorb a smooth sky gradient (§7).

### 6.6 Full-frame output and application order

Write `master_self_flat.f32raw` at full sensor resolution with the standard 8-byte header, plus `master_self_flat.jpg` preview. Apply to each calibrated frame **before** resampling in Pass 2:

```
f.Pix /= F(x,y,c)   // sensor coordinates, full frame
resample(f)
```

This is the physically correct order; the registered stack then needs no flat-aware reduction code. For unregistered outputs, dividing the reduced sum/mean (and median/max/min maps) by `F` after reduction is mathematically equivalent because `F` is constant across frames, and much cheaper. `stacked_no_correction_mean` stays intentionally uncorrected; other unregistered outputs are corrected consistently rather than left mislabelled. Encoding note: this is encoded-space division (§7, gamma).

### 6.7 Streaming integration

Production integration streams in Pass 1 without `spillPaths1`:

1. Before Pass 1, allocate a `selfFlatGridAccumulator` (per-cell histograms or per-frame cell quantiles) when `--self-flat` is active.
2. In the Pass 1 loop, after `calibrate`, black-level subtraction and BPM, build the mask and add unmasked normalised pixels to the accumulator — a single streaming pass with a few MB of state.
3. After Pass 1, finalise the cell grid, fit and validate the polynomial.
4. In Pass 2, apply the full-frame flat after calibration/BPM and before resampling.

This removes the v1 mean-path re-accumulation, the `flatCorrection` type, and the `flat`/`frameScales` parameters of `reduceTilesStack`/`reduceTile`. A Phase-1 prototype could read `spillPaths1`; production must not use `spillPaths2` for the flat.

### 6.8 Flags and eligibility

- `--self-flat` (kept), `--self-flat-model` (`grid` default; `registered` for A/B), `--self-flat-degree` (default 4), `--self-flat-cell` (default 64); quantile/loss stay internal.
- `selfFlatEligible`: keep real-flat precedence and the 15-frame minimum; the registration gate is a v1 artefact and should be removed only once the unregistered application path is fully tested; add a dithering diagnostic (median dither translation, warn when very small, not a hard gate without evidence).

---

## 7. Identifiability, degeneracies and physical limits

These are properties of the single-session self-flat problem, not implementation bugs; report the relevant caveats in every self-flat run.

- **Static scene vs. flat:** in sensor coordinates the scene moves only by the dither. A nebula hundreds of pixels wide is effectively static at ~13 px dither; its smooth envelope cannot be separated from `F` without masking. The mask, small coefficient count and cell rejection are mitigations, not a proof.
- **Sky gradient vs. flat:** a stable spatial sky gradient (light pollution, Moon, twilight) is multiplicative with `F`; a 2D polynomial absorbs it. No single-night method separates them without a prior. A radial physical model is more identifiable than a general 2D polynomial (cannot absorb an arbitrary linear gradient): fit radial first, mild 2D residual second.
- **Dust shadows:** high-frequency multiplicative structure that a degree-3/4 polynomial cannot represent; the 5×5 cell rejection removes rather than corrects it. Real flats are required. Large dithers plus min/max rejection can reduce dust in the stack but ~13 px is not enough. Out of scope for the self-flat.
- **Gamma / encoded space:** the pipeline operates on sRGB-encoded values; the flat is multiplicative in linear light. Estimating and applying consistently in encoded space cancels the first-order factor, but the sRGB transfer function is nonlinear (with a linear toe), so the correction is not exact; residual depends on sky level and vignetting strength. Small for `F ~ 0.8`, larger for strong vignetting. Options: (a) gamma-aware division via an sRGB EOTF LUT (local, testable), or (b) a later linear-light pipeline (larger change to normalisation, sigma clipping and outputs). Do not silently mix. Not implemented.
- **Colour:** vignetting/transmission are wavelength dependent; fit and normalise per channel with a shared mask. A luminance-only flat is cheaper but leaves colour-dependent corners.
- **Noise:** the low quantile is biased low; more frames and larger cells reduce bias, and the robust mode is less biased. Report fit RMS and samples per cell.
- **Dithering:** the method needs relative scene/sensor motion; a few px plus a mask can work for stars, not for a large nebula. Compute and warn on small median dither; do not hard-gate without evidence.
- **Registration requirement:** the flat itself does not need registration (built in sensor coordinates); the v1 gate reflects v1's application order. Keep it only until the unregistered path is tested.
- **v1-specific interpretation:** astrophysical bias is real — M42 is common to every frame and not a sigma-clipping outlier, so the v1 master preview shows center/border near 1.9 and the `.f32raw` is the honest artifact, not the preview. JPEG display pedestal compresses the recovered ratio; the `.f32raw` and report numbers avoid it. `--local-normalize` may remove the same large-scale signal twice when combined with `--self-flat`.

---

## 8. Integration map (v1 code → sensor-space pipeline)

Line numbers approximate; function names are the stable reference.

1. **Pass 1 spill/accumulation** (`main.go` ~5878, ~5951): v1 spills `spillPaths1` only for `median`/`sigma`; stream in Pass 1 (preferred) or force `spillDir1` when `--self-flat` is active so `mean` also has unregistered frames. Never use `spillPaths2` for the flat.
2. **Build flat before Pass 2** (replace `buildSelfFlat`, ~5113): must be known before the resampling loop; v1 builds after Pass 2 because it needs `spillPaths2`. Use `refW × refH`, no `cropRect`.
3. **Apply before resampling** (Pass 2 after `calibrate`/BPM, ~6326): replace post-registration `applyFlatToFrame` with full-frame sensor correction, then `resampleToReference*`.
4. **Remove the registered mean self-flat special case** (~6298, ~6351, ~6590): no `spillPaths2`, `spillMedians`, post-derivation or mean re-accumulation; accumulate corrected resampled frames directly.
5. **Unregistered outputs** (~6620): post-divide unregistered mean after `divideByCount`, median/sigma `unregMeanPix`/`unregSumPix` after `reduceTilesStack`, and the max/min maps; preserve `stacked_no_correction_mean`.
6. **Reduce the reduction API** (~4621, ~4718, ~4826): remove `flatCorrection` and the `flat`/`frameScales` reduction parameters once the flat is applied before resampling; keep `frameScales` only if documented.
7. **Result and report** (`selfFlatResult` ~5067; report ~3074/3150/7071): add `Model`, `Degree`, `CellSize`, `ValidCells`, `RejectedCells`, `MaskedFraction`, `FitRMS`, `Min`, `Max`, `DitherMedian`; keep `CenterEdge` and `ResidualRMS` but redefine the latter as "fit residual"; emit model and coefficients.
8. **Flags:** keep `--self-flat`; the temporary `--self-flat-model` default moved to `grid` only after acceptance (§11); add `--self-flat-degree`/`--self-flat-cell`; document planned flags only once they exist.
9. **Eligibility:** real-flat precedence + 15-frame minimum; remove the registration gate when the unregistered path is tested; add dithering diagnostic.
10. **Master and docs:** full-frame master; update README, TECHNICAL_SPEC, prompt-ai and the historical v1 note; `--export-masters` unchanged.

---

## 9. Validation and acceptance criteria

Synthetic tests separate "looks flatter" from "recovered the illumination": generate a known `F_true`, a scene with moving stars and a large static nebula, and check the recovered flat, not just the output image.

- **Known vignetting:** `F_true = 1 - a r^2 - b r^4`, center/edge 1.2–1.4; recovered RMS < 0.02 in valid region, center/edge within 0.02 of truth.
- **Moving stars:** different positions each frame; no star-shaped residuals; compare with/without the source mask.
- **Large nebula:** Gaussian or M42-like extended source, dither ≪ source size; flat at nebula location within 0.02 of `F_true` — regression test for the v1 failure.
- **Sky gradient:** radial model must not absorb it; 2D polynomial must emit the warning/report — documents the degeneracy rather than solving it.
- **Grid rejection:** bright patch + dust shadow; affected cells rejected, surrounding fit unbiased.
- **IRLS:** outliers in the cell grid; Huber/Tukey must beat OLS by ≥2× and stay < 0.02 RMS.
- **Application order:** synthetic translation/rotation; flat-before-resampling matches sensor-space illumination; flat-after-resampling measurably worse.
- **Methods and registration:** `mean`, `median`, `sigma`, with/without `--no-register`; corrected outputs consistent and unregistered stack corrected.
- **Gamma:** with gamma-aware division a synthetic sRGB vignette must flatten within 1% (naive division must fail the tighter bound); otherwise the report must warn about encoded space.
- **Master round-trip:** full-frame dimensions, header, preview, `report.txt` diagnostics; `--export-masters` still writes three notes.
- **Real-data acceptance (outside repo):** compare derived flat with the real flat from OrionNebula (17 flats) or 04_emission_nebula_Flame (1 flat) after resampling to the same geometry — expect smooth agreement within 5–10% in the valid region, report dust-scale differences. On Orion-135mm the M42-region ratio must not reach ~1.9, map smooth/plausible, no dark halo. Re-run the section-E calibration rejection tests unchanged; do not weaken thresholds to pass.

---

## 10. Merged roadmap — Phases 0–7 and Tiers 1–4

**Method drift (one lineage, three estimators):** the original self-flat used a **median** (one-iteration sigma-clipped mean) of registered lights in reference space; the sensor-space prototype replaced it with a **per-cell/per-frame low quantile + plain least-squares polynomial**; the production default is now the **grid Chebyshev degree-4 + asymmetric Huber IRLS** with masking and 5×5 rejection. Each step exists to stop the estimator absorbing the static sky. Tiers 1–4 are the small-to-architectural delivery bands from the synthetic-calibration research.

**Calibration Phases**

- **Phase 0 — analysis and guardrails (done, no code change).** Document v1 failure, sensor-space design, degeneracies, roadmap; test count stayed 169.
- **Phase 1 — sensor-space prototype (shipped 2026-11-15).** Streaming accumulator, per-frame median normalisation, 64 px cells, low quantile, degree-4 least squares, pre-resampling application; no registration requirement to build. Acceptance: M42 dome gone, map full-frame, at least as flat as v1. Left a 67.6% mask fraction and low corner (0.49).
- **Phase 2 — robust masking and cell rejection (shipped 2026-11-15).** Local-contrast mask, dilation, SExtractor mode, 5×5 median/MAD rejection, mode/quantile consistency; nebula cutout stays within 0.02 of truth.
- **Phase 3 — IRLS polynomial (shipped 2026-11-16).** Chebyshev degree 4, Huber asymmetric weights, validity weights, positivity/extrapolation checks, radial comparison; robust fit beats OLS on outliers.
- **Phase 4 — integration and cleanup (planned).** Remove `flatCorrection`, `flat`/`frameScales`, mean re-accumulation; correct unregistered mean/median/sigma and max/min; relax registration gate, test `--no-register`; update report/docs; E2E Orion mean/sigma. Default already switched to `grid`; full cleanup still pending.
- **Phase 5 — gamma-aware correction (not implemented).** sRGB EOTF LUT; flat division in linear light behind a flag; decide default from synthetic/real comparisons. Independent axis — do not mix with Phases 1–4.
- **Phase 6 — real-flat validation (informal only).** Compare with OrionNebula/Flame flats, quantify smooth agreement and dust residual; scientific acceptance gate for "validated".
- **Phase 7 — dust/defect handling (optional, out of scope).** Requires real flats or a dedicated high-frequency residual model; do not promise it in flag descriptions.

Priority: Phases 1–4 replace the unsafe default; 5–6 establish scientific quality; 7 is explicitly out of scope. Each phase keeps `gofmt`/`go vet` clean, keeps the section-E rejection tests intact, and updates the self-test count and `PROJECT_CONTEXT.md`.

**Delivery Tiers (from the synthetic-calibration research)**

- **Tier 1 — Small (S):** (1) `--black-level auto` reading EXIF `BlackLevel`/FITS `BLACK_LVL` and subtracting the scalar pedestal in `calibrate()` (off by default to avoid a wrong tag biasing every frame); (2) report per-frame read-noise sigma from `estimateFrameBgNoise` (report-only).
- **Tier 2 — Medium (M):** (3) `--bpm PATH` bad-pixel map loaded once, masked in pass 1 (explicit opt-in to avoid masking real stars) — **shipped**; (4) `--self-flat` from dithered lights, ≥15 registered frames, median across frames after star rejection — **shipped and evolved** into the grid estimator (risk of removing nebulosity mitigated by gating + residual report vs `--local-normalize`).
- **Tier 3 — Large (L):** (5) per-component confidence map attached to `report.txt` — report-only, not shipped.
- **Tier 4 — Architectural / out of scope (XL):** (6) synthetic bias/dark/flat generation — **refused**; tier exists only to record that it was considered and rejected (§12).

---

## 11. Implementation history, self-test counts and full validation record

### 11.1 Chronology and self-test counts

Registered self-test counts are snapshots: **169** (v1 baseline) → **173** (Phase 1) → **177** (v0.6.0 status note) → **178** (reference-crop addendum) → **179** (Phase 2) → **181** (Phase 3) → **203** (GEMINI survey snapshot) → **208** (deep-dive correction, 2026-11-17) → **231 (CURRENT)**. Do not treat every `func test*` as a registered case.

- **2026-11-14** — deep-research pass supersedes the seed; three-category split, tagging convention, refusal guardrails.
- **2026-11-15** — v1 master export + registered self-flat; Phase 1 grid default; `--reference-crop` added (each light star-matched to an external reference and warped in memory; see `COMPARISON.md` for Fitswork comparison); v0.6.0 status note: `--export-masters`, `--bpm`, v1 `--self-flat`, Phase-1 grid, and default recoverable path (metadata pedestal + grid self-flat when no user masters exist); darks not fabricated; `--synthetic-*` refused. Not implemented then: Phase 2 mask relaxation/5×5 rejection, Phase 3 IRLS, gamma-aware division, real-flat validation.
- **2026-11-16** — Phase 3 IRLS shipped; deep dive synthesised; OrionNebula grid-vs-real-flat quick and protocol runs; SIP-in-resampler limitation note.
- **2026-11-17** — GEMINI outcome append: `--resample-sip` implemented (§15) and single-pass sigma-clip equivalence; curated cycle-5 master-frame corpus; deep-dive correction to v0.6.1.

### 11.2 Orion-135mm validation (no real flat; 66 lights, 2400×1600, crop 2395×1583)

Command (v1): `./skystacker assets/Orion-135mm --self-flat --export-masters --stack-only --method mean --no-cache --output /tmp/orion_selfflat`.

**v1 registered model** — registered frames 66; self-flat center/edge **1.2512**; residual RMS **0.19360**; mean preview center/border uncorrected **1.134** → self-flat **1.000**. Recovered ratio matches measured vignetting (1.240–1.253); corrected mean flat at the 1% level. `--method sigma --limit 20` reproduced **1.2497** and **1.000** (exercises `reduceTile`, not the `mean` re-accumulation). E2E outputs: `/tmp/orion_selfflat/` (mean), `/tmp/orion_selfflat_sigma/` (sigma); never write into `assets/Orion-135mm/0_skystacker_results/`. v1 master preview reached ~1.9 near M42 with raw `max 9.20992` — scene, not flat.

**Grid model, all three phases** (`--limit 20 --method mean --stack-only --self-flat-model=grid --export-masters`; `/tmp/orion_grid`, `/tmp/orion_p2_grid`, `/tmp/orion_p3_grid` — not repo output):

| Quantity | Phase 1 | Phase 2 | Phase 3 |
| --- | --- | --- | --- |
| Mask fraction | 67.6% | 31.7% | 31.7% |
| Valid cells | 391/950 | 851/950 (66 rejected) | 851/950 (66 rejected) |
| Fit RMS | 0.01337 | 0.01606 | **0.01634** |
| OLS RMS | — | — | 0.01608 |
| Radial RMS | — | — | 0.07496 |
| IRLS iterations | — | — | **11** |
| Center/edge | 1.2449 | 1.2226 | **1.2180** |
| Min / max | 0.4855 / 1.2081 | 0.5316 / 1.1315 | 0.5312 / 1.1300 |
| Master min / max | — | 0.48913 / 1.15756 | — |

Phase 1: full-frame master 2400×1600, center/edge **1.2449** (matches ~1.24 vignetting), fit RMS **0.01337**, **391/950** valid cells, mask **67.6%**; low corner 0.4855 flagged degree-4 extrapolation. Phase 2: corrected `stacked_mean` center/border **1.019** (within 2% of flat), M42 dome absent (grid max 1.13); sigma path reproduced cell statistics exactly; master min 0.489 is border extrapolation, not scene absorption. Phase 3: corrected `stacked_mean` linear center/border **1.023** (matches Phase 2); radial model much worse (0.075) because Orion has a non-radial illumination gradient, so the 2-D model remains correct; OLS RMS slightly lower on this clean field because it minimises squared cell residual, while IRLS trades a little RMS for contamination stability (synthetic outlier test wins decisively).

### 11.3 Synthetic regression tests

- `testSelfFlatEligible`, `testSelfFlatRecovery` (synthetic vignetting + moving outliers, RMS < 0.03), `testExportCalibrationMasters` (6 files, 3 notes, vignetting/dust/hot pixel visible in previews).
- `testSelfFlatGridRecovery` (RMS < 0.05), `testSelfFlatGridApplication`, `testSelfFlatGridNoRegistration`, `testSelfFlatModelDefault`.
- `testSelfFlatGridNebula`: 384×288, 30 frames, moving stars, static `sigma=24 px` Gaussian nebula, 32×32 bright patch and 32×32 dark (dust) patch; mask fraction < 50%, ≥1 cell removed by 5×5 filter, global flat RMS < 0.03, recovered flat at nebula centre within 0.02 of truth.
- `testSelfFlatIRLSRobust`: 16×12 cells, degree-4 truth, right four columns omitted, every ninth cell spiked +0.45; robust must beat OLS ≥2×, stay < 0.02 RMS, extrapolated edge within 0.05. `testSelfFlatRadialModel`: truth `1 + 0.12 r^2 - 0.03 r^4`; radial fit < 0.005 RMS, within 0.002 of 2-D polynomial, corners within 0.02.

### 11.4 OrionNebula validation (16 CR2 lights; real calibration data)

Dataset `assets/OrionNebula/` (16 lights in `lights/`, calibration in lowercase `darks/`, `biases/`, `flats/`). Initially only 3 lights (<15) so grid self-flat was not built; explicit `--dark/--flat/--bias` applied calibration and exported masters. After reaching 16 lights, a lowercase-vs-capitalised path issue (`Darks` vs `darks`) was fixed; case-insensitive calib discovery/fallback now resolves `--calib-root`/per-path args, and `detectCalibrationSubdirs` was corrected (§15).

Quick comparison (16 CR2):

- **Grid only** (`--dark --bias`, grid default): fit RMS **0.11222**, min **0.8737**, max **1.0979**, center/edge **1.1097**, extrapolated, masked **4.8%**, **2220/2542** cells valid. Approx `stacked_mean.jpg` luminance center/border (gamma-encoded): gamma ratio **~0.995**, linear approx **~0.990**.
- **Real flat** (`--dark --flat --bias`, no self-flat): approx `stacked_mean.jpg`: gamma ratio **~1.016**, linear approx **~1.033**.

Reproducible protocol commands (repo root):

```bash
./skystacker --output /tmp/onb_val_grid --overwrite --stack-only --self-flat-model=grid --dark assets/OrionNebula/darks --bias assets/OrionNebula/biases --limit 16 assets/OrionNebula/lights
./skystacker --output /tmp/onb_val_flat --overwrite --stack-only --dark assets/OrionNebula/darks --flat assets/OrionNebula/flats --bias assets/OrionNebula/biases --limit 16 assets/OrionNebula/lights
```

`report.txt` (2026-11-16): **grid** run (`/tmp/onb_val_grid`) — model=grid, 16 frames, degree=4, 64 px cells, **2220/2542** valid, rejected **308**, mask **4.8%**, center/edge **1.1097**, fit RMS **0.11222**, OLS **0.10119**, radial **0.12333**, IRLS **16**, min **0.8737**, max **1.0979**, extrapolated, with dark+bias. **Real flat** run (`/tmp/onb_val_flat`) — dark+flat+bias, no grid self-flat, sum normalisation factor **16.0**. This is an informal comparison (separate outputs, not a single toggle in one process); numerical parity is indicative only. Benchmark context (toolchain details in [01-architecture-and-core-engine.md](01-architecture-and-core-engine.md)): 16-CR2 runtime **6m05s** grid+dark/bias vs **7m03s** real flat; 4 lights ~2m02s.

### 11.5 Reproducibility checklist

`gofmt -l main.go` → empty; `go vet ./...` → clean; `./skystacker --selftest` → all registered tests pass (231 current). E2E outputs under `/tmp/` only; never write into `assets/Orion-135mm/0_skystacker_results/`.

---

## 12. Refusal policy — exact strings and silent-synthesis rule

A synthetic calibration frame that looks correct but is not the actual sensor response is worse than no calibration at all. The software must refuse and warn in these cases (exit 1, exact text):

- `--synthetic-dark`:
  `SYNTHETIC CALIBRATION REFUSED: dark current is exposure-, gain- and temperature-dependent; a synthetic frame cannot reproduce hot pixels, amp glow or the fixed-pattern dark current of THIS sensor. Use --dark with real frames.`
- `--synthetic-flat`:
  `SYNTHETIC CALIBRATION REFUSED: dust shadows and pixel-response non-uniformity require the actual optical train. Use --flat with real frames, or --local-normalize to recover the fixed illumination component from dithered lights.`
- `--synthetic-bias`:
  `SYNTHETIC CALIBRATION REFUSED: only the constant pedestal is recoverable (from EXIF/header metadata); the spatial readout pattern requires a real bias frame. Use --bias.`
- **Silent synthesis:** must never happen. Any code path that would fabricate a frame from lights alone must log a `WARNING` line and be gated behind an explicit opt-in flag whose default is `false`.

These warnings are the implementable form of the research stop condition and the guardrails landed before any generator. Current state: darks not fabricated; `--synthetic-dark/flat/bias` remain refused.

---

## 13. Detector-physics research survey (not implemented)

All items in this section are research directions or design references. **None is implemented in SkyStacker core.** Status markers: [KB-covered] = already in the knowledge base, [adjacent] = adjacent but not implemented, [new] = new to the project per the GEMINI survey; all are non-goals for the current default path.

- **sCMOS RTN/RTS** [KB-covered]: CCD 1/f read-noise floors vs CMOS random telegraph noise from charge traps near the source-follower oxide/silicon interface; integer-centred dark-sequence histograms fitted with **1G/3G/5G** Gaussian mixtures (robust fits, careful initial bounds, shared sigma for central peak/side lobes = shared RON, lobe positions from mean offsets; overfitting at higher mixtures needs human checks) — Özdoğru et al. context, May 2025. A synthetic master-dark simulator should sample non-Gaussian tails via per-pixel parameterised mixtures [new, not implemented].
- **H2RG / CDS** [KB-covered]: CDS vs pseudo-CDS alters RTS signatures; RTN detection fractions differed at **140 K vs 160 K** because small-amplitude peaks were masked by higher thermal read noise (Penn State H2RG X-ray context, March 2025).
- **SPAD (2026)** [KB-covered]: optical light leaks produce binary count-rate changes indistinguishable from intrinsic RTS unless shielding is controlled.
- **Brighter-Fatter Effect** [KB-covered]: Antilogus et al. 2014 (thin fully depleted CCDs) — accumulated charge perturbs drift fields and compresses fuller-pixel boundaries; 3–4 px stars grow in linear FWHM by up to **~2%** over the dynamic range, flat-field variance grows slower than mean through inter-pixel correlations. First-order neighbour charge transfer:

  ```
  δQ_0,0^X = Σ_i,j a_i,j^X · Q_i,j · (Q_0,0 + Q_X)
  ```

  with parity, translational and zero-sum constraints on the neighbourhood coefficients. **LSSTCam DMTN-329** [KB-covered]: BFE coefficients depend on **incident wavelength**; achromatic/linear corrections are insufficient for sub-percent photometry. Filter/wavelength-dependent kernels, additive vs multiplicative separation, and PRNU/DSNU distinction are conceptual only.
- **Ubercal** [KB-covered]: space-based (ACS/WFC F606W; Cohen, Sep 2024 / ISR ACS 2024-06) multiplicative L-flat over pixel flats via orthonormal 2D polynomials of combined order `Z`, giving `Z(Z+3)/2` free parameters per detector — **27 for Z = 6** — plus temporal amplifier decay terms `β` (mag/year). Magnitude-space formulation preserves linearity for high-SNR stars; equivalent flux-space form breaks it more. Synthetic gap tests (e.g. 50 px chip gaps) recovered injected parameters to **≲0.1%**.
- **FGCM** [KB-covered]: ground-based (DES/HSC PDR3/Rubin) physical two-stage atmosphere + chromatic system-response modelling instead of empirical star-flats with global linear ambiguities and missing chromatic terms.
- **HSC PDR3 sky** [KB-covered]: reinject subtracted sky per CCD before a global sky fit to avoid oversubtracting large galaxy halos.
- **Gnuastro NoiseChisel / Segment** [KB-covered]: mean–median asymmetry plus `--widekernel` to detect diffuse structure to very faint surface brightness (e.g. **~25.97 mag/arcsec² at S/N 0.25** in favourable cases) without eroding sharp edges (Borlaff, Müller, Trujillo references).
- **Self-supervised denoising** [KB-covered]: Noise2Void/Noise2Self assume uncorrelated zero-mean noise and avoid clean–noisy pairs; PAUS-style evaluation (arXiv:2611.05447v1) found **systematic flux suppression** in faint-source aperture photometry with standard N2V and **background artifacts** with PN2V under noise mismatch. AstroSURE notes constant sky dominates MSE if not subtracted first; cross-instrument transfer (HST→CFHT) limited; detectability gains do not validate photometric use. Do **not** adopt blind-spot denoisers as default without photometric validation.
- **TDR (Self2Self family)** [KB-covered]: single-image training, batch inference, restore pixels only when change exceeds threshold `R` bounded by initial noise (example: HMI LOS noise ~8 G → ~2 G with `R = 0.3`; 10 s HST exposures without underestimating strong peaks); document `R → ∞` (disables restoration).
- **ASTERIS (2026)** [KB-covered]: self-supervised spatio-temporal multi-exposure transformer (arXiv:2602.17205 style), JWST/Subaru contexts, ~**+1.0 mag** at 90% completeness/purity while preserving PSF/photometry; requires multiple co-aligned exposures.
- **Cosmic rays / satellites** [KB-covered]: Cosmic-CoNN adapts deepCR U-Net with **Group Normalisation** and class-imbalance loss; DECam tests cite **~97.96% TPR at 0.1% FPR**, attention gates aid the decoder. IAU CPS **SatHub** tools (satchecker crossings, satmetrics/reca-streaks; 2026 fast-aperture vs PSF-convolved box-profile comparisons) and **Median Radon Transform** streak extraction below background noise. SkyStacker has stacking rejection; learned CR masks/streak tools are future research.
- **Finite-sample MAD / dark QA** [new]: PixInsight 1.9.6 finite-sample consistency constants (N = 3–30) and ASTAP July-2026 single-pass sigma-clip (25% speedup) are adjacent; `--finite-sample-scale` and dark-series QA are survey proposals (§14 for the shipped dark QA). No 2024–2026 comparative benchmarks of MAD variants/Winsorized/linear-fit/GESDT on faint nebulosity were found.
- **Overall:** RTN/BFE synthetic calibration, denoisers and learned artifact masks are not in the SkyStacker core; if ever added they must be optional, flagged and provenance-tagged. Neural inference is infeasible in the single-file `CGO_ENABLED=0` binary; external sidecars are a separate topic ([06-external-ai-sidecars.md](06-external-ai-sidecars.md)).

---

## 14. Master-frame practice from curated sources

Curated corpus (cycle 5, 2026-11-17; goCrawler filtered English text + `output/ranking-URL.csv`) — practice that informs master-frame construction and stacking, distinct from the self-flat estimator:

- **PixInsight master-frame guide** (`pixinsight.com/tutorials/master-frames/`, rank 50.50): ImageCalibration (IC, since v1.6) works only with **master** calibration frames. Take **at least 10 dark and bias frames**; never use fewer bias than dark frames — a 50-frame master dark carries almost all the read noise of a 10-frame master bias. Build master bias and master dark with ImageIntegration using identical settings, run twice (bias subtraction of darks happens in IC). **Do not normalise** bias/dark frames — the bias pedestal must be preserved (disable both normalisation settings). In the Master Dark section enable **Optimize**, which rescales the master-bias-subtracted master dark to each flat's thermal noise: a 1000 s master dark vs a 10 s flat is rescaled by **0.01**. After bias/dark subtraction flats are pure illumination data, so **match illumination levels** with multiplicative normalisation (multiply each flat to the average pixel value of all flats) before integration; rejection choice depends on the number/type of flats. Dark scaling works best when read noise in the master frames is negligible; IC is designed for calibration libraries with varying temperatures/exposures and always rescales dark noise to each light.
- **PixInsight WBPP** (`miklosoft.se/learn/path/preprocessing-in-depth/`, rank 70.45): stages — files/grouping keywords/global options; calibration, cosmetic correction, debayering (master bias/darks/flats, dark optimization, output pedestal); measuring/weighting/frame selection (PSF Signal Weight, minimum weight, interactive selection); registration and local normalization (distortion correction); integration and pixel rejection (percentile, sigma, Winsorized, linear fit, ESD, RCR); drizzle, fast integration, autocrop, plate solving; pipeline goDiagnostics/logs/troubleshooting; plus Siril/DSS/APP equivalents.
- **NOVA stack paths** (`measurethepixels.com/handbook/stacking-integration-weighting-rejection.html`, rank 72.59): four meaningful paths — **Siril** (reference strategy: Siril registration + Winsorized-rejection mean, additive-scale normalization, background-noise weighting, output normalization), **ImageMM** (Siril-registered frames, multi-frame deconvolution, 20 iterations, kappa 2.0, PerChannel), **pixinsight_wbpp** (hybrid: Siril registration + PixInsight ImageIntegration with Winsorized rejection, additive scaling, requested SNR weighting), **pixinsight_register** (changes registration, grid and integration together). Requested behaviour can fall back, so comparisons must record the **executed treatment**, not just the engine name. Frame-level culling (whole exposure) and pixel-level rejection (outlier aligned sample) are different decisions. Ideal random-noise improvement ≈ √N for comparable exposures, but gradients, seeing, correlated noise, resampling and unequal frames prevent that from being guaranteed; a comparison that changes framing/normalization/etc. is not proof one pixel-combination algorithm is intrinsically better.
- **Drizzle** (`siril.readthedocs.io/.../drizzle.html`, rank 49.80; CFA detail from the GEMINI survey): variable-pixel linear reconstruction (Fruchter & Hook, HST); pixel fraction 0 = pure interlacing, 1 = shift-and-add; registration homography is 3×3/8-DOF; interpolation smears the PSF (Siril clamps to limit artefacts). Square flux-conserving kernels suit diffuse nebulosity; Point kernels lower correlated noise but risk null holes. Bayer CFA at scale=1.0 already has 2×-mono droplet sparsity (green 50%, R/B 25% density); 2× upscaling needs 4× coverage and terabyte-scale scratch. Practitioner guidance: random non-integer dithers, **15–30 subframes** (acceptable 10–15, artifacts <10). SkyStacker currently treats drizzle as a non-goal; see [01-architecture-and-core-engine.md](01-architecture-and-core-engine.md).
- **Shipped dark-series QA** (deep dive §13.5, 2026-11-17): master dark construction rejects >3-sigma outliers by median and hot-pixel fraction when ≥3 dark subframes exist, with a mean-absolute-deviation fallback for degenerate MAD (`testCalibrationQA`). This is the implemented low-effort pure-Go gate that the GEMINI survey proposed as `--dark-qa-reject`. Pre-solve gradient detrending (`detectStarsWithDetrend`, `testDetrendStarDetection`) and Gaia 429 handling are also shipped but belong to astrometry/toolchain files.

---

## 15. Consolidated open gaps

Sensor-characterisation and calibration reference sources (SharpCap Sensor
Analysis at 1-2% linearity, HFR autofocus with a 15% rejection threshold,
DeepSkyStacker 16-sigma hot-pixel rule and entropy-based dark scaling, and
stack-size-dependent rejection heuristics) are indexed in
[03-reference-and-literature.md](03-reference-and-literature.md) §6; the
details are not duplicated here.

1. **Gamma / EOTF (Phase 5) — experimental option validated 2026-11-18.** The self-flat is still applied in sRGB-encoded space by default; `--self-flat-linear-srgb` decodes with the sRGB EOTF before division and re-encodes afterwards, and is off by default. A two-run OrionNebula validation (16 lights, dark+bias, `--method mean`, 64 px cells, degree 4; grid diagnostics identical in both runs: center/edge **1.0018**, fit RMS **0.06787**, 760/2542 cells, mask **13.2 %**) gave global stack means **0.00942616** (encoded) vs **0.00943065** (EOTF), center/border **0.99198** vs **0.99173**, mean absolute pixel difference **6.2e-6** and max **0.0316** on bright cores. With this mildly vignetted field the EOTF correction is negligible at the background level and changes mainly bright pixels; no artifact was observed. Default remains encoded; the report now states which path was used. A full linear-light pipeline (normalisation, sigma clipping, outputs) is still a larger, separate change.
2. **Real-flat validation informal / incomplete (Phase 6).** Only separate-run, JPEG-approximate grid-vs-real-flat comparisons exist (§11.4). Smooth agreement within 5–10% and dust residuals are not yet quantified under a dedicated dataset/protocol; "validated" cannot be claimed.
3. **Single-process ablation executed 2026-11-18, gate still blocked.** With 16 `assets/OrionNebula` lights, `--method mean`, dark+bias only (grid, `/tmp/gamma_encoded`) versus dark+flat+bias (real flat, `/tmp/flat_validation_real`), both runs completed. Grid stack center/border **0.992**, real-flat stack center/border **1.522**; global means **0.00943** vs **0.30285** (flat division rescales the level, so absolute means are not comparable). The centre/border gap shows the bundled flats are not a matched calibration set for these lights (exposure/gain differ); this run cannot establish numerical parity and the acceptance gate remains blocked pending a matched dataset.
4. **Dust / defect handling out of scope (Phase 7).** Polynomial grid cannot represent high-frequency dust; 5×5 rejection removes rather than corrects it. Real flats or a dedicated residual model required; do not promise it in flag descriptions.
5. **`--local-normalize` interaction measured 2026-11-18 (no severe double correction).** Three controlled 16-light OrionNebula runs (`--method mean --scale 2 --dark --bias`) gave stacked center/border **1.00972** (grid self-flat only), **1.00972** (plus global `--normalize`, numerically identical) and **1.02599** (plus `--local-normalize`, a +1.6% shift). Global means were 0.009423 / 0.009423 / 0.009337. The local run reported self-flat center/edge **0.9824** and a normalization tile-median ratio **1.2667**, so the two paths do touch the same low-frequency field, but the effect on the final background is small rather than a runaway double division. A synthetic gradient-only selftest (`testSelfFlatNormalizeInteraction`) confirms the ordering: raw centre/border ≈1.15 → 0.9975 after flat → within 0.05 of 1 after local normalization. The report now emits an explicit overlap warning whenever both paths are active, with the measured tile-median ratio; keep the warning and compare against `--no-local-normalize` before publishing a master.
6. **Lowercase `--calib-root` bug fixed.** The earlier failure looked for `assets/OrionNebula/Darks` while the tree uses lowercase `darks/flats/biases/lights`; `detectCalibrationSubdirs`/`--calib-root` now use case-insensitive discovery/fallback. Note as a resolved gap, not open.
7. **SIP resampling no longer a gap.** The former "SIP not applied in stacking resampler" limitation is closed: `--resample-sip` loads stored order-2 SIP from `plate_solution.json`, undistorts reference pixels, applies the similarity/affine transform in undistorted space, and redistorts source pixels before bilinear sampling (`testResampleSIP`). Per-frame astrometric reprojection and drizzle remain open. Implementation and WCS detail lives in [01-architecture-and-core-engine.md](01-architecture-and-core-engine.md) and [02-astrometry-and-identification.md](02-astrometry-and-identification.md).
8. **Astrophysical bias / residual-RMS metric.** Residual RMS is a departure-from-unity metric, not radial goodness-of-fit; a radial metric would separate vignetting from astrophysical structure. Masking detected stars/high-surface-brightness regions before reduction is a natural next experiment (not implemented).
9. **Pedestal and transfer assumptions.** JPEG display pedestal compresses recovered ratios (raw `.f32raw` and report numbers avoid it); EXIF pedestal scalarity and OSIRIS-REx→consumer-CMOS transfer remain **[assumed]**, not measured.
10. **Finite-sample scale / small-N rejection.** 1.4826 MAD underestimates scale at N = 3–30 (over-rejection); finite-sample corrections are proposed (`--finite-sample-scale`), not wired in. Single-pass sigma-clip equivalence is implemented; FITS Rice and XISF Rev 2 items from the survey remain open (toolchain file).

---

## Sources absorbed

- `research/04-calibration-and-self-flat.md` (the former append-only v1 record, §1–§22; rewritten here)
- `research/SYNTHETIC_CALIBRATION_RESEARCH.md` (2026-11-14 deep-research pass + preserved seed document)
- `research/Synthetic generation of calibration frames for astrophotography.md` (seed research question, physical models, bibliography, final assessment)
- `research/05-research-deep-dive-2026-11-16.md` — Pillar A (§3.1–§3.6), §7 OrionNebula note, §8 roadmap hooks, §13.5 shipped items, §290 status correction (calibration portions only)
- `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — §A calibration/noise findings, feasibility matrix, Proposals 2/6/7, and the 2026-11-17 SIP + single-pass-sigma-clip append (calibration portions only)
- `research/GEMINI_DEEP_RESEARCH_PROMPT.md` — §1 ground-truth pipeline/status, §3 KB inventory, §4 domain questions (calibration context only)
- `research/08-curated-content-cycle-5-2026-11-17.md` — PixInsight master frames, WBPP, NOVA stack paths and drizzle excerpts (master-frame practice)

A cycle-6 curated corpus of calibration/WBPP/RAW sources is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §9.

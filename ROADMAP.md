# SkyStacker roadmap

**Updated:** 2026-11-18 · **Current state:** v0.6.1, 247 registered self-tests,
Go 1.27.4, goPureRaw v0.1.1.

Ordered by ascending estimated effort. Work the first item marked **Open**;
items marked **Blocked** or **Deferred** are skipped until unblocked. Effort
scale: **S** ≤ 1 h, **M** 1–4 h, **L** 4–8 h, **XL** > 8 h. Each item lists the
canonical research file that owns the evidence; acceptance criteria there are
authoritative. Numbers are stable IDs, not sort keys; rows are ordered by effort.

**Summary (2026-11-18):** 37 Done · 11 Open · 6 Blocked · 1 Executed/not accepted
· 7 Deferred. Items 2–8 and 11 were delivered in commit `8b99533`; item 18
(the portable diagnostic module) was delivered afterwards. Items 19–28 were
promoted from the feature comparison of mature stacking tools and delivered
except item 27, which remains blocked by item 13. Items 29–46 were
promoted from the canonical research knowledge base, item 47 was added by
user request, and items 48–49 came from the HiPS-cutout/SVG-map analysis in
[02](research/02-astrometry-and-identification.md) §13; items 50–57 were promoted
from the cycle-7 corpus in [08](research/08-crawler-source-discovery.md) §10,
and items 58–62 add a standalone multi-image processing mode analysed in
[01](research/01-architecture-and-core-engine.md). The status column below is
the authoritative view.

## Completed (2026-11-17)

1. Finite-sample MAD correction for small N (`finiteMADScale`).
2. Go 1.27.4 toolchain upgrade and benchmark re-baseline.
3. Single-pass sigma-clip accumulation with bit-exact equivalence tests.
4. External AI CLI sidecar runner (`--external-bge/-denoise/-deconv/-starless`).
5. Order-2 SIP in the stacking resampler (`--resample-sip`).
6. Research folder consolidated into flat thematic files plus the ingestion
   script `scripts/consolidate-research.py`.

## Issue tracker — 62 items (ordered by effort)

| # | Milestone | Effort | Status | Research owner | Acceptance / scope |
|---|---|---|---|---|---|
| 1 | **ASTAP interoperability hardening** | S (~45 min) | **Blocked** — no ASTAP adapter in code | [02](research/02-astrometry-and-identification.md) §ASTAP audit | Correct the `%errorlevel%` HFD scale in docs, add a decoding unit test, prefer textual `HFD_MEDIAN`/`STARS`, clamp HFD > 21.46 px, verify `-ra` units. No plate-solver code changes. |
| 2 | **Encoded-space self-flat warning** | S (~30 min) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §15 | When the grid self-flat is applied in sRGB-encoded space and no linear-light option is active, `report.txt`/`report.json` state the approximation; no numerical change. |
| 3 | **Benchmark methodology extension** | S–M (~1 h) | **Done** 2026-11-18 | [05](research/05-toolchain-and-modularity.md) §6, [01](research/01-architecture-and-core-engine.md) §14 | Extend the existing `BenchmarkSigmaClip*` harness to cover tile reduction and decode, report mean/CV, pin CPU governor where possible; document in `BENCHMARKS.md`. |
| 4 | **XISF Rev 2 conformance (`--xisf-rev2`)** | S–M (~1–2 h) | **Done** 2026-11-18 | [05](research/05-toolchain-and-modularity.md) §11, [03](research/03-reference-and-literature.md) §4 | Strict sample-mapping bounds and corrupt-block rejection; SHA-256/512 natively, SHA-3 only after stdlib verification; malformed headers fail gracefully. |
| 5 | **BigTIFF float32 export (`--bigtiff`)** | M (~2–3 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §10, [05](research/05-toolchain-and-modularity.md) §11 | 64-bit `LONG8` offsets/byte counts for stacks > 4 GiB; round-trip selftest reads the written file back. |
| 6 | **Validate gamma-aware self-flat** | M (~2 h) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §15 | `--self-flat-linear-srgb` exists but is unvalidated; compare encoded vs EOTF-corrected division on a real dataset, keep default unchanged, document the result. |
| 7 | **Catalog annotation / anomaly extension** | M (~3–4 h) | **Done** 2026-11-18 | [02](research/02-astrometry-and-identification.md) §4 | Complete the existing `--annotate` baseline: elliptical apertures, multi-catalog cross-match, `SNR > 5` unassociated-source flag; register `testWCSCatalogOverlay` and `testUnassociatedSourceFlagging` (round-trip ≤ 0.05 px). |
| 8 | **Spatially gridded PSF for Richardson–Lucy (`--deconvolve-grid`)** | M (~3–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §15.3 | Local-star PSF over a 3×3/4×4 grid with bilinear interpolation, opt-in, star-masked; synthetic wide-field test showing reduced corner degradation. |
| 9 | **Pure-Go FITS Rice (`.fits.fz`) + `fitsio/` module** | M–L (~4–5 h) | **Blocked** — needs pushed module tag + cfitsio validation | [05](research/05-toolchain-and-modularity.md) §11 | Golomb–Rice tile codec, bit-exact integer/quantized round-trip; extract into a tagged sibling `fitsio/` module following the goPureRaw pattern. |
| 10 | **CR2 bit-exact validation vs dcraw** | M–L (~3–4 h) | **Blocked** — needs raw-Bayer oracle dumps | [07](research/07-native-cr2-decoding.md) §6 | Compare raw Bayer planes after cropping margins and black/white normalisation; explain 16372/1181.84 vs 16011/1180.96 and the old NCC < 0.1. `cr2_slice` is refuted; do not repeat that search. |
| 11 | **IVOA ProvenanceDM emission** | M–L (~3–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §16 | Per-run/per-stage provenance graph (binary version, plate-solve tolerances, tile/rejection/stretch parameters, exact catalogue/cache version) in `report.json`/VOTable under `skystacker_results/`. |
| 12 | **Real-flat validation gate for grid self-flat** | M–L (~3–4 h) | **Executed, not accepted** — flats not matched to lights | [04](research/04-calibration-and-self-flat.md) §15 | Single-process ablation toggling only the illumination source; require a dataset with ≥ 15 lights and a real flat; publish the numeric comparison, no dataset modification. |
| 13 | **Per-frame astrometric reprojection** | L (~5–6 h) | **Deferred** — L scope | [02](research/02-astrometry-and-identification.md) §11 | Replace the shared-distortion assumption of `--resample-sip` with per-light WCS solutions (SIP order 2 including AP/BP linear terms); synthetic corner-residual gate < 0.2 px, < 15 % runtime overhead. |
| 14 | **CFA/Bayer drizzle at scale 1.0 (`--bayer-drizzle`)** | L (~6–8 h) | **Deferred** — L scope | [01](research/01-architecture-and-core-engine.md) §7 | Raw Bayer planes via `dcraw -D -4`, square flux-proportional kernel, droplet 0.7–1.0, per-channel coverage weights, skip null samples, warn for N < 15 / integer-locked dithers; flux within 0.1 %. |
| 15 | **CR3 native decoding** | L–XL (~8 h) | **Deferred** — L–XL scope | [07](research/07-native-cr2-decoding.md) §0 | Remove the external-decoder dependency for CR3 input; pure-Go HEVC/HEIF container + lossless payload path, gated behind a flag until oracle-validated. |
| 16 | **PCC/SPCC + PSF-fitting photometry** | L–XL (~8 h) | **Deferred** — L–XL scope | [01](research/01-architecture-and-core-engine.md) §13 | Linear-data colour calibration against Gaia, PSF-fitting ensemble photometry with MAD rejection; opt-in, provenance-tagged. |
| 17 | **Observatory automation sidecar** | L–XL (~8 h) | **Deferred** — design only | [01](research/01-architecture-and-core-engine.md) §16 | External daemon writing an Alpaca-property → FITS-keyword mapping before header sealing; metadata ingestion only, interactive control out of scope. |
| 18 | **Standalone diagnostic module (`goDiagnostics/`)** | S–L (~1–6 h, phased) | **Done** 2026-11-18 | [goDiagnostics/README.md](goDiagnostics/README.md) | Self-contained Go module with OS/runtime/CPU/memory/disk/process/update/tool collectors, `--diagnose` flag, text/JSON output, redaction, timeouts and copy-paste portability to other Go projects. |
| 19 | **Frame metrics + culling (`--frame-metrics`, `--include-list`)** | S–M (~1–2 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §quality/diagnostics | From PixInsight SubframeSelector, DeepSkyStacker and Siril: export a CSV with per-frame HFR/FWHM/eccentricity/SNR/background/weight; `--include-list` filters discovered frames before stacking; selftest verifies filtering and CSV schema. |
| 20 | **Sharpest-N% frame selection (`--keep-sharpest N`)** | S–M (~1–2 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §13.4, [03](research/03-reference-and-literature.md) §5 | From AutoStakkert! lucky imaging: rank frames by median star HFR, keep the sharpest N%, apply before accumulation, log kept/rejected frames; opt-in and compatible with `--method mean/sigma`. |
| 21 | **Calibration library with metadata matching (`--calib-library PATH`)** | M (~2–3 h) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §2 | From PixInsight/MaxIm DL/DeepSkyStacker calibration libraries: index master dark/flat/bias by EXIF/FITS temperature, exposure, gain and ISO; select the best match per light, report the choice, fall back to explicit `--dark/--flat/--bias`. |
| 22 | **Dual-stack comet+stars recombination (`--comet-combine`)** | M (~3–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §12.1 | From PixInsight/DSS comet workflows: build a stars stack and a comet stack from the existing comet modes, recombine with a mask/blend into `stacked_comet_combined.f32raw/.jpg`, keep both inputs, selftest with a synthetic moving source. |
| 23 | **Multi-run batch queue (`--batch FILE`)** | M (~3–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §12 CLI | From Siril scripts, PixInsight PJSR and MaxIm COM: JSON array of runs (input, output, flags), sequential execution with one report per run, stop on first error, no shell interpolation; selftest with two synthetic runs. |
| 24 | **PSF-based starless mask (`--starless-psf`)** | M (~3–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §8, [03](research/03-reference-and-literature.md) §5 | From PixInsight/Siril starless workflows: estimate a local PSF from bright-star HFR and eccentricity, build a star mask, replace masked stars with local background; compare against the annulus baseline; selftest on a synthetic star field. |
| 25 | **À trous wavelet multiscale (`--wavelets`)** | M–L (~4–5 h) | **Done** 2026-11-18 | [03](research/03-reference-and-literature.md) §wavelets, [01](research/01-architecture-and-core-engine.md) §8 | From PixInsight/Fitswork wavelets: stdlib à trous decomposition, per-scale noise sigma from MAD, threshold/reconstruct, write preview layers; selftest verifies noise suppression without flux bias on synthetic data. |
| 26 | **Wiener deconvolution (`--deconvolve-wiener`)** | M–L (~4–5 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §8/§15.3 | From Fitswork: FFT-based Wiener/regularized deconvolution with a noise power spectrum estimated from the frame, opt-in, star-masked, complementary to Richardson–Lucy; selftest on a synthetic blurred star. |
| 27 | **Per-frame WCS reprojection mosaic (`--mosaic`)** | L–XL (~8–12 h) | **Blocked** — depends on item 13 | [02](research/02-astrometry-and-identification.md) §11 | From PixInsight, Siril, APP, IRIS, MaxIm DL, ASTAP and Fitswork mosaics: assemble plate-solved panels on a common WCS canvas with bilinear reprojection and overlap weighting, output FITS+JPEG; blocked until per-frame WCS solutions exist. |
| 28 | **Ensemble photometry + light curves (`--photometry`)** | L (~6–8 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §12.2 | From AstroImageJ and MaxIm DL: aperture photometry on the plate-solved stack, Gaia ensemble zero point with MAD rejection, CSV/JSON light curve and residual table; selftest on a synthetic catalogued field. |
| 29 | **Dither-magnitude diagnostic and small-dither warning** | S (~1 h) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §6.8/§7 | Estimate median frame-to-frame translation and warn when dithering is too small for a valid self-flat; report-only, no hard gate without evidence. |
| 30 | **Rejection-method heuristic by stack size** | S (~1 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §5.3, [03](research/03-reference-and-literature.md) §6.6 | Dispatch Min/Max (3-4), Percentile (5-9), Averaged Sigma (9-15), Winsorized (15-25) and Linear Fit (>25) over the existing estimators; opt-in flag. |
| 31 | **Sidecar argv with quoted arguments** | S (~1 h) | **Done** 2026-11-18 | [06](research/06-external-ai-sidecars.md) §5 | Replace `strings.Fields` with a small shell-free tokenizer supporting quoted paths/arguments; keep `exec.CommandContext` and add a tokenizer selftest. |
| 32 | **`--post-preset` stage ordering** | S (~1 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §8/§17 | Allow detail enhancement after denoise instead of before it; preserve existing step-output meanings and default unchanged. |
| 33 | **GHS parameter-mapping validation** | S (~1 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §8 | Validate the GHS formula parameters against Siril/GHS documentation and add a round-trip test against the documented reference curve. |
| 52 | **`.xnml` / `PSFScaleSNR` ingestion** | S (~1 h) | **Open** | [04](research/04-calibration-and-self-flat.md) §10 | Read per-subframe normalisation scale from `<basename>.xnml` (fallback folder, zero when missing) and use it for PSF-based frame weights; selftest with synthetic XML. |
| 47 | **Rotating thank-you / USDT-USDC donation footer** | S (~1 h) | **Done** 2026-11-18 | `donations.go` (user request) | Dedicated ETH (`0xE5f68950D479fAB12797dABbe5a4B0d88eC7A722`) and BTC (`bc1qz4zvf42l3fu4axp0nd85fp2jr0gnavrr2v6xsj`) constants; 110 rotating English templates with `{address}`/`{btc_address}` placeholders; terminal QR for both via MIT `github.com/skip2/go-qrcode`; printed only after a fully successful stack+postprocess run, never in reports; standalone `--donation` shows message and QR without images; `--no-donation` / `--no-donation-qr` opt-outs, batch-child suppression; `testDonationFooter`. |
| 34 | **Self-flat × local-normalise double-correction validation** | S–M (~1 h) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §15 item 5 | Controlled synthetic gradient-free run to quantify double correction and add a warning or guard if both paths are active. |
| 35 | **WCS/SIP fixture cross-validated against Astropy** | S–M (~1–2 h) | **Done** 2026-11-18 | [02](research/02-astrometry-and-identification.md) §2.3 | Mirror the healpy fixture approach with an Astropy-generated WCS/SIP reference JSON and test forward/inverse round-trips within 0.05 px. |
| 50 | **SSO annotation with velocity vectors** | S–M (~1–2 h) | **Open** | [02](research/02-astrometry-and-identification.md) §10.2 | Support `a:/c:/p:/dp:/s:` object IDs, 3-symbol IAU observatory code, hard `DATE-OBS` requirement, geocentric fallback and velocity-vector arrows from cached ephemerides; selftest on synthetic site headers. |
| 51 | **Frame-selection expression engine** | S–M (~1–2 h) | **Open** | [01](research/01-architecture-and-core-engine.md) §10.2 | Add `--frame-expr` boolean/weight expressions over the frame-metrics CSV with per-variable Min/Max/Median/Sigma and an `SSWEIGHT`-like output keyword; selftest with a small expression set. |
| 53 | **CFA-aware pre-drizzle background extraction** | S–M (~1–2 h) | **Blocked** — needs item 14 | [01](research/01-architecture-and-core-engine.md) §10.2 | Treat Bayer CFA as four interleaved subchannels, fit order-1 polynomial per subchannel and recombine before Bayer Drizzle (X-Trans unsupported); blocked until raw Bayer planes are available. |
| 58 | **Standalone single-image post-processing CLI** | S–M (~1–2 h) | **Open** | [01](research/01-architecture-and-core-engine.md) "Standalone image-processing mode" | `--process-image PATH[,PATH...]` + `--image-op blur|sharpen|background|stretch|denoise|deconvolve|starless`: decode with `decodeImageFile`, reuse existing pure `frame` functions, write `<basename>_processed.f32raw` + JPEG and report provenance; never modify inputs. |
| 59 | **Multi-image arithmetic operations** | S–M (~1–2 h) | **Open** | [01](research/01-architecture-and-core-engine.md) "Standalone image-processing mode" | `--image-op sum|subtract|average|median|min|max` over equal-size inputs with valid-pixel masks and dimension/normalisation diagnostics; reuse `accumulator`/`reduceTilesStack`; selftest on synthetic frames. |
| 36 | **Gaia DR4 / Archive v3.15 TAP adaptation** | M (~2–4 h) | **Done** 2026-11-18 | [02](research/02-astrometry-and-identification.md) §11 | Add pagination, queue/backoff handling, renamed DataLink IDs, <=4 active requests and persistent HEALPix cache fallback during the 25 Nov-2 Dec 2026 blackout. |
| 37 | **Decoupled rejection sub-tiles and dedicated buffer pool** | M (~2–3 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §3.3 | Use 64x64/128x128 rejection sub-tiles independent of alignment tiles with a `sync.Pool`, bounding RAM for deep stacks on 15 GiB hosts. |
| 38 | **XISF / PGM / PPM input ingestion** | M (~2–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §2.1/§12.3, [03](research/03-reference-and-literature.md) §4 | Add uncompressed readers for these formats so discovery accepts them as lights/masters; no compression support required. |
| 39 | **Sidecar FITS transport: 16-bit and monochrome outputs** | S–M (~1–2 h) | **Done** 2026-11-18 | [06](research/06-external-ai-sidecars.md) §5 | Extend the strict sidecar reader to accept BITPIX=16 and NAXIS=2 and convert to the planar float32 RGB contract; keep corrupt-block rejection. |
| 40 | **Multiscale adaptive denoise** | M (~2–4 h) | **Done** 2026-11-18 | [03](research/03-reference-and-literature.md) §5.4, [01](research/01-architecture-and-core-engine.md) §17 | Use per-scale MAD from the wavelet bands (item 25) to apply spatially adaptive strength; selftest checks noise suppression without flux bias. |
| 41 | **Entropy-minimisation dark scaling** | M (~2–3 h) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §14, [03](research/03-reference-and-literature.md) §6.6 | 1-D scalar search over [0,1] minimising entropy after dark subtraction; report the factor per light and selftest on synthetic thermal drift. |
| 42 | **Single-image cosmic-ray rejection (L.A.Cosmic-style)** | M (~2–4 h) | **Done** 2026-11-18 | [03](research/03-reference-and-literature.md) §6.3 | Stdlib convolution and threshold on the linear master to mask cosmic rays in low-N sessions where temporal rejection is weak. |
| 43 | **9-zone tilt, roundness, off-axis HFD and SQM diagnostics** | M (~2–4 h) | **Done** 2026-11-18 | [01](research/01-architecture-and-core-engine.md) §9 | Compute tilt %, three-corner mode, off-axis HFD, roundness and SQM/extinction notes from existing per-star HFR positions; report-only. |
| 54 | **Matched-filter source detection** | M (~2–3 h) | **Open** | [02](research/02-astrometry-and-identification.md) §10.2 | Implement `T = αC⁻¹S` with per-pixel noise weights and positive/negative source output on linear frames; selftest shows improved faint-star recovery over the current smoothed-S/N threshold. |
| 55 | **Layered SVG annotation extension** | M (~2–3 h) | **Open** | [02](research/02-astrometry-and-identification.md) §10.2 | Extend item 49 with independent layers, ellipse axes/PA corrected for field rotation and mirroring, and label placement (8 fields, 16 candidate positions, 20 collision passes); SVG only. |
| 56 | **Asymmetric-clipping background with harmonic inpainting** | M (~3–4 h) | **Open** | [01](research/01-architecture-and-core-engine.md) §10.2 | Implement σ=1.4826×MAD, asymmetric low/high clipping, multiscale harmonic inpainting, Scale [1,10], polynomial degree [1,6] and RBF alternative; selftest on a synthetic gradient. |
| 60 | **Pairwise transform and quality measurement** | M (~2–3 h) | **Open** | [01](research/01-architecture-and-core-engine.md) "Standalone image-processing mode" | `--image-op measure-transform|measure-quality`: report dx/dy, rotation, scale and inlier RMS from `detectStars`+`matchStarsToReference`, plus per-image HFR/HFD, eccentricity, background sigma and frame-quality scores; CSV/JSON output, no image rewrite. |
| 61 | **Standalone Gaia SVG sky map for arbitrary images** | M (~2–3 h) | **Open** | [02](research/02-astrometry-and-identification.md) §13, item 49 | Extend `--sky-map` to accept an explicit image path, run `--plate-solve` when needed (seed via `--plate-center` or FITS RA/DEC) and emit `stacked_sky_map.svg` at the exact input dimensions; reuse the offline Gaia cache and item 55 layered rendering. |
| 62 | **Standalone HiPS survey cutout aligned to arbitrary images** | M (~3–4 h) | **Open** | [02](research/02-astrometry-and-identification.md) §13, item 48 | Extend `--survey-cutout` to accept an explicit image path, solve WCS if needed, call CDS hips2fits with the image width/height/FOV/rotation and write `survey_reference.fits/.jpg` plus a star-match diagnostic; opt-in network, never block offline runs. |
| 49 | **SVG sky map of the field** | M (~3-4 h) | **Done** 2026-11-18 | [02](research/02-astrometry-and-identification.md) §13.2 | Emit `stacked_sky_map.svg` at the exact width/height/orientation of the solved frame: project cached Gaia sources with `wcsTanForward`, draw magnitude-sized circles/labels, and optionally overlay SIMBAD/VizieR extended objects as ellipses only when major/minor axis and PA exist; label each object source; write beside `stacked_annotated.jpg` without touching scientific outputs. |
| 48 | **HiPS survey cutout with exact frame geometry** | M–L (~4-6 h) | **Done** 2026-11-18 | [02](research/02-astrometry-and-identification.md) §13.1 | Opt-in network stage using CDS hips2fits (`hips`, `width`, `height`, `projection=TAN`, `ra`, `dec`, `fov`, `rotation_angle`, `format=fits`, or a `wcs` JSON) to fetch a DSS2/Pan-STARRS cutout at exactly `refW x refH`, cache it, align with existing star matching and write `survey_reference.fits/.jpg`; require a plate solution, never block offline runs, respect the 50 Mpix service limit. |
| 44 | **Satellite-streak extraction (Median Radon Transform)** | M–L (~3–5 h) | **Done** 2026-11-18 | [04](research/04-calibration-and-self-flat.md) §13 | Deterministic line-integral detection of sub-noise streaks on the linear master; write a streak mask and report, complementing max-stack diagnostics. |
| 45 | **Higher-order SIP fit (order 3-5) with inverse AP/BP terms** | M (~2–4 h) | **Blocked** — needs item 13 | [01](research/01-architecture-and-core-engine.md) §6.3, [02](research/02-astrometry-and-identification.md) §2.2 | Extend the least-squares SIP fit beyond order 2 and include linear inverse coefficients; corner-residual gate <0.2 px. |
| 57 | **Spline-distortion model R&D spike** | L (~5–8 h) | **Deferred** | [02](research/02-astrometry-and-identification.md) §10.2 | Evaluate thin-plate/RBF surface splines versus order-2 SIP using PI ImageSolver parameters (<0.005 px convergence, 4000 control points, XISF-native); research spike only, no commitment. |
| 46 | **Full linear-light pipeline (normalisation, clipping, outputs)** | XL (~12-16 h) | **Deferred** | [04](research/04-calibration-and-self-flat.md) §7/§15 | Remove the sRGB-encoded approximation across calibration, normalisation, stacking and writers; item 6 is only the stopgap. |

## Deferred / explicitly rejected

Recorded so they are not reopened without new evidence:

* Embedded ONNX/GGUF/WASM inference and native BXT/NXT/SXT equivalents
  (284–456 MB weights, incompatible with the static stdlib-only binary) —
  [06](research/06-external-ai-sidecars.md) §2.
* `simd/archsimd` (experimental, amd64-only GOEXPERIMENT) and a GPU HAL for
  current workloads — [01](research/01-architecture-and-core-engine.md) §4.2.
* 2× mono/CFA upscale drizzle as a general mode, GZIP FITS, synthetic
  bias/dark/flat generation — [01](research/01-architecture-and-core-engine.md)
  §7, [05](research/05-toolchain-and-modularity.md) §11,
  [04](research/04-calibration-and-self-flat.md) §12.
* Fully general multi-night mosaics and real-time observatory control are out
  of scope; per-frame reprojection (item 13) is the enabling research step,
  not a commitment to mosaicking.

## Maintenance

After each milestone: rebuild the canonical binary, rerun `--selftest`,
`go vet`, `go test`, update the self-test count in `prompt-ai.txt`,
`PROJECT_CONTEXT.md` and `TECHNICAL_SPEC.md`, and record new evidence in the
owning canonical research file (never in a new partial file). Use
`scripts/consolidate-research.py` for incoming research contributions.

After every research knowledge-base update (or `COMPARISON.md` change), run
the mandatory gap analysis from `prompt-ai.txt`: scan canonical files `01-08`
for open/not-implemented challenges, compare with this tracker, add new items
in ascending-effort order with status and research owner, and refresh the
summary counts. Do not duplicate existing or rejected items.

New Go modules must follow the `go<Name>` convention for directory and module
path (`goPureRaw`, `goDiagnostics`, future `goFitsio` →
`github.com/computerphysicslab/skystacker/go<Name>`); see `prompt-ai.txt` and
`PROJECT_CONTEXT.md`. A published semver tag freezes the module path.

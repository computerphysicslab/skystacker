# Research deep dive (2026-11-16) — SkyStacker

**Status:** Synthesised from an internal deep-dive note (`research/last-research.md`, Spanish draft) plus prior `research/*.md` in this tree. Translate and organise into English; preserve all actionable technical items for SkyStacker.  
**Date:** 2026-11-16 (research snapshot).

## 1. Purpose and constraints

This document consolidates research directions relevant to SkyStacker in three pillars (detector/calibration, astrometry/catalogues/spatial indexing, architecture/automation/provenance). It is intended to be the single English reference for the 2026 deep-dive pass.

**Important constraints (from the source note):**

- The `research.zip` tree was inspected structurally (13 Markdown files under `research/`, including `research/01-architecture-and-core-engine/current-implementation-v0.4.1.md`), but internal text of those files may not have been legible in that inspection context. Do **not** assume implementation details of `v0.4.1` from filenames alone without reading the actual files.
- There is **no public changelog/repo tag evidence** in the provided materials for a **SkyStacker 0.6.0** release beyond what the current working tree states (e.g. `main.go`, docs showing `0.6.0`, 181 self-tests, grid self-flat default). Tables below that say “update plan to 0.6.0 baseline” are **normative engineering plans** pending reconciliation with the real codebase.

## 2. Structural audit of the `research/` tree (verified)

The tree contains (non-exhaustive listing from repo scan):

- `research/README.md`, `research/RESEARCH_SYNTHESIS.md`
- `research/SYNTHETIC_CALIBRATION_RESEARCH.md`, `research/Synthetic generation of calibration frames for astrophotography.md`, `research/04-calibration-and-self-flat.md`
- `research/01-architecture-and-core-engine/core-engine-technical-specification.md`, `research/01-architecture-and-core-engine/current-implementation-v0.4.1.md`, `research/01-architecture-and-core-engine/cli-integration-and-observatory-automation.md`
- `research/02-astrometry-and-identification/astrometric-catalog-annotation-and-anomalies.md`, `research/02-astrometry-and-identification/astronomical-identification-and-sky-charts.md`, `research/02-astrometry-and-identification/healpix-reference-verification.md`
- `research/03-reference-and-literature/comprehensive-technical-glossary.md`, `research/03-reference-and-literature/source-catalog-and-microarchitecture.md`
- `research/last-research.md` (Spanish draft, 2026-11-16 context) — **superseded in content by this file** for the deep-dive narrative; keep as archive or delete after review.

Use this file as the **English hub** for the deep-dive; update the other files when integrating concrete actions.


## 3. Pillar A — Calibration, detector physics, self-flat, sky and noise/artifacts

### 3.1 sCMOS and Random Telegraph Noise (RTN/RTS)

Classical CCD read-noise (RON) floors are often dominated by 1/f; in many CMOS architectures the floor is influenced by **random telegraph noise (RTN/RTS)** from charge traps near the source-follower oxide/silicon interface producing discrete multi-level jumps in pixel signal. A practical workflow (e.g. Özdoğru et al. context, May 2025 style): build histograms from dark sequences with **integer-centred bins**, fit mixtures of **1G/3G/5G** Gaussians via robust fitting (careful initial bounds from visual inspection), couple sigma between central peak and side lobes (shared RON), and derive lobe positions from mean offset parameters. Overfitting at higher mixtures requires human checks.

**CMOS hybrid detectors (e.g. H2RG X-ray context, March 2025):** CDS vs pseudo-CDS alters RTS signatures; RTN detection fractions differed between **140 K and 160 K** because smaller-amplitude peaks were masked by higher thermal read noise. **SPAD (2026):** optical light leaks can produce binary count-rate changes indistinguishable from intrinsic RTS unless shielding is controlled. A synthetic master-dark simulator for sCMOS should sample non-Gaussian tails via **per-pixel parameterized mixtures** and pair with mitigation suited to high frame-rate photometry.

### 3.2 Brighter-Fatter Effect (BFE), wavelength dependence

Electrostatics break the “isolated pixels” assumption at high signal in flat illumination (Antilogus et al. 2014, thin fully depleted CCDs). Accumulated charge perturbs drift fields, compresses effective boundaries of fuller pixels for later electrons: typical 3–4 px stars can grow in linear FWHM by up to **~2%** over the dynamic range; flat-field variance grows slower than mean due to inter-pixel correlations. First-order net charge transfer to neighbour X (schematic):

$$
\delta Q_{0,0}^X = \sum_{i,j} a_{i,j}^X Q_{i,j} (Q_{0,0} + Q_X)
$$

with parity, translational and zero-sum constraints on coefficients over the neighbourhood (as in the cited literature). **LSSTCam commissioning (DMTN-329):** BFE coefficients depend on **incident wavelength**; achromatic/linear corrections in current pipelines are insufficient for sub-percent photometry/shape work. For synthetic calibration and precision reduction: parameterize **filter-/wavelength-dependent BFE kernels** conceptually; separate additive vs multiplicative signatures; distinguish temporal noise from fixed PRNU/DSNU structure.

### 3.3 Self-flat, multiplicative large-scale calibration, sky subtraction

Avoid deriving large-field corrections from isolated local fits when possible.

- **Ubercal (space, example: ACS/WFC F606W context, Cohen Sep 2024 / ISR ACS 2024-06 style):** model multiplicative L-flat over pixel flats with **orthonormal 2D polynomials** of combined order Z (e.g. \(Z(Z+3)/2\) free parameters per detector, 27 for Z=6), plus temporal amplifier decay terms \(\beta\) (mag/year). Formulation in **magnitude space** preserves linearity for high SNR stars when restricted appropriately; equivalent multiplicative form in **flux space** breaks linearity more. Synthetic gap tests (e.g. 50 px chip gaps) recovered injected parameters to **≲0.1%** error in that study.
- **FGCM (ground, DES/HSC PDR3/Rubin context):** physical two-stage atmosphere + chromatic system response modelling instead of empirical star-flats with global linear ambiguities and missing chromatic terms.
- **HSC PDR3 sky:** reinject subtracted sky per CCD before a global sky fit to avoid oversubtracting large galaxy halos.
- **Gnuastro NoiseChisel (Segment in recent versions; `--widekernel` option discussed in literature):** mean–median asymmetry plus optional wide kernel to detect diffuse structures down to very faint surface brightness in favourable regimes (e.g. cited ~25.97 mag/arcsec² at S/N 0.25 in some contexts) without eroding sharp edges; validated in deep studies (Borlaff et al., Müller et al., Trujillo et al. references in source note).

### 3.4 Self-supervised denoising (2025–2026): photometric caution

Single-image blind-spot networks (Noise2Void, Noise2Self) assume uncorrelated zero-mean noise per pixel and avoid clean–noisy pairs; evaluation on PAUS-like simulations/real context (arXiv:2611.05447v1 style) reported **systematic flux suppression** in aperture photometry for faint sources with standard N2V and **background artifacts** with probabilistic PN2V under noise mismatch — requires flux-aware modifications and strict photometric diagnostics. AstroSURE notes constant sky dominates MSE if not subtracted first; cross-instrument transfer (HST→CFHT example) limited; detectability gains do not validate photometric/PSF-sensitive use alone.

**Promising with safeguards:**
- **TDR (Self2Self family context, e.g. arXiv:2502.16807v1):** single-image training, batch inference, restore any pixel whose change exceeds threshold **R** bounded by initial noise (example: HMI LOS noise reduced from ~8 G to ~2 G with R=0.3; validated on 10 s HST exposures without underestimating strong peaks). If R→∞ behaviour should be documented (can disable restoration).
- **ASTERIS (2026, arXiv:2602.17205 style):** self-supervised spatio-temporal multi-exposure transformer validated on JWST/Subaru-style contexts; cited gains include **~+1.0 mag** at 90% completeness/purity in examples, preserving PSF and photometric precision; requires multiple co-aligned exposures.

### 3.5 CRs, linear artifacts, satellite streaks

Deep learning for CR masks/spectra: **Cosmic-CoNN** adapts deepCR U-Net ideas with **Group Normalisation** and a loss suited to class imbalance; on DECam tests cited **~97.96% TPR at 0.1% FPR** (precision example at high TPR) with attention gates helping decoder; models for ground imaging / spectroscopy referenced. LEO megaconstellations: **IAU CPS SatHub** tools (satchecker crossings, satmetrics/reca-streaks workflows, 2026 comparisons of fast aperture vs PSF-convolved box profile fitting) and **Median Radon Transform** approaches can extract streaks below background noise without obvious false positives from bright stars/galaxies when tuned carefully.

### 3.6 SkyStacker-relevant implications (from this research)

- **Grid self-flat (current default in repo):** implemented pipeline is described in `research/04-calibration-and-self-flat.md` and code; the research emphasises **degeneracies** (static nebula vs flat, stable sky gradient vs flat, dust out of scope, **encoded/sRGB vs linear light** — current pipeline notes “Gamma-aware (EOTF) correction is not applied…” in grid notes). Future work: explicit gamma/EOTF-aware division path (flagged), real-flat validation when datasets exist, and documentation that large dithers help stars more than large static nebulae.
- **RTN/BFE synthetic calibration:** not implemented in SkyStacker core today; could become optional modules feeding synthetic master darks/flat models with careful provenance (`--synthetic-*` style flags already exist for bias/dark/flat stubs in parts of UI — see code/docs for current behaviour).
- **Denoising:** post-processing exists (curves/sharpen etc. in CLI); neural denoisers are **high risk** for photometry unless they are flow-controlled (e.g. TDR-like bounds) and evaluated; treat as research/optional experiments, not default stack science path without validation.
- **CR/satellites:** sigma/median rejection in stacking is present conceptually; dedicated learned CR masks and streak handling could be explored later (outside current default scope).


## 4. Pillar B — Astrometry, catalogues, spatial indexing, cross-matching

### 4.1 Gaia timeline and Gaia ESA Archive (v3.15 context, 2026)

As of the research snapshot:
- **Gaia DR4** official publication stated as **2 December 2026 ~12:00 CET** (subject to official ESA confirmation in your operational context); technical blackout noted from **25 November 2026 12:00 CET** in that note. DR4 expected ~**66 months** of nominal observations (plus months of inverted precession in some descriptions) and broad products (epoch astrometry, BP/RP/RVS spectra, non-single star solutions, exoplanets list in expectations) — volume estimates cited around **~400 TB** in commentary. Flight operations ended **15 January 2025**; **DR5** to follow full mission legacy later.
- **ESA Gaia Archive v3.15 (14 October 2026)** migration described in the source note: backend change (e.g. from Greenplum to EDB WarehousePG in that account), **DataLink** structures renamed (examples: **DATAMODEL_GAIA**, **DATAMODEL_STANDARD** replacing older naming in that note), and service constraints including **max 5 concurrent jobs per user**, **queue up to 50** (excess rejected), **1 minute timeout** for sync/async queries in that description, and **≤50,000 rows** returned per query in that description. Any legacy client doing open-ended TAP scans in dense fields will fail without spatial chunking, concurrency control, and local caching. A **pre-release** sample (dr4-prerelease) was mentioned as available from **29 June 2026** in that note for testing workflows.

**SkyStacker relevance:** if/when SkyStacker adds automated Gaia/TAP-based plate solving or catalogue annotation clients, they must implement pagination, backoff, local cache, and the renamed DataLink product names. The current codebase includes plate-solving/Gaia-related code paths (see `main.go` and docs such as `TECHNICAL_SPEC.md` mentioning plate solving wired, SIP not applied) — audit against v3.15 constraints before production use.

### 4.2 Reference catalogues (examples from deep dive)

- **SkyMapper DR4 (II/379/smssdr4 style):** ~26,000 deg², ~724M rows, bands u,v,g,r,i,z; astrometric calibration to Gaia; fringe removal in i via PCA in described workflows.
- **Legacy Surveys DR10/DR11:** >20,000 deg² context, large DECam-derived catalogues (The Tractor) for g,r,i,z; DR11 layers referenced in official viewers in that note.

### 4.3 DCR, WCS, large-field astrometry

Differential chromatic refraction (DCR) is a first-order astrometric limitation in wide-field imagers without atmospheric dispersion correctors in many cases:
- **Rubin/LSSTCam DP2 COSMOS-extended context (2026):** astrometric residuals projected toward zenith related to source SED vs reference stars in u,g,r even after standard calibration (enables constraints on effective wavelength in sub-bands in that work).
- **WFST global astrometric calibration (Yang 2026 context):** per-CCD independent calibration can reach tens of mas with Gaia DR3 but star sparsity limits simultaneous DCR + lateral colour modelling; DES experience cited (joint distortion/DCR/lateral colour, ~3–6 mas per exposure; GP regression for turbulence). Tools/methods referenced: **SCAMP** (stereographic intermediate reprojection, high-order polynomials, DCR corrections), **galsim.dcr** (Edlén/Filippenko style physics with T/P/water vapour), **LSST Science Pipelines / The Monster** catalogue concepts.

### 4.4 Plate solving, spatial indexing, cross-matching

- **Quad hashing:** fast blind solve for narrow/wide? contexts — **Cedar-Solve** (esa/tetra3 lineage improvements noted), **tetra3 Rust v0.14.0** for fast solves; **ASTAP** with large D50 database can solve very narrow fields quickly in some comparisons noted.
- **HEALPix / MOC / HiPS:** **cds-healpix-rust**, **MOCPy/MOCli** (IVOA MOC boolean ops on mixed-order cells), **HiPS/hips2fits**. At billion-object scale, **HATS** (Hierarchical Adaptive Tiling Scheme) + **LSDB (LINCC)** store adaptive HEALPix pixels as **Apache Parquet** files enabling distributed queries/unions without central TAP servers in the same way.
- **Bayesian cross-matching:** **NWAY** (Salvato et al. 2018, maintained notes to 2025 context in source) simultaneous N-catalog matching with positional uncertainties, non-association probability, colour/SED priors; reported agreements vs LR in XMM-COSMOS examples.


## 5. Pillar C — Architecture, observatory automation, CLI, provenance

### 5.1 Observatory automation and APIs

- **N.I.N.A. + ASCOM Alpaca:** discovery/connection via REST/JSON; scripted sequences (weather safety monitors, power, camera cooling, dome/roof, guiding PHD2, autofocus, rotator, flats panels, meridian flips, safe shutdown with state persistence per frame as described in community/plugin notes).
- **2026 architecture directions (arXiv-style note in source):** REST/WebSocket APIs for weather time series and operational commands (close observatory, cool cameras, complete FITS headers) synchronised over time.
- **SkyStacker implication:** decouple environmental telemetry (T, P, water vapour relevant for DCR models like `galsim.dcr`) via REST/WebSocket or Alpaca adapters before sealing FITS headers; design CLI/runtime hooks carefully (SkyStacker is currently a batch CLI tool; interactive observatory control is out of scope but **metadata ingestion** is not).

### 5.2 Provenance and traceability

- **IVOA ProvenanceDM** (validated April 2020 context over W3C PROV-DM): core entities **Entity, Activity, Agent** with extensions **ActivityDescription** (method/algorithm/software version) and **ActivityConfiguration** (execution parameters, e.g. TDR threshold R, Ubercal order Z, Gaia catalogue version) serialised as XML/JSON/SVG (PROV) or VOTable.
- **SkyStacker implication:** combine synthetic calibrations, BFE/L-flat models, denoiser thresholds, multiple reference catalogues → emit provenance records per run (or per major stage) aligned with ProvenanceDM concepts; tag outputs explicitly with tool version (currently `0.6.0` in tree) and key flags/caches.

### 5.3 “0.6.0 baseline update” plan (normative — do not assume done)

The source note proposed a plan (translated intent) to reconcile toward a **0.6.0** operational baseline (pending audit of real repo): prioritise Gaia Archive v3.15/DR4 continuity, add HATS/MOC-style local spatial strategies where appropriate, consider quad-based fast plate-solve fallback + NWAY multiband association as research directions, optional DCR/WCS metadata and corrections where feasible, optional RTN/BFE/synthetic profiling behind config + provenance, decouple sky vs artifact steps conceptually (NoiseChisel-style ideas vs learned masks), and add Alpaca/REST telemetry + ProvenanceDM emission as architecture goals. **Do not** mark any of these as implemented in SkyStacker without verifying `main.go`, docs, and tests.


## 6. Inventory: new content vs historical `research.zip` notes (classification)

Because internal texts of the 13 Markdown files under `research/` were not fully legible in the original inspection context, classify findings as **confirmed recent (2024–Nov 2026)** vs **candidates to verify against the actual `.md` files**. Below is the translated inventory intent.

### 6.1 Confirmed (chronologically verified 2024–Nov 2026) — examples

- Gaia ESA Archive **v3.15 (14 Oct 2026)** changes and **DR4** calendar (shutdown 25 Nov, release 2 Dec 2026, pre-release 29 Jun 2026 notes): DataLink renames to **DATAMODEL_GAIA / DATAMODEL_STANDARD**, limits (5 concurrent jobs, queue 50, 1 min timeout, 50k rows).  
- First **DCR** detection in Rubin/LSSTCam DP2 (COSMOS extended context, 2026) and WFST global framework (Yang 2026): SED-dependent shifts in u,g,r; per-CCD vs global multi-CCD calibration trade-offs.  
- **RTN in sCMOS** via 1G/3G/5G mixtures (2025), mitigation notes (PASP 2026 context), RTN in **H2RG** (2025), and SPAD artefacts from light leaks (2026).  
- **Wavelength-dependent BFE** in LSSTCam commissioning (DMTN-329).  
- **Ubercal ACS/WFC** L-flat orthonormal expansion + amplifier β decay (Cohen Sep 2024, ISR ACS 2024-06 style).  
- **Photometric evaluation of N2V/PN2V** on PAUS (2026), **AstroSURE** (2026), **ASTERIS** (2026), **TDR/Self2Self** (2025 context) with risk/benefit notes.  
- **IAU CPS SatHub** tools (satchecker, satmetrics, reca-streaks 2026, Median Radon Transform workflows) and **Cosmic-CoNN / attention-gate** evaluations.  
- **HATS/LSDB** (HEALPix-adaptive Parquet, 2025–2026 tutorials) as alternative to monolithic TAP/local caches at billion-row scale.  
- **SkyMapper DR4**, **Legacy Surveys DR10/DR11** scale notes.  
- **REST/WebSocket observatory APIs** (2026) and **ASCOM Alpaca / N.I.N.A. Planner** automation patterns (Oct 2026 context).  
- **Cedar-Solve**, **tetra3 Rust v0.14.0**, **ASTAP D50** vs SharpSolve-style trade-offs (quad hashing for fast initial solve).  
- **NoiseChisel/Segment**, **HSC PDR3 Global Sky**, **NWAY**, **cds-healpix-rust/MOCPy**, **IVOA ProvenanceDM/W3C PROV** (candidates to verify exact placement against `research/*.md` contents if files are now readable).

### 6.2 References (key URLs cited in the source note)

See the source deep-dive note for full URL list; key anchors include: arXiv entries for Özdoğru et al. 2025, Penn State H2RG 2025, Antilogus et al. 2014, DMTN-329, LSST DMTN-291, Cohen ISR ACS 2024-06, PAUS denoising 2026, AstroSURE 2026, ASTERIS 2026, TDR 2025, Cosmic-CoNN/deepCR papers, HATS arXiv 2501.02103, IVOA ProvenanceDM, Gaia Cosmos pages, SatHub repos, SCAMP/GalSim DCR docs, NWAY repo/paper, Cedar/tetra3/LSDB/CDS links as listed in `research/last-research.md` (archive). **Do not invent URLs** beyond those verified in the repo or cited in that archive when updating code/docs.


## 7. OrionNebula dataset note (local validation context, 2026-11-16)

- `assets/OrionNebula/`: includes real `biases/`, `darks/`, `flats/` and `lights/`. Initial checks found only 3 lights (< 15) → grid self-flat not built (needs ≥15). With explicit `--dark/--flat/--bias` on that small set, calibration applied (bias/dark/flat) and masters exported; grid was not used due to insufficient frames.
- After adding lights (now **16** CR2 files under `assets/OrionNebula/lights/`), a stack run with `--calib-root assets/OrionNebula` against the whole `assets/OrionNebula` directory failed early because the tool looked for `assets/OrionNebula/Darks` (capitalised plural) while the tree uses `darks/`, `flats/`, `biases/`, `lights/` (lowercase). Running against `assets/OrionNebula/lights` with `--calib-root assets/OrionNebula` succeeded in a local test: 16 frames, registration OK, grid self-flat diagnostics appeared in logs/reports in that run context (see `/tmp/onb_grid16b` artefacts if still present). This is a **path/case-sensitivity** issue in calibration auto-detection (`detectCalibrationSubdirs` / `--calib-root` handling) worth fixing in code for Linux repos using lowercase folder names.
- No dataset in `assets/` currently provides **≥15 lights + a real flat** for a clean “real flat vs grid self-flat” comparison except workflows that require careful layout; validation deferred. Document any future comparison in `research/04-calibration-and-self-flat.md` and `PROJECT_CONTEXT.md`.


## 8. Roadmap hooks (to sync with `ROADMAP_SYNTHETIC_CALIBRATION.md` and others)

Suggested additions (keep as research-driven backlog, not all for next session):

- **Calibration research:** optional RTN 1G/3G/5G profiling from dark stacks (flagged, provenance); wavelength-aware BFE experiments (research only unless validated); document explicit non-goals (dust in self-flat stays out of scope for polynomial grid).
- **Gamma/EOTF:** tracked in code notes; a proper linear-light option is a larger pipeline change (Phase 5 in older notes) — needs design (decode path, normalisation, sigma, outputs).
- **Astrometry:** Gaia Archive v3.15 client updates if TAP/DataLink is used; prepare for DR4 cutover windows; evaluate quad fast-solve fallback (Cedar/tetra3) integration points; HATS/LSDB exploration for offline large catalogues (separate from in-repo solver unless scoped).
- **Automation/provenance:** Alpaca/REST metadata ingest design doc; ProvenanceDM JSON/VOTable emission plan tied to run reports.
- **Repo hygiene:** fix `--calib-root` / `detectCalibrationSubdirs` to accept common lowercase names (`lights`, `darks`, `flats`, `biases`) on Linux (observed with `assets/OrionNebula`).


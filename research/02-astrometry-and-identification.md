# 02 — Astrometry & Identification (canonical research)

This is the single, deduplicated English reference for SkyStacker's astrometric identification and astronomical scene-understanding research. It consolidates the three formerly separate documents in `research/02-astrometry-and-identification/` plus the astrometry-relevant findings of the 2026-11-16 deep dive, the Gemini external survey, and the cycle-5 curated crawler text. Each unique fact, number, date, DOI, URL, algorithm, flag, test, status and gap appears once.

**Cross-links (canonical files, do not duplicate their content here):**

- [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) — stacking engine, memory model, provenance, observatory automation.
- [`03-reference-and-literature.md`](03-reference-and-literature.md) — master bibliography and comprehensive technical glossary.
- [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) — calibration, self-flat, denoising, CR/satellite rejection, Bayer drizzle.
- [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md) — Go version, Swiss Tables, GC, benchmark methodology, modularisation.
- [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) — RC-Astro/GraXpert/StarNet2 CLI sidecars.
- [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) — CR2/CHDK bit-exact decoding.
- [`08-crawler-source-discovery.md`](08-crawler-source-discovery.md) — crawler runs, URL/keyword corpora and cycle results.
- [`README.md`](README.md) — project entry point.

Toolchain, calibration, sidecar, CR2 and crawler-run details are deliberately not repeated here; follow the links above.

---

## Scope

Extend SkyStacker beyond conventional stacking/plate solving into an **independent astrometric identification and astronomical scene-understanding subsystem**: determine precisely where an image points, identify which celestial objects are present, render their identities and coordinates over the original image, and flag sources that do not correspond to known catalogue entries. The pipeline consumes a local-first HDR `float32` stack and produces WCS metadata, non-destructive annotation layers and structured anomaly reports.

### Six core research pillars

1. **Astrometric plate solving** — determine exact celestial coordinates, orientation, scale and projection of an image.
2. **Image-to-sky-chart registration** — transform astronomical catalogues and sky-chart data into the pixel coordinate system of the photograph.
3. **Astronomical object detection and classification** — identify stars, galaxies, nebulae, quasars, clusters and other deep-sky objects.
4. **Transient and moving-object detection** — identify novae, supernovae, variable stars, asteroids and comets.
5. **Anomaly and unknown-object detection** — isolate sources absent from known catalogues, image artefacts and potentially new phenomena.
6. **Scientific validation and cross-matching** — associate detections with authoritative databases and quantify identification reliability.

> **Key scientific distinction:** *detecting* an object, *classifying its morphology*, and *identifying it as a particular known object* are three distinct operations requiring different algorithms and evidence. Likewise, SkyStacker's existing triangle/RANSAC registration aligns exposures **relative to one another**, whereas absolute astrometric calibration establishes celestial coordinates against an **external reference frame**.

---

## 1. Plate solving

Highest priority. Plate solving determines the transform between pixels $(x,y)$ and celestial coordinates $(\alpha,\delta)$, yielding: RA/Dec of the image centre, orientation relative to celestial north, angular resolution (arcsec/pixel) and field of view (FOV), and per-pixel coordinate transforms stored as FITS WCS headers.

The four foundational papers are cited once in [Academic references](#12-academic-references-consolidated): Lang et al. 2010 (Astrometry.net — geometric hashing of star asterisms, indexed catalogues, Bayesian hypothesis testing without pointing metadata), Bertin 2006 (SCAMP — multi-exposure astrometric and photometric calibration), Calabretta & Greisen 2002 (FITS coordinate representations) and Bertin & Arnouts 1996 (SExtractor). The classification/morphology/transient/visualisation papers are likewise listed once there.

### 1.1 Core plate-solving algorithms

| Algorithm | Purpose | Relevance |
| :--- | :--- | :--- |
| Triangle matching | Match geometric configurations of 3 stars | High |
| Quad matching | Identify four-star patterns (tetragons) | Very high |
| Geometric hashing | Search precomputed star-pattern indices | Very high |
| RANSAC | Reject false correspondences | Very high |
| Least-squares fitting | Refine coordinate transformations | Very high |
| SIP distortion modelling | Correct optical field distortion | High |
| Bayesian verification | Evaluate whether a solution is genuine | High |
| Polynomial WCS fitting | Model higher-order image distortions | High |

**Architectural recommendation:** do not build a blind solver from scratch initially. Study Astrometry.net and ASTAP, implement CLI adapters for them first, then build a native solver that leverages SkyStacker's existing star-detection and registration pipeline.

### 1.2 Reference implementations and formats

| Implementation | Role / notes | Links |
| :--- | :--- | :--- |
| **Astrometry.net** | Blind plate solving and WCS generation; `solve-field` engine | [Website](https://astrometry.net/) · [Source](https://github.com/dstndstn/astrometry.net) |
| **ASTAP** | Fast local plate solving; native solver usable by CCDCiel, N.I.N.A., APT, Voyager, SGP | [Website](https://www.hnsky.org/astap.htm) · [Source](https://github.com/astap/astap) · [Version history](https://www.hnsky.org/history_astap.htm) |
| **SCAMP** | Multi-exposure astrometric calibration | [Website](https://www.astromatic.net/software/scamp/) |
| **Siril** | Reference for solving, retaining WCS headers, catalogue queries and annotation drawing | [Website](https://siril.org/) · [Source](https://github.com/AstroSharp/siril) · [Astrometry docs](https://siril.readthedocs.io/en/stable/astrometry/) · [Plate solving docs](https://siril.readthedocs.io/en/latest/astrometry/platesolving.html) |
| **Cedar Solve / Cedar Detect** | Permissive evolution of Tetra3 (blind solve) plus centroiding; practitioner report in AstroHopper thread | [AstroHopper report](https://www.cloudynights.com/forums/topic/993559-astrohopper-experience/) |
| **SkySolver** | Drop-in Astrometry.net web API | [API docs](https://skysolver.app/api-docs) |
| **tetra3rs** | Python/Rust tetra3 tutorial: Gaia DR3 bundled via `gaia-catalog` (no manual download), geometric hash patterns for star quads; TESS FFI (Camera 1, CCD 1, Sector 1) science data in HDU 1 trimmed by a 44-column left overscan to 2048×2048; `extract_centroids` finds connected regions above a local background then computes intensity-weighted centroid + covariance in *centered* pixel coordinates (+X right, +Y down); validation against FITS CRPIX converted to centered coordinates; result carries camera attitude, matched star count, residual statistics and attitude covariance (`attitude_sigma_rad`, 1σ of roll and two boresight offsets) | [Tutorial](https://tetra3rs.dev/tutorials/basic-solve/) |

No SCAMP successor was identified in the 2024–2026 material. Quad hashing for fast initial solves is studied under Cedar-Solve (esa/tetra3 lineage), **tetra3 Rust v0.14.0** and ASTAP's large D50 database; a maintainer-reported 5% ASTAP solver speedup appeared in Feb 2025 with star-poor-image enhancements through June 2026, while practitioner reports (Cloudy Nights, May 2025) note closed-source PlaneWave PlateSolve3 remains more reliable at 1600–2745 mm, star-sparse or narrowband fields — underscoring sensitivity to accurate pixel-scale hints.

### 1.3 Astrometry.net index construction (build-index / code / axy)

From the authoritative Astrometry.net docs ([build-index](https://astrometry.net/doc/build-index.html), [code](https://astrometry.net/doc/code.html)):

- Index files contain a set of bright, uniformly distributed "stars" (possibly galaxies) plus many features/quads describing the local shape of sets of (usually four) stars; each feature points back to its constituent stars. The engine detects stars in the image, examines sets of four, computes local shape, searches for similar index features, then verifies that other reference stars align with other image stars.
- Distributed indexes are based on **2MASS** and **Tycho-2**; custom indexes can be built from other reference catalogues.
- Many astrometric catalogues are FITS BINTABLE. Converters: `text2fits.py` (simple ASCII/CSV parser; huge memory on big files, not streamed), `2masstofits`, `nomadtofits`, `ucac3tofits`, `usnobtofits`; VizieR can often serve FITS binary tables; or write a custom converter (rewriting the `Xtofits` tools in Python/`struct` suggested). cfitsio tools: `liststruc`, `listhead`, `fitscopy`.
- A field to solve is encapsulated in an **"axy" file** — a FITS binary table containing X,Y star positions plus FITS header cards describing command-line arguments and image metadata. "axy" = *augmented x,y list* (abbrev. "xylist"); created by `augment-xylist.c`.

### 1.4 Siril plate-solving interface (reference)

From [Siril plate solving docs](https://siril.readthedocs.io/en/latest/astrometry/platesolving.html): many Siril tools (SPCC/PCC) need accurate coordinates. If an image is already solved nothing happens unless `-force` is passed. Approximate centre can be given in decimal degrees or J2000 colon-separated values as RA,Dec (not mandatory for Astrometry.net); SIP distortion can be solved up to **order 5** with `-order=` (values 1–5, default from preferences); `-radius=` sets a cone-search radius for local/Astrometry.net solving (Siril near search can be disabled with `0`, cannot be disabled for Astrometry.net); `-limitmag=` takes a `+`/`-` offset or absolute positive magnitude (limit auto-computed from FOV); `-catalog=` forces `tycho2`, `nomad`, `localgaia`, `gaia`, `ppmxl`, `brightstars`, `apass` (otherwise automatic: local catalogs if installed, else FOV/limitmag choice). Since v1.2 two algorithms exist: Siril's global registration star matching (register onto a virtual catalogue image of the same FOV) and local `solve-field` from Astrometry.net (on Windows via `ansvr`). Since v1.3 the internal solver adds further capabilities.

### 1.5 ASTAP catalogue databases and CLI interoperability

**Databases** ([ASTAP homepage](https://www.hnsky.org/astap.htm)): a 5-million-galaxy update exists; the 2.1-million-object version is bundled with the star database; variable stars dated v2025.12.22; macOS Apple Silicon build v2026.11.12 (code signing required). For FOV ≥ 0.6° choose D05, D20, D50 or D80 (D05 smallest, D80 largest ≈1.25 GB; D80 has no drawback other than size). New databases use a **star-density limit instead of a magnitude limit** — sorted at 500, 2000, 5000 or 8000 stars/deg², going as deep as magnitude 21 in sparse regions while keeping bright-star counts moderate in rich fields. **V50** has D50's 5000 stars/deg² density with calculated Johnson-V magnitude plus Gaia Bp−Rp colour (also good for FOV > 10°); **V05** is like G05 with Johnson-V and Bp−Rp. G05 is the automatic-cascade bridge (W08 very wide, D80/V50/D50/D20/D05 branches) — use surface density per deg², not a single uniform mag limit. The barebone CLI solver is compatible with the GUI when renamed; it rejects raw files and will not work with SharpCap because FOV is not stored. ASTAP imports FITS, RAW DSLR, XISF, PGM, PPM, TIF, PNG and JPG.

**`-analyse` / `%errorlevel%` encoding (Windows):** for `HFD 3.26` with `666` stars the packed value is **326000666**. A wrong draft formula `round(median HFD × 1,000,000) + stars` gives 3,260,666 and decoding `trunc(%errorlevel%/1e8)/100` yields 0.03 (wrong ×100). Correct encoding:

- `errorlevel_real = round(median_HFD × 100) × 1_000_000 + stars_detected`
- `median_HFD = trunc(errorlevel_real / 1_000_000) / 100`

With 6 digits reserved for star count the **32-bit signed ceiling is 21.46 px** median HFD before INT32_MAX overflow; heavily defocused runs should parse stdout (`HFD_MEDIAN=... STARS=...` on Linux/macOS) instead. `-focus1`..`-focus8` take 4–8 samples; hyperbolic focus fitting is done in the GUI/`-debug` path, not a standalone `astap_cli` call. Observed exit codes include 1, 2, 16, 32, 33, 34 (context-dependent); Apple Silicon needs `codesign`. ASTAP Android integrations (OpenLiveStacker, Stellar Finder) and StarNet2 integration (noted ~2026-09) support comet star separation. Verify `-ra` units against the installed version (hours-decimal vs degrees risk); the ARM64 "375%" figure is developer-reported. Adapter actions: fix any wrong `%errorlevel%` scale docs, add a decoding unit test, prefer textual `HFD_MEDIAN`/`STARS`, clamp HFD > 21.46 px, do not call `astap_cli` for hyperbolic autofocus.

---

## 2. WCS, TAN/SIP mathematics and FITS keywords

### 2.1 Gnomonic (TAN) forward projection

Given target $(\alpha,\delta)$ and reference centre $(\alpha_0,\delta_0)$, intermediate world coordinates $(\xi,\eta)$ in degrees under a gnomonic (TAN) projection:

$$
\cos \phi = \sin \delta \sin \delta_0 + \cos \delta \cos \delta_0 \cos(\alpha - \alpha_0)
$$

$$
\xi = \frac{\cos \delta \sin(\alpha - \alpha_0)}{\cos \phi}, \qquad
\eta = \frac{\sin \delta \cos \delta_0 - \cos \delta \sin \delta_0 \cos(\alpha - \alpha_0)}{\cos \phi}
$$

### 2.2 Two SIP conventions

**(a) Forward-distortion formulation used by the annotation specification.** Optical distortion is added to the intermediate coordinates with Simple Imaging Polynomial coefficients $A_{p,q}, B_{p,q}$ of order $N$ (typically $N=2$ or $3$):

$$
\xi' = \sum_{p=0}^{N}\sum_{q=0}^{N-1-p} A_{p,q}\,\xi^p\eta^q, \qquad
\eta' = \sum_{p=0}^{N}\sum_{q=0}^{N-1-p} B_{p,q}\,\xi^p\eta^q
$$

The distorted coordinates map to pixels through the linear CD matrix, with $(x_R,y_R)$ the reference pixel for $(\alpha_0,\delta_0)$:

$$
\begin{pmatrix} x-x_R \\ y-y_R \end{pmatrix}
= \begin{pmatrix} CD_{11} & CD_{12} \\ CD_{21} & CD_{22} \end{pmatrix}^{-1}
\begin{pmatrix} \xi' \\ \eta' \end{pmatrix}
$$

**(b) FITS WCS SIP convention (Shupe et al. 2005, SIP v1.0).** With `CTYPE1/2 = RA---TAN-SIP / DEC--TAN-SIP`, relative pixel $(u,v)=(x-\mathrm{CRPIX1},\,y-\mathrm{CRPIX2})$ is forward-distorted to $(U,V)$ before the CD matrix, with $A\_ORDER/B\_ORDER$ from 2 to 9 and the linear terms carried in CD:

$$
U = u + \sum_{p+q\ge 2}^{A\_ORDER} A_{p,q}\,u^p v^q, \qquad
V = v + \sum_{p+q\ge 2}^{B\_ORDER} B_{p,q}\,u^p v^q
$$

Inverse coefficients **AP/BP** must include the linear terms $(p+q=1)$ for accurate inversion on large sensors; omitting them degrades corners. Bidirectional mapping is $(x,y)\leftrightarrow(\alpha,\delta)$ with $(x_{\text{object}},y_{\text{object}})=\mathrm{WCS}^{-1}(\alpha,\delta)$.

### 2.3 FITS WCS keywords

- `CRPIX1`, `CRPIX2` — reference pixel coordinates.
- `CRVAL1`, `CRVAL2` — reference celestial coordinates (RA/Dec).
- `CTYPE1`, `CTYPE2` — coordinate system/projection (e.g. `RA---TAN`, `DEC--TAN`, or `RA---TAN-SIP`).
- `CDELT1`, `CDELT2` — pixel scale in simple representations.
- `CD1_1`, `CD1_2`, `CD2_1`, `CD2_2` — linear transformation matrix.
- `CROTA2` — legacy rotation representation.
- `A_ORDER`, `B_ORDER` plus `A_p_q`, `B_p_q` — SIP polynomial distortion coefficients.

References: Calabretta & Greisen 2002 (§12); WCSLIB C implementation ([docs](https://www.atnf.csiro.au/computing/software/wcs/)); Astropy WCS as reference test oracle for Go routines ([docs](https://docs.astropy.org/en/stable/wcs/)); Siril Annotations for practical rendering ([docs](https://siril.readthedocs.io/en/stable/astrometry/annotations.html)); FITS WCS page ([link](https://fits.gsfc.nasa.gov/fits_wcs.html)).

---

## 3. Catalogues and multi-provider routing

### 3.1 Core databases

- **Gaia DR3 (essential):** principal reference for stellar astrometry — positions, parallaxes, proper motions, magnitudes, colours, astrophysical parameters, variables, extragalactic candidates. [Archive](https://gea.esac.esa.int/archive/) · [DR3 docs](https://gea.esac.esa.int/archive/documentation/GDR3/) · overview paper Gaia Collaboration 2023 (§12).
- **SIMBAD (essential):** name/alias resolution, object types, bibliography beyond the Solar System. [Website](https://simbad.cds.unistra.fr/) · [Guide](https://simbad.cds.unistra.fr/guide/) · Wenger et al. 2000, *A&AS*, 143, 9.
- **VizieR (essential):** thousands of specialised published catalogues. [Website](https://vizier.cds.unistra.fr/) · Ochsenbein et al. 2000, *A&AS*, 143, 23–32 (both VizieR and paper DOIs in §12).
- **NED:** galaxies, clusters, AGN, quasars, redshifts. [Website](https://ned.ipac.caltech.edu/) · [References](https://ned.ipac.caltech.edu/Documents/References/NEDpapers).
- **SDSS:** spectroscopic/photometric validation. [Website](https://www.sdss.org/) · [SkyServer](https://skyserver.sdss.org/).
- **AAVSO VSX:** variables, suspects, nova candidates. [Website](https://www.aavso.org/vsx/) · [VizieR record](https://cdsarc.cds.unistra.fr/viz-bin/cat/B/vsx).

Large-scale reference examples from the deep dive: **SkyMapper DR4** (`II/379/smssdr4`, ~26,000 deg², ~724M rows, bands u,v,g,r,i,z, Gaia-calibrated, i-band fringe removal via PCA) and **Legacy Surveys DR10/DR11** (>20,000 deg², DECam catalogues via The Tractor in g,r,i,z).

### 3.2 Multi-provider catalogue routing matrix

The subsystem uses pluggable providers rather than a hard-coded single database.

| Object class | Primary sources | Additional / secondary |
| :--- | :--- | :--- |
| Stars | Gaia DR3 | Tycho-2, UCAC, APASS |
| Double stars | WDS | Gaia DR3 |
| Variable stars | AAVSO VSX, Gaia DR3 | GCVS |
| Galaxies | NED, HyperLEDA | SDSS, VizieR |
| Quasars | Gaia DR3, SDSS | Milliquas |
| AGN | SIMBAD, NED | SDSS |
| Nebulae | SIMBAD, VizieR | Sharpless, Lynds |
| Planetary nebulae | HASH, SIMBAD | VizieR |
| Globular clusters | Harris catalogue, SIMBAD | VizieR |
| Open clusters | Gaia DR3, SIMBAD | MWSC |
| Supernovae | Transient Name Server (TNS) | SIMBAD |
| Novae | AAVSO VSX, SIMBAD | TNS (relevant transient reports) |
| Comets | Minor Planet Center (MPC) | JPL Horizons |
| Asteroids | Minor Planet Center (MPC) | JPL Horizons |
| Satellites | CelesTrak | Space-Track |
| Unknown objects | Cross-match all relevant sources | Historical image comparison |

---

## 4. Annotation and anomaly specification

### 4.1 Annotation pipeline

Query local/remote layers bounded by the image bounding polygon evaluated with HEALPix nested geometry:

$$
\Omega_{\text{image}} = \{(\alpha,\delta)\mid x_{\min}\le x(\alpha,\delta)\le x_{\max},\; y_{\min}\le y(\alpha,\delta)\le y_{\max}\}
$$

Point sources come from Gaia DR3; extended objects from SIMBAD/HyperLEDA/PGC/NGC.

**Elliptical aperture rendering.** Extended objects are drawn as projected bounding ellipses with major axis $a$, minor axis $b$, position angle $\theta$:

$$
\frac{x'^2}{a^2} + \frac{y'^2}{b^2} \le 1
$$

where $(x',y')$ are rotated by $-\theta$ relative to the pixel centroid. Labels include designations (e.g. `NGC 5078`, `PGC 04661`), morphological types (`Sa-b`, `S0`) and apparent magnitudes ($V_{\text{mag}}$).

**Multi-layer annotation engine.** Layers are decoupled from the pixels so users can toggle, adjust opacity, inspect metadata and export non-destructively:

| Layer | Visual representation |
| :--- | :--- |
| Stars | Small circles, coloured by spectral/photometric information |
| Double stars | Paired markers and separation lines |
| Galaxies | Ellipses showing position angle and angular dimensions |
| Nebulae | Outlines or translucent polygons |
| Globular clusters | Circles indicating apparent extent |
| Open clusters | Star-membership markers or cluster boundaries |
| Quasars | Special symbols and catalogue identifiers |
| Supernovae | Distinct transient markers |
| Novae | Transient markers with discovery dates |
| Comets | Position markers, direction, optional trajectory |
| Asteroids | Position markers and motion vectors |
| Unknown sources | Highlighted detections with no catalogue match |
| Celestial grid | RA/Dec coordinate lines |
| Constellations | Boundaries, lines and names |

### 4.2 Unassociated-source (anomaly) detection

1. **Blind extraction:** background-subtract and connected-component / MAD-threshold the `float32` stack → $S_{\text{detected}}=\{\langle x_i,y_i,F_i,\mathrm{FWHM}_i\rangle\}$.
2. **Spatial cross-match via k-d tree:** map detections to $(\alpha_i,\delta_i)$ with the inverse WCS; query a spatial k-d tree for the nearest projected catalogue source $C_{\text{ref}}$ within a configurable radius $r_{\text{match}}$ (e.g. **2.5 arcsec**):
   $$
   \min_{(\alpha_c,\delta_c)\in C_{\text{ref}}} \Delta\theta\big((\alpha_i,\delta_i),(\alpha_c,\delta_c)\big)\le r_{\text{match}}
   $$
3. **Flagging:** any source with **SNR > 5.0** and no counterpart within $r_{\text{match}}$ is an **unassociated source / astrometric anomaly**.

Classification buckets: **transients** (supernovae, novae, dwarf-nova outbursts, variables near peak), **Solar System objects** (asteroids/comets with linear or curved trails across exposures), **high-proper-motion stars** (epoch mismatch), **residual artefacts** (unclipped satellite trails, hot pixels, cosmic-ray hits, optical ghosts). Anomalies are highlighted with high-contrast warning reticles (e.g. dashed yellow/magenta boxes) and serialised to **`anomalies.json`** with centroid coordinates, estimated fluxes and angular-separation residuals.

**Uncatalogued vs anomalous:** *uncatalogued* = no counterpart in the queried catalogues (e.g. faint background galaxy); *anomalous* = measured photometric/morphological/temporal features deviate significantly from expected populations (cf. Gaia DR3 Apsis — Creevey et al. 2023, §12).

**Six-stage anomaly pipeline:** (1) calibrated `float32` input with noise model and bad-pixel/saturation masks; (2) source detection (points, extended, trails, irregular); (3) multi-catalogue cross-match (Gaia DR3, SIMBAD, VizieR, NED, VSX, MPC — matched → annotate, unmatched → continue); (4) characterisation via morphology, photometry, PSF consistency, local noise, inter-frame motion/light curves (Isolation Forest, LOF, DBSCAN/HDBSCAN, autoencoders, ZOGY statistics); (5) candidate classification (possible transient, moving object, uncatalogued extended source, instrumental artefact, unknown); (6) scientific report with pixel/sky coordinates, flux, uncertainty, evidence, nearest matches and confidence score.

### 4.3 Proposed modules and tests (specification only)

- `wcsCatalogOverlay` — WCS projection, SIP inverse polynomial evaluation, HEALPix bounding-polygon clipping.
- `crossMatchKDTree` — fast nearest-neighbour queries against local SQLite/JSON catalogue caches.
- `detectAnomalies` — filters unassociated sources, generates overlays and `anomalies.json`.
- `testWCSCatalogOverlay` — validates pixel↔sky round-trip within **0.05 pixels**.
- `testUnassociatedSourceFlagging` — verifies isolation of synthetic injected transients against a mock Gaia catalogue.

Both tests are **specified here and not registered in `runSelfTests`**; catalog annotation and anomaly flagging are not implemented ([status](#10-implementation-status-consolidated)).

---

## 5. Pixel-level morphology

Catalogue coordinates do not describe visible extent or shape in a given photograph. Extract visible boundaries, centroid, major/minor axes, ellipticity, position angle, integrated flux, surface brightness and point-vs-extended classification directly from pixels.

**Tools:** SExtractor (Bertin & Arnouts 1996), DAOStarFinder (Stetson 1987), SEP, Photutils ([docs](https://photutils.readthedocs.io/) · [detection guide](https://docs.astropy.org/projects/photutils/en/stable/user_guide/detection.html)).

| Measurement | Meaning |
| :--- | :--- |
| FWHM | Apparent source width |
| Ellipticity / eccentricity | Degree of elongation and shape |
| Concentration index | Central light concentration |
| Gini coefficient | Distribution of pixel intensities |
| $M_{20}$ | Spatial distribution of brightest regions |
| Sérsic index | Approximate light-profile structure |
| Image moments & radial profile | Orientation, shape, radial brightness |

**ML and domain shift:** Galaxy Zoo 1 (Lintott et al. 2011, ~900,000 classifications) provides training labels; survey-trained CNNs/ViTs must be adapted carefully to stretched, noisy, undersampled or narrowband amateur images.

**Uncatalogued extended-object workflow:** detect in pixel space → measure centroid/dimensions/ellipticity → verify distinguishability from the local stellar PSF → query catalogues for a counterpart → compare measured morphology to available classifications and report candidates with confidence (enables faint PGC galaxies absent from basic Messier/NGC/IC overlays).

---

## 6. Time domain: transients, moving objects, brokers

### 6.1 ZOGY image subtraction

Detecting novae, supernovae or outbursts needs temporal differencing $D(x,y)=I_{\text{new}}(x,y)-I_{\text{reference}}(x,y)$. Raw subtraction fails under seeing/transparency changes; images must be astrometrically aligned, photometrically normalised and PSF-matched using **ZOGY** (Zackay, Ofek & Gal-Yam 2016, §12).

### 6.2 Detection strategies

| Phenomenon | Strategy |
| :--- | :--- |
| Supernova | New point source, often near a host galaxy |
| Nova | New or brightened stellar source |
| Variable star | Photometric change across epochs |
| Microlensing | Characteristic symmetric brightness evolution |
| Asteroid | Linear motion between sequential exposures |
| Comet | Motion plus extended coma/tail |
| Satellite | Linear trail or rapid sequential displacement |
| Cosmic ray | Single-exposure transient inconsistent with local PSF |
| Optical reflection | Ghost geometry relative to bright stars |
| Sensor artefact | Recurrence at fixed detector $(x,y)$ |

**Brokers/databases:** Transient Name Server ([TNS](https://www.wis-tns.org/); Galactic novae follow a separate reporting pathway), Zwicky Transient Facility ([ZTF](https://www.ztf.caltech.edu/)), Fink ([site](https://fink-broker.org/), Möller et al. 2021 and Fraga et al. 2024, both §12). **Reporting policy:** flag detections as *unmatched transient candidates*, never claim confirmed discoveries.

### 6.3 Moving objects

Reference algorithm **A-Track** (2016), *Computer Physics Communications*, 207, 524–530 (§12), using a modified line-detection algorithm (MILD) across sequential FITS exposures. Ephemeris services: Minor Planet Center ([website](https://minorplanetcenter.net/) · [astrometry guide](https://docs.minorplanetcenter.net/mpc-ops-docs/astrometry/) · [observations API](https://docs.minorplanetcenter.net/tutorials/notebooks/mpc_tutorial_api_observations/) · [orbits API](https://docs.minorplanetcenter.net/tutorials/notebooks/mpc_tutorial_api_orbits/)) and JPL Horizons ([ephemeris service](https://ssd.jpl.nasa.gov/horizons/)). Sequential pipeline: plate-solve each frame → record exact UTC/time-standard timestamps → detect sources per frame → transform to $(\alpha,\delta)$ → cross-match MPC/Horizons ephemerides → link consistent inter-frame motion vectors to estimate angular velocity and separate known bodies from unidentified movers.

---

## 7. IVOA and interoperability standards

**Sky-chart tools:** SkyChart / Cartes du Ciel ([site](https://www.ap-i.net/skychart/en/start) · [pictures docs](https://www.ap-i.net/skychart/en/documentation/pictures)) can ingest WCS-bearing FITS into its image archive and overlay charts but is not a per-image detection annotation engine — **use it as an interoperability export target**. Aladin Sky Atlas & HiPS ([Aladin Desktop](https://aladin.cds.unistra.fr/AladinDesktop/) · [Aladin Lite](https://aladin.cds.unistra.fr/AladinLite/doc/) · Bonnarel et al. 2000 · Fernique et al. 2015) supports multi-wavelength overlays, interactive SIMBAD/VizieR/NED queries, VOTable cross-matching and HiPS multi-resolution HEALPix tiling — **use it as the visualisation architecture reference**.

| Standard | Purpose | Documentation |
| :--- | :--- | :--- |
| FITS | Astronomical image/metadata format | [NASA FITS](https://fits.gsfc.nasa.gov/) |
| WCS | Pixel-to-sky transforms | [WCSLIB](https://www.atnf.csiro.au/computing/software/wcs/) |
| SIP | Polynomial distortion conventions | [FITS WCS](https://fits.gsfc.nasa.gov/fits_wcs.html) |
| VOTable | Catalogue exchange | [IVOA VOTable](https://www.ivoa.net/documents/VOTable/) |
| TAP / ADQL | Database querying | [IVOA TAP](https://www.ivoa.net/documents/TAP/) |
| Cone Search | Cone retrieval around coordinates | [IVOA ConeSearch](https://www.ivoa.net/documents/ConeSearch/) |
| HiPS | Multi-resolution progressive surveys | [CDS HiPS](https://aladin.cds.unistra.fr/hips/) |
| MOC | Multi-order sky coverage maps | [IVOA MOC](https://ivoa.net/documents/MOC/) |
| ADES | Minor-body astrometry exchange | [MPC ADES](https://docs.minorplanetcenter.net/mpc-ops-docs/observations/) |
| ProvenanceDM | Entity/Activity/Agent provenance graphs (+ `ActivityDescription`, `ActivityConfiguration`, `WasConfiguredBy`) | See [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) |

---

## 8. Proposed Go subsystem and data model

To preserve the zero-dependency `CGO_ENABLED=0` binary, implement the subsystem natively in Go, wrapping external solvers (Astrometry.net/ASTAP) via clean adapters rather than introducing a Python runtime.

| Package | Responsibility |
| :--- | :--- |
| `astrometry` | Plate solving, WCS, coordinate transforms |
| `astrometry/catalog` | Local star index and HEALPix spatial queries |
| `astrometry/solver` | Astrometry.net, ASTAP adapters, future native solver |
| `catalogues` | SIMBAD, Gaia DR3, VizieR, NED, VSX, MPC providers |
| `sources` | Detection, segmentation, sub-pixel centroiding |
| `classification` | Star/galaxy/nebula/quasar classification |
| `morphology` | Shape, ellipticity, size, surface brightness |
| `annotations` | Non-destructive labels, markers, ellipses, contours |
| `transients` | Nova/supernova/variable difference analysis |
| `movingobjects` | Comet/asteroid inter-frame motion |
| `anomaly` | Unassociated source and artefact detection |
| `photometry` | Flux, magnitude, uncertainty |
| `skyview` | Interactive visualisation |
| `interoperability` | FITS, WCS, VOTable, SkyChart, Aladin export |

```go
type AstronomicalObject struct {
    ID            string
    Name          string
    Designation   []string
    Type          ObjectType
    RA            float64
    Dec           float64
    X             float64
    Y             float64
    Magnitude     float64
    Flux          float64
    FluxError     float64
    MajorAxis     float64
    MinorAxis     float64
    PositionAngle float64
    Redshift        *float64
    ProperMotionRA  *float64
    ProperMotionDec *float64
    Confidence    float64
    Catalogue     string
    Identified    bool
    Transient     bool
    Moving        bool
}
```

**Schema note:** production structs should separate (1) catalogue entry, (2) pixel-space detection, (3) association/cross-match hypothesis, (4) morphological classification, while retaining coordinate frame, observation epoch, photometric filter, units and provenance.

### 8.1 Development roadmap and validation

Six phases: **1 Foundation** (FITS WCS, Astrometry.net/ASTAP adapters, transforms, FOV, round-trip validation) → **2 Identification** (Gaia DR3, SIMBAD, VizieR, NED, Messier/NGC/IC, offline cache) → **3 Visualization** (toggleable labels, ellipses, nebula boundaries, RA/Dec grid, SkyChart FITS+WCS export) → **4 Analysis** (extended sources, morphology, PSF, segmentation, photometry) → **5 Time domain** (ZOGY subtraction, variables, SN/nova, asteroid/comet tracking, MPC/TNS) → **6 Advanced research** (uncatalogued sources, ML, unsupervised outliers, historical comparison, confidence, structured reports).

Validation resources: Gaia DR3 (astrometric accuracy, proper-motion epoch propagation), SDSS DR17 (classification precision/recall), Galaxy Zoo (morphology), ZTF public (transient completeness/false positives), AAVSO VSX (variables), MPC observations (ephemeris/trajectory), Astrometry.net reference solutions (solve rate), synthetic FITS (round-trip accuracy, injected transient recall). Metrics: astrometric residual (arcsec), solve success rate (%), false-positive/negative rates, cross-match precision/recall, centroid error (px), photometric error (mag), transient completeness, confusion matrices, runtime RAM/CPU.

---

## 9. Appendix: HEALPix nested verification and face geometry

SkyStacker implements the HEALPix nested scheme of Górski et al. 2005 §5 with zero external dependencies. Verification is anchored to **`healpy 1.16.5`** (`hp.ang2pix(nside, theta, phi, nest=True)`):

```
theta = π/2 − dec   (colatitude, radians)
phi   = ra          (longitude, radians)
nest  = True        → nested scheme
```

This convention is confirmed by the three `nside=1` cases in `testHealpixNested`, all passing against `hp.ang2pix` under this exact mapping.

### 9.1 Face geometry (empirical)

HEALPix's 12 base faces are **spherical diamonds**, not constant-declination bands; the naive `|dec| > 41.81° = polar, else equatorial` model is incorrect:

```
pix2ang(256,     0) = (ra=45.00°, dec= 0.15°)  face 0, equatorial vertex
pix2ang(256, 65535) = (ra=45.00°, dec=89.82°)  face 0, polar vertex
pix2ang(256, 18344) = (ra=70.66°, dec=32.62°)  face 0, mid-diamond
```

- Face `0` spans a diamond from (RA=45°, Dec=0.15°) to the north celestial pole, reaching Dec=41.81° only along RA≈0°.
- The $z=2/3$ boundary marks where **faces 4–7 end**, not where faces 0–3 begin.
- The boundary with faces `4` and `5` follows a great circle, not a constant-declination parallel:
  - `dec=32.6° at ra=10°..80° → face 0`
  - `dec=32.6° at ra= 0° → face 4`
  - `dec=32.6° at ra=85°..90° → face 5`

**Root cause and fix:** the pre-`A1a.5` branch (`za <= 2/3 → equatorial`) misassigned points inside the equatorial vertices of polar faces. The bug was in **face-assignment logic**, not the within-face bit-interleave. Resolution: **A1a.5 code fix at commit `e47c3fd`**.

### 9.2 Fixture regeneration and tests

To regenerate `assets/testdata/healpix_reference.json` after a `healpy` upgrade or pixel-math refactor:

```bash
python3 -m venv /tmp/healpix-venv
/tmp/healpix-venv/bin/pip install healpy==1.16.5
/tmp/healpix-venv/bin/python scripts/healpix/generate_fixture.py \
    > assets/testdata/healpix_reference.json
./skystacker --selftest
```

Self-tests consuming the fixture: `testHealpixNside16Match`, `testHealpixNside256Match`, `testHealpixCornersMatch`.

Calibration run log (`healpy 1.16.5`, **2026-11-13**), re-executable via `scripts/healpix/check_calibration.py` to detect upstream behaviour changes:
- Part 1 — three-line `nside=1` output confirming `theta=π/2−dec`, `phi=ra`, `nest=True`.
- Part 2 — declination scan at RA=0° mapping Dec→face; RA scan at Dec=32.6° confirming transitions across faces `4`, `0`, `5`.

---

## 10. Implementation status (consolidated)

- **Historical snapshot (v0.6.0):** `main.go` already contained a Gaia ADQL client, HEALPix nested queries, a TAN WCS fit and `plate_solution.json` via `--plate-solve`; nested HEALPix helpers were covered by `testHealpix*` with the fixture at `assets/testdata/healpix_reference.json`. Not present then: sky-chart rendering, multi-catalogue annotation and transient overlays. `testWCSCatalogOverlay` and `testUnassociatedSourceFlagging` were specified but not registered in `runSelfTests`; catalog annotation and anomaly flagging were not implemented. The HEALPix verification document remains a verification log, not a pending implementation task.
- **Current state (v0.6.1):** the repository advanced to **v0.6.1 with 274 registered self-tests**, native pure-Go CR2 decoding, `--config`, `--resume`, deconvolution, narrowband and the goPureRaw subproject (details in [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) and [`README.md`](README.md)). The extended astrometric catalog-annotation and anomaly-flagging subsystem specified in §4 was implemented on 2026-11-18 (`--annotation-catalogs`, elliptical apertures, `catalog_matches.json`, SNR>5 unassociated-source gate; `testWCSCatalogOverlay` and `testUnassociatedSourceFlagging` are registered).
- **SIP in the resampler:** SkyStacker fits/stores order-2 SIP in the plate solution and historically did **not** apply it in pixel-registration resampling. As of 2026-11-17 **`--resample-sip`** loads stored order-2 SIP from `plate_solution.json`, undistorts reference pixels, applies the similarity/affine registration transform in undistorted space and redistorts source pixels before bilinear sampling — closing the "SIP in resampler" proposal within the shared-distortion assumption. `testResampleSIP` verifies distort/undistort round-trip, sampled mapping composition and a synthetic star relocation. Per-frame astrometric reprojection and drizzle remain open. See `TECHNICAL_SPEC.md` §2/§4.5 and [`README.md`](README.md).
- **Gaia TAP resilience:** HTTP 429 (queue full) is retried with capped `Retry-After`; the client's 1 req/s pacing and `MAXREC=2000` already fit Archive v3.15 quota limits. `testGaiaQueryTile429Retry` covers this.
- **Pre-solve detrending:** `detectStarsWithDetrend` retries star detection on a scratch copy processed with the existing coarse percentile background when the first pass finds <20 stars; it does not modify the light buffer. `testDetrendStarDetection`.
- **HEALPix:** nested helpers in `main.go`, verified against `healpy 1.16.5`, face-assignment fix at commit `e47c3fd` (see §9).

Toolchain, calibration/QA, denoising, sidecar and drizzle implementation notes are tracked in [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md), [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) and [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md).

---

## 11. Research gaps and open problems (consolidated)

- **Differential chromatic refraction (DCR):** first-order limitation in wide-field chromatic astrometry without atmospheric dispersion correctors. Rubin/LSSTCam DP2 (COSMOS-extended, 2026) found zenith-projected residuals tied to source SED vs reference stars in u,g,r; WFST global calibration (Yang 2026) reaches tens of mas per CCD with Gaia DR3 but star sparsity limits simultaneous DCR + lateral-colour modelling (DES: joint distortion/DCR/lateral colour ~3–6 mas per exposure, GP regression for turbulence). Methods: SCAMP (stereographic reprojection, high-order polynomials, DCR corrections), `galsim.dcr` (Edlén/Filippenko physics with T/P/water vapour), LSST Science Pipelines / The Monster catalogues. SkyStacker stores plate solutions/SIP in limited form (`TECHNICAL_SPEC.md`); advanced DCR needs zenith/parallactic angle and atmospheric metadata plus model work — environmental telemetry should be ingested via external adapters, not embedded in the binary (see [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md)).
- **Gaia DR4 / Archive v3.15:** DR4 (≈66 months nominal + extended; extended phase completed 15 Jan 2025; ~400 TB footprint estimate) scheduled **2 Dec 2026 ~12:00 CET** with archive blackout **25 Nov 2026 12:00 CET → 2 Dec 2026 12:00 CET**; pre-release from **29 Jun 2026**; DR5 to follow. v3.15 deployed **14 Oct 2026** (backend Greenplum → EDB WarehousePG, `gaiadr3.gaia_source` partitioning, UDFs `gaia_healpix_index` / `ivo_healpix_index` / `ivo_healpix_center`), DataLink products served from Parquet and renamed `RAW`/`INDIVIDUAL` → **`DATAMODEL_GAIA` / `DATAMODEL_STANDARD`** (`COMBINED` removed earlier; also in Astropy astroquery changelog). Quotas: anonymous sync 60 s, async 1 min, ≤50,000 rows; authenticated 60 s sync / 120 min async / unlimited rows; DataLink 50 sources anonymous vs 5000 authenticated; ~100 anonymous concurrency pool, 5 parallel authenticated jobs, queue 50 then reject. Any live TAP/ADQL client needs pagination, backoff, local caching, the renamed DataLink IDs and ≤4 active requests; dense Galactic-plane cone searches will truncate anonymously. Authoritative sources: [DR4](https://www.cosmos.esa.int/web/gaia/dr4) · [bulletin](https://www.cosmos.esa.int/web/gaia/bulletin-archive) · [v3.15 release notes](https://www.cosmos.esa.int/web/gaia-users/archive/release-notes) · [astroquery changelog](https://docs.astropy.org/projects/astroquery/en/latest/changelog.html) · [400 TB commentary](https://medium.com/@andy.a.g/gaia-dr4-is-scheduled-for-december-2-at-about-400-tb-esa-changed-the-archive-rules-first-61dd4a08fe22).
- **Quad / fast solvers:** evaluate Cedar-Solve (tetra3 lineage), tetra3 Rust v0.14.0 / tetra3rs and ASTAP D50 as lost-in-space or narrow-field fallbacks; no SCAMP successor found. Astrometry.net custom-index builders (`text2fits.py`) are memory-hungry on large files.
- **NWAY:** Salvato et al. 2018 (maintained notes to 2025) simultaneous N-catalog association using positional uncertainties, non-association probability and colour/SED priors, reported agreement with likelihood-ratio in XMM-COSMOS — a research direction for annotation, not a current feature.
- **Billion-row catalogues:** HATS (Hierarchical Adaptive Tiling Scheme) + LSDB (LINCC) store adaptive HEALPix pixels as Apache Parquet for distributed unions without central TAP; cds-healpix-rust, MOCPy/MOCli (IVOA MOC boolean ops on mixed-order cells), HiPS/hips2fits are the modern MOC/HiPS path. Consider for offline catalogue caches if annotation grows (see §9).
- **ASTAP 32-bit HFD ceiling:** 21.46 px in packed `%errorlevel%`; prefer `HFD_MEDIAN`/`STARS` stdout, clamp/guard higher values, verify `-ra` units, do not assume a CLI hyperbolic autofocus path, codesign on Apple Silicon.
- **Distortion/resampling comparisons:** no 2024–2026 peer-reviewed comparison of TAN-SIP vs TPV vs TNX/GNOM in resamplers, quad-tree/HEALPix distortion lookups, or phase-correlation/Fourier-Mellon registration for wide-field amateur rotation/scale was found. Seti Astro Suite Pro's SIP unwarping and anti-ringing Lanczos warp are reference behaviours; per-frame reprojection and drizzle remain open.
- **Reporting discipline:** all transient/moving-object candidates must be labelled unmatched candidates, not discoveries; reporting policy for Galactic novae differs from TNS.

---

## 12. Academic references (consolidated)

The 76-source advanced reference survey (FITS WCS/PV/SIP, blind and low-latency
solvers, SExtractor/PSFEx source extraction, Gaia/ATLAS catalogues and TAP/ADQL,
HFR autofocus, Drizzle and rejection heuristics) is indexed in
[03-reference-and-literature.md](03-reference-and-literature.md) §6. Its
quantitative claims are summarised there; they are not repeated in this file.

Single consolidated list; each DOI/URL appears once. The **master bibliography and glossary** live in [`03-reference-and-literature.md`](03-reference-and-literature.md).

**Astrometry and plate solving**
1. Lang, D. et al. (2010). *Astrometry.net: Blind astrometric calibration of arbitrary astronomical images.* AJ, 139, 1782–1800. [DOI](https://doi.org/10.1088/0004-6256/139/5/1782) · [arXiv:0910.2233](https://arxiv.org/abs/0910.2233).
2. Bertin, E. (2006). *Automatic Astrometric and Photometric Calibration with SCAMP.* ADASS XV. [PDF](https://articles.adsabs.harvard.edu/pdf/2006ASPC..351..112B).
3. Calabretta, M. R. & Greisen, E. W. (2002). *Representations of celestial coordinates in FITS.* A&A, 395, 1077–1122. [DOI](https://doi.org/10.1051/0004-6361:20021327).
4. Bertin, E. & Arnouts, S. (1996). *SExtractor: Software for source extraction.* A&AS, 117, 393–404. [DOI](https://doi.org/10.1051/aas:1996164).
5. Shupe, D. L. et al. (2005). * SIP — Simple Imaging Polynomial* convention v1.0 (referenced in §2.2; no DOI in source material).

**Catalogues and identification**
6. Wenger, M. et al. (2000). *The SIMBAD astronomical database.* A&AS, 143, 9.
7. Ochsenbein, F., Bauer, P. & Marcout, J. (2000). *The VizieR database of astronomical catalogues.* A&AS, 143, 23–32. [Paper DOI](https://doi.org/10.1051/aas:2000169) · [VizieR DOI](https://doi.org/10.26093/cds/vizier).
8. Gaia Collaboration (2023). *Gaia DR3: Summary of the content and survey properties.* A&A, 674, A1. [DOI](https://doi.org/10.1051/0004-6361/202243940).
9. Gaia Collaboration (2023). *Gaia DR3: The extragalactic content.* A&A, 674, A41. [DOI](https://doi.org/10.1051/0004-6361/202243232).
10. Creevey, O. L. et al. (2023). *Gaia DR3: Apsis. I. Methods and content overview.* A&A, 674, A26. [DOI](https://doi.org/10.1051/0004-6361/202243688).
11. Clarke, A. O. et al. (2020). *Identifying galaxies, quasars, and stars with machine learning.* A&A, 639, A84. [DOI](https://doi.org/10.1051/0004-6361/201936770) (111M SDSS sources).
12. *Machine learning-based photometric classification of galaxies, quasars, emission-line galaxies, and stars* (2024). MNRAS, 527, 4677. [DOI](https://doi.org/10.1093/mnras/stad3436) (Random Forest on SDSS DR17 + AllWISE).

**Classification and morphology**
13. Stetson, P. B. (1987). *DAOPHOT: A computer program for crowded-field stellar photometry.* PASP, 99, 191.
14. Lintott, C. et al. (2011). *Galaxy Zoo 1: data release of morphological classifications for nearly 900,000 galaxies.* MNRAS, 410, 166. [DOI](https://doi.org/10.1111/j.1365-2966.2010.17432.x).
15. Fraix-Burnet, D. (2023). *Machine learning and galaxy morphology: for what purpose?* MNRAS, 523, 3974. [DOI](https://doi.org/10.1093/mnras/stad1654).

**Transients and moving objects**
16. Zackay, B., Ofek, E. O. & Gal-Yam, A. (2016). *Proper Image Subtraction — Optimal Transient Detection, Photometry, and Hypothesis Testing.* ApJ, 830, 27. [DOI](https://doi.org/10.3847/0004-637X/830/1/27) · [arXiv:1601.02655](https://arxiv.org/abs/1601.02655).
17. *A-Track: A new approach for detection of moving objects in FITS images* (2016). Computer Physics Communications, 207, 524–530. [DOI](https://doi.org/10.1016/j.cpc.2016.07.023).
18. Möller, A. et al. (2021). *fink, a new generation of broker for the LSST community.* MNRAS, 501, 3274. [DOI](https://doi.org/10.1093/mnras/staa3602).
19. Fraga, B. M. O. et al. (2024). *Transient classifiers for Fink: Benchmarks for LSST.* A&A, 692, A208. [DOI](https://doi.org/10.1051/0004-6361/202450370).
20. Salvato, M. et al. (2018). *NWAY* Bayesian cross-matching (referenced in §11; no DOI in source material).

**Visualisation and spatial indexing**
21. Bonnarel, F. et al. (2000). *The ALADIN interactive sky atlas.* A&AS, 143, 33. [DOI](https://doi.org/10.1051/aas:2000331).
22. Fernique, P. et al. (2015). *Hierarchical progressive surveys: Multi-resolution HEALPix data structures.* A&A, 578, A114. [DOI](https://doi.org/10.1051/0004-6361/201526075).
23. Górski, K. M. et al. (2005). *HEALPix: A Framework for High-Resolution Discretization and Fast Analysis of Data Distributed on the Sphere* (§5 nested scheme; no DOI in source material).
24. Fruchter, A. S. & Hook, R. N. (1997). *Drizzle* variable-pixel linear reconstruction (referenced in Siril docs; see §1 for link).

---

## Sources absorbed

- `research/02-astrometry-and-identification/astrometric-catalog-annotation-and-anomalies.md` — WCS/SIP forward projection, elliptical apertures, k-d tree cross-match, SNR>5.0, `anomalies.json`, module/test names, DCR append.
- `research/02-astrometry-and-identification/astronomical-identification-and-sky-charts.md` — six pillars, plate-solving papers/algorithms, FITS keywords, catalogue matrix, morphology, ZOGY/transients/A-Track, IVOA table, Go subsystem/struct, roadmap, three duplicated bibliography blocks (deduplicated into §12).
- `research/02-astrometry-and-identification/healpix-reference-verification.md` — nested convention, face-geometry numbers, commit `e47c3fd`, `healpy 1.16.5`, fixture commands/tests.
- `research/05-research-deep-dive-2026-11-16.md` — Pillar B (Gaia v3.15/DR4, DCR, quad solvers, HATS/LSDB, NWAY), §9 ASTAP CLI audit, §11.1 TAN-SIP, §12 quotas, §13.5 test statuses, v0.6.1 correction.
- `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — Gaia DR4 quotas/blackout, Cedar/tetra3rs/SkySolver, ASTAP updates, Siril drizzle/plate-solving context, `--resample-sip` status, survey gaps.
- `research/08-curated-content-cycle-5-2026-11-17.md` — Astrometry.net `build-index.html`/`code.html`/`axy`, `tetra3rs` tutorial, Siril plate solving, ASTAP databases.

A cycle-6 curated corpus of plate-solving, index, catalog and PCC sources is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §9.

## 13. Survey cutout and SVG sky map (2026-11-18, append-only)

Two user-proposed deliverables were analysed; both are feasible under the CLI architecture and are now roadmap items 48 and 49.

### 13.1 Survey cutout with the exact frame geometry

The CDS **hips2fits** service serves HiPS survey cutouts through a plain HTTP GET. Verified parameters: `hips` (e.g. `CDS/P/DSS2/red`, `CDS/P/DSS2/color`, `CDS/P/PanSTARRS/DR1/g`), `width`, `height`, `projection` (`TAN`, `SIN`, ...), `ra`, `dec` (degrees), `fov` (degrees, largest image dimension), `coordsys=icrs`, `rotation_angle` (degrees), `format=fits|jpg|png`, and an alternative `wcs` JSON object that defines the cutout astrometry directly; cutouts are limited to 50 million pixels. Endpoints: `https://alasky.cds.unistra.fr/hips-image-services/hips2fits` and mirror `https://alaskybis.cds.unistra.fr/hips-image-services/hips2fits`.

Feasibility notes: with a stored `plate_solution.json`, SkyStacker already has CRVAL/CRPIX/CD/SIP, so it can compute the frame centre, largest-dimension FOV and rotation, request a FITS cutout at exactly `refW × refH`, then refine alignment with the existing triangle matcher and `matchStarsToReference`/`resampleToReference`. Without a plate solution the request can be seeded from FITS RA/DEC plus a pixel-scale estimate, but the exact match is not guaranteed. Network access must stay opt-in and cached; the tool must never block an offline stacking run. The result is a real observatory reference frame, not a synthetic image — useful for visual comparison and provenance, not for altering the stack.

### 13.2 SVG map of catalogued objects in the field

The existing Gaia HEALPix tile cache plus `wcsTanForward` can project every cached Gaia source into the solved frame, giving a one-to-one star map with the same width, height and orientation as the processed image. Emitting an SVG with circles sized by magnitude and labels for bright stars is straightforward (M). Extended objects (galaxies, nebulae) are **not** in Gaia: their shapes, position angles and dimensions require SIMBAD, VizieR/HyperLEDA/NED or a bundled static catalogue, and those values are incomplete and heterogeneous. A correct implementation must therefore separate point sources (Gaia, always available offline) from extended sources (optional online catalogues), draw ellipses only when major/minor axis and PA exist, and label the source of every object. The SVG is a diagnostic overlay written beside `stacked_annotated.jpg`, not a replacement for the scientific FITS products.

**Implementation status (2026-11-18):** both designs are shipped as roadmap items 48 and 49 in `sky_products.go`; `--survey-cutout` uses the verified hips2fits parameters above and `--sky-map` uses the offline Gaia cache, with extended-object ellipses only when a local catalog supplies axis/PA data.

A cycle-7 curated corpus of ASTAP/Tetra3/PI ImageSolver/AnnotateImage/SEP/SPCC sources is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §10.

A full goCrawler export with Gaia DR4/DR3 sizes, Astropy SIP and service limits is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §11.

## 14. Blind plate solving through astrometry.net (2026-11-18, append-only)

**Problem confirmed.** The internal solver is *seeded*: `solvePlate` derives a single HEALPix tile index from a known (RA, Dec), and its "blind" branch only scans a 3×3 `nside=64` neighbourhood around (0°, 0°). Without `--plate-center` or FITS `RA`/`DEC`, a real field cannot be solved; JPEG EXIF RA/Dec was never read. Star detection alone cannot generate a Gaia query because the query key (the tile) already requires the answer (the centre). This was the motivation for roadmap item 68.

**Design shipped (item 68, Done).** Following the earlier architectural recommendation in §1 — *adapters first, native solver later* — the fallback chain is:

1. **Seeded Gaia solve** (unchanged) whenever a flag or FITS header supplies a pointing.
2. **Local astrometry.net adapter**: `--astrometry-cmd` (default `solve-field`) runs on the actual image file with `--no-plots`, `-D <tmp>` and `--wcs <tmp>/solved.wcs`; optional arcsec/px bounds from `--solver-focal-length`/`--solver-pixel-size` via `f[mm]=(pixel[µm]/scale[″/px])×206.26`. A missing executable is reported with an actionable message, never fatal to the rest of the pipeline.
3. **Web adapter**: nova's documented JSON API — `POST /api/login` with `request-json={"apikey":…}` returns a session; `POST /api/upload` is multipart with a `request-json` field (session, licensing defaults, optional `scale_units=arcsecperpix`, `scale_type=ev`, `scale_est`, `scale_err=20`) plus the `file` part; `GET /api/submissions/<id>` yields job IDs, `GET /api/jobs/<id>` yields `status: success|failure`, and `GET /wcs_file/<id>` returns the WCS FITS (the API help requires a `Referer` header for programmatic file downloads). API key comes from `--astrometry-api-key`, `AN_API_KEY` or `ASTROMETRY_API_KEY` and is redacted from report command lines. Uploads default to `publicly_visible=n`.

**Normalisation of external results.** `parseAstrometryWCS` reads the FITS WCS produced by either backend: CTYPE TAN (including `-SIP`), CRVAL/CRPIX/CD, the `CDELT`/`PC` fallback, and the six quadratic SIP terms `A_2_0,A_1_1,A_0_2,B_2_0,B_1_1,B_0_2`. CRPIX is converted from FITS 1-based to SkyStacker's internal 0-based convention; orders above 2 are degraded to quadratic with a warning. `refineAstrometrySolution` then queries the Gaia tile at the solved centre and calls the shared `plateSolutionFromGaia`, so an external solve emits the same `plate_solution.json` (CRVAL, CD, SIP, RMS, match count) as an internal one. A failed Gaia refinement keeps the external WCS.

**Coverage and limits.** The chain is wired into the stacking pipeline, `--post-only` (a temporary JPEG is rendered from the stored `stacked_mean.f32raw` for upload) and standalone image mode; `--image-op plate-solve` solves a single bare JPEG/FITS. Four selftests cover WCS parsing (CD and CDELT/PC), a mock `solve-field` executable, a mock nova HTTP service (login/upload/submission/job/WCS download) and the fallback/error messages. Sync/reslew/repeat-until-error from item 68 belongs to the observatory-automation sidecar (item 17) and remains out of scope; offline operation still depends on installed astrometry.net indexes or a pre-seeded Gaia cache.

**New roadmap items from this work:** 72 (read GPS IFD `0x8825`/XMP RA/Dec as an optional seed), 73 (native all-sky quad/asterism index R&D, Deferred) and 74 (ASTAP blind-solver adapter with D50 catalogue fallback).

## 15. Calibrationless invariant-matching corpus (2026-11-19, append-only)

**Provenance.** `research/pendingToAnalyse/Algoritmos de Plate Solving Ciego.md` was a 124 KB deep-research report (41 citations) on solving an image with *no* pointing, pixel pitch, focal length or roll. It was deduplicated against §1, §6, §11 and §14 of this file and against [03](03-reference-and-literature.md) §6.2; the processed file was removed and `research/pendingToAnalyse/` kept with `.gitkeep`. Speed figures in the corpus are developer/community reports, not independent benchmarks — consistent with the reliability caveat already recorded in [03](03-reference-and-literature.md) §6.

### 15.1 Mathematical framing (new detail)

With known focal scale, `n` centroids supply `2n` coordinates and lose three degrees of freedom to translation and roll, leaving `2n−3` invariants. With **all** metadata absent the isotropic scale (FOV) is a fourth unknown, leaving **`2n−4` invariants**: a triangle has 2, a quad 4, a quintuple 6. Quads are the practical optimum — they encode two edge-sharing triangles, roughly squaring triangle distinctiveness while keeping completeness; triangles are heavily oversubscribed, and quintuples lose completeness sharply on shallow or off-band images (the joint probability that all five catalogue stars survive drops, and 6-D tree lookups slow down). Six measured inter-star distances carry redundant information relative to a canonical 4-D encoding, but sorted distance ratios are naturally invariant to mirror **parity**, which a canonical backbone is not and therefore must be queried twice. `C(500,4) > 2.57e9` quads explains why naive enumeration is impossible.

### 15.2 New engineering facts not already in §1/§6.2

- **Flux bounds search cardinality, it is not a hash key.** Top-`N` flux ranking (`N ≈ 200–500`) plus spatially local quads (each bright star paired with nearest neighbours in a 2-D k-d tree, yielding ~300–1000 compact quads) controls the combinatorics; OPMb-style optimistic ordering allows immediate termination. Passband mismatch (Hα, UV, NIR, unfiltered vs Gaia G, Tycho-2, 2MASS) reorders brightness and 0.1-mag-quantised indexes cause colour-term dropouts, so robust builders generate redundant quads from *subsets* of bright stars rather than requiring the four brightest image stars verbatim. Compact baselines also limit barrel/pincushion distortion and differential refraction, which helps dense Milky Way fields.
- **Astrometry.net index layout:** HEALPix `nside=1` (12 tiles) or `nside=2` (48 tiles) with overlap margins about half the image width; the 4100-series (Tycho-2, 0.36 GB, scales 7–19, fields >1°) and 5200-series light indexes (Tycho-2 + Gaia DR2, 36.14 GB, scales 0–6, fields <1°) total about 36.5 GB; scale bands span 2′–2000′ (scales 0–19). Lang et al. report >99.9% success with zero false positives and tolerance to 1000–2000 years of proper motion; a hobbyist Skysolve setup reports ~0.5 s.
- **Tetra/Tetra3/Cedar-Solve/olive-solve:** normalised 4-D coordinates binned exponentially, with bin edges spaced recursively from centroid and FOV error bounds so noisy detections land in the true catalogue bin without bloating collision lists; an open-addressing hash table with quadratic probing (4 KB cache-aligned) gives O(1) lookup, typically one database access. A single table covering arcminutes to 45° is impractical. Developer reports: ~20 ms for Tetra3/Cedar and <1 ms for olive-solve on a Raspberry Pi 5 (~15× faster extraction, ~130× faster solving than Python cedar-solve).
- **ASTAP:** hashes are generated *dynamically at runtime* from ~300–500 brightest quads for each FOV; `FOV=auto` decreases FOV in ~30% steps (e.g. 10° → 0.3°); verification uses median/SD filtering plus a least-squares six-constant fit from ≥3 quads. Sorted distance ratios give parity invariance. The author reports solving 100°–170° fisheye frames; at a 0.007 distance-ratio tolerance matched quads halve after ~150 years and fail near ~300 years from an Epoch-2025/J2000 catalogue, while nova tolerates 1000–2000 years.
- **Pyramid (Mortari 2004)** uses six angular distances across four stars with a monotone k-vector search array and fourth-star triangle verification; it assumes calibrated FOV and is superseded by direct 4-D hashing when scale is unknown.
- **Tiered refinement:** tetra3rs documentation reports <9″ RMSE and <3″ FITS WCS centre agreement after four solve-and-calibrate passes; wide-field distortion and atmospheric refraction need multi-pass SIP (order 2–9), otherwise a strictly linear six-constant fit leaves large corner residuals.
- **Observable recovery:** CD column norms give CDELT1/2 in deg/px and hence FOV = dimension × scale; `atan2` ratios of CD elements give CROTA1/2; `sign(det CD)` gives parity; the converged tangent point evaluated at CRPIX gives the frame-centre RA/Dec.

### 15.3 Six-stage reference architecture for a future native solver

(1) 2×/4× downsampling above ~1280×960 plus tiled BlockMedian/LineMedian background equalisation, PSF-width rejection of hot pixels/cosmic rays and intensity-weighted centroids; (2) flux ranking and spatial-neighbour quad construction; (3) canonical 4-D hashing *or* sorted six-distance ratios with an explicit parity twin; (4) coarse-to-fine multi-scale catalog lookup with optimistic early termination; (5) per-quad scale consistency (median + dispersion of arcsec/px, reject outliers) followed by Bayesian log-odds against a uniform random-match null including catalog/detection dropouts, accepting only >threshold or ≥3 corroborating quads; (6) overdetermined least-squares TAN fit (six plate constants), iterative tangent-point refinement and SIP distortion. This is the design target behind roadmap item 73; nothing in this section is implemented natively today (item 68 delegates to external solvers).

### 15.4 Gap analysis and roadmap candidates promoted

Genuinely new challenges, not covered by items 1, 13, 27, 45, 54, 57, 68, 69 or 73 and not duplicated elsewhere, were promoted as items **75** (scale-ratio + Bayesian log-odds verification), **76** (proper-motion epoch propagation for blind indexes) and **77** (independent blind-solve benchmark corpus). The index-layout, binning, flux-ranking and tiered-refinement numbers above are absorbed into the acceptance text of item 73; the ASTAP FOV-stepping details are absorbed into item 74. Explicitly **not** promoted: replacing the shipped external-solver chain (item 68 is Done), a general mosaic solver, or a GPU/ONNX inference path (already rejected).

**New source index (12 URLs, deduplicated against [03](03-reference-and-literature.md) §6.2):** ASTAP pattern-recognition page `hnsky.org/astap_astrometric_solving.htm`; TETRA hash-table paper `digitalcommons.usu.edu/cgi/viewcontent.cgi?article=3655&context=smallsat`; `github.com/oakamil/olive-solve`; `github.com/ssmichael1/tetra3rs`; ASTAP database-age thread `cloudynights.com/forums/topic/880640`; FITS SIP convention `fits.gsfc.nasa.gov/registry/sip/SIP_distortion_v1_0.pdf`; Astrometry.net README `astrometry.net/doc/readme.html`; index download `data.astrometry.net/`; lost-in-space survey `pmc.ncbi.nlm.nih.gov/articles/PMC7248786/`; OPMb paper `arxiv.org/html/1903.09423`; Tabur OPM `arxiv.org/pdf/0710.3618`; WCS Paper II revision `listmgr.nrao.edu/pipermail/fitsbits/1999-April/000880.html`. Previously indexed here: `github.com/smroid/cedar-solve`.

## 16. Solver-chain items delivered 2026-11-19 (append-only)

Seven roadmap items linked to plate solving were closed in one pass. **72** adds EXIF GPS IFD (`0x8825`) and XMP `GPSLatitude`/`GPSLongitude` seeds to `exifData`, so images with metadata avoid a network solve; it remains optional because item 68 already solves metadata-free images. **76** propagates Gaia DR3 positions from J2017.5 to the EXIF observation epoch via `pmra`/`pmdec` (the MAS-to-radian conversion and `mu_alpha*` convention are in §15), applied to both seeded and blind-solve refinement. **74** adds a fully offline ASTAP backend (`--astap-cmd`, `-f -fov 0 -z 0 -r 30`), copying the image into a temp directory and parsing the returned `<base>.wcs`; ASTAP's FOV=auto stepping is thus available without touching user files. **67** exposes Shupe-style order-2 SIP→PV coefficients (`PV1`/`PV2` in `plate_solution.json`) with a 7×7 grid round-trip test. **75** records report-only `Verify` metrics (per-quad arcsec/px median and RMS plus a simplified random-match log-odds) on blind solutions. **69** adds a cached VOTable cone-search client for SDSS (≤180″), IRSA TAP ADQL (≤60″) and VizieR, wired into annotations via `--cone-catalog`/`--cone-radius`. **77** adds `--blind-benchmark DIR` producing `blind_benchmark.csv` (success, seconds, RMS, matches, method) for any subset of backends.

**Annotation layers delivered 2026-11-19:** items 50 and 55 are now Done. The sky-map SVG is emitted as independent `<g>` layers (background, point sources, extended objects, SSO vectors, labels, footer) with stable ids and `class="skystacker-layer"`, so external applications can toggle or restyle groups. Ellipse PAs are mapped from the celestial to the pixel frame with `correctedPA`, which adds `atan2(CD1_2, CD2_2)` field rotation and negates the PA when `det(CD) < 0`. Labels use eight selectable fields, sixteen candidate anchors (eight directions × two radii) and twenty deterministic collision passes. `--sso-ephemeris` parses `a:/c:/p:/dp:/s:` MPC-style IDs, a three-symbol IAU site code and mandatory `DATE-OBS`, falls back to geocentric positions when the site header is incomplete, and draws velocity vectors in both the SVG layer and the JPEG overlay.

**Native blind-solver prototype (2026-11-19).** Item 73 moved from Deferred to Open with a working prototype: `buildNativeIndexFromCache` derives parity-tolerant four-star descriptors from cached Gaia tiles into a JSON index (`--build-blind-index`), and `nativeBlindSolve` votes for HEALPix tiles from image quads, filters by the focal-length scale hint and verifies the best tiles with the existing `matchStarsToGaia`/`plateSolutionFromGaia`/`verifySolution` chain. `--plate-solver native` integrates it into the fallback chain; `testNativeBlindIndex` solves a synthetic mirrored field end to end. The index now carries three density/scale bands (25/50/100 bright stars for 5–60, 0.8–6 and 0.05–1.2 arcsec/px) selected by the focal-length hint, and `--blind-allsky`/`--blind-max-tiles` sweep HEALPix tiles into the Gaia cache before building. Fine-grained full-sky index generation and large-index tooling remain the R&D part of item 73. Magnitude-limited catalogs are now published in `catalogs/` with a checksummed manifest; three real Gaia DR3 all-sky indexes at nside 4 are included: `mag07` (G≤7, 17,426 stars, ~1.3 MB), `mag10` (G≤10, 57,600 stars, ~3.6 MB) and `mag10deep` (G≤10, 333,567 stars, 309,078 coarse+fine multi-quad descriptors, ~25 MB). They were built from all 192 HEALPix tiles using resumable chunked TAP queries (`--blind-tile-offset`, `--blind-refresh`); deeper/finer catalogs can be built with `--blind-mag-limit`/`--blind-brightest` or downloaded on demand. Candidate gathering uses an angular radius around winning cells and merges the top voted cells, covering wide fields that span tile boundaries.

**Frame selection, detection and photometry delivered 2026-11-19.** Item 51 adds a stdlib recursive-descent expression engine (`--frame-expr`) over hfr/bg/blob/eccentricity/psf-scale/stars with `_MIN/_MAX/_MED/_SIGMA` variables and an `ssweight` CSV column. Item 54 implements the calibrated `T = αC⁻¹S` matched filter with a local diagonal variance map, Gaussian template and positive/negative components behind `--matched-filter`. Item 70 adds local annulus sky, Kron radius, FLUX_RADIUS and segmentation masking behind `--aperture-photometry`. Item 63 adds `CALSTAT` and dark master temperature/exposure/gain keywords to reports and FITS headers, plus a native width/height gate that refuses to resize masters.

**Drizzle and RAW-ingestion metrology delivered 2026-11-19.** Item 64 implements the Fruchter & Hook filled-dither noise ratio R (r=pixfrac/scale, R(0.6,0.5)=1.662) and flags weight RMS above 20% of the median in `report.txt`/`report.json`. Item 65 writes per-pixel `stacked_wht.f32raw` and 32-bit contributor `stacked_ctx.u32raw` sidecars with versioned headers. Item 66 classifies 2/3/4/6/9-point dither patterns from registration shifts and predicts null-pixel hole risk by sampling a unit cell with pixfrac droplets. Item 71 adds `--check-acquisition-keywords` to validate `ROWORDER`, `BAYERPAT`, `XBAYROFF/YBAYROFF`, `EGAIN` and `PIERSIDE` against the N.I.N.A. vocabulary (JSON, exit 1 on invalid values).

**Real-image corpus (2026-11-19).** 14 processed images of known fields at effective focal lengths 50–1200 mm (`assets/Blind-Plate-Solving/`, no EXIF metadata) were run through `scripts/bench-blind.sh` with the `mag10` index: 0/14 solved; votes 9–24 and 44–63 s per image. Even a correctly seeded Gaia solve of `M42.jpg` fails with `0 matched pairs`, pointing to single-tile catalog sparsity (≈30 Gaia stars per nside=64 tile) and processed-image centroid quality rather than voting alone. Geometric quad hypotheses were added to the native verifier but did not reach ≥6 inliers on real images. See `docs/blind-corpus-results.md`.

**Native solver robustness (2026-11-19).** `nativeQuads` now emits up to ten quads per bright star (combinations of its five nearest neighbours) and encodes each at both the legacy 0.05 bin and a prefixed fine 0.02 bin; the index stores both variants so old catalogs remain readable. This raises `mag10deep` from 20,181 to 309,078 quad keys and lets `testNativeBlindNoisy` solve a mirrored field with 1 px centroid noise that the single-quad version could not. Real-image success still depends on clean star extraction; `MilkyWay.jpg` produces votes but no consistent Gaia transform.

**Asymmetric background and regional indexes delivered 2026-11-19.** Item 56 adds `--bg-asymmetric`: sigma=1.4826×MAD with asymmetric low/high clipping, a degree 1-6 polynomial or Gaussian RBF coarse fit, and multiscale harmonic inpainting of masked regions, with configurable scale [1,10]. Item 73 gains `--blind-region RA,DEC,RADIUS_DEG` for cone-limited high-nside indexes and `--blind-index-info` for metadata inspection; fine-grained full-sky generation and large-index storage tooling remain the R&D remainder.

**Still open or intentionally deferred:** item 1 (ASTAP interoperability hardening) remains Blocked pending documentation/CLI conformance tests beyond the adapter; items 13, 27, 45 and 57 remain Deferred/Blocked because they require per-frame WCS solutions, higher-order SIP or spline distortion models. Registered self-test count: 274.

## 17. Efficient blind solving — second corpus (2026-11-20, append-only)

**Provenance.** `research/pendingToAnalyse/Blind Plate Solving Eficiente.md` (75 KB, 23 references) was a deep-research report on fast native blind solving. It was deduplicated against §15, §16 and `docs/blind-corpus-results.md`; the processed file was removed and `research/pendingToAnalyse/` kept with `.gitkeep`. The mathematical foundations (2n−4 invariants, quad optimality, parity handling, SIP/epoch effects) and the architecture comparison (Astrometry.net k-d trees, TETRA exponential hashing, ASTAP dynamic hash, Pyramid) are **already recorded in §15** and are not repeated here.

### 17.1 Root-cause diagnosis of the 0/14 corpus result

Five interacting defects explain the 2026-11-19 regression: (1) stretched/tone-mapped JPEGs yield noise blobs and bloated centroids without detrending and HFD/FWHM gating; (2) global `G ≤ 10` cuts starve narrow fields and saturate Milky Way tiles unevenly, unlike ASTAP's density caps (D05/D20/D50/D80 = 500/2,000/5,000/8,000 stars/deg², down to mag 21 in sparse regions); (3) fixed linear descriptor bins (0.05/0.02) lose correct quads at bin edges and create collision storms (9–24 votes for `mag10`, 39–76 for `mag10deep`); (4) unconstrained all-band scans plus single-tile Gaia starvation (an `nside=64` tile covers ~0.84 deg², ~30 stars for a 7°×5° frame); (5) vote counts plus a single similarity fit cannot separate true alignments from collisions, and loose thresholds produced a false 0.26 arcsec/px, 17 px RMS fit that the new sanity guards reject.

### 17.2 Four-stage blueprint (new acceptance material)

1. **Detrended extraction and scale pre-estimate**: 2×/4× downsampling above ~1280×960 px (or >3,000 px height), tiled BlockMedian/LineMedian or `--bg-asymmetric`, HFD hot-pixel floor (ASTAP 1.5 arcsec solver / 0.8 alignment), eccentricity rejection of nebular ridges, flux-ranked 100–500 clean centroids, and nearest-neighbour pixel spacing as a band hint.
2. **Density-uniform multi-scale index**: nested FOV tiers stepped ~1.4–2×, `nside=1/2/4` with overlap margins for wide fields and `nside=16–64` for sub-degree fields, per-cell brightest-star quotas (~500–5,000 stars/deg²) instead of magnitude cuts, and Gaia DR4/Archive v3.15 compatibility (blackout 25 Nov–2 Dec 2026, anonymous sync limits 60 s / 50,000 rows / ~100 connections, ≤4 concurrent TAP workers with Retry-After).
3. **Redundant quads and error-optimal lookup**: 5–7 nearest bright neighbours per anchor with overlapping 4-star subsets, and either TETRA exponential bins in a flat open-addressing hash table (quadratic probing, constant-time lookup) or a flat binary 4-D k-d tree; sorted five-distance ratios plus the longest edge avoid dual-parity queries.
4. **Scale-consistent RANSAC and Bayesian acceptance**: optimistic quad verification as hits arrive, implied arcsec/px pruning, median scale-ratio outlier filtering, radius-based multi-tile Gaia gathering, RANSAC similarity seeding with iterative sigma-clipped SVD/6-constant refinement, and Astrometry.net-style foreground-vs-random log-odds with ≥6 inliers (or ≥3 corroborating quads), RMS ≤5 px and order-2 SIP when coverage permits.

### 17.3 Roadmap candidates promoted

Four items were added to `ROADMAP.md`, ordered by effort: extraction/seeded baseline (78), fast RANSAC + log-odds verifier (80), binary density-uniform index (79) and sub-degree/DR4 tooling (81). Their engineering deliverables are implemented in `blind_engine.go` and `blind_index.go` (SBI1 binary format with proper-motion records, density caps, exponential keys, Bayesian acceptance), but the corpus acceptance criteria are still unmet (0/14), so they are recorded as executed/not accepted; item 73 remains the umbrella R&D item.

**New source index (deduplicated against §15):** `lib.rs/crates/tetra3` (Rust Tetra3), `astrokeith.com/blogs/latest-blogs/plate-solvers-compared.html` and the AstroKeith latest-blogs feed (practitioner solve-time comparisons), `cloudynights.com/forums/topic/927634-introducing-the-cedar-e-finder/` (Cedar eFinder), `groups.io/g/eFinder/topics` (user-group latency reports), `mdpi.com/1424-8220/21/9/3084` (multilayer voting star identification), `researchgate.net/publication/26584290_A_Survey_on_Star_Identification_Algorithms` (Pyramid/Grid survey), plus the already-indexed TETRA paper, ASTAP pages and Astrometry.net README/index pages. Each latency figure remains a developer or field report, not a standardised benchmark.

## 18. Full corpus merge (2026-11-20, append-only)

**Provenance.** `research/pendingToAnalyse/corpus-skystacker-research-all.txt` (779 KB, 100 URL blocks) was a merged export of earlier research cycles. It was deduplicated against §9–§11 of [08](08-crawler-source-discovery.md) and §15–§17 of this file; 100 URLs collapsed to a small set of genuinely new facts below. The export was removed and `research/pendingToAnalyse/` kept with `.gitkeep`.

### 18.1 New survey-cutout and catalogue facts

- **Pan-STARRS PS1 caveats** (`archive.stsci.edu/panstarrs/`, `outerspace.stsci.edu/display/PANSTARRS/`): DR2 (>1.6 PB) uses a 1.8 m telescope and 1.4 Gpix GPC1 camera with grizy filters; ~0.016% of the sky has no sources. Positional errors reach ~0.5 arcsec above Dec 85°, which can leave a wrong WCS and duplicated objects in stacks. FITS headers use obsolete `PC001001/PC001002` keywords, scaled `asinh` flows, lack `RADESYS` (DS9 v8 may assume B1950; add `RADESYS='FK5'`) and lack `TIMESYS='TAI'` (matters at 30 s precision). Catalogue quantities are more precise than header quantities. This qualifies (does not invalidate) the survey-cutout reference in items 48/62.
- **2MASS / Tycho-2 / tetra3**: 2MASS (UMass+IPAC, Skrutskie 2006) offers Atlas/Catalog/Bulk/Large Galaxy Atlas and 6×6° mosaics; Tycho-2 has 2.5M stars with proper motions and two colours (Høg 2000, A&A 357 367); tetra3's default DB covers 10–30° to mag 7 and reports ~10 ms / 10 arcsec (50 µrad) solves for ≥512×512, ~10° fields using Yale BSC/Hipparcos/Tycho catalogues (Apache-2.0, ESA).

### 18.2 Astrometric fitting details (PI ImageSolver)

Convergence requires shift <0.005 px **and** stable RMS within 0.01 px; stop after 4 non-improving iterations or 100 maximum; default smoothing 0.005 px. Thin-plate spline distortion uses 1000–50,000 control points (default 4000, standard max 2100), spline order 2–6 (recommended 3–4) and recursive partition-of-unity (C²). Homography = 8 parameters from 4 star pairs. VizieR returns ≤200,000 rows (silent truncation); magnitude limit is bisected to ~1.25× the detected star count; the pointing seed must be inside the image and resolution within a factor ~2; initial matching uses the 2000 brightest stars with mirror and apparent-coordinate retries; sparsity-based outlier rejection uses Rousseeuw–Croux robust scale. Measured examples: narrow field 26,517 detections / 25,826 valid PSFs, mag limit 19.80 in 9 queries, initial RMS 0.747 px → 0.22 px in 6 iterations, 17 s; wide field 6064×4040 at 51.7 arcsec/px, 37,261 stars, mag 9.21, 9 iterations, 53 s; healthy match fraction 50–80%; diurnal aberration ~0.32 arcsec at the equator.

### 18.3 Annotation and photometry details

- **PI AnnotateImage**: 88 IAU constellations; duplicate suppression <3 arcsec for point sources and 10 arcsec for extended pairs; XEPH uses JPL DE440 plus 343 asteroids/30 KBO perturbers with Chebyshev coefficients, first 750,000 asteroids in 15 files of ~50,000 objects (~90 MB), coverage 2024-01-01→2027-04-15, 749,629 objects read in 35 s on 192 logical CPUs; topocentric-geocentric offsets reach 1° for the Moon/near asteroids; DE440 valid 1550–2650 with ≥5 days between epochs; asteroid magnitude from H,G or M1,K1.
- **Siril SPCC**: `xp_sampled` only for sources brighter than mag 17.6; DSLR sensor curves already include the stock LPF, so selecting an OEM LPF applies it twice and underestimates Hα by ~2.6×; narrow bands 3–35 nm; white reference = Average Spiral Galaxy (SWIRE templates), Sun = G2(v); atmospheric correction models only Rayleigh λ⁻⁴ (aerosols, Chappuis O₃ and Fraunhofer B not modelled); `SITEELEV` defaults 10 m, pressure 1013.25 hPa, airmass from `AIRMASS`→`CENTALT`→mean zenith angle of regions >30°; default demosaic is RCD.
- **SEP photometry**: `fluxerr` is zero unless `err`/`var` (per-pixel noise) or `gain` (Poisson term) is supplied; annulus masked pixels are ignored, annulus radii may be arrays and background error is propagated; `MASK_TYPE=CORRECT` fills masked pixels by reflection; negative `seg_id` restricts measurement to one segment; matched filtering uses per-pixel noise weights (unlike SExtractor).
- **pyvo/astroquery**: TAP sync vs async (async queued and more robust; `maxrec` truncates silently unless warned), 10 s default async job-poll timeout, Datalink/SODA; IRSA SIAv2/SSA and `list_collections`; SDSS default DR17, rectangular `width`/`height` searches without cosDec correction and full-plate image downloads.
- **APP beta47/48 (2026-10-03/09)**: Adaptive Deep Sky Stretch preserves colour from background level and signal width, colour-preserving general levels stretch, rewritten RGB Combine with corrected luminance injection, PNG added to the 2.0.0 format list.
- **LSST IsrTask**: segmentable overscan for discontinuous amplifier responses; saturation detection must precede `assembleCcd`; suspect pixels above `amp.getSuspectLevel()` (NaN disables).

### 18.4 Roadmap candidates promoted

Six new Open items were promoted to `ROADMAP.md` (see §19 for the star-catalog export item and diffuse-object classification): survey-cutout WCS conformance guard (82), cone-search truncation detection (83), airmass/Rayleigh extinction report (84), colour-preserving stretch (85), automatic PSF model selection (86) and full-solution verification against the catalogue (87). They are report-only or opt-in and do not duplicate items 16, 32, 50, 55, 57, 63, 69, 71 or 75. New URLs recorded here: Pan-STARRS archive pages, PI ImageSolver/AnnotateImage docs, Siril SPCC, APP tutorial/manual pages, LSST `IsrTask`, SEP `apertures`/`filter`, pyvo DAL, 2MASS and the Astropy FITS/NDData/WCS API pages (reference-only).

## 19. Star-catalog export and diffuse-object classification (2026-11-20, append-only)

### 19.1 Delivered: flux-sorted star list

SkyStacker's `detectStars` already returns centroids sorted by decreasing integrated flux, but the only public per-image export was the aggregate `image_measurements.json` (star count, median HFR/eccentricity, SNR, quality). The internal star cache (`_work/cache/stars_s<N>_<hash>.json`, `--keep-cache`) contains the full list but is keyed by a path hash and deleted by default. Roadmap item 88 adds a first-class `--export-stars FILE` path for standalone image mode: a `starCatalog` JSON object (`image`, `width`, `height`, `detection`, `count`, `stars[]`) or CSV with `x_px,y_px,flux,size_px,eccentricity,hfr_px`, always sorted brightest-first. No stacking or plate solving is needed; coordinates are 0-based pixels and flux is background-subtracted high-pass integrated intensity.

### 19.2 Standards for object classification (research for item 89)

- **Star/galaxy separation**: SExtractor's `CLASS_STAR` is a multilayer perceptron fed by eight exponentially spaced isophotal areas, peak surface brightness and a seeing input; values near 1 are point sources, near 0 extended objects. The docs state it is **superseded by `SPREAD_MODEL`**, which compares the PSF fit against a PSF broadened by a fixed factor and uses the full variable PSF model. SDSS DR17 uses a simple, reproducible criterion: an object is extended when `psfMag - cmodelMag > 0.145`, with numeric type codes `UNK=0, CR=1, DEFECT=2, GALAXY=3, GHOST=4, KNOWNOBJ=5, STAR=6, TRAIL=7, SKY=8`.
- **Non-parametric morphology**: CAS = Concentration `C = 5·log10(R80/R20)`, Asymmetry `A = Σ|I − I_rot180| / ΣI − A_bkg` and Smoothness/Clumpiness `S`. Gini measures flux inequality over object pixels (0 = uniform, 1 = one pixel); `M20 = log10(Σ_{brightest 20%} M_i / M_tot)` with `M_i = I_i[(x_i−x_c)²+(y_i−y_c)²]`. These four are the standard objective descriptors used by Lotz et al. (2004), Conselice (2003) and modern ML classifiers.
- **Probabilistic foundation**: Slater, Ivezić & Lupton (AJ 159:65, 2020) show that common separators (SDSS, SExtractor `CLASS_STAR`, `SPREAD_MODEL`) are monotonic transforms of a Bayesian likelihood odds ratio; seeing and depth trade off (~0.4 mag deeper compensates 10% worse seeing). Multiple measurements (bands, exposures, morphology+colour) should be combined probabilistically; no single cutoff is universally correct.
- **IVOA vocabularies**: UCD1+ provides `src.class`, `src.class.starGalaxy` (stellarity index), `src.morph` and `src.morph.param` for table-column semantics. The IVOA `object-type` RDF/SKOS vocabulary (draft, used by SIMBAD and ObsCore `target_class`) supplies hierarchical terms such as `star`, `galaxy`, `agn`→`seyfert` etc.; NED publishes a parallel flat object-type list (`G`, `Neb`, `HII`, `PN`, `SNR`, `QSO`, ...). A standards-compliant export should use IVOA terms in a `classification` field and UCDs in the metadata, rather than ad-hoc strings.
- **ML approaches**: XGBoost on CAS+Gini+M20+colours (MNRAS 2025), CNNs trained on Galaxy Zoo labels and 2024–26 unsupervised multimodal autoencoders give high accuracy but require external training data, large labelled sets and non-stdlib runtimes. They are **out of scope** for the static binary; the feasible native path is explicit morphology metrics plus catalogue cross-match (`--cone-catalog`, item 69), with optional external classifiers as sidecars.
- **Amateur-pipeline fit**: `detectStars` intentionally rejects extended blobs (component aspect/eccentricity filters), so a diffuse-object stage must work on multiscale background residuals after star masking, emitting report-only masks/metrics rather than feeding the registration matcher.

### 19.3 Roadmap candidates promoted

Item 88 (star-catalog export) is **Done**: a clean, flux-sorted JSON/CSV per-image export. Item 89 is **Done** (2026-11-20): `--classify-objects FILE` performs report-only diffuse-object detection on star-masked multiscale residuals, computing concentration/asymmetry/smoothness/Gini/M20/ellipticity and mapping the result onto IVOA `object-type` terms with `src.class`/`src.morph.param` UCD metadata. `TestDiffuseObjectClassification` uses synthetic star/galaxy/nebula scenes and verifies 100% point-vs-extended separation plus correct galaxy/nebula labels; a real M42 frame yields a conservative report with many `unknown` components. Explicitly not promoted: embedded ML classifiers (rejected by the stdlib-only constraint), Galaxy-Zoo-style training, and any modification of the scientific stack pixels. New URLs recorded: `sextractor.readthedocs.io/en/latest/ClassStar.html`, `sdss4.org/dr17/algorithms/classify`, `ivoa.net/rdf` (`object-type`, UCD), `ned.ipac.caltech.edu` object-type list, Lotz 2004 / Conselice 2003 morphology definitions via the 2025–26 MNRAS/Scientific Reports papers.

## 20. Additional-sources merge (2026-11-20, append-only)

**Provenance.** `research/pendingToAnalyse/skystacker-research-additional-sources.md` (21 KB, ~59 new URLs) was reviewed against §9–§11 of [08](08-crawler-source-discovery.md) and §15–§19 of this file. The export was removed and `research/pendingToAnalyse/` kept with `.gitkeep`. Forum items are community opinion; 2026 arXiv IDs were only seen as search snippets and are **not** treated as established facts.

### 20.1 Astrometry and solver options

- **solve-field option reference** (Ubuntu noble / Debian testing / FreeBSD man pages, helpmanual v0.72): `--scale-low/--scale-high`, `--downsample`, `--sigma`, `--tweak-order`, `--crpix-center`, `--odds-to-solve`, `--ra/--dec/--radius`, plus newer `--no-verify-uniformize`, `--no-verify-dedup`, `--cpulimit`, `--axy`, `--just-augment`, `--no-background-subtraction` and `--guess-scale`. Our local adapter currently passes only `--no-plots --overwrite --skip-solved -D --wcs` and optional scale bounds.
- **Siril Gaia DR3 astrometric extract** (Zenodo record 14692304): the even-density `localgaia` catalogue behind Siril's plate solving, also usable for colour calibration; complements the Gaia tile cache with a downloadable pre-built reference set.
- **StellarSolver / Ekos**: StellarSolver combines Astrometry.net with SEP and needs no `astrometry.cfg`; KStars Ekos exposes Internal StellarSolver, Local ASTAP, Watney and online Astrometry.net profiles with a "Builtin" extraction method. Confirms the profile/fallback architecture used in `--plate-solver auto`.
- **SCAMP/SExtractor/SWarp practice**: required `FITS_LDAC` columns, `.head` output and TPV distortion hand-off; DES/BCS/KiDS papers give copyable parameter sets (`FGROUP_RADIUS`, `ASTREF_CATALOG`, `DETECT_THRESH`, `INTERP_TYPE`), polynomial degrees 2–5 and ~0.01 arcsec offsets against USNO-B. TPV is the natural export target complementing the shipped SIP→PV helper (item 67), but full SCAMP-style calibration remains out of scope.

### 20.2 Formats, drizzle and weighting

- **Astro-TIFF 1.0** stores a full FITS WCS header in TIFF tag 270 (`ImageDescription`); sample files contain plate solutions and `BAYERPAT`. Originated among ASTAP, CCDciel, N.I.N.A. and Siril developers (Siril 1.0.0, 2022-03-09). SkyStacker's TIFF writer currently emits an uncompressed little-endian float32 RGB strip with no WCS header.
- **Drizzle practice**: Siril recommends scale=1, pixfrac=1 for CFA data; APP's developer guidance says Bayer drizzle needs roughly double the droplet size of normal drizzle for equal noise; PixInsight practitioners report ~15–20 well-dithered frames at scale 1 and 50+ at scale 2; STScI notes ignoring weight maps costs ~10% S/N and PSF FWHM varies ±5%. These are refinements of deferred item 14 (CFA drizzle) and item 66 (coverage validator), not new solver work.
- **Frame weighting**: SubframeSelector formulas normalise `FWHM`, `Eccentricity` and `SNRWeight` by their Min/Max and combine them with a floor (40–100%); a separate "approval expression" decides keep/drop. `SSWEIGHT`/FITS-keyword export is the same mechanism our item 51 uses. Practitioner reports that culling the worst 50% improves FWHM by only ~0.1 arcsec and that undersampling inflates eccentricity — useful calibration cautions.
- **Gaia ADQL**: official `EPOCH_PROP_POS` examples propagate proper motion server-side; `POINT`/`CIRCLE`/`CONTAINS` tutorials and `GaiaClass.cone_search_async/launch_job_async/load_data/get_datalinks` complement our cone client (item 69) and local propagation (item 76).

### 20.3 Background and photometry

- **GraXpert** exposes a CLI (`-cli`, `-correction Subtraction|Division`, `-smoothing`, `-bg`) and its RBF algorithm was adopted by Siril; Siril docs note Spline/Kriging do not generally beat Siril RBF. Our `--external-bge` runner already supports this pattern.
- **AAVSO DSLR/OSC photometry**: DSLR Observing Manual v1.4 ch.6 covers aperture photometry, comparison/check stars, RGB spectral response, extinction and transformation; forum guidance standardises `OBSTYPE`/`FILTER`/`TRANS`, green-channel use and linearity tests; transformation coefficients need non-crowded fields and multiple comparison stars. Relevant to items 28/70 and the deferred PCC/SPCC item 16.

### 20.4 Roadmap candidates promoted

Three new Open items were added, ordered by effort: external solver option tuning (90), Astro-TIFF WCS ingest/export (91) and report-only photometric transformation coefficients (92). Items 14, 66, 69, 76 and 51 were **not** reopened: their acceptance text absorbs the new drizzle/catalogue/weighting facts above. New URLs recorded here: Zenodo 14692304, solve-field man pages, StellarSolver/KStars Ekos, SCAMP "Using SCAMP" + DES/BCS/KiDS/SPLASH/GALACTICNUCLEUS papers, Astro-TIFF spec and Siril 1.0.0 notes, STScI drizzle combination notes, DrizzlePac Handbook v2/fruchter-ar5iv, AstroBin/IceInSpace weighting and drizzle threads, Gaia ADQL examples/Data Carpentry/astroquery GaiaClass, GraXpert and AAVSO DSLR/transformation documents.

## 21. VO services and WCS refinement (2026-11-20, append-only)

**Provenance.** The all-scope goCrawler export merged in [08](08-crawler-source-discovery.md) §12 added nine source URLs whose facts were not already indexed. Python-only tools (Ginga, astroquery itself) are references only — no new runtime dependency is proposed.

- **Async TAP (PyVO DAL).** Sync queries hold a connection for minutes and hit provider timeouts; async submission returns a job URL that can be polled and queued, with `submit_job()` for manual lifecycle control. `maxrec` defaults are conservative and must be overridden explicitly; `verbosity` values 1/2/3 are service-specific; TAP services can be discovered via the VO registry. Our `gaiaTAPQuery` (items 36/69) is synchronous with a 180 s timeout and a `MAXREC=2000` guard; item 94 will add opt-in async submission/polling plus truncation reporting, complementing item 83.
- **Iterative WCS refinement (`tweakwcs`).** Given an existing FITS WCS and a reference catalog, compute positional residuals and solve small corrections (shift/rotation/scale/distortion) rather than refitting from scratch. This fits between the seeded solver (item 58/69) and the per-frame reprojection blocked item 13; item 95 will expose a report-only refinement stage with before/after RMS and outlier rejection.
- **Archive clients.** MAST offers mission-specific queries plus a low-level portal API and token-based access; ESASky adds cone/region catalog, map/spectra and SSO crossmatch with 10k/100k row limits; IRSA MOST finds images covering predicted solar-system-object positions (2MASS/WISE/ZTF). These are new opt-in sources for items 48/62/69, grouped as item 96.
- **Space motion.** Astropy `apply_space_motion` propagates proper motion and RV assuming straight-line constant velocity, RV=0 when absent. This confirms item 76's approach; no new item is opened for radial-velocity terms.
- **VOTable metadata (→ [01](01-architecture-and-core-engine.md)).** `BINARY2` masks integer/bit-field columns that legacy `BINARY` cannot; preserving `PARAM`/`INFO` requires serialising an existing VOTable rather than re-deriving it from a plain table. Bit-field DQ arrays become boolean masks only after applying ignore flags, matching the contributor-mask semantics of item 85.

## 22. Survey and cone guards delivered (2026-11-20, append-only)

Items 82 and 83 close the report-only caveats from §18. `--check-survey-fits FILE` reads the full primary FITS header and emits a `survey_wcs_flags` object for legacy `PC001001/PC001002`, missing `RADESYS`/`TIMESYS`, `asinh` stretching and Pan-STARRS Dec>85° risk; pixels are untouched.

VizieR now requests `-out.max=200000` and IRSA TAP sends `MAXREC=200000`/`TOP 200000`. `coneSearchDetailed` parses `INFO` status, counts rows, records the effective covered radius/area from returned coordinates and marks truncation. `processingReport` carries `cone_truncated`, `cone_rows`, `cone_covered_radius_arcsec`, `cone_covered_area_deg2` and `cone_limit_reason`; annotation suppresses unassociated-source flags outside the returned coverage when truncated. Cache keys were bumped to `cone2_` so old payloads cannot hide truncation metadata. Mock VOTable and synthetic-FITS tests cover both paths.

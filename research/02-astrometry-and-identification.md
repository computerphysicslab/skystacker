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

No SCAMP successor was identified in the 2024–2026 material. Quad hashing for fast initial solves is studied under Cedar-Solve (esa/tetra3 lineage), **tetra3 Rust v0.14.0** and ASTAP's large D50 database; a maintainer-reported 5% ASTAP solver speedup appeared in Feb 2025 with star-poor-image enhancements through June 2026, while practitioner reports (Cloudy Nights, May 2025) note closed-source PlaneWave PlateSolve3 remains more reliable at 1600–2725 mm, star-sparse or narrowband fields — underscoring sensitivity to accurate pixel-scale hints.

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
- **Current state (v0.6.1):** the repository advanced to **v0.6.1 with 230 registered self-tests**, native pure-Go CR2 decoding, `--config`, `--resume`, deconvolution, narrowband and the goPureRaw subproject (details in [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) and [`README.md`](README.md)). The extended astrometric catalog-annotation and anomaly-flagging subsystem specified in §4 was implemented on 2026-11-18 (`--annotation-catalogs`, elliptical apertures, `catalog_matches.json`, SNR>5 unassociated-source gate; `testWCSCatalogOverlay` and `testUnassociatedSourceFlagging` are registered).
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
18. Möller, A. et al. (2021). *fink, a new generation of broker for the LSST community.* MNRAS, 501, 3272. [DOI](https://doi.org/10.1093/mnras/staa3602).
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

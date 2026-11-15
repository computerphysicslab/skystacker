# Astrometric Plate Solving, Astronomical Object Identification, and Sky-Chart Integration

**Scope:** Architectural and scientific research blueprint for building an independent **astrometric identification and astronomical scene-understanding subsystem** for SkyStacker.

---

## Executive Overview

Extending SkyStacker beyond conventional image stacking and plate solving requires developing an **astrometric identification and astronomical scene-understanding system**—an independent subsystem capable of taking an astrophotography image, determining precisely where it is pointing, identifying what celestial objects are present, rendering their identities and coordinates over the original image, and flagging sources that do not correspond to known catalogue entries.

### Six Core Research Pillars
1. **Astrometric plate solving:** Determine exact celestial coordinates, orientation, scale, and projection of an image.
2. **Image-to-sky-chart registration:** Transform astronomical catalogues and sky-chart data into the pixel coordinate system of the photograph.
3. **Astronomical object detection and classification:** Identify stars, galaxies, nebulae, quasars, clusters, and other deep-sky objects.
4. **Transient and moving-object detection:** Identify novae, supernovae, variable stars, asteroids, and comets.
5. **Anomaly and unknown-object detection:** Isolate sources absent from known catalogues, image artefacts, and potentially new astronomical phenomena.
6. **Scientific validation and cross-matching:** Associate detections with authoritative astronomical databases and quantify identification reliability.

> **Key Scientific Distinction:** *Detecting an object*, *classifying its morphology*, and *identifying it as a particular known astronomical object* are three distinct operations requiring different algorithms and scientific evidence. Similarly, SkyStacker's existing triangle/RANSAC registration aligns exposures relative to one another, whereas absolute astrometric calibration establishes celestial coordinates against an external reference frame.

---

## 1. Astrometric Plate Solving: Scientific Foundations

*(Highest priority for SkyStacker)*

Plate solving determines the transformation between image pixels $(x,y)$ and celestial coordinates $(\alpha,\delta)$, yielding:
* Right ascension and declination of the image centre.
* Image orientation relative to celestial north.
* Angular resolution in arcseconds per pixel and total field of view (FOV).
* Celestial coordinate transformations for every pixel, stored as a World Coordinate System (WCS) in FITS headers.

### 1.1 Essential Scientific Papers
1. **Astrometry.net: Blind Astrometric Calibration of Arbitrary Astronomical Images** — Dustin Lang, David W. Hogg, Keir Mierle, Michael Blanton & Sam Roweis (2010), *The Astronomical Journal*, 139, 1782–1800. DOI: [10.1088/0004-6256/139/5/1782](https://doi.org/10.1088/0004-6256/139/5/1782) · [arXiv:0910.2233](https://arxiv.org/abs/0910.2233). *(Foundational paper on geometric hashing of star asterisms, indexed catalogues, and Bayesian hypothesis testing without pointing metadata.)*
2. **Automatic Astrometric and Photometric Calibration with SCAMP** — Emmanuel Bertin (2006), ADASS XV. [PDF](https://articles.adsabs.harvard.edu/pdf/2006ASPC..351..112B). *(Multi-exposure astrometric and photometric calibration across stacked FITS datasets.)*
3. **Representations of celestial coordinates in FITS** — Calabretta & Greisen (2002), *Astronomy & Astrophysics*, 395, 1077–1122. DOI: [10.1051/0004-6361:20021327](https://doi.org/10.1051/0004-6361:20021327). *(Mathematical foundations for FITS projections, coordinate frames, and distortion conventions.)*
4. **SExtractor: Software for source extraction** — Bertin & Arnouts (1996), *Astronomy & Astrophysics Supplement Series*, 117, 393–404. DOI: [10.1051/aas:1996164](https://doi.org/10.1051/aas:1996164). *(Detecting astronomical sources, extracting positions/shapes, and separating point-like from extended sources.)*

### 1.2 Core Plate-Solving Algorithms

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

**Architectural Recommendation:** Do not build a blind solver from scratch initially. Study **Astrometry.net** and **ASTAP**, implement CLI adapters for them first, and subsequently build a native solver that leverages SkyStacker's existing star-detection and registration pipeline.

### 1.3 Reference Implementations to Study
* **Astrometry.net:** Blind plate solving and WCS generation ([Website](https://astrometry.net/) · [Source](https://github.com/dstndstn/astrometry.net)).
* **ASTAP:** Fast local plate solving ([Website](https://www.hnsky.org/astap.htm) · [Source](https://github.com/astap/astap)).
* **SCAMP:** Multi-exposure astrometric calibration ([Website](https://www.astromatic.net/software/scamp/)).
* **Siril:** Reference implementation for solving an image, retaining WCS headers, querying catalogues, and drawing annotations ([Website](https://siril.org/) · [Source](https://github.com/AstroSharp/siril) · [Astrometry Docs](https://siril.readthedocs.io/en/stable/astrometry/)).

---

## 2. Mapping an Astronomical Image with a Sky Chart

*(Highest priority for the visualization engine)*

### 2.1 Mathematical Transformation & FITS WCS Keywords
The bidirectional mapping between pixel space $(x,y)$ and equatorial coordinates $(\alpha,\delta)$ is:

$$(x,y) \longleftrightarrow (\alpha,\delta), \qquad (x_{\mathrm{object}}, y_{\mathrm{object}}) = \operatorname{WCS}^{-1}(\alpha,\delta)$$

In FITS headers, WCS metadata is stored across keywords such as:
* `CRPIX1`, `CRPIX2`: Reference pixel coordinates.
* `CRVAL1`, `CRVAL2`: Reference celestial coordinates (RA/Dec).
* `CTYPE1`, `CTYPE2`: Coordinate system and projection type (e.g., `RA---TAN`, `DEC--TAN`).
* `CDELT1`, `CDELT2`: Pixel scale in simple representations.
* `CD1_1`, `CD1_2`, `CD2_1`, `CD2_2`: Linear coordinate transformation matrix.
* `CROTA2`: Legacy rotation representation.
* `A_ORDER`, `B_ORDER` (plus `A_p_q`, `B_p_q`): Polynomial distortion coefficients in SIP conventions.

**References:**
* Calabretta & Greisen (2002) — FITS WCS conventions ([DOI](https://doi.org/10.1051/0004-6361:20021327)).
* **WCSLIB:** Established C implementation of FITS celestial transformations ([Docs](https://www.atnf.csiro.au/computing/software/wcs/)).
* **Astropy WCS:** Reference implementation and test oracle for validating Go WCS routines ([Docs](https://docs.astropy.org/en/stable/wcs/)).
* **Siril Annotations:** Practical catalogue rendering over WCS-calibrated images ([Docs](https://siril.readthedocs.io/en/stable/astrometry/annotations.html)).

### 2.2 Multi-Layer Annotation Engine Design
Annotations must be decoupled from the underlying astronomical pixels so users can toggle layers independently, adjust opacity, inspect metadata, and export annotated previews non-destructively:

| Layer | Visual Representation |
| :--- | :--- |
| **Stars** | Small circles, coloured by spectral or photometric information |
| **Double stars** | Paired markers and separation lines |
| **Galaxies** | Ellipses indicating position angle and angular dimensions |
| **Nebulae** | Outlines or translucent polygons |
| **Globular clusters** | Circles indicating apparent extent |
| **Open clusters** | Star-membership markers or cluster boundaries |
| **Quasars** | Special symbols and catalogue identifiers |
| **Supernovae** | Distinct transient markers |
| **Novae** | Transient markers with discovery dates |
| **Comets** | Position markers, direction, and optional trajectory |
| **Asteroids** | Position markers and motion vectors |
| **Unknown sources** | Highlighted detections with no catalogue match |
| **Celestial grid** | RA/Dec coordinate lines |
| **Constellations** | Boundaries, lines, and names |

---

## 3. Astronomical Catalogues & Cross-Identification

### 3.1 Core Astronomical Databases
* **Gaia DR3 (Essential):** Principal reference for stellar astrometry (positions, parallaxes, proper motions, magnitudes, colours, astrophysical parameters, variable stars, and extragalactic candidates). [Archive](https://gea.esac.esa.int/archive/) · [DR3 Docs](https://gea.esac.esa.int/archive/documentation/GDR3/) · [Overview Paper](https://doi.org/10.1051/0004-6361/202243940).
* **SIMBAD (Essential):** General-purpose database beyond the Solar System for resolving object names/aliases, object types, and bibliographic references. [Website](https://simbad.cds.unistra.fr/) · [Guide](https://simbad.cds.unistra.fr/guide/) · Wenger et al. (2000), *A&AS*, 143, 9.
* **VizieR (Essential):** Access to thousands of published specialized catalogues (galaxies, quasars, nebulae, variables). [Website](https://vizier.cds.unistra.fr/) · [DOI](https://doi.org/10.26093/cds/vizier) · Ochsenbein et al. (2000), *A&AS*, 143, 23–32.
* **NASA/IPAC Extragalactic Database (NED):** Essential for galaxies, galaxy clusters, AGNs, quasars, and redshifts. [Website](https://ned.ipac.caltech.edu/) · [References](https://ned.ipac.caltech.edu/Documents/References/NEDpapers).
* **Sloan Digital Sky Survey (SDSS):** Spectroscopic and photometric validation for galaxies, quasars, and stars. [Website](https://www.sdss.org/) · [SkyServer](https://skyserver.sdss.org/).
* **AAVSO VSX:** Variable Star Index for known variables, suspected variables, and nova candidates. [Website](https://www.aavso.org/vsx/) · [VizieR Record](https://cdsarc.cds.unistra.fr/viz-bin/cat/B/vsx).

### 3.2 Multi-Provider Catalogue Routing Matrix

The catalogue subsystem should support pluggable providers rather than hard-coding a single database:

| Object Class | Primary Sources | Additional / Secondary Sources |
| :--- | :--- | :--- |
| **Stars** | Gaia DR3 | Tycho-2, UCAC, APASS |
| **Double stars** | WDS | Gaia DR3 |
| **Variable stars** | AAVSO VSX, Gaia DR3 | GCVS |
| **Galaxies** | NED, HyperLEDA | SDSS, VizieR |
| **Quasars** | Gaia DR3, SDSS | Milliquas |
| **AGN** | SIMBAD, NED | SDSS |
| **Nebulae** | SIMBAD, VizieR | Sharpless, Lynds |
| **Planetary nebulae** | HASH, SIMBAD | VizieR |
| **Globular clusters** | Harris catalogue, SIMBAD | VizieR |
| **Open clusters** | Gaia DR3, SIMBAD | MWSC |
| **Supernovae** | Transient Name Server (TNS) | SIMBAD |
| **Novae** | AAVSO VSX, SIMBAD | TNS (for relevant transient reports) |
| **Comets** | Minor Planet Center (MPC) | JPL Horizons |
| **Asteroids** | Minor Planet Center (MPC) | JPL Horizons |
| **Satellites** | CelesTrak | Space-Track |
| **Unknown objects** | Cross-match against all relevant sources | Historical image comparison |

### 3.3 Stellar & Extragalactic Classification Literature
1. *Machine learning-based photometric classification of galaxies, quasars, emission-line galaxies, and stars* (2024), *MNRAS*, 527, 4677. [DOI](https://doi.org/10.1093/mnras/stad3436) *(Random Forest on SDSS DR17 + AllWISE photometry).*
2. Clarke et al. (2020), *Identifying galaxies, quasars, and stars with machine learning*, *A&A*, 639, A84. [DOI](https://doi.org/10.1051/0004-6361/201936770) *(Classifies 111M SDSS sources).*
3. Gaia Collaboration (2023), *Gaia DR3: The extragalactic content*, *A&A*, 674, A41. [DOI](https://doi.org/10.1051/0004-6361/202243232).
4. Fraix-Burnet (2023), *Machine learning and galaxy morphology: for what purpose?*, *MNRAS*, 523, 3974. [DOI](https://doi.org/10.1093/mnras/stad1654).

---

## 4. Pixel-Level Morphology & Deep-Sky Object Extraction

Catalogue coordinates alone do not describe an object's visible extent or shape in a specific photograph. SkyStacker should extract visible boundaries, centroid, major/minor axes, ellipticity, position angle, integrated flux, surface brightness, and point-vs-extended classification directly from pixels.

### 4.1 Extraction & Morphological Metrics
* **Source Extraction Tools:** SExtractor (Bertin & Arnouts 1996), DAOStarFinder (Stetson 1987), SEP (Python Source Extractor), and **Photutils** ([Docs](https://photutils.readthedocs.io/) · [Detection Guide](https://docs.astropy.org/projects/photutils/en/stable/user_guide/detection.html)), which cleanly separates point-source detection from general image segmentation.
* **Morphological Measurements:**

| Measurement | Physical / Structural Meaning |
| :--- | :--- |
| **FWHM** | Apparent source width |
| **Ellipticity / Eccentricity** | Degree of elongation and shape characteristics |
| **Concentration index** | Central light concentration |
| **Gini coefficient** | Distribution of pixel intensities |
| **$M_{20}$** | Spatial distribution of the brightest regions |
| **Sérsic index** | Approximate light-profile structure |
| **Image Moments & Radial Profile** | Orientation, shape, and radial brightness distribution |

* **Machine Learning & Domain Shift:** Datasets like **Galaxy Zoo 1** (Lintott et al. 2011, *MNRAS*, 410, 166; [DOI](https://doi.org/10.1111/j.1365-2966.2010.17432.x)) provide ~900,000 morphological classifications, though survey-trained CNNs/ViTs must be adapted carefully to stretched, noisy, undersampled, or narrowband amateur astrophotographs.

### 4.2 Uncatalogued Extended-Object Detection Workflow
1. Detect an extended source in pixel space.
2. Measure centroid, dimensions, and ellipticity.
3. Verify distinguishability from the local stellar PSF.
4. Query catalogues for a counterpart.
5. Compare measured morphology against available classifications and report candidates with confidence estimates (enabling detection of faint PGC galaxies absent from basic Messier/NGC/IC overlays).

---

## 5. Time-Domain Astronomy: Transients & Moving Objects

### 5.1 Proper Image Subtraction (ZOGY)
Detecting novae, supernovae, or variable outbursts requires temporal difference imaging:

$$D(x,y) = I_{\mathrm{new}}(x,y) - I_{\mathrm{reference}}(x,y)$$

Because raw subtraction fails under seeing or transparency changes, images must be astrometrically aligned, photometrically normalized, and PSF-matched using the **ZOGY algorithm**:
* **Zackay, Ofek & Gal-Yam (2016)**, *Proper Image Subtraction—Optimal Transient Detection, Photometry, and Hypothesis Testing*, *ApJ*, 830, 27. [DOI](https://doi.org/10.3847/0004-637X/830/1/27) · [arXiv:1601.02655](https://arxiv.org/abs/1601.02655).

### 5.2 Transient & Artifact Detection Strategies

| Phenomenon | Detection Strategy |
| :--- | :--- |
| **Supernova** | New point source, often near a host galaxy |
| **Nova** | New or brightened stellar source |
| **Variable star** | Photometric change across multiple epochs |
| **Microlensing** | Characteristic symmetric brightness evolution |
| **Asteroid** | Linear motion between sequential exposures |
| **Comet** | Motion between exposures plus extended coma/tail |
| **Satellite** | Linear trail or rapid sequential displacement |
| **Cosmic ray** | Single-exposure transient inconsistent with local PSF |
| **Optical reflection** | Ghost geometry relative to bright stars |
| **Sensor artefact** | Recurrence at fixed detector $(x,y)$ coordinates |

* **Transient Brokers & Databases:** **Transient Name Server (TNS)** ([Website](https://www.wis-tns.org/); note Galactic novae follow a separate reporting pathway), **Zwicky Transient Facility (ZTF)** ([Website](https://www.ztf.caltech.edu/)), and **Fink** ([Website](https://fink-broker.org/) · Möller et al. 2021 [DOI](https://doi.org/10.1093/mnras/staa3602) · Fraga et al. 2024 [DOI](https://doi.org/10.1051/0004-6361/202450370)).
* **Reporting Policy:** SkyStacker must flag detections as *unmatched transient candidates* rather than claiming confirmed astronomical discoveries.

### 5.3 Moving Objects: Comets & Asteroids
* **Reference Algorithm:** **A-Track** (2016), *A new approach for detection of moving objects in FITS images*, *Computer Physics Communications*, 207, 524–530 ([DOI](https://doi.org/10.1016/j.cpc.2016.07.023)), using a modified line-detection algorithm (MILD) across sequential FITS exposures.
* **Ephemeris & Astrometry Services:** **Minor Planet Center (MPC)** ([Website](https://minorplanetcenter.net/) · [Astrometry Guide](https://docs.minorplanetcenter.net/mpc-ops-docs/astrometry/) · [Observations API](https://docs.minorplanetcenter.net/tutorials/notebooks/mpc_tutorial_api_observations/) · [Orbits API](https://docs.minorplanetcenter.net/tutorials/notebooks/mpc_tutorial_api_orbits/)) and **JPL Horizons** ([Ephemeris Service](https://ssd.jpl.nasa.gov/horizons/)).
* **Sequential Pipeline:** Plate-solve each frame $\rightarrow$ record exact UTC/time-standard timestamps $\rightarrow$ detect sources per frame $\rightarrow$ transform to $(\alpha,\delta)$ $\rightarrow$ cross-match MPC/Horizons ephemerides $\rightarrow$ link consistent inter-frame motion vectors to estimate angular velocity and separate known bodies from unidentified moving candidates.

---

## 6. Unknown Sources & Anomaly Detection Pipeline

### 6.1 Uncatalogued vs. Anomalous
* **Uncatalogued:** A detected source has no counterpart in the queried catalogues (e.g., a faint uncatalogued background galaxy).
* **Anomalous:** A source whose measured photometric, morphological, or temporal characteristics deviate significantly from expected astronomical or instrumental populations (cf. **Gaia DR3 Apsis**, Creevey et al. 2023, *A&A*, 674, A26; [DOI](https://doi.org/10.1051/0004-6361/202243688)).

### 6.2 Six-Stage Anomaly Pipeline
1. **Calibrated Image Input:** Float32 stack with noise model and bad-pixel/saturation masks.
2. **Source Detection:** Point sources, extended profiles, trails, and irregular structures.
3. **Multi-Catalogue Cross-Matching:** Gaia DR3, SIMBAD, VizieR, NED, VSX, MPC. *(Matched sources $\rightarrow$ annotate; Unmatched sources $\rightarrow$ continue.)*
4. **Anomaly Characterization:** Evaluate morphology, photometry, PSF consistency, local noise, and inter-frame motion/light curves (using Isolation Forest, LOF, DBSCAN/HDBSCAN, autoencoders, or ZOGY statistics).
5. **Candidate Classification:** Categorize as *possible transient*, *moving object*, *uncatalogued extended source*, *instrumental artefact*, or *unknown*.
6. **Scientific Report:** Output pixel coordinates $(x,y)$, $(\alpha,\delta)$, flux, uncertainty, supporting evidence, nearest catalogue matches, and confidence score.

---

## 7. Sky-Chart Interoperability & Virtual Observatory Standards

### 7.1 SkyChart / Cartes du Ciel vs. Aladin Sky Atlas
* **SkyChart / Cartes du Ciel** ([Website](https://www.ap-i.net/skychart/en/start) · [Pictures Docs](https://www.ap-i.net/skychart/en/documentation/pictures)): Can ingest FITS images containing valid WCS headers into its image archive and overlay them onto sky charts, though it is not a general-purpose per-image detection annotation engine. **Use SkyChart as an interoperability export target.**
* **Aladin Sky Atlas & HiPS** ([Aladin Desktop](https://aladin.cds.unistra.fr/AladinDesktop/) · [Aladin Lite](https://aladin.cds.unistra.fr/AladinLite/doc/) · Bonnarel et al. 2000 [DOI](https://doi.org/10.1051/aas:2000331) · Fernique et al. 2015 [DOI](https://doi.org/10.1051/0004-6361/201526075)): Supports multi-wavelength survey overlays, interactive SIMBAD/VizieR/NED queries, VOTable cross-matching, and **HiPS (Hierarchical Progressive Surveys)** multi-resolution HEALPix tiling. **Use Aladin/HiPS as the architectural reference for SkyStacker's interactive visualization experience.**

### 7.2 Virtual Observatory (IVOA) Standards

| Standard / Technology | Purpose | Documentation |
| :--- | :--- | :--- |
| **FITS** | Astronomical image and metadata format | [NASA FITS](https://fits.gsfc.nasa.gov/) |
| **WCS** | Pixel-to-sky coordinate transformations | [WCSLIB](https://www.atnf.csiro.au/computing/software/wcs/) |
| **SIP** | Polynomial image distortion conventions | [FITS WCS](https://fits.gsfc.nasa.gov/fits_wcs.html) |
| **VOTable** | Astronomical catalogue exchange | [IVOA VOTable](https://www.ivoa.net/documents/VOTable/) |
| **TAP / ADQL** | Querying astronomical databases | [IVOA TAP](https://www.ivoa.net/documents/TAP/) |
| **Cone Search** | Retrieving sources around coordinates | [IVOA ConeSearch](https://www.ivoa.net/documents/ConeSearch/) |
| **HiPS** | Multi-resolution progressive sky surveys | [CDS HiPS](https://aladin.cds.unistra.fr/hips/) |
| **MOC** | Multi-order sky coverage maps | [IVOA MOC](https://ivoa.net/documents/MOC/) |
| **ADES** | Minor-body astrometry data exchange | [MPC ADES](https://docs.minorplanetcenter.net/mpc-ops-docs/observations/) |

---

## 8. Proposed Go Subsystem Architecture & Data Model

To preserve SkyStacker's zero-dependency Go binary architecture (`CGO_ENABLED=0`), the production subsystem should be implemented natively in Go, wrapping external solvers (Astrometry.net/ASTAP) via clean adapters rather than introducing a Python runtime dependency.

### 8.1 Proposed Go Package Structure

| Package | Responsibilities |
| :--- | :--- |
| `astrometry` | Plate solving, WCS, coordinate transformations |
| `astrometry/catalog` | Local star catalogue indexing and HEALPix spatial queries |
| `astrometry/solver` | Astrometry.net, ASTAP adapters, and future native solver |
| `catalogues` | SIMBAD, Gaia DR3, VizieR, NED, VSX, MPC providers |
| `sources` | Detection, segmentation, sub-pixel centroiding |
| `classification` | Star, galaxy, nebula, and quasar classification |
| `morphology` | Shape, ellipticity, size, and surface brightness |
| `annotations` | Non-destructive rendering of labels, markers, ellipses, and contours |
| `transients` | Novae, supernovae, and variable source difference analysis |
| `movingobjects` | Comets, asteroids, and inter-frame motion tracking |
| `anomaly` | Unassociated source and instrumental artefact detection |
| `photometry` | Flux, magnitude, and uncertainty calculations |
| `skyview` | Interactive celestial visualization |
| `interoperability` | FITS, WCS, VOTable, SkyChart, and Aladin export |

### 8.2 Core Go Data Model

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

> **Schema Design Note:** Production structs should explicitly separate (1) a scientific catalogue entry, (2) a pixel-space image detection, (3) an association/cross-match hypothesis, and (4) a morphological classification, while retaining coordinate reference frame, observation epoch, photometric filter, units, and provenance.

---

## 9. Phased Implementation Roadmap & Validation Metrics

### 9.1 Six-Phase Development Sequence
1. **Phase 1 · Foundation (Astrometric calibration):** FITS WCS support, Astrometry.net & ASTAP adapters, WCS coordinate transforms, FOV calculation, and round-trip astrometric validation.
2. **Phase 2 · Identification (Catalogue cross-matching):** Gaia DR3, SIMBAD, VizieR, NED, Messier/NGC/IC catalogues, and offline local cache support.
3. **Phase 3 · Visualization (Astronomical annotations):** Toggleable object labels, star markers, galaxy ellipses, nebula boundaries, RA/Dec grids, interactive inspection, and SkyChart FITS+WCS exports.
4. **Phase 4 · Analysis (Morphological classification):** Extended-source detection, galaxy morphology, stellar PSF analysis, nebula segmentation, photometry, and quasar/galaxy candidate classification.
5. **Phase 5 · Time-domain astronomy (Transients & moving objects):** Reference-image subtraction (ZOGY), variable-star analysis, supernova/nova candidates, asteroid/comet tracking, and MPC/TNS cross-referencing.
6. **Phase 6 · Advanced research (Anomaly detection & discovery assistance):** Uncatalogued source detection, ML classification, unsupervised outlier detection, historical image comparisons, confidence estimation, and structured candidate reporting.

### 9.2 Benchmarking & Validation Suite

| Dataset / Resource | Validation Target |
| :--- | :--- |
| **Gaia DR3** | Astrometric accuracy and proper-motion epoch propagation |
| **SDSS DR17** | Star / galaxy / quasar classification precision & recall |
| **Galaxy Zoo** | Extended-source galaxy morphology metrics |
| **ZTF public data** | Transient detection completeness and false-positive rate |
| **AAVSO VSX** | Variable-star identification |
| **MPC observations** | Moving-object ephemeris matching and trajectory fitting |
| **Astrometry.net reference solutions** | Plate-solving reliability and solve rate |
| **Synthetic FITS images** | Known coordinate round-trip accuracy and injected transient recall |

**Key Metrics Tracked:** Astrometric residual (arcsec), solve success rate (%), false-positive/false-negative rates, cross-match precision/recall, centroid error (pixels), photometric error (mag), transient completeness, classification confusion matrices, and runtime RAM/CPU footprint.

---

## 10. Consolidated Academic Bibliography

### A. Astrometry and Plate Solving
1. Lang, D. et al. (2010). *Astrometry.net: Blind astrometric calibration of arbitrary astronomical images.* AJ, 139, 1782. [DOI](https://doi.org/10.1088/0004-6256/139/5/1782).
2. Bertin, E. (2006). *Automatic Astrometric and Photometric Calibration with SCAMP.* ADASS XV. [Paper](https://articles.adsabs.harvard.edu/pdf/2006ASPC..351..112B).
3. Calabretta, M. R. & Greisen, E. W. (2002). *Representations of celestial coordinates in FITS.* A&A, 395, 1077. [DOI](https://doi.org/10.1051/0004-6361:20021327).
4. Bertin, E. & Arnouts, S. (1996). *SExtractor: Software for source extraction.* A&AS, 117, 393. [DOI](https://doi.org/10.1051/aas:1996164).

### B. Astronomical Catalogues and Identification
5. Wenger, M. et al. (2000). *The SIMBAD astronomical database.* A&AS, 143, 9.
6. Ochsenbein, F., Bauer, P. & Marcout, J. (2000). *The VizieR database of astronomical catalogues.* A&AS, 143, 23. [DOI](https://doi.org/10.1051/aas:2000169).
7. Gaia Collaboration (2023). *Gaia DR3: Summary of the content and survey properties.* A&A, 674, A1. [DOI](https://doi.org/10.1051/0004-6361/202243940).
8. Gaia Collaboration (2023). *Gaia DR3: The extragalactic content.* A&A, 674, A41. [DOI](https://doi.org/10.1051/0004-6361/202243232).
9. Creevey, O. L. et al. (2023). *Gaia DR3: Astrophysical parameters inference system (Apsis). I. Methods and content overview.* A&A, 674, A26. [DOI](https://doi.org/10.1051/0004-6361/202243688).
10. Clarke, A. O. et al. (2020). *Identifying galaxies, quasars, and stars with machine learning.* A&A, 639, A84. [DOI](https://doi.org/10.1051/0004-6361/201936770).

### C. Classification and Morphology
11. Stetson, P. B. (1987). *DAOPHOT: A computer program for crowded-field stellar photometry.* PASP, 99, 191.
12. Lintott, C. et al. (2011). *Galaxy Zoo 1: data release of morphological classifications for nearly 900,000 galaxies.* MNRAS, 410, 166. [DOI](https://doi.org/10.1111/j.1365-2966.2010.17432.x).
13. Fraix-Burnet, D. (2023). *Machine learning and galaxy morphology: for what purpose?* MNRAS, 523, 3974. [DOI](https://doi.org/10.1093/mnras/stad1654).
14. *Machine learning-based photometric classification of galaxies, quasars, emission-line galaxies, and stars* (2024). MNRAS, 527, 4677. [DOI](https://doi.org/10.1093/mnras/stad3436).

### D. Transients and Moving Objects
15. Zackay, B., Ofek, E. O. & Gal-Yam, A. (2016). *Proper Image Subtraction—Optimal Transient Detection, Photometry, and Hypothesis Testing.* ApJ, 830, 27. [DOI](https://doi.org/10.3847/0004-637X/830/1/27).
16. *A-Track: A new approach for detection of moving objects in FITS images* (2016). Computer Physics Communications, 207, 524–530. [DOI](https://doi.org/10.1016/j.cpc.2016.07.023).
17. Möller, A. et al. (2021). *fink, a new generation of broker for the LSST community.* MNRAS, 501, 3272. [DOI](https://doi.org/10.1093/mnras/staa3602).
18. Fraga, B. M. O. et al. (2024). *Transient classifiers for Fink: Benchmarks for LSST.* A&A, 692, A208. [DOI](https://doi.org/10.1051/0004-6361/202450370).

### E. Astronomical Visualization and Interoperability
19. Bonnarel, F. et al. (2000). *The ALADIN interactive sky atlas: A reference tool for identification of astronomical sources.* A&AS, 143, 33. [DOI](https://doi.org/10.1051/aas:2000331).
20. Fernique, P. et al. (2015). *Hierarchical progressive surveys: Multi-resolution HEALPix data structures for astronomical images, catalogues, and 3-dimensional data cubes.* A&A, 578, A114. [DOI](https://doi.org/10.1051/0004-6361/201526075).

## Implementation status (v0.6.0)

In `main.go`: Gaia ADQL client, HEALPix nested queries, TAN WCS fit, and
`plate_solution.json` via `--plate-solve`. Not in v0.6.0: sky-chart
rendering, multi-catalogue annotation, and transient overlays. Those remain
research.

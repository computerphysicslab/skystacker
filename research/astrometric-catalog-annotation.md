# Technical Research Paper & Specification: Astrometric Catalog Annotation and Unassociated-Source Anomaly Flagging in Local Astrophotography Pipelines

## Abstract

This document provides a rigorous academic and technical foundation for integrating **Astrometric Catalog Annotation** and **Unassociated-Source Anomaly Flagging** into local-first astrophotography reduction engines (such as `SkyStacker`). Leveraging an established World Coordinate System (WCS) plate solution with Simple Imaging Polynomial (SIP) distortion coefficients, we formulate the end-to-end mathematical pipeline required to map celestial catalog entries (e.g., Gaia DR3, SIMBAD, NED, and PGC/Messier databases) onto detector image planes. Furthermore, we formalize a robust spatial cross-matching framework using $k$-d trees to decouple catalogued structures from unknown or transient astronomical phenomena (supernovae, novae, high-proper-motion stars, moving asteroids, and cosmic-ray/detector artifacts).

---

## 1. Introduction and Architectural Motivation

Modern local-first stacking tools successfully bridge raw multi-frame astrophotography data into high-dynamic-range (HDR) float32 products. However, bridging the gap between raw pixel integration and semantic astronomical discovery requires moving beyond star alignment into **astrometric intelligence**.

When an image is plate-solved—mapping pixel coordinates $(x, y)$ to celestial sphere coordinates $(\alpha, \delta)$—the resulting World Coordinate System (WCS) transformation opens two powerful capabilities:

1. **Catalog Annotation:** Projecting known deep-sky objects (galaxies, nebulae, clusters) and stars with correct morphological bounding geometries (elliptical apertures, orientation angles) and metadata labels directly onto output previews.
2. **Anomaly & Transient Discovery:** Performing blind source extraction on the stacked result, cross-referencing detections against all-sky catalog layers, and isolating unassociated sources that exhibit statistically significant flux ($> N\sigma$) without a catalogued counterpart.

---

## 2. Mathematical Formulation of WCS Projection & Distortion

### 2.1 Forward Projection: Celestial $(\alpha, \delta)$ to Intermediate World Coordinates $(\xi, \eta)$

Given a celestial target with Right Ascension $\alpha$ and Declination $\delta$, and a reference projection center $(\alpha_0, \delta_0)$, the intermediate world coordinates $(\xi, \eta)$ in degrees under a gnonomic (TAN) projection are given by:

$$\cos \phi = \sin \delta \sin \delta_0 + \cos \delta \cos \delta_0 \cos(\alpha - \alpha_0)$$

$$\xi = \frac{\cos \delta \sin(\alpha - \alpha_0)}{\cos \phi}, \quad \eta = \frac{\sin \delta \cos \delta_0 - \cos \delta \sin \delta_0 \cos(\alpha - \alpha_0)}{\cos \phi}$$

### 2.2 Application of SIP Optical Distortion

To account for optical distortions (field curvature, barrel/pincushion distortion, differential refraction), standard TAN projections are augmented with Simple Imaging Polynomial (SIP) coefficients $A_{p,q}$ and $B_{p,q}$ of order $N$ (typically $N=2$):

$$\xi' = \sum_{p=0}^{N} \sum_{q=0}^{N-1 - p} A_{p,q} \, \xi^p \, \eta^q$$

$$\eta' = \sum_{p=0}^{N} \sum_{q=0}^{N-1 - p} B_{p,q} \, \xi^p \, \eta^q$$

### 2.3 Linear CD Matrix Transformation to Pixel Space $(x, y)$

The distorted intermediate coordinates $(\xi', \eta')$ are converted to pixel coordinates via the linear CD matrix:

$$\begin{pmatrix} x - x_R \\ y - y_R \end{pmatrix} = \begin{pmatrix} CD_{11} & CD_{12} \\ CD_{21} & CD_{22} \end{pmatrix}^{-1} \begin{pmatrix} \xi' \\ \eta' \end{pmatrix}$$

where $(x_R, y_R)$ is the reference pixel coordinate corresponding to $(\alpha_0, \delta_0)$.

---

## 3. Astrometric Catalog Annotation Pipeline

### 3.1 Multi-Catalog Query and Spatial Bounding

To annotate an image, the pipeline queries local or remote database layers (e.g., Gaia DR3 for point sources; SIMBAD/HyperLeda/PGC/NGC for extended objects) bounded by the image's bounding polygon evaluated via HEALPix nested geometry:

$$\Omega_{\text{image}} = \Big\{ (\alpha, \delta) \mid x_{\min} \le x(\alpha, \delta) \le x_{\max}, \, y_{\min} \le y(\alpha, \delta) \le y_{\max} \Big\}$$

### 3.2 Elliptical Aperture Rendering for Extended Objects

Extended objects (galaxies, nebulae) are rendered using projected bounding ellipses defined by major axis $a$, minor axis $b$, and position angle $\theta$:

$$\frac{x'^2}{a^2} + \frac{y'^2}{b^2} \le 1$$

where $(x', y')$ are rotated by $-\theta$ relative to the pixel centroid. Labels include object designations (e.g., `NGC 5078`, `PGC 04661`), morphological types (`Sa-b`, `S0`), and apparent magnitudes ($V_{\text{mag}}$).

---

## 4. Unassociated-Source (Anomaly) Detection & Flagging

### 4.1 Blind Source Extraction

Independent of catalog priors, the stacked float32 image undergoes background subtraction and connected-component source extraction (or thresholded MAD-based star/object detection), yielding a set of measured sources $S_{\text{detected}} = \{ \langle x_i, y_i, F_i, \text{FWHM}_i \rangle \}$.

### 4.2 Spatial Cross-Matching via $k$-d Trees

To separate known objects from anomalies, $S_{\text{detected}}$ is mapped to celestial coordinates $(\alpha_i, \delta_i)$ via the inverse WCS transformation. A spatial $k$-d tree query matches detected sources against the projected reference catalog $C_{\text{ref}}$ within a configurable matching radius $r_{\text{match}}$ (e.g., $r_{\text{match}} = 2.5\text{ arcsec}$):

$$\min_{(\alpha_c, \delta_c) \in C_{\text{ref}}} \Delta \theta(\!( \alpha_i, \delta_i ), ( \alpha_c, \delta_c )\!) \le r_{\text{match}}$$

### 4.3 Anomaly Classification and Manifest Generation

Any detected source exceeding a signal-to-noise ratio threshold ($SNR > 5.0$) that fails to find a catalog counterpart within $r_{\text{match}}$ is flagged as an **unassociated source / astrometric anomaly**.

Potential astrophysical and artifact classifications include:

* **Transients:** Supernovae, novae, dwarf novae outbursts, or variable stars near peak brightness.
* **Solar System Objects:** Asteroids (minor planets) or comets leaving linear or curved motion trails across multi-exposure stacks.
* **High-Proper-Motion Stars:** Fast-moving stellar objects whose epochs differ significantly from catalog epochs.
* **Residual Artifacts:** Unclipped satellite trails, hot pixels, cosmic-ray hits, or optical ghosts.

Flagged anomalies are highlighted on preview outputs using high-contrast warning reticles (e.g., dashed yellow/magenta bounding boxes) and serialized into a structured JSON manifest (`anomalies.json`) containing centroid coordinates, estimated fluxes, and angular separation residuals.

---

## 5. Recommended Implementation Roadmap & Testing Suite

To integrate this specification seamlessly into `SkyStacker` or similar Go/Python architectures, the following modules and self-tests are proposed:

1. **`wcsCatalogOverlay` Module:** Implements WCS projection, SIP inverse polynomial evaluation, and bounding polygon clipping.
2. **`crossMatchKDTree` Module:** Executes fast spatial nearest-neighbor queries against local SQLite/JSON catalog caches.
3. **`detectAnomalies` Module:** Filters unassociated sources and generates annotation overlays and anomaly logs.
4. **Mandatory Unit Tests:**
* `testWCSCatalogOverlay`: Validates pixel-to-sky and sky-to-pixel round-trip accuracy within $0.05$ pixels.
* `testUnassociatedSourceFlagging`: Verifies correct identification and isolation of synthetic injected transient sources against a mock Gaia catalog.


# **Deep research: Astrometric plate solving, astronomical image identification and sky-chart integration for SkyStacker**

The subject you are describing goes considerably beyond conventional plate solving. It is essentially the development of an **astrometric identification and astronomical scene-understanding system**: a system capable of taking an astrophotography image, determining precisely where it is pointing, understanding what celestial objects are present, placing their identities and coordinates over the original image, and detecting objects that do not correspond to known catalogue entries.

For SkyStacker, I would approach this as a new, independent subsystem, rather than simply adding a plate-solving function to the stacking pipeline.

There are six major research areas:

1. **Astrometric plate solving:** determine the exact celestial coordinates, orientation, scale and projection of an image.
2. **Image-to-sky-chart registration:** transform astronomical catalogues and sky-chart data into the pixel coordinate system of the photograph.
3. **Astronomical object detection and classification:** identify stars, galaxies, nebulae, quasars, clusters and other deep-sky objects.
4. **Transient and moving-object detection:** identify novae, supernovae, variable stars, asteroids and comets.
5. **Anomaly and unknown-object detection:** identify sources that are not present in known catalogues, image artefacts and potentially new astronomical phenomena.
6. **Scientific validation and cross-matching:** associate detections with authoritative astronomical databases and quantify the reliability of each identification.

An important distinction throughout this research is that *detecting an object*, *classifying its morphology*, and *identifying it as a particular known astronomical object* are three different operations. They require different algorithms and different scientific evidence.

## **1. Astrometric plate solving: the scientific foundations**

Highest priority for SkyStacker

Plate solving is the process of determining the transformation between image pixels \\((x,y)\\) and celestial coordinates \\((\alpha,\delta)\\).

A successful solution provides:

- Right ascension and declination of the image centre.
- Image orientation relative to celestial north.
- Angular resolution in arcseconds per pixel.
- Field of view.
- Celestial coordinate transformations for every pixel.
- A World Coordinate System (WCS) description, normally stored in FITS headers.

### **1.1 Essential scientific papers**

[How Astrometry.net works (plate solving) - Discussions - Software - Stargazers Lounge](https://images.openai.com/static-rsc-4/3ZA7XCX56LtyeNLAynwsmNd3ME6LBkZnWjZO1OanPA_p-OmXYA5cpjX3tC344OzW75zcRB7CuFMFUUoO65Cuq3dNofUH5hhqSi-wqg_axG22HBvEEd5Xmnl1U-Zcswat3mdWf0AKsVBxswsKqEyp3I4eUKURrGUdK6Qen2KaH7I?purpose=inline)

**1. Astrometry.net: Blind Astrometric Calibration of Arbitrary Astronomical Images**

Dustin Lang, David W. Hogg, Keir Mierle, Michael Blanton & Sam Roweis (2010).

*The Astronomical Journal*, 139, 1782–1800.

DOI: [10.1088/0004-6256/139/5/1782](https://doi.org/10.1088/0004-6256/139/5/1782?utm_source=chatgpt.com).

FOUNDATIONAL PAPER

Explains blind plate solving using geometrical star patterns (asterisms), geometric hashing, indexed catalogues and Bayesian hypothesis testing. Particularly important when the image has no pointing metadata.\
[Free full paper on arXiv](https://arxiv.org/abs/0910.2233?utm_source=chatgpt.com).

[PPT - Astrometry @ TERAPIX: SCAMP PowerPoint Presentation, free download - ID:4628638](https://images.openai.com/static-rsc-4/CTmfGq2T2np_4MpmS_tbLdH-Vesu86RhAkJchmCBcI2nuEAhnRh0AhHAW1WdYuDPGCBVsvk03W1JjuWuOLto6EeZnBLbf1IpsMoeiEr6nMZ4N4pdytCvF_OiRpWxpMOA1meRJb0uDFytEYrdTQSd4Uc5T6MHIs8YP46-SwWhCm8?purpose=inline)

**2. Automatic Astrometric and Photometric Calibration with SCAMP**

Emmanuel Bertin (2006), ADASS XV.

Explains automatic astrometric and photometric calibration across multiple FITS exposures. Especially relevant to registering multiple images and maintaining consistent astrometry across a stacked dataset.

[Read paper](https://articles.adsabs.harvard.edu/pdf/2006ASPC..351..112B?utm_source=chatgpt.com).

[Making plots with world coordinates (WCSAxes) — Astropy v7.2.0](https://images.openai.com/static-rsc-4/A9gfFPQnmP6NAeQGpYuZpMtq3Q6USkhiCNQ5aXlZkUx_0AKaXRxxAnqC1wr2Xj6Znxae57DpJ3OZOiU2WrlN0GsopV358wLs5U3RIlqNYYzAfINDwy6MekLjg3GTxVXS8dwVQY5PMpwAEiuZexwXmjS-iVTQ-SY_2Ys0VKz5Xv0?purpose=inline)

**3. Representations of celestial coordinates in FITS**

Calabretta & Greisen (2002).

*Astronomy & Astrophysics*, 395, 1077–1122.

DOI: [10.1051/0004-6361:20021327](https://doi.org/10.1051/0004-6361:20021327?utm_source=chatgpt.com).

Essential reference for implementing the mathematical relationship between pixels and celestial coordinates, including projections, coordinate frames and distortion conventions.

[Astrometry.net](https://images.openai.com/static-rsc-4/wXw5PXFUJOkffKDfRZQdgeov5vNB-vHfZJBEzs82nqSyN1-dDVL2EzLra9iFoOCIPe6dQl5Fc5lUSM3WWH4Pj3CUekFYP64znWtnre8dIbkwRpvu7SfMVavW9C1Da4E849qGwJoV3IHRTfdWvlTZKmTPcgGWYpQfOKodD9CpB8c?purpose=inline)

**4. SExtractor: Software for source extraction**

Bertin & Arnouts (1996).

*Astronomy & Astrophysics Supplement Series*, 117, 393–404.

DOI: [10.1051/aas:1996164](https://doi.org/10.1051/aas:1996164?utm_source=chatgpt.com).

Fundamental for understanding how to detect astronomical sources, extract their positions, measure their shapes and distinguish point-like sources from extended objects.

### **1.2 Algorithms worth studying**

| Algorithm                | Purpose                                 | Relevance |
| ------------------------ | --------------------------------------- | --------- |
| Triangle matching        | Match geometric configurations of stars | High      |
| Quad matching            | Identify four-star patterns             | Very high |
| Geometric hashing        | Search precomputed star-pattern indices | Very high |
| RANSAC                   | Reject false correspondences            | Very high |
| Least-squares fitting    | Refine coordinate transformations       | Very high |
| SIP distortion modelling | Correct optical distortion              | High      |
| Bayesian verification    | Evaluate whether a solution is genuine  | High      |
| Polynomial WCS fitting   | Model more complex image distortions    | High      |

**My recommendation:** don't develop a blind solver from scratch initially. Study Astrometry.net and ASTAP, implement an adapter for one of them, and subsequently develop an independent solver that can exploit SkyStacker's existing star-detection and registration pipeline.

That distinction matters: your existing SkyStacker architecture already includes centroid/triangle/RANSAC registration. Those techniques provide a useful starting point, but registration between photographs and absolute astrometric calibration against an external celestial reference frame are not interchangeable.

### **1.3 Existing implementations to investigate**

[Astrometry.net orientation 180 degrees off? - Astronomy Software & Computers - Cloudy Nights](https://images.openai.com/static-rsc-4/UfCg3ij_96mSvSGvzoOkkXZVy6puQoQRqG8_x8XA7_5nMNwkRR8xuhwsN8dVd4X3S1tXpQ77w4fKlu7e9x7awtDqiC0dGWtSV_rbV_FwpwEfsPnr6nz3Ibxy8hPQs4PUDg7saeamqPPc0vd2NwQrEv3O63tDR-fojBKM7plKBeA?purpose=inline)

**Astrometry.net**

Blind plate solving and WCS generation

[Website](https://astrometry.net/?utm_source=chatgpt.com) · [Source code](https://github.com/dstndstn/astrometry.net?utm_source=chatgpt.com)

**ASTAP**

Fast local plate solving

[Website](https://www.hnsky.org/astap.htm?utm_source=chatgpt.com) · [Source code](https://github.com/astap/astap?utm_source=chatgpt.com)

**SCAMP**

Multi-exposure astrometric calibration

[Website](https://www.astromatic.net/software/scamp/?utm_source=chatgpt.com)

[File:Siril.svg - Wikimedia Commons](https://images.openai.com/static-rsc-4/8_efoOOlNxqDNsHrlHDne5BXlUuZMjY4cDcOD2sCW03yLc---7L1lDafK2r6fIZ4go0Bg_pSLzYxpyJp4MpdcHN1sDNsKVZNi27bMSJTrQWiC63ywM8Z6AptKTL84hbs-qs4f2n-CeSYY3UkX1XIbpFP_3_J4dY1iKT8LOFyo2c?purpose=inline)

**Siril**

Particularly relevant reference implementation for plate solving and image annotations

[Website](https://siril.org/?utm_source=chatgpt.com) · [Astrometry documentation](https://siril.readthedocs.io/en/stable/astrometry/?utm_source=chatgpt.com)

Siril is especially interesting because its documentation describes precisely the workflow you want: solve an image, retain WCS information, query object catalogues, and draw object annotations over the image.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://siril.readthedocs.io\&sz=32)

Siril 1.4.4 documentation+2



## **2. Mapping an astronomical image with a sky chart**

Highest priority for the visualization engine

This is the next essential step.

Imagine loading a photograph of the Andromeda Galaxy. SkyStacker should be able to:

1. Solve the photograph.
2. Determine the celestial coordinates of every pixel.
3. Retrieve all known astronomical objects within its field of view.
4. Transform the coordinates of those objects into image pixels.
5. Draw markers, labels, contours and object boundaries.
6. Allow the user to select an object and inspect its scientific properties.

[Reprocessing Project: Messier 31 - The Andromeda Galaxy (6 hours in LHaRGB) — Cosgrove's Cosmos](https://images.openai.com/static-rsc-4/O_cD0lMSAE8QPKYKen3Zl6AwCm_lN9fc4O06___OsqbQgXeLb5-es7jcsURmGs1gWlW8fvFewr7fUj4KGDKZ7kA31kkJ6EdeimXR1dpqVRCwvEvCghIgcVri_B6SK64YXjxhiQkQM6mWN8k_0CVTIhnjLhmNIfzlZGR2baBrlZQ?purpose=inline)

[M81 Bode’s Galaxy and M82 Cigar Galaxy Revisited | mj's photography](https://images.openai.com/static-rsc-4/gfDA7JOLD4V-eetzsuXWH9IdAuWvdd4EJ8mRlkT_kjqoJ9oPrES6JFxxPVfeCqxpVj2w6HrgZLWUZs2oTbMZqgO5I-PLXGDdKLBQRmzX2462xQlYhNRWSN1RMcoEQTk-yZvlKb7lhID-r1M3fwiMVSauIzKwqNYG_6Z2Mr9nyfA?purpose=inline)

[4.5 Displaying Imaging Data - HST User Documentation](https://images.openai.com/static-rsc-4/dAgIcjAqm8vWgdhD5EKonmM_EmH82mgweBxDBriLCBF74_OYCbbOBJRfEK4qJJhPe2fYmBwOiZiGPiOGXEEIR2jZ-RRqypvgCF2TbfUpDk_h8WbO4ODdg6jVbk4ztV1wvQRVGczWuUMBobD-I1GCAXGhw7ZrRdMuXpExNqOi8Hc?purpose=inline)

14

### **2.1 The mathematical transformation**

The key operation is:

\\[ (x,y) \longleftrightarrow (\alpha,\delta) \\]

Where:

- \\(x,y\\): pixel coordinates.
- \\(\alpha\\): right ascension.
- \\(\delta\\): declination.

For an astronomical object with known coordinates, the WCS transformation computes its location in the image:

\\[ (x\_{\mathrm{object}},y\_{\mathrm{object}}) = \operatorname{WCS}^{-1}(\alpha,\delta) \\]

In practice, the WCS transformation involves a celestial projection, a reference pixel, a reference coordinate and a transformation matrix, potentially with additional distortion corrections.

For FITS images, this information is generally stored in header keywords such as:

- `CRPIX1`, `CRPIX2`: reference pixel.
- `CRVAL1`, `CRVAL2`: reference celestial coordinates.
- `CTYPE1`, `CTYPE2`: coordinate projection.
- `CDELT1`, `CDELT2`: pixel scale in simple representations.
- `CD1_1`, `CD1_2`, `CD2_1`, `CD2_2`: coordinate transformation matrix.
- `CROTA2`: legacy rotation representation.
- `A_ORDER`, `B_ORDER`: polynomial distortion information in SIP conventions.

Do not assume that every FITS image uses the same WCS representation.

### **2.2 Research and software references**

- **Calabretta & Greisen (2002):** the fundamental FITS WCS conventions. [Paper](https://doi.org/10.1051/0004-6361:20021327?utm_source=chatgpt.com).
- **WCSLIB:** established implementation of FITS celestial transformations. [Documentation](https://www.atnf.csiro.au/computing/software/wcs/?utm_source=chatgpt.com).
- **Astropy WCS:** excellent reference implementation and test oracle for validating your Go implementation. [Documentation](https://docs.astropy.org/en/stable/wcs/?utm_source=chatgpt.com).
- **Siril annotations:** a practical implementation of catalogue objects rendered over WCS-calibrated images. [Documentation](https://siril.readthedocs.io/en/stable/astrometry/annotations.html?utm_source=chatgpt.com).

### **2.3 What should SkyStacker render?**

[Astrometry tools — AstroBin](https://images.openai.com/static-rsc-4/kQGyCn2cuWR9tzR8TeRyIpKgLGDHbEWmgWkaBNgnuUElpRE3-NcX35OVXPwB_m_ZP1YQGSIB5klaPQhdj4WHEAeWmFFq-9eq0qfgHtjgUNT4gKxVBEPgS4Ome2pqB_Z40KJh9zIzPfVc-1Gg-eXQOxsnJtVXs_HYWnZPxJvg1k8?purpose=inline)

[My new Skywatcher Quattro 150P / Blog - Thoughts, news, activties | Benny's AstroBlog](https://images.openai.com/static-rsc-4/WOA44bB-4BWPzs5_jiXiM6t-yx61ZX1PUREmu4BSnmQcUZL3WoG48cEw5pu1vhGQ3ZrdMvprKkYVT3BwQqicOSGaX1caYpCJfUTiuKdn_PU2yAnFlyggked5OooMc9b-Ln5pmy5BNUveCr5VAOK8tHB2C5ELvvyxFNftsg8ZibM?purpose=inline)

[NGC 2174 - The Monkey Head Nebula - 5.1 hours SHO — Cosgrove's Cosmos](https://images.openai.com/static-rsc-4/lwAGZJTuAdXWqhLKxSAGKfQalw-YAVmMxne9PW577iPvSsXpMCSZvctDcrEznc8pNjD0zc0moMDDbd1PgujI2flTHPyHMjitop7_GIbg9xycy_kviiy9l-0Gdc01YKCVjdTdVft46nCbjXLUQATz-xFDXToaRfOoPplaGje4S2c?purpose=inline)

16

I would design the annotation engine to support several independent visual layers:

| Layer             | Visual representation                                          |
| ----------------- | -------------------------------------------------------------- |
| Stars             | Small circles, coloured by spectral or photometric information |
| Double stars      | Paired markers and separation lines                            |
| Galaxies          | Ellipses indicating position angle and angular dimensions      |
| Nebulae           | Outlines or translucent polygons                               |
| Globular clusters | Circles indicating apparent extent                             |
| Open clusters     | Star-membership markers or cluster boundaries                  |
| Quasars           | Special symbols and catalogue identifiers                      |
| Supernovae        | Distinct transient markers                                     |
| Novae             | Transient markers with discovery dates                         |
| Comets            | Position markers, direction and optional trajectory            |
| Asteroids         | Position markers and motion vectors                            |
| Unknown sources   | Highlighted detections with no catalogue match                 |
| Celestial grid    | RA/Dec coordinate lines                                        |
| Constellations    | Boundaries, lines and names                                    |

The important design decision is to keep the annotations separate from the actual astronomical pixels. The user must be able to toggle layers independently, change opacity, inspect objects and export an annotated version without modifying the original image.

## **3. Astronomical catalogues: how to know what is in the image**

A plate solver tells you where the image is pointing. It does not tell you everything that exists in that field.

For that, SkyStacker needs a catalogue-query and cross-identification system.

The ideal approach is to query several astronomical databases using the image centre, field of view and limiting magnitude.

### **3.1 The most important astronomical databases**

[European Space Agency (@esa) on X](https://images.openai.com/static-rsc-4/W36TlCu5jcw6X4Eg7q9qy-UUKpkuGjDXou3MH16ajFHdvRNViOWVB1laQGjBOHcM1oPP3orhp73DL5YyXmLO44jUlFCccCZz00I-xQ9lxcRFfq5g3ji3xMjCEF9wLvTdbw2OEA1b17mIXnvpvba3nPtCpqu2dOUosh7PkwlBBF8?purpose=inline)

**Gaia DR3**

Essential

The principal reference for stellar astrometry.

Provides positions, parallaxes, proper motions, magnitudes, colours, astrophysical parameters, variable-star classifications and extragalactic candidates.

Particularly valuable for identifying stars and distinguishing nearby stellar sources from distant extragalactic objects.

- [Gaia Archive](https://gea.esac.esa.int/archive/?utm_source=chatgpt.com)
- [DR3 scientific documentation](https://gea.esac.esa.int/archive/documentation/GDR3/?utm_source=chatgpt.com)
- [Gaia DR3 overview paper](https://doi.org/10.1051/0004-6361/202243940?utm_source=chatgpt.com)

[image](https://www.google.com/s2/favicons?domain=https://gea.esac.esa.int\&sz=32)

gea.esac.esa.int+1

[SIMBAD Astronomical Database](https://images.openai.com/static-rsc-4/K9shlWGn-PnG8uV0NbvwflcN3Q7AsaTEcgkKCs6froalBZp5XiKJdMUkZCuMc-9XrlfFYcneA0jyDv0_DQBa46BrYyWj1R9dmn1ag6M62Ke54wvWfEc1jbJr3v9WR-2JeDEGTef7G63CfeFLoHWad3SATOQzPe4Yj-CH21RKUdk?purpose=inline)

**SIMBAD**

Essential

A general-purpose database of astronomical objects beyond the Solar System.

Particularly useful for resolving object names and aliases, finding object types, retrieving bibliographic references and cross-identifying known objects.

- [SIMBAD](https://simbad.cds.unistra.fr/?utm_source=chatgpt.com)
- [User documentation](https://simbad.cds.unistra.fr/guide/?utm_source=chatgpt.com)

Foundational paper: Wenger et al. (2000), *The SIMBAD astronomical database*, A&AS, 143, 9.

[image](https://www.google.com/s2/favicons?domain=https://simbad.astro.unistra.fr\&sz=32)

CDS (Strasbourg)

[Online data query — CARTA 5.1.0 documentation](https://images.openai.com/static-rsc-4/M0IvoFZf3J1qhlqnFvn2ugvZ3IahSDo_HjKJz8euzEgG6igMjTuf1315U0scac_dA7B6TsVIAfoKFi-g6jqbTgC52VkrkDpeJQ4jR1WHEYXJ07scclgOl6Q7V20L68bDf8cpJZOuEJcUDhp0q-dbWOgswhxc8oWwdt38rsTR6BA?purpose=inline)

**VizieR**

Essential

One of the most important resources for retrieving astronomical catalogues.

It provides access to thousands of published catalogues, including specialized catalogues for galaxies, quasars, nebulae, variable stars and other objects.

- [VizieR](https://vizier.cds.unistra.fr/?utm_source=chatgpt.com)
- [Service DOI](https://doi.org/10.26093/cds/vizier?utm_source=chatgpt.com)

Foundational paper: Ochsenbein, Bauer & Marcout (2000), *The VizieR database of astronomical catalogues*, A&AS, 143, 23–32.

[image](https://www.google.com/s2/favicons?domain=https://www.researchgate.net\&sz=32)

ResearchGate+1

[NED - NASA/IPAC Extragalactic Database](https://images.openai.com/static-rsc-4/rsk0o9q8KD4zT-EwBh0icdsZe8a1FV8eOxFjwvSfIixikRjWx_EISyBiGLa00I0FQuL8skzTvMg-vYq0ivKl4SvHhszcZ1X0l-vqkPqU7qi6GsRrQrZWWzmE3OC4ukXmfWwuAjJN7IWx1bx1x2p-l0RYrTZq_uK-Jr2FutTNVmM?purpose=inline)

**NASA/IPAC Extragalactic Database (NED)**

Especially important for galaxies, galaxy clusters, active galactic nuclei, quasars and redshift information.

It maintains cross-identifications, positions, redshifts, photometry and scientific references.

- [NED](https://ned.ipac.caltech.edu/?utm_source=chatgpt.com)
- [NED scientific publications](https://ned.ipac.caltech.edu/Documents/References/NEDpapers?utm_source=chatgpt.com)

[image](https://www.google.com/s2/favicons?domain=https://science.nasa.gov\&sz=32)

NASA Science+1

[APOD: June 10, 1998 - NGC 6070: First Light for Sloan](https://images.openai.com/static-rsc-4/76Bc9b9z_9UMzpKz6adTimgigozbXwGDJsztpiOxbpzlx3Q8iTovEeEPF11e5G1KeH74qCplbLLWvJne1z0LhkB6pca_B0Xw-4k2DjNB_c99AQmZLD2xhIBELC0XNAYb3gbSweARgNMnfUGqBPPSJftBQnLv2urOWyi05ubPnI8?purpose=inline)

**Sloan Digital Sky Survey (SDSS)**

Very valuable for galaxies, quasars and stellar classification.

Its spectroscopic measurements provide a way to validate classifications that would otherwise be based only on image appearance.

- [SDSS](https://www.sdss.org/?utm_source=chatgpt.com)
- [SkyServer](https://skyserver.sdss.org/?utm_source=chatgpt.com)

[V1535 Sco Photometry](https://images.openai.com/static-rsc-4/JWgN-xP-4Jv_ZEAFegr2R_IhKeWUc824nCHx7vDeJSYsNAA5wU-zRWcZsTsrxwt5QhwYLS-gYM32HJkCyvzJI-mbC6MNl-Og8PBUo9wMhHcMELm6KkHw6npbZs-OprUR9WXFAKe02r-cUhv_36a5G10uxaGRwli41OKV8Vw-VBc?purpose=inline)

**AAVSO VSX**

Important for variable stars, suspected variables, nova candidates and historical variability information.

- [Variable Star Index](https://www.aavso.org/vsx/?utm_source=chatgpt.com)
- [VizieR catalogue record](https://cdsarc.cds.unistra.fr/viz-bin/cat/B/vsx?utm_source=chatgpt.com)

The catalogue is also useful for distinguishing a new transient from an already known variable source.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://cdsarc.cds.unistra.fr\&sz=32)

cdsarc.cds.unistra.fr+1



### **3.2 Which catalogue should identify which object?**

This is an important architectural decision. No single database is sufficient.

| Object            | Primary sources                          | Additional information             |
| ----------------- | ---------------------------------------- | ---------------------------------- |
| Stars             | Gaia DR3                                 | Tycho-2, UCAC, APASS               |
| Double stars      | WDS                                      | Gaia DR3                           |
| Variable stars    | AAVSO VSX, Gaia DR3                      | GCVS                               |
| Galaxies          | NED, HyperLEDA                           | SDSS, VizieR                       |
| Quasars           | Gaia DR3, SDSS                           | Milliquas                          |
| AGN               | SIMBAD, NED                              | SDSS                               |
| Nebulae           | SIMBAD, VizieR                           | Sharpless, Lynds                   |
| Planetary nebulae | HASH, SIMBAD                             | VizieR                             |
| Globular clusters | Harris catalogue, SIMBAD                 | VizieR                             |
| Open clusters     | Gaia DR3, SIMBAD                         | MWSC                               |
| Supernovae        | Transient Name Server                    | SIMBAD                             |
| Novae             | AAVSO VSX, SIMBAD                        | TNS for relevant transient reports |
| Comets            | Minor Planet Center                      | JPL Horizons                       |
| Asteroids         | Minor Planet Center                      | JPL Horizons                       |
| Satellites        | CelesTrak                                | Space-Track                        |
| Unknown objects   | Cross-match against all relevant sources | Historical image comparison        |

The catalogue layer should support multiple providers, not hard-code a particular database into the detection algorithms.

### **3.3 Important research on stellar and extragalactic classification**

These are particularly relevant because you want SkyStacker to distinguish stars, galaxies and quasars rather than merely display catalogue markers.

**Paper 1 — Machine learning-based photometric classification of galaxies, quasars, emission-line galaxies, and stars (2024).**

Uses Random Forest classification with SDSS DR17 and AllWISE photometry.

[Read the paper](https://doi.org/10.1093/mnras/stad3436?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

DOI



**Paper 2 — Identifying galaxies, quasars, and stars with machine learning (2020).**

Clarke et al. classify 111 million SDSS sources using Random Forest and photometric information.

This is particularly relevant to building an object-classification engine.

[Read the paper](https://doi.org/10.1051/0004-6361/201936770?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

DOI



**Paper 3 — Gaia DR3: The extragalactic content (2023).**

Investigates the identification of quasars and galaxy candidates using machine learning and Gaia observations.

[Read the paper](https://doi.org/10.1051/0004-6361/202243232?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

The extragalactic content



**Paper 4 — Machine learning and galaxy morphology: for what purpose? (2023).**

Fraix-Burnet discusses the application and limitations of machine learning in galaxy morphology.

[Read the paper](https://doi.org/10.1093/mnras/stad1654?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

DOI



## **4. Identifying galaxies, nebulae and deep-sky objects directly from pixels**

This is a different problem from catalogue cross-matching.

Suppose a galaxy appears in your photograph. Its centre might be catalogued, but the catalogue position alone doesn't describe its visible shape.

SkyStacker should ideally be able to determine:

- The object's actual visible boundaries.
- Its centroid.
- Major and minor axes.
- Ellipticity.
- Position angle.
- Integrated flux.
- Surface brightness.
- Morphological characteristics.
- Whether it is extended or point-like.

[M 101 - Pinwheel Galaxy. High resolution and deep field with C2PU T 1000. - AstroBin](https://images.openai.com/static-rsc-4/L9Qrs-rLoBZ8zH1NcMcKMQoD0UpugNOOh0j0ua3RydNheEV6VUOyHMl0nQ3fMZIDTFs-ePIALbyB8p3HF-7Dn3rBniZwSBEjn2c3-f4m8ewSfcsHysusNJajjTNj8SUBwWBbgOKajMTdCi7AshVOzRxwP2MC3WYmQKgJ8lIg7oU?purpose=inline)

[Team Omicron - The supergiant M87 with its jet and globular clusters - AstroBin](https://images.openai.com/static-rsc-4/iTmQlYmiDv5g1BLd-OdFZWws9Lfop0iGgeBiK6IspNBip2YL7MWmyZ7PANi1BZe9MYqRJUT94Ol1Cy-Hi4OH8kMKnsiLwZeu589GfFTjcaEvnyWR6FXnE7VdQPpXLsqBOkIl5YuE4GXD_W2FsKyJnImXRw0Fs1l6EdOznyW0i7c?purpose=inline)

[La nébuleuse planétaire PK 164  31.1](https://images.openai.com/static-rsc-4/1mFXERFGLYVeMPhNAJNmdEOC_R3u-2g9gqSKXqFNgd8VSXvqYuqZrB5bE3d0TAGiY0RyBivbA8GMzl2AMpbvrmQkkOi0EChyncVyiaS5JCaRkTLauFgLen61vvZFfizl5lbEHPqrM9SXDk-A1Tfg9Apb4edNV4uEri-H00mbY7I?purpose=inline)

8

### **4.1 Scientific techniques to investigate**

**A. Source extraction**

The foundational approach is to detect connected groups of pixels whose signal exceeds the estimated local background.

Research and implementations:

- SExtractor — Bertin & Arnouts (1996).
- DAOStarFinder — Stetson (1987).
- SEP — Source Extractor in Python.
- Photutils — astronomical source detection and segmentation.

References:

- [SExtractor paper](https://doi.org/10.1051/aas:1996164?utm_source=chatgpt.com).
- [Photutils source detection](https://docs.astropy.org/projects/photutils/en/stable/user_guide/detection.html?utm_source=chatgpt.com).
- [Photutils documentation](https://photutils.readthedocs.io/?utm_source=chatgpt.com).

Photutils is particularly interesting because it distinguishes point-source detection from general image segmentation.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://docs.astropy.org\&sz=32)

photutils 3.1.1+1



**B. Morphological classification**

Once an object has been detected, its morphology can be analysed.

Useful measurements include:

| Measurement         | What it reveals                               |
| ------------------- | --------------------------------------------- |
| FWHM                | Apparent source width                         |
| Ellipticity         | Degree of elongation                          |
| Eccentricity        | Shape characteristics                         |
| Concentration index | Central light concentration                   |
| Gini coefficient    | Distribution of pixel intensities             |
| M20                 | Spatial distribution of the brightest regions |
| Sérsic index        | Approximate light-profile structure           |
| Moments             | Orientation and shape                         |
| Radial profile      | Distribution of brightness                    |

**C. Machine learning**

For more advanced identification, convolutional neural networks and vision transformers can be trained to classify objects based on their appearance.

An excellent research dataset is Galaxy Zoo.

Lintott et al. (2011), *Galaxy Zoo 1: data release of morphological classifications for nearly 900,000 galaxies*.

[Scientific paper](https://doi.org/10.1111/j.1365-2966.2010.17432.x?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://agupubs.onlinelibrary.wiley.com\&sz=32)

AGU Journals



The Galaxy Zoo classifications are valuable training and validation material, but the data domain differs from amateur astrophotography. A model trained on survey images should not automatically be assumed to perform equally well on stretched, noisy, undersampled or narrowband astrophotographs.

### **4.2 An important opportunity for SkyStacker**

I would implement a morphological analysis system that does not depend on identifying the object first.

For example:

**Example: unknown extended source**

[PGC galaxy astrophotography? Anyone - Experienced Deep Sky Imaging - Cloudy Nights](https://images.openai.com/static-rsc-4/BLtfTjSSPJnagLisQyFbHtumeZsuRAjvNM-vTENerg6ouXfjIHr-8Ff6LdjH9hKdf2jV03Nl4e7mylRb-iCcj0iXnjjlYsK1NPE79Bun9sriOdcyDhVmI93uDFzPY3mL8r2sSKT2Kf8I4hzoDITiKhhQyWNDzJ-bD5xR3wy3Ojg?purpose=inline)

[cloudynights.com](https://www.cloudynights.com/forums/topic/840678-pgc-galaxy-astrophotography-anyone/?utm_source=chatgpt.com)

1. Detect an extended source.
2. Measure its centroid, dimensions and ellipticity.
3. Determine whether it is distinguishable from the local PSF.
4. Query catalogues for a counterpart.
5. Compare its morphology with the available object classifications.
6. Report the best candidates and confidence estimates.

Potential SkyStacker feature

**Uncatalogued extended-object detection**

This would allow SkyStacker to flag faint galaxies that are visible in the image but absent from a basic Messier/NGC/IC annotation catalogue.

## **5. Detecting novae, supernovae and other transient phenomena**

Very interesting advanced feature

This is one of the most scientifically interesting parts of your proposal.

A supernova or nova may appear as a new point-like source in a galaxy, while the same position in an older image contains no detectable source.

A conventional plate solver cannot identify this phenomenon. It requires temporal analysis.

[Supernova in M101 | The Planetary Society](https://images.openai.com/static-rsc-4/eHv4cyq2z5DhCyf8jLxp3nnMRdT6lvZn78Rlft6qe6wTLIJDjWHgaKeQ1ZB5wKjzECIp6JeXNce1y68nYJwzFYfGRlMsi9hIEVZqrIQybgC9gY5b-DEmGwS5N2YNCQjn6ISWJqVeaHzmM2bodMHx0xdO5CmU2U_JR71W3kYyXnI?purpose=inline)

[Hubble Pinpoints Doomed Star that Explodes as Supernova | ESA/Hubble](https://images.openai.com/static-rsc-4/SzyXz6zVAdqIqGUUNv5zE1F4OHrGCjwnLoDKqu1OJTv-bPBGHrG4yDgzf8AKfc7Sgj2dngYifOwpIh3wo5E72xx-xA5xglrED79AFU0nCOP_k0coO1aRWtEWvFrt7ILHUyAJ7Ps_x2G73GaiEM3Q6c6puGjTG5BSaU7RMbMU6IA?purpose=inline)

[V1405 Cas - Nova in Cassiopeia — AAPOD2.COM](https://images.openai.com/static-rsc-4/7rO7HKOzF0366qBIWdUQU86HW-46GvhkhrPueUADqIbd8dZY5MuThaSuj4rA6DYGy0Zp-Mx_eEug2hirzUIbhX_toJQMsbh0A1W2xxBd5RjZz-ePu-8mhWLFYYjbWrh31d6doRYkQvs7TYzCV2ME87KsrBojDTjsQ0TOTlNrwu4?purpose=inline)

11

### **5.1 Difference imaging**

The fundamental technique is to compare a new observation with an existing reference image.

\\[ D(x,y)=I\_{\mathrm{new}}(x,y)-I\_{\mathrm{reference}}(x,y) \\]

However, simple subtraction is not sufficient. Images need to be astrometrically aligned, photometrically normalized, and their point-spread functions matched or otherwise accounted for.

One of the most important papers is:

**Zackay, Ofek & Gal-Yam (2016).**

*Proper Image Subtraction—Optimal Transient Detection, Photometry, and Hypothesis Testing.*

The ZOGY algorithm derives an optimal statistical method for detecting transient sources and distinguishing genuine astrophysical changes from image artefacts.

It is particularly interesting because it also discusses particle hits, registration errors, bright-object artefacts and difference-image statistics.

- [Scientific paper](https://doi.org/10.3847/0004-637X/830/1/27?utm_source=chatgpt.com)
- [Free arXiv version](https://arxiv.org/abs/1601.02655?utm_source=chatgpt.com)

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

DOI+1

### **5.2 What can be detected?**

| Phenomenon         | Detection strategy                                     |
| ------------------ | ------------------------------------------------------ |
| Supernova          | New point source, often near a galaxy                  |
| Nova               | New or brightened stellar source                       |
| Variable star      | Photometric change over multiple epochs                |
| Microlensing       | Characteristic brightness evolution                    |
| Asteroid           | Motion between sequential exposures                    |
| Comet              | Motion plus possible extended coma                     |
| Satellite          | Linear trail or sequential displacement                |
| Cosmic ray         | Single-exposure transient, often inconsistent with PSF |
| Optical reflection | Shape and position relative to bright sources          |
| Sensor artefact    | Detector-coordinate recurrence                         |

### **5.3 Transient databases and scientific resources**

[Home | Transient Name Server](https://images.openai.com/static-rsc-4/0mjzNPSxsMdWWT7UZdHDoslUMfZYoxt2zqCD-ySRc8i2-OcZmIkf1HPo_8M4GlgAk1WnY4PslVmtUA6chWXtLWFEHJwHqylOEpe3vL7qB1uiJx4RUvh619lClp0SjZOI77ckIkJB3IhtNM9kpOmmoUpUk8aN3Y3-1BGje9KuLfA?purpose=inline)

**Transient Name Server (TNS)**

The official IAU mechanism for reporting new astronomical transients, including supernova candidates.

[TNS website](https://www.wis-tns.org/?utm_source=chatgpt.com).

Important distinction: Galactic nova candidates have a separate reporting pathway and should not simply be treated as ordinary TNS supernova reports.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://www.wis-tns.org\&sz=32)

Transient Name Server



[Caught in the Act: Astronomers Detect a Star Devouring a Planet for the First Time](https://images.openai.com/static-rsc-4/JKog4tIvX4jbLhhxF3MunEBxsSYsme8Vpp_hE-SxMSO73kPhiULQFIVmkR1rk-qyfjE_PHLBAMliPlAQBHze78Ym3lBMOOEhVJjye-K_3yODJKAOi9glcbt17-XX7lU1eGK56VIr2cBWt-fs3k93Tmvpv0Q13_r93Ro_UebIgCM?purpose=inline)

**Zwicky Transient Facility (ZTF)**

A major source of transient detections and historical observations.

[ZTF](https://www.ztf.caltech.edu/?utm_source=chatgpt.com).

Particularly interesting for building historical image comparisons.

[Fink broker: fast and efficient framework for machine learning with LSST](https://images.openai.com/static-rsc-4/TfGcTHx--e377oSsVe4XAcbG2DDtJMMjPVrSGoBNarVWRR-Zo3LhjAoldNJ30UaDP1j-F1mfVqL66-vSFajl3o9qbvxHi_APMeiLFZR1is0_vH4DHmhnMBu2hLr48Y8bKAHNQgSORBPzRKPpnbDrkS_tSDbnnvNcQiXPLNfpK4E?purpose=inline)

**Fink**

A scientific alert broker that performs transient annotation, selection and classification.

Paper: *fink, a new generation of broker for the LSST community* (2021).

[Scientific paper](https://doi.org/10.1093/mnras/staa3602?utm_source=chatgpt.com) · [Project website](https://fink-broker.org/?utm_source=chatgpt.com).

[image](https://www.google.com/s2/favicons?domain=https://academic.oup.com\&sz=32)

OUP Academic

[Getting started -](https://images.openai.com/static-rsc-4/-G_K1POqZh2WvxIGDlHorH22-3JK6uOQmfKe9RR0v0uR9oBmAqTDT6sRZLUCGvzmKR7JkGC3CGvhyXCzRttq3wUcVmjnhlj8sO82OSY7_4h4Q9nDEIWi92rGyi-iBv0AYH6sGiEEuIGxaExvvA4UaHNwzSp6CeWCypv3Y-HDkyU?purpose=inline)

**Fink transient classifiers for LSST (2024)**

Research on classification methods for future large-scale astronomical alert streams.

[Read the paper](https://doi.org/10.1051/0004-6361/202450370?utm_source=chatgpt.com).

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

Benchmarks for LSST

**Important practical distinction:** identifying a transient candidate is feasible from amateur images. Confirming that it is a nova or supernova generally requires additional observations, temporal evolution, comparison with catalogues and, in many cases, spectroscopy.

SkyStacker should therefore report something like *unmatched transient candidate*, not declare a discovery.

## **6. Identifying comets, asteroids and moving objects**

This requires a different algorithmic approach because their apparent positions change with time.

[Comet 12P/Pons-Brooks, the Devil Comet | The Planetary Society](https://images.openai.com/static-rsc-4/-AtknFrORKbVyq4nOW_LQPGxxtzi0CGZMC_w9BGp7V_gDWQUtPTnh5B-bSw9zyWFa5GHU3Wcj9yFHiMhyrmRGsnKNqGIFcCYDO7rvkNcIsB44K_X28ND1ZTKoWp3YGOYlSh1PNouzfN8L6rGY7urNTXxNhqTc9t3WvxFl2Pq4OY?purpose=inline)

[Gaia Space Observatory Revolutionizes Asteroid Tracking](https://images.openai.com/static-rsc-4/PMWafGtZWMjevG17oKXtSTQnuKtkP5BRSXjo95tLaNuYoZ4ifF4yo_nJXDVDdnatVtF8Bz52j7v1M4nJnfTaY7LfJFY5W72xBWET2hPnS94WPxKbieB2K725BvgG2wRTcee7a5f32oG6tl7jnKgDJ1wnfgbMlEfwyoZDweJjqew?purpose=inline)

[Aug. 18, 2002, Asteroid Gallery](https://images.openai.com/static-rsc-4/Y0N8pslTd5PxiBvcNB_I_Z7qVbgrvEmPbFTEWxyUjJaieNVZfN43wnPcCQa0hh-ahBE9_AyUMI5BXdDy7wUskVkf2QWI124OWfKwdvJ3VSRuT7yxGPQ4yGymWQqaxEb3h-7TwOpzb2s3vYkFXWj5YTMpu0vG0BQsPId3UpWDLYQ?purpose=inline)

12

### **6.1 Scientific paper: A-Track**

**A-Track: A new approach for detection of moving objects in FITS images (2016).**

Published in *Computer Physics Communications*, Volume 207, pages 524–530.

This is directly relevant to your proposed functionality.

It describes an open-source pipeline for detecting asteroids and comets in sequential FITS images using a modified line-detection algorithm called MILD.

[Read the scientific paper](https://doi.org/10.1016/j.cpc.2016.07.023?utm_source=chatgpt.com).&#x20;

### **6.2 Essential astronomical services**

- Minor Planet Center: [Website](https://minorplanetcenter.net/?utm_source=chatgpt.com).
- [Guide to Minor Body Astrometry](https://docs.minorplanetcenter.net/mpc-ops-docs/astrometry/?utm_source=chatgpt.com).
- [MPC observations API](https://docs.minorplanetcenter.net/tutorials/notebooks/mpc_tutorial_api_observations/?utm_source=chatgpt.com).
- [MPC orbital data API](https://docs.minorplanetcenter.net/tutorials/notebooks/mpc_tutorial_api_orbits/?utm_source=chatgpt.com).
- JPL Horizons: [Ephemeris service](https://ssd.jpl.nasa.gov/horizons/?utm_source=chatgpt.com).

[image](https://www.google.com/s2/favicons?domain=https://docs.minorplanetcenter.net\&sz=32)

MPC Public Documentation Hub+2

### **6.3 Recommended implementation**

For a sequence of photographs, SkyStacker should:

1. Plate-solve each image.
2. Determine the precise observation time, including time standard.
3. Detect sources in every frame.
4. Transform detections into celestial coordinates.
5. Cross-match known Solar System objects against time-dependent ephemerides.
6. Identify sources that move consistently between exposures.
7. Estimate angular velocity and trajectory.
8. Distinguish known objects from unidentified moving candidates.

A particularly useful detail: a comet may appear stationary relative to the stars during a short exposure, while its motion becomes apparent between images. Its extended coma and tail can also provide morphological evidence.

## **7. Unknown objects and astronomical anomalies**

Potentially the most original SkyStacker feature

This is where I think your proposal becomes particularly interesting.

Consider the following situation:

You photograph a region of the sky. The image is plate-solved correctly. All known stars have been identified. Known galaxies and nebulae have been matched against catalogues.

But there is a bright source in the image that doesn't appear in any catalogue.

What is it?

Possible explanations include:

- An uncatalogued galaxy.
- A faint background galaxy.
- A previously unidentified quasar.
- A transient astronomical source.
- A previously unknown variable star.
- An asteroid or comet.
- A satellite.
- A cosmic-ray hit.
- An internal reflection.
- A diffraction artefact.
- A hot pixel or defective sensor element.
- A stacking or registration artefact.

A robust system must not automatically interpret every unmatched source as a new astronomical discovery.

### **7.1 A scientific anomaly-detection pipeline**

I would implement an independent anomaly-detection engine with the following stages.

## **Unknown astronomical source detection**

**1. Input astronomical image**

Calibrated image, preferably with noise and mask information

**2. Source detection**

Stars, extended objects, trails and unusual structures

**3. Catalogue cross-matching**

Gaia, SIMBAD, VizieR, NED, VSX, MPC and others

**Known source**

Identify and annotate

**Unmatched source**

Continue analysis

**4. Anomaly classification**

Morphology, photometry, PSF, noise, temporal behaviour and motion

**5. Candidate classification**

Possible transient · moving object · extended source · instrumental artefact · unknown

**6. Scientific report**

Pixel coordinates, RA/Dec, flux, uncertainty, evidence, catalogue matches and confidence

### **7.2 Research areas worth investigating**

There are several scientific disciplines that should inform this subsystem.

| Research area                  | Techniques                               |
| ------------------------------ | ---------------------------------------- |
| Outlier detection              | Isolation Forest, LOF, robust statistics |
| Unsupervised learning          | DBSCAN, HDBSCAN, clustering              |
| Deep learning                  | Autoencoders, CNNs, vision transformers  |
| Anomaly segmentation           | Pixel-level reconstruction errors        |
| Time-domain astronomy          | Light-curve analysis                     |
| Statistical hypothesis testing | Significance, false-positive probability |
| Image differencing             | ZOGY, PSF matching                       |
| Morphological analysis         | Shape, concentration, surface brightness |

A particularly useful starting point is the Gaia DR3 Apsis research.

Creevey et al. (2023), *Gaia Data Release 3: Astrophysical parameters inference system (Apsis). I. Methods and content overview*.

It describes the classification and characterisation of astronomical sources, including an unsupervised analysis of approximately 50 million outlier sources.

[Read the paper](https://doi.org/10.1051/0004-6361/202243688?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://doi.org\&sz=32)

DOI



### **7.3 A useful distinction: unknown versus uncatalogued**

I would make these two concepts explicit in the software.

- **Uncatalogued:** a detected source has no counterpart in the catalogues queried.
- **Anomalous:** its measured characteristics are inconsistent with expected astronomical or instrumental populations.

An uncatalogued source can simply be a faint galaxy missing from the available catalogues. An anomaly is a source whose behaviour or characteristics merit further investigation.

That distinction is scientifically important.

## **8. SkyChart / Cartes du Ciel: how to display images and astronomical objects**

You specifically mentioned SkyChart, so I investigated the actual Cartes du Ciel documentation.

[en:screenshot:start \[Skychart\]](https://images.openai.com/static-rsc-4/2bnW_C549yjG9976i9rEFOEMPIo8Bw52wTpQelxTDCKJmRZ9A6NnZ7m3n6c3Dmjze29qw0vhoDwC-uk2Vdc4VTuJRTgeYq5TfvJPIHmpRARWwP1QRHkoUviM7jCAHQ7Hn6frAuI2eQRw684FmyicIq8-_l6sMDD7toh5iqaTBgc?purpose=inline)

[Têtes en l'air](https://images.openai.com/static-rsc-4/dN_L_G3d_VBOL0OPIBqdRQ2qoaGCVYTd4Zdtfci_2sSDBdiPBJlf8FBnxKcVyuSnTKviwfjtxDKM3kL0wW9gJZSk2o8S-JrqERjg0ar34cHrh2QDZa4_io0UMN7pVAz6KzN7L8KoGFDeq-P8ccWiyYEslquLaCVkPKYaqjV9UJE?purpose=inline)

[en:screenshot:start \[Skychart\]](https://images.openai.com/static-rsc-4/3p-OY5udjqNYW0WCZvUZsZhimNQkeQnjb3_mG1WkTasVuGnbZayKu3zfyWwQLeruEn_v0seu4xbYAcTH_879b_rtyz2d2aTWjZL4lE2-SnQClEVnBIDdriiiWL5wPQAWSo2ndv7wyQiZzE11J-EVhY6VsNydHSRoNNX62CnnaOc?purpose=inline)

12

The software is SkyChart / Cartes du Ciel.

Official website: [SkyChart](https://www.ap-i.net/skychart/en/start?utm_source=chatgpt.com).

### **8.1 Existing capabilities**

SkyChart already supports several features relevant to your project:

- Displaying stars and deep-sky objects.
- Displaying planets, asteroids and comets.
- Displaying celestial coordinate grids.
- Loading deep-sky object photographs.
- Displaying FITS images with WCS information.
- Overlaying archived astronomical images onto sky charts.
- Managing image archives and their display settings.

Its documentation explicitly states that FITS images with WCS information are eligible for display in the image archive.

References:

- [SkyChart: Pictures documentation](https://www.ap-i.net/skychart/en/documentation/pictures?utm_source=chatgpt.com).
- [SkyChart user manual](https://www.ap-i.net/pub/skychart/doc/doc_4.0_en.pdf?utm_source=chatgpt.com).

[image](https://www.google.com/s2/favicons?domain=https://www.ap-i.net\&sz=32)

ap-i.net+1

### **8.2 How I would connect SkyStacker to SkyChart**

The best integration mechanism is to make SkyStacker generate astrometrically calibrated FITS images.

The workflow would be:

[ASTROTNSTACK align and stack astro-photography pictures - File Exchange - MATLAB Central](https://images.openai.com/static-rsc-4/PtQEldwaPoABP-X8_ix-xLTXVNDHLCdPtTYBJIkGr3TlSk7SMwj4QiX_aqk9I6doJfFxhgTEi2Vc8m19D9jnd6CYUFS2eADHbySRvUGBcWVhxD1Ebfbpo6bf_LjQLFXjfDpoTmrQYg7siC2EWSSSD7qfBh2I8KODor_R_ESFVp8?purpose=inline)

**SkyStacker**

Calibrate, register, stack and process the images

**Astrometry engine**

Plate solving, WCS generation and validation

**Export**

FITS image + WCS header + optional annotation catalogue

[en:screenshot:start \[Skychart\]](https://images.openai.com/static-rsc-4/dN_L_G3d_VBOL0OPIBqdRQ2qoaGCVYTd4Zdtfci_2sSDBdiPBJlf8FBnxKcVyuSnTKviwfjtxDKM3kL0wW9gJZSk2o8S-JrqERjg0ar34cHrh2QDZa4_io0UMN7pVAz6KzN7L8KoGFDeq-P8ccWiyYEslquLaCVkPKYaqjV9UJE?purpose=inline)

**SkyChart**

Load the calibrated image into its image archive and display it on the sky chart

There is one important limitation: SkyChart's image archive is designed to display FITS images carrying WCS information, but it is not itself a general-purpose annotation engine for arbitrary per-image detections. The exact annotation and user-catalogue interoperability would need to be tested.

### **8.3 An alternative worth studying: Aladin**

Excellent reference implementation

[Planetarium software to display solved images within - Astronomy Software & Computers - Cloudy Nights](https://images.openai.com/static-rsc-4/trkqxzWiKhk5xhLBOtSE28JLpKHgE-xnEKXzLQPjEOw7w2xxERjpRu5-Q5eI5BTY7RIJStI1GvE6HcAu5fWSholPuQ3EJLOcFT5mkyjoVbFfYZwUjo_sp3IKCy_9WNxfY4E98c0Ct_91yGhaI-ggC1bVWgvW7jBbXvrrjL9nRVA?purpose=inline)

[Using Aladin with APT in Phase I](https://images.openai.com/static-rsc-4/_rg9PUYb0Y_7tHzsvNrJbiUpuHEkk-ZdIOKDTa6jty8bWtrKDKUpMCoLmjEcUpju-5pRFcI-Tgcr5dnyKFXjQ0iRRJ9DN50Jc4mqC_JTe0SQee9aJC6TgpmeigjNBKOSbDJm1ZQ2NkAgVOvSHK1fQ1l58XN3Ssf5i-cKcHqAD-8?purpose=inline)

[Aladin Lite interactive map | Jesus Iniesta](https://images.openai.com/static-rsc-4/FD58fZx1m-9YlLLdv_5Iux0eAN9nl_Yvvck586tsXuoz_yfRoWyDAvRFhm2eLMosBnXArUDfdIoNb5aA3IQjzoaNXlMavNlxc4km1BaMtTUCJCgn9FTSeV3_e8UbV3tqQ2R8IQ7nibU2Gmk9OB04eq8m28W8YHsfjqzzudSx_dQ?purpose=inline)

12

Aladin Sky Atlas is arguably an even more relevant reference for the functionality you want.

It can:

- Display astronomical images and surveys.
- Superimpose catalogue objects.
- Compare different wavelengths.
- Interact with SIMBAD, VizieR and NED.
- Display large source catalogues.
- Perform cross-matching.
- Handle astronomical coordinate systems.
- Visualize FITS and VOTable data.

Official references:

- [Aladin Desktop](https://aladin.cds.unistra.fr/AladinDesktop/?utm_source=chatgpt.com).
- [Aladin Lite documentation](https://aladin.cds.unistra.fr/AladinLite/doc/?utm_source=chatgpt.com).
- [Aladin Sky Atlas overview](https://virtualadin.cds.unistra.fr/?utm_source=chatgpt.com).

Scientific papers:

1. Bonnarel et al. (2000), *The ALADIN interactive sky atlas: A reference tool for identification of astronomical sources*. DOI: [10.1051/aas:2000331](https://doi.org/10.1051/aas:2000331?utm_source=chatgpt.com).
2. Fernique et al. (2015), *Hierarchical progressive surveys: Multi-resolution HEALPix data structures for astronomical images, catalogues, and 3-dimensional data cubes*. DOI: [10.1051/0004-6361/201526075](https://doi.org/10.1051/0004-6361/201526075?utm_source=chatgpt.com).

[image](https://www.google.com/s2/favicons?domain=https://www.researchgate.net\&sz=32)

ResearchGate+2

I particularly recommend studying HiPS (Hierarchical Progressive Surveys).

HiPS is a hierarchical tiling system for astronomical images and catalogues. It allows huge sky surveys to be displayed efficiently at different zoom levels.

This could be useful if SkyStacker eventually develops its own interactive celestial atlas.

**My conclusion:** use SkyChart as an interoperability target, but study Aladin as the reference for the complete astronomical visualization and catalogue-overlay experience.

## **9. Additional scientific standards and technologies**

These are not individual object catalogues, but they are important for building a scientifically interoperable application.

| Standard or technology | Purpose                                 | Documentation                                                                               |
| ---------------------- | --------------------------------------- | ------------------------------------------------------------------------------------------- |
| FITS                   | Astronomical image and metadata format  | [FITS](https://fits.gsfc.nasa.gov/?utm_source=chatgpt.com)                                  |
| WCS                    | Pixel-to-sky transformations            | [WCSLIB](https://www.atnf.csiro.au/computing/software/wcs/?utm_source=chatgpt.com)          |
| SIP                    | Polynomial image distortion conventions | [FITS WCS](https://fits.gsfc.nasa.gov/fits_wcs.html?utm_source=chatgpt.com)                 |
| VOTable                | Astronomical catalogue exchange         | [IVOA](https://www.ivoa.net/documents/VOTable/?utm_source=chatgpt.com)                      |
| TAP / ADQL             | Query astronomical databases            | [TAP](https://www.ivoa.net/documents/TAP/?utm_source=chatgpt.com)                           |
| Cone Search            | Retrieve sources around coordinates     | [IVOA](https://www.ivoa.net/documents/ConeSearch/?utm_source=chatgpt.com)                   |
| HiPS                   | Multi-resolution sky surveys            | [HiPS](https://aladin.cds.unistra.fr/hips/?utm_source=chatgpt.com)                          |
| MOC                    | Multi-order sky coverage maps           | [MOC](https://ivoa.net/documents/MOC/?utm_source=chatgpt.com)                               |
| ADES                   | Minor-body astrometry data exchange     | [MPC](https://docs.minorplanetcenter.net/mpc-ops-docs/observations/?utm_source=chatgpt.com) |

One particularly valuable design decision would be to support the Virtual Observatory standards wherever possible. This avoids creating proprietary formats for every catalogue, survey and external astronomical application.

## **10. Proposed SkyStacker architecture**

Taking into account the existing Go implementation, the current image-processing pipeline, parallel CPU workers, memory-mapped/tile processing, and existing centroid/triangle/RANSAC registration, I would introduce a dedicated astrometry and astronomical identification subsystem.&#x20;

I would avoid making the entire project dependent on an external Python installation. Python astronomy libraries are excellent scientific references, but the production implementation can remain in Go, using external solvers or native libraries where appropriate.

### **10.1 Proposed modules**

| Module               | Responsibilities                                 |
| -------------------- | ------------------------------------------------ |
| `astrometry`         | Plate solving, WCS, coordinate transformations   |
| `astrometry/catalog` | Star catalogue indexing and queries              |
| `astrometry/solver`  | Astrometry.net, ASTAP adapters and native solver |
| `catalogues`         | SIMBAD, Gaia, VizieR, NED, MPC providers         |
| `sources`            | Detection, segmentation, centroiding             |
| `classification`     | Star, galaxy, nebula and quasar classification   |
| `morphology`         | Shape, ellipticity, size and surface brightness  |
| `annotations`        | Render labels, markers, ellipses and contours    |
| `transients`         | Novae, supernovae and variable sources           |
| `movingobjects`      | Comets, asteroids and other moving objects       |
| `anomaly`            | Unknown source and artefact detection            |
| `photometry`         | Flux, magnitude and uncertainty calculations     |
| `skyview`            | Interactive celestial visualization              |
| `interoperability`   | FITS, WCS, VOTable, SkyChart and Aladin          |

### **10.2 Suggested object data model**

Every detected object should have a common representation, independent of the catalogue from which it originated.

For example:

```
type AstronomicalObject struct {
    ID          string
    Name        string
    Designation []string

    Type        ObjectType

    RA          float64
    Dec         float64

    X           float64
    Y           float64

    Magnitude   float64
    Flux        float64
    FluxError   float64

    MajorAxis   float64
    MinorAxis   float64
    PositionAngle float64

    Redshift    *float64
    ProperMotionRA  *float64
    ProperMotionDec *float64

    Confidence  float64

    Catalogue   string
    Identified  bool
    Transient   bool
    Moving      bool
}
```

This is a conceptual starting point, not a complete astronomical data schema. In particular, coordinates should carry their reference frame and epoch, and photometric measurements should retain their filter, units and provenance.

The data model should also distinguish between:

- An object in a scientific catalogue.
- A source detected in the image.
- An association between a detection and a catalogue object.
- A classification hypothesis.

These are not necessarily the same entity.

### **10.3 Example of the final user experience**

[NGC 5078 and NGC 5101 in Hydra](https://images.openai.com/static-rsc-4/6AaZ_k2fOS0DYsOD7xMo7U6_OjUKJO4sd4jjktVGx22Foz-uWgGSJUuO1bFOLUrBAtAQjRlCCHxwelgfMWUd8RiMA3HaQ0GOjK6pgkBw-dBbr436_7BpllK1Y2Vg8qCKNkqXU_IMFxcZJo9p6rE2OhmW6Uc47iln2eBSGlUHHuc?purpose=inline)

[jthommes.com](https://jthommes.com/Astro/NGC5078-5101.htm?utm_source=chatgpt.com)

Imagine loading a stacked image and activating *Astronomical Identification*.

The application would show:

- Identified stars, with Gaia identifiers.
- Galaxies, with NGC, IC and PGC identifiers.
- Quasars and AGNs.
- Nebulae, with catalogue boundaries.
- Variable stars and their historical classifications.
- Known asteroids and comets at the observation time.
- Previously reported supernovae and novae.
- Unmatched detections.
- Suspected anomalies.

Clicking an object would open its information panel:

- Name and alternative designations.
- RA/Dec.
- Angular dimensions.
- Magnitude and photometry.
- Object type and classification.
- Redshift, where available.
- Scientific references.
- Catalogue provenance.
- Confidence and matching distance.

This would turn SkyStacker from a stacking application into a considerably more complete astronomical image-analysis environment.

## **11. Implementation roadmap: recommended priorities**

I would not attempt to implement all these features simultaneously. There is a very logical development sequence.

## **SkyStacker astronomical intelligence roadmap**

1. Phase 1 · Foundation

   **Astrometric calibration**
   - FITS WCS support.
   - Astrometry.net integration.
   - ASTAP integration.
   - WCS coordinate transformations.
   - Image-centre and field-of-view calculation.
   - Astrometric validation.
   Deliverable: every stacked image can be accurately located in the sky.
2. Phase 2 · Identification

   **Catalogue cross-matching**
   - Gaia DR3.
   - SIMBAD.
   - VizieR.
   - NED.
   - Messier, NGC, IC and other deep-sky catalogues.
   - Offline catalogue support.
   Deliverable: identify known objects within an image.
3. Phase 3 · Visualization

   **Astronomical annotations**
   - Object labels.
   - Star markers.
   - Galaxy ellipses.
   - Nebula boundaries.
   - RA/Dec grids.
   - Interactive object inspection.
   - SkyChart-compatible exports.
   Deliverable: a scientifically annotated astrophotograph.
4. Phase 4 · Analysis

   **Morphological classification**
   - Extended-source detection.
   - Galaxy morphology.
   - Stellar PSF analysis.
   - Nebula segmentation.
   - Photometry.
   - Quasar and galaxy candidate classification.
   Deliverable: identification based on both image appearance and catalogue information.
5. Phase 5 · Time-domain astronomy

   **Transient and moving-object detection**
   - Reference-image subtraction.
   - ZOGY-inspired algorithms.
   - Variable-star analysis.
   - Supernova and nova candidates.
   - Asteroid and comet tracking.
   - MPC and TNS cross-referencing.
   Deliverable: identify changes and moving objects between observations.
6. Phase 6 · Advanced research

   **Anomaly detection and discovery assistance**
   - Uncatalogued source detection.
   - Machine-learning classification.
   - Unsupervised anomaly detection.
   - Historical image comparisons.
   - Confidence estimation.
   - Candidate reporting.
   Deliverable: identify unusual astronomical sources that deserve further investigation.

## **12. Benchmarking and scientific validation**

I would make scientific validation part of the development process, not something added at the end.

Some useful datasets and tests:

| Dataset / resource                 | What to validate                        |
| ---------------------------------- | --------------------------------------- |
| Gaia DR3                           | Astrometric accuracy                    |
| SDSS DR17                          | Star/galaxy/quasar classification       |
| Galaxy Zoo                         | Galaxy morphology                       |
| ZTF public data                    | Transient detection                     |
| AAVSO VSX                          | Variable-star identification            |
| MPC observations                   | Moving-object identification            |
| Astrometry.net reference solutions | Plate-solving reliability               |
| Synthetic FITS images              | Known coordinate and detection accuracy |

Important metrics:

- Astrometric residual in arcseconds.
- Percentage of correctly solved images.
- False-positive and false-negative rates.
- Catalogue cross-match precision and recall.
- Centroid error in pixels.
- Photometric error.
- Transient detection completeness.
- Classification confusion matrices.
- Processing time and memory consumption.

One particular issue deserves attention: the WCS solution must account for optical distortion, atmospheric refraction, proper motion and observation epoch where relevant. Comparing a modern image against an old catalogue without epoch correction can produce misleading matches.

## **13. Consolidated academic bibliography**

Here is a curated reading list of the most relevant scientific publications identified in this research. I would use it as the starting bibliography for a dedicated SkyStacker research document.

### **A. Astrometry and plate solving**

1. Lang, D. et al. (2010). *Astrometry.net: Blind astrometric calibration of arbitrary astronomical images.* AJ, 139, 1782. [DOI](https://doi.org/10.1088/0004-6256/139/5/1782?utm_source=chatgpt.com).
2. Bertin, E. (2006). *Automatic Astrometric and Photometric Calibration with SCAMP.* ADASS XV. [Paper](https://articles.adsabs.harvard.edu/pdf/2006ASPC..351..112B?utm_source=chatgpt.com).
3. Calabretta, M. R. & Greisen, E. W. (2002). *Representations of celestial coordinates in FITS.* A&A, 395, 1077. [DOI](https://doi.org/10.1051/0004-6361:20021327?utm_source=chatgpt.com).
4. Bertin, E. & Arnouts, S. (1996). *SExtractor: Software for source extraction.* A&AS, 117, 393. [DOI](https://doi.org/10.1051/aas:1996164?utm_source=chatgpt.com).

### **B. Astronomical catalogues and identification**

5. Wenger, M. et al. (2000). *The SIMBAD astronomical database.* A&AS, 143, 9.
6. Ochsenbein, F., Bauer, P. & Marcout, J. (2000). *The VizieR database of astronomical catalogues.* A&AS, 143, 23. [DOI](https://doi.org/10.1051/aas:2000169?utm_source=chatgpt.com).
7. Gaia Collaboration (2023). *Gaia DR3: Summary of the content and survey properties.* A&A, 674, A1. [DOI](https://doi.org/10.1051/0004-6361/202243940?utm_source=chatgpt.com).
8. Gaia Collaboration (2023). *Gaia DR3: The extragalactic content.* A&A, 674, A41. [DOI](https://doi.org/10.1051/0004-6361/202243232?utm_source=chatgpt.com).
9. Creevey, O. L. et al. (2023). *Gaia DR3: Astrophysical parameters inference system (Apsis). I. Methods and content overview.* A&A, 674, A26. [DOI](https://doi.org/10.1051/0004-6361/202243688?utm_source=chatgpt.com).
10. Clarke, A. O. et al. (2020). *Identifying galaxies, quasars, and stars with machine learning.* A&A, 639, A84. [DOI](https://doi.org/10.1051/0004-6361/201936770?utm_source=chatgpt.com).

### **C. Classification and morphology**

11. Stetson, P. B. (1987). *DAOPHOT: A computer program for crowded-field stellar photometry.* PASP, 99, 191.
12. Lintott, C. et al. (2011). *Galaxy Zoo 1: data release of morphological classifications for nearly 900,000 galaxies.* MNRAS, 410, 166. [DOI](https://doi.org/10.1111/j.1365-2966.2010.17432.x?utm_source=chatgpt.com).
13. Fraix-Burnet, D. (2023). *Machine learning and galaxy morphology: for what purpose?* MNRAS, 523, 3974. [DOI](https://doi.org/10.1093/mnras/stad1654?utm_source=chatgpt.com).
14. *Machine learning-based photometric classification of galaxies, quasars, emission-line galaxies, and stars* (2024). MNRAS, 527, 4677. [DOI](https://doi.org/10.1093/mnras/stad3436?utm_source=chatgpt.com).

### **D. Transients and moving objects**

15. Zackay, B., Ofek, E. O. & Gal-Yam, A. (2016). *Proper Image Subtraction—Optimal Transient Detection, Photometry, and Hypothesis Testing.* ApJ, 830, 27. [DOI](https://doi.org/10.3847/0004-637X/830/1/27?utm_source=chatgpt.com).
16. *A-Track: A new approach for detection of moving objects in FITS images* (2016). Computer Physics Communications, 207, 524–530. [DOI](https://doi.org/10.1016/j.cpc.2016.07.023?utm_source=chatgpt.com).
17. *fink, a new generation of broker for the LSST community* (2021). MNRAS, 501, 3272. [DOI](https://doi.org/10.1093/mnras/staa3602?utm_source=chatgpt.com).
18. Fraga, B. M. O. et al. (2024). *Transient classifiers for Fink: Benchmarks for LSST.* A&A, 692, A208. [DOI](https://doi.org/10.1051/0004-6361/202450370?utm_source=chatgpt.com).

### **E. Astronomical visualization and interoperability**

19. Bonnarel, F. et al. (2000). *The ALADIN interactive sky atlas: A reference tool for identification of astronomical sources.* A&AS, 143, 33. [DOI](https://doi.org/10.1051/aas:2000331?utm_source=chatgpt.com).
20. Fernique, P. et al. (2015). *Hierarchical progressive surveys: Multi-resolution HEALPix data structures for astronomical images, catalogues, and 3-dimensional data cubes.* A&A, 578, A114. [DOI](https://doi.org/10.1051/0004-6361/201526075?utm_source=chatgpt.com).

These 20 references give you a solid scientific starting point. The list is curated rather than exhaustive: specialist work on planetary nebulae, cometary morphology, transient spectroscopy, variable-star classification and astronomical anomaly detection deserves additional dedicated literature searches.

## **14. Where I would concentrate research first**

My strongest recommendation is to study three existing systems in depth:

[Astrometry.net](https://images.openai.com/static-rsc-4/X0_Z80gLnnXEf9dvYLDcgSFWkRoy1vge66b3eXY_PqprstM6hsskVGvVTcTcnGoFekHCaIErdz-EwZnnOejULx-HyGJr-36dtSEJemZdyJiE9ZaCeLsuowr5Y0HnsquB5IO0rBnAo-P3bfzJAp3X0iA68jLclvs40IqTQgbhRwA?purpose=inline)

**1. Astrometry.net**

Learn how to solve an arbitrary astronomical photograph and generate reliable WCS information.

[Source code](https://github.com/dstndstn/astrometry.net?utm_source=chatgpt.com).

[Annotations — Siril 1.2.6 documentation](https://images.openai.com/static-rsc-4/ex-2PkU4M-OjPiqv5nmGnmMOwrJm4zAb0F7iik1Xht-1yrspivh_WuOH3fmrXnnqCClO_W1Atmmd3Etk7YN5aDA4HusHoL9h7tYRtsuKaHHKVN1ntnsZfUW6rccc8aDa1eZMrXyEzz6tcX0i0Y4G1H4qmOrh90NghUfqwSHjU1s?purpose=inline)

**2. Siril**

Learn how to implement the actual user experience of astrometry, catalogue queries and object annotations in an astrophotography application.

[Source code](https://github.com/AstroSharp/siril?utm_source=chatgpt.com).

[Planetarium software to display solved images within - Astronomy Software & Computers - Cloudy Nights](https://images.openai.com/static-rsc-4/trkqxzWiKhk5xhLBOtSE28JLpKHgE-xnEKXzLQPjEOw7w2xxERjpRu5-Q5eI5BTY7RIJStI1GvE6HcAu5fWSholPuQ3EJLOcFT5mkyjoVbFfYZwUjo_sp3IKCy_9WNxfY4E98c0Ct_91yGhaI-ggC1bVWgvW7jBbXvrrjL9nRVA?purpose=inline)

**3. Aladin**

Learn how to build a comprehensive astronomical identification and visualization environment.

[Aladin Lite source code](https://github.com/cds-astro/aladin-lite?utm_source=chatgpt.com).

The three systems address different parts of the problem. Combining their scientific and engineering approaches would be much more productive than attempting to reproduce everything independently.

**My final assessment:** the most valuable first milestone for SkyStacker is not AI-based object recognition. It is a reliable WCS pipeline combined with multi-catalogue cross-matching and an interactive annotation layer. Once those foundations work, transient detection, morphology and unknown-object analysis become much more meaningful.

One further point: the distinction between image registration and absolute astrometry should remain explicit in the codebase. Your existing triangle/RANSAC registration can align exposures, but it cannot independently establish the celestial identity of their pixels.

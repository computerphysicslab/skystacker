# **Synthetic generation of calibration frames for astrophotography**

The research question is whether we can computationally generate **synthetic bias, dark and flat frames** when the original calibration images were not captured, and whether these synthetic frames can produce scientifically reliable calibration results.

This is a legitimate image-processing and sensor-modelling problem. There are techniques worth investigating, although an important distinction must be maintained:

- **Synthetic calibration:** modelling the physical and electronic characteristics of the camera sensor to construct replacement calibration frames.
- **Mathematical correction:** estimating and removing specific image defects directly from the light frames.
- **Empirical reconstruction:** extracting information from existing light frames to estimate missing calibration data.

These approaches are related, but they are not interchangeable. A synthetic frame can reproduce a known systematic pattern without reproducing the actual random noise or time-dependent sensor behaviour of the original exposure.

## **1. Synthetic bias frames**

Technically feasible

A bias frame records the electronic offset introduced during sensor readout, together with spatially structured readout effects and noise.

A synthetic bias can be generated using:

- A constant pedestal corresponding to the camera's black-level offset.
- A per-pixel offset map, if one can be recovered from previous images.
- Row and column patterns representing readout electronics.
- A model of read noise and its statistical distribution.

However, adding random Gaussian noise does not reconstruct the actual sensor's readout pattern.

**Particularly relevant research and documentation:**

1\. Siril — Synthetic biases. A practical investigation of when a synthetic offset can replace a master bias, particularly when calibrating flats. It also explains why subtracting a bias is not equivalent to removing read noise.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://siril.org\&sz=32)

Synthetic biases+1



&#x20;  [Synthetic biases – Siril tutorial](https://siril.org/tutorials/synthetic-biases/?utm_source=chatgpt.com).

2\. Astropy CCD Reduction Guide — Provides mathematical and computational foundations for bias, dark and flat calibration, including simulated detector sensitivity.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://www.astropy.org\&sz=32)

CCD Data Reduction Guide+1



&#x20;  [Nonuniform sensitivity and simulated flat images](https://www.astropy.org/ccd-reduction-and-photometry-guide/v/2.0.1/notebooks/01-04-Nonuniform-sensitivity.html?utm_source=chatgpt.com).

**Practical conclusion:** A synthetic bias is one of the easiest calibration frames to approximate, especially if the camera's offset is stable and known. But a constant value cannot reproduce spatially varying electronic patterns.

## **2. Synthetic dark frames**

Partially feasible

This is more complicated because dark frames contain several physically distinct phenomena:

- Dark current accumulated during exposure.
- Hot and warm pixels.
- Fixed-pattern dark-current variations.
- Amp glow.
- Thermal noise.
- Readout offset and read noise.

A useful physical model is:

\\[ D(x,y)=B(x,y)+t\\,I_d(x,y,T)+N(x,y) \\]

Where:

- \\(B(x,y)\\): bias pattern.
- \\(t\\): exposure duration.
- \\(I_d(x,y,T)\\): dark-current map, dependent on pixel and sensor temperature.
- \\(N(x,y)\\): stochastic noise.

A synthetic dark can be constructed by estimating these components separately.

**Three interesting approaches:**

**A. Temperature-dependent dark-current modelling**

Build a dark-current map from camera characterisation data, ideally using exposures at different temperatures and exposure times.

Dark current is strongly temperature-dependent. A model based on a single nominal temperature may be inadequate if the sensor temperature changes significantly.

**B. Hot-pixel map reconstruction**

Use previous images to identify persistent defective pixels. A statistical model can identify pixels that repeatedly show abnormally high values.

**C. Amp-glow modelling**

Estimate the spatial structure of the amplifier glow, potentially using previous dark frames or multiple science exposures. A smooth synthetic glow pattern may approximate the broad structure, but it will not reproduce the exact exposure-dependent signal.

An important scientific reference is the OSIRIS-REx camera calibration study, which measures dark current as a function of exposure and temperature and constructs master darks from multiple observations.

[Ground and In-Flight Calibration of the OSIRIS-REx Camera Suite](https://pmc.ncbi.nlm.nih.gov/articles/PMC6979463/?utm_source=chatgpt.com).&#x20;

[image](https://www.google.com/s2/favicons?domain=https://pmc.ncbi.nlm.nih.gov\&sz=32)

PubMed Central (PMC)+1



Another useful source is the 2026 paper on physics-based noise synthesis for astronomical imaging, which models dark-current noise, readout effects, photo-response non-uniformity and hot pixels.

[Denoising the Deep Sky: Physics-Based CCD Noise Formation for Astronomical Imaging (2026)](https://arxiv.org/abs/2601.23276?utm_source=chatgpt.com).

Although its principal objective is denoising and training-data generation, its physical noise-generation approach is relevant to synthetic calibration.

## **3. Synthetic flat frames**

Feasible for some components

Flat frames are particularly interesting because their structure is partly optical and partly detector-dependent.

A simplified model is:

\\[ F(x,y)=V(x,y)\\,P(x,y)\\,Q(x,y) \\]

Where:

- \\(V(x,y)\\): optical illumination and vignetting.
- \\(P(x,y)\\): pixel-response non-uniformity.
- \\(Q(x,y)\\): dust shadows and optical obstructions.

The challenge is reconstructing these three components without an actual flat exposure.

[Calibration — Siril 1.5.0 documentation](https://images.openai.com/static-rsc-4/ZMyLI7sII6UGVb-zZ-m6pUda39SsvDB9jTULxIcubbC6uBvzcgaMim48BXGU7xXUh0DYxVNwmGIbXStcvPUCywC6QJe6cvHUrjfVc25qTeSpnvo8VLFuFYrmOzR8-AUXwP3888QT8wADb4wN2b2TsCzPjd4MKYu5qoJJczj1KKE?purpose=inline)

[A Flat Frame - FLC Observatory](https://images.openai.com/static-rsc-4/goZWEJL1BTnh0OLQJMnnsxg0uT2ZYuiTAm7_uNGig4yN25N4m8bBHCrxfHOKoZsbdJgQFYeXTLRYqH75eFxxZ8YdvR8pO_yIOhQ6WT7vXPWMoctWJHJuiTiV1cLiU2-Hq4fTkb1JL1AjLVCQwxGRCz-0NxLOiN8L8S8zovGtqz4?purpose=inline)

[SBIG Aluma 8300 Flat: why this vignetting? - Beginning Deep Sky Imaging - Cloudy Nights](https://images.openai.com/static-rsc-4/wj24pPkWFWTRmpzUMrqjEKQ9v-WyQ1nt0WU-_Tgm7cQ-096bvysELAOEbvSiXaNg2XkavbrLWlnR5foof3LbxlVS4SIKhCn6FcXPIUazoL3uuw-hrdihCB_xxfJZMAgmPxkIT8AJIutz2tj09gGGn74IW8Zu9QdjgS5OazHKvr4?purpose=inline)

8

There are four promising approaches:

**A. Analytical vignetting reconstruction**

Model the illumination falloff using radial functions, polynomial surfaces or optical models.

For example:

\\[ V(r)=1+a_2r^2+a_4r^4+a_6r^6 \\]

This can approximate smooth vignetting, but it cannot reliably reconstruct pixel-level sensitivity variations.

**B. Dust-shadow reconstruction**

Detect dust motes in existing images by analysing persistent circular or approximately circular structures. Their position and shape can be used to construct an approximate multiplicative correction map.

This is especially useful when the optical train has not changed.

**C. Flat reconstruction from science frames**

Estimate the flat-field response by combining multiple science images, rejecting astronomical sources and separating fixed optical features from moving celestial objects.

A particularly interesting scientific reference is:

\- Zaritsky et al. (2024), *The Astronomical Journal*, 168, 69.

The authors construct master flats from science images acquired during the same night, using at least 15 images and rejecting unsuitable illumination patterns.

[Zaritsky et al. (2024) – Astronomical Journal](https://pure.rug.nl/ws/portalfiles/portal/1065684953/Zaritsky_2024_AJ_168_69.pdf?utm_source=chatgpt.com).

This is not simply generating a flat from nothing: it is recovering the detector's illumination response from the available astronomical observations.

**D. Background modelling**

Use robust surface fitting, polynomial models, splines or spatial filtering to estimate smooth illumination variations.

However, there is a serious danger: extended galaxies, nebulae, Galactic cirrus and other low-surface-brightness structures can be mistaken for background gradients.

The following research is relevant:

\- Liu et al. (2023), *A Recipe for Unbiased Background Modeling in Deep Wide-Field Astronomical Images*.

[Read the paper on arXiv](https://arxiv.org/abs/2306.07922?utm_source=chatgpt.com).

This is important because an aggressive synthetic flat can remove real astronomical structures.

## **4. The most promising technique: self-calibration using multiple light frames**

Strong research potential

This is probably the most interesting direction for a software implementation.

Instead of creating a synthetic flat from an arbitrary mathematical function, use the actual light frames to estimate the fixed response of the optical system.

Suppose we have multiple images of the same astronomical field, captured with small telescope movements (dithering).

The stars and galaxies move relative to the detector, but the following remain approximately fixed:

- Vignetting.
- Pixel sensitivity variations.
- Dust shadows.
- Sensor defects.

By analysing the differences between the images, we can estimate the fixed detector response.

This approach has scientific precedents.

**Important references:**

1\. *A Synthesis of Star Calibration Techniques for Ground-Based Narrowband Electron-Multiplying Charge-Coupled Device Imagers Used in Auroral Photometry* (2016). Discusses two methods of flat-field construction: uniform illumination and dithering.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://agupubs.onlinelibrary.wiley.com\&sz=32)

AGU Journals



&#x20;  [Read the scientific paper](https://agupubs.onlinelibrary.wiley.com/doi/full/10.1002/2015JA022186?utm_source=chatgpt.com).

2\. Bramich and Freudling-related illumination modelling, extended in *Removal of systematics in photometric measurements: static and rotating illumination corrections in FORS2\@VLT data* (2014). Explores deriving illumination corrections from dithered stellar observations.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://academic.oup.com\&sz=32)

OUP Academic



&#x20;  [Read the MNRAS paper](https://academic.oup.com/mnras/article/438/2/1256/1008247?utm_source=chatgpt.com).

3\. *The Astronomical Journal* (2024), Zaritsky et al. — constructing master flats from science exposures.&#x20;

[image](https://www.google.com/s2/favicons?domain=https://pure.rug.nl\&sz=32)

Zaritsky et al.

.



This approach is especially attractive for astrophotography software because it doesn't require the user to have taken dedicated calibration exposures.

## **5. What can actually be reconstructed?**

Here is my assessment of the possibilities.

| Calibration component       | Synthetic reconstruction                  | Reliability              |
| --------------------------- | ----------------------------------------- | ------------------------ |
| Constant bias offset        | Estimate from metadata or existing images | High, if stable          |
| Spatial bias pattern        | Recover from historical frames            | Medium                   |
| Read noise                  | Simulate statistically                    | Statistical only         |
| Dark current                | Model using temperature and exposure      | Medium, if characterised |
| Hot pixels                  | Detect from existing exposures            | Medium–high              |
| Amp glow                    | Reconstruct from historical darks         | Medium                   |
| Vignetting                  | Analytical or empirical model             | Medium–high              |
| Dust shadows                | Detect and model from existing images     | Medium                   |
| Pixel response variations   | Estimate from multiple dithered images    | Medium                   |
| Flat illumination gradients | Model from science images                 | Medium–high              |

These ratings are qualitative, not measured performance guarantees.

## **6. Recommended scientific and technical bibliography**

The following resources are particularly relevant to implementing this functionality.

**Fundamental calibration**

1\. Astropy CCD Reduction Guide — Comprehensive treatment of CCD calibration, synthetic detector sensitivity and calibration mathematics.

[Nonuniform sensitivity](https://www.astropy.org/ccd-reduction-and-photometry-guide/v/2.0.1/notebooks/01-04-Nonuniform-sensitivity.html?utm_source=chatgpt.com) · [Calibrating the flats](https://www.astropy.org/ccd-reduction-and-photometry-guide/v/2.0.1/notebooks/05-03-Calibrating-the-flats.html?utm_source=chatgpt.com).

**Synthetic bias and practical calibration**

2\. Siril — Synthetic biases and mathematical analysis of calibration without complete calibration sets.

[Synthetic biases](https://siril.org/tutorials/synthetic-biases/?utm_source=chatgpt.com) · [Calibration documentation](https://siril.readthedocs.io/en/stable/preprocessing/calibration.html?utm_source=chatgpt.com).

**Detector physics and dark current**

3\. *Ground and In-Flight Calibration of the OSIRIS-REx Camera Suite* — Dark-current characterisation, temperature dependence and master calibration construction.

[Read the paper](https://pmc.ncbi.nlm.nih.gov/articles/PMC6979463/?utm_source=chatgpt.com).

4\. Liu et al. (2026), *Denoising the Deep Sky: Physics-Based CCD Noise Formation for Astronomical Imaging* — Physical modelling and synthesis of detector noise.

[Read on arXiv](https://arxiv.org/abs/2601.23276?utm_source=chatgpt.com).

**Flat-field reconstruction**

5\. Zaritsky et al. (2024), *The Astronomical Journal*, 168, 69 — Building master flats from astronomical science images.

[Read the paper](https://pure.rug.nl/ws/portalfiles/portal/1065684953/Zaritsky_2024_AJ_168_69.pdf?utm_source=chatgpt.com).

6\. *A Synthesis of Star Calibration Techniques...* (2016) — Dithering-based flat-field generation.

[Read the paper](https://agupubs.onlinelibrary.wiley.com/doi/full/10.1002/2015JA022186?utm_source=chatgpt.com).

7\. *Removal of Systematics in Photometric Measurements* (2014) — Illumination correction using dithered stellar observations.

[Read the paper](https://academic.oup.com/mnras/article/438/2/1256/1008247?utm_source=chatgpt.com).

**Background modelling and preservation of astronomical structures**

8\. Liu et al. (2023), *A Recipe for Unbiased Background Modeling in Deep Wide-Field Astronomical Images*.

[Read on arXiv](https://arxiv.org/abs/2306.07922?utm_source=chatgpt.com).

## **7. Final assessment: an implementation strategy**

For an astrophotography processing application, I would investigate a **Synthetic Calibration Engine** with three independent modules:

## **Synthetic Calibration Engine**

**1. Synthetic Bias Generator**

- Estimate electronic pedestal.
- Recover fixed-pattern offsets.
- Generate statistically modelled read noise.

**2. Synthetic Dark Generator**

- Temperature-dependent dark-current model.
- Exposure-time scaling.
- Hot-pixel database.
- Amp-glow estimation.

**3. Synthetic Flat Generator**

- Radial vignetting reconstruction.
- Dust-mote detection.
- Pixel-response estimation.
- Self-flat-fielding from dithered lights.
- Illumination-gradient modelling.

**4. Calibration confidence assessment**

Each generated frame should include:

- Estimated versus measured components.
- Confidence map.
- Reconstruction uncertainty.
- Warnings about unrecoverable information.

**My main conclusion:** Don't approach this merely as an image-generation problem. Approach it as a detector-characterisation and inverse-problem-solving system.

There is a fundamental difference between generating something that looks like a dark or flat and recovering the actual systematic errors present in the original exposures.

The most promising combination is to use physical sensor modelling for bias and dark frames, and empirical self-calibration from multiple science exposures for flat frames. Where the original calibration information is irretrievably missing, the software should explicitly report that limitation rather than silently manufacture a supposedly accurate calibration frame.

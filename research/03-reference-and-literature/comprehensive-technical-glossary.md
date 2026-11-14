# Comprehensive Technical Glossary (100 Definitions)

**Scope:** Complete domain-grouped glossary defining all 100 core architectural, mathematical, photometric, astrometric, diagnostic, and data-standard terms used across the SkyStacker research corpus.

---

## 1. System Architecture, Go Runtime & Hardware Performance

1. **SkyStacker** — Open-source CLI astrophotography processing engine written in Go.
2. **Local-First Architecture** — Design principle prioritizing local compute execution over cloud APIs.
3. **Static Compilation** — Compilation mode (`CGO_ENABLED=0`) producing self-contained binaries without dynamic OS dependencies.
4. **Spatial Tiles (`tile-based streaming`)** — Subdivision of image grids into uniform spatial blocks to bound memory consumption.
5. **Memory Buffer Recycling (`sync.Pool`)** — Object pool mechanism in Go used to recycle floating-point slice allocations.
6. **Heap Escape Analysis** — Compiler analysis determining whether variables are allocated on the stack or heap.
7. **Goroutine Leaks** — Condition where concurrent Go routines remain running in memory due to missing exit signals.
8. **Pipeline Concurrency Pattern** — Execution pattern linking concurrent stages via Go channels (`chan`).
9. **`context.Context` Cancellation** — Go mechanism for signaling process cancellation across goroutine hierarchies.
10. **Swiss Tables Architecture** — High-performance hash table design utilizing SIMD vector instructions and control byte metadata.
11. **Structure of Arrays (SoA)** — Data layout separating control metadata from data payloads to improve CPU cache efficiency.
12. **Array of Structures (AoS)** — Traditional data layout storing keys, values, and control fields contiguously.
13. **$H_1 / H_2$ Hash Splitting** — Hash division technique allocating 57 bits to bucket indexing ($H_1$) and 7 bits to fingerprinting ($H_2$).
14. **SIMD / SWAR Vector Inspection** — Single Instruction Multiple Data technique inspecting multiple control bytes concurrently.
15. **L1/L3 Cache Optimization** — Data structuring designed to fit within CPU L1/L3 cache lines (64 bytes).
16. **Load Factor Efficiency** — Ratio of stored entries to total table capacity; Swiss Tables maintain performance up to $87.5\%$.
17. **Roofline Performance Model** — Model relating computing speed (GFLOPS) to Arithmetic Intensity and RAM bandwidth.
18. **Arithmetic Intensity** — Ratio of floating-point calculations performed per byte of data transferred from memory.
19. **Compute-Bound Regime** — Execution state limited by CPU/GPU calculation speed rather than memory bandwidth.
20. **Memory-Bound Regime** — Execution state limited by system RAM bandwidth rather than compute unit capacity.
21. **Continuous Profiling (eBPF)** — Kernel-level profiling measuring CPU performance counters without instrumentation overhead.

---

## 2. Photometric Calibration & Subframe Quality

22. **Photometric Calibration** — Removal of sensor and optical path artifacts from raw imaging data.
23. **Master Bias Frame** — Calibration master recording electronic readout offset at minimal exposure duration.
24. **Master Dark Frame** — Calibration master recording exposure- and temperature-dependent thermal dark current.
25. **Master Flat Frame** — Calibration master mapping optical vignetting and dust mote shadows across the light path.
26. **Flat Normalization Factor ($\bar{I}_{\text{flat}}$)** — Mean scalar intensity value of the Master Flat frame used to preserve scale.
27. **Fixed-Pattern Noise (FPN)** — Spatial variations in pixel sensitivity across a digital image sensor.
28. **Full Width at Half Maximum (FWHM)** — Spatial width of a stellar intensity profile measured at half its peak amplitude.
29. **Stellar Eccentricity ($e$)** — Measure of stellar profile ellipticity; $0$ represents a circular profile.
30. **Quality Metric Score ($Q_i$)** — Dynamic rating combining FWHM, eccentricity, SNR, and PSF weight to score subframes.

---

## 3. Star Detection, Registration & Astrometry

31. **Subpixel Centroid Estimation** — Computation of true star center coordinates using intensity-weighted moments.
32. **8-Connected Component Labeling** — Algorithm isolating contiguous pixel clusters above sky background noise thresholds.
33. **Triangle Star-Pattern Matching** — Geometric registration matching scale- and rotation-invariant star triangles.
34. **Quad Matching Engine** — Registration method grouping 4 star centroids into irregular 2D tetragons to compute 6-digit hashes.
35. **RANSAC Filter** — Iterative algorithm estimating transformation models while ignoring false-match outliers.
36. **Lanczos / Sinc Resampling** — High-order subpixel interpolation kernel used during spatial frame alignment.
37. **Safe Overlapping Crop** — Bounding box reduction isolating spatial image areas valid across all aligned subframes.
38. **World Coordinate System (WCS)** — Standard mapping discrete image pixel coordinates $(x,y)$ to celestial coordinates $(\alpha, \delta)$.
39. **Simple Imaging Polynomial (SIP)** — 3rd-order polynomial model correcting optical field curvature and coma distortions.
40. **Blind Plate Solving** — Astrometric identification of celestial fields without initial position coordinates.
41. **Gaia DR3 Catalog** — High-precision astrometric and photometric catalog containing position data for over one billion stars.

---

## 4. Statistical Integration & Hardware Offloading

42. **Stochastic Noise Reduction** — Attenuation of random photon shot noise and sensor read noise via temporal integration.
43. **Square Root Scaling Law** — Rule dictating that integrated Signal-to-Noise Ratio scales as $\text{SNR}_{\text{stack}} = \sqrt{N} \cdot \text{SNR}_{\text{sub}}$.
44. **Arithmetic Mean Integration** — Stacking method averaging frame values; achieves maximum theoretical SNR for pure Gaussian noise.
45. **Median Integration** — Stacking method taking central value of sorted pixel stack; suppresses extreme transient outliers.
46. **Sigma-Clipping (`sigma`)** — Iterative rejection algorithm discarding pixels deviating beyond $k\sigma$ boundaries from median.
47. **Winsorized Sigma Clipping** — Rejection method replacing rejected outlier values with accepted boundary values before averaging.
48. **Percentile Clipping** — Outlier rejection method dropping extreme percentile boundaries; suited for small stack depths.
49. **Linear Fit Clipping** — Rejection method fitting a linear function along temporal pixel arrays to handle varying sky glow.
50. **Generalized Extreme Studentized Deviate (GESDT)** — Statistical test isolating multiple outlier values within small-to-moderate sample sizes.
51. **Maximum Pixel Stack (`Max Stack`)** — Diagnostic image composite retaining maximum recorded intensity per spatial pixel.
52. **Minimum Pixel Stack (`Min Stack`)** — Diagnostic image composite retaining minimum recorded intensity per spatial pixel.
53. **Hardware Abstraction Layer (HAL)** — Software interface decoupling image algorithms from underlying hardware backends.
54. **GPU Acceleration** — Transferring spatial tile calculation matrices to GPU compute pipelines (CUDA, Vulkan, Metal).

---

## 5. Demosaicing, Non-Linear Post-Processing & AI Models

55. **Bayer Color Filter Array (CFA)** — Matrix of RGB color filters placed over digital image sensor pixels.
56. **AstroC Demosaicing** — Debayering routine designed to reconstruct color in saturated stellar cores.
57. **AstroM Demosaicing** — Debayering routine rendering undersampled star profiles neutral white to prevent color artifacts.
58. **AstroSimple Debayering** — Demosaicing algorithm extracting RGB channels within localized $2 \times 2$ pixel neighborhoods.
59. **Linear Dynamic Range State** — Raw un-stretched image state where pixel values remain proportional to photon arrival counts.
60. **Non-Linear Stretch** — Mathematical transformation redistributing shadow dynamic range to render faint details visible.
61. **Generalized Hyperbolic Stretch (GHS)** — Non-linear stretching transform offering independent control over midtone contrast and highlights.
62. **Inverse Hyperbolic Sine Stretch (AsinhStretch)** — Stretch transform preserving color channel saturation across varying luminance levels.
63. **Midtone Transfer Function (MTF)** — Stretch curve remapping histogram values based on a selectable midtone inflection point.
64. **Background Gradient Removal** — 2D surface fitting subtracting spatial light pollution gradients from image fields.
65. **Background Structure Masking (`--mask-bg`)** — Masking technique protecting bright astronomical targets during background extraction passes.
66. **Chromatic Aberration Suppression** — Filtering pass targeting and removing purple/magenta color fringing around star profiles.
67. **Spectrophotometric Color Calibration (SPCC)** — Color balance calibration matching star colors against synthetic catalog spectra.
68. **Subtractive Chromatic Noise Reduction (SCNR)** — Algorithm suppressing non-astronomical green color casts caused by Bayer filter ratios.
69. **Wavelet *À Trous* Processing** — Multiscale spatial frequency decomposition isolating detail layers without halo artifacts.
70. **Starless Post-Processing** — Workflow separating stellar profiles from images to process background nebulae independently.
71. **Neural Star Removal (StarXTerminator)** — Convolutional neural network model isolating and removing stars from astronomical frames.
72. **AI Denoising (NoiseXTerminator)** — Deep learning network trained to attenuate background sensor noise while preserving structural detail.

---

## 6. Optical Diagnostics, Ephemeris Stacking & Photometry

73. **Half Flux Diameter (HFD)** — Diameter (in pixels) enclosing $50\%$ of a star profile's total cumulative light energy.
74. **Sensor Tilt Analysis** — Evaluation measuring focus variation across discrete spatial zones of an image sensor.
75. **9-Area Spatial Octagon** — Sensor partition layout dividing fields into 4 corners, 4 edges, and 1 center zone for tilt mapping.
76. **3-Corner Triangular Analysis** — Spatial partition mapping tilt variations to physical 120-degree tilt adapter adjustment screws.
77. **Off-Axis Field Aberration ($\Delta\text{HFD}$)** — Metric measuring field curvature as the HFD difference between outer corners and frame center.
78. **Stellar Aspect Ratio (Roundness)** — Ratio of maximum to minimum radial distance across a star profile; $1.0$ represents a circular profile.
79. **Sky Quality Meter (SQM)** — Measurement quantifying background sky brightness in units of $\text{mag/arcsec}^2$.
80. **Pickering Airmass Correction** — Atmospheric model adjusting background brightness metrics for target elevation above horizon.
81. **Minor Planet Center Format (MPC1992)** — Standardized 80-column text format reporting astrometric observations of minor planets and comets.
82. **Gaia-Based Nova Identification** — Verification engine flagging stars missing from Gaia catalogs or exhibiting anomalous flux increases.
83. **Dual-Stack Ephemeris Recombination** — Processing technique separating stars and comets to stack each along their respective motion vectors.
84. **Orbital Motion Alignment** — Differential alignment matching non-sidereal targets using ephemeris drift vectors ($\Delta\alpha, \Delta\delta$).
85. **Ensemble Photometric Zero-Point ($m_{0,\text{ensemble}}$)** — Weighted mean calibration reference derived from multiple catalog stars.
86. **Median Absolute Deviation (MAD)** — Robust statistical scale estimator measuring dispersion resilient to extreme outliers.
87. **Passband System Transformation** — Conversion equations transforming instrumental magnitude colors into standard photometric filters ($B, V, R$).
88. **Local Normalization Correction (LNC)** — Equalization algorithm matching spatial background variations across subframe sequences.
89. **Drizzle Subpixel Integration** — Linear image reconstruction algorithm restoring spatial resolution on undersampled subframe sets.

---

## 7. Scientific File Formats, Metadata & Build Hygiene

90. **IEEE 754 32-Bit Float Format** — Binary floating-point data representation preserving scientific dynamic range without clipping.
91. **TIFF 6.0 Standard** — Scientific raster file specification supporting floating-point sample formats (`SampleFormat=3`).
92. **64-Bit BigTIFF Format** — File standard extension breaking the classic $4\text{ GB}$ structural limit using 64-bit offsets (`LONG8`).
93. **Image File Directory (IFD)** — Internal structure in TIFF files storing metadata tags, offset pointers, and channel dimensions.
94. **Flexible Image Transport System (FITS)** — NASA/IAU standard scientific file format utilizing 2880-byte header blocks.
95. **Rice Compression (`.fits.fz`)** — Tile-based lossless compression algorithm reducing FITS storage requirements.
96. **Extensible Image Serialization Format (XISF)** — Astronomical file standard using XML metadata and compressed memory streams.
97. **Zero-Allocation Metadata Reader (`imagemeta`)** — Go package reading EXIF/IPTC metadata headers without triggering heap allocation.
98. **Code Smell Refactoring** — Systematic code cleanup eliminating anti-patterns to improve performance and maintainability.
99. **`CGO_ENABLED=0` Native Compilation** — Build setting disabling C bindings to produce pure statically compiled Go binaries.
100. **Audit Telemetry Log (`telemetry_audit.log`)** — Structured JSON execution trace recording rejection counts, HFD metrics, and system performance.

# Comprehensive Source Catalog, Runtime Microarchitecture & Keyword Index

**Scope:** Consolidated catalog of all 50 primary project references, systems-level microarchitecture notes (Go runtime internals, eBPF continuous profiling, and the Roofline performance model), and a categorized 100-term technical keyword index.

---

## 1. Master Catalog of 50 Primary Project Sources

This table consolidates all 41 original notebook references together with the 9 extended references covering AI-based processing, WCS/SIP astrometry, FITS/XISF standards, Go concurrency, and GPU acceleration:

| ID | Source Title | Source Type | Technical Domain and Contribution |
| :--- | :--- | :--- | :--- |
| **01** | ASTAP, Astrometric Stacking Program - HNSky | URL | Astrometric alignment algorithms, mosaic stacking, plate solving, and calibration. |
| **02** | Architectural Specification and Mathematical Foundations for a Local-First, Concurrent Go CLI Engine for Astronomical Image Integration | Markdown Specification | Go CLI engine architecture, memory management with `sync.Pool`, spatial tiles, and IEEE 754 `float32` TIFF encoding. |
| **03** | Asinh transformation [Siril 1.0] | URL | Mathematical foundations and color preservation through the inverse hyperbolic sine transformation. |
| **04** | AsinhStretch — Astropy v8.0.2 | URL | Standard reference implementation for the non-linear `AsinhStretch` in astrophotography. |
| **05** | Astrophotography — Art and Science - Bluewater Astronomical Society | PDF Document | Comparative workflows between Siril, PixInsight, Affinity Photo, and complementary AI tools. |
| **06** | Best Deep-Sky Stacking Software for Mac (2026 Guide) | URL | RAM memory requirements, macOS Apple Silicon scalability, and comparative analysis of stacking software. |
| **07** | Best Practices for Identifying and Eliminating Code Smells - Codacy \| Blog | URL | Static analysis techniques and elimination of degrading patterns (code smells) in software development. |
| **08** | BornAgain-21 TIFF | URL | TIFF format specifications for high-precision scientific data storage. |
| **09** | C++ Swiss Table Architecture: SoA Layout and H1/H2 Hash Splitting - StudyPlan.dev | URL | Open addressing hash table architecture, SoA design, control bytes, and $H_1/H_2$ signatures. |
| **10** | Comprehensive Technical Architecture of Next-Generation Astronomical Image Processing Engines: Algorithms, Photometry, and High-Performance Pipelines | Markdown Specification | Photometric quality metrics ($Q_i$), hardware abstraction (HAL), GPU Compute, PSF deconvolution, and DAG. |
| **11** | Faster Go maps with Swiss Tables - The Go Programming Language | URL | Internal replacement of the `map` type in Go 1.24 with Swiss Tables and extended table directories. |
| **12** | GitHub - bep/imagemeta: Go library for reading EXIF, IPTC and XMP image meta data... | URL | Efficient extraction of EXIF, IPTC, and XMP metadata in Go without dynamic heap allocations. |
| **13** | GitHub - sramuschkat/Siril-Scripts: Python scripts for richer Siril image experiences. | URL | Automation scripts, sensor banding diagnostics, aperture photometry, and calibration algorithms. |
| **14** | Glossary [Siril 1.0] | URL | Formal definition of astronomical technical terms, IKSS estimators, FWHM, PSF, and Siril commands. |
| **15** | Guide to Preprocessing of Raw Data with PixInsight - AWS | PDF Document | Optimized dark calibration, local normalization (*LocalNormalization*), and noise evaluation in PixInsight. |
| **16** | How Go Maps Work: From Buckets to Swiss Tables - DZone | URL | Internal mechanics of Go maps, CPU cycle comparison, memory impact, and quadratic probing. |
| **17** | PI SubframeSelector Script - Beginning Deep Sky Imaging - Cloudy Nights | URL | Individual frame weighting based on FWHM, eccentricity, and signal-to-noise ratio. |
| **18** | PIXINSIGHT Process Tutorial: Subframe Selector | YouTube Video | Practical methodology for evaluating and discarding frames prior to master integration. |
| **19** | PixInsight 1.9.6 Lockhart Released | URL | New measurement processes (`ImageAnalysis`), parallel XISF compression, BigTIFF, and algorithm acceleration. |
| **20** | Pixinsight: What does LocalNormalization do? - Experienced Deep Sky Imaging - Cloudy Nights | URL | Local gradient correction mechanism using reference images and `.xnml` files. |
| **21** | Preprocessing in PixInsight (Part 4) - Local Normalization (8 23 20) | YouTube Video | Tutorial on illumination equalization and local normalization application across image sequences. |
| **22** | README.md | Markdown Specification | SkyStacker project overview, CLI capabilities, static compilation, and integrated self-tests. |
| **23** | Revision 6.0 - ITU | PDF Document | Official technical specification of the TIFF 6.0 standard, IFD structure, tags, and data types. |
| **24** | Siril:Tutorial preprocessing - FreeAstro | URL | Fundamental preprocessing equations, flat calibration, and optical shadow correction. |
| **25** | Stacking Options panel (Astrophotography Stack Studio) - Affinity Help Center | URL | Stacking methods and rejection parameters in Affinity Photo. |
| **26** | Stacking — Siril 1.4.4 documentation | URL | Scale estimators (IKSS, MAD), frame selection by percentage/k-sigma, and additive/multiplicative normalization. |
| **27** | Static Code Analysis: Top 7 Methods, Pros/Cons and Best Practices - Oligo Security | URL | Static code analysis strategies for vulnerability prevention and software optimization. |
| **28** | SwissMap: A smaller, faster Golang Hash Table \| DoltHub Blog | URL | Go port of Swiss Tables (`dolthub/swiss`), fast modulo reduction (`fastModN`), and SIMD/AVO optimizations. |
| **29** | TECHNICAL_SPEC.md | Markdown Specification | Detailed technical specification of SkyStacker `v0.5.0`: CLI contract, 6-stage pipeline, and numerical limits. |
| **30** | TIFF file format: specification, software and libraries · fileformats.dev | URL | Internal TIFF structure, strips, tiles, 64-bit BigTIFF, and GeoTIFF extensions. |
| **31** | TIFF: Summary from the Encyclopedia of Graphics File Formats - FileFormat.Info | URL | Historical and structural summary of TIFF 6.0 formats, byte order, and IEEE data types. |
| **32** | Tiff - MATLAB Gateway to LibTIFF library routines - MathWorks | URL | Properties and functions of the MATLAB LibTIFF interface for reading and writing scientific TIFF tags. |
| **33** | Two Indexed Hash Tables \| Vladimir Makarov | URL | Double-indexed hash table design for collision minimization and L1 cache optimization. |
| **34** | Understanding Code Smells: Types, Refactoring & Best Practices - Aubergine Solutions | URL | Code smell classification and refactoring strategies for system maintainability. |
| **35** | Understanding Image Integration: Why Stacking Makes Your Astrophotos Better | URL | Stochastic noise theory, SNR gain by the square root law, and outlier rejection methods. |
| **36** | Understanding code smells and how refactoring can help - TechTarget | URL | Technical debt diagnosis and continuous source code improvement. |
| **37** | User guide | PDF Document | Siril 0.9.6 user manual: preprocessing functions, registration, aperture photometry, and CLI commands. |
| **38** | What Is Code Review? \| Types, Process, and Tools Guide - Sonar | URL | Code review methodologies to ensure quality standards in concurrent repositories. |
| **39** | geotiff package - github.com/gden173/geotiff/geotiff - Go Packages | URL | Implementation of GeoTIFF readers in Go with support for spatial coordinates and strip/tile structures. |
| **40** | imagemeta package - github.com/evanoberholster/imagemeta - Go Packages | URL | Low-memory impact decoding of EXIF and MakerNotes tags for RAW and TIFF files in Go. |
| **41** | tifffile \| Documentation - Christoph Gohlke | URL | Python scientific library for manipulating NumPy arrays in TIFF, BigTIFF, and OME-TIFF formats. |
| **42** | FITS Standard - NASA/IAU | PDF Document | Structure of the FITS (Flexible Image Transport System) format, a global standard for storage and metadata in professional astronomy. |
| **43** | XISF Specification 1.0 - Pleiades Astrophoto | Specification | Architecture of the Extensible Image Serialization Format from PixInsight, a robust alternative to TIFF for preserving processing history. |
| **44** | GraXpert: AI-based Background Extraction & Denoising | URL | Application of artificial intelligence and neural networks for precise removal of complex light pollution gradients. |
| **45** | Advanced Wavelet Processing in Astrophotography | Scientific Article | Mathematical implementation of *à trous* wavelet transforms for isolating and enhancing nebular structures at multiple scales. |
| **46** | Understanding SIP (Simple Imaging Polynomial) Distortion - Astropy | URL | Advanced mathematical modeling of optical distortions for accurate correction in spatial WCS coordinates. |
| **47** | Concurrency in Go: Advanced Pipelines and Context - Go Blog | URL | Advanced concurrency design patterns for safe goroutine lifecycle management using `context.Context`. |
| **48** | FITS WCS Standard (World Coordinate System) | PDF Document | Specification of transformation algorithms mapping physical pixel coordinates to celestial equatorial coordinates (RA/Dec). |
| **49** | The Role of CUDA and Vulkan in GPU-Accelerated Image Integration | Technical Article | Hardware abstractions (HAL) to offload intensive mathematical computations to the GPU in astrophotography CLI engines. |
| **50** | NoiseXTerminator & StarXTerminator Architecture - RC-Astro | URL | Convolutional Neural Networks (CNNs) applied to the deconstruction of astrophotographic images for star separation. |

---

## 2. Software Engineering, Profiling & Hardware Performance Models

### 2.1 Code Smell Detection and Refactoring Standards
SkyStacker's codebase adheres to static analysis rules designed to eliminate performance-degrading patterns:
* **Complex or Lengthy Functions:** Decomposing monolithic routines into atomic modules with single responsibility.
* **Hidden Heap Allocations:** Eliminating implicit interface conversions (`any`) that force stack variables to escape to the heap.
* **I/O Coupling:** Decoupling disk reads from numerical loops using bounded concurrent channel buffers.

### 2.2 Continuous Profiling with eBPF
To inspect runtime performance in production without instrumentation overhead, kernel-level continuous profilers based on **eBPF** (*Extended Berkeley Packet Filter*), such as **Parca** or **Pyroscope**, monitor CPU hardware performance counters and memory allocations, generating low-overhead flamegraphs of active goroutines.

### 2.3 The Roofline Performance Model
Numerical throughput in SkyStacker is evaluated using the **Roofline Model**, which bounds achievable execution speed (in GFLOPS) by peak hardware compute capacity and memory bandwidth scaled by **Arithmetic Intensity** $I$ (floating-point operations per byte transferred from RAM):

$$\text{Performance (GFLOPS)} = \min\left( \text{Peak ALU Performance}, \text{RAM Bandwidth} \times I \right)$$

* **Memory-Bound Regimes:** Operations with low arithmetic intensity (e.g., simple pixel addition) are bottlenecked by DRAM bandwidth.
* **Compute-Bound Regimes:** Algorithms with high arithmetic intensity (e.g., FFT convolutions, subpixel Lanczos/Sinc interpolation, iterative Winsorized sorting) are bounded by CPU vector execution units (AVX2/AVX-512) or GPU compute shaders.

By partitioning images into L2-cache-friendly spatial tiles, SkyStacker maximizes data locality and shifts execution toward the compute-bound regime.

---

## 3. Categorized Technical Keyword Index (100 Core Concepts)

### Core System & Architecture
1. SkyStacker
2. Astrometric Stacking Program
3. Go CLI Engine
4. Local-First Design
5. Static Compilation
6. `CGO_ENABLED=0`
7. Spatial Tiles
8. IEEE 754 `float32` TIFF
9. Garbage Collector Optimization
10. `sync.Pool`
11. `TileBuffer`
12. Goroutine leaks
13. `context.Context`
14. Concurrency Pipeline
15. Hardware Abstraction Layer (HAL)
16. GPU Compute
17. Metal compute shaders
18. CUDA
19. Vulkan

### Data Structures & Performance Optimization
20. Swiss Table Architecture
21. SoA Layout (*Structure of Arrays*)
22. $H_1/H_2$ Hash Splitting
23. Control bytes
24. SIMD vector register
25. SWAR technique
26. Load factor
27. Escape analysis
28. Fingerprint ($H_2$)
29. Roofline Model
30. Arithmetic Intensity
31. Memory-Bound Regimes
32. Compute-Bound Regimes
33. Code Smell Detection
34. Continuous Profiling
35. eBPF

### Calibration & Photometric Preprocessing
36. Master Bias
37. Master Dark
38. Master Flat
39. Photometric calibration
40. Vignetting correction
41. Photometric quality metrics ($Q_i$)
42. `SubframeSelector`
43. Noise evaluation
44. `LocalNormalization`
45. FWHM (Full Width at Half Maximum)
46. Eccentricity
47. Signal-to-noise ratio (SNR)
48. `PSFWeight`

### Detection, Registration & Astrometry
49. Point Spread Function (PSF)
50. Centroid calculation
51. Aperture photometry
52. Star-Pattern Matching
53. Similarity Transformation
54. RANSAC Filtering
55. Blind Plate Solving
56. World Coordinate System (WCS)
57. SIP Distortion (Simple Imaging Polynomial)
58. Gaia DR3
59. Right Ascension (RA)
60. Declination (Dec)
61. PSF deconvolution

### Stacking & Pixel Rejection Algorithms
62. Stochastic noise theory
63. Arithmetic Mean
64. Median Stacking
65. Sigma Clipping
66. Winsorized Sigma
67. Linear Fit Clipping
68. GESDT test (Extreme Studentized Deviate)
69. Pixel Maximum (`Max Stack`)
70. Pixel Minimum (`Min Stack`)
71. Pixel rejection filters

### File Formats & Metadata Processing
72. TIFF 6.0
73. BigTIFF format
74. Image File Directory (IFD)
75. EXIF metadata
76. IPTC metadata
77. XMP metadata
78. `imagemeta` package
79. Flexible Image Transport System (FITS)
80. Extensible Image Serialization Format (XISF)

### Non-Linear Post-Processing & AI Integration
81. Asinh transformation
82. `AsinhStretch`
83. `ArcsinhStretch`
84. Generalized Hyperbolic Stretch (GHS)
85. Midtone Transfer Function (MTF)
86. Spectrophotometric Color Calibration (SPCC)
87. Subtractive Chromatic Noise Reduction (SCNR)
88. Background Extraction
89. Chromatic Aberration Correction
90. Unsharp Mask
91. Wavelet Processing
92. Wavelet *À Trous* Transforms
93. StarXTerminator
94. NoiseXTerminator
95. GraXpert
96. Calibration algorithms
97. IKSS estimator
98. Directed Acyclic Graph (DAG)
99. Non-linear post-processing
100. Starless Workflows

## v0.6.0 map

`main.go` remains a single file. Logical blocks now include FITS decode and
debayer, Gaia/HEALPix, TAN WCS, calibration masters, self-flat, SER, and
comet translation. There is no package split. Registered self-tests: 177.

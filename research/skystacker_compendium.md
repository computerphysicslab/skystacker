# Comprehensive Technical Compendium of SkyStacker Sources and Technological Architecture

This document constitutes an exhaustive technical and documentary synthesis of the 41 original sources (now expanded to 49) integrated into the **SkyStacker** project. Its purpose is to provide a structured compendium encompassing both the complete catalog of references and the entirety of the mathematical, algorithmic, microarchitectural, and software engineering foundations described in the project's literature. To improve and update the documentation, unreleased content regarding advanced AI-based processing, WCS astrometry, standardized astronomical formats (FITS/XISF), and GPU acceleration has been incorporated.

---

## 1. General Catalog of Project Sources

Below is a detailed list of all primary information sources selected in the notebook. The table specifies the internal identifier, the official title of the document or resource, its original format, and its domain of contribution to the SkyStacker project:

| ID | Source Title | Source Type | Technical Domain and Contribution |
| :--- | :--- | :--- | :--- |
| **01** | ASTAP, Astrometric Stacking Program - HNSky | URL | Astrometric alignment algorithms, mosaic stacking, plate solving, and calibration. |
| **02** | Architectural Specification and Mathematical Foundations for a Local-First, Concurrent Go CLI Engine for Astronomical Image Integration | Markdown Specification | Go CLI engine architecture, memory management with `sync.Pool`, spatial tiles, and IEEE 754 float32 TIFF encoding. |
| **03** | Asinh transformation [Siril 1.0] | URL | Mathematical foundations and color preservation through the inverse hyperbolic sine transformation. |
| **04** | AsinhStretch — Astropy v8.0.2 | URL | Standard reference implementation for the non-linear AsinhStretch in astrophotography. |
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
| **29** | TECHNICAL_SPEC.md | Markdown Specification | Detailed technical specification of SkyStacker v0.5.0: CLI contract, 6-stage pipeline, and numerical limits. |
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
| **49** | The Role of CUDA and Vulkan in GPU-Accelerated Image Integration | Technical Article | Hardware abstractions (HAL) to offload intensive mathematical computations to graphics processing units. |

---

## 2. Technical Foundations & Architectural Synthesis

### 2.1 World Coordinate System (WCS) Astrometry & SIP Distortion Correction
The **World Coordinate System (WCS)** standardizes transformations mapping discrete image pixel coordinates $(x, y)$ to celestial equatorial coordinates $(\alpha, \delta)$ (Right Ascension and Declination).
* **Gnomonic Projection (TAN):** Projects spherical coordinates onto a tangent plane.
* **Simple Imaging Polynomial (SIP):** Extends WCS by modeling optical field distortions introduced by camera lenses and focal reducers via explicit polynomial expressions:
  $$f(u, v) = \sum_{p+q \le A} A_{p,q} \, u^p v^q, \quad g(u, v) = \sum_{p+q \le B} B_{p,q} \, u^p v^q$$
  where $u, v$ are standard focal-plane coordinates, and $A_{p,q}, B_{p,q}$ are the SIP distortion coefficients stored directly within FITS file headers.

### 2.2 Deep Learning AI Denoising & Background Extraction
Modern astrophotography pipelines complement traditional rejection stacking with deep neural models (e.g., U-Nets, Noise2Noise, self-supervised networks):
* **Background Gradient Removal:** Neural networks isolate complex non-linear light pollution gradients without requiring manual background sample placement.
* **Supervised vs. Self-Supervised Denoising:** U-Nets trained on synthetic or high-SNR astronomical master frames achieve substantial signal-to-noise ratio gains ($>1.5\times$) while retaining point-source stellar flux and diffraction spikes.
* **Pre-processing Requirements:** Outlier artifacts like cosmic rays must be rejected prior to neural inference to prevent them from being mistaken for stellar point sources.

### 2.3 Multiscale Wavelet Analysis (*À Trous* Transform)
The *à trous* ("with holes") discrete wavelet transform provides scale-invariant decomposition of deep-sky images:
* **Scale Separation:** Decomposes an image $I$ into an array of wavelet scales $w_k$ and a residual smooth background layer $c_N$:
  $$I(x, y) = c_N(x, y) + \sum_{k=1}^{N} w_k(x, y)$$
* **Feature Isolation:** Finer detail layers ($w_1, w_2$) isolate high-frequency sensor noise and small stars, while coarser layers ($w_3..w_N$) isolate diffuse nebular structures. This multiresolution structure allows targeted noise suppression and contrast enhancement without inducing halo artifacts.

### 2.4 GPU Offloading & Execution Pipelines
To maintain real-time performance on high-resolution sensor frames, image integration engines employ Hardware Abstraction Layers (HAL):
* **Execution APIs:** CUDA and Vulkan backends execute dense pixel-level matrix operations (e.g., interpolation, cross-correlation alignment, frame normalization, and weighting).
* **Parallel Pipelines:** Utilizing concurrent worker goroutines with `context.Context` cancellation and thread-safe memory allocations (`sync.Pool`), CPU data transfer overhead is minimized while offloading heavy transformations to GPU compute queues.
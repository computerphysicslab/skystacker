# Comprehensive Technical Compendium of SkyStacker Sources and Technological Architecture

This document constitutes an exhaustive technical and documentary synthesis of the 41 original sources (now expanded to 50) integrated into the **SkyStacker** project. Its purpose is to provide a structured compendium encompassing both the complete catalog of references and the entirety of the mathematical, algorithmic, microarchitectural, and software engineering foundations described in the project's literature. To improve and update the documentation, unreleased content regarding advanced AI-based processing, WCS astrometry, standardized astronomical formats (FITS/XISF), and GPU acceleration has been incorporated.

---

## 1. General Catalog of Project Sources

Below is a detailed list of all primary information sources selected in the notebook. The table specifies the internal identifier, the official title of the document or resource, its original format, and its domain of contribution to the SkyStacker project (including the 9 references added in this extended revision):

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
| **49** | The Role of CUDA and Vulkan in GPU-Accelerated Image Integration | Technical Article | Hardware abstractions (HAL) to offload intensive mathematical calculations to the GPU in astrophotography CLI engines. |
| **50** | NoiseXTerminator & StarXTerminator Architecture - RC-Astro | URL | Convolutional Neural Networks (CNNs) applied to the deconstruction of astrophotographic images for star separation. |

---

## 2. SkyStacker Engine Architecture and Concurrent Execution in Go

SkyStacker is conceived as a local, autonomous, and scriptable command-line interface (*CLI*) tool designed to process massive volumes of astrophotographic data without relying on graphical components or cloud services.

### 2.1. *Local-First* Design Principles and Static Compilation
In automated observatories and field environments, transferring RAW images exceeding 60 megapixels per frame to the cloud is unfeasible. Therefore, SkyStacker's architecture utilizes the Go programming language due to its native compilation features. By disabling C-linking (`CGO_ENABLED=0`), the engine generates an independent static executable that requires no operating system dynamic libraries (such as `libc` or `libtiff`).

The binary includes self-diagnostic tools (`--selftest`), resource estimation reports, and deterministic interfaces that guarantee processing reproducibility.

### 2.2. Processing via Spatial Tiles
Simultaneously loading hundreds of high-resolution frames into memory can exhaust system RAM. For instance, a sequence of 100 frames at 61 megapixels requires over 24.4 GB of continuous memory allocation if loaded as 32-bit floating-point matrices.

To overcome this constraint, SkyStacker implements a spatial tile processing model. Each full frame is subdivided into uniform blocks (typically $512 \times 512$ or $1024 \times 1024$ pixels). Processing goroutines stream only the required tiles from disk to memory buffers, keeping RAM consumption bounded by the number of concurrent worker threads rather than the total number of images in the dataset.

### 2.3. Memory Management and Garbage Collector Optimization
In intensive numerical processing loops, the constant creation and destruction of slices in Go causes severe pressure on the Garbage Collector, generating execution pauses (*GC pauses*).

To mitigate this issue, SkyStacker employs a buffer pooling scheme using `sync.Pool`. Due to the mechanics of Go's compiler escape analysis, passing value slices (`[]float32`) directly to `sync.Pool` causes slice headers to escape to the heap. To guarantee zero-allocations in critical paths, the architecture uses explicit pointers to structs wrapping the buffers (`*TileBuffer`):

```go
type TileBuffer struct {
    Pixels []float32
}

type TilePool struct {
    pool sync.Pool
}

func NewTilePool(tileSize int) *TilePool {
    return &TilePool{
        pool: sync.Pool{
            New: func() any {
                return &TileBuffer{
                    Pixels: make([]float32, tileSize),
                }
            },
        },
    }
}
```

By requesting and returning clean `*TileBuffer` pointers to the memory pool, the RAM footprint remains strictly deterministic (between 100 MB and 300 MB) even when stacking hundreds of exposures.

### 2.4. Concurrency Patterns, Pipelines, and Synchronization in Go
To maximize multi-core performance, SkyStacker structures its 6 processing stages (calibration, measurement, registration, stacking, rejection, and normalization) using the **Concurrency Pipeline** design pattern based on Go channels (`chan`). Goroutines act as independent processing nodes that consume from input channels (unprocessed files) and emit results to output channels.

To prevent *goroutine leaks* in the event of an error reading a corrupt frame, the architecture integrates `context.Context` with cancellation functions (`context.WithCancel`). If a RANSAC registration subroutine fails or the allocated memory limit is exceeded, the context is globally canceled, instructing all other active goroutines to abandon their calculations, releasing pointers back to the `sync.Pool` immediately and safely.

---

## 3. Advanced Data Structures: Swiss Tables and SIMD Instructions

Efficiency in image indexing operations, star mapping, and hash tables within SkyStacker's Go ecosystem relies on the **Swiss Tables** architecture, officially adopted in the Go 1.24 runtime based on C++ Abseil and `dolthub/swiss` developments.

### 3.1. SoA (*Structure of Arrays*) vs. AoS (*Array of Structures*) Design
Conventional hash tables organize their entries as arrays of structures (AoS), where the key, value, and control metadata reside contiguously in memory. This design means a single L1 cache line (64 bytes) contains only a couple of entries, dragging unread key data when only checking the cell's status is desired.

Swiss Tables reorganize the structure via a SoA (*Structure of Arrays*) design, separating control metadata from data into two parallel arrays:
*   **Metadata Array:** A buffer of control bytes, 1 byte per slot.
*   **Key/Value Array:** A contiguous buffer storing keys and payloads.

By using 1 byte per slot in the metadata array, a single 64-byte cache line stores the status of 64 contiguous slots, maximizing the L1/L3 cache hit probability.

### 3.2. Hash Splitting ($H_1$ / $H_2$) and SIMD Inspection
When inserting or searching for a key, its 64-bit hash is calculated and divided into two sections:
*   **$H_1$ (Upper 57 bits):** Determines the initial bucket index in the table.
*   **$H_2$ (Lower 7 bits):** Stored in the slot's control byte as a reduced signature or *fingerprint*.

Control bytes use special values to indicate empty (`-128` or `0b10000000`) or deleted (`-2` or `0b11111110`) slots. When the byte contains a positive value ($0 \dots 127$), it represents the $H_2$ signature.

To search for a key within a group of 8 or 16 slots, the engine loads the group of control bytes into a SIMD vector register or applies the SWAR (*SIMD Within A Register*) technique. By broadcasting the $H_2$ byte across the register, a parallel comparison is performed in a single clock instruction:

$$\text{matchMask} = (\text{ctrl} \oplus \text{broadcast}(H_2) - 0x0101 \dots) \ \& \ \sim(\text{ctrl} \oplus \text{broadcast}(H_2)) \ \& \ 0x8080 \dots$$

This operation filters out $99.22\%$ of collisions without needing to access the main key array. Only when an $H_2$ match is detected is the full key verification performed.

### 3.3. Performance and Incremental Growth
Unlike traditional maps that limit their maximum load factor to $81.25\%$, Swiss Tables operate efficiently with a load factor of up to **$87.5\%$** ($7/8$), reducing total memory consumption by $30\text{--}60\%$.

To avoid sudden pauses during the resizing of massive tables in low-latency services, the Go implementation extends the design by grouping entries into a directory of independent sub-tables of a maximum of 1024 entries, allowing bounded incremental growth per operation.

---

## 4. Mathematical-Photometric Calibration and Subframe Preprocessing

The calibration process corrects sensor and optical system imperfections prior to performing any geometric or spatial manipulation.

### 4.1. Calibration Frames
*   **Bias Frames (Master Bias):** Captured at the shortest possible exposure with the lens cap on. They record the electronic offset level of the readout sensor.
*   **Dark Frames (Master Dark):** Exposures taken in darkness with the same duration and temperature as the light frames. They record the thermal dark current and identify hot pixels.
*   **Flat Frames (Master Flat):** Exposures to a homogeneous light source. They record the non-uniform photometric response of the sensor, dust shadows, and peripheral light fall-off (vignetting).

### 4.2. Fundamental Calibration Equation
The photometric calibration of each pixel $(x,y)$ in a light frame ($I_{\text{light}}$) is governed by the following mathematical relationship:

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

where $I_{\text{dark}}$, $I_{\text{flat}}$ and $I_{\text{bias}}$ correspond to the pre-integrated master calibration frames, and $\bar{I}_{\text{flat}}$ represents the mean scalar intensity value of the master flat, used to preserve the global ADU scale of the image.

### 4.3. Photometric Evaluation and Frame Selection (`ImageAnalysis` / SubframeSelector)
Before stacking, the calibrated frames undergo quantitative quality evaluation. A unified photometric score $Q_i$ is calculated for each frame $i$:

$$Q_i = w_1 \cdot \left(\frac{1}{\text{FWHM}_i}\right) + w_2 \cdot (1 - e_i) + w_3 \cdot \text{SNR}_i + w_4 \cdot \text{PSFWeight}_i$$

where $\text{FWHM}$ represents the Full Width at Half Maximum of the stars, $e$ is the eccentricity of the point spread function (PSF), $\text{SNR}$ is the signal-to-noise ratio of the sky background, and $\text{PSFWeight}$ is the integrated stellar signal mass relative to sensor noise.

---

## 5. Detection Algorithms, Spatial Registration, and Star Alignment

To combine multiple frames exposed at different times, it is necessary to geometrically register each image against a common reference frame.

### 5.1. Detection and Subpixel Centroids
The detection algorithm locates 8-connected components whose brightness values exceed a threshold of $5\sigma$ above the local background. For each detected star, its exact center position $(x_c, y_c)$ is calculated using an intensity-weighted centroid:

$$x_c = \frac{\sum_{i \in S} x_i \cdot (I_i - B)}{\sum_{i \in S} (I_i - B)}, \quad y_c = \frac{\sum_{i \in S} y_i \cdot (I_i - B)}{\sum_{i \in S} (I_i - B)}$$

where $I_i$ is the intensity of pixel $i$, $B$ is the estimated background level, and $S$ is the set of pixels forming the stellar profile.

### 5.2. Star-Pattern Matching via Triangles
The engine selects the 30 brightest stars in the frame and constructs a network of triangles. For each triangle with sides ordered by length ($L_{\text{long}} \ge L_{\text{medium}} \ge L_{\text{short}}$), an invariant geometric descriptor to rotation, translation, and scale is generated:

$$\text{Descriptor} = \left( \frac{L_{\text{short}}}{L_{\text{long}}}, \frac{L_{\text{medium}}}{L_{\text{long}}} \right)$$

Descriptors from the current frame are compared against those of the reference image by searching for nearest neighbors within a tolerance threshold in feature space.

### 5.3. Similarity Transformation and RANSAC Filtering
From the matched star pairs obtained, a closed-form geometric similarity transformation is calculated (rotation $\theta$, scale $s$, translations $t_x, t_y$):

$$\begin{bmatrix} x' \\ y' \end{bmatrix} = s \begin{bmatrix} \cos\theta & -\sin\theta \\ \sin\theta & \cos\theta \end{bmatrix} \begin{bmatrix} x \\ y \end{bmatrix} + \begin{bmatrix} t_x \\ t_y \end{bmatrix}$$

To eliminate false matches caused by hot pixels or noise, an iterative **RANSAC** process is applied, discarding star pairs whose reprojection deviations exceed a determined subpixel threshold.

Subsequently, the image is resampled via subpixel interpolation, and a *safe crop* is performed to the valid area shared by all registered images.

### 5.4. Advanced Astrometry, Plate Solving, and WCS Metadata
Complementary to geometric registration via *Star-Pattern Matching*, SkyStacker is compatible with **Blind Plate Solving** algorithms. Instead of aligning against its own relative reference frame, this stage maps detected stars against global astrometric catalogs (e.g., Gaia DR3) to determine the exact center of the image in Right Ascension (RA) and Declination (Dec).

This calculation injects **WCS** (*World Coordinate System*) headers into the resulting file, employing SIP (*Simple Imaging Polynomial*) polynomials to model and inversely correct complex optical distortions like spherical coma aberration at the edges of the star field. Thanks to the WCS standard, the master image acquires real scientific value and allows for the overlay of professional celestial survey data.

---

## 6. Statistical Pixel Combination and Artifact Rejection Filters

The ultimate goal of stacking is to maximize the signal-to-noise ratio ($\text{SNR}$) and remove transient events such as satellite trails, cosmic rays, and sensor artifacts.

### 6.1. SNR Improvement Theory
The astronomical photon signal $S$ accumulates linearly with the number of integrated frames ($N$), whereas the stochastic noise from the sensor and sky background $\sigma$ adds in quadrature ($\sqrt{N}$). The SNR gain in the resulting master image follows the square root law:

$$\text{SNR}_{\text{stack}} = \sqrt{N} \cdot \text{SNR}_{\text{sub}}$$

### 6.2. Comparison of Integration Algorithms
The following table analyzes the computational and photometric characteristics of the integration methods supported in SkyStacker's technical literature:

| Stacking Method | SNR Efficiency | Outlier Filtering | Complexity | Recommended Application |
| :--- | :--- | :--- | :--- | :--- |
| **Arithmetic Mean** | Theoretical maximum ($\sqrt{N}$) | None (preserves satellites and cosmic rays) | $\mathcal{O}(N)$ | Clean, homogeneous datasets free of artifacts. |
| **Median Stacking** | Reduced ($\approx 0.8 \cdot \sqrt{N}$) | High (filters extremes without parameters) | $\mathcal{O}(N \log N)$ | Creation of calibration masters (*Darks*, *Flats*). |
| **Sigma Clipping** | Near optimal ($\approx 0.95 \cdot \sqrt{N}$) | Iterative by standard deviation ($k \cdot \sigma$) | $\mathcal{O}(I \cdot N)$ | Standard deep-sky stacking ($N > 30$). |
| **Winsorized Sigma** | Near optimal ($\approx 0.98 \cdot \sqrt{N}$) | Replaces outliers with limit values | $\mathcal{O}(I \cdot N \log N)$ | Standard deep-sky sequences ($10 \le N \le 30$). |
| **Linear Fit Clipping** | High in long sequences | Linear fit $y=ax+b$ along the stack | $\mathcal{O}(N)$ | Large stacks with varying sky gradients. |
| **GESDT** | High | Extreme Studentized Deviate test | $\mathcal{O}(N \log N)$ | Sequences of 31 to 300 frames. |

### 6.3. Auxiliary Diagnostic Stacks
Alongside the main stacked image, the system generates two control stacks:
*   **Pixel Maximum (`Max Stack`):** Retains the highest value recorded at each pixel. Used to map satellite trails or star streaks.
*   **Pixel Minimum (`Min Stack`):** Retains the lowest value. Allows evaluation of sky background uniformity and valid cropping edges.

### 6.4. GPU Acceleration and Hardware Abstraction Layer (HAL)
Statistical pixel rejection (e.g., Winsorized Sigma) on stacks of over 300 frames suffers severe bottlenecks due to matrix sorting $\mathcal{O}(N \log N)$. To address this "Compute-Bound" regime, the architecture introduces a **HAL** (Hardware Abstraction Layer) that transfers the 3D matrix from RAM to the graphics card's VRAM via standard interfaces. By utilizing tensor cores and compute shaders in Metal (Apple Silicon) or CUDA/Vulkan (Windows/Linux), SkyStacker massively parallelizes iterative rejection and subpixel interpolation, reducing integration times by orders of magnitude.

---

## 7. Scientific File Format Specifications and Metadata Processing

SkyStacker and associated tools base their data exchange on high-precision raster file standards.

### 7.1. TIFF 6.0 and IEEE 754 Float32 Structure
Master scientific outputs are encoded in **32-bit floating-point TIFF 6.0** format. A TIFF file begins with an 8-byte header followed by one or more Image File Directories (IFD):
*   **Byte Order:** `0x4949` (`II`, Little-Endian) or `0x4D4D` (`MM`, Big-Endian).
*   **Magic Identifier:** 42 (`0x002A`).
*   **Key Tags:**
    *   **Tag 258 (`BitsPerSample`):** 32 bits per channel.
    *   **Tag 339 (`SampleFormat`):** Set to `3` (`IEEEFP`), indicating the data represents floats in IEEE 754 format.
    *   **Tag 262 (`PhotometricInterpretation`):** Set to `1` (`BlackIsZero`) for monochrome images or `2` (`RGB`) for color.

### 7.2. BigTIFF Specification for Large Datasets
When integrated images or wide-field mosaics exceed the $4\text{ GB}$ structural limit imposed by 32-bit pointers ($2^{32}$ bytes), the engine automatically switches to the **BigTIFF** format:
*   **Magic Identifier:** Changes from 42 (`0x002A`) to 43 (`0x002B`).
*   **64-bit Pointers:** Tag count fields, strip offsets, and byte counts expand from `LONG` types (32 bits) to `LONG8` structures (64 bits).

### 7.3. Metadata Ingestion with `imagemeta`
To verify frame compatibility before stacking without incurring excessive I/O costs, low-level Go libraries like `imagemeta` are used. This library reads EXIF, IPTC, and XMP headers directly via fixed buffer reads without generating heap allocations:

```go
type FrameDescriptor struct {
    FilePath     string
    Width        uint32
    Height       uint32
    ExposureTime float64
    CameraModel  string
    Timestamp    time.Time
}
```

### 7.4. Alternative Astronomical Standards: FITS and XISF
Despite the ubiquity of TIFF/BigTIFF, the engine supports ingestion and encoding in native astronomical formats geared towards scientific rigor:
*   **FITS (Flexible Image Transport System):** Endorsed by NASA and the IAU, this format is based on 2880-byte ASCII text blocks for headers and binary data blocks, making it the non-negotiable format for astrometry and observatory transfer.
*   **XISF (Extensible Image Serialization Format):** Developed as the foundation of the PixInsight processing environment, it is driven by XML descriptors and can encapsulate astrometric metadata, cryptographic modification histories, and compressed memory sub-blocks via LZ4 algorithms.

---

## 8. Non-Linear Post-Processing Pipeline, Stretching, and Color Calibration

Once the master image is stacked, the result is in a **linear** state (a dark image where data is concentrated in the first percentile of the dynamic range). The post-processing stage converts this linear result into an image suitable for visualization.

### 8.1. Non-Linear Stretching Transformations
To make the signal visible without saturating bright stellar cores, non-linear functions are applied:

1.  **Inverse Hyperbolic Sine (AsinhStretch / ArcsinhStretch):**
    Mimics the perceptual response of the human eye and maintains linear behavior near zero:

    $$f(x) = \frac{\text{asinh}(\beta \cdot (x - c))}{\text{asinh}(\beta)}$$

    where $\beta$ is the stretch factor and $c$ represents the black point. Unlike logarithmic stretching, AsinhStretch preserves the color relationships of RGB channels from faint structures to star centers.

2.  **Generalized Hyperbolic Stretch (GHS):**
    Provides independent control over the contrast inflection point, allowing the stretching of faint nebulosity exclusively while protecting stars.

3.  **Midtone Transfer Function (MTF):**
    Transformation based on the midtone parameter $m \in (0,1)$:

    $$\text{MTF}(x, m) = \frac{(m - 1) \cdot x}{(2m - 1) \cdot x - m}$$

### 8.2. Color Calibration and Green Noise Removal (SPCC and SCNR)
*   **Spectrophotometric Color Calibration (SPCC / PCC):** Adjusts color balances by calculating the magnitudes of detected stars and comparing them against astronomical photometric catalogs (like Gaia). It is strictly executed in a linear state before any stretching.
*   **Subtractive Chromatic Noise Reduction (SCNR / HLVG):** Removes the non-astronomical greenish tint caused by the Bayer matrix (which has twice as many green pixels). It operates on the green channel by reducing non-neutral deviations.

### 8.3. Integrated Post-Processing Stages in SkyStacker
 SkyStacker executes a 6-step non-linear post-processing pipeline:
1.  **Background Extraction:** Fits a 2D second-degree polynomial to the image to remove light pollution gradients.
2.  **Chromatic Aberration Correction:** Suppresses magenta/violet halos by detecting pixels where R and B intensities significantly exceed G.
3.  **Color Calibration:** Calculates the median of the channels in the $10\text{--}50$ percentile luminance range and applies calibrated scaling factors.
4.  **Luminance GHS Stretch:** Applies the GHS function only to the luminance channel $L$ and scales RGB channels proportionally, preventing color desaturation.
5.  **Residual Vignetting Flattening:** Re-applies a secondary background extraction after stretching.
6.  **Unsharp Mask:** Applies a high-pass filter to enhance the contrast of fine nebular structures.

### 8.4. Neural Network and Artificial Intelligence Integration
The contemporary non-linear post-processing pipeline implements pre-trained Convolutional Neural Network (CNN) layers and next-generation algorithmic tools:
*   **Starless Workflows:** Networks like StarXTerminator predictively detect Point Spread Function (PSF) profiles to remove them cleanly and iteratively fill the resulting empty space, allowing for radical stretching of gas contrasts and colors without distorting or bloating stars.
*   **Wavelet *à Trous* Transforms:** Instead of conventional unsharp masks, luminance data is deconstructed into multiple overlapping layers of different spatial resolutions (*Wavelet Layers*). The user can sharpen intricate fine-scale details and apply destructive noise reduction algorithms exclusively to low-frequency layers, preserving the maximum structural detail of the signal.

---

## 9. Code Quality, Continuous Profiling, and the Roofline Model

Developing scientific engines requires maintaining strict software quality standards and hardware performance optimization.

### 9.1. Code Smell Detection and Refactoring
SkyStacker's source code follows static analysis rules to prevent degrading code smells:
*   **Complex or Lengthy Functions:** Decomposing functions into atomic modules with single responsibility.
*   **Hidden Allocations:** Eliminating implicit conversions that force variables to transfer from the stack to the heap.
*   **I/O Coupling:** Isolating disk reads from calculation loops using concurrent channel buffers.

### 9.2. Continuous Profiling with eBPF
To analyze engine runtime behavior without introducing instrumentation overhead, continuous profilers based on **eBPF** (*Extended Berkeley Packet Filter*), such as Parca or Pyroscope, are used. eBPF inspects CPU performance counters and memory allocation at the kernel level, generating precise flamegraphs of goroutines.

### 9.3. The Roofline Model
Stacking performance is evaluated using the **Roofline Model**, which relates execution speed in GFLOPS to **Arithmetic Intensity** $I$ (floating-point operations per byte transferred from RAM):

$$\text{Performance (GFLOPS)} = \min\left( \text{Peak ALU Performance}, \text{RAM Bandwidth} \times I \right)$$

*   **Memory-Bound Regimes:** Operations like simple matrix addition have low arithmetic intensity and are limited by RAM bandwidth.
*   **Compute-Bound Regimes:** Algorithms like FFT transformations in convolutions or subpixel interpolation have high arithmetic intensity and are limited by the CPU's vector units (AVX2/AVX-512).

Through spatial tiling, SkyStacker maximizes data reuse in the L2 cache, shifting execution towards the compute-bound regime and fully leveraging the processor's capability.
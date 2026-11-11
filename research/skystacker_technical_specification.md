# SkyStacker: Technical Specification, System Architecture, and Astrophotography Processing Engine Guide

## 1. Executive Overview and Core Purpose

**SkyStacker** is an open-source, high-performance, local-first command-line tool (CLI) engineered in Go for processing, aligning, calibrating, and integrating deep-sky astrophotography image sequences. Modern Complementary Metal-Oxide-Semiconductor (CMOS) and Charge-Coupled Device (CCD) astronomical sensors generate ultra-high-resolution sub-exposures (frequently exceeding 60 megapixels at 16 bits per sample). Imaging sessions routinely yield hundreds of sub-exposures, resulting in raw datasets scaling from tens to hundreds of gigabytes.

SkyStacker eliminates network transfer bottlenecks, desktop Graphical User Interface (GUI) latency, dynamic heap thrashing, and third-party cloud service dependencies by running natively on local observatory edge hardware or user workstations. It provides an automated, reproducible, resource-conscious, and mathematically rigorous workflow that transforms raw, uncalibrated sub-exposures into scientific masters and post-processed visualizations.

---

## 2. Core Execution Workflows

SkyStacker provides two primary operational execution topologies: the **SkyStacker 9-Step Automated Pipeline** and the internal **ASTAP 5-Step In-Memory Stacking Workflow**.

```
                         [ Input Data Directories ]
                                     │
      ┌──────────────────────────────┴──────────────────────────────┐
      │                                                             │
      ▼                                                             ▼
[ SkyStacker 9-Step Pipeline ]               [ ASTAP 5-Step In-Memory Engine ]
 1. Discover Input Frames                     1. Generate Master Flat
 2. Decode Pixel Data                         2. Generate Master Dark
 3. Photometric Calibration                   3. Dark Subtraction (In-Memory)
 4. Star Detection & Ref Frame Selection       4. Flat Division (In-Memory)
 5. Triangle/Quad Star Registration           5. Statistical Integration
 6. Subpixel Resampling & Crop                               │
 7. Statistical Pixel Combination                            │
 8. Non-Linear Post-Processing                               │
 9. Scientific Export & Telemetry                            │
      │                                                             │
      └──────────────────────────────┬──────────────────────────────┘
                                     ▼
                     [ Master Outputs & Diagnostic Reports ]
```

### 2.1 The SkyStacker 9-Step Automated Pipeline

1. **File Discovery & Header Indexing:** Recursively inspects session directories to locate supported formats (`.fit`, `.fits`, `.fits.fz`, `.cr2`, `.cr3`, `.jpg`, `.jpeg`, `.tiff`, `.xisf`, `.pgm`, `.ppm`). Headers (EXIF/FITS) are parsed to extract metadata (exposure duration, sensor temperature, gain, Bayer matrix pattern, and timestamps).
2. **Frame Decoding & Resource Safeguards:** Decodes pixel channels into 32-bit floating-point (`float32`) memory buffers. Memory safety checks evaluate the dataset dimensions against available system RAM to prevent Out-Of-Memory (OOM) kernel panics.
3. **Photometric Calibration:** Subtraction of thermal dark current and electronic bias offset, followed by division by spatial master flat fields.
4. **Star Detection & Reference Frame Selection:** Identifies stellar Point Spread Functions (PSFs) above local sky noise ($>5\sigma$). Automatically ranks sub-exposures based on Full Width at Half Maximum (FWHM), eccentricity ($e$), and Signal-to-Noise Ratio (SNR), selecting the optimal frame as the global geometric reference.
5. **Spatial Frame Registration:** Computes geometric star-pattern triangles or irregular 2D quad structures. Evaluates affine and similarity transformations using RANSAC (RANdom SAmple Consensus) outlier filtering.
6. **Subpixel Resampling and Safe Cropping:** Aligned sub-exposures are resampled using high-order Lanczos/Sinc interpolation kernels. Bounding-box cropping isolates the overlapping spatial domain valid across all registered subframes.
7. **Statistical Pixel Integration (Stacking):** Accumulates registered temporal pixel arrays using statistical rejection filters (`sigma`, `winsorized`, `mean`, `median`). Concurrently computes diagnostic maximum and minimum intensity stacks.
8. **Non-Linear Post-Processing:** Applies background-gradient extraction, chromatic-aberration reduction, color calibration, Generalized Hyperbolic Stretch (GHS), and local detail enhancement.
9. **Scientific Export & Audit Telemetry:** Writes uncompressed 32-bit IEEE 754 float master files (`.tiff` / `.fits`), 8-bit visual previews (`.jpg`), and a structured session audit report (`report.txt` / `telemetry_audit.log`).

### 2.2 ASTAP Core 5-Step In-Memory Stacking Workflow

For environments optimized strictly for memory throughput:

1. **Master Flat Normalization:** Master flat subframes are integrated into an average frame. Combined flat-dark or bias frames are subtracted. The master flat undergoes $2 \times 2$ neighborhood mean spatial normalization:
   $$\text{Master Flat} = \frac{\frac{1}{N} \sum \text{Flats} - \frac{1}{N} \sum \text{Flat-Darks}}{\bar{I}_{\text{flat}}}$$
2. **Master Dark Integration:** Dark sub-exposures taken at identical sensor temperature and exposure durations are combined:
   $$\text{Master Dark} = \frac{1}{N} \sum \text{Darks}$$
3. **Pure Signal Extraction:** For each raw sub-exposure, the Master Dark is subtracted in-memory to isolate photon arrival counts from thermal dark current and electronic readout offset.
4. **Optical Normalization:** Dark-subtracted frame buffers are divided by the Master Flat in-memory to eliminate dust motes and vignetting.
5. **In-Memory Frame Combination:** Corrected subframes are registered and accumulated directly within thread-safe spatial memory buffers without intermediate disk I/O.

---

## 3. High-Throughput System Architecture & Go Concurrency Model

### 3.1 Static Compilation & Zero-Dependency Binary Design
Building SkyStacker with native Go tools (`CGO_ENABLED=0`) produces a statically linked, standalone binary. This design operates without external system dynamic library dependencies (such as `libc`, `libtiff`, or GTK runtimes), ensuring deployment consistency across Linux, macOS, Windows, and ARM-based edge observatories.

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Go Application Code                             │
├────────────────────────────────────────────────────────────────────────┤
│           Tile Streaming Pipeline          sync.Pool Allocators        │
├────────────────────────────────────────────────────────────────────────┤
│               Statically Compiled Go Runtime (CGO_ENABLED=0)            │
└────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Spatial Tile Streaming (`tile-based streaming`)
To prevent system RAM exhaustion when processing massive subframe sequences, SkyStacker splits dynamic frame grids into discrete spatial tiles (e.g., $512 \times 512$ or $1024 \times 1024$ pixels).

Instead of loading full multi-gigabyte subframe arrays into memory, execution threads stream corresponding spatial tiles sequentially from disk into pre-allocated memory buffers. System memory usage is governed by the number of active CPU worker threads rather than the total dataset size or stack depth:

$$\text{RAM}_{\text{peak}} \approx N_{\text{workers}} \times (\text{Tile}_{\text{width}} \times \text{Tile}_{\text{height}} \times 4 \text{ bytes}) \times N_{\text{buffer\_slots}}$$

### 3.3 Memory Management, Heap Escape Analysis, and `sync.Pool`
High-frequency allocation of floating-point slice buffers (`[]float32`) within inner loops triggers Garbage Collector (GC) thrashing and runtime pauses (`GC pauses`). Passing slice headers directly into Go `sync.Pool` structures causes heap escape through interface casting (`any`).

To achieve zero dynamic allocations in critical streaming loops, SkyStacker wraps tile allocations inside explicit structures and manages pointers to those structures (`*TileBuffer`):

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

func (tp *TilePool) Get() *TileBuffer {
    return tp.pool.Get().(*TileBuffer)
}

func (tp *TilePool) Put(tb *TileBuffer) {
    tp.pool.Put(tb)
}
```

By requesting and returning `*TileBuffer` pointers to the pool, heap allocations during spatial tile processing remain at zero, keeping system memory consumption stable (typically between 100 MB and 300 MB).

### 3.4 Concurrency Patterns and Goroutine Lifecycle
Processing tasks execute across a concurrent pipeline using Go channels (`chan`). Worker goroutines act as independent processing nodes consuming work units (file paths or spatial tile indices) and pushing processed outputs down the pipeline.

```
[ Disk Read Stage ] ──chan *TileBuffer──► [ Calibration Stage ] ──chan *TileBuffer──► [ Stacking Stage ]
         │                                       │                                     │
         └─────────────────── context.Context (Cancellation & Telemetry) ──────────────┘
```

Goroutine lifecycles are controlled using `context.Context` cancellation objects (`context.WithCancel`). If a registration subroutine fails or a file read error occurs, the context is globally canceled. This signals all concurrent goroutines to terminate processing and return active `*TileBuffer` pointers to `sync.Pool`, preventing goroutine leaks and memory locks.

### 3.5 Advanced Hash Engine: Swiss Tables Architecture
SkyStacker utilizes the Swiss Tables hash table design (integrated natively into the Go 1.24+ runtime and implemented via `dolthub/swiss`). Swiss Tables improve key indexing, star matching lookup, and EXIF tag caching compared to traditional hash structures.

#### Structure of Arrays (SoA) vs. Array of Structures (AoS)
Traditional hash tables organize data as an Array of Structures (AoS), storing keys, values, and control flags contiguously. A single 64-byte L1 CPU cache line holds few entries, dragging unneeded payload data into cache during lookup operations.

Swiss Tables use a Structure of Arrays (SoA) design, separating control metadata from data payloads:

```
AoS Layout (Standard Hash Table):
┌───────────┬───────────┬───────────┬───────────┐
│ Key/Val/  │ Key/Val/  │ Key/Val/  │ Key/Val/  │  <-- Low Cache Density
│ Control 1 │ Control 2 │ Control 3 │ Control 4 │
└───────────┴───────────┴───────────┴───────────┘

SoA Layout (Swiss Tables):
Control Array (1 byte/slot):  [ Ctrl 1 ][ Ctrl 2 ][ Ctrl 3 ]...[ Ctrl 64 ] <-- Single L1 Cache Line
Key/Value Data Array:        [ Key/Val 1 ][ Key/Val 2 ][ Key/Val 3 ]...
```

* **Control Byte Array:** 1 byte per slot storing status flags or hash signatures.
* **Key/Value Slot Array:** Contiguous arrays storing key and value pairs.

A single 64-byte CPU cache line holds 64 slot control status bytes, maximizing L1/L3 cache hit rates.

#### Hash Splitting ($H_1 / H_2$) and SIMD Inspection
When indexing a key, its 64-bit hash code is split into two components:

1. **$H_1$ (Upper 57 bits):** Determines the primary bucket group index.
2. **$H_2$ (Lower 7 bits):** Stored in the control byte array as a fingerprint signature ($0 \dots 127$). Control values also designate empty (`-128`) or deleted (`-2`) slots.

To locate a key within a group of control bytes, SWAR (SIMD Within A Register) operations or SIMD vector instructions inspect up to 16 slots concurrently:

$$\text{matchMask} = (\text{ctrl} \oplus \text{broadcast}(H_2) - 0x0101 \dots) \ \& \ \sim(\text{ctrl} \oplus \text{broadcast}(H_2)) \ \& \ 0x8080 \dots$$

This parallel bitwise evaluation eliminates up to $99.22\%$ of key mismatch checks. Full key equality is verified only when an $H_2$ fingerprint match occurs. Swiss Tables maintain performance at up to an **$87.5\%$** load factor ($7/8$), reducing memory footprint by $30\text{--}60\%$ relative to traditional maps.

---

## 4. Photometric Calibration & Noise Modeling

Light sub-exposures record astronomical photon flux along with sensor noise, thermal electron accumulation, electronic read noise, optical vignetting, and light path obstruction.

### 4.1 Calibration Mathematical Formulation
Photometric calibration converts raw Analog-to-Digital Units ($\text{ADU}$) into corrected pixel intensities $I_{\text{calibrated}}(x,y)$:

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

Where:
* $I_{\text{light}}(x,y)$ is the target raw sub-exposure intensity at coordinates $(x,y)$.
* $I_{\text{dark}}(x,y)$ is the Master Dark frame intensity, measuring exposure-duration and temperature-dependent thermal dark current accumulation and fixed-pattern noise.
* $I_{\text{bias}}(x,y)$ is the Master Bias frame intensity, measuring uniform sensor electronic readout offset.
* $I_{\text{flat}}(x,y)$ is the Master Flat field intensity, mapping optical throughput variations, vignetting, and optical dust motes.
* $\bar{I}_{\text{flat}}$ is the scalar mean intensity of the Master Flat, used to normalize the output values to the sensor's dynamic range.

### 4.2 Frame Evaluation Metrics & Quality Score ($Q_i$)
Before integration, subframes undergo photometric analysis. Individual sub-exposures are scored using a dynamic quality metric ($Q_i$):

$$Q_i = w_1 \cdot \left(\frac{1}{\text{FWHM}_i}\right) + w_2 \cdot (1 - e_i) + w_3 \cdot \text{SNR}_i + w_4 \cdot \text{PSFWeight}_i$$

Where:
* $\text{FWHM}_i$ is the average Full Width at Half Maximum of stellar profiles (measured in pixels or arcseconds).
* $e_i$ is the average stellar eccentricity ($0 \le e_i < 1$, where $e_i=0$ represents a circular star profile).
* $\text{SNR}_i$ is the background Signal-to-Noise Ratio.
* $\text{PSFWeight}_i$ is the integrated stellar signal energy relative to background noise variance.
* $w_1, w_2, w_3, w_4$ are configurable weighting coefficients.

Sub-exposures falling below user-defined quality thresholds are flagged and excluded from the registered master stack.

---

## 5. Star Detection, Subpixel Alignment, & Astrometry

### 5.1 Subpixel Centroid Estimation
Star detection isolates 8-connected pixel clusters exceeding a dynamic threshold of $5\sigma$ above local background noise. Subpixel star centroids $(x_c, y_c)$ are computed using intensity-weighted spatial moments:

$$x_c = \frac{\sum_{i \in S} x_i \cdot (I_i - B)}{\sum_{i \in S} (I_i - B)}, \quad y_c = \frac{\sum_{i \in S} y_i \cdot (I_i - B)}{\sum_{i \in S} (I_i - B)}$$

Where $I_i$ is the intensity of pixel $i$, $B$ is the local background luminance level, and $S$ is the set of pixels contained within the star profile radius.

### 5.2 Geometric Registration Algorithms

#### 1. Triangle-Based Pattern Matching
Using the top 30 brightest stars in a subframe, the registration engine generates scale- and rotation-invariant geometric triangles. For a triangle with side lengths $L_{\text{long}} \ge L_{\text{medium}} \ge L_{\text{short}}$, a scale- and rotation-invariant descriptor is calculated:

$$\text{Descriptor} = \left( \frac{L_{\text{short}}}{L_{\text{long}}}, \frac{L_{\text{medium}}}{L_{\text{long}}} \right)$$

Descriptors from target subframes are matched against the reference subframe catalog using nearest-neighbor searches in descriptor space.

#### 2. Quad Matching Algorithm (ASTAP Quad Engine)
The engine groups four detected star centroids into irregular 2D tetragons (quads). Six internal geometric distances between the four stars are computed and converted into a 6-digit invariant hash code.

```
       Star A *────────────────────* Star B
               \                  /
                \   Distance 1-4 /
                 \              /
                  *────────────* Star C
                 Star D
  [ 4 Star Detections = 1 Irregular 2D Quad ]
  [ 6 Inter-Star Distances -> Invariant Hash ]
```

* **Hash Tolerance (`-t`):** Sets the maximum allowable deviation when matching quad geometric hash metrics (default: `0.007`).
* **Star Detection Limit (`-s`):** Limits quad construction to the brightest stars (default: `500`) to bound solve time.
* **HFD Filter (`-m`):** Excludes single-pixel noise spikes by filtering out detections below `0.8` pixels.

### 5.3 Transformation Matrix Estimation & RANSAC Filtering
Matched star pairs are used to compute a 2D similarity transformation matrix (scale $s$, rotation angle $\theta$, translations $t_x, t_y$):

$$\begin{bmatrix} x' \\ y' \end{bmatrix} = s \begin{bmatrix} \cos\theta & -\sin\theta \\ \sin\theta & \cos\theta \end{bmatrix} \begin{bmatrix} x \\ y \end{bmatrix} + \begin{bmatrix} t_x \\ t_y \end{bmatrix}$$

An iterative RANSAC filter selects random minimum sample subsets, fits candidate transformation matrices, and identifies inliers. Matches exceeding subpixel reprojection error thresholds are rejected, suppressing false stellar matches caused by hot pixels or cosmic ray hits.

After matrix validation, target subframes are resampled using Lanczos/Sinc interpolation and cropped to the valid spatial overlap region common across all stacked frames.

### 5.4 Astrometric Plate Solving & SIP Polynomial Distortion Modeling
SkyStacker supports blind astrometric plate solving to match detected stars against reference catalogs (such as Gaia DR3). Solved frames are assigned World Coordinate System (WCS) headers mapping physical pixel coordinates $(x,y)$ to celestial equatorial coordinates $(\alpha, \delta)$ (Right Ascension and Declination).

To correct field curvature, coma, and optical distortion, the engine computes 3rd-order Simple Imaging Polynomial (SIP) distortion coefficients:

$$u = f(x,y) = \sum_{p+q \le 3} A_{p,q} \, x^p y^q = x + A_{2,0} x^2 + A_{1,1} xy + A_{0,2} y^2 + A_{3,0} x^3 + A_{2,1} x^2 y + A_{1,2} x y^2 + A_{0,3} y^3$$

$$v = g(x,y) = \sum_{p+q \le 3} B_{p,q} \, x^p y^q = y + B_{2,0} x^2 + B_{1,1} xy + B_{0,2} y^2 + B_{3,0} x^3 + B_{2,1} x^2 y + B_{1,2} x y^2 + B_{0,3} y^3$$

Where $u, v$ are true spherical focal plane coordinates, and $A_{p,q}, B_{p,q}$ are SIP distortion matrix parameters written to output headers.

---

## 6. Statistical Pixel Integration (Stacking Methods)

The primary mathematical objective of stacking is to increase Signal-to-Noise Ratio (SNR) while rejecting transient non-Gaussian artifacts (satellite trails, cosmic ray strikes, airplane lights).

### 6.1 Stochastic Noise Theory & Theoretical SNR Gain
Astronomical photon signal ($S$) accumulates linearly with the number of integrated subframes ($N$), while stochastic sensor read noise and background noise ($\sigma$) sum in quadrature ($\sqrt{N}$). The SNR of the integrated master image ($\text{SNR}_{\text{stack}}$) scales according to:

$$\text{SNR}_{\text{stack}} = \sqrt{N} \cdot \text{SNR}_{\text{sub}}$$

```
SNR Gain
  ▲
 5│                                                    ....... Theoretical Limit (√N)
 4│                                       . . . . . . '
 3│                         . . . . . . '
 2│           . . . . . . '
 1│ . . . . '
 0└─┬─────────┬─────────┬─────────┬─────────┬─────────► N (Number of Frames)
    0        10        20        30        40        50
```

### 6.2 Comparison of Stacking Algorithms

| Stacking Method | SNR Gain Factor | Outlier Rejection Performance | Mathematical Complexity | Primary Astrophotography Application |
| :--- | :--- | :--- | :--- | :--- |
| **Sum Integration** | $\sqrt{N}$ (Expands dynamic range) | None (Retains all artifacts) | $\mathcal{O}(N)$ | Photometric counting, high-speed Lucky Imaging. |
| **Arithmetic Mean (`mean`)** | $1.00 \cdot \sqrt{N}$ (Theoretical Max) | Zero (Distorted by non-Gaussian noise) | $\mathcal{O}(N)$ | Master Calibration generation ($Darks/Flats$) for clean data. |
| **Median (`median`)** | $\approx 0.80 \cdot \sqrt{N}$ (~20% loss) | Complete (Rejects extreme boundaries) | $\mathcal{O}(N \log N)$ | Small stacks ($N \le 9$) or master calibration frames. |
| **Percentile Clipping** | $\approx 0.85 \cdot \sqrt{N}$ | Fixed percentile boundaries | $\mathcal{O}(N \log N)$ | Small sub-exposure counts ($N \le 6$) where $\sigma$ is unstable. |
| **Sigma-Clipping (`sigma`)** | $\approx 0.95 \cdot \sqrt{N}$ | Iterative standard deviation ($k\sigma$) rejection | $\mathcal{O}(I \cdot N \log N)$ | Standard default integration for deep-sky series ($N \ge 31$). |
| **Winsorized Sigma Clipping** | $\approx 0.98 \cdot \sqrt{N}$ | Outliers replaced with boundary values | $\mathcal{O}(I \cdot N \log N)$ | Moderate deep-sky datasets ($10 \le N \le 30$). |
| **Linear Fit Clipping** | High on long series | Fits linear slope $y=ax+b$ along stack axis | $\mathcal{O}(N)$ | Wide-field sequences with varying sky background gradients. |
| **GESDT Clipping** | High | Generalized Extreme Studentized Deviate test | $\mathcal{O}(N \log N)$ | Sequences of 31 to 300 sub-exposures with high outlier density. |

### 6.3 Diagnostic Auxiliary Stacks
In addition to the primary integrated master file, SkyStacker can generate two control outputs:

* **Pixel Maximum (`Max Stack`):** Retains the absolute maximum pixel intensity recorded along the temporal axis. Used to analyze satellite density, meteor trails, and transient optical reflections.
* **Pixel Minimum (`Min Stack`):** Retains the absolute minimum pixel intensity. Used to inspect background sky uniformity, residual vignetting, and sensor obstructions.

### 6.4 Hardware Abstraction Layer (HAL) & GPU Acceleration
When processing stacks ($N > 300$), iterative statistical sorting operations ($\mathcal{O}(N \log N)$) can become compute-bound. SkyStacker incorporates a Hardware Abstraction Layer (HAL) to offload matrix operations. Using SIMD CPU extensions (AVX2/AVX-512) or optional GPU compute backends (CUDA, Vulkan Compute, or Apple Silicon Metal Framework), pixel rejection, matrix transformation, and subpixel interpolation tasks are parallelized across available compute hardware.

---

## 7. One-Shot Color (OSC) Demosaicing

Raw data captured by One-Shot Color (OSC) sensors contains a Bayer Color Filter Array (CFA) matrix pattern. Reconstruction of full-color RGB data requires specialized debayering algorithms:

* **AstroC (Saturated Star Reconstruction):** Evaluates saturated star cores. When star pixels hit sensor saturation boundaries, channel ratios are interpolated from surrounding unsaturated non-saturated pixels to prevent artificial color tinting in star centers.
* **AstroM (Monochrome Star Balancing):** Analyzes local $2 \times 2$ Bayer blocks ($R, G_1, G_2, B$). For undersampled subframes where color sampling is unstable, it derives a monochrome luminance value, keeping star profiles neutral white.
* **AstroSimple (Han Kleijn Method):** Extracts discrete $R, G_1, G_2, B$ information from localized $2 \times 2$ pixel neighborhoods without cross-channel spatial bleeding. Preserves high-frequency stellar details while minimizing edge color artifacts.
* **Standard Bilinear:** Computes channel interpolation by averaging pixel values across a $3 \times 3$ grid. Produces smooth spatial transitions, but can blur high-frequency stellar profiles.

---

## 8. Non-Linear Post-Processing Pipeline

Stacked astronomical master files exist in a **linear** dynamic state, where faint nebular structures are concentrated in the lower percentiles of the histogram. SkyStacker includes an automated 6-stage post-processing engine to prepare images for visual rendering:

```
[ Linear Integrated Master ]
             │
             ▼
[ 1. Background Extraction & 2D Polynomial Gradient Fitting ]  <-- (--mask-bg)
             │
             ▼
[ 2. Chromatic Aberration Suppression ]
             │
             ▼
[ 3. Spectrophotometric Color Neutralization ]
             │
             ▼
[ 4. Non-Linear Stretching (GHS / Asinh / MTF) ]
             │
             ▼
[ 5. Vignette Residual Flattening ]
             │
             ▼
[ 6. High-Pass Spatial Sharpening (Unsharp Mask) ]
             │
             ▼
[ Rendered Post-Processed Image Output ]
```

### 8.1 6-Stage Integrated Processing Pipeline

1. **Background Extraction & Gradient Removal:** Models and subtracts background light pollution gradients using a 2D second-degree polynomial surface. Enabling `--mask-bg` masks bright celestial targets (luminance $> 0.1$) to prevent object structures from biasing background polynomial calculations.
2. **Chromatic Aberration Suppression:** Identifies and dampens purple/magenta halos by evaluating spatial locations where Red and Blue channel responses deviate from the Green channel.
3. **Color Calibration:** Neutralizes background color casting by evaluating channel medians within the 10th to 50th luminance percentile range, applying channel scaling factors bounded within $[0.5, 1.5]$.
4. **Non-Linear Stretching Transformations:** Transforms linear data for display without saturating bright stellar cores:
   * **Generalized Hyperbolic Stretch (GHS):** Governed by stretching factor $b$ and black point protection parameter $D$:
     $$f(x) = \frac{\sinh^{-1}\left( \frac{x - D}{b} \right) - \sinh^{-1}\left( \frac{-D}{b} \right)}{\sinh^{-1}\left( \frac{1 - D}{b} \right) - \sinh^{-1}\left( \frac{-D}{b} \right)}$$
   * **Inverse Hyperbolic Sine Stretch (AsinhStretch):** Preserves RGB color relationships from faint background regions to bright star cores:
     $$f(x) = \frac{\text{asinh}(\beta \cdot (x - c))}{\text{asinh}(\beta)}$$
     Where $\beta$ is the stretching intensity factor and $c$ is the black point offset.
   * **Midtone Transfer Function (MTF):** Adjusts midtone contrast based on parameter $m \in (0,1)$:
     $$\text{MTF}(x, m) = \frac{(m - 1) \cdot x}{(2m - 1) \cdot x - m}$$
5. **Vignette Residual Flattening:** Re-applies a smoothed spatial illumination pass post-stretch to minimize remaining edge falloff.
6. **High-Pass Sharpening (Unsharp Mask):** Enhances spatial micro-contrast across nebular filaments using high-pass spatial filtering.

### 8.2 Color Calibration & Green Noise Suppression
* **Spectrophotometric Color Calibration (SPCC / PCC):** Computes instrumental magnitudes of detected stars and matches them against reference stellar spectral energy distributions from catalogs (e.g., Gaia DR3). SPCC executes strictly on linear data prior to non-linear stretching.
* **Subtractive Chromatic Noise Reduction (SCNR / HLVG):** Suppresses non-astronomical green color casts introduced by Bayer matrix channel distribution (which contains twice as many green pixels as red or blue).

### 8.3 Wavelet Multiscale Processing (*À Trous* Transform)
The *à trous* ("with holes") discrete wavelet transform decomposes images into scale-invariant components:

$$I(x, y) = c_N(x, y) + \sum_{k=1}^{N} w_k(x, y)$$

Where $c_N$ is the residual low-frequency background layer, and $w_k$ represents distinct spatial frequency detail scales. High-frequency noise is isolated in fine layers ($w_1, w_2$), while nebular structures are isolated in broader layers ($w_3 \dots w_N$), enabling targeted noise reduction and micro-contrast enhancement.

### 8.4 Deep Learning AI Model Integration
SkyStacker supports integration with external AI inference models (such as StarXTerminator, NoiseXTerminator, and GraXpert):

* **Starless Workflows:** Convolutional Neural Networks (U-Nets) identify stellar profiles, separate star layers from extended nebular structures, and interpolate background voids. Faint gas structures can then be stretched independently without bloating stellar profiles.
* **AI Denoising:** Self-supervised neural networks suppress background read noise and shot noise while preserving diffraction spikes and point-source stellar flux.

---

## 9. Automated Image Quality Diagnostics

```
┌─────────────────────────┬─────────────────────────┬─────────────────────────┐
│  Area 1A (Corner TL)    │   Area 2A (Top Center)  │  Area 3A (Corner TR)    │
├─────────────────────────┼─────────────────────────┼─────────────────────────┤
│  Area 1B (Left Center)  │   Area 2B (CENTER)      │  Area 3B (Right Center) │
├─────────────────────────┼─────────────────────────┼─────────────────────────┤
│  Area 1C (Corner BL)    │   Area 2C (Bot Center)  │  Area 3C (Corner BR)    │
└─────────────────────────┴─────────────────────────┴─────────────────────────┘
```

### 9.1 Half Flux Diameter (HFD) Focus & Sharpness
Half Flux Diameter (HFD) measures stellar energy distribution to determine focus quality. HFD represents the diameter (in pixels) containing $50\%$ of a star's total cumulative photon flux.

### 9.2 Sensor Tilt Analysis (9-Area Spatial Octagon)
The sensor plane is partitioned into nine spatial zones. Sensor tilt is calculated as the percentage deviation between the best and worst corner median HFD values:

$$\text{Tilt } \% = 100 \times \frac{\text{HFD}_{\text{worst\_corner}} - \text{HFD}_{\text{best\_corner}}}{\text{HFD}_{\text{median}}}$$

#### Tilt Severity Scale:
* **$< 5\%:$** None (Optimal optical alignment)
* **$5\% - 10\%:$** Negligible (Operational baseline)
* **$10\% - 15\%:$** Mild (Acceptable operational threshold)
* **$15\% - 20\%:$** Moderate (Noticeable star degradation in corners)
* **$20\% - 30\%:$** Severe (Requires mechanical tilt adjustment)
* **$> 30\%:$** Extreme (Critical optical misalignment)

### 9.3 Off-Axis Optical Aberrations & Roundness
Off-axis aberration evaluates field curvature as the difference between outer field HFD and central field HFD:

$$\Delta\text{HFD} = \text{HFD}_{\text{out}} - \text{HFD}_{\text{centre}}$$

Stellar roundness (aspect ratio) is calculated by dividing stellar profiles along a rotating 180-degree split line. The ratio of maximum to minimum average radial distance yields the aspect ratio ($1.0$ indicates a circular stellar profile).

### 9.4 Sky Quality Meter (SQM) & Extinction
Absolute sky background brightness (in $\text{mag/arcsec}^2$) is measured by calibrating background flux against catalog stars and applying Pickering airmass formulas to account for atmospheric extinction:

$$\text{Airmass} \approx \text{Pickering}(h) \implies \text{Extinction} = 0.2811 \times \text{Airmass}$$

Where $h$ is target altitude angle above the horizon.

---

## 10. Data Formats and Specifications

SkyStacker supports scientific imaging file standards for astronomical data exchange.

### 10.1 TIFF 6.0 and IEEE 754 32-Bit Float Format
Master scientific stacks are exported as uncompressed **32-bit floating-point TIFF 6.0** files.

* **Byte Order Markers:** `0x4949` (`II`, Little-Endian) or `0x4D4D` (`MM`, Big-Endian).
* **Magic Identifier:** `42` (`0x002A`).
* **Required Tag Values:**
  * **Tag 258 (`BitsPerSample`):** `32` bits per channel.
  * **Tag 339 (`SampleFormat`):** Set to `3` (`IEEEFP`), indicating 32-bit floating-point values.
  * **Tag 262 (`PhotometricInterpretation`):** Set to `1` (`BlackIsZero`) for monochrome or `2` (`RGB`) for color.

### 10.2 BigTIFF Specification ($> 4\text{ GB}$)
When high-resolution master stacks or wide-field mosaics exceed the $4\text{ GB}$ structural limit imposed by 32-bit offsets ($2^{32}$ bytes), SkyStacker switches to BigTIFF:

* **Magic Identifier:** Changes from `42` (`0x002A`) to `43` (`0x002B`).
* **64-bit Pointers:** Offset pointers, byte counts, and tag parameters expand from `LONG` (32-bit) to `LONG8` (64-bit) data types.

### 10.3 FITS and FITS.FZ (Rice Compressed)
The Flexible Image Transport System (FITS) standard structures data into 2880-byte ASCII header blocks followed by binary data arrays. Rice-compressed FITS (`.fits.fz`) compresses imagery tile-by-tile to reduce disk I/O while preserving raw dynamic range.

### 10.4 XISF (Extensible Image Serialization Format)
XISF (native to PixInsight) uses monolithic or distributed XML metadata headers alongside compressed binary data blocks (using LZ4/Zstandard algorithms). It stores processing history, astrometric WCS solutions, and cryptographic checksums within the file structure.

---

## 11. Command-Line Interface (CLI) Reference

SkyStacker is executed from the terminal using the following invocation syntax:

```bash
skystacker [flags] [input-directory]
```

### 11.1 Complete Flag Options

| Flag | Argument Syntax | Technical Description | Default Value |
| :--- | :--- | :--- | :--- |
| `--output` | `string` | Sets explicit output path directory | `<input>/skystacker_results` |
| `--method` | `string` | Integration algorithm (`sigma`, `winsorized`, `mean`, `median`) | `sigma` |
| `--workers` | `int` | Maximum concurrent worker threads | CPU thread count |
| `--recursive` | `bool` | Recursively inspect subdirectories for supported frames | `false` |
| `--overwrite` | `bool` | Overwrite existing output directory contents | `false` |
| `--no-register` | `bool` | Disable star-based registration alignment pass | `false` |
| `--dark` | `string` | Path to explicit Master Dark file or directory | None |
| `--flat` | `string` | Path to explicit Master Flat file or directory | None |
| `--bias` | `string` | Path to explicit Master Bias file or directory | None |
| `--calib-root` | `string` | Root folder for automated Bias/Dark/Flat calibration structure | None |
| `--stack-only` | `bool` | Execute integration pass only; skip post-processing pipeline | `false` |
| `--post-only` | `bool` | Run post-processing pass using existing intermediate files | `false` |
| `--mask-bg` | `bool` | Mask bright astronomical structures during background fitting | `false` |
| `--bg-degree` | `int` | Polynomial degree for background surface modeling (1–3) | `2` |
| `--stretch` | `float` | Non-linear Asinh/GHS stretching intensity factor | `10.0` |
| `--sigma-low` | `float` | Lower standard deviation rejection threshold boundary | `3.0` |
| `--sigma-high` | `float` | Upper standard deviation rejection threshold boundary | `3.0` |
| `--sharpen-amount`| `float` | Unsharp mask filter strength | `0.8` |
| `--sharpen-radius`| `float` | Unsharp mask filter radius (pixels) | `1.5` |
| `--hdr-amount` | `float` | HDR local-contrast enhancement intensity | `0.5` |
| `--star-radius` | `int` | Star removal mask radius in pixels (`0` = auto-sizing) | `0` |
| `--star-sigma` | `float` | Star detection threshold in standard deviation units | `4.0` |
| `--bg-percentile` | `float` | Luminance percentile used for black point neutralization | `10.0` |
| `--scale` | `int` | Downscaling factor (`2` = half dimension, `4` = quarter) | `1` |
| `--force` | `bool` | Bypass safety checks for system RAM bounds | `false` |
| `--tiff` | `bool` | Export optional intermediate raw uncompressed float32 TIFF sum | `false` |
| `--post-ca` | `bool` | Enable chromatic aberration reduction pass | `false` |
| `--post-color` | `bool` | Enable color calibration normalization pass | `false` |
| `--post-flatten` | `bool` | Enable residual vignetting flattening pass | `false` |
| `--post-sharpen` | `bool` | Enable unsharp masking detail enhancement pass | `false` |
| `--post-hdr` | `bool` | Enable HDR local contrast enhancement pass | `false` |
| `--post-stretch` | `bool` | Enable 3-level contrast stretching pass | `false` |
| `--no-color-calib`| `bool` | Disable color calibration during post-processing | `false` |
| `--no-sharpen` | `bool` | Disable sharpening filter during post-processing | `false` |
| `--selftest` | `bool` | Execute internal self-tests and exit binary | N/A |
| `--version` | `bool` | Print current system version identifier and exit | N/A |
| `--help` | `bool` | Print command line option documentation and exit | N/A |

### 11.2 Execution Commands

#### Standard Pipeline Execution
```bash
./skystacker \
  --method winsorized \
  --sigma-low 2.8 \
  --sigma-high 3.2 \
  --calib-root ./M31-session/calibration \
  --mask-bg \
  --stretch 12.5 \
  --output ./M31_stacked_output \
  ./M31-session/lights
```

#### Diagnostic Execution Analysis (ASTAP Engine Flag)
```bash
astap -f target.fits -r 30 -analyse -z 0
```

* **Windows OS Return Encoding (`%errorlevel%`):** Encodes median HFD and star detection count into a single integer return code:
  $$\%errorlevel\% = \text{int}(\text{median HFD} \times 1,000,000) + \text{stars detected}$$
* **Linux/macOS Output Stream (`stdout`):**
  ```text
  HFD_MEDIAN=3.3 STARS=666
  ```

---

## 12. Output Directory Schema (`skystacker_results/`)

Upon execution completion, SkyStacker outputs scientific, visual, and telemetry artifacts to the output directory (`skystacker_results/`):

```text
skystacker_results/
├── master_stack_integration.fits  # 32-bit IEEE 754 Float Master FITS File
├── master_stack_preview.tiff       # Uncompressed 32-bit Float TIFF Master (SampleFormat=3)
├── stacked_processed.jpg           # 8-bit stretched visual RGB preview image
├── stacked_max.jpg                 # Diagnostic Maximum-Pixel composite visual image
├── stacked_min.jpg                 # Diagnostic Minimum-Pixel composite visual image
├── target_observation.wcs          # Solved 80-column WCS Celestial Coordinate Header
├── execution_parameters.ini        # Machine-readable System Execution State Configuration
└── telemetry_audit.log             # JSON Telemetry Audit Log & Processing Trace Report
```

---

## 13. Advanced Architectural Extensions & Dual-Stack Recombination

### 13.1 Dual-Stack Comet and Ephemeris Recombination
Comets and solar system targets move across background star fields. Standard sidereal alignment results in either trailed stars or a blurred comet nucleus. SkyStacker supports a dual-stack recombination workflow (utilizing StarNet2 integration):

```
                       [ Calibrated Sub-Exposures ]
                                    │
                                    ▼
                      [ Neural Star Separation Engine ]
                                   ╱ ╲
                                  ╱   ╲
                                 ╱     ╲
                                ▼       ▼
                     [ Star-Only ]     [ Target-Only ]
                        Frames             Frames
                          │                  │
                          ▼                  ▼
                   [ Sidereal Star    [ Ephemeris Drift
                     Alignment ]         Alignment ]
                          │                  │
                          ▼                  ▼
                   [ Integrated Star  [ Integrated Target
                     Master Stack ]      Master Stack ]
                          ╲                  ╱
                           ╲                ╱
                            ▼              ▼
                     [ Recombined Master Output Stack ]
```

1. **Subframe Conditioning:** Calibrated subframes are converted into floating-point buffers.
2. **Neural Separation:** AI layers split subframes into star-only images and target-only images.
3. **Sidereal & Orbital Drift Alignment:** Star-only subframes are aligned to stellar coordinates. Target-only subframes are aligned using JPL Horizons ephemeris drift vectors ($\Delta\alpha, \Delta\delta$ in arcseconds/hour).
4. **Dual Integration:** Independent master stacks are integrated using sigma rejection.
5. **Floating-Point Recombination:** The star master stack and target master stack are merged in 32-bit floating-point space, producing an output image with both sharp stars and a sharp comet nucleus.

### 13.2 Ensemble Photometric Calibration Framework
Instrumental magnitudes ($m = m_0 - 2.5 \log_{10}(\text{Flux})$) are calibrated against catalog standards using ensemble zero-point aggregation ($m_{0,\text{ensemble}}$):

$$m_{0,\text{ensemble}} = \sum_{i=1}^{K} \left( \frac{w_i}{\sum_{j=1}^{K} w_j} \cdot m_{0,i} \right) \quad \text{where} \quad w_i = \text{SNR}_i$$

Outlier zero-point residuals deviating by $> 1.5\sigma$ from the median are rejected using Median Absolute Deviation (MAD):

$$\text{MAD} = \text{median}\left( |m_{0,i} - \text{median}(m_0)| \right) \implies \sigma_{\text{MAD}} = 1.4826 \times \text{MAD}$$

Transformation parameters convert instrumental color indices into standard filter passbands ($B, V, R$):

$$V_{\text{var}} = \Delta v + T_{v\_bv} \cdot T_{bv} \cdot \left( (b-v)_{\text{var}} - (b-v)_{\text{comp}} \right) + V_{\text{comp}}$$

---

## 14. Technology Roadmap

| Phase | Architectural Feature | Functional Specification | System Benefit |
| :--- | :--- | :--- | :--- |
| **Phase 1** | **Advanced LNC Stacking** | Local Normalization Correction spatial gradient matching across frames. | Removes local background skyglow shifts across subframes. |
| **Phase 1** | **Drizzle Reconstruction** | Variable-pixel linear reconstruction for undersampled frame sets. | Restores subpixel spatial resolution on short-focal datasets. |
| **Phase 2** | **Native SPCC Engine** | Spectrophotometric Color Calibration against Gaia DR3 spectral database. | Establishes photometric color accuracy. |
| **Phase 2** | **Embedded AI Model Runtime** | Integrated neural network inference runtime for star separation. | Removes external tool binary dependencies. |
| **Phase 3** | **Native GPU HAL Integration** | CUDA, Vulkan, and Metal GPU acceleration for spatial tile stacking. | Accelerates integration processing on large datasets. |
| **Phase 3** | **Multi-Panel Mosaic Engine** | Automated WCS-SIP polynomial stitching for wide-field mosaics. | Assembles multi-panel wide-field sky surveys. |
| **Phase 4** | **AAVSO Scientific Reporting** | Automated export of variable star photometry reports. | Formats research data for submission to scientific databases. |
| **Phase 4** | **Real-Time Live API** | REST / gRPC live streaming API for observatory control systems. | Enables remote observatory automation. |
| **Phase 4** | **Compressed FITS Ingestion** | Direct tile read/write support for `.fits.fz` Rice compression. | Reduces storage footprint and disk I/O throughput requirements. |

---

## 15. Source Catalog & Literature References

| ID | Source Title | Source Type | Technical Domain & Contribution |
| :--- | :--- | :--- | :--- |
| **01** | ASTAP, Astrometric Stacking Program - HNSky | Software Reference | Astrometric alignment algorithms, mosaic stacking, plate solving, and calibration. |
| **02** | Architectural Specification for a Go CLI Engine | Engineering Spec | Engine architecture, memory management with `sync.Pool`, spatial tiles, float32 TIFF. |
| **03** | Asinh transformation [Siril 1.0] | Documentation | Mathematical foundations of inverse hyperbolic sine transformation. |
| **04** | AsinhStretch — Astropy v8.0.2 | Code Library | Reference implementation for non-linear AsinhStretch in astrophotography. |
| **05** | Astrophotography — Art and Science | Academic Paper | Comparative workflows between Siril, PixInsight, Affinity Photo, and AI tools. |
| **06** | Deep-Sky Stacking Software Benchmark | Technical Guide | RAM memory requirements, Apple Silicon scalability, performance comparisons. |
| **07** | Eliminating Code Smells in Software Systems | Code Audit Guide | Static analysis techniques and elimination of performance-degrading patterns. |
| **08** | BornAgain-21 TIFF Scientific Data Format | Specification | TIFF format specifications for high-precision scientific data storage. |
| **09** | C++ Swiss Table Architecture: SoA Layout | Systems Paper | Open addressing hash table design, SoA layout, control bytes, $H_1/H_2$ signatures. |
| **10** | Technical Architecture of Processing Engines | Engineering Spec | Photometric quality metrics ($Q_i$), HAL abstraction, GPU Compute, DAG pipelines. |
| **11** | Faster Go Maps with Swiss Tables | Language Specification | Internal replacement of `map` runtime in Go 1.24+ with Swiss Tables. |
| **12** | imagemeta Go Library | Code Library | Efficient extraction of EXIF, IPTC, and XMP metadata in Go without heap allocation. |
| **13** | Siril Automation Scripts | Script Repository | Automation scripts, sensor banding diagnostics, aperture photometry. |
| **14** | Astronomical Processing Terminology Glossary | Technical Manual | Definitions of astronomical terms, IKSS scale estimators, FWHM, PSF. |
| **15** | Preprocessing of Raw Data with PixInsight | Technical Manual | Dark calibration, local normalization (*LocalNormalization*), noise evaluation. |
| **16** | Go Map Mechanics: Buckets to Swiss Tables | Systems Paper | Internal mechanics of Go maps, CPU cycle comparison, memory footprint analysis. |
| **17** | PI SubframeSelector Script Architecture | Forum Literature | Individual frame weighting based on FWHM, eccentricity, and signal-to-noise ratio. |
| **18** | Subframe Selector Methodology | Video Demonstration | Practical methodology for evaluating subframes prior to master integration. |
| **19** | PixInsight Image Processing Engine | Software Release | Image measurement, parallel XISF compression, BigTIFF processing. |
| **20** | Local Normalization Mechanics | Technical Forum | Local gradient correction mechanism using spatial reference images. |
| **21** | Sequence Illumination Equalization | Educational Resource | Tutorial on local illumination normalization across image sequences. |
| **22** | SkyStacker README Repository File | Project Documentation | SkyStacker capabilities, static compilation, self-tests, CLI options. |
| **23** | ITU Revision 6.0 TIFF Standard Specification | Standard Document | Official technical specification of TIFF 6.0, IFD structure, tags, data types. |
| **24** | Raw Data Preprocessing & Optical Shadows | Tutorial Manual | Preprocessing equations, flat calibration, optical shadow correction. |
| **25** | Stacking Rejection Options in Photo Engines | Software Documentation | Stacking methods and outlier rejection parameters in photo software. |
| **26** | Stacking Estimators — Siril Documentation | Software Manual | Robust scale estimators (IKSS, MAD), frame selection, background normalization. |
| **27** | Static Code Analysis for Concurrent Systems | Security Manual | Static analysis strategies for vulnerability prevention and system optimization. |
| **28** | SwissMap: Golang Hash Table Implementation | Engineering Blog | Go port of Swiss Tables (`dolthub/swiss`), fast modulo reduction, SIMD optimizations. |
| **29** | SkyStacker Technical Specification Document | Engineering Spec | Specification of SkyStacker engine: CLI contract, 6-stage pipeline, numerical bounds. |
| **30** | TIFF File Format Specification & BigTIFF | Standard Document | Internal TIFF structure, strips, tiles, 64-bit BigTIFF, and GeoTIFF extensions. |
| **31** | Graphics File Formats: TIFF Technical Summary | Encyclopedia Reference | Structural summary of TIFF 6.0 formats, byte order, IEEE float data types. |
| **32** | MATLAB Interface to LibTIFF Routines | Code Documentation | MATLAB LibTIFF interface properties for reading/writing scientific TIFF tags. |
| **33** | Double Indexed Hash Table Design | Systems Paper | Double-indexed hash table design for collision minimization and L1 cache optimization. |
| **34** | Refactoring Software Code Smells | Engineering Guide | Code smell classification and refactoring strategies for software maintainability. |
| **35** | Image Integration & Stochastic Noise Theory | Educational Resource | Stochastic noise theory, SNR gain by square root law, outlier rejection. |
| **36** | Technical Debt Diagnosis in Concurrent Systems | Technical Article | Technical debt diagnosis and continuous source code improvement. |
| **37** | Preprocessing, Registration & Photometry | User Manual | Preprocessing functions, registration, aperture photometry, CLI commands. |
| **38** | Code Review Methodologies for Engineering Teams | Engineering Standard | Code review methodologies to ensure quality standards in concurrent repositories. |
| **39** | GeoTIFF Readers in Go Runtime | Code Library | Implementation of GeoTIFF readers in Go with spatial coordinate support. |
| **40** | Low-Allocation EXIF Reader for Go | Code Library | Low-memory impact decoding of EXIF and MakerNotes tags in Go. |
| **41** | Scientific Array Manipulation in BigTIFF | Software Documentation | Python library for manipulating NumPy arrays in TIFF, BigTIFF, and OME-TIFF formats. |
| **42** | FITS Standard Specification (NASA/IAU) | Standard Document | Structure of Flexible Image Transport System (FITS) format and metadata headers. |
| **43** | XISF Specification 1.0 (Pleiades Astrophoto) | Standard Document | Architecture of Extensible Image Serialization Format from PixInsight. |
| **44** | GraXpert AI Gradient Extraction Engine | Software Manual | Application of neural networks for removing complex light pollution gradients. |
| **45** | Multiscale Wavelet Analysis in Astronomy | Scientific Article | Mathematical implementation of *à trous* wavelet transforms for detail enhancement. |
| **46** | Simple Imaging Polynomial (SIP) Distortion | Standard Document | Mathematical modeling of optical field distortions in spatial WCS coordinates. |
| **47** | Concurrency Pipelines & Context in Go | Engineering Specification | Design patterns for goroutine lifecycle management using `context.Context`. |
| **48** | FITS World Coordinate System (WCS) Standard | Standard Document | Algorithms mapping physical pixel coordinates to celestial equatorial coordinates. |
| **49** | GPU Offloading in Image Integration Engines | Systems Paper | Hardware Abstraction Layers (HAL) to offload matrix operations to GPUs. |
| **50** | Convolutional Networks for Neural Star Removal | Software Manual | Neural networks (StarXTerminator) applied to image deconstruction and star isolation. |

---

## 16. Technical Keyword Glossary

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
22. **Photometric Calibration** — Removal of sensor and optical path artifacts from raw imaging data.
23. **Master Bias Frame** — Calibration master recording electronic readout offset at minimal exposure duration.
24. **Master Dark Frame** — Calibration master recording exposure- and temperature-dependent thermal dark current.
25. **Master Flat Frame** — Calibration master mapping optical vignetting and dust mote shadows across the light path.
26. **Flat Normalization Factor ($\bar{I}_{\text{flat}}$)** — Mean scalar intensity value of the Master Flat frame used to preserve scale.
27. **Fixed-Pattern Noise (FPN)** — Spatial variations in pixel sensitivity across a digital image sensor.
28. **Full Width at Half Maximum (FWHM)** — Spatial width of a stellar intensity profile measured at half its peak amplitude.
29. **Stellar Eccentricity ($e$)** — Measure of stellar profile elipticity; $0$ represents a circular profile.
30. **Quality Metric Score ($Q_i$)** — Dynamic rating combining FWHM, eccentricity, SNR, and PSF weight to score subframes.
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
55. **Bayer Color Filter Array (CFA)** — Matrix of RGB color filters placed over digital image sensor pixels.
56. **AstroC Demosaicing** — Debayering routine designed to reconstruct color in saturated stellar cores.
57. **AstroM Demosaicing** — Debayering routine rendering undersampled star profiles neutral white to prevent color artifacts.
58. **AstroSimple Debayering** — De-mosaicing algorithm extracting RGB channels within localized $2 \times 2$ pixel neighborhoods.
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
83. **IEEE 754 32-Bit Float Format** — Binary floating-point data representation preserving scientific dynamic range without clipping.
84. **TIFF 6.0 Standard** — Scientific raster file specification supporting floating-point sample formats (`SampleFormat=3`).
85. **64-Bit BigTIFF Format** — File standard extension breaking the classic $4\text{ GB}$ structural limit using 64-bit offsets (`LONG8`).
86. **Image File Directory (IFD)** — Internal structure in TIFF files storing metadata tags, offset pointers, and channel dimensions.
87. **Flexible Image Transport System (FITS)** — NASA/IAU standard scientific file format utilizing 2880-byte header blocks.
88. **Rice Compression (`.fits.fz`)** — Tile-based lossless compression algorithm reducing FITS storage requirements.
89. **Extensible Image Serialization Format (XISF)** — Astronomical file standard using XML metadata and compressed memory streams.
90. **Zero-Allocation Metadata Reader (`imagemeta`)** — Go package reading EXIF/IPTC metadata headers without triggering heap allocation.
91. **Dual-Stack Ephemeris Recombination** — Processing technique separating stars and comets to stack each along their respective motion vectors.
92. **Orbital Motion Alignment** — Differential alignment matching non-sidereal targets using ephemeris drift vectors ($\Delta\alpha, \Delta\delta$).
93. **Ensemble Photometric Zero-Point ($m_{0,\text{ensemble}}$)** — Weighted mean calibration reference derived from multiple catalog stars.
94. **Median Absolute Deviation (MAD)** — Robust statistical scale estimator measuring dispersion resilient to extreme outliers.
95. **Passband System Transformation** — Conversion equations transforming instrumental magnitude colors into standard photometric filters ($B, V, R$).
96. **Local Normalization Correction (LNC)** — Equalization algorithm matching spatial background variations across subframe sequences.
97. **Drizzle Subpixel Integration** — Linear image reconstruction algorithm restoring spatial resolution on undersampled subframe sets.
98. **Code Smell Refactoring** — Systematic code cleanup eliminating anti-patterns to improve performance and maintainability.
99. **`CGO_ENABLED=0` Native Compilation** — Build setting disabling C bindings to produce pure statically compiled Go binaries.
100. **Audit Telemetry Log (`telemetry_audit.log`)** — Structured JSON execution trace recording rejection counts, HFD metrics, and system performance.
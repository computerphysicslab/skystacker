# SkyStacker: Core Engine Technical Specification & Processing Architecture

## 1. Executive Overview and Core Purpose

**SkyStacker** is an open-source, high-performance, local-first command-line tool (CLI) engineered in Go for processing, aligning, calibrating, and integrating deep-sky astrophotography image sequences. Modern Complementary Metal-Oxide-Semiconductor (CMOS) and Charge-Coupled Device (CCD) astronomical sensors generate ultra-high-resolution sub-exposures (frequently exceeding 60 megapixels at 16 bits per sample). Imaging sessions routinely yield hundreds of sub-exposures, resulting in raw datasets scaling from tens to hundreds of gigabytes.

SkyStacker eliminates network transfer bottlenecks, desktop Graphical User Interface (GUI) latency, dynamic heap thrashing, and third-party cloud service dependencies by running natively on local observatory edge hardware or user workstations. It provides an automated, reproducible, resource-conscious, and mathematically rigorous workflow that transforms raw, uncalibrated sub-exposures into scientific masters and post-processed visualizations.

---

## 2. Core Execution Workflows

SkyStacker models two primary operational execution topologies: the **SkyStacker 9-Step Automated Pipeline** and the internal **ASTAP 5-Step In-Memory Stacking Workflow**.

```text
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
2. **Frame Decoding & Resource Safeguards:** Decodes pixel channels into 32-bit floating-point (`float32`) memory buffers. Memory safety checks evaluate dataset dimensions against available system RAM to prevent Out-Of-Memory (OOM) kernel panics.
3. **Photometric Calibration:** Subtraction of thermal dark current and electronic bias offset, followed by division by spatial master flat fields.
4. **Star Detection & Reference Frame Selection:** Identifies stellar Point Spread Functions (PSFs) above local sky noise ($>5\sigma$). Automatically ranks sub-exposures based on Full Width at Half Maximum (FWHM), eccentricity ($e$), and Signal-to-Noise Ratio (SNR), selecting the optimal frame as the global geometric reference.
5. **Spatial Frame Registration:** Computes geometric star-pattern triangles or irregular 2D quad structures. Evaluates affine and similarity transformations using RANSAC (RANdom SAmple Consensus) outlier filtering.
6. **Subpixel Resampling and Safe Cropping:** Aligned sub-exposures are resampled using high-order Lanczos/Sinc interpolation kernels. Bounding-box cropping isolates the overlapping spatial domain valid across all registered subframes.
7. **Statistical Pixel Integration (Stacking):** Accumulates registered temporal pixel arrays using statistical rejection filters (`sigma`, `winsorized`, `mean`, `median`). Concurrently computes diagnostic maximum and minimum intensity stacks.
8. **Non-Linear Post-Processing:** Applies background-gradient extraction, chromatic-aberration reduction, color calibration, Generalized Hyperbolic Stretch (GHS), and local detail enhancement.
9. **Scientific Export & Audit Telemetry:** Writes uncompressed 32-bit IEEE 754 float master files (`.tiff` / `.fits`), 8-bit visual previews (`.jpg`), and a structured session audit report (`report.txt` / `telemetry_audit.log`).

### 2.2 ASTAP Core 5-Step In-Memory Stacking Workflow

For environments optimized strictly for memory throughput without intermediate disk I/O:

1. **Master Flat Normalization:** Master flat subframes are integrated into an average frame. Combined flat-dark or bias frames are subtracted. The master flat undergoes $2 \times 2$ neighborhood mean spatial normalization:
   $$\text{Master Flat} = \frac{\frac{1}{N} \sum \text{Flats} - \frac{1}{N} \sum \text{Flat-Darks}}{\bar{I}_{\text{flat}}}$$
2. **Master Dark Integration:** Dark sub-exposures taken at identical sensor temperature and exposure durations are combined:
   $$\text{Master Dark} = \frac{1}{N} \sum \text{Darks}$$
3. **Pure Signal Extraction:** For each raw sub-exposure, the Master Dark is subtracted in-memory to isolate photon arrival counts from thermal dark current and electronic readout offset.
4. **Optical Normalization:** Dark-subtracted frame buffers are divided by the Master Flat in-memory to eliminate dust motes and vignetting.
5. **In-Memory Frame Combination:** Corrected subframes are registered and accumulated directly within thread-safe spatial memory buffers without intermediate disk writes.

---

## 3. High-Throughput System Architecture & Go Concurrency Model

### 3.1 Static Compilation & Zero-Dependency Binary Design
Building SkyStacker with native Go tools (`CGO_ENABLED=0`) produces a statically linked, standalone binary. This design operates without external system dynamic library dependencies (such as `libc`, `libtiff`, or GTK runtimes), ensuring deployment consistency across Linux, macOS, Windows, and ARM-based edge observatories.

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        Go Application Code                             │
├────────────────────────────────────────────────────────────────────────┤
│           Tile Streaming Pipeline          sync.Pool Allocators        │
├────────────────────────────────────────────────────────────────────────┤
│               Statically Compiled Go Runtime (CGO_ENABLED=0)            │
└────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Spatial Tile Streaming (`tile-based streaming`)
To prevent system RAM exhaustion when processing massive subframe sequences (e.g., 100 frames at 61 MP requires $>24.4\text{ GB}$ if materialized simultaneously), SkyStacker splits dynamic frame grids into discrete spatial tiles (e.g., $512 \times 512$ or $1024 \times 1024$ pixels).

Execution threads stream corresponding spatial tiles sequentially from disk into pre-allocated memory buffers. System memory usage is governed by the number of active CPU worker threads rather than the total dataset size or stack depth:

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

By requesting and returning `*TileBuffer` pointers to the pool, heap allocations during spatial tile processing remain at zero, keeping system memory consumption deterministic (typically between `100 MB` and `300 MB`).

### 3.4 Concurrency Patterns and Goroutine Lifecycle
Processing tasks execute across a concurrent pipeline using Go channels (`chan`). Worker goroutines act as independent processing nodes consuming work units (file paths or spatial tile indices) and pushing processed outputs down the pipeline.

```text
[ Disk Read Stage ] ──chan *TileBuffer──► [ Calibration Stage ] ──chan *TileBuffer──► [ Stacking Stage ]
         │                                       │                                     │
         └─────────────────── context.Context (Cancellation & Telemetry) ──────────────┘
```

Goroutine lifecycles are controlled using `context.Context` cancellation objects (`context.WithCancel`). If a registration subroutine fails or a file read error occurs, the context is globally canceled. This signals all concurrent goroutines to terminate processing and return active `*TileBuffer` pointers to `sync.Pool`, preventing goroutine leaks and memory locks.

### 3.5 Advanced Hash Engine: Swiss Tables Architecture
SkyStacker leverages the Swiss Tables hash table design (integrated natively into the Go 1.24+ runtime and implemented via `dolthub/swiss`). Swiss Tables improve key indexing, star matching lookup, and EXIF tag caching compared to traditional hash structures.

#### Structure of Arrays (SoA) vs. Array of Structures (AoS)
Traditional hash tables organize data as an Array of Structures (AoS), storing keys, values, and control flags contiguously. A single 64-byte L1 CPU cache line holds few entries, dragging unneeded payload data into cache during lookup operations.

Swiss Tables use a Structure of Arrays (SoA) design, separating control metadata from data payloads:

```text
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
2. **$H_2$ (Lower 7 bits):** Stored in the control byte array as a fingerprint signature ($0 \dots 127$). Control values also designate empty (`-128` / `0b10000000`) or deleted (`-2` / `0b11111110`) slots.

To locate a key within a group of control bytes, SWAR (SIMD Within A Register) operations or SIMD vector instructions inspect up to 16 slots concurrently:

$$\text{matchMask} = (\text{ctrl} \oplus \text{broadcast}(H_2) - 0x0101 \dots) \ \& \ \sim(\text{ctrl} \oplus \text{broadcast}(H_2)) \ \& \ 0x8080 \dots$$

This parallel bitwise evaluation eliminates up to $99.22\%$ of key mismatch checks. Full key equality is verified only when an $H_2$ fingerprint match occurs. Swiss Tables maintain performance at up to an **$87.5\%$** load factor ($7/8$), reducing memory footprint by $30\text{--}60\%$ relative to traditional maps. Go's runtime further groups entries into a directory of independent sub-tables (up to 1024 entries each) to enable bounded incremental growth without latency spikes.

---

## 4. Photometric Calibration & Noise Modeling

### 4.1 Calibration Frames & Mathematical Formulation
* **Master Bias Frame:** Captured at shortest exposure with shutter closed; records uniform electronic readout offset.
* **Master Dark Frame:** Captured in darkness at matching exposure duration and sensor temperature; records thermal dark current and hot pixels.
* **Master Flat Frame:** Captured against a uniform light field; records pixel-to-pixel sensitivity variation, optical vignetting, and dust shadows.

Photometric calibration converts raw Analog-to-Digital Units ($\text{ADU}$) into corrected pixel intensities $I_{\text{calibrated}}(x,y)$:

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

Where $\bar{I}_{\text{flat}}$ is the scalar mean intensity of the Master Flat, used to normalize the output values back to the sensor's dynamic range.

### 4.2 Frame Evaluation Metrics & Quality Score ($Q_i$)
Before integration, subframes undergo photometric analysis. Individual sub-exposures are scored using a dynamic quality metric ($Q_i$):

$$Q_i = w_1 \cdot \left(\frac{1}{\text{FWHM}_i}\right) + w_2 \cdot (1 - e_i) + w_3 \cdot \text{SNR}_i + w_4 \cdot \text{PSFWeight}_i$$

Where:
* $\text{FWHM}_i$ is the average Full Width at Half Maximum of stellar profiles (in pixels or arcseconds).
* $e_i$ is the average stellar eccentricity ($0 \le e_i < 1$, where $e_i=0$ represents a circular star profile).
* $\text{SNR}_i$ is the background Signal-to-Noise Ratio.
* $\text{PSFWeight}_i$ is the integrated stellar signal energy relative to background noise variance.
* $w_1, w_2, w_3, w_4$ are configurable weighting coefficients.

Sub-exposures falling below user-defined quality thresholds are flagged and excluded from the registered master stack.

---

## 5. Star Detection, Subpixel Alignment & Astrometry

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

```text
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
* **HFD Filter (`-m`):** Excludes single-pixel noise spikes by filtering out detections below `0.8` pixels (or `< 1.5"` for plate solving).

### 5.3 Transformation Matrix Estimation & RANSAC Filtering
Matched star pairs are used to compute a 2D similarity transformation matrix (scale $s$, rotation angle $\theta$, translations $t_x, t_y$):

$$\begin{bmatrix} x' \\ y' \end{bmatrix} = s \begin{bmatrix} \cos\theta & -\sin\theta \\ \sin\theta & \cos\theta \end{bmatrix} \begin{bmatrix} x \\ y \end{bmatrix} + \begin{bmatrix} t_x \\ t_y \end{bmatrix}$$

An iterative RANSAC filter selects random minimum sample subsets, fits candidate transformation matrices, and identifies inliers. Matches exceeding subpixel reprojection error thresholds are rejected, suppressing false stellar matches caused by hot pixels or cosmic ray hits. After matrix validation, target subframes are resampled using Lanczos/Sinc interpolation and cropped to the valid spatial overlap region common across all stacked frames.

### 5.4 Astrometric Plate Solving & SIP Polynomial Distortion Modeling
SkyStacker supports blind astrometric plate solving to match detected stars against reference catalogs (such as Gaia DR3). Solved frames are assigned World Coordinate System (WCS) headers mapping physical pixel coordinates $(x,y)$ to celestial equatorial coordinates $(\alpha, \delta)$ (Right Ascension and Declination).

To correct field curvature, coma, and optical distortion, the engine computes 3rd-order Simple Imaging Polynomial (SIP) distortion coefficients:

$$u = f(x,y) = \sum_{p+q \le 3} A_{p,q} \, x^p y^q = x + A_{2,0} x^2 + A_{1,1} xy + A_{0,2} y^2 + A_{3,0} x^3 + A_{2,1} x^2 y + A_{1,2} x y^2 + A_{0,3} y^3$$

$$v = g(x,y) = \sum_{p+q \le 3} B_{p,q} \, x^p y^q = y + B_{2,0} x^2 + B_{1,1} xy + B_{0,2} y^2 + B_{3,0} x^3 + B_{2,1} x^2 y + B_{1,2} x y^2 + B_{0,3} y^3$$

Where $u, v$ are true spherical focal plane coordinates, and $A_{p,q}, B_{p,q}$ are SIP distortion matrix parameters written to output headers.

---

## 6. Statistical Pixel Integration (Stacking Methods)

### 6.1 Stochastic Noise Theory & Theoretical SNR Gain
Astronomical photon signal ($S$) accumulates linearly with the number of integrated subframes ($N$), while stochastic sensor read noise and background noise ($\sigma$) sum in quadrature ($\sqrt{N}$). The SNR of the integrated master image ($\text{SNR}_{\text{stack}}$) scales according to:

$$\text{SNR}_{\text{stack}} = \sqrt{N} \cdot \text{SNR}_{\text{sub}}$$

```text
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

| Stacking Method | Mathematical Formula | SNR Gain Factor | Outlier Rejection Performance | Complexity | Primary Astrophotography Application |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Sum Integration** | $I_{\text{sum}} = \sum_{i=1}^{N} I_i$ | $\sqrt{N}$ (Expands dynamic range) | None (Retains all transient artifacts) | $\mathcal{O}(N)$ | Photometric counting, high-speed Lucky Imaging. |
| **Arithmetic Mean (`mean`)** | $I_{\text{mean}} = \frac{1}{N} \sum_{i=1}^{N} I_i$ | $1.00 \cdot \sqrt{N}$ (Theoretical Max) | Zero (Distorted by non-Gaussian noise) | $\mathcal{O}(N)$ | Master Calibration ($Darks/Flats$) or clean $N < 5$ sets. |
| **Median (`median`)** | $I_{\text{median}} = \text{median}(I_i)$ | $\approx 0.80 \cdot \sqrt{N}$ (~20% loss) | Complete (Rejects extreme boundaries) | $\mathcal{O}(N \log N)$ | Small stacks ($N \le 9$) or master calibration frames. |
| **Percentile Clipping** | $I_{\text{percentile}} = \frac{1}{\|P\|} \sum_{i \in P} I_i$ | $\approx 0.85 \cdot \sqrt{N}$ | Fixed percentile boundaries | $\mathcal{O}(N \log N)$ | Small sub-exposure counts ($N \le 6$) where $\sigma$ is unstable. |
| **Sigma-Clipping (`sigma`)** | $I_{\text{sigma}} = \frac{1}{\|M\|} \sum_{i \in M} I_i$ | $\approx 0.95 \cdot \sqrt{N}$ | Iterative standard deviation ($k\sigma$) rejection | $\mathcal{O}(I \cdot N \log N)$ | Standard default integration for deep-sky series ($N \ge 31$). |
| **Winsorized Sigma Clipping** | $I_{\text{win}} = \frac{1}{N} \sum_{i=1}^{N} I_i^{\text{win}}$ | $\approx 0.98 \cdot \sqrt{N}$ | Outliers replaced with boundary values | $\mathcal{O}(I \cdot N \log N)$ | Moderate deep-sky datasets ($10 \le N \le 30$). |
| **Linear Fit Clipping** | Fits $y = ax + b$ along stack axis | High on long series | Rejects deviations from linear sky trend | $\mathcal{O}(N)$ | Wide-field sequences with varying sky background gradients. |
| **GESDT Clipping** | Generalized Extreme Studentized Deviate | High | Statistical multi-outlier hypothesis test | $\mathcal{O}(N \log N)$ | Sequences of 31 to 300 sub-exposures with high outlier density. |

### 6.3 Diagnostic Auxiliary Stacks
In addition to the primary integrated master file, SkyStacker generates two control outputs:
* **Pixel Maximum (`Max Stack`):** Retains the absolute maximum pixel intensity recorded along the temporal axis. Used to analyze satellite density, meteor trails, and transient optical reflections.
* **Pixel Minimum (`Min Stack`):** Retains the absolute minimum pixel intensity. Used to inspect background sky uniformity, residual vignetting, and sensor obstructions.

### 6.4 Hardware Abstraction Layer (HAL) & GPU Acceleration
When processing stacks ($N > 300$), iterative statistical sorting operations ($\mathcal{O}(N \log N)$) become compute-bound. SkyStacker incorporates a Hardware Abstraction Layer (HAL) to offload matrix operations. Using SIMD CPU extensions (AVX2/AVX-512) or optional GPU compute backends (CUDA, Vulkan Compute, or Apple Silicon Metal Framework), pixel rejection, matrix transformation, and subpixel interpolation tasks are parallelized across available compute hardware.

---

## 7. One-Shot Color (OSC) Demosaicing

Raw data captured by One-Shot Color (OSC) sensors contains a Bayer Color Filter Array (CFA) matrix pattern. Reconstruction of full-color RGB data requires specialized debayering algorithms:

* **AstroC (Saturated Star Reconstruction):** Evaluates saturated star cores. When star pixels hit sensor saturation boundaries, channel ratios are interpolated from surrounding unsaturated pixels to prevent artificial color tinting in star centers.
* **AstroM (Monochrome Star Balancing):** Analyzes local $2 \times 2$ Bayer blocks ($R, G_1, G_2, B$). For undersampled subframes where color sampling is unstable, it derives a monochrome luminance value, keeping star profiles neutral white.
* **AstroSimple (Han Kleijn Method, CC BY 4.0):** Extracts discrete $R, G_1, G_2, B$ information from localized $2 \times 2$ pixel neighborhoods without cross-channel spatial bleeding. Preserves high-frequency stellar details while minimizing edge color artifacts.
* **Standard Bilinear:** Computes channel interpolation by averaging pixel values across a $3 \times 3$ grid. Produces smooth spatial transitions, but can blur high-frequency stellar profiles.

---

## 8. Non-Linear Post-Processing Pipeline

Stacked astronomical master files exist in a **linear** dynamic state, where faint nebular structures are concentrated in the lower percentiles of the histogram. SkyStacker includes an automated 6-stage post-processing engine to prepare images for visual rendering:

```text
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
   * **Generalized Hyperbolic Stretch (GHS):** Governed by stretching factor $b$ and black point protection parameter $D$ (applied to luminance $L$ with proportional RGB scaling to prevent desaturation):
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

Where $c_N$ is the residual low-frequency background layer, and $w_k$ represents distinct spatial frequency detail scales. High-frequency noise is isolated in fine layers ($w_1, w_2$), while nebular structures are isolated in broader layers ($w_3 \dots w_N$), enabling targeted noise reduction and micro-contrast enhancement without halo artifacts.

### 8.4 Deep Learning AI Model Integration
SkyStacker supports integration with external AI inference models (such as StarXTerminator, NoiseXTerminator, and GraXpert):
* **Starless Workflows:** Convolutional Neural Networks (U-Nets) identify stellar profiles, separate star layers from extended nebular structures, and interpolate background voids. Faint gas structures can then be stretched independently without bloating stellar profiles.
* **AI Denoising:** Self-supervised neural networks suppress background read noise and shot noise while preserving diffraction spikes and point-source stellar flux (requiring cosmic-ray outlier rejection prior to inference).

---

## 9. Automated Image Quality Diagnostics

```text
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

### 9.2 Sensor Tilt Analysis (9-Area Spatial Octagon & 3-Corner Triangle)
The sensor plane is partitioned into nine spatial zones (Corners `1A`, `3A`, `1C`, `3C`; Edges `2A`, `1B`, `3B`, `2C`; Center `2B`). Sensor tilt is calculated as the percentage deviation between the best and worst corner median HFD values:

$$\text{Tilt } \% = 100 \times \frac{\text{HFD}_{\text{worst\_corner}} - \text{HFD}_{\text{best\_corner}}}{\text{HFD}_{\text{median}}}$$

#### Tilt Severity Scale
* **$< 5\%$:** None (Optimal optical alignment)
* **$5\% - 10\%$:** Negligible / Almost None (Operational baseline)
* **$10\% - 15\%$:** Mild (Acceptable operational threshold)
* **$15\% - 20\%$:** Moderate (Noticeable star degradation in corners)
* **$20\% - 30\%$:** Severe (Requires mechanical tilt adjustment)
* **$> 30\%$:** Extreme (Critical optical misalignment)

For tilt adapters equipped with three $120^\circ$ push-pull adjustment screws, a symmetrical **3-corner triangular analysis mode** evaluates three $120^\circ$ radial segments (excluding the inner $25\%$ center), mapping HFD variances directly to physical adapter screws.

### 9.3 Off-Axis Optical Aberrations & Roundness
Off-axis aberration evaluates field curvature as the difference between outer field HFD ($\ge 75\%$ radial distance from sensor center) and central field HFD:

$$\Delta\text{HFD} = \text{HFD}_{\text{out}} - \text{HFD}_{\text{centre}}$$

Spatial HFD variations can be rendered visually via 2D HFD contour maps and Voronoi diagram partitions. Stellar roundness (aspect ratio) is calculated by dividing stellar profiles along a rotating $180^\circ$ split line; the ratio of maximum to minimum average radial distance yields the aspect ratio ($1.0$ indicates a perfectly circular profile).

### 9.4 Sky Quality Meter (SQM) & Extinction
Absolute sky background brightness (in $\text{mag/arcsec}^2$) is measured by calibrating background flux against catalog stars and applying Pickering airmass formulas to account for atmospheric extinction:

$$\text{Airmass} \approx \text{Pickering}(h) \implies \text{Extinction} = 0.2811 \times \text{Airmass}$$

Where $h$ is the target altitude angle above the horizon.

---

## 10. Data Formats and Metadata Specifications

### 10.1 TIFF 6.0 and IEEE 754 32-Bit Float Format
Master scientific stacks are exported as uncompressed **32-bit floating-point TIFF 6.0** files (beginning with an 8-byte header followed by Image File Directories, IFDs):
* **Byte Order Markers:** `0x4949` (`II`, Little-Endian) or `0x4D4D` (`MM`, Big-Endian).
* **Magic Identifier:** `42` (`0x002A`).
* **Required Tag Values:**
  * **Tag 258 (`BitsPerSample`):** `32` bits per channel.
  * **Tag 339 (`SampleFormat`):** Set to `3` (`IEEEFP`), indicating 32-bit floating-point values.
  * **Tag 262 (`PhotometricInterpretation`):** Set to `1` (`BlackIsZero`) for monochrome or `2` (`RGB`) for color.

### 10.2 BigTIFF Specification ($> 4\text{ GB}$)
When high-resolution master stacks or wide-field mosaics exceed the $4\text{ GB}$ structural limit imposed by 32-bit offsets ($2^{32}$ bytes), SkyStacker switches to **BigTIFF**:
* **Magic Identifier:** Changes from `42` (`0x002A`) to `43` (`0x002B`).
* **64-bit Pointers:** Offset pointers, byte counts, and tag parameters expand from `LONG` (32-bit) to `LONG8` (64-bit) data types.

### 10.3 FITS and FITS.FZ (Rice Compressed)
The Flexible Image Transport System (FITS) standard (NASA/IAU) structures data into 2880-byte ASCII header blocks followed by binary data arrays. Rice-compressed FITS (`.fits.fz`) compresses imagery tile-by-tile to reduce disk I/O while preserving raw dynamic range.

### 10.4 XISF (Extensible Image Serialization Format)
XISF (native to PixInsight) uses monolithic or distributed XML metadata headers alongside compressed binary data blocks (using LZ4/Zstandard algorithms). It stores processing history, astrometric WCS solutions, and cryptographic checksums within the file structure.

### 10.5 Zero-Allocation Metadata Ingestion (`imagemeta`)
To verify frame compatibility before stacking without incurring heap allocation overhead, Go libraries such as `imagemeta` parse EXIF, IPTC, and XMP headers directly via fixed buffer reads into lightweight descriptors:

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

---

## 11. SkyStacker Command-Line Interface (CLI) Reference

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

### 11.2 Standard Pipeline Execution Example

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

---

## 12. Advanced Architectural Extensions

### 12.1 Dual-Stack Comet and Ephemeris Recombination
Comets and solar system targets move across background star fields. Standard sidereal alignment results in either trailed stars or a blurred comet nucleus. SkyStacker models a dual-stack recombination workflow (utilizing StarNet2 integration):

```text
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

1. **Subframe Conditioning:** Calibrated subframes undergo Inverse Hyperbolic Sine (`sinh`) stretching to optimize contrast boundaries for neural network evaluation.
2. **Star/Comet Separation:** StarNet2 splits each frame into a star-only image and a comet-only image.
3. **Centroid Remnant Extraction:** When "extract comet remnant" is enabled, star shapes within a $\sim 5$-pixel radius of the calculated ephemeris comet centroid are removed from the star-only frame and shifted into the comet-only stream, preventing residual star cores from corrupting the comet nucleus.
4. **Dual Parallel Integration:** Star-only frames are aligned astrometrically on stellar quads and stacked into a star master stack. Comet-only frames are aligned using JPL Horizons ephemeris drift vectors ($\Delta\alpha, \Delta\delta$ in arcseconds/hour) and stacked into a comet master stack.
5. **Floating-Point Recombination:** The star master stack and target master stack are merged in 32-bit floating-point dynamic space without non-linear clipping, preserving both sharp point-source stars and faint extended comet structures.

### 12.2 Ensemble Photometric Calibration Framework
Instrumental magnitudes ($m = m_0 - 2.5 \log_{10}(\text{Flux})$) are calibrated against catalog standards (e.g., Gaia DR3, Johnson-V) using ensemble zero-point aggregation ($m_{0,\text{ensemble}}$):

$$m_{0,\text{ensemble}} = \sum_{i=1}^{K} \left( \frac{w_i}{\sum_{j=1}^{K} w_j} \cdot m_{0,i} \right) \quad \text{where} \quad w_i = \text{SNR}_i$$

Outlier zero-point residuals deviating by $> 1.5\sigma$ from the median are rejected using Median Absolute Deviation (MAD):

$$\text{MAD} = \text{median}\left( |m_{0,i} - \text{median}(m_0)| \right) \implies \sigma_{\text{MAD}} = 1.4826 \times \text{MAD}$$

Transformation parameters convert instrumental color indices ($b, v, r$) into standard filter passbands ($B, V, R$):

$$V_{\text{var}} = \Delta v + T_{v\_bv} \cdot T_{bv} \cdot \left( (b-v)_{\text{var}} - (b-v)_{\text{comp}} \right) + V_{\text{comp}}$$

Where $T_{bv}$ transforms instrumental color differences into standard color indices, and $T_{v\_bv}$ corrects magnitude shifts resulting from target color variations ($B-V$).

---

## 13. Long-Term Technology Roadmap

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

## Concordance with v0.6.0

`main.go` is still one file. Shipped relative to this design: native FITS,
debayer, calibration masters, self-flat, SER input, pixel-mode comet stack,
Gaia/HEALPix, TAN WCS. Not shipped: compressed FITS, XISF, drizzle, live
API, SIP applied in the resampler. See `TECHNICAL_SPEC.md` for the short
current spec.

---

## 2026-11-16 deep dive addendum (SkyStacker)

See `../05-research-deep-dive-2026-11-16.md` §10–§11: tile-vs-temporal rejection memory budgets (recommend 64×64 sub-tiles for Stage 5 at deep stacks), Swiss Tables/Go 1.24+ notes (no `dolthub/swiss`), TAN–SIP direct/inverse polynomials and need for AP/BP linear terms in resampling, robust rejection algorithms (MAD/Winsorized/linear-fit/GESDT), and non-linear stretch math (MTF, Arcsinh, GHS branch forms) for documentation alignment. Also note SIP is fitted/stored but not applied in the current pixel-registration resampler.

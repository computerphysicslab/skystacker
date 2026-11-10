# Advanced Technical Guide to Stacking Technologies, Processing Pipeline, and Architecture in SkyStacker

SkyStacker is a high-performance, open-source astrophotography processing engine built with a local-first architecture optimized for command-line execution (CLI) using Go. Its core purpose is to fully automate the complex deep-sky astrophotography processing pipeline: from raw data ingestion and advanced photometric calibration, through subpixel star detection and geometric alignment, to statistical pixel integration (stacking) and non-linear post-processing.

This guide provides an exhaustive documentation of the stacking technologies, mathematical foundations, spatial registration algorithms, and configuration parameters embedded in the engine.

---

## 1. Overview and High-Throughput Astrophotography Data Pipelines

Deep-sky astrophotography involves handling massive data volumes. Modern CMOS and scientific CCD sensors produce ultra-high-resolution frames (frequently exceeding 60 megapixels at 16 bits per sample), resulting in tens to hundreds of gigabytes per imaging session.

To operate efficiently without exhausting system RAM or relying on external cloud infrastructure, SkyStacker implements a decoupled, low-latency pipeline driven by **tile-based spatial streaming** (`tile-based streaming`).

### The End-to-End Astrophotography Pipeline

1. **Raw Metadata Ingestion & Indexing:** Recursive directory inspection, parsing of raw sensor headers (Bayer pattern, gain, offset, exposure time, temperature), and TIFF/FITS/EXIF property mapping.
2. **Photometric Calibration Preprocessing:** Pixel-by-pixel application of master calibration frames (`bias`, `dark`, `flat`) in high-precision 32-bit floating-point format (`float32`).
3. **Subpixel Star Registration & Alignment:** Extraction of stellar sources, centroid estimation, triangle pattern matching, and RANSAC-filtered affine/similarity transformation matrix calculation.
4. **Statistical Spatial Integration (Stacking):** Spatial partitioning of frames into memory-mapped buffers (e.g., $1024 \times 1024$ pixel tiles) and temporal accumulation using robust statistical rejection models.
5. **Non-Linear Post-Processing & GHS Stretching:** Background gradient extraction, chromatic aberration correction, color balancing, and Generalized Hyperbolic Stretch (GHS).
6. **Scientific Export & Telemetry:** Atomic writing of 32-bit IEEE float master TIFFs, 8-bit diagnostic pre-visualizations, and comprehensive session audit logs (`report.txt`).

---

## 2. Advanced Photometric Calibration and Noise Modeling

Light frames captured during deep-sky sessions are inherently contaminated by sensor artifacts, thermal current noise, fixed-pattern noise, electronic read noise, vignetting, and optical dust motes. Mathematical calibration is mandatory prior to alignment or stacking.

### Mathematical Formulation of Calibration

The corrected pixel value $I_{\text{calibrated}}(x,y)$ applied to each raw light frame $I_{\text{light}}(x,y)$ is governed by the rigorous sensor model:

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

Where:
* $I_{\text{dark}}(x,y)$ incorporates both the electronic bias offset and the accumulated thermal dark current proportional to exposure time and sensor temperature.
* $I_{\text{flat}}(x,y)$ represents the optical response field (vignetting, dust shadows, vignetting asymmetry).
* $\bar{I}_{\text{flat}}$ is the global scalar average intensity of the master flat frame, acting as a normalization factor to preserve the original dynamic range and mean ADU (Analog-to-Digital Units) levels of the sensor.

---

## 3. Subpixel Star Detection and Geometric Registration (*Star Registration*)

Due to mechanical imperfections in telescope mounts, flexure, polar alignment errors, and field rotation, individual frames exhibit spatial shifts, rotation, and scaling differences. The registration engine aligns all subframes to a common reference framework.

### Algorithmic Stages of Star Registration

1. **Reference Frame Selection:** SkyStacker computes a quality index for every frame by analyzing stellar density, Full Width at Half Maximum (FWHM), and background noise, automatically selecting the sharpest frame with the highest star count as the reference frame.
2. **Subpixel Centroid Estimation:**
   * 8-connected component labeling is performed to isolate stellar profiles exceeding a dynamic threshold of $5\sigma$ above local background noise.
   * Star centroids are calculated using intensity-weighted moments to achieve subpixel precision:
     
     $$x_c = \frac{\sum x \cdot I(x,y)}{\sum I(x,y)}, \quad y_c = \frac{\sum y \cdot I(x,y)}{\sum I(x,y)}$$

3. **Triangle-Based Star-Pattern Matching:**
   * Geometric triangles are constructed using the top 30 brightest stars in the scene.
   * A scale- and rotation-invariant descriptor is calculated based on edge length ratios: $(\min(l_1, l_2, l_3) / \max(l_1, l_2, l_3))$.
4. **Transformation Estimation & RANSAC:**
   * A similarity transformation (translation, rotation, uniform scale) is estimated via closed-form least squares.
   * A **RANSAC (RANdom SAmple Consensus)** robust filter is applied iteratively to eliminate false positive matches (such as hot pixels or cosmic ray hits misidentified as stars).
5. **Subpixel Resampling and Safe Cropping:** Images are resampled using high-order sinc/Lanczos interpolation kernels to prevent aliasing artifacts, followed by a safe bounding-box crop to retain only the common valid overlap region across all stacked frames.

---

## 4. Stacking Technologies and Statistical Pixel Integration

Stacking is the core mathematical process of combining registered frames to suppress random noise while amplifying faint nebulosity and galaxy details.

### Theoretical SNR Improvement

Photon shot noise follows Poisson statistics, where signal accumulates linearly with the number of frames $N$, while random noise (read noise, thermal noise) adds in quadrature ($\sqrt{N}$). The Signal-to-Noise Ratio of the stacked image ($\text{SNR}_{\text{stack}}$) scales according to the square-root rule:

$$\text{SNR}_{\text{stack}} = \sqrt{N} \cdot \text{SNR}_{\text{sub}}$$

### Comprehensive Comparison of Stacking & Rejection Algorithms

| Stacking Method | Theoretical SNR Gain | Outlier Rejection Performance | Computational Complexity | Primary Astrophotography Application |
| :--- | :--- | :--- | :--- | :--- |
| **Sigma-Clipping (`sigma`)** | Near-optimal ($\approx 0.95 \sqrt{N}$) | Excellent (iteratively discards pixel values deviating $> k\sigma$ from median) | $\mathcal{O}(I \cdot N \log N)$ | Default standard for deep-sky integration ($N > 10$). |
| **Winsorized Sigma-Clipping** | Optimal ($\approx 0.97 \sqrt{N}$) | Superior (replaces outliers with boundary values rather than discarding) | $\mathcal{O}(I \cdot N \log N)$ | Deep-sky stacks with severe satellite or airplane trails. |
| **Mean / Average (`mean`)** | Maximum theoretical ($\sqrt{N}$) for pure Gaussian noise | Zero (retains satellite tracks, airplanes, and cosmic ray strikes) | $\mathcal{O}(N)$ | Clean calibration master generation or planetary imaging. |
| **Median (`median`)** | Reduced ($\approx 0.80 \sqrt{N}$, ~20% penalty) | High (eliminates extreme transient artifacts without tuning) | $\mathcal{O}(N \log N)$ | Master calibration frame creation (Darks, Flats, Biases). |
| **Max Stack (`Pixel Max`)** | Not applicable for SNR | Retains absolute maximum pixel value | $\mathcal{O}(N)$ | Diagnostic inspection, meteor showers, and star trails. |
| **Min Stack (`Pixel Min`)** | Not applicable for SNR | Retains absolute minimum pixel value | $\mathcal{O}(N)$ | Inspection of frame boundaries, vignetting residuals, and optical obstructions. |

---

## 5. Non-Linear Post-Processing and Generalized Hyperbolic Stretching (GHS)

Because stacked master files are linear, faint structures (nebulas, dust lanes) reside in the bottom 5% of the dynamic range. SkyStacker features an integrated 6-stage post-processing pipeline.

### Pipeline Stages

1. **Background Extraction & Gradient Removal:** Fits and subtracts a 2nd-degree 2D polynomial surface to model sky glow and light pollution gradients. The `--mask-bg` flag masks bright celestial objects (luminance $> 0.1$) to prevent object clipping during gradient fitting.
2. **Chromatic Aberration Suppression:** Targets purple/magenta fringes by identifying pixels where Red and Blue channel intensities vastly exceed the Green channel response.
3. **Color Calibration:** Normalizes channel medians based on the 10th to 50th luminance percentile range, applying bound-constrained scaling factors within $[0.5, 1.5]$.
4. **Generalized Hyperbolic Stretch (GHS):** Applies a non-linear luminance stretch function governed by stretching factor $b$ and black point protection parameter $D$:
   
   $$f(x) = \frac{\sinh^{-1}\left( \frac{x - D}{b} \right) - \sinh^{-1}\left( \frac{-D}{b} \right)}{\sinh^{-1}\left( \frac{1 - D}{b} \right) - \sinh^{-1}\left( \frac{-D}{b} \right)}$$
   
   This preserves stellar star color saturation and prevents unsightly highlight burnout compared to traditional gamma or simple arcsinh stretches.
5. **Vignette Residual Flattening:** Re-applies a smoothed residual illumination correction post-stretch.
6. **Unsharp Masking:** High-pass spatial filtering to enhance local micro-contrast in nebular filaments and galactic arms.

---

## 6. Command-Line Interface (CLI) and Configuration Reference

SkyStacker is executed from the terminal using the following syntax:

```bash
skystacker [flags] [input-directory]
```

### Key CLI Flags for Stacking and Processing

| Flag | Description | Default Value |
| :--- | :--- | :--- |
| `--method` | Stacking integration method (`sigma`, `winsorized`, `mean`, `median`) | `sigma` |
| `--sigma-low` | Lower sigma rejection threshold | `3.0` |
| `--sigma-high` | Upper sigma rejection threshold | `3.0` |
| `--output` | Output directory for stacked masters and reports | `<input>/skystacker_results` |
| `--dark`, `--flat`, `--bias` | Explicit file paths or folders for calibration frames | None |
| `--calib-root` | Root directory for automated calibration frame discovery | None |
| `--workers` | Maximum concurrent execution worker threads | System CPU thread count |
| `--mask-bg` | Masks bright stellar objects during background extraction | `false` |
| `--stretch` | Generalized Hyperbolic Stretch intensity factor | `10.0` |
| `--stack-only` | Performs pixel stacking only, bypassing post-processing | `false` |

### Example Execution Command

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

## 7. Conclusion and Summary of Output Products

Upon completion, SkyStacker generates the following professional deliverables in the target directory:

* **`TIFF/stacked_master.tiff`:** 32-bit floating-point IEEE uncompressed scientific master integration.
* **`stacked_processed.jpg`:** 8-bit non-linearly stretched visual pre-view with GHS applied.
* **`stacked_max.jpg` / `stacked_min.jpg`:** Diagnostic composite files for artifact and defect inspection.
* **`report.txt`:** Full telemetry audit log detailing frame rejection counts, measured FWHM, SNR improvements, and memory footprints.
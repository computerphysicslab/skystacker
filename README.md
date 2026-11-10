# SkyStacker



SkyStacker is a local-first command-line tool for stacking astrophotography images. It takes a folder of night-sky exposures, detects stars, aligns the frames, combines them into useful image stacks and can apply an automated post-processing workflow.



---

## What does SkyStacker do?



Astrophotography sessions usually produce many individual exposures of the same region of the sky. Even when the camera or mount moves slightly between exposures, the stars should occupy the same positions in the final combined image.

SkyStacker automates the core workflow:

1. **Discovers input frames** in a session directory.


2. **Decodes the images** and prepares their pixel data.


3. **Optionally calibrates** light frames using dark, flat and bias frames.


4. **Detects stars** and selects a reference frame.


5. **Registers the frames** by matching star patterns and estimating geometric transformations.


6. **Resamples and aligns** the exposures before combining them.


7. **Creates image stacks** using the selected statistical method, plus maximum- and minimum-pixel combinations.


8. **Optionally post-processes** the resulting image to improve its background, colour, contrast and detail.


9. **Writes outputs and a processing report** to a results directory.



The aim is to provide a transparent, reproducible and resource-conscious workflow that runs locally, without requiring a cloud service.

## Main capabilities



### Frame registration



* Automatic star detection with sub-pixel centroid estimation.


* Reference-frame selection based on detected star count.


* Triangle-based star-pattern matching.


* Similarity-transform estimation and robust inlier filtering (RANSAC-style matching).


* Sub-pixel image resampling.


* Safe cropping to the region valid across registered frames.


* Frames that cannot be registered are reported and excluded from the registered stack; they can still contribute to the unregistered mean.



### Calibration



Calibration frames can be supplied explicitly:

* **Dark frames** help correct sensor dark signal and hot pixels.


* **Flat frames** help correct illumination variation and vignetting.


* **Bias frames** can be used with flat calibration.



You can pass individual calibration paths or use `--calib-root` to let SkyStacker look for calibration folders under a common directory.

### Stacking methods



| Method | Purpose | 

 |
| --- | --- | --- |
| `mean` | Arithmetic average of registered frames.

 |  |
| `median` | Median combination, useful for rejecting transient features.

 |  |
| `sigma` | Sigma-clipping combination, intended to reject outliers such as hot pixels and transient signals.

 |  |

The default method is `sigma`.

SkyStacker also calculates maximum- and minimum-pixel combinations. These can be useful for inspecting transient features and background behaviour, although they are diagnostic combinations rather than substitutes for a normal integration.

### Post-processing



The post-processing workflow includes background removal and optional processing stages controlled by command-line flags:

* Background-gradient modelling and removal.


* Optional masking of bright objects during background extraction.


* Chromatic-aberration reduction.


* Colour calibration.


* Vignette/background flattening.


* Unsharp-mask sharpening.


* HDR local-contrast enhancement.


* Contrast stretching at gentle, moderate and full levels.


* Star removal and black-point adjustment in the current processing workflow.



The processing stages and their settings are evolving during development. Consult `skystacker --help` for the exact options available in the version you have built.

## Supported input formats



SkyStacker currently discovers these extensions:

* `.jpg` and `.jpeg`


* `.cr2` and `.cr3`


* `.fit` and `.fits`



JPEG decoding is handled by Go's image packages. RAW and FITS decoding uses external command-line tools, so support depends on the relevant decoder being installed and available in `PATH`.

### External tools



On Debian/Ubuntu, install ImageMagick:

```bash
sudo apt update
sudo apt install imagemagick
```[cite: 2]

For Canon RAW files, SkyStacker can use `dcraw` or `dcraw_emu` when available[cite: 2]. Install a compatible RAW decoder if your system does not already provide one[cite: 2].

> **Note:** The exact RAW/FITS decoding capabilities depend on the ImageMagick delegates and RAW decoder installed on your system[cite: 2]. A file extension alone does not guarantee that a particular camera file can be decoded[cite: 2].

## Requirements[cite: 2]

- Go 1.20 or later[cite: 2].
- ImageMagick for the external RAW/FITS decoding paths[cite: 2].
- `dcraw` or `dcraw_emu` for the preferred Canon RAW decoding path, where required[cite: 2].
- A directory containing compatible astrophotography exposures[cite: 2].
- Enough RAM and disk space for the image dimensions and number of frames in the session[cite: 2].

SkyStacker includes memory-safety checks intended to warn about potentially excessive memory requirements[cite: 2]. Avoid bypassing those checks unless you understand the resource implications[cite: 2].

## Build[cite: 2]

Clone the repository and build the executable[cite: 2]:

```bash
git clone https://github.com/computerphysicslab/skystacker.git
cd skystacker

go build -o skystacker main.go
```[cite: 2]

Run the internal self-tests[cite: 2]:

```bash
./skystacker --selftest
```[cite: 2]

Print the version or command-line help[cite: 2]:

```bash
./skystacker --version
./skystacker --help
```[cite: 2]

## Quick start[cite: 2]

Put the light frames from one imaging session in a directory, for example[cite: 2]:

```text
M31-session/
├── light_001.jpg
├── light_002.jpg
├── light_003.jpg
└── ...
```[cite: 2]

Run the default workflow[cite: 2]:

```bash
./skystacker ./M31-session
```[cite: 2]

By default, the results are written to[cite: 2]:

```text
M31-session/skystacker_results/
```[cite: 2]

Choose a different stacking method[cite: 2]:

```bash
./skystacker --method mean ./M31-session
./skystacker --method median ./M31-session
./skystacker --method sigma ./M31-session
```[cite: 2]

Use a separate output directory[cite: 2]:

```bash
./skystacker --output ./results/M31 ./M31-session
```[cite: 2]

Search subdirectories recursively[cite: 2]:

```bash
./skystacker --recursive ./M31-session
```[cite: 2]

Run stacking without post-processing[cite: 2]:

```bash
./skystacker --stack-only ./M31-session
```[cite: 2]

Run post-processing against existing intermediate data[cite: 2]:

```bash
./skystacker --post-only ./M31-session
```[cite: 2]

Allow replacement of existing output files[cite: 2]:

```bash
./skystacker --overwrite ./M31-session
```[cite: 2]

## Calibration example[cite: 2]

A session may keep its light frames and calibration frames in separate directories[cite: 2]:

```text
M31-session/
├── lights/
│   ├── light_001.cr2
│   ├── light_002.cr2
│   └── ...
└── calibration/
    ├── darks/
    ├── flats/
    └── bias/
```[cite: 2]

Supply calibration files or directories explicitly[cite: 2]:

```bash
./skystacker \
  --dark ./M31-session/calibration/darks/master-dark.fit \
  --flat ./M31-session/calibration/flats/master-flat.fit \
  --bias ./M31-session/calibration/bias/master-bias.fit \
  ./M31-session/lights
```[cite: 2]

Or ask SkyStacker to search a calibration root[cite: 2]:

```bash
./skystacker \
  --calib-root ./M31-session/calibration \
  ./M31-session/lights
```[cite: 2]

Use calibration frames with matching camera, sensor settings, exposure/gain and relevant acquisition conditions[cite: 2]. Calibration is only as reliable as the calibration data supplied[cite: 2].

## Command-line options[cite: 2]

The command syntax is[cite: 2]:

```bash
skystacker [flags] [input-directory]
```[cite: 2]

The input directory defaults to the current directory (`.`)[cite: 2]. Flags can be supplied before or after the positional input directory[cite: 2].

| Flag | Description | Default |[cite: 2]
| --- | --- | --- | --- |
| `--output string` | Output directory | `<input>/skystacker_results` |[cite: 2]
| `--method string` | Stacking method: `mean`, `median` or `sigma` | `sigma` |[cite: 2]
| `--workers int` | Maximum number of concurrent workers | Number of CPUs |[cite: 2]
| `--recursive` | Search input subdirectories recursively | `false` |[cite: 2]
| `--overwrite` | Permit replacing files in the output directory | `false` |[cite: 2]
| `--no-register` | Disable star-based registration | `false` |[cite: 2]
| `--dark string` | Dark frame or calibration path | — |[cite: 2]
| `--flat string` | Flat frame or calibration path | — |[cite: 2]
| `--bias string` | Bias frame or calibration path | — |[cite: 2]
| `--calib-root string` | Root directory containing Bias/Darks/Flats folders | — |[cite: 2]
| `--stack-only` | Stack frames and skip post-processing | `false` |[cite: 2]
| `--no-postprocess` | Alias for `--stack-only` | `false` |[cite: 2]
| `--post-only` | Run post-processing using existing intermediate data | `false` |[cite: 2]
| `--mask-bg` | Mask bright objects during background extraction | `false` |[cite: 2]
| `--bg-degree int` | Background polynomial degree (1–3) | `2` |[cite: 2]
| `--stretch float` | Asinh stretch factor | `10` |[cite: 2]
| `--sharpen-amount float` | Unsharp-mask amount | `0.8` |[cite: 2]
| `--sharpen-radius float` | Unsharp-mask radius in pixels | `1.5` |[cite: 2]
| `--hdr-amount float` | HDR local-contrast amount | `0.5` |[cite: 2]
| `--star-radius int` | Star-removal radius in pixels; `0` selects automatic sizing | `0` |[cite: 2]
| `--star-sigma float` | Star-detection threshold in sigma units | `4` |[cite: 2]
| `--bg-percentile float` | Luminance percentile used for black-point adjustment; `0` disables it | `10` |[cite: 2]
| `--scale int` | Downscaling factor; `2` halves each dimension, `4` quarters it | `1` |[cite: 2]
| `--force` | Skip the out-of-memory safety check (not recommended) | `false` |[cite: 2]
| `--tiff` | Write the optional TIFF sum output | `false` |[cite: 2]
| `--post-ca` | Enable chromatic-aberration reduction | `false` |[cite: 2]
| `--post-color` | Enable colour calibration | `false` |[cite: 2]
| `--post-flatten` | Enable vignette/background flattening | `false` |[cite: 2]
| `--post-sharpen` | Enable unsharp-mask sharpening | `false` |[cite: 2]
| `--post-hdr` | Enable HDR local contrast | `false` |[cite: 2]
| `--post-stretch` | Enable the three-level contrast stretch | `false` |[cite: 2]
| `--no-color-calib` | Skip colour calibration | `false` |[cite: 2]
| `--no-sharpen` | Skip sharpening | `false` |[cite: 2]
| `--selftest` | Run internal self-tests and exit | — |[cite: 2]
| `--version` | Print version and exit | — |[cite: 2]
| `--help` | Print command-line help | — |[cite: 2]

For the authoritative list and defaults in your checked-out build, run[cite: 2]:

```bash
./skystacker --help
```[cite: 2]

## Output[cite: 2]

SkyStacker creates a `skystacker_results/` directory by default (or the directory selected with `--output`)[cite: 2].

Depending on the selected workflow and enabled options, it can contain[cite: 2]:

- Registered and unregistered stack products[cite: 2].
- Maximum- and minimum-pixel diagnostic stacks[cite: 2].
- TIFF data products when requested[cite: 2].
- JPEG previews from post-processing stages[cite: 2].
- Intermediate working data used by the post-processing pass[cite: 2].
- `report.txt`, containing processing information and per-frame outcomes[cite: 2].

The precise set of files depends on the version, flags and input data[cite: 2]. The report is the best place to check which frames were loaded, registered, skipped or processed[cite: 2].

## Processing pipeline at a glance[cite: 2]

```text
Astrophotography session
        │
        ▼
 Discover JPG / JPEG / CR2 / CR3 / FIT / FITS
        │
        ▼
 Decode frames and estimate resource requirements
        │
        ▼
 Optional dark / flat / bias calibration
        │
        ▼
 Detect stars and choose a reference frame
        │
        ▼
 Match star patterns and register frames
        │
        ▼
 Resample, align and crop valid image area
        │
        ▼
 Mean / Median / Sigma stacking
        │
        ├── Maximum-pixel stack
        ├── Minimum-pixel stack
        └── Optional TIFF output
        │
        ▼
 Background and optional image processing
        │
        ▼
 JPEG previews + image products + report
```[cite: 2]

## Development status[cite: 2]

SkyStacker is under active development[cite: 2]. The repository currently uses a single-file implementation in `main.go`; the version reported by the current source is **0.4.1**[cite: 2].

The feature set, output names and command-line interface may change between development revisions[cite: 2]. Treat the source code and `--help` output from the checked-out commit as authoritative[cite: 2].

Potential future improvements include[cite: 2]:

- Better automatic routing of different input formats through format-specific pipelines[cite: 2].
- Weighted stacking based on image quality metrics such as star count or FWHM[cite: 2].
- Deconvolution[cite: 2].
- Further calibration and registration improvements[cite: 2].
- A more modular architecture and expanded automated test coverage[cite: 2].

---

## Planned Potential Features and Technologies Roadmap

To evolve SkyStacker into a more complete scientific astrophotography preprocessing and image-processing platform, the following features and technologies are planned for future integration:

### 1. Advanced Stacking & Image Integration
* **Frame quality weighting**: Combine SNR, FWHM, eccentricity, and background measurements ($Q_i = w_1 \text{SNR}_i - w_2 \text{FWHM}_i - w_3 E_i - w_4 B_i$) to give sharper, cleaner frames more influence.
* **Local Normalization Correction (LNC)**: Reduce differences in background and illumination between individual frames.
* **Drizzle & Bayer Drizzle Integration**: Recover better sampling from undersampled data and improve colour reconstruction without conventional demosaicing first.
* **Adaptive Rejection & Maps**: Iterative sigma-clipping, MAD, asymmetric thresholds, and generation of per-pixel rejection and weight maps.
* **Per-Frame Normalization**: Additive, multiplicative, and scaling normalization to handle sky brightness and transparency variations.

### 2. Advanced Star Detection & Optical Modelling
* **PSF Modelling & HFR**: Gaussian and Moffat profiles, Half Flux Radius (HFR) measurement, and elliptical Gaussian fitting for tracking errors, tilt, or coma.
* **Astrometric Plate Solving & WCS**: Automatic celestial coordinate identification using Astrometry.net and local Gaia catalogues, storing WCS metadata and SIP polynomial distortion coefficients.

### 3. Advanced Photometric & Colour Calibration
* **Photometric & Spectrophotometric Colour Calibration (PCC / SPCC)**: Use stellar spectral information from Gaia DR3 combined with camera sensor response and filter transmission characteristics.
* **Narrowband & Advanced Calibration**: Support for monochrome H-alpha, OIII, and SII filters with custom mappings (HOO, SHO, LRGB), dark-frame scaling, Bad Pixel Maps (BPM), and separate RGB channel normalization.

### 4. Advanced Background Extraction & Post-Processing
* **Multiscale Background Modelling**: Sample-based extraction, robust polynomial fitting protecting extended astronomical objects, and multiscale wavelet processing.
* **PSF-Based Deconvolution**: Richardson-Lucy, Wiener filtering, and regularized deconvolution to recover fine details lost to atmospheric seeing.
* **Star Separation**: Generate starless images and independent star layers for isolated nebular processing.
* **Advanced Noise Reduction**: Multiscale noise estimation and spatially adaptive denoising.

### 5. New Astrophotography Processing Modes
* **Multi-Night Integration**: Intelligent session classifiers, cross-session normalization, and calibration matching across nights.
* **Mosaic Construction**: Multi-panel alignment, overlap-based photometric normalization, feathered blending, and multiband seam elimination.
* **Comet & Moving-Object Stacking**: Dual registration preserving both star-aligned and comet-aligned structures.

### 6. High-Performance Computing & GPU Acceleration
* **Hardware Abstraction Layer**: Optional CUDA, OpenCL, and Vulkan Compute support alongside optimized SIMD/AVX2/AVX-512 CPU pixel operations and memory-mapped tiles.

### 7. Artificial Intelligence & Machine Learning (Optional Modules)
* **Optional AI Tools**: CNN/U-Net AI denoising, automated star removal, anomaly-based defect detection, and ML regression frame selection.

### 8. Scientific Interoperability & Interchange Formats
* **Expanded Formats**: Full support for XISF, FITS with compression, SER video sequences, and RAW/DNG ingestion while preserving acquisition metadata and calibration provenance.

### 9. Automation, Observability & Architecture
* **Robust Workflow Control**: Processing cache, checkpoint and resume capabilities, YAML/JSON pipeline configuration files, structured JSON telemetry reports, a modular plugin architecture, and an optional REST API.

---

## Project goals[cite: 2]

- **Local-first:** process astrophotography data on the user's own machine[cite: 2].
- **Transparent:** make processing steps and per-frame outcomes inspectable[cite: 2].
- **Resource-conscious:** estimate memory needs and provide safeguards[cite: 2].
- **Numerically careful:** use floating-point image data for accumulation and processing[cite: 2].
- **Practical:** provide a straightforward CLI and useful intermediate outputs[cite: 2].

## License[cite: 2]

SkyStacker is distributed under the [MIT License](LICENSE)[cite: 2].

## Keywords[cite: 2]

SkyStacker
Astrophotography
Image Stacking
Deep Sky Imaging
Astronomy
GoLang
Go CLI
Command Line Tool
TIFF
JPEG
JPG
HDR
High Dynamic Range
Frame Discovery
Frame Accumulation
Image Integration
Pixel Processing
Float32
Numerical Precision
Offline Processing
Local First
Batch Processing
Image Pipeline
Exposure Stacking
Light Frames
Star Fields
Night Sky
Telescope Imaging
CCD Imaging
CMOS Imaging
Astro Processing
Scientific Imaging
Signal Enhancement
Noise Reduction
Mean Stack
Sum Stack
Arithmetic Mean
Pixel-wise Sum
Image Averaging
Integrated Exposure
Pre-aligned Frames
Registration
Calibration
Image Alignment
Raw Data Processing
FITS Alternative
TIFF Export
JPEG Preview
Image Preview
Processing Report
Reproducibility
Memory Efficient
Streaming Processing
Tile Processing
Cross Compilation
Static Binary
Lightweight Software
Open Source
GitHub
Automation
Workflow Automation
Session Processing
Astrophotography Workflow
Observatory Data
Star Tracking
Long Exposure
Exposure Integration
Sky Imaging
Deep Space Objects
Nebula Imaging
Galaxy Imaging
Star Cluster Imaging
Image Analysis
Pixel Accumulation
Floating Point Output
Scientific Computing
Image Reconstruction
Performance Optimization
Concurrency
Workers
Parallel Processing
File Discovery
Directory Scanning
Metadata Reporting
Processing Statistics
Astro Software
CLI Application
Image Enhancement
Local Processing
Frame Aggregation
Data Integrity
Astronomical Photography
Computational Photography
Image Composition
Exposure Combination
Image Rendering
Astroinformatics
Image Dataset
MVP Software
Astrophotography Automation
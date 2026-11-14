# Cross-Platform CLI Integration & Observatory Automation Architecture

**Scope:** Headless execution topologies, 7-stage decoupled pipeline state transitions, multi-platform binary deployment, ASTAP CLI interoperability, return-code encodings, and `skystacker_results/` output schemas.

*(Note: Core mathematical formulations for calibration, quad hashing, stacking algorithms, OSC demosaicing, optical diagnostics, dual-stack comet recombination, and ensemble photometry are documented in [`core-engine-technical-specification.md`](core-engine-technical-specification.md).)*

---

## 1. Headless Observatory Pipeline Topology

In modern automated observatories and high-throughput astronomical imaging facilities, contemporary CMOS and CCD detectors regularly generate 60+ megapixel sub-exposures per frame, scaling raw session datasets to hundreds of gigabytes. Relying on GUI execution loops or cloud-based microservice APIs introduces unacceptable bandwidth latency, network overhead, and dynamic resource consumption.

Executing statically compiled, multi-threaded Go 1.20+ binaries directly on local edge hardware or observatory control nodes saturates available CPU cores while maintaining deterministic memory bounds via spatial tile streaming ($1024 \times 1024$ `float32` tiles = $4\text{ MB}$ per tile) and `sync.Pool` buffer recycling.

```text
[ Input Raw Dataset: FITS, FITS.FZ, RAW, TIFF, XISF ]
   │
   ▼
1. FILE DISCOVERY & INDEXING   ──► [ Extract EXIF/FITS Headers & Validate Catalog ]
   │
   ▼
2. CALIBRATION PREPROCESSING   ──► [ Apply Master Dark, Master Bias & Master Flat ]
   │
   ▼
3. ALIGNMENT & WCS CHECK       ──► [ Quad Hash Matching (0.007 Tol) & Matrix Offsets ]
   │
   ▼
4. PARALLEL TILE ACCUMULATOR   ──► [ Partition Spatial Planes (1024x1024) into float32 Buffers ]
   │
   ▼
5. OUTLIER REJECTION           ──► [ Temporal Filtering (Sigma-Clipping / Winsorized) ]
   │
   ▼
6. SCIENTIFIC ENCODING         ──► [ IEEE 754 32-bit Float Output & Asinh/MTF Render ]
   │
   ▼
7. TELEMETRY AUDIT REPORT      ──► [ Export Machine-Readable INI / JSON Metadata ]
```

### Stage-by-Stage Memory State & Artifact Matrix

| Pipeline Stage | Input Artifacts | Processing Memory State | Generated Output Artifacts |
| :--- | :--- | :--- | :--- |
| **1. File Discovery & Indexing** | Directory path, FITS/RAW/TIFF files | RAM (Metadata Index Table) | `catalog.json` / Validated file list |
| **2. Calibration Preprocessing** | Light sub-frames, Master Dark, Master Flat | RAM (In-Memory `float32` Buffers) | Calibrated light frame pixel streams |
| **3. Alignment & Verification** | Calibrated frame streams, Reference frame | RAM (Quad matrices & WCS parameters) | Registered coordinate transformation matrices |
| **4. Spatial Tile Accumulation** | Aligned pixel streams, Tile bounds ($1024\times1024$) | RAM (Concurrent worker pool `sync.Pool`) | Partitioned 32-bit float tile matrices |
| **5. Outlier Rejection & Stack** | Partitioned float tile matrices | RAM (Temporal array allocation) | Filtered, integrated single master tile array |
| **6. Encoding & Rendering** | Integrated master tile array | RAM to Disk (I/O Stream) | 32-bit IEEE 754 Float FITS/TIFF, stretched preview |
| **7. Telemetry & Reporting** | System state, rejection counts, HFD metrics | RAM to Disk | `skystacker_results/` JSON audit log & `.ini` file |

---

## 2. Multi-Platform Binary Deployment & Runtime Dependencies

### 2.1 Operating System Architectures
* **Windows System Deployment:** Supported via 64-bit (`astap_setup.exe`), 32-bit (`astapwin32.zip`), and native Windows 11 ARM64 execution binaries (`astap_cli.exe`). Native ARM64 binaries deliver up to **375% performance improvements** over x86 emulation on ARM architectures.
* **Linux Environments:** Packaged for `x86_64`, `aarch64` (ARM64), and `armhf` (ARM32) supporting GTK2 and GTK3 widget models across `.deb` (Debian/Ubuntu/Raspbian), `.rpm` (OpenSUSE/Fedora), `.pkg.tar.zst` (Arch/Manjaro), and standalone `.tar.gz` archives.
  * Default system installation path: `/opt/astap/`
  * Secondary star database path: `/usr/share/astap/data/`
* **macOS Platforms:** Provided as native PKG installers for Intel 64-bit and Apple Silicon (M1/M2/M3) architectures. System star databases install to `/usr/local/opt/astap/`.
  * **Mandatory Code-Signing (macOS Sequoia 15.1+):** Modern macOS security frameworks enforce code-signing for headless CLI execution:
    ```bash
    codesign --force -s - /Applications/ASTAP.app/Contents/MacOS/astap
    ```
* **Android OS Architectures:** Executable via compiled `astap_cli` command-line tools for `aarch64`, `armhf`, `x86_64`, and `x86`, integrated within mobile observatory control suites such as **OpenLiveStacker** and **Stellar Finder**.

### 2.2 Supported Input Extensions & External Tools
* **Input File Format Support:** FITS (`.fits`, `.fit`), Rice-compressed FITS (`.fits.fz`), Canon RAW (`.cr2`, `.cr3`), standard image formats (`.jpg`, `.png`), Tagged Image Format (`.tiff` / ASTRO-TIFF), PixInsight XISF uncompressed (`.xisf`), and portable graymaps (`.pgm`, `.ppm`).
* **Runtime & External Dependencies:**
  * Built upon Go 1.20+ concurrency engines.
  * RAW debayering utilizes `unprocessed_raw` (a modified LibRaw binary extracting active sensor areas), David Coffin's `dcraw` utility, ImageMagick, and OpenSSL 1.1 runtime libraries (`libssl-dev` / `openssl_libraries.pkg`) for online catalog querying.
  * **StarNet2** is supported as an external module for star/comet isolation, accelerated via GPU architectures using `DirectML.dll` runtime dependencies.

---

## 3. Analytical Error-Level & Diagnostic Return Encoding

When invoked in automated headless environments, diagnostic CLI execution flags encode analytical returns directly into system process exit codes (`%errorlevel%` on Windows) or standard output streams (`stdout` on Linux/macOS):

### 3.1 `-analyse` Execution Return Encoding
* **Windows OS (`%errorlevel%`):** Encodes median HFD and star detection count into a single integer return code:
  $$\text{\%errorlevel\%} = \text{integer}(\text{median HFD} \times 1,000,000) + \text{stars detected}$$
  Calling automation scripts decode median HFD using integer arithmetic:
  $$\text{median HFD} = \frac{\text{trunc}(\text{\%errorlevel\%} / 1,000,000)}{100}$$
* **Linux & macOS (`stdout`):**
  ```text
  HFD_MEDIAN=3.3 STARS=666
  ```

### 3.2 `-focus1..N` Focuser Calibration Encoding
* **Windows OS (`%errorlevel%`):** Evaluates hyperbola curve fitting across focus positions:
  $$\text{\%errorlevel\%} = \text{integer}(\text{focuspos} \times 10,000) + \text{integer}(\text{rem.error} \times 1,000)$$
* **Linux & macOS (`stdout`):** Emits curve fit coefficients directly to standard output.

---

## 4. ASTAP Interoperability CLI Flag Reference

For headless plate-solving, batch photometry, and diagnostic analysis adapters:

| CLI Command Flag | Argument Syntax | Technical Description | Operational Context |
| :--- | :--- | :--- | :--- |
| `-f` | `<filepath>` | Specifies target input image file path for solving or processing. | Standard solving / analysis input |
| `-r` | `<degrees>` | Sets search radius in degrees around initial start coordinates. | Spiral search solve limit |
| `-fov` | `<degrees>` | Forces explicit image height FOV in degrees ($0 = \text{Auto}$). | Spatial scale override |
| `-ra` | `<hours>` | Specifies right ascension center hint (decimal hours or `HH:MM:SS`). | Non-FITS solve coordinate input |
| `-spd` | `<degrees>` | Specifies South Pole Distance ($\text{Dec} + 90^\circ$, always positive). | Non-FITS solve coordinate input |
| `-z` | `<0..4>` | Sets image downsampling factor ($0 = \text{Auto Binning}$). | Speed/SNR optimization |
| `-s` | `<count>` | Sets maximum star detection limit for quad formation (default: `500`). | Registration/solve performance |
| `-t` | `<tolerance>` | Sets quad matching hash tolerance (default: `0.007`). | Optical distortion tolerance |
| `-m` | `<min_hfd>` | Filters out star detections with HFD below specified threshold. | Hot pixel suppression |
| `-check` | `y / n` | Toggles Bayer pattern check filter prior to solving raw OSC files. | Raw OSC debayer check |
| `-d` | `<path>` | Sets explicit file system search directory path to star database. | Database path override |
| `-D` | `<code_name>` | Selects active catalog database (`d80`, `d50`, `d20`, `d05`, `v50`, `w08`). | Star database selection |
| `-o` | `<filepath>` | Overrides default output base path and filename prefix. | Pipeline output routing |
| `-sip` | `y / n` | Enables addition of 3rd-order Simple Image Polynomial coefficients. | Wide-field distortion modeling |
| `-speed` | `slow / auto` | Toggles slow search mode, forcing 50% search field spatial overlap. | Low star count solve fallback |
| `-wcs` | *(None)* | Generates standalone `.wcs` header file with fixed 80-char block formatting. | Astrometry.net compatibility |
| `-update` | *(None)* | Writes solved WCS coordinates directly into source file header. | In-place FITS header update |
| `-log` | *(None)* | Forces execution telemetry logging to a `.log` text file. | Pipeline log generation |
| `-analyse` | *(None)* | Executes diagnostic inspection; reports median HFD and star count. | Fast quality control check |
| `-extract` | `<min_snr>` | Exports detected star centroids, flux, and HFD metrics to CSV. | Photometric/star extraction |
| `-extract2` | `<min_snr>` | Exports star centroids and solved RA/Dec coordinates to CSV (with SIP). | Astrometric catalog extraction |
| `-sqm` | *(None)* | Calculates sky background brightness in $\text{mag/arcsec}^2$. | Sky quality evaluation |
| `-focus1..N` | *(None)* | Evaluates focus sequence using hyperbola curve fitting. | Automated focuser calibration |
| `-stack` | *(None)* | Launches live directory monitoring and automated stacking routines. | Real-time live stacking mode |
| `-p` | *(None)* | Executes batch photometry pipeline using local `.ini` configuration. | Automated light-curve generation |

---

## 5. Output Folder Schema Specification (`skystacker_results/`)

Executing processing tasks via CLI generates output artifacts structured under the designated results path:

```text
skystacker_results/
├── master_stack_integration.fits  # Integrated 32-bit IEEE 754 Float Master Stack
├── master_stack_preview.tiff       # 32-bit Float TIFF (SampleFormat=3) / Preview
├── stacked_processed.jpg           # 8-bit stretched visual RGB preview image
├── stacked_max.jpg                 # Diagnostic Maximum-Pixel composite visual image
├── stacked_min.jpg                 # Diagnostic Minimum-Pixel composite visual image
├── target_observation.wcs          # Fixed 80-character line WCS Header Block
├── execution_parameters.ini        # Machine-readable Execution State INI File
└── telemetry_audit.log             # JSON Audit Export & Log Trace Data
```

### Artifact Specifications

1. **Integrated Master Stack File (`.fits` / `.tiff`):** Encoded as standard 32-bit floating-point IEEE 754 FITS or 32-bit float TIFF (`SampleFormat=3`), preserving unclipped negative sub-bias pixel values and broad dynamic ranges.
2. **WCS Header File (`.wcs`):** Standalone text block composed of standard FITS 80-character header records without carriage returns, containing complete celestial plate solutions compatible with Astrometry.net workflows.
3. **Execution State INI File (`execution_parameters.ini`):** Formatted control block exposing system variables:
   ```ini
   PLTSOLVD=T
   CRPIX1=1.1645000000000000E+003
   CRPIX2=8.8050000000000000E+002
   CRVAL1=1.5463033992314939E+002
   CRVAL2=2.2039358425145043E+001
   CDELT1=-7.4798001762187193E-004
   CDELT2=7.4845252983311850E-004
   CROTA1=-1.1668387329628058E+000
   CROTA2=-1.1900321176194073E+000
   CD1_1=-7.4781868711882519E-004
   CD1_2=1.5241315209850368E-005
   CD2_1=1.5534412042060001E-005
   CD2_2=7.4829732842251226E-004
   CMDLINE=astap -f target.fits -r 30 -update
   WARNING=None
   ERROR=None
   ```
4. **Telemetry Audit Log (`telemetry_audit.log` / JSON Export):** Exports execution log data containing rejected pixel percentages, HFD median values, runtime frame counts, and standardized system exit/error codes:
   * `0` = Success
   * `1` = Solve Failure
   * `2` = Low Stars
   * `16` = File Read Error
   * `32` = Database Missing
   * `33` = Database Read Error
   * `34` = Header Update Failure

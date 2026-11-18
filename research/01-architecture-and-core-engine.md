# SkyStacker — Architecture & Core Engine (Canonical Consolidation)

> **Canonical status:** This is the single authoritative research file for SkyStacker's architecture, core engine, CLI/ASTAP interoperability, memory and rejection models, stretch math, benchmark methodology, formats, engineering survey, consolidated roadmap and status. It merges, deduplicates and reconciles seven source documents; each fact is stated once. Historical snapshots are explicitly tagged **[HISTORICAL — v0.4.1]**, **[HISTORICAL — v0.6.0]** or **[SURVEY PROPOSAL, NOT IMPLEMENTED]**. **Current production state: v0.6.1, 247 registered self-tests, Go 1.27.4.**
>
> **Scope boundaries — do not duplicate here.** Astrometry/catalogue/plate-solving detail → [02](02-astrometry-and-identification.md); glossary and bibliography → [03](03-reference-and-literature.md); calibration, master frames and grid self-flat → [04](04-calibration-and-self-flat.md); Go toolchain upgrade analysis and modularization → [05](05-toolchain-and-modularity.md); external AI sidecar design → [06](06-external-ai-sidecars.md); native pure-Go CR2 decoding → [07](07-native-cr2-decoding.md); crawler and source-discovery runs → [08](08-crawler-source-discovery.md); project overview and flag reference → [README.md](README.md).

---

## 0. What SkyStacker is

SkyStacker is an open-source, high-performance, **local-first command-line tool written in Go** for processing, aligning, calibrating and integrating deep-sky astrophotography sequences. Modern CMOS and CCD sensors produce ultra-high-resolution sub-exposures (frequently **>60 MP at 16 bits/sample**); sessions yield hundreds of sub-exposures and raw datasets of **tens to hundreds of gigabytes**. SkyStacker removes network-transfer bottlenecks, GUI latency, dynamic heap thrashing and third-party cloud dependencies by running natively on local observatory edge hardware or user workstations. It provides an automated, reproducible, resource-conscious and mathematically rigorous workflow from raw sub-exposures to scientific masters and post-processed visualizations. **[HISTORICAL v0.6.1 architecture baseline]** the shipped binary is a static `CGO_ENABLED=0` build; as of the current state it is a single stdlib-only executable plus one first-party module (`goPureRaw`) — see [05](05-toolchain-and-modularity.md) and [07](07-native-cr2-decoding.md).

Guiding principles: local-first (no cloud), resource-conscious (pre-flight memory/disk estimation), transparent (per-frame diagnostics in `report.txt`, intermediate previews, rejection/weight maps), numerically careful (`float32` end-to-end, atomic file writes), concurrent (worker pools, `context.Context` cancellation), static-binary-friendly (`CGO_ENABLED=0` cross-platform distribution).

---

## 1. Version lineage and contradictions resolved

The source documents span several snapshots. The following table resolves every contradiction explicitly; the right-hand column is authoritative.

| Topic | Conflicting statements in sources | Resolution (authoritative) |
| :--- | :--- | :--- |
| **Product version** | v0.4.1 snapshot; "0.4.1 moving toward 0.5.0" (synthesis, 2026-11-11); "0.6.0 / 181 self-tests"; "0.6.1 / 203 self-tests" (Gemini prompt); "v0.6.1 exists but main advanced beyond it"; README "0.6.1 / 208" | **Current = v0.6.1, 247 self-tests.** All others are historical snapshots, tagged in place. |
| **Self-test count** | 177 (v0.6.0 transfer, 2026-11-15) → 181 (deep-dive §1) → 182 (v0.6.1 bump) → 203 (Gemini prompt) → 208 (README/appendix) → 209 (goPureRaw modularization / Go upgrade) → 210 (fused sigma-clip) → 211 (sidecars) → 212 (`--resample-sip`) → 219 (roadmap items 2-8/11) → 220 (goDiagnostics) → 229 (items 19-28) | **247 registered self-tests** today; each earlier number belongs to the tagged snapshot that reported it. |
| **Go version** | Go 1.20+ / 1.20.3 baseline (CLI doc, synthesis, Gemini prompt, benchmark rows); native Swiss Tables require Go 1.24+ (core spec, deep dive); proposal "upgrade to Go 1.26" (Gemini survey); interim **go1.26.11** adopted; **go1.27.4** current | Historical baseline **1.20.3** → survey proposal **1.26** → interim **1.26.11** → **CURRENT Go 1.27.4** (`go 1.27` + `toolchain go1.27.4` in both modules, CI pinned). |
| **Output directory default** | Core spec CLI table: `<input>/skystacker_results`; v0.4.1 snapshot: `<input>/0_skystacker_results`; CLI integration folder schema: `skystacker_results/` | Design/current documented default = **`<input>/skystacker_results`**; **`0_skystacker_results`** is the v0.4.1-era literal (still seen in local artifacts). Both recorded; `--output` overrides either. |
| **Swiss Tables library** | Core spec: "integrated natively into Go 1.24+ and implemented via `dolthub/swiss`"; deep dive: `dolthub/swiss` is archived | **`dolthub/swiss` is deprecated/archived — do not depend on it.** Use native Go 1.24+ `internal/runtime/maps`. Group-width folklore (16 in C++ designs vs Go's 8) is not portable; measure with `pprof`. |
| **Resampling kernel** | Core spec step 6 / §5.3: "high-order Lanczos/Sinc interpolation"; v0.4.1 snapshot and synthesis: **bilinear** resampling | **Implemented = bilinear** (sub-pixel, into reference frame, safe crop). Lanczos/Sinc is an aspirational design statement; Seti Astro's anti-ringing Lanczos warp is an external reference only. |
| **Post-processing stage count** | Core spec: "automated **6-stage** engine"; v0.4.1 snapshot and README: **8-step** pipeline | The **6-stage** list is a conceptual design grouping; the **8-step** order is what v0.4.1 shipped and what the current README documents (background → star removal → black-point → STF stretch → guided+chroma denoise → curves → HDR blend/weighted mixes → diffraction spikes). |
| **SIP order** | Core spec §5.4: 3rd-order SIP ($A_{p,q},B_{p,q}$, $p+q\le3$); current code: **order-2** fit | Design allows higher order; **implemented fit is order-2**, stored in `plate_solution.json`; now applied in the resampler via `--resample-sip` under a shared-distortion assumption. |
| **SIP in resampler** | "SIP fitted/stored but not applied" (v0.6.0-era notes); "SIP applied in WCS helpers but not stacking resampler" (Gemini prompt) | **Implemented 2026-11-17:** `--resample-sip` loads stored order-2 SIP, undistorts reference pixels, applies similarity/affine transform in undistorted space, redistorts source pixels before bilinear sampling. Per-frame astrometric reprojection and drizzle remain open. |
| **Source layout** | "single-file `main.go`" everywhere (historical); current: modularized | Historically one file; **current = single stdlib-only executable plus one first-party module** (`goPureRaw v0.1.1`); see [05](05-toolchain-and-modularity.md), [07](07-native-cr2-decoding.md). |
| **ASTAP `-analyse` encoding** | CLI doc draft: `integer(median HFD × 1,000,000) + stars`, decode `trunc(%errorlevel%/1e6)/100` (wrong by ×100); deep-dive §9.1: corrected formula | **Corrected:** `errorlevel = round(median_HFD × 100) × 1_000_000 + stars_detected`; `median_HFD = trunc(errorlevel / 1_000_000) / 100`. Example HFD 3.26 / 666 stars → **326000666** (the draft formula gives 3,260,666 and decodes to 0.03). |
| **ASTAP `-focus`** | CLI doc: hyperbola fit encoded in `%errorlevel%`; deep dive: fit is GUI/`-debug` path only | Do **not** assume `astap_cli` returns a hyperbolic-fit code; implement focusing natively or use the GUI debug path. |
| **`-ra` units** | CLI doc: decimal hours or `HH:MM:SS` | Verify against installed CLI version — drafts note a hours-vs-degrees inconsistency risk. |
| **ARM64 375% gain** | CLI doc states "up to 375% performance improvements" over emulation | Developer-reported, not an independent benchmark. |
| **Drizzle** | Explicit non-goal; `--drizzle-upsample` diagnostic shipped; survey proposes `--bayer-drizzle` | True multi-frame drizzle and Bayer/CFA drizzle **remain open**; the shipped flag is a diagnostic upsample of the final stack only. |

---

## 2. Core execution workflows

Three workflows are documented and must be preserved: the **SkyStacker 9-Step Automated Pipeline**, the **ASTAP 5-Step In-Memory Stacking Workflow**, and the **7-Stage Headless Observatory Pipeline** (Section 13).

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
  6. Subpixel Resampling & Crop
  7. Statistical Pixel Combination
  8. Non-Linear Post-Processing
  9. Scientific Export & Telemetry
       │                                                             │
       └──────────────────────────────┬──────────────────────────────┘
                                      ▼
                      [ Master Outputs & Diagnostic Reports ]
```

### 2.1 SkyStacker 9-Step Automated Pipeline

1. **File Discovery & Header Indexing:** Recursively inspects session directories for `.fit`, `.fits`, `.fits.fz`, `.cr2`, `.cr3`, `.jpg`, `.jpeg`, `.tiff`, `.xisf`, `.pgm`, `.ppm`. EXIF/FITS headers yield exposure duration, sensor temperature, gain, Bayer pattern and timestamps.
2. **Frame Decoding & Resource Safeguards:** Decodes pixel channels into `float32` buffers; memory checks compare dataset dimensions against available RAM to prevent OOM kernel panics.
3. **Photometric Calibration:** Dark-current and bias subtraction, then division by spatial master flats.
4. **Star Detection & Reference Selection:** PSFs above local sky noise ($>5\sigma$); ranks sub-exposures by FWHM, eccentricity $e$ and SNR; optimal frame becomes global geometric reference.
5. **Spatial Frame Registration:** Scale/rotation-invariant star triangles or irregular 2D quads; affine and similarity transforms evaluated with RANSAC outlier filtering.
6. **Subpixel Resampling and Safe Cropping:** Aligned sub-exposures resampled with high-order Lanczos/Sinc kernels *(implemented: bilinear — see §1)*; bounding-box cropping isolates the overlap valid across all subframes.
7. **Statistical Pixel Integration:** Statistical rejection filters (`sigma`, `winsorized`, `mean`, `median`); concurrent diagnostic max/min intensity stacks.
8. **Non-Linear Post-Processing:** Background-gradient extraction, chromatic-aberration reduction, color calibration, Generalized Hyperbolic Stretch (GHS), local detail enhancement.
9. **Scientific Export & Audit Telemetry:** Uncompressed 32-bit IEEE 754 float masters (`.tiff`/`.fits`), 8-bit visual previews (`.jpg`), structured session audit (`report.txt` / `telemetry_audit.log`).

### 2.2 ASTAP 5-Step In-Memory Stacking Workflow

For environments optimized strictly for memory throughput without intermediate disk I/O:

1. **Master Flat Normalization:** Flat subframes averaged; combined flat-dark/bias subtracted; $2\times2$ neighborhood mean spatial normalization:
   $$\text{Master Flat} = \frac{\frac{1}{N}\sum \text{Flats} - \frac{1}{N}\sum \text{Flat-Darks}}{\bar{I}_{\text{flat}}}$$
2. **Master Dark Integration:** Darks at identical temperature and exposure combined: $\text{Master Dark} = \frac{1}{N}\sum \text{Darks}$.
3. **Pure Signal Extraction:** Master Dark subtracted in-memory per raw sub-exposure, isolating photon counts from thermal dark current and readout offset.
4. **Optical Normalization:** Dark-subtracted buffers divided by Master Flat in-memory to remove dust motes and vignetting.
5. **In-Memory Frame Combination:** Corrected subframes registered and accumulated in thread-safe spatial memory buffers with no intermediate disk writes.

---

## 3. Memory model, concurrency and tile streaming

### 3.1 Static compilation and zero-dependency binary

`CGO_ENABLED=0` produces a statically linked standalone binary with no external dynamic libraries (`libc`, `libtiff`, GTK), ensuring deployment consistency across Linux, macOS, Windows and ARM edge observatories. Layers: Go application code → tile streaming pipeline + `sync.Pool` allocators → statically compiled Go runtime (`CGO_ENABLED=0`).

### 3.2 Spatial tile streaming and the RAM formula

Materializing 100 frames at 61 MP requires **>24.4 GB**, so dynamic frame grids are split into spatial tiles (e.g. **512×512 or 1024×1024** pixels). Threads stream corresponding tiles sequentially from disk into pre-allocated buffers; memory is governed by active worker count, not dataset size or stack depth:

$$\text{RAM}_{\text{peak}} \approx N_{\text{workers}} \times (\text{Tile}_{\text{width}} \times \text{Tile}_{\text{height}} \times 4\ \text{bytes}) \times N_{\text{buffer\_slots}}$$

A 1024×1024 `float32` tile = **4 MB**; 512×512 is smaller. Calibration/alignment stages keep working sets small with this scheme.

### 3.3 Temporal rejection memory budget — the critical constraint

**Stage 5 (temporal outlier rejection) must hold $N$ frames of pixel values simultaneously.** Buffering a full spatial tile across all $N$ frames explodes memory (example sensor class: **~6.25 GiB at $N=100$**, **~18.75 GiB at $N=300$**) versus a **64×64** sub-tile scheme at **~75 MiB for $N=300$ with 16 workers**. Therefore **decouple input/alignment tile size from rejection sub-tiles** (use 64×64 or 128×128 with concurrency limits), and consider a dedicated `sync.Pool` for temporal rejection buffers. This is the decisive argument for tiled spill-file reducers on the developer's **15 GiB** host (Seti Astro Suite Pro needs **32 GiB** for full-memory stacking).

### 3.4 Heap escape analysis and `sync.Pool`

Frequent `[]float32` allocation in inner loops triggers GC thrashing and pauses; passing slice headers into `sync.Pool` causes heap escape via interface casting (`any`). SkyStacker wraps tile allocations in explicit `*TileBuffer` pointers for zero dynamic allocations in critical loops:

```go
type TileBuffer struct { Pixels []float32 }
type TilePool struct { pool sync.Pool }
func NewTilePool(tileSize int) *TilePool {
    return &TilePool{pool: sync.Pool{New: func() any {
        return &TileBuffer{Pixels: make([]float32, tileSize)}
    }}}
}
func (tp *TilePool) Get() *TileBuffer { return tp.pool.Get().(*TileBuffer) }
func (tp *TilePool) Put(tb *TileBuffer) { tp.pool.Put(tb) }
```

Requesting/returning `*TileBuffer` pointers keeps allocations at zero and memory deterministic, typically **100–300 MB**.

### 3.5 Goroutine lifecycle and cancellation

Tasks flow through Go channels: `[ Disk Read ] ──chan *TileBuffer──► [ Calibration ] ──chan *TileBuffer──► [ Stacking ]`, with `context.Context` (`context.WithCancel`) spanning all stages for cancellation and telemetry. On registration failure or file-read error the context is globally cancelled; all goroutines terminate and return active `*TileBuffer` pointers to the pool, preventing goroutine leaks and memory locks.

---

## 4. Go runtime, Swiss Tables and SIMD decisions

### 4.1 Swiss Tables design (native Go 1.24+)

Swiss Tables improve key indexing, star-matching lookup and EXIF tag caching. The design separates control metadata from payloads (**Structure of Arrays / SoA**) instead of traditional **Array of Structures / AoS**:

```text
AoS Layout:  [ Key/Val/Control 1 ][ Key/Val/Control 2 ] ...   <-- low cache density
SoA Layout:  Control Array (1 byte/slot): [Ctrl 1]...[Ctrl 64] <-- one L1 cache line
             Key/Value Data Array:        [Key/Val 1][Key/Val 2]...
```

* **Control byte array:** 1 byte per slot (status flags / hash signatures).
* **Key/value slot array:** contiguous key/value pairs.
* A single **64-byte CPU cache line holds 64 control status bytes**, maximizing L1/L3 hit rates.

**Hash splitting ($H_1/H_2$) and SIMD inspection:** a 64-bit hash splits into **$H_1$ (upper 57 bits)** selecting the bucket group and **$H_2$ (lower 7 bits)** stored as fingerprint ($0\dots127$); control values also mark empty (`-128` / `0b10000000`) and deleted (`-2` / `0b11111110`) slots. SWAR/SIMD inspects up to **16 slots concurrently**:

$$\text{matchMask} = (\text{ctrl} \oplus \text{broadcast}(H_2) - 0x0101\dots)\ \&\ \sim(\text{ctrl} \oplus \text{broadcast}(H_2))\ \&\ 0x8080\dots$$

Full key equality is checked only on $H_2$ match; this eliminates up to **99.22%** of key-mismatch checks. Tables sustain an **87.5% load factor ($7/8$)** with **30–60% lower memory** than traditional maps, and Go groups entries into directories of independent sub-tables (**up to 1024 entries each**) for bounded incremental growth.

Survey benchmark numbers for the Go runtime (Swiss Tables, Green Tea GC, allocator, I/O) and the full adoption chronology are the single responsibility of [05-toolchain-and-modularity.md](05-toolchain-and-modularity.md) §3–§6. **`dolthub/swiss` is archived — do not use it** (see §1); require **Go 1.24+** where Swiss behavior is assumed and measure real maps with `pprof`.

### 4.2 SIMD / GPU decisions

`simd/archsimd` (Go 1.26) is an **off-by-default `GOEXPERIMENT` restricted to amd64**, breaking arm64/Apple-Silicon parity — **rejected** for portable static builds; revisit only if cross-architecture SIMD reaches stable stdlib. The Hardware Abstraction Layer (HAL) design offloads pixel rejection, matrix transforms and interpolation for stacks with **$N>300$** using SIMD CPU extensions (**AVX2/AVX-512**) or optional GPU backends (**CUDA, Vulkan Compute, Apple Metal**) — this remains a design goal, not shipped behavior (Tier 3/4 in [README.md](README.md)).

---

## 5. Photometric calibration, quality scoring and stacking math

### 5.1 Calibration frames and master construction

* **Master Bias:** shortest exposure, shutter closed → uniform electronic readout offset.
* **Master Dark:** darkness, matching exposure and temperature → thermal dark current and hot pixels.
* **Master Flat:** uniform light field → pixel-to-pixel sensitivity variation, vignetting, dust shadows.

$$I_{\text{calibrated}}(x,y) = \frac{I_{\text{light}}(x,y) - I_{\text{dark}}(x,y)}{I_{\text{flat}}(x,y) - I_{\text{bias}}(x,y)} \cdot \bar{I}_{\text{flat}}$$

$\bar{I}_{\text{flat}}$ is the scalar mean of the Master Flat, normalizing output back to sensor dynamic range. **[HISTORICAL v0.4.1]** calibration applied dark subtraction then flat division (bias correction on the flat); a single master file or a folder mean-stacked into a master. Detailed calibration and grid self-flat → [04](04-calibration-and-self-flat.md).

### 5.2 Frame quality score $Q_i$

$$Q_i = w_1 \cdot \left(\frac{1}{\text{FWHM}_i}\right) + w_2 \cdot (1 - e_i) + w_3 \cdot \text{SNR}_i + w_4 \cdot \text{PSFWeight}_i$$

$\text{FWHM}_i$ = average stellar FWHM (px or arcsec); $e_i$ = average stellar eccentricity ($0\le e_i<1$, 0 = circular); $\text{SNR}_i$ = background SNR; $\text{PSFWeight}_i$ = integrated stellar signal energy relative to background noise variance; $w_1..w_4$ configurable. Frames below user thresholds are flagged/excluded. **[HISTORICAL v0.6.1]** shipped weighting is `computeFrameQuality × computeIntrinsicQuality / 100` (unregistered frames: intrinsic only), with `Q_i = w₁·SNR − w₂·FWHM − w₃·E − w₄·B` in `computeAdvancedQuality` and `--hfr-quality` opting into HFR-derived FWHM; scores appear in `report.txt` (`IntrQ`, `Weight`). Per-star flux-weighted HFR + eccentricity recorded (`testStarHFR`).

### 5.3 SNR theory and stacking algorithm efficiency table

Photon signal accumulates linearly ($N$), noise in quadrature ($\sqrt{N}$): $\text{SNR}_{\text{stack}} = \sqrt{N}\cdot\text{SNR}_{\text{sub}}$.

| Method | Formula | SNR gain factor | Outlier rejection | Complexity | Primary use |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Sum** | $I_{\text{sum}}=\sum_{i=1}^{N} I_i$ | $\sqrt{N}$ (expands dynamic range) | None (retains transients) | $\mathcal{O}(N)$ | Photometric counting, lucky imaging |
| **Mean** (`mean`) | $I_{\text{mean}}=\frac{1}{N}\sum I_i$ | $1.00\cdot\sqrt{N}$ (theoretical max) | Zero (distorted by non-Gaussian noise) | $\mathcal{O}(N)$ | Master calibration (darks/flats), clean $N<5$ |
| **Median** (`median`) | $I_{\text{median}}=\text{median}(I_i)$ | $\approx0.80\cdot\sqrt{N}$ (~20% loss) | Complete (rejects extremes) | $\mathcal{O}(N\log N)$ | Small stacks ($N\le9$), calibration masters |
| **Percentile clipping** | $I_{\text{percentile}}=\frac{1}{\|P\|}\sum_{i\in P} I_i$ | $\approx0.85\cdot\sqrt{N}$ | Fixed percentile boundaries | $\mathcal{O}(N\log N)$ | Small counts ($N\le6$) where $\sigma$ unstable |
| **Sigma clipping** (`sigma`) | $I_{\text{sigma}}=\frac{1}{\|M\|}\sum_{i\in M} I_i$ | $\approx0.95\cdot\sqrt{N}$ | Iterative $k\sigma$ rejection | $\mathcal{O}(I\cdot N\log N)$ | Standard deep-sky default ($N\ge31$) |
| **Winsorized sigma** | $I_{\text{win}}=\frac{1}{N}\sum I_i^{\text{win}}$ | $\approx0.98\cdot\sqrt{N}$ | Outliers replaced by boundary values | $\mathcal{O}(I\cdot N\log N)$ | Moderate datasets ($10\le N\le30$) |
| **Linear fit clipping** | Fits $y=ax+b$ along stack axis | High on long series | Rejects deviations from linear sky trend | $\mathcal{O}(N)$ | Wide-field, varying gradients |
| **GESDT clipping** | Generalized Extreme Studentized Deviate | High | Statistical multi-outlier hypothesis test | $\mathcal{O}(N\log N)$ | 31–300 subs, high outlier density |

Empirical notes: sigma-clipping approaches $\sim0.95\sqrt{N}$; median is $\sim20\%$ less efficient; pure mean optimal ($1.00\sqrt{N}$) only for Gaussian noise without outliers. Classical $\sigma$-clipping fails under masking when one extreme inflates mean/$\sigma$ (audit example: sky ~100 ADU + 2450 ADU outlier); MAD-based $\sigma_{\text{MAD}} = 1.4826\cdot\text{MAD}$ is robust. Choose algorithm by $N$, sky gradients and artifact density.

### 5.4 Diagnostic auxiliary stacks and shipped rejection state

* **Max Stack:** absolute maximum along temporal axis — satellite density, meteor trails, reflections.
* **Min Stack:** absolute minimum — sky uniformity, residual vignetting, sensor obstructions.

**[HISTORICAL v0.4.1]** also produced **no-correction mean and sum** diagnostics. Current: MAD sigma-clipping (default **3.0σ, 2 iterations**), per-pixel **rejection/weight maps** (`stacked_rejection_map.jpg`, `stacked_weight_map.jpg`, unregistered counterparts, `stacked_coverage_map.jpg`), `--sigma-clip-low/high` asymmetric thresholds. **[IMPLEMENTED 2026-11-17]** single-pass sigma-clip: final mean folded into last clip pass, survivor list carried between iterations, tile sums gathered in one pass; **median/MAD rejection deliberately retained** (full pass-to-variance replacement rejected — it would change rejection semantics and break bit-exact equivalence). `testSinglePassSigmaClipEquivalence` verifies exact float32 parity against test-only two-pass references; OrionNebula 4-frame sigma stack produced a byte-identical `stacked_mean.f32raw`.

---

## 6. Star detection, registration and distortion (core math)

Catalog-level astrometry, HEALPix, Gaia and annotation → [02](02-astrometry-and-identification.md). This section keeps only the engine math.

### 6.1 Subpixel centroid estimation

8-connected pixel clusters exceeding $5\sigma$ above local background; intensity-weighted spatial moments:

$$x_c = \frac{\sum_{i\in S} x_i\,(I_i-B)}{\sum_{i\in S}(I_i-B)},\quad y_c = \frac{\sum_{i\in S} y_i\,(I_i-B)}{\sum_{i\in S}(I_i-B)}$$

$I_i$ = pixel intensity, $B$ = local background, $S$ = star-profile pixel set. **[HISTORICAL v0.4.1]** detection used a high-pass residual (box blur radius **12** + robust MAD), 8-connectivity, component size **1–200 px**, aspect-ratio filter, sub-pixel flux-weighted centroids; reference = highest star count.

### 6.2 Geometric registration

**Triangle matching:** top **30** brightest stars → scale/rotation-invariant descriptor $\left(\frac{L_{\text{short}}}{L_{\text{long}}}, \frac{L_{\text{medium}}}{L_{\text{long}}}\right)$ for $L_{\text{long}}\ge L_{\text{medium}}\ge L_{\text{short}}$; nearest-neighbor matching against the reference catalog. **[HISTORICAL v0.4.1]** canonical side ratios + chirality check + **0.01** grid bucketing, similarity transform refined by inliers, registration gates **≥8 inliers** and **inlier ratio ≥0.20**, quality penalty for large scale/rotation deviations.

**Quad matching (ASTAP Quad Engine):** four centroids → irregular 2D tetragon; **six** inter-star distances → **6-digit invariant hash**. Flags: hash tolerance `-t` default **0.007**; star limit `-s` default **500**; HFD filter `-m` excludes detections below **0.8 px** (or **<1.5″** for plate solving).

**Similarity transform (RANSAC-filtered):**

$$\begin{bmatrix}x'\\y'\end{bmatrix} = s\begin{bmatrix}\cos\theta & -\sin\theta\\ \sin\theta & \cos\theta\end{bmatrix}\begin{bmatrix}x\\y\end{bmatrix} + \begin{bmatrix}t_x\\t_y\end{bmatrix}$$

RANSAC fits candidate matrices from random minimal subsets, rejects matches beyond subpixel reprojection error (suppressing hot-pixel/cosmic-ray false matches). **[HISTORICAL v0.4.1]** registration is similarity-only (no optical distortion model, no full affine) — adequate for short-to-medium focal lengths, weak for wide fields / field rotation. Optional full affine refinement is a later addition. Resampling to reference coordinates uses bilinear (implemented) with pixels mapping outside marked invalid (`-1`) and a safe crop to the largest axis-aligned rectangle where every frame contributed — maximal-rectangle-in-histogram algorithm.

### 6.3 SIP polynomial distortion

Blind solving assigns WCS headers mapping $(x,y)\to(\alpha,\delta)$ (Gaia DR3 reference design). 3rd-order SIP (design; **order-2 implemented**) corrects field curvature, coma and distortion:

$$u = f(x,y) = \sum_{p+q\le3} A_{p,q}\,x^p y^q = x + A_{2,0}x^2 + A_{1,1}xy + A_{0,2}y^2 + A_{3,0}x^3 + A_{2,1}x^2y + A_{1,2}xy^2 + A_{0,3}y^3$$

$$v = g(x,y) = \sum_{p+q\le3} B_{p,q}\,x^p y^q = y + B_{2,0}x^2 + B_{1,1}xy + B_{0,2}y^2 + B_{3,0}x^3 + B_{2,1}x^2y + B_{1,2}xy^2 + B_{0,3}y^3$$

TAN–SIP detail (Shupe et al. 2005, SIP v1.0; CTYPE `RA---TAN-SIP`/`DEC--TAN-SIP`): relative pixel $(u,v)=(x-\text{CRPIX1}, y-\text{CRPIX2})$; direct polynomials $U=u+\sum_{p+q\ge2}^{A\_ORDER}A_{p,q}u^p v^q$, $V=v+\sum_{p+q\ge2}^{B\_ORDER}B_{p,q}u^p v^q$ (orders 2–9) before the linear CD matrix. **Inverse AP/BP coefficients must include linear terms ($p+q=1$)** for accurate large-sensor inversion; omitting them degrades corners.

**Status:** `fitSIPFromMatches`, `wcsTanInverseSIP`, order-2 in WCS forward/inverse helpers and `--resample-sip` are implemented/tested (`testWCSSIP`, `testResampleSIP`); current resampling assumes a shared distortion model from the stored plate solution. Per-frame astrometric reprojection remains open.

---

## 7. OSC demosaicing and the CFA/Bayer drizzle decision

One-Shot Color raw data uses a Bayer CFA; reconstruction options:

* **AstroC (saturated star reconstruction):** interpolates channel ratios from surrounding unsaturated pixels when star cores saturate, preventing artificial color tinting.
* **AstroM (monochrome star balancing):** analyzes local $2\times2$ Bayer blocks ($R,G_1,G_2,B$); derives monochrome luminance for undersampled data, keeping stars neutral white.
* **AstroSimple (Han Kleijn, CC BY 4.0):** extracts discrete $R,G_1,G_2,B$ from local $2\times2$ neighborhoods without cross-channel bleeding; preserves high-frequency detail, minimizes edge color artifacts.
* **Standard Bilinear:** averages across a $3\times3$ grid; smooth but blurs high-frequency stellar profiles.

**Drizzle decision (survey, open work):** official Siril documentation establishes the **square drizzle kernel** distributes each pixel's flux exactly in proportion to overlap area (flux conservation essential for diffuse nebulosity), while **Point** kernels lower correlated noise at high risk of null-pixel holes. Because a Bayer CFA samples green at 50% and red/blue at 25% density, **CFA drizzle at scale=1.0 already has the droplet-coverage sparsity of 2× mono drizzle**; upscaling on top needs 4×-equivalent coverage. Scale=1.0 CFA drizzle eliminates demosaicing color artifacts, suppresses green midtone bias and yields finer noise even on well-sampled small-pixel CMOS **provided frames have random non-integer dithers** (e.g. 2.45 or 3.65 px). Practitioner reports: good at **15–30 frames**, acceptable at **10–15**, pattern/hole artifacts **<10**; 1.5×–2× drizzle can consume massive scratch space (one report: **1.25 TB in APP for 8 h of M51 at 1.5×/1.5**, vs minimal at scale=1.0, droplet 0.7). Null pixels reduce effective sample counts for rejection. **Decision:** true drizzle is an explicit non-goal / open gap; **[SURVEY PROPOSAL, OPEN]** `--bayer-drizzle` = scale=1.0 square-kernel CFA drizzle on raw Bayer planes (`dcraw -D -4`, configurable droplet 0.7–1.0), with per-channel coverage weights and warnings for $N<15$ / integer-locked dithers. 2× mono/CFA upscale drizzle as a general mode is **explicitly rejected** (helps only when stars are blocky, >4″/px, needs 30–300 subs and TB-scale I/O; degrades SNR and causes rejection holes on typical 4–16 frame sessions). Shipped: `drizzleUpsample` + `--drizzle-upsample/--drizzle-pixfrac` write a diagnostic upsample of the final stack only (`testDrizzleUpsample`).

---

## 8. Non-linear post-processing

Stacked masters are **linear** (faint nebulosity in low histogram percentiles). Two views of the pipeline exist (see §1): the design's **6-stage** grouping and the implemented/README **8-step** order.

```text
[ 1. Background Extraction & 2D Polynomial Gradient Fitting ]  <-- (--mask-bg)
                    ▼
[ 2. Chromatic Aberration Suppression ]
                    ▼
[ 3. Spectrophotometric Color Neutralization ]
                    ▼
[ 4. Non-Linear Stretching (GHS / Asinh / MTF) ]
                    ▼
[ 5. Vignette Residual Flattening ]
                    ▼
[ 6. High-Pass Spatial Sharpening (Unsharp Mask) ]
```

**6-stage design details:**
1. **Background extraction:** 2D second-degree polynomial surface; `--mask-bg` masks luminance $>0.1$ targets so objects do not bias the fit. **Color calibration:** channel medians in the **10th–50th luminance percentile** with scaling factors bounded to **$[0.5,1.5]$**.
2. **Chromatic aberration suppression:** dampens purple/magenta halos where R/B deviate from G.
3. **Non-linear stretches:**
   * **GHS:** $f(x)=\dfrac{\sinh^{-1}\!\left(\frac{x-D}{b}\right)-\sinh^{-1}\!\left(\frac{-D}{b}\right)}{\sinh^{-1}\!\left(\frac{1-D}{b}\right)-\sinh^{-1}\!\left(\frac{-D}{b}\right)}$ — $b$ stretch factor, $D$ black-point protection, applied to luminance with proportional RGB scaling.
   * **AsinhStretch:** $f(x)=\dfrac{\operatorname{asinh}(\beta\,(x-c))}{\operatorname{asinh}(\beta)}$ — $\beta$ intensity, $c$ black point; preserves RGB relations.
   * **MTF:** $\text{MTF}(x,m)=\dfrac{(m-1)\,x}{(2m-1)\,x-m}$, $m\in(0,1)$. Normalize to $[0,1]$ via $x_p=(\text{orig}-\text{shadows})/(\text{highlights}-\text{shadows})$.
4. **Vignette residual flattening:** smoothed illumination pass post-stretch.
5. **High-pass sharpening (unsharp mask).**
6. **Spectrophotometric/photometric color:** SPCC/PCC computes instrumental magnitudes vs reference SEDs (e.g. Gaia DR3) on **linear** data before stretch; SCNR/HLVG suppresses Bayer-induced green casts (2× as many green pixels).

**Wavelet multiscale (*à trous*):** $I(x,y)=c_N(x,y)+\sum_{k=1}^{N} w_k(x,y)$ — $c_N$ low-frequency background, $w_k$ detail scales; noise in $w_1,w_2$, nebulosity in $w_3\dots w_N$ for targeted processing without halos. **[IMPLEMENTED]** `--wavesharp`/`--wave-gains` = coarse-to-fine multi-scale Gaussian detail enhancement (default gains **0.25/0.15/0.08** at sigma **18/6/2 px**; `testWaveSharpen`) — deliberately conservative classical baseline, not neural.

**AI/ML in design (not embedded):** the design section describes StarXTerminator/NoiseXTerminator/GraXpert integration — U-Net starless workflows and self-supervised denoising (requiring cosmic-ray rejection first). **Current authoritative decision: embedded neural inference is out of scope**; only external licensed CLI sidecars are orchestrated — see [06](06-external-ai-sidecars.md) and §16.

**[HISTORICAL v0.4.1]** 8-stage order reading the `float32` raw mean stack: (1) polynomial background degree 1–3 with optional mask; (2) optional CA/color/flatten/unsharp/HDR; (3) star removal (annulus median, size-adaptive); (4) percentile black-point; (5) PixInsight-style STF/MTF auto-stretch (linked/unlinked, σ shadow clip); (6) guided-filter denoise; (7) HDR blend + weighted mixes; (8) optional multi-level stretch variants. `--tiff` writes uncompressed `float32` RGB TIFF of the raw sum.

**Default pipeline review (2026-11-17):** background polynomial → star removal → black-point → STF stretch → guided+chroma denoise → curves → HDR blend/mix → spikes. Findings: star removal and black-point occur in the linear-ish domain before stretch; HDR blend reintroduces linear detail from `step1Frame` (captured before star removal), so output is not fully starless — no correctness change needed. New functions `--bg-multiscale`, `--deconvolve`, `--wavesharp` are deliberately opt-in and run before star removal/stretch on linear data (defaults risk noise amplification/ringing). Implemented improvement: report reference-frame HFR (seeing proxy) in log/`report.txt`/`report.json` so users can judge whether sharpening is worthwhile. Ordering note: optional sharpening runs before denoise (denoise partly cancels it); a future `--post-preset` could move detail enhancement after denoise. **[HISTORICAL]** `--post-*` flags (CA, color, flatten, sharpen, HDR, stretch), `--no-color-calib`, `--no-sharpen` appear in the design CLI table; current controls are the 8-step defaults plus opt-ins.

**GHS stretch math (reference detail):** uses $D=e^{\text{Stretch}}-1$ (or related parameters), midpoint SP, shadow/highlight protection LP/HP, branch functions **T1–T4** with $C^1$ continuity and normalization **NormT**; inverse forms exist for round-trip editing. Validate exact parameter mapping against Siril/GHS docs. Arcsinh variant: $\operatorname{asinh}(r\cdot\text{stretch})/(r\cdot\operatorname{asinh}(\text{stretch}))$ with blackpoint; RGB blend clipping (clamp factor $k$) may avoid channel overflow.

---

## 9. Automated image quality diagnostics

```text
┌─────────────────────────┬─────────────────────────┬─────────────────────────┐
│  Area 1A (Corner TL)    │   Area 2A (Top Center)  │  Area 3A (Corner TR)    │
├─────────────────────────┼─────────────────────────┼─────────────────────────┤
│  Area 1B (Left Center)  │   Area 2B (CENTER)      │  Area 3B (Right Center) │
├─────────────────────────┼─────────────────────────┼─────────────────────────┤
│  Area 1C (Corner BL)    │   Area 2C (Bot Center)  │  Area 3C (Corner BR)    │
└─────────────────────────┴─────────────────────────┴─────────────────────────┘
```

* **HFD (Half Flux Diameter):** diameter in pixels containing **50%** of a star's cumulative photon flux — focus quality.
* **Sensor tilt (9 zones: corners 1A/3A/1C/3C; edges 2A/1B/3B/2C; center 2B):**
  $$\text{Tilt \%}=100\times\frac{\text{HFD}_{\text{worst corner}}-\text{HFD}_{\text{best corner}}}{\text{HFD}_{\text{median}}}$$
  Severity: **<5%** None; **5–10%** Negligible; **10–15%** Mild (acceptable); **15–20%** Moderate; **20–30%** Severe (mechanical adjustment); **>30%** Extreme (critical). Three-corner triangular mode for adapters with three **120°** push-pull screws evaluates three 120° radial segments excluding the inner **25%** center, mapping HFD variance to physical screws.
* **Off-axis aberration:** $\Delta\text{HFD}=\text{HFD}_{\text{out}}-\text{HFD}_{\text{centre}}$, outer field at **≥75%** radial distance. 2D HFD contour maps and Voronoi partitions visualize spatial variation. Roundness = rotating 180° split-line profile; max/min average radial distance ratio (**1.0** = circular).
* **SQM/extinction:** sky brightness in $\text{mag/arcsec}^2$ by calibrating background flux against catalog stars with Pickering airmass; $\text{Extinction}=0.2811\times\text{Airmass}$.

---

## 10. Data formats and metadata

* **TIFF 6.0 (32-bit float masters):** 8-byte header + IFDs; byte order `0x4949` (`II`, little-endian) or `0x4D4D` (`MM`, big-endian); magic **42 (`0x002A`)**; Tag **258 (`BitsPerSample`) = 32**, Tag **339 (`SampleFormat`) = 3** (`IEEEFP`), Tag **262 (`PhotometricInterpretation`) = 1** (`BlackIsZero`, mono) or **2** (`RGB`).
* **BigTIFF (>4 GB / $2^{32}$ bytes):** magic **43 (`0x002B`)**; 64-bit `LONG8` offsets/byte counts/tag parameters. **[OPEN]** BigTIFF export not shipped.
* **FITS / FITS.FZ (Rice):** 2880-byte ASCII header blocks + binary arrays; `.fits.fz` compresses tile-by-tile preserving dynamic range. **[OPEN]** pure-Go FITS Rice codec (`.fits.fz`) not shipped; survey notes ASTAP added native FITS Rice (July 2026) and explicitly excluded GZIP FITS.
* **XISF (PixInsight):** monolithic/distributed XML metadata + LZ4/Zstandard-compressed binary blocks; stores processing history, WCS and cryptographic checksums. **[OPEN]** only minimal monolithic XISF 1.0 is written (`--xisf`); **XISF Specification Revision 2** conformance (SHA3-256/SHA3-512 block checksums, parallel compressed I/O, strict sample-mapping validation, corrupt-block rejection) is open — see §16.
* **Zero-allocation metadata ingestion (`imagemeta`):** fixed-buffer reads of EXIF/IPTC/XMP into lightweight descriptors:
  ```go
  type FrameDescriptor struct {
      FilePath string; Width, Height uint32; ExposureTime float64
      CameraModel string; Timestamp time.Time
  }
  ```
* **[CURRENT]** `--fits` writes `stacked_sum.fits` (big-endian `BITPIX=-32` + TAN WCS keywords when plate-solved); `--xisf` minimal monolithic XISF 1.0; `.f32raw` intermediates; `--json-report`; `--config` JSON flag files; `--resume`. Tests `testFITSWriterRoundTrip`, `testXISFWrite`.

---

## 11. SkyStacker CLI flag reference (design + current)

Invocation: `skystacker [flags] [input-directory]`. The table below is the design/spec flag set preserved in full; current shipped additions are listed after it. Defaults use the design convention (`skystacker_results`); see §1 for the v0.4.1 `0_skystacker_results` literal.

| Flag | Argument | Description | Default |
| :--- | :--- | :--- | :--- |
| `--output` | `string` | Explicit output directory | `<input>/skystacker_results` |
| `--method` | `string` | Integration (`sigma`, `winsorized`, `mean`, `median`) | `sigma` |
| `--workers` | `int` | Max concurrent worker threads | CPU thread count |
| `--recursive` | `bool` | Recurse subdirectories | `false` |
| `--overwrite` | `bool` | Overwrite output directory contents | `false` |
| `--no-register` | `bool` | Disable registration pass | `false` |
| `--dark` / `--flat` / `--bias` | `string` | Master calibration file or directory | None |
| `--calib-root` | `string` | Root of automated Bias/Dark/Flat structure | None |
| `--stack-only` / `--post-only` | `bool` | Integration-only / post-only pass | `false` |
| `--mask-bg` | `bool` | Mask bright structures during background fit | `false` |
| `--bg-degree` | `int` | Background polynomial degree (1–3) | `2` |
| `--stretch` | `float` | Asinh/GHS stretch intensity | `10.0` |
| `--sigma-low` / `--sigma-high` | `float` | Rejection thresholds | `3.0` / `3.0` |
| `--sharpen-amount` / `--sharpen-radius` | `float` | Unsharp mask strength / radius (px) | `0.8` / `1.5` |
| `--hdr-amount` | `float` | HDR local-contrast intensity | `0.5` |
| `--star-radius` | `int` | Star removal mask radius (`0` = auto) | `0` |
| `--star-sigma` | `float` | Star detection threshold (σ units) | `4.0` |
| `--bg-percentile` | `float` | Black-point luminance percentile | `10.0` |
| `--scale` | `int` | Downscale factor (`2`=half, `4`=quarter) | `1` |
| `--force` | `bool` | Bypass RAM safety checks | `false` |
| `--tiff` | `bool` | Export raw uncompressed `float32` TIFF sum | `false` |
| `--post-ca` / `--post-color` / `--post-flatten` / `--post-sharpen` / `--post-hdr` / `--post-stretch` | `bool` | Enable individual post passes | `false` |
| `--no-color-calib` / `--no-sharpen` | `bool` | Disable color calibration / sharpening | `false` |
| `--selftest` / `--version` / `--help` | `bool` | Run self-tests / print version / help | N/A |

Standard example (design):

```bash
./skystacker \
  --method winsorized --sigma-low 2.8 --sigma-high 3.2 \
  --calib-root ./M31-session/calibration --mask-bg --stretch 12.5 \
  --output ./M31_stacked_output ./M31-session/lights
```

**[CURRENT shipped flags, beyond the design table]** `--self-flat`, `--bpm`, `--plate-solve`, `--comet-center`, SER input; `--fits`, `--xisf`, `--json-report`, `--config`, `--resume`; `--bpm-auto`, `--bpm-auto-sigma` (default 5); `--normalize`, `--local-normalize`, `--normalize-mode`, `--normalize-per-channel`; `--bg-multiscale`, `--deconvolve`, `--wavesharp`, `--wave-gains`, `--narrowband HOO|SHO`, `--starless`, `--annotate`; `--sigma-clip-low/high`, `--hfr-quality`; `--drizzle-upsample`, `--drizzle-pixfrac`; `--reference-crop`, `--test-gaia RA,DEC`, `--cache-dir`, `--no-cache`; `--comet-radec`, `--comet-sky-rate-arcsec`, `--comet-target`, `--comet-ephem-strict-exif`; `--resample-sip`; `--external-bge`, `--external-denoise`, `--external-deconv`, `--external-starless`, `--external-timeout`; `--no-native-cr2` (native pure-Go CR2 decode is default; CR3 still needs an external decoder). Observatory automation, return-code protocols and live APIs from the CLI doc are design notes, not shipped behavior.

---

## 12. Headless observatory pipeline, ASTAP interop and deployment

### 12.1 7-stage decoupled headless pipeline

```text
1. FILE DISCOVERY & INDEXING  ──► [ Extract EXIF/FITS Headers & Validate Catalog ]
2. CALIBRATION PREPROCESSING  ──► [ Apply Master Dark, Master Bias & Master Flat ]
3. ALIGNMENT & WCS CHECK      ──► [ Quad Hash Matching (0.007 Tol) & Matrix Offsets ]
4. PARALLEL TILE ACCUMULATOR  ──► [ Partition Spatial Planes (1024x1024) into float32 Buffers ]
5. OUTLIER REJECTION          ──► [ Temporal Filtering (Sigma-Clipping / Winsorized) ]
6. SCIENTIFIC ENCODING        ──► [ IEEE 754 32-bit Float Output & Asinh/MTF Render ]
7. TELEMETRY AUDIT REPORT     ──► [ Export Machine-Readable INI / JSON Metadata ]
```

| Stage | Input artifacts | Memory state | Output artifacts |
| :--- | :--- | :--- | :--- |
| 1. Discovery & indexing | Directory, FITS/RAW/TIFF | RAM (metadata index table) | `catalog.json` / validated file list |
| 2. Calibration | Lights, master dark/flat | RAM (`float32` buffers) | Calibrated light pixel streams |
| 3. Alignment & verification | Calibrated streams, reference | RAM (quad matrices, WCS) | Registered transforms |
| 4. Tile accumulation | Aligned streams, tile bounds 1024×1024 | RAM (worker pool `sync.Pool`) | Partitioned `float32` tile matrices |
| 5. Rejection & stack | Partitioned tiles | RAM (temporal array allocation) | Filtered integrated master tile array |
| 6. Encoding & rendering | Master tile array | RAM → disk I/O stream | 32-bit float FITS/TIFF, stretched preview |
| 7. Telemetry & reporting | System state, rejection counts, HFD | RAM → disk | `skystacker_results/` JSON audit + `.ini` |

Headless execution saturates CPU cores with deterministic memory via tile streaming and `sync.Pool`. Runtime inputs: FITS/FITS.FZ/RAW/TIFF/XISF.

### 12.2 ASTAP interoperability flag reference

| Flag | Argument | Description | Context |
| :--- | :--- | :--- | :--- |
| `-f` | `<filepath>` | Target input image | Solve/analysis input |
| `-r` | `<degrees>` | Search radius | Spiral search limit |
| `-fov` | `<degrees>` | Forced image height FOV (`0`=Auto) | Scale override |
| `-ra` | `<hours>` | RA center hint (decimal hours or `HH:MM:SS`) | Non-FITS solve (verify units) |
| `-spd` | `<degrees>` | South Pole Distance ($\text{Dec}+90°$, positive) | Non-FITS solve |
| `-z` | `<0..4>` | Downsampling (`0`=Auto Binning) | Speed/SNR |
| `-s` | `<count>` | Max stars for quads (default `500`) | Performance |
| `-t` | `<tolerance>` | Quad hash tolerance (default `0.007`) | Distortion tolerance |
| `-m` | `<min_hfd>` | Reject detections below HFD threshold | Hot-pixel suppression |
| `-check` | `y`/`n` | Bayer pattern check before solving raw OSC | Raw debayer check |
| `-d` | `<path>` | Star database directory | DB path override |
| `-D` | `<code_name>` | Catalog (`d80`, `d50`, `d20`, `d05`, `v50`, `w08`) | DB selection |
| `-o` | `<filepath>` | Output base path/prefix | Output routing |
| `-sip` | `y`/`n` | Add 3rd-order SIP coefficients | Wide-field distortion |
| `-speed` | `slow`/`auto` | Slow search, 50% field overlap | Low-star fallback |
| `-wcs` | — | Standalone `.wcs` (80-char blocks) | Astrometry.net compat |
| `-update` | — | Write solved WCS into source header | In-place FITS update |
| `-log` | — | Log telemetry to `.log` | Pipeline logging |
| `-analyse` | — | Median HFD + star count | Fast QC |
| `-extract` | `<min_snr>` | Centroids, flux, HFD → CSV | Photometry |
| `-extract2` | `<min_snr>` | Centroids + solved RA/Dec → CSV (SIP) | Astrometric extraction |
| `-sqm` | — | Sky brightness $\text{mag/arcsec}^2$ | Sky quality |
| `-focus1..N` | — | Hyperbola focus-sequence evaluation | Focuser calibration (GUI/debug fit) |
| `-stack` | — | Live directory monitoring + stacking | Live stacking |
| `-p` | — | Batch photometry via `.ini` | Light curves |

**ASTAP return encodings (corrected — see §1):**
* `-analyse` Windows `%errorlevel%`: `errorlevel = round(median_HFD × 100) × 1_000_000 + stars_detected`; `median_HFD = trunc(errorlevel / 1_000_000) / 100`. Example HFD **3.26**, **666** stars → **326000666**. **32-bit signed ceiling = 21.46 px** (6 digits reserved for star count); defocused sessions should parse stdout instead. Linux/macOS emit `HFD_MEDIAN=3.3 STARS=666`.
* `-focus1..N` draft encoding: `errorlevel = integer(focuspos × 10,000) + integer(rem.error × 1,000)`; Linux/macOS emit curve-fit coefficients on stdout — **do not rely on a CLI hyperbolic-fit return code** (fit is GUI/`-debug` path).
* Exit codes: **0** Success, **1** Solve Failure, **2** Low Stars, **16** File Read Error, **32** Database Missing, **33** Database Read Error, **34** Header Update Failure.
* Catalogue selection is by **surface density per deg²**, not a uniform mag limit; include **G05** as a mid-range bridge in the cascade (W08 very wide → G05 → D80/V50/D50/D20/D05 branches).

### 12.3 OS / deployment matrix and external dependencies

* **Windows:** `astap_setup.exe` (64-bit), `astapwin32.zip` (32-bit), `astap_cli.exe` native Windows 11 ARM64 (claimed up to **375%** vs x86 emulation — developer-reported). StarNet2 uses `DirectML.dll` GPU acceleration.
* **Linux:** `x86_64`, `aarch64` (ARM64), `armhf` (ARM32); GTK2/GTK3; `.deb` (Debian/Ubuntu/Raspbian), `.rpm` (OpenSUSE/Fedora), `.pkg.tar.zst` (Arch/Manjaro), `.tar.gz`. Default install `/opt/astap/`; star data `/usr/share/astap/data/`.
* **macOS:** native PKG for Intel 64-bit and Apple Silicon (M1/M2/M3); star DB `/usr/local/opt/astap/`. **Mandatory code-signing (Sequoia 15.1+):** `codesign --force -s - /Applications/ASTAP.app/Contents/MacOS/astap`.
* **Android:** `astap_cli` for `aarch64`, `armhf`, `x86_64`, `x86`; used by **OpenLiveStacker** and **Stellar Finder**. ASTAP adopted StarNet2 for comet stacking around **2026-09**.
* **Runtime deps (design):** Go 1.20+ concurrency (now 1.27.4); RAW debAYERING via `unprocessed_raw` (modified LibRaw), David Coffin's `dcraw`, ImageMagick, OpenSSL 1.1 (`libssl-dev`/`openssl_libraries.pkg`) for online catalogs. **[CURRENT]** native pure-Go CR2 decode is default (see [07](07-native-cr2-decoding.md)); CR3 still external.
* **Input extensions:** FITS (`.fits`, `.fit`), `.fits.fz`, Canon RAW (`.cr2`, `.cr3`), `.jpg`, `.png`, `.tiff`/ASTRO-TIFF, uncompressed `.xisf`, `.pgm`, `.ppm`; SER video input expanded per `#frame=N`.

### 12.4 Output folder schema and telemetry

```text
skystacker_results/
├── master_stack_integration.fits  # Integrated 32-bit IEEE 754 Float Master
├── master_stack_preview.tiff      # 32-bit Float TIFF (SampleFormat=3)
├── stacked_processed.jpg          # 8-bit stretched RGB preview
├── stacked_max.jpg                # Diagnostic maximum-pixel composite
├── stacked_min.jpg                # Diagnostic minimum-pixel composite
├── target_observation.wcs         # Fixed 80-char WCS Header Block
├── execution_parameters.ini       # Machine-readable Execution State INI
└── telemetry_audit.log            # JSON Audit Export & Log Trace
```

* **WCS file:** FITS 80-character records without carriage returns, Astrometry.net compatible.
* **INI sample:** `PLTSOLVD=T`, `CRPIX1=1.1645000000000000E+003`, `CRPIX2=8.8050000000000000E+002`, `CRVAL1=1.5463033992314939E+002`, `CRVAL2=2.2039358425145043E+001`, `CDELT1=-7.4798001762187193E-004`, `CDELT2=7.4845252983311850E-004`, `CROTA1=-1.1668387329628058E+000`, `CROTA2=-1.1900321176194073E+000`, `CD1_1=-7.4781868711882519E-004`, `CD1_2=1.5241315209850368E-005`, `CD2_1=1.5534412042060001E-005`, `CD2_2=7.4829732842251226E-004`, `CMDLINE=astap -f target.fits -r 30 -update`, `WARNING=None`, `ERROR=None`.
* **Telemetry log:** rejected pixel percentages, HFD medians, frame counts, exit codes (§12.2).
* **[HISTORICAL v0.4.1]** `report.txt` holds per-frame star counts, inlier counts, scale/rotation/offset, quality scores, transformed bounding boxes; atomic file writes and peak-memory estimator with GC lag.

---

## 13. Advanced architectural extensions

### 13.1 Dual-stack comet and ephemeris recombination

Comets/solar-system targets move against star fields; sidereal alignment yields trailed stars or blurred nuclei. Dual-stack recombination (design, using StarNet2):

```text
[ Calibrated Sub-Exposures ] → [ Neural Star Separation ] → [ Star-Only ] / [ Target-Only ]
   → [ Sidereal Star Alignment ] / [ Ephemeris Drift Alignment ]
   → [ Integrated Star Master ] / [ Integrated Target Master ] → [ Recombined Master ]
```

1. **Conditioning:** calibrated subframes get inverse-sinh (`sinh`) stretch for neural evaluation.
2. **Separation:** StarNet2 splits star-only and comet-only images.
3. **Centroid remnant extraction:** with "extract comet remnant" enabled, stars within **~5 px** of the ephemeris comet centroid are removed from the star-only frame and shifted to the comet stream.
4. **Dual integration:** star frames aligned on stellar quads; comet frames aligned using JPL Horizons ephemeris drift vectors ($\Delta\alpha,\Delta\delta$ in arcsec/hour).
5. **Recombination:** merged in 32-bit float space without non-linear clipping.

**[CURRENT]** pixel mode `--comet-center X,Y --comet-rate dX,dY` (px/s, EXIF `DateTimeOriginal`), RA/Dec mode `--comet-radec RA,DEC --comet-sky-rate-arcsec dRA*,dDec` (decimal degrees, arcsec/s, requires `--plate-solve`), and ephemeris mode `--comet-target PATH` (`ISO8601 RA_deg Dec_deg` lines, linear interpolation by EXIF time, reference time as base; missing EXIF → reference time + warning). Outputs `stacked_comet.f32raw`/`.jpg`; star stack unchanged. Tests `testCometTranslation`, `testCometEphemerisParse`. RA/Dec comet ephemeris was research at v0.6.0 and is now shipped in these modes; per-frame WCS astrometric reprojection remains open.

### 13.2 Ensemble photometric calibration

Instrumental magnitudes $m=m_0-2.5\log_{10}(\text{Flux})$ calibrated against catalog standards (Gaia DR3, Johnson-V) via SNR-weighted zero points:

$$m_{0,\text{ensemble}}=\sum_{i=1}^{K}\left(\frac{w_i}{\sum_{j=1}^{K} w_j}\,m_{0,i}\right),\quad w_i=\text{SNR}_i$$

Outlier zero points ($>1.5\sigma$ from median) rejected via MAD: $\text{MAD}=\text{median}(|m_{0,i}-\text{median}(m_0)|)\Rightarrow\sigma_{\text{MAD}}=1.4826\times\text{MAD}$. Color transformation:

$$V_{\text{var}}=\Delta v + T_{v\_bv}\cdot T_{bv}\cdot\big((b-v)_{\text{var}}-(b-v)_{\text{comp}}\big)+V_{\text{comp}}$$

$T_{bv}$ maps instrumental to standard color indices; $T_{v\_bv}$ corrects magnitude shifts from target $B-V$ color. **Status:** design; full PCC/SPCC and PSF-fitting photometry remain open (see [02](02-astrometry-and-identification.md)).

---

## 14. Benchmarks and methodology

**Developer host:** Intel i7-8565U (4c/8t, 15 W ULV, ~2–4.6 GHz), **15 GiB RAM**, Ubuntu 20.04; original toolchain **Go 1.20.3**.

| Run | Frames / config | Time (historical baseline) |
| :--- | :--- | :--- |
| OrionNebula CR2 @ 2602×3908, grid self-flat + dark/bias | 16 lights | **6m05s** |
| OrionNebula with real flat | 16 lights | **7m03s** |
| OrionNebula | 4 lights | **~2m02s** |
| Post-processing of 2599×3894 stack | default | **22.4 s** (22.43 s) |
| Post-processing + deconvolution + multi-scale sharpening | — | **38.3 s** |

**[Go 1.27.4 re-baseline, 2026-11-17]** (same data/flags; not a controlled A/B — toolchain, CR2 decoder and thermal variance all changed):

| Run | Frames | Flags (abridged) | Duration |
| :--- | ---: | :--- | :--- |
| Native CR2 stack | 4 | `sigma`, `--stack-only --dark --bias` | **39.031 s** (wall 39.25 s) |
| Post-processing + starless | 1 stack | `--resume --starless` | **21.11 s** wall |

**Sigma-clip kernel microbenchmark** (`go test -run '^$' -bench BenchmarkSigmaClip -benchtime=2s -count=3`, i7-8565U, 16 frames, 2 iterations, one injected outlier, median of 3): **Unweighted fused ~763 ns/op vs two-pass ~1048 ns/op; Weighted fused ~833 ns/op vs ~1033 ns/op** (~20–25% faster kernels; end-to-end gains masked by CR2 decoding, registration and disk spill I/O; fused output bit-identical — same SHA-256 of `stacked_mean.f32raw`). Earlier full-pass fusion was ~20–25% faster in the survey estimate.

**Reproducible benchmark methodology (survey):** no standardized 2024–2026 astro-stacking benchmark suite was found; adopt `goperf.dev`'s Go 1.24+ approach — `testing.B.Loop()`, per-benchmark mean/stddev/**coefficient of variation (CV)** and reliability classification pinned to kernel/hardware versions; combine with fixed CR2/FITS test sets, pinned CPU frequencies/governors on thermally throttling laptops, and identical debayering/rejection parameters. External comparisons (DSS on i5-6200U; Siril/PI/APP on FX-8320/i7-9700K) are documented but **not apples-to-apples**. Proposal 1 acceptance: 5 repeated OrionNebula 4- and 16-CR2 passes at fixed governor on the i7-8565U, logging wall time, peak RSS and GC pause totals, with all 203 self-tests/vet/-race/`CGO_ENABLED=0` cross-builds green on Go 1.26 and beating 6m05s/22.4s.

**[Validation context, 2026-11-16]** `assets/OrionNebula/` initially had only **3 lights (<15)** so grid self-flat was not built (needs **≥15**); with explicit `--dark/--flat/--bias` calibration applied and masters exported. After adding lights to **16 CR2 files**, running against `assets/OrionNebula` with `--calib-root assets/OrionNebula` failed early because the tool looked for `assets/OrionNebula/Darks` (capitalized plural) while the tree uses lowercase `darks/flats/biases/lights`; running against `assets/OrionNebula/lights` succeeded (16 frames, registration OK, grid diagnostics). This is a **path/case-sensitivity issue** in `detectCalibrationSubdirs`/`--calib-root` worth fixing on Linux. No dataset provides ≥15 lights + a real flat for a clean grid-vs-real-flat comparison; validation deferred.

---

## 15. External state-of-the-art survey (Gemini Deep Research, 2026-11-17) — consolidated

The former Gemini Deep Research prompt and outcome are absorbed into this section and the thematic files below; they surveyed 2024–2026 advances against the single-file/`CGO_ENABLED=0`/stdlib-only float32 architecture. **[SURVEY PROPOSAL, NOT IMPLEMENTED]** unless a status note in §17 says otherwise.

### 15.1 Where the survey findings now live

The 2024–2026 survey is distributed by theme; this section keeps only the consolidated feasibility matrix (§15.3) and the preserved source URLs (§15.4):

| Survey area | Canonical home |
|---|---|
| Go runtime, GC, module/toolchain decisions, FITS Rice / BigTIFF / XISF Rev 2 | [05-toolchain-and-modularity.md](05-toolchain-and-modularity.md) §3–§11 |
| RC-Astro / GraXpert / StarNet2 ecosystem, embedded-ML infeasibility, sidecar runner | [06-external-ai-sidecars.md](06-external-ai-sidecars.md) |
| Gaia DR4 / Archive v3.15, DCR, solvers, HEALPix/MOC | [02-astrometry-and-identification.md](02-astrometry-and-identification.md) §11 |
| Detector physics, dark QA, detrending, self-flat, calibration survey | [04-calibration-and-self-flat.md](04-calibration-and-self-flat.md) §13 |
| Drizzle/CFA decision, finite-sample MAD, single-pass sigma-clip, post-processing review | this file §5 and §7–§8 |
| Crawler and curated corpus that supplied several of these findings | [08-crawler-source-discovery.md](08-crawler-source-discovery.md) |

### 15.2 Domain findings (index only, no restatement)

The survey's domain-by-domain evidence, numbers and references are maintained once in the thematic files listed in §15.1. This architecture file keeps only cross-cutting consequences: finite-sample MAD and conservative single-pass sigma-clip are folded into §5, the CFA/Bayer drizzle decision into §7, and the rejection of embedded ML / `simd/archsimd` into §4.2. Historic survey-only gaps (no comparative MAD-variant benchmarks on faint nebulosity, no peer-reviewed TAN–SIP vs TPV comparisons, no Rust/C++ vs Go pipeline study) remain recorded in the thematic files rather than repeated here.

### 15.3 Consolidated feasibility matrix

| Technique | Benefit | Effort | Stdlib-Go | Dependency | Flag |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Upgrade Go toolchain (now 1.27.4) | Faster I/O, lower map RAM, GC gains | S | Yes | Toolchain | N/A |
| Finite-sample MAD scale (N<30) | Prevents over-rejection | S | Yes | None | `--finite-sample-scale` (**implemented** as `finiteMADScale`) |
| Single-pass mean+variance sigma-clip | ~20–25% faster reduction | S | Yes | None | default optimizer (**implemented**) |
| Dark-series QA & drift rejection | Excludes hot/drifted darks | S | Yes | None | `--dark-qa-reject` (**implemented**) |
| Gaia DR4 TAP throttling & cache | Avoids 5-job/50-queue rejection | S | Yes | ESA Archive | internal client (**implemented**) |
| CFA/Bayer drizzle scale=1.0 | Removes demosaic artifacts/green bias | L | Yes | `dcraw -D -4` | `--bayer-drizzle` (**open**) |
| SIP order-2 in resampler | Cuts wide-field corner error | M | Yes | None | `--resample-sip` (**implemented**) |
| Spatially gridded PSF for RL | Wide-field classical deconvolution | M | Yes | None | `--deconvolve-grid` (**open**) |
| Pre-solve gradient detrending | Rescues solves on gradients | S | Yes | None | `--solve-detrend` (**implemented** as `detectStarsWithDetrend`) |
| External AI CLI sidecars | Linear BGE/decon/denoise/starless | M | Partial (runner yes, ML no) | GraXpert/StarNet2 free; RC-Astro paid | `--external-bge/-denoise/-deconv/-starless` (**implemented**) |
| FITS Rice `.fits.fz` | Compact lossless tile I/O | M | Yes | None | `--fits-rice` (**open**) |
| BigTIFF float32 export | >4 GiB mosaics | M | Yes | None | `--bigtiff` (**open**) |
| XISF Rev 2 validation + checksums | PI parity, corrupt-block guard | S–M | Partial (SHA-3 check) | None | `--xisf-rev2` (**open**) |
| Embedded ONNX/GGUF inference | Native BXT/NXT | XL | **No** (284–456 MB, static binary) | ONNX Runtime/weights | **Do not implement** |
| `simd/archsimd` intrinsics | Minor amd64 pixel-loop gain | M | Partial (GOEXPERIMENT, amd64-only) | Breaks arm64 parity | **Do not implement** |

### 15.4 Survey source URLs (preserved verbatim)

**Gaia/IVOA:** https://www.cosmos.esa.int/web/gaia/dr4 ; https://www.cosmos.esa.int/web/gaia/bulletin-archive ; https://www.cosmos.esa.int/web/gaia-users/archive/release-notes ; https://wiki.ivoa.net/internal/IVOA/InterOpNov2025/State_of_the_IVOA__Nov_2025_FINAL.pdf ; https://docs.astropy.org/projects/astroquery/en/latest/changelog.html ; https://medium.com/@andy.a.g/gaia-dr4-is-scheduled-for-december-2-at-about-400-tb-esa-changed-the-archive-rules-first-61dd4a08fe22

**ASTAP/solvers:** https://www.hnsky.org/history_astap.htm ; https://www.hnsky.org/astap.htm ; https://www.cloudynights.com/forums/topic/793549-astrometric-plate-solving-on-your-phone-astap-for-android/page/2/ ; https://www.cloudynights.com/forums/topic/963963-is-astap-still-the-best-plate-solver-for-nina/ ; https://www.cloudynights.com/forums/topic/993559-astrohopper-experience/ ; https://skysolver.app/api-docs

**PixInsight/XISF/detector/QA:** https://pixinsight.net/dev/index.php?articles/pixinsight-1-9-6-lockhart-released.28/ ; https://pixinsight.net/dev/index.php?articles/pixinsight-1-9-6-lockhart-build-1718-released.30/ ; https://github.com/open-astro/openastro-ara/blob/master/CHANGELOG.md ; https://github.com/sergio-dr/xisf ; https://ericfossum.com/Publications/Papers/2025%20JATIS%20Shade%20Characterization%20of%20Photon-Counting%20Image%20Sensors.pdf ; https://pixinsight-scripts.caelo.works/en/scripts/dark-frame-analyzer

**RC-Astro/AI tools:** https://siril.org/2026/06/rc-astro-tools-available-in-siril/ ; https://www.astropixelprocessor.com/community/rfcs-request-for-changes/blurxterminator-support-integration-or-equiv/paged/2/ ; https://www.astropixelprocessor.com/community/tutorials-workflows/bayer-drizzle/ ; https://measurethepixels.com/changelog.html ; https://www.zelpio.com/es/blog/rc-astro-tools-standalone-guide/ ; https://github.com/Steffenhir/GraXpert ; https://github.com/Steffenhir/GraXpert/releases ; https://siril.readthedocs.io/en/latest/processing/graxpert.html ; https://siril.readthedocs.io/en/latest/preprocessing/drizzle.html ; https://www.deepskycolors.com/astro/docs/tools/DSPX/dscp/GraXpertDenoise/GraXpertDenoise.html ; https://www.deepskycolors.com/astro/docs/tools/DSPX/dscp/GraXpertDeconvolution/GraXpertDeconvolution.html ; https://polaris-astro.app.br/handbook/23-ai-cleanup.html ; https://www.setiastro.com/cosmic-clarity ; https://www.setiastro.com/seti-astro-suite-pro ; https://thenightmap.com/guides/blurxterminator-photoshop.php ; https://astrowizard.lukomatico.com/

**Drizzle/artifacts:** https://mypetstars.com/tutorials/pixinsight/processes/drizzleintegration/cfa-drizzle ; https://www.cloudynights.com/forums/topic/775840-bayer-drizzle-a-slight-tweak-or-a-serious-improvement/page/2/ ; https://www.cloudynights.com/forums/topic/886412-bayer-drizzling-for-osc-cameras/ ; https://www.cloudynights.com/forums/topic/956850-blurxterminator-introducing-artifacts/

**Go toolchain:** https://go.dev/blog/swisstable ; https://www.datadoghq.com/blog/engineering/go-swiss-tables/ ; https://goperf.dev/03-version-tracking/ ; https://www.alexedwards.net/blog/go-experiments-explained ; https://ajitem.com/blog/performance-optimizations-in-go-1-24-swiss-table-maps-and-more/

---

## 16. Observatory automation, telemetry and provenance (design, not shipped)

* **ASCOM Alpaca / N.I.N.A.:** REST/JSON discovery (often port **11111**), binary `imagebytes`, UDP discovery (**32227**); scripts for weather safety, power, cooling, dome/roof, PHD2 guiding, autofocus, rotator, flats, meridian flips, safe shutdown with per-frame state. 2026 directions: REST/WebSocket weather time series and operational commands, time-synchronized FITS-header completion.
* **Recommended pattern for static `CGO_ENABLED=0` SkyStacker:** **do not embed hardware control**; use an external daemon/sidecar polling REST/WebSocket and writing a **declarative mapping file** (CSV/JSON) of Alpaca properties → FITS keywords (`TELESCOP`, coordinates, environmental telemetry) before header sealing. Serve JPEG previews in memory over HTTP. Interactive observatory control is out of scope; **metadata ingestion is not**.
* **IVOA ProvenanceDM** (validated April 2020, over W3C PROV-DM): core **Entity, Activity, Agent**; extensions **ActivityDescription** (method/algorithm/software version) and **ActivityConfiguration** (execution parameters; typed `Parameter.valueType`) linked by **WasConfiguredBy**; serialize XML/JSON/SVG (PROV) or VOTable. Emit a structured provenance graph under `skystacker_results/` capturing binary version, plate-solve tolerances, tile/rejection/stretch params and exact catalogue/cache version (Gaia DR3 vs DR4, ASTAP D50/V50). Current reports are text/JSON-adjacent; provenance emission is a future enhancement.
* **Gaia TAP/DataLink client constraints** apply to any TAP/DataLink client (§15.2); 429/queue-full/timeout never panic or drop cached tiles. Local persistent HEALPix cache fallback is essential during the **25 Nov–2 Dec 2026** blackout.

---

## 17. Consolidated roadmap

Statuses reflect §1/§17 appendix. Effort S/M/L/XL/∞ per the survey and [README.md](README.md) tiers.

**Delivered (2026-11-17 unless noted)**
1. **Finite-sample MAD correction for small N** — `finiteMADScale` table + tests (S).
2. **Go 1.27.4 toolchain upgrade** — both modules `go 1.27` + pinned toolchain, CI/release pinned, full matrix green, BENCHMARKS re-baselined (S).
3. **Single-pass sigma-clip accumulation** — fused, bit-identical, median/MAD retained (S).
4. **External AI CLI sidecars** — `--external-bge/-denoise/-deconv/-starless`, whitespace-split argv (no shell), private 32-bit FITS temp dir, WCS round-trip, linear-stage guard + override, timeouts, cleanup, self-executing mock test (M).
5. **`--resample-sip`** order-2 SIP in stacking resampler (M).
6. **Pre-solve gradient detrending** — `detectStarsWithDetrend` retries on a scratch copy with coarse percentile background when <20 stars; `testDetrendStarDetection` (S).
7. **Dark-series QA** — reject >3σ outliers by median/hot-pixel fraction when ≥3 darks, MAD fallback for degenerate MAD; `testCalibrationQA` (S).
8. **Gaia TAP 429 handling** — capped `Retry-After`, 1 req/s pacing, MAXREC=2000; `testGaiaQueryTile429Retry` (S).
9. **Reference-frame HFR reporting** in log and `report.txt`/`report.json` (post-processing review).
10. **`--wavesharp`/`--wave-gains`**, native pure-Go CR2 decoding (default), `--config`, `--resume`, narrowband, annotation, deconvolution, `--starless` layers, `--bpm-auto`, SER input (v0.6.0/v0.6.1 lineage; details in [07](07-native-cr2-decoding.md)).

**Open — ordered by ascending effort**
1. **CR2 bit-exact vs dcraw** (M–L, ~2–4 h): `cr2_slice[]` hypothesis refuted by dcraw v9.28 source check; compare raw Bayer planes after black/white normalization → [07](07-native-cr2-decoding.md).
2. **CFA/Bayer drizzle scale=1.0 square kernel** (`--bayer-drizzle`, L, ~4–8 h): flux conservation within 0.1%, coverage weights, N<15 / integer-dither warnings, synthetic dither tests at N=20 (§7).
3. **FITS Rice + BigTIFF + XISF Rev 2 checksums** (L, ~4–6 h): pure-Go Golomb-Rice codec, 64-bit offsets, strict sample mapping + corrupt-block rejection, SHA-256/512 (SHA-3 only if verified).
4. **Spatially gridded PSF for Richardson–Lucy** (`--deconvolve-grid`, M): bilinear local PSFs over 3×3/4×4 grid, linear-stage ordering; accept if corner residuals improve and no ringing.
5. **Frame-quality weighting refinement + PSF-derived FWHM/eccentricity**, Gaussian/Moffat fits (`testPSFFit` exists), `--hfr-quality` expansion.
6. **Stronger background modeling** — multiscale/sample-based extraction, wavelet scales (beyond `--bg-multiscale` 20th-percentile grid), LNC extension.
7. **Improved rejection & maps** — richer per-pixel metadata, rejection/weight maps beyond current JPGs.
8. **Modularization** — `fitsio/` next candidate module after goPureRaw → [05](05-toolchain-and-modularity.md).
9. **Native format improvements** — reduce external-tool dependency; XISF/metadata parity.
10. **Advanced registration** — per-frame astrometric reprojection, higher-order transforms, field-rotation handling; catalogue detail → [02](02-astrometry-and-identification.md).
11. **Photometric color calibration (PCC/SPCC)** — Gaia DR3 SEDs + sensor/filter response.
12. **Advanced modes** — multi-night integration, mosaics (WCS-SIP stitching, feathered blending, seam elimination), moving-object dual registration extension.
13. **Performance** — tile streaming, pointer-wrapped pools, optional SIMD/GPU HAL, mmap.
14. **Provenance/telemetry** — ProvenanceDM emission, Alpaca/REST metadata adapter, structured JSON telemetry, `--post-preset`, REST API (Tier 3/4).
15. **Repo hygiene** — fix `--calib-root`/`detectCalibrationSubdirs` case sensitivity for lowercase Linux folders (§14).

**Explicit non-goals / rejected:** GUI; native neural-network inference (embedded ONNX/GGUF/WASM; 284–456 MB weights); `simd/archsimd` (amd64-only GOEXPERIMENT); 2× mono/CFA upscale drizzle as a general mode; GZIP-compressed FITS (ASTAP chose Rice); denoising/deconvolving individual lights or stretched masters (restoration locked to linear integrated masters); plugin architecture; REST API today; true multi-frame drizzle, mosaics, multi-night integration, PSF-fitting photometry, PCC/SPCC calibration, SIMD/GPU paths — all current non-goals per the Gemini prompt baseline.

---

## 18. Status appendix — CURRENT and historical snapshots

### 18.1 CURRENT (2026-11-18)

* **Version:** v0.6.1 (binary `--version` prints 0.6.1); tag exists, main advanced beyond it.
* **Self-tests:** **219 registered** (`./skystacker --selftest`); plus a Go 1.26 `testing/synctest` rate-limiter test via `go test ./...`.
* **Toolchain:** **Go 1.27.4** at `/home/jips/.local/go1.27.4/bin`; `go 1.27` + `toolchain go1.27.4` in both modules; `go.work` (gitignored) links both; CI/release pinned; `go fix` modernizers applied; `main_test.go` uses `testing/synctest`. Full matrix green: gofmt, vet, build, 247 self-tests, `go test ./...`, goPureRaw CGO=0 + race, race build, six static cross-build targets.
* **Roadmap pass 2026-11-18:** encoded/EOTF self-flat report note; extended Go benchmarks (tile reduction, native CR2 decode, mean/CV); `--xisf-rev2` (SHA3-256 checksum, strict sample validation, corrupt-block rejection); `--bigtiff` (64-bit offsets); multi-catalog annotation (`--annotation-catalogs`, elliptical apertures, `catalog_matches.json`, SNR>5 gate); `--deconvolve-grid` (HFR-derived local PSF, 3×3/4×4 blended tiles); `--provenance` (ProvenanceDM-style entity/activity/agent graph); portable `goDiagnostics/` module with `--diagnose` (see item 18 of `ROADMAP.md` and `goDiagnostics/README.md`). Blocked/deferred: ASTAP adapter (none in code), FITS Rice + `fitsio/` tag, CR2 oracle dumps, per-frame reprojection, CFA drizzle, CR3, PCC/SPCC, observatory sidecar.
* **`--resample-sip` implemented:** order-2 SIP from `plate_solution.json` applied in stacking resampler (undistort reference → similarity/affine warp → redistort source → bilinear sample); `testResampleSIP` verifies distort/undistort round-trip, composed mapping, synthetic star relocation, no-op with nil WCS, flag parsing. Shared-distortion assumption; per-frame reprojection open.
* **Sigma-clip fused:** final mean folded into last clip pass, survivor list carried, tile sums gathered in one pass; bit-identical to two-pass reference; median/MAD deliberately retained.
* **External AI sidecars implemented:** `--external-bge`, `--external-denoise`, `--external-deconv`, `--external-starless`; private 32-bit float FITS temp dir (no shell), WCS round-trip, linear-stage guard + override, `--external-timeout`, cleanup, self-executing mock test (`--internal-ai-sidecar-mock`). No ML runtime/weights embedded; tools user-supplied/licensed → [06](06-external-ai-sidecars.md).
* **goPureRaw v0.1.1:** nested Go module `github.com/computerphysicslab/skystacker/goPureRaw`, imported at v0.1.1; v0.1.0 had a CR2 `Probe` offset bug (signature at bytes 8–9, not 4–5); duplicated `pr*` decoder removed from `main.go`; E2E 1-light CR2 stack 7.5 s; single stdlib-only executable + one first-party module → [07](07-native-cr2-decoding.md).
* **Other implemented:** finite-sample MAD, dark QA, Gaia 429 handling, pre-solve detrending, native CR2 decode (default), FITS/XISF writers, JSON report, config/resume, deconvolution, annotation, narrowband, `--starless`, `--bpm-auto`, SER input, grid self-flat, TAN WCS + HEALPix + TAP, reference-frame crop, comet pixel/RA-Dec/ephemeris modes.
* **Open items:** FITS Rice compression, BigTIFF, XISF Rev 2, true/CFA drizzle, spatially gridded PSF, per-frame reprojection, embedded ML rejected. Crawler/source-discovery runs → [08](08-crawler-source-discovery.md).

### 18.2 HISTORICAL snapshots (tagged)

* **[HISTORICAL — v0.4.1]** (file `current-implementation-v0.4.1.md`): single-file Go CLI (JPG/JPEG/CR2/CR3/FITS), optional registration/calibration/stacking/post, output `<input>/0_skystacker_results`, box-blur-radius-12 detection, bilinear resample, similarity-only registration, mean/median/sigma accumulators with `GOGC=50`/`--force`, 8-stage post chain, external dcraw+ImageMagick, `report.txt`. Archived snapshot — not current.
* **[HISTORICAL — v0.6.0]** (2026-11-15/16): native FITS read/debayer, Gaia TAP + HEALPix nested queries, TAN WCS (`plate_solution.json`), star/transform cache v2, calibration masters, self-flat v1 + grid Phase 1, SER input, pixel-mode comet stacking; SIP fit but **not** applied; catalog annotation/drizzle/XISF/RA-Dec comet ephemeris still research; **177 self-tests** (deep-dive §1 said 181, v0.6.1 bump 182); `assets/testdata/` fixtures. "0.6.0 baseline" plans in deep dive §5.3 are normative, not implemented.
* **[HISTORICAL — survey baseline]** Gemini prompt ground truth: single `main.go`, **203 self-tests**, Go 1.20.3, explicit non-goals (GUI, native neural inference, true drizzle, mosaics, multi-night, PSF photometry, PCC/SPCC, SIMD/GPU, plugins, REST API; SIP in WCS helpers not resampler).
* **[SURVEY PROPOSAL, NOT IMPLEMENTED]** RC-Astro/GraXpert sidecar orchestration, toolchain-to-1.26 (superseded by 1.27.4), finite-sample MAD (implemented), single-pass clip (implemented), `--bayer-drizzle`, `--deconvolve-grid`, FITS Rice, BigTIFF, XISF Rev 2.

---

## Sources absorbed

* `research/01-architecture-and-core-engine/core-engine-technical-specification.md` — 9-step/5-step workflows, concurrency/tile/Swiss Tables math, calibration/quality/SNR/stacking tables, registration/SIP, demosaic, 6-stage post, diagnostics, formats, CLI table, comet/photometry extensions, Phase 1–4 roadmap.
* `research/01-architecture-and-core-engine/cli-integration-and-observatory-automation.md` — 7-stage headless pipeline + artifact matrix, OS/deployment matrix, ASTAP flag table, return encodings/exit codes, output schema/INI, 2026-11-16 append.
* `research/01-architecture-and-core-engine/current-implementation-v0.4.1.md` — **[HISTORICAL v0.4.1]** audit of discovery, calibration, registration, stacking, 8-stage post, design choices, limitations.
* `research/RESEARCH_SYNTHESIS.md` — status/design principles, math foundations, 10-item engineering roadmap, v0.6.0 transfer status, BXT/NXT/SXT pointer, Gemini outcome summary.
* `research/GEMINI_DEEP_RESEARCH_PROMPT.md` — program ground truth, benchmark context, KB inventory, research mission, non-goals.
* `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — state-of-the-art survey: executive findings, domain findings A–G, feasibility matrix, 8 proposals, rejected items, full source URL list; append-only SIP and sigma-clip implementation statuses.
* `research/05-research-deep-dive-2026-11-16.md` (architecture-relevant §§1, 5, 6–14) — ASTAP interop audit/corrected encodings/32-bit HFD ceiling, memory/rejection RAM model and 64×64 sub-tiles, Swiss Tables/Go notes, TAN–SIP/AP-BP, stretch math, Gaia v3.15/DR4, Alpaca/REST telemetry, ProvenanceDM, BXT/NXT/SXT analysis, detrend/dark QA/Gaia 429, post-processing review, drizzle/CFA decision, Orion validation, correction note.

Current-state appendix cross-checked against `NEXT_SESSION_PROMPT.md`, `TECHNICAL_SPEC.md`, `BENCHMARKS.md`, `CHANGELOG.md` and `README.md` (no content duplicated from the canonical cross-reference files listed in the header).

A cycle-6 curated corpus of stacking/drizzle/rejection sources is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §9; its quantitative findings are consolidated there.

## DeepSkyStacker kernel reference (2026-11-18, append-only)

Three pending notes (`dss.md`, `dss2.md`, `dss3.md`) describing the DSS source
were translated/reviewed and consolidated here; the originals were removed.

**Architecture.** BSD-3-Clause, Qt, Windows/Linux/macOS; `DeepSkyStackerKernel`
holds `CRegisterEngine`, `CStackingEngine`, `MatchingStars.cpp`, `DSSTools.h`,
`BackgroundCalibration.*` and `AHDDemosaicing.*`. Pipeline: load → optional
calibration → star detection/registration → background calibration → stacking
→ post-processing (MTF/autostretch). Registration/intermediate results are
cached in `.stackinfo.txt`.

**Star detection and registration.** Stars are near-circular, luminance falls
regularly in all directions, radius ≤~50 px; Gaussian fitting refines centres;
threshold ~10% of maximum; aim for ~20 to a few hundred stars; quality score
favours round, well-sized stars; ≥8 common stars required. Registration uses
triangle distances filtered by side-ratio (~0.9), a voting grid, then
least-squares fitting (`CBilinearParameters`). Models: linear, bilinear,
bisquared (≥~25 pairs), bicubic (≥~40 pairs), with corner locking and residual
outlier rejection.

**Demosaicing.** Bilinear, AHD (adaptive homogeneity-directed), super-pixel
(2×2 block → one pixel, 4× less resolution) and Bayer Drizzle (sub-pixel shift
reconstruction); super-pixel and Bayer Drizzle retain more detail for stacking.

**Background models.** None, Offset (median difference), Linear (piecewise
below/above median), Rational; per-channel or RGB-neutral, with histogram
parameters.

| DSS stacking method | Behaviour |
|---|---|
| Average | Arithmetic mean |
| Median | Median; master frames/outliers |
| Maximum | Highest value; defect visualisation |
| Kappa-Sigma | Iterative hard rejection outside μ±κσ, then mean; ≥~10 frames |
| Median Kappa-Sigma | Rejected values replaced by median; masters |
| Auto Adaptive Weighted Average | Soft re-weighting (Stetson 1989); 3–5 iterations |
| Entropy Weighted Average | Entropy weighting for mixed exposures (HDR) |

Kappa-Sigma uses κ≈1.5–3 (commonly 2–3), 2–5 passes, and ≥10 frames for a
stable σ; `CDynamicStats` avoids full recomputation. Median Kappa-Sigma swaps
buffers and substitutes the median. Auto Adaptive Weighted Average starts from
the mean and iterates `w_i = 1/(1+(|x_i−μ|/σ)²)`, with the constant `1/σ²`
omitted because normalisation cancels it; constant data (σ≤5) returns the
plain mean. DSS also supports dark entropy scaling, hot pixels >median+~16σ
with bad-column correction, comet star-freeze dual stacking, Delaunay tools,
SIMD (SSE2/AVX) and OpenMP. Calibration specifics are in
[04](04-calibration-and-self-flat.md); tool positioning is in `COMPARISON.md`.

A cycle-7 curated corpus of drizzle kernels, DSS theory and background models is indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §10; its quantitative findings are consolidated there.

## Standalone image-processing mode (2026-11-18, append-only)

The user requested a CLI mode that treats existing JPG/PNG/FITS images as
first-class inputs without running the stacking pipeline. Analysis: nearly all
building blocks already exist as pure functions over `*frame`, so the new work
is flag plumbing, input loading, operation dispatch and provenance output —
not new algorithms.

**Proposed contract.** `--process-image PATH[,PATH...]` (or repeated flag)
switches to image mode; `--image-op OP` selects the operation; `--image-output
DIR` defaults to `<cwd>/image_processing`. Inputs are decoded with
`decodeImageFile` (JPG/PNG/FITS/RAW/SER), never modified, and every output is
written as `_processed.f32raw` + JPEG preview with the source basename in the
name and in `report.txt`/`report.json`.

**Operation families.**

- *Single-image post-processing*: `blur` (`gaussianBlur`), `sharpen`
  (`multiScaleSharpen`, `unsharpMask`), `background` (`removeBackground`,
  `multiscaleBackground`, `--bg-multiscale`), `stretch` (`autoStretch`, MTF),
  `denoise` (`denoiseFrame`, `waveletDenoise`), `deconvolve`
  (`richardsonLucy`, `deconvolveWiener`) and `starless` (`removeStarsForOptions`).
  These reuse the existing post-processing functions unchanged.
- *Multi-image arithmetic*: `sum`, `subtract`, `average`, `median`, `min`,
  `max` over equal-size frames, with per-pixel valid masks and a report of
  dimensions/normalisation. Existing `accumulator` and `reduceTilesStack`
  helpers cover most cases.
- *Pairwise measurement*: `measure-transform` uses `detectStars` +
  `matchStarsToReference` / `estimateSimilarity` to report dx, dy, rotation,
  scale and inlier RMS; `measure-quality` reports `computeFrameQuality`,
  `computeAdvancedQuality`, median HFR/HFD, eccentricity and background sigma
  per image, reusing `ditherDiagnostic` and `computeTiltDiagnostic` where
  applicable.
- *Astrometric products*: `sky-map` and `survey-cutout` are already implemented
  for the stacked frame in `sky_products.go`; in image mode they must accept an
  explicit input path and, when needed, run `--plate-solve` (seeded by
  `--plate-center` or FITS RA/DEC) before projecting Gaia sources or calling
  CDS hips2fits with the image's exact width/height/FOV/rotation.

**Constraints.** No external runtime dependencies beyond the existing QR module;
no mutation of source files; batch through `--batch`; opt-in network only for
plate solving and survey cutouts; output naming must avoid collisions when the
same operation is run twice. Feasibility is high because every listed operation
already has a tested implementation; the roadmap tracks the plumbing and
integration separately by effort.

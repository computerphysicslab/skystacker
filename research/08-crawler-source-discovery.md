# 08 — Crawler-based source discovery (canonical research)

**Status:** Canonical, deduplicated English consolidation of the `goCrawler` discovery runs.
**Current date:** 2026-11-18. The three absorbed incremental reports are dated **2026-11-17**.
**This file owns:** the crawler methodology, run metrics, `robots.txt` enforcement, fail-open hosts,
CPU/concurrency observations, the master source table (all cycles), and the cycle-5 curated excerpt
corpus. It does **not** duplicate the technical conclusions those sources were used to support.

**Canonical cross-links (do not duplicate their content here):**
[`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) (stacking, rejection,
resampling, CFA/Bayer drizzle decision), [`02-astrometry-and-identification.md`](02-astrometry-and-identification.md)
(plate solving, SIP/WCS, Gaia, ASTAP, catalogs), [`03-reference-and-literature.md`](03-reference-and-literature.md)
(master bibliography, glossary — this file is its **C8** entry),
[`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) (master frames, WBPP, self-flat;
§14 summarises the cycle-5 calibration excerpts),
[`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md) (Go version, Swiss Tables, GC,
worker-pool context), [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) (StarNet/AI sidecars,
starless prior art), [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md),
[`README.md`](README.md) (project entry point).

> **Where the technical conclusions live.** The excerpts in §6 are the *retrieved text* kept by the
> crawler. Interpretations, implementation status, flag decisions and final conclusions are owned by
> the thematic files above — e.g. drizzle/CFA decisions in [01 §7](01-architecture-and-core-engine.md),
> ASTAP/Astrometry.net/tetra3rs and Siril plate-solving conclusions in
> [02](02-astrometry-and-identification.md), and PixInsight master-frame/WBPP conclusions in
> [04 §14](04-calibration-and-self-flat.md). They are linked, not restated here.

---

## 1. Methodology (single consolidated description)

All incremental runs used the same profile and pipeline:

- **Tool:** `goCrawler`, with profile `profiles/skystacker-research.yaml`. The first run is recorded
  as **Go 1.20**; the cycle-4 run as **Go 1.26.11** (the toolchain upgrade is documented in
  [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md)).
- **Caching / services:** local **SQLite cache** enabled; **Redis was not running / not used** in any run.
- **Seed extraction:** seed URLs were taken from the **140 external references** already cited in the
  research folder (`03-reference-and-literature/source-catalog-and-microarchitecture.md` and the
  synthesis documents). Domains banned in `crawler.yaml` (`github.com`, `doi.org`, `google.com`, …)
  were excluded, as were malformed suffixes and tracking query strings. This produced **21 seeds**.
- **Seed focus:** calibration/stacking (Siril, DeepSkyStacker, PixInsight, Astro Pixel Processor);
  astrometry/plate solving (ASTAP, SCAMP, Astrometry.net, Gaia DR4); standards and data (WCS/FITS,
  IVOA, SIMBAD, Aladin/HiPS, Minor Planet Center); pixel-level techniques (drizzle in Bayer and CFA
  form, photometry, source detection).
- **Filtering:** English paragraphs only; stopword, length, relevance and uniqueness filters; a
  **`scoreThreshold` of 15**.
- **`robots.txt` policy:** the `robotspolicy` package fetches and caches one `robots.txt` per host
  before the first online request (see §2).
- **Outputs:** downloaded pages enter `goCrawler/logs/corpusCuratedText.log` (the curated filtered
  text); **ranking scores** come from `output/ranking-URL.csv`; the final topic dump is
  `corpusNoEngFrequencies.txt`; cycle-4 full session output is
  `goCrawler/logs/Skystacker-research-2026-11-17-Z21-48-15.txt`.
- **Cycle 5** is not a new crawl configuration but the curated-text extraction pass over the
  accumulated corpus: scores are the `output/ranking-URL.csv` ranks attached to the retrieved text.

---

## 2. `robots.txt` policy and exact blocked URLs

The crawler behaves as a well-behaved client and does not bypass `robots.txt`. In the first run the
new `robotspolicy` package fetched/cached one `robots.txt` per host and **blocked 8 URLs**:

**IVOA standard documents**
- `https://ivoa.net/documents/MOC/`
- `https://www.ivoa.net/documents/VOTable/`
- `https://www.ivoa.net/documents/ConeSearch/`

**Astrometry.net submission UI**
- `http://nova.astrometry.net/`
- `https://nova.astrometry.net/`

**Documentation / other**
- `https://docs.astropy.org`
- `http://tdc-www.harvard.edu/software/wcstools.html` (WCStools)
- `http://flickr.com/groups/astrometry` (Flickr astrometry group)

**Fail-open hosts.** Missing or unreachable `robots.txt` files were treated as **allow** after logging
the error. This happened for:
- `iau-commission5.wikispaces.com` — dead DNS;
- `cdsarc.cds.unistra.fr` and `tapvizier.cds.unistra.fr` — timeouts;
- `www.stsci.edu` — unreachable policy file.

**Implication:** blocked sources must be read manually or fetched from an alternative endpoint. The
separate cycle-4 run reported **15 `robots.txt` blocks** (a different run against a larger link
pool); both figures are preserved as run-specific observations.

---

## 3. Runtime, CPU and time-budget observations

- The host has **8 cores**, but the crawler is **a single sequential loop** and does not use them.
- Controlled measurement: **24.78 s user + 0.88 s system CPU over 106 s wall time = 24 % of a single
  core** (network-bound). `GOMAXPROCS` defaults to 8, but no work is parallelised; the only extra
  goroutine is the optional external indexer, disabled by default.
- The first run hit the **240 s cap** while the link pool was still non-empty — the **18 retained
  documents are therefore a partial result**.
- Concurrency would require the roadmap's **worker-pool** item plus **per-domain rate limiting**
  (see §8).

---

## 4. Run summaries (all cycles)

| Metric | Cycle 1 (initial discovery) | Cycle 4 (new sources) | Cycle 5 (curated text extraction) |
|---|---|---|---|
| Crawl wall time | hit **240 s** cap | **338 s** | n/a (extraction pass) |
| Links processed | not reported | **217** | n/a |
| Curated documents | **18** retained | **52** (**1602-line** corpus) | 10 scored excerpt sources (§6) |
| New vs earlier corpora | — | **51 of 52** new | — |
| Cache hits | all downloads via SQLite cache | **144** (**85.2 %** of **169** attempts) | — |
| Online downloads | — | **25** (of which **6** passed all filters) | — |
| `robots.txt` blocks | **8** exact URLs (§2) | **15** | — |
| Download errors | — | **48** | — |
| Score threshold | **15** | **15** | scores from `output/ranking-URL.csv` |

**Cycle-1 relevance assessment:** the 18 documents cover the three core research domains —
registration (ASTAP, SCAMP, Astrometry.net), stacking/calibration (DSS, Siril, PixInsight, APP) and
astronomical identification (Gaia DR4, WCS, photometry). The drizzle cluster (Siril, CFA PixInsight,
APP Bayer, STScI original paper) directly supports the CFA/Bayer drizzle work in the external
deep-research outcome (absorbed into [01 §7](01-architecture-and-core-engine.md) and
[04 §14](04-calibration-and-self-flat.md)).

---

## 5. Master source table (all cycles, deduplicated)

One row per (source, cycle) with its own score, so that score drift between runs is preserved.
Characters shown in parentheses in the *Score* field are the curated-text lengths reported for
cycle 1. Sources with no reported score are marked `—`. “Where it was used” names the canonical
file/section that consumes the source; full technical conclusions are **not** repeated here.

| Source | Cycle | Score | Theme | Where it was used |
|---|---:|---|---|---|
| `https://photutils.readthedocs.io/` | 1 | 71.71 (69 ch) | photometry, source detection | [02](02-astrometry-and-identification.md) (photometry/SIMBAD line) |
| `https://siril.readthedocs.io/en/latest/preprocessing/drizzle.html` | 1 | 47.63 (3213 ch) | drizzle | [01 §7](01-architecture-and-core-engine.md), [04 §14](04-calibration-and-self-flat.md) |
| `https://siril.readthedocs.io/en/latest/preprocessing/drizzle.html` | 5 | 49.80 | drizzle (curated excerpt §6.3) | [01 §7](01-architecture-and-core-engine.md), [04 §14](04-calibration-and-self-flat.md) |
| `http://deepskystacker.free.fr/english/technical.htm` | 1 | 43.63 (1181 ch) | stacking / calibration | [01](01-architecture-and-core-engine.md) |
| `https://mypetstars.com/tutorials/pixinsight/processes/drizzleintegration/cfa-drizzle` | 1 | 40.11 (2243 ch) | CFA drizzle | [01 §7](01-architecture-and-core-engine.md), [04 §14](04-calibration-and-self-flat.md) |
| `https://www.hnsky.org/astap.htm` (ASTAP) | 1 | 36.53 (17269 ch) | plate solving | [02](02-astrometry-and-identification.md) |
| `https://www.hnsky.org/astap.htm` | 4 | 65.8 | ASTAP CLI behaviour | [02](02-astrometry-and-identification.md) |
| `https://www.hnsky.org/astap.htm` | 5 | 63.13 | ASTAP databases (curated excerpt §6.8) | [02](02-astrometry-and-identification.md) |
| `https://www.stsci.edu/~fruchter/dither/drizzle.html` (original drizzle paper) | 1 | 36.06 (1729 ch) | drizzle | [01 §7](01-architecture-and-core-engine.md), [03](03-reference-and-literature.md) |
| `http://astrometry.net/doc/readme.html` | 1 | 35.32 (1354 ch) | astrometry | [02](02-astrometry-and-identification.md) |
| `https://reproject.readthedocs.io` | 1 | 34.26 (212 ch) | WCS reprojection | [01](01-architecture-and-core-engine.md), [02](02-astrometry-and-identification.md) |
| `https://siril.readthedocs.io/en/stable/astrometry/annotations.html` | 1 | 32.91 (1116 ch) | catalog annotation | [02](02-astrometry-and-identification.md) |
| `https://www.astropixelprocessor.com/community/tutorials-workflows/bayer-drizzle/` | 1 | 26.98 (1373 ch) | Bayer drizzle (APP) | [01 §7](01-architecture-and-core-engine.md), [04 §14](04-calibration-and-self-flat.md) |
| `https://www.astromatic.net/software/scamp/` | 1 | 23.90 (212 ch) | registration (SCAMP) | [02](02-astrometry-and-identification.md) |
| `http://www.minorplanet.info/ObsGuides/Misc/photometryguide.htm` | 1 | 22.00 (9108 ch) | MPC photometry | [02](02-astrometry-and-identification.md), [03](03-reference-and-literature.md) |
| `https://astrometry.net/` | 1 | — | astrometry | [02](02-astrometry-and-identification.md) |
| `https://astrometry.net/use.html` | 1 | — | astrometry | [02](02-astrometry-and-identification.md) |
| `https://www.cosmos.esa.int/web/gaia/dr4` | 1 | — | Gaia DR4 | [02](02-astrometry-and-identification.md) |
| `https://astroquery.readthedocs.io` | 1 | — | catalog access | [02](02-astrometry-and-identification.md), [03](03-reference-and-literature.md) |
| `https://www.ucolick.org/~sla/fits/` | 1 | — | FITS/WCS | [01](01-architecture-and-core-engine.md), [03](03-reference-and-literature.md) |
| `https://www.atnf.csiro.au/people/mcalabre/WCS/index.html` | 1 | — | WCS | [02](02-astrometry-and-identification.md) |
| `https://tetra3rs.dev/tutorials/basic-solve/` | 4 | — | blind solving | [02](02-astrometry-and-identification.md) |
| `https://tetra3rs.dev/tutorials/basic-solve/` | 5 | 51.87 | blind solving / TESS (curated excerpt §6.5) | [02](02-astrometry-and-identification.md) |
| `https://markus-enzweiler.de/software/starstax/` | 5 | 23.95 | stack-based star reduction | [01](01-architecture-and-core-engine.md), [06](06-external-ai-sidecars.md) (prior art) |
| `https://starnetastro.com/` | 4 | — | neural star removal | [06-external-ai-sidecars.md](06-external-ai-sidecars.md) (prior art; embedded ML out of scope) |
| `https://www.cloudynights.com/forums/topic/808556-starnet-v2/` | 4 | — | neural star removal | [06-external-ai-sidecars.md](06-external-ai-sidecars.md) (prior art) |
| `http://knro.blogspot.com/2017/03/how-to-get-100-million-stars-in-kstars.html` | 4 | — | KStars catalog scale | [02](02-astrometry-and-identification.md), [03](03-reference-and-literature.md) |
| `https://knro.blogspot.com/2026/10/kstars-385-released.html` | 4 | — | KStars 2026 status | [03](03-reference-and-literature.md) |
| `https://starlink.eao.hawaii.edu/devdocs/sun139.htx/sun139se21.html` | 4 | — | Starlink AST docs | [02](02-astrometry-and-identification.md), [03](03-reference-and-literature.md) |
| `https://measurethepixels.com/handbook/stacking-integration-weighting-rejection.html` | 4 | 72.4 | rejection / weighting | [01](01-architecture-and-core-engine.md) (stacking statistics) |
| `https://measurethepixels.com/handbook/stacking-integration-weighting-rejection.html` | 5 | 72.59 | NOVA stack paths (curated excerpt §6.1) | [01](01-architecture-and-core-engine.md) |
| `https://miklosoft.se/learn/path/preprocessing-in-depth/` | 4 | 70.4 | WBPP preprocessing | [04 §14](04-calibration-and-self-flat.md) |
| `https://miklosoft.se/learn/path/preprocessing-in-depth/` | 5 | 70.45 | WBPP stages (curated excerpt §6.2) | [04 §14](04-calibration-and-self-flat.md) |
| `https://prose.readthedocs.io/.../catalogs.html` | 4 | — | catalog handling | [02](02-astrometry-and-identification.md), [03](03-reference-and-literature.md) |
| `https://www.pixinsight.com/tutorials/master-frames/` | 5 | 50.50 | master frames (curated excerpt §6.4) | [04 §14](04-calibration-and-self-flat.md) |
| `https://astrometry.net/doc/code.html` | 5 | 49.26 | `axy` internals (curated excerpt §6.7) | [02](02-astrometry-and-identification.md) |
| `https://astrometry.net/doc/build-index.html` | 5 | 34.59 | index construction (curated excerpt §6.6) | [02](02-astrometry-and-identification.md) |
| `https://siril.readthedocs.io/en/latest/astrometry/platesolving.html` | 5 | 37.76 | plate solving flags (curated excerpt §6.9) | [02](02-astrometry-and-identification.md) |
| `https://opticalmechanics.com/...calibration-frames...` | 4 | — | calibration frames | [04](04-calibration-and-self-flat.md) |

**Discarded / low relevance (cycle 4):** flathub, bluesky and SourceForge mirrors were downloaded but
did not pass content filters (or are packaging noise); the crawler correctly discarded most of them.
**Crawler-enriched seed follow-ups** (cycle 4): `tetra3rs.dev/tutorials/basic-solve/`,
`measurethepixels.com/handbook/...` and `knro.blogspot.com/2017/03/...` are candidates for the
reference catalog in `03-reference-and-literature/source-catalog-and-microarchitecture.md` at its
next revision.

---

## 6. Cycle-5 curated excerpt corpus

Source: `goCrawler/logs/corpusCuratedText.log` — the **actual filtered English text** kept by the
crawler, not just URLs/keywords; scores from `output/ranking-URL.csv`. Each block below preserves the
technical content of the retrieved excerpt (numbers, flags, versions, catalog names). The
*conclusions* drawn from these texts are owned by the linked thematic files (see note at top).

### 6.1 MeasureThePixels — stacking / integration / rejection — score **72.59**
`https://measurethepixels.com/handbook/stacking-integration-weighting-rejection.html`

Key retrieved points:
- **NOVA exposes four meaningful stack paths.** Siril is the reference strategy: Siril registration,
  **Winsorized-rejection mean**, additive-scale normalization, background-noise weighting, output
  normalization. **ImageMM** uses Siril-registered frames for multi-frame deconvolution — currently
  **20 iterations, kappa 2.0, PerChannel**. The historically named `pixinsight_wbpp` path is a hybrid:
  Siril registration + PixInsight ImageIntegration with Winsorized rejection, additive scaling and
  requested SNR weighting. `pixinsight_register` changes registration, grid and integration together.
  Requested behavior can fall back or change — comparisons must record the **executed treatment**, not
  just the requested engine name.
- **Frame-level culling ≠ pixel-level rejection:** subframe inspection decides whether a whole
  exposure enters the stack; rejection decides whether an individual aligned pixel sample is an
  outlier during integration.
- Ideal random-noise improvement ≈ **√(comparable exposure count)**, but gradients, seeing, correlated
  noise, resampling and unequal frames prevent this from being a guarantee.
- An engine comparison that also changes frame selection, registration grid, drizzle, framing,
  normalization or support is a comparison, not proof that one pixel-combination algorithm is
  intrinsically better.
- Tool-equivalence table: `nova` (exact replay; reproducible linear integration with manifest,
  executed treatment, evidence), `nova-stacking-source`/`pixinsight` (functional alternative with
  explicit registration/parameter differences), `nova-stacking-source`/`siril` (same engine, adapted
  host; robust weighted reference using `siril-stacking-1.4.4`), `nova-stacking-source`/`saspro`
  (restored multi-frame integration; `saspro-imagemm-vendor`).
- → stacking-statistics conclusions: [01](01-architecture-and-core-engine.md).

### 6.2 MikloSoft — PixInsight WBPP preprocessing in depth — score **70.45**
`https://miklosoft.se/learn/path/preprocessing-in-depth/`

Retrieved stage list (a real-run walkthrough from raw folders to one master):
1. files, grouping keywords and global options (masters/cache, multi-night and mosaic keywords,
   exposure tolerance);
2. calibration, **cosmetic correction** and debayering (master bias/darks/flats, Calibration tab,
   **dark optimization**, **output pedestal**, automatic/template cosmetic correction);
3. measuring, weighting and frame selection (**PSF Signal Weight** and other options, minimum
   weight, interactive selection);
4. registration and local normalization (distortion correction, normalization reference);
5. integration and pixel rejection (percentile, sigma, Winsorized, linear fit, ESD, RCR, Auto…);
6. drizzle, fast integration, autocrop and plate solving (combined/separate colour channels, drizzle
   settings);
7. pipeline, diagnostics, logs and troubleshooting (Pipeline tab, Diagnostics, Execution Monitor,
   output folders, log reading);
8. the same preprocessing in **Siril, Deep Sky Stacker and APP** (scripts, registration/stacking with
   rejection, DSS modes, APP normalisation and mosaics).
- → calibration/master-frame conclusions: [04 §14](04-calibration-and-self-flat.md).

### 6.3 Siril — Drizzle preprocessing — score **49.80**
`https://siril.readthedocs.io/en/latest/preprocessing/drizzle.html`

- Variable-pixel linear reconstruction ("Drizzle") developed for HST by **Andy Fruchter and Richard
  Hook** (Fruchter & Hook 1997), initially to combine dithered images of the **Hubble Deep Field
  North (HDF-N)**. It behaves as a continuous set of linear functions between **interlacing** (optimum
  linear combination) and **shift-and-add**, often improving resolution and reducing correlated noise.
- Controlled by the **pixel fraction** (how much input pixels are shrunk before mapping to the output
  plane): **0 = pure interlacing; 1 = shift-and-add**.
- Standard registration maps a **3×3 homography encoding an 8-DOF linear transform** into the reference
  frame; interpolation smears the PSF (especially upscaling) and can cause artefacts, which Siril
  limits with a clamping mechanism.
- Drizzle turns each pixel into a "droplet" mapped through a grid pattern onto the output frame; a
  scaled-up output grid with smaller droplets can improve resolution **if the imaging train is
  undersampled** — otherwise (correct sampling) drizzle cannot help.
- → flux-conservation/kernel/coverage conclusions and implementation status:
  [01 §7](01-architecture-and-core-engine.md) and [04 §14](04-calibration-and-self-flat.md).

### 6.4 PixInsight — Master frames — score **50.50**
`https://www.pixinsight.com/tutorials/master-frames/`

- Since **version 1.6**, PixInsight includes the standard **ImageCalibration (IC)** module; IC works
  **only with master calibration frames** and is optimised for calibration libraries (it rescales
  dark noise to match every light frame).
- Recommend **at least 10 dark and bias frames**; **never fewer bias than dark** — a 50-frame master
  dark keeps almost all the read noise of a 10-frame master bias.
- Masters are built with **ImageIntegration**; run it twice (bias, then dark; bias subtraction of
  darks happens in IC). **Do not normalize** — the bias pedestal must be preserved (disable both
  normalization settings).
- In the Master Dark section enable **Optimize**: rescales the bias-subtracted master dark to each
  flat's thermal noise (a 1000 s master dark on a 10 s flat is multiplied by **0.01**).
- After bias/dark subtraction flats are pure illumination data: match illumination levels by
  **multiplicative normalization** in ImageIntegration (scale each flat to the average pixel value of
  all flats). The pixel-rejection algorithm depends on the number and type of flats.
- → master-frame conclusions: [04 §14](04-calibration-and-self-flat.md).

### 6.5 Tetra3rs — TESS blind-solve example — score **51.87**
`https://tetra3rs.dev/tutorials/basic-solve/`

- Python example (`tetra3rs as t3rs`, numpy, `astropy.io.fits`, `astropy.wcs.WCS`, `SkyCoord`,
  units, `plotly` with `pio.renderers.default = "notebook"`).
- Builds a **solver database from the Gaia DR3 catalog** (bundled automatically via the
  `gaia-catalog` package — no manual download), precomputing **geometric hash patterns for star
  quads** for fast plate solving.
- Loads a **TESS Full Frame Image (Camera 1, CCD 1, Sector 1)**; science data is in **HDU 1** with
  extra calibration columns, trimmed to the **2048×2048** science region by removing the
  **44-column overscan on the left**.
- `extract_centroids` finds connected pixel regions above a local background threshold, then computes
  **intensity-weighted centroid and covariance**; centroids are in **centered pixel coordinates**
  (origin at geometric image centre, +X right, +Y down).
- The TESS FITS header WCS is used to validate the solve: extract **CRPIX** and convert to centered
  coordinates. After iterative calibration the result carries camera attitude, matched star count,
  residual statistics and **attitude covariance**; **`attitude_sigma_rad`** is the 1σ uncertainty of
  roll and the two boresight offsets.
- → blind-solve/catalog conclusions: [02](02-astrometry-and-identification.md).

### 6.6 Astrometry.net — Building custom index files — score **34.59**
`https://astrometry.net/doc/build-index.html`

- Index files contain relatively bright, uniformly distributed "stars" (or galaxies) plus many
  **features/quads** describing the local shape of sets of (usually four) stars, each pointing back
  to its constituent stars. The engine detects stars, computes local shapes of 4-star sets and
  searches the indexes for similar shapes, then verifies that other reference stars align.
- Distributed indexes are based on **2MASS and Tycho-2**; the document explains building custom
  indexes from other reference catalogs.
- Conversion to **FITS BINTABLE**: `text2fits.py` (simple CSV/ASCII parser, very memory-hungry on big
  files), custom converters `2masstofits`, `nomadtofits`, `ucac3tofits`, `usnobtofits`, VizieR output
  (FITS binary table via output-format preferences), or a custom converter (author would rewrite the
  Xtofits tools in Python using `struct`).
- `cfitsio` helpers: `liststruc` (file structure), `listhead` (headers), `fitscopy` (copy with
  manipulation, extended filename syntax). The excerpt ends mid-thought on making cuts/removing
  irrelevant columns.
- → index/quad-hash conclusions: [02](02-astrometry-and-identification.md).

### 6.7 Astrometry.net — Engine internals (`axy`) — score **49.26**
`https://astrometry.net/doc/code.html`

- A field to solve is encapsulated in an **`axy` file** — a FITS binary table containing X,Y star
  positions plus FITS header cards describing command-line arguments and image metadata.
- **`axy` = "augmented x,y list"** (often shortened to "xylist"); created by **`augment-xylist.c`**.
- → solver internals conclusions: [02](02-astrometry-and-identification.md).

### 6.8 ASTAP — databases, densities and CLI — score **63.13**
`https://www.hnsky.org/astap.htm`

- Database update: **5 million galaxies**; the **2.1 million object** version is included with the
  star database. Variable stars **v2025.12.22**; macOS 64-bit Apple-silicon (M-series) program
  **v2026.11.12** (code signing required); Windows 64-bit `ASTAP_installer`/`astap_setup.exe`
  **v2026.11.12**.
- Only one database is needed. For FOV ≥ **0.6°** choose **D05, D20, D50 or D80** — D05 smallest,
  D80 largest (~**1.25 GB**, no drawback apart from size). Older H17/H18/V17/G17/G18 databases can be
  replaced.
- New databases use a **density limit instead of a magnitude limit**, sorted at **500, 2000, 5000 or
  8000 stars per square degree**, guaranteeing faint stars in star-poor areas while limiting bright
  stars in rich areas; they go as deep as **magnitude 21** if required.
- **V50** photometry database: like D50, **5000 stars/deg²**, magnitude = calculated **Johnson-V**,
  plus **Gaia Bp−Rp** colour; **V05** like G05 but with Johnson-V and Bp−Rp. V50 also works best for
  solving images with FOV > **10°**; colour information is **(G_Bp − G_Rp)**.
- **Barebone command-line solver** compatible with the GUI when renamed, no pop-up notifier; it will
  **not accept raw files** and **will not work with SharpCap** (FOV not stored).
- ASTAP is a free stacking/plate-solving program for FITS, with RAW DSLR, XISF, PGM, PPM, TIF, PNG
  and JPG import; native astrometric solver usable by CCDCiel, NINA, APT, Voyager, SGP. Stacking
  raises SNR, prevents saturation, corrects dark current/flat field and drops bad images (guiding,
  focus, clouds).
- → database/pixel-scale/p solving conclusions: [02](02-astrometry-and-identification.md).

### 6.9 Siril — Plate solving: flags, distortion order, catalogs — score **37.76**
`https://siril.readthedocs.io/en/latest/astrometry/platesolving.html`

- Plate solving associates images with celestial coordinates; **SPCC/PCC** need sufficiently accurate
  coordinates.
- If already solved, nothing is done unless **`-force`** is passed. Approximate image centre may be
  given in decimal degrees or J2000 colon-separated RA/Dec, comma- or space-separated (not mandatory
  for Astrometry.net).
- Distortion solved using **SIP polynomials up to order 5**, default from preferences, changed with
  **`-order=` (1–5)**.
- For local catalogues or local Astrometry.net, if the initial solve fails the solver searches within
  a cone of radius **`-radius=`** (default from preferences; **0** disables Siril's near search, but
  it cannot be disabled for Astrometry.net).
- Limit magnitude is computed from FOV size, overridden with a `+`/`-` offset or absolute value via
  **`-limitmag=`**.
- Catalog choice is automatic unless **`-catalog=`** is passed; possible values:
  **`tycho2, nomad, localgaia, gaia, ppmxl, brightstars, apass`** (local catalogs preferred when
  installed).
- Since **version 1.2** two algorithms exist: global-registration star matching (registering onto a
  virtual catalog image) and external **`solve-field`** from Astrometry.net (on Windows via **ansvr**).
  Since **version 1.3** the internal solver also supports additional capabilities (excerpt truncated).
- → plate-solving conclusions: [02](02-astrometry-and-identification.md).

### 6.10 StarStaX — stack-based star reduction — score **23.95**
`https://markus-enzweiler.de/software/starstax/`

- Fast multi-platform stacking/blending for **star-trail photography** (relative star motion produces
  trails), also light painting, noise reduction, synthetic exposure enlargement; interactive
  **gap-filling** and image-sequence output for time-lapse; **Flickr group** referenced.
- Formats: JPG, TIFF, PNG; **RAW cannot be developed** in StarStaX (convert to TIFF first). Portable:
  extract the ZIP, run `StarStaX.exe` from folder **StarStaX-0.71**, no installer; uninstall by
  deleting the folder.
- → star-reduction/starless prior art: [01](01-architecture-and-core-engine.md),
  [06-external-ai-sidecars.md](06-external-ai-sidecars.md).

---

## 7. Topic frequencies (final dump, cycle 4)

From the crawler's final frequency dump (preserved as reported):

`image 366, frames 215, stars 206, images 180, frame 165, star 153, drizzle 147, pixels 125, gaia 122, noise 120, flat 112, magnitude 107, fits 107, siril 99, stacking 97, cfa 94, astap 93, astrometry 90`

> Do not use `corpusNoEngFrequencies.txt` as a keyword source until the frequency-dump bug is fixed
> (§8).

---

## 8. Relevance assessment (consolidated)

- **High:** rejection/weighting (MeasureThePixels) and drizzle/CFA material directly support the
  statistical-stacking and CFA-drizzle sections of [01](01-architecture-and-core-engine.md).
- **High:** tetra3rs, Astrometry.net internals (`code.html`, `build-index.html`) and KStars catalog
  material support plate solving and catalog cross-matching in
  [02](02-astrometry-and-identification.md). Cycle 4 marked an improvement over earlier cycles:
  these are **implementation-level** references, not just end-user tutorials.
- **Moderate:** StarStax/StarNet are useful prior art for starless processing, but the research folder
  already concludes embedded ML is out of scope on the target platform
  ([06](06-external-ai-sidecars.md)).
- **Low:** flathub/bluesky/SourceForge packaging noise was correctly discarded.
- Blocked sources need manual/alternative-endpoint reading (§2); the crawler will not bypass rules.

---

## 9. Follow-up and improvements

1. **Worker pool + per-domain rate limits.** Parallelise the sequential loop (the 24 %/one-core
   observation, §3) via the roadmap's worker-pool item, paired with per-domain rate limiting so the
   crawler stays polite under the `robots.txt` policy.
2. **Fix the frequency-dump bug** (roadmap item 18) before using `corpusNoEngFrequencies.txt` as a
   keyword source for new research documents (§7).
3. **New seeds:** add `astrometry.net/doc/`, `www.astromatic.net/software/` and `hips` documentation
   landing pages to broaden registration coverage; also add the cycle-4 finds
   `tetra3rs.dev/tutorials/basic-solve/`, `measurethepixels.com/handbook/...` and
   `knro.blogspot.com/2017/03/...` to `03-reference-and-literature/source-catalog-and-microarchitecture.md`.
4. **Synthetic-calibration profile:** add a dedicated profile for "synthetic calibration frames /
   self-flat" matching [04](04-calibration-and-self-flat.md), with keywords
   `flat|dark|bias|self-flat|synthetic|master`.
5. **Longer time budget:** cycle 1 hit the 240 s cap with a non-empty link pool (18 docs = partial);
   re-run with a higher cap. Cycle 4's full session log is
   `goCrawler/logs/Skystacker-research-2026-11-17-Z21-48-15.txt`.
6. **Blocked endpoints:** read the 8 blocked URLs manually or via alternative endpoints (§2).

---

## Sources absorbed

- `research/06-crawler-based-source-discovery-2026-11-17.md` — methodology, 21 seeds from 140
  references, 18 retained documents and ranked score/characters/URL table, 8 blocked URLs, fail-open
  hosts, 24 %/8-core observation, 240 s cap, follow-ups.
- `research/07-crawler-cycle-4-new-sources-2026-11-17.md` — cycle-4 run metrics (338 s, 217 links,
  52 documents/1602 lines, cache hits 144/85.2 %, 25 downloads, 15 robots blocks, 48 errors, 51 new),
  new-source URLs, 72.4/70.4/65.8 scores, topic frequencies, relevance buckets.
- `research/08-curated-content-cycle-5-2026-11-17.md` — the 10 cycle-5 curated excerpt sources with
  scores (72.59, 70.45, 50.50, 51.87, 49.80, 49.26, 37.76, 34.59, 63.13, 23.95) and their key
  technical excerpts (§6).
- `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — the external deep-research outcome whose source
  list and CFA/Bayer-drizzle, ASTAP, PixInsight and plate-solving material seeded and contextualised
  these crawler runs (e.g. `hnsky.org/astap.htm`, `measurethepixels.com`, Siril drizzle docs,
  MyPetStars CFA drizzle, APP Bayer drizzle); its technical conclusions remain with
  [01](01-architecture-and-core-engine.md) and the other thematic files.
- `research/last-research.md` — superseded Spanish draft note; confirms the canonical English files
  and that crawler runs belong in this file (C8).
- Cross-linked canonical files: [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md),
  [`02-astrometry-and-identification.md`](02-astrometry-and-identification.md),
  [`03-reference-and-literature.md`](03-reference-and-literature.md),
  [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md),
  [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md),
  [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md),
  [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md), [`README.md`](README.md).

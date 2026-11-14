# SkyStacker Research & Technical Architecture

This repository folder contains the reorganized, cleanly structured technical specifications, mathematical formulations, astrometric research, and literature compendia for **SkyStacker** (a local-first, single-binary Go CLI engine for astrophotography calibration, registration, stacking, post-processing, and astronomical identification).

Every piece of technical content, equation, benchmark, CLI flag, code snippet, and bibliographic reference from the original research documents has been preserved and organized into logical domains for clarity and maintainability.

---

## Folder Structure & Document Guide

### Root Entry Point
* **[`RESEARCH_SYNTHESIS.md`](RESEARCH_SYNTHESIS.md)** — **Start here.** Master synthesis covering current implementation status (`v0.4.1` → `v0.5.0`), core mathematical foundations, architectural principles, strengths/limitations, and the prioritized engineering roadmap.

### `01-architecture-and-core-engine/` (Stacking, Go Runtime & CLI)
* **[`01-architecture-and-core-engine/current-implementation-v0.4.1.md`](01-architecture-and-core-engine/current-implementation-v0.4.1.md)** — Concrete snapshot analysis of the `v0.4.1` single-file Go CLI codebase (pipeline steps, notable design choices, strengths, and observed limitations).
* **[`01-architecture-and-core-engine/core-engine-technical-specification.md`](01-architecture-and-core-engine/core-engine-technical-specification.md)** — Comprehensive system architecture: 9-step SkyStacker pipeline vs. 5-step ASTAP in-memory workflow, Go concurrency & `sync.Pool` memory management, Swiss Tables (`SoA` / SIMD), photometric calibration, registration, statistical stacking, OSC demosaicing, post-processing, optical diagnostics (HFD/tilt/SQM), file formats (TIFF/BigTIFF/FITS/XISF), CLI reference, dual-stack comet recombination, and ensemble photometry.
* **[`01-architecture-and-core-engine/cli-integration-and-observatory-automation.md`](01-architecture-and-core-engine/cli-integration-and-observatory-automation.md)** — Cross-platform binary deployment (Windows ARM64/x64, Linux, macOS code-signing, Android), headless observatory integration, 7-stage decoupled pipeline, ASTAP CLI flags & return-code encodings (`%errorlevel%` / `stdout`), and `skystacker_results/` output schemas (`.wcs`, `.ini`, JSON telemetry).

### `02-astrometry-and-identification/` (Plate Solving, Sky Charts & Anomaly Detection)
* **[`02-astrometry-and-identification/astronomical-identification-and-sky-charts.md`](02-astrometry-and-identification/astronomical-identification-and-sky-charts.md)** — Broad research blueprint for an astrometric intelligence subsystem: blind plate solving (Astrometry.net, ASTAP, SCAMP, Siril), sky-chart integration (SkyChart/Cartes du Ciel vs. Aladin/HiPS), multi-catalogue cross-matching (Gaia DR3, SIMBAD, VizieR, NED, SDSS, AAVSO VSX, MPC, TNS), pixel-level morphology, transient/ZOGY difference imaging, moving-object tracking (A-Track), Go module layout, and a 6-phase implementation roadmap.
* **[`02-astrometry-and-identification/astrometric-catalog-annotation-and-anomalies.md`](02-astrometry-and-identification/astrometric-catalog-annotation-and-anomalies.md)** — Formal mathematical specification for WCS/SIP forward projection, HEALPix bounding queries, elliptical aperture rendering, $k$-d tree spatial cross-matching, unassociated-source anomaly flagging (`anomalies.json`), and unit test suites (`testWCSCatalogOverlay`, `testUnassociatedSourceFlagging`).
* **[`02-astrometry-and-identification/healpix-reference-verification.md`](02-astrometry-and-identification/healpix-reference-verification.md)** — Reference verification and calibration log (`healpy 1.16.5`, Gorski et al. 2005 §5) documenting the nested scheme convention, spherical diamond face-boundary geometry findings (commit `e47c3fd`), and fixture regeneration instructions.

### `03-reference-and-literature/` (Source Catalogs, Keywords & Reading Lists)
* **[`03-reference-and-literature/source-catalog-and-microarchitecture.md`](03-reference-and-literature/source-catalog-and-microarchitecture.md)** — Exhaustive catalog of all 50 primary project references (software, standards, papers, Go libraries), deep-dive notes on Go runtime internals (`sync.Pool`, Swiss Tables, eBPF profiling, Roofline model), scientific formats, and a categorized 100-term technical keyword index.
* **[`03-reference-and-literature/comprehensive-technical-glossary.md`](03-reference-and-literature/comprehensive-technical-glossary.md)** — Full 100-item numbered glossary with detailed technical definitions spanning architecture, data structures, calibration, astrometry, stacking statistics, post-processing, diagnostics, and file standards.

---

## Original Source Mapping

For traceability, the original files in `research/` map to the reorganized structure as follows:

| Original File | Reorganized Location(s) |
| :--- | :--- |
| `README.md` | `README.md` |
| `RESEARCH_SYNTHESIS.md` | `RESEARCH_SYNTHESIS.md` |
| `analysis.md` | `01-architecture-and-core-engine/current-implementation-v0.4.1.md` |
| `skystacker_technical_specification.md` | `01-architecture-and-core-engine/core-engine-technical-specification.md` & `03-reference-and-literature/comprehensive-technical-glossary.md` |
| `TECHNICAL SPECIFICATION_ CROSS-PLATFORM CLI INTEGRATION AND AUTOMATION ARCHITECTURE FOR OBSERVATORY IMAGE STACKING.md` | `01-architecture-and-core-engine/cli-integration-and-observatory-automation.md` |
| `astronomical-object-identification-and-astrometry.md` | `02-astrometry-and-identification/astronomical-identification-and-sky-charts.md` |
| `astrometric-catalog-annotation.md` | `02-astrometry-and-identification/astrometric-catalog-annotation-and-anomalies.md` |
| `healpix-methodology.md` | `02-astrometry-and-identification/healpix-reference-verification.md` |
| `skystacker_compendium.md` & `skystacker_technical_compendium_english.md` | `03-reference-and-literature/source-catalog-and-microarchitecture.md` (consolidated superset of all 50 sources, microarchitecture notes, and categorized keywords) |

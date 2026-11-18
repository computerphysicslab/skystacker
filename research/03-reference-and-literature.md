# SkyStacker — Reference & Literature Compendium

**Canonical file:** [`03-reference-and-literature.md`](03-reference-and-literature.md) · Part of the SkyStacker research corpus.

**Scope.** This is the single canonical, deduplicated reference document for SkyStacker. It absorbs and supersedes the two earlier `03-reference-and-literature/` working files (the 100-definition glossary and the 50-source catalog + microarchitecture notes), the consolidated academic bibliography and all URLs/DOIs in `02-astrometry-and-identification/astronomical-identification-and-sky-charts.md`, and the reference lists (Go internals, XISF/FITS, AI tooling) in `GEMINI_DEEP_RESEARCH_PROMPT-outcome.md`. Where the old files disagreed, one answer is given here.

**This file is reference-only.** It does not repeat full astrometry algorithms, calibration phases, toolchain history, sidecar implementation, CR2 decoding details, or crawler runs. Those live in the canonical files listed below and are cross-linked, not duplicated.

---

## 0. Reading guide, cross-links and one-time status resolutions

### 0.1 Canonical cross-links (shorthand used in tags below)

| Shorthand | Canonical file | Topic (do not duplicate here) |
| :--- | :--- | :--- |
| **C1** | [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) | Stacking pipeline, Go runtime/CLI engine, integration algorithms |
| **C2** | [`02-astrometry-and-identification.md`](02-astrometry-and-identification.md) | Plate solving, WCS/SIP maths, catalogue cross-match, annotation, anomalies |
| **C3** | [`03-reference-and-literature.md`](03-reference-and-literature.md) | This file — bibliography, glossary, performance, formats, BXT analysis |
| **C4** | [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) | Calibration master phases, self-flat derivation, validation |
| **C5** | [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md) | Go toolchain upgrade history, modularisation analysis |
| **C6** | [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) | External AI sidecar/CLI implementation, FITS sidecar runner |
| **C7** | [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) | Native CR2 / lossless-JPEG / bit-exact investigation |
| **C8** | [`08-crawler-source-discovery.md`](08-crawler-source-discovery.md) | goCrawler discovery cycles, curated corpus |
| **R** | [`README.md`](README.md) | Project overview, CLI flags, capability tiers |

### 0.2 Where-used tag legend (bibliography, §1)

| Tag | Meaning |
| :--- | :--- |
| `CAT #nn` | Entry `nn` of the former 50-source catalog |
| `GLO #nn` | Definition `nn` of the former 100-item glossary |
| `AST §x` / `AST-BIB xN` | Section / consolidated bibliography item in `astronomical-identification-and-sky-charts.md` |
| `GEM §x` / `GEM-SRC` | Domain section / "Sources" URL list in `GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` |
| `MICRO` | Former microarchitecture notes (Go runtime, Roofline, eBPF, code smells) |
| `FMT` | File-format reference material (now §4) |
| `BXT` | BXT/NXT/SXT analysis (now §5) |

A tag such as `→ C2` is the canonical file where the *topic* is developed; tags are provenance of the reference, not a copy of its content.

### 0.3 One-time status and conflict resolutions

These statements appear **once** in the whole corpus; all other sections refer back here.

* **Current version — v0.6.1.** `main.go` declares `version = "0.6.1"`. The engine remains a single-file, standard-library-only, `CGO_ENABLED=0` float32 RGB Go CLI. Earlier research snapshots quoted v0.4.1, v0.5.0 and v0.6.0; those are historical and are not restated in this document. (See C1 / **R** for the implementation itself.)

* **SIP order conflict — resolved in favour of order 2 (implemented), not order 3 (literature).** The legacy glossary defined SIP as a "3rd-order polynomial model," and the academic papers describe general/higher-order SIP distortion (AST-BIB A3, CAT #46, #67). The shipped implementation fits and stores **order-2 SIP only** (six coefficients per axis: `A02, A11, A20, B02, B11, B20`; `A_ORDER`/`B_ORDER` = 2). Order > 2 is not implemented. `--resample-sip` now loads that stored order-2 SIP from `plate_solution.json`, undistorts reference pixels, applies the similarity/affine transform in undistorted space, and redistorts source pixels before bilinear sampling (verified by `testResampleSIP`); per-frame reprojection and drizzle remain open. **Therefore: wherever this compendium mentions SIP, read "order-2 SIP as implemented"; the 3rd-order wording is deprecated.** Full WCS/SIP mathematics: C2. (Provenance: GEMINI append §"SIP in stacking resampler"; `main.go` comments at `wcsTanInverseSIP` and `fitSIP`.)

* **VizieR DOI conflict — resolved by recording both.** The VizieR database is cited with two distinct DOIs in the source material: the CDS record/landing DOI **`10.26093/cds/vizier`** (AST §3.1) and the Ochsenbein et al. (2000) journal-paper DOI **`10.1051/aas:2000169`** (AST-BIB B6). They are not competing values for one object but two layers of citation (database record vs. descriptive paper); entry **A6** lists both.

* **Single-pass sigma-clip status (stated once).** Proposal 2 was implemented conservatively: mean accumulation folded into the last clipping pass, survivor list carried between iterations, tile sums gathered in one pass; median/MAD rejection deliberately retained for bit-exact equivalence (`testSinglePassSigmaClipEquivalence`, ~20–25% faster kernels, end-to-end gains masked by decode/I/O). FITS Rice and XISF Rev 2 from the same survey remain **open**. Details: C1; underlying sources in §1 entries 72–75 (Go performance) and 47, 63, 102–103 (formats).

---

## 1. Master bibliography (unified, numbered, deduplicated)

**Merge policy.** One continuous numbering across five thematic blocks. The 50 catalog entries are preserved (§1.2); the 20 consolidated academic references are preserved (§1.1); every unique URL/DOI found anywhere in the absorbed sources is included (§§1.1–1.5). Duplicates were collapsed: where the same resource appeared in two sources (e.g. Swiss Tables blog = CAT #11 = GEM-SRC "The Go Team"; ASTAP homepage = CAT #01 = GEM-SRC "Kleijn"; GraXpert repo = CAT #65 = GEM-SRC "GraXpert Project"), a **single** entry carries all where-used tags. Each entry ends with `Tags: … | Used: …`.

### 1.1 Peer-reviewed & academic literature

1. **Astrometry.net: Blind Astrometric Calibration of Arbitrary Astronomical Images** — Lang, D., Hogg, D. W., Mierle, K., Blanton, M. & Roweis, S. (2010), *AJ*, 139, 1782–1800. DOI `10.1088/0004-6256/139/5/1782` · arXiv:0910.2233. Foundational geometric hashing of star asterisms, indexed catalogues, Bayesian verification. `Tags: AST §1.1, AST-BIB A1 | Used: C2`

2. **Automatic Astrometric and Photometric Calibration with SCAMP** — Bertin, E. (2006), ADASS XV. ADS PDF `articles.adsabs.harvard.edu/pdf/2006ASPC..351..112B`. Multi-exposure calibration. `Tags: AST §1.1, AST-BIB A2 | Used: C2`

3. **Representations of celestial coordinates in FITS** — Calabretta, M. R. & Greisen, E. W. (2002), *A&A*, 395, 1077–1122. DOI `10.1051/0004-6361:20021327`. FITS projections, frames, distortion conventions (basis of the general/higher-order SIP description). `Tags: AST §1.1, §2.1, AST-BIB A3, CAT #46/#67 | Used: C2, C3`

4. **SExtractor: Software for source extraction** — Bertin, E. & Arnouts, S. (1996), *A&AS*, 117, 393–404. DOI `10.1051/aas:1996164`. `Tags: AST §1.1, §4.1, AST-BIB A4 | Used: C2`

5. **The SIMBAD astronomical database** — Wenger, M. et al. (2000), *A&AS*, 143, 9. `Tags: AST §3.1, AST-BIB B5 | Used: C2`

6. **The VizieR database of astronomical catalogues** — Ochsenbein, F., Bauer, P. & Marcout, J. (2000), *A&AS*, 143, 23–32. **Both DOIs retained:** journal paper `10.1051/aas:2000169` and CDS VizieR record `10.26093/cds/vizier` (see §0.3). `Tags: AST §3.1, AST-BIB B6, GEM | Used: C2`

7. **Gaia DR3: Summary of the content and survey properties** — Gaia Collaboration (2023), *A&A*, 674, A1. DOI `10.1051/0004-6361/202243940`. `Tags: AST §3.1, AST-BIB B7 | Used: C2`

8. **Gaia DR3: The extragalactic content** — Gaia Collaboration (2023), *A&A*, 674, A41. DOI `10.1051/0004-6361/202243232`. `Tags: AST §3.3, AST-BIB B8 | Used: C2`

9. **Gaia DR3: Astrophysical parameters inference system (Apsis). I. Methods and content overview** — Creevey, O. L. et al. (2023), *A&A*, 674, A26. DOI `10.1051/0004-6361/202243688`. `Tags: AST §6.1, AST-BIB B9 | Used: C2`

10. **Identifying galaxies, quasars, and stars with machine learning** — Clarke, A. O. et al. (2020), *A&A*, 639, A84. DOI `10.1051/0004-6361/201936770`. `Tags: AST §3.3, AST-BIB B10 | Used: C2`

11. **DAOPHOT: A computer program for crowded-field stellar photometry** — Stetson, P. B. (1987), *PASP*, 99, 191. `Tags: AST §4.1, AST-BIB C11 | Used: C2`

12. **Galaxy Zoo 1: data release of morphological classifications for nearly 900,000 galaxies** — Lintott, C. et al. (2011), *MNRAS*, 410, 166. DOI `10.1111/j.1365-2966.2010.17432.x`. `Tags: AST §4.1, AST-BIB C12 | Used: C2`

13. **Machine learning and galaxy morphology: for what purpose?** — Fraix-Burnet, D. (2023), *MNRAS*, 523, 3974. DOI `10.1093/mnras/stad1654`. `Tags: AST §3.3, AST-BIB C13 | Used: C2`

14. **Machine learning-based photometric classification of galaxies, quasars, emission-line galaxies, and stars** — (2024), *MNRAS*, 527, 4677. DOI `10.1093/mnras/stad3436`. `Tags: AST §3.3, AST-BIB C14 | Used: C2`

15. **Proper Image Subtraction—Optimal Transient Detection, Photometry, and Hypothesis Testing (ZOGY)** — Zackay, B., Ofek, E. O. & Gal-Yam, A. (2016), *ApJ*, 830, 27. DOI `10.3847/0004-637X/830/1/27` · arXiv:1601.02655. `Tags: AST §5.1, AST-BIB D15 | Used: C2`

16. **A-Track: A new approach for detection of moving objects in FITS images** — (2016), *Computer Physics Communications*, 207, 524–530. DOI `10.1016/j.cpc.2016.07.023`. `Tags: AST §5.3, AST-BIB D16 | Used: C2`

17. **fink, a new generation of broker for the LSST community** — Möller, A. et al. (2021), *MNRAS*, 501, 3272. DOI `10.1093/mnras/staa3602`. `Tags: AST §5.2, AST-BIB D17 | Used: C2`

18. **Transient classifiers for Fink: Benchmarks for LSST** — Fraga, B. M. O. et al. (2024), *A&A*, 692, A208. DOI `10.1051/0004-6361/202450370`. `Tags: AST §5.2, AST-BIB D18 | Used: C2`

19. **The ALADIN interactive sky atlas: A reference tool for identification of astronomical sources** — Bonnarel, F. et al. (2000), *A&AS*, 143, 33. DOI `10.1051/aas:2000331`. `Tags: AST §7.1, AST-BIB E19 | Used: C2`

20. **Hierarchical progressive surveys: Multi-resolution HEALPix data structures (HiPS)** — Fernique, P. et al. (2015), *A&A*, 578, A114. DOI `10.1051/0004-6361/201526075`. `Tags: AST §7.1, AST-BIB E20 | Used: C2`

21. **Characterization of Photon-Counting Image Sensors** — Fossum, E. & Shade (2025), *JATIS*. PDF `ericfossum.com/Publications/Papers/2025 JATIS Shade Characterization of Photon-Counting Image Sensors.pdf`. `Tags: GEM §A | Used: C4`

### 1.2 The 50 primary project sources (former catalog, all preserved)

22. **ASTAP, Astrometric Stacking Program – HNSky** — URL. Homepage `hnsky.org/astap.htm`; version history `hnsky.org/history_astap.htm`; source `github.com/astap/astap`. `Tags: CAT #01, GEM §B/GEM-SRC | Used: C2, C8`

23. **Architectural Specification and Mathematical Foundations for a Local-First, Concurrent Go CLI Engine for Astronomical Image Integration** — Markdown specification. `Tags: CAT #02 | Used: C1`

24. **Asinh transformation [Siril 1.0]** — URL (Siril 1.0 documentation). `Tags: CAT #03 | Used: C1, C3`

25. **AsinhStretch — Astropy v8.0.2** — URL (Astropy visualization docs). `Tags: CAT #04 | Used: C1, C3`

26. **Astrophotography — Art and Science, Bluewater Astronomical Society** — PDF. `Tags: CAT #05 | Used: C3, C6`

27. **Best Deep-Sky Stacking Software for Mac (2026 Guide)** — URL. RAM / Apple Silicon scalability. `Tags: CAT #06 | Used: C1`

28. **Best Practices for Identifying and Eliminating Code Smells — Codacy Blog** — URL. `Tags: CAT #07 | Used: C3 (§3)`

29. **BornAgain-21 TIFF** — URL. TIFF for high-precision scientific data. `Tags: CAT #08, FMT | Used: C3 (§4)`

30. **C++ Swiss Table Architecture: SoA Layout and H1/H2 Hash Splitting — StudyPlan.dev** — URL. `Tags: CAT #09, MICRO | Used: C3 (§3)`

31. **Comprehensive Technical Architecture of Next-Generation Astronomical Image Processing Engines** — Markdown specification. `Tags: CAT #10 | Used: C1`

32. **Faster Go maps with Swiss Tables — The Go Programming Language** — URL `go.dev/blog/swisstable`. (Merged with GEM-SRC "The Go Team".) `Tags: CAT #11, GEM-SRC, MICRO | Used: C3 (§3), C5`

33. **GitHub — bep/imagemeta: Go library for EXIF/IPTC/XMP metadata** — URL. `Tags: CAT #12, FMT | Used: C3 (§4)`

34. **GitHub — sramuschkat/Siril-Scripts** — URL. `Tags: CAT #13 | Used: C5`

35. **Glossary [Siril 1.0]** — URL. `Tags: CAT #14, GLO | Used: C2, C3`

36. **Guide to Preprocessing of Raw Data with PixInsight — AWS** — PDF. `Tags: CAT #15 | Used: C4`

37. **How Go Maps Work: From Buckets to Swiss Tables — DZone** — URL. `Tags: CAT #16, MICRO | Used: C3 (§3)`

38. **PI SubframeSelector Script — Cloudy Nights** — URL. `Tags: CAT #17 | Used: C4`

39. **PIXINSIGHT Process Tutorial: Subframe Selector** — YouTube. `Tags: CAT #18 | Used: C4`

40. **PixInsight 1.9.6 Lockhart Released** — URL, merged with GEM-SRC developer notes: `pixinsight.net/dev/index.php?articles/pixinsight-1-9-6-lockhart-released.28/` and `…build-1718-released.30/`. `Tags: CAT #19, GEM §D/G §G | Used: C3 (§4), C5`

41. **Pixinsight: What does LocalNormalization do? — Cloudy Nights** — URL. `Tags: CAT #20 | Used: C4`

42. **Preprocessing in PixInsight (Part 4) – Local Normalization** — YouTube. `Tags: CAT #21 | Used: C4`

43. **README.md** — Markdown specification (project overview). `Tags: CAT #22 | Used: R`

44. **Revision 6.0 — ITU (TIFF 6.0 specification)** — PDF. `Tags: CAT #23, FMT | Used: C3 (§4)`

45. **Siril: Tutorial preprocessing — FreeAstro** — URL. `Tags: CAT #24 | Used: C4`

46. **Stacking Options panel (Astrophotography Stack Studio) — Affinity Help Center** — URL. `Tags: CAT #25 | Used: C1`

47. **Stacking — Siril 1.4.4 documentation** — URL. IKSS/MAD estimators, frame selection. `Tags: CAT #26 | Used: C1, C4`

48. **Static Code Analysis: Top 7 Methods — Oligo Security** — URL. `Tags: CAT #27 | Used: C3 (§3)`

49. **SwissMap: A smaller, faster Golang Hash Table — DoltHub Blog** — URL. `Tags: CAT #28, MICRO | Used: C3 (§3)`

50. **TECHNICAL_SPEC.md** — Markdown specification (v0.5.0). `Tags: CAT #29 | Used: C1`

51. **TIFF file format: specification, software and libraries — fileformats.dev** — URL. `Tags: CAT #30, FMT | Used: C3 (§4)`

52. **TIFF: Summary from the Encyclopedia of Graphics File Formats — FileFormat.Info** — URL. `Tags: CAT #31, FMT | Used: C3 (§4)`

53. **Tiff — MATLAB Gateway to LibTIFF library routines — MathWorks** — URL. `Tags: CAT #32, FMT | Used: C3 (§4)`

54. **Two Indexed Hash Tables — Vladimir Makarov** — URL. `Tags: CAT #33, MICRO | Used: C3 (§3)`

55. **Understanding Code Smells: Types, Refactoring & Best Practices — Aubergine Solutions** — URL. `Tags: CAT #34 | Used: C3 (§3)`

56. **Understanding Image Integration: Why Stacking Makes Your Astrophotos Better** — URL. `Tags: CAT #35 | Used: C1`

57. **Understanding code smells and how refactoring can help — TechTarget** — URL. `Tags: CAT #36 | Used: C3 (§3)`

58. **User guide (Siril 0.9.6)** — PDF. `Tags: CAT #37 | Used: C1, C4`

59. **What Is Code Review? — Sonar** — URL. `Tags: CAT #38 | Used: C5`

60. **geotiff package — github.com/gden173/geotiff/geotiff — Go Packages** — URL. `Tags: CAT #39, FMT | Used: C3 (§4)`

61. **imagemeta package — github.com/evanoberholster/imagemeta — Go Packages** — URL. Distinct from entry 33 (different library). `Tags: CAT #40, FMT | Used: C3 (§4)`

62. **tifffile — Documentation — Christoph Gohlke** — URL. `Tags: CAT #41, FMT | Used: C3 (§4)`

63. **FITS Standard — NASA/IAU** — PDF. `Tags: CAT #42, FMT | Used: C3 (§4), C7`

64. **XISF Specification 1.0 — Pleiades Astrophoto** — Specification. `Tags: CAT #43, FMT | Used: C3 (§4), C1`

65. **GraXpert: AI-based Background Extraction & Denoising** — URL (merged with GEM-SRC GraXpert repo/releases in entry 80). `Tags: CAT #44, GEM §E | Used: C6`

66. **Advanced Wavelet Processing in Astrophotography** — Scientific article (*à trous* wavelets). `Tags: CAT #45 | Used: C1, C3`

67. **Understanding SIP (Simple Imaging Polynomial) Distortion — Astropy** — URL. `Tags: CAT #46, GLO | Used: C2` (order-2 implementation governs — see §0.3)

68. **Concurrency in Go: Advanced Pipelines and Context — Go Blog** — URL. `Tags: CAT #47, MICRO | Used: C1, C3 (§3)`

69. **FITS WCS Standard (World Coordinate System)** — PDF. `Tags: CAT #48, FMT | Used: C2, C3 (§4)`

70. **The Role of CUDA and Vulkan in GPU-Accelerated Image Integration** — Technical article. `Tags: CAT #49 | Used: C1, C3 (§3)`

71. **NoiseXTerminator & StarXTerminator Architecture — RC-Astro** — URL. `Tags: CAT #50, BXT | Used: C3 (§5), C6`

### 1.3 Go runtime, toolchain & performance (GEMINI / microarchitecture URLs)

72. **GoPerf — Go Version Performance Tracking (Go 1.24–1.26)** — `goperf.dev/03-version-tracking/`. `Tags: GEM §F/GEM-SRC, MICRO | Used: C5, C3 (§3)`

73. **Edwards, A. — Go Experiments Explained (simd/archsimd, Go 1.26 GC)** — `alexedwards.net/blog/go-experiments-explained`. `Tags: GEM-SRC, MICRO | Used: C5, C3 (§3)`

74. **Sahasrabuddhe, A. — Performance Optimizations in Go 1.24: Swiss Table Maps and More** — `ajitem.com/blog/performance-optimizations-in-go-1-24-swiss-table-maps-and-more/`. `Tags: GEM-SRC, MICRO | Used: C5, C3 (§3)`

75. **Datadog Engineering — How Go 1.24's Swiss Tables saved hundreds of gigabytes** — `datadoghq.com/blog/engineering/go-swiss-tables/`. `Tags: GEM §F/GEM-SRC, MICRO | Used: C3 (§3), C5`

### 1.4 AI / astronomical tooling & external-sidecar URLs

76. **Siril Team — RC-Astro tools available in Siril (June 2026)** — `siril.org/2026/06/rc-astro-tools-available-in-siril/`. `Tags: GEM §E/GEM-SRC, BXT | Used: C6, C3 (§5)`

77. **AstroPixelProcessor — APP 2.0.0-beta47 RC-Astro CLI support** — `astropixelprocessor.com/community/rfcs-request-for-changes/blurxterminator-support-integration-or-equiv/paged/2/`. `Tags: GEM-SRC, BXT | Used: C6`

78. **MeasureThePixels — Changelog (RC-Astro CLI 2.6.6 / SASpro 1.21.0)** — `measurethepixels.com/changelog.html`. `Tags: GEM-SRC, BXT | Used: C6`

79. **Zelpio — Herramientas Standalone de RC Astro (Aug 2026 practitioner guide)** — `zelpio.com/es/blog/rc-astro-tools-standalone-guide/`. `Tags: GEM-SRC, BXT | Used: C6`

80. **GraXpert Project — Repository & Releases** — `github.com/Steffenhir/GraXpert`, `…/releases`. (Merged with CAT #44 = entry 65.) `Tags: GEM §E/GEM-SRC | Used: C6`

81. **Siril Documentation — GraXpert AI Interface** — `siril.readthedocs.io/en/latest/processing/graxpert.html`. `Tags: GEM-SRC | Used: C6`

82. **DSPX Documentation — GraXpert Denoise / Deconvolution** — `deepskycolors.com/astro/docs/tools/DSPX/dscp/GraXpertDenoise/…` and `…/GraXpertDeconvolution/…`. `Tags: GEM §E/GEM-SRC | Used: C6`

83. **Polaris Astro Handbook — AI Cleanup (GraXpert / ONNX / NPU)** — `polaris-astro.app.br/handbook/23-ai-cleanup.html`. `Tags: GEM §E/GEM-SRC, BXT | Used: C6, C3 (§5)`

84. **Seti Astro — Cosmic Clarity and Seti Astro Suite Pro** — `setiastro.com/cosmic-clarity`, `setiastro.com/seti-astro-suite-pro`. `Tags: GEM §E/GEM-SRC | Used: C6`

85. **The Night Map — BlurXTerminator Photoshop/CLI Guide** — `thenightmap.com/guides/blurxterminator-photoshop.php`. `Tags: GEM-SRC, BXT | Used: C3 (§5), C6`

86. **AstroWizard Guide** — `astrowizard.lukomatico.com/`. `Tags: GEM-SRC, BXT | Used: C3 (§5), C6`

87. **Caelo Works — Dark Frame Analyzer for PixInsight** — `pixinsight-scripts.caelo.works/en/scripts/dark-frame-analyzer`. `Tags: GEM §A/GEM-SRC | Used: C4`

88. **Cloudy Nights — BlurXTerminator introducing artifacts (March 2025)** — `cloudynights.com/forums/topic/956850-blurxterminator-introducing-artifacts/`. `Tags: GEM-SRC, BXT | Used: C3 (§5), C6`

89. **Siril Documentation — Drizzle Preprocessing** — `siril.readthedocs.io/en/latest/preprocessing/drizzle.html`. `Tags: GEM §C/GEM-SRC | Used: C1`

90. **AstroPixelProcessor — Bayer Drizzle Tutorial** — `astropixelprocessor.com/community/tutorials-workflows/bayer-drizzle/`. `Tags: GEM §C/GEM-SRC | Used: C1`

### 1.5 Standards, catalogues, solvers, VO services & miscellaneous URLs

91. **ESA Gaia — Gaia Data Release 4 (DR4) Overview** — `cosmos.esa.int/web/gaia/dr4`. `Tags: GEM-SRC | Used: C2, C5`

92. **ESA Gaia — News on the Gaia Archive and Data (Archive Bulletin)** — `cosmos.esa.int/web/gaia/bulletin-archive`. `Tags: GEM-SRC | Used: C2, C5`

93. **ESA Gaia — Gaia ESA Archive Release Notes (Version 3.15)** — `cosmos.esa.int/web/gaia-users/archive/release-notes`. `Tags: GEM-SRC | Used: C2, C5`

94. **IVOA — State of the IVOA (InterOp November 2025)** — `wiki.ivoa.net/internal/IVOA/InterOpNov2025/State_of_the_IVOA__Nov_2025_FINAL.pdf`. `Tags: GEM-SRC | Used: C2`

95. **Astropy Project — astroquery Changelog (Gaia DR4 DataLink types)** — `docs.astropy.org/projects/astroquery/en/latest/changelog.html`. `Tags: GEM-SRC | Used: C2`

96. **Garcia, A. — Gaia DR4 scheduled for December 2 at ~400 TB (secondary blog)** — `medium.com/@andy.a.g/gaia-dr4-is-scheduled-for-december-2-at-about-400-tb-esa-changed-the-archive-rules-first-61dd4a08fe22`. `Tags: GEM-SRC | Used: C2`

97. **Kleijn, H. — ASTAP Version History** — `hnsky.org/history_astap.htm`. (Merged with CAT #01 = entry 22, listed here for its GEMINI provenance.) `Tags: GEM §B/GEM-SRC | Used: C2, C8`

98. **Cloudy Nights — Astrometric plate solving on your phone (ASTAP for Android)** — `cloudynights.com/forums/topic/793549-astrometric-plate-solving-on-your-phone-astap-for-android/page/2/`. `Tags: GEM-SRC | Used: C2`

99. **Cloudy Nights — Is ASTAP still the best plate solver for NINA?** — `cloudynights.com/forums/topic/963963-is-astap-still-the-best-plate-solver-for-nina/`. `Tags: GEM-SRC | Used: C2`

100. **Cloudy Nights — AstroHopper experience (Cedar Solve / Cedar Detect)** — `cloudynights.com/forums/topic/993559-astrohopper-experience/`. `Tags: GEM-SRC | Used: C2`

101. **SkySolver — Free Plate Solving API Documentation** — `skysolver.app/api-docs`. `Tags: GEM-SRC | Used: C2`

102. **openastro-ara — CHANGELOG** — `github.com/open-astro/openastro-ara/blob/master/CHANGELOG.md`. `Tags: GEM §G/GEM-SRC | Used: C3 (§4)`

103. **sergio-dr/xisf — XISF 1.0 Revision 2 implementation** — `github.com/sergio-dr/xisf`. `Tags: GEM §G/GEM-SRC | Used: C3 (§4)`

104. **Astrometry.net — website & source** — `astrometry.net/`, `github.com/dstndstn/astrometry.net`. `Tags: AST §1.3 | Used: C2`

105. **SCAMP — website** — `astromatic.net/software/scamp/`. `Tags: AST §1.3 | Used: C2`

106. **Siril — website & source** — `siril.org/`, `github.com/AstroSharp/siril`. `Tags: AST §1.3 | Used: C2, C5`

107. **Siril — Astrometry docs & Annotations docs** — `siril.readthedocs.io/en/stable/astrometry/`, `…/annotations.html`. `Tags: AST §1.3, §2.1 | Used: C2`

108. **WCSLIB — documentation** — `atnf.csiro.au/computing/software/wcs/`. `Tags: AST §2.1, §7.2 | Used: C2, C3 (§4)`

109. **Astropy WCS — documentation** — `docs.astropy.org/en/stable/wcs/`. `Tags: AST §2.1 | Used: C2`

110. **Gaia Archive & DR3 documentation** — `gea.esac.esa.int/archive/`, `…/documentation/GDR3/`. `Tags: AST §3.1 | Used: C2`

111. **SIMBAD — website & guide** — `simbad.cds.unistra.fr/`, `…/guide/`. `Tags: AST §3.1 | Used: C2`

112. **NED — website & references** — `ned.ipac.caltech.edu/`, `…/Documents/References/NEDpapers`. `Tags: AST §3.1 | Used: C2`

113. **SDSS — website & SkyServer** — `sdss.org/`, `skyserver.sdss.org/`. `Tags: AST §3.1 | Used: C2`

114. **AAVSO VSX — website & VizieR record** — `aavso.org/vsx/`, `cdsarc.cds.unistra.fr/viz-bin/cat/B/vsx`. `Tags: AST §3.1 | Used: C2`

115. **Photutils — docs & detection guide** — `photutils.readthedocs.io/`, `docs.astropy.org/projects/photutils/en/stable/user_guide/detection.html`. `Tags: AST §4.1 | Used: C2`

116. **Transient Name Server (TNS)** — `wis-tns.org/`. `Tags: AST §5.2 | Used: C2`

117. **Zwicky Transient Facility (ZTF)** — `ztf.caltech.edu/`. `Tags: AST §5.2 | Used: C2`

118. **Fink broker** — `fink-broker.org/`. `Tags: AST §5.2 | Used: C2`

119. **Minor Planet Center — website & API docs** — `minorplanetcenter.net/`, `docs.minorplanetcenter.net/mpc-ops-docs/astrometry/`, `…/tutorials/notebooks/mpc_tutorial_api_observations/`, `…/tutorials/notebooks/mpc_tutorial_api_orbits/`. `Tags: AST §5.3 | Used: C2`

120. **JPL Horizons — Ephemeris Service** — `ssd.jpl.nasa.gov/horizons/`. `Tags: AST §5.3 | Used: C2`

121. **SkyChart / Cartes du Ciel — website & pictures docs** — `ap-i.net/skychart/en/start`, `…/documentation/pictures`. `Tags: AST §7.1 | Used: C2`

122. **Aladin Desktop & Aladin Lite** — `aladin.cds.unistra.fr/AladinDesktop/`, `…/AladinLite/doc/`. `Tags: AST §7.1 | Used: C2`

123. **CDS HiPS** — `aladin.cds.unistra.fr/hips/`. `Tags: AST §7.1, §7.2 | Used: C2`

124. **NASA FITS — homepage & FITS WCS page** — `fits.gsfc.nasa.gov/`, `…/fits_wcs.html`. `Tags: AST §7.2, FMT | Used: C2, C3 (§4)`

125. **IVOA VOTable** — `ivoa.net/documents/VOTable/`. `Tags: AST §7.2 | Used: C2`

126. **IVOA TAP** — `ivoa.net/documents/TAP/`. `Tags: AST §7.2 | Used: C2`

127. **IVOA ConeSearch** — `ivoa.net/documents/ConeSearch/`. `Tags: AST §7.2 | Used: C2`

128. **IVOA MOC** — `ivoa.net/documents/MOC/`. `Tags: AST §7.2 | Used: C2`

129. **MPC ADES — observations documentation** — `docs.minorplanetcenter.net/mpc-ops-docs/observations/`. `Tags: AST §7.2 | Used: C2`

130. **MyPetStars — CFA Drizzle Tutorial** — `mypetstars.com/tutorials/pixinsight/processes/drizzleintegration/cfa-drizzle`. `Tags: GEM-SRC | Used: C1`

131. **Cloudy Nights — Bayer drizzle threads** — `cloudynights.com/forums/topic/775840-bayer-drizzle-a-slight-tweak-or-a-serious-improvement/page/2/`, `…/topic/886412-bayer-drizzling-for-osc-cameras/`. `Tags: GEM-SRC | Used: C1`

---

## 2. Unified glossary & keyword index (one definition per concept)

**Merge policy.** The former 100-definition glossary and the former 100-term keyword index overlapped by roughly 70%. Here each concept is **defined exactly once**, in one of seven domain groups, with all aliases folded into that single definition. A compact alias→ID lookup (§2.8) provides keyword-index behaviour without duplicating definitions. Definitions unique to one legacy file or to the deep-research outcome are retained. The former "Terms active in v0.6.0" list is folded into the relevant entries, and its content is governed by the v0.6.1 / order-2 SIP resolutions in §0.3. Full mathematical treatment of the astrometry terms is in C2, of calibration in C4.

### 2.1 System architecture, Go runtime & hardware performance

* **G1 · SkyStacker** — Open-source CLI astrophotography processing engine written in Go; local-first, single-file (`main.go`), stdlib-only, `CGO_ENABLED=0`, float32 RGB. *(Aliases: Go CLI Engine.)*
* **G2 · Local-First Architecture** — Design principle prioritising local compute over cloud APIs.
* **G3 · Static Compilation / `CGO_ENABLED=0`** — Build mode producing self-contained binaries with no dynamic OS/C bindings (also listed as build-hygiene term).
* **G4 · Spatial Tiles (`tile-based streaming`; `TileBuffer`)** — Subdivision of image grids into uniform blocks to bound memory and improve cache locality.
* **G5 · Memory Buffer Recycling (`sync.Pool`)** — Go object pool that recycles floating-point slice allocations.
* **G6 · Garbage Collector Optimization / Heap Escape Analysis** — Compiler analysis deciding stack vs. heap allocation, minimising GC pressure (Go 1.26 default GC gains are in §3).
* **G7 · Goroutine Leaks** — Concurrent routines left running because exit signals are missing.
* **G8 · Pipeline Concurrency Pattern / `context.Context` Cancellation** — Channels (`chan`) link stages; `context.Context` propagates cancellation across goroutine hierarchies. *(Alias: Concurrency Pipeline.)*
* **G9 · Hardware Abstraction Layer (HAL)** — Interface decoupling image algorithms from backends.
* **G10 · GPU Acceleration / GPU Compute** — Offloading tile matrices to CUDA, Vulkan or Metal compute pipelines. *(Aliases: CUDA, Vulkan, Metal compute shaders.)*
* **G11 · Swiss Tables Architecture** — High-performance open-addressed hash table using SIMD control bytes and an SoA control/data split; Go's built-in map since Go 1.24. *(Aliases: SwissMap.)*
* **G12 · Structure of Arrays (SoA) / Array of Structures (AoS)** — SoA separates control metadata from payloads for cache efficiency; AoS stores key/value/control contiguously.
* **G13 · $H_1/H_2$ Hash Splitting / Fingerprint ($H_2$)** — 57 bits for bucket indexing ($H_1$), 7 bits for the control-byte fingerprint ($H_2$); control bytes are the stored metadata.
* **G14 · SIMD / SWAR Vector Inspection** — Single-Instruction-Multiple-Data / SWAR inspection of multiple control bytes concurrently using vector registers.
* **G15 · L1/L3 Cache Optimization** — Structuring data to fit 64-byte cache lines.
* **G16 · Load Factor Efficiency** — Entries/capacity ratio; Swiss Tables sustain performance to 87.5%.
* **G17 · Roofline Performance Model** — Bounds GFLOPS by min(peak ALU, bandwidth × arithmetic intensity). *(Aliases: Arithmetic Intensity, Compute-Bound Regime, Memory-Bound Regime — all quantified once in §3.)*
* **G18 · Continuous Profiling (eBPF)** — Kernel-level profiling via extended Berkeley Packet Filter; Parca/Pyroscope watch CPU counters and allocations without instrumentation.
* **G19 · Code Smell Refactoring / Detection** — Systematic removal of anti-patterns (long functions, hidden heap allocations, I/O coupling) for performance and maintainability.
* **G20 · Directed Acyclic Graph (DAG)** — Dependency graph of processing stages.
* **G21 · Audit Telemetry Log (`telemetry_audit.log`)** — Structured JSON execution trace of rejection counts, HFD metrics and system performance.
* **G22 · Zero-Allocation Metadata Reader (`imagemeta`)** — Go package (both `bep/imagemeta` and `evanoberholster/imagemeta`) reading EXIF/IPTC/XMP without heap allocation.
* **G23 · Astrometric Stacking Program (ASTAP)** — Fast local plate solver and stacker (HNSky); reference implementation and CLI adapter target.

### 2.2 Calibration, photometry & subframe quality

* **G24 · Photometric Calibration** — Removal of sensor/optical-path artifacts from raw data. *(Alias: Calibration algorithms.)*
* **G25 · Master Bias Frame** — Electronic readout offset at minimal exposure.
* **G26 · Master Dark Frame** — Exposure- and temperature-dependent thermal dark current.
* **G27 · Master Flat Frame** — Vignetting and dust-mote shadow map. *(Alias: Vignetting correction.)*
* **G28 · Flat Normalization Factor ($\bar I_{\text{flat}}$)** — Mean scalar intensity of the Master Flat used to preserve scale.
* **G29 · Fixed-Pattern Noise (FPN)** — Spatial pixel-sensitivity variation.
* **G30 · Self-Flat** — Relative illumination model derived from the lights themselves; `registered` is v1, `grid` is the Phase-1 sensor-space prototype (see C4).
* **G31 · Quality Metric Score ($Q_i$)** — Dynamic rating combining FWHM, eccentricity, SNR and PSF weight; `SubframeSelector` is the reference tool.
* **G32 · FWHM (Full Width at Half Maximum)** — Stellar profile width at half peak.
* **G33 · Stellar Eccentricity / Ellipticity** — Profile ellipticity; 0 is circular.
* **G34 · Signal-to-Noise Ratio (SNR)** — Signal relative to noise; `PSFWeight` is a related per-star weight.
* **G35 · Noise Evaluation / IKSS Estimator** — Robust noise estimators (IKSS, MAD) used for frame weighting.
* **G36 · Median Absolute Deviation (MAD)** — Robust dispersion estimator; used by sigma-clipping and cell statistics; finite-sample corrections apply for N = 3–30 (see §0.3, C1).
* **G37 · Local Normalization Correction (LNC)** — Equalisation of spatial background across a subframe sequence via reference images/`.xnml`.
* **G38 · Ensemble Photometric Zero-Point ($m_{0,\text{ensemble}}$)** — Weighted mean calibration reference from multiple catalog stars; **Passband System Transformation** converts instrumental colours to standard $B,V,R$; **Pickering Airmass Correction** adjusts for elevation; **Sky Quality Meter (SQM)** reports sky brightness in mag/arcsec² (all grouped here as photometric correction terms; detail in C4).

### 2.3 Star detection, registration & astrometry

* **G39 · Subpixel Centroid Estimation / Centroid Calculation** — True star centre via intensity-weighted moments; aperture photometry measures flux in an annulus/aperture.
* **G40 · 8-Connected Component Labeling** — Isolation of contiguous above-threshold pixel clusters.
* **G41 · Triangle Star-Pattern Matching** — Geometric registration of scale/rotation-invariant star triangles.
* **G42 · Quad Matching Engine** — Four centroids grouped into 2D tetragons producing 6-digit hashes.
* **G43 · RANSAC Filter / Similarity Transformation** — Iterative outlier-tolerant fit of the transformation model.
* **G44 · Lanczos / Sinc Resampling** — High-order subpixel interpolation kernel for alignment.
* **G45 · Safe Overlapping Crop** — Bounding-box reduction to areas valid in all aligned subframes.
* **G46 · World Coordinate System (WCS)** — Mapping $(x,y)\leftrightarrow(\alpha,\delta)$; **TAN WCS** is the gnomonic projection used by `--plate-solve`. RA = Right Ascension, Dec = Declination.
* **G47 · Simple Imaging Polynomial (SIP)** — Polynomial correction of optical field distortion, stored from plate solutions; **order 2 is what is implemented** (see §0.3) despite 3rd-order/higher-order literature. *(Alias: SIP Distortion.)*
* **G48 · Blind Plate Solving** — Astrometric identification without initial pointing; ASTAP/Astrometry.net/SCAMP/Siril are reference implementations; `plate_solution.json` carries the result.
* **G49 · Gaia DR3 Catalog** — Sub-billion-star astrometric/photometric reference; indexed via HEALPix nested tiles (fixtures in `assets/testdata/`).
* **G50 · HEALPix nested** — Hierarchical Equal Area isoLatitude Pixelisation tiling scheme used for the Gaia tile index (Gorski et al. convention, see C2 companion docs).
* **G51 · Point Spread Function (PSF)** — Star intensity profile; **PSF deconvolution** reverses blur; **aperture photometry** and HFR derive from it.
* **G52 · Minor Planet Center Format (MPC1992)** — 80-column astrometric observation format; ADES is the modern exchange standard.
* **G53 · Dual-Stack Ephemeris Recombination / Orbital Motion Alignment** — Separate stacking of stars and moving targets along ephemeris drift vectors $(\Delta\alpha,\Delta\delta)$; **derotation** (WinJUPOS) is the external planetary analogue.

### 2.4 Statistical integration & rejection

* **G54 · Stochastic Noise Reduction** — Attenuation of shot/read noise by temporal integration. *(Alias: Stochastic noise theory.)*
* **G55 · Square Root Scaling Law** — $\text{SNR}_{\text{stack}}=\sqrt{N}\cdot\text{SNR}_{\text{sub}}$.
* **G56 · Arithmetic Mean Integration** — Mean stacking; maximum SNR for Gaussian noise.
* **G57 · Median Integration** — Central-value stacking; suppresses transients.
* **G58 · Sigma Clipping** — Iterative rejection beyond $k\sigma$; the single-pass mean/variance variant is in §0.3.
* **G59 · Winsorized Sigma Clipping** — Rejected values replaced by boundary values before averaging.
* **G60 · Percentile Clipping** — Extreme-percentile rejection for small stacks.
* **G61 · Linear Fit Clipping** — Linear fit along temporal pixel arrays for varying sky glow.
* **G62 · Generalized Extreme Studentized Deviate (GESDT)** — Multiple-outlier test for small/moderate samples.
* **G63 · Max Stack / Min Stack** — Diagnostic composites keeping per-pixel maximum/minimum. *(Alias: Pixel rejection filters.)*
* **G64 · Drizzle Subpixel Integration** — Linear reconstruction restoring resolution; **CFA/Bayer drizzle** works on raw Bayer planes at scale 1.0 with a square flux-preserving kernel (C1).
* **G65 · IRLS (Iteratively Reweighted Least Squares)** — Specified for self-flat Phase 3, not implemented (C4).

### 2.5 Demosaicing, non-linear post-processing & AI models

* **G66 · Bayer Color Filter Array (CFA)** — RGB filter matrix over sensor pixels. **AstroC** reconstructs saturated stellar cores; **AstroM** renders undersampled stars neutral; **AstroSimple** extracts RGB in 2×2 neighbourhoods.
* **G67 · Linear Dynamic Range State** — Un-stretched state where pixel values are proportional to photon counts.
* **G68 · Non-Linear Stretch** — Redistribution of shadow dynamic range. **STF/MTF** is the screen/midtone transfer function used by auto-stretch; **Midtone Transfer Function (MTF)** remaps histograms about a selectable midtone.
* **G69 · Generalized Hyperbolic Stretch (GHS)** — Non-linear transform with independent midtone/highlight control.
* **G70 · AsinhStretch / Asinh (Arcsinh) Transformation** — Inverse-hyperbolic-sine stretch preserving colour saturation. *(Aliases: Asinh transformation, ArcsinhStretch.)*
* **G71 · Background Gradient Removal / Background Extraction** — 2D surface fitting subtracting light-pollution gradients; **Background Structure Masking (`--mask-bg`)** protects bright targets.
* **G72 · Chromatic Aberration Suppression / Correction** — Filtering of purple/magenta star fringing.
* **G73 · Spectrophotometric Color Calibration (SPCC)** — Colour balance against synthetic catalog spectra.
* **G74 · Subtractive Chromatic Noise Reduction (SCNR)** — Suppression of green cast from Bayer ratios.
* **G75 · Wavelet *À Trous* Processing** — Multiscale frequency decomposition without halo artifacts. *(Aliases: Wavelet Processing, multi-scale wavelets.)*
* **G76 · Unsharp Mask** — Edge-contrast boost that amplifies noise and creates dark ringing; the classical foil for BXT (§5).
* **G77 · Starless Post-Processing / Starless Workflows** — Separation of stars from background nebulae.
* **G78 · Neural Star Removal (StarXTerminator / SXT)** — CNN for star isolation/removal.
* **G79 · AI Denoising (NoiseXTerminator / NXT)** — Deep network attenuating background noise while preserving structure.
* **G80 · GraXpert** — Open-source AI background extraction / denoising / deconvolution tool (external sidecar, C6).
* **G81 · Lucky-Image Frame Selection** — Keeping the sharpest fraction of frames (e.g. 5–15%) to beat seeing.
* **G82 · MURE Denoise ordering / linear-stage restoration** — Denoise/deconvolution must run on integrated **linear** masters, before stretching; running on unstacked lights degrades SNR.
* **G83 · Gradient Detrending** — Target-aware background suppression before star detection/plate solving; pre-solve scratch-buffer variant is adjacent (C2).

### 2.6 Optical diagnostics, ephemeris & photometry

* **G84 · Half Flux Diameter (HFD)** — Diameter enclosing 50% of a star's cumulative light.
* **G85 · Sensor Tilt Analysis** — Focus variation across sensor zones. **9-Area Spatial Octagon** = 4 corners + 4 edges + centre; **3-Corner Triangular Analysis** maps tilt to 120° adapter screws; **Off-Axis Field Aberration ($\Delta$HFD)** is corner-minus-centre HFD.
* **G86 · Stellar Aspect Ratio (Roundness)** — Max/min radial distance; 1.0 is circular.
* **G87 · Gaia-Based Nova Identification** — Flagging stars missing from Gaia or with anomalous flux.
* **G88 · Local Normalization / cell statistics** — *See G36/G37; listed here only as the cell-statistics alias.* (No separate definition.)

### 2.7 Scientific file formats, metadata & build hygiene

* **G89 · IEEE 754 32-Bit Float / `.f32raw`** — Binary float32 representation preserving dynamic range; SkyStacker's `.f32raw` is raw little-endian float32 RGB with an 8-byte width/height header.
* **G90 · TIFF 6.0 Standard** — Scientific raster spec with floating-point samples (`SampleFormat=3`), strips/tiles, IFDs, GeoTIFF extension.
* **G91 · 64-Bit BigTIFF** — Extension past the 4 GB limit using 64-bit `LONG8` offsets.
* **G92 · Image File Directory (IFD)** — TIFF structure storing tags, offset pointers and channel dimensions.
* **G93 · Flexible Image Transport System (FITS)** — NASA/IAU standard using 2880-byte header blocks.
* **G94 · Rice Compression (`.fits.fz`)** — Tile-based lossless FITS compression.
* **G95 · XISF (Extensible Image Serialization Format)** — PixInsight standard with XML metadata and compressed streams; Revision 2 adds strict validation and SHA3 checksums.
* **G96 · EXIF / IPTC / XMP metadata** — Embedded metadata headers read without allocation (see G22).
* **G97 · VOTable / TAP-ADQL / Cone Search / HiPS / MOC / ADES** — IVOA interchange and query standards (see §1.5 and C2).

### 2.8 Keyword → canonical definition lookup (aliases only, no redefinition)

| Keyword / alias | Canonical ID | Keyword / alias | Canonical ID |
| :--- | :--- | :--- | :--- |
| Go CLI Engine, Local-First Design | G1, G2 | Static Compilation, `CGO_ENABLED=0` | G3 |
| `TileBuffer`, spatial tiles | G4 | `sync.Pool`, GC optimization | G5, G6 |
| Goroutine leaks, `context.Context` | G7, G8 | HAL, GPU Compute, CUDA, Vulkan, Metal | G9, G10 |
| SwissMap, SoA, AoS, $H_1/H_2$, control bytes | G11–G13 | SIMD, SWAR, load factor, L1/L3 cache | G14–G16 |
| Roofline, Arithmetic Intensity | G17 (see §3) | eBPF, Parca, Pyroscope, Code Smell | G18, G19 |
| DAG, telemetry log | G20, G21 | `imagemeta`, EXIF/IPTC/XMP | G22, G96 |
| ASTAP, Astrometric Stacking Program | G23 | Master Bias/Dark/Flat, FPN | G25–G29 |
| `SubframeSelector`, $Q_i$, PSFWeight, SNR | G31–G34 | IKSS, MAD, Noise evaluation | G35, G36 |
| LNC, LocalNormalization | G37 | Zero-point, airmass, SQM, passband | G38 |
| Centroid, aperture photometry | G39 | Connected components, triangle/quad | G40–G42 |
| RANSAC, similarity, Lanczos/Sinc | G43, G44 | WCS, TAN, RA/Dec | G46 |
| SIP Distortion | G47 (order 2 — §0.3) | Blind solving, `plate_solution.json` | G48 |
| Gaia DR3, HEALPix nested | G49, G50 | PSF, PSF deconvolution | G51 |
| MPC1992, ADES | G52, G97 | Ephemeris alignment, derotation | G53 |
| Stochastic noise, $\sqrt N$ law | G54, G55 | Mean/Median, sigma/Winsorized/percentile/linear-fit/GESDT | G56–G62 |
| Max/Min stack, pixel filters | G63 | Drizzle, CFA/Bayer drizzle | G64 |
| IRLS | G65 | Bayer CFA, AstroC/AstroM/AstroSimple | G66 |
| Linear state, STF/MTF, GHS | G67–G69 | Asinh/Arcsinh, background extraction, SCNR/SPCC | G70–G74 |
| Wavelet *À Trous*, unsharp mask | G75, G76 | Starless, SXT, NXT, GraXpert | G77–G80 |
| Lucky imaging, MURE ordering, detrending | G81–G83 | HFD, tilt octagon, roundness | G84–G86 |
| Nova ID, float32/`.f32raw` | G87, G89 | TIFF, BigTIFF, IFD, FITS, Rice, XISF | G90–G95 |

---

## 3. Microarchitecture & performance reference

*Consolidated once from source-catalog §2 (Go internals, eBPF, Roofline, code smells) and GEMINI §F. Numeric claims appear once here; other files link to this section rather than restate them.*

### 3.1 Roofline model

Achievable throughput is bounded by peak compute and memory bandwidth scaled by arithmetic intensity $I$ (FLOPs per byte moved):

$$\text{Performance (GFLOPS)}=\min\big(\text{Peak ALU},\ \text{RAM Bandwidth}\times I\big)$$

* **Memory-bound:** low $I$ (e.g. pixel addition) → limited by DRAM bandwidth.
* **Compute-bound:** high $I$ (FFT convolution, Lanczos/Sinc interpolation, iterative Winsorized sorting) → limited by AVX2/AVX-512 or GPU shaders.
* L2-friendly spatial tiles raise data locality and push work toward the compute-bound regime.

### 3.2 Go map / Swiss Table numbers (stated once)

* Built-in Go `map` switched to Swiss Tables in **Go 1.24**: 8-slot groups with 64-bit control words, extendible hashing to 128 groups / 1024 slots per sub-table, **87.5% max load factor**, no overflow buckets. Hash split: **57 bits** bucket index / **7 bits** fingerprint; control bytes inspected via SIMD/SWAR; SoA control/data layout fits 64-byte cache lines.
* Go 1.24 RC2 microbenchmark: **31.65% faster inserts** (137.10 → 93.70 ns) and **21.40% faster lookups** (41.17 → 32.36 ns); reduced lock contention, faster disjoint-key `sync.Map`.
* Datadog production case study: ~**500–510 MiB** live-heap saving on a ~726 MiB map (~70% with struct trimming); tradeoffs noted for low-traffic workloads.
* Go 1.26: default GC gives **7–11%** overall throughput gain, **18–20%** faster GC with many small objects, sub-32-byte allocation specialisation, **15–20%** faster `encoding/binary`, ~**2×** large `io.ReadAll`, **8–16%** faster `sync.Map`, experimental `RuntimeFreegc`.
* `simd/archsimd` is an off-by-default, **amd64-only** `GOEXPERIMENT` — rejected for cross-platform arm64 builds (see C5). `testing/synctest` (GA in Go 1.25) supports deterministic concurrency tests; `testing.B.Loop()` underpins the benchmark methodology.

### 3.3 eBPF continuous profiling

Kernel-level profilers **Parca** and **Pyroscope** use **eBPF** (extended Berkeley Packet Filter) to sample CPU hardware counters and allocations without instrumentation overhead, producing low-overhead flamegraphs of active goroutines.

### 3.4 Code-smell rules

* **Complex/lengthy functions** → atomic single-responsibility modules.
* **Hidden heap allocations** → eliminate implicit `any` conversions that force stack escape.
* **I/O coupling** → decouple disk reads from numerical loops with bounded concurrent channel buffers.
* Methods: Codacy, Oligo Security, Aubergine Solutions, TechTarget, Sonar (entries 28, 48, 55, 57, 59).

---

## 4. File-format reference table

*One table; symbolic numbers appear once. Binary details of `.f32raw` and CR2 decoding are in C1 / C7, not repeated here.*

| Format | Key structure / limits | SkyStacker relevance | Refs |
| :--- | :--- | :--- | :--- |
| **TIFF 6.0** | 8/16/32-bit samples, `SampleFormat=3` = IEEE float; IFD tags/offsets; strips & tiles; little/big endian; GeoTIFF extension | float32 TIFF read/write baseline; `SampleFormat=3` required | 29, 44, 51–53, 60–62; G90, G92 |
| **BigTIFF (`LONG8`)** | 64-bit offsets break the classic 4 GB ceiling; `LONG8` tag types | `--bigtiff` shipped 2026-11-18 (`stacked_sum/mean.bigtif`, 64-bit ready, round-trip tested); >4 GiB file not yet exercised | 51, 63; G91 |
| **FITS** | NASA/IAU standard; **2880-byte** header blocks; WCS keywords; HDU structure | Plate-solution WCS, sidecar transport, catalogues | 63, 69, 124; G93, G46 |
| **FITS Rice (`.fits.fz`)** | Tile-based lossless Golomb–Rice compression; ASTAP chose it over GZIP FITS (July 2026) | Pure-Go codec is an **open** item (`--fits-rice`) | 63, 97; G94 |
| **XISF 1.0 / Revision 2** | XML metadata + compressed memory streams; Rev 2 adds strict sample-mapping checks, corrupt-block rejection, SHA3-256/512 checksums, parallel I/O (1.5–4.6× writes, 1.8–4× reads, 3.3–4× checksummed) | `--xisf-rev2` shipped 2026-11-18: SHA3-256 checksum, strict sample-mapping and corrupt-block rejection; full PixInsight parallel-I/O parity remains open | 40, 64, 102, 103; G95 |
| **`.f32raw`** | Raw little-endian float32 RGB, 8-byte width/height header; no metadata | Swap format between pipeline stages; produced byte-identically by single-pass sigma-clip test | G89 |
| **IEEE 754 float32** | 32-bit binary floating point; preserves scientific dynamic range without clipping | Universal sample type across TIFF/FITS/XISF/sidecars | G89, G90 |
| **Metadata (EXIF/IPTC/XMP)** | Embedded header tags; zero-allocation readers `bep/imagemeta`, `evanoberholster/imagemeta` | Ingestion metadata without heap churn | 33, 61; G22, G96 |

---

## 5. BXT / NXT / SXT reference & analysis

*Consolidated from glossary §13 and the GEMINI AI sections. Implementation of the external sidecar runner is in C6; the capability tier is in R. This section is the analysis/reference home only.*

### 5.1 What the tools are

* **BXT = BlurXTerminator**, **NXT = NoiseXTerminator**, **SXT = StarXTerminator** — AI tools by **RC Astro** (Russell Croman), entry 71.
* **StellaImage 10** (AstroArts) integrated native BXT/NXT/SXT behind a separate RC Astro license.
* **June 2026:** RC Astro shipped **standalone cross-platform CLIs** (Windows/macOS/Linux; BXT v2.6.6 running ML5) — Siril and APP announcements (entries 76–79). This removes PixInsight/Photoshop host lock-in but stays license-gated (reported ≈$100 BXT / $60 NXT).
* **Mechanism:** unlike unsharp masking (edge contrast that amplifies noise and dark ringing — G76), BXT uses a neural network trained on astronomical data to model the optical **PSF** and reverse blur from diffraction, guiding error and mild seeing. It restores geometry of existing signal (e.g. Saturn's Encke gap), not invented texture.

### 5.2 The poor-seeing wall and external mitigations

* BXT measures local blur from field stars; on planets (no field stars) it estimates from planetary edges. **If seeing smears fine divisions across several pixels, contrast is already lost pre-stacking and no deconvolution recovers it.**
* Planetary poor-seeing mitigations (external workflows):
  * **Lucky imaging** — AutoStakkert!, keep sharpest **5–15%** of frames.
  * **Multi-scale wavelets** — **waveSharp**, **RegiStax**, targeting spatial frequencies.
  * **Longer integration with derotation** — **WinJUPOS**, raising SNR before sharpening.
* Documented failure modes: plastic-looking stars when denoise >0.7; hallucinations when BXT runs on stretched data; SNR loss when denoise/deconvolution runs on unstacked lights; strict linear-stage ordering (G82, G81).

### 5.3 Why native inference is out of scope

Under the single-file, stdlib-only, `CGO_ENABLED=0` constraint (Tier 4∞ in **R**): no ML runtime, no model weights, no GPU. GraXpert's ONNX denoise weights alone are **284 MB (v2) → 456 MB (v3)**; pure-Go U-Nets over 10–24 MP float32 stacks would be prohibitively slow on 15 W ULV laptops; `simd/archsimd` is amd64-only (§3.2). Calling a licensed RC Astro plugin from the Go binary is not a reproducible stdlib-only path and is license-gated. **Treat all such tools as external sidecars only** (DSPX-style private-temp-directory float32 FITS handoff via `os/exec`, no shell), detailed in C6.

### 5.4 Classical equivalents feasible in the current pipeline

* **BXT analogue:** `--deconvolve` (Richardson–Lucy, Gaussian PSF) is the shipped analogue — global, fixed PSF, CPU-heavy, no spatially-varying blur. Upgrade path: PSF estimation from detected stars (HFR/eccentricity now recorded), per-tile kernels, star-masked deconvolution; ordering locked to linear masters.
* **SXT analogue:** `--starless` uses annulus-median removal; a PSF-derived mask + reconstruction is closer.
* **NXT analogue:** guided filter + two-scale chroma pass; next step is multiscale noise estimation with spatially adaptive strength.
* **Lucky-imaging analogue:** frame-quality weighting (`computeAdvancedQuality`, `--hfr-quality`, MAD clipping) already downweights blurred frames; a "keep sharpest N%" by HFR percentile is the concrete planetary-facing feature.
* **Multi-scale wavelets / derotation:** radial-frequency band split with per-band gain is feasible in stdlib Go; derotation stays external (WinJUPOS) and is documented as a handoff.
* **Honest boundary:** when seeing destroys contrast before capture, no post-processing restores information — recommend capture-side mitigations alongside any sharpening flag.

---

## Sources absorbed

The following source files were read in full and merged/deduplicated into this document; no other file was modified.

1. `research/03-reference-and-literature/comprehensive-technical-glossary.md` — 100 definitions, "Terms active in v0.6.0", BXT/NXT/SXT §13 → §2 and §5.
2. `research/03-reference-and-literature/source-catalog-and-microarchitecture.md` — 50-source catalog, software/profiling/Roofline notes, 100-term keyword index, v0.6.0 map → §1, §3, §2.
3. `research/02-astrometry-and-identification/astronomical-identification-and-sky-charts.md` — bibliography (§10 A–E), essential papers (§1.1), reference-implementation and standards URLs (§§1.3, 2.1, 3.1, 3.3, 4.1, 5–7) → §1.
4. `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — Go-internals/toolchain, XISF/FITS and AI-tool reference lists (the "Sources" URL list), SIP-in-resampler and single-pass sigma-clip status notes → §1, §3, §4, §5 and §0.3.

Supporting context (read for accurate cross-links/status only, not absorbed as new reference content): `research/README.md`, `main.go` (version `0.6.1`, SIP order-2, `--resample-sip`).

*All full algorithms, calibration phases, toolchain history, sidecar implementation, CR2 details and crawler runs remain in their canonical files listed in §0.1.*

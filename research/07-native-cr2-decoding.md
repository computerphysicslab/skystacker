# SkyStacker — Native CR2 Decoding & Bit-Exact Investigation

**Canonical file:** [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) · Part of the SkyStacker research corpus.

**Scope.** This is the single canonical, deduplicated home for everything concerning Canon CR2 / lossless-JPEG (SOF3) decoding in SkyStacker and goPureRaw: the file/stream facts for the EOS 40D test asset, the decoder algorithm, the measurements against `dcraw`, the authoritative `dcraw.c` v9.28 source findings, the refuted `cr2_slice` hypothesis, and the revised validation experiment. It absorbs the archived CR2 bit-exact prompt and its local outcome. It is **not** a history of the Go toolchain or modularisation, nor of external AI sidecars: those topics are cross-linked, not repeated.

**Current status (stated once).** Native pure-Go CR2 decoding is **enabled by default**; `--no-native-cr2` restores the dcraw/ImageMagick fallback. The decoder is **usable but not bit-exact validated**. The empirical per-pixel/geometric agreement with `dcraw` is still unresolved, and the open validation item is tracked in the goPureRaw roadmap (Phase 1b). CR3 input still needs an external decoder. (Implementation/flag details: [01](01-architecture-and-core-engine.md), [README.md](README.md).)

---

## 0. Cross-links and scope boundaries

| Shorthand | Canonical file | Topic (do not duplicate here) |
| :--- | :--- | :--- |
| **C1** | [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) | Stacking pipeline, Go runtime/CLI engine, shipped flags |
| **C2** | [`02-astrometry-and-identification.md`](02-astrometry-and-identification.md) | Plate solving, WCS/SIP maths, catalogue cross-match |
| **C3** | [`03-reference-and-literature.md`](03-reference-and-literature.md) | Bibliography, glossary, file-format reference table |
| **C4** | [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) | Calibration masters and self-flat derivation |
| **C5** | [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md) | Go toolchain upgrade history, modularisation analysis, `goPureRaw` module release history |
| **C6** | [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) | External AI sidecar/CLI implementation and FITS sidecar runner |
| **C7** | [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) | This file — native CR2 / lossless-JPEG / bit-exact investigation |
| **C8** | [`08-crawler-source-discovery.md`](08-crawler-source-discovery.md) | goCrawler discovery cycles, curated corpus |
| **R** | [`README.md`](README.md) | Project overview, capability tiers, CLI flag reference |

Deliberate non-duplication: the `goPureRaw` module path/tag/`go.work` history is in **C5**; the `--no-native-cr2` flag in the full shipped-flag list is in **C1/R**; demosaicing/Bayer-drizzle theory is in **C1** and the glossary in **C3**. Only the CR2-specific facts are developed here.

---

## 1. Provenance of this investigation

- The intended **Google Gemini Deep Research** run could not be executed: the Gemini API returned **`HTTP 429 — "limit: 0 input tokens per minute on Free Tier"`** for **both** deep-research agents, `deep-research-preview-04-2026` and `deep-research-max-preview-04-2026`. The two source documents are therefore a **local web/source research substitute**, not a Gemini report.
- The prompt is archived verbatim in `GEMINI_CR2_BITEXACT_PROMPT.md` for a possible paid-tier run; the local outcome is in `GEMINI_CR2_BITEXACT_PROMPT-outcome.md`. This file deduplicates both into one canonical record.
- Gemini Deep Research API docs: `https://ai.google.dev/gemini-api/docs/deep-research` (agents listed above; free tier observed at 0 input tokens/minute).
- `GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` was also reviewed for CR2/native-decoding content; it contains **none** (it covers Go toolchain gains, RC-Astro sidecars, SIP resampling and single-pass sigma-clip), so nothing from it is absorbed here. Its own topics are in **C3/C5/C6**.

Historical code layout at the time of the investigation: SkyStacker was described as a single-file Go CLI (stdlib only, `CGO_ENABLED=0`) with a nested `goPureRaw/` module (then declared `github.com/youruser/pureraw`). The relevant decoder files were `internal/ljpeg/ljpeg.go` (SOF3 lossless-JPEG decoder), `formats/cr2/decoder.go` (container probe, lossless-JPEG stream finder, two-plane → Bayer assembly), and `formats/cr2/decoder_test.go` (`TestDecodeOracle`, gated by the `PURERAW_CR2` env var). `main.go` held an equivalent embedded copy as `prDecodeLJPEG`, `prDemosaicBayer`, `prDecodeCR2Native`; that duplicate was later deleted when SkyStacker imported the module (see §7 and **C5**).

---

## 2. The archived research prompt (questions and requested deliverables)

Recorded here so the investigation's intent is not lost; the answers found locally follow in §§3–6.

Context in the prompt: test file **Canon EOS 40D CR2 `IMG_7005.CR2`, 11,892,827 bytes**; `dcraw` installed on the host. It asked for an exhaustive, source-backed answer rather than general RAW advice.

Seven research questions were posed:

1. **Canon CR2 lossless-JPEG layout** — authoritative description (dcraw `lossless_jpeg_load_raw()`, LibRaw, ExifTool CR2 docs, DNG spec, an RC paper) of how the two SOF3 components map to the sensor Bayer grid for EOS 40D-class cameras: even/odd columns, left/right halves, even/odd rows, or camera-specific.
2. **dcraw predictor semantics** — quote/explain dcraw's predictor handling for lossless JPEG with `predictors=1`: initial predictor, first sample of each row, Canon-specific overrides; does it use a different predictor table than the JPEG spec?
3. **dcraw `-D` output transform** — exactly what document mode writes (black subtraction, 16-bit scaling, transpose, crop, flip, Bayer reorder), with the exact code path and `white`/`black`/`output_bps` formulas.
4. **Why the max differs** (ours 16372 vs dcraw 16011) — black/white scaling, white-balance multipliers even in `-D`, clipping, or a different decode.
5. **Open-source reference implementations** — how LibRaw, rawpy or ImageMagick decode Canon CR2 SOF3 streams; is there a known two-component interleave rule?
6. **Validation methodology** — building tiny synthetic lossless-JPEG/CR2 fixtures with known ground truth; any public CC0 CR2 corpus with reference raw dumps.
7. **Go-specific subtle bugs** — plausible bugs that preserve histogram/mean but permute spatial order (MCU ordering, component scan interleaving, predictor context per component, stuffed-byte handling).

Requested deliverables: (1) root-cause hypothesis ranking with evidence; (2) exact conversion algorithm/pseudocode; (3) concrete code-level fixes; (4) oracle test plan with synthetic fixtures, dcraw flags, metrics/tolerances; (5) numbered primary sources with dates/URLs, forum reports marked as practitioner reports. Guardrails: distinguish authoritative source code from inference; state camera coverage for camera-specific rules; **no AI/neural upscaling or demosaicing** as a workaround (goal is bit-exact sensor data); final language English.

What was actually established locally (§§3–6) covers the authoritative dcraw source path, the predictor and component mapping, the value-transform explanation, the refutation of the leading hypothesis, and the revised oracle plan. LibRaw/rawpy/ImageMagick code-level comparison and a public CC0 corpus were not resolved by the local substitute.

---

## 3. Verified file/stream facts for `IMG_7005.CR2`

### 3.1 Container and IFD structure

- File: **11,892,827 bytes**, Canon **EOS 40D** (`assets/OrionNebula/lights/IMG_7005.CR2`).
- CR2 header: **`II*\0`** (little-endian TIFF), with **`"CR"` + version at bytes 4..7**. *(The later `CR2 Probe` byte-8 signature fix is a separate matter — see §7.)*
- **IFD0 is a thumbnail**: 1936x1288, JPEG compression **6**.
- **IFD1** is a JPEG thumbnail.
- Exif IFD carries **MakerNote tag `0x927C`, 34,058 bytes**.

### 3.2 The lossless-JPEG (SOF3) raw stream

Found by scanning for the marker pair **`FF C3`**:

- **SOI** at file offset **1,366,852**; **DHT** at **1,366,854**; **SOF3** at **~1,366,924**; **SOS** shortly after.
- **SOF3**: precision **14 bits**, height **2622**, width **1972**, **2 components** (ids 1 and 2), sampling **0x11**.
- **SOS**: `ns=2`, component selectors **`01 00`** and **`02 10`** (DC tables 0 and 1 respectively); final byte **`0x00`** → **predictor selection 1**.
- **DHT**: two tables (ids 0 and 1), counts `[0,2,2,2,3,1,1,1,1,1,1,0,0,0,0,0]`, **15 symbols each**: `[4,5,3,6,7,2,8,0,1,9,10,12,11,13,14]`.
- Entropy stream is **byte-stuffed (`FF 00`)**; **no restart markers observed**.

### 3.3 Decoder algorithm (ours)

- **Canonical Huffman** decode with an **MSB-first bit reader**; difference reconstruction `v - (1<<S) + 1` when `v < 1<<(S-1)`.
- **Critical fix already applied:** initial predictor `2^(P-1)` = **8192**; at the start of each row predictor 1 uses the **sample above** (or the initial predictor on row 0). Without this fix the output looked random; with it the result is plausible.
- Reconstructed as **two planes of 1972x2622 `uint16`** samples each.
- Mapped to a **3944x2622 Bayer mosaic**: **even columns = plane 0, odd columns = plane 1**; **RGGB** assumed; **black = 1018**, **white = 16224** (EOS 40D values from `dcraw -v`).

---

## 4. Measurements and the failed alignment

- **Our decoded full Bayer:** min ~1, **max 16372**, **mean 1181.84**, horizontal-diff mean **60.8**, vertical-diff mean **71.4**.
- **`dcraw -D -4 -c file`:** **P5 3908x2602**, 16-bit container, min 4, **max 16011**, **mean 1180.96**, horizontal diff **63.0**, vertical diff **59.3**.
- **`dcraw -D -4 -t 1 -c`:** still **3908x2602** (not 3944x2622) — document mode appears to output a transposed/cropped buffer regardless of the orientation flag.
- Histograms/percentiles are **very close**, so the data is likely the same image content, but **spatial correspondence is unresolved**.
- **Failed alignment attempts:** all **8 dihedral transforms** (flip H/V + transpose) × crop offsets up to **80 px** (downsampled 16x), even/odd column interleaving, left/right halves, row shifts ±2, and plane swaps. Best normalized cross-correlation **< 0.1** (essentially no match).
- Local-outcome refinements of the same negative result: Spearman **~0.14** on aligned patches across the 8 transforms + crop offsets; a **1-D column-profile correlation reaches only ~0.44 at shift 9**.

---

## 5. `dcraw` v9.28 source findings (authoritative)

Source inspected: **`dcraw.c` v9.28** — `ncruces/dcraw`, RawTherapee `rtengine/dcraw.c`, Debian sources 9.28-2 — specifically the functions `ljpeg_start`, `ljpeg_row`, `ljpeg_diff`, `lossless_jpeg_load_raw`.

1. **SOF3 metadata** — `ljpeg_start()` reads precision/height/width/components from SOF3 exactly as we do. It also computes `sraw = ((data[7] >> 4) * (data[7] & 15) - 1) & 3`; for sampling `0x11` this is **0**, so the "sraw" path is **not** taken for the EOS 40D.
2. **Huffman tables** — DHT entries are accepted with the same class/id byte convention; our canonical Huffman decoding and difference conversion (`v - (1<<len) + 1` when the sign bit is 0) match `ljpeg_diff()`.
3. **Predictor** — `ljpeg_row()`:
   - at `col == 0`, predictor is `vpred[c]`, initialised to `1 << (bits-1)` and updated to the current col-0 sample each row;
   - at `col > 0`, predictor is `row[0][-clrs]` (left sample, same component);
   - `psv` comes from SOS byte `data[1 + data[0]*2]`; for our file it is **1**.
   - These match our corrected implementation (initial predictor 8192, row start uses the sample above).
4. **Component mapping** — `lossless_jpeg_load_raw()` iterates `jwide = jh.wide * jh.clrs` decoded samples **row-serially**, writing them into `raw_image` with `RAW(row,col)=val` and wrapping `col` at `raw_width`. With `wide=1972`, `clrs=2`, `raw_width=3944`, samples alternate component 0/1 → our **even/odd column interleaving is correct**.
5. **Values / `-D -4` output path** — dcraw applies `curve[*rp++]` before storing, and later **black-level subtraction (`cblack[]`) plus white scaling** when writing 16-bit PPM. This is the plausible source of the max 16372 (ours) vs 16011 (dcraw) and the shifted percentiles — a **value transform, not a geometry transform**. (The append-only check in §6 refines what `curve[]` actually contains for this camera.)

Net effect of §5: the decoder core (SOF3 parsing, Huffman, predictor, component interleave) is confirmed correct against authoritative source; the discrepancy must be sought in the post-JPEG pixel pipeline or in a still-unexplained crop/orientation behaviour, not in the entropy decoder.

---

## 6. Hypothesis history: `cr2_slice` raised, then refuted; revised next experiment

### 6.1 Original ranking (from the local outcome, before the source check)

With geometry apparently correct per source review, the candidate explanations were:

- **Canon linearization curve not applied to our samples** — a value transform, should not break rank correlation, so probably not the whole story.
- **`cr2_slice[]` remapping on the EOS 40D** — dcraw applies a slice table when present; this *would* permute pixels, so it was the **leading hypothesis**.
- **Crop/orientation in dcraw document mode** not covered by the transform search.

The corresponding plan estimated ~2–4 h total: extract `cr2_slice`/linearization data from the maker note (1–2 h), build a synthetic CR2 with known samples (30–45 min), then repeat the oracle comparison with rank correlation and add bit-exact unit tests if it approaches 1 (30 min).

### 6.2 Append-only refutation (2026-11-17)

A direct source check of `dcraw.c` v9.28 (ncruces/dcraw, downloaded and grepped locally) established:

- **`cr2_slice[3]` is only ever reset to zero** (line 8631) and read in `lossless_jpeg_load_raw` / `canon_sraw_load_raw`; **no Canon maker-note tag assigns it in this version**.
- The EOS 40D maker-note Exif tag **`0x0009`** is a **32-byte ASCII/artist field, not a slice table**, and it is **all zeros** in `assets/OrionNebula/lights/IMG_7005.CR2`.
- Therefore the leading `cr2_slice` hypothesis **cannot** explain the mismatch and the 1–2 h slice-extraction step was **dropped**.

The same check also resolved the `curve[]` question: **`curve[]` is initialised to identity for this camera** (no Canon linearization table is loaded). The remaining value differences must come from dcraw's output pipeline — **black-level subtraction (`cblack[]`) and white scaling before writing the 16-bit PPM** — not from a linearization table or a slice permutation.

### 6.3 Revised next experiment (current recommendation)

Perform an **oracle comparison at the raw Bayer plane level**: compare our decoder's output against `dcraw -D -4 -t 1` **after cropping the margins and normalising black/white** (i.e. map both to the same black=1018 / white=16224 scale and compare the 3944x2622 Bayer planes). This is explicitly **not another geometry search over dihedral transforms** — source inspection has already ruled out the geometry doubts that motivated those searches.

Supporting validation work retained from the plan: build a **synthetic CR2 with known samples and a tiny maker note** so the expected mapping is known without dcraw, then, if rank correlation after normalisation approaches 1, add bit-exact unit tests to goPureRaw (`TestDecodeOracle`) and SkyStacker. The concrete symptom to explain remains the pair **max 16372 / mean 1181.84 (ours)** vs **max 16011 / mean 1180.96 (dcraw)**, together with the previously low NCC (< 0.1) and Spearman (~0.14).

---

## 7. `CR2 Probe` byte-8 fix and goPureRaw v0.1.1 (brief)

For module path, tags, `go.work` and release mechanics see **C5**. CR2-specific highlights only:

- Tagging `goPureRaw/v0.1.0` exposed a **CR2 `Probe` offset bug**: the CR2 signature is at **bytes 8..9, not 4..5**. It was fixed and **goPureRaw/v0.1.1** published.
- SkyStacker then declared `require ...goPureRaw v0.1.1`, imported `raw` + `formats/cr2`, and the duplicated `pr*` CR2 decoder was **deleted from `main.go`**. The `--no-native-cr2` fallback was deliberately kept until the oracle test passes.
- `GOWORK=off` build/test and E2E (1-light CR2 stack, **7.5 s**) were green (209 self-tests at that point). The binary is no longer "single source file" but remains a single stdlib-only executable plus one first-party module.
- Current decoder implementation is the native pure-Go path described in §3, default-on, with `--no-native-cr2` restoring dcraw/ImageMagick (see **C1/R**); current self-test count and flag list are documented there, not here.

---

## Sources absorbed

- `GEMINI_CR2_BITEXACT_PROMPT.md` — archived Deep Research prompt (verified facts, 7 questions, deliverables, guardrails).
- `GEMINI_CR2_BITEXACT_PROMPT-outcome.md` — local research substitute, dcraw source findings, unresolved alignment, append-only `cr2_slice` refutation, revised next step, source URLs.
- `MODULARIZATION_ANALYSIS.md` — CR2-specific progress notes only: `Probe` byte-8 signature fix, goPureRaw v0.1.0→v0.1.1, import of `raw` + `formats/cr2`, removal of duplicate `pr*` decoder, kept `--no-native-cr2` fallback (full modularisation history → **C5**).
- `GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — reviewed; no CR2/native-decoding facts to absorb.
- `dcraw.c` v9.28 — `https://github.com/ncruces/dcraw/blob/master/dcraw.c` (source).
- RawTherapee mirror — `https://github.com/Beep6581/RawTherapee/blob/dev/rtengine/dcraw.c`.
- Debian sources — `https://sources.debian.org/src/dcraw/9.28-2/dcraw.c/`.
- Gemini Deep Research API docs — `https://ai.google.dev/gemini-api/docs/deep-research` (agents `deep-research-preview-04-2026`, `deep-research-max-preview-04-2026`; free tier rate limit observed 0 input tokens/minute).
- Current-status verification — `NEXT_SESSION_PROMPT.md`, `PROJECT_CONTEXT.md`, `CHANGELOG.md`, `TECHNICAL_SPEC.md`, `main.go` (`--no-native-cr2` flag, default-on native decode, "usable but not bit-exact" qualification).

LibRaw/CFA preservation notes from the cycle-6 corpus are indexed in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §9.

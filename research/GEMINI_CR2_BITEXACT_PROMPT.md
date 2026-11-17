# Gemini Deep Research prompt — Canon CR2 bit-exact mapping vs dcraw

Copy everything between the markers into Google Gemini Deep Research.

---

## PROMPT (copy from here)

You are researching a very specific reverse-engineering problem in a pure-Go astrophotography RAW decoder. I need an exhaustive, source-backed answer — not general advice about RAW files.

### 1. Context

Project: SkyStacker (single-file Go CLI, stdlib only, `CGO_ENABLED=0`) with a subproject **PureRaw** (`PureRaw/`, module `github.com/youruser/pureraw`). Relevant PureRaw files:

- `internal/ljpeg/ljpeg.go` — our lossless JPEG decoder (SOF3).
- `formats/cr2/decoder.go` — CR2 container probe, lossless-JPEG stream finder, two-plane → Bayer assembly.
- `formats/cr2/decoder_test.go` — `TestDecodeOracle` (runs with `PURERAW_CR2` env var).
- SkyStacker `main.go` contains an equivalent embedded copy (`prDecodeLJPEG`, `prDemosaicBayer`, `prDecodeCR2Native`).

Test file used: Canon EOS 40D CR2 (`IMG_7005.CR2`, 11,892,827 bytes). `dcraw` is installed on the host.

### 2. What we know (verified facts)

- CR2 header: `II*\0`, `"CR"` + version at bytes 4..7; IFD0 is a **thumbnail** (1936x1288, JPEG compression 6); IFD1 is a JPEG thumbnail; Exif IFD has MakerNote tag 0x927C (34,058 bytes).
- The raw image is a **lossless JPEG (SOF3)** stream found by scanning for `FF C3`:
  - SOI at file offset 1,366,852; DHT at 1,366,854; SOF3 at ~1,366,924; SOS shortly after.
  - SOF3: precision **14 bits**, height **2622**, width **1972**, **2 components** (ids 1 and 2, sampling 0x11).
  - SOS: `ns=2`, component selectors `01 00` and `02 10` (DC tables 0 and 1), final byte `0x00` → predictor selection 1.
  - DHT: two tables (ids 0 and 1), counts `[0,2,2,2,3,1,1,1,1,1,1,0,0,0,0,0]`, 15 symbols each: `[4,5,3,6,7,2,8,0,1,9,10,12,11,13,14]`.
  - Entropy stream is byte-stuffed (`FF 00`), no restart markers observed.
- Our decoder:
  - Canonical Huffman decode, MSB-first bit reader, difference value `v - (1<<S) + 1` when `v < 1<<(S-1)`.
  - **Critical fix already applied**: initial predictor `2^(P-1)` = 8192; at the start of each row predictor 1 uses the sample above (or the initial predictor on row 0). Without this fix the output looked random; with it the result is plausible.
  - Reconstructed two planes of 1972x2622 uint16 samples each.
  - Mapped to a 3944x2622 Bayer mosaic as even columns = plane 0, odd columns = plane 1; RGGB assumed; black=1018, white=16224 (EOS 40D values from dcraw `-v`).
- Measured results:
  - Our decoded full Bayer: min ~1, max **16372**, mean **1181.84**, horizontal diff mean 60.8, vertical diff mean 71.4.
  - `dcraw -D -4 -c file` → P5 3908x2602, 16-bit container, min 4, max **16011**, mean **1180.96**, horizontal diff 63.0, vertical 59.3.
  - `dcraw -D -4 -t 1 -c` → still 3908x2602 (not 3944x2622), so document mode appears to output a transposed/cropped buffer regardless of orientation flag.
  - Histograms/percentiles are very close, so the data is likely the same image content, but **spatial correspondence is unresolved**.
- Failed attempts to align: all 8 dihedral transforms (flip H/V + transpose) × crop offsets up to 80 px (downsampled 16x), even/odd column interleaving, left/right halves, row shifts ±2, plane swaps. Best normalized cross-correlation < 0.1 (essentially no match).

### 3. Research questions

1. **Canon CR2 lossless JPEG layout**: is there an authoritative description (dcraw source `lossless_jpeg_load_raw()`, LibRaw, ExifTool CR2 docs, DNG spec, RC paper) of how the two SOF3 components map to the sensor Bayer grid for EOS 40D-class cameras? Even/odd columns, left/right halves, even/odd rows, or something camera-specific?
2. **dcraw predictor semantics**: quote and explain dcraw's predictor handling for lossless JPEG with ` predictors=1`, especially the initial predictor, the first sample of each row, and any Canon-specific overrides. Does dcraw use a different predictor table than the JPEG spec?
3. **dcraw -D output transform**: in document mode, what exactly does dcraw write? Does it subtract black level, scale to 16-bit, transpose, crop, flip, or reorder Bayer? Give the exact code path and formulas (including `white`, `black`, `output_bps` handling).
4. **Why the max differs** (ours 16372 vs dcraw 16011): is this black/white scaling, white-balance multipliers applied even in `-D`, clipping, or a different decode?
5. **Open-source reference implementations**: how do LibRaw (`src/decoders/...`), rawpy, or ImageMagick decode Canon CR2 SOF3 streams? Is there a known 'two-component interleave' rule? Provide file/function names and code snippets.
6. **Validation methodology**: how do projects build tiny synthetic lossless JPEG / CR2 fixtures with known ground-truth samples to validate a decoder without a proprietary camera file? Any public test corpus with CC0 CR2 files and reference raw dumps?
7. **Go-specific**: given our bit reader and Huffman implementation, list plausible subtle bugs that preserve the histogram/mean but permute spatial order (e.g., MCU ordering, component scan interleaving, predictor context per component, stuffed-byte handling).

### 4. Required output

1. **Root-cause hypothesis ranking** — top 3 explanations for the failed alignment, with evidence for/against each from the numbers above.
2. **Exact algorithm** — how to convert our two planes into the same Bayer grid dcraw outputs (or how to reproduce dcraw's output transform), with pseudocode.
3. **Code-level fixes** — concrete changes to `internal/ljpeg` / `formats/cr2` (or SkyStacker's embedded copy), including predictor initialization and component mapping.
4. **Oracle test plan** — how to prove bit-exactness: synthetic fixture design, dcraw command-line flags that disable transforms, comparison metrics and tolerances.
5. **Sources** — numbered primary sources (dcraw source files with line numbers/function names, LibRaw source, official specs, papers) with dates and URLs. Mark forum reports as practitioner reports. Do not invent URLs or citations; if uncertain, say so.

### 5. Guardrails

- Be explicit about what is authoritative source code vs inference.
- If the answer requires camera-specific quirks, say which cameras/models are covered by each rule.
- Do not propose AI/neural upscaling or demosaicing as a workaround; the goal is bit-exact sensor data.
- Final language: English.

## PROMPT (copy until here)

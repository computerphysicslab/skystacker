# CR2 bit-exact investigation — local research outcome (2026-11-17)

**Provenance note:** the intended Gemini Deep Research run could not be executed:
the Gemini API returned `HTTP 429 — "limit: 0 input tokens per minute on Free
Tier"` for both `deep-research-preview-04-2026` and
`deep-research-max-preview-04-2026`. The document below is a **local web/source
research substitute**, not a Gemini report. The prompt remains archived in
`GEMINI_CR2_BITEXACT_PROMPT.md` for a paid-tier run.

## Findings from dcraw source (authoritative)

Source inspected: `dcraw.c` v9.28 (`ncruces/dcraw`, RawTherapee `rtengine/dcraw.c`,
Debian sources 9.28-2) — functions `ljpeg_start`, `ljpeg_row`,
`ljpeg_diff`, `lossless_jpeg_load_raw`.

1. **SOF3 metadata** — `ljpeg_start()` reads precision/height/width/components
   from SOF3 exactly as we do. It also computes
   `sraw = ((data[7] >> 4) * (data[7] & 15) - 1) & 3`; for sampling `0x11` this
   is 0, so the "sraw" path is not taken for EOS 40D.
2. **Huffman tables** — DHT entries are accepted with the same class/id byte
   convention; our canonical Huffman decoding and difference conversion
   (`v - (1<<len) + 1` when sign bit is 0) match `ljpeg_diff()`.
3. **Predictor** — `ljpeg_row()`:
   - at `col == 0`, predictor is `vpred[c]`, initialised to
     `1 << (bits-1)` and updated to the current col-0 sample each row;
   - at `col > 0`, predictor is `row[0][-clrs]` (left sample, same component);
   - `psv` comes from SOS byte `data[1 + data[0]*2]`; for our file it is 1.
   These match our corrected implementation (initial predictor 8192, row start
   uses sample above).
4. **Component mapping** — `lossless_jpeg_load_raw()` iterates
   `jwide = jh.wide * jh.clrs` decoded samples **row-serially**, writing them
   into `raw_image` with `RAW(row,col)=val` and wrapping `col` at `raw_width`.
   With `wide=1972`, `clrs=2`, `raw_width=3944`, samples alternate
   component 0/1 → our even/odd column interleaving is correct.
5. **Values** — dcraw applies `curve[*rp++]` (Canon linearization table from
   maker notes) before storing, and later black/white scaling when writing
   PPM. This plausibly explains max 16372 (ours) vs 16011 (dcraw) and the
   shifted percentiles.

## Still unresolved

Empirical alignment with `dcraw -D -4 -t 1` output (3908x2602) remains poor:
normalized cross-correlation < 0.1 and Spearman ~0.14 on aligned patches across
8 dihedral transforms + crop offsets. A 1-D column-profile correlation reaches
only ~0.44 at shift 9. Since source inspection says our geometry should be
right, the likely explanations are now:

- Canon linearization curve not applied to our samples (value transform, not
  geometry) — should not break rank correlation, so probably not the whole story;
- `cr2_slice[]` remapping on EOS 40D (dcraw applies a slice table when present)
  — this **would** permute pixels and is the leading hypothesis;
- crop/orientation applied by dcraw in document mode in a way not covered by
  our transform search.

## Concrete next steps (revised estimate)

1. Extract `cr2_slice` / linearization data from the EOS 40D maker note and
   apply it before comparison (1–2 h).
2. Build a synthetic CR2 with known samples and a tiny maker note, so the
   expected mapping is known without dcraw (30–45 min).
3. Repeat the oracle comparison with rank correlation after step 1; if it
   approaches 1, add bit-exact unit tests to PureRaw/SkyStacker (30 min).

Total focused work: roughly **2–4 h**, lower than the previous estimate now
that the source review confirms the decoder core is correct.

### Source check (2026-11-17, append-only): cr2_slice hypothesis refuted

A direct source check of `dcraw.c` v9.28 (ncruces/dcraw, downloaded and grepped
locally) shows that `cr2_slice[3]` is **only ever reset to zero** (line 8631) and
read in `lossless_jpeg_load_raw` / `canon_sraw_load_raw`; no Canon maker-note tag
assigns it in this version. The EOS 40D maker note Exif tag `0x0009` is a 32-byte
ASCII/artist field, not a slice table, and it is all zeros in
`assets/OrionNebula/lights/IMG_7005.CR2`. Therefore the "leading hypothesis"
above cannot explain the mismatch and the 1–2 h slice-extraction step should be
dropped from the work plan.

The remaining value differences must come from the pixel pipeline after JPEG
decode: `curve[]` is initialised to identity for this camera (no Canon
linearization table is loaded), while dcraw's output path applies black-level
subtraction (`cblack[]`) and white scaling before writing 16-bit PPM. The next
concrete step is an oracle comparison at the **raw Bayer plane** level (our
decoder vs `dcraw -D -4 -t 1` after cropping the margins and normalising black/
white), not another geometry search over dihedral transforms.

## Sources

- `dcraw.c` v9.28 — `https://github.com/ncruces/dcraw/blob/master/dcraw.c` (source).
- RawTherapee mirror — `https://github.com/Beep6581/RawTherapee/blob/dev/rtengine/dcraw.c`.
- Debian sources — `https://sources.debian.org/src/dcraw/9.28-2/dcraw.c/`.
- Gemini Deep Research API docs — `https://ai.google.dev/gemini-api/docs/deep-research`
  (agents `deep-research-preview-04-2026`, `deep-research-max-preview-04-2026`;
  free tier rate limit observed 0 input tokens/minute).

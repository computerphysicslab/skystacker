# SkyStacker roadmap

**Updated:** 2026-11-18 · **Current state:** v0.6.1, 212 registered self-tests,
Go 1.27.4, PureRaw v0.1.1.

Ordered by ascending estimated effort. Take the first pending item unless a
user request overrides it. Effort scale: **S** ≤ 1 h, **M** 1–4 h, **L** 4–8 h,
**XL** > 8 h. Each item lists the canonical research file that owns the
evidence; acceptance criteria there are authoritative.

## Completed (2026-11-17)

1. Finite-sample MAD correction for small N (`finiteMADScale`).
2. Go 1.27.4 toolchain upgrade and benchmark re-baseline.
3. Single-pass sigma-clip accumulation with bit-exact equivalence tests.
4. External AI CLI sidecar runner (`--external-bge/-denoise/-deconv/-starless`).
5. Order-2 SIP in the stacking resampler (`--resample-sip`).
6. Research folder consolidated into flat thematic files plus the ingestion
   script `scripts/consolidate-research.py`.

## Pending — easy to hard

| # | Milestone | Effort | Research owner | Acceptance / scope |
|---|---|---|---|---|
| 1 | **ASTAP interoperability hardening** | S (~45 min) | [02](research/02-astrometry-and-identification.md) §ASTAP audit | Correct the `%errorlevel%` HFD scale in docs, add a decoding unit test, prefer textual `HFD_MEDIAN`/`STARS`, clamp HFD > 21.46 px, verify `-ra` units. No plate-solver code changes. |
| 2 | **Encoded-space self-flat warning** | S (~30 min) | [04](research/04-calibration-and-self-flat.md) §15 | When the grid self-flat is applied in sRGB-encoded space and no linear-light option is active, `report.txt`/`report.json` state the approximation; no numerical change. |
| 3 | **Benchmark methodology extension** | S–M (~1 h) | [05](research/05-toolchain-and-modularity.md) §6, [01](research/01-architecture-and-core-engine.md) §14 | Extend the existing `BenchmarkSigmaClip*` harness to cover tile reduction and decode, report mean/CV, pin CPU governor where possible; document in `BENCHMARKS.md`. |
| 4 | **XISF Rev 2 conformance (`--xisf-rev2`)** | S–M (~1–2 h) | [05](research/05-toolchain-and-modularity.md) §11, [03](research/03-reference-and-literature.md) §4 | Strict sample-mapping bounds and corrupt-block rejection; SHA-256/512 natively, SHA-3 only after stdlib verification; malformed headers fail gracefully. |
| 5 | **BigTIFF float32 export (`--bigtiff`)** | M (~2–3 h) | [01](research/01-architecture-and-core-engine.md) §10, [05](research/05-toolchain-and-modularity.md) §11 | 64-bit `LONG8` offsets/byte counts for stacks > 4 GiB; round-trip selftest reads the written file back. |
| 6 | **Validate gamma-aware self-flat** | M (~2 h) | [04](research/04-calibration-and-self-flat.md) §15 | `--self-flat-linear-srgb` exists but is unvalidated; compare encoded vs EOTF-corrected division on a real dataset, keep default unchanged, document the result. |
| 7 | **Catalog annotation / anomaly extension** | M (~3–4 h) | [02](research/02-astrometry-and-identification.md) §4 | Complete the existing `--annotate` baseline: elliptical apertures, multi-catalog cross-match, `SNR > 5` unassociated-source flag; register `testWCSCatalogOverlay` and `testUnassociatedSourceFlagging` (round-trip ≤ 0.05 px). |
| 8 | **Spatially gridded PSF for Richardson–Lucy (`--deconvolve-grid`)** | M (~3–4 h) | [01](research/01-architecture-and-core-engine.md) §15.3 | Local-star PSF over a 3×3/4×4 grid with bilinear interpolation, opt-in, star-masked; synthetic wide-field test showing reduced corner degradation. |
| 9 | **Pure-Go FITS Rice (`.fits.fz`) + `fitsio/` module** | M–L (~4–5 h) | [05](research/05-toolchain-and-modularity.md) §11 | Golomb–Rice tile codec, bit-exact integer/quantized round-trip; extract into a tagged sibling `fitsio/` module following the PureRaw pattern. |
| 10 | **CR2 bit-exact validation vs dcraw** | M–L (~3–4 h) | [07](research/07-native-cr2-decoding.md) §6 | Compare raw Bayer planes after cropping margins and black/white normalisation; explain 16372/1181.84 vs 16011/1180.96 and the old NCC < 0.1. `cr2_slice` is refuted; do not repeat that search. |
| 11 | **IVOA ProvenanceDM emission** | M–L (~3–4 h) | [01](research/01-architecture-and-core-engine.md) §16 | Per-run/per-stage provenance graph (binary version, plate-solve tolerances, tile/rejection/stretch parameters, exact catalogue/cache version) in `report.json`/VOTable under `skystacker_results/`. |
| 12 | **Real-flat validation gate for grid self-flat** | M–L (~3–4 h) | [04](research/04-calibration-and-self-flat.md) §15 | Single-process ablation toggling only the illumination source; require a dataset with ≥ 15 lights and a real flat; publish the numeric comparison, no dataset modification. |
| 13 | **Per-frame astrometric reprojection** | L (~5–6 h) | [02](research/02-astrometry-and-identification.md) §11 | Replace the shared-distortion assumption of `--resample-sip` with per-light WCS solutions (SIP order 2 including AP/BP linear terms); synthetic corner-residual gate < 0.2 px, < 15 % runtime overhead. |
| 14 | **CFA/Bayer drizzle at scale 1.0 (`--bayer-drizzle`)** | L (~6–8 h) | [01](research/01-architecture-and-core-engine.md) §7 | Raw Bayer planes via `dcraw -D -4`, square flux-proportional kernel, droplet 0.7–1.0, per-channel coverage weights, skip null samples, warn for N < 15 / integer-locked dithers; flux within 0.1 %. |
| 15 | **CR3 native decoding** | L–XL (~8 h) | [07](research/07-native-cr2-decoding.md) §0 | Remove the external-decoder dependency for CR3 input; pure-Go HEVC/HEIF container + lossless payload path, gated behind a flag until oracle-validated. |
| 16 | **PCC/SPCC + PSF-fitting photometry** | L–XL (~8 h) | [01](research/01-architecture-and-core-engine.md) §13 | Linear-data colour calibration against Gaia, PSF-fitting ensemble photometry with MAD rejection; opt-in, provenance-tagged. |
| 17 | **Observatory automation sidecar** | L–XL (~8 h) | [01](research/01-architecture-and-core-engine.md) §16 | External daemon writing an Alpaca-property → FITS-keyword mapping before header sealing; metadata ingestion only, interactive control out of scope. |

## Deferred / explicitly rejected

Recorded so they are not reopened without new evidence:

* Embedded ONNX/GGUF/WASM inference and native BXT/NXT/SXT equivalents
  (284–456 MB weights, incompatible with the static stdlib-only binary) —
  [06](research/06-external-ai-sidecars.md) §2.
* `simd/archsimd` (experimental, amd64-only GOEXPERIMENT) and a GPU HAL for
  current workloads — [01](research/01-architecture-and-core-engine.md) §4.2.
* 2× mono/CFA upscale drizzle as a general mode, GZIP FITS, synthetic
  bias/dark/flat generation — [01](research/01-architecture-and-core-engine.md)
  §7, [05](research/05-toolchain-and-modularity.md) §11,
  [04](research/04-calibration-and-self-flat.md) §12.
* Fully general multi-night mosaics and real-time observatory control are out
  of scope; per-frame reprojection (item 13) is the enabling research step,
  not a commitment to mosaicking.

## Maintenance

After each milestone: rebuild the canonical binary, rerun `--selftest`,
`go vet`, `go test`, update the self-test count in `prompt-ai.txt`,
`PROJECT_CONTEXT.md` and `TECHNICAL_SPEC.md`, and record new evidence in the
owning canonical research file (never in a new partial file). Use
`scripts/consolidate-research.py` for incoming research contributions.

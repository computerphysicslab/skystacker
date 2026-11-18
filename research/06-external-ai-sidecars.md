# SkyStacker — External AI Sidecars

**Canonical file:** [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) · Part of the SkyStacker research corpus.

**Scope.** This is the single canonical, deduplicated, English-language document for SkyStacker's optional external AI/CLI orchestration. It absorbs and supersedes:

- `research/09-external-ai-sidecar-2026-11-17.md` (implementation note and smoke evidence);
- the external-AI/ML portions of `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` (§E "AI/ML Advances", the embedded-inference feasibility assessment, the feasibility-matrix rows, Proposal 4, and the "Things Explicitly NOT Worth Pursuing" list);
- the BXT/NXT/SXT discussion in `research/05-research-deep-dive-2026-11-16.md` §13 **only where it bears on the sidecar decision**; the glossary definitions and analysis home remain [03](03-reference-and-literature.md) §5.

Where the earlier files disagreed or used different flag names, one answer is given here.

**Status (current).** SkyStacker is **v0.6.1** with **231 registered self-tests**. Proposal 4 is **implemented** as a runner only: the binary ships **no ML runtime, model weights, or GPU code**. It orchestrates user-installed, licensed executables through a private directory of 32-bit float FITS files. The feasibility matrix classifies this as *Partial — CLI runner Yes, ML No*.

---

## Scope boundaries — do not duplicate here

| Canonical file | Topic (cross-linked, not repeated in this document) |
| :--- | :--- |
| [`01-architecture-and-core-engine.md`](01-architecture-and-core-engine.md) | Core engine, post-processing stage order, resampling, `--resample-sip`, Go runtime/toolchain status |
| [`02-astrometry-and-identification.md`](02-astrometry-and-identification.md) | Plate solving, TAN/SIP WCS maths, `plate_solution.json`, Gaia catalogues |
| [`03-reference-and-literature.md`](03-reference-and-literature.md) §5 | BXT/NXT/SXT glossary definitions (G76–G80), poor-seeing wall, classical-equivalent analysis, bibliography/URLs — **do not duplicate definitions here** |
| [`04-calibration-and-self-flat.md`](04-calibration-and-self-flat.md) | Calibration masters, dark-series QA, gradient detrending, validation |
| [`05-toolchain-and-modularity.md`](05-toolchain-and-modularity.md) | Go toolchain upgrade history, `simd/archsimd` amd64-only GOEXPERIMENT detail, modularisation analysis |
| [`06-external-ai-sidecars.md`](06-external-ai-sidecars.md) | **This file** — ecosystem survey, infeasibility, linear-stage safety, FITS sidecar runner, limits |
| [`07-native-cr2-decoding.md`](07-native-cr2-decoding.md) | Native pure-Go CR2 / lossless-JPEG decoding, bit-exact investigation |
| [`08-crawler-source-discovery.md`](08-crawler-source-discovery.md) | goCrawler discovery cycles, curated corpus, source ranking |
| [`README.md`](README.md) | Project overview, CLI flag reference, capability tiers (Tier 4∞ = no embedded neural inference) |

Calibration history, toolchain history and BXT glossary definitions are deliberately *not* repeated below; follow the links.

---

## 1. Ecosystem survey (2025–2026)

The survey below is stated **once**. Later sections refer back to it rather than restating versions, prices or CLI syntax. All entries are external, optional and user-supplied; none is bundled with SkyStacker.

| Vendor / tool | Version & date | Price / licence | CLI interface (as documented) | SkyStacker role |
| :--- | :--- | :--- | :--- | :--- |
| **RC Astro** — BlurXTerminator (BXT), NoiseXTerminator (NXT), StarXTerminator (SXT) | Standalone cross-platform CLIs **June 2026**; **BXT v2.6.6 ("ML5")**; APP `2.0.0-beta47` support noted **Oct 2026** | Commercial, licence-gated; reported **$100 BXT / $60 NXT** (practitioner guide) | Binaries `rc-astro.exe` / macOS / Linux; per-tool subcommand syntax is **not** fixed by the survey — passed through as a user-supplied `CMD` | BXT→`--external-deconv`, NXT→`--external-denoise`, SXT→`--external-starless` |
| **GraXpert** (Steffenhir) | **v3.0 / v3.1+**, 2025–2026 | Free, open-source (GPL) | `GraXpert -cli` with `-cmd background-extraction`, `-cmd denoising`, `-cmd deconv-obj` (v3.1+); **ONNX Runtime 1.20**, CUDA 12 / DirectML / CoreML, `-gpu false` CPU fallback; example: `GraXpert -cli -cmd denoising -gpu false -i {input} -o {output}` | `--external-bge`, `--external-denoise`, `--external-deconv` |
| **Seti Astro Cosmic Clarity / Seti Astro Suite Pro (SASpro)** | **v6.5 sharpen/denoise Dec 2025**, **Dark Star v2.1c Jun 2025**, **Satellite Trail Removal v2.3 Jan 2025**; SASpro `1.21.0` in the 2026 changelog | GPLv3 suite | Headless CLI in the GPLv3 suite; walking-noise and linear restoration models; supports non-CUDA GPUs | `--external-denoise`, `--external-deconv` |
| **StarNet2** (free) + **ASTAP** integration | Free CLI; ASTAP adoption for comet stacking noted **~Sep 2026** | Free; Windows GPU acceleration via `DirectML.dll` | CLI; exact argv left to the user's `CMD` | `--external-starless` |
| **MURE** (practitioner reports) | **March 2025** Cloudy Nights artifact thread | n/a (reference point) | n/a — comparative guidance only | Informs ordering (§3) |

Notes (each fact cited once here):

- The RC Astro CLIs remove PixInsight/Photoshop **host lock-in** but remain **licence-gated**; the survey's exact per-tool CLI grammar is not authoritative, hence the runner treats the whole command as an opaque `CMD` template. See [03](03-reference-and-literature.md) §5.1 for the glossary definitions of BXT/NXT/SXT.
- GraXpert's `-cli` broadened from background extraction to **linear denoising** and **object/stellar deconvolution**; DSPX integration docs note it **may connect externally to list/download AI models if not cached locally** — a reason the runner enforces a bounded timeout (§4.3).
- Cosmic Clarity/SASpro adds walking-noise and linear restoration models; the suite's full-memory stacking can require **~32 GiB RAM**, which reinforces SkyStacker's tiled spill-file design ([01](01-architecture-and-core-engine.md)) rather than any in-process equivalent.
- MURE denoise was reported competitive with NXT **only before deconvolution** (March 2025 practitioner thread) — one input to the linear-stage ordering rule in §3.

---

## 2. Why embedded ML is infeasible

Embedding ONNX, GGUF or WASM neural inference inside SkyStacker's single-file, standard-library-only, `CGO_ENABLED=0` static binary is **infeasible** and is formally a *Do not implement* item:

- **Model size.** GraXpert's linear ONNX **denoise weights alone are 284 MB (v2) → 456 MB (v3)**, with deconvolution heavier still. Even browser WebGPU apps run BGE/denoise in-browser but delegate GraXpert **deconvolution to the native CLI**. A 284–456 MB payload cannot be folded into a portable single-file binary.
- **Compute.** Executing multi-layer U-Nets over **2602×3908** (≈10–24 MP) float32 RGB stacks in pure Go, without GPU/CUDA/DirectML/CoreML or CGO/BLAS bindings, incurs prohibitive CPU latency on the reference host (i7-8565U, 4c/8t, **15 W ULV**, **15 GiB RAM**).
- **No evidence to the contrary.** No 2024–2026 benchmark of pure-Go GGUF or WASM execution of astro-restoration models was found.
- **Licensing/host coupling.** Calling a licensed RC Astro plugin from the Go binary is neither a reproducible stdlib-only path nor unlicensed; the BXT/NXT/SXT analysis home is [03](03-reference-and-literature.md) §5.3, and the capability tier (Tier 4∞) is in [`README.md`](README.md).
- **SIMD is not an escape hatch.** `simd/archsimd` is an off-by-default `GOEXPERIMENT` restricted to **amd64**, breaking reproducible cross-platform builds and leaving Apple Silicon (arm64) unaccelerated; full detail and toolchain history live in [05](05-toolchain-and-modularity.md).

**Decision:** orchestrate external, user-installed executables as sidecars using the DSPX-style **private-temporary-directory, 32-bit float FITS handoff via `os/exec` (no shell)**. This delivers the same user capability while preserving the `CGO_ENABLED=0` static binary. Other rejected items (2× mono/CFA drizzle, GZIP-compressed FITS, plugin architecture, REST API) are listed in [03](03-reference-and-literature.md)/[README.md](README.md) and are not repeated here.

---

## 3. Linear-stage safety and failure modes

Restoration is **locked to linear integrated masters**. The documented failure modes (survey §E and the deep-dive §13) are:

- **Plastic-looking stars** when denoise strength exceeds **0.7**.
- **Unpredictable artifacts / hallucinations** when BXT-class deconvolution runs on **nonlinear/stretched data**.
- **Stack SNR degradation** when denoise/deconvolution runs on **unstacked subframes**.
- **Ringing / artifacts** when deconvolution runs after stretching or after prior denoising.
- **Poor-seeing wall:** if seeing smears fine divisions across several pixels, contrast is already lost pre-stacking and no deconvolution (classical or neural) recovers it; planetary mitigations are external workflows (lucky imaging, wavelets, derotation). Definitions and the full analysis are in [03](03-reference-and-literature.md) §5.2; do not duplicate them here.

Enforced consequences:

1. The sidecar is a **post-stack** stage — it never runs on individual lights (matches the "no per-subframe use" limit in §5).
2. `--external-denoise` and `--external-deconv` are refused whenever `frame.Nonlinear` is set, unless the user explicitly passes `--external-allow-nonlinear`. This guard exists precisely because BXT/NXT-style models expect linear data.
3. External stages are inserted while the frame is still linear: background extraction before the built-in background pass, and denoise/deconv after background removal but before the built-in deconvolution/STF stretch (§4.6). BGE and starless are not gated by the nonlinear flag because they do not perform the restoration that produces the documented artifacts.

---

## 4. Implementation — the shipped FITS sidecar runner

This is the **single** implementation description. It corresponds to Proposal 4 of the Gemini survey and to the shipped code in `main.go` (v0.6.1).

### 4.1 Flags and argv contract

- `--external-bge CMD`, `--external-denoise CMD`, `--external-deconv CMD`, `--external-starless CMD` — each takes a **whitespace-separated argv** template (`main.go:482`–`486`), with `{input}` and `{output}` placeholders required by `parseArgs` (`main.go:640`–`652`).
- `--external-allow-nonlinear` — override for the denoise/deconv linear guard (`main.go:486`).
- `--external-timeout N` — per-stage timeout in seconds, **default 1800 s**, `0` disables the timeout (`main.go:487`, validated `>= 0` at `main.go:637`).
- `CMD` is split with `strings.Fields`; each `{input}`/`{output}` token is replaced by the private temp path, and `exec.CommandContext(argv[0], argv[1:]...)` is used — **no shell interpolation** (`main.go:3568`–`3592`).

### 4.2 FITS transport contract

`writeFloat32RGBFITS` (`main.go:3330`) emits a single-image FITS whose header is:

- `SIMPLE = T`, **`BITPIX = -32`** (IEEE big-endian float32), **`NAXIS = 3`**, **`NAXIS3 = 3`** (RGB, channel-major `[plane][row][col]`), `BSCALE = 1.0`, `BZERO = 0.0`;
- 80-byte cards padded to a 2880-byte header block, data padded to a multiple of 2880 bytes;
- a **TAN WCS** (`CTYPE1='RA---TAN'`, `CTYPE2='DEC--TAN'`, `CRVAL*`, `CRPIX*`, `CD*_*`) written from the frame's `plate_solution.json`-derived WCS when available, otherwise `CTYPE='LINEAR'`.

The returned file is parsed by `readFloat32RGBFITS` (`main.go:3409`), which strictly requires `BITPIX=-32 / NAXIS=3 / NAXIS3=3`, positive dimensions and sufficient data, and decodes big-endian float32; it also re-reads a TAN WCS if present.

### 4.3 Process execution, timeouts and diagnostics

- `exec.LookPath(argv[0])` runs first; a **missing binary** returns a diagnostic naming the stage and executable and advising installation/PATH: `external AI <stage>: executable "<bin>" not found: … (install the CLI and put it on PATH)` (`main.go:3583`–`3585`).
- `context.WithTimeout` bounds each stage; combined stdout/stderr is captured and the **last 2000 bytes** are appended to any error message (`main.go:3586`–`3602`).
- An empty command is rejected up front (`main.go:3541`); a bad/non-FITS output, wrong header, or truncated data produces a descriptive `read output FITS` error.

### 4.4 WCS round-trip and dimension checks

- The output must have **exactly the input dimensions**; otherwise the run fails with `output WxH != input WxH` (`main.go:3607`–`3609`).
- If `f.WCS != nil` and the returned WCS differs (`wcsTanEqual`, `main.go:3514`), the input WCS is **restored** and a warning is logged distinguishing "tool dropped WCS" from "tool changed WCS"; a WCS-preserving tool round-trips bit-for-bit (`main.go:3610`–`3617`).

### 4.5 Cleanup

A private directory is created with `os.MkdirTemp("", "skystacker-ai-"+stage+"-")` and removed via `defer os.RemoveAll(dir)` (`main.go:3550`–`3554`), so `input.fits`/`output.fits` are deleted on **success and every error path**.

### 4.6 Pipeline insertion points

`extTimeout := time.Duration(opts.ExternalTimeoutSec) * time.Second` (`main.go:23123`). Stages are invoked in `runPostProcessing` as follows:

- `--external-bge` before the built-in multiscale/degree background removal; result written to `_work/external_bge.f32raw` (`main.go:23124`–`23134`).
- `--external-denoise` then `--external-deconv` after background removal (`step1Frame`), before built-in Richardson–Lucy/`--wavesharp` and the STF stretch; each result written to `_work/external_<stage>.f32raw` (`main.go:23150`–`23170`).
- `--external-starless` **in place of** the built-in `removeStars` when `--starless`/`--external-starless` is set, still in the linear domain before auto-stretch (`main.go:23238`–`23258`).

An error from any stage aborts post-processing and propagates the diagnostic; work-file write failures are logged as warnings rather than fatal.

### 4.7 Tests and smoke evidence

`testExternalAISidecar` (`main.go:14876`) is registered in the self-test table (`main.go:16456`) and verifies:

- the **self-executing mock**: the binary re-invokes itself with `--internal-ai-sidecar-mock` (`main.go:24341`–`24366`), passing the private temp dir back through a marker file so cleanup can be asserted;
- input FITS is `SIMPLE`, `BITPIX=-32`, `NAXIS=3`, `NAXIS3=3`, big-endian, channel-major, with the TAN WCS from the test frame;
- returned buffer has identical dimensions **and pixel values**, and the WCS round-trips (`wcsTanEqual`);
- the temp directory no longer exists after the call (`os.Stat` → `IsNotExist`);
- a missing executable yields a `not found` diagnostic; a nonlinear frame is refused for `denoise` but accepted with `allowNonlinear=true`; `--external-timeout 7` parses; and a command missing `{output}` is rejected.

**E2E smoke:** `--post-only --external-bge … --external-denoise …` completed with `_work/external_bge.f32raw` and `_work/external_denoise.f32raw` written (implementation note, 2026-11-17). The mock self-test is part of the **231** registered self-tests at v0.6.1.

---

## 5. Limits and open items

- **Arguments containing spaces are unsupported.** `strings.Fields` cannot represent a quoted argument; use a wrapper script or a space-free path.
- **Output format is fixed.** Only `BITPIX=-32`, `NAXIS=3`, 3-plane big-endian float32 FITS is accepted. A tool that writes **16-bit or monochrome** FITS needs a conversion wrapper to that contract.
- **No per-subframe use.** The runner is post-stack only; running restoration on individual lights is deliberately excluded (§3 failure modes).
- **No embedded inference.** Weights/runtime/GPU are not shipped; the tools are user-supplied and licensed (§2). This is a permanent constraint, not a backlog item.
- **Drizzle remains open.** True multi-frame / CFA-Bayer drizzle, per-frame astrometric reprojection and the `--deconvolve-grid` spatially gridded local-PSF upgrade are tracked elsewhere ([01](01-architecture-and-core-engine.md), [02](02-astrometry-and-identification.md)); they are independent of the sidecar runner.
- **Exact RC Astro CLI grammar is not pinned by the survey**, so the command template is the user's responsibility; no per-tool validation beyond `{input}`/`{output}` presence is performed.
- **Model download on first use** (e.g. GraXpert fetching uncached ONNX models) is bounded by `--external-timeout`, not pre-fetched or vendored.

---

## Sources absorbed

The following source material was read and merged/deduplicated into this document; no other file was modified.

1. `research/09-external-ai-sidecar-2026-11-17.md` — implementation note: flags, `--external-allow-nonlinear`, `--external-timeout` default 1800 s, `strings.Fields` argv with `{input}`/`{output}`, `os/exec` no shell, private temp dir with 32-bit big-endian `BITPIX=-32` `NAXIS=3` RGB FITS, WCS from `plate_solution.json` and restoration, dimension checks, missing-binary diagnostics, cleanup, `testExternalAISidecar` self-executing mock, E2E smoke outputs, limits → §4, §5.
2. `research/GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` — §E "AI/ML Advances Relevant to a Non-ML Binary" (external CLI ecosystem, RC Astro June 2026 / BXT v2.6.6 ML5 / $100–$60, GraXpert v3.0/3.1+ `-cli` commands and ONNX Runtime 1.20 / GPU flags, Cosmic Clarity/SASpro, StarNet2/ASTAP Sep 2026, failure modes denoise >0.7, MURE March 2025, 284–456 MB weights, DSPX sidecar pattern); embedded-inference infeasibility; feasibility matrix rows; Proposal 4 acceptance criteria; "Things Explicitly NOT Worth Pursuing" → §1, §2, §3, §4.
3. `research/05-research-deep-dive-2026-11-16.md` §13 — BXT/NXT/SXT mechanism, poor-seeing wall, classical-equivalent discussion and the decision that native inference is out of scope → §2, §3 (glossary definitions and full analysis remain in [03](03-reference-and-literature.md) §5).
4. `main.go` (v0.6.1) — `writeFloat32RGBFITS`/`readFloat32RGBFITS`, `runExternalSidecar`, flag definitions/validation, `runPostProcessing` insertion points, `testExternalAISidecar`, `--internal-ai-sidecar-mock` → §4 (implementation detail verified, not absorbed as research claims).

Supporting context (read for accurate cross-links/status only, not absorbed as new research): `research/README.md`, `research/01-architecture-and-core-engine.md`, `research/02-astrometry-and-identification.md`, `research/03-reference-and-literature.md`, `research/04-calibration-and-self-flat.md`, and the canonical cross-link file list.

ASTAP StarNet2 comet splitting details from the cycle-6 corpus are in [08-crawler-source-discovery.md](08-crawler-source-discovery.md) §9.

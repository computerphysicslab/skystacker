# External AI CLI sidecar runner — implementation note (2026-11-17)

## Scope

Proposal 4 of `GEMINI_DEEP_RESEARCH_PROMPT-outcome.md` is implemented as an
optional post-processing path. SkyStacker still ships no ML runtime: it only
orchestrates user-installed executables (GraXpert, StarNet2, RC-Astro CLIs) via
a private temporary directory of 32-bit float FITS files.

## Interface

- `--external-bge CMD`, `--external-denoise CMD`, `--external-deconv CMD`,
  `--external-starless CMD`: whitespace-separated argv (no shell), requiring
  `{input}` and `{output}` placeholders, e.g.
  `GraXpert -cli -cmd denoising -gpu false -i {input} -o {output}`.
- `--external-allow-nonlinear`: denoise/deconv stages are otherwise refused
  when `frame.Nonlinear` is set, because BXT/NXT-style models expect linear
  data and produce artifacts on stretched pixels.
- `--external-timeout N` (default 1800 s, `0` = no timeout).

## Guarantees verified by `testExternalAISidecar`

- The input FITS is `SIMPLE`, `BITPIX=-32`, `NAXIS=3`, `NAXIS3=3`, big-endian,
  channel-major, and carries the TAN WCS from `plate_solution.json` when one is
  available.
- The returned buffer must have exactly the input dimensions. If the tool
  drops or changes the WCS, the input WCS is restored on the returned frame and
  a warning is logged; a preserving tool round-trips WCS bit-for-bit.
- `exec.CommandContext` with `os/exec` only: no shell interpolation. A missing
  executable returns a clear diagnostic that names the stage and binary.
- The temp directory is removed on success and on every error path. The
  self-test uses the binary itself as a mock sidecar (`--internal-ai-sidecar-mock`)
  and asserts the directory no longer exists after the call.
- End-to-end smoke: `--post-only --external-bge … --external-denoise …`
  completed with `_work/external_bge.f32raw` and
  `_work/external_denoise.f32raw` written.

## Limits

- `CMD` is split with `strings.Fields`; arguments containing spaces are not
  representable. Use a wrapper script or a path without spaces.
- Only `BITPIX=-32`, `NAXIS=3`, 3-plane output is accepted. Tools that write
  16-bit or monochrome FITS need a conversion wrapper.
- The sidecar is called after background removal and before built-in
  deconvolution/stretch (or in place of built-in star removal for
  `--external-starless`). It does not run on individual subframes.

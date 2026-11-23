# SkyStacker documentation

SkyStacker is a standard-library-only Go command-line astrophotography stacker
with an embedded pure-Go CR2 decoder, Gaia-based astrometric plate solving,
blind-solve fallbacks, scientific image writers and machine-readable reports.

This directory is the structured, user-facing documentation. It complements
`README.md` (project overview) and `research/` (design evidence and literature).
All documents are written in English.

## Document map

| Document | What it covers |
|---|---|
| [installation.md](installation.md) | Toolchain, build, install on PATH, nested Go modules, CI verification. |
| [quickstart.md](quickstart.md) | First stack, single-image processing, typical session workflow. |
| [cli-reference.md](cli-reference.md) | Every CLI flag grouped by subsystem, with defaults and dependencies. |
| [plate-solving.md](plate-solving.md) | All plate-solving paths, real-command examples, WCS outputs and integration recipes. |
| [astrometry-products.md](astrometry-products.md) | Layered SVG sky maps, HiPS cutouts, annotation, cone search, SSO velocity vectors and photometry. |
| [calibration-and-stacking.md](calibration-and-stacking.md) | Master calibration, frame selection, rejection, weights, XNML ingestion. |
| [post-processing.md](post-processing.md) | The eight display steps, presets and opt-in enhancements. |
| [output-formats.md](output-formats.md) | TIFF/FITS/XISF/BigTIFF, float32 raw planes, layered SVG, CSV and JSON schemas. |
| [integration.md](integration.md) | Calling SkyStacker from another application: exit codes, reports, batch mode, remote use. |
| [troubleshooting.md](troubleshooting.md) | Common failures and the exact logs that identify them. |
| [catalog-attribution.md](catalog-attribution.md) | Third-party catalogue notices for the offline annotation bundle. |
| [catalogs/README.md](../catalogs/README.md) | Published magnitude-limited Gaia quad indexes, download, checksums and publishing workflow. |
| [blind-corpus-results.md](blind-corpus-results.md) | Real 14-image blind-solving corpus: commands, results and diagnosis. |

## Conventions used in examples

- Commands use `skystacker` as the executable name; substitute the absolute path
  from [installation.md](installation.md) if it is not on your `PATH`.
- Image paths such as `assets/OrionNebula/lights/IMG_7005.CR2` refer to the
  datasets shipped in this repository. They are real files, not placeholders.
- Output paths under `/tmp/` are scratch directories used for examples; replace
  them with your own results directory in production.
- Every example can be checked locally: `--version` prints the build version and
  `--selftest` runs the embedded regression suite (298 registered cases in
  v0.6.1).

## Status

Documentation reflects SkyStacker **v0.6.1** on **Go 1.27.4**. The authoritative
roadmap and status counts live in `ROADMAP.md`; algorithms and source evidence
live in `research/`, including §19–§21 of `research/02` for star-catalog export,
diffuse-object classification, VO services and WCS refinement. When code
changes, update the affected page in this folder and the self-test count in
`prompt-ai.txt`, `PROJECT_CONTEXT.md` and `TECHNICAL_SPEC.md`.

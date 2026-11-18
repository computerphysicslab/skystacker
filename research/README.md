# Research — consolidated index

This directory is the single source of truth for SkyStacker research. It is
flat by design: **no subfolders**, one canonical Markdown file per theme, and
no duplicated content between files. Raw contributions (Gemini Deep Research,
Google DeepMind, goCrawler, curated corpus dumps, local analyses) are
temporary: their unique facts are folded into the canonical theme files and
the contribution files are then removed.

## Canonical files

| File | Scope |
|---|---|
| [README.md](README.md) | This index, current status and the consolidation protocol |
| [01-architecture-and-core-engine.md](01-architecture-and-core-engine.md) | Pipeline workflows, memory model, star detection/registration, stacking math, post-processing, diagnostics, formats, CLI reference, ASTAP interop, roadmap and status |
| [02-astrometry-and-identification.md](02-astrometry-and-identification.md) | Plate solving, TAN/SIP, catalogues, annotation/anomalies, morphology, time domain, IVOA, HEALPix verification and implementation status |
| [03-reference-and-literature.md](03-reference-and-literature.md) | One master bibliography, one glossary/index, microarchitecture reference, file formats and BXT/NXT/SXT analysis |
| [04-calibration-and-self-flat.md](04-calibration-and-self-flat.md) | Master frames, physical models, recoverability table, self-flat v1/grid/IRLS, validation record, refusal policy, detector physics and open gaps |
| [05-toolchain-and-modularity.md](05-toolchain-and-modularity.md) | Go toolchain chronology and final state, Go 1.24–1.27 gains, modularization decision, PureRaw module, file-format interoperability backlog |
| [06-external-ai-sidecars.md](06-external-ai-sidecars.md) | GraXpert / RC-Astro / StarNet2 ecosystem, linear-stage safety, shipped FITS sidecar runner and limits |
| [07-native-cr2-decoding.md](07-native-cr2-decoding.md) | CR2 stream facts, decoder measurements, dcraw v9.28 source check, `cr2_slice` refutation and open validation |
| [08-crawler-source-discovery.md](08-crawler-source-discovery.md) | goCrawler method, robots.txt policy, all run cycles, master source table and curated excerpts index |

Each canonical file ends with a `## Sources absorbed` list naming the
original contribution files whose unique content it now owns. No other
`.md` file in this directory is canonical.

## Current state

SkyStacker v0.6.1, Go 1.27.4, 212 registered self-tests. Native CR2 decoding
and the PureRaw library are in use; `--resample-sip`, fused sigma-clip,
external AI sidecars and FITS/XISF writers are shipped. Still open: CFA/Bayer
drizzle, FITS Rice, BigTIFF, XISF Rev 2, per-frame astrometric reprojection
and bit-exact CR2 validation. Authoritative code-facing docs are the root
`README.md`, `TECHNICAL_SPEC.md`, `PROJECT_CONTEXT.md` and `BENCHMARKS.md`.

<!-- consolidation-status:start -->
Last consolidation: 2026-11-18
Canonical theme files: 8
Pending incoming files: none
<!-- consolidation-status:end -->

## Consolidation protocol

New research material arrives as loose `.md` files dropped into this
directory (for example `GEMINI_...-outcome.md`, `07-crawler-....md`,
`crawler-cycle-N.md`, `deep-research-....md`). Do not edit those files into
permanent documents and do not create subfolders. Instead:

1. **Drop** the contribution files into `research/` with any name.
2. **Preview** the consolidation:

   ```bash
   python3 scripts/consolidate-research.py
   ```

   The script prints, for each incoming file, the target theme, the number of
   unique paragraphs and the number of duplicates skipped. Nothing is written.
3. **Resolve ambiguity** if a file has no clear theme. Either rename it with
   a theme keyword or pass an explicit mapping:

   ```bash
   python3 scripts/consolidate-research.py --map incoming.md=02-astrometry-and-identification.md
   ```

4. **Apply**:

   ```bash
   python3 scripts/consolidate-research.py --apply
   ```

   Unique paragraphs are appended to the target theme file under an
   `## Imported from ...` heading with a provenance line. The incoming file is
   then deleted. Exact paragraph-level deduplication is performed against all
   canonical files; paraphrased duplication cannot be detected mechanically,
   so review the preview and edit the contribution before applying `--apply`
   when a file spans several themes.
5. **Verify** with `git diff`, `gofmt -l .`, `go vet ./...` and
   `./skystacker --selftest`, then commit only when explicitly asked.

Rules:

* Canonical files are never removed or renamed by the script. Only files not
  in the canonical list are consumed.
* One fact lives in one file. Cross-references use relative links, not copies.
* Historical claims stay inside the owning canonical file as clearly dated
  history; do not create a second "historical" file.
* Non-Markdown attachments (logs, fixtures) belong in `assets/testdata/` or
  outside `research/`; the script warns about them but never deletes them.
* The script is idempotent: after a successful run, a second run reports no
  incoming files.

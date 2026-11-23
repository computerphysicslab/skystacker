# Annotation example corpus

This folder holds reference material for the layered annotation subsystem
(roadmap items 137–141 and 143). It is a documentation/QA corpus: the files are
never read by the binary at runtime.

## Vendor-style annotated images

The `*.jpg` files are real vendor-style annotated screenshots included as visual
references for the expected layered result (grids, object shapes, labels and
finding-chart insets). They are supplied by the upstream research material and
are used only for visual comparison; no pixel-level assertion is made against
them. They are gitignored (copyrighted local reference material); only the
synthetic fixture and this README are versioned.

## M31 examples (2026-11-23)

Two new local WEBP references were added by the user. They are gitignored like
the vendor JPGs and are used for visual acceptance only (no pixel assertions):

- `m31camera-annotated.webp` (640x572) — wide field around Andromeda. Readable
  labels mix proper names (`Mirach`, `Alpheratz / Sirrah`) with Flamsteed
  numbers (`45 And`, `65 Psc`, `69 Psc`) and the constellation name
  `Andromeda`; stars are small open circles. Acceptance mapping: roadmap item
  155 (offline proper-name/Bayer/Flamsteed catalogue and cross-match priority).
- `bc9374_cacccbac9cb2432e9061aae64f812dfd.webp` (860x552) — dense galaxy
  field labelled by PGC number (`PGC 2167368`, `PGC 2183568`, `PGC 21991`,
  ...). Acceptance mapping: roadmap item 156 (PGC catalogue support, still
  Open); it is the reference for the next density/label-collision wave.

`testVendorExampleCorpus` checks that these files are documented, carry a valid
WebP header when present locally, and that the bundled name catalogue can
produce the Andromeda-field labels seen in the first example.

## Synthetic fixture

- `synthetic-field.json` — 40 deterministic sources in an M31-like field
  centred at RA 10°, Dec 41°, with a 512 × 512 frame at 60 arcsec/pixel. RA and
  Dec are stored in **degrees** for readability and converted to radians by the
  embedded selftests. Each source carries catalogue provenance, magnitude,
  priority and (for extended objects) major/minor axes and position angle.
- `synthetic-field.svg` — SVG 2 output from `buildAnnotationSVG` with every
  stable `<g data-layer>` group enabled.
- `synthetic-field.png` — raster output generated from the same unified source
  model and the same label placement as the SVG.

The golden JSON is embedded in `annotation_selftests.go` so the bundle/golden
field test runs from any working directory. The checked-in SVG and PNG are the
reference outputs emitted by the same renderer and placement code; they can be
regenerated with a temporary Go harness that calls `projectAnnotationSources`,
`buildAnnotationSVG`, `placeAnnotationLabels` and `renderAnnotationRaster`.

## Acceptance checks

- `testAnnotationBundle`: ≥95 % of the 40 golden sources are projected and
  annotated; the offline bundle is below 2 MB.
- `testAnnotationVO`: mock SIMBAD success, timeout, truncation, cache reuse and
  credential redaction, plus a mock VizieR VOTable cone response.
- `testAnnotationPlacement`: zero overlaps in 20 deterministic fixtures;
  10 000 labels placed in ≤200 ms.
- `testAnnotationSVG`: SVG 2 with all stable layers, a ≤2 MiB budget for
  10 000 sources, no CSS `var()` presentation attributes (non-browser
  viewers render those black-on-black) and magnitude-encoded star colours.
- `testAnnotationMultiTileCoverage`: cached Gaia stars from every tile
  crossing the frame are unioned, de-duplicated and deterministic.
- `testGaiaFieldTileBorderCoverage`: the 9x9 sampling grid includes all four
  frame borders (the old half-cell offset missed right/bottom tiles).
- `testGaiaStarsForFieldCoverage`: offline gaps are counted, an online fetch
  fills them, star unions are deterministic and a failed fetch stays visible
  in the coverage report.
- `testGaiaNameCrossmatch`: every Gaia star gets a unique `Gaia J…`
  designation; a close SIMBAD/VizieR source supplies a real name, the matched
  catalog entry is not duplicated and proper motion is used for the epoch gap.
- `testIRSANameFallback`: the AllWISE VOTable fallback parses designations,
  caches them and degrades cleanly when the service is unavailable.
- `testAnnotationRasterParity`: deterministic raster, SSIM ≥0.99 and glyph
  baselines aligned within 1 px.
- `testLayeredAnnotationEndToEnd`: one plate-solved fixture produces SVG, PNG
  and `annotation_provenance.json` with no network access.

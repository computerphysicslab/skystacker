# Output formats

SkyStacker separates scientific data (`float32` planes, FITS, XISF, BigTIFF)
from display previews (JPEG/PNG/BMP) and machine-readable metadata.

## Float32 stacks

`stacked_sum.f32raw`, `stacked_mean.f32raw`, `stacked_no_correction_mean.f32raw`
and mask products are planar interleaved RGB `float32` arrays written in native
byte order. They are the canonical hand-off format between pipeline stages and
are never gamma-encoded.

## TIFF and FITS

- `--tiff`: uncompressed little-endian TIFF, RGB IEEE float32, single strip.
- `--fits`: big-endian `BITPIX=-32`, `NAXIS=3` FITS. When plate-solved it carries
  `CTYPE1='RA---TAN'`, `CTYPE2='DEC--TAN'`, `CRVAL1/2` (degrees), `CRPIX1/2`
  and `CD1_1..CD2_2` (degrees).
- `--bigtiff`: BigTIFF float32 with 64-bit offsets for stacks >4 GiB.

The first-party `goTiff` module additionally reads TIFF inputs with 8/16-bit
integer or 32/64-bit float samples, grayscale/RGB photometry, classic/BigTIFF
containers, PackBits/LZW/Deflate compression, horizontal prediction, multipage
IFDs and Astro-TIFF/CFA tags; those capabilities are library-level for now,
while the writer CLI remains unchanged.

`--xisf` writes a monolithic XISF 1.0 file with planar float32 samples;
`--xisf-rev2` adds SHA-256/512 or verified SHA-3 checksums and strict
sample-mapping validation.

## JPEG previews

8-bit display previews use gamma 0.5. They are diagnostics, not scientific
outputs; use `.f32raw`/FITS/XISF for measurements.

## Machine-readable products

### `report.json`

Enabled with `--json-report`. Mirrors `report.txt` with arrays for accepted and
rejected frames, per-frame diagnostics (including `PSFScale` and `Weight`),
resolved flags, exposure summary, output files and notes.

### `plate_solution.json`

Full schema and units are documented in
[plate-solving.md](plate-solving.md) §5. Fields: `WCS` (`CRVAL1/2`, `CRPIX1/2`,
`CD`, `SIP`), `SIP`, `PV1`, `PV2`, optional `Verify`, `RMSArcsec`, `NMatches`,
`NStars`, `Method`.

### `metrics.csv`

Header from `writeFrameMetricsCSV`:

```text
frame,stars,inliers,hfr_px,fwhm_px,eccentricity,snr,bg_sigma,
intrinsic_quality,weight,psf_scale,ssweight,note
```

### Star catalog export (`--export-stars`)

`--export-stars FILE` writes a `starCatalog` object with `image`, `width`,
`height`, `detection`, `count` and `stars[]`; each entry has `x_px`, `y_px`,
`flux`, `size_px`, `eccentricity` and `hfr_px`, sorted brightest-first. A `.csv`
path writes the same columns as CSV with the header
`x_px,y_px,flux,size_px,eccentricity,hfr_px`. Coordinates are 0-based pixels
and `flux` is the background-subtracted integrated intensity from the high-pass
detector. Run with `--process-image` and no `--image-op`.

### Diffuse-object classification (`--classify-objects`)

`--classify-objects FILE` writes a report-only JSON object with `image`,
`width`, `height`, `standards` (`class_ucd: src.class`, `morph_ucd:
src.morph.param`, IVOA `object-type` vocabulary), `threshold_sigma`,
`star_mask_pixels`, `count` and `objects[]`. Each object carries `id`, `x_px`,
`y_px`, `area_px`, `flux`, `concentration`, `asymmetry`, `smoothness`, `gini`,
`m20`, `ellipticity`, `object_type` and `class_ucd`. Stars are masked before
component detection; the pipeline pixels are never modified.

### `photometry.csv` / `photometry.json`

Produced by `--photometry`: x/y, RA/Dec, flux, instrumental and calibrated
magnitude, Gaia residual and per-star quality.

### `blind_benchmark.csv`

Produced by `--blind-benchmark DIR`:

```text
image,solver,success,seconds,rms_arcsec,matches,method,detail
```

### `provenance.json`

Enabled with `--provenance`; an IVOA ProvenanceDM-style graph of binary
version, plate-solve tolerances, tile/rejection/stretch parameters and
catalogue/cache versions. `report.json` also carries `DrizzleNoise` (Fruchter &
Hook R, weight median/RMS, coverage flag) and `DitherCoverage` (pattern class,
min separation, null fraction, hole risk).

### Other sidecars

| File | Producer |
|---|---|
| `catalog_matches.json`, `anomalies.json` | `--annotate` |
| `stacked_sky_map.svg` | `--sky-map` (layered `<g>` groups: background, gaia-stars, extended-objects, sso-vectors, labels, footer) |
| `survey_reference.fits/.jpg` | `--survey-cutout` |
| `diagnostics.txt/.json` | `--diagnose`, `--diagnose-integrated` |
| `stacked_wht.f32raw`, `stacked_ctx.u32raw` | `--wht-ctx` (per-pixel weights and 32-bit contributor mask, versioned headers) |
| `master_*.f32raw/.jpg` | `--export-masters` |

### Native blind index

`blind_index.sbi` (SBI1) is the compact binary index produced when
`--build-blind-index` targets a `.sbi` path: a fixed header, band table,
deduplicated star table (RA/Dec/mag/`pmra`/`pmdec`), tile table and quad table
referencing star IDs. JSON/`.gz` catalogs remain readable and interchangeable.
`blind_index.json` (produced by `--build-blind-index`) is a compact JSON
artefact (version 2) with `version`, `nside`, `bin`, `brightest`, `bands`,
`tiles` (brightest Gaia stars per HEALPix tile) and `quads`. Quad keys are
prefixed with a scale band (`wide|`, `medium|`, `narrow|`) and values contain
tile ids, band and backbone length in the catalogue's tangent-plane units. The
index is portable between machines and lets `--plate-solver native` work fully
offline for the indexed fields. Published catalogs are gzip-compressed
(`.json.gz`) and registered in `catalogs/manifest.json` with SHA-256, byte
count and build parameters; see [catalogs/README.md](../catalogs/README.md).

### Layered SVG schema

`stacked_sky_map.svg` and `<base>_sky_map.svg` are plain SVG 1.1 with stable
group ids so consumers can select, restyle or hide layers without re-parsing
the WCS:

| Group id | `data-layer` | Content |
|---|---|---|
| `background` | `background` | Frame rectangle. |
| `gaia-stars` | `point-sources` | Magnitude-sized `<circle>` elements with `<title>`. |
| `extended-objects` | `extended-sources` | `<ellipse>` with `data-pa` (pixel-frame degrees). |
| `sso-vectors` | `motion` | `<line>` with `data-object` and `data-site`, arrow marker `sso-arrow`. |
| `labels` | `labels` | `<text>` from deterministic collision placement; group declares `data-fields="8"`, `data-candidates="16"`, `data-passes="20"`. |
| `footer` | `footer` | Source and vector counts. |

Every group also carries `class="skystacker-layer"` and
`style="isolation:isolate"` via the embedded stylesheet. Ellipse position
angles already include field rotation and mirror correction; consumers must not
apply a second rotation.

## Coordinate conventions for consumers

| Quantity | Internal | FITS/web convention |
|---|---|---|
| CRVAL | radians | degrees |
| CRPIX | 0-based | 1-based (add 1) |
| CD | radians/pixel | degrees/pixel |
| Parity | `sign(det CD)` | same |
| SIP order | 2 (`A20,A11,A02,B20,B11,B02`) | FITS SIP keywords |

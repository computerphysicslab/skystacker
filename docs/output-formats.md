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
intrinsic_quality,weight,psf_scale,note
```

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
catalogue/cache versions.

### Other sidecars

| File | Producer |
|---|---|
| `catalog_matches.json`, `anomalies.json` | `--annotate` |
| `stacked_sky_map.svg` | `--sky-map` |
| `survey_reference.fits/.jpg` | `--survey-cutout` |
| `diagnostics.txt/.json` | `--diagnose`, `--diagnose-integrated` |
| `master_*.f32raw/.jpg` | `--export-masters` |

## Coordinate conventions for consumers

| Quantity | Internal | FITS/web convention |
|---|---|---|
| CRVAL | radians | degrees |
| CRPIX | 0-based | 1-based (add 1) |
| CD | radians/pixel | degrees/pixel |
| Parity | `sign(det CD)` | same |
| SIP order | 2 (`A20,A11,A02,B20,B11,B02`) | FITS SIP keywords |

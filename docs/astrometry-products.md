# Astrometry products

Once `plate_solution.json` exists, SkyStacker can derive several non-destructive
products. All are opt-in and never replace the scientific stack.

## Gaia SVG sky map

```bash
skystacker --post-only --sky-map --plate-solve \
  --plate-center 83.822,22.015 ./assets/OrionNebula/0_skystacker_results
```

`--sky-map` projects cached Gaia sources with `wcsTanForward` into
`stacked_sky_map.svg` at the exact image size and orientation. Point sources are
circles sized by magnitude with labels for bright stars. Extended objects are
drawn only when an external catalogue supplies major/minor axes and position
angle. Standalone mode solve a file directly:

```bash
skystacker --process-image photo.jpg --image-op sky-map \
  --image-output /tmp/maps --plate-solve --plate-solver auto
```

## HiPS survey cutout

```bash
skystacker --post-only --survey-cutout \
  --survey-hips CDS/P/DSS2/color --survey-timeout 30 \
  ./assets/OrionNebula/0_skystacker_results
```

`--survey-cutout` calls CDS `hips2fits` with the solved image's width, height,
FOV, rotation and centre, writing `survey_reference.fits` and
`survey_reference.jpg` plus a star-match diagnostic. It requires a plate
solution, respects the 50 Mpix service limit and never blocks offline runs.
The standalone variant is `--image-op survey-cutout`.

## Annotation and anomaly detection

```bash
skystacker --post-only --annotate \
  --annotation-catalogs my_objects.json \
  --cone-catalog sdss --cone-radius 60 \
  ./assets/OrionNebula/0_skystacker_results
```

`--annotate` writes `stacked_annotated.jpg` and `catalog_matches.json`;
unmatched detections farther than 8 px from any catalogue source are listed in
`anomalies.json`. `--annotation-catalogs` accepts JSON objects with
`ra`, `dec` (degrees) and optional `name`, `type`, `mag`, `major`, `minor`, `pa`.

`--cone-catalog` adds a cached VOTable cone search around the solved centre:

| Service | Radius cap | Query form |
|---|---|---|
| `sdss` | 180 arcsec | SkyServer cone search, `format=votable`. |
| `irsa` | 60 arcsec | IRSA TAP ADQL `CIRCLE` query. |
| `vizier` | 180 arcsec | VizieR Gaia DR2 `I/345/gaia2`. |

Results are cached as JSON under `~/.cache/skystacker/gaia_cache/` and merged
into the annotation table. See [plate-solving.md](plate-solving.md) §6 for the
integration contract.

## Ensemble photometry

```bash
skystacker --post-only --photometry \
  ./assets/OrionNebula/0_skystacker_results
```

`--photometry` performs aperture photometry, sets a Gaia ensemble zero point
with MAD rejection, and writes `photometry.csv` and `photometry.json` with a
light curve and residual table.

## Proper-motion and epoch handling

Gaia DR3 positions are propagated from J2017.5 to the frame epoch taken from
EXIF `DateTimeOriginal` before matching or annotation. An image carrying GPS
EXIF coordinates supplies its own pointing seed; no user input is required.

## Scientific downstream uses

| Product | Consumer |
|---|---|
| `plate_solution.json` | Any FITS/WCS library; convert CRVAL to degrees and CRPIX to 1-based. |
| `stacked_*.fits` | Already carries `RA---TAN`/`DEC--TAN`, `CRVAL`, `CRPIX`, `CD`. |
| `stacked_sky_map.svg` | Reports, web galleries. |
| `survey_reference.fits` | Visual comparison, blink checking, registration reference. |
| `photometry.csv/.json` | Variable-star and transient analysis. |

See [research/02](../research/02-astrometry-and-identification.md) §13–§16 for
the underlying algorithms and service limits.

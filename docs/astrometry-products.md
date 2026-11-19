# Astrometry products

Once `plate_solution.json` exists, SkyStacker can derive several non-destructive
products. All are opt-in and never replace the scientific stack.

## Gaia SVG sky map

```bash
skystacker --post-only --sky-map --plate-solve \
  --plate-center 83.822,22.015 ./assets/OrionNebula/0_skystacker_results
```

`--sky-map` projects cached Gaia sources with `wcsTanForward` into
`stacked_sky_map.svg` at the exact image size and orientation. The SVG is
**layered**: independent `<g id="..." class="skystacker-layer">` groups are
emitted for `background`, `gaia-stars`, `extended-objects`, `sso-vectors`,
`labels` and `footer`. Each group carries a stable `data-layer` key so another
application can toggle, restyle or recompose it without re-solving. Point
sources are circles sized by magnitude; extended objects are drawn only when an
external catalogue supplies major/minor axes and position angle. Ellipse PAs are
corrected for field rotation from the CD matrix and negated for mirrored WCS
solutions. Labels use eight selectable fields, sixteen candidate anchors (eight
directions × two radii) and twenty deterministic collision passes. Standalone
mode solves a file directly:

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

## Solar System Objects and velocity vectors

```bash
skystacker --post-only --annotate --sky-map \
  --sso-ephemeris sso.txt \
  ./assets/OrionNebula/0_skystacker_results
```

The ephemeris file uses MPC-style object IDs and comment headers:

```text
# OBSERVATORY 568
# LAT 40.4
# LON -3.7
# DATE-OBS 2026-11-19T21:00:00Z
a:2026 AB1 2026-11-19T21:00:00Z 83.8221 22.0145
a:2026 AB1 2026-11-19T23:00:00Z 83.9123 22.1010
c:2026 C1  2026-11-19T21:00:00Z 84.0100 22.3000
c:2026 C1  2026-11-19T23:00:00Z 84.0200 22.3500
```

Accepted prefixes: `a:` asteroid, `c:` comet, `p:` planet, `dp:` dwarf planet,
`s:` satellite. `DATE-OBS` is mandatory; a three-symbol IAU observatory code
plus `LAT`/`LON` marks a topocentric site. Without a complete site header
SkyStacker logs a geocentric fallback. One hour of motion at `DATE-OBS` is
projected through the solved WCS and drawn as an orange arrow in the
`sso-vectors` SVG layer and in `stacked_annotated.jpg`.

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

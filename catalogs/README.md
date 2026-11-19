# Plate-solving catalogs

This directory publishes small, versioned Gaia-derived quad indexes for
offline blind plate solving with `--plate-solver native`. They contain only
bright-star geometry: no images, no astrometry.net index files and no external
solver is needed at solve time.

## Available catalogs

| Id | Gaia limit | HEALPix nside | Compressed size | Status |
|---|---|---|---|---|
| `mag07` | G ≤ 7 | 4 | ~1.3 MB | available in this repo |
| `mag10` | G ≤ 10 | 4 | larger | buildable locally (see below) |

`manifest.json` is the authoritative registry: it records the file name,
checksum, byte count, star/quad counts and status of every catalog.

## Using a published catalog

```bash
# From a checkout of this repository the catalog is already local.
skystacker --process-image photo.jpg --image-op plate-solve \
  --image-output /tmp/out --plate-solver native --blind-catalog mag07

# From anywhere else, download it once into the OS cache (checksum verified).
skystacker --download-blind-catalog mag07
skystacker --blind-catalog mag07 ...

# Inspect the registry.
skystacker --list-blind-catalogs
```

`--blind-catalog` resolves to `catalogs/<file>` when a manifest is found next
to the running process (or via `SKYSTACKER_CATALOGS`), otherwise to
`~/.cache/skystacker/catalogs/<file>`. Downloads use the repository raw URL by
default; a catalog entry may override it with its own `url`. SHA-256 is always
verified before the index is accepted.

## Building a custom catalog

```bash
skystacker --build-blind-index /tmp/custom.json.gz \
  --blind-allsky --gaia-nside 4 \
  --blind-mag-limit 9 --blind-brightest 200
```

The builder queries Gaia TAP tile by tile, caches the tiles, generates
parity-tolerant four-star descriptors in three scale bands
(`wide`/`medium`/`narrow`) and writes a gzip-compressed JSON index. Use
`--blind-max-tiles N` for a partial first pass, or `--gaia-offline` to rebuild
from already cached tiles without network access.

To publish a new catalog: add its `.gz` file here, compute `sha256sum` and
byte size, and add an entry to `manifest.json` with `status: "available"`.

## Design notes

- The index is portable and self-contained: it stores bright Gaia stars per
  HEALPix tile plus quantised quad descriptors, so matching and WCS fitting
  reuse SkyStacker's existing Gaia code.
- Verification gathers catalogue stars by angular radius around the winning
  cell and merges the top voted cells, which covers fields spanning tile
  boundaries and wide-field images.
- Deeper limits increase star density and index size roughly linearly; tile
  count grows as `12 × nside²`. A coarser nside is cheaper but yields fewer
  stars per tile.
- Gaia DR3 positions are propagated from J2017.5 using proper motion when the
  frame has an EXIF epoch; without metadata the catalogue epoch is used.

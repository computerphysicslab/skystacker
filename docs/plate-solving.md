# Plate solving

SkyStacker resolves the absolute sky coordinates of an image by matching
detected stellar centroids against Gaia DR3 star positions. The result is a
TAN (gnomonic) World Coordinate System with optional order-2 SIP distortion,
written as `plate_solution.json` and reused by stacking, annotation, sky maps,
survey cutouts, photometry and comet tracking.

The same binary supports **seeded** solving (a pointing is already known) and
**blind** solving (no pointing, no pixel scale, no roll, no metadata). Blind
solving is delegated to astrometry.net or ASTAP; the internal Gaia matcher is a
verification/refinement engine and requires a centre to query the correct
HEALPix tile.

## 1. Which solver should I use?

| Situation | Recommended invocation | Network needed? |
|---|---|---|
| JPEG/FITS with a known RA/Dec from EXIF, XMP or FITS header | `--plate-solve` (auto chain picks the seeded Gaia solver) | Gaia TAP only if the tile is not cached |
| Coordinates supplied by the user | `--plate-solve --plate-center RA,DEC` | Same as above |
| No metadata at all, astrometry.net installed with indexes | `--plate-solve --plate-solver astrometry-local` | No |
| No metadata, ASTAP installed with a D05–D80 database | `--plate-solve --plate-solver astap` | No |
| No metadata, no local solver, nova account available | `--plate-solve --plate-solver astrometry-web --astrometry-api-key KEY` | Yes (upload + polling) |
| Native prototype, no external solver at all | `--plate-solve --plate-solver native` | No (needs a Gaia quad index built from cached tiles) |
| Default, want the best available path automatically | `--plate-solve` (`--plate-solver auto`) | Only if no local backend is installed |
| One image only, no stacking | `--process-image IMG --image-op plate-solve` | Depends on backend |

The auto chain is deterministic:

1. Seeded Gaia solve when a pointing seed exists (`--plate-center`, FITS
   `RA`/`DEC`, EXIF/XMP GPS). On success it returns immediately.
2. Local astrometry.net `solve-field` (`--astrometry-cmd`, default
   `solve-field`).
3. Local ASTAP (`--astap-cmd`, default `astap`).
4. nova.astrometry.net web API (`--astrometry-api-key`, `AN_API_KEY` or
   `ASTROMETRY_API_KEY`).

A failed backend is logged and the chain continues. If none is available the
command fails with an actionable message. Pin a single backend with
`--plate-solver gaia|astrometry-local|astap|native|astrometry-web`.

## 2. Seeded solving with the internal Gaia matcher

`--plate-solver gaia` (or the first step of `auto`) uses the embedded TaLP
client:

1. Detect stars on the linear frame (`detectStars`).
2. Convert the seed (RA, Dec) into a HEALPix tile index at `nside = 64`.
3. Load the tile from the persistent Gaia cache, or query the ESA TAP service
   (`gaiaDefaultMirror`) and cache the JSON result.
4. Propagate catalogue positions from Gaia's J2017.5 reference epoch to the
   observation epoch derived from EXIF `DateTimeOriginal`.
5. Match frame stars to Gaia stars (`matchStarsToGaia`, 5 arcsec tolerance),
   fit a TAN WCS (`fitWCSFromMatches`) and order-2 SIP (`fitSIPFromMatches`).
6. Compute the RMS reprojection error in arcsec and write the solution.

**Seed priority:** `--plate-center R,D` (decimal degrees) → FITS `RA`/`DEC` of
the reference frame → EXIF GPS IFD / XMP `GPSLatitude`/`GPSLongitude` → no seed.
A JPEG carrying GPS metadata therefore solves without any user coordinates.

When all seeds are absent, the built-in `--plate-solver native` performs a
genuine blind solve with a Gaia quad index (see §4). The old 3×3 `nside=64`
scan around RA/Dec = (0°, 0°) remains only as a legacy self-test fixture.

For seeded solves on wide or processed frames, `--gaia-multitile` grows the
Gaia catalogue from every cached tile around the pointing (a single `nside=64`
tile can contain only ~30 stars), and `--gaia-loose-match` relaxes the matcher
to 8 arcsec and 1/8 of the brightest stars. Solutions are still rejected when
the reprojection RMS exceeds 5 px or the centre is more than 2° from the seed.
Set `SKYSTACKER_DEBUG_MATCH=1` to print matcher diagnostics. See
[blind-corpus-results.md](blind-corpus-results.md) for real-image outcomes:
the native solver currently succeeds on noisy synthetic fields but not yet on
the 14 processed images of the user corpus.

### Diagnostic: one-shot solve of a synthetic field against the real Gaia cache

```bash
skystacker --test-plate 83.822,22.015
```

Observed output on this checkout (the Gaia tile was already cached):

```text
SkyStacker 0.6.1 | https://github.com/computerphysicslab/skystacker | built 2026-11-19T07:27:57Z
  gaia: tile nside=64 ipix=24185 [cache hit]
Plate solved (method=header)
  CRVAL1 = 83.821210 deg
  CRVAL2 = 22.015715 deg
  CRPIX1 = 252.806
  CRPIX2 = 252.880
  CD     = [[4.000e-06, 2.066e-11], [2.066e-11, -4.000e-06]]
  SIP    = [-1.4096458072833951e-10 -2.5323818527975384e-11 -9.696205454024223e-11
            2.5345969402599433e-11 -5.585559984142594e-11 -2.746587768006367e-11]
  RMS    = 0.000 arcsec
  NMatches = 30 / 30
```

`--test-plate` synthesises its own star field and is useful to validate the
Gaia cache and the fit maths; it does **not** solve the user's image.

### Seeded solve of a real image (offline)

```bash
skystacker --process-image \
  "assets/M42-2017-11-18-Leciñena-50mm/0_skystacker_results/stacked_mean.jpg" \
  --image-op plate-solve \
  --image-output /tmp/opencode/docs-plate-m42 \
  --plate-solver gaia \
  --plate-center 83.822,-5.391 \
  --gaia-offline
```

Observed output with the bundled Gaia cache:

```text
  gaia: tile nside=64 ipix=21439 [cache hit]
  plate: seeded Gaia solve failed (plate solve: no consistent transform (0 matched pairs))
skystacker: plate solve: seeded Gaia solve failed: plate solve: no consistent transform (0 matched pairs)
```

This is a genuine possible outcome: a processed JPEG with few reliable
centroids may not produce five-arcsecond matches against a magnitude-limited
Gaia tile. Use the original linear lights, add a scale hint, or use a blind
backend that performs its own source extraction. The cache hit confirms the
seed and epoch path worked; the failure is in hypothesis verification.

## 3. Blind solving with astrometry.net

### Local `solve-field`

`--plate-solver astrometry-local` runs the image through a local
`astrometry.net` installation:

```bash
skystacker --plate-solve --plate-solver astrometry-local \
  --astrometry-cmd "solve-field" \
  --workers 8 ./assets/OrionNebula/lights
```

Requirements: `solve-field` on `PATH` and the series 4100/5200 index files
(`data.astrometry.net`). The adapter passes `--no-plots --overwrite
--skip-solved`, writes products to a temporary directory and reads the
resulting `*.wcs`. The user's input files are never modified. An optional
scale hint narrows the index search:

```bash
skystacker --process-image photo.jpg --image-op plate-solve \
  --plate-solver astrometry-local \
  --solver-focal-length 135 --solver-pixel-size 4.3
```

The hint is `arcsec/px = pixel[µm] / focal[mm] × 206.26` (here ≈6.55
arcsec/px). The adapter requests ±20% bounds. The same formula is used after
solving to warn when the recovered scale disagrees with the optics by more
than a factor of 1.4.

### nova.astrometry.net web API

No local indexes are required; an API key from `nova.astrometry.net` is:

```bash
export AN_API_KEY='...'
skystacker --plate-solve --plate-solver astrometry-web \
  --astrometry-timeout 300 ./assets/M42-2017-11-18-Leciñena-50mm
```

The adapter logs in (`POST /api/login`), uploads the image as multipart
(`POST /api/upload`), polls `submissions/<id>` and `jobs/<id>`, and downloads
`wcs_file/<job>` with the required `Referer` header. Uploads default to
`publicly_visible=n`. The API key is **redacted** from `report.txt`'s command
line. Use `--astrometry-url` to point at a compatible service such as a local
SkySolver instance.

### ASTAP

`--plate-solver astap` uses ASTAP's native blind solver with `-fov 0`, which
steps the field of view down from ~10° to 0.3° in ~30% increments:

```bash
skystacker --plate-solve --plate-solver astap \
  --astap-cmd "astap" ./assets/OrionNebula/lights
```

The image is copied to a temporary directory, so ASTAP's `<basename>.wcs`
sidecar never appears next to the user's light frames. A D05–D80 star database
must be installed. ASTAP is fully offline and parity-invariant, but is more
sensitive to bloated or out-of-focus stars than astrometry.net.

## 4. Native Gaia quad index (prototype, no astrometry.net)

`--plate-solver native` removes the external-solver dependency for fields
covered by an index built from the local Gaia cache:

```bash
# 1. Populate the Gaia cache for the fields you care about.
skystacker --test-gaia 83.822,22.015

# 2. Build the index (default: <gaia-cache-dir>/blind_index.json).
skystacker --build-blind-index /tmp/blind_index.json

# 3. Optional: extend the index across the sky (bounded for a first pass).
skystacker --build-blind-index /tmp/blind_index.json \
  --blind-allsky --blind-max-tiles 200 --gaia-nside 8

# 4. Solve without any seed or network.
skystacker --process-image photo.jpg --image-op plate-solve \
  --image-output /tmp/out --plate-solver native \
  --blind-index /tmp/blind_index.json \
  --solver-focal-length 135 --solver-pixel-size 4.3
```

The index stores, per cached HEALPix tile, up to 100 bright Gaia stars and
quantised descriptors of quads built from each star's three nearest neighbours.
Descriptors use the longest pair as a backbone and encode the two remaining
stars in normalised coordinates with `|v|`, making them invariant to rotation,
scale, translation and mirror parity. Three **scale bands** index different
star densities: `wide` (5–60 arcsec/px, 25 stars), `medium` (0.8–6, 50) and
`narrow` (0.05–1.2, 100). The focal-length hint selects the band at query time;
without a hint all bands are searched. Votes select the best tiles and the
existing `matchStarsToGaia` → `fitWCSFromMatches` → `verifySolution` chain
produces the usual `plate_solution.json` with `Method: "native"`.

### Published magnitude-limited catalogs

Small, versioned indexes live in `catalogs/` and are registered in
`catalogs/manifest.json`. They let users solve offline without building an
index themselves:

```bash
skystacker --list-blind-catalogs
skystacker --process-image photo.jpg --image-op plate-solve \
  --image-output /tmp/out --plate-solver native --blind-catalog mag07
skystacker --download-blind-catalog mag07   # outside a checkout, verifies SHA-256
```

Published tiers: `mag07` (G ≤ 7, ~1.3 MB, 17,426 stars), `mag10`
(G ≤ 10, 300 stars/tile, ~3.6 MB, 57,600 stars) and `mag10deep`
(G ≤ 10, 2,000 stars/tile, ~25 MB, 333,567 stars, 309,078 multi-quad descriptors). All are all-sky at
`nside=4` and built from 192 Gaia tiles. Deeper indexes can be built locally
with `--build-blind-index FILE --blind-allsky --gaia-nside 4
--blind-mag-limit 10 --blind-brightest 2000 --blind-max-tiles 24
--blind-tile-offset N --blind-refresh`. See
[catalogs/README.md](../catalogs/README.md) for the format and publishing
workflow, and [blind-corpus-results.md](blind-corpus-results.md) for measured
results on 14 real images from 50 to 1200 mm. Indexes written to `.sbi` use the
compact SBI1 binary format (shared star table with proper motions, band/tile/
quad tables); hostile table counts are rejected before allocation; JSON and
`.gz` catalogs remain readable.

**Scope of the prototype:** `--blind-allsky` can now sweep tiles across the sky
at a chosen `--gaia-nside` (bounded by `--blind-max-tiles`), but a fine-grained
all-sky index still needs a large TAP campaign and index tooling. Coarse nside
values give wide-field coverage; narrow fields need higher nside and more
tiles. Adding fields with `--test-gaia R,D` and rebuilding extends coverage
incrementally. Full all-sky generation remains the R&D part of roadmap item 73.

## 5. Worked example: blind solve of a real image with a mock backend

The internal mock exists for integration tests and documentation; it returns a
fixed but valid WCS. It demonstrates the exact command contract and JSON
artifacts without requiring external software:

```bash
skystacker --process-image \
  "assets/M42-2017-11-18-Leciñena-50mm/0_skystacker_results/stacked_mean.jpg" \
  --image-op plate-solve \
  --image-output /tmp/opencode/docs-real \
  --plate-solver astrometry-local \
  --astrometry-cmd '"/home/jips/.local/bin/skystacker" --internal-astrometry-mock' \
  --gaia-offline
```

Observed output:

```text
  plate: blind solve via local astrometry.net
  gaia: tile nside=64 ipix=24185 [cache hit]
  plate: Gaia refinement skipped: plate solve: no consistent transform (0 matched pairs)
Plate solved: CRVAL=(83.822100, 22.014500) RMS=0.00 arcsec method=astrometry-local
```

The Gaia refinement was skipped because the mock WCS does not correspond to
the real pixels; with a genuine solve the refinement re-fits the WCS and fills
`RMSArcsec`, `NMatches` and `Verify`.

## 6. What a solve produces

### `plate_solution.json`

Written beside the stack (or as `<basename>_plate_solution.json` in standalone
image mode). All angle fields are **radians** unless noted; `CD` is in
radians/pixel and `CRPIX` is **0-based**.

```json
{
  "WCS": {
    "CRVAL1": 1.462971630935939,
    "CRVAL2": 0.3842255081802917,
    "CRPIX1": 511,
    "CRPIX2": 341,
    "CD": [[-0.0000017453292519943296, 0], [0, 0.0000017453292519943296]],
    "SIP": null
  },
  "SIP": [0.00001, 0.00002, 0.00003, 0.00004, 0.00005, 0.00006],
  "PV1": [0, 1, 0, 0.00001, 0.00002, 0.00003],
  "PV2": [0, 1, 0, 0.00004, 0.00005, 0.00006],
  "RMSArcsec": 0,
  "NMatches": 0,
  "NStars": 0,
  "Method": "astrometry-local"
}
```

Field meanings:

| Field | Meaning |
|---|---|
| `WCS.CRVAL1/2` | Tangent point RA/Dec in radians. |
| `WCS.CRPIX1/2` | Reference pixel, 0-based (subtract 1 from FITS conventions). |
| `WCS.CD` | 2×2 linear matrix, radians per pixel. Its determinant sign gives parity. |
| `WCS.SIP` / `SIP` | Order-2 distortion `[A20,A11,A02,B20,B11,B02]`, dimensionless. |
| `PV1` / `PV2` | Shupe-style PV export of the same distortion (indices 0..5). |
| `Verify` | Optional report-only `{scale_median_arcsec_per_px, scale_sigma_arcsec_per_px, log_odds, quads, consistent}`. |
| `RMSArcsec` | Reprojection RMS of the matched stars, arcsec. |
| `NMatches` / `NStars` | Matched / detected star counts. |
| `Method` | `header`, `blind`, `astrometry-local`, `astap`, `native` or `astrometry-web`. |

Derived quantities for downstream applications:

- Pixel scale: `sqrt(|det(CD)|) × 206264.806` arcsec/px.
- FOV: `width × scale` by `height × scale` (use the larger dimension for
  single-value APIs).
- Centre: evaluate the tangent point at the image centre with
  `wcsTanInverseSIP` or any FITS WCS library.
- Orientation: `atan2(CD[0][1], CD[1][1])` (radians); parity from `det(CD)`.

### FITS and report integration

With `--fits`, SkyStacker writes `CTYPE1='RA---TAN'`, `CTYPE2='DEC--TAN'`,
`CRVAL1/2` (degrees), `CRPIX1/2` and `CD1_1..CD2_2` (degrees) into the stacked
FITS header. `report.txt` records the method under `Plate solution:` and the
command line under `Command line:`; `--json-report` adds a structured copy of
the run report.

Downstream products consume the same solution. `--sky-map` writes a layered SVG
(independent groups for stars, extended objects, SSO motion vectors, labels and
footer), and `--annotate --sso-ephemeris FILE` adds MPC-style Solar System
objects with velocity arrows. Their schemas are described in
[astrometry-products.md](astrometry-products.md) and
[output-formats.md](output-formats.md).

## 7. Consuming the solution from another application

SkyStacker is a CLI, not a daemon. A remote service can invoke it as a process
and treat `plate_solution.json` as the machine interface:

```bash
#!/usr/bin/env bash
set -euo pipefail
skystacker --process-image "$1" --image-op plate-solve \
  --image-output "$2" --plate-solver auto \
  --astrometry-timeout 180 >"$2/solve.log" 2>&1
python3 - "$2" <<'PY'
import json, math, sys, pathlib
d = pathlib.Path(sys.argv[1])
sol = json.loads((d / "plate_solution.json").read_text())
cd = sol["WCS"]["CD"]
det = cd[0][0]*cd[1][1] - cd[0][1]*cd[1][0]
print("method:", sol["Method"])
print("ra_deg:", math.degrees(sol["WCS"]["CRVAL1"]))
print("dec_deg:", math.degrees(sol["WCS"]["CRVAL2"]))
print("scale_arcsec_per_px:", math.sqrt(abs(det)) * 206264.806)
PY
```

Integration rules:

| Topic | Contract |
|---|---|
| Exit status | `0` success; `1` runtime failure (including a failed solve); `2` invalid flags. |
| Stdout | Human-readable banner and progress; parse `plate_solution.json`, not stdout. |
| Stderr | `skystacker: <error>` on failure. |
| Idempotence | Inputs are never modified; all products go to `--image-output` or the results directory. |
| Concurrency | One process per image; each uses its own temporary directory. Set `--workers` to bound CPU usage. |
| Network | Opt-in per backend; set a `--astrometry-timeout`; use `--gaia-offline` for air-gapped hosts. |
| Secrets | Prefer `AN_API_KEY` over `--astrometry-api-key`; the latter is redacted in reports but still visible in the process table. |
| Batching | `--batch FILE` runs a JSON queue of CLI argument lists sequentially; see [integration.md](integration.md). |

## 8. Flags used by the solver

| Flag | Default | Purpose |
|---|---|---|
| `--plate-solve` | off | Solve after stacking or in `--post-only`. |
| `--plate-solver MODE` | `auto` | `auto`, `gaia`, `astrometry-local`, `astap`, `native`, `astrometry-web`. |
| `--blind-index FILE` | `<gaia-cache-dir>/blind_index.json` | JSON quad index used by the native solver. |
| `--build-blind-index FILE` | — | Build the native index from cached Gaia tiles and exit. |
| `--blind-allsky` | off | With `--build-blind-index`: query Gaia tiles across the sky first. |
| `--blind-max-tiles N` | `0` (all) | Bound the all-sky tile sweep (useful for a first pass). |
| `--blind-tile-offset N` / `--blind-refresh` | `0` / off | Chunked/resumable builds; refresh forces fresh TAP queries. |
| `--blind-catalog ID` | — | Use a published catalog from `catalogs/manifest.json`. |
| `--list-blind-catalogs` | off | List available/buildable catalogs and exit. |
| `--download-blind-catalog ID` | — | Download a catalog into the cache with SHA-256 verification. |
| `--blind-mag-limit M` / `--blind-brightest N` | `14` / `100` | Catalog build parameters. |
| `--plate-center R,D` | — | Explicit RA/Dec seed in decimal degrees. |
| `--astrometry-cmd CMD` | `solve-field` | Local astrometry.net command (quoted argv supported). |
| `--astap-cmd CMD` | `astap` | Local ASTAP command. |
| `--astrometry-api-key KEY` | env | nova API key (`AN_API_KEY`, `ASTROMETRY_API_KEY`). |
| `--astrometry-url URL` | nova | Web API base URL (injectable for compatible services). |
| `--astrometry-timeout N` | `180` | Timeout in seconds for each blind backend. |
| `--solver-focal-length MM` / `--solver-pixel-size UM` | — | Optional scale hint and post-solve sanity check. |
| `--gaia-offline` | off | Never query Gaia; cache miss is a warning during refinement. |
| `--gaia-mirror URL` / `--gaia-cache-dir DIR` | ESA / OS cache | Gaia TAP endpoint and tile cache. |
| `--image-op plate-solve` | — | Solve one image without running the stacking pipeline. |

## 9. Troubleshooting

| Symptom | Likely cause | Action |
|---|---|---|
| `image needs a plate solution: pass --plate-solve or place plate_solution.json beside it` | No solution and no explicit solve requested | Add `--plate-solve` or drop a `plate_solution.json` next to the image. |
| `no consistent transform (0 matched pairs)` | Bad seed, saturated/processed image, or Gaia tile too shallow | Use a linear frame, verify `--plate-center`, or switch to a blind backend. |
| `no pointing seed` with `--plate-solver gaia` | No flag/FITS/EXIF seed | Supply `--plate-center` or use `auto`/`astrometry-*`/`astap`. |
| `solve-field not found` | astrometry.net not installed | Install it and its indexes, or use `astap` / `astrometry-web`. |
| `astap: "astap" not found` | ASTAP not installed | Install ASTAP plus a D05–D80 database, or pick another backend. |
| `no API key` | No nova credentials | Export `AN_API_KEY` or pass `--astrometry-api-key`. |
| `Gaia refinement skipped` | External WCS has no Gaia matches in the cached tile | Keep the external WCS; add the correct Gaia tile to the cache if you need RMS/Verify. |
| Large scale-sanity warning | Wrong focal length/pixel size or a wrong solve | Recheck `--solver-focal-length`/`--solver-pixel-size`. |

Further background: [research/02](../research/02-astrometry-and-identification.md)
§14–§16 and [integration.md](integration.md).

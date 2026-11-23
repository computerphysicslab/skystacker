# Troubleshooting

## Plate solving

| Log message | Meaning | Fix |
|---|---|---|
| `image needs a plate solution: pass --plate-solve or place plate_solution.json beside it` | No solution found next to the image and no solve requested. | Add `--plate-solve` or provide `plate_solution.json`. |
| `only N stars detected; cannot plate-solve` | Fewer than three centroids. | Use the linear frame; avoid heavily stretched JPEGs. |
| `no pointing seed` | `--plate-solver gaia` without a seed. | Use `--plate-center`, FITS RA/DEC, EXIF GPS, or switch to `auto`/blind. |
| `no consistent transform (0 matched pairs)` | Gaia tile has stars but none match the detected centroids. | Verify the seed, add `--solver-focal-length/--solver-pixel-size`, or use a blind backend. |
| `solve-field not found` | astrometry.net CLI missing. | Install it plus indexes, or use `--plate-solver astap|astrometry-web`. |
| `astap: "astap" not found` | ASTAP missing. | Install ASTAP and a D05–D80 database. |
| `no API key` | nova credentials missing. | Export `AN_API_KEY` or pass `--astrometry-api-key`. |
| `Gaia refinement skipped` | External solve centre has no matching cached Gaia tile. | Keep external WCS or pre-cache the tile with `--test-gaia R,D`. |
| `blind fallback unavailable` | No local solver and no API key. | Install one backend. |
| Large `solved scale … differs from optics estimate` warning | Wrong focal length/pixel size or a wrong solution. | Recheck `--solver-focal-length`/`--solver-pixel-size`. |

The legacy internal "blind" mode scans only a 3×3 `nside=64` patch around
RA/Dec 0°, so a real field needs an external backend. This is documented in
[plate-solving.md](plate-solving.md) §2.

## Gaia cache and network

- `gaia: cache miss for nside=… ipix=… in offline mode` — tile not cached and
  `--gaia-offline` is set. Pre-cache with `--test-gaia R,D`.
- `TAP query timed out after …; ESA was too slow` — retry later or use a mirror
  with `--gaia-mirror`; reduce `--gaia-nside`/`--gaia-mag-limit`.
- `429` responses are retried with backoff; the client limits concurrency to
  four active requests.

## Native blind solver

- `native blind solve: no index … (build it with --build-blind-index)` — run
  `--build-blind-index` after populating the Gaia cache with `--test-gaia R,D`.
- `no nside=64 Gaia tiles in …` — the Gaia cache is empty for the configured
  `--gaia-nside`; add tiles first.
- `no quad matches in N image quads` — the field is not covered by the index;
  add nearby fields to the cache and rebuild.
- `no candidate tile verified` — the index voted but Gaia matching failed;
  check the focal-length hint and the quality of the detected centroids.
- `unknown catalog` / `catalog is not available locally` — run
  `--list-blind-catalogs`, then `--download-blind-catalog ID`.
- `catalog checksum mismatch` — the download is corrupt or the manifest is
  stale; delete the cached copy under `~/.cache/skystacker/catalogs/` and retry.
- `blind index: N of M Gaia tile queries failed` — TAP throttling or an archive
  problem during `--blind-allsky`; rerun with a smaller `--blind-max-tiles` or
  use `--gaia-offline` to rebuild from cached tiles only.

## Annotation and SSO layers

- `SSO ephemeris …: DATE-OBS header is required` — add a `# DATE-OBS <ISO>` line
  to the ephemeris; velocity vectors need an observation epoch.
- `object ID must start with a:/c:/p:/dp:/s:` — use MPC-style prefixes in the
  first column of each ephemeris row.
- `SSO: no complete site header; using geocentric positions` — add `# OBSERVATORY CODE`,
  `# LAT` and `# LON` for a topocentric site; otherwise positions are geocentric.
- `Sky map: SSO ephemeris ignored: …` — the file could not be parsed; the SVG is
  still written without the `sso-vectors` layer.
- Missing ellipse shapes in the SVG — an external catalogue entry needs both
  `major` and `minor` to be positive; PA is already pixel-frame corrected.

## Calibration and stacking

- `input folder mixes images of different dimensions` — move the mismatching
  files out; all lights must share one size after `--scale`.
- `no valid images (all candidates rejected)` — format or decode failure; check
  the report's rejection reasons.
- Self-flat + `--local-normalize` overlap warning — expected; compare with
  `--no-local-normalize` before publishing.
- Zero weights with `--psf-scale-snr` — missing `.xnml` sidecars; point
  `--xnml-dir` at their folder or disable the flag.

## RAW and decoding

- `--no-native-cr2` switches to `dcraw`/ImageMagick; install them if native CR2
  fails on an exotic variant.
- CR3 still needs an external decoder.
- FITS debayer diagnostics: `--test-debayer IMAGE`.

## Performance and memory

- Reduce `--workers` on memory-constrained hosts.
- `--tile-size` bounds the out-of-core reduction tile buffer (capped at 256 MB).
- `--no-cache` avoids `_work/cache` if disk is tight; `--keep-cache` speeds up
  repeated runs.
- `--clean` removes `_work` after a successful run.

## Self-test and build

- `--selftest` must print `all 306 self-tests passed`; a failure exits `1`.
- `gofmt -l *.go` must print nothing and `go vet ./...` must be clean.
- Always install the rebuilt binary on `PATH` and verify `which skystacker` and
  `skystacker --version` before running a production batch.

For background and exact algorithms, consult `research/`; for flag defaults see
[cli-reference.md](cli-reference.md).

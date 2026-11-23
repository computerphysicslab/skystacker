# Integrating SkyStacker from another application

SkyStacker is a stateless CLI. Integration is based on process invocation,
filesystem products and exit codes; there is no daemon or long-lived service.
The pattern works identically for local scripts and remote job runners.

## Process contract

| Aspect | Contract |
|---|---|
| Invocation | `skystacker [flags] [input-directory]` |
| Success | exit `0` |
| Runtime failure | exit `1`, message on stderr as `skystacker: <error>` |
| Flag error | exit `2`, plus a pointer to `--help` |
| Stdout | progress and banner; do not parse it for machine data |
| Inputs | never modified; all products go to the output directory |
| Concurrency | one process per job; each uses private `_work`/temp dirs |

## Minimal remote job wrapper

```bash
#!/usr/bin/env bash
set -euo pipefail
job_id="$1"
src="$2"
out="/srv/jobs/$job_id"
mkdir -p "$out"
skystacker --process-image "$src" --image-op plate-solve \
  --image-output "$out" --plate-solver auto \
  --astrometry-timeout 180 >"$out/solve.log" 2>&1
test -f "$out/$(basename "${src%.*}")_plate_solution.json"
```

A worker can then read `plate_solution.json`, insert the WCS into its database
and return the JSON verbatim to the caller. See the complete Python parser in
[plate-solving.md](plate-solving.md) §6.

## Queueing multiple runs

`--batch FILE` accepts a JSON array of argument lists and executes them
sequentially, stopping on the first error. Batch children suppress the donation
footer.

```json
[
  ["--process-image", "a.jpg", "--image-op", "plate-solve", "--image-output", "/tmp/a"],
  ["--process-image", "b.jpg", "--image-op", "plate-solve", "--image-output", "/tmp/b"]
]
```

## Configuration files

`--config FILE` prepends flags from a JSON object; explicit CLI flags win.
Values may be strings, numbers, booleans or arrays:

```json
{
  "method": "sigma",
  "workers": 8,
  "json-report": true
}
```

## Network and secrets

- Gaia TAP and nova uploads are opt-in per backend.
- Set `AN_API_KEY` in the worker environment rather than passing it on the
  command line; API keys are redacted from report command lines, but process
  tables are not.
- `--gaia-offline` guarantees no Gaia network access.
- `--astrometry-timeout` bounds local CLI and web calls; combine with an
  external supervisor timeout for hard SLAs.
- Uploads default to private (`publicly_visible=n`).

## Recommended pipeline for an online service

1. Accept an image upload into a job directory.
2. Run `skystacker --process-image … --image-op plate-solve --plate-solver auto`.
3. On exit `0`, parse `plate_solution.json` and convert CRVAL/CD to degrees for
   the API response.
4. On exit `1`, return the stderr line and keep `solve.log` for diagnostics.
5. Optionally chain `--image-op sky-map` or `survey-cutout` with the same output
   directory, which reuses `plate_solution.json`.
6. Delete the job directory or keep it according to retention policy.

## Observability

- `--json-report` and `--provenance` provide structured run metadata.
- `--diagnose-integrated` adds host/runtime diagnostics.
- `--blind-benchmark` plus a fixture corpus provides regression metrics across
  solver backends; `scripts/bench-blind.sh` runs the real 14-image corpus.
- For offline deployments, ship the small Gaia quad catalogs from `catalogs/`
  (or prefetch with `--download-blind-catalog`) alongside the binary and call
  `--plate-solver native --blind-catalog ID`; no network is needed at solve
  time. See [plate-solving.md](plate-solving.md) and
  [catalogs/README.md](../catalogs/README.md).

## Consuming layered SVG products

The sky-map SVG is designed for machine consumption: select groups by
`#gaia-stars`, `#extended-objects`, `#sso-vectors`, `#legend`, `#labels` or
`#footer`, individual symbol families through `[data-symbol="<family>"]`, key
swatches through `.skystacker-key-<family>`, proper-motion arrows and
variability brackets through `[data-overlay="proper-motion"]` and
`[data-overlay="variability"]`, or filter on
`data-layer`/`class="skystacker-layer"`. Indexed stars also expose
`data-spectral` and `data-colour-mode="index"`. Group ids and `data-*`
attributes are stable; text content and styling may change. A web frontend can
therefore hide the label layer, restyle point sources or render SSO arrows in a
different colour without invoking SkyStacker again. The companion
`annotation_provenance.json` reports `geometry`, `overlays` and `name_matches`
counts for the
same products. The exact group schema is
listed in [output-formats.md](output-formats.md).

## Limits

| Resource | Control |
|---|---|
| CPU | `--workers` |
| Memory | `--tile-size`, stack size, `--force` only as a last resort |
| Disk | output directory, `_work` scratch, `--clean` |
| Network | backend choice, `--astrometry-timeout`, `--gaia-offline` |
| Time | external supervisor timeout |

See [troubleshooting.md](troubleshooting.md) for failure signatures.

# SkyStacker benchmarks

Orientation data, not a rigorous benchmark suite. Times are wall-clock
measurements on the developer's machine with the settings shown. They are
intended to make performance regressions visible and to give a rough sense of
cost for each stage; **they are not a comparison of output quality**.

## 1. Environment

| Item | Value |
|---|---|
| CPU | Intel Core i7-8565U (4 cores / 8 threads, 1.80 GHz base, up to 4.6 GHz turbo, 15 W ULV) |
| Memory | 15 GiB RAM (8.3 GiB available when measured) |
| Architecture / OS | x86_64, Ubuntu 20.04.6 LTS, Linux 5.15.0-139-generic |
| Go toolchain | go1.27.4 linux/amd64 (upgraded 2026-11-17; earlier rows used go1.26.11 or go1.20.3) |
| SkyStacker | v0.6.1 (`main` at time of measurement; `--version` prints 0.6.1). Later builds add native CR2 decoding, which further reduces CR2 stack time vs the dcraw path |
| Canonical binary | `/home/jips/.local/bin/skystacker` (built with `-ldflags "-X main.buildDate=..."`) |
| RAW decoder | external `dcraw`/`dcraw_emu` or ImageMagick (CR2/CR3 inputs) |
| Storage | local SSD/HDD, output under `/tmp` or `assets/.../0_skystacker_results` |
| Workers | 8 (unless noted) |
| Thermal / background load | not controlled; run-to-run variance of several seconds is normal |

## 2. Measurement method

- **Stacking runs:** the number reported as `Duration:` in `report.txt` (the
  same value as `Actual processing time:` on stdout), i.e. from `runPipeline`
  start to report write.
- **Post-processing-only runs:** external `/usr/bin/time -f 'WALL=%e s'`
  around `skystacker --output <dir> --resume ...`, which detects the existing
  `_work/stacked_mean.f32raw` and goes straight to post-processing.
- Command lines are quoted verbatim so the flags that affect cost are visible.
- **Kernel benchmarks:** `go test -run '^$' -bench 'Benchmark(SigmaClip|ReduceTilesStack|NativeCR2Decode)' -benchtime=2s -count=3 ./...`.
  `go test` reports ns/op per run; compute the mean and coefficient of variation
  (CV = σ/mean) across the three samples and record both. Pin the CPU governor
  where possible and keep the machine otherwise idle; on this 15 W ULV laptop
  thermal variance is otherwise several seconds even for end-to-end runs.

## 3. SkyStacker stacking measurements

Source: `report.txt` files kept under `/tmp/onb_*` (OrionNebula data set:
16 CR2 lights, 2602x3908 px, plus lowercase `darks/`, `biases/`, `flats/`).

| Run | Frames | Method / flags (abridged) | Duration |
|---|---:|---|---:|
| Grid self-flat vs dark+bias | 16 | `sigma`, `--stack-only --self-flat-model=grid --dark darks --bias biases` | **6m 4.865s** |
| Real flat vs dark+flat+bias | 16 | `sigma`, `--stack-only --dark darks --flat flats --bias biases` | **7m 2.905s** |
| Flags smoke (few frames) | 4 | `sigma`, `--stack-only --fits --xisf --bpm-auto --dark darks --bias biases` | **2m 2.361s** |
| Combined flags | 4 | `sigma`, `--stack-only --fits --xisf --json-report --drizzle-upsample 2 --normalize --normalize-mode additive --sigma-clip-low 2 --dark darks --bias biases` | **3m 41.374s** |

Notes:

- Duration includes CR2 decoding, calibration, star detection, registration,
  tiled sigma reduction (176 tiles for full-frame runs), map/JPEG writing and
  (when requested) FITS/XISF/JSON/drizzle output. It excludes process startup.
- The 16-frame runs have `--limit 16`; the 4-frame runs use `--limit 4`.
- `report.txt` records the exact command line, resolved flags, crop and
  normalisation factor for each run; no data was modified.

Historical calibration points used by the built-in time estimator
(`TECHNICAL_SPEC.md` §5), recorded before this environment was standardised:

| Data set | Frames | Duration (as documented) |
|---|---:|---:|
| Orion (135 mm) | 70 | 9m 58s |
| M42 | 152 | 25m 22s |

These two rows are **not** directly comparable with the runs above (different
sensors, resolutions, flags and machine conditions); they are kept only as the
time-model calibration history.

The M42 50 mm session executed by the user in this environment (11 JPG frames,
2256x1504 px, 8 workers) printed an **estimated** time of ~1m 40s; no measured
duration was captured for that run.

## 4. Post-processing-only measurements

Same i7-8565U, 8 workers, input stack `_work/stacked_mean.f32raw` from
`/tmp/onb_combo` (OrionNebula, 2599x3894 px after crop), reached with `--resume`:

| Run | Flags | Wall time |
|---|---|---:|
| Default post pipeline + starless layers | `--resume --starless` | **22.43 s** |
| Richardson–Lucy + multi-scale detail | `--resume --deconvolve --deconv-iters 1 --wavesharp` | **38.30 s** |

The delta (~16 s) is dominated by the separable-Gaussian convolutions used by
`--deconvolve` and `--wavesharp`. Both are deliberately opt-in; the cost scales
roughly linearly with pixel count, iterations and kernel radius.

## 5. Built-in time model

For reference, `runPipeline` estimates duration as (see `TECHNICAL_SPEC.md` §5):

- `sigma`: `1.03 * N^1.20 * MP`
- `median`: `0.88 * N^1.18 * MP`
- `mean`: `0.55 * N * MP`

plus ~12 s/frame for CR2/CR3 decoding and post-processing terms. This is a
planning heuristic, not a measurement.

## 6. Comparative section (other stacking applications)

No other stacking application is installed on this machine, so there is **no
same-data, same-machine comparison available**. The only responsible way to
compare is to collect published measurements and discuss CPU equivalence
explicitly. The figures below are external reference points; none of them is
an apples-to-apples benchmark against SkyStacker.

### 6.1 CPU equivalence discussion

Our CPU is an **i7-8565U**, a 15 W ULV laptop part (4 cores / 8 threads,
~2.0 GHz all-core, up to 4.6 GHz single-core turbo). Public benchmark suites
use desktop parts or higher-power laptop parts:

- **Closest low-power analogue found:** i5-6200U (2 cores / 4 threads,
  15 W, Skylake-U). It has half the cores and no modern ISA advantages; DSS
  timings on it are a *lower bound* for what our i7-8565U should do on the same
  workload, not a direct comparison.
- **Closest core-count analogue:** i7-7700HQ (4 cores / 8 threads, 45 W laptop
  part). Same logical thread count as our CPU but much higher sustained power
  and clocks, so it is likely substantially faster in CPU-bound stacking.
- Desktop i7-9700K (8c/8t), Ryzen 5 5500, FX-8320, etc., are not equivalent;
  they are listed only as context.

### 6.2 Published reference points (external, not reproducible here)

| Source | Hardware | Workload | Reported time |
|---|---|---|---:|
| [DSS benchmark, Stargazers Lounge](https://stargazerslounge.com/topic/357613-i-benchmarked-dss-on-5-different-pcs/) | i5-6200U 2c/4t, 2.7 GHz | DeepSkyStacker, 10 Canon 700D RAW lights (single-thread / multi-thread) | 275 s / **141 s** |
| [Benny's AstroBlog](https://www.bennys-astroblog.de/blog-thoughts-news-activties/1635809_stacking-with-pixinsight-vs-siril) | FX-8320 8c @3.5 GHz, 16 GB | 41 lights 4144x2822 + 18 dark + 15 flat + 40 bias, with local normalisation | PixInsight WBPP **40m 07s**; Siril **9m 55s** |
| [Astrotreff comparison](https://www.astrotreff.de/forum/index.php?thread%2F278318-stackingvergleich-app-dss-siril-und-pi%2F=) | differing Windows/Linux PCs | ~88–118 lights, ~7 GB total | Siril 6m 14s (Win) / 4m 30s (Linux); APP 1h 53m; DSS 46m; PI 2h 45m (crashed) |
| [Cloudy Nights PI vs Siril](https://www.cloudynights.com/forums/topic/921166-stacking-performance-pi-vs-siril/) | i7-9700K 8c, 64 GB, SSD | ~400 lights | PI WBPP ~4 h; Siril ~30 min or less |
| [astrojolo CPU hunger](https://astrojolo.com/gears/astro-applications-cpu-hunger/) | many CPUs incl. i7-7700HQ 4c/8t and i5-6200U 2c/4t | PI star alignment/integration, MaxIm DL 6, AutoStakkert! 3 | numeric table is an image; qualitative conclusion: CPU-bound, good multicore scaling for PI/AS!, MaxIm mostly single-core |

### 6.3 What can and cannot be concluded

- Published runs differ in sensor, frame count, resolution, calibration frames,
  debayer path, weighting/normalisation, drizzle and output format. Do **not**
  compare their raw seconds against the SkyStacker rows in §3 and declare a
  winner.
- The consistent qualitative finding is that stacking is CPU-bound and
  scales with cores/clock; that matches our estimator and the observed cost of
  extra stages (FITS/XISF writing, drizzle, deconvolution).
- A fair comparison would require the same input files on the same machine.
  The practical next step, if another stacking tool is installed later, is to
  add a row with the exact command/config and a shared dataset hash.
- Planetary poor-seeing workflows (AutoStakkert! lucky imaging, waveSharp /
  RegiStax wavelets, WinJUPOS derotation) solve a different problem and are not
  comparable to deep-sky stacking times; see `research/03-reference-and-literature.md` §5.

## 7. Reproducing a row

```bash
# Stacking row (writes report.txt with Duration)
./skystacker --output /tmp/bench_run --overwrite --stack-only --limit 16 \
  --dark assets/OrionNebula/darks --bias assets/OrionNebula/biases \
  assets/OrionNebula/lights

# Post-processing row (requires an existing stack in that output dir)
/usr/bin/time -f 'WALL=%e s' ./skystacker --output /tmp/bench_run --resume --starless \
  assets/OrionNebula/lights
```

Record: CPU/RAM, OS, Go version, SkyStacker version/commit, exact flags, frame
count and resolution, method (`mean`/`median`/`sigma`), workers, and the
duration from `report.txt`.

`scripts/bench-row.sh` extracts those fields from a `report.txt` and prints a
Markdown row (CPU/RAM and thread count are read from the current machine):

```bash
./scripts/bench-row.sh /tmp/bench_run/0_skystacker_results/report.txt
# append to this file instead:
./scripts/bench-row.sh /tmp/bench_run/0_skystacker_results/report.txt --append BENCHMARKS.md
```

For post-processing-only rows, time the run externally (see §4) and add the
wall time manually to the row; `report.txt` is not rewritten by `--resume`.
### Post-upgrade measurements (Go 1.26.11, native CR2 decoder)

| Run | Frames | Flags (abridged) | Duration |
|---|---:|---|---:|
| Native CR2 stack | 4 | `sigma`, `--stack-only --dark --bias` (built-in CR2 decode) | **59.937 s** (wall 60.15 s) |
| Post-processing + starless | 1 stack | `--resume --starless` | **23.00 s** wall |

The earlier 4-frame row (2m 02s) used Go 1.20.3 and the external dcraw path; it is not directly comparable because both the toolchain and the CR2 decoder changed. The 22.43 s post-processing baseline was Go 1.20.3; 23.00 s is within run-to-run variance on this thermal-throttling laptop.

### Go 1.27.4 re-baseline (2026-11-17)

| Run | Frames | Flags (abridged) | Duration |
|---|---:|---|---:|
| Native CR2 stack | 4 | `sigma`, `--stack-only --dark --bias` (built-in CR2 decode) | **39.031 s** (wall 39.25 s) |
| Post-processing + starless | 1 stack | `--resume --starless` | **21.11 s** wall |

Commands: `/home/jips/.local/bin/skystacker --output /tmp/bench_go127 --overwrite --stack-only --limit 4 --dark assets/OrionNebula/darks --bias assets/OrionNebula/biases assets/OrionNebula/lights` and the same output dir with `--resume --starless`. Same data and flags as the Go 1.26.11 rows above; the 59.937 s → 39.031 s delta includes Go 1.27 runtime improvements and normal run-to-run variance on this thermally throttled laptop, not a controlled A/B.

All 264 self-tests, `go vet`, `go test`, goPureRaw CGO=0/race tests, race build, and six cross-compilation targets are green on go1.27.4.

### Sigma-clip kernel microbenchmark (fused vs two-pass, 2026-11-17)

`go test -run '^$' -bench BenchmarkSigmaClip -benchtime=2s -count=3` on the i7-8565U, 16 frames, 2 iterations, one injected outlier (median of three runs):

| Variant | Fused (ns/op) | Two-pass reference (ns/op) |
|---|---:|---:|
| Unweighted | ~763 | ~1048 |
| Weighted | ~833 | ~1033 |

The end-to-end 4-frame sigma stack remains ~39–45 s under variable machine load; the kernel gain is partly masked by CR2 decoding, registration and disk spill I/O. The fused output is bit-identical to the two-pass reference (same SHA-256 of `stacked_mean.f32raw` for the same OrionNebula 4-frame run).



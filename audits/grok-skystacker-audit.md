# SkyStacker — Deep Audit

Scope note: this audit uses only the serialized tree. `main.go` itself is **not** in the bundle (102 files; the monolith appears only as `main-function-map.txt` plus `package main` satellites). Line-level claims about `main.go` are limited to that map and to statements already written in the docs. Prior-review citations such as `main.go:721-741` are **not re-verified** against source. Satellite files (`blind_engine.go`, `goTiff/`, `goPureRaw/`, `goDiagnostics/`, `archive_adapters.go`, and the docs) were read in full.

## 1. Executive summary

SkyStacker v0.6.1 is a capable local stacking CLI with a real scientific pipeline (calibration, registration, tiled sigma clipping, plate solving, scientific writers) and an unusually large regression surface (274 embedded cases plus a growing `*_test.go` set). Recent TIFF/CR2 review items were closed with tests rather than hand-waving (ROADMAP 104, 105, 107, 108, 110). That is healthy.

The weak spots are not the mean/sigma stacker. They are (1) a confirmed verifier bug on the blind-solve path that is still 0/14 on the published corpus, (2) parser allocation bounds that are strict in `goTiff` and loose in lossless JPEG / SBI1, (3) user-visible documentation that contradicts itself on defaults, and (4) a diagnostics/network surface that can record or transmit more than the operator expects. The monolith constraint is still coherent; it is not the thing to “fix” next.

Five findings, in impact order:

1. **Confirmed:** `scaleConsistentMatches` does not implement the median scale gate its comment describes. It uses the first quartile and anchors every comparison to `matches[0]` (`blind_engine.go`, `scaleConsistentMatches`). A collision in the first pair can keep the outlier and drop true matches. The existing test only puts the outlier last.
2. **Confirmed:** untrusted-file bounds are uneven. `goTiff` caps reads at 1 GiB and pixels at 2e8. `ljpeg.Decode` allocates `width*height` per component with no cap. `readNativeIndexBinary` caps stars at `1<<24` but not tile or quad counts.
3. **Confirmed doc drift with possible behaviour impact:** `--method` is `sigma` in `TECHNICAL_SPEC.md` and `README.md`, and `mean` in `docs/cli-reference.md`. Self-flat `grid` is both “the default” and “not the default yet” in `README.md`. Recovery text still expects 169 self-tests.
4. **Confirmed privacy:** Linux diagnostics copy `/proc/<pid>/cmdline` into JSON with no redaction (`goDiagnostics/collect_linux.go` `Processes`). `--diagnose-integrated` on a process whose argv contains `--astrometry-api-key` or `--mast-token` will persist those secrets. Separately, the documented service recipe (`--plate-solver auto` plus `AN_API_KEY` in the environment) uploads the image to nova.
5. **Confirmed test-architecture gap:** `main_test.go` does not call `runSelfTests`. `go test ./...` does not run the 274 authoritative cases. `goDiagnostics` tests swap a fake platform and never exercise `collect_linux.go`.

## 2. Architecture & module boundaries

The split matches the project rule and is coherent.

| Unit | Role | Boundary judgement |
|---|---|---|
| Root `package main` (`main.go` + satellites + `donations.go`) | CLI, pipeline, astrometry, reports | Correct under the no-split rule. Satellites (`image_mode.go`, `sky_products.go`, `blind_*.go`, `tap_async.go`, …) are the right way to grow without a new package. |
| `goPureRaw` | CR2 decode + RAW container TIFF/Exif | Right extraction. ARW/CR3/RAF are probe-only stubs returning `ErrUnsupportedVariant`. |
| `goTiff` | Image TIFF/BigTIFF codec | Right extraction. `tiff_output.go` / `tiff_input.go` are thin wrappers after item 110. |
| `goDiagnostics` | Host snapshot behind a `platform` interface | Right extraction. Build-tagged collectors, no CGO. |
| `github.com/skip2/go-qrcode` | Terminal QR only | Isolated in `donations.go`. Do not grow this dependency set. |

Two TIFF readers is not a layering bug. `goPureRaw/internal/tiff` is a RAW-IFD walker (Make/Model/Exif, SubIFD). `goTiff` is a pixel codec. They should stay separate; a RAW container must not depend on the scientific image writer.

What should **not** be extracted now: HEALPix, WCS, Gaia TAP, and the blind index. They are pure enough for a future `goHealpix` / `goWCS`, but nothing in the current defects requires that move, and the project rule is not to split the root package unless asked. A module extraction would also freeze a path once tagged. Defer it.

Misplaced or awkward responsibilities that are already inside the allowed shape:

- Package-level solver switches (`plateGaiaOffline`, `plateEpochYear`, `matchedFilterEnabled`) are assigned from `image_mode.go` `imageWCS` / `runImageCommand` and restored with `defer`. Safe for a single-threaded CLI; unsafe if batch or a future worker ever overlaps two solves. `fitsBlackLevelFor` is still in the function map (`main-function-map.txt:31767`) beside `parseFITSHeader` (`:31698`). `PROJECT_CONTEXT.md` already called that side channel a follow-up. It was not done.
- `goDiagnostics` names a collector `Updates` and fills it with installed packages (`collect_linux.go` `Updates`, dpkg/rpm/apk/pacman `-Q`). That rename is already ROADMAP 106. Not repeated below.
- The comment immediately above `estimateArcsecPerPixel` in `blind_engine.go` describes `detrendedBlindStars`. The function it documents is the next one down. Small, but it shows the blind-solver file is being edited faster than it is being read.

No import cycle is possible across the module boundary: `goPureRaw`, `goTiff`, and `goDiagnostics` do not import `main`.

## 3. Correctness & robustness

### CONFIRMED

**C1. Scale gate is not a median and is anchored to pair 0.** `blind_engine.go`, `scaleConsistentMatches`.

The comment says the median pairwise scale rejects descriptor collisions. The code does:

- `sort.Float64s(scales)` then `median := scales[len(scales)/4]` — index `len/4` is the first quartile, not `len/2`.
- The acceptance loop always calls `matchStar(matches[0], ...)` and measures pixel/sky separation against `matches[0]`, not against a robust reference.

`TestScaleConsistentMatches` (`blind_index_test.go`) builds matches `{0,0},{1,1},{2,2},{3,3}` with the bad catalogue star last, so `matches[0]` is a good anchor and the test passes. Swap the outlier to index 0 and the gate measures every other pair against the collision. That is the failure mode the blind corpus write-up already describes (spurious quad votes, verification never reaching 6 inliers). This bug can make that worse. It is not listed in `ROADMAP.md`.

**C2. Lossless JPEG has no pixel cap.** `goPureRaw/internal/ljpeg/ljpeg.go`, `Decode`, `img.Planes[c] = make([]uint16, width*height)`. SOF3 width/height are only checked for `> 0`. `goPureRaw/formats/cr2/decoder.go` `findLosslessJPEG` rejects absurd precision/component counts, then still `ReadAt`s the entire tail and `Decode`s it. A crafted CR2/SOF3 is an OOM, not a `ErrCorrupt`. `goTiff` already refuses this class of input (`maxPixels`, `maxReadSize` in `goTiff/header.go`).

**C3. SBI1 reader bounds stars only.** `blind_index.go` `readNativeIndexBinary`: `starCount > 1<<24` errors. `header.Tiles` (`uint32`) and `header.Quads` (`uint64`) drive `for range` loops that allocate before any sanity check. `--blind-index` points at a local file; that is still an untrusted-file parser if indexes are downloaded (`--download-blind-catalog`) or shared.

**C4. CR2 photometric constants are an EOS 40D special case.** `goPureRaw/formats/cr2/decoder.go` `Decode` always returns `CFAPattern` RGGB, `BlackLevel` 1018, `WhiteLevel` 16224. Item 107 correctly closed the 2× width question. Item 10 (blocked) is geometric agreement with dcraw, not this. Any other CR2 body gets the 40D pedestal and a forced RGGB mosaic. `TestDecodeOracle` skips unless `PURERAW_CR2` is set, and it does not check black level.

**C5. Linux diagnostics ignore cancellation and mis-label CPU.** `goDiagnostics/collect_linux.go` `Processes(ctx, n)` never reads `ctx`. For each PID (cap 4000) it `ReadDir`s `/proc/<pid>/fd`. That is ROADMAP 106; reconfirmed here. Independently, `CPUPct` is `(utime+stime)/total_boot_ticks*100`, a lifetime share of all CPU time since boot, then clamped to 100. `Text()` prints it as if it were current CPU. Darwin `Memory` assigns `pages("Swapused")` to `SwapTotalMB` (`collect_darwin.go`).

**C6. Process command lines are not redacted.** `RedactValue` (`goDiagnostics/common.go`) applies to the env whitelist only. `Processes` stores `Cmd` raw. `Report.JSON()` emits it. A secret in argv survives. `RedactValue` itself redacts any token longer than 40 characters with no space, and any string containing `token`/`secret`/`password`. Short API keys and flag forms like `--mast-token secret` are not covered by that helper even if it were applied, because the needles are substrings of the whole command line only if someone calls `RedactValue` on the full argv — which nobody does.

**C7. Archive HTTP client has no caller context and follows redirects with a bearer token.** `archive_adapters.go` `queryArchiveAdapter` builds `http.NewRequest` (no `context`), sets `Authorization: Bearer` when `TokenHeader` is set (MAST), and uses a default `http.Client` (follows redirects, 30 s timeout, 8 MiB body cap). `tap_async.go` correctly sets `CheckRedirect` to `http.ErrUseLastResponse`. The archive client does not. `MAST_TOKEN` / `--mast-token` is the credential.

**C8. Cone truncation failures are dropped.** `cone_report.go` `computeConeTruncationReport` returns `nil` on `coneSearchCachedDetailed` error. Archive queries record `Error` on the result (`computeArchiveAdaptersReport`). Cone search does not. A TAP/VO outage looks like “no truncation section”.

**C9. RA mean in density capping wraps badly.** `blind_index.go` `selectDensityUniformStars` averages `s.RA` arithmetically, then bins offsets from that centre. Stars on either side of RA 0 land in different cells. `normalizedAngle` exists and is not used for the centre. Low-nside index builds are the ones that span the wrap.

**C10. Doc/code default contradictions (behaviour unverified).** See section 6. Until `parseArgs` (`main-function-map.txt:456`) is read, which default `--method` actually has is unknown. The docs cannot all be right.

**C11. Implicit web upload in the documented auto chain.** `docs/plate-solving.md` §1 step 4 and `docs/integration.md` tell a worker to export `AN_API_KEY` and run `--plate-solver auto`. Auto falls through to nova when local solvers are missing. The image leaves the machine. Uploads are documented as `publicly_visible=n`, which is not the same as “not uploaded”.

**C12. `goTiff` cannot read back every file it can describe.** `DecodeAll` (`goTiff/tiff.go`) `ReadAll`s up to `maxReadSize+1` (1 GiB) and errors with `file too large`. `--bigtiff` exists specifically for stacks above 4 GiB (`TECHNICAL_SPEC.md`, README). A BigTIFF larger than 1 GiB is writable in intent and unreadable by this module. `TestBigTIFFRoundTrip` uses a 16×8 image. `TestBigTIFFLONG8OffsetParsing` checks an offset, not a large payload.

### HYPOTHESES (need the missing `main.go` body or a runtime repro)

**H1. EXIF GPS is stored in RA/Dec and used as a celestial seed.** `image_mode.go` `imageWCS` does `meta.RADec()` and passes the result to `solvePlateWithFallback` as `seedRA, seedDec`. ROADMAP 72 (Done) says GPS latitude/longitude are parsed into `exifData.RA/Dec`. Geographic latitude is not declination. If `RADec()` (`main-function-map.txt:5012`) does not convert alt/az + time + site, every GPS JPEG is seeded at the wrong sky position and can emit a confident wrong solve or a false “no consistent transform”. The conversion is not visible in this bundle. `testEXIFGPSSeed` exists (`:19696`); its assertions are not.

**H2. `fitsBlackLevelFor(path)` returns the last parsed file, not `path`.** The side channel is still a separate function. `PROJECT_CONTEXT.md` says it is mutex-guarded. Path identity under concurrency is not visible. Single-threaded CLI use is probably fine; two FITS masters decoded on different goroutines would not be.

**H3. Gaia rate limiter sleeps under `gaiaRateMu`.** ROADMAP P2-4. `gaiaRateLimit` is only a map entry (`:17164`). `TestGaiaRateLimitSynctest` proves a 1 s wait, not lock scope. Do not treat P2-4 as reconfirmed.

**H4. Plate solve with fewer than 5 stars is silent.** ROADMAP P3-5, `main.go:11725-11883`, not in this bundle.

**H5. `runPostProcessing` ignores `--workers`.** ROADMAP P2-2. Signature is `runPostProcessing(outputDir string, opts runOptions)` at map line 31281. Body not present.

**H6. VOTable BINARY2 nulls omit payload bytes.** `votable.go` writer and reader both skip null field bytes and round-trip with each other (`TestVOTableBINARY2RoundTrip`). Whether that matches IVOA fixed-stride BINARY2 is not established by anything in the bundle. Internal consumers are fine; foreign VOTable tools are an open question.

**H7. `--mast-token` / `--astrometry-api-key` appear in `report.txt` `Command line:`.** Docs say astrometry keys are redacted. The redaction function is not in the bundle (`formatCommandLine` at map `:5583`). MAST is not named in that redaction sentence.

**H8. Relative `Location` on async TAP.** `tap_async.go` `submitTAPAsyncJob` uses the `Location` header as the next URL. A relative Location fails the poll and the caller falls back to sync (`computeTAPAsyncReport`). Degraded, not corrupt, if the fallback path works. Not reproduced against ESA.

## 4. Performance & scalability

Numbers below are from the serialized docs and from buffer arithmetic on the stated float32 RGB layout (12 bytes/pixel). No new benchmarks were run.

A 60 MP frame is 720 MB. The documented mean estimate is `frameMB * 5` plus a `1.5× + 2000 MB` peak margin (`TECHNICAL_SPEC.md` §5) — on the order of 7 GB before the safety check fires. Sigma/median spill is `frames * frameBytes * 2 + 500 MB` on the output filesystem only (the docs already say the spill dirs share that filesystem). Two hundred 60 MP frames are on the order of 280 GB of spill. The pre-flight is the right design; it will abort, which is what you want. It will not make the run fit.

What the hot path actually is, from the function map and the satellites:

- Detection: `detectStarsSigma` (`:2937`), high-pass radius `clamp(min(W,H)/200, 8, 24)`. At 60 MP the radius hits the cap of 24. That is O(pixels × radius) unless `boxBlurLuminance` (`:2722`) is separable — the body is not in the bundle, so separability is unknown.
- Resample: `resampleToReferenceFn` (`:3690`) takes `workers`. This path is already shaped for parallelism.
- Reduction: `reduceTilesStack` / `reduceTile` (`:8377`, `:8488`) are tiled and out-of-core, with a ~256 MB tile cap. Single-pass sigma clip is done and bit-checked (`testSinglePassSigmaClipEquivalence`, map `:19509`). Do not reopen that.
- Self-flat grid: `selfFlatGridAccumulator.addFrame` (`:9174`) walks every pixel of every light in Pass 1. On hundreds of 60 MP frames this dominates calibration. It is streaming (one frame at a time), which is the correct memory shape.
- Post-processing: `runPostProcessing` (`:31281`) is a long sequential chain (background, stars, STF, guided filter, curves, HDR, spikes, optional RL/Wiener/wavelets). P2-2 (workers) and P2-5 (Wiener PSF FFT per tile) are already in the ROADMAP backlog. Not re-specified here.
- Blind products: `placeLabels` in `sky_products.go` is 20 passes × 16 anchors × a scan of already placed boxes. A mag-14 tile over a wide field is thousands of sources. That is O(N²) and runs on the annotation path, not the stacker. It will show up the first time `--sky-map` is used on a dense field.
- Gaia: 1 request/second is intentional (`TestGaiaRateLimitSynctest`). A 100-tile blind build is minutes of sleep even when ESA is fast. That is a policy limit, not a bug. P2-4 (lock held during sleep) is the only code-level improvement, and it is already tracked.
- CR2: Huffman decode via `map[uint16]byte` in `ljpeg.go` is fine for one 40D file and poor if hundreds of CR2s are the common case. Native CR2 is still not bit-exact (item 10, blocked). Do not optimize it before the oracle exists.
- `goDiagnostics` process scan is the one collector that can stall a `--diagnose` that was supposed to honour `--diagnose-timeout` (item 106).

Parallelism already present and worth leaving alone: resample workers, accumulator workers, tile reduction, `--workers`. Parallel registration is explicitly deferred in `PROJECT_CONTEXT.md` and is not re-proposed.

Memory behaviour that is already good: sigma/median spill, rejection buffer pool (item 37, Done), atomic writes, disk pre-flight, `--scale` for previews. The 60 MP risk is peak float32 copies in post-processing, which cannot be counted without `main.go`. Treat “eight full-frame float buffers resident” as a hypothesis, not a finding.

## 5. Testing gaps

What is actually tested, from this bundle:

- Stacking numerics, WCS fit, HEALPix vs healpy fixtures, Gaia retry/429, self-flat grid, BPM, FITS/XISF, external sidecar mock, donation footer, and a long embedded list registered from `runSelfTests` (`main-function-map.txt:21915`).
- Newer work is in real `go test` files: `blind_index_test.go`, `archive_adapters_test.go`, `goTiff/tiff_multistrip_test.go`, `goTiff/fuzz_test.go`, `star_detection_test.go`, `wcs_refine_test.go`, `verify_solution_test.go`, and others.
- `main_test.go` has sigma-clip and tile benchmarks, a CR2 decode benchmark that skips without the fixture, and `TestGaiaRateLimitSynctest`. It does **not** call `runSelfTests`.

Gaps that can hide the confirmed bugs:

| Gap | Why it matters |
|---|---|
| No test with the scale-gate outlier in `matches[0]` | C1 is green today |
| `TestBinaryIndexRoundTrip` checks quad band, not quad star RA/Dec | A `0xffffffff` lookup that zeroes a quad vertex would pass |
| No ljpeg/CR2 fuzz file (goTiff has `FuzzDecode`) | C2 has no crash oracle |
| No SBI1 hostile-header test | C3 has no crash oracle |
| `goDiagnostics/diagnostics_test.go` only uses `fakePlatform` | `collect_linux.go` ctx, fd walk, and CPU formula never run under `go test` |
| `TestDecodeOracle` / `TestConfigOracle` skip without `PURERAW_CR2` | Item 10 cannot go green in CI; black-level hardcode is also untested |
| Blind corpus 0/14 (`docs/blind-corpus-results.md`) | Items 78–81 are executed, not accepted. Another engineering pass without a C1 fix and a seeded control is not evidence |
| GPS seed test body not visible | H1 may already be locked, or the test may only check that lat/lon parse |

Sustainability of `--selftest` vs `go test`: not sustainable as a dual source of truth, and it does not require splitting `main.go`. The embedded suite is the contract (`TECHNICAL_SPEC.md` §2: the count `runSelfTests` prints is authoritative). `*_test.go` is the right place for new cases — several November features already did this — but the 274 cases are invisible to `go test` and to coverage. CI workflow YAML is not in the bundle, so it is **not** verified that every job runs `./skystacker --selftest`. Docs say both; that is a claim, not a log.

A durable rule that respects the monolith constraint: new tests go in `*_test.go`; `main_test.go` gains one test that calls `runSelfTests` and fails if the returned failure count is non-zero; stop copying assertions into both places (that duplication is already P4-1, effort L — do not start P4-1 before the bridge).

## 6. CLI, docs & UX consistency

Contradictions a user will hit. The live flag is whichever `parseArgs` / `printUsage` (`:456`, `:1418`) implement; those bodies are not here.

| Topic | Says A | Says B |
|---|---|---|
| `--method` default | `sigma` — `TECHNICAL_SPEC.md` §3, `README.md` stacking table | `mean` — `docs/cli-reference.md` |
| Self-flat model | `grid` default — CLI table in `README.md`, `TECHNICAL_SPEC.md`, `docs/cli-reference.md` | “It is not the default yet” — `README.md` calibration section, same page |
| Self-test count | 274 — headers of `PROJECT_CONTEXT.md`, `README.md`, `TECHNICAL_SPEC.md`, `docs/installation.md` | “all 169 self-tests passed” — `PROJECT_CONTEXT.md` Recovery section |
| Program shape | Multi-file root + three modules — `README.md` Scope | “single-file Go program, `main.go`” — `PROJECT_CONTEXT.md` header |
| Go version | 1.27.4 — `README.md` requirements, `docs/installation.md` | Badge “Go 1.20+” — `README.md` top |
| Spike example | Flags default length 10, intensity 0.3 — `PROJECT_CONTEXT.md` flag list and `README.md` | Worked table still uses `baseLength=100`, `baseIntensity=0.7` — same `PROJECT_CONTEXT.md` section |
| Blind solve | Native quad index + astrometry.net/ASTAP/nova — `docs/plate-solving.md`, `TECHNICAL_SPEC.md` §2 | “legacy internal blind mode scans only a 3×3 nside=64 patch … a real field needs an external backend” — `docs/troubleshooting.md` |
| Native RAW | CR2 pure-Go included — `TECHNICAL_SPEC.md` §2 Included, `README.md` | “Native RAW decoding (delegated to ImageMagick / dcraw)” — `TECHNICAL_SPEC.md` §2 Excluded, same section |
| Comparison gaps | `--drizzle-upsample`, `--deconvolve`, `--deconvolve-wiener`, `--fits`, `--xisf` shipped | `README.md` comparison cell: “No GUI, no drizzle/mosaic, no PSF deconvolution” |
| Dependencies | `go-qrcode` is a compile dependency; dcraw/ImageMagick/solve-field/astap are optional runtime | Comparison table: “zero external runtime dependencies” |

`printUsage` vs `parseArgs` drift has bitten this project before (spike defaults, 2026-11-14). `testParseArgs` exists (`:24325`) and is the right lock, but it cannot fix `docs/cli-reference.md` unless someone diffs the two. `docs/cli-reference.md` claims to mirror `--help`. If `--help` matches `parseArgs` and the reference matches neither, the reference is the stale artifact. That is the first doc to correct after reading `printUsage`.

`PROJECT_CONTEXT.md` is append-only by policy, so historical “169 tests” lines in session logs should stay. The Recovery block and the opening spike table are presented as current. Those should be corrected in place; that is not a violation of append-only history.

UX that is actually consistent and worth keeping: synthetic calibration refusal (`checkSyntheticCalibRefusal`), `--gaia-offline` vs `--gaia-cache-refresh` mutual exclusion, API-key redaction claim for nova, donation footer suppressed on `--stack-only` / batch children, inputs never modified, atomic writes. Do not “simplify” the refusal contract.

## 7. Security & privacy

This is a local CLI. The operator is trusted with their own files. The interesting boundaries are untrusted images, opt-in network, and diagnostics.

**Parsing.** TIFF input is in good shape after items 104–110 and the goTiff caps (`maxIFDEntries` 256, `maxReadSize` 1 GiB, PackBits bounded in `goTiff/compress.go`). CR2/ljpeg and SBI1 are not (C2, C3). SER, FITS, and the in-tree FITS writer live in `main.go` and were not re-audited. `goPureRaw/internal/tiff.readIFD` rejects 0 or >4096 entries (P3-1, applied). `Register` panics on a duplicate format (`goPureRaw/raw/registry.go`); that is P4-4, already backlog, and it is a startup panic not a file-triggered one unless a test or a future `init` double-registers.

**Network.** Leaving the machine, when the matching flag or auto-chain step is on:

- Gaia TAP: pointing polygon, User-Agent `SkyStacker/0.6.1` (`tap_async.go`).
- nova.astrometry.net: the image (C11).
- CDS hips2fits: RA/Dec/FOV/size, up to 512 MiB response (`sky_products.go` `fetchHiPSCutout`).
- MAST/ESASky/IRSA: RA/Dec plus optional bearer token (C7).
- Cone search: SDSS/IRSA/VizieR.

`--gaia-offline` does not stop hips, archives, cone, or nova. There is no single offline switch. That matters for the air-gapped claim in the donation copy and in `docs/integration.md`.

**Secrets.** `AN_API_KEY`, `ASTROMETRY_API_KEY`, `MAST_TOKEN`, `--astrometry-api-key`, `--mast-token`. Env whitelist in `goDiagnostics/common.go` does not include those names, which is correct. Cmdline capture defeats that (C6). Report-line redaction is documented for the nova key only (H7).

**Shell.** External sidecars and `solve-field` / ASTAP are `exec` with a quoted tokenizer (`splitCommandArgv`, map `:4398`; `solve_field_args_test.go` rejects an unterminated quote). No shell is the right design. It is still arbitrary command execution if a remote wrapper forwards user flags (`--astrometry-cmd`, `--external-denoise`, `--batch`, `--config`). `docs/integration.md` shows a fixed argv. Keep it that way; do not pass through client flags.

**Diagnostics collectors** run fixed commands (`df`, `ps`, `dpkg-query`, `powershell` with an integer interpolated into `-First`). That is not injection. Windows `psJSON` timeout is 2 s. Not a vulnerability.

**Path traversal.** Output, cache, BPM, include-list, and batch paths are operator paths. No evidence of sanitization, and none is required for a local tool. Do not add a path jail unless the CLI is exposed as a service — and if it is, the nova upload and the exec flags are the larger problems.

**Resource exhaustion.** TIFF is capped. Archive bodies are capped at 8 MiB. HiPS at 512 MiB. ljpeg and SBI1 are not capped (C2, C3). `placeLabels` and an uncapped blind-index load are the CPU/memory pair on the astrometry side.

Donation QR codes encode the two public addresses in `donations.go`. They do not phone home.

## 8. Proposed NEW roadmap items

Not in `ROADMAP.md` (including the 2026-11-21 P-list and items 1–110). No new dependencies, no GPU/SIMD/ML, no split of `package main`. Research owners are the canonical files the tracker already uses. Ordered by ascending effort.

| ID | Title | Effort | Status | Owner | Falsifiable acceptance |
|---|---|---|---|---|---|
| 111 | Reconcile `--method` default across `parseArgs`, `--help`, `TECHNICAL_SPEC.md`, `README.md`, and `docs/cli-reference.md` | S | Open | `docs/cli-reference.md` | One default string remains. `testParseArgs` asserts it. `rg` over those four docs finds no other default. |
| 112 | Correct current-section drift in `PROJECT_CONTEXT.md` (recovery count 169, spike table 100/0.7, “single-file”) | S | Open | `PROJECT_CONTEXT.md` | Recovery line expects 274. The current spike table uses length 10 and intensity 0.3. Opening paragraph does not say the program is only `main.go`. Session logs stay untouched. |
| 113 | Remove README self-contradictions (grid “not the default yet”, Go 1.20 badge, “no PSF deconvolution / no drizzle”) | S | Open | `README.md` | The calibration section does not say grid is not the default. The badge matches 1.27.4. The comparison cell does not deny shipped `--deconvolve` / `--drizzle-upsample`. |
| 114 | Replace the troubleshooting 3×3-only blind paragraph and the TECHNICAL_SPEC “no native RAW” exclusion | S | Open | `docs/troubleshooting.md` | Troubleshooting names `native` / `astrometry-local` / `astap` / `astrometry-web`. `TECHNICAL_SPEC.md` §2 Excluded does not deny CR2 native decode. |
| 115 | Fix `scaleConsistentMatches` quartile index and `matches[0]` anchor | S | Open | `research/02-astrometry-and-identification.md` | New test: outlier at index 0 is dropped and the other three remain. `scales[len/2]` (or an explicit median helper) is what the gate uses. Existing `TestScaleConsistentMatches` still passes. |
| 116 | Redact diagnostics process command lines | S | Open | `research/05-toolchain-and-modularity.md` | A fake-platform or fixture cmdline containing `api_key=secret` and `--mast-token hunter2` produces JSON/text with neither secret. A normal `skystacker --selftest` cmdline is still present. |
| 117 | Fix Darwin swap field and stop labelling Linux lifetime CPU share as current CPU | S | Open | `research/05-toolchain-and-modularity.md` | `Swapused` is not stored in `SwapTotalMB`. `Text()` does not print that ratio as an unqualified CPU percent; a test locks the label or the formula. |
| 118 | Give archive HTTP calls a context and a redirect policy that cannot forward `Authorization` | S | Open | `research/02-astrometry-and-identification.md` | `httptest` redirect to a second host receives no `Authorization`. A cancelled context returns before the handler is hit. Existing `TestArchiveAdapterQuery` still sees `Bearer secret` on the first hop. |
| 119 | Refuse survey cutouts over the documented 50 Mpix limit before dialing | S | Open | `research/02-astrometry-and-identification.md` | `width*height > 50_000_000` returns an error and the test server receives zero requests. A 64×64 request still works. |
| 120 | Cap `ljpeg.Decode` pixel count and CR2 decode allocation | S | Open | `research/07-native-cr2-decoding.md` | SOF3 with `width*height` above the same order as `goTiff` `maxPixels` returns an error and does not allocate the planes. `TestMinimalStream` and `TestRestartMarkers` still pass. With `PURERAW_CR2` set, `TestDecodeOracle` still passes. |
| 121 | Cap SBI1 tile and quad counts before allocation | S | Open | `research/02-astrometry-and-identification.md` | A header with `tileCount` or `quadCount` above a documented bound returns an error. A round-trip of the existing small fixture still passes, and the test compares quad star RA/Dec, not only band id. |
| 122 | Use circular mean (or `normalizedAngle`) in `selectDensityUniformStars` | S | Open | `research/02-astrometry-and-identification.md` | Stars at RA `0.01` and `6.27` rad in one tile are not split across a centre near π. A non-wrapping tile still returns a subset under the quota. |
| 123 | Run the embedded suite from `go test` without splitting `main.go` | S | Open | `research/01-architecture-and-core-engine.md` | `go test -run TestEmbeddedSelftest -count=1` calls `runSelfTests` and fails if the failure count is non-zero. This does not delete or rewrite the 274 cases (that remains P4-1). |
| 124 | Read CR2 black level, white level, and CFA from the container instead of EOS 40D constants | M | Open | `research/07-native-cr2-decoding.md` | A synthetic CR2/TIFF header with a non-1018 black tag and a non-RGGB CFA yields those values on `Image`. The 40D oracle, when `PURERAW_CR2` is set, still reports 14-bit and the current geometry. |
| 125 | Delete the `fitsBlackLevelFor` side channel | M | Open | `research/04-calibration-and-self-flat.md` | `fitsBlackLevelFor` is gone from the function map. Two FITS files with different `BLACK_LVL` cards return their own levels from a pure function. A test decodes both without relying on call order. |
| 126 | Prove EXIF GPS is not used as ICRS RA/Dec | M | Open | `research/02-astrometry-and-identification.md` | A JPEG whose only coordinates are GPS (40° N, 3° W) does not call the solver with RA=3°, Dec=40°. Either the seed is rejected with a log line containing `geographic`, or a tested alt/az conversion is locked to a fixture. `testEXIFGPSSeed` is extended or replaced; it must not only check that the GPS IFD parses. |
| 127 | One offline switch, and no nova upload unless the web backend was selected explicitly | M | Open | `research/02-astrometry-and-identification.md` | With `AN_API_KEY` set and no local solver, `--plate-solver auto` makes zero POSTs to the nova test server. `--plate-solver astrometry-web` still uploads. `--offline` (name flexible) plus `--survey-cutout` / `--archive-adapters` / `--cone-catalog` fails before dial. `--gaia-offline` behaviour stays. |
| 128 | Replace quadratic sky-map label placement with a spatial hash | M | Open | `research/02-astrometry-and-identification.md` | A fixed 1 000-source fixture keeps the same label strings. No two emitted boxes overlap. The overlap check is against hash neighbours, not the full placed slice. `testLayeredSVGAnnotation` still passes. |
| 129 | Surface cone-search failures in the report instead of returning nil | M | Open | `research/02-astrometry-and-identification.md` | A mock cone endpoint that returns HTTP 500 sets an error field on the processing report and does not omit the section. A complete VOTable still sets `cone_truncated=false`. |
| 130 | Fuzz `ljpeg.Decode` and CR2 `findLosslessJPEG` the way `goTiff` already fuzzes | M | Open | `research/07-native-cr2-decoding.md` | `go test -fuzz=FuzzLJPEG -fuzztime=10s` in `goPureRaw/internal/ljpeg` exits 0. Seeds include `minimalStream` and `minimalStreamWithRestart`. A second fuzz target covers `findLosslessJPEG` on a short buffer with a stray `FFC3`. |

Items deliberately not added: 106, 109, P2-1..P2-5, P3-5, P4-1, P4-3, P4-4, parallel registration (already deferred in `PROJECT_CONTEXT.md`), FITS Rice (item 9), CR2-vs-dcraw geometry (item 10), higher-order SIP (item 45), mosaic (item 27), native all-sky generation (item 73), and anything GPU, SIMD, or ONNX.

## 9. Prioritised top-10 action list

Do **111’s neighbour in code, item 115, first** — fix `scaleConsistentMatches` — before any other blind-solver work and before the doc pass.

Why this and not the docs: items 78–81 are already executed and not accepted, at 0/14, and the write-up says verification never reaches the inlier floor because collisions dominate. The shipped gate does not do what its comment says, and the test cannot see the bug. Another index or log-odds pass on top of a verifier that trusts `matches[0]` will not flip the corpus. The change is local, stdlib-only, and has a one-function regression test. It does not need `main.go`.

Then, in order:

2. **111** — read `parseArgs` / `printUsage` and make `--method` one value everywhere. If the reference is the only outlier, it is a 20-minute doc fix; if `parseArgs` really defaults to `mean` while the spec says `sigma`, it is a user-visible behaviour bug and jumps ahead of everything except 115.
3. **120 and 121 together** — pixel and index caps. Same class of bug `goTiff` already fixed. Do them before any catalog-download feature is pointed at untrusted hosts.
4. **116** — cmdline redaction. Small, and it closes a real secret leak in a flag the project tells people to run (`--diagnose-integrated`).
5. **127** — stop auto-uploading to nova when the key is merely in the environment. The integration doc currently recommends that combination.
6. **112, 113, 114** — one doc commit. Operators are being told three different stories about blind solve and two about the self-flat default.
7. **123** — `TestEmbeddedSelftest` so `go test` and `--selftest` cannot diverge. Do not start the L-sized P4-1 deletion.
8. **124** — CR2 black/CFA from metadata. Required before item 10’s oracle can mean anything on a second camera body.
9. **125** — remove `fitsBlackLevelFor`. The follow-up was written down in November and the symbol is still in the map.
10. **126** — GPS seed. If H1 is true, every phone JPEG “solve” is aimed at the wrong sky. Confirm or kill it with one fixture before building more EXIF seed features.

After those, the next *scientific* session is still the blind corpus — but only with 115 landed, `SKYSTACKER_DEBUG_MATCH` logs saved, and the seeded M42 control from `docs/blind-corpus-results.md` treated as a gate. Do not mark 78–81 accepted on engineering diffs alone.

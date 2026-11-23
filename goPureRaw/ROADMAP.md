# Roadmap

The project is structured in phases. Each phase produces something
usable and testable, even if the overall decoder is incomplete.

## Phase 0 — Foundation *(current)*

- [x] Define `raw.Decoder` interface and `raw.Image` type
- [x] Implement global decoder registry + format detection
- [x] Implement MSB-first bit reader
- [x] Stub decoders for RAF / ARW / CR3 with `Probe` implementations
- [x] `examples/dump-metadata` exercising detection
- [x] CI: build with `CGO_ENABLED=0`, `go vet`, `go test -race` (`.github/workflows/ci.yml`)
- [x] Synthetic unit tests for the MSB-first bit reader, registry and all three `Probe` implementations
- [ ] Collect a small, licensed test corpus (see `testdata/README.md`)
- [ ] Optional `golangci-lint` job once the linter config is agreed

## Phase 1b — Canon CR2 (new, in progress)

- [x] CR2 `Probe` and CR2 format constant
- [x] Pure-Go lossless JPEG decoder (`internal/ljpeg`, SOF3, Huffman, predictors 1–7)
- [x] SOF3/CR2 allocation guards (200 Mpix per plane, 400 M samples, 200 Mpix Bayer raster, 512 MiB stream tail)
- [x] CR2 sensor decode: locate SOF3 stream, reconstruct two sample planes into a Bayer mosaic
- [x] Oracle smoke test with `PURERAW_CR2` (dimensions, bit depth, plausible mean)
- [ ] Bit-exact oracle comparison against `dcraw -D -4` including orientation/crop (2026-11-17: mean matches 1181.0 vs 1181.8, but downsampled cross-correlation across dihedral+crop variants <0.1 — mapping still unresolved)
- [ ] Extract black/white level and CFA from Canon maker notes instead of EOS 40D defaults

## Phase 1 — Metadata & Containers *(1–3 months)*

- [ ] Integrate a pure-Go EXIF/XMP parser (evaluate
      `rwcarlsen/goexif`, `dsoprea/go-exif`)
- [x] Minimal pure-Go TIFF IFD reader (`internal/tiff`, IFD0 + ExifIFD, LE/BE)
- [x] `internal/metadata` maps standard Exif tags to `raw.Metadata`; `arw.ReadMetadata` wired
- [ ] Populate `raw.Metadata` for RAF (needs TIFF block offset from the RAF header) and CR3 (ISOBMFF boxes)
- [ ] Identify and extract maker-note blocks needed for decoding
      (black level, white balance, CFA pattern)
- [x] Synthetic TIFF/Exif round-trip unit tests (no external corpus)
- [ ] Oracle tests: metadata matches `exiftool` output on a licensed corpus

## Phase 2 — Fujifilm RAF decoder *(4–6 months)*

- [ ] Implement uncompressed RAF path end-to-end
- [ ] Implement lossless compressed RAF (adaptive Rice coding)
- [ ] Implement lossy compressed RAF
- [ ] Validate sensor data bit-for-bit against `rawpy` / `dcraw`
- [ ] Benchmarks and per-stripe parallelism

**Why RAF first:** compression algorithm is the most thoroughly
documented (see `docs/formats/raf.md`). Container is custom but
simple; TIFF IFD reuses Phase 1 work.

## Phase 3 — Second format: ARW *or* CR3 *(7–9 months)*

- [ ] **ARW**: parse ARW6/LLVC track, implement `green_lo`,
      `green_hi`, `chroma_r`, `chroma_b` decompression, wavelet
      reconstruction
- [ ] **CR3**: parse ISOBMFF boxes, locate CRX Bayer track, implement
      CRX decompressor
- [ ] Same bit-exact validation methodology as RAF

## Phase 4 — Third format *(10–12 months)*

- [ ] The remaining of ARW / CR3
- [ ] Cross-format conformance suite

## Backlog / Ideas

- `golangci-lint` configuration and CI job
- LSB-first bit reader variant for ARW
- SIMD-friendly inner loops (via pure-Go array batching, not cgo)
- Streaming decode API for very large files
- Pluggable `raw.Image` post-processing hooks
- Optional `wasm` target for browser use

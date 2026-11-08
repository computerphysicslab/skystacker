# SkyStacker — Technical Specification

**Document:** Technical Design Specification  
**Target release:** 0.1 (MVP)  
**Project:** SkyStacker  
**Implementation language:** Go  
**Status:** Proposed specification; implementation decisions may be refined during prototyping.

## 1. Purpose

SkyStacker 0.1 is a local command-line program that discovers JPEG images in a directory and creates two integrated images:

1. A pixel-wise sum of all accepted input frames.
2. A pixel-wise arithmetic mean of all accepted input frames.

The initial target use case is a sequence of similar astronomical exposures captured with a tracking equatorial mount. Version 0.1 does **not** perform star registration, calibration or rejection of poor frames. It combines images in their existing pixel coordinate system and must communicate this limitation clearly.

## 2. Scope

### 2.1 Included

- CLI executable named `skystacker`.
- Input directory argument, defaulting to the current working directory.
- Automatic discovery of `.jpg` and `.jpeg` files, case-insensitive.
- Non-recursive discovery by default.
- Image decoding and validation.
- Support for a consistent RGB image layout in the initial implementation.
- Sum and arithmetic-mean accumulation.
- Floating-point or otherwise overflow-safe intermediate representation.
- Concurrent processing with bounded memory use.
- Output directory creation.
- Two primary output images and a processing report.
- Clear errors and non-zero exit status on fatal failures.
- Unit tests, integration tests and benchmarks.

### 2.2 Excluded

- Star detection and registration.
- Sub-pixel geometric alignment.
- Dark, flat or bias calibration.
- RAW, FITS, PNG or TIFF input in the first implementation unless added as a low-risk extension.
- Median, sigma clipping, winsorisation or weighted stacking.
- Drizzle, deconvolution, gradient removal or colour calibration.
- GUI and cloud processing.

## 3. User stories

- As a user, I can run `skystacker` in a folder of JPG exposures without configuring a processing pipeline.
- As a user, I receive a results folder containing a sum image and a mean image.
- As a user, I can see which files were included and which were skipped.
- As a user, I receive a clear error if no supported images are found.
- As a user, I can limit worker concurrency when memory or CPU resources are constrained.
- As a developer, I can test the numerical core independently of JPEG decoding and TIFF encoding.

## 4. CLI contract

Proposed interface:

```text
skystacker [flags] [input-directory]

Arguments:
  input-directory       Directory containing JPG/JPEG files.
                        Defaults to the current working directory.

Flags:
  --output string       Output directory. Default: <input>/skystacker_results
  --workers int         Maximum concurrent workers. Default: runtime.NumCPU()
  --recursive           Search subdirectories. Default: false
  --overwrite           Permit replacing files in the output directory.
  --version             Print version and exit.
  --help                Print help.
```

Only implement flags that are supported and tested. `--recursive` and `--overwrite` may be deferred if they complicate the first release.

Example:

```bash
skystacker
skystacker ./night-session
skystacker --workers 4 --output ./results ./night-session
```

The program must avoid processing its own output directory if it is inside the input directory.

## 5. Input discovery and validation

### 5.1 Discovery

- Match `.jpg` and `.jpeg` extensions case-insensitively.
- Ignore directories and non-regular files.
- Sort discovered paths deterministically.
- Exclude the configured output directory and its descendants.
- Do not silently recurse unless requested.
- If no candidate files are found, return a clear error.

### 5.2 Decode and validate

For each candidate:

- Decode the image and report decode errors.
- Read width, height and colour model.
- Require identical dimensions for the MVP.
- Convert samples to a documented internal channel representation.
- Define explicit handling for grayscale, palette and alpha-bearing images. A conservative first release may accept RGB JPEGs only and skip unsupported layouts with a report entry.
- Do not assume every decoded Go `color.Color` value is linear-light RGB. Define and test the conversion path.

### 5.3 Failure policy

A corrupt or incompatible file should not crash the process. The default policy should be:

- Record the filename and reason.
- Exclude the file from the stack.
- Continue if at least one valid image remains.
- Fail if fewer than one valid image remains.
- Report accepted and rejected counts.

A strict mode that fails on any rejected input can be considered later.

## 6. Numeric model

### 6.1 Internal samples

Use a representation that cannot overflow for the expected frame count. `float32` is a reasonable initial choice for accumulators, but `float64` may be useful for reference tests and numerical validation.

A possible internal pixel representation is three `float32` values (R, G, B). The implementation should benchmark this against planar channel buffers and other layouts.

### 6.2 Accumulation

For \(N\) accepted frames and channel sample \(I_i(x,y,c)\):

\[
S(x,y,c)=\sum_{i=1}^{N} I_i(x,y,c)
\]

\[
M(x,y,c)=\frac{S(x,y,c)}{N}
\]

The sum and mean must be computed from the same accepted frame set. The denominator is the number of accepted frames, not the number discovered.

### 6.3 Colour-space caveat

JPEG samples are normally gamma-encoded (often sRGB-like), not linear measurements of photon flux. Averaging encoded samples is not physically equivalent to integrating linear-light sensor data. For the MVP, document the exact behaviour and preserve a predictable output. A later scientifically oriented pipeline should linearise samples using an appropriate transfer function before integration and should prefer RAW/FITS inputs where possible.

Do not claim that the MVP recovers calibrated linear astronomical signal from camera-rendered JPG files.

### 6.4 Output scaling and clipping

The sum can exceed the maximum value of the input sample range. Never cast a raw sum directly to 8-bit output.

The output encoding policy must be explicit:

- Prefer a TIFF representation that preserves sufficient precision and range (for example, floating-point samples if supported by the selected encoder).
- If a chosen TIFF encoder only supports integer output, define a deterministic scaling policy and record the scale in the report.
- The mean can be represented in a normalised range, but its bit depth and transfer characteristics must still be documented.
- Generate a display preview separately, applying an explicit stretch or normalisation. Do not confuse the preview with the scientific output.

The TIFF encoder/library must be selected and validated with round-trip tests before the MVP is considered complete. If suitable TIFF support is not available in a pure-Go dependency, document the trade-off rather than silently reducing precision.

## 7. Processing architecture

Proposed logical pipeline:

```text
CLI
 |
 +-- Discover input files
 |
 +-- Decode and validate
 |
 +-- Build processing plan
 |
 +-- Accumulate image data
 |     |
 |     +-- Tile scheduler
 |     +-- Worker pool
 |     +-- Per-tile sum buffers
 |
 +-- Derive mean from sum
 |
 +-- Encode sum and mean
 |
 +-- Write report
```

Suggested package layout:

```text
skystacker/
├── cmd/
│   └── skystacker/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── run.go
│   ├── cli/
│   │   └── flags.go
│   ├── discovery/
│   │   └── discover.go
│   ├── imaging/
│   │   ├── decode.go
│   │   ├── types.go
│   │   └── convert.go
│   ├── stack/
│   │   ├── accumulator.go
│   │   ├── mean.go
│   │   └── tiles.go
│   ├── output/
│   │   ├── tiff.go
│   │   ├── preview.go
│   │   └── report.go
│   └── report/
│       └── types.go
├── testdata/
├── go.mod
├── README.md
└── TECHNICAL_SPEC.md
```

This is a suggested layout, not a requirement to create a package for every file. Keep the first implementation small and avoid abstractions without a concrete use.

## 8. Concurrency and memory strategy

### 8.1 Memory constraint

A 6000 × 4000 RGB frame contains 24 million pixels. At three `float32` values per pixel, one decoded frame requires approximately 288 MB. One hundred such frames would require approximately 28.8 GB if all were expanded to float32 simultaneously.

The implementation must therefore use bounded memory.

### 8.2 Recommended first strategy

Prototype a streaming frame strategy:

1. Discover and validate metadata for all inputs.
2. Allocate the required output accumulator.
3. Decode one input frame at a time.
4. Convert its pixels into the chosen numeric representation.
5. Add its samples to the accumulator.
6. Release the decoded frame before moving to the next input.
7. Derive the mean from the final sum.

This is simple and memory-conscious, though it may be limited by sequential decoding.

### 8.3 Parallel strategy

For the MVP, benchmark two designs:

- **Parallel tiles:** workers own disjoint output regions. This can work well if the decoder and source data provide efficient access to regions.
- **Parallel frames with private accumulators:** workers process separate frames and combine partial accumulators. This increases memory consumption and requires careful bounds.

A practical first release may decode frames sequentially and parallelise conversion/accumulation by rows or tiles where this provides measurable benefit. Do not add goroutine complexity without benchmarks.

Requirements:

- Bound the number of active workers.
- Avoid data races on accumulator memory.
- Use cancellation/error propagation.
- Do not spawn one goroutine per pixel.
- Keep memory usage predictable.
- Benchmark worker counts rather than assuming `NumCPU()` is always optimal.

## 9. Image registration boundary

Registration is intentionally outside version 0.1, but the architecture should leave a clear extension point before accumulation:

```text
Decoded frame
     |
     v
[Future: detect stars and estimate transform]
     |
     v
[Future: resample/align frame]
     |
     v
Accumulator
```

The future registration subsystem should be able to provide a transformed frame or a sampling function without coupling star detection to the accumulator.

A later registration design should consider:

- Star detection and centroid estimation.
- Reference-frame selection.
- Star correspondence and outlier-resistant transform fitting (for example, RANSAC).
- Translation, rotation and scale; possibly affine or distortion models.
- Sub-pixel resampling and interpolation choice.
- Valid-pixel masks and safe common-area cropping.

## 10. Output contract

Default output directory:

```text
<input-directory>/skystacker_results/
```

Expected files:

```text
skystacker_results/
├── stacked_sum.tiff
├── stacked_mean.tiff
├── stacked_preview.jpg
└── report.txt
```

The report should include:

- SkyStacker version.
- Processing timestamp.
- Input directory.
- Discovered, accepted and rejected image counts.
- Accepted filenames.
- Rejected filenames and reasons.
- Image width, height and channel layout.
- Algorithm used.
- Numeric accumulator type.
- Output sample format, bit depth and scaling policy.
- Worker count.
- Processing duration.
- Explicit note: no star registration or calibration was performed in version 0.1.

Output creation should be safe. Do not overwrite existing outputs without explicit permission or a clearly documented default policy. Write temporary files and rename them into place where practical, so interrupted processing does not leave apparently complete output files.

## 11. Error handling and exit status

Suggested exit behaviour:

- `0`: processing completed and outputs written.
- Non-zero: invalid arguments, no inputs, no valid frames, output creation failure or unrecoverable processing/encoding error.

Errors should identify the relevant path and operation. Avoid swallowing errors. A rejected individual input is a reportable non-fatal error under the default policy.

## 12. Dependencies and implementation choices

Prefer the Go standard library for:

- CLI support where practical.
- Filesystem traversal.
- JPEG decoding.
- Basic image types.
- Concurrency primitives.
- Logging and reporting.

Evaluate external dependencies for:

- TIFF encoding with the required sample format and precision.
- More efficient image-buffer operations, if benchmarks justify them.
- Future star detection and registration.

Potential projects to investigate include:

- `golang.org/x/image` — additional Go image formats and utilities.
- `gocv.io/x/gocv` — OpenCV bindings for future computer-vision work.
- `github.com/anthonynsimon/bild` — image-processing operations.
- `github.com/go-images/images` — image-processing utilities.

These are candidates for evaluation, not mandatory dependencies. Verify maintenance status, supported formats, platform constraints and license compatibility before adoption.

## 13. Testing strategy

### 13.1 Unit tests

Create tiny synthetic RGB images with known pixel values.

Example: two 1 × 1 images:

```text
Image A: RGB(10, 20, 30)
Image B: RGB(30, 40, 50)

Expected sum:  RGB(40, 60, 80)
Expected mean: RGB(20, 30, 40)
```

Also test:

- One-frame input.
- Multiple frames.
- Independent RGB channel calculations.
- Values whose sum exceeds 255.
- Large frame counts in numeric tests.
- Dimension mismatch.
- Empty directory.
- Corrupt JPEG.
- Case-insensitive extensions.
- Output-directory exclusion.
- Deterministic ordering.
- Correct denominator after rejected inputs.

### 13.2 Integration tests

Use a small fixture directory and verify:

- Expected output files are created.
- TIFF files can be decoded again.
- Output dimensions and sample values match expected results within documented tolerances.
- The report accurately lists inputs and rejections.
- Existing outputs are handled according to the overwrite policy.

### 13.3 Benchmarks

Add Go benchmarks for:

- Pixel conversion.
- Accumulation of one frame.
- Accumulation of a complete stack.
- Tile sizes and worker counts.
- Output encoding.

Record CPU, RAM, Go version, image dimensions, frame count and output representation for benchmark reports.

## 14. Acceptance criteria for version 0.1

Version 0.1 is acceptable when all of the following are true:

1. Running `skystacker` in a directory discovers supported JPG/JPEG files without requiring a list of filenames.
2. The application creates the results directory.
3. It creates a sum image and an arithmetic-mean image.
4. Accumulation does not overflow 8-bit values.
5. The output representation and any scaling are documented.
6. Input images with incompatible dimensions are reported and handled according to the defined policy.
7. Corrupt files do not crash the whole process.
8. The report identifies accepted and rejected files.
9. Automated tests verify known numerical results.
10. Memory use is bounded and measured on a representative high-resolution sequence.
11. The application clearly states that no registration or calibration was performed.
12. The project builds and runs on the initial target platform (Linux); cross-platform builds should be added as part of release validation.

## 15. Open technical decisions

Resolve these through small prototypes before declaring the implementation stable:

- Which TIFF library supports the desired floating-point or high-bit-depth output reliably?
- Should the primary scientific output use floating-point TIFF, integer TIFF with documented scaling, or another format?
- Should the first implementation preserve JPEG-encoded values or convert them to linear light before accumulation?
- Which internal memory layout gives the best balance of speed, clarity and memory use?
- Is sequential frame decoding plus parallel tile processing faster than a bounded parallel-frame design?
- What should the default behaviour be when one or more files are rejected?
- Should grayscale JPEGs be converted to RGB or handled as a separate one-channel mode?

## 16. Future roadmap

### 0.2 — Star registration

- Star detection and centroid calculation.
- Reference image selection.
- Star matching.
- Robust geometric transform estimation.
- Sub-pixel resampling.
- Common valid-area cropping.

### 0.3 — Robust stacking

- Median.
- Sigma clipping.
- Weighted mean.
- Frame-quality scoring and optional rejection.

### 0.4 — Astrophotography formats and calibration

- RAW and FITS input.
- Dark, flat and bias calibration.
- Background correction.
- Drizzle and advanced integration options.

---

**End of specification.**

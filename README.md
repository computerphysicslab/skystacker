# SkyStacker

**Open-source astrophotography image stacker written in Go.**

> Drop a folder of astrophotography images. Get a better image.

SkyStacker is a planned, local-first command-line application for combining a sequence of astronomical photographs into integrated images. Its initial target is a simple workflow: run the program in a directory containing JPG exposures and let it create a results directory with summed and averaged images.

The project name **SkyStacker** is provisional. It is the working name for development and can be changed later.

## Project status

**Planning / MVP definition — version 0.1**

The first milestone is deliberately limited. It should establish a reliable image-processing pipeline, validate the numerical core and provide a convenient command-line experience. Automatic star detection and registration are planned for a later milestone; the first MVP should not silently imply that unaligned frames have been registered.

## Goals

- Process a directory of astronomical images with minimal user configuration.
- Work locally and offline; no account, cloud service or network connection is required.
- Use Go as the primary implementation language.
- Use concurrency where it improves throughput without making memory usage unpredictable.
- Preserve numerical precision during accumulation.
- Produce reproducible outputs and a useful processing report.
- Keep the architecture modular so that registration, calibration and more advanced stacking algorithms can be added later.
- Prefer a small number of dependencies and Go-native implementations where practical.

## Non-goals for version 0.1

The initial MVP is not intended to replace a complete astrophotography suite such as Siril. In particular, version 0.1 does not need to implement:

- Automatic star detection and sub-pixel image registration.
- Dark, flat and bias calibration.
- RAW camera decoding or FITS input.
- Sigma clipping, weighted integration or drizzle.
- Gradient removal, background extraction or colour calibration.
- A graphical user interface.

These are possible future capabilities, not requirements for the first release.

## Existing software and related projects

The project should learn from established astronomy software rather than duplicate an entire processing suite.

- **Siril** — a mature open-source astrophotography processing application with registration, calibration, stacking and scripting. Website: <https://siril.org/> · Source: <https://github.com/Astro-Siril/Siril>
- **DeepSkyStacker** — a widely used stacking application focused on deep-sky astrophotography. Project: <https://github.com/outback-deepskystacker>
- **Sequator** — software oriented particularly towards night-sky landscape photography. Website: <https://sites.google.com/site/sequatorglobal/>
- **ASTAP** — astronomy software including star detection, plate solving and image-processing tools. Website: <https://www.hnsky.org/astap.htm>
- **OpenSkyStacker** — another open-source stacking project worth reviewing: <https://github.com/ostap-tech/OpenSkyStacker>

There are also Go-based astronomy/image-processing projects. Before implementation, inspect their current status, licenses, architecture and overlap with this project. Their existence does not prevent SkyStacker from pursuing a distinct, small, Go-native and local-first workflow.

## Why Go?

Go is a good fit for a portable CLI and for the data-parallel parts of image processing:

- Goroutines and channels make it straightforward to distribute independent image tiles or rows across workers.
- A single compiled executable is convenient to distribute.
- Cross-compilation supports Linux, Windows and macOS.
- The standard library includes basic image decoding and encoding facilities.
- The language encourages relatively simple deployment and maintainable concurrency.

The computationally intensive work is not automatically faster just because it is written in Go. Performance must be measured. The first implementation should favour a correct, testable numerical core before introducing SIMD, assembly, cgo or OpenCV.

## Core processing concepts

### Sum

For each pixel position and colour channel, sum the corresponding sample from every accepted frame:

\[
S(x,y,c)=\sum_{i=1}^{N} I_i(x,y,c)
\]

A sum is useful for inspection and for understanding accumulated intensity, but its numerical range grows with the number of frames. It must not be accumulated in an 8-bit integer.

### Arithmetic mean

The arithmetic mean is:

\[
M(x,y,c)=\frac{1}{N}\sum_{i=1}^{N} I_i(x,y,c)
\]

For independent random noise, stacking \(N\) equally exposed, aligned frames can improve signal-to-noise ratio approximately in proportion to \(\sqrt{N}\). With 100 ideal frames, that is a theoretical factor of about 10 compared with one frame. Real results depend on tracking, sky conditions, sensor noise, exposure consistency, calibration and image alignment.

### Median and outlier rejection

Median stacking and sigma clipping can reject transient outliers such as satellite trails, cosmic-ray hits and occasional hot pixels. They are valuable future features, but are outside the first MVP.

### Alignment is essential for astronomical stacking

A tracking mount reduces apparent motion, but does not guarantee that all frames have identical pixel coordinates. Small translation, rotation, scale changes, flexure or tracking errors can blur stars if frames are combined without registration.

Version 0.1 should therefore be explicit: it combines images in their existing pixel coordinate system. Automatic star registration is a planned later milestone. A useful first test is to use frames that have already been aligned, or a synthetic set with identical geometry.

## Input and output

### Initial input

- JPEG/JPG images in the working directory, non-recursively by default.
- All accepted images must have compatible dimensions and colour layout for the first release.
- The program should report unsupported, corrupt or incompatible files rather than failing silently.
- File discovery should be deterministic (for example, lexicographic filename order).

JPEG is a practical starting point because it is common in camera workflows, but it is a lossy, usually 8-bit format with a camera-rendered tone curve. It is not an ideal scientific acquisition format. A future release should consider camera RAW and FITS support.

### Initial output

Create a results directory, for example `skystacker_results/`, containing:

- `stacked_sum.tiff` — accumulated pixel values, exported with an explicitly documented scaling/encoding policy.
- `stacked_mean.tiff` — arithmetic mean, preserving the mean image values as far as the selected output format allows.
- `stacked_preview.jpg` — optional preview suitable for quick viewing.
- `report.txt` — input count, accepted/rejected files, image dimensions, processing time and method details.

The TIFF output implementation must make bit depth and sample representation explicit. Do not simply cast a floating-point sum to 8-bit. If a conventional TIFF encoder cannot preserve the required floating-point or high-bit-depth representation, select and document a suitable library or use a scientifically appropriate intermediate format in a later release.

## Intended command-line experience

The default workflow should require no arguments:

```bash
# Process supported images in the current directory
skystacker
```

Possible advanced options:

```bash
# Process a specific directory
skystacker ./session/

# Choose a processing method (future option)
skystacker --method mean

# Limit worker count
skystacker --workers 8

# Preserve intermediate products (future option)
skystacker --save-intermediate
```

The exact flags are to be finalised during implementation. The MVP should keep the default path simple and should not expose options that are not implemented.

## Proposed processing pipeline

1. Discover candidate image files.
2. Decode images and validate dimensions, colour model and readability.
3. Record image metadata and input ordering.
4. Establish a documented policy for incompatible or corrupt images.
5. Process image data using a precision-safe accumulator.
6. Produce the sum and arithmetic mean.
7. Encode outputs without unintended clipping or wraparound.
8. Write a report describing what was processed.

A future registered pipeline can add quality analysis, star detection, reference-frame selection, geometric registration and resampling before accumulation.

## Performance and memory

A 6000 × 4000 RGB image contains 24 million pixels. One such image occupies approximately 72 MB as three `float32` channels, before auxiliary buffers. Keeping 100 decoded RGB images in float32 would require roughly 7.2 GB, so the implementation should not assume that all frames fit in memory.

A tile-based or streaming design is preferable. For each output tile, decode or access the corresponding source region, accumulate it, and release temporary buffers. The best strategy depends on the chosen decoder and image representation. Benchmark memory and throughput on representative camera files.

Parallelise independent output tiles or regions. Avoid concurrent writes to the same accumulator region unless ownership or synchronisation is explicit.

## Quality and validation

A credible MVP needs automated tests, not just visual inspection:

- Unit tests for sum and mean using tiny known images.
- Tests for RGB channel independence.
- Tests for overflow prevention and values above 255 in the sum accumulator.
- Tests for mismatched dimensions and corrupt files.
- Tests for deterministic file discovery and reporting.
- Integration tests that verify the expected output files.
- Benchmarks for representative image dimensions and frame counts.
- Visual comparison against a trusted reference implementation on a small, controlled dataset.

The first MVP should be benchmarked on identical input data against a trusted workflow such as Siril, while recognising that comparisons are only meaningful when alignment, scaling, colour handling and rejection settings are equivalent.

## Suggested milestones

### 0.1 — Basic stacking MVP

- Automatic JPG/JPEG discovery in the selected directory.
- Input validation and clear error reporting.
- Sum and arithmetic mean.
- Floating-point accumulation.
- Concurrent, memory-conscious processing.
- TIFF outputs with documented numeric representation, plus a convenient preview.
- Processing report and automated tests.

### 0.2 — Registration

- Star detection.
- Reference-frame selection.
- Star matching and robust geometric transform estimation.
- Sub-pixel alignment and resampling.
- Safe output cropping.

### 0.3 — Quality-aware stacking

- Frame quality metrics and optional rejection of poor frames.
- Median integration.
- Sigma clipping.
- Weighted average.
- More detailed diagnostics.

### 0.4 — Expanded astrophotography pipeline

- RAW and FITS support.
- Dark, flat and bias calibration.
- Background/gradient correction.
- Drizzle and larger-session optimisation.

## Design principles

1. **Correctness before speed.** Establish numeric and image-format correctness before micro-optimisation.
2. **Explicit behaviour.** Never imply that images have been aligned or calibrated when they have not.
3. **Local-first.** No network access is required for normal operation.
4. **Reproducibility.** Same inputs and options should produce predictable results.
5. **Modularity.** Decoding, validation, accumulation and encoding should be separable.
6. **Measured optimisation.** Benchmark before adopting complex concurrency or native dependencies.

## License

The intended project is open source. The final license has not yet been selected. MIT and Apache-2.0 are reasonable permissive candidates; check dependency license compatibility before publishing a release.

## Contributing

The initial contribution priorities are:

- Review the MVP scope and technical specification.
- Build a small, representative test dataset.
- Implement and test the numerical accumulator.
- Validate image encoding and output bit depth.
- Benchmark memory use and processing speed.

---

**Project:** SkyStacker  
**Working tagline:** Fast. Local. Astronomical.  
**Status:** MVP planning

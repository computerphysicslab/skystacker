# Canon CR2 Format

## Container

CR2 is TIFF-like: `II*\0`, `CR` + version at bytes 4..7, IFD0 at the offset
stored in the header. Standard TIFF/Exif tags are read by `internal/tiff`;
`cr2.ReadMetadata` maps them to `raw.Metadata`.

## Sensor data

Most Canon DSLR CR2 files store the raw sensor image as a **lossless JPEG**
(SOF3) stream somewhere in the file. `findLosslessJPEG` locates an SOI
marker followed by an SOF3 marker, and `internal/ljpeg` decodes it:

- canonical Huffman tables from DHT (DC class, up to 16-bit codes),
- byte-stuffed entropy stream (`FF 00`),
- JPEG lossless predictors 1–7 with initial predictor `2^(P-1)`,
- two sample planes (1972x2622 on EOS 40D) interleaved into a 3944x2622
  Bayer mosaic (RGGB assumed; black/white levels default to EOS 40D values
  1018/16224 until maker-note parsing is implemented).
- shared SOF3 allocation guards: `ljpeg.ValidateGeometry` rejects planes above
  200 Mpix, more than 400 M total samples, precision outside 1..16 or more
  than 4 components; the CR2 adapter also caps the Bayer raster at 200 Mpix
  and the copied stream tail at 512 MiB. All checks run before allocations.

## Validation status

- Oracle smoke test (`PURERAW_CR2=<file>`): EOS 40D CR2 decodes to
  3944x2622 @ 14 bits with mean 1181.8, matching `dcraw -D -4` mean 1181.0.
- Bit-exact per-pixel agreement and the orientation/crop transform are
  **not yet verified**; see `ROADMAP.md` Phase 1b.

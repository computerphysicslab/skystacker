# Sony ARW Format

## Container

ARW is **TIFF** with vendor tags. Use `golang.org/x/image/tiff` for
the container and IFD parsing; extract:
- `CFARepeatPatternDim`, `CFAPattern`
- Black level / white level tags
- White balance (maker note)
- The **Sony Raw Image** tag (offset and length of raw track)

## Compression Variants

| Variant | Codec               | Notes                           |
|---------|---------------------|---------------------------------|
| ARW1/2  | uncompressed/packed | Legacy models                   |
| ARW6    | SMPTE RDD 34 (LLVC) | Modern models (A7, A9, ...)     |

## LLVC (ARW6)

Raw data is split into four sub-streams:

- **green_lo**: low-frequency luma (green)
- **green_hi**: high-frequency detail
- **chroma_r**, **chroma_b**: chroma

Each is decoded with a low-latency wavelet transform and entropy coding
(primarily a Rice-family coder with per-block parameters). After
decoding, channels are recombined into Bayer sensor data.

## References

- `abbradar/arw6_decode` — reverse-engineering write-up
- SMPTE RDD 34 (LLVC) specification
- LibRaw `parse_sony_llvc()`

## Status in pureraw

- [ ] TIFF IFD + Sony Raw Image tag extraction
- [ ] LLVC bitstream header parsing
- [ ] green_lo / green_hi decoders
- [ ] chroma_r / chroma_b decoders
- [ ] Bayer recombination
- [ ] Bit-exact oracle test

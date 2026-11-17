# Fujifilm RAF Format

## File Layout

```
[0x00] "FUJIFILMCCD-RAW " (16 bytes)   magic
[0x10] version (4 bytes)
[0x14] camera model (32 bytes)
...
[?]    JPEG preview (optional)
[?]    TIFF block: EXIF, IFDs, WB, black level, CFA pattern
[?]    Raw data block:
         [0x00] "IS" 16-byte header
                - width (u16)
                - height (u16)
                - bit depth (10/12/14)
                - compression mode (0=uncompressed, 1=lossless, 2=lossy)
                - CFA layout
                - stripe count
         [0x10] stripes...
```

## Stripe Encoding

Each stripe is **768 pixels wide**. Stripes are entropy-coded
independently, so they parallelize perfectly across goroutines.

## Adaptive Rice Coding

Each stripe begins with a **16-bit `IS`-derived header** that selects
the initial Rice parameter `k`. Predictions are formed from previously
decoded neighbors in a fixed scan order, and the prediction error is
encoded as:

- `k` bits of magnitude prefix (or unary, depending on mode)
- remainder bits

The parameter `k` adapts between blocks based on recent error
magnitudes. See the LightCraft `raf-compression.md` for the exact
update rule.

## References

- LightCraft `raf-compression.md` and `raf.md`
- LibRaw `dcraw.c` `parse_fuji_compressed()`
- `lclevy` RAF notes

## Status in pureraw

- [ ] IS header parsing
- [ ] Uncompressed stripes
- [ ] Lossless Rice stripes
- [ ] Lossy stripes
- [ ] Bit-exact oracle test vs. `dcraw -D -4`

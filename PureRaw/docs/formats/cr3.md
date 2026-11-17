# Canon CR3 Format

## Container: ISOBMFF

CR3 is an **ISOBMFF (MP4-style) file** — boxes with `[size:u32] [type:4] [payload]`.
The `ftyp` major brand is `"crx "`. Parse the box tree:

```
ftyp (crx )
moov
  trak (preview) — JPEG/HEVC preview
  trak (raw)     — CRX Bayer track
mfra
uuid (Canon-specific: CTMD maker notes)
```

## CRX Codec

The raw sensor data is stored as a **CRX** track. Decompression involves:

1. **Locating the `CMP1` sample description** in the raw track — it
   carries the CRX parameters (tile geometry, quantization tables).
2. **Reading tile bitstreams**, each entropy-decoded.
3. **Inverse DCT + reconstruction** into CFA samples.
4. **Metadata** from `CTMD` boxes (black level, white balance, CFA).

Several CRX variants exist across camera generations (CRX v1, v2, ...).
Identify the variant from `CMP1` before dispatching.

## References

- `lclevy/canon_cr3` — box-level documentation
- LightCraft `cr3.md` and `crx.md`
- LibRaw `parse_canon_crx()`

## Status in pureraw

- [ ] ISOBMFF box walker
- [ ] `CMP1` variant detection
- [ ] CRX v1 tile decoder
- [ ] CRX v2 tile decoder (if needed)
- [ ] `CTMD` metadata extraction
- [ ] Bit-exact oracle test

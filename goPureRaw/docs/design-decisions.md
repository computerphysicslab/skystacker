# Design Decisions

## Why start with Fujifilm RAF?

1. **Best-documented compression.** The LightCraft `raf-compression.md`
   spec describes the 16-byte `IS` header, 768-pixel stripes, and
   adaptive Rice coding in enough detail to implement without
   guessing.
2. **Simple container.** Custom header + TIFF block + raw block.
3. **No video codecs.** Unlike ARW6 (LLVC) or CR3 (CRX), RAF's
   compression is a self-contained entropy coder — no DCT, no wavelets.
4. **Good test availability.** RAF samples are widely available under
   permissive licenses.

## Why not demosaic in pureraw?

Demosaicing is well-served in Go (`disintegration/imaging`,
`esimov/caire`, etc.) and couples decoder releases to algorithm
experiments. Keeping `raw.Image` as sensor data keeps the decoder
deterministic and easy to oracle-test against `dcraw -D -4`.

## Why a global registry?

Two alternatives were considered:

- **Explicit factory**: `raf.New()` at the call site. More boilerplate,
  forces callers to know the format.
- **Blank-import registry**: consumer imports format packages, calls
  `raw.Decode`. Zero-config detection, familiar Go idiom (cf.
  `image/jpeg`).

We chose the registry. Embedders that need binary-size discipline can
call `raf.Decoder{}.Decode` directly without the registry.

## Why `io.ReaderAt`, not `io.Reader`?

Two-pass decoding is common (pass 1: scan IFDs and locate tracks;
pass 2: decompress). `ReaderAt` makes seeking trivial without buffering
the whole file. Callers with only an `io.Reader` can wrap in
`bytes.NewReader` (memory trade-off) or `os.File` (the common case).

## Testing strategy: the oracle

Bit-exactness is the goal. `dcraw -D -4 -T` (or `rawpy` wrapping it)
produces raw sensor data that we can compare sample-for-sample. This
is stronger than "the output looks right" and catches subtle
bitstream bugs early.

Oracle tests are gated behind `//go:build oracle` so normal CI doesn't
require dcraw/rawpy.

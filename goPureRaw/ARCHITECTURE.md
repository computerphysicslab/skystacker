# Architecture

## Package Layout

```
raw/              Public API: Decoder, Image, Metadata, registry
formats/raf/      Fujifilm RAF decoder
formats/arw/      Sony ARW decoder
formats/cr3/      Canon CR3 decoder
internal/...      Shared implementation details (bit readers, etc.)
examples/         Runnable demos
```

## Design Principles

### 1. One interface, many formats

All format decoders implement `raw.Decoder`:

```go
type Decoder interface {
    Format() Format
    Probe(r io.ReaderAt) bool
    Decode(r io.ReaderAt, size int64) (*Image, error)
}
```

`Probe` must only read the first few KiB and must never panic on
garbage. `Decode` is the only entry point that allocates large buffers.

### 2. Registration via `init()`

Each format package registers itself on import:

```go
func init() { raw.Register(Decoder{}) }
```

Consumers import the formats they need (blank import). This keeps the
core dependency-light and lets embedders trim binary size by omitting
unused formats.

### 3. `raw.Image` is sensor data, not RGB

The decoder's job ends at: **CFA samples at native bit depth + the
geometry and color metadata needed to interpret them**. Demosaicing,
white balance application, and output encoding are deliberately out of
scope. This mirrors LibRaw's `raw_image` stage and keeps the decoder
deterministic and easy to test.

### 4. Errors are typed

`raw.ErrUnsupportedFormat`, `raw.ErrCorrupt`, and
`raw.ErrUnsupportedVariant` let callers fall back to another decoder or
surface a user-friendly message. Format packages wrap these with
`fmt.Errorf("%w: ...", raw.ErrCorrupt)` for context.

### 5. Parallelism is a format concern

Where a format decomposes into independent units (RAF stripes, ARW
tracks, CR3 tiles), the format package is responsible for parallelizing
it. The `raw` core stays single-threaded and simple.

### 6. No `unsafe`

Decoders operate on `[]byte` and `[]uint16`. If a hot loop needs
`binary.LittleEndian.Uint32` optimizations, they're allowed; pointer
casts are not. This is a hard rule — image parsing is a classic
memory-safety attack surface.

## Testing Methodology

1. **Unit tests** for bit readers, Rice decoder, container parsers —
   table-driven, no external data required.
2. **Golden tests** against a small committed corpus of CC0-licensed
   sample files (see `testdata/README.md`).
3. **Oracle tests** in a separate `//go:build oracle` file: shell out to
   `rawpy`/`dcraw`, compare sensor data bit-for-bit. Skipped in normal
   CI; run nightly.
4. **Fuzz tests** (`go test -fuzz`) on every `Probe` and every container
   parser.

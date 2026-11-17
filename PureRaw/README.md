# pureraw

A **pure-Go** decoder for camera RAW files. No cgo, no libraw, no
external binaries — just Go.

`pureraw` aims to cover a focused subset of formats first, rather than
attempting the full LibRaw compatibility matrix out of the gate:

| Format       | Vendor    | Status         |
|--------------|-----------|----------------|
| Fujifilm RAF | Fujifilm  | 🚧 In progress |
| Sony ARW     | Sony      | 📋 Planned     |
| Canon CR2    | Canon     | 🚧 In progress (SOF3 lossless) |
| Canon CR3    | Canon     | 📋 Planned     |

See [ROADMAP.md](ROADMAP.md) for the phased plan and
[docs/prior-art.md](docs/prior-art.md) for why a pure-Go decoder is
worth building even though LibRaw exists.

## Goals

- **Pure Go.** `CGO_ENABLED=0` must always build and pass tests.
- **Memory safe.** No unsafe pointer arithmetic in decoders. A malicious
  or malformed file must not corrupt memory.
- **Streaming-friendly.** Decode from an `io.ReaderAt`; don't require
  the whole file in memory unless the format forces it.
- **Parallel by design.** Exploit `goroutine`s for independent decode
  units (stripes, tiles, tracks).
- **Correct before fast.** Match a reference decoder (LibRaw / `rawpy`)
  bit-for-bit on test corpus before optimizing.

## Non-Goals (for now)

- Every RAW format under the sun. Start narrow.
- A full post-processing pipeline (demosaic, tone curve, output
  encoding). `pureraw` returns *sensor data + metadata*; downstream
  libraries handle the rest.
- TIFF/JPEG previews beyond what's needed for metadata extraction.

## Install

```bash
go get github.com/computerphysicslab/skystacker/PureRaw@latest
```

After the first `PureRaw/vX.Y.Z` tag is published, pin a version instead of
`@latest` for reproducible builds.

## Quick Start

```go
import (
    "os"

    "github.com/computerphysicslab/skystacker/PureRaw/raw"
    _ "github.com/computerphysicslab/skystacker/PureRaw/formats/raf"
    _ "github.com/computerphysicslab/skystacker/PureRaw/formats/arw"
    _ "github.com/computerphysicslab/skystacker/PureRaw/formats/cr2"
    _ "github.com/computerphysicslab/skystacker/PureRaw/formats/cr3"
)

f, _ := os.Open("DSCF0001.RAF")
defer f.Close()
st, _ := f.Stat()

img, err := raw.Decode(f, st.Size())
if err != nil {
    log.Fatal(err)
}
fmt.Printf("%s %s — %dx%d @ %d bits\n",
    img.Metadata.Make, img.Metadata.Model,
    img.Width, img.Height, img.BitDepth)
```

## Status

Pre-alpha. The public interfaces in `raw/` are the contract. RAF/ARW/CR3
currently implement `Probe` only. CR2 implements `Probe`, `ReadMetadata`
and a pure-Go lossless-JPEG (SOF3) sensor decoder; the geometric mapping
against dcraw is still being validated (see `docs/formats/cr2.md`).

## License

MIT. See [LICENSE](LICENSE).

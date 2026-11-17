# Prior Art

## The cgo Wrappers

- **`raw-go`** — statically links LibRaw so consumers don't need a
  system install, but still requires `CGO_ENABLED=1`. Main motivation is
  packaging convenience, not Go-native decode.
- **`golibraw`, `LibRawGo`** — direct wrappers (one via SWIG) around
  the LibRaw C API. Require LibRaw headers/libs at build time.

All of these give up the benefits we care about: easy cross-compilation,
`CGO_ENABLED=0` builds, memory safety, and hackable Go code.

## The Middle Ground

- **`dcraw` compiled to WASM/WASI**, invoked from Go. No cgo, but still
  ships C code; debugging across the WASM boundary is painful and perf
  is dominated by WASM overhead.

## Pure-Go Decoders That Don't Cover RAW

- **`stbimagego`** — pure-Go port of `stb_image`. Handles JPEG/PNG/TGA,
  not camera RAW.
- **`hdr`** — Radiance HDR, not camera RAW.
- **`golang.org/x/image/tiff`** — usable for ARW/RAF container parsing,
  but does not decode the vendor-specific sensor data.

## The Argument for Building

- **`CGO_ENABLED=0` is a distribution superpower.** Static binaries,
  trivial cross-compile, no build toolchain on the target.
- **Memory safety.** Parsing attacker-controlled image files is a
  classic RCE surface. Pure Go eliminates whole classes of bugs.
- **Hackability.** Want to add a new compression mode for a fresh
  camera? Edit Go, run `go test`, ship. No C toolchain, no memory
  leaks to chase.
- **Parallelism.** Per-stripe and per-tile decode maps cleanly to
  goroutines.

## The Counterargument

LibRaw encodes two decades of reverse-engineering across hundreds of
camera models. Any pure-Go effort must accept a much narrower scope.
That's why we **start with a subset** (RAF → ARW/CR3) rather than
chase LibRaw parity.

# Contributing

Thanks for your interest! This project is early — good time to shape it.

## Ground Rules

- **`CGO_ENABLED=0` must build and test cleanly.** No exceptions.
- **No `unsafe` in `formats/`.** Pointer arithmetic in image decoders
  is how CVEs happen.
- **Correctness before performance.** A slow-but-correct decoder beats
  a fast-but-wrong one. Benchmarks come after a passing golden test.
- **Small PRs.** One logical change per PR. Format support arrives in
  multiple PRs (probe → metadata → decompressor → polish).

## Development Workflow

```sh
make fmt        # gofmt
make vet        # go vet
make test       # unit + golden tests
make lint       # golangci-lint
```

## Adding a Format

1. Create `formats/<name>/` with a `decoder.go` that implements
   `raw.Decoder`.
2. Add `Probe` first, with a test that covers valid magic and several
   malformed headers.
3. Add a doc page under `docs/formats/<name>.md` summarizing the file
   structure and citing reverse-engineering sources.
4. Populate `raw.Metadata` in `Decode` before touching sensor data.
5. Only then implement the decompressor, validated against the oracle
   test harness.

## Sample Files

Do **not** commit commercial RAW files. See `testdata/README.md` for
the sourcing policy.

## Commit Messages

Conventional-ish: `raf: implement IS header parser`,
`arw: add LLVC bitstream decoder`, `raw: add LSB-first bit reader`.

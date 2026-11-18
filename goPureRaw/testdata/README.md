# Test Data

Do **not** commit commercial RAW files. We use only files that are:

1. Explicitly released under CC0 / Public Domain, or
2. Synthetically constructed by us (small hand-crafted files that
   exercise edge cases in `Probe` and container parsing).

## Local Testing

For full decoder validation, place a licensed sample corpus under
`testdata/private/` (gitignored). CI never sees it; oracle tests
reference paths via the `PURERAW_TESTDATA` environment variable.

## Requesting Samples

If you need a specific camera's RAW to test, ask on the issue tracker
with the model and format — the maintainers can often source a
permissively licensed sample or build a synthetic one.

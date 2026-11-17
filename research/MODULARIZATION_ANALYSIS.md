# Modularisation analysis: SkyStacker + PureRaw as Go modules (2026-11-17)

## Current situation

- `github.com/computerphysicslab/skystacker` is a Go module (`go 1.26`,
  `toolchain go1.26.11`) whose CLI lives in a single `main.go`.
- `PureRaw/` is a **nested module** with its own `go.mod`, currently declaring
  the placeholder path `github.com/youruser/pureraw`. It is not imported by the
  parent: SkyStacker embeds an equivalent CR2 decoder as `pr*` functions in
  `main.go`.
- Both modules are standard-library-only and build with `CGO_ENABLED=0`.

## Option A — one GitHub repo, several subdirectory Go modules (recommended)

Go supports modules in repository subdirectories. A module at `PureRaw/` must
declare `module github.com/computerphysicslab/skystacker/PureRaw` and be tagged
as `PureRaw/v0.1.0`, `PureRaw/v0.2.0`, etc. Consumers can then:

```bash
go get github.com/computerphysicslab/skystacker/PureRaw@v0.1.0
```

Advantages:
- One CI, one issue tracker, one LICENSE, one release workflow (with per-module
  tag patterns), one place for research/docs.
- SkyStacker and its libraries evolve in lockstep; version skew is easier to
  control during rapid development.
- No extra repo secrets/permissions; contributors find everything in one place.
- Go module zip serving already handles subdirectory modules: consumers do not
  clone the whole repo.

Disadvantages:
- Discoverability is lower than a dedicated repo named `pureraw`.
- Per-module semantic versioning requires subdirectory tags (`PureRaw/vX.Y.Z`)
  and discipline in the release workflow.
- Local development needs `go.work` (or a temporary `replace` that must NOT be
  committed if `go install ...@latest` support is desired — replace directives
  in a dependency module are ignored by the Go proxy).

## Option B — one independent GitHub repo per library

Advantages:
- Clean import path (`github.com/computerphysicslab/pureraw`), maximum
  discoverability, independent license/version/release cadence.
- Each library can attract its own contributors; CI per repo is simpler.
- Smaller clones for library-only users.

Disadvantages:
- N repos → N CI configs, N issue trackers, N release processes, cross-repo
  version bumps and higher coordination cost.
- Local development across repos requires `go.work` or `replace` anyway.
- Higher risk of version skew while the code is changing quickly.

## Decision

Start with **Option A (monorepo + subdirectory modules)**. It matches the
current single-repo workflow, keeps co-evolution cheap, and still makes
`PureRaw` `go get`-able. Revisit Option B only if a library gains an external
contributor base or needs a different release cadence/license.

## Concrete migration plan (ordered by effort)

1. **Fix the module path now** (S, ~15 min): `PureRaw/go.mod` →
   `module github.com/computerphysicslab/skystacker/PureRaw`. Do this before any
   tag, because changing a published module path is a breaking change.
2. **Add `go.work`** at repo root (S, ~10 min) listing `.` and `./PureRaw` for
   local development; add `go.work` to `.gitignore` (or commit it if the team
   prefers; it is ignored by `go install` either way).
3. **Publish v0.1.0** (S, ~20 min): tag `PureRaw/v0.1.0`, update `PureRaw/README.md`
   with `go get` usage, add a CI job that tests the nested module on its own.
4. **Use the library from SkyStacker** (M, ~1–2 h): in SkyStacker `go.mod` add
   `require github.com/computerphysicslab/skystacker/PureRaw v0.1.0` (no
   committed `replace`), blank-import `formats/cr2`, call `raw.Decode`, and
   delete the duplicated `pr*` decoder from `main.go`. Keep `--no-native-cr2`
   fallback until the oracle test passes.
5. **Document the change** (S, ~30 min): README/TECHNICAL_SPEC now say
   "single executable, stdlib-only" rather than "single source file"; CI must
   build from a clean checkout without `go.work`.
6. **Future modules** (M–L): `fitsio/` (FITS read/write + Rice), `photometry/`
   (HFR/PSF helpers) as sibling directories with their own `go.mod` and
   `fitsio/vX.Y.Z` tags, following the same pattern.

## Risks / caveats

- The `internal/` packages inside PureRaw stay private to PureRaw by Go design;
  SkyStacker should consume only exported packages (`raw`, `formats/cr2`).
- `go install github.com/computerphysicslab/skystacker@latest` needs the
  parent `go.mod` to resolve the nested module from the proxy: use a real
  tagged version, not a local `replace`.
- Adding a dependency (even our own module) changes the "zero external
  dependencies" claim. It remains stdlib-only at the code level, but users
  building from source now need module resolution. Document it honestly.
- Do not split until the API of a library is stable: a published module path
  plus semver is a compatibility promise.

## Sources

- Go modules reference, module paths and subdirectory modules: <https://go.dev/ref/mod>.
- Go workspaces: <https://go.dev/doc/workspaces>.
- `go install pkg@version` and replace-directive behaviour: <https://go.dev/ref/mod#go-install>.

### Progress (2026-11-17)

Steps 1 and 2 executed: `PureRaw/go.mod` now declares `github.com/computerphysicslab/skystacker/PureRaw` (all imports updated), and a root `go.work` (gitignored) lists `.` and `./PureRaw`. Both modules build/test cleanly. Step 3 (tag `PureRaw/v0.1.0` + CI job) and step 4 (SkyStacker imports the library, removing duplicated `pr*` code) remain.

### Progress update (2026-11-17, steps 3–4)

Tagged `PureRaw/v0.1.0`, found and fixed a CR2 `Probe` offset bug (signature is at bytes 8..9, not 4..5), published `PureRaw/v0.1.1`. SkyStacker now declares `require ...PureRaw v0.1.1`, imports `raw` + `formats/cr2`, and the duplicated `pr*` CR2 decoder was deleted from `main.go`. `GOWORK=off` build/test and E2E (1-light CR2 stack, 7.5 s) are green; 209 self-tests. The binary is no longer "single source file" but remains a single stdlib-only executable plus one first-party module.

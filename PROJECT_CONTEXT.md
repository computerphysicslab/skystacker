# SkyStacker — project context (last updated 2026-11-11)

## Version
v0.5.0 (single-file Go program, `main.go`)

## Current pipeline steps (post-processing)
1. Background removal (poly 1–3)
2. Star removal
3. Black-point shift (intermediate, not written)
4. STF/MTF auto-stretch
5. Guided-filter denoise
6. **Curves (new)** — `x' = clamp((x−black)/(1−black),0,1)`, `out = x'^gamma`
7. HDR blend (step 6 + step 1)
8. Weighted mix (4 variants of step 1 + step 7)

## Current flags relevant to recent work
- `--curves-black` (default 0.03)
- `--curves-gamma` (default 1.20)
- `--no-curves`

## Known constraints
- Registration requires ≥ 8 inliers and ratio ≥ 0.20.
- Crop is the largest fully-valid rectangle.
- Self-tests: 77 passing (as of last patch).

## Recent decisions / caveats
- Curves are applied identically per channel to preserve hue.
- Step 7 uses the curves output as its display base.
- Backups: `main.go.before_curves_step.bak` kept locally.
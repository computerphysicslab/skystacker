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
## Last validated run (2026-11-11)
- Dataset: 2019-01-25-SanJuan/Orion-135mm, 70 × JPG, 2400×1600
- Registration: 70/70 matched, 72–94 inliers, scale ~1.001, rot ≤ 0.07°
- Affine refinement: 0 frames (similarity was already sufficient; no `[affine]` markers)
- Crop: 2397×1355 (99.87% W, 84.7% H) — loss is real Y drift (~245 px), not registration error
- Auto-stretch: aggressive (m ≈ 0.015–0.021, c0 = 0); unlinked per-channel
- Runtime: 9 m 58 s, peak memory 646 MB, 6.1 GB disk spill (sigma method)
- Post-processing: all 8 steps produced outputs (post_step1 … post_step8_4)
- No warnings, no failures

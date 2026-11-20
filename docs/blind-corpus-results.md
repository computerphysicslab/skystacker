# Blind plate-solving corpus results (2026-11-19)

The user-supplied corpus in `assets/Blind-Plate-Solving/` contains 14 processed
images of known deep-sky fields at effective focal lengths from 50 mm to
1200 mm (focal length is **not** present in EXIF). It is used as a realistic
blind-solve test set: no pointing seed, no pixel-scale hint, no metadata.

## How to reproduce

```bash
scripts/bench-blind.sh assets/Blind-Plate-Solving mag10 /tmp/blind-corpus
```

The script runs `--image-op plate-solve --plate-solver native` with a published
magnitude-limited Gaia index and writes `results.csv` with success, method,
RMS, match count, wall time and the last error.

## Results with the `mag10` index (G ≤ 10, nside 4)

| Image | Result | Votes | Time |
|---|---|---|---|
| Horsehead.jpg | no consistent transform | 14 | 50 s |
| Las 3 Marias.jpg | no consistent transform | 9 | 59 s |
| M17 con refractor de 150mm y CLS - Paniza.jpg | no consistent transform | 20 | 54 s |
| M31.jpg | no consistent transform | 20 | 53 s |
| M33-hdr.jpg | no consistent transform | 14 | 50 s |
| M42.jpg | no consistent transform | 20 | 51 s |
| M51-2018-08-13.png | no consistent transform | 24 | 55 s |
| M8 con refractor de 150mm y CLS - Paniza.jpg | no consistent transform | 19 | 57 s |
| MilkyWay.jpg | no consistent transform | 20 | 56 s |
| Nebulosa California con 300mm y CLS - Lecinena.png | no consistent transform | 24 | 54 s |
| Orion.jpg | no consistent transform | 24 | 53 s |
| Pleiades.jpg | no consistent transform | 17 | 44 s |
| Rosetta.jpg | no consistent transform | 14 | 44 s |
| Trifid.png | no consistent transform | 19 | 63 s |

**Success rate: 0/14.** With `mag10deep` (333,567 stars) a few images reach
39–76 votes but still fail verification.

## Diagnosis

1. **Voting is not the only problem**: quad matches produce 9–24 votes for
   `mag10`, i.e. spurious descriptor collisions dominate. Geometric quad
   verification was added (similarity fit from a matched quad plus inlier
   counting, both parities), but no real image reached the ≥6-inlier threshold.
2. **Seeded control also fails**: `M42.jpg` with the correct
   `--plate-center 83.822,-5.391` and a cached Gaia tile still reports
   `0 matched pairs`, so the Gaia matcher itself struggles on these processed
   JPEGs. The seeded solver only loads one `nside=64` tile (about 30 stars for
   that field), which is too few for a 7°×5° image.
3. **Descriptor tolerance**: fixed linear bins (0.05 and 0.02) without
   exponential binning lose correct quads when centroids are noisy or stars
   are blended; Astrometry.net uses a 4-D k-d tree with Bayesian verification.
4. **Processed images**: these are stretched/compressed JPEGs, not linear
   lights; source extraction is noisier than for the raw-frame pipeline.

## Second pass: matcher diagnostics (2026-11-19)

To raise the success probability the matcher was changed in four ways:

- `--gaia-multitile` gathers Gaia stars from every cached tile around the
  seed (the single tile has only ~30 stars for a wide field);
- `--gaia-loose-match` relaxes the acceptance threshold from 1/4 to 1/8 of
  the brightest stars and widens tolerance to 8 arcsec for processed JPEGs;
- the inlier threshold now uses the *brightest 40* stars (`nF`) rather than all
  detections, which could be hundreds of noise blobs;
- nearest-neighbour spacing replaces all-pairs median for the initial scale,
  and a focal-length hint can override it entirely.

Additionally, solutions are rejected when the reprojection RMS exceeds 5 px
or the solved centre is more than 2° from a user-supplied seed. With these
guards the seeded M42 attempt produces a plausible-looking but false solution
(RMS 17 px, 0.26 arcsec/px for a 50 mm image), which is now correctly rejected
instead of reported. The remaining work is a better scale estimator and a
proper multi-hypothesis (RANSAC + log-odds) verifier.

**Second-pass engineering (2026-11-20).** The four blueprint phases were implemented: `--blind-detrend` with HFD/eccentricity gating, multi-tile Gaia gathering by default, Bayesian log-odds with scale-consistent pruning and best-hypothesis selection, an SBI1 binary index with density caps and exponential descriptor keys, and sub-degree nside-16/32 regional tooling with stored proper motions. Seven Go tests cover the new units. The corpus result is still **0/14**: the engineering is shipped but the scientific acceptance criteria are not met, so the roadmap rows are marked executed/not accepted.

Set `SKYSTACKER_DEBUG_MATCH=1` to print matcher diagnostics
(`nF`, `nG`, scale estimate, triangle counts, best inliers, threshold).

## Concrete next steps

- Gather Gaia stars from multiple cached tiles around the seed (radius-based)
  for the seeded solver, not just one tile.
- Replace linear descriptor bins with Tetra-style exponential bins and a
  spatial index; increase the number of quads per star selectively.
- Add RANSAC over quad-derived similarity hypotheses with scale/rotation/
  parity and use log-odds to rank hypotheses.
- Estimate an initial pixel scale from star density when no EXIF focal length
  exists, and restrict the search to the matching band.
- Keep these 14 images as the regression corpus: any solver change must be run
  through `scripts/bench-blind.sh` and improve this table.

See [plate-solving.md](plate-solving.md) for the solver architecture and
`research/02` §15–§16 for the algorithm background.

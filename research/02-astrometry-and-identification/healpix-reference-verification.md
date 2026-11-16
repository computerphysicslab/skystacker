# HEALPix Reference Verification & Face-Geometry Calibration

**Scope:** Reference convention, calibration procedure against `healpy 1.16.5`, and empirical resolution of the spherical diamond face-assignment geometry used in SkyStacker's zero-dependency HEALPix nested implementation (Górski et al. 2005, §5).

---

## 1. Coordinate & Indexing Convention

SkyStacker implements the HEALPix nested scheme from Górski et al. (2005, §5) with zero external dependencies. Verification is anchored against `healpy 1.16.5` (`hp.ang2pix(nside, theta, phi, nest=True)`):

```text
theta = π/2 − dec   (colatitude, radians)
phi   = ra          (longitude, radians)
nest  = True        → nested scheme
```

This convention is confirmed by the three `nside=1` cases in `testHealpixNested`, all of which pass against `hp.ang2pix` under this exact mapping.

---

## 2. Face Geometry: Empirical Findings

HEALPix's 12 base faces are **spherical diamonds**, not constant-declination bands. The naive geometric model (`|dec| > 41.81° = polar face, else equatorial`) is **incorrect**:

```text
pix2ang(256,     0) = (ra=45.00°, dec= 0.15°)   face 0, equatorial vertex
pix2ang(256, 65535) = (ra=45.00°, dec=89.82°)   face 0, polar vertex
pix2ang(256, 18344) = (ra=70.66°, dec=32.62°)   face 0, mid-diamond
```

* Face `0` spans a diamond from $(\text{RA}=45^\circ, \text{Dec}=0.15^\circ)$ to the north celestial pole, reaching down to $\text{Dec}=41.81^\circ$ only along $\text{RA}\approx 0^\circ$.
* The $z = 2/3$ boundary marks where **faces 4–7 end**, not where **faces 0–3 begin**.
* Its boundary with faces `4` and `5` follows a great circle rather than a constant-declination parallel:
  ```text
  dec=32.6° at ra=10°..80°  -> face 0
  dec=32.6° at ra= 0°       -> face 4
  dec=32.6° at ra=85°..90°  -> face 5
  ```

### Root Cause & Code Resolution
The pre-`A1a.5` branch structure (`za <= 2/3 → equatorial`) misassigned points falling inside the equatorial vertices of the polar faces. The bug resided in the **face-assignment logic**, not in the within-face bit-interleave logic as originally hypothesised.

* **Resolution:** **A1a.5 code fix applied at commit `e47c3fd`.**

---

## 3. Regenerating the Reference Test Fixture

To regenerate `assets/testdata/healpix_reference.json` after a `healpy` upgrade or a refactor of SkyStacker's pixel math:

```bash
python3 -m venv /tmp/healpix-venv
/tmp/healpix-venv/bin/pip install healpy==1.16.5
/tmp/healpix-venv/bin/python scripts/healpix/generate_fixture.py \
    > assets/testdata/healpix_reference.json
```

Run the Go self-test suite to verify exact agreement:

```bash
./skystacker --selftest
```

The following self-tests consume `assets/testdata/healpix_reference.json`:
* `testHealpixNside16Match`
* `testHealpixNside256Match`
* `testHealpixCornersMatch`

---

## 4. Calibration Run Log (`healpy 1.16.5`, 2026-11-13)

The diagnostic calibration run that confirmed the `theta`/`phi` convention and exposed the face-boundary geometry can be re-executed at any time via `scripts/healpix/check_calibration.py` to verify that upstream `healpy` behavior has not changed:

* **Part 1 (`nside=1` convention verification):** Three-line `nside=1` output confirming `theta = π/2 - dec`, `phi = ra`, `nest=True`.
* **Part 2 (Face-scan verification):**
  * Declination scan at $\text{RA}=0^\circ$ mapping $\text{Dec} \rightarrow \text{face}$.
  * Right Ascension scan at $\text{Dec}=32.6^\circ$ confirming transitions across faces `4`, `0`, and `5`.

## Implementation status (v0.6.0)

The nested HEALPix helpers are in `main.go` and covered by `testHealpix*`.
The fixture path is `assets/testdata/healpix_reference.json`. This document
remains the verification log, not a pending implementation task.

---

## 2026-11-16 deep dive append (SkyStacker)

See `../05-research-deep-dive-2026-11-16.md` (pillar B). HATS/LSDB (Apache Parquet per adaptive HEALPix pixel) and cds-healpix-rust/MOCPy/HiPS/MOC workflows are the modern path for billion-row reference handling outside central TAP; consider for offline catalogue caches if SkyStacker grows annotation features.

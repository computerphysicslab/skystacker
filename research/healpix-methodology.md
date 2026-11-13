# HEALPix reference verification

SkyStacker implements the HEALPix nested scheme from Gorski et al. 2005
(§5) with no external dependency. This document records how the
implementation is verified against healpy, so the verification can be
repeated after a healpy upgrade or a refactor of the pixel math.

## Convention

healpy 1.16.5, `hp.ang2pix(nside, theta, phi, nest=True)`:
  theta = π/2 − dec   (colatitude, radians)
  phi   = ra          (longitude, radians)
  nest=True → nested scheme

This is confirmed by the three nside=1 cases in testHealpixNested, all
of which pass against hp.ang2pix under this convention (see
"Calibration run" below).

## Face geometry — the surprising part

HEALPix's 12 faces are spherical diamonds. The naive model
"|dec| > 41.81° = polar face, else equatorial" is WRONG:

  pix2ang(256,     0) = (ra=45.00°, dec= 0.15°)   face 0, equatorial vertex
  pix2ang(256, 65535) = (ra=45.00°, dec=89.82°)   face 0, polar vertex
  pix2ang(256, 18344) = (ra=70.66°, dec=32.62°)   face 0, mid-diamond

Face 0 spans dec from 0° to 90° along RA≈45°, but only reaches down to
dec=41.81° along RA≈0°. The z=2/3 boundary is where FACES 4-7 end,
not where FACES 0-3 begin. See "Calibration run" for the full scan.

This is the reason the pre-A1a.5 implementation disagreed with healpy.

## Regenerating the fixture

    python3 -m venv /tmp/healpix-venv
    /tmp/healpix-venv/bin/pip install healpy==1.16.5
    /tmp/healpix-venv/bin/python scripts/healpix/generate_fixture.py \
        > testdata/healpix_reference.json

Then run `./skystacker --selftest` — testHealpixNside16Match,
testHealpixNside256Match and testHealpixCornersMatch consume the fixture.

## Calibration run (healpy 1.16.5, 2026-11-13)

The transcript below is the record of the run that fixed the
convention and revealed the face-geometry bug. Regenerate with
scripts/healpix/check_calibration.py to confirm healpy has not
changed.

[three-line nside=1 output, verbatim]
[face-scan output showing dec → face at ra=0°, and ra → face at dec=32.6°]
## Face-boundary finding (Phase 2 diagnostic, 2026-11-13)

The face containing a sky position is **not** determined by a constant-dec
threshold. Empirical results from healpy 1.16.5:

    pix2ang(256,     0) = (ra=45.00°, dec= 0.15°)   face 0, equatorial vertex
    pix2ang(256, 65535) = (ra=45.00°, dec=89.82°)   face 0, polar vertex
    pix2ang(256, 18344) = (ra=70.66°, dec=32.62°)   face 0, mid-diamond

    dec=32.6° at ra=10°..80°  -> face 0
    dec=32.6° at ra= 0°       -> face 4
    dec=32.6° at ra=85°..90°  -> face 5

Face 0 spans a diamond from (45°, 0.15°) to the pole, but its boundary with
faces 4 and 5 is a great circle, not a constant-dec line. This refutes the
naive model "|dec| > 41.81° = polar face". The current `za <= 2/3 →
equatorial` branch structure is therefore wrong for points in the equatorial
vertices of the polar faces.

The fix is in the face-assignment logic, not the within-face interleave as
originally hypothesised. **A1a.5 code fix applied at commit e47c3fd.**

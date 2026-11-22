# Post-processing

Post-processing works on the stacked linear data and writes a sequence of
independently inspectable JPEG previews. Stage selection is controlled by
`--stack-only`, `--post-only`, `--resume` and the individual disable flags.

## Default display pipeline

| Step | File | Operation |
|---|---|---|
| 0 | `post_step0-stretched.jpg` | Initial stretch for inspection. |
| 1 | `post_step1_background_removed.jpg` | Polynomial background removal (`--bg-degree`). |
| 2 | `post_step2_star_removed.jpg` | Star replacement via annulus or PSF radii. |
| 4 | `post_step4_auto_stretched.jpg` | PixInsight-style STF/MTF auto-stretch. |
| 5 | `post_step5_denoised.jpg` | Guided-filter denoise. |
| 5b | `post_step5_hdr.jpg` | HDR local contrast. |
| 6 | `post_step6_curves.jpg` | Background-darkening curves. |
| 7 | `post_step7_1..4.jpg` | Weighted mild variants of steps 5–7. |
| 8 | `post_step8_1..4.jpg` | Final mix, optional diffraction spikes. |

`--no-curves`, `--no-denoise`, `--no-hdr-blend`, `--no-hdr-mix`, `--no-sharpen`,
`--no-color-calib` and `--no-auto-stretch` disable individual steps.
`--post-preset detail-last` moves detail enhancement after denoise.

## Denoise and sharpening

- `--denoise-strength N` (0–10) controls guided-filter iterations;
  `--denoise-radius N` sets the spatial radius and `--denoise-chroma F` the
  chroma power.
- `--wavelets` adds à trous multiscale denoise, optionally with
  `--wavelet-adaptive` per-tile MAD thresholds.
- `--deconvolve` uses Richardson–Lucy; `--deconvolve-wiener` switches to FFT
  Wiener deconvolution; `--deconvolve-grid` fits a local PSF over a 3×3/4×4 grid.
- `--wavesharp` performs multi-scale Gaussian detail enhancement controlled by
  `--wave-gains` (coarse-to-fine list).

## Specialised modes

- `--starless` writes `starless` and `star-layer` float32 stacks plus JPEGs.
- `--narrowband HOO|SHO` combines `--nb-a/--nb-b/--nb-c` mono `.f32raw` stacks.
- `--drizzle-upsample N` writes a drizzled upsample; `--drizzle-pixfrac F`
  controls the kernel footprint.
- `--comet-combine` recombines comet and star stacks with a mask/blend.
- `--external-bge/-denoise/-deconv/-starless CMD` delegates to licensed external
  AI CLIs through a private 32-bit float FITS sidecar. Commands are argv lists
  with `{input}`/`{output}` placeholders, never shell strings.
- `--annotate`, `--sky-map`, `--survey-cutout`, `--photometry` add astrometric
  products; see [astrometry-products.md](astrometry-products.md).

## Standalone image operations

```bash
skystacker --process-image photo.jpg --image-op blur --image-output /tmp/ops
skystacker --process-image a.jpg,b.jpg --image-op sum --image-output /tmp/ops
skystacker --process-image photo.jpg --image-op measure-quality --image-output /tmp/ops
```

Operations: `resize`, `blur`, `sharpen`, `background`, `stretch`, `denoise`,
`deconvolve`, `starless`, `sum`, `subtract`, `average`, `median`, `min`, `max`,
`measure-transform`, `measure-quality`, `sky-map`, `survey-cutout`,
`plate-solve`. All of them leave the inputs untouched except `resize --overwrite`.

### Downscale images (resize)

`resize` uses area averaging (no aliasing) and keeps the source format. It
accepts files, a comma-separated list or a whole directory; `--scale N` means
1/N in each dimension. JPG output keeps the original APP1 Exif/XMP metadata.

```bash
# Copies: <name>_resize.jpg in /tmp/resized
skystacker --process-image assets/2024-08-07-Riofrio-M31-refr150   --image-op resize --scale 3 --image-output /tmp/resized

# In place (atomic replacement of every JPG in the folder)
skystacker --process-image assets/2024-08-07-Riofrio-M31-refr150   --image-op resize --scale 3 --overwrite
```

6000x4000 lights become 2000x1333 at `--scale 3`. Make a backup before an
in-place run; without `--overwrite` an existing output is refused rather than
silently replaced.

## Reproducibility

`report.txt` records the command line and resolved flags; `--provenance` writes
a per-stage IVOA-style graph and `--json-report` writes a structured report.
Use `--resume` to repeat post-processing with different flags without touching
the stack.

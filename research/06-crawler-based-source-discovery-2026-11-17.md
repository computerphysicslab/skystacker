# Crawler-based source discovery for SkyStacker research lines

**Date:** 2026-11-17
**Tool:** `goCrawler` (Go 1.20, SQLite cache, `robots.txt` policy enabled)
**Profile:** `profiles/skystacker-research.yaml`
**Scope:** English-only web sources aligned with the SkyStacker research lines in
`01-architecture-and-core-engine/`, `02-astrometry-and-identification/` and
`03-reference-and-literature/`.

---

## 1. Method

Seed URLs were extracted from the 140 external references already cited in this research folder
(`source-catalog-and-microarchitecture.md` and the synthesis documents). Banned domains in
`crawler.yaml` (`github.com`, `doi.org`, `google.com`, ...) were excluded, as were URLs with
malformed suffixes or tracking query strings. The resulting 21 seeds focus on:

- calibration / stacking: Siril, DeepSkyStacker, PixInsight, Astro Pixel Processor;
- astrometry / plate solving: ASTAP, SCAMP, Astrometry.net, Gaia DR4;
- standards and data: WCS/FITS, IVOA, SIMBAD, Aladin/HiPS, Minor Planet Center;
- pixel-level techniques: drizzle (Bayer and CFA), photometry, source detection.

The crawler keeps only English paragraphs, applies stopword/length/relevance/uniqueness filters and
a `scoreThreshold` of 15. All downloads used the local SQLite cache and the new `robots.txt`
policy; Redis was not running.

---

## 2. Discovered sources kept in the curated corpus

18 documents were retained. Highest-ranked sources (score, curated length, URL):

| Score | Chars | Source |
|---:|---:|---|
| 71.71 | 69 | `https://photutils.readthedocs.io/` |
| 47.63 | 3213 | `https://siril.readthedocs.io/en/latest/preprocessing/drizzle.html` |
| 43.63 | 1181 | `http://deepskystacker.free.fr/english/technical.htm` |
| 40.11 | 2243 | `https://mypetstars.com/tutorials/pixinsight/processes/drizzleintegration/cfa-drizzle` |
| 36.53 | 17269 | `https://www.hnsky.org/astap.htm` (ASTAP) |
| 36.06 | 1729 | `https://www.stsci.edu/~fruchter/dither/drizzle.html` (original drizzle paper) |
| 35.32 | 1354 | `http://astrometry.net/doc/readme.html` |
| 34.26 | 212 | `https://reproject.readthedocs.io` |
| 32.91 | 1116 | `https://siril.readthedocs.io/en/stable/astrometry/annotations.html` |
| 26.98 | 1373 | `https://www.astropixelprocessor.com/community/tutorials-workflows/bayer-drizzle/` |
| 23.90 | 69 | `https://www.astromatic.net/software/scamp/` |
| 22.00 | 9108 | `http://www.minorplanet.info/ObsGuides/Misc/photometryguide.htm` |

Other retained sources: `astrometry.net/`, `astrometry.net/use.html`,
`www.cosmos.esa.int/web/gaia/dr4`, `astroquery.readthedocs.io`,
`www.ucolick.org/~sla/fits/`, `www.atnf.csiro.au/people/mcalabre/WCS/index.html`.

**Relevance assessment:** the yield is well aligned with the project. The corpus covers the three
core research domains: registration (ASTAP, SCAMP, Astrometry.net), stacking/calibration (DSS,
Siril, PixInsight, APP) and astronomical identification (Gaia DR4, WCS, photometry). The drizzle
cluster (Siril, CFA PixInsight, APP Bayer, STScI original paper) is directly useful for the
CFA/Bayer drizzle work mentioned in the external deep-research outcome.

---

## 3. `robots.txt` enforcement observations

The new `robotspolicy` package fetched and cached one `robots.txt` per host before the first online
request and blocked 8 URLs:

- `https://ivoa.net/documents/MOC/`, `https://www.ivoa.net/documents/VOTable/`,
  `https://www.ivoa.net/documents/ConeSearch/` — IVOA standard documents;
- `http://nova.astrometry.net/`, `https://nova.astrometry.net/` — Astrometry.net submission UI;
- `https://docs.astropy.org` — documentation root;
- `http://tdc-www.harvard.edu/software/wcstools.html`, `http://flickr.com/groups/astrometry`.

Missing or unreachable `robots.txt` files were treated as "allow" (fail open) after logging the
error; this happened for `iau-commission5.wikispaces.com` (dead DNS),
`cdsarc.cds.unistra.fr` and `tapvizier.cds.unistra.fr` (timeouts) and `www.stsci.edu`.

**Implication for the research:** blocked sources must be read manually or obtained from an
alternative endpoint; the crawler is now a well-behaved client and will not bypass those rules.

---

## 4. Runtime and CPU note

The machine has **8 cores**. The crawler is **not** using them: it is a single sequential loop.
A controlled run measured 24.78 s user + 0.88 s system CPU over 106 s wall time, i.e. **24 % of a
single core** (network-bound). `GOMAXPROCS` defaults to 8, but no work is parallelised; the only
extra goroutine is the optional external indexer, disabled by default. Concurrency would require
the roadmap's worker-pool item plus per-domain rate limiting.

---

## 5. Follow-up suggestions

1. Add `astrometry.net/doc/`, `www.astromatic.net/software/` and `hips` documentation landing
   pages as seeds to broaden registration coverage (the crawler already discovers many from them).
2. Add a dedicated profile for "synthetic calibration frames / self-flat", matching
   `04-calibration-and-self-flat.md`, with `flat|dark|bias|self-flat|synthetic|master` keywords.
3. Fix the frequency-dump bug (roadmap item 18) before using `corpusNoEngFrequencies.txt` as a
   keyword source for new research documents.
4. Re-run with a longer time budget: this run hit the 240 s cap while the link pool was still
   non-empty, so the 18 documents are a partial result.

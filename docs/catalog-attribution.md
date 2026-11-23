# Catalogue attribution

## Bright-star names, Bayer and Flamsteed designations

The offline annotation bundle embeds a compact derived table of 3,241 stars
(V <= 6.5) with proper names, Bayer letters, Flamsteed numbers, constellation
abbreviations, J2000 positions and magnitudes. The table is generated from:

- `data/stars.6.json` and `data/starnames.json` of the **d3-celestial** project
  (<https://github.com/ofrohn/d3-celestial>), Copyright (c) 2015 Olaf Frohn,
  licensed under the BSD 2-Clause License.
- Those data are themselves derived from the Yale Bright Star Catalogue and the
  Hipparcos catalogue (public astronomical data).

Derived data is stored in `annotation_starnames.go` as a compact string table.
Greek Bayer letters are transliterated to ASCII (`alpha And`) so the raster
bitmap font can draw every label; the original Greek form is not required by
the renderer. The full upstream notice follows.

> Copyright (c) 2015, Olaf Frohn
> All rights reserved.
>
> Redistribution and use in source and binary forms, with or without
> modification, are permitted provided that the following conditions are met:
>
> 1. Redistributions of source code must retain the above copyright notice,
>    this list of conditions and the following disclaimer.
> 2. Redistributions in binary form must reproduce the above copyright notice,
>    this list of conditions and the following disclaimer in the documentation
>    and/or other materials provided with the distribution.
> 3. Neither the name of the copyright holder nor the names of its contributors
>    may be used to endorse or promote products derived from this software
>    without specific prior written permission.
>
> THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
> AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
> IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
> ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
> LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
> CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
> SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
> INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
> CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
> ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
> POSSIBILITY OF SUCH DAMAGE.

## PGC dense-field names

The offline annotation bundle also embeds four `PGC` galaxies (with `LEDA`
aliases accepted as input) from NASA/IPAC Extragalactic Database (NED) object
queries performed on 2026-11-23:

- PGC 21991, PGC 2167368, PGC 2183568 and PGC 3087883.

NED is operated by the Jet Propulsion Laboratory, California Institute of
Technology, under contract with NASA (<https://ned.ipac.caltech.edu/>). Only
factual coordinates, magnitudes and published major/minor axes are stored in
`annotation_pgc.go`; where NED has no diameter, the marker stays a point
instead of inventing an ellipse. These factual data are used for visual
identification in annotation maps and are attributed here.

## SAO bright-star identifiers

The offline annotation bundle embeds a compact V <= 8.0 subset of the
Smithsonian Astrophysical Observatory Star Catalog (SAO, CDS `I/131A`, SAO Staff
1966, SAO Special Report 119) generated from the HEASARC mirror
(`I/131A/sao`, <https://heasarc.gsfc.nasa.gov/W3Browse/star-catalog/sao.html>).
The source is the HEASARC TDAT dump
(`https://heasarc.gsfc.nasa.gov/FTP/heasarc/dbase/dump/heasarc_sao.tdat.gz`,
258,944 rows); 46,136 stars with V <= 8.0 are stored in `annotation_sao.go`
with their J2000 position, V magnitude, spectral type and `DM`/`HD`/`GC`
cross-identifiers. HEASARC publishes this catalogue as a **U.S. Government
Work** (<https://www.usa.gov/government-works>), free to use and redistribute;
it is used here as factual positional/cross-ID data for bright-star labelling.

## Digitized Sky Survey plates

Aligned survey cutouts fetched with `--survey-source dss` come from the STScI/MAST
Digitized Sky Survey (<https://archive.stsci.edu/dss/>). MAST distributes DSS
data under **CC BY-NC-SA 4.0**, free for scientific, educational and non-profit
use with attribution; the data are **not** public domain and individual plate
rights vary. Every aligned product preserves the source `PLATEID` and
`COPYRGHT` cards (for example `Caltech/Palomar` for POSS-II) in the FITS header
and records the retrieval URL, licence and attribution in `<stem>_alignment.json`.
Verify rights with the plate holder before any commercial use.

## Other catalogues

Gaia DR3 tile data is fetched from the CDS/ESA services and remains subject to
their respective terms. Messier/NGC/IC, LDN/Sh2 and IAU constellation lines in
the offline bundle are short factual tables compiled for this project.

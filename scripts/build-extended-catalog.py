#!/usr/bin/env python3
"""Generate the compact offline deep-sky catalogue used by SkyStacker.

The table keeps one deterministic row per astronomical object, merging the
licence-clean OpenNGC database with the public HEASARC dumps of several
specialised catalogues. Rows written to annotation_dso_catalogs.go use the
format:

    name|RA(deg)|Dec(deg)|mag|major(")|minor(")|PA(deg)|type|priority|aliases

Sources (downloaded once into the input directory):

  NGC.csv / addendum.csv
    https://raw.githubusercontent.com/mattiaverga/OpenNGC/master/database_files/
    CC BY-SA 4.0, Mattia Verga. The generated derived table stays CC BY-SA 4.0.

  heasarc_<name>.tdat.gz
    https://heasarc.gsfc.nasa.gov/FTP/heasarc/dbase/dump/
    HEASARC public mirrors of the published catalogues (US Government Work).

Usage:
    python3 scripts/build-extended-catalog.py --input /tmp/opencode/skystacker-catalogs \
        --output annotation_dso_catalogs.go
"""

import argparse
import csv
import gzip
import math
import os
import re
from collections import Counter


# Catalogues are merged in this order; the first name wins and later entries
# contribute aliases. Priorities are used only by the annotation label placer.
CATALOG_PRIORITY = {
    "messier": 85,
    "opengc": 75,
    "lbn": 70,
    "ldn": 60,
    "sh2": 70,
    "magakian": 70,
    "pn": 70,
    "snr": 70,
    "openclust": 70,
    "globclust": 70,
}
CATALOG_ORDER = list(CATALOG_PRIORITY)

# OpenNGC type -> unified annotation type. Stellar, duplicate and non-existent
# entries are skipped: the star catalogues already cover those objects.
OPENNGC_TYPES = {
    "G": "galaxy", "GPAIR": "galaxy", "GTRPL": "galaxy", "GGROUP": "galaxy",
    "OCL": "cluster", "GCL": "cluster", "CL+N": "nebula", "*ASS": "cluster",
    "HII": "nebula", "NEB": "nebula", "EMN": "nebula", "RFN": "nebula",
    "DRKN": "dark-nebula",
    "PN": "planetary",
    "SNR": "remnant",
    "OTHER": "nebula",
}
SKIP_OPENNGC_TYPES = {"DUP", "NONEX", "*", "**", "NOVA"}

CATALOG_TYPES = {"galaxy", "nebula", "dark-nebula", "planetary", "cluster", "remnant"}

SKY_SPLIT = re.compile(r"\s*[,;]\s*")


def clean(value):
    """Return a stripped field with no table separator characters."""
    return str(value or "").replace("|", " ").replace("\n", " ").strip()


def sexagesimal(value, is_ra):
    """Convert an OpenNGC sexagesimal RA/Dec string to degrees."""
    parts = clean(value).replace(":", " ").split()
    if len(parts) < 3:
        try:
            return float(value)
        except (TypeError, ValueError):
            return None
    try:
        sign = -1 if parts[0].startswith("-") else 1
        first = abs(float(parts[0]))
        result = first + float(parts[1]) / 60 + float(parts[2]) / 3600
    except (TypeError, ValueError):
        return None
    return result * 15 if is_ra else sign * result


def numeric(value, default=0.0):
    """Parse a numeric field, returning default for empty or malformed values."""
    try:
        result = float(clean(value))
    except (TypeError, ValueError):
        return default
    return result if math.isfinite(result) else default


def make_entry(name, ra, dec, mag, major, minor, pa, kind, catalog, aliases=()):
    """Build a catalogue entry after validating coordinates and geometry."""
    name = clean(name)
    kind = clean(kind).lower()
    if not name or kind not in CATALOG_TYPES:
        return None
    if ra is None or dec is None:
        return None
    ra, dec = float(ra), float(dec)
    if not (0 <= ra < 360 and -90 <= dec <= 90):
        return None
    major = max(0.0, min(600.0 * 60, major))
    minor = max(0.0, min(600.0 * 60, minor))
    if minor > major:
        major, minor = minor, major
    priority = CATALOG_PRIORITY["ldn"] if kind == "dark-nebula" else CATALOG_PRIORITY[catalog]
    alias_set = []
    for alias in aliases:
        alias = clean(alias)
        if alias and alias.lower() != name.lower() and alias not in alias_set:
            alias_set.append(alias)
    return {
        "name": name, "ra": ra, "dec": dec, "mag": mag,
        "major": major, "minor": minor, "pa": pa, "type": kind,
        "catalog": catalog, "priority": priority, "aliases": alias_set,
    }


def dimension(value):
    """Parse a HEASARC dimension string such as '11X10' or '8' into arcmin."""
    numbers = re.findall(r"\d+(?:\.\d+)?", clean(value))
    if not numbers:
        return 0.0, 0.0
    major = float(numbers[0])
    minor = float(numbers[1]) if len(numbers) > 1 else major
    return major * 60, minor * 60


def read_tdat(path):
    """Yield each row of a HEASARC TDAT dump as a field dictionary."""
    opener = gzip.open if str(path).endswith(".gz") else open
    with opener(path, "rt", encoding="utf-8", errors="replace") as handle:
        columns = []
        in_data = False
        for raw in handle:
            line = raw.rstrip("\n")
            if line.startswith("line[1] = "):
                columns = line.split("=", 1)[1].split()
                continue
            if line.strip() == "<DATA>":
                in_data = True
                continue
            if not in_data or line.startswith("<") or not line.strip():
                continue
            cells = line.split("|")
            if len(cells) < len(columns):
                cells += [""] * (len(columns) - len(cells))
            yield dict(zip(columns, cells))


def add_tdat(entries, path, catalog, extractor):
    """Append every valid row of a HEASARC dump through a catalog extractor."""
    count = 0
    for row in read_tdat(path):
        entry = extractor(row, catalog)
        if entry is not None:
            entries.append(entry)
            count += 1
    print(f"  {catalog:10s} {count:6d} rows")


def extract_messier(row, catalog):
    major, minor = dimension(row.get("dimension", ""))
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      numeric(row.get("vmag")), major, minor, 0, "nebula", catalog,
                      [row.get("alt_name", "")])


def extract_lbn(row, catalog):
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      0, numeric(row.get("large_dimension")) * 60,
                      numeric(row.get("small_dimension")) * 60, 0, "nebula", catalog,
                      [row.get("alt_name", "")])


def extract_ldn(row, catalog):
    area = numeric(row.get("cloud_area"))
    diameter = 2 * math.sqrt(area / math.pi) * 60 if area > 0 else 0
    aliases = [f"B {v.strip()}" for v in clean(row.get("barnard_ids", "")).split(",") if v.strip()]
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      0, diameter, diameter, 0, "dark-nebula", catalog, aliases)


def extract_sh2(row, catalog):
    diameter = numeric(row.get("diameter")) * 60
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      0, diameter, diameter, 0, "nebula", catalog)


def extract_magakian(row, catalog):
    aliases = []
    for field in ("ngc_ic_names", "vdb_name", "vdbh_name", "ced_name", "gn_name",
                  "brand_name", "ss_name", "dg_name", "dg_name_2", "rk_name", "alt_names"):
        aliases.extend(SKY_SPLIT.split(clean(row.get(field, ""))))
    primary = next((a for a in aliases if re.match(r"^(NGC|IC)\s*\d+", a, re.I)), None)
    if primary is None:
        primary = next((a for a in aliases if re.match(r"^(VDB|GN|CED)\s*[\d.\-]+", a, re.I)), None)
    if primary is None:
        primary = clean(row.get("name"))
    return make_entry(primary, numeric(row.get("ra")), numeric(row.get("dec")),
                      0, 0, 0, 0, "nebula", catalog, [clean(row.get("name", ""))] + aliases)


def extract_pn(row, catalog):
    name = clean(row.get("pn_name")) or clean(row.get("name")) or clean(row.get("pk_name"))
    aliases = [clean(row.get(k, "")) for k in ("name", "pk_name", "alt_name_1", "alt_name_2",
                                               "alt_name_3", "alt_name_4")]
    diameter = numeric(row.get("opt_diameter")) or numeric(row.get("rad_diameter"))
    return make_entry(name, numeric(row.get("ra")), numeric(row.get("dec")),
                      0, diameter, diameter, 0, "planetary", catalog, aliases)


def extract_snr(row, catalog):
    major = numeric(row.get("major_diameter")) * 60
    minor = numeric(row.get("minor_diameter")) * 60
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      0, major, minor, 0, "remnant", catalog,
                      SKY_SPLIT.split(clean(row.get("alt_names", ""))))


def extract_openclust(row, catalog):
    diameter = numeric(row.get("apparent_diameter")) * 60
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      0, diameter, diameter, 0, "cluster", catalog)


def extract_globclust(row, catalog):
    return make_entry(row.get("name"), numeric(row.get("ra")), numeric(row.get("dec")),
                      numeric(row.get("vmag")), 0, 0, 0, "cluster", catalog,
                      [row.get("alt_name", "")])


def add_opengc(entries, input_dir):
    """Append valid NGC/IC objects from the CC BY-SA 4.0 OpenNGC database."""
    count, skipped = 0, Counter()
    for filename in ("NGC.csv", "addendum.csv"):
        path = os.path.join(input_dir, filename)
        with open(path, newline="", encoding="utf-8") as handle:
            for row in csv.DictReader(handle, delimiter=";"):
                raw_type = clean(row.get("Type", "")).upper()
                kind = OPENNGC_TYPES.get(raw_type)
                if raw_type in SKIP_OPENNGC_TYPES or kind is None:
                    skipped[raw_type] += 1
                    continue
                name = clean(row.get("Name", ""))
                match = re.match(r"^(NGC|IC)\s*(\d+)$", name, re.I)
                if match:
                    name = f"{match.group(1).upper()} {match.group(2)}"
                aliases = []
                if clean(row.get("M", "")).isdigit():
                    aliases.append("M " + str(int(clean(row["M"]))))
                aliases.extend(SKY_SPLIT.split(clean(row.get("Common names", ""))))
                entry = make_entry(
                    name, sexagesimal(row.get("RA", ""), True),
                    sexagesimal(row.get("Dec", ""), False),
                    numeric(row.get("V-Mag")) or numeric(row.get("B-Mag")),
                    numeric(row.get("MajAx")) * 60, numeric(row.get("MinAx")) * 60,
                    numeric(row.get("PosAng")), kind, "opengc", aliases)
                if entry is not None:
                    entries.append(entry)
                    count += 1
    print(f"  {'opengc':10s} {count:6d} rows (skipped {sum(skipped.values())} stellar/duplicate rows)")


def angular_sep_arcsec(a, b):
    """Small-angle separation between two equatorial positions."""
    dra = math.radians((a["ra"] - b["ra"]) * math.cos(math.radians((a["dec"] + b["dec"]) / 2)))
    ddec = math.radians(a["dec"] - b["dec"])
    return math.hypot(dra, ddec) * 180 / math.pi * 3600


def identity(value):
    """Normalise a designation for alias-based duplicate detection."""
    return re.sub(r"\s+", "", clean(value)).lower()


def merge_entries(entries):
    """Merge aliases and duplicate names/positions deterministically.

    Catalogues were added in priority order, so the first entry supplies the
    primary name and type. A 12 arcsec positional tolerance catches the usual
    coordinate differences between catalogues without merging distinct objects.
    """
    merged = []
    by_name = {}
    cells = {}
    cell_deg = 0.05
    for entry in entries:
        keys = [identity(value) for value in [entry["name"]] + entry["aliases"]]
        twin = next((by_name[k] for k in keys if k in by_name), None)
        if twin is None:
            cx = int(entry["ra"] / cell_deg)
            cy = int((entry["dec"] + 90) / cell_deg)
            for dx in (-1, 0, 1):
                for dy in (-1, 0, 1):
                    for idx in cells.get((cx + dx, cy + dy), ()):
                        if angular_sep_arcsec(entry, merged[idx]) <= 12:
                            twin = merged[idx]
                            break
                    if twin is not None:
                        break
                if twin is not None:
                    break
        if twin is None:
            idx = len(merged)
            merged.append(entry)
            for key in keys:
                by_name[key] = entry
            cx = int(entry["ra"] / cell_deg)
            cy = int((entry["dec"] + 90) / cell_deg)
            cells.setdefault((cx, cy), []).append(idx)
            continue
        for alias in [entry["name"]] + entry["aliases"]:
            if identity(alias) != identity(twin["name"]) and alias not in twin["aliases"]:
                twin["aliases"].append(alias)
                by_name[identity(alias)] = twin
        if twin["major"] <= 0 and entry["major"] > 0:
            twin["major"], twin["minor"], twin["pa"] = entry["major"], entry["minor"], entry["pa"]
        if twin["mag"] == 0 and entry["mag"] != 0:
            twin["mag"] = entry["mag"]
        # The Messier dump uses one generic "nebula" class for galaxies,
        # clusters and planetary nebulae. OpenNGC carries the precise type,
        # so refine the merged row instead of mislabelling M 31 or M 57.
        specific = ("galaxy", "cluster", "planetary", "remnant", "dark-nebula")
        if twin["type"] == "nebula" and entry["type"] in specific:
            twin["type"] = entry["type"]
    return merged


def render(entries):
    """Render the deterministic Go source table."""
    rows = []
    for e in sorted(entries, key=lambda x: (x["ra"], x["name"].lower())):
        aliases = ",".join(sorted(set(e["aliases"]), key=str.lower))
        rows.append("|".join((
            e["name"], f"{e['ra']:.5f}", f"{e['dec']:.5f}", f"{e['mag']:.2f}",
            f"{e['major']:.1f}", f"{e['minor']:.1f}", f"{e['pa']:.0f}",
            e["type"], str(e["priority"]), aliases,
        )))
    counts = Counter(e["catalog"] for e in entries)
    provenance = ", ".join(f"{k} {counts[k]}" for k in CATALOG_ORDER if counts[k])
    return f'''package main

// Generated by scripts/build-extended-catalog.py. Do not edit by hand.
// Source catalogues and licences are documented in docs/catalog-attribution.md.
// The OpenNGC-derived rows remain CC BY-SA 4.0 (Mattia Verga); this data table
// is distributed separately from the MIT-licensed SkyStacker source code.
// Merged rows: {len(rows)} ({provenance}).
//
// Format: name|RA(deg)|Dec(deg)|mag|major(")|minor(")|PA(deg)|type|priority|aliases
const offlineExtendedDeepSky = `''' + "\n".join(rows) + "`\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="directory with the downloaded source dumps")
    parser.add_argument("--output", required=True, help="generated Go source path")
    args = parser.parse_args()
    entries = []
    add_tdat(entries, os.path.join(args.input, "heasarc_messier.tdat.gz"), "messier", extract_messier)
    add_opengc(entries, args.input)
    for filename, catalog, extractor in (
        ("heasarc_lbn.tdat.gz", "lbn", extract_lbn),
        ("heasarc_ldn.tdat.gz", "ldn", extract_ldn),
        ("heasarc_hiiregion.tdat.gz", "sh2", extract_sh2),
        ("heasarc_refnebulae.tdat.gz", "magakian", extract_magakian),
        ("heasarc_plnebulae.tdat.gz", "pn", extract_pn),
        ("heasarc_snrgreen.tdat.gz", "snr", extract_snr),
        ("heasarc_openclust.tdat.gz", "openclust", extract_openclust),
        ("heasarc_globclust.tdat.gz", "globclust", extract_globclust),
    ):
        add_tdat(entries, os.path.join(args.input, filename), catalog, extractor)
    merged = merge_entries(entries)
    with open(args.output, "w", encoding="utf-8") as handle:
        handle.write(render(merged))
    print(f"wrote {len(merged)} rows to {args.output}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Consolidate loose research contributions into the canonical theme files.

Dry-run by default. Use --apply to append unique paragraphs to the target
theme file and delete the consumed contribution files.

Protocol: research/README.md
"""

from __future__ import annotations

import argparse
import hashlib
import re
import sys
from datetime import date
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RESEARCH = ROOT / "research"
TODAY = date.today().isoformat()

CANONICAL = {
    "README.md": (),
    "01-architecture-and-core-engine.md": (
        "pipeline", "architecture", "worker", "tile", "sync.pool", "go runtime",
        "clip", "sigma", "median", "lanczos", "bilinear", "demosaic", "post-process",
        "morphology", "diagnostic", "cli", "astap", "observatory", "report.txt",
        "benchmark", "hal", "photometry", "skystacker", "self-test",
    ),
    "02-astrometry-and-identification.md": (
        "astrometry", "plate solve", "plate-solving", "wcs", "sip", "gaia", "healpix",
        "simbad", "vizier", "ned", "tid", "tns", "ztf", "fink", "mpc", "horizons",
        "catalog", "transient", "zogy", "anomal", "quad", "tetra3", "scamp", "dcr",
        "sky chart", "ivao", "aladin", "hips", "moc", "novae", "supernova",
    ),
    "03-reference-and-literature.md": (
        "glossary", "bibliography", "reference", "literature", "keyword index",
        "doi", "arxiv", "roofline", "ebpf", "parca", "pyroscope", "file format",
        "tiff", "xisf", "fits standard", "bxt", "nxt", "sxt", "stellaimage",
    ),
    "04-calibration-and-self-flat.md": (
        "calibration", "self-flat", "master dark", "master flat", "master bias",
        "bias", "dark", "flat", "vignett", "prnu", "dust", "bpm", "bad pixel",
        "irls", "chebyshev", "synthetic", "detector", "scmos", "rtn", "bfe",
        "uberkal", "fgcm", "noisechisel", "dither", "illumination", "pedestal",
    ),
    "05-toolchain-and-modularity.md": (
        "go.mod", "go toolchain", "toolchain", "go1.", "gofmt", "go vet", "gorace",
        "go install", "module", "module path", "pureraw", "go.work", "garbage collector",
        "swiss table", "modular", "rice", "bigtiff", "fits.fz", "json/v2", "synctest",
    ),
    "06-external-ai-sidecars.md": (
        "graxpert", "starnet", "star terminator", "noise terminator", "blur terminator",
        "rc-astro", "cosmic clarity", "sidecar", "onnx", "external-bge", "external-denoise",
        "external-star", "machine learning", "neural", "gpu", "license",
    ),
    "07-native-cr2-decoding.md": (
        "cr2", "cr3", "dcraw", "lossless jpeg", "sof3", "huffman", "bayer", "demosaic",
        "maker note", "eos 40d", "byte-exact", "bit-exact", "raw decode",
    ),
    "08-crawler-source-discovery.md": (
        "crawler", "gocrawler", "seed url", "robots.txt", "curated",
        "curated corpus", "corpuscuratedtext", "scorethreshold", "crawl cycle",
        "cycle", "excerpt", "discovered source", "frequency dump", "sqlite cache",
    ),
}

MARKER_START = "<!-- consolidation-status:start -->"
MARKER_END = "<!-- consolidation-status:end -->"


def normalize(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip().casefold()


def digest(text: str) -> str:
    return hashlib.sha256(normalize(text).encode("utf-8")).hexdigest()


def classify(path: Path, text: str, overrides: dict[str, str]) -> str | None:
    if path.name in overrides:
        return overrides[path.name]
    haystack = text.casefold()
    name = path.name.casefold()
    # Strong filename signals: crawler/curated dumps always belong to 08,
    # even when their excerpts mention Gaia, ASTAP or calibration.
    if "crawler" in name or "curated" in name or "source-discovery" in name:
        return "08-crawler-source-discovery.md"
    best, best_score = None, 0
    scores: dict[str, int] = {}
    for target, keywords in CANONICAL.items():
        if target == "README.md":
            continue
        score = sum(haystack.count(kw) for kw in keywords)
        for token in re.split(r"[^a-z0-9]+", name):
            if len(token) >= 4 and token in haystack:
                score += 1
            if token in target.casefold():
                score += 3
        scores[target] = score
        if score > best_score:
            best, best_score = target, score
    if best is None or best_score < 3:
        return None
    ranked = sorted(scores.values(), reverse=True)
    if len(ranked) > 1 and ranked[1] * 1.5 >= best_score:
        # Multi-theme hubs must be split by a human; appending them to one
        # theme would duplicate content that already lives in other files.
        return None
    return best


def collect_paragraphs(text: str) -> list[str]:
    out = []
    for block in re.split(r"\n\s*\n", text):
        block = block.strip()
        if not block or block == "---":
            continue
        if block.startswith("<!--") and block.endswith("-->"):
            continue
        if block.startswith("> Consolidated by scripts/consolidate-research.py"):
            continue
        out.append(block)
    return out


def update_status(readme: str) -> str:
    if MARKER_START not in readme or MARKER_END not in readme:
        return readme
    before, rest = readme.split(MARKER_START, 1)
    _, after = rest.split(MARKER_END, 1)
    block = (
        f"{MARKER_START}\n"
        f"Last consolidation: {TODAY}\n"
        f"Canonical theme files: {len(CANONICAL) - 1}\n"
        f"Pending incoming files: none\n"
        f"{MARKER_END}"
    )
    return before + block + after


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="write files and delete sources")
    parser.add_argument(
        "--map",
        action="append",
        default=[],
        metavar="FILE=TARGET.md",
        help="force the target theme for one incoming file (repeatable)",
    )
    args = parser.parse_args()

    missing = [name for name in CANONICAL if not (RESEARCH / name).is_file()]
    if missing:
        print("ERROR: missing canonical files:", ", ".join(missing), file=sys.stderr)
        return 2

    overrides: dict[str, str] = {}
    for item in args.map:
        if "=" not in item:
            print(f"ERROR: --map expects FILE=TARGET.md, got {item!r}", file=sys.stderr)
            return 2
        src, target = item.split("=", 1)
        if target not in CANONICAL or target == "README.md":
            print(f"ERROR: unknown canonical target {target!r}", file=sys.stderr)
            return 2
        overrides[src] = target

    texts = {name: (RESEARCH / name).read_text(encoding="utf-8") for name in CANONICAL}
    seen = {digest(p) for name, text in texts.items() for p in collect_paragraphs(text)}

    incoming = sorted(
        p
        for p in RESEARCH.iterdir()
        if p.is_file()
        and p.suffix.lower() == ".md"
        and p.name not in CANONICAL
        and not p.name.startswith(".")
    )
    other = sorted(p for p in RESEARCH.iterdir() if p.is_file() and p.suffix.lower() != ".md")

    consume: list[Path] = []
    plan: list[tuple[Path, str, int, int]] = []
    for path in incoming:
        text = path.read_text(encoding="utf-8")
        target = classify(path, text, overrides)
        if target is None:
            print(f"SKIP   {path.name}: no clear theme; use --map FILE=TARGET.md")
            continue
        unique = []
        skipped = 0
        for para in collect_paragraphs(text):
            key = digest(para)
            if key in seen:
                skipped += 1
            else:
                seen.add(key)
                unique.append(para)
        plan.append((path, target, len(unique), skipped))
        if unique:
            block = (
                f"\n---\n\n## Imported from `{path.name}` ({TODAY})\n\n"
                f"> Consolidated by scripts/consolidate-research.py. "
                f"Source file removed after ingestion.\n\n"
                + "\n\n".join(unique)
                + "\n"
            )
            texts[target] = texts[target].rstrip() + "\n" + block
        consume.append(path)

    action = "APPLY" if args.apply else "DRY-RUN"
    print(f"{action}: {len(plan)} file(s) classified, {len(consume)} will be deleted on --apply")
    for path, target, unique, skipped in plan:
        print(f"  {path.name:55s} -> {target:36s} +{unique:4d} unique / -{skipped:4d} duplicates")
    for path in other:
        print(f"WARN  non-Markdown file left untouched: {path.name}")

    if not args.apply:
        print("Dry run only; no files changed. Re-run with --apply to consolidate.")
        return 0

    for name, text in texts.items():
        if name == "README.md":
            text = update_status(text)
        (RESEARCH / name).write_text(text.rstrip() + "\n", encoding="utf-8")
    for path in consume:
        path.unlink()
    print(f"Consolidated {len(consume)} contribution file(s) into {len(CANONICAL) - 1} theme files.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

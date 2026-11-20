#!/usr/bin/env bash
# Seeded plate-solve acceptance harness for the blind-solver corpus.
# Usage: scripts/bench-seeded.sh [OUT_DIR]
# The M42 control uses the published centre from docs/blind-corpus-results.md.
set -uo pipefail

out="${1:-/tmp/opencode/seeded-corpus}"
mkdir -p "$out"
image="assets/Blind-Plate-Solving/M42.jpg"
ra=83.822
dec=-5.391
work="$out/m42"
rm -rf "$work"
log=$(skystacker --process-image "$image" \
  --image-op plate-solve --image-output "$work" \
  --plate-solver auto --plate-center "$ra,$dec" \
  --blind-detrend --gaia-offline 2>&1)
json="$work/M42_plate_solution.json"
if [[ ! -f "$json" ]]; then
  echo "M42: FAIL (no solution)"
  printf '%s\n' "$log" | tail -3
  exit 1
fi
python3 - "$json" "$ra" "$dec" <<'PY'
import json, math, sys
d = json.load(open(sys.argv[1]))
ra, dec = d['WCS']['CRVAL1'], d['WCS']['CRVAL2']
sep = math.degrees(math.acos(max(-1, min(1, math.sin(dec)*math.sin(math.radians(float(sys.argv[3]))) + math.cos(dec)*math.cos(math.radians(float(sys.argv[3])))*math.cos(ra-math.radians(float(sys.argv[2])))))))
ok = sep < 2 and d.get('RMSArcsec', 1e9) <= 5
print(f"M42: {'PASS' if ok else 'FAIL'} sep={sep:.4f}deg rms={d.get('RMSArcsec')} matches={d.get('NMatches')}")
sys.exit(0 if ok else 1)
PY

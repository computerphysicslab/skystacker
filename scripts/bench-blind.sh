#!/usr/bin/env bash
# Blind plate-solving corpus benchmark.
# Usage: scripts/bench-blind.sh [CORPUS_DIR] [CATALOG_ID] [OUT_DIR]
set -uo pipefail

corpus="${1:-assets/Blind-Plate-Solving}"
catalog="${2:-mag10deep}"
out="${3:-/tmp/opencode/blind-corpus}"
mkdir -p "$out"
result="$out/results.csv"
echo "image,success,method,rms_arcsec,matches,seconds,note" > "$result"

while IFS= read -r -d '' image; do
  base="$(basename "$image")"
  stem="${base%.*}"
  work="$out/$stem"
  rm -rf "$work"
  start=$(date +%s)
  log=$(timeout 90 skystacker --process-image "$image" \
    --image-op plate-solve --image-output "$work" \
    --plate-solver native --blind-catalog "$catalog" \
    --gaia-offline 2>&1)
  elapsed=$(( $(date +%s) - start ))
  json="$work/${stem}_plate_solution.json"
  if [[ -f "$json" ]]; then
    read -r method rms matches <<< "$(python3 - "$json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
print(d.get('Method',''), d.get('RMSArcsec',0), d.get('NMatches',0))
PY
)"
    echo "$base,true,$method,$rms,$matches,$elapsed," >> "$result"
    printf '%-55s OK   %-8s RMS=%8.2f N=%-4d %ss\n' "$base" "$method" "$rms" "$matches" "$elapsed"
  else
    note=$(printf '%s' "$log" | grep 'skystacker:' | tail -1 | tr ',' ' ' | cut -c1-80)
    echo "$base,false,,,,$elapsed,$note" >> "$result"
    printf '%-55s FAIL %ss %s\n' "$base" "$elapsed" "$note"
  fi
done < <(find "$corpus" -maxdepth 1 -type f \( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)

echo "Results: $result"

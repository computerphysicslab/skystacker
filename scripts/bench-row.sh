#!/usr/bin/env bash
# bench-row.sh — print (or append) a Markdown benchmark row from a SkyStacker
# report.txt. Usage:
#   ./scripts/bench-row.sh <path/to/report.txt> [--append [BENCHMARKS.md]]
set -euo pipefail

report="${1:-}"
[ -n "$report" ] || { echo "usage: $0 <report.txt> [--append [file]]" >&2; exit 2; }
[ -f "$report" ] || { echo "no such report: $report" >&2; exit 1; }
append="${2:-}"

md_escape() {
  # Escape pipes and newlines for Markdown table cells.
  printf '%s' "$1" | tr '\r\n' '  ' | sed 's/|/\\|/g'
}

field() {
  # field <label-prefix> — return the trimmed value after a label line.
  awk -v key="$1" '
    index($0, key) == 1 {
      sub(/^[^:]*:[[:space:]]*/, "", $0)
      print
      exit
    }
  ' "$report"
}

version="$(head -1 "$report" | awk '{print $2}')"
cmd="$(md_escape "$(awk '/^Command line:/{getline; print; exit}' "$report")")"
discovered="$(field 'Discovered files:')"
accepted="$(field 'Accepted:')"
dimensions="$(field 'Image dimensions:')"
duration="$(field 'Duration:' | sed 's/[[:space:]]*$//')"
method="$(field 'Method:' | sed 's/ *;.*//')"
registration="$(field 'Registration:')"
workers="$(field 'Workers:')"
cpu="$(lscpu 2>/dev/null | awk -F: '/^Model name/{gsub(/^ +/,"",$2); print $2; exit}')"
threads="$(nproc 2>/dev/null || echo '?')"
ram="$(free -h 2>/dev/null | awk '/^Mem:/{print $2" total / "$7" available"}')"

row="| $(md_escape "$cpu") ($threads threads, $ram) | $version | $method | $registration | $workers | $accepted/$discovered | $dimensions | $duration | \`$cmd\` |"

if [ "$append" = "--append" ]; then
  out="${3:-BENCHMARKS.md}"
  printf '%s\n' "$row" >> "$out"
  echo "appended row to $out"
else
  printf '%s\n' "$row"
fi

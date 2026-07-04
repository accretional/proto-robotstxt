#!/usr/bin/env bash
# profile.sh — CPU-profile the gluon robots.txt parser benchmarks and print
# the hot frames. This is the exact workflow that found gluon's O(n^2)
# loc() rescan (accretional/gluon#6 -> PR #7); see the "Profiling" section
# of bench/README.md for how to read the output, and gluon's PERF.md for the
# grammar-agnostic version of this harness.
#
# Usage:
#   bench/profile.sh                          # default: scaling bench @1000 lines
#   bench/profile.sh -b 'GluonParse'          # profile a different benchmark regex
#   bench/profile.sh -t 5x                    # override -benchtime (default 2x)
#   bench/profile.sh -l 'astParser.loc'       # also print pprof -list for a func
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BENCH_RE='GluonEventsScaling/lines=1000$'
BENCHTIME='2x'
LIST_FN=''

while getopts "b:t:l:h" opt; do
  case "$opt" in
    b) BENCH_RE="$OPTARG" ;;
    t) BENCHTIME="$OPTARG" ;;
    l) LIST_FN="$OPTARG" ;;
    h|*) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
  esac
done

out_dir="$(mktemp -d "${TMPDIR:-/tmp}/robotstxt-prof.XXXXXX")"
prof="$out_dir/cpu.prof"
bin="$out_dir/bench.test"

cd "$REPO_ROOT"
echo "[profile] benching '$BENCH_RE' (-benchtime $BENCHTIME) with CPU profile"
go test -run '^$' -bench "$BENCH_RE" -benchtime "$BENCHTIME" -benchmem \
  -cpuprofile "$prof" -o "$bin" ./bench/

echo
echo "[profile] top frames:"
go tool pprof -top -nodecount=15 "$bin" "$prof" 2>/dev/null | sed -n '4,24p'

if [ -n "$LIST_FN" ]; then
  echo
  echo "[profile] line-level cost of '$LIST_FN':"
  go tool pprof -list "$LIST_FN" "$bin" "$prof" 2>/dev/null | head -50
fi

echo
echo "[profile] explore interactively: go tool pprof $bin $prof"

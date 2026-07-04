#!/usr/bin/env bash
# bench/bench.sh — run the benchmark suite and record results.
#
# Runs the Go benchmarks in bench/ (no unit tests: -run '^$'), tees the raw
# output to gen/bench-latest.txt (gen/ is git-ignored), then prints a short
# ns/op comparison table per input.
#
# Usage: ./bench/bench.sh [extra go-test args...]
#   e.g. ./bench/bench.sh -count=5          # for use with benchstat
#        ./bench/bench.sh -benchtime=2s
#
# Prerequisites: ./build.sh (builds gen/bin/robots_dump; without it the
# BenchmarkGoogleDump rows are skipped, the gluon rows still run).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

mkdir -p gen
OUT="gen/bench-latest.txt"

# Generous -timeout: BenchmarkGluonEventsScaling/lines=10000 can take minutes
# while the engine has super-linear hotspots. Trailing "$@" still wins for any
# flag passed by the caller (go test uses the last occurrence).
echo "[bench] go test -bench . -benchmem -run '^\$' -timeout 60m ./bench/ $*"
go test -bench . -benchmem -run '^$' -timeout 60m ./bench/ "$@" | tee "${OUT}"

echo
echo "[bench] raw results saved to ${OUT}"

if ! grep -q '^Benchmark' "${OUT}"; then
  echo "[bench] no benchmark results found (everything skipped?) — nothing to summarize"
  exit 0
fi

echo "[bench] ns/op by input (GoogleDump includes subprocess spawn overhead):"
echo
awk '
/^Benchmark/ {
  name = $1
  sub(/-[0-9]+$/, "", name)          # strip -GOMAXPROCS suffix
  sub(/^Benchmark/, "", name)
  n = split(name, parts, "/")
  bench = parts[1]
  cs = (n > 1) ? parts[2] : "(none)"
  ns = ""; mbs = ""
  for (i = 2; i < NF; i++) {
    if ($(i + 1) == "ns/op") ns = $i
    if ($(i + 1) == "MB/s")  mbs = $i
  }
  if (ns == "") next
  cell = ns
  if (mbs != "") cell = ns " (" mbs " MB/s)"
  val[cs "|" bench] = cell
  if (!(cs in seenCase)) { seenCase[cs] = 1; caseOrder[++nCases] = cs }
  if (!(bench in seenBench)) { seenBench[bench] = 1; benchOrder[++nBenches] = bench }
}
END {
  fmtRow = "%-28s"
  printf fmtRow, "input"
  for (j = 1; j <= nBenches; j++) printf "  %-28s", benchOrder[j]
  printf "\n"
  printf fmtRow, "-----"
  for (j = 1; j <= nBenches; j++) printf "  %-28s", "-----"
  printf "\n"
  for (i = 1; i <= nCases; i++) {
    cs = caseOrder[i]
    printf fmtRow, cs
    for (j = 1; j <= nBenches; j++) {
      k = cs "|" benchOrder[j]
      printf "  %-28s", (k in val) ? val[k] : "-"
    }
    printf "\n"
  }
  printf "\n(values are ns/op; \"-\" = benchmark skipped or not applicable for that input)\n"
}
' "${OUT}"

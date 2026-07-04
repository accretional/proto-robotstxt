#!/usr/bin/env bash
# repin.sh — update this repo's github.com/accretional/gluon dependency and
# verify nothing broke. See tools/gluon/README.md for what we depend on and
# why. This replaces ad-hoc `go get` invocations so every gluon bump runs
# the same gauntlet.
#
# Usage:
#   tools/gluon/repin.sh            # pin latest gluon main
#   tools/gluon/repin.sh <ref>      # pin a specific commit/branch/tag
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REF="${1:-main}"
cd "${REPO_ROOT}"

log() { printf '\033[1;34m[gluon-repin]\033[0m %s\n' "$*"; }

before="$(grep 'github.com/accretional/gluon ' go.mod | awk '{print $2}')"
log "current pin: ${before}"
log "pinning github.com/accretional/gluon@${REF}"
go get "github.com/accretional/gluon@${REF}"
go mod tidy
after="$(grep 'github.com/accretional/gluon ' go.mod | awk '{print $2}')"
log "new pin: ${after}"

log "building + testing (unit, cross-parser conformance, fuzz seeds)"
go build ./...
go test ./src-gluon/ ./fuzz/ ./bench/

log "parse-scaling sanity (should be linear post gluon#6; ~10ms @1k lines)"
go test -run '^$' -bench 'GluonEventsScaling/lines=1000$' -benchtime=2x ./bench/ | grep lines= || true

log "done: ${before} -> ${after}"
log "next: ./run.sh (full gate) and record numbers via bench/bench.sh into"
log "      docs/progresslog/benchmarks.md if this bump was perf-relevant."

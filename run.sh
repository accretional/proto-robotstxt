#!/usr/bin/env bash
# run.sh — the end-to-end check. Calls test.sh by default (which calls
# build.sh, which calls setup.sh), then exercises the real binaries against a
# real robots.txt:
#   1. fetch https://accretional.com/robots.txt (falls back to the checked-in
#      copy in testdata/ when offline)
#   2. run the vendored google parser CLI (gen/bin/robots_main) on it
#   3. run our gluon-grammar CLI (gen/bin/gluon) on it
#   4. cross-check: both parsers must agree (gen/bin/gluon -check)
#
# CLAUDE.md rule: this script must succeed end-to-end before any git push.
#
# Usage:
#   ./run.sh                 # full: tests + e2e demo
#   ./run.sh --skip-tests    # just the e2e demo (assumes prior build)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${REPO_ROOT}"

log() { printf '\033[1;32m[run]\033[0m %s\n' "$*"; }

if [ "${1:-}" != "--skip-tests" ]; then
  "${REPO_ROOT}/test.sh"
else
  "${REPO_ROOT}/build.sh"
fi

# --- 1. get a real robots.txt -------------------------------------------------
mkdir -p gen
ROBOTS="gen/accretional-robots.txt"
if curl -fsSL --max-time 10 https://accretional.com/robots.txt -o "${ROBOTS}.tmp" 2>/dev/null; then
  mv "${ROBOTS}.tmp" "${ROBOTS}"
  log "fetched live https://accretional.com/robots.txt"
else
  cp testdata/accretional-robots.txt "${ROBOTS}"
  log "offline — using checked-in testdata/accretional-robots.txt"
fi
sed 's/^/    /' "${ROBOTS}"

# --- 2. google parser (vendored C++) -----------------------------------------
AGENT="${AGENT:-Googlebot}"
URL="${URL:-https://accretional.com/some/page}"
log "google robots_main: can ${AGENT} fetch ${URL}?"
# robots_main exits 0 (allowed) / 1 (disallowed); both are valid outcomes here.
set +e
gen/bin/robots_main "${ROBOTS}" "${AGENT}" "${URL}"
status=$?
set -e
case "${status}" in
  0) log "robots_main: ALLOWED" ;;
  1) log "robots_main: DISALLOWED" ;;
  *) echo "[run] robots_main failed with status ${status}" >&2; exit "${status}" ;;
esac

# --- 3. gluon grammar parser --------------------------------------------------
log "gluon typed rep (grammar/rep.ebnf -> proto/rep.proto shape):"
gen/bin/gluon -grammar grammar/rep.ebnf rep "${ROBOTS}" | sed 's/^/    /'
log "gluon events (google-deserialization form):"
gen/bin/gluon events "${ROBOTS}" | sed 's/^/    /'

# --- 4. cross-check both parsers agree ----------------------------------------
log "cross-checking gluon vs google parser on ${ROBOTS} + strict testdata/"
gen/bin/gluon check -dump gen/bin/robots_dump "${ROBOTS}" testdata/*.txt

log "e2e OK"

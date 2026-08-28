#!/usr/bin/env bash
# run.sh — the end-to-end check. Calls test.sh by default (which calls
# build.sh, which calls setup.sh), then exercises the real binaries against a
# real robots.txt:
#   1. fetch https://accretional.com/robots.txt (falls back to the checked-in
#      copy in testdata/ when offline)
#   2. run the vendored google parser CLI (gen/bin/robots_main) on it
#   3. run our gluon-grammar CLI (gen/bin/gluon) on it
#   4. cross-check: both parsers must agree (gen/bin/gluon -check)
#   5. two-tier recovery cross-check over both corpus tiers
#   6. exercise the robotstxt-svc gRPC service against the same file
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

# Same decision from our matcher port (gluon allowed mirrors robots_main's
# argument and exit-code contract).
set +e
gen/bin/gluon allowed "${ROBOTS}" "${AGENT}" "${URL}"
gluon_status=$?
set -e
if [ "${gluon_status}" != "${status}" ]; then
  echo "[run] MATCHER DIVERGENCE: robots_main=${status} gluon=${gluon_status}" >&2
  exit 1
fi
log "gluon matcher agrees with robots_main"

# --- 3. gluon grammar parser --------------------------------------------------
log "gluon typed rep (grammar/rep.ebnf -> proto/rep.proto shape):"
gen/bin/gluon -grammar grammar/rep.ebnf rep "${ROBOTS}" | sed 's/^/    /'
log "gluon events (google-deserialization form):"
gen/bin/gluon events "${ROBOTS}" | sed 's/^/    /'

# --- 4. cross-check both parsers agree ----------------------------------------
# The LIVE file goes through -recover (if accretional.com ever serves
# google-lenient-but-RFC-invalid content, the gate should not fail on it);
# the checked-in strict corpus stays on the strict path.
log "cross-checking gluon vs google parser (live file via -recover)"
gen/bin/gluon check -recover -dump gen/bin/robots_dump "${ROBOTS}"
log "cross-checking strict corpus (strict tier)"
gen/bin/gluon check -dump gen/bin/robots_dump testdata/*.txt

# --- 5. two-tier recovery cross-check (strict + malformed tiers) ---------------
log "cross-checking two-tier recovery (gluon check -recover) on BOTH corpus tiers"
gen/bin/gluon check -recover -dump gen/bin/robots_dump testdata/*.txt testdata/malformed/*.txt

# --- 6. robotstxt-svc: the gRPC service over the same parser -------------------
# The service is what the crawl pipeline calls, so the gate proves it starts,
# serves, and reaches the same verdict as the matcher above. It is served a
# local file rather than a live origin so this step stays offline-safe; the
# fetch path's status-code semantics are covered by cmd/robotstxt-svc's own
# tests, as is the gRPC surface itself (TestGRPCSurface).
log "robotstxt-svc: serving ${ROBOTS} to a local origin and querying the service"

if ! command -v grpcurl >/dev/null 2>&1; then
  log "installing grpcurl (needed to exercise the gRPC service)"
  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
  export PATH="$(go env GOPATH)/bin:${PATH}"
fi

# Serve it under the name the service will ask for: /robots.txt.
rm -rf gen/svc-origin && mkdir -p gen/svc-origin
cp "${ROBOTS}" gen/svc-origin/robots.txt

python3 -m http.server 8079 --directory gen/svc-origin >/dev/null 2>&1 &
origin_pid=$!
gen/bin/robotstxt-svc -addr :8078 >/dev/null 2>&1 &
svc_pid=$!
cleanup() { kill "${origin_pid}" "${svc_pid}" 2>/dev/null || true; }
trap cleanup EXIT

# Wait for the gRPC health service to report SERVING.
for _ in $(seq 1 40); do
  grpcurl -plaintext localhost:8078 grpc.health.v1.Health/Check >/dev/null 2>&1 && break
  sleep 0.25
done
grpcurl -plaintext localhost:8078 grpc.health.v1.Health/Check >/dev/null 2>&1 || {
  echo "[run] robotstxt-svc did not become healthy" >&2; exit 1; }

svc_filter=$(grpcurl -plaintext -d "$(python3 -c '
import json,sys
robots = open(sys.argv[1], "rb").read().decode("utf-8", "replace")
print(json.dumps({"robots_txt": robots, "agent": sys.argv[2], "urls": [sys.argv[3]]}))
' "${ROBOTS}" "${AGENT}" "${URL}")" \
  localhost:8078 robotstxt.svc.v1.RobotsService/Filter)

# 0 = allowed above; the service must put the URL in the matching bucket.
if [ "${status}" = "0" ]; then want="allowed"; else want="disallowed"; fi
if ! printf '%s' "${svc_filter}" | tr -d ' \n' | grep -q "\"${want}\":\[\"${URL}\"\]"; then
  echo "[run] SERVICE DIVERGENCE: robots_main said ${want}, service said: ${svc_filter}" >&2
  exit 1
fi
log "robotstxt-svc Filter agrees with robots_main (${want})"

svc_parse=$(grpcurl -plaintext -d "{\"domain\":\"http://localhost:8079\",\"agent\":\"${AGENT}\"}" \
  localhost:8078 robotstxt.svc.v1.RobotsService/Parse)
printf '%s\n' "${svc_parse}" | sed 's/^/    /'
printf '%s' "${svc_parse}" | grep -q 'FETCH_OUTCOME_SUCCESS' || {
  echo "[run] robotstxt-svc Parse did not succeed against the local origin" >&2; exit 1; }
log "robotstxt-svc Parse OK"

# Reflection must be registered: it is how grpcurl (and any operator) calls this
# service without a .proto on hand.
grpcurl -plaintext localhost:8078 list | grep -q 'robotstxt.svc.v1.RobotsService' || {
  echo "[run] robotstxt-svc is not advertising its service via reflection" >&2; exit 1; }
log "robotstxt-svc reflection OK"

cleanup
trap - EXIT

log "e2e OK"

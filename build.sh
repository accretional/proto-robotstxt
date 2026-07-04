#!/usr/bin/env bash
# build.sh — build everything. Calls setup.sh first (idempotent), then:
#   * bazel: vendored google/robotstxt C++ parser + robots_main CLI (src-google/)
#   * go:    gluon EBNF parser + cmd/ CLIs, installed into gen/bin/ (git-ignored)
# Called by test.sh (which is called by run.sh).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${REPO_ROOT}"

log() { printf '\033[1;34m[build]\033[0m %s\n' "$*"; }

"${REPO_ROOT}/setup.sh"

# Resolve a bazel launcher (PATH first, then repo-local ./bin from setup.sh).
BAZEL="$(command -v bazelisk || command -v bazel || true)"
[ -z "${BAZEL}" ] && [ -x "${REPO_ROOT}/bin/bazel" ] && BAZEL="${REPO_ROOT}/bin/bazel"
if [ -z "${BAZEL}" ]; then
  echo "[build] no bazel launcher found after setup.sh" >&2
  exit 1
fi

log "bazel build C++ targets"
"${BAZEL}" build //src-google:robots //src-google:reporting_robots \
  //src-google:robots_main //tools/robots-dump:robots_dump

# Copy the C++ CLIs into gen/bin so tools/tests have a stable, non-bazel path.
mkdir -p gen/bin
for tgt in //src-google:robots_main //tools/robots-dump:robots_dump; do
  out="$(${BAZEL} cquery --output=files "${tgt}" 2>/dev/null | tail -1)"
  cp -f "${out}" "gen/bin/$(basename "${out}")"
  chmod +wx "gen/bin/$(basename "${out}")"
done

log "go build ./..."
go build ./...
go build -o gen/bin/ ./cmd/...

log "build complete (binaries in gen/bin/)"

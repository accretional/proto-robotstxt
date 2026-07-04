#!/usr/bin/env bash
# test.sh — run all tests. Calls build.sh first (which calls setup.sh), then:
#   * bazel: upstream robots_test + reporting_robots_test (src-google/)
#   * go:    unit + cross-parser conformance tests (src-gluon/, cmd/)
# Called by run.sh.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${REPO_ROOT}"

log() { printf '\033[1;34m[test]\033[0m %s\n' "$*"; }

"${REPO_ROOT}/build.sh"

BAZEL="$(command -v bazelisk || command -v bazel || true)"
[ -z "${BAZEL}" ] && [ -x "${REPO_ROOT}/bin/bazel" ] && BAZEL="${REPO_ROOT}/bin/bazel"

log "bazel test //src-google:all tests"
"${BAZEL}" test --test_output=errors //src-google:robots_test //src-google:reporting_robots_test

log "go test ./..."
go test ./...

log "all tests passed"

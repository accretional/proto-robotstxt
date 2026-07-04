#!/usr/bin/env bash
# pull-rfc.sh — fetch RFC 9309 (Robots Exclusion Protocol) from rfc-editor.org
# into docs/rfc/9309/. The raw copies are GIT-IGNORED by default because of the
# RFC Editor's licensing (IETF Trust Legal Provisions); run this script to
# (re)materialize them locally. Our own summary of the important parts is
# checked in at docs/rfc/9309/README.md, and the extracted grammar lives at
# grammar/rep.ebnf.
#
# Usage: tools/rfc/pull-rfc.sh [rfc-number]   (default: 9309)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RFC="${1:-9309}"
OUT_DIR="${REPO_ROOT}/docs/rfc/${RFC}"
mkdir -p "${OUT_DIR}"

echo "[pull-rfc] fetching RFC ${RFC} -> ${OUT_DIR}"
curl -fsSL "https://www.rfc-editor.org/rfc/rfc${RFC}.html" -o "${OUT_DIR}/raw.html"
curl -fsSL "https://www.rfc-editor.org/rfc/rfc${RFC}.txt" -o "${OUT_DIR}/raw.txt"

echo "[pull-rfc] done:"
ls -l "${OUT_DIR}"/raw.*

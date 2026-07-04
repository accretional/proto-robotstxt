#!/usr/bin/env bash
# setup.sh — idempotently provision the toolchains this repo needs:
#   * bazel (via bazelisk) for the vendored C++ google/robotstxt parser
#     (src-google/). Version pinned by .bazelversion.
#   * go for the gluon-based EBNF parser (src-gluon/, cmd/).
#
# Re-runnable: if everything is already present this is a fast no-op.
# Called by build.sh (which is called by test.sh, which is called by run.sh).
#
# Strategy for bazel, in order of preference (mirrors kvq/scripts/setup-bazel.sh):
#   1. bazel or bazelisk already on PATH            -> done.
#   2. Homebrew available                           -> brew install bazelisk.
#   3. Otherwise                                    -> download the bazelisk
#      release binary into ./bin (repo-local, git-ignored).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_BIN="${REPO_ROOT}/bin"
BAZELISK_VERSION="${BAZELISK_VERSION:-v1.25.0}"

log()  { printf '\033[1;34m[setup]\033[0m %s\n' "$*"; }
err()  { printf '\033[1;31m[setup]\033[0m %s\n' "$*" >&2; }

have_bazel() {
  command -v bazelisk >/dev/null 2>&1 && return 0
  command -v bazel >/dev/null 2>&1 && return 0
  [ -x "${LOCAL_BIN}/bazel" ] && return 0
  return 1
}

download_bazelisk() {
  local os arch url out
  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux)  os="linux" ;;
    *) err "Unsupported OS $(uname -s); install bazelisk manually."; return 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch="arm64" ;;
    x86_64|amd64)  arch="amd64" ;;
    *) err "Unsupported arch $(uname -m); install bazelisk manually."; return 1 ;;
  esac
  url="https://github.com/bazelbuild/bazelisk/releases/download/${BAZELISK_VERSION}/bazelisk-${os}-${arch}"
  mkdir -p "${LOCAL_BIN}"
  out="${LOCAL_BIN}/bazelisk"
  log "Downloading bazelisk ${BAZELISK_VERSION} -> ${out}"
  curl -fSL "${url}" -o "${out}"
  chmod +x "${out}"
  ln -sf bazelisk "${LOCAL_BIN}/bazel"
  log "Installed. Scripts in this repo find it automatically; for your shell:"
  log "    export PATH=\"${LOCAL_BIN}:\$PATH\""
}

ensure_bazel() {
  if have_bazel; then
    log "bazel launcher present: $(command -v bazelisk || command -v bazel || echo "${LOCAL_BIN}/bazel")"
    return 0
  fi
  if command -v brew >/dev/null 2>&1; then
    log "Installing bazelisk via Homebrew"
    brew install bazelisk
    return 0
  fi
  download_bazelisk
}

ensure_go() {
  if command -v go >/dev/null 2>&1; then
    log "go present: $(go version)"
  else
    err "go toolchain not found. Install Go >= 1.26 (https://go.dev/dl/ or 'brew install go')."
    err "The C++ (src-google) build works without it, but src-gluon/cmd do not."
    return 1
  fi
  # Warm the module cache so build.sh works offline afterwards.
  ( cd "${REPO_ROOT}" && go mod download ) || true
}

main() {
  ensure_bazel
  ensure_go
  log "setup complete"
}

main "$@"

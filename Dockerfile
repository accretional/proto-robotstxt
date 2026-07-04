# syntax=docker/dockerfile:1.7
#
# proto-robotstxt — reproducible Linux container that builds and e2e-tests the
# whole repo (vendored google/robotstxt C++ parser via Bazel + the Go/gluon
# EBNF parser). See docker/README.md for usage.
#
# Layering rationale (instead of a single `COPY . . && RUN ./run.sh`):
#   1. toolchains        — apt + Go + bazelisk; changes ~never.
#   2. pinned Bazel      — `bazel version` downloads the .bazelversion Bazel;
#                          invalidated only when .bazelversion/MODULE.* move.
#   3. C++ build + test  — src-google only; caches the expensive abseil/gtest
#                          compile and keeps the C++ half verifiable on its own
#                          even while the Go half is in flux.
#   4. full repo + gate  — COPY . . then ./run.sh (the repo's e2e gate).
#                          Skippable with --build-arg RUN_E2E=0.
#
# Multi-arch: linux/amd64 and linux/arm64 via TARGETARCH (bazelisk binary) and
# the multi-arch golang/ubuntu base images.

ARG GO_VERSION=1.26

# Toolchain donor only: /usr/local/go is copied out below. Using the official
# image avoids hardcoding a go1.26.x patch release URL from go.dev/dl.
FROM golang:${GO_VERSION} AS go-dist

FROM ubuntu:24.04 AS build

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        python3 \
    && rm -rf /var/lib/apt/lists/*

# Go >= 1.26 (Ubuntu's golang-go is too old for this module).
COPY --from=go-dist /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH

# bazelisk launcher; it downloads the Bazel release pinned by .bazelversion.
ARG TARGETARCH
ARG BAZELISK_VERSION=v1.25.0
RUN curl -fsSL "https://github.com/bazelbuild/bazelisk/releases/download/${BAZELISK_VERSION}/bazelisk-linux-${TARGETARCH}" \
        -o /usr/local/bin/bazelisk \
    && chmod +x /usr/local/bin/bazelisk \
    && ln -s bazelisk /usr/local/bin/bazel

WORKDIR /work

# --- layer: pinned Bazel itself (re-downloads only when the pin changes) ----
COPY .bazelversion MODULE.bazel MODULE.bazel.lock ./
RUN bazel version

# --- layer: vendored C++ parser — build + upstream tests --------------------
# Mirrors the bazel half of build.sh/test.sh so the C++ side is verified (and
# its abseil/googletest deps compiled + cached) independently of the Go side.
COPY src-google/ src-google/
RUN bazel build //src-google:robots //src-google:reporting_robots //src-google:robots_main \
    && bazel test --test_output=errors //src-google:robots_test //src-google:reporting_robots_test

# --- layer: Go module downloads (cache separately from source edits) --------
COPY go.mod go.sum ./
RUN go mod download

# --- layer: full repo + e2e gate ---------------------------------------------
COPY . .
ARG RUN_E2E=1
RUN if [ "${RUN_E2E}" = "1" ]; then \
        ./run.sh; \
    else \
        echo "RUN_E2E=0 — skipping ./run.sh build-time gate (toolchain-only image)"; \
    fi

# `docker run --rm proto-robotstxt` re-runs the full e2e (fast: Bazel disk
# cache and Go module cache are baked into the image).
CMD ["./run.sh"]

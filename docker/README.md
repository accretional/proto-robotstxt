# Docker: reproducible Linux build + e2e-test container

The root [`Dockerfile`](../Dockerfile) produces a self-contained Linux builder
image with everything this repo needs — gcc/g++, git, curl, python3,
Go (>= 1.26, from the official `golang` image), and bazelisk (which downloads
the Bazel pinned by `.bazelversion`) — then copies the repo in and runs
`./run.sh`, the repo's end-to-end gate, as the final build step.

It is a **builder/dev image**, not a slim runtime image: the Bazel disk cache
and Go module cache are deliberately baked in so that re-running tests inside
a container is fast.

## Build

```sh
docker build -t proto-robotstxt .
```

A successful build means `./run.sh` (tests + e2e demo) passed on Linux.

Options (build ARGs):

| ARG                | Default   | Meaning                                                        |
|--------------------|-----------|----------------------------------------------------------------|
| `RUN_E2E`          | `1`       | Run `./run.sh` as the final build step. `0` skips it (toolchain + C++-only image; see status below). |
| `GO_VERSION`       | `1.26`    | Tag of the official `golang` image used as the Go toolchain donor. |
| `BAZELISK_VERSION` | `v1.25.0` | bazelisk release to install (Bazel itself is pinned by `.bazelversion`). |

Multi-arch: the Dockerfile respects `TARGETARCH`, so both work natively:

```sh
docker buildx build --platform linux/amd64 -t proto-robotstxt:amd64 .
docker buildx build --platform linux/arm64 -t proto-robotstxt:arm64 .
```

## Run the e2e

The default command is `./run.sh`:

```sh
docker run --rm proto-robotstxt
```

This rebuilds/retests (cheap — caches are in the image), fetches
`https://accretional.com/robots.txt` (falling back to
`testdata/accretional-robots.txt` when offline), runs the vendored google
parser and the gluon CLI on it, and cross-checks both parsers.

## One-off commands

Everything lives under `/work` inside the image; built binaries are in
`gen/bin/` (present when the image was built with the default `RUN_E2E=1`):

```sh
# google C++ parser CLI: robots_main <robots.txt> <user-agent> <url>
docker run --rm proto-robotstxt \
  gen/bin/robots_main testdata/accretional-robots.txt Googlebot https://accretional.com/x

# gluon CLI
docker run --rm proto-robotstxt \
  gen/bin/gluon -grammar grammar/rep.ebnf parse testdata/accretional-robots.txt

# interactive shell
docker run --rm -it proto-robotstxt bash

# hack on your working tree with the container's toolchain
docker run --rm -it -v "$PWD":/work proto-robotstxt bash
```

Note on `-v "$PWD":/work`: mounting your checkout hides the image's `/work`
(including the baked `gen/bin/`), so run `./build.sh` first inside the
container; Bazel/Go caches under `/root` still apply, and Linux build outputs
land in your mounted `gen/` and Bazel's in-container cache.

## Expected build time / size

Measured on an arm64 host (Docker Desktop 29, macOS, fast network), cold
cache:

* base image pulls (ubuntu + golang) : ~15 s
* apt + Go copy + bazelisk           : ~17 s
* `bazel version` (downloads 9.1.1)  : ~3 s
* C++ build + upstream tests
  (abseil + googletest from source)  : ~42 s
* `go mod download` + `./run.sh`     : ~30 s

Total cold build: **~2 min** (network-dominated; budget ~5 min on slow CI
egress). Warm rebuilds after a source-only change re-run only the last
layer(s). Image size: **~1.75 GB** (Ubuntu + Go toolchain + Bazel + caches) —
expected for a builder image. Build context is ~300 kB thanks to
`.dockerignore`.

## CI notes

* `docker build -t proto-robotstxt .` **is** the CI gate: it fails iff
  `./run.sh` fails. No host toolchain needed beyond docker/buildx.
* Build steps have network access under BuildKit by default; the image needs
  it to download Bazel/modules. `run.sh`'s live-fetch of accretional.com
  degrades gracefully to checked-in testdata, so airgapped `docker run` still
  passes once the image is built.
* For speed, persist the BuildKit layer cache between CI runs (e.g.
  `--cache-to/--cache-from type=gha` or a registry cache). The expensive
  layers (toolchains, Bazel download, abseil/gtest compile) only invalidate
  when `.bazelversion`, `MODULE.bazel{,.lock}`, or `src-google/` change.
* Multi-arch publishing: `docker buildx build --platform
  linux/amd64,linux/arm64` works; cross-arch emulation of the Bazel/C++ build
  is slow — prefer native runners per arch.

## Current status (2026-07-03)

The Go/gluon half of the repo (`src-gluon/`, `cmd/gluon`) is still being
written by another workstream, so the default build currently fails at the
final `./run.sh` layer inside that Go code (at the time of writing: compile
errors in `cmd/gluon/main.go` against not-yet-written `src-gluon` symbols).
Everything before that — toolchains, `go mod download` of the gluon module,
and the C++ parser build with its upstream Bazel tests (2/2 pass) — is
verified green. In the meantime use:

```sh
docker build --build-arg RUN_E2E=0 -t proto-robotstxt .
```

to get the verified toolchain + C++ image (smoke-tested: `robots_main`
built and run inside the container against `testdata/`). Remove this section
(and the `RUN_E2E=0` advice) when the default build goes green.

# Progress log: docker (reproducible Linux build + e2e container)

Owner scope: `Dockerfile`, `.dockerignore`, `docker/**`, this file.

## 2026-07-03

### What was done

* **`Dockerfile`** (repo root): single builder image on `ubuntu:24.04` with
  build-essential, git, curl, ca-certificates, python3; Go >= 1.26 copied from
  the official multi-arch `golang:${GO_VERSION}` image (donor stage — Ubuntu's
  distro Go is too old); bazelisk `${BAZELISK_VERSION}` (default v1.25.0)
  downloaded per `TARGETARCH` (amd64/arm64), which in turn downloads the Bazel
  pinned by `.bazelversion` (9.1.1). Tool versions are build ARGs; apt is
  non-interactive.
* Layered for caching + partial verifiability rather than a single
  `COPY . . && RUN ./run.sh`:
  1. toolchains (apt + Go + bazelisk)
  2. `.bazelversion` + `MODULE.bazel{,.lock}` → `bazel version` (downloads the
     pinned Bazel; invalidates only on pin changes)
  3. `src-google/` → `bazel build` of `:robots`, `:reporting_robots`,
     `:robots_main` + `bazel test` of both upstream test targets (mirrors the
     bazel half of build.sh/test.sh; caches the abseil/googletest compile)
  4. `go.mod` + `go.sum` → `go mod download` (gluon module cache, separate
     from source edits)
  5. `COPY . .` → `./run.sh` (the repo e2e gate), skippable with
     `--build-arg RUN_E2E=0`
* `CMD ["./run.sh"]` so `docker run --rm proto-robotstxt` re-runs the full
  e2e using the baked-in Bazel/Go caches.
* **`.dockerignore`**: excludes `.git`, `bazel-*`, `bin/`, `gen/**` (keeps
  `gen/.gitkeep`), `docs/google-dev-docs/rawhtml/`, `docs/rfc/*/raw.*`,
  docker files themselves, and misc junk. Build context: ~277 kB.
* **`docker/README.md`**: build/run/one-off usage, ARG table, multi-arch
  notes, measured times, CI notes, current-status section.
* Followed the house patterns from `kvq/docker/Dockerfile.cpp.bazel`
  (debian-family base, TARGETARCH-pinned bazelisk, deps-vs-source layering)
  but kept this repo's version minimal: one image, no distroless runtime
  stage — the deliverable here is a build/e2e-test environment, not a
  shipping binary.

### Verification (arm64 host, Docker Desktop 29, macOS)

* `docker build --build-arg RUN_E2E=0 -t proto-robotstxt .` — **passes**.
  All toolchain + C++ layers green; upstream Bazel tests
  `//src-google:robots_test` and `//src-google:reporting_robots_test`
  **2/2 PASSED** inside the image.
* In-container smoke test — **passes**:
  `robots_main testdata/accretional-robots.txt Googlebot
  https://accretional.com/some/page` → `ALLOWED` (exit 0);
  `go version` → go1.26.4 linux/arm64; `bazel --version` → 9.1.1.
* `.dockerignore` verified inside the image: `gen/` contains only `.gitkeep`;
  `docs/rfc/9309/` has only its README (no `raw.*`); no `rawhtml/`, `.git`,
  or `bin/`.
* `docker build -t proto-robotstxt .` (default `RUN_E2E=1`) — **fails at the
  final `./run.sh` layer only, as expected**: the Go/gluon workstream is
  mid-flight. It progressed under me across three build attempts — (1) missing
  `go.sum` entry for `github.com/accretional/proto-expr`; (2) src-gluon
  compiled clean but `cmd/gluon/` was empty (`"./cmd/..." matched no
  packages`); (3) `cmd/gluon/main.go` landed but references src-gluon symbols
  that don't exist yet (`robotsgluon.GrammarDescriptor`, `.Genproto`,
  `.GenprotoOptions`). All my layers cache and pass each time; only the gate
  layer fails, in their code. Not touched — owned by the other agent; the
  Dockerfile needs no changes to go green once their code compiles.
* Image size: **1.75 GB** (builder image: Ubuntu + Go toolchain + Bazel +
  build caches).
* Cold build times (fast network): base pulls ~15 s, apt+Go+bazelisk ~17 s,
  Bazel 9.1.1 download ~3 s, C++ build+tests ~42 s, go mod download ~2 s,
  run.sh layer ~30 s → **~2 min total**. Warm rebuild after source-only
  change: only the last layers re-run (seconds + run.sh time).

### Open items

* Re-run the default `docker build` once `cmd/gluon` lands; expected green
  with no Dockerfile changes. Then remove the "Current status" section and
  `RUN_E2E=0` advice from `docker/README.md`.
* Not verified on linux/amd64 (this host is arm64). The Dockerfile is
  TARGETARCH-clean; confirm via `docker buildx build --platform linux/amd64`
  on a native amd64 runner (emulated C++ builds are slow).
* `GO_VERSION=1.26` floats to the latest 1.26.x patch of the `golang` image
  (currently 1.26.4). If tighter pinning is wanted later, pin the full tag or
  digest.
* If CI wants a slimmer artifact later, add a second stage copying only
  `gen/bin/*` onto a small runtime base; intentionally out of scope now.

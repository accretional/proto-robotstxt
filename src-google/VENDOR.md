# Vendored: google/robotstxt

- Upstream: https://github.com/google/robotstxt
- Commit: `22b355ff855419e6a3ff8ff09c0ad7fdb17116f9` (2026-04-01, "Merge pull request #83 from AVGP:wasm")
- License: Apache-2.0 (see `LICENSE` in this directory)

## Local changes vs upstream

- `MODULE.bazel` removed — its `bazel_dep`s live in the repo-root
  `MODULE.bazel` instead (Bazel only reads the root module file). The `emsdk`
  dep was intentionally not carried over.
- `BUILD` — dropped the `robots_js`/`robots_wasm` (emscripten/WASM) targets and
  the `@emsdk` load; everything else matches upstream. `robots_wasm.cc` is
  still vendored so a future re-diff against upstream stays clean.
- No changes to any `.cc`/`.h` source file. Keep it that way: if parser
  behavior needs to change, do it in our own code (src-gluon/, cmd/), not in
  the vendored tree, so upstream updates stay a clean re-copy.

## How to update

```sh
git clone --depth 1 https://github.com/google/robotstxt /tmp/robotstxt
rsync -a --exclude .git --exclude MODULE.bazel --exclude BUILD /tmp/robotstxt/ src-google/
# then diff upstream BUILD/MODULE.bazel by hand against src-google/BUILD and
# the root MODULE.bazel, and update this file's commit hash.
```

# fuzz/

Fuzzing harnesses for the gluon-grammar robots.txt parser (src-gluon/).

## Harnesses

```sh
go test ./fuzz/                                    # replay corpus + seeds (runs in test.sh)
go test -fuzz=FuzzParse        -fuzztime=60s ./fuzz/
go test -fuzz=FuzzRecover      -fuzztime=60s ./fuzz/
go test -fuzz=FuzzDifferential -fuzztime=300s ./fuzz/   # needs ./build.sh first
```

| fuzzer | invariants | speed (M4) |
|---|---|---|
| `FuzzParse` | strict parser never panics/hangs; every accepted input lowers cleanly to the typed rep AND to ordered events | ~16k execs/s |
| `FuzzRecover` | the two-tier parse is **total** (any bytes → a result); metadata records consecutive from 1; tier 2 never shadows tier 1 (strict-accepted input ⇒ identical events) | ~2k execs/s |
| `FuzzDifferential` | **the phase-5 gate** (docs/design/malformed-input.md): for arbitrary bytes, recovery events AND per-line metadata are byte-identical to google's parser (real `robots_dump` subprocess per exec) | ~200 execs/s |

Any `FuzzDifferential` failure is a real finding: either a bug in our
grammar/recovery or an undocumented google leniency. Triage the minimized
crasher, fix or fold the behavior into the recovery layer, and graduate the
input into `testdata/malformed/` so it stays covered forever.

Found crashers land in `testdata/fuzz/<FuzzName>/` (Go's default corpus dir
for this package); check them in after triage so regressions stay covered.

Seeds come from `testdata/` (both tiers). All fuzzers skip gracefully when
their prerequisites are missing.

## Next: structure-aware mutation (docs/TODO.md item 2)

Byte-level mutation reaches shallow paths quickly but deep grammar shapes
slowly. The planned upgrade — something like
[google/libprotobuf-mutator](https://github.com/google/libprotobuf-mutator)
with libFuzzer — mutates `robotstxt.rep.Robotstxt` messages
(proto/rep.proto) instead of bytes, renders each mutant to robots.txt text,
and feeds the SAME differential check as `FuzzDifferential`.

Dependency: the rep → text renderer (docs/TODO.md item 5, the
"generating robots.txt files" tooling) does not exist yet. Once it does,
the cheap version is a pure-Go structured fuzzer (mutate the dynamicpb rep,
render, reuse `FuzzDifferential`'s body); the libFuzzer/C++ version
(cc_fuzz target on `//src-google:robots` + `@libprotobuf_mutator` from BCR)
buys coverage-guided C++-side feedback on top.

# fuzz/

Fuzzing harnesses for the gluon-grammar robots.txt parser (src-gluon/).

## Now: Go-native fuzzing

```sh
go test ./fuzz/                 # replay corpus + seeds (runs in test.sh)
go test -fuzz=FuzzParse -fuzztime=60s ./fuzz/
```

`FuzzParse` feeds arbitrary bytes to the strict parser and asserts: no
panics/hangs; every input the grammar accepts also lowers cleanly to the
typed rep (proto/rep.proto) and to google-form events with sane, ordered
line numbers. Seeds come from `testdata/` (both tiers).

Found crashers land in `testdata/fuzz/FuzzParse/` (Go's default corpus dir
for this package); check them in after triage so regressions stay covered.

## Next: structure-aware differential fuzzing (docs/TODO.md)

The plan from the project README is to use something like
[google/libprotobuf-mutator](https://github.com/google/libprotobuf-mutator)
with libFuzzer:

1. mutate `robotstxt.rep.Robotstxt` messages (proto/rep.proto) rather than
   raw bytes — structure-aware mutation reaches deep grammar paths that
   byte-flipping cannot;
2. render each mutant to robots.txt text (needs the rep -> text renderer,
   also a TODO — the "generating robots.txt files" tooling);
3. run BOTH parsers on the rendered text and diff:
   gluon events (src-gluon) vs google's (tools/robots-dump handler stream),
   plus RobotsMatcher allow/disallow decisions once the matcher lands;
4. any divergence on grammar-valid input is a bug: either in our grammar
   formalization or a documented google-parser leniency to fold into the
   malformed-input layer.

The C++ side would live here (cc_fuzz target depending on
`//src-google:robots` and `@libprotobuf_mutator`, driven by the checked-in
`proto/rep.proto`); Bazel Central Registry has libprotobuf-mutator, so the
dep is one `bazel_dep` away when we pick this up.

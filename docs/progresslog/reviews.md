# reviews — google-docs aggregation + three-way review sweep

## 2026-07-04 — full docs pull, then docs-accuracy / port-fidelity / gaps reviews

Four parallel agents: one writer (full Google-docs aggregation, its own log:
[google-docs-aggregation.md](google-docs-aggregation.md)) and three
read-only reviewers over the docs and codebase. Everything below was
triaged and fixed the same day unless marked TODO.

### Port-fidelity review (the headline): 2 real parity bugs found & fixed

An adversarial line-by-line comparison of the Go ports against
src-google/robots.cc, with ~130 hand-built differential probes, found two
CONFIRMED divergences the fuzzers had never hit (both in hand-ported glue,
neither in the grammar-driven core):

1. **Missing-colon value kept leading `\v`/`\f`** (metadata.go): robots.cc
   uses kWhite (`" \t"`) for the two-token separator CHECK but re-trims the
   EMITTED value with the full absl whitespace set. Repro: `a \vb` →
   google `UNKNOWN "a" "b"`, ours kept `"\vb"`; also flipped a
   DISALLOW decision. Fixed (value recomputed with the full set);
   regression-pinned by `testdata/malformed/vertical-tab.txt` (in the
   recovery cross-check gate), `TestParseGoogleLine` cases, and a fuzz
   seed carrying `\v`/`\f`. Why fuzzers missed it: the corpus contained no
   `\v`/`\f` byte anywhere, and the needle is a 4-byte-class conjunction.
2. **Agent matching used Unicode case folding** (matcher.go):
   `strings.EqualFold` folds U+212A KELVIN SIGN ↔ `k` etc.;
   `absl::EqualsIgnoreCase` is byte-wise ASCII-only. An exotic caller
   agent could match a group google never matches. Fixed with
   `asciiEqualFold`; pinned by `TestAgentFoldIsASCIIOnly`.

Plus one CLI parity gap fixed (`gluon allowed` now exits 2 on file/arg
errors and prints the empty-file notice, matching robots_main), and two
THEORETICAL divergences documented in code comments rather than "fixed":
C `isspace()` locale sensitivity in the `'*'`-agent rule (robots_main never
changes locale) and NUL-in-URL truncation for library callers (unreachable
via argv). The review also verified-faithful: escapePattern bounds/NUL
semantics, BOM quirks, the 16663-byte cap boundary, NUL/`#`/`:` ordering,
typo-flag attribution, the pos-array matcher (mid-`$`, leading-`*`, empty
cases), GetPathParamsQuery's npos cases, and the ABNF fidelity of
grammar/rep.ebnf incl. the UTF-8 second-byte constraints. Unmirrored
google surface (reporting_robots, multi-agent parity coverage,
kAllowFrequentTypos=false, IsValidUserAgentToObey) → TODO item 8.

### Docs-accuracy review: 16 findings, all applied

Every load-bearing technical claim verified correct (RFC quotes, constants,
counts, CLI behavior); staleness was concentrated in status lines written
mid-flight. Fixed: docker/README's obsolete "default build fails" section
(it is green; re-timed), malformed-input.md's contradictory status line,
testdata/README's malformed-tier definition (4 of 12 files actually parse
strictly via Normalize/otherline), CLAUDE.md's CLI list + rule 5/6,
src-gluon/README's retired-pin reference and "hard-coded Rules[0]"
phrasing, bench/README's missing recover-benchmark rows, the RFC README's
500 KiB parenthetical (fetch policy, not parser behavior) and the RFC §5.2
example typo (`disallow.gif` vs `disallowed.gif`), README's CLI list and
the ABNF anchor (#name-formal-syntax), and superseded-name notes in two
progress logs.

### Gaps review: wiring hardened

- **CI added** (.github/workflows/ci.yml): gate job runs `./run.sh`
  (bazel + Go caches); fuzz job runs FUZZ_SMOKE=1 test.sh (30s/fuzzer on
  push/PR, 5m nightly) + `go test -race`. run.sh hardened: the LIVE
  accretional.com file is now checked via `-recover` so upstream content
  changes can't fail unrelated PRs.
- **test.sh** gained the env-gated fuzz smoke (FUZZ_SMOKE/FUZZ_TIME).
- **.gitignore** `gen/` → `gen/*` + `!gen/.gitkeep` (negation inside an
  ignored DIRECTORY never applies; .gitkeep is now actually tracked).
- **MODULE.bazel.lock** regenerated — `--lockfile_mode=error` now passes.
- **TestMatcherGridVsGoogle** parallelized (per-file subtests + 8-way
  triple shards): 21.5s → 7.4s, same 3,180 triples.
- **TestConcurrentGrammarUse** added (shared *Grammar across goroutines;
  race-clean; CI runs it under -race) — guards the concurrency invariant
  against gluon bumps.
- Verified green by the reviewer: default `docker build` (full in-image
  run.sh gate), `go test -race`, bench suite, `go mod tidy -diff`,
  genproto-vs-checked-in-proto sync, protoc-free build path.
- New TODO items: tier-2 render-back scope decision (item 7), unmirrored
  surface (item 8).

### Assessment

The bijectivity claim survived its first adversarial audit with two
narrow, now-fixed exceptions — both in robots.cc-mirroring glue code, both
now regression-pinned in the corpus and unit tests. The differential-fuzz
+ corpus-gate architecture did exactly what it should: the reviewers'
hand-built probes became permanent corpus/test entries the moment they
found anything.

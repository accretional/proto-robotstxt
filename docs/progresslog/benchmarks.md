# Progress log: benchmarks

Task-specific log for the benchmark suite in `bench/`. See `bench/README.md`
for full methodology and interpretation notes.

## 2026-07-03 — suite authored (pre-integration)

Authored the benchmark suite against the agreed `robotsgluon` API contract
(`Default`, `Grammar.Parse`, `Grammar.Events`, `Event`/`EventKind`,
`GoogleEvents`) while the core `src-gluon` package was still being written.
The code was first compile-checked against a stub implementation of the
contract; the real package then landed in the working tree mid-task, so the
full suite was also vetted and run for real — first results and findings are
recorded at the bottom of this file.

Files written:

- `bench/bench_test.go` — `package bench`, four benchmarks:
  - `BenchmarkGluonParse` — `Grammar.Parse` (full CST) per `testdata/*.txt`
    file, one sub-benchmark per file, `b.SetBytes` for MB/s.
  - `BenchmarkGluonEvents` — `Grammar.Events` likewise (the fair comparison
    path vs. Google's parser).
  - `BenchmarkGoogleDump` — one `gen/bin/robots_dump` subprocess per
    iteration via `robotsgluon.GoogleEvents`; documented as including
    fork/exec + pipe overhead; skipped under `-short` or when the binary is
    missing.
  - `BenchmarkGluonEventsScaling` — synthetic in-memory inputs
    (`lines=100|1000|10000`, groups of 1 user-agent + 9 rules) to expose
    scaling shape independent of the corpus.

  Robustness properties: corpus is globbed at runtime by walking up from the
  test's working directory to the dir containing `go.mod`; grammar compiled
  once per process via `sync.Once`; all missing inputs (`testdata` empty,
  grammar load failure, `robots_dump` absent) degrade to `b.Skipf`, never a
  hard failure. Only imports: stdlib + `robotsgluon`.

- `bench/bench.sh` — executable driver. From the repo root runs
  `go test -bench . -benchmem -run '^$' ./bench/ | tee gen/bench-latest.txt`
  (creates `gen/`, which is git-ignored), then prints an awk-generated
  per-input ns/op (+MB/s) comparison table. Extra args are forwarded to
  `go test` (e.g. `-count=5` for benchstat).

- `bench/README.md` — what each benchmark measures, how to run, methodology
  caveats (subprocess overhead on the C++ side; CST parsing expected to be
  orders of magnitude slower than Google's hand-rolled scanner — the signal
  is scaling shape, events-vs-parse overhead, and allocs/op), and how to
  read the output.

## How to run once the core lands

```sh
./build.sh          # builds gen/bin/robots_dump + robots_main (optional for gluon-only rows)
./bench/bench.sh    # full run; raw output -> gen/bench-latest.txt + summary table
```

Gluon-only quick pass (no C++ build needed):

```sh
go test -bench . -benchmem -run '^$' -short ./bench/
```

## Checklist for the integrating agent

- [x] `go vet ./bench/` clean and the suite compiles against the real
      `src-gluon` package (the core landed in the working tree while the
      suite was being authored, so this was verified directly in addition to
      the stub compile-check).
- [x] Corpus pickup confirmed: all 13 curated `testdata/*.txt` files appear
      as sub-benchmarks (`testdata/malformed/` is intentionally not globbed).
- [x] First real numbers recorded below (preliminary: `-benchtime=10x`,
      uncommitted working tree).
- [x] Scaling sanity check done — and it FAILED the linearity expectation;
      see finding (1) below. This is a real signal for the core agent, not a
      harness bug.
- [ ] `./build.sh` then `./bench/bench.sh`: confirm `BenchmarkGoogleDump`
      actually runs (not skipped) once `gen/bin/robots_dump` exists — it was
      skipped in the recorded run because the binary was not built here.
- [ ] Re-record steady-state numbers once the core settles, from a committed
      tree (include the commit hash), with default `-benchtime` and
      `-count=10` + benchstat for anything quotable.

## Recorded results

### 2026-07-03 — first full run (preliminary)

Environment: Apple M4, darwin/arm64, go1.26.2, `-benchtime=10x` (only 10
iterations per benchmark — treat as indicative, not quotable), uncommitted
working tree (core `src-gluon` fresh, `gen/bin/robots_dump` not built, so
`BenchmarkGoogleDump` skipped). Total suite time 484s. Raw output:
`gen/bench-latest.txt` (git-ignored).

`bench.sh` summary table (ns/op):

```
input                         GluonParse                    GluonEvents                   GluonEventsScaling
accretional-robots            67675 (1.02 MB/s)             59367 (1.16 MB/s)             -
case-variation                274754 (0.95 MB/s)            201867 (1.29 MB/s)            -
comments-everywhere           361021 (1.66 MB/s)            345571 (1.74 MB/s)            -
cr-endings                    58125 (1.17 MB/s)             59371 (1.15 MB/s)             -
crlf-endings                  66996 (1.70 MB/s)             88542 (1.29 MB/s)             -
empty-values                  93058 (1.06 MB/s)             100604 (0.98 MB/s)            -
groups-merging                286721 (1.67 MB/s)            330338 (1.45 MB/s)            -
percent-encoding-utf8         132375 (1.51 MB/s)            115004 (1.74 MB/s)            -
realistic-large               5999583 (0.53 MB/s)           5959658 (0.53 MB/s)           -
rfc-example                   163079 (1.50 MB/s)            172575 (1.42 MB/s)            -
unknown-directives            195358 (2.34 MB/s)            190383 (2.40 MB/s)            -
whitespace-torture            122388 (1.97 MB/s)            133388 (1.81 MB/s)            -
wildcards                     132742 (1.55 MB/s)            135146 (1.52 MB/s)            -
lines=100                     -                             -                             6479629 (0.46 MB/s)
lines=1000                    -                             -                             425437862 (0.07 MB/s)
lines=10000                   -                             -                             43723028558 (0.01 MB/s)
```

Findings (in priority order):

1. **Super-linear (~quadratic) scaling in `Grammar.Events`** — the headline.
   Each 10x step in input lines costs far more than 10x in time:
   6.48 ms @ 100 lines -> 425 ms @ 1000 (65.7x) -> 43.7 s @ 10000 (102.8x).
   Allocations grow linearly (27k -> 259k -> 2.6M allocs/op), so the blowup
   is CPU-side re-scanning/backtracking in the engine or grammar, not
   allocation volume. Until fixed, large real-world robots.txt files
   (100KB+ is common; Google caps at 500KiB) are impractical, and the
   `lines=10000` case alone takes ~44s/iteration (bench.sh passes
   `-timeout 60m` for this reason). **Action for the core agent:** profile
   `Events` on `syntheticRobots(1000)` (see bench/bench_test.go); the
   corpus rows hint the same effect (realistic-large, 3.2KB, is already
   down to 0.53 MB/s vs ~2 MB/s for sub-1KB files).
2. **`Events` ≈ `Parse` in cost** on every corpus file (ratio ~0.9-1.3x,
   identical allocs within ~1%): the event path pays for full CST
   materialization. Expected for a first implementation; headroom exists if
   event extraction can avoid building/retaining the whole tree.
3. Absolute throughput on small files is ~1-2.4 MB/s with ~35 allocs per
   input byte (e.g. 2374 allocs for a 69-byte file) — orders of magnitude
   below Google's hand-rolled parser, as anticipated in bench/README.md.
   Not a concern per se; track the trend, not the level.
4. Harness note: the synthetic generator initially emitted digit-bearing
   user-agent tokens (`synthetic-bot-0`), which the grammar correctly
   rejects — RFC 9309 `identifier` allows only `-`/`_`/ALPHA, no digits.
   Fixed with base-26 alphabetic names. Real-world UA strings with digits
   will need the planned malformed-input handling (docs/TODO.md).

---

## Integration update (bootstrap agent, 2026-07-03)

Checklist item completed: `BenchmarkGoogleDump` now runs against the built
`gen/bin/robots_dump` (post `./build.sh`) — ~4.9 ms/op flat across all 13
corpus files at `-benchtime=20x` (109–144 KB/op, 71–449 allocs/op on the Go
side). The flat profile confirms it measures subprocess spawn+pipe overhead,
not parsing; per-byte C++ throughput only shows on realistic-large
(0.64 MB/s including spawn).

The quadratic `Events` scaling finding was root-caused to upstream gluon
(`astParser.loc()` rescans from offset 0 per node; longest-match alternation
re-parses all alternatives) and filed as docs/TODO.md item 7.

---

## Upstream fix validated (bootstrap agent, 2026-07-03, later same day)

The quadratic scaling was fixed in gluon (issue
https://github.com/accretional/gluon/issues/6, PR
https://github.com/accretional/gluon/pull/7 — `loc()` now binary-searches a
precomputed newline index; equivalence-tested upstream). Validated against
the local gluon checkout via a temporary go.work (Apple M4, -benchtime=2x):

| BenchmarkGluonEventsScaling | pinned gluon (before) | patched gluon (after) |
|---|---|---|
| lines=100 | 6.5 ms | 0.90 ms |
| lines=1000 | 425 ms | 10.4 ms |
| lines=10000 | 43.7 s | 105 ms |

After: linear (10.4×/10.1× per 10×) at ~3 MB/s, allocs unchanged. The repo
still pins the pre-fix gluon; docs/TODO.md items 7–8 track re-pinning to
main once the PR merges. Profiling workflow is now reusable:
`bench/profile.sh` here, `scripts/bench-parse.sh` + `PERF.md` upstream.

---

## Re-pinned to gluon main (2026-07-04)

gluon PR #7 merged (rebased to additive-only after main independently
landed both code fixes: 3b97bbf loc(), 8266db6 ParseCSTWithOptions); this
repo now tracks gluon main (`117ed15`, pseudo-version
v0.0.0-20260704042112) — see tools/gluon/README.md; updates via
tools/gluon/repin.sh. Full bench.sh run on the real pin (Apple M4):

- GluonEventsScaling: 0.99ms / 10.1ms / 102ms at 100/1k/10k lines —
  linear, ~3.0 MB/s constant.
- realistic-large.txt: GluonParse 1.09ms, GluonEvents 1.03ms (3.1 MB/s) —
  now FASTER than the robots_dump subprocess round-trip (5.4ms,
  spawn-dominated).
- DisableAutoComments (now functional upstream) enabled in
  src-gluon/parse.go; all suites + 15s fuzz green post-repin.

Two-tier parsing primitives for gluon filed as gluon#8 (StartRule option,
reusable Parser handle, structured/partial errors) — see
docs/design/two-tier-parsing.md.

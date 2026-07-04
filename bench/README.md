# bench/ — parser benchmarks

Benchmarks comparing the gluon-based EBNF robots.txt parser
(`src-gluon`, import `github.com/accretional/proto-robotstxt/src-gluon`)
against Google's hand-rolled C++ parser (`src-google`, exercised through the
`gen/bin/robots_dump` event-dump binary).

## What each benchmark measures

| Benchmark | What it times | Notes |
|---|---|---|
| `BenchmarkGluonParse` | `Grammar.Parse` — full CST, materializing a complete `gluonpb.ASTDescriptor` | One sub-benchmark per `testdata/*.txt` file; `b.SetBytes` set, so MB/s is reported |
| `BenchmarkGluonEvents` | `Grammar.Events` — the google-parser-equivalent event stream | Same corpus; this is the semantically comparable path to Google's parser |
| `BenchmarkGoogleDump` | One `gen/bin/robots_dump` subprocess invocation per iteration | **Includes fork/exec + pipe I/O + output re-parsing**, see caveats; skipped under `-short` or if the binary isn't built |
| `BenchmarkGluonEventsScaling` | `Grammar.Events` on synthetic in-memory inputs of 100 / 1000 / 10000 lines (groups of 1 `User-agent` + 9 `Allow`/`Disallow` rules) | Isolates scaling shape from corpus idiosyncrasies |

The corpus is every `testdata/*.txt` file at the repo root, discovered at
runtime (the suite walks up from its working directory to the directory
containing `go.mod`). Files are read into memory before the timer starts;
the grammar is compiled once per process via `robotsgluon.Default()` and
never inside a timed region.

## How to run

From the repo root, after `./build.sh` (needed only for the Google side):

```sh
./bench/bench.sh                 # runs everything, tees raw output to gen/bench-latest.txt,
                                 # prints a per-input ns/op comparison table
./bench/bench.sh -count=5        # extra args are passed to `go test` (use with benchstat)
go test -bench . -benchmem -run '^$' -timeout 60m ./bench/          # bare invocation
go test -bench . -benchmem -run '^$' -timeout 60m -short ./bench/   # skips the subprocess benchmark
```

Note the generous `-timeout` (bench.sh passes it automatically): while the
engine has super-linear scaling hotspots, the `lines=10000` synthetic case
can take minutes per iteration, and `go test`'s default 10m timeout kills
the whole run mid-benchmark.

Every benchmark degrades to `b.Skipf` (not a failure) when its inputs are
missing: no `testdata/*.txt` files, grammar failing to load, or
`gen/bin/robots_dump` not built.

For statistically meaningful comparisons across code changes, run with
`-count=10` on a quiet machine and feed two saved outputs to
[`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat).

## Methodology caveats — read before quoting numbers

**The C++ numbers include subprocess overhead.** `BenchmarkGoogleDump` spawns
`robots_dump` once per iteration, so each sample pays process fork/exec, pipe
setup and teardown, and re-parsing of the dumped event text — typically
hundreds of microseconds to milliseconds of roughly constant per-file cost,
while Google's actual parse of a small robots.txt is on the order of
microseconds. These numbers are a *ceiling* on Google's cost and an anchor
for "what does the end-to-end CLI path cost", **not** a head-to-head parser
comparison. A fair in-process comparison would require cgo or a
persistent-server harness; deliberately out of scope for now.

**The gluon parser is expected to lose on raw throughput — by orders of
magnitude.** A generic EBNF/CST engine building a full parse tree cannot
match a hand-rolled, allocation-averse, line-oriented C++ scanner, and that
is fine: the goal of this project is a *formally grounded, bijective*
implementation, not a faster one. The interesting metrics are:

1. **Scaling shape** (`BenchmarkGluonEventsScaling`): ns/op should grow
   linearly with input lines (equivalently, MB/s should stay roughly flat)
   across the 100 → 1000 → 10000 steps. Super-linear growth means the
   grammar or engine has a backtracking/ambiguity hotspot worth fixing.
2. **Events-vs-Parse overhead** (`BenchmarkGluonEvents` vs
   `BenchmarkGluonParse` on the same file): how much of the cost is CST
   materialization vs. event extraction. If `Events` is nearly as expensive
   as `Parse`, the event path is doing (or forcing) full CST work and there
   is headroom to shortcut it.
3. **Allocations** (`-benchmem`: B/op, allocs/op): allocation count usually
   dominates generic-parser cost and is the first knob for optimization.

Other standard caveats: run on a quiet machine (no builds/browsers churning
in the background), beware of thermal throttling on laptops for long runs,
and never compare single runs — use `-count` + benchstat.

## Interpreting results

- `ns/op` — time per full parse of one file (or one subprocess round-trip
  for `GoogleDump`). Lower is better.
- `MB/s` — input bytes processed per second (`b.SetBytes`). Comparable
  across files of different sizes; this is the best single throughput number
  for the gluon rows. For `GoogleDump` it is distorted on small files by the
  constant spawn overhead.
- `B/op`, `allocs/op` — allocation footprint per parse (gluon rows only;
  the subprocess rows mostly measure the harness).
- Table cells with `-` in the `bench.sh` summary mean the benchmark was
  skipped or doesn't apply to that input (e.g. synthetic `lines=N` inputs
  only exist for the scaling benchmark).

Raw output of the latest run is kept at `gen/bench-latest.txt` (git-ignored).
When recording milestone numbers, copy them into
`docs/progresslog/benchmarks.md` with the date, machine, and commit hash.

## Profiling (finding out WHY a benchmark is slow)

`bench/profile.sh` wraps the workflow that found gluon's O(n²) `loc()`
rescan (accretional/gluon#6, fixed in PR #7):

```sh
bench/profile.sh                          # CPU-profile the 1000-line scaling bench
bench/profile.sh -l 'astParser.loc'       # + line-level cost of one function
bench/profile.sh -b 'GluonParse' -t 5x    # any benchmark regex / benchtime
```

Methodology (also documented grammar-agnostically in gluon's `PERF.md`,
which ships the same harness for any .ebnf):

1. **Detect** with scaling ratios, not absolute numbers: the
   `GluonEventsScaling` sub-benchmarks grow input 10× per step, so ~10×
   time is linear; 30×+ is superlinear; ~100× is quadratic. Allocations
   growing linearly while time grows quadratically = a CPU-side rescan
   (cost per node × scan length), not an allocation problem.
2. **Localize** with a short profile: pick a size where the slowdown is
   pronounced but one iteration stays around a second (1,000 lines,
   `-benchtime=2x` — pprof needs ~1s of samples, not minutes), then read
   `go tool pprof -top`. One dominant *flat* frame is the culprit
   (`loc()` was 62% flat at 1k lines).
3. **Confirm** at line level with `pprof -list <func>`, fix, and re-run the
   scaling suite: the fix is real when the ratios return to ~10× AND
   `allocs/op` is unchanged (behavior-preserving) — see
   docs/progresslog/benchmarks.md for the before/after record.

Note: profiles taken in *this* repo attribute time to the gluon version
pinned in go.mod. To profile a local gluon checkout (e.g. to validate a fix
before it merges), add a temporary workspace first:
`go work init . /path/to/gluon`, profile, then delete `go.work` (don't
commit it).

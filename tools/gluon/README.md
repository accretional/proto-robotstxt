# tools/gluon — managing the gluon dependency

This repo's parser is built on [gluon](https://github.com/accretional/gluon)
(`github.com/accretional/gluon`, tracked on **main** — no fork, no vendor,
no replace directive; plain module-proxy pseudo-versions in go.mod).

## What we need from gluon (the API contract src-gluon relies on)

- `v2/metaparser.ParseEBNF` — grammar/rep.ebnf → GrammarDescriptor
- `v2/metaparser.ParseCSTWithOptions` + `ParseOptions{TokenMatchers,
  DisableAutoComments}` — whitespace-significant, matcher-driven CST parse
  (both landed on main 2026-07-04: `8266db6`)
- `v2/compiler.GrammarToAST` / `Compile` — grammar → proto/rep.proto
- linear-time parsing: `lexkit` `loc()` line-index fix (`3b97bbf`,
  [gluon#6](https://github.com/accretional/gluon/issues/6)); guarded
  upstream by `lexkit/parse_ast_loc_test.go` + the scaling benchmarks in
  `v2/metaparser/parse_bench_test.go` (landed via
  [gluon#7](https://github.com/accretional/gluon/pull/7))

Behavioral assumptions that would break us if gluon changed them are listed
in `src-gluon/README.md` ("Design decisions & gotchas") — start-rule =
`Rules[0]`, longest-match alternation with first-wins ties, matcher
priority over grammar bodies, byte-counted columns.

## Updating

```sh
tools/gluon/repin.sh          # latest main
tools/gluon/repin.sh <sha>    # specific commit
```

The script pins, tidies, builds, runs the unit + cross-parser + fuzz-seed
+ bench suites, and finishes with a parse-scaling sanity check (~10ms @
1k lines; if it regresses toward 100ms+, suspect a gluon perf regression —
profile with `bench/profile.sh`, methodology in gluon's `PERF.md`).
Always follow a bump with the full `./run.sh` gate before pushing.

## History

- Bootstrap (2026-07-03) pinned branch commit `3ab5064`
  (`xmile-gluon-cst-options`) because `ParseCSTWithOptions` wasn't on main.
- 2026-07-04: main gained the matcher API (`8266db6`), the O(n²) fix
  (`3b97bbf`), and the perf tooling (`117ed15`, PR #7); repo re-pinned to
  main and the branch pin retired. Details:
  docs/progresslog/benchmarks.md, docs/TODO.md Done section.

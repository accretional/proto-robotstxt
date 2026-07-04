# CLAUDE.md — agent guide for proto-robotstxt

Read the root `README.md` first — it is the project spec. Then skim
`docs/README.md` (docs conventions) and `src-gluon/README.md` (parser design
+ gotchas) before touching parser code.

## Hard rules

1. **ALWAYS make sure `./run.sh` works end-to-end before `git push`.**
   `run.sh` → `test.sh` → `build.sh` → `setup.sh`; it builds both parsers,
   runs the upstream C++ tests, the Go tests (including the gluon-vs-google
   cross-parser conformance check over `testdata/`), and the live
   accretional.com e2e demo. No green `./run.sh`, no push.
2. **Document as you go.** Keep an active record of notes, shared context,
   learnings and findings in `docs/`. For every task, keep a progress log at
   `docs/progresslog/<taskname>.md` (see `docs/progresslog/README.md` for
   the format) and update it as you work — decisions, gotchas, state.
3. **Maintain `docs/TODO.md`** — add follow-ups you discover; remove items
   you complete.
4. **Never edit vendored code.** `src-google/` must stay byte-identical to
   upstream google/robotstxt except for the documented BUILD/MODULE changes
   (`src-google/VENDOR.md`). Behavior changes belong in our layers.
5. **Don't deviate from the BNF.** `grammar/rep.ebnf` is the RFC 9309
   formalization; google-parser leniencies (typos, missing colons, junk
   lines) are handled OUTSIDE the grammar (events compiler, future
   preprocessing layer — docs/TODO.md "Malformed-input handling"), never by
   loosening the RFC core rules.
6. `proto/rep.proto` is generated (from the grammar, via
   `go run ./cmd/gluon genproto`) — regenerate + re-consolidate, don't
   hand-edit. Everything else under `gen/` is git-ignored build output.

## Layout / where things go

- `src-google/` — vendored google/robotstxt (Bazel: `//src-google:...`)
- `src-gluon/` — grammar-driven parser + events compiler (Go)
- `grammar/rep.ebnf` — the EBNF formalization (start rule first!)
- `proto/rep.proto` — consolidated derived proto rep
- `cmd/gluon` — CLI (`parse` / `rep` / `events` / `check` / `genproto`)
- `tools/robots-dump/` — C++ event dumper over the vendored parser
- `tools/{rfc,google-dev}/` — docs pullers (`docs/rfc/`, `docs/google-dev-docs/`)
- `testdata/` — strict corpus (cross-checked); `testdata/malformed/` —
  google-lenient corpus (excluded from strict checks); see its README
- `fuzz/` — Go-native fuzzing now; libprotobuf-mutator differential plan
- `bench/` — benchmark suite (`bench/bench.sh`)
- `docs/` — knowledgebase (RFC summary, google dev docs, TODO, progress logs)

## Testing quick reference

```sh
./run.sh                        # full gate (required before push)
go test ./src-gluon/            # unit + cross-parser conformance
go test -fuzz=FuzzParse -fuzztime=30s ./fuzz/
bazel test //src-google:robots_test //src-google:reporting_robots_test
gen/bin/gluon check testdata/*.txt   # after ./build.sh
```

Prefer testing against `https://accretional.com/robots.txt` for live checks
(run.sh does; `testdata/accretional-robots.txt` is the offline fallback).

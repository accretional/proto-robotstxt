# two-tier-phases2-5 — metadata, recovery proto, limits, differential fuzz

## 2026-07-04 — phases 2–5 of docs/design/malformed-input.md implemented

Continuation of [two-tier-phase1.md](two-tier-phase1.md). All four
remaining phases landed same-day; the malformed-input design is COMPLETE.
The strict BNF core (grammar/rep.ebnf) was not touched by any phase.

### Phase 2 — metadata bijectivity

- tools/robots-dump emits `META\tline\t<7 flags>` records (robots.h
  LineMetadata declaration order), interleaved exactly as google calls
  `ReportLineMetadata` — including the phantom EOF record google emits
  after the last terminator.
- Go mirror is a pure line-local pass (src-gluon/metadata.go):
  `googleLines` (BOM skip incl. partial, CR/LF/CRLF split, always-final
  segment) + `parseGoogleLine` (GetKeyAndValueFrom port, now also the
  source of recovery's fallback — one scan, so events and metadata can
  never disagree) + `classifyKeyTypo` (GetKeyType port with google's exact
  typo sets and short-circuit order).
- `GoogleParse` returns events+metadata; `DiffMetadata`;
  `gluon check -recover` and `TestRecoverCrossGoogle` now compare BOTH
  streams — byte-equal across all 25 corpus files. New `gluon meta` CLI.
- Gotcha recorded: `is_acceptable_typo` is only computed for directive
  lines (GetKeyType is only called when has_directive) — mirrored.

### Phase 4 (done before 3 — it shares the phase-2 scanner)

- `googleLines` ports `kMaxLineLen` (2083×8): content past 16663 bytes
  dropped, line flagged `is_line_too_long`.
- A document with ANY too-long line bypasses tier 1: google parses
  truncated content, so even a spec-valid document would deserialize
  differently; recovery applies the same truncation. Strict `Parse()`
  stays RFC-pure (no cap — the RFC has none).
- CONFIRMED from robots.cc Parse(): no total-input cap exists; RFC 9309
  §2.5's 500 KiB is a crawler *processing minimum* (consumer policy), not
  parser behavior. Neither tier caps total size.
- `testdata/malformed/line-too-long.txt` + `TestRecoverTooLongLine`
  (differential incl. metadata).

### Phase 3 — typed rep for recovery

- `proto/recover.proto` checked in: `RecoveredRobotstxt{strict, lines,
  metadata}`, `RecoveredLine`, `IrregularDirective`, `LineMetadata`.
  Descriptor hand-built in src-gluon/recoverproto.go (irregular lines are
  not in the grammar; proto/rep.proto stays purely grammar-derived) and
  emitted into the same FileDescriptorSet — `gluon genproto` writes both
  files; both validate with protoc.
- text/key/value are `bytes`: recovered lines can carry arbitrary non-UTF-8
  bytes that proto3 `string` rejects at marshal time.
- `RecoveredToRep` (dynamicpb) + `gluon rep -recover`.

### Phase 5 — differential fuzzing targets Recover

- `FuzzDifferential`: per execution, `Recover(bytes)` vs the real
  robots_dump subprocess; events AND metadata must be byte-identical for
  ARBITRARY input. ~200 execs/s (subprocess-bound). Session results below.
- Structure-aware mutation (libprotobuf-mutator style) documented as the
  upgrade path in fuzz/README.md — blocked on the rep→text renderer
  (docs/TODO.md item 5); once that exists, the cheap version is a pure-Go
  structured fuzzer feeding the same differential check.

### Differential fuzz session results (2026-07-04, Apple M4)

First sustained `FuzzDifferential` session: **5m01s, 46,670 executions,
ZERO divergences** — every arbitrary byte string produced byte-identical
events and per-line metadata from `Recover` and the real google parser.
Coverage-guided corpus grew to 107 interesting inputs and was still
growing at cutoff; throughput ~155 execs/s average (subprocess-bound, as
predicted — gluon#8 item 2 would mostly help the in-process side).
Longer sessions are cheap to run: `go test -fuzz=FuzzDifferential
-fuzztime=30m ./fuzz/` after `./build.sh`.

# matcher-and-renderer — TODO items 4, 5, and 2 (structure-aware fuzzing)

## 2026-07-04 — bijective matcher + rep→text renderer + structured fuzzing

Both big rocks after the two-tier completion, landed together (they were
requested together and the renderer unblocks structured fuzzing).

### RobotsMatcher port (src-gluon/matcher.go) — TODO 4 ✅

Byte-for-byte port of robots.cc RobotsMatcher as an EVENT-STREAM consumer
(google's matcher is itself a RobotsParseHandler), so it runs identically
over strict and recovered documents and needed zero new parsing code:

- group state machine: seen_separator semantics (consecutive UA lines
  merge; any rule closes the group for subsequent UA lines), "*" and
  "* <junk>" as global, ExtractUserAgent ([a-zA-Z_-] prefix) matched
  case-insensitively against caller agents
- longest-match precedence: pattern length = priority, specific group
  beats global, allow wins ties (disallow needs strictly greater),
  kNoMatchPriority=-1 initialization semantics preserved
- the index.htm/index.html → "<dir>$" allow-normalization recursion
- the pos-array wildcard matcher ('*' any run; '$' special only at end)
- GetPathParamsQuery URL→path (no percent-normalization of the URL — the
  testsets log's observation that raw-UTF-8 URLs don't match escaped
  patterns is faithful google behavior, preserved)

Proof: matcher-cases.tsv 15/15; TestMatcherGridVsGoogle 3,180
(file, agent, url) triples across both corpus tiers — identical decisions
to robots_main; FuzzMatcher 32,137 fuzzed triples vs the real binary, zero
divergences. CLI `gluon allowed` has robots_main argument/exit parity and
run.sh asserts live agreement.

### rep→text renderer (src-gluon/render.go) — TODO 5 (core) ✅

`RenderRep(msg, {Validate})` renders a robotstxt.rep.Robotstxt (dynamicpb)
to canonical text ("Key: value\n", blank line between top-level items).
Two documented guarantees:
1. parser-produced reps: parse∘render is the IDENTITY (proto.Equal), all
   13 strict corpus files + FuzzRenderRoundTrip (identity is asserted from
   one parse in — a MUTATED rep may hold shapes no parse produces, e.g. a
   top-level directive after a group, which reparsing legitimately folds
   into the group; first-render idempotence is deliberately NOT claimed)
2. arbitrary reps, Validate off: field bytes emitted verbatim →
   adversarial text for fuzzing; Validate on: fields must satisfy their
   grammar rules (product_token, path_pattern, key/value classes) so
   output reparses strictly.

CLI: `gluon render [-raw] rep.textproto` (inverse of `gluon rep`);
`NewRepMessage` exposes the runtime-derived schema for external builders.
Remaining from TODO 5 (deliberately not built yet): edit operations
(add/remove rules, merge groups) — trivial consumers of NewRepMessage +
RenderRep when needed.

### Structure-aware differential fuzzing (fuzz/) — TODO 2 ✅

`FuzzStructured`: the fuzz input is WIRE BYTES of a Robotstxt rep
(libprotobuf-mutator's core trick, in pure Go) → raw render → the same
recovery-vs-robots_dump differential as FuzzDifferential (events +
metadata). Seeded from marshaled corpus reps so mutation starts from deep
valid shapes. `FuzzRenderRoundTrip` guards the renderer's identity
guarantee under the same mutation. The libFuzzer/C++ variant (BCR
libprotobuf-mutator + cc_fuzz on //src-google:robots) remains optional
future work for C++-side coverage feedback — noted in fuzz/README.md.

### Session results (2026-07-04, Apple M4)

- FuzzMatcher: 32,137 fuzzed (robots, agent, url) triples vs robots_main —
  zero decision divergences (2-minute session).
- FuzzStructured: 90s of rep wire-byte mutation → raw render →
  events+metadata differential vs robots_dump — zero divergences.
- FuzzRenderRoundTrip: 45s — parse∘render identity held for every
  validating mutated rep.
- TestMatcherGridVsGoogle: 3,180 grid triples in ~21s (subprocess-bound).

Gotcha recorded during FuzzRenderRoundTrip design: first-render idempotence
is NOT an invariant for mutated reps (a top-level directive after a group
legitimately folds into the group on reparse); the identity guarantee
starts one parse in. The initial assertion tripped on exactly this and was
corrected before any session ran against it.

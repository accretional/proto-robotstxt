# two-tier-phase1 — recovery core (malformed-input phase 1)

## 2026-07-04 — implemented, acceptance met

Design: docs/design/malformed-input.md (instance) +
docs/design/two-tier-parsing.md (pattern). Everything landed in one pass;
acceptance gate green on first full run.

### What landed

- **gluon `ParseOptions.StartRule`** upstream first (gluon PR #9 →
  `5d4e3ca`, item 1 of gluon#8): parse a fragment against any sub-rule of
  the same grammar. Re-pinned via `tools/gluon/repin.sh 5d4e3ca`. The
  rotated-grammar workaround from the design draft was never needed.
- **src-gluon/recover.go**: `Grammar.Recover` — tier 1 strict parse; on
  failure, `splitPhysicalLines` (google's CR/LF/CRLF rules), per-line parse
  against `startgroupline → rule → sitemapline → otherline → emptyline`
  (StartRule), `extractIrregular` fallback (byte-for-byte GetKeyAndValueFrom
  port: NUL truncation, '#' comment strip, ASCII trim, colon else
  exactly-two-token whitespace separator, empty-key = no directive),
  `irregularEvent` classification via the existing classifyKey +
  escapePattern. Output: `Recovered{Strict, Lines []LineResult, Events}`.
- **CLI**: `gluon events -recover`, `gluon check -recover` (tier tagged in
  PASS lines). `parse/rep -recover` deferred to phase 3.
- **run.sh step 5**: `check -recover` over BOTH corpus tiers is now part of
  the pre-push gate.
- **Tests**: `TestExtractIrregular` (13 fallback cases incl. NUL edge
  cases), `TestSplitPhysicalLines`, `TestRecoverStrictUsesTier1`,
  `TestRecoverGolden` (mixed doc, line-record assertions),
  `TestRecoverCrossGoogle` — the acceptance gate: recovery events ==
  robots_dump events for all 13 strict + 11 malformed corpus files.
  `FuzzRecover`: totality (never errors), event ordering, and
  no-tier-1-shadowing (strict-accepted input ⇒ recovery events ==
  strict events); 63k execs clean at bootstrap.

### Decisions / gotchas

- The final empty segment google "emits" at EOF is dropped by the splitter:
  it is metadata-only bookkeeping and can never carry a directive; revisit
  in phase 2 when the metadata stream is mirrored.
- NUL truncation must happen BEFORE comment-strip (C strchr semantics —
  a '#' after a NUL is invisible to robots.cc). Pinned by tests
  "nul truncates" / "nul kills separator".
- Trim set is absl::ascii_isspace = " \t\n\v\f\r" — wider than the
  grammar's WS (SP/TAB only). Matters for values arriving via fallback.
- Per-exec fuzz throughput on recovery (~2k/s) is well below strict
  FuzzParse (~16k/s): each irregular line pays gluon's per-call grammar
  conversion — gluon#8 item 2 (reusable Parser handle) is the fix.

### Open (next phases, per design doc)

- Phase 2: metadata bijectivity (extend robots_dump with ReportLineMetadata
  records; mirror + diff). LineResult.Reason is the seed.
- Phase 3: proto/recover.proto + `rep -recover`.
- Phase 4: line-too-long (2083×8) truncation semantics.
- Phase 5: differential fuzzing targets Recover (libprotobuf-mutator plan).

### Addendum (same day): recovery cost quantified

Added `bench/recover_bench_test.go` (tier-2 vs tier-1 cost; the before/after
record for gluon#8 item 2). Tier-2 recovery: ~0.13–0.18 MB/s (~220µs per
irregular line — up to 5 StartRule attempts × ~44µs per-call grammar
conversion) vs tier-1 passthrough ~3.1 MB/s. Posted to gluon#8 with the
assessment: correctness unaffected, not blocking phase 2; becomes material
for phase-5 fuzz volume and larger grammars. Fun fact from the run:
value-no-slash.txt actually passes tier 1 (otherline covers it), confirming
recovery only engages where the grammar genuinely can't.


---

*Superseded names (2026-07-04, phase 2): `splitPhysicalLines`/
`extractIrregular` were replaced by `googleLines`/`parseGoogleLine` in
src-gluon/metadata.go, and the tests renamed accordingly
(`TestGoogleLines`/`TestParseGoogleLine`). See two-tier-phases2-5.md.*

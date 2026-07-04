# Design: malformed-input handling (strict BNF core + line-level recovery)

Status: PHASE 1 IMPLEMENTED (2026-07-04; src-gluon/recover.go, progress log
`docs/progresslog/two-tier-phase1.md`). Phases 2–5 open. Tracks docs/TODO.md
item 3; realizes the project README's goal of a **fully bijective**
googlebot-parser implementation on top of the EBNF core, with the explicit
constraint that **we do not deviate from the BNF formalization**.

This is the first instance of the repo's general **two-tier parsing**
pattern — [`two-tier-parsing.md`](two-tier-parsing.md) defines the pattern
and audits gluon's support for it. The `ParseOptions.StartRule` primitive
this design needs landed upstream before implementation started
([gluon#8](https://github.com/accretional/gluon/issues/8) item 1, gluon
`5d4e3ca`), so the rotated-grammar workaround described in early drafts was
never built.

## Problem

`grammar/rep.ebnf` is a faithful formalization of RFC 9309 §2.2 (plus the
§2.2.4-sanctioned sitemap/other extension lines). google's parser
(src-google/robots.cc) accepts strictly more than the ABNF does. Today those
inputs fail our strict parse outright — the whole file, not just the bad
line. The divergence catalogue is `testdata/malformed/` (byte-verified, each
with google's observed behavior in `testdata/README.md`):

| case | google behavior | strict grammar |
|---|---|---|
| missing colon (`Disallow /tmp`) | whitespace separator iff exactly two tokens | reject |
| UA value w/ spaces, digits, `/` (`Example Bot/1.0`, `bot123`) | full trimmed value at parse level | reject (product-token: `-A-Za-z_` only) |
| UA-like key typos (`useragent:`, `user agent:`) | classified USER_AGENT (typo tolerance) | reject (other_key refuses UA-like) |
| rule-key typos (`Dissallow:`) | DISALLOW | accepted already (otherline → compiler) |
| junk line (no separator, >2 tokens) | metadata only, no event | reject |
| empty key (`: value`) | no directive, ignored | reject |
| control bytes / invalid UTF-8 | byte-agnostic (NUL truncates the line) | reject (UTF8-char-noctl is exact) |
| BOM, partial BOM, missing final NL | canonicalized | handled already (`Normalize`) |
| line > 2083×8 bytes | truncated + `is_line_too_long` | parsed in full |

## Non-goals

- Loosening any rule in `grammar/rep.ebnf`. The RFC core is frozen; even the
  extension section only ever grows by RFC-invited line *types*, not by
  weakened tokens.
- Canonicalizing input text before the strict parse (beyond the existing
  BOM/EOF `Normalize`). Rewriting bytes would make the CST/rep lie about the
  document and breaks event fidelity (e.g. google emits DISALLOW for
  `Dissallow:` — the *typo* must survive into the UNKNOWN-key machinery, not
  be repaired). This rules out README option (b) as the primary mechanism.

## Approach: two-tier parse with line-level recovery (README option (a))

robots.txt is line-oriented and google's parser is strictly per-line: no
construct, error, or comment spans a newline. So "shift past the malformed
section and rerun on the data before it" has a natural, deterministic unit —
**the physical line**. No general error-recovery machinery needed.

```
input bytes ── Normalize ──▶ Tier 1: strict whole-document parse (today's path)
                                 │ success → CST → rep/events (unchanged)
                                 ▼ failure
                             Tier 2: recovery
                               1. split into physical lines (google's rules:
                                  CR / LF / CRLF — lineIndex already does this)
                               2. per line, try the strict LINE rules from the
                                  same grammar: startgroupline, rule,
                                  sitemapline, otherline, emptyline
                               3. lines matching no rule → "irregular line":
                                  port of robots.cc GetKeyAndValueFrom
                                  (comment strip → trim → ':' else
                                  two-token-whitespace separator → key/value)
                               4. reassemble: grouping state machine +
                                  event stream + diagnostics
```

Key properties:

- **Tier 1 unchanged.** A BNF-valid file never touches recovery; the strict
  path stays the proof that the formalization is sufficient for valid input.
- **Tier 2 reuses the grammar.** Step 2 parses each line against the *same*
  rules in rep.ebnf via gluon's `ParseOptions.StartRule` (landed upstream as
  gluon#8 item 1 for exactly this). The BNF stays the single source of
  truth; recovery only adds *fallback*, never overrides (a line that parses
  strictly is always taken as its strict parse — rule order:
  startgroupline, rule, sitemapline, otherline, emptyline).
- **Irregular lines mirror robots.cc exactly.** Step 3 is a small, totally
  line-local port: comment strip at first `#`, ASCII trim, `strchr ':'`,
  else whitespace separator iff exactly two token runs, empty-key ⇒ no
  directive. Classification and %-escaping reuse the existing
  `classifyKey`/`escapePattern` (already byte-exact). Byte-agnostic like
  google: no UTF-8 validation in this tier (NUL: truncate line, matching the
  robots_main observation in docs/progresslog/testsets.md).
- **Grouping.** For events, grouping is irrelevant (flat stream). For the
  typed rep, irregular USER_AGENT-classified lines open/extend a group
  exactly as startgroupline does; everything else follows the existing
  greedy group shape. This matches google's matcher-level treatment of typo
  UA lines (verify against robots_test.cc `ID_UserAgent…` cases when
  implementing).
- **Diagnostics are part of the contract.** Every recovered document reports
  per-line records mirroring google's `RobotsParseHandler::LineMetadata`:
  `{line, is_empty, is_comment, has_comment, has_directive,
  is_missing_colon_separator, is_acceptable_typo, is_line_too_long,
  matched_strict_rule | irregular_reason}`. That is what makes the result
  auditable ("this file needed recovery because lines 3, 17 were X") and is
  itself a comparison surface (phase 2).

## API / CLI surface

```go
// src-gluon (names bikesheddable):
func (g *Grammar) ParseRecover(src []byte) (*Recovered, error)
type Recovered struct {
    Strict     *gluonpb.ASTDescriptor // non-nil iff tier 1 succeeded
    Lines      []LineResult           // per physical line: strict rule or irregular
    Events     []Event                // == google's stream, both tiers
    Diagnostics []LineMetadata
}
```

- `gluon parse|rep|events|check` gain `-recover`; strict remains the
  default everywhere (and the default posture of the repo).
- `run.sh` gains a recovery cross-check:
  `gluon check -recover testdata/*.txt testdata/malformed/*.txt`.

## Proto shape

`proto/rep.proto` stays purely grammar-derived — irregular lines are not in
the grammar, so they don't belong there. New sibling `proto/recover.proto`:

```proto
message RecoveredRobotstxt {
  robotstxt.rep.Robotstxt strict = 1;    // strict-parsed portion, grammar shape
  repeated IrregularLine irregular = 2;  // line no. + raw bytes + extraction
  repeated LineMetadata metadata = 3;
}
```

(Exact field set fixed during implementation; `IrregularLine` carries
`{int32 line, bytes raw, string key, string value, KeyKind classified,
Reason reason}`.)

## Phases

1. **Recovery core.** ✅ DONE 2026-07-04. Line splitter + per-line
   strict-rule fallback + GetKeyAndValueFrom port + event assembly +
   `-recover` on `gluon events`/`gluon check` + `FuzzRecover` (totality +
   tier-1-shadowing invariants) + run.sh gate step 5.
   *Acceptance met:* `gluon check -recover` and `TestRecoverCrossGoogle`
   pass on **both** corpus tiers (events identical to robots_dump for every
   file in `testdata/` AND `testdata/malformed/`); strict-tier tests
   unchanged. Notes: `parse -recover`/`rep -recover` deferred to phase 3
   (no single CST exists for a recovered doc); LineResult.Reason carries
   the phase-2 metadata seed.
2. **Metadata bijectivity.** Extend tools/robots-dump to also emit
   `ReportLineMetadata` records; mirror them in recovery; extend
   `DiffEvents` to a `DiffParse` covering events + metadata.
   *Acceptance:* metadata streams byte-equal across both tiers.
3. **Typed rep for recovery.** `proto/recover.proto` + `Recovered → proto`
   lowering; `gluon rep -recover` prints it.
4. **Size/limit semantics.** Port google's line-length cap (2083×8 bytes,
   `is_line_too_long`, truncation point) and document the 500 KiB
   RFC 9309 §2.5 processing minimum (matcher-level concern; robots.cc does
   not cap parse size — confirm and record).
5. **Differential fuzzing flips to recovery.** The libprotobuf-mutator plan
   (fuzz/README.md) targets `ParseRecover` vs robots_dump: *any* divergence
   on *any* byte string is a bug. Crashers graduate into
   `testdata/malformed/`. This is the long-run bijectivity proof.

## Alternatives considered

- **Preprocessing to well-formed text (README option (b)) as primary**:
  rejected — lossy w.r.t. event fidelity (typo keys must survive), needs a
  source map for line/offset fidelity, and hides diagnostics. May reappear
  narrowly inside phase 4 if a byte-level case can't be expressed line-wise.
- **Relaxing matchers behind a flag** (e.g. lenient product_token):
  rejected — it silently forks the grammar's meaning; the grammar must have
  exactly one semantics.
- **Generic GLR/error-recovery in gluon**: attractive later as an upstream
  contribution ("line-oriented recovery mode"), but robots.txt only needs
  the line-local version; build it here first, upstream the pattern if it
  generalizes (same trajectory as the perf fix / bench harness).

## Open questions

1. Should typo-UA lines (`useragent:`) open a *new* group in the rep when a
   strict group is already open, or merge (google's matcher semantics —
   answer empirically from robots_test.cc / robots_main during phase 1)?
2. robots_dump metadata (phase 2) changes its output format — version the
   format (e.g. `META` record prefix) so old consumers keep working.
3. Do we cap recovery input at 500 KiB to mirror google-at-crawl behavior,
   or parse unboundedly and leave capping to the future matcher? (Leaning:
   parse unboundedly; caps are consumer policy.)

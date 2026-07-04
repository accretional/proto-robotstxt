# Design: two-tier parsing with gluon (the general pattern)

Status: ADOPTED as the project's parsing posture (2026-07-04). The first
concrete instance is robots.txt malformed-input handling —
[`malformed-input.md`](malformed-input.md) — but the pattern is
format-agnostic and we expect to reuse it for other gluon-parsed formats.
This doc defines the pattern and analyzes what gluon supports today vs what
should move upstream ([gluon#8](https://github.com/accretional/gluon/issues/8)).

## The pattern

Real-world inputs for any format come in two populations: documents that
satisfy the published grammar, and documents that dominant implementations
accept anyway. A parser that wants both *fidelity to the spec* and
*bijectivity with reality* should not blur them into one lenient grammar —
it should parse in two tiers:

- **Tier 1 — strict**: the input is matched, whole, against the faithful
  grammar (for us: an EBNF formalization loaded by gluon, lexical atoms as
  token matchers). Success is a *proof* the document is spec-valid, and the
  CST is the spec's own structure. The grammar is never loosened to admit
  real-world junk; it has exactly one meaning.
- **Tier 2 — recovery**: on tier-1 failure, reparse by *recovery units*
  with the same grammar's sub-rules, falling back per unit to a
  reference-implementation-equivalent extraction for units no rule matches.
  Output = strict fragments + "irregular unit" records + diagnostics that
  say exactly where and why the document left the spec.

The recovery unit is format-specific: for line-oriented formats
(robots.txt, INI, CSV-ish logs) it is the physical line; for
bracket/tag-structured formats (XML-ish, JSON-ish) it is the smallest
resynchronizable region (e.g. element/member boundary). The unit choice is
the only part of tier 2 that needs format knowledge; the machinery
(re-entrant sub-rule parse + fallback + diagnostics) is generic.

Why not one lenient grammar: leniency is implementation-defined, changes
over time, and often isn't context-free (typo tolerance, "accept whitespace
for colon iff exactly two tokens"). Encoding it in the grammar destroys the
spec-fidelity claim and makes the grammar unfalsifiable. Keeping tier 2
separate keeps the BNF pure AND gives a labeled corpus of real-world
deviations for free (our `testdata/malformed/`).

## What tier 2 needs from the grammar engine

1. **Parse from any rule** (not just the document root): tier 2 parses a
   unit against `startgroupline | rule | sitemapline | ...` — i.e. "try
   sub-rule R at this string".
2. **Cheap re-entry**: tier 2 calls the parser once per unit — thousands of
   times per document. Grammar setup cost must be paid once, not per call.
3. **Structured failure**: to pick recovery points (and to report *why* a
   unit is irregular) the engine must say where matching stopped
   (consumed-offset / failing rule), not just a formatted error string.
   For non-line formats this is what makes "shift past the malformed
   region" possible at all.
4. Already present in gluon and load-bearing: token matchers
   (`ParseCSTWithOptions`) for lexical atoms, `DisableAutoComments`,
   whitespace control via the grammar lex, longest-match alternation.

## Gluon today: gaps and workarounds (audited at `117ed15`; row 1 updated after `5d4e3ca` landed)

| need | gluon main today | workaround here | upstream fix (small→large) |
|---|---|---|---|
| (1) start-rule selection | ~~hard-coded `gd.Rules[0]`~~ **LANDED**: `ParseOptions.StartRule` (gluon `5d4e3ca`, PR #9) | — (used directly by src-gluon/recover.go) | done — #8 item 1 |
| (2) re-entrant parsing | every call re-runs `convertGrammarToV1` (pretty-print each rule to EBNF text) AND `newASTParser` re-`ParseExpr`s every rule body | acceptable for rep.ebnf (24 rules; measured ~µs/line at our scale) but wasteful at fuzzing volume and for bigger grammars | `metaparser.NewParser(gd, opts) *Parser` with prebuilt v1 grammar + Expr table; `Parser.Parse(doc, startRule)`. Medium; also removes the v2→v1 print/reparse from the hot path |
| (3) structured errors / partial parse | errors are `fmt.Errorf` strings (`"unconsumed input at offset %d of %d"`); no typed offset, no partial-success mode | line-oriented formats don't need it (unit boundaries are known a priori) — this is why robots.txt recovery is buildable TODAY | typed `ParseError{Offset, Rule}` + `ParseOptions.AllowPartial` returning (root, consumed). Needed before two-tier works for non-line-oriented formats |

**Conclusion: nothing about two-tier parsing is difficult or impossible for
gluon.** Robots.txt recovery is implementable against today's main using
only workaround (1) — rotated-grammar clones — because line boundaries make
(3) unnecessary and our grammar is small enough that (2) doesn't bite. The
three upstream additions are all additive API (no breaking changes, no new
concepts — (1) and (3) surface parameters lexkit already has or nearly has),
and they're what makes the pattern reusable for the other formats we want
to two-tier-parse later. Filed as
[gluon#8](https://github.com/accretional/gluon/issues/8); once (1) lands,
delete the rotation workaround, and once (2)/(3) land, tier 2 becomes a
thin format-specific shim over gluon primitives.

## Sequencing

1. Build robots.txt tier 2 here with the rotation workaround
   (`malformed-input.md` phases 1–3) — proves the pattern end-to-end
   against a reference implementation (google's parser) with differential
   corpora + fuzzing.
2. Land gluon#8 (1) then (2) upstream; simplify tier 2 here accordingly
   (benchmarks in gluon's `PERF.md` harness + our `bench/` guard the
   re-entry cost).
3. Land (3) upstream when the first non-line-oriented format needs
   recovery; the robots.txt suite acts as the regression bed for the
   line-oriented case.

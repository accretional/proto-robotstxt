# TODO

Maintained project TODO list. Keep items ordered roughly by priority; when you finish
one, move it to a "Done" section at the bottom with a date and a pointer to the
relevant `docs/progresslog/<taskname>.md` entry.

1. **Aggregate ALL Google crawling/indexing docs into `docs/google-dev-docs/`.**
   Crawl the full <https://developers.google.com/search/docs/crawling-indexing> tree
   (and <https://developers.google.com/crawling>) — not just the 8 seed pages — via a
   parser or an LLM pass over the output of `tools/google-dev/pull-docs.sh`. Abide by
   CC BY 4.0 for the text (keep the attribution headers the script already emits).
   Update the index README (`docs/google-dev-docs/README.md`) when done. One known
   candidate page: `/crawling/docs/robots-txt/useful-robots-txt-rules` (linked from
   the create-robots-txt page but not in the seed set).

2. **Fuzzing: optional libFuzzer/C++ variant.** Structure-aware
   differential fuzzing is DONE in pure Go (2026-07-04, `FuzzStructured`:
   rep wire-byte mutation → raw render → recovery-vs-robots_dump events +
   metadata; plus `FuzzRenderRoundTrip` for the renderer identity).
   Remaining (optional): the libFuzzer/C++ target with real
   libprotobuf-mutator (BCR has it; cc_fuzz on //src-google:robots) for
   coverage-guided feedback from the C++ side. See fuzz/README.md.

3. **Malformed-input handling: COMPLETE (phases 1–5, 2026-07-04).** All
   phases of `docs/design/malformed-input.md` are done — recovery core,
   metadata bijectivity, proto/recover.proto, line-length semantics, and
   differential fuzzing (see docs/progresslog/two-tier-phase1.md and
   two-tier-phases2-5.md). The strict BNF core was never loosened.
   (Historical motivation: even the RFC's own `Disallow: *.gif$` example,
   §5.1, is rejected by its own ABNF — see `docs/rfc/9309/README.md`.)

4. **Bijective matcher: DONE (2026-07-04).** src-gluon/matcher.go ports
   robots.cc RobotsMatcher over the event stream (works on strict AND
   recovered documents); proven against robots_main via matcher-cases.tsv,
   a 3,180-triple corpus grid, and 32k fuzzed triples — zero divergences.
   CLI `gluon allowed`; run.sh asserts live agreement. See
   docs/progresslog/matcher-and-renderer.md.

5. **Generation/modification tooling.** rep→text emission is DONE
   (2026-07-04: `RenderRep` with Validate/raw modes, `gluon render` CLI,
   identity + event-preservation guarantees fuzz-tested — see
   docs/progresslog/matcher-and-renderer.md). Remaining: edit operations
   (add/remove rules, merge groups) as thin helpers over
   `NewRepMessage`/`RenderRep` when a consumer needs them.

6. **Machine-readable Google crawler registry.** Wire the Google crawler user-agent
   data (HTTP UA strings, robots.txt user-agent tokens, crawler category, IP-range
   JSON object URLs) from `docs/google-dev-docs/` into a machine-readable registry
   (proto or JSON) usable by the matcher and tests. The three crawler-list pages in
   `docs/google-dev-docs/` (common / special-case / user-triggered) are the source
   data.

7. **Ask gluon for semver tags.** The repo now tracks gluon main via
   pseudo-versions (see `tools/gluon/README.md`); tagged releases would make
   go.mod human-readable and downgrades deliberate.

## Done

- **Gluon perf + un-pin** (2026-07-04, see docs/progresslog/benchmarks.md
  and tools/gluon/README.md): the ~quadratic parse scaling (43.7s per
  10k-line file) was root-caused to gluon's `astParser.loc()` offset-0
  rescan ([gluon#6](https://github.com/accretional/gluon/issues/6)), fixed
  upstream (`3b97bbf`, 10k lines now ~105ms, linear), and the perf/bench
  tooling merged via
  [gluon PR #7](https://github.com/accretional/gluon/pull/7). gluon main
  now also carries `ParseCSTWithOptions` (`8266db6`), so the side-branch
  pin is retired — go.mod tracks main; updates via `tools/gluon/repin.sh`.

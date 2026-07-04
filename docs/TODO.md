# TODO

Maintained project TODO list. Keep items ordered roughly by priority; when you finish
one, move it to a "Done" section at the bottom with a date and a pointer to the
relevant `docs/progresslog/<taskname>.md` entry.

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

7. **Decide the scope of "fully bijective" for tier-2 documents.**
   Deserialization-equality (events + metadata) and decision-equality
   (matcher) are done and fuzz-proven, and `RenderRep` round-trips strict
   reps — but there is no renderer for `RecoveredRobotstxt` (tier-2), even
   though `RecoveredLine.text` retains the original bytes, so a
   byte-reconstructing render-back of malformed documents is buildable if
   wanted. Decide whether the README's "fully bijective" goal includes
   that, and implement or explicitly close it.

8. **Unmirrored google surface (port-fidelity review, 2026-07-04).**
   Inventory of vendored behavior we neither mirror nor differential-test,
   in rough materiality order: (a) multi-agent `AllowedByRobots` — our
   `AllowedByEvents` accepts `[]string` but no test passes >1 agent; add
   grid/fuzz coverage; (b) `reporting_robots.{h,cc}` (`RobotsParsedLine`,
   unused-directive taxonomy via `kUnsupportedTags`) — a richer per-line
   report than LineMetadata; (c) `kAllowFrequentTypos=false` mode (we
   hardcode the shipping default); (d) `IsValidUserAgentToObey`,
   `matching_line()`/`disallow_ignore_global()` accessors (match.line is
   stored, unread, for a future port). Full details in
   docs/progresslog/reviews.md.

9. **Ask gluon for semver tags.** The repo now tracks gluon main via
   pseudo-versions (see `tools/gluon/README.md`); tagged releases would make
   go.mod human-readable and downgrades deliberate.

## Done

- **Aggregate ALL Google crawling/indexing docs** (2026-07-03, was item 1; see
  docs/progresslog/google-docs-aggregation.md): full doc tree enumerated by the new
  `tools/google-dev/discover-pages.sh` (nav-link BFS, redirect-canonicalized) — 62
  pages pulled via `tools/google-dev/pull-docs.sh` (now stdin-capable; no-arg =
  discover + pull everything) into `docs/google-dev-docs/` with CC BY 4.0
  attribution headers, incl. `/crawling/docs/robots-txt/useful-robots-txt-rules`.
  Extractor hardened (devsite chrome skipped, fenced code blocks, table-cell
  separators); categorized index rewritten in `docs/google-dev-docs/README.md`.

- **Gluon perf + un-pin** (2026-07-04, see docs/progresslog/benchmarks.md
  and tools/gluon/README.md): the ~quadratic parse scaling (43.7s per
  10k-line file) was root-caused to gluon's `astParser.loc()` offset-0
  rescan ([gluon#6](https://github.com/accretional/gluon/issues/6)), fixed
  upstream (`3b97bbf`, 10k lines now ~105ms, linear), and the perf/bench
  tooling merged via
  [gluon PR #7](https://github.com/accretional/gluon/pull/7). gluon main
  now also carries `ParseCSTWithOptions` (`8266db6`), so the side-branch
  pin is retired — go.mod tracks main; updates via `tools/gluon/repin.sh`.

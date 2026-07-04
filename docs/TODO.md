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

2. **Fuzzing: structure-aware mutation.** Byte-level DIFFERENTIAL fuzzing
   exists (`FuzzDifferential`, 2026-07-04: recovery events + metadata vs the
   real robots_dump per exec — any divergence on any bytes is a bug).
   Remaining: structure-aware mutation à la
   <https://github.com/google/libprotobuf-mutator> — mutate
   `robotstxt.rep.Robotstxt` messages, render to text, reuse the same
   differential check. BLOCKED ON item 5 (rep→text renderer). Cheap first
   step once unblocked: pure-Go structured fuzzer over the dynamicpb rep;
   the libFuzzer/C++ target (BCR has libprotobuf-mutator) adds
   coverage-guided C++ feedback. See fuzz/README.md.

3. **Malformed-input handling: COMPLETE (phases 1–5, 2026-07-04).** All
   phases of `docs/design/malformed-input.md` are done — recovery core,
   metadata bijectivity, proto/recover.proto, line-length semantics, and
   byte-level differential fuzzing (see docs/progresslog/two-tier-phase1.md
   and two-tier-phases2-5.md). The strict BNF core was never loosened.
   Remaining related work lives in items 2 (structure-aware fuzzing) and 4
   (bijective matcher).
   Note the RFC's own `Disallow: *.gif$` example (RFC 9309 §5.1) is rejected by its
   own ABNF (`path-pattern = "/" *UTF8-char-noctl`, §2.2) — see
   `docs/rfc/9309/README.md` — so even "spec-level" inputs need this layer.

4. **Bijective matcher.** Implement the allow/disallow decision logic (a
   `RobotsMatcher` equivalent: user-agent group selection + longest-match rule
   precedence per RFC 9309 §2.2.1–2.2.3) on top of the gluon rep, and
   differential-test it against `src-google` `robots_main` across many
   (robots.txt, user-agent, URL) triples.

5. **Generation/modification tooling.** Tools for generating and modifying robots.txt
   files from the proto rep (`proto/rep.proto`) — rep→text emission, edits (add/remove
   rules, merge groups), round-trip (text→rep→text) guarantees.

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

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

2. **Fuzzing: structured differential fuzzing.** Replace/augment the Go-native fuzz
   harness with <https://github.com/google/libprotobuf-mutator> + libfuzzer driving
   structured `proto/rep.proto` inputs, fuzzing the gluon parser differentially
   against the vendored google parser (`src-google/`) — same input, compare
   parse/match results.

3. **Malformed-input handling for the gluon parser.** RFC-invalid lines currently fail
   strict parsing (e.g. missing colon `Disallow /x`, user-agent values with spaces,
   junk lines). DESIGN AGREED: two-tier parse with line-level recovery — see
   `docs/design/malformed-input.md` for the full plan, phases, and acceptance
   criteria (strict BNF core untouched; per-line fallback ports robots.cc
   GetKeyAndValueFrom; `-recover` CLI flags; both corpus tiers must cross-check).
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

7. **Performance: re-pin gluon once accretional/gluon#7 merges.** The
   ~quadratic parse scaling (6.5ms/100 lines → 43.7s/10k, Apple M4) was
   root-caused to gluon's `astParser.loc()` rescanning from offset 0 per
   emitted node, filed as
   [gluon#6](https://github.com/accretional/gluon/issues/6) and FIXED in
   [gluon PR #7](https://github.com/accretional/gluon/pull/7) (O(log n)
   binary search over a precomputed newline index; behavior-preserving,
   equivalence-tested upstream). Validated here via a temporary go.work:
   10k-line robots.txt 43.7s → 105ms, linear at ~3 MB/s (see
   docs/progresslog/benchmarks.md). After the PR merges:
   `go get github.com/accretional/gluon@main && go mod tidy`, rerun
   `bench/bench.sh`, record numbers in the progresslog.

8. **Un-pin gluon: track main instead of a side-branch pseudo-version.**
   gluon PR #7 deliberately stacks the `xmile-gluon-cst-options` commit
   (`ParseCSTWithOptions`/TokenMatchers — required by
   `src-gluon/matchers.go`) with the perf fix, so merging it puts
   everything this repo needs on gluon main. Then go.mod pins a plain main
   pseudo-version (or a semver tag if gluon starts tagging — worth asking
   for). gluon v2 has no separate go.mod, so `.../v2/...` imports keep
   resolving through the root module; nothing to vendor.

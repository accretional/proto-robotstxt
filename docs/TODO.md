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
   junk lines). Design a preprocessing/recovery layer so the parser accepts everything
   google's parser accepts WITHOUT changing the faithful BNF core. Options per the
   project README: (a) shift past the malformed section and reparse the not-yet-parsed
   data before it, or (b) preprocess the input into a well-formed structure first.
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

7. **Performance: gluon parse path is ~quadratic in line count.**
   `bench/` scaling runs show `Grammar.Events` at 6.5ms/100 lines,
   425ms/1k, 43.7s/10k (Apple M4) — see docs/progresslog/benchmarks.md.
   Root causes in upstream gluon (pinned dep, not our code):
   `lexkit/parse_ast.go` `astParser.loc()` recomputes line/column by
   scanning from offset 0 for every emitted node, and longest-match
   alternation re-parses every alternative. Fix upstream (memoized line
   index keyed on offset; possibly memoized production results) or wrap
   locally. Matters for real-world files: google's parser must handle
   500 KiB (RFC 9309 §2.5); the C++ side does ~0.6 MB/s vs our current
   ~0.02 MB/s on large inputs.

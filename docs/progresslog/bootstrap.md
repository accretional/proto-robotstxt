# bootstrap — initial implementation of the project (2026-07-03)

Task: implement the project per README.md from an empty repo. Done in one
pass with subagents for docs / testsets / docker / benchmarks (their logs:
`docs.md`, `testsets.md`, `docker.md`, `benchmarks.md`).

## State: what exists and works

- **src-google/**: google/robotstxt vendored at `22b355ff` (see VENDOR.md).
  Builds with Bazel 9.1.1 (bzlmod, BCR deps; emsdk/WASM targets dropped).
  Upstream tests pass: `//src-google:robots_test`, `:reporting_robots_test`.
- **Script chain**: `run.sh` → `test.sh` → `build.sh` → `setup.sh` per the
  README contract; `run.sh` is the pre-push gate (CLAUDE.md rule 1).
- **grammar/rep.ebnf**: RFC 9309 §2.2 ABNF formalized in gluon's ISO 14977
  dialect; every rule carries its ABNF original in a comment. Lexical atoms
  (case-insensitive keys, NL, UTF8-char-noctl, …) are matcher-implemented
  rules (`= ;`) — ISO 14977 can't express them (no `? ... ?` support in
  gluon). Extensions (sitemapline/otherline) fenced under the §2.2.4
  invitation.
- **src-gluon/** + **cmd/gluon**: grammar-driven parser via gluon v2
  `ParseCSTWithOptions` + token matchers; CST → typed rep (dynamicpb over a
  runtime-derived schema) and CST → google-form events. CLI: `grammar`,
  `parse`, `rep`, `events`, `check`, `genproto`.
- **proto/rep.proto**: derived from the grammar by the kvq-style genproto
  pipeline (`GrammarToAST` → `typedRepAST` → `compiler.Compile`, printed
  with jhump protoprint). Regenerate: `go run ./cmd/gluon genproto -out gen`,
  then re-prepend the provenance banner (top of proto/rep.proto) onto
  `gen/rep.proto` — that concatenation IS the consolidation step.
- **tools/robots-dump/**: C++ `RobotsParseHandler` event printer (base64
  TSV) over the vendored parser — the comparison surface.
- **Cross-parser conformance**: `TestCrossGoogle` + `gluon check` — gluon
  events == google events across all 13 strict `testdata/*.txt` files.
- **fuzz/**: Go-native `FuzzParse` (no panics; accepted input must lower to
  rep + events cleanly). 258k execs clean at bootstrap. libprotobuf-mutator
  differential fuzzing is designed but not built (fuzz/README.md, TODO).

## Key decisions (and why)

1. **gluon pinned at `3ab5064`** (`v0.0.0-20260616152123-3ab506480c4b`,
   branch `xmile-gluon-cst-options`, pushed): kvq's older pin is an
   ancestor; this commit adds `ParseCSTWithOptions`/TokenMatchers, which the
   line-oriented grammar needs (newline tokens, ABNF case-insensitive keys,
   UTF-8 classes). Fetched via the public module proxy — NO replace
   directive; the repo builds anywhere.
2. **Whitespace-significant parsing**: WHITESPACE symbols stripped from the
   grammar lex after ParseEBNF (supported path in gluon's
   `convertGrammarToV1`); all spacing explicit in the grammar.
3. **Comparison surface = handler event stream** (robots.cc has no AST; its
   deserialization is the `RobotsParseHandler` callback sequence). The
   events compiler ports ONLY key classification (prefix matching + typo
   spellings) and `MaybeEscapePattern` (byte-exact, incl. hex-uppercasing
   and high-bit %-escaping); line split / comment strip / trim happen
   structurally in the grammar. Line numbers counted robots.cc-style
   (CR, LF, CRLF each one line) from node offsets — NOT gluon's Line field
   (it only counts `\n`).
4. **Normalization** mirrors google exactly and only: BOM prefix strip
   (incl. partial-BOM byte consumption — robots.cc post-increment quirk) and
   final-newline append. Everything else strict per BNF; google-only
   leniencies are catalogued in `testdata/malformed/` for the future
   malformed-input layer (README anticipates this; see docs/TODO.md).
5. **rep schema derived at runtime** from the embedded grammar
   (`Genproto("")` → fdset → dynamicpb): the checked-in proto/rep.proto can
   never drift from the grammar, and no protoc/codegen step is needed at
   build time.

## Gotchas for future agents (beyond src-gluon/README.md's list)

- zsh: a leading `=word` in a command (e.g. `echo ======`) triggers
  =command expansion — quote separators in scripts run through zsh.
- gluon's `skipWSAndComments` would treat `//`-prefixed robots.txt paths as
  comments at non-lexical levels; all our rules are lowercase (= lexical
  mode) so it never fires, but don't add TitleCase rules to rep.ebnf.
- upstream robotstxt HEAD requires `emsdk` for its WASM target — kept out of
  our MODULE.bazel deliberately (multi-GB toolchain).
- `gluon genproto`'s `typedRepAST` drops/scalarizes rules by NAME
  (droppedRules/scalarRules in src-gluon/genproto.go); renaming grammar
  rules requires updating those tables.

## Open items at handoff

See docs/TODO.md — top items: full google-docs tree aggregation,
libprotobuf-mutator differential fuzzing, malformed-input handling layer,
bijective RobotsMatcher on the gluon rep, rep→text generation tooling.
First benchmark numbers: to be recorded by bench/bench.sh run
(docs/progresslog/benchmarks.md has the checklist).

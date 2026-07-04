# src-gluon — grammar-driven robots.txt parser

This package parses robots.txt **from the RFC 9309 grammar itself**: the
ABNF of RFC 9309 §2.2, formalized as EBNF in [`grammar/rep.ebnf`](../grammar/rep.ebnf),
is loaded through [gluon v2](https://github.com/accretional/gluon)'s
metaparser and matched against documents. There is no hand-written
robots.txt parser here — the grammar is the parser.

```
grammar/rep.ebnf ──ParseEBNF──▶ GrammarDescriptor (proto)
                                      │
robots.txt bytes ──Normalize──▶ ParseCSTWithOptions(+token matchers)
                                      │
                                ASTDescriptor (CST, proto)
                                      │
                     ┌────────────────┴───────────────┐
              CSTToRep (rep.go)                Events (events.go)
                     │                                │
      robotstxt.rep.Robotstxt                google-form event stream
      (proto/rep.proto, dynamicpb)     (== robots.cc handler callbacks)
```

## Pipeline pieces

| file | role |
|---|---|
| `parse.go` | grammar loading (`Default`/`LoadGrammar`), input `Normalize`, `Parse` → CST |
| `matchers.go` | token matchers for the grammar's lexical atoms (the ABNF character classes, case-insensitive keys, newlines — everything ISO 14977 can't express) |
| `events.go` | the **compiler to google's deserialized format**: CST → `[]Event`, byte-exact against src-google/robots.cc's `RobotsParseHandler` callback stream (key classification via prefix+typos, `MaybeEscapePattern` port, robots.cc line numbering) |
| `rep.go` | CST → typed rep message (`robotstxt.rep.Robotstxt`); schema derived from the grammar at runtime, instantiated with dynamicpb |
| `genproto.go` | grammar → proto schema (`Genproto`), the kvq/proto-sqlite pipeline: `GrammarToAST` → `typedRepAST` → `compiler.Compile`; source of `proto/rep.proto` |
| `google.go` | runs `tools/robots-dump` (the C++ side's event printer) and diffs event streams |

## Design decisions & gotchas (hard-won; keep in mind when touching this)

- **Whitespace-significant parsing.** `ParseEBNF` attaches a lex whose
  WHITESPACE symbols would make the CST parser skip spaces/newlines between
  tokens — fatal for a line-oriented format. `fromEBNF` strips them; all
  spacing is explicit `ws`/`nl` grammar tokens (mirroring the ABNF's `*WS`
  and `NL`).
- **First rule = start rule.** gluon hard-codes `Rules[0]` as the start
  symbol; `robotstxt` must stay the first rule in rep.ebnf (enforced in
  `fromEBNF`).
- **Token matchers over char-by-char alternations.** gluon's keyword
  boundary check silently rejects an all-alpha terminal followed by a
  letter/digit/underscore, so enumerating letters as `"a" | "b" | …`
  misparses; character classes use ranges (`"a" ... "z"`) or matchers.
  Matchers also give us ABNF case-insensitive keys and CRLF/CR/LF newline
  tokens. `ParseCSTWithOptions` (the matcher hook) needs gluon @ `3ab5064`+
  (pinned in go.mod; kvq's older pin is an ancestor).
- **Greedy `[ ]`/`{ }`, longest-match `|`, first-wins ties.** The grammar is
  written so alternation prefixes are disjoint per line kind, and RFC-core
  alternatives are listed before extension lines so ties resolve to the RFC.
- **Normalization = exactly google's two input canonicalizations** (BOM
  prefix strip — including partial-BOM byte consumption — and final-newline
  append). Nothing else: a file google accepts but the ABNF rejects is a
  parse error here, by design ("we don't deviate from the BNF"). The
  catalogue of those leniencies lives in `testdata/malformed/` and
  docs/TODO.md ("Malformed-input handling").
- **Extension lines** (`sitemapline`, `otherline`) follow RFC 9309 §2.2.4's
  explicit invitation. `other_key` refuses user-agent-like keys so an
  extension line can never swallow a group boundary.
- **The comparison surface is the handler event stream.** google's parser
  has no exposed AST; its deserialization IS the `RobotsParseHandler`
  callback sequence. `Events` reproduces it (line numbers counted robots.cc
  style: CR, LF, CRLF each once), `tools/robots-dump` prints it from the
  real C++ parser, `DiffEvents`/`gluon check` assert equality. This runs
  across `testdata/*.txt` in `TestCrossGoogle` and in `./run.sh`.

## CLI

```sh
go build -o gen/bin/ ./cmd/...          # or just ./build.sh
gen/bin/gluon grammar                    # validate grammar, list rules
gen/bin/gluon parse    file.txt          # CST textproto
gen/bin/gluon rep      file.txt          # typed rep textproto (proto/rep.proto)
gen/bin/gluon events   file.txt          # google-form events
gen/bin/gluon check    file.txt ...      # gluon vs google cross-check
gen/bin/gluon genproto -out gen          # grammar -> gen/rep.{proto,fdset}
```

# Task: test corpus for parser cross-testing (testsets)

Date: 2026-07-03
Scope: `testdata/**`, `docs/progresslog/testsets.md` only.

## What was created

A two-tier robots.txt corpus for cross-testing the vendored google C++
parser (`bazel-bin/src-google/robots_main`) against the RFC 9309
EBNF/gluon parser. Full manifest, strict-tier authoring constraints, and
per-file expected behaviors live in `testdata/README.md` (kept as the
single source of truth; this log records process and findings).

- Strict tier, `testdata/*.txt` (12 new files + pre-existing
  `accretional-robots.txt`, untouched): `rfc-example.txt`,
  `groups-merging.txt`, `case-variation.txt`, `comments-everywhere.txt`,
  `whitespace-torture.txt`, `percent-encoding-utf8.txt`,
  `wildcards.txt`, `empty-values.txt`, `unknown-directives.txt`,
  `crlf-endings.txt`, `cr-endings.txt`, `realistic-large.txt`.
  All are strictly valid per the RFC 9309 ABNF + documented extensions
  (sitemap lines, non-user-agent-like otherlines, comments/emptylines).
- Malformed tier, `testdata/malformed/*.txt` (11 files): `bom.txt`,
  `partial-bom.txt`, `missing-colon.txt`, `agent-version.txt`,
  `agent-digits.txt`, `key-typos.txt`, `value-no-slash.txt`,
  `junk-lines.txt`, `no-final-newline.txt`, `control-bytes.txt`,
  `invalid-utf8.txt`. Google handles all of them; the strict grammar
  must not. Excluded from the strict cross-check suite.
- `testdata/matcher-cases.tsv`: 15 (file, agent, url, expected) tuples,
  every one verified against `robots_main` exit codes (0=allowed,
  1=disallowed). Tab-separated with a `#` header describing the format;
  file paths relative to `testdata/`.

Byte-sensitive files (`whitespace-torture.txt`, `empty-values.txt`,
`crlf-endings.txt`, `cr-endings.txt`, everything in `malformed/`) were
generated with `printf` and verified with `xxd` (BOM bytes, NUL, lone
CR, trailing spaces/tabs, missing final newline all confirmed on disk).

`rfc-example.txt` reproduces the RFC 9309 §5.1 example with two noted
adjustments: `Disallow: *.gif$` → `Disallow: /*.gif$` (the RFC's own
example violates its own `path-pattern` ABNF; the verbatim line is
preserved in `malformed/value-no-slash.txt`), and the figure's `EOF`
marker dropped.

## Surprising / notable robots_main behaviors observed

1. **User-agent lines merge across blank lines and comments.** A
   `User-agent: quxbot` line followed only by blanks/comments and then
   `User-agent: *` + rules forms ONE group: quxbot IS subject to those
   rules (verified: quxbot disallowed `/secret/x` in
   `groups-merging.txt`). Mechanism: the matcher only resets its
   seen-agent flags on a user-agent line after at least one rule was
   seen (`seen_separator_` in `robots.cc`). A truly empty group is only
   possible at EOF. This matches a greedy reading of the ABNF
   (`group = startgroupline *(startgroupline / emptyline) ...`), but a
   non-greedy gluon parse could group differently — harmless for the
   line-level event-stream cross-check, load-bearing for any future
   group-level comparison.
2. **Patterns are escaped; URLs are not.** The parser %-escapes
   non-ASCII bytes and uppercases hex escapes in allow/disallow values
   (`MaybeEscapePattern`), but the matcher compares the URL path
   byte-for-byte without applying the same normalization. So
   `Disallow: /日本語/` blocks `/%E6%97%A5%E6%9C%AC%E8%AA%9E/page` but
   NOT a raw-UTF-8 `/日本語/page` URL (both verified). robots_main's
   help does say the URI must be RFC 3986 %-encoded. Same story for
   case: pattern `/%aa` becomes `/%AA` and will not match a literal
   `/%aa` in the URL.
3. **Escaping applies to unknown directives too.**
   `NeedEscapeValueForKey` returns true for the UNKNOWN key type, so
   `HandleUnknownAction` values get the same %-escape normalization as
   allow/disallow; only user-agent and sitemap values are passed raw.
4. **Key matching is case-insensitive PREFIX matching**
   (`absl::StartsWithIgnoreCase`): `Disallowing:` emits DISALLOW,
   `Allowed:` emits ALLOW, `user-agents-are-fun:` starts a group (all
   verified in `malformed/key-typos.txt`). Plus the known typo list:
   `useragent`, `user agent`, `dissallow`, `dissalow`, `disalow`,
   `diasllow`, `disallaw`, `site-map`.
5. **Whitespace-as-separator fallback is exactly-two-tokens only.**
   `Disallow /tmp` parses (missing colon), but a junk line with 3+
   whitespace-separated tokens is dropped entirely, and a random
   2-token line becomes an UnknownAction event (`random junkvalue` ⇒
   `HandleUnknownAction("random", "junkvalue")`).
6. **Agent-name extraction stops at the first non-`[a-zA-Z_-]` byte**
   (`RobotsMatcher::ExtractUserAgent`): a `User-agent: bot123` group
   can never be matched by an agent literally named `bot123` (verified
   allowed), but IS matched by agent `bot` (verified disallowed).
   Likewise `Example Bot/1.0` ⇒ group for `Example`. Digits really are
   invalid in RFC product tokens and google's matcher half-agrees.
7. **Partial BOMs are swallowed.** The BOM-skipping loop consumes any
   matching prefix of `EF BB BF`; a file starting `EF BB U s e r...`
   parses as if the two stray bytes weren't there (verified with
   `malformed/partial-bom.txt`).
8. **NUL truncates the line.** Parsing uses C-string routines on the
   line buffer, so `Disallow: /before<NUL>after-nul` emits
   `HandleDisallow("/before")`; other control bytes (BEL, ESC) are kept
   verbatim in values since they're below 0x80 and thus not escaped.
9. **Bare-CR line endings work** (and are RFC-valid: `NL = %x0D`);
   `cr-endings.txt` verdicts match its LF equivalent. For CRLF, the LF
   is treated as a continuation, not an extra empty line.

## Follow-ups for other phases

- The event-stream cross-check should treat `matcher-cases.tsv` as a
  separate matcher-level test (it includes `malformed/` rows on
  purpose; the strict grammar suite must glob only `testdata/*.txt`).
- When the malformed-input phase starts, behaviors 1-8 above are the
  divergences to encode first.
- The otherline extension's value grammar was kept conservative in the
  strict tier (no internal whitespace in otherline values) — if the
  gluon grammar ends up allowing spaces there, a
  `Clean-param: ref /articles/`-style case can be promoted from a
  future malformed file into the strict tier.

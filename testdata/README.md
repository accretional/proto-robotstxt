# testdata/ — robots.txt test corpus

Two tiers of robots.txt inputs used to cross-test the google C++ parser
(`src-google/`, CLI at `bazel-bin/src-google/robots_main`) against the
RFC 9309 EBNF/gluon parser (`src-gluon/`):

1. **Strict tier** (`testdata/*.txt`): files that are **strictly valid**
   per the RFC 9309 ABNF plus the documented grammar extensions below.
   For each of these, the google parser's parse-event stream
   (`HandleUserAgent`/`HandleAllow`/`HandleDisallow`/`HandleSitemap`/
   `HandleUnknownAction`) must equal the gluon-side compiled events.
2. **Malformed tier** (`testdata/malformed/*.txt`): files the google
   parser accepts/handles but which are **not** valid per the strict RFC
   grammar. These are reserved for the future malformed-input phase and
   for documenting parser divergences. **`malformed/` is excluded from
   the strict cross-check suite.**

`matcher-cases.tsv` holds (file, agent, url, expected) tuples verified
against `robots_main`; a future matcher test consumes it. Format is
documented in its header comment.

## Strict-tier constraints

Every file in `testdata/*.txt` (not `malformed/`) MUST satisfy the
RFC 9309 §2.2 ABNF plus these extensions, so the strict gluon grammar
accepts it. Keep future additions within these rules:

- Grammar extensions beyond the bare RFC ABNF (all still "strict"):
  - `Sitemap: <value>` lines, inside or outside groups;
  - "otherline" `key: value` lines whose key is not user-agent-like
    (e.g. `Crawl-delay`, `Host`, `Noindex`, `Clean-param`);
  - comments and empty lines per the RFC EOL rule
    (`EOL = *WS [comment] NL`).
- Every line ends with a newline. `NL = %x0D / %x0A / %x0D.0A`, so LF,
  CRLF, and even bare CR are all valid line endings.
- Keys contain no spaces (no `user agent:`). Whitespace *around* the
  key is fine: the ABNF allows `*WS key *WS ":" *WS value`, i.e. also
  space/tab between the key and the colon.
- No lines missing the `:` separator.
- User-agent values are a single RFC product token:
  `identifier = 1*("-" / A-Z / "_" / a-z)` or `*`. **No spaces, no
  digits, no dots/slashes** (so no `Bot/1.0`, no `bot123`).
- Allow/Disallow values are either empty (`empty-pattern = *WS`) or
  start with `/` (`path-pattern = "/" *UTF8-char-noctl`). Because
  `UTF8-char-noctl` excludes space, `#`, and control bytes, paths must
  not contain internal whitespace; a `#` after the value starts a
  trailing comment.
- ASCII or valid UTF-8 only; no control bytes, no BOM.
- To be conservative about the otherline extension, keep otherline
  values free of internal whitespace too (e.g. `Crawl-delay: 10`, not
  `Clean-param: ref /articles/`).

Byte-exactness matters (trailing spaces/tabs, CR bytes). Some files are
generated with `printf`; do not let editors/formatters normalize them.

## Strict tier manifest

| File | Exercises | Expected notable behavior |
|---|---|---|
| `accretional-robots.txt` | Pre-existing minimal real-world file (do not modify). | `*` group, `Allow: /`, sitemap outside group; everything allowed. |
| `rfc-example.txt` | RFC 9309 §5.1 example, adjusted (see below). | Group shared by `barbot`+`bazbot`; `Disallow:/` with no space; true empty group (`quxbot`) at EOF ⇒ quxbot unrestricted. |
| `groups-merging.txt` | Same product token in two groups (`foobot`), multiple UA lines per group, UA line followed only by blanks/comments, true empty group at EOF. | `foobot` gets rules of both its groups merged; `quxbot` **merges into the following `*` group** (only blank/comment lines separate the UA lines) so it is subject to `/secret/`; `emptybot` (empty group at EOF) is unrestricted. |
| `case-variation.txt` | `USER-AGENT`/`user-agent`/`uSeR-aGeNt`, `DISALLOW`/`disallow`/`dIsAlLoW` keys; mixed-case agent names. | Keys and agent-name matching are case-insensitive (`FOOBOT` matches `FooBot` group). |
| `comments-everywhere.txt` | Full-line, trailing, indented, `#`-touching-value, `##`, bare-`#` comments. | Google strips everything from the first `#`; comment-only lines emit no directive; `Disallow: /a # c` ⇒ value `/a`. |
| `whitespace-torture.txt` | Leading/trailing spaces and tabs around keys, colons, values; WS-only line; `Allow:/x` with no space. | All whitespace trimmed; `key<WS>:` accepted (ABNF `*WS ":"`); values identical to untortured forms. |
| `percent-encoding-utf8.txt` | Raw UTF-8 in paths (`é`, `日本語`, emoji), lowercase hex escapes (`/%aa`, `%e2%82%ac`), pre-encoded paths, lone `%`. | Google %-escapes non-ASCII bytes (`é` ⇒ `%C3%A9`) and uppercases hex escapes (`%aa` ⇒ `%AA`) in allow/disallow values; the *URL* passed to the matcher is not escaped, so only %-encoded URLs match (raw `…/日本語/page` does NOT match, its encoded form does). |
| `wildcards.txt` | `/*.gif$`, `/foo*bar$`, `/*/private/*`, `/*?sessionid=`, `/$`. | `*` matches any chars, `$` anchors end; longest-match rule picks `Allow: /foo*barbaz` over `Disallow: /foo*bar$` for `/fooXbarbaz`. |
| `empty-values.txt` | `Disallow:` / `Allow:` with empty value, `Disallow: ` with trailing space (byte-exact), `Allow: /`. | Google emits Allow/Disallow events with empty value; empty patterns match nothing ⇒ everything stays allowed unless another rule matches. |
| `unknown-directives.txt` | `Crawl-delay`, `Noindex`, `Clean-param`, `Host`, `Request-rate`, `Visit-time`; `Sitemap` before the first group, inside a group, and after the last group. | Unknown keys emit `HandleUnknownAction` (values also get %-escape normalization); sitemap emits `HandleSitemap` regardless of position and does not terminate a group for the matcher. |
| `crlf-endings.txt` | Every line ends `\r\n`. | Identical parse to LF file; no empty-line events for the `\n` of `\r\n`. |
| `cr-endings.txt` | Bare `\r` line endings only (valid: `NL = %x0D`). | Google splits lines on lone CR; matcher verdicts identical to LF equivalent. |
| `realistic-large.txt` | ~130-line wikipedia/google-style file: many groups, shared UA lines, crawl-delays, `?`/`&`/`=`/`:`/`%3A` in paths, wildcard+anchor patterns, interleaved comments, multiple sitemaps. | Realistic longest-match interplay, e.g. `Googlebot` blocked on `/search` but allowed `/search/about`. |

### `rfc-example.txt` adjustments vs. the RFC text

The RFC 9309 §5.1 example is reproduced with two adjustments so the file
meets the strict-tier constraints:

1. `Disallow: *.gif$` became `Disallow: /*.gif$` — the RFC's own example
   line violates its own ABNF (`path-pattern = "/" *UTF8-char-noctl`
   must start with `/`). The original, unadjusted line lives in
   `malformed/value-no-slash.txt`.
2. The trailing `EOF` marker in the RFC figure is presentation, not file
   content, and was dropped.

## Malformed tier manifest (`testdata/malformed/`)

Google's parser handles all of these; the strict RFC grammar must reject
(at least part of) each. Excluded from the strict cross-check suite.

| File | Exercises | Google behavior (verified via robots_main / code) |
|---|---|---|
| `bom.txt` | UTF-8 BOM (`EF BB BF`) before valid content. | BOM silently skipped; first line parsed as a normal user-agent line. |
| `partial-bom.txt` | Only the first two BOM bytes (`EF BB`) then content. | Partial BOM prefix bytes are silently swallowed; parsing continues at `User-agent`. |
| `missing-colon.txt` | `Disallow /tmp`, `Allow /tmp/ok` — whitespace instead of `:`. | Whitespace accepted as separator, but only when the line has exactly two non-WS token sequences. |
| `agent-version.txt` | `User-agent: Example Bot/1.0` (spaces + version). | `HandleUserAgent` gets the full raw value; the matcher extracts only `[a-zA-Z_-]*` ⇒ group matches agent `Example`; agent string `Example Bot/1.0` itself never matches and falls through to `*`. |
| `agent-digits.txt` | `User-agent: bot123`, `User-agent: v2bot` (digits invalid in product tokens). | Matcher extraction stops at the first digit: the groups effectively belong to `bot` and `v`; agents `bot123`/`v2bot` do NOT match them (verified: `bot123` allowed, `bot` disallowed). |
| `key-typos.txt` | `useragent:`, `user agent:`, `Dissallow:`/`Disalow:`/`Dissalow:`/`Diasllow:`/`Disallaw:`, `site-map:`, plus prefix keys `user-agents-are-fun:`, `Disallowing:`, `Allowed:`. | All typos accepted (`kAllowFrequentTypos`); google matches keys by case-insensitive **prefix** (`absl::StartsWithIgnoreCase`), so `Disallowing:` emits DISALLOW and `Allowed:` emits ALLOW. |
| `value-no-slash.txt` | `Disallow: *.gif$` (verbatim RFC §5.1 example line), `Allow: example`, `Disallow: ?query=`. | Patterns kept as-is and matched (`/pic.gif` disallowed via `*.gif$`) even though they violate `path-pattern`. |
| `junk-lines.txt` | Token lines with no separator; a 4-token line; a 2-token line (`random junkvalue`). | No-separator and >2-token lines are dropped silently; the 2-token line becomes `HandleUnknownAction("random", "junkvalue")` via the whitespace-separator fallback. |
| `no-final-newline.txt` | Last line has no trailing newline (created with `printf`, verified with `xxd`). | Final unterminated line is still parsed (`/truncated/x` disallowed); strict grammar requires NL on every line. |
| `control-bytes.txt` | NUL byte mid-value; BEL (0x07) and ESC (0x1B) in values. | NUL truncates the rest of the line for key/value extraction (C-string parsing); other control bytes are kept verbatim in the emitted value (< 0x80 so not escaped). |
| `invalid-utf8.txt` | Latin-1 `0xE9`, `0xFF 0xFE`, overlong `0xC0 0xAF` in paths. | Any byte with the high bit set is blindly %-escaped (`0xE9` ⇒ `%E9`) with no UTF-8 validation; `/overlong-%C0%AF` verified disallowed. |

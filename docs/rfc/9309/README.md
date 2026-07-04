# RFC 9309 — Robots Exclusion Protocol: implementer's summary

RFC 9309 (Koster, Illyes, Zeller, Sassman; September 2022; Standards Track) is the
normative spec for robots.txt. Canonical text:
<https://www.rfc-editor.org/rfc/rfc9309.html> (also `.txt`).

This directory holds local copies of the RFC as `raw.html` and `raw.txt`. They are
**gitignored on purpose** (IETF Trust licensing) — rematerialize them with:

```sh
tools/rfc/pull-rfc.sh        # default: RFC 9309
```

This README is our own checked-in summary of the parts that matter for implementing a
parser and matcher. Section numbers refer to RFC 9309. The machine-readable grammar
derived from the RFC lives at `grammar/rep.ebnf` (top level of the repo).

---

## 1. Protocol model (§2.1)

- **Rule**: a line with a key-value pair (`allow`/`disallow`) that defines how a crawler
  may access URIs.
- **Group**: one or more `user-agent` lines followed by one or more rules. A group is
  **terminated by the next user-agent line or end of file**.
- The last group may have no rules — that implicitly **allows everything** for the
  matched agents.
- The rules are *not* a form of access authorization (§1, §3).

## 2. Formal syntax — the full ABNF (§2.2)

Quoted verbatim from §2.2 (ABNF per RFC 5234):

```abnf
robotstxt = *(group / emptyline)
group = startgroupline                ; We start with a user-agent
                                      ; line
       *(startgroupline / emptyline)  ; ... and possibly more
                                      ; user-agent lines
       *(rule / emptyline)            ; followed by rules relevant
                                      ; for the preceding
                                      ; user-agent lines

startgroupline = *WS "user-agent" *WS ":" *WS product-token EOL

rule = *WS ("allow" / "disallow") *WS ":"
      *WS (path-pattern / empty-pattern) EOL

; parser implementors: define additional lines you need (for
; example, Sitemaps).

product-token = identifier / "*"
path-pattern = "/" *UTF8-char-noctl ; valid URI path pattern
empty-pattern = *WS

identifier = 1*(%x2D / %x41-5A / %x5F / %x61-7A)
comment = "#" *(UTF8-char-noctl / WS / "#")
emptyline = EOL
EOL = *WS [comment] NL ; end-of-line may have
                       ; optional trailing comment
NL = %x0D / %x0A / %x0D.0A
WS = %x20 / %x09

; UTF8 derived from RFC 3629, but excluding control characters

UTF8-char-noctl = UTF8-1-noctl / UTF8-2 / UTF8-3 / UTF8-4
UTF8-1-noctl = %x21 / %x22 / %x24-7F ; excluding control, space, "#"
UTF8-2 = %xC2-DF UTF8-tail
UTF8-3 = %xE0 %xA0-BF UTF8-tail / %xE1-EC 2UTF8-tail /
         %xED %x80-9F UTF8-tail / %xEE-EF 2UTF8-tail
UTF8-4 = %xF0 %x90-BF 2UTF8-tail / %xF1-F3 3UTF8-tail /
         %xF4 %x80-8F 2UTF8-tail

UTF8-tail = %x80-BF
```

### Grammar notes and known quirks

- **The RFC's own example violates its own ABNF.** §5.1 uses `Disallow: *.gif$`, but
  `path-pattern = "/" *UTF8-char-noctl` requires the pattern to **start with `/`**
  (`*.gif$` starts with `*`; the only other alternative is `empty-pattern`). Real-world
  parsers (including Google's, see `src-google/`) accept patterns that don't start with
  `/`. A faithful BNF parser rejects the RFC's own §5.1 example — this is exactly the
  kind of gap our malformed-input recovery layer must handle (see `docs/TODO.md`).
- `identifier` allows **only** `-`, `A-Z`, `_`, `a-z` — no digits, dots, or slashes.
  Real files commonly contain tokens like `Googlebot/2.1`; those are not derivable
  from the ABNF either. (§2.2.1 prose repeats the letters/underscore/hyphen-only rule.)
- `UTF8-1-noctl` is commented "excluding control, space, `#`" but its range `%x24-7F`
  **includes DEL (0x7F)**, which is a control character. Minor spec nit; be deliberate
  about which reading the grammar encodes.
- `$` (0x24) and `*` (0x2A) are ordinary members of `UTF8-char-noctl` — the wildcard /
  end-anchor semantics (§2.2.3) live entirely in the *matcher*, not the grammar.
- The grammar has no production for rules that appear **before** the first user-agent
  line; §2.2.2 prose says crawlers SHOULD ignore rules that are not in any group.
- Field names are matched case-insensitively in practice (`User-Agent`, `user-agent`,
  `USER-AGENT` all occur; the RFC writes the literals in lowercase — ABNF string
  literals are case-insensitive by RFC 5234 default, so this *is* covered by the ABNF).
- Line ends: `NL = CR / LF / CRLF`. `EOL` permits an optional trailing `# comment`.

## 3. User-agent matching (§2.2.1)

- Crawlers pick groups by their **product token**. The token MUST contain only
  `a-z A-Z _ -`, and SHOULD be a substring of the crawler's identification string
  (e.g. the HTTP `User-Agent` header).
- Matching the product token against `user-agent:` values MUST be **case-insensitive**.
- **Merging:** if more than one group matches the crawler's token, the matching groups'
  rules MUST be **combined into one group** before evaluation (Figure 2).
- **Fallback:** if no group matches, the crawler MUST obey the `user-agent: *` group,
  if present (Figure 3).
- If nothing matches and there is no `*` group (or no groups at all), **no rules
  apply** (everything is allowed).
- Note: the RFC does *not* define "most specific user-agent wins" — that is a Google
  extension. Google picks only the **single most specific matching group** (e.g.
  `googlebot-news` beats `googlebot`) and ignores less specific ones, and strips
  non-matching text so `googlebot/1.2` and `googlebot*` are treated as `googlebot`.
  See <https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt>
  ("Order of precedence for user agents"). Google *does* merge groups that name the
  same token, per the RFC.

## 4. Allow / Disallow evaluation (§2.2.2)

- Paths from `allow`/`disallow` rules are matched against the URI. Matching SHOULD be
  **case-sensitive** and MUST start at the **first octet** of the path.
- **Most-specific-match rule:** the most specific match found MUST be used, where most
  specific = **most octets** (i.e. longest match wins — see the worked example in
  §5.2, where `Disallow: /example/page/disallowed.gif` beats `Allow: /example/page/`).
- **Tie-break:** if an `allow` and a `disallow` rule are equivalent, the `allow` rule
  SHOULD win.
- Duplicate rules in a group MAY be deduplicated.
- If no rule matches, or the group has no rules, the URI is **allowed**.
- `/robots.txt` itself is **implicitly allowed**.
- **Percent-encoding normalization** (§2.2.2, Figure 4): before comparison,
  - octets outside ASCII and RFC 3986 *reserved* characters MUST be percent-encoded;
  - percent-encoded **unreserved** ASCII octets in the URI MUST be decoded (e.g.
    `%62%61%7A` → `baz`), but reserved characters (e.g. `%2F`) and non-ASCII escapes
    (e.g. `%E3%83%84`) stay encoded.
  - "The match evaluates positively if and only if the end of the path from the rule
    is reached before a difference in octets is encountered."
- Rules outside any group (e.g. before the first user-agent line) SHOULD be ignored.
- Implementors MAY bridge encoding mismatches if the file isn't valid UTF-8.

## 5. Special characters (§2.2.3)

Crawlers MUST support:

| Char | Meaning | Example |
|------|---------|---------|
| `#`  | line comment | `allow: / # comment in line` |
| `$`  | **end anchor** — designates the end of the match pattern | `allow: /this/path/exactly$` |
| `*`  | wildcard — 0 or more of **any** character | `allow: /this/*/exactly` |

To match a literal `*` or `$` in a URI, the pattern SHOULD use percent-encoding
(`%2A`, `%24`) — Figure 6.

## 6. Other records — the extension point (§2.2.4)

The ABNF itself carries the invitation: *"parser implementors: define additional lines
you need (for example, Sitemaps)."* §2.2.4 adds:

- Crawlers MAY interpret records that are not part of the protocol (e.g. `Sitemap`,
  per <https://www.sitemaps.org/>), and MAY be lenient (accept common misspellings).
- Parsing of other records **MUST NOT interfere** with the defined records — e.g. a
  `Sitemaps` record **MUST NOT terminate a group**. (So `sitemap:` lines can sit in
  the middle of a group without splitting it.)

This is the hook our grammar uses for `sitemap` and unknown-key lines.

## 7. Access method (§2.3)

- The file MUST be at `"/robots.txt"` (all lowercase), top-level path:
  `scheme:[//authority]/robots.txt` (works for HTTP, FTP, ...).
- MUST be UTF-8 (RFC 3629), media type `text/plain` (RFC 2046).

## 8. HTTP status / fetch-result handling (§2.3.1)

| Result | RFC 9309 requirement | Section |
|--------|----------------------|---------|
| Success (e.g. 2xx) | MUST follow the parseable rules | §2.3.1.1 |
| Redirects (301/302) | SHOULD follow ≥ 5 consecutive redirects, even cross-authority; rules apply to the *initial* authority; > 5 → MAY treat as unavailable | §2.3.1.2 |
| **"Unavailable"** (e.g. **4xx**) | Crawler **MAY access any resources** — i.e. allow all | §2.3.1.3 |
| **"Unreachable"** (server/network errors, e.g. **5xx**) | robots.txt is *undefined*; crawler **MUST assume complete disallow** — i.e. disallow all | §2.3.1.4 |
| Unreachable "for a reasonably long period" (e.g. 30 days) | MAY then treat as unavailable (allow all) or keep using a cached copy | §2.3.1.4 |
| Parsing errors | MUST try to parse **each line**; MUST use the parseable rules (never discard the whole file over one bad line) | §2.3.1.5 |

### Google's documented nuances (diverge from / refine the RFC)

Source: <https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt>
("Handling of errors and HTTP status codes"):

- **3xx:** follows at least five redirect hops, then treats the fetch as a **404**
  (→ allow all). Logical redirects (frames, JS, meta-refresh) are not followed.
- **4xx except 429:** treated as if no valid robots.txt exists → **no crawl
  restrictions** (matches RFC "unavailable"). Google explicitly warns against using
  401/403 to mean "disallow" — they mean *allow* here.
- **429 and 5xx:** initially treated as full disallow (matches RFC "unreachable"):
  first 12 hours Google stops crawling the site but keeps retrying robots.txt; for the
  next 30 days it uses the **last cached good copy** (if none: no crawl restrictions);
  after 30 days, if the site is generally available Google behaves **as if there is no
  robots.txt** (→ allow all) — a divergence from a naive forever-disallow reading of
  §2.3.1.4, but consistent with its 30-day escape hatch.
- **DNS/network failures** (timeouts, reset connections, chunking errors, invalid
  responses) are treated as server errors (5xx path).

## 9. Caching (§2.4)

- Crawlers MAY cache robots.txt and MAY use standard HTTP cache control (RFC 9111).
- Crawlers **SHOULD NOT use a cached copy for more than 24 hours**, unless the file is
  unreachable (then a stale copy is allowed, cf. §2.3.1.4).
- Google: caches generally up to 24 hours, longer when refreshing isn't possible
  (timeouts/5xx); the cached response may be shared across different crawlers; cache
  lifetime may be raised/lowered by `max-age` Cache-Control headers.

## 10. Limits (§2.5)

- Crawlers SHOULD impose a parsing limit to protect themselves; the limit **MUST be at
  least 500 kibibytes (KiB)** — i.e. everything up to 500 KiB must be parsed.
- Google enforces exactly 500 KiB at FETCH time and ignores content past the
  limit — **crawler-side policy, not parser behavior**: robots.cc itself has
  no total-input cap (confirmed; docs/design/malformed-input.md phase 4), so
  the limit is invisible to this repo's differential surface.

## 11. Security considerations (§3, §1)

robots.txt provides **no access-control guarantees**:

- "These rules are not a form of access authorization" (§1). It is not a substitute
  for content security; use real auth (e.g. HTTP Authentication, RFC 9110) for
  anything sensitive.
- Listing paths in robots.txt **publicly exposes them** and makes them discoverable.
- Parser hardening guidance (§3): the 500 KiB floor doubles as an out-of-memory guard
  (memory management); characters outside the §2.2 grammar should be rejected as
  invalid (invalid characters); treat file content as **untrusted input**.

## 12. Worked examples in the RFC (§5)

- **§5.1** — a four-group file: `*` group (with the ABNF-violating `Disallow: *.gif$`
  line, plus `Disallow: /example/`, `Allow: /publications/`); a `foobot` group showing
  that the space after `:` is optional (`Disallow:/`); a two-token group
  (`barbot` + `bazbot` sharing rules); and an empty `quxbot` group at EOF (allows
  everything for quxbot).
- **§5.2** — longest match: with `Allow: /example/page/` and
  `Disallow: /example/page/disallowed.gif`, the URI
  `example.com/example/page/disallow.gif` MUST be matched by the disallow rule
  (note: an RFC example typo — as literally written the rule does NOT match
  that URI (`disallowed.gif` vs `disallow.gif`); read the URI as
  `…/disallowed.gif`)
  (it has more octets).

## Related material in this repo

- `grammar/rep.ebnf` — the grammar extracted/derived from §2.2.
- `src-google/` — Google's C++ reference parser (the behavior target for the
  bijective matcher; see `docs/TODO.md`).
- `docs/google-dev-docs/` — Google Search Central pages, incl. Google's REP
  interpretation and crawler user-agent lists.

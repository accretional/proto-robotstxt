# Task: docs — documentation bootstrap

## 2026-07-03 (later) — extractor fixed upstream; index refreshed (docs agent)

The coordinator fixed the truncation bug in `tools/google-dev/pull-docs.sh` (gotcha 1
below): the extractor now captures the whole `<article>`/`<main>` region instead of
stopping at the first `</div><devsite-`, and the pull was re-run. Re-verified all 8
pages myself (sizes up e.g. robots_txt 648 lines / meta-tag 453 lines / common
crawlers 351 lines; every page now runs to the devsite "Send feedback" footer; key
content phrase-checked per page — syntax + precedence tables in robots_txt, full
crawler tables in the three crawler-list pages, X-Robots-Tag rules list, etc.).
One check note: "useful robots.txt rules" is genuinely absent from
create-robots-txt.md because it lives on a separate page
(`/crawling/docs/robots-txt/useful-robots-txt-rules`, only a nav link here) — added
as a candidate to TODO item 1, not a truncation.
Updated accordingly: cleared all per-page truncation flags in
`docs/google-dev-docs/README.md` (dropped the "why the truncation happens" section,
kept the fix-tool-not-files rule); trimmed the extractor-bug text from
`docs/TODO.md` item 1 and the stale "crawler-list pages are truncated" note from
item 6. Nothing else touched.

## 2026-07-03 — initial docs/ population (docs agent)

**Scope:** docs/ only (RFC + Google dev-doc knowledgebase, conventions, TODO,
progresslog). Ran (did not modify) `tools/rfc/pull-rfc.sh` and
`tools/google-dev/pull-docs.sh`.

### What was done

- Ran `tools/rfc/pull-rfc.sh` → `docs/rfc/9309/raw.html` (82 KB) + `raw.txt` (25 KB),
  both gitignored as intended. No failures.
- Wrote `docs/rfc/9309/README.md` — implementer's summary of RFC 9309 with section
  citations: full ABNF (§2.2) quoted verbatim from `raw.txt`; user-agent matching /
  group merging (§2.2.1); allow/disallow most-specific(longest)-match + percent-
  encoding normalization (§2.2.2, §5.2); `*`/`$`/`#` specials (§2.2.3); "define
  additional lines" extension point incl. Sitemaps (§2.2.4); access method (§2.3);
  HTTP status handling (§2.3.1) with a table of RFC requirements vs Google's
  documented nuances (429-as-5xx, 12h/30d cached-copy behavior, 30-day allow-all
  fallback, DNS errors as 5xx); caching (§2.4); 500 KiB minimum parse limit (§2.5);
  security non-guarantees (§1, §3); §5 examples.
- Ran `tools/google-dev/pull-docs.sh` → all 8 seed pages fetched OK
  (rawhtml/ + extracted .md each). No fetch failures.
- Wrote `docs/google-dev-docs/README.md` — index (title/URL/description per page),
  CC BY 4.0 attribution, re-run/extend instructions, TODO note about full-tree
  aggregation, and per-page extraction-quality flags (see gotcha below).
- Wrote `docs/TODO.md` seeded with the 6 planned items in priority order
  (full Google-doc aggregation; libprotobuf-mutator+libfuzzer differential fuzzing;
  malformed-input recovery layer; bijective RobotsMatcher; generation/modification
  tooling; machine-readable crawler registry).
- Wrote `docs/README.md` (docs/ conventions + layout) and
  `docs/progresslog/README.md` (progress-log convention), plus this log.

### Gotchas / findings

1. **Extractor truncation bug in `tools/google-dev/pull-docs.sh`** (not fixed — tools/
   is outside this task's scope; flagged in the index and folded into TODO item 1).
   The article-body regex ends the capture at the first `</div>` followed by
   `<devsite-`, but devsite emits `</div><devsite-code>` around every code sample
   *inside* the article body. Result: **6 of 8** extracted .md files are cut at the
   page's first code sample/expandable. Truncated: create-robots-txt, robots_txt,
   robots-meta-tag, and all three crawlers-fetchers list pages (their per-crawler
   tables — the useful part — are lost). Complete: robots-intro,
   overview-google-crawlers (verified by phrase-sampling md vs raw HTML).
   The raw HTML is complete in all cases; the generated .md files were deliberately
   NOT hand-edited (convention: fix tool, re-run).
2. **RFC self-inconsistency worth remembering:** the §5.1 example `Disallow: *.gif$`
   is not derivable from the §2.2 ABNF (`path-pattern = "/" *UTF8-char-noctl`
   requires a leading `/`). Also: `identifier` admits no digits (so `Googlebot/2.1`
   style tokens are non-conforming), and `UTF8-1-noctl`'s `%x24-7F` range includes
   DEL (0x7F) despite the "excluding control" comment. All recorded in
   `docs/rfc/9309/README.md` — directly relevant to TODO item 3 (malformed-input
   recovery for the gluon parser).
3. Google's HTTP-status nuances were grounded against the *raw HTML* of the
   robots_txt interpretation page (the extracted .md is truncated); key facts are
   preserved in `docs/rfc/9309/README.md` §8–10 so they survive even without a
   local rawhtml/ copy.

### State / next steps

- All planned docs written; both pull scripts run cleanly end-to-end and are
  documented as re-runnable.
- Next: TODO item 1 (fix extractor + aggregate the full crawling-indexing tree),
  which also unblocks the crawler-registry data (TODO item 6).
- Not touched (other owners): root README.md, CLAUDE.md, grammar/, src-*/, tools/
  scripts, `docs/progresslog/bootstrap.md`.

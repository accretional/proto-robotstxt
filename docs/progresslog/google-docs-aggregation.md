# Task: google-docs-aggregation — full Google crawling/indexing doc tree

## 2026-07-03 — full-tree aggregation (TODO item 1) (docs-aggregation agent)

**Scope:** `tools/google-dev/**`, `docs/google-dev-docs/**`, this log, TODO item 1.
Superseded the 8-page seed set with the complete tree: **62 pages** across
`/search/docs/crawling-indexing` and `/crawling/docs`, zero fetch failures.

### How discovery works

New script `tools/google-dev/discover-pages.sh`. Devsite pages embed their
section ("book") nav in the static HTML, so the tree is enumerable without a
sitemap: the script BFS-crawls from three seeds (the crawling-indexing landing,
the `/crawling` landing, and `/crawling/docs/about-crawling` — the `/crawling`
landing is a marketing page *without* the book nav, hence the third seed),
extracts `href`s under the two doc prefixes, and enqueues unseen ones until
fixpoint (73 fetches for 62 pages on this run). Normalization before dedup:

- **redirects followed** — the canonical (`url_effective`) URL is emitted;
- anchors (`#...`) and query params (`?hl=<locale>`) stripped;
- asset extensions (`.json`, images, ...) excluded.

Output: one URL per line, sorted, on stdout (log on stderr) — pipe into
`pull-docs.sh -`. `DISCOVER_SLEEP` (default 0.3s) throttles between fetches.

### pull-docs.sh changes

- Input modes: explicit URLs (as before), `-` = URLs from stdin (blank lines/`#`
  comments ok), **no args = run discover-pages.sh and pull the whole tree** (the
  old 8-URL `PAGES` seed array is gone — it was the TODO placeholder).
- Follows redirects and slugs by the *effective* URL, so a moved page lands
  under one canonical slug; the attribution header records both (`Source:` +
  `(requested as: ...)`).
- Per-page failures no longer abort the run (collected, reported, exit 1).
- 0.3s politeness sleep between fetches (`PULL_SLEEP`).

### Extraction issues found / fixed (all in the pull-docs.sh extractor)

1. **Devsite chrome inside `<article>`** leaked into extractions (breadcrumb
   "Home / ... / Docs", "Send feedback", "Stay organized with collections...").
   These elements are consistently marked `class="nocontent"` and/or
   `data-nosnippet` — the extractor now skips any subtree with either marker
   (plus `devsite-feedback`/`devsite-actions`/`button` etc. tags). Skipping is
   now stack-based (tolerates unbalanced markup) instead of per-tag counters.
2. **Code samples** were flattened into prose. `<pre>` regions now emit fenced
   ``` blocks with whitespace preserved verbatim (robots.txt samples, UA
   strings, sitemap XML all survive intact).
3. **Tables** had cells run together. `td`/`th` boundaries now emit ` | `
   separators (rows were already newline-separated), which keeps the
   robots-txt-spec syntax/precedence tables and the crawler UA tables readable.
4. List items now get `- ` bullets; whitespace is normalized outside code fences
   (per-line trim, blank-run collapse).

### Verification

- Discovery output includes the known must-have
  `/crawling/docs/robots-txt/useful-robots-txt-rules`, and the extracted file
  set exactly matches the discovery list (diff-checked).
- Deep spot-check of 12+ pages covering every page type (big-table pages:
  robots-txt-spec, http-status-codes, special-case/common crawlers; code-heavy:
  useful-robots-txt-rules, build-sitemap, robots-meta-tag; long reference:
  video-sitemaps; short pages: ask-google-to-recrawl; new pages: web-bot-auth,
  changelog): every one has a real `# ` title and runs to the last section of
  its article; tables and fences intact.
- One **false alarm worth remembering**: `http-status-codes.md` ends on bare
  `502`/`503` rows — that matches the raw HTML (Google's table genuinely has
  description-less rows there), not a truncation. Checked against
  `rawhtml/crawling-docs-troubleshooting-http-status-codes.html`.
- Known cosmetic artifact (not fixed, harmless): a stray `|` on its own line
  where a table cell's content starts with a block element.

### Gotchas

- **Google is mid-restructure**: robots.txt and crawler pages are moving from
  `/search/docs/crawling-indexing/...` to `/crawling/docs/...`, and old URLs
  301 to the new homes. Different pages serve *different cached navs* (some
  still list the old URLs), so discovery canonicalizes through redirects and
  dedupes — otherwise the same page appears under two slugs. Three previously
  committed files were stale duplicates under old slugs and were deleted:
  `search-docs-crawling-indexing-robots-robots_txt.md` (→
  `crawling-docs-robots-txt-robots-txt-spec.md`),
  `...-robots-create-robots-txt.md` (→ `crawling-docs-robots-txt-create-robots-txt.md`),
  `...-overview-google-crawlers.md` (→
  `crawling-docs-crawlers-fetchers-overview-google-crawlers.md`). No file
  outside `docs/google-dev-docs/` referenced the old slugs (grep-checked).
- Some in-tree URLs redirect *out* of the doc tree (e.g.
  `/search/docs/crawling-indexing/safesearch` → `/search/docs/specialty/...`);
  the prefix filter is applied to the post-redirect URL, so these drop out.

### How to refresh the knowledgebase

```sh
tools/google-dev/pull-docs.sh        # no args: discover + pull all (~2×60 fetches, throttled)
# or: tools/google-dev/discover-pages.sh | tools/google-dev/pull-docs.sh -
```

Then: delete committed `.md` files whose slug is no longer in the discovery
output (Google moved/removed the page), update the index in
`docs/google-dev-docs/README.md` (the changelog page,
`crawling-docs-changelog.md`, summarizes what changed on Google's side), and
spot-check a few table/code-heavy pages against `rawhtml/`.

### State / next steps

- Done end-to-end; TODO item 1 moved to Done. The crawler-registry data for
  TODO item 6 (three crawler-list pages + now also feedfetcher/read-aloud/
  apis-user-agent pages) is fully extracted and table-intact.
- Not touched (other owners): `docs/progresslog/docs.md` (predecessor log),
  `docs/rfc/`, root README/CLAUDE.md, run/test scripts.

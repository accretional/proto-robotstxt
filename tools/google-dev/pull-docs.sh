#!/usr/bin/env bash
# pull-docs.sh — pull Google Search Central crawling/indexing documentation
# into docs/google-dev-docs/ as a local, grounded knowledgebase.
#
# Content on developers.google.com is licensed CC BY 4.0
# (https://developers.google.com/terms/site-policies). We comply by:
#   * keeping the source URL + license attribution header on every saved file
#   * not modifying the extracted text beyond format conversion (HTML -> text)
# Raw HTML goes to docs/google-dev-docs/rawhtml/ (git-ignored by default —
# rerun this script to rematerialize); extracted text goes to
# docs/google-dev-docs/<slug>.md with an attribution header.
#
# The page list below seeds the robots.txt-relevant pages. TODO(docs/TODO.md):
# crawl the full https://developers.google.com/search/docs/crawling-indexing
# tree (and https://developers.google.com/crawling) and aggregate everything.
#
# Usage: tools/google-dev/pull-docs.sh [url ...]   (no args: fetch PAGES below)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${REPO_ROOT}/docs/google-dev-docs"
RAW_DIR="${OUT_DIR}/rawhtml"
mkdir -p "${RAW_DIR}"

PAGES=(
  "https://developers.google.com/search/docs/crawling-indexing/robots/intro"
  "https://developers.google.com/search/docs/crawling-indexing/robots/create-robots-txt"
  "https://developers.google.com/search/docs/crawling-indexing/robots/robots_txt"
  "https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag"
  "https://developers.google.com/search/docs/crawling-indexing/overview-google-crawlers"
  "https://developers.google.com/crawling/docs/crawlers-fetchers/google-common-crawlers"
  "https://developers.google.com/crawling/docs/crawlers-fetchers/google-special-case-crawlers"
  "https://developers.google.com/crawling/docs/crawlers-fetchers/google-user-triggered-fetchers"
)

urls=("$@")
[ ${#urls[@]} -eq 0 ] && urls=("${PAGES[@]}")

slugify() {
  # https://developers.google.com/search/docs/x/y -> search-docs-x-y
  printf '%s' "$1" | sed -E 's#^https?://developers\.google\.com/##; s#[/]+#-#g; s#[^a-zA-Z0-9._-]##g'
}

extract_text() {
  # HTML -> markdown-ish text on stdout. Prefers the article body div used by
  # devsite; falls back to whole-page text.
  python3 - "$1" <<'PYEOF'
import re, sys, html
from html.parser import HTMLParser

src = open(sys.argv[1], encoding='utf-8', errors='replace').read()

class Text(HTMLParser):
    SKIP = {'script', 'style', 'nav', 'header', 'footer', 'aside', 'devsite-toc'}
    BLOCK = {'p','div','li','tr','h1','h2','h3','h4','h5','h6','pre','br','section','article','table','ul','ol'}
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.out, self.skip, self.pre = [], 0, 0
    def handle_starttag(self, tag, attrs):
        if tag in self.SKIP: self.skip += 1
        if tag == 'pre': self.pre += 1
        if tag in self.BLOCK: self.out.append('\n')
        if tag.startswith('h') and len(tag) == 2 and tag[1].isdigit():
            self.out.append('\n' + '#' * int(tag[1]) + ' ')
    def handle_endtag(self, tag):
        if tag in self.SKIP: self.skip = max(0, self.skip - 1)
        if tag == 'pre': self.pre = max(0, self.pre - 1)
        if tag in self.BLOCK: self.out.append('\n')
    def handle_data(self, data):
        if self.skip: return
        self.out.append(data if self.pre else re.sub(r'\s+', ' ', data))

# Prefer the whole <article>/<main> region (devsite nests code samples in
# divs, so a non-greedy "first </div>" match truncates at the first sample);
# the Text parser below drops nav/toc/script noise itself.
m = re.search(r'<article[^>]*>(.*?)</article>', src, re.S) \
    or re.search(r'<main[^>]*>(.*?)</main>', src, re.S) \
    or re.search(r'<body[^>]*>(.*?)</body>', src, re.S)
body = m.group(1) if m else src
t = Text(); t.feed(body)
text = re.sub(r'\n{3,}', '\n\n', ''.join(t.out)).strip()
print(text)
PYEOF
}

for url in "${urls[@]}"; do
  slug="$(slugify "${url}")"
  raw="${RAW_DIR}/${slug}.html"
  out="${OUT_DIR}/${slug}.md"
  echo "[pull-docs] ${url}"
  curl -fsSL "${url}" -o "${raw}"
  {
    echo "<!--"
    echo "  Source: ${url}"
    echo "  Fetched: $(date -u +%Y-%m-%dT%H:%M:%SZ) by tools/google-dev/pull-docs.sh"
    echo "  Text content © Google, licensed under CC BY 4.0"
    echo "  (https://creativecommons.org/licenses/by/4.0/), per"
    echo "  https://developers.google.com/terms/site-policies. Converted from"
    echo "  HTML to text; content otherwise unmodified. Code samples are"
    echo "  Apache-2.0 per the same policy."
    echo "-->"
    echo
    extract_text "${raw}"
  } > "${out}"
  echo "[pull-docs]   -> ${out}"
done

echo "[pull-docs] done. Extracted pages:"
ls "${OUT_DIR}"/*.md 2>/dev/null | sed 's/^/    /'

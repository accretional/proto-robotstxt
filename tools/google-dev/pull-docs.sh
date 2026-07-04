#!/usr/bin/env bash
# pull-docs.sh — pull Google Search Central / Crawling-infrastructure
# documentation into docs/google-dev-docs/ as a local, grounded knowledgebase.
#
# Content on developers.google.com is licensed CC BY 4.0
# (https://developers.google.com/terms/site-policies). We comply by:
#   * keeping the source URL + license attribution header on every saved file
#   * not modifying the extracted text beyond format conversion (HTML -> text)
# Raw HTML goes to docs/google-dev-docs/rawhtml/ (git-ignored by default —
# rerun this script to rematerialize); extracted text goes to
# docs/google-dev-docs/<slug>.md with an attribution header.
#
# Usage:
#   tools/google-dev/pull-docs.sh              # full tree: discover-pages.sh -> pull everything
#   tools/google-dev/pull-docs.sh <url> [...]  # pull specific pages
#   ... | tools/google-dev/pull-docs.sh -      # read URLs from stdin (one per line, # comments ok)
#
# Redirects are followed and the slug/attribution use the *effective* URL, so
# a moved page (Google 301s old /search/docs/crawling-indexing/robots/* URLs
# to /crawling/docs/robots-txt/*) lands under one canonical slug.
#
# Env: PULL_SLEEP  seconds between fetches (default 0.3)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUT_DIR="${REPO_ROOT}/docs/google-dev-docs"
RAW_DIR="${OUT_DIR}/rawhtml"
SLEEP="${PULL_SLEEP:-0.3}"
mkdir -p "${RAW_DIR}"

urls=()
if [ $# -eq 0 ]; then
  echo "[pull-docs] no URLs given - discovering the full doc tree via discover-pages.sh"
  while IFS= read -r line; do urls+=("${line}"); done \
    < <("${SCRIPT_DIR}/discover-pages.sh")
elif [ $# -eq 1 ] && [ "$1" = "-" ]; then
  while IFS= read -r line; do
    line="${line%%#*}"                      # strip comments
    line="$(printf '%s' "${line}" | tr -d '[:space:]')"
    [ -n "${line}" ] && urls+=("${line}")
  done
else
  urls=("$@")
fi
[ ${#urls[@]} -gt 0 ] || { echo "[pull-docs] no URLs to pull" >&2; exit 1; }

slugify() {
  # https://developers.google.com/search/docs/x/y -> search-docs-x-y
  printf '%s' "$1" | sed -E 's/[?#].*$//; s#^https?://developers\.google\.com/##; s#/+$##; s#[/]+#-#g; s#[^a-zA-Z0-9._-]##g'
}

extract_text() {
  # HTML -> markdown-ish text on stdout. Prefers the article body region used
  # by devsite; falls back to whole-page text.
  python3 - "$1" <<'PYEOF'
import re, sys
from html.parser import HTMLParser

src = open(sys.argv[1], encoding='utf-8', errors='replace').read()

FENCE = '\x00FENCE\x00'   # placeholder for ``` so cleanup can find code blocks

class Text(HTMLParser):
    # Tags whose subtree is never article text. Attribute-based skipping (the
    # devsite chrome inside <article>: breadcrumbs, feedback buttons, bookmark
    # tooltips) is handled via class="nocontent" / data-nosnippet in _skippable.
    SKIP = {'script', 'style', 'nav', 'header', 'footer', 'aside',
            'devsite-toc', 'devsite-feedback', 'devsite-thumb-rating',
            'devsite-actions', 'devsite-page-rating', 'button'}
    BLOCK = {'p','div','li','tr','h1','h2','h3','h4','h5','h6','pre','br',
             'section','article','table','ul','ol','dt','dd'}
    VOID = {'br','hr','img','input','meta','link','source','wbr','area','base','col','embed','track','param'}
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.out, self.stack, self.skip, self.pre, self.cell = [], [], 0, 0, 0
    def _skippable(self, tag, attrs):
        if tag in self.SKIP:
            return True
        a = dict(attrs)
        if 'data-nosnippet' in a:
            return True
        return 'nocontent' in (a.get('class') or '')
    def handle_starttag(self, tag, attrs):
        if tag in self.VOID:
            if tag == 'br' and not self.skip: self.out.append('\n')
            return
        skipping = self._skippable(tag, attrs)
        self.stack.append((tag, skipping))
        if skipping:
            self.skip += 1
            return
        if self.skip: return
        if tag == 'pre':
            self.pre += 1
            if self.pre == 1: self.out.append('\n' + FENCE + '\n')
            return
        if self.pre: return
        if tag in self.BLOCK: self.out.append('\n')
        if tag == 'li': self.out.append('- ')
        if tag == 'tr': self.cell = 0
        if tag in ('td', 'th'):
            if self.cell: self.out.append(' | ')
            self.cell += 1
        if len(tag) == 2 and tag[0] == 'h' and tag[1].isdigit():
            self.out.append('\n' + '#' * int(tag[1]) + ' ')
    def handle_endtag(self, tag):
        if tag in self.VOID: return
        if tag not in (t for t, _ in self.stack): return  # stray close
        while self.stack:
            t, skipping = self.stack.pop()
            if skipping:
                self.skip = max(0, self.skip - 1)
            elif t == 'pre':
                self.pre = max(0, self.pre - 1)
                if self.pre == 0 and not self.skip: self.out.append('\n' + FENCE + '\n')
            elif not self.skip and t in self.BLOCK:
                self.out.append('\n')
            if t == tag: break
    def handle_data(self, data):
        if self.skip: return
        self.out.append(data if self.pre else re.sub(r'\s+', ' ', data))

# Take the whole <article>/<main> region (devsite nests code samples in divs,
# so any "first </div>" heuristic truncates at the first sample); the Text
# parser drops nav/chrome noise itself.
m = re.search(r'<article[^>]*>(.*?)</article>', src, re.S) \
    or re.search(r'<main[^>]*>(.*?)</main>', src, re.S) \
    or re.search(r'<body[^>]*>(.*?)</body>', src, re.S)
body = m.group(1) if m else src
t = Text(); t.feed(body)
raw = ''.join(t.out)

# Whitespace cleanup outside code fences; fences become ``` markers.
parts = raw.split(FENCE)
clean = []
for i, part in enumerate(parts):
    if i % 2:  # inside a code block: keep verbatim, minus outer blank lines
        clean.append('```\n' + part.strip('\n') + '\n```')
    else:
        part = '\n'.join(line.strip() for line in part.split('\n'))
        part = re.sub(r'\n{3,}', '\n\n', part)
        clean.append(part.strip('\n'))
text = re.sub(r'\n{3,}', '\n\n', '\n'.join(clean)).strip()
print(text)
PYEOF
}

ok=0
failed=()
for url in "${urls[@]}"; do
  echo "[pull-docs] ${url}"
  tmp="$(mktemp)"
  if ! eff="$(curl -fsSL --connect-timeout 15 --max-time 120 \
              -w '%{url_effective}' -o "${tmp}" "${url}")"; then
    echo "[pull-docs]   FETCH FAILED: ${url}" >&2
    failed+=("${url}")
    rm -f "${tmp}"
    continue
  fi
  eff="${eff%%\#*}"; eff="${eff%%\?*}"
  slug="$(slugify "${eff}")"
  raw="${RAW_DIR}/${slug}.html"
  out="${OUT_DIR}/${slug}.md"
  mv "${tmp}" "${raw}"
  {
    echo "<!--"
    echo "  Source: ${eff}"
    if [ "${eff}" != "${url}" ]; then echo "  (requested as: ${url})"; fi
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
  ok=$((ok + 1))
  sleep "${SLEEP}"
done

echo "[pull-docs] done: ${ok} pages extracted into ${OUT_DIR}"
if [ ${#failed[@]} -gt 0 ]; then
  echo "[pull-docs] FAILED (${#failed[@]}):" >&2
  printf '    %s\n' "${failed[@]}" >&2
  exit 1
fi

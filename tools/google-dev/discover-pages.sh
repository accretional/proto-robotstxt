#!/usr/bin/env bash
# discover-pages.sh — enumerate every doc page of the Google crawling/indexing
# documentation tree, one canonical URL per line on stdout (progress on stderr).
#
# Covered sections (the two doc-path prefixes we aggregate):
#   /search/docs/crawling-indexing   Search Central: crawling & indexing
#   /crawling/docs                   Crawling infrastructure (crawlers-fetchers,
#                                    robots-txt, troubleshooting, ...)
#
# How it works: devsite pages embed their section ("book") nav in the static
# HTML, so the full tree is reachable within a hop or two of the landing pages.
# This script does a small BFS anyway — fetch page, extract hrefs under the two
# prefixes, enqueue new ones — because individual pages carry slightly
# different (cached / mid-restructure) navs and some pages are only linked
# from article bodies. URLs are normalized before dedup:
#   * redirects followed -> the canonical URL is what gets emitted
#     (Google is moving robots.txt pages from /search/docs/crawling-indexing
#     to /crawling/docs, and old URLs 301 to the new homes)
#   * anchors (#...) and query params (?hl=<locale>, ...) stripped
#   * non-doc assets (.json/.xml/images/...) excluded
#
# Usage:
#   tools/google-dev/discover-pages.sh                       # list to stdout
#   tools/google-dev/discover-pages.sh | tools/google-dev/pull-docs.sh -
#
# Env: DISCOVER_SLEEP  seconds between fetches (default 0.3)
set -euo pipefail
export LC_ALL=C

BASE="https://developers.google.com"
PREFIX_RE='^/(search/docs/crawling-indexing|crawling/docs)(/|$)'
ASSET_RE='\.(json|xml|txt|csv|png|jpe?g|gif|svg|webp|ico|pdf|zip|js|css)$'
SEEDS=(
  "/search/docs/crawling-indexing"  # Search Central section landing (carries its book nav)
  "/crawling"                       # Crawling-infrastructure landing (marketing page linking into /crawling/docs)
  "/crawling/docs/about-crawling"   # a /crawling doc page (carries the full /crawling book nav)
)
SLEEP="${DISCOVER_SLEEP:-0.3}"

wd="$(mktemp -d)"
trap 'rm -rf "${wd}"' EXIT
touch "${wd}/visited" "${wd}/discovered"
printf '%s\n' "${SEEDS[@]}" > "${wd}/queue"

# normalize an href to a site-absolute path: strip scheme+host, ?query, #anchor,
# trailing slashes. Emits nothing if the href points off-site.
normalize() {
  printf '%s\n' "$1" \
    | sed -E "s#^${BASE}##" \
    | grep -E '^/' \
    | sed -E 's/[?#].*$//; s:/+$::' \
    || true
}

fetched=0
while [ -s "${wd}/queue" ]; do
  path="$(head -n 1 "${wd}/queue")"
  tail -n +2 "${wd}/queue" > "${wd}/queue.tmp" && mv "${wd}/queue.tmp" "${wd}/queue"
  grep -qxF "${path}" "${wd}/visited" && continue
  echo "${path}" >> "${wd}/visited"

  if ! eff_url="$(curl -fsSL --connect-timeout 15 --max-time 60 \
                  -w '%{url_effective}' -o "${wd}/page.html" "${BASE}${path}")"; then
    echo "[discover] WARN: fetch failed: ${BASE}${path}" >&2
    continue
  fi
  fetched=$((fetched + 1))
  eff_path="$(normalize "${eff_url}")"
  if [ "${eff_path}" != "${path}" ]; then
    echo "[discover] ${path} -> ${eff_path}" >&2
  else
    echo "[discover] ${path}" >&2
  fi

  # Record the canonical page (if it is a doc page under our prefixes) and
  # mark it visited so a later nav link to the canonical URL isn't refetched.
  if [ -n "${eff_path}" ] \
     && printf '%s' "${eff_path}" | grep -qE "${PREFIX_RE}" \
     && ! printf '%s' "${eff_path}" | grep -qE "${ASSET_RE}"; then
    grep -qxF "${eff_path}" "${wd}/discovered" || echo "${eff_path}" >> "${wd}/discovered"
    grep -qxF "${eff_path}" "${wd}/visited"    || echo "${eff_path}" >> "${wd}/visited"
  fi

  # Extract candidate links and enqueue unseen ones.
  grep -oE 'href="[^"]*"' "${wd}/page.html" 2>/dev/null \
    | sed -E 's/^href="//; s/"$//' \
    | sed -E "s#^${BASE}##" \
    | grep -E '^/' \
    | sed -E 's/[?#].*$//; s:/+$::' \
    | grep -E "${PREFIX_RE}" \
    | grep -vE "${ASSET_RE}" \
    | sort -u \
    | while IFS= read -r p; do
        grep -qxF "${p}" "${wd}/visited" || grep -qxF "${p}" "${wd}/queue" \
          || echo "${p}" >> "${wd}/queue"
      done || true   # no in-scope links on a page is fine (pipefail)

  sleep "${SLEEP}"
done

count="$(sort -u "${wd}/discovered" | grep -c . || true)"
echo "[discover] done: ${count} doc pages (${fetched} URLs fetched)" >&2
sort -u "${wd}/discovered" | sed "s#^#${BASE}#"

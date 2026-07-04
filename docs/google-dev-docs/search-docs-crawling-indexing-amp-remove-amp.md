<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/amp/remove-amp
  Fetched: 2026-07-04T06:22:59Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Remove your AMP pages from Google Search

This page describes how web developers can remove their AMP pages from Google Search.

There are three options for removing AMP content:

- Quickly remove all versions of a page, including the AMP and canonical non-AMP pages

- Remove only the AMP page that is paired with a canonical non-AMP page, while keeping the canonical non-AMP page live

- Remove AMP content with a CMS (with options to remove all versions of a page, or only the AMP version)

## Remove all versions of AMP content, including AMP and non-AMP

This section describes how to remove all versions of AMP content from Google Search, which includes AMP and non-AMP pages.

To remove AMP and non-AMP pages from Google Search, follow these steps:

- Delete the AMP and non-AMP versions of the page from your server or CMS.

- Use the Remove outdated content tool to request the removal of your page. Enter the URLs (web addresses) to your AMP and non-AMP versions of the page that you want to remove.

- Verify the removal of your AMP page by searching for your content using Google Search. To verify the removal of a large number of AMP pages, use the AMP status report in Search Console. Watch for a decreasing trendline on the "indexed AMP pages" graph.

You can check the status of your request on the Remove outdated content page.

## Remove only AMP pages, while preserving the canonical non-AMP pages

This section describes how to remove only AMP pages from Google Search, while still preserving your canonical non-AMP pages.

To remove the AMP version of your page from Google Search (while preserving the canonical non-AMP page), follow these steps:

- Remove the rel="amphtml" link from the canonical non-AMP page in the source code.

- Configure your server to return either an HTTP 301 Moved Permanently or 302 Found response for the removed AMP page.

- Configure a redirect from the removed AMP page to the canonical non-AMP page.

- If you want to remove an AMP page from non-Google platforms in addition to removing from Google Search, complete these steps:

- Remove your AMP page so that it is no longer accessible by configuring your server to send an HTTP 404 Not Found for your removed AMP page.

- Verify the removal of your AMP page by searching for your content in Google Search. To verify removal of a large number of AMP pages, use the AMP status report in Search Console. Watch for a decreasing trendline on the "indexed AMP pages" graph.

- If you want to keep permalinks active, configure your server to send an HTTP 301 Redirect for your removed AMP page to your canonical non-AMP page.

## Remove AMP and non-AMP pages with a CMS

Generally, CMS providers publish AMP and non-AMP pages at the same time. To remove a single page, unpublish or delete the page, which removes both the AMP and non-AMP versions of that page.

### Delete a single page

To delete a page and stop publishing it in both its AMP and non-AMP forms, use the CMS interface. Check your CMS provider's help page for details on how to stop serving AMP:

- WordPress.com help

- Drupal help

- SquareSpace help

### Remove all AMP pages

Another option is to disable AMP from your CMS.

To disable AMP, check your CMS provider's help page or contact your CMS provider. If your site is hosted on a CMS domain, the CMS can redirect users to the canonical non-AMP page after AMP is disabled. If the redirect doesn't occur, contact your CMS provider for assistance.

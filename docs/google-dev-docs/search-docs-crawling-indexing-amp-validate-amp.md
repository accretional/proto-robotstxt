<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/amp/validate-amp
  Fetched: 2026-07-04T06:23:00Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Validate your AMP content

After you've created AMP content, here are some ways to validate your AMP content:

- Use the AMP Test Tool to ensure that your AMP content is valid.

- For applicable AMP content types, use the Rich Results Test to verify that your structured data parses properly.

- Use the AMP status report to monitor the performance of all AMP pages on your site.

## Fix common AMP errors

If your AMP page doesn't appear in Google Search, try the following steps:

- Make your page discoverable by linking your pages.

- Did you add rel="amphtml" to the canonical page?

- Did you add rel="amphtml" to other non-AMP pages (for example, mobile)?

- Did you add rel="canonical" to the AMP page?

- Follow the Google Search guidelines for AMP pages.

- Make your AMP content accessible to Googlebot:

- Edit your site's robots.txt to allow Googlebot to crawl the canonical page, AMP page, and links in the structured data (if applicable).

- Remove all robots meta tags and X-Robots-Tag HTTP headers from your canonical and AMP content. For more information, see Robots meta tag and X-Robots-Tag HTTP header specifications.

- Ensure that your structured data follows the structured data guidelines for your page and feature type. For more information about structured data requirements for AMP, see About AMP on Google Search.

If your AMP page still isn't appearing in Google Search after completing the steps, here are some additional reasons:

- Certain Google Search features might not be available in your country.

- Your site might not be indexed yet. For more information about crawling and indexing, see the Crawling and indexing FAQ.

## Resources

To debug validation errors, see the following ampproject.org resources:

- AMP validation errors

- How do I fix validation errors?

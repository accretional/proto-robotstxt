<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/sitemaps/large-sitemaps
  Fetched: 2026-07-04T06:23:31Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Manage your sitemaps with a sitemap index file

If you have a sitemap that exceeds the size limits, you'll need to split up your large sitemap into multiple sitemaps such that each new sitemap is below the size limit. Once you've split up your sitemap, you can use a sitemap index file as a way to submit many sitemaps at once.

## Sitemap index best practices

The XML format of a sitemap index file is very similar to the XML format of a sitemap file, and it's defined by the Sitemap Protocol. This means that all the sitemap requirements apply to sitemap index files also.

The referenced sitemaps must be hosted on the same site as your sitemap index file. This requirement is waived if you set up cross-site submission.

Sitemaps that are referenced in the sitemap index file must be in the same directory as the sitemap index file, or lower in the site hierarchy. For example, if the sitemap index file is at https://example.com/public/sitemap_index.xml, it can only contain sitemaps that are in the same or deeper directory, like https://example.com/public/shared/....

You can submit up to 500 sitemap index files for each site in your Search Console account.

## Example sitemap index

The following example shows a sitemap index in XML format that lists two sitemaps:
```
<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://www.example.com/sitemap1.xml.gz</loc>
    <lastmod>2024-08-15</lastmod>
  </sitemap>
  <sitemap>
    <loc>https://www.example.com/sitemap2.xml.gz</loc>
    <lastmod>2022-06-05</lastmod>
  </sitemap>
</sitemapindex>
```
## Sitemap index reference

The sitemap index tags are defined by the same namespace as generic sitemaps: http://www.sitemaps.org/schemas/sitemap/0.9

To make sure Google can use your sitemap index, you must use the following required tags:

Required tags

sitemapindex  | The root tag of the XML tree. It contains all the other tags.

sitemap  |  The parent tag for each sitemap listed in the file. It's the only direct child of the sitemapindex tag.

loc  |  The location (URL) of the sitemap. It's a child of the sitemap tag. A sitemap index file may have up to 50,000 loc tags.

Additionally, the following optional tags may help Google schedule your sitemaps for crawling:

Optional tags

lastmod  |  Identifies the time that the corresponding sitemap file was modified. It can be a child of a sitemap tag. The value for the lastmod tag must be in W3C Datetime format.

## Troubleshooting sitemaps

If you're having trouble with your sitemap, you can investigate the errors with Google Search Console. See Search Console's sitemaps troubleshooting guide for help.

##  Additional resources

Want to learn more? Check out the following resources:

-  Submit your sitemap to Google

-  Learn how to combine sitemap extensions

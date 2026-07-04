<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/sitemaps/combine-sitemap-extensions
  Fetched: 2026-07-04T06:23:29Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# How to combine sitemap extensions

Sitemap extensions are a great way to tell Google about the different kinds of content and their metadata that you're using on your site. Often the content on your pages may fit into multiple kinds of extensions; for example, you might be publishing news articles that embed images and videos. Additionally, your pages may be localized as well, which might mean that you could add hreflang annotations for your localized pages.

## Namespaces

For each sitemap extension that you want to use in a sitemap you need to specify the respective namespace that declares the tags the extension supports. This is done with the xmlns attribute of the urlset tag. The namespaces for the sitemap extensions Google supports are:

Extension tags and their namespace definitions

image:  |  http://www.google.com/schemas/sitemap-image/1.1

news:  |  http://www.google.com/schemas/sitemap-news/0.9

video:  |  http://www.google.com/schemas/sitemap-video/1.1

xhtml: (for hreflang)  |  http://www.w3.org/1999/xhtml

### Declaring multiple namespaces

To declare multiple namespaces, add the respective namespace references to your sitemap as described in the documentation of the respective extensions. Here's an example that shows how to add the news, video, and xhtml (for hreflang) extensions to a sitemap:
```
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
           xmlns:news="http://www.google.com/schemas/sitemap-news/0.9"
           xmlns:video="http://www.google.com/schemas/sitemap-video/1.1"
           xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
<!-- rest of the sitemap -->
```
## Combining sitemap extensions

Once you declared the namespaces, follow the implementation details from the respective sitemap extension documentation you're going to use.

- Image sitemap

- News sitemap

- Video sitemap

-  hreflang

To combine extensions, add the tags from whatever sitemap extensions you're using, one after the other, to the appropriate <url> tag, as described in each sitemap extension's documentation.

For example, to add news, video, and xhtml (hreflang) extensions to a sitemap:
```
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
    xmlns:news="http://www.google.com/schemas/sitemap-news/0.9"
    xmlns:video="http://www.google.com/schemas/sitemap-video/1.1"
    xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>https://www.example.com/english/page.html</loc>
    <!-- Starting with the news extension tags -->
    <news:news>
      <news:publication>
        <news:name>The Example Times</news:name>
        <news:language>en</news:language>
      </news:publication>
      <news:publication_date>2008-12-23</news:publication_date>
      <news:title>Companies A, B in Merger Talks</news:title>
    </news:news>
    <!-- Next we add video extension tags -->
    <video:video>
      <video:thumbnail_loc>https://www.example.com/thumbs/123.jpg</video:thumbnail_loc>
      <video:title>Lizzi is painting the wall</video:title>
      <video:description>
        Gary is watching the paint dry on the wall Lizzi painted.
      </video:description>
      <video:player_loc>
        https://player.example.com/video/987654321
      </video:player_loc>
    </video:video>
    <!-- And finally the xhtml tags for hreflang -->
    <xhtml:link
                rel="alternate"
                hreflang="de"
                href="https://www.example.de/deutsch/page.html"/>
    <xhtml:link
                rel="alternate"
                hreflang="de-ch"
                href="https://www.example.de/schweiz-deutsch/page.html"/>
    <xhtml:link
                rel="alternate"
                hreflang="en"
                href="https://www.example.com/english/page.html"/>
  </url>
<!-- Add more <url> tags -->
```
The order of the extension in the sitemap is irrelevant after the <loc> tag. Keep in mind the general sitemap best practices, especially the file size limits. Combining sitemap extensions increases the file size of your sitemap significantly.

## Troubleshooting sitemaps

If you're having trouble with your sitemap, you can investigate the errors with Google Search Console. See Search Console's sitemaps troubleshooting guide for help.

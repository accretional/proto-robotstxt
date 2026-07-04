<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/sitemaps/image-sitemaps
  Fetched: 2026-07-04T06:23:30Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Image sitemaps

Image sitemaps are a way of telling Google about other images on your site, especially those that we might not otherwise find (such as images your site reaches with JavaScript code). You can create a separate image sitemap or add image sitemap tags to your existing sitemap; either approach is equally fine for Google.

Image sitemaps are based on generic sitemaps so the general sitemap best practices also apply to image sitemaps. We also recommend that you follow the general best practices for publishing images.

## Example image sitemap

The following example shows a regular sitemap with image sitemap extension, with two <url> elements:

- https://example.com/sample1.html, which contains two images

- https://example.com/sample2.html, which contains one image
```
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
    xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">
  <url>
    <loc>https://example.com/sample1.html</loc>
    <image:image>
      <image:loc>https://example.com/image.jpg</image:loc>
    </image:image>
    <image:image>
      <image:loc>https://example.com/photo.jpg</image:loc>
    </image:image>
  </url>
  <url>
    <loc>https://example.com/sample2.html</loc>
    <image:image>
      <image:loc>https://example.com/picture.jpg</image:loc>
    </image:image>
  </url>
</urlset>
```
## Image sitemap reference

The image tags are defined in the Image Sitemaps namespace: http://www.google.com/schemas/sitemap-image/1.1

To make sure Google can use your image sitemap, you must use the following required tags:

Required tags

<image:image>  |  Encloses all information about a single image. Each <url> tag can contain up to 1,000 <image:image> tags.

<image:loc>  |
The URL of the image.

In some cases, the image URL may not be on the same domain as your main site. This is fine, as long as you verify both domains in Search Console. If, for example, you use a content delivery network such as Google Sites to host your images, make sure that the hosting site is verified in Search Console. In addition, make sure that your robots.txt file doesn't disallow the crawling of any content you want indexed.

### Deprecated tags and attributes

We removed the following tags and attributes from our documentation: <image:caption>, <image:geo_location>, <image:title>, <image:license>. See the deprecation announcement for more information.

## Troubleshooting sitemaps

If you're having trouble with your sitemap, you can investigate the errors with Google Search Console. See Search Console's sitemaps troubleshooting guide for help.

##  Additional resources

Want to learn more? Check out the following resources:

-  Submit your sitemap to Google

-  Learn how to combine sitemap extensions

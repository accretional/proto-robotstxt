<!--
  Source: https://developers.google.com/crawling/docs/robots-txt/useful-robots-txt-rules
  Fetched: 2026-07-04T06:22:50Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Useful robots.txt rules

Here are some common useful robots.txt rules:

Useful rules

Disallow crawling of the entire site   |
Keep in mind that in some situations URLs from the site may still be indexed, even if they haven't been crawled.
```
User-agent: *
Disallow: /
```
Allow crawling of an entire site (with an empty Disallow rule)   |
This explicitly allows all crawlers to access the entire site. It is functionally equivalent to having no robots.txt file at all, or using an Allow: / rule.
```
User-agent: *
Disallow:
```
Disallow crawling of a directory and its contents   |
Append a forward slash to the directory name to disallow crawling of a whole directory.
```
User-agent: *
Disallow: /calendar/
Disallow: /junk/
Disallow: /books/fiction/contemporary/
```
Disallow crawling of a single web page
|
For example, disallow the useless_file.html page located at https://example.com/useless_file.html, and other_useless_file.html in the junk directory.
```
User-agent: *
Disallow: /useless_file.html
Disallow: /junk/other_useless_file.html
```
Disallow crawling of the whole site except a subdirectory
|
Crawlers may only access the public subdirectory.
```
User-agent: *
Disallow: /
Allow: /public/
```
Allow access to a single crawler   |
Only Googlebot-News may crawl the whole site.
```
User-agent: Googlebot-News
Allow: /

User-agent: *
Disallow: /
```
Allow access to all but a single crawler   |
Unnecessarybot may not crawl the site, all other bots may.
```
User-agent: Unnecessarybot
Disallow: /

User-agent: *
Allow: /
```
Disallow crawling of an entire site, but allow Storebot-Google
|
This implementation hides your pages from Google Search results, but the Storebot-Google web crawler can still analyze them to show your products on Google Shopping.
```
User-agent: *
Disallow: /

User-agent: Storebot-Google
Allow: /
```
Block all images on your site from Google (includes anywhere images are displayed on Google, including Google Images and Discover)
|
Google can't index images and videos without crawling them.
```
User-agent: Googlebot-Image
Disallow: /
```
Block a specific image from Google Images
|
For example, disallow the dogs.jpg image.
```
User-agent: Googlebot-Image
Disallow: /images/dogs.jpg
```
Disallow crawling of files of a specific file type
|
For example, disallow for crawling all .gif files.
```
User-agent: Googlebot
Disallow: /*.gif$
```
Use the * and $ wildcards to match URLs that end with a specific string   |
For example, disallow all .xls files:
```
User-agent: Googlebot
Disallow: /*.xls$
```
Combine multiple user agents in a single group   |
Consolidating rules for multiple crawlers into one group makes the file shorter and easier to manage, as all rules in the group apply to every user agent listed. This is the same as listing the user agents twice with the respective rules.
```
User-agent: Googlebot
User-agent: Storebot-Google
Allow: /cats
Disallow: /
```

<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/block-indexing
  Fetched: 2026-07-04T06:23:02Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Block Search indexing with noindex

noindex is a rule set with either a <meta> tag or HTTP response header and is used to prevent indexing content by search engines that support the noindex rule, such as Google. When Googlebot crawls that page and extracts the tag or header, Google will drop that page entirely from Google Search results, regardless of whether other sites link to it.

Using noindex is useful if you don't have root access to your server, as it allows you to control access to your site on a page-by-page basis.

## Implementing noindex

There are two ways to implement noindex: as a <meta> tag and as an HTTP response header. They have the same effect; choose the method that is more convenient for your site and appropriate for the content type. Specifying the noindex rule in the robots.txt file is not supported by Google.

You can also combine the noindex rule with other rules that control indexing. For example, you can join a nofollow hint with a noindex rule: <meta name="robots" content="noindex, nofollow" />.

### <meta> tag

To prevent all search engines that support the noindex rule from indexing a page on your site, place the following <meta> tag into the <head> section of your page:
```
<meta name="robots" content="noindex">
```
To prevent only Google web crawlers from indexing a page:
```
<meta name="googlebot" content="noindex">
```
Be aware that some search engines might interpret the noindex rule differently. As a result, it is possible that your page might still appear in results from other search engines.

Read more about the noindex <meta> tag.

### HTTP response header

Instead of a <meta> tag, you can return an X-Robots-Tag HTTP header with a value of either noindex or none in your response. A response header can be used for non-HTML resources, such as PDFs, video files, and image files. Here's an example of an HTTP response with an X-Robots-Tag header instructing search engines not to index a page:
```
HTTP/1.1 200 OK
(...)
X-Robots-Tag: noindex
(...)
```
Read more about the noindex response header.

### Debugging noindex issues

We have to crawl your page in order to see <meta> tags and HTTP headers. If a page is still appearing in results, it's probably because we haven't crawled the page since you added the noindex rule. Depending on the importance of the page on the internet, it may take months for Googlebot to revisit a page. You can request that Google recrawl a page using the URL Inspection tool.

If you need to remove a page of your site quickly from Google's search results, see our documentation about removals.

Another reason could also be that the robots.txt file is blocking the URL from Google web crawlers, so they can't see the tag. To unblock your page from Google, you must edit your robots.txt file.

Finally, make sure that the noindex rule is visible to Googlebot. To test if your noindex implementation is correct, use the URL Inspection tool to see the HTML that Googlebot received while crawling the page. You can also use the Page Indexing report in Search Console to monitor the pages on your site from which Googlebot extracted a noindex rule.

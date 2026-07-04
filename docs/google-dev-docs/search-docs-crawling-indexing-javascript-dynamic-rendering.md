<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/javascript/dynamic-rendering
  Fetched: 2026-07-04T06:23:09Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Dynamic rendering as a workaround

On some websites, JavaScript loads additional content when the page is open in a browser. This is called client-side rendering. Google Search sees this content along with the content in the HTML of a website. Keep in mind that there are some limitations for JavaScript in Google Search and some pages may encounter problems with content not showing up in the rendered HTML. Other search engines may choose to ignore JavaScript and won't see JavaScript-generated content.

Dynamic rendering is a workaround for websites where JavaScript-generated content is not available to search engines. A dynamic rendering server detects bots that may have problems with JavaScript-generated content and serves a server-rendered version without JavaScript to these bots while showing the client-side rendered version of the content to users.

Dynamic rendering is a workaround and not a recommended solution, because it creates additional complexities and resource requirements.

## Sites that might use dynamic rendering

Dynamic rendering was a workaround for indexable, public JavaScript-generated content that changes rapidly, or content that uses JavaScript features that aren't supported by the crawlers you care about. Not all sites need to use dynamic rendering, and there are better solutions than dynamic rendering as explained in an overview of rendering on the web.

## Understand how dynamic rendering works

Dynamic rendering requires your web server to detect crawlers (for example, by checking the user agent). When your web server identifies a request from a crawler that does not support JavaScript or the JavaScript features required to render your content, this request is routed to a rendering server. Requests from users and crawlers without JavaScript issues are served normally. The rendering server responds to requests with a version of the content that's suitable to the crawler, for example, it may serve a static HTML version. You can choose to enable the dynamic renderer for all pages or on a per-page basis.

## Dynamic rendering is not cloaking

Googlebot generally doesn't consider dynamic rendering as cloaking. As long as your dynamic rendering produces similar content, Googlebot won't view dynamic rendering as cloaking.

When you're setting up dynamic rendering, your site may produce error pages. Googlebot doesn't consider these error pages as cloaking and treats the error as any other error page.

Using dynamic rendering to serve completely different content to users and crawlers can be considered cloaking. For example, a website that serves a page about cats to users and a page about dogs to crawlers is cloaking.

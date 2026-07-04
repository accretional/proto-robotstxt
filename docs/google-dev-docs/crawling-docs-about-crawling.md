<!--
  Source: https://developers.google.com/crawling/docs/about-crawling
  Fetched: 2026-07-04T06:22:31Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Things to know about Google's web crawling

Google has been crawling the open web for over 30 years now, and we regularly get asked questions about how our web crawlers work. To answer some of them, here are a few facts about Google's crawlers and how they help us organize the world's information, connecting people to content from across the web.

## What is crawling? In short, crawling is how Google "sees" the web

Crawling is the process of using automated software to discover new web pages and to understand them. That way, when you come to Google to find a web page, we know that it exists and we can include it in your search results. All search engines rely on crawling to know what pages and information may be out there. You can watch our video on how Google Search crawls pages to learn more.

## We have many crawlers; they each have important jobs

Googlebot is our most well-known crawler, and it's used to keep results in Google Search fresh and up-to-date. We also have crawlers that are specific to our other surfaces, such as Google Images and Google Shopping. We provide full documentation of our most commonly used crawlers and what they're for. Our crawlers use easily identifiable user-agent names and known internet addresses. This way, site owners can be confident that the Google crawlers they're seeing are legitimate.

## We perform repeat crawls to find the latest updates and to provide the freshest search results

To catch breaking news articles, we may recrawl news homepages every few minutes. In other cases we might have seen that nothing has changed for years, so we might wait a month to recrawl. Site owners can influence how often recrawling happens using sitemap files that tell us about new and updated pages.

## Frequent crawling is a good sign!

If we're crawling your site a lot, it's an indication your pages have fresh or highly relevant content that people want to find, and that our systems are recognizing that demand. Online shopping is a great example: we crawl ecommerce sites often so that our results will display retailers' most up-to-date prices, promotions, and inventory status.

## Google's crawling has grown over time as pages have become more complex

Another reason we recrawl frequently is to fully understand the richness of a web page and what it offers. Our crawlers use a technique called rendering, which loads a site in full to "see" a page just as a real person would. Over the years, web pages have gotten more sophisticated; the median mobile page has grown in size from 816 kilobytes to 2.3 megabytes, and now has more than 60 different files to load, from images to interactive components. So to get a representative snapshot of a web page in all its glory, we might need to crawl the same page several times — or more, as new elements get added all the time.

## We optimize crawling automatically

Our crawlers are engineered for efficiency, and they adjust themselves to minimize the impact on site owners. For example, when a site slows down or returns errors, our crawl rate changes automatically to avoid overloading the site's servers. We try to limit wasteful crawling by caching the crawled content. And as our crawlers discover more of a website, they're also able to recognize sections that can be covered with less crawling; for example, calendars that go to the year 9999 probably don't need to be crawled in their entirety. Site owners can help by identifying what content doesn't need to be crawled, which saves websites money by lowering their infrastructure costs and makes the internet more efficient as a whole.

## Google crawlers never go into paywall or subscription content without permission

By default, if a page isn't accessible on the open web — for example, if the content is behind a login page — our crawlers can't access it, either. We have specific guidance for site owners if they want to give Google explicit permission to access subscription pages (for example, so that Google can refer users to that content). If you choose to provide subscription access to our crawlers, you can use structured data to continue showing human visitors a login screen without triggering our rules on spam. And you can keep subscription content from appearing in page previews by taking advantage of preview controls.

## Site owners have control over what gets crawled, and how

We honor open web standards such as robots.txt, a simple text file that lets site owners declare how crawlers like ours should interact with their pages. Robots.txt, along with robots meta tags, empowers websites to easily communicate to Google and other services how to access their content. They can block pages from appearing in Search. They can tell us about new content they want crawled using sitemaps. And they can manage how frequently we crawl their sites via their crawl budget.

## Our standard crawlers always respect websites' choices about how their content is accessed and used

After a crawl, we may use the crawled data multiple times to reduce the need for wasteful repeat requests on sites. Even when we reuse this data, we continue to respect the choices sites make through robots.txt and the controls we offer through that open web protocol. For example, sites can use Google-Extended in robots.txt to control, among other things, whether their content helps train future versions of Gemini models. Utilizing Google-Extended doesn't affect a site's inclusion in Search, nor do we use Google-Extended as a ranking signal in Search.

We provide many tools for site owners to manage their Google crawling experience, including Google Search Console, which is available at no cost to site owners. It provides information on how much we've crawled, and why. It also helps sites diagnose problems such as server downtime or speed issues. In addition, Search Console provides comprehensive information on how a site's pages are visible in Search and how users are engaging with them.

Our crawlers help connect people to the best of the web, and we're always looking for ways to make them more capable and efficient.

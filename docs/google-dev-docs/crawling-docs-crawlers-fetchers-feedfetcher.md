<!--
  Source: https://developers.google.com/crawling/docs/crawlers-fetchers/feedfetcher
  Fetched: 2026-07-04T06:22:35Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Feedfetcher

Feedfetcher is how Google crawls RSS or Atom feeds for Google News and WebSub. Feedfetcher stores and periodically refreshes feeds that are requested by users of an app or service. Only podcast feeds get indexed in Google Search; however, if a feed doesn't follow the Atom or RSS specification, it may still be indexed. Here are some answers to the most commonly asked questions about how this user-controlled feed grabber works.

## How do I request that Google not retrieve some or all of my site's feeds?

When users add a service or app that uses Feedfetcher data, Google's Feedfetcher attempts to obtain the content of the feed in order to display it. Since Feedfetcher requests come from explicit action by human users, and not from automated crawlers, Feedfetcher ignores robots.txt rules.

If your feed is publicly available, Google can't restrict users from accessing it. One solution is to configure your site to serve a 404, 410, or other error status message to Feedfetcher-Google user agent.

If your feed is provided by a blog or site hosting service, work directly with that service to restrict access to your feed.

## How often will Feedfetcher retrieve my feeds?

Feedfetcher shouldn't retrieve feeds from most sites more than once every hour on average. Some frequently updated sites may be refreshed more often. Note, however, that due to network delays, it's possible that Feedfetcher may briefly appear to retrieve your feeds more frequently.

##  Why is Feedfetcher trying to download incorrect links from my server, or from a domain that doesn't exist?

Feedfetcher retrieves feeds at the request of services or apps installed by users. It is possible that a user has requested a feed URL that does not exist.

## Why is Feedfetcher downloading information from my "secret" web server?

Feedfetcher retrieves feeds at the request of services or apps installed by users. It is possible that the request came from a user who knows about your "secret" server or typed it in by mistake.

## Why isn't Feedfetcher obeying my robots.txt file?

Feedfetcher retrieves feeds only after users have explicitly started a service or app that requests data from the feed. Feedfetcher behaves as a direct agent of the human user, not as a robot, so it ignores robots.txt entries. Since Feedfetcher acts as an agent for multiple users, it conserves bandwidth by making requests for common feeds only once for all users who requested the feed through an app or service. The common feeds are RSS and Atom.

You can prevent Feedfetcher from crawling your site by configuring your server to serve a 404, 410, or other error status message to the Feedfetcher-Google user agent.

## Why are there visits from multiple machines at Google.com, all with user-agent Feedfetcher?

Feedfetcher was designed to be distributed on several machines to improve performance and scale as the web grows. To cut down on bandwidth usage, the machines used are often located near the sites that they're retrieving in the network.

##  Can you tell me the IP addresses from which Feedfetcher makes requests so that I can filter my logs?

The IP addresses used by Feedfetcher are included in the user-triggered-fetchers-google.json object.

## Why is Feedfetcher downloading the same page on my site multiple times?

In general, Feedfetcher only downloads one copy of each file from your site during a given feed retrieval. Very occasionally, the machines are stopped and restarted, which may cause it to again retrieve pages that it's recently visited.

## What kinds of links does Feedfetcher crawl?

Unlike normal web crawlers, Feedfetcher isn't discovering links to crawl at all; instead, it crawls a single URL that's provided to it by users of a service or app that uses Feedfetcher.

## My Feedfetcher question isn't answered here. Where can I get more help?

If you're still having trouble, try posting your question in the Search Central forum.

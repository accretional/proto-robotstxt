<!--
  Source: https://developers.google.com/crawling/docs/crawlers-fetchers/google-user-triggered-fetchers
  Fetched: 2026-07-04T06:22:38Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# List of Google user-triggered fetchers

User-triggered fetchers are initiated by users to perform a fetching function within a Google product. For example, Google Site Verifier acts on a user's request, or a site hosted on Google Cloud (GCP) has a feature that allows the site's users to retrieve an external RSS feed. Because the fetch was requested by a user, these fetchers generally ignore robots.txt rules. The general technical properties of Google's crawlers also apply to the user-triggered fetchers.

The IP ranges the user-triggered fetchers use are published in the user-triggered-fetchers.json, user-triggered-fetchers-google.json, and user-triggered-agents.json objects. The user-triggered fetchers' reverse DNS mask, depending on whether the fetcher is Google or user owned, matches ***-***-***-***.gae.googleusercontent.com or google-proxy-***-***-***-***.google.com respectively.

The following list shows the user-triggered fetchers, their user agent strings as they appear in the HTTP requests, and the products they are associated with. The list is not exhaustive, it only covers the requestors that are more likely to show up in log files and that we've received questions about.

## Chrome Web Store

User-Agent in HTTP requests  |
```
Mozilla/5.0 (compatible; Google-CWS)
```
Associated products  |  The Chrome Web Store fetcher requests URLs that developers provide in the metadata of their Chrome extensions and themes.

## Feedfetcher

User-Agent in HTTP requests  |
```
FeedFetcher-Google; (+http://www.google.com/feedfetcher.html)
```
Associated products  |  Feedfetcher is used for crawling RSS or Atom feeds for Google News and WebSub.

## Google-Agent

User-Agent in HTTP requests  |

Mobile agent  |
```
Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/W.X.Y.Z Mobile Safari/537.36 (compatible; Google-Agent; +https://developers.google.com/crawling/docs/crawlers-fetchers/google-agent)
```
Desktop agent  |
```
Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko; compatible; Google-Agent; +https://developers.google.com/crawling/docs/crawlers-fetchers/google-agent) Chrome/W.X.Y.Z Safari/537.36
```
Associated products  |  Google-Agent is used by agents hosted on Google infrastructure to navigate the web and perform actions upon user request (for example, Project Mariner). It uses IP ranges from user-triggered-agents.json.

## Google Messages

User-Agent in HTTP requests  |
```
GoogleMessages
```
Associated products  |  Google Messages fetcher is used to generate link previews for URLs sent in chat messages.

## Google NotebookLM

User-Agent in HTTP requests  |
```
Google-NotebookLM
```
Associated products  |  The Google-NotebookLM fetcher requests individual URLs that NotebookLM users have provided as sources for their projects.

## Google Pinpoint

User-Agent in HTTP requests  |
```
Google-Pinpoint
```
Associated products  |  The Google-Pinpoint fetcher requests individual URLs that Pinpoint users specified as sources for their personal collections of documents.

## Google Publisher Center

User-Agent in HTTP requests  |
```
GoogleProducer; (+https://developers.google.com/search/docs/crawling-indexing/google-producer)
```
Associated products  |  Google Publisher Center fetches and processes feeds that publishers explicitly supplied for use in Google News landing pages.

## Google Read Aloud

User-Agent in HTTP requests  |

Mobile agent  |
```
Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Mobile Safari/537.36 (compatible; Google-Read-Aloud; +https://support.google.com/webmasters/answer/1061943)
```
Desktop agent  |
```
Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36 (compatible; Google-Read-Aloud; +https://support.google.com/webmasters/answer/1061943)
```
Former agent (deprecated)  |  google-speakr

Associated products  |  Upon user request, Google Read Aloud fetches and reads out web pages using text-to-speech (TTS).

## Google Site Verifier

User-Agent in HTTP requests  |
```
Mozilla/5.0 (compatible; Google-Site-Verification/1.0)
```
Associated products  |  Google Site Verifier fetches Search Console verification tokens.

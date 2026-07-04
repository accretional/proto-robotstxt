<!--
  Source: https://developers.google.com/crawling/docs/robots-txt/submit-updated-robots-txt
  Fetched: 2026-07-04T06:22:49Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Update your robots.txt file

To update the rules in your existing robots.txt file, download a copy of your robots.txt file from your site and make the necessary edits. Then, upload the updated file to your site.

## Download your robots.txt file

You can download your robots.txt file various ways, for example:

-  Navigate to your robots.txt file, for example https://example.com/robots.txt and copy its contents into a new text file on computer. Make sure you follow the guidelines related to the file format when creating the new local file.

-  Download an actual copy of your robots.txt file with a tool like curl. For example:
```
curl https://example.com/robots.txt -o robots.txt
```
-  Use the robots.txt report in Search Console to copy the content of your robots.txt file, which you can then paste into a file on your computer.

## Edit your robots.txt file

Open the robots.txt file you downloaded from your site in a text editor and make the necessary edits to the rules. Make sure you use the correct syntax and that you save the file with UTF-8 encoding.

## Upload your robots.txt file

Upload your new robots.txt file to the root directory of your site as a text file named robots.txt. The way you upload a file to your site is highly platform and server dependent. Check out our tips for finding help with uploading a robots.txt file to your site.

## Refresh Google's robots.txt cache

During the automatic crawling process, Google's crawlers notice changes you made to your robots.txt file and update the cached version every 24 hours. If you need to update the cache faster, use the Request a recrawl function of the robots.txt report.

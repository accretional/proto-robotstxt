<!--
  Source: https://developers.google.com/search/docs/crawling-indexing/amp/enhance-amp
  Fetched: 2026-07-04T06:22:57Z by tools/google-dev/pull-docs.sh
  Text content © Google, licensed under CC BY 4.0
  (https://creativecommons.org/licenses/by/4.0/), per
  https://developers.google.com/terms/site-policies. Converted from
  HTML to text; content otherwise unmodified. Code samples are
  Apache-2.0 per the same policy.
-->

# Enhance AMP content in Google Search

You can enhance your AMP content in Google Search by creating a basic AMP page, adding structured data, monitoring your pages, and practicing with codelabs.

## Create a basic AMP page

-  Create your first AMP page.

-  Follow the Google Search guidelines for AMP pages.

-  Make your content discoverable by linking your pages. For crawling and indexing, Google Search requires that an AMP page links to a canonical page. The canonical page can be either a non-AMP version of the page or it can be the AMP page itself.

-  Ensure that users can experience the same content and complete the same actions on AMP pages as on the corresponding canonical pages, where possible.

-  Use the AMP Test Tool to ensure that your page meets the Google Search requirements for a valid AMP HTML document.

-  Use the same structured data markup across both the canonical and AMP pages.

-  Apply common content best practices:

- Make sure that your robots.txt file doesn't block your AMP page. Use robots meta tags, data-nosnippet, and X-Robots-Tag where appropriate.

-  Follow the guidelines for hreflang for language and regional URLs. For AMP specific examples, see Internationalization.

## Create an AMP page using a CMS

If you serve your web content through a Content Management System (CMS), you can use an existing CMS plugin (such as for WordPress, Drupal, or Joomla) or implement custom functionality in your CMS to generate AMP content. If you intend to customize your CMS, follow these guidelines in addition to Create a basic AMP page:

- Consider how AMP HTML files will fit your site's URL path scheme. If you're generating an AMP page in addition to a canonical non-AMP page, we recommend choosing one of the following URL schemes:

- https://www.example.com/myarticle/amp

- https://www.example.com/myarticle.amp.html

- Develop a structured data markup template. Here are some guidelines:

- Construct the template based on the requirements for the type of content you are publishing.

- Refer to AMP Project metadata examples for sample templates for recipes, articles, videos, and reviews.

## Optimize for rich results

You can use structured data to enhance the appearance of your page in search results. AMP pages with structured data can appear as a rich result, like the Top stories carousel or host carousel.

- Implement structured data.

- Verify that your structured data parses correctly by using the Rich Results Test.

- Verify your complete AMP setup by using the AMP Test Tool.

## Monitor and improve your pages

Periodically check all of your AMP pages by monitoring the following reports:

- AMP status report: Catch site templating and other site-wide implementation issues that can affect large numbers of your AMP pages.

- Rich result status reports: Identify problems with your structured data and opportunities to provide additional structured data.

If you need to stop serving your AMP pages from Google Search results, follow Remove AMP from Google Search results.

## Practice with codelabs

Here are some codelabs to practice building AMP pages:

- Learn how to build AMP pages with the AMP foundations codelab.

- Add features like analytics, video embedding, social media integration, and image carousels to your AMP pages with the Advanced concepts codelab.

- Build a beautiful, interactive, and canonical AMP pages codelab that incorporates many AMP features and extended components.

- Use AMP components to build a PWA experience with the AMP+PWA codelab.

## Resources

Now that you've created your AMP pages, here are some resources for learning more about other Google product integrations for AMP:

- Learn how to Create AMP ad units.

- Learn how to monetize your AMP pages.

- Use Google Tag Manager to optimize and measure marketing initiatives.

- Add analytics to your AMP pages to track user interactions with built-in Google Analytics support.

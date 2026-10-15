# Bot Protection Basics

In this tutorial, we will turn on bot protection for the link from [Your first redirect link](first-redirect-link.md), then check what it blocks. You need that link before starting.

## Turn on known-bot blocking

First, open your `/hello` link and go to its protection settings. Switch on **Block Known Bots**, then save.

This blocks search engine crawlers, scraper signatures and headless browser user agents. Real visitors are unaffected.

## Send a bot request

Now request the link with a bot user agent from a terminal:

```bash
curl -i -A "Googlebot" https://YOUR-DOMAIN/hello
```

Notice that you are not redirected to the destination. Request it again in your browser and notice that you still are.

## Read the threat log

Open the **Threat Log** in the sidebar. You will see your `curl` request with its timestamp, IP address, user agent and the block reason. Your browser visit is not listed, because it was allowed.

## Let your own IP through

Open **IP Lists** and add your own IP to the whitelist. Whitelisted IPs bypass all protection on all your links, so your own testing is never blocked.

You have enabled bot protection, seen it block a bot, and read the log that records it.

## Where next

- [Botection callback API](../reference/botection-callback-api.md) for how blocking decisions are made
- [About bot protection enforcement](../explanation/enforcement-architecture.md)

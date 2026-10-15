# Your First Redirect Link

In this tutorial, we will create a redirect link, visit it, and find the visit in your analytics. It takes about five minutes. You need a GuardBot account and either a domain you control or a shared domain for testing.

## Create a link

First, open **Redirect Links** in the sidebar and click **New Link**.

Fill in the form:

- **Domain**: pick one of your verified domains (or a shared domain)
- **Path**: enter `/hello`
- **Destination**: enter `https://example.com`

Leave bot protection off. Click **Create**. A toast notification confirms the link exists.

## Visit the link

Now open your domain followed by `/hello` in a browser. Notice that you land on `https://example.com`.

## Find your visit in analytics

Go back to the panel and click your new link. Within a few seconds the visit appears. You will see the total clicks, your country, your device type, and whether the visit was flagged as a bot.

You have created a working redirect link and watched it record traffic.

## Where next

- [Bot protection basics](configure-bot-protection.md), the next tutorial
- the Domains page in the panel if you need your own domain
- [Public API reference](../../docs-site/src/content/docs/api/overview.md) to create links programmatically

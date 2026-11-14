---
title: Cloudflare Integration Is Here
slug: cloudflare-integration-is-here
date: 2026-11-14
author: GuardBot Team
category: updates
tags:
  - cloudflare
  - dns
  - ssl
  - integrations
excerpt: Import your Cloudflare domains with one click. DNS, SSL, everything configured automatically.
featured: true
cover_image: /static/uploads/intel/2026/11/cloudflare-integration-banner.png
---

You asked for it. A lot.

Every week we'd get the same request: "I use Cloudflare for DNS. Can you just pull my domains in?" And every week we'd say "soon." 

Today, soon is now.

## One Click. That's It.

![One-click Cloudflare import flow](/static/uploads/intel/2026/11/cloudflare-integration-inpost.png)

Connect your Cloudflare account, pick a domain, hit Import. Done.

You don't copy DNS records. You don't sit there refreshing, waiting for propagation. You don't touch SSL or nginx. GuardBot handles it while you grab coffee.

Here's what happens behind the scenes when you click that button:

1. We point your A records to the right server
2. Add the verification TXT record
3. Set up the CNAME for automatic SSL renewals
4. Generate a wildcard certificate (covers every subdomain)
5. Configure nginx to proxy traffic through botection
6. Mark your domain ready

Takes about two minutes. You don't touch any of it.

## Why This Matters

Before today, setting up a Cloudflare domain meant bouncing between three dashboards. GuardBot for the domain setup. Cloudflare for DNS records. Back to GuardBot to verify. Then wait. Then SSL. Then more waiting.

Now? Connect once, import forever. Every domain in your Cloudflare account shows up in a list. Click import on any of them.

## The Technical Bit (For Those Who Care)

We're using acme-dns for SSL generation. Your `_acme-challenge` record points to our acme-dns server via CNAME, which means certificate renewals happen instantly without touching your DNS again. Renewals won't fail because propagation was slow.

If acme-dns isn't available for some reason, we fall back to adding TXT records directly through the Cloudflare API. Slower, but it works. You'll get your wildcard cert either way.

## Already Using NameSilo?

Works the same as our NameSilo integration. If you've imported domains from NameSilo before, Cloudflare will feel familiar.

## Get Started

Head to **Domains > Connect Cloudflare** in your dashboard. You'll need your Cloudflare email and Global API Key (grab it from your Cloudflare profile under API Tokens).

That's it. Connect, import, and you're protected.

More DNS providers are coming. Got one you want? Tell us.

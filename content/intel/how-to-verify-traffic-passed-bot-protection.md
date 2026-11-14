---
title: How to Verify Traffic Passed Bot Protection
slug: how-to-verify-traffic-passed-bot-protection
date: 2026-11-14
author: GuardBot Team
category: security
tags:
  - signed-urls
  - verification
  - security
  - api
excerpt: Your destination server can cryptographically verify that traffic passed bot detection. No more guessing.
featured: true
cover_image: /static/uploads/intel/2026/11/signed-urls-banner.png
---

Bot protection is great until you realize you're trusting a redirect.

Think about it. GuardBot blocks the bots, lets humans through, and sends them to your landing page. But your landing page has no idea who sent that visitor. Could be GuardBot. Could be someone who found the destination URL and shared it directly. Could be a bot that figured out where to go.

You're blind.

## Now You're Not

Signed URL verification changes this. When you enable it, GuardBot adds a cryptographic signature to every redirect. Your server checks the signature. If it's valid, you know for certain that visitor passed bot detection. If it's missing or wrong, they didn't.

No guessing. Proof.

## What This Lets You Do

Skip your own captcha for verified traffic. Why make someone prove they're human twice?

Block direct visits entirely. If the signature isn't there, the request didn't come through GuardBot.

Log which traffic is verified vs unverified. See exactly how much of your traffic is taking shortcuts around your protection.

Build stricter funnels. Payment pages, signup forms, gated content. Accept only verified visitors.

## How It Works

![How signed URL verification works](/static/uploads/intel/2026/11/signed-urls-inpost.png)

GuardBot signs the destination URL with Ed25519 (the same crypto used in SSH keys and secure messaging). The signature includes a timestamp, so old links expire. Your server verifies the signature using our public key.

The signed URL looks like this:
```
https://yoursite.com/landing?_bt=1731628800&_bs=abc123...
```

The `_bt` is the timestamp. The `_bs` is the signature. Your backend checks both.

We've got verification code ready to copy for PHP, Python, Node.js, Go, Ruby, and browser JavaScript. Drop it in, point it at the public key, done.

## The Trust Problem This Solves

Before signed URLs, your setup looked like this: GuardBot filters traffic → redirects humans to your site → your site accepts everyone who shows up.

See the gap? Your site accepts everyone who shows up. Including anyone who found the URL another way.

Signed URLs close that gap. Now it's: GuardBot filters traffic → signs the redirect → your site verifies the signature → accepts only verified visitors.

The redirect becomes proof, not a promise.

## Get Started

Enable "Verification Signature" on any redirect link. That's it on the GuardBot side.

Then add the verification code to your backend. We've got copy-paste examples for every major language in the [verification guide](https://docs.guardbot.sbs/integrations/verify-signed-urls/).

Takes maybe ten minutes to set up. And after that, you'll know exactly which visitors earned their way in.

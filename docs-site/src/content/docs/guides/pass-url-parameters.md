---
title: Forwarding URL Parameters
description: Pass query parameters through your redirect links to pre-fill forms and track campaign data.
---

URL parameters let you carry data from your redirect link to your destination. When a visitor clicks a link with `?email=user@example.com`, that parameter arrives at your landing page where it can pre-fill a form or track the source.

This guide shows you how to enable parameter forwarding and use it with email platforms.

## Enable Pass Params

Open your redirect link's settings. Find the **Pass Params** toggle and turn it on. Save your changes.

That's it. Any query parameters on your redirect URL now pass through to the destination.

## How It Works

When someone visits your link with parameters:

```
https://yourdomain.com/offer?email=user@example.com&source=newsletter
```

They land on your destination with those same parameters:

```
https://landing-page.com/signup?email=user@example.com&source=newsletter
```

Multiple parameters work the same way. Everything after the `?` in the original URL gets appended to the destination.

## Use With Email Platforms

Most email platforms support merge tags that insert subscriber data into URLs. Add them as query parameters in your email links.

| Platform | Merge Tag Example |
|----------|-------------------|
| Mailchimp | `?email=*\|EMAIL\|*` |
| Klaviyo | `?email={{ email }}` |
| ActiveCampaign | `?email=%EMAIL%` |
| HubSpot | `?email={{contact.email}}` |
| GetResponse | `?email=[[email]]` |

When the email sends, the platform replaces the tag with the actual subscriber email. The visitor clicks through, and your landing page receives the email address in the URL.

## Capture Parameters on Your Page

Pass Params delivers parameters to your destination URL. Your landing page needs code to read and use them.

Here's simple JavaScript that reads the email parameter and fills a form field:

```javascript
const params = new URLSearchParams(window.location.search);
const email = params.get('email');
if (email) {
    document.getElementById('email-field').value = email;
}
```

If your page uses a form builder or landing page tool, check its documentation for parameter capture settings. Most tools have built-in support for pre-filling fields from URL parameters.

## Common Issues

**Parameters disappear before reaching your page.** If your destination has intermediate steps like bot checks or loading screens, each step must preserve the parameters. Check every page in your redirect chain.

**Page receives parameters but doesn't use them.** Your landing page needs code to read URL parameters. Contact your page developer to confirm it supports parameter capture.

**Wrong parameter name.** Your destination may expect a specific parameter name like `login_hint` instead of `email`. Check with your page developer for the correct name.

## Related Topics

- [About Redirect Links](/features/redirect-links/) for other link options
- [Links API Reference](/api/links/) for creating links with Pass Params via API

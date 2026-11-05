# How to forward URL parameters to your destination

Pass Params forwards query parameters from your redirect link to your destination URL. Use it to pass tracking data, email addresses, or any value through to your landing page.

## Enable Pass Params

1. Edit your redirect link
2. Toggle **Pass Params** on
3. Save

## Add parameters to your link

Append parameters to your redirect URL when sharing it:

```
https://your-redirect.example.com/abc123?email=user@example.com
```

With Pass Params enabled, your destination receives:

```
https://your-destination.com/page?email=user@example.com
```

Multiple parameters work the same way:

```
https://your-redirect.example.com/abc123?email=user@example.com&source=newsletter
```

## Use with email platforms

Most email platforms support merge tags that insert recipient data. Add them as URL parameters:

| Platform | Example |
|----------|---------|
| Mailchimp | `?email=*|EMAIL|*` |
| Klaviyo | `?email={{ email }}` |
| ActiveCampaign | `?email=%EMAIL%` |
| HubSpot | `?email={{contact.email}}` |

The platform replaces the tag with the actual email before sending.

## Requirements for your destination page

Pass Params delivers parameters to your destination URL. Your destination page must read and use them.

If your page does not have code to read URL parameters, the values arrive but nothing happens with them.

Example JavaScript to read the email parameter:

```javascript
const params = new URLSearchParams(window.location.search);
const email = params.get('email');
if (email) {
    document.getElementById('emailField').value = email;
}
```

## Common issues

### Parameters disappear before reaching final page

If your destination has intermediate steps (bot checks, redirects, loading screens), each step must preserve and forward the parameters. If any step rewrites the URL without the parameters, they are lost.

Check each page in your redirect chain to confirm parameters pass through.

### Destination receives parameters but does not use them

Your destination page needs code to:
1. Read the parameter from the URL
2. Do something with the value (prefill a form, store it, send it to your backend)

Contact your page developer to confirm the page supports URL parameter capture and which parameter names it expects.

### Wrong parameter name

Your destination may expect a specific parameter name. Common variations:

- `email`
- `login_hint`
- `prefill_email`
- `user`

Check with your page developer for the correct name.

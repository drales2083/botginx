---
title: cPanel Integration
description: Connect your cPanel hosting account to manage domains and auto-configure DNS.
---

Connect your cPanel hosting account to GuardBot. Import domains directly from your hosting, get automatic DNS configuration, and manage everything from one dashboard.

## Why Connect cPanel?

When you connect cPanel, GuardBot can:

- **Import domains** directly from your hosting account
- **Auto-configure DNS** records when you add domains
- **Set up wildcard subdomains** automatically for your redirect links
- **Add verification records** without manual DNS editing
- **Configure SSL certificates** via ACME DNS challenges

No more copying DNS records manually. Connect once, import domains with one click.

## Getting Your API Token

cPanel uses API tokens for secure third-party access. Here's how to create one:

### Step 1: Log Into cPanel

Go to your hosting provider's cPanel login page. Enter your credentials.

### Step 2: Find API Tokens

Look for **Security** section in cPanel. Click **Manage API Tokens**.

If you don't see this option, your hosting provider may have disabled API access. Contact their support.

### Step 3: Create a Token

Click **Create** to make a new token.

- **Name**: Enter something recognizable like `GuardBot`
- **Expiration**: Set to "Never expires" or a far future date

Click **Create**. cPanel shows the token once. Copy it immediately — you won't see it again.

### Step 4: Save the Token

Store the token securely. You'll need it to connect GuardBot.

## Connecting to GuardBot

### Step 1: Go to cPanel Settings

In your GuardBot dashboard, navigate to **Domains** → **Connect cPanel**.

### Step 2: Enter Connection Details

Fill in the form:

| Field | What to Enter |
|-------|---------------|
| **cPanel Host** | Your cPanel URL (e.g., `server.yourhost.com` or `yourhost.com:2083`) |
| **Username** | Your cPanel username |
| **API Token** | The token you created |

### Step 3: Test Connection

Click **Test Connection**. GuardBot verifies it can reach your cPanel and authenticate.

If the test fails:
- Check the hostname is correct
- Verify the username matches your cPanel login
- Make sure the API token was copied completely
- Confirm your hosting allows API access

### Step 4: Save

Once the test passes, click **Connect**. Your cPanel is now linked.

## Importing Domains

After connecting, you can import domains from cPanel.

### Step 1: View Your Domains

Go to **Domains** → **cPanel**. Click on your connection. You'll see all domains in your cPanel account:

- Main domain
- Addon domains
- Subdomains
- Parked domains

### Step 2: Select a Domain

Click **Import** next to the domain you want to add to GuardBot.

### Step 3: Auto-Configure DNS

When importing, GuardBot offers to configure DNS automatically:

- **Verification TXT record**: Proves you own the domain
- **Wildcard A record**: Points `*.yourdomain.com` to the deploy server
- **ACME CNAME**: For automatic SSL certificate generation

Leave auto-configure enabled unless you manage DNS elsewhere (like Cloudflare).

### Step 4: Complete Setup

Click **Import**. GuardBot:

1. Adds the domain to your account
2. Creates DNS records via cPanel API
3. Verifies the domain
4. Starts SSL certificate generation

The domain is ready to use within minutes.

## Managing Connections

### View Connected Accounts

Go to **Domains** → **cPanel** to see all your connections. Each shows:

- Hostname and username
- Number of domains imported
- Last used date
- Connection status

### Update Credentials

If you regenerate your API token, update it in GuardBot:

1. Click on the connection
2. Click **Edit**
3. Enter the new token
4. Test and save

### Remove a Connection

To disconnect cPanel:

1. Click on the connection
2. Click **Disconnect**
3. Confirm

Your imported domains remain in GuardBot. Only the cPanel link is removed. DNS records created by GuardBot stay in place unless you delete them manually.

## Auto-DNS Features

### Verification Records

When you add a domain, GuardBot creates a TXT record:

```
_guardbot-verify.yourdomain.com → [verification-token]
```

This proves domain ownership without uploading files.

### Wildcard A Records

GuardBot adds a wildcard A record pointing to the deploy server:

```
*.yourdomain.com → [deploy-server-ip]
```

This enables any subdomain to work for your redirect links.

### ACME Challenge Records

For SSL certificates, GuardBot manages ACME DNS challenges:

```
_acme-challenge.yourdomain.com → [acme-dns-target]
```

This enables automatic wildcard SSL without manual intervention.

## Troubleshooting

### Connection Fails

**"Invalid credentials"**: Double-check username and API token. Tokens are long strings — make sure you copied the whole thing.

**"Cannot connect to host"**: Verify the hostname. Try with and without port number. Some hosts use non-standard ports.

**"API access denied"**: Your hosting provider may restrict API access. Contact their support to enable it.

### DNS Not Updating

**Records not appearing**: DNS changes can take 5-15 minutes to propagate. Wait and check again.

**Wrong IP address**: Make sure your deploy server is configured in GuardBot admin settings.

**Permission denied**: The cPanel user must have DNS zone access for the domain. Check addon domain permissions.

### Domains Not Showing

**Missing domains**: Only domains with DNS zones in cPanel appear. Domains using external DNS (like Cloudflare) won't show even if hosted on cPanel.

**Addon vs subdomain confusion**: Addon domains show as their own entry. Subdomains of addon domains show under the addon.

## Security Notes

- API tokens have full account access. Only connect cPanel accounts you control.
- Store tokens securely. Never share them in tickets or chat.
- You can revoke a token anytime in cPanel without affecting other connections.
- GuardBot stores tokens encrypted. Even database access doesn't reveal them.

## Multiple Accounts

You can connect multiple cPanel accounts. Useful when you have:

- Different hosting providers
- Separate accounts for different clients
- Staging and production environments

Each connection is independent. Import domains from any connected account.

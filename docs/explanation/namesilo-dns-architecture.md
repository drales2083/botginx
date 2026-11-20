# About NameSilo DNS Auto-Configuration Architecture

This document explains why the NameSilo integration is designed the way it is, the trade-offs made, and how the pieces fit together.

## Why Auto-Configure DNS?

Traditional domain setup requires users to:
1. Log into their registrar
2. Navigate to DNS settings
3. Add an A record for `@` pointing to the server IP
4. Add an A record for `*` (wildcard) pointing to the same IP
5. Wait for DNS propagation
6. Return to the app to verify

This takes 15-30 minutes and is error-prone. Users often:
- Forget the wildcard record
- Use wrong IP addresses
- Create CNAME instead of A records
- Get confused by registrar UIs

Auto-configuration reduces this to one click: "Import with DNS auto-setup".

## Why Store API Keys Per User?

The API key is stored in the `users` table (one per user) rather than a separate table because:

1. **NameSilo accounts are personal** - Unlike cPanel where users might connect multiple servers, NameSilo is typically one account per user.

2. **Simpler querying** - When reconfiguring all domains, we can join directly to users to get their API key.

3. **Security isolation** - Each user's domains are managed with their own credentials.

## Why Encrypted Storage?

API keys are encrypted with AES-256-GCM before storage because:

1. **Database leaks happen** - If the database is compromised, plaintext API keys would allow attackers to hijack domains.

2. **Compliance** - Many security frameworks require encryption of credentials at rest.

3. **Defense in depth** - Even with database access, attackers need the encryption key (from environment variables) to use the credentials.

The encryption uses:
- AES-256 (32-byte key)
- GCM mode (authenticated encryption)
- Random nonce per encryption
- Base64 encoding for storage

## Why Rate Limiting in the Client?

NameSilo enforces a strict rate limit: **1 request per 2 seconds**. Violations result in:
- HTTP 429 responses
- Temporary IP bans
- Potential account suspension

The client implements this with a mutex and timestamp:

```go
c.mu.Lock()
if elapsed := time.Since(c.lastRequest); elapsed < 2*time.Second {
    time.Sleep(2*time.Second - elapsed)
}
c.lastRequest = time.Now()
c.mu.Unlock()
```

This ensures even concurrent requests are properly spaced.

## Why Handle Multiple Response Formats?

NameSilo's API is inconsistent in how it returns lists:

**Single domain:**
```json
{"reply": {"domains": {"domain": "example.com"}}}
```

**Multiple domains:**
```json
{"reply": {"domains": {"domain": ["a.com", "b.com"]}}}
```

**Or sometimes:**
```json
{"reply": {"domains": ["a.com", "b.com"]}}
```

The client must handle all formats gracefully using type switches.

## Why Delete Before Create for DNS Records?

When updating DNS records, we delete existing ones first because:

1. **NameSilo rejects duplicates** - Adding an A record for `@` when one exists returns error code 261.

2. **Values may differ** - The existing record might point to an old IP.

3. **Clean state** - Ensures we know exactly what records exist.

The pattern is:
```go
// List existing
records, _ := client.ListDNSRecords(ctx, domain)

// Delete matching records
for _, r := range records {
    if r.Type == "A" && (r.Host == "@" || r.Host == "*") {
        client.DeleteDNSRecord(ctx, domain, r.RecordID)
    }
}

// Add new records
client.AddDNSRecord(ctx, domain, "A", "", serverIP, 3600)
client.AddDNSRecord(ctx, domain, "A", "*", serverIP, 3600)
```

## Why acme-dns for Wildcard SSL?

Wildcard SSL certificates (`*.example.com`) require DNS-01 challenge, which involves:
1. Certbot requests a challenge token
2. You create a TXT record `_acme-challenge.example.com` with the token
3. Let's Encrypt verifies the record
4. Certificate is issued

**Problem:** NameSilo DNS propagation takes 5-15 minutes. This means:
- 5-15 minute wait per domain during SSL generation
- Certbot times out waiting for propagation
- Poor user experience

**Solution:** acme-dns delegation

acme-dns is a small DNS server you control. Instead of a TXT record, you create a CNAME:

```
_acme-challenge.example.com → abc123.acme.yourserver.com
```

Now when Let's Encrypt looks up `_acme-challenge.example.com`, it follows the CNAME to your acme-dns server, where you can update TXT records **instantly** via API.

The flow becomes:
1. Register subdomain with acme-dns (once per domain)
2. Add CNAME in NameSilo pointing to acme-dns (90 second propagation)
3. Certbot requests challenge
4. Update TXT via acme-dns API (instant)
5. Let's Encrypt verifies (instant)
6. Certificate issued

Total time: ~2 minutes instead of 10-15 minutes.

## Why Background SSL Generation?

SSL generation runs in a goroutine because:

1. **HTTP request would timeout** - SSL generation takes 90+ seconds for CNAME propagation alone.

2. **User experience** - User sees "Domain imported" immediately, SSL happens in background.

3. **Error isolation** - SSL failure doesn't prevent domain import.

The pattern:
```go
domainID, err := importDomain(...)
if err != nil {
    return err
}

// Return success immediately
go func() {
    // This runs in background
    setupAcmeDNSAndGenerateSSL(domainID)
}()

return domainID, nil
```

## Why Reconfigure All Domains?

When the deploy server IP changes (new VPS, IP rotation, migration), all DNS records need updating. Without automation, this requires:

1. Admin logs into each user's NameSilo account (impossible)
2. Each user manually updates their DNS (slow, error-prone)
3. Admin updates DNS for each domain one by one (tedious)

Auto-reconfigure solves this:

```go
// Get all auto-configured domains
domains := SELECT ... WHERE source = 'namesilo' AND dns_auto_configured = true

// Group by user (each has their own API key)
for userID, userDomains := range groupByUser(domains) {
    client := getClientForUser(userID)
    for _, domain := range userDomains {
        client.SetupDNS(ctx, domain.Name, newIP)
    }
}
```

One admin API call updates all domains across all users.

## Why Track `dns_auto_configured`?

This flag distinguishes between:

1. **Auto-configured domains** - We manage DNS, should be reconfigured when IP changes
2. **Manual domains** - User manages DNS, we shouldn't touch them

Without this flag, reconfigure would try to update domains we don't control, causing errors or overwriting user's custom DNS setup.

## Why Normalize Domain Names to Lowercase?

DNS is case-insensitive (`Example.COM` = `example.com`), but:

1. **Filesystem paths are case-sensitive** (Linux) - `/var/www/sites/Example.COM/` ≠ `/var/www/sites/example.com/`

2. **nginx extracts hostname lowercase** - Regardless of how the user types the URL

3. **Database comparisons** - `WHERE name = 'Example.COM'` won't match `example.com` without `LOWER()`

Normalizing on input prevents mismatches throughout the system.

## Security Considerations

### API Key Storage
- Encrypted with AES-256-GCM
- Encryption key from environment variable
- Never logged or exposed in API responses

### Rate Limiting
- Prevents accidental API abuse
- Protects NameSilo account from suspension

### Domain Verification
- Domains verified to exist in user's NameSilo account before import
- Prevents importing domains user doesn't own

### SSH Credentials
- Deploy server credentials stored in `servers` table
- Used only for certbot operations via SSH
- Connections are short-lived and authenticated

## Trade-offs Made

| Decision | Trade-off |
|----------|-----------|
| One API key per user | Can't support users with multiple NameSilo accounts |
| AES encryption | Requires environment variable for key; lost key = lost access |
| Background SSL | User doesn't see errors immediately; must check domain status |
| acme-dns | Requires additional infrastructure; adds complexity |
| Delete before create | Extra API calls; slightly slower |
| Rate limit in client | Slower operations; can't parallelize requests |

These trade-offs favor reliability and security over raw performance.

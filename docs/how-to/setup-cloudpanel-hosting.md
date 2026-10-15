# How to Setup a CloudPanel Hosting Server

This guide walks through setting up a new CloudPanel VPS server for botginx hosting with antibot protection.

## Prerequisites

- A fresh VPS with Ubuntu 22.04+
- Root SSH access
- The `deploy-cloudpanel.sh` script from the botginx repository

## Step 1: Deploy CloudPanel

```bash
ssh root@NEW_SERVER 'bash -s' < deploy-cloudpanel.sh
```

This installs CloudPanel and prepares directories for botection integration.

## Step 2: Apply Branding

Upload the logo and run the branding script:

```bash
# Upload logo first
scp assets/logos/guardbotpanel-logo.png root@SERVER:/opt/cloudpanel-branding/logos/

# Run branding script
scp scripts/cloudpanel-branding.sh root@SERVER:/opt/
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
```

This applies:
- **Logo:** Custom panel logo
- **Theme:** Dark mode default
- **Color:** Purple (#6c5ce7)
- **Footer:** Clean footer without CloudPanel links

## Step 3: Deploy Botection

```bash
ssh root@SERVER
cd /var/www
git clone https://github.com/robertp2083/antibot.git
SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh
```

## Step 4: Configure Botection Callback

Edit `/var/www/antibot/config/config.yaml`:

```yaml
callback:
  url: https://YOUR_BOTGINX_PANEL/api/botection/should-block
  secret: your-webhook-secret
  cache_ttl: 30s
```

## Step 5: Add Server to Botginx

In the botginx panel, go to **Admin → Hosting → Servers → Add Server**:

| Field | Value |
|-------|-------|
| Name | CloudPanel-{IP} |
| Type | CloudPanel |
| Hostname | Server IP address |
| Port | 22 |
| Username | root |
| Password | SSH password |

## Step 6: Route Traffic

Configure your edge (Cloudflare, external nginx) to route port 443 traffic to botection on port 8080.

## One-Liner Setup

For quick deployment to a fresh server:

```bash
SERVER=root@1.2.3.4

# Step 1: CloudPanel
ssh $SERVER 'bash -s' < deploy-cloudpanel.sh

# Step 2: Branding
ssh $SERVER 'mkdir -p /opt/cloudpanel-branding/logos'
scp assets/logos/guardbotpanel-logo.png $SERVER:/opt/cloudpanel-branding/logos/
scp scripts/cloudpanel-branding.sh $SERVER:/opt/
ssh $SERVER 'bash /opt/cloudpanel-branding.sh'

# Step 3: Botection
ssh $SERVER 'cd /var/www && git clone https://github.com/robertp2083/antibot.git && SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh'
```

## Re-apply Branding After Updates

If CloudPanel updates overwrite branding:

```bash
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
```

Or it auto-reapplies daily via cron.

## Verify Setup

1. Open `https://SERVER_IP:8443`
2. Should show custom logo
3. Dark theme should be default
4. Check botection is running: `systemctl status antibot`

## Troubleshooting

**CSS not loading:**
```bash
ssh root@SERVER 'ls -la /home/clp/htdocs/app/files/public/css/custom-branding.css'
```

**Check injection:**
```bash
ssh root@SERVER 'grep "custom-branding" /home/clp/htdocs/app/files/templates/Frontend/layout.html.twig'
```

**Clear cache:**
```bash
ssh root@SERVER 'rm -rf /home/clp/htdocs/app/files/var/cache/*'
```

## Related Documentation

- [CloudPanel Architecture](../explanation/cloudpanel-architecture.md) - How the system works
- [CloudPanel Settings Reference](../reference/cloudpanel-settings.md) - Settings and directories

# CloudPanel Quick Setup Guide

Quick reference for setting up new CloudPanel VPS servers for botginx hosting.

## 1. Deploy CloudPanel

```bash
ssh root@NEW_SERVER 'bash -s' < deploy-cloudpanel.sh
```

## 2. Apply Branding (GuardBotPanel)

```bash
# Upload and run branding script
scp scripts/cloudpanel-branding.sh root@SERVER:/opt/
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
```

This applies:
- **Name:** GuardBotPanel (replaces CloudPanel)
- **Theme:** Dark mode default
- **Color:** Red (#dc3545)

## 3. Deploy Botection

```bash
ssh root@SERVER
cd /var/www
git clone https://github.com/robertp2083/antibot.git
SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh
```

## 4. Configure Botection Callback

Edit `/var/www/antibot/config/config.yaml`:

```yaml
callback:
  url: https://YOUR_BOTGINX_PANEL/api/botection/should-block
  secret: your-webhook-secret
```

## 5. Add Server to Botginx

In botginx: **Admin → Hosting → Servers → Add Server**

| Field | Value |
|-------|-------|
| Name | CloudPanel-{IP} |
| Type | CloudPanel |
| Hostname | {IP} |
| Port | 22 |
| Username | root |
| Password | {SSH password} |

## One-Liner Setup

For quick deployment:

```bash
# Replace with actual server details
SERVER=root@1.2.3.4

# Step 1: CloudPanel
ssh $SERVER 'bash -s' < deploy-cloudpanel.sh

# Step 2: Branding
scp scripts/cloudpanel-branding.sh $SERVER:/opt/ && ssh $SERVER 'bash /opt/cloudpanel-branding.sh'

# Step 3: Botection
ssh $SERVER 'cd /var/www && git clone https://github.com/robertp2083/antibot.git && SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh'
```

## Re-apply Branding After Updates

If CloudPanel updates overwrite branding:

```bash
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
```

Or it auto-reapplies daily via cron.

## Verify Branding

1. Open `https://SERVER_IP:8443`
2. Should show "GuardBotPanel" name
3. Dark theme should be default
4. Red accent colors

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

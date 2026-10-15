# How to Set Up a CloudPanel Hosting Server

This guide shows you how to deploy a CloudPanel server with botection and connect it to botginx. For how the pieces fit, see [About the CloudPanel hosting architecture](../explanation/cloudpanel-architecture.md).

## Steps

### 1. Deploy CloudPanel Server

```bash
# From your local machine
ssh root@NEW_SERVER 'bash -s' < deploy-cloudpanel.sh
```

This installs CloudPanel and prepares directories for botection integration.

### 2. Add Server to Botginx

In botginx panel: **Admin → Hosting → Servers → Add Server**

| Field | Value |
|-------|-------|
| Name | CloudPanel-{IP} |
| Type | CloudPanel |
| Hostname | Server IP |
| Panel URL | https://{IP}:8443 |
| Port | 22 |
| Username | root |
| Password | SSH password |

### 3. Deploy Botection

```bash
ssh root@SERVER
cd /var/www
git clone https://github.com/robertp2083/antibot.git
SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh
```

### 4. Configure Botection Callback

Edit `/var/www/antibot/config/config.yaml`:

```yaml
callback:
  url: https://YOUR_BOTGINX_PANEL/api/botection/should-block
  secret: your-webhook-secret
  cache_ttl: 30s
```

### 5. Route Traffic

Configure your edge (Cloudflare, external nginx) to route port 443 traffic to botection on port 8080.

## Troubleshooting

### Settings Not Applying

1. Check SSH connection from botginx server:
   ```bash
   ssh root@CLOUDPANEL_SERVER "cat /etc/botection/links/{domainID}.json"
   ```

2. Verify botection is reading the settings:
   ```bash
   tail -f /var/www/antibot/logs/antibot.log
   ```

3. Check settings file permissions:
   ```bash
   ls -la /etc/botection/links/
   ```

### Analytics Not Loading

1. Check botection is running:
   ```bash
   systemctl status antibot
   ```

2. Test stats API locally:
   ```bash
   curl http://127.0.0.1:8080/api/stats
   ```

3. Check SSH connection from botginx.

### Traffic Not Routing Through Botection

1. Verify botection is listening:
   ```bash
   netstat -tlnp | grep 8080
   ```

2. Check CloudPanel nginx is on backend port:
   ```bash
   netstat -tlnp | grep 8081
   ```

3. Verify site configs have been modified:
   ```bash
   grep "listen 8081" /etc/nginx/sites-enabled/*.conf
   ```

## Migration from Antibot-Dashboard

If you previously used antibot-dashboard:

1. **No action needed** - settings are now managed directly in botginx
2. Users access settings at: **Hosting → Domain → Settings**
3. Analytics are displayed in the same Settings page (Analytics tab)
4. You can remove antibot-dashboard from CloudPanel servers:
   ```bash
   systemctl stop antibot-dashboard
   systemctl disable antibot-dashboard
   rm -rf /home/clp/antibot-dashboard
   ```


## Related

- [How to deploy botginx](deploy-botginx.md)
- [Botection callback API](../reference/botection-callback-api.md)
- [cPanel domain flow](../explanation/cpanel-domain-flow.md)
- [How to customize CloudPanel branding](customize-cloudpanel-branding.md)

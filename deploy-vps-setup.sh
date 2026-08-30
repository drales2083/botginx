#!/bin/bash
set -e

# Botginx Deploy VPS Setup
# Sets up a redirect links server with botection + nginx
#
# Usage:
#   scp deploy-vps-setup.sh root@VPS_IP:/root/
#   ssh root@VPS_IP 'bash /root/deploy-vps-setup.sh'
#
# Or pipe directly:
#   ssh root@VPS_IP 'bash -s' < deploy-vps-setup.sh

echo "=== Botginx Deploy VPS Setup ==="
echo ""

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: Run as root"
    exit 1
fi

# Configuration
GITHUB_TOKEN="${GITHUB_TOKEN:-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}"
ANTIBOT_DIR="/var/www/antibot"
SITES_DIR="/var/www/sites"

# Panel URL for callback/webhook (required for analytics)
PANEL_URL="${PANEL_URL:-}"
WEBHOOK_SECRET="${WEBHOOK_SECRET:-$(openssl rand -hex 32)}"

if [ -z "$PANEL_URL" ]; then
    echo "WARNING: PANEL_URL not set. Analytics will not work."
    echo "         Set PANEL_URL=https://your-panel.com to enable analytics."
fi

# Get VPS IP
VPS_IP=$(curl -4 -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
echo "VPS IP: $VPS_IP"

# =============================================================================
# 1. System packages
# =============================================================================
echo ""
echo "[1/7] Installing system packages..."
apt-get update -qq
apt-get install -y -qq nginx redis-server jq curl certbot python3-certbot-nginx ufw cron

systemctl enable nginx redis-server cron
systemctl start nginx redis-server cron

# Fix nginx for long domain names
if ! grep -q "^[[:space:]]*server_names_hash_bucket_size 128;" /etc/nginx/nginx.conf; then
    sed -i '/server_names_hash_bucket_size/d' /etc/nginx/nginx.conf
    sed -i 's/sendfile on;/sendfile on;\n\tserver_names_hash_bucket_size 128;/' /etc/nginx/nginx.conf
    echo "  Set server_names_hash_bucket_size 128"
fi

# =============================================================================
# 2. Botection
# =============================================================================
echo ""
echo "[2/7] Installing Botection..."
mkdir -p "$ANTIBOT_DIR"/{config,data,logs}
cd "$ANTIBOT_DIR"

# Download latest botection
ASSET="botection-linux-amd64"
rel=$(curl -fsSL -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/robertp2083/antibot/releases/latest")

VERSION=$(echo "$rel" | jq -r ".tag_name")
echo "  Version: $VERSION"

url=$(echo "$rel" | jq -r ".assets[] | select(.name==\"$ASSET\") | .url")
curl -fsSL -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/octet-stream" -o botection "$url"
chmod +x botection

# Create .env.local
cat > "$ANTIBOT_DIR/.env.local" << EOF
GITHUB_TOKEN=$GITHUB_TOKEN
ANTIBOT_ADMIN_TOKEN=$(openssl rand -hex 32)
EOF

# Create config - upstream to internal nginx (8081), preserve_host for domain routing
cat > "$ANTIBOT_DIR/config/config.yaml" << 'CONFIGEOF'
server:
  listen: ":8080"
  upstream: "http://127.0.0.1:8081"
  preserve_host: true
  read_timeout: "30s"
  write_timeout: "30s"
  api_bypass_paths:
    - "/.well-known/"
    - "/health"
    - "/healthz"

admin:
  listen: "127.0.0.1:9090"
  token: ""
  rate_limit: 100

panel_callback:
  enabled: ${PANEL_ENABLED:-false}
  url: "${PANEL_URL}/api/botection/should-block"
  timeout: "100ms"
  cache_ttl: "30s"
  fallback: "allow"

redis:
  addr: "localhost:6379"
  db: 1
  prefix: "antibot:"

database:
  path: "data/antibot.db"

decision:
  strategy: "weighted"
  block_threshold: 0.8
  challenge_threshold: 0.5

modules:
  ip_reputation:
    enabled: true
    weight: 0.8
    config:
      block_datacenters: false
      block_tor: true

  rate_limiter:
    enabled: true
    weight: 1.0
    config:
      window: "60s"
      max_requests: 200

  fingerprint:
    enabled: true
    weight: 0.9
    config:
      check_headers: true
      allow_tags: ["search-engine", "social-preview"]
      block_tags: ["scanner", "http-library", "browser-automation"]

  challenge:
    enabled: true
    weight: 1.0
    config:
      template: "ember"
      type: "pow"
      difficulty: 1000

webhooks:
  enabled: ${PANEL_ENABLED:-false}
  batch_size: 1
  flush_interval: "1s"
  endpoints:
    - url: "${PANEL_URL}/webhooks/antibot/webhook"
      secret: "${WEBHOOK_SECRET}"
      events: ["*"]
      timeout: "5s"
      retry_max: 3

link_settings:
  enabled: true
  directory: "/etc/botection/links"
  watch: true
CONFIGEOF

# Create link settings directory
mkdir -p /etc/botection/links

# Replace panel enabled based on PANEL_URL presence
if [ -n "$PANEL_URL" ]; then
    sed -i 's/\${PANEL_ENABLED:-false}/true/g' "$ANTIBOT_DIR/config/config.yaml"
    sed -i "s|\${PANEL_URL}|$PANEL_URL|g" "$ANTIBOT_DIR/config/config.yaml"
    sed -i "s|\${WEBHOOK_SECRET}|$WEBHOOK_SECRET|g" "$ANTIBOT_DIR/config/config.yaml"
    echo "  Panel callback enabled: $PANEL_URL"
    echo "  Webhook secret: $WEBHOOK_SECRET"
    echo ""
    echo "  IMPORTANT: Add this secret to your panel's .env:"
    echo "  ANTIBOT_WEBHOOK_SECRET=$WEBHOOK_SECRET"
else
    sed -i 's/\${PANEL_ENABLED:-false}/false/g' "$ANTIBOT_DIR/config/config.yaml"
    sed -i "s|\${PANEL_URL}||g" "$ANTIBOT_DIR/config/config.yaml"
    sed -i "s|\${WEBHOOK_SECRET}||g" "$ANTIBOT_DIR/config/config.yaml"
fi

# Set admin token from .env.local
source "$ANTIBOT_DIR/.env.local"
sed -i "s/token: \"\"/token: \"$ANTIBOT_ADMIN_TOKEN\"/" "$ANTIBOT_DIR/config/config.yaml"

# Create systemd service
cat > /etc/systemd/system/botection.service << EOF
[Unit]
Description=Botection Reverse Proxy
After=network.target redis-server.service
Requires=redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=$ANTIBOT_DIR
EnvironmentFile=$ANTIBOT_DIR/.env.local
ExecStart=$ANTIBOT_DIR/botection daemon -config $ANTIBOT_DIR/config/config.yaml
Restart=always
RestartSec=5
StandardOutput=append:$ANTIBOT_DIR/logs/botection.log
StandardError=append:$ANTIBOT_DIR/logs/botection.log

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable botection
systemctl start botection

echo "  Botection installed and running"

# =============================================================================
# 3. Auto-deploy script
# =============================================================================
echo ""
echo "[3/7] Setting up auto-deploy..."

curl -fsSL -H "Authorization: token $GITHUB_TOKEN" \
    "https://raw.githubusercontent.com/robertp2083/antibot/main/auto-deploy.sh" \
    -o "$ANTIBOT_DIR/auto-deploy.sh"
chmod +x "$ANTIBOT_DIR/auto-deploy.sh"

# Add cron
CRON_ANTIBOT="*/2 * * * * /bin/bash $ANTIBOT_DIR/auto-deploy.sh >> $ANTIBOT_DIR/logs/auto-deploy.log 2>&1"
(crontab -l 2>/dev/null | grep -v "$ANTIBOT_DIR/auto-deploy.sh"; echo "$CRON_ANTIBOT") | crontab -
echo "  Auto-deploy cron added"

# =============================================================================
# 4. Nginx - Outer (port 80) + Internal upstream (8081)
# =============================================================================
echo ""
echo "[4/7] Configuring nginx..."
mkdir -p "$SITES_DIR"

# Outer nginx - receives from Cloudflare, forwards to botection
cat > /etc/nginx/sites-available/botection-proxy.conf << 'NGINXEOF'
# Outer proxy - Cloudflare -> nginx (80) -> botection (8080)
server {
    listen 80 default_server;
    server_name _;

    # ACME challenge for SSL (if needed)
    location /.well-known/acme-challenge/ {
        root /var/www/sites;
        allow all;
    }

    # Everything else goes through botection
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
NGINXEOF

# Internal nginx - receives from botection, routes by Host header
cat > /etc/nginx/sites-available/redirect-upstream.conf << 'NGINXEOF'
# Internal upstream - botection (8080) -> nginx (8081) -> site files
server {
    listen 127.0.0.1:8081 default_server;
    server_name _;

    # Dynamic root based on Host header
    set $site_domain "";
    set $site_subdomain "";

    # Root domain: domain.tld -> /var/www/sites/domain.tld/_root
    if ($host ~* ^[^.]+\.[^.]+$) {
        set $site_domain $host;
        set $site_subdomain "_root";
    }

    # Subdomain: sub.domain.tld -> /var/www/sites/domain.tld/sub
    if ($host ~* ^([^.]+)\.([^.]+\.[^.]+)$) {
        set $site_subdomain $1;
        set $site_domain $2;
    }

    root /var/www/sites/$site_domain/$site_subdomain;
    index index.html;

    # Domain verification
    location /.well-known/domain-verify {
        default_type application/json;
        return 200 '{"verified":true}';
    }

    location / {
        try_files $uri $uri/ /index.html =404;
    }
}
NGINXEOF

ln -sf /etc/nginx/sites-available/botection-proxy.conf /etc/nginx/sites-enabled/
ln -sf /etc/nginx/sites-available/redirect-upstream.conf /etc/nginx/sites-enabled/

# Create default site
mkdir -p "$SITES_DIR/default/_root"
echo "<h1>Redirect Server</h1><p>Site not configured</p>" > "$SITES_DIR/default/_root/index.html"

# Remove default nginx site
rm -f /etc/nginx/sites-enabled/default

nginx -t && systemctl reload nginx
echo "  Nginx configured (outer:80 -> botection:8080 -> internal:8081)"

# =============================================================================
# 5. Cloudflare-only firewall
# =============================================================================
echo ""
echo "[5/7] Configuring Cloudflare-only firewall..."

ufw --force reset >/dev/null
ufw default deny incoming >/dev/null
ufw default allow outgoing >/dev/null
ufw allow 22/tcp >/dev/null

# Cloudflare IPs
for ip in $(curl -s https://www.cloudflare.com/ips-v4); do
    ufw allow from $ip to any port 80,443 proto tcp >/dev/null 2>&1
done
for ip in $(curl -s https://www.cloudflare.com/ips-v6); do
    ufw allow from $ip to any port 80,443 proto tcp >/dev/null 2>&1
done

ufw --force enable >/dev/null
echo "  Firewall enabled (SSH + Cloudflare only)"

# Weekly Cloudflare IP update
cat > /etc/cron.weekly/update-cloudflare-ips << 'CFEOF'
#!/bin/bash
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
for ip in $(curl -s https://www.cloudflare.com/ips-v4); do
    ufw allow from $ip to any port 80,443 proto tcp
done
for ip in $(curl -s https://www.cloudflare.com/ips-v6); do
    ufw allow from $ip to any port 80,443 proto tcp
done
ufw --force enable
CFEOF
chmod +x /etc/cron.weekly/update-cloudflare-ips

# =============================================================================
# 6. Add-site helper script
# =============================================================================
echo ""
echo "[6/7] Creating helper scripts..."

cat > /usr/local/bin/add-site << 'SCRIPTEOF'
#!/bin/bash
# Add a new redirect site
# Usage: add-site domain.com [subdomain]

DOMAIN="$1"
SUBDOMAIN="${2:-}"
SITES_DIR="/var/www/sites"

if [ -z "$DOMAIN" ]; then
    echo "Usage: add-site domain.com [subdomain]"
    exit 1
fi

if [ -n "$SUBDOMAIN" ]; then
    FULL_DOMAIN="${SUBDOMAIN}.${DOMAIN}"
else
    FULL_DOMAIN="$DOMAIN"
fi

SITE_DIR="$SITES_DIR/$FULL_DOMAIN"
mkdir -p "$SITE_DIR"

# Create nginx server block
cat > "/etc/nginx/sites-available/${FULL_DOMAIN}.conf" << EOF
server {
    listen 127.0.0.1:8081;
    server_name $FULL_DOMAIN;

    root $SITE_DIR;
    index index.html;

    location / {
        try_files \$uri \$uri/ =404;
    }
}
EOF

ln -sf "/etc/nginx/sites-available/${FULL_DOMAIN}.conf" /etc/nginx/sites-enabled/

nginx -t && systemctl reload nginx
echo "Site created: $FULL_DOMAIN -> $SITE_DIR"
SCRIPTEOF

chmod +x /usr/local/bin/add-site

# =============================================================================
# 7. Verification
# =============================================================================
echo ""
echo "[7/7] Verifying installation..."

echo ""
echo "Services:"
echo "  Botection: $(systemctl is-active botection)"
echo "  Nginx:     $(systemctl is-active nginx)"
echo "  Redis:     $(systemctl is-active redis-server)"

echo ""
echo "Botection version: $($ANTIBOT_DIR/botection -v)"

echo ""
echo "========================================="
echo "         SETUP COMPLETE"
echo "========================================="
echo ""
echo "VPS IP: $VPS_IP"
echo ""
echo "Traffic flow:"
echo "  Internet -> Cloudflare -> :80/:443 -> nginx"
echo "  nginx -> :8080 (botection) -> :8081 (internal nginx) -> site files"
echo ""
echo "Sites directory: $SITES_DIR"
echo ""
echo "To add a new site:"
echo "  add-site example.com"
echo "  add-site example.com subdomain"
echo ""
echo "Admin token: $ANTIBOT_ADMIN_TOKEN"
echo ""

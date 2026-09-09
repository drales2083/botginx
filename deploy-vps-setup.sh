#!/bin/bash
set -e

# Botginx Deploy VPS Setup
# Sets up a redirect links server with nginx (botection installed separately)
#
# Usage:
#   scp deploy-vps-setup.sh root@VPS_IP:/root/
#   ssh root@VPS_IP 'bash /root/deploy-vps-setup.sh'
#
# Or pipe directly:
#   ssh root@VPS_IP 'bash -s' < deploy-vps-setup.sh
#
# After this script completes, install botection:
#   git clone https://github.com/robertp2083/antibot.git /var/www/antibot
#   cd /var/www/antibot && SERVER_TYPE=templates bash deploy.sh

echo "=== Botginx Deploy VPS Setup ==="
echo ""

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: Run as root"
    exit 1
fi

# Configuration
SITES_DIR="/var/www/sites"

# Get VPS IP
VPS_IP=$(curl -4 -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
echo "VPS IP: $VPS_IP"

# =============================================================================
# 1. System packages
# =============================================================================
echo ""
echo "[1/5] Installing system packages..."
apt-get update -qq
apt-get install -y -qq nginx redis-server jq curl certbot python3-certbot-nginx ufw cron git php-fpm

systemctl enable nginx redis-server cron php8.3-fpm
systemctl start nginx redis-server cron php8.3-fpm

# Install lego (for wildcard SSL via DNS-01 challenge)
echo "Installing lego ACME client..."
cd /tmp
LEGO_VERSION="v5.3.0"
curl -fsSL "https://github.com/go-acme/lego/releases/download/${LEGO_VERSION}/lego_${LEGO_VERSION}_linux_amd64.tar.gz" -o lego.tar.gz
tar xzf lego.tar.gz lego
mv -f lego /usr/local/bin/
chmod +x /usr/local/bin/lego
rm -f lego.tar.gz
echo "  lego installed: $(lego --version 2>&1 | head -1)"

# Install acme-dns (for CNAME delegation - users add CNAME once, renewals are automatic)
echo "Installing acme-dns server..."
# ACME_DOMAIN must be set - the domain for acme-dns CNAME delegation
# Example: ACME_DOMAIN=acme.pamach.xyz
if [ -z "$ACME_DOMAIN" ]; then
    echo "ERROR: ACME_DOMAIN is required (e.g., acme.pamach.xyz)"
    exit 1
fi
cd /tmp
ACMEDNS_VERSION="1.0"
curl -fsSL "https://github.com/joohoi/acme-dns/releases/download/v${ACMEDNS_VERSION}/acme-dns_${ACMEDNS_VERSION}_linux_amd64.tar.gz" -o acme-dns.tar.gz
tar xzf acme-dns.tar.gz
mv -f acme-dns /usr/local/bin/
chmod +x /usr/local/bin/acme-dns
rm -f acme-dns.tar.gz

mkdir -p /etc/acme-dns /var/lib/acme-dns

cat > /etc/acme-dns/config.cfg << EOF
[general]
listen = "0.0.0.0:53"
protocol = "both"
domain = "$ACME_DOMAIN"
nsname = "$ACME_DOMAIN"
nsadmin = "admin.$ACME_DOMAIN"
records = [
    "$ACME_DOMAIN. A $VPS_IP",
]
debug = false

[database]
engine = "sqlite3"
connection = "/var/lib/acme-dns/acme-dns.db"

[api]
ip = "127.0.0.1"
port = "8053"
tls = "none"
disable_registration = false

[logconfig]
loglevel = "info"
logformat = "text"
EOF

cat > /etc/systemd/system/acme-dns.service << 'EOF'
[Unit]
Description=acme-dns server for ACME DNS-01 challenges
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/acme-dns -c /etc/acme-dns/config.cfg
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Disable systemd-resolved (conflicts on port 53)
systemctl stop systemd-resolved 2>/dev/null || true
systemctl disable systemd-resolved 2>/dev/null || true
if [ -L /etc/resolv.conf ]; then
    rm /etc/resolv.conf
    echo -e "nameserver 8.8.8.8\nnameserver 1.1.1.1" > /etc/resolv.conf
fi

systemctl daemon-reload
systemctl enable acme-dns
systemctl start acme-dns
echo "  acme-dns installed and running"

# Fix nginx for long domain names
if ! grep -q "^[[:space:]]*server_names_hash_bucket_size 128;" /etc/nginx/nginx.conf; then
    sed -i '/server_names_hash_bucket_size/d' /etc/nginx/nginx.conf
    sed -i 's/sendfile on;/sendfile on;\n\tserver_names_hash_bucket_size 128;/' /etc/nginx/nginx.conf
    echo "  Set server_names_hash_bucket_size 128"
fi

# =============================================================================
# 2. Nginx - Internal upstream (8081) for site routing
# =============================================================================
echo ""
echo "[2/5] Configuring nginx..."
mkdir -p "$SITES_DIR"

# Internal nginx - receives from botection (or direct), routes by Host header
cat > /etc/nginx/sites-available/redirect-upstream.conf << 'NGINXEOF'
# Internal upstream - botection (8080) -> nginx (8081) -> site files
# Or direct access if botection not installed
server {
    listen 127.0.0.1:8081 default_server;
    listen 80 default_server;
    server_name _;

    # Prevent nginx from adding internal port to redirects
    port_in_redirect off;
    absolute_redirect off;

    # Prevent caching of redirects
    add_header Cache-Control "no-cache, no-store, must-revalidate" always;

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
    index index.php index.html;

    # ACME challenge for SSL
    location /.well-known/acme-challenge/ {
        root /var/www/sites;
        allow all;
    }

    # Domain verification
    location /.well-known/domain-verify {
        default_type application/json;
        return 200 '{"verified":true}';
    }

    # PHP processing for randomized redirect pages
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php8.3-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    }

    location / {
        # Try exact file, directory, then PHP index, then HTML index
        try_files $uri $uri/ @php;
    }

    location @php {
        # Rewrite to index.php for PHP processing
        rewrite ^ /index.php last;
    }
}
NGINXEOF

ln -sf /etc/nginx/sites-available/redirect-upstream.conf /etc/nginx/sites-enabled/

# Create default site
mkdir -p "$SITES_DIR/default/_root"
echo "<h1>Redirect Server</h1><p>Site not configured</p>" > "$SITES_DIR/default/_root/index.html"

# Remove default nginx site
rm -f /etc/nginx/sites-enabled/default

# Create self-signed cert for default SSL (rejects unknown domains)
mkdir -p /etc/nginx/ssl
if [ ! -f /etc/nginx/ssl/default.pem ]; then
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout /etc/nginx/ssl/default.key \
        -out /etc/nginx/ssl/default.pem \
        -subj '/CN=invalid.local' 2>/dev/null
    echo "  Created default SSL certificate"
fi

# Default SSL server - prevents serving wrong cert for unconfigured domains
cat > /etc/nginx/sites-available/default-ssl.conf << 'NGINXEOF'
# Default SSL handler - rejects requests for unconfigured domains
# Without this, nginx serves the first SSL cert it finds (wrong!)
server {
    listen 443 ssl default_server;
    server_name _;

    ssl_certificate /etc/nginx/ssl/default.pem;
    ssl_certificate_key /etc/nginx/ssl/default.key;

    # Return 444 (close connection) for unknown domains
    # This prevents leaking other domains' certificates
    return 444;
}
NGINXEOF

ln -sf /etc/nginx/sites-available/default-ssl.conf /etc/nginx/sites-enabled/

# Create botection link settings directory (for when botection is installed)
mkdir -p /etc/botection/links

nginx -t && systemctl reload nginx
echo "  Nginx configured (listening on :80 and 127.0.0.1:8081)"

# =============================================================================
# 3. Firewall
# =============================================================================
echo ""
echo "[3/5] Configuring firewall..."

ufw --force reset >/dev/null
ufw default deny incoming >/dev/null
ufw default allow outgoing >/dev/null
ufw allow 22/tcp >/dev/null
ufw allow 53/udp >/dev/null  # DNS for acme-dns
ufw allow 53/tcp >/dev/null  # DNS for acme-dns
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null

ufw --force enable >/dev/null
echo "  Firewall enabled (SSH + DNS + HTTP/HTTPS)"

# Add cron job to ensure port 53 stays open for acme-dns
cat > /etc/cron.d/check-acmedns-firewall << 'CRONEOF'
# Check every hour that port 53 is open for acme-dns
0 * * * * root ufw status | grep -q '53/udp.*ALLOW' || (ufw allow 53/udp && ufw allow 53/tcp && logger 'Fixed missing port 53 firewall rule')
CRONEOF
chmod 644 /etc/cron.d/check-acmedns-firewall

# =============================================================================
# 4. Add-site helper script
# =============================================================================
echo ""
echo "[4/5] Creating helper scripts..."

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
# 5. Done
# =============================================================================
echo ""
echo "[5/5] Verifying installation..."

echo ""
echo "Services:"
echo "  Nginx:     $(systemctl is-active nginx)"
echo "  PHP-FPM:   $(systemctl is-active php8.3-fpm)"
echo "  Redis:     $(systemctl is-active redis-server)"
echo "  acme-dns:  $(systemctl is-active acme-dns)"

echo ""
echo "========================================="
echo "         SETUP COMPLETE"
echo "========================================="
echo ""
echo "VPS IP: $VPS_IP"
echo ""
echo "Current traffic flow (no protection):"
echo "  Internet -> :80/:443 -> nginx -> site files"
echo ""
echo "Sites directory: $SITES_DIR"
echo ""
echo "========================================="
echo "  NEXT STEPS"
echo "========================================="
echo ""
echo "1. Add DNS records for acme-dns (if not already done):"
echo "   ${ACME_DOMAIN}  A   $VPS_IP"
echo "   ${ACME_DOMAIN}  NS  ${ACME_DOMAIN}"
echo ""
echo "2. Add this server to botginx panel at /admin/servers"
echo ""
echo "3. Install Botection for protection:"
echo "   git clone https://github.com/robertp2083/antibot.git /var/www/antibot"
echo "   cd /var/www/antibot && SERVER_TYPE=templates bash deploy.sh"
echo ""
echo "After botection is installed, traffic flow becomes:"
echo "  Internet -> :80/:443 -> nginx -> botection:8080 -> nginx:8081 -> site files"
echo ""
echo "4. Update Guard VPS deploy.env with:"
echo "   ACME_DNS_DOMAIN=${ACME_DOMAIN}"
echo ""
echo "Wildcard SSL (CNAME delegation):"
echo "  Users add: _acme-challenge.domain.com CNAME xxx.${ACME_DOMAIN}"
echo "  Renewals are automatic - no user action needed"
echo ""

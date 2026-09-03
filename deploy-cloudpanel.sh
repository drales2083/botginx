#!/usr/bin/env bash
#
# Deploy CloudPanel server for botginx hosting module.
#
# Installs CloudPanel with nginx, PHP, MySQL/MariaDB. Prepares nginx
# to route through botection (antibot installed separately via its deploy.sh).
#
# Traffic flow (after botection is installed):
#   Internet → :443 (nginx) → :8080 (botection) → :8081 (CloudPanel backends)
#
# Settings/Analytics flow:
#   Botginx → SSH → /etc/botection/links/{domain}.json (settings push)
#   Botginx → SSH → curl 127.0.0.1:8080/api/stats (analytics fetch)
#
# Usage:
#   # From local machine
#   ssh root@SERVER 'bash -s' < deploy-cloudpanel.sh
#
#   # With custom admin credentials
#   CLOUDPANEL_ADMIN_PASSWORD=mysecret ssh root@SERVER 'bash -s' < deploy-cloudpanel.sh
#
# After installation:
#   1. Add the server to botginx: Admin → Hosting → Servers
#   2. Deploy botection:
#      SERVER_TYPE=cloudpanel ssh root@SERVER 'bash -s' < antibot/packaging/deploy.sh
#   3. Configure botection callback URL in /var/www/antibot/config/config.yaml

set -euo pipefail

# ----------------------------------------------------------------------------
# Configuration
# ----------------------------------------------------------------------------

# CloudPanel admin credentials (will be shown after install)
CLOUDPANEL_ADMIN_USER="${CLOUDPANEL_ADMIN_USER:-admin}"
CLOUDPANEL_DB_ENGINE="${CLOUDPANEL_DB_ENGINE:-MARIADB_10.11}"

# Botection port (antibot reverse proxy)
BOTECTION_PORT="${BOTECTION_PORT:-8080}"

# CloudPanel backend port (nginx serves sites here, botection proxies to it)
CLOUDPANEL_BACKEND_PORT="${CLOUDPANEL_BACKEND_PORT:-8081}"

# CloudPanel admin panel port (default 8443)
CLOUDPANEL_PANEL_PORT="${CLOUDPANEL_PANEL_PORT:-8443}"

# Botginx panel URL (for analytics webhooks and panel callbacks)
# Change this when deploying for a different botginx instance
BOTGINX_PANEL_URL="${BOTGINX_PANEL_URL:-https://guardbot.sbs}"

# Webhook secret (shared between botection and botginx)
BOTECTION_WEBHOOK_SECRET="${BOTECTION_WEBHOOK_SECRET:-b75cf8584c56fa27f7209a02136e2f33159c223b0a600a6be7a062614e1db0cf}"

# ----------------------------------------------------------------------------
# Output
# ----------------------------------------------------------------------------

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[0;33m'
BLUE=$'\033[0;34m'
DIM=$'\033[2m'
RESET=$'\033[0m'

log()   { printf '%s[%s]%s %s\n' "$DIM" "$(date '+%H:%M:%S')" "$RESET" "$*"; }
info()  { printf '\n%s==>%s %s\n' "$BLUE" "$RESET" "$*"; }
ok()    { printf '%s  ✓%s %s\n' "$GREEN" "$RESET" "$*"; }
warn()  { printf '%s⚠%s %s\n' "$YELLOW" "$RESET" "$*" >&2; }
die()   { printf '%s✗%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }

# ----------------------------------------------------------------------------
# Preflight
# ----------------------------------------------------------------------------

preflight() {
    info "Preflight checks"

    [[ $EUID -eq 0 ]] || die "This script must be run as root"

    # Check if CloudPanel is already installed
    if [[ -f /home/clp/htdocs/app/files/public/index.php ]]; then
        warn "CloudPanel is already installed"
        read -p "Continue with configuration updates only? [y/N] " -n 1 -r
        echo
        [[ $REPLY =~ ^[Yy]$ ]] || exit 0
        CLOUDPANEL_INSTALLED=true
    else
        CLOUDPANEL_INSTALLED=false
    fi

    # Check OS
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        log "detected: $PRETTY_NAME"
        case "$ID-$VERSION_ID" in
            ubuntu-22.04|ubuntu-24.04|debian-11|debian-12) ;;
            *) die "Only Ubuntu 22.04/24.04 or Debian 11/12 supported (detected: $ID $VERSION_ID)" ;;
        esac
    else
        die "Cannot detect OS"
    fi

    # Check memory (CloudPanel recommends 1GB+)
    local mem_kb
    mem_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    if (( mem_kb < 900000 )); then
        warn "Low memory detected ($(( mem_kb / 1024 ))MB). CloudPanel recommends 1GB+"
    fi

    # Check disk space (10GB minimum recommended)
    local disk_avail
    disk_avail=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
    if (( disk_avail < 10 )); then
        warn "Low disk space (${disk_avail}GB available). CloudPanel recommends 10GB+"
    fi

    ok "preflight passed"
}

# ----------------------------------------------------------------------------
# Install CloudPanel
# ----------------------------------------------------------------------------

install_cloudpanel() {
    info "Installing CloudPanel"

    if [[ "$CLOUDPANEL_INSTALLED" == true ]]; then
        log "skipping CloudPanel installation (already installed)"
        return 0
    fi

    # Update system
    log "updating system packages"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get -y upgrade -qq

    # Install dependencies
    apt-get install -y -qq curl wget gnupg apt-transport-https ca-certificates jq

    # Detect OS and run appropriate installer
    . /etc/os-release

    log "running CloudPanel installer (this takes 5-15 minutes)"

    case "$ID-$VERSION_ID" in
        ubuntu-24.04)
            curl -fsSL https://installer.cloudpanel.io/ce/v2/install.sh -o /tmp/install-cloudpanel.sh
            bash /tmp/install-cloudpanel.sh "$CLOUDPANEL_DB_ENGINE"
            ;;
        ubuntu-22.04)
            curl -fsSL https://installer.cloudpanel.io/ce/v2/install.sh -o /tmp/install-cloudpanel.sh
            bash /tmp/install-cloudpanel.sh "$CLOUDPANEL_DB_ENGINE"
            ;;
        debian-12)
            curl -fsSL https://installer.cloudpanel.io/ce/v2/install.sh -o /tmp/install-cloudpanel.sh
            bash /tmp/install-cloudpanel.sh "$CLOUDPANEL_DB_ENGINE"
            ;;
        debian-11)
            curl -fsSL https://installer.cloudpanel.io/ce/v2/install.sh -o /tmp/install-cloudpanel.sh
            bash /tmp/install-cloudpanel.sh "$CLOUDPANEL_DB_ENGINE"
            ;;
        *)
            die "Unsupported OS: $ID $VERSION_ID"
            ;;
    esac

    if [[ $? -ne 0 ]]; then
        die "CloudPanel installation failed"
    fi

    ok "CloudPanel installed"

    # Clean up
    rm -f /tmp/install-cloudpanel.sh
}

# ----------------------------------------------------------------------------
# Configure nginx for botection reverse proxy
# ----------------------------------------------------------------------------

configure_nginx_botection() {
    info "Configuring nginx for botection integration"

    # Traffic flow when botection is installed:
    #   Internet → nginx:443 (SSL) → botection:8080 → nginx:8081 → PHP
    #
    # This is handled by the "Botection" vhost template (created in create_botection_template)
    # which CloudPanel uses when creating sites. No post-creation modification needed.

    # Create marker file for botection deploy to detect CloudPanel
    mkdir -p /var/www/cloudpanel
    echo "cloudpanel" > /var/www/cloudpanel/.server-type
    log "created server type marker"

    # Create config directory for botection
    mkdir -p /etc/botection
    cat > /etc/botection/cloudpanel.env <<EOF
# CloudPanel botection configuration
CLOUDPANEL_BACKEND_PORT=${CLOUDPANEL_BACKEND_PORT}
BOTECTION_PORT=${BOTECTION_PORT}
EOF
    log "created /etc/botection/cloudpanel.env"

    # Create nginx config snippet (informational)
    cat > /etc/nginx/conf.d/botection-info.conf <<EOF
# Botection Integration (CloudPanel)
#
# Sites use the "Botection" vhost template which routes:
#   - External traffic (80/443) → botection (${BOTECTION_PORT})
#   - Botection → backend nginx (${CLOUDPANEL_BACKEND_PORT}) → PHP
#
# Template created via: clpctl vhost-template:add --name="Botection"
# Use template when creating sites from botginx panel.
EOF

    ok "nginx configured for botection"
}

# ----------------------------------------------------------------------------
# Prepare botection directories for botginx integration
# ----------------------------------------------------------------------------

prepare_botection_dirs() {
    info "Preparing botection directories"

    # Create directories for botginx integration
    mkdir -p /etc/botection/links    # Domain settings pushed via SSH
    mkdir -p /etc/botection/config   # Botection configuration

    # Create default settings template
    cat > "/etc/botection/default-settings.json" <<'EOF'
{
  "country_mode": "off",
  "country_list": [],
  "device_mode": "off",
  "device_list": [],
  "block_bots": true,
  "block_tor": true,
  "block_proxy": true,
  "block_datacenter": true,
  "block_headless": true,
  "min_behavior_score": 0,
  "redirect_on_block": ""
}
EOF

    # Set permissions (botection runs as root typically)
    chmod 755 /etc/botection
    chmod 755 /etc/botection/links

    ok "botection directories prepared"
    log "  /etc/botection/links/ - domain settings (pushed from botginx)"
    log "  /etc/botection/config/ - botection configuration"
}

# ----------------------------------------------------------------------------
# Configure Botection for hosting analytics integration
# ----------------------------------------------------------------------------

configure_botection_analytics() {
    info "Configuring botection for hosting analytics"

    # Only run if botection is installed
    if [[ ! -f /var/www/antibot/config/config.yaml ]]; then
        warn "Botection not installed yet - run antibot deploy.sh first"
        log "  After installing botection, re-run this script or manually update config"
        return 0
    fi

    # Backup existing config
    cp /var/www/antibot/config/config.yaml /var/www/antibot/config/config.yaml.bak

    # Write the complete config with panel_callback and webhooks for analytics
    cat > /var/www/antibot/config/config.yaml << BOTECTIONCFG
# Botection configuration for CloudPanel hosting server
# Configured for hosting analytics integration with botginx

server:
  listen: ":8080"
  upstream: "http://127.0.0.1:8081"
  read_timeout: "120s"
  write_timeout: "120s"
  preserve_host: true
  api_bypass_paths:
    - "/api/"
    - "/__verify"
    - "/.well-known/"
    - "/.health"
    - "/health"
    - "/healthz"

admin:
  listen: "127.0.0.1:9090"
  token: ""
  rate_limit: 100

# Local link settings - fastest, checked first
# Settings files pushed by botginx via SSH to /etc/botection/links/{domainID}.json
link_settings:
  enabled: true
  directory: "/etc/botection/links"
  watch: true

# Panel callback - fallback when local file not found
panel_callback:
  enabled: true
  url: "${BOTGINX_PANEL_URL}/api/botection/should-block"
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
  rate_limiter:
    enabled: true
    weight: 1.0
    config:
      window: "60s"
      max_requests: 100
      by: ["ip", "fingerprint"]

  ip_reputation:
    enabled: true
    weight: 0.8
    config:
      block_datacenters: true
      block_tor: false
      block_proxies: false

  fingerprint:
    enabled: true
    weight: 0.9
    config:
      check_headers: true
      allow_tags: ["search-engine", "social-preview"]
      block_tags: ["scanner", "http-library", "browser-automation"]
      challenge_tags: ["ai-crawler", "seo"]

  geo_fence:
    enabled: true
    weight: 0.6
    config:
      mode: "blocklist"
      blocked_countries: []
      blocked_asns: []

  behavioral:
    enabled: true
    weight: 0.7
    config:
      min_requests_to_profile: 5
      anomaly_threshold: 2.5

  challenge:
    enabled: true
    weight: 1.0
    config:
      type: "js"
      difficulty: 1000
      template: "cloudflare"
      captcha_mode: "math"

proxy:
  timeout: "60s"
  retry_count: 3
  retry_delay: "2s"
  buffer_size: "64k"

# Webhooks - send ALL visit events to botginx for analytics
webhooks:
  enabled: true
  batch_size: 1
  flush_interval: "1s"
  endpoints:
    - url: "${BOTGINX_PANEL_URL}/webhooks/antibot/webhook"
      secret: "${BOTECTION_WEBHOOK_SECRET}"
      events: ["request.blocked", "request.challenged", "request.challenge_passed", "request.challenge_failed", "request.allowed", "session.start", "page_view"]
      timeout: "5s"
      retry_max: 3

ip_lists:
  enabled: true
  whitelist_dir: "/etc/botection/whitelists"
  blocklist_dir: "/etc/botection/blocklists"
  watch: true

log:
  level: "info"
BOTECTIONCFG

    # Restart botection to apply config
    if systemctl is-active --quiet botection; then
        systemctl restart botection
        ok "botection config updated and restarted"
    else
        ok "botection config updated (not running yet)"
    fi

    log "  panel_callback: ${BOTGINX_PANEL_URL}/api/botection/should-block"
    log "  webhooks: ${BOTGINX_PANEL_URL}/webhooks/antibot/webhook"
    log "  events: request.allowed + 6 more (full analytics)"
}

# ----------------------------------------------------------------------------
# Create Botection vhost template for CloudPanel
# ----------------------------------------------------------------------------

create_botection_template() {
    info "Creating Botection vhost template"

    # Check if template already exists
    if clpctl vhost-templates:list 2>/dev/null | grep -q "Botection"; then
        log "Botection template already exists"
        return 0
    fi

    # Create the template file
    # This routes traffic through botection (port 8080) with backend on port 8081
    cat > /tmp/botection-template.tpl << 'TPLEOF'
#{"rootDirectory":"","phpVersion":"8.2"}
server {
  listen 80;
  listen [::]:80;
  listen 443 quic;
  listen 443 ssl;
  listen [::]:443 quic;
  listen [::]:443 ssl;
  http2 on;
  http3 off;
  {{ssl_certificate_key}}
  {{ssl_certificate}}
  {{server_name}}
  {{root}}

  {{nginx_access_log}}
  {{nginx_error_log}}

  if ($scheme != "https") {
    rewrite ^ https://$host$request_uri permanent;
  }

  location ~ /.well-known {
    auth_basic off;
    allow all;
  }

  {{settings}}

  # Route all traffic through Botection (antibot proxy on port 8080)
  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_hide_header X-Varnish;
    proxy_redirect off;
    proxy_max_temp_file_size 0;
    proxy_connect_timeout 720;
    proxy_send_timeout 720;
    proxy_read_timeout 720;
    proxy_buffer_size 128k;
    proxy_buffers 4 256k;
    proxy_busy_buffers_size 256k;
    proxy_temp_file_write_size 256k;
  }

  location ~* ^.+\.(css|js|jpg|jpeg|gif|png|ico|gz|svg|svgz|ttf|otf|woff|woff2|eot|mp4|ogg|ogv|webm|webp|zip|swf|map|mjs)$ {
    add_header Access-Control-Allow-Origin "*";
    add_header alt-svc 'h3=":443"; ma=86400';
    expires max;
    access_log off;
  }

  location ~ /\.(ht|svn|git) {
    deny all;
  }

  if (-f $request_filename) {
    break;
  }
}

# Backend server - receives traffic from Botection on port 8081
server {
  listen 127.0.0.1:8081;
  {{server_name}}
  {{root}}

  include /etc/nginx/global_settings;

  try_files $uri $uri/ /index.php?$args;
  index index.php index.html;

  location ~ \.php$ {
    include fastcgi_params;
    fastcgi_intercept_errors on;
    fastcgi_index index.php;
    fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    try_files $uri =404;
    fastcgi_read_timeout 3600;
    fastcgi_send_timeout 3600;
    fastcgi_param HTTPS "on";
    fastcgi_param SERVER_PORT 443;
    fastcgi_pass 127.0.0.1:{{php_fpm_port}};
    fastcgi_param PHP_VALUE "{{php_settings}}";
  }

  if (-f $request_filename) {
    break;
  }
}
TPLEOF

    # Add the template to CloudPanel
    if clpctl vhost-template:add --name="Botection" --file=/tmp/botection-template.tpl 2>/dev/null; then
        ok "Botection vhost template created"
        log "  Sites created from botginx will auto-route through antibot"
    else
        warn "Failed to create Botection template (CloudPanel may not be fully initialized)"
        log "  Template will be created when first site is provisioned"
    fi

    rm -f /tmp/botection-template.tpl
}

# ----------------------------------------------------------------------------
# Update default index.php template
# ----------------------------------------------------------------------------

update_default_template() {
    info "Updating default site template"

    local template_file="/home/clp/htdocs/app/files/resources/php/index.php"

    if [[ -f "$template_file" ]]; then
        # Backup original
        cp "$template_file" "${template_file}.bak"

        cat > "$template_file" << 'PHPEOF'
<?php
/**
 * Default landing page - Bullet Proof Hosting
 */
$domain = $_SERVER['HTTP_HOST'] ?? 'your-domain.com';
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?= htmlspecialchars($domain) ?> - Protected by Bullet Proof Hosting</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #fff;
        }
        .container {
            text-align: center;
            padding: 40px;
            max-width: 600px;
        }
        .shield {
            font-size: 72px;
            margin-bottom: 20px;
            filter: drop-shadow(0 0 20px rgba(108, 92, 231, 0.5));
        }
        h1 { font-size: 2rem; margin-bottom: 8px; color: #fff; }
        .tagline {
            font-size: 0.95rem;
            color: #6c5ce7;
            margin-bottom: 25px;
            font-weight: 500;
            letter-spacing: 1px;
        }
        .domain {
            font-size: 1.3rem;
            color: #a29bfe;
            margin-bottom: 30px;
            word-break: break-all;
            padding: 12px 24px;
            background: rgba(108, 92, 231, 0.15);
            border-radius: 30px;
            display: inline-block;
        }
        .status {
            background: rgba(255,255,255,0.05);
            border: 1px solid rgba(108, 92, 231, 0.3);
            border-radius: 12px;
            padding: 20px 30px;
            margin-bottom: 30px;
        }
        .status-item {
            display: flex;
            justify-content: space-between;
            padding: 10px 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        .status-item:last-child { border-bottom: none; }
        .status-label { color: #b2bec3; }
        .check { color: #00b894; font-weight: 500; }
        .info {
            font-size: 0.9rem;
            color: #636e72;
            line-height: 1.6;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid rgba(255,255,255,0.05);
        }
        .footer-brand {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            color: #6c5ce7;
            font-weight: 600;
            font-size: 0.85rem;
        }
        .footer-brand span { font-size: 18px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="shield">🛡️</div>
        <h1>Site is Ready!</h1>
        <div class="tagline">BULLET PROOF HOSTING</div>
        <div class="domain"><?= htmlspecialchars($domain) ?></div>

        <div class="status">
            <div class="status-item">
                <span class="status-label">Protection</span>
                <span class="check">✓ Active</span>
            </div>
            <div class="status-item">
                <span class="status-label">Web Server</span>
                <span class="check">✓ Online</span>
            </div>
            <div class="status-item">
                <span class="status-label">PHP</span>
                <span class="check">✓ <?= PHP_VERSION ?></span>
            </div>
            <div class="status-item">
                <span class="status-label">SSL/HTTPS</span>
                <span class="check">✓ <?= (!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off') ? 'Encrypted' : 'Available' ?></span>
            </div>
        </div>

        <p class="info">
            Your site is protected and ready for deployment.<br>
            Upload your files to get started.
        </p>

        <div class="footer">
            <div class="footer-brand">
                <span>🛡️</span> Protected by Bullet Proof Hosting
            </div>
        </div>
    </div>
</body>
</html>
PHPEOF
        ok "default index.php template updated"
    else
        warn "CloudPanel template file not found (may not be initialized yet)"
    fi
}

# ----------------------------------------------------------------------------
# Apply custom branding (GaurdBotPanel theme)
# ----------------------------------------------------------------------------

apply_branding() {
    info "Applying custom branding"

    # Download branding script from GitHub
    local branding_url="https://raw.githubusercontent.com/robertp2083/botginx/main/scripts/cloudpanel-branding.sh"

    if curl -fsSL "$branding_url" -o /tmp/cloudpanel-branding.sh 2>/dev/null; then
        chmod +x /tmp/cloudpanel-branding.sh

        # Run branding script
        if bash /tmp/cloudpanel-branding.sh; then
            ok "branding applied"
        else
            warn "branding script failed (non-critical)"
        fi

        # Save for future re-application
        mkdir -p /opt/cloudpanel-branding
        cp /tmp/cloudpanel-branding.sh /opt/cloudpanel-branding/
        rm -f /tmp/cloudpanel-branding.sh
    else
        warn "Could not download branding script (non-critical)"
        log "  Run manually later: curl -fsSL $branding_url | bash"
    fi
}

# ----------------------------------------------------------------------------
# Print summary
# ----------------------------------------------------------------------------

print_summary() {
    local ip
    ip=$(curl -4 -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

    echo
    echo "=========================================="
    printf '%s CloudPanel Server Ready %s\n' "$GREEN" "$RESET"
    echo "=========================================="
    echo
    echo "CloudPanel Panel:"
    echo "  URL:      https://${ip}:${CLOUDPANEL_PANEL_PORT}"
    echo "  (Create admin account on first visit)"
    echo
    echo "Add to Botginx (Admin → Hosting → Servers):"
    echo "  Name:     CloudPanel-${ip}"
    echo "  Type:     CloudPanel"
    echo "  Hostname: ${ip}"
    echo "  Port:     22"
    echo "  Username: root"
    echo "  Password: <your-root-password>"
    echo
    echo "Traffic Flow (after botection install):"
    echo "  Internet → :443 → botection (:${BOTECTION_PORT}) → CloudPanel nginx (:${CLOUDPANEL_BACKEND_PORT})"
    echo
    echo "=========================================="
    printf '%s Next Steps %s\n' "$YELLOW" "$RESET"
    echo "=========================================="
    echo
    echo "1. Access CloudPanel at https://${ip}:${CLOUDPANEL_PANEL_PORT}"
    echo "   Create your admin account on first visit"
    echo
    echo "2. Add this server to botginx panel"
    echo "   Admin → Hosting → Servers → Add Server"
    echo
    echo "3. Deploy botection (antibot) with CloudPanel mode:"
    echo "   cd /var/www && git clone https://github.com/robertp2083/antibot.git"
    echo "   SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh"
    echo
    echo "4. Configure botection callback in /var/www/antibot/config/config.yaml:"
    echo "   callback:"
    echo "     url: https://YOUR_BOTGINX_PANEL/api/botection/should-block"
    echo
    echo "5. Route external traffic through botection:"
    echo "   - Cloudflare/nginx on 443 → botection on ${BOTECTION_PORT}"
    echo "   - Botection proxies to CloudPanel nginx on ${CLOUDPANEL_BACKEND_PORT}"
    echo
    echo "=========================================="
    printf '%s How It Works %s\n' "$BLUE" "$RESET"
    echo "=========================================="
    echo
    echo "Domain Settings (managed from botginx):"
    echo "  - Users configure settings at: Hosting → Domain → Settings"
    echo "  - Botginx pushes settings via SSH to: /etc/botection/links/{domain}.json"
    echo "  - Botection reads settings and enforces rules"
    echo
    echo "Analytics (fetched by botginx):"
    echo "  - Botginx fetches stats via SSH: curl 127.0.0.1:8080/api/stats"
    echo "  - Displayed in: Hosting → Domain → Settings (Analytics tab)"
    echo
    echo "No separate antibot-dashboard needed - all features in botginx."
    echo
    echo "=========================================="
    printf '%s Custom Branding (Optional) %s\n' "$BLUE" "$RESET"
    echo "=========================================="
    echo
    echo "To customize CloudPanel with your branding:"
    echo "  scp scripts/cloudpanel-branding.sh root@${ip}:/opt/"
    echo "  ssh root@${ip} 'PANEL_NAME=YourBrand bash /opt/cloudpanel-branding.sh'"
    echo
    echo "Options:"
    echo "  PANEL_NAME=GuardHost        # Custom panel name"
    echo "  PRIMARY_COLOR=#dc3545       # Theme color (hex)"
    echo "  FORCE_DARK_THEME=true       # Dark mode default"
    echo
}

# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------

main() {
    echo
    printf '%s=== CloudPanel Deployment for Botginx ===%s\n' "$BLUE" "$RESET"
    echo

    preflight
    install_cloudpanel
    configure_nginx_botection
    prepare_botection_dirs
    create_botection_template
    configure_botection_analytics
    update_default_template
    apply_branding
    print_summary
}

main "$@"

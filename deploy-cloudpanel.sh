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
#   3. Deploy antibot-dashboard (optional):
#      See antibot-dashboard/scripts/deploy.sh
#   4. Configure botection callback URL in /var/www/antibot/config/config.yaml

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

    # CloudPanel nginx serves sites. When botection is installed:
    # - nginx listens on 8081 (backend port)
    # - botection listens on 8080, proxies to 8081 with preserve_host=true
    # - External nginx/cloudflare routes 443 → 8080

    local nginx_conf="/etc/nginx/nginx.conf"
    local sites_dir="/etc/nginx/sites-enabled"

    # CloudPanel uses a different nginx structure
    # Main nginx conf is at /etc/nginx/nginx.conf
    # Site configs are in /etc/nginx/sites-enabled/

    # Create a snippet for botection backend port configuration
    log "creating botection nginx snippet"
    cat > /etc/nginx/conf.d/botection-backend.conf <<EOF
# Botection backend port configuration
# When botection is installed, site vhosts listen on this port
# instead of 80/443 directly
# Port: ${CLOUDPANEL_BACKEND_PORT}
EOF

    # CloudPanel manages site configs dynamically
    # We'll create a hook script that modifies new sites to use backend port
    log "creating CloudPanel site hook for botection"
    mkdir -p /opt/botection/hooks

    cat > /opt/botection/hooks/cloudpanel-site-hook.sh <<'HOOKEOF'
#!/bin/bash
# Hook script to modify CloudPanel site configs for botection
# Called after CloudPanel creates/updates a site

BACKEND_PORT="${CLOUDPANEL_BACKEND_PORT:-8081}"
SITES_DIR="/etc/nginx/sites-enabled"

# Process all site configs
for conf in "$SITES_DIR"/*.conf; do
    [[ -f "$conf" ]] || continue

    # Skip if already modified
    grep -q "# botection-modified" "$conf" && continue

    # Modify listen directives to use backend port
    # Only modify HTTP port (80 -> backend port)
    # Keep SSL port as-is for direct SSL connections
    sed -i "s/listen 80;/listen ${BACKEND_PORT};/g" "$conf"
    sed -i "s/listen \[::\]:80;/listen [::]:${BACKEND_PORT};/g" "$conf"

    # Add marker comment
    sed -i "1i # botection-modified" "$conf"
done

# Reload nginx if running
if systemctl is-active --quiet nginx; then
    nginx -t 2>/dev/null && systemctl reload nginx
fi
HOOKEOF
    chmod +x /opt/botection/hooks/cloudpanel-site-hook.sh

    # Create marker file for botection deploy to detect CloudPanel
    mkdir -p /var/www/cloudpanel
    echo "cloudpanel" > /var/www/cloudpanel/.server-type

    # Export backend port for hook script
    echo "CLOUDPANEL_BACKEND_PORT=${CLOUDPANEL_BACKEND_PORT}" > /etc/botection/cloudpanel.env

    ok "nginx configured for botection (backend port: ${CLOUDPANEL_BACKEND_PORT})"

    # Test and reload nginx
    if nginx -t 2>/dev/null; then
        systemctl reload nginx 2>/dev/null || true
    fi
}

# ----------------------------------------------------------------------------
# Prepare settings directory for botection
# ----------------------------------------------------------------------------

prepare_settings_dir() {
    info "Preparing domain settings directory"

    mkdir -p /etc/botection/domains

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

    ok "settings directory prepared at /etc/botection/domains"
}

# ----------------------------------------------------------------------------
# Prepare antibot-dashboard directory
# ----------------------------------------------------------------------------

prepare_dashboard_dir() {
    info "Preparing antibot-dashboard directory"

    local dashboard_dir="/home/clp/antibot-dashboard"
    mkdir -p "$dashboard_dir"/{templates,static}

    # Create placeholder for deployment
    cat > "$dashboard_dir/README.txt" <<'EOF'
Antibot Dashboard Directory

Deploy the antibot-dashboard application here using:
  cd /path/to/antibot-dashboard
  ./scripts/deploy.sh

Or manually:
  1. Build: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o antibot-dashboard ./cmd/dashboard
  2. Upload: scp antibot-dashboard templates/ static/ root@SERVER:/home/clp/antibot-dashboard/
  3. Configure systemd service
  4. Create CloudPanel reverse proxy site pointing to localhost:8888
EOF

    # Create systemd service template
    cat > /etc/systemd/system/antibot-dashboard.service <<'EOF'
[Unit]
Description=Antibot Dashboard
After=network.target

[Service]
Type=simple
User=clp
Group=clp
WorkingDirectory=/home/clp/antibot-dashboard
ExecStart=/home/clp/antibot-dashboard/antibot-dashboard
Restart=always
RestartSec=5

# Environment
Environment=PORT=8888
Environment=BOTGINX_URL=http://127.0.0.1:3001
Environment=SESSION_SECRET=change-this-to-random-32-char-string

[Install]
WantedBy=multi-user.target
EOF

    chown -R clp:clp "$dashboard_dir" 2>/dev/null || true

    ok "antibot-dashboard directory prepared at $dashboard_dir"
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
    echo "5. (Optional) Deploy antibot-dashboard:"
    echo "   - Upload antibot-dashboard binary to /home/clp/antibot-dashboard/"
    echo "   - Update /etc/systemd/system/antibot-dashboard.service with BOTGINX_URL"
    echo "   - systemctl enable --now antibot-dashboard"
    echo "   - In CloudPanel: Create Reverse Proxy site → antibot.yourdomain.com → http://127.0.0.1:8888"
    echo
    echo "6. Route external traffic through botection:"
    echo "   - Cloudflare/nginx on 443 → botection on ${BOTECTION_PORT}"
    echo "   - Botection proxies to CloudPanel nginx on ${CLOUDPANEL_BACKEND_PORT}"
    echo
    echo "Domain Settings:"
    echo "  Each domain's bot settings stored in botginx database"
    echo "  Managed via: Hosting → Account → Domain → Settings"
    echo "  Or via antibot-dashboard (if deployed)"
    echo "  Botection fetches settings via callback API"
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
    prepare_settings_dir
    prepare_dashboard_dir
    print_summary
}

main "$@"

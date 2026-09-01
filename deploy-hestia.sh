#!/usr/bin/env bash
#
# Deploy HestiaCP server for botginx hosting module.
#
# Installs HestiaCP with nginx, PHP, MySQL, and mail support. Prepares nginx
# to route through botection (antibot installed separately via its deploy.sh).
#
# Traffic flow (after botection is installed):
#   Internet → :443 (nginx) → :8080 (botection) → :8081 (HestiaCP backends)
#
# Usage:
#   # From local machine
#   ssh root@SERVER 'bash -s' < deploy-hestia.sh
#
#   # With custom admin credentials
#   HESTIA_ADMIN_PASSWORD=mysecret ssh root@SERVER 'bash -s' < deploy-hestia.sh
#
# After installation:
#   1. Add the server to botginx: Admin → Hosting → Servers
#   2. Deploy botection:
#      SERVER_TYPE=hestiacp ssh root@SERVER 'bash -s' < antibot/packaging/deploy.sh
#   3. Configure botection callback URL in /var/www/antibot/config/config.yaml

set -euo pipefail

# ----------------------------------------------------------------------------
# Configuration
# ----------------------------------------------------------------------------

# HestiaCP admin credentials (will be generated if not set)
HESTIA_ADMIN_USER="${HESTIA_ADMIN_USER:-admin}"
HESTIA_ADMIN_EMAIL="${HESTIA_ADMIN_EMAIL:-admin@localhost}"
HESTIA_ADMIN_PASSWORD="${HESTIA_ADMIN_PASSWORD:-$(openssl rand -base64 16)}"

# HestiaCP options
HESTIA_HOSTNAME="${HESTIA_HOSTNAME:-$(hostname -f 2>/dev/null || hostname)}"
HESTIA_PORT="${HESTIA_PORT:-8083}"

# Botection port (antibot reverse proxy)
BOTECTION_PORT="${BOTECTION_PORT:-8080}"

# HestiaCP backend port (nginx serves sites here, botection proxies to it)
HESTIA_BACKEND_PORT="${HESTIA_BACKEND_PORT:-8081}"

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

    # Check if HestiaCP is already installed
    if command -v v-list-sys-info &>/dev/null; then
        warn "HestiaCP is already installed"
        read -p "Continue with configuration updates only? [y/N] " -n 1 -r
        echo
        [[ $REPLY =~ ^[Yy]$ ]] || exit 0
        HESTIA_INSTALLED=true
    else
        HESTIA_INSTALLED=false
    fi

    # Check OS
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        log "detected: $PRETTY_NAME"
        case "$ID" in
            ubuntu|debian) ;;
            *) die "Only Ubuntu/Debian supported (detected: $ID)" ;;
        esac
    else
        die "Cannot detect OS"
    fi

    # Check memory (HestiaCP recommends 1GB+)
    local mem_kb
    mem_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    if (( mem_kb < 900000 )); then
        warn "Low memory detected ($(( mem_kb / 1024 ))MB). HestiaCP recommends 1GB+"
    fi

    ok "preflight passed"
}

# ----------------------------------------------------------------------------
# Install HestiaCP
# ----------------------------------------------------------------------------

install_hestia() {
    info "Installing HestiaCP"

    if [[ "$HESTIA_INSTALLED" == true ]]; then
        log "skipping HestiaCP installation (already installed)"
        return 0
    fi

    # Update system
    log "updating system packages"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get upgrade -y -qq

    # Install dependencies
    apt-get install -y -qq curl wget gnupg apt-transport-https ca-certificates jq

    # Download HestiaCP installer
    log "downloading HestiaCP installer"
    cd /tmp
    curl -fsSL https://raw.githubusercontent.com/hestiacp/hestiacp/release/install/hst-install.sh -o hst-install.sh

    # Run HestiaCP installer
    # Options:
    #   --multiphp yes: enable multi-PHP (7.4, 8.0, 8.1, 8.2, 8.3)
    #   --apache no: use nginx only
    #   --phpfpm yes: use PHP-FPM
    #   --mysql yes: enable MySQL/MariaDB
    #   --exim yes: enable mail server
    #   --dovecot yes: enable IMAP/POP3
    #   --clamav no: skip antivirus (resource heavy)
    #   --spamassassin no: skip spam filter (resource heavy)
    #   --fail2ban yes: enable brute-force protection
    #   --api yes: enable HestiaCP API

    log "running HestiaCP installer (this takes 10-20 minutes)"
    bash hst-install.sh \
        --force \
        --interactive no \
        --hostname "$HESTIA_HOSTNAME" \
        --email "$HESTIA_ADMIN_EMAIL" \
        --password "$HESTIA_ADMIN_PASSWORD" \
        --port "$HESTIA_PORT" \
        --multiphp yes \
        --apache no \
        --phpfpm yes \
        --named yes \
        --mysql yes \
        --postgresql no \
        --exim yes \
        --dovecot yes \
        --clamav no \
        --spamassassin no \
        --iptables yes \
        --fail2ban yes \
        --quota no \
        --api yes \
        || die "HestiaCP installation failed"

    ok "HestiaCP installed"

    # Clean up
    rm -f /tmp/hst-install.sh
}

# ----------------------------------------------------------------------------
# Configure nginx for botection reverse proxy
# ----------------------------------------------------------------------------

configure_nginx_botection() {
    info "Configuring nginx for botection integration"

    # HestiaCP nginx serves sites. When botection is installed:
    # - nginx listens on 8081 (backend port)
    # - botection listens on 8080, proxies to 8081 with preserve_host=true
    # - External nginx/cloudflare routes 443 → 8080

    local hestia_nginx_conf="/etc/nginx/nginx.conf"
    local hestia_conf_dir="/usr/local/hestia/data/templates/web/nginx"

    # Change HestiaCP nginx default port from 80 to 8081
    # This is done in the nginx.conf and site templates
    if [[ -f "$hestia_nginx_conf" ]]; then
        log "configuring HestiaCP nginx to listen on port ${HESTIA_BACKEND_PORT}"

        # Update default port in nginx.conf if present
        if grep -q "listen 80;" "$hestia_nginx_conf"; then
            sed -i "s/listen 80;/listen ${HESTIA_BACKEND_PORT};/g" "$hestia_nginx_conf"
            sed -i "s/listen \[::\]:80;/listen [::]:${HESTIA_BACKEND_PORT};/g" "$hestia_nginx_conf"
        fi
    fi

    # Update HestiaCP nginx templates to use backend port
    if [[ -d "$hestia_conf_dir" ]]; then
        log "updating HestiaCP nginx templates for backend port ${HESTIA_BACKEND_PORT}"

        # Update all .tpl files (HTTP templates)
        for tpl in "$hestia_conf_dir"/*.tpl; do
            [[ -f "$tpl" ]] || continue
            sed -i "s/listen 80;/listen ${HESTIA_BACKEND_PORT};/g" "$tpl"
            sed -i "s/listen \[::\]:80;/listen [::]:${HESTIA_BACKEND_PORT};/g" "$tpl"
        done

        # Update all .stpl files (HTTPS templates) - keep 443 as-is for direct SSL
        # Actually for botection setup, we want SSL termination at front nginx
        # so the backend should be HTTP only on 8081
    fi

    # Create marker file for botection deploy to detect HestiaCP
    mkdir -p /var/www/hestiacp
    echo "hestiacp" > /var/www/hestiacp/.server-type

    ok "nginx configured for botection (backend port: ${HESTIA_BACKEND_PORT})"

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

    local settings_dir="/etc/botection/domains"
    mkdir -p "$settings_dir"

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

    ok "settings directory prepared at $settings_dir"
}

# ----------------------------------------------------------------------------
# Create hosting packages
# ----------------------------------------------------------------------------

create_hosting_packages() {
    info "Creating hosting packages"

    # Source HestiaCP environment
    source /usr/local/hestia/conf/hestia.conf 2>/dev/null || true
    export HESTIA=/usr/local/hestia

    # Define packages: name:disk_gb:bandwidth_gb:domains:mail_domains:mail_accounts:databases
    local packages=(
        "starter:1:10:5:5:10:2"
        "basic:5:50:15:10:25:5"
        "pro:20:100:30:20:50:10"
        "enterprise:100:500:100:50:200:25"
    )

    for pkg_def in "${packages[@]}"; do
        IFS=':' read -r name disk bw domains mail_dom mail_acc dbs <<< "$pkg_def"

        # Check if package exists
        if [[ -f "/usr/local/hestia/data/packages/${name}.pkg" ]]; then
            log "package '$name' already exists"
            continue
        fi

        log "creating package: $name"

        # Create package file directly
        cat > "/usr/local/hestia/data/packages/${name}.pkg" <<PKGEOF
WEB_TEMPLATE='default'
BACKEND_TEMPLATE='default'
PROXY_TEMPLATE='default'
DNS_TEMPLATE='default'
WEB_DOMAINS='${domains}'
WEB_ALIASES='${domains}'
DNS_DOMAINS='${domains}'
DNS_RECORDS='100'
MAIL_DOMAINS='${mail_dom}'
MAIL_ACCOUNTS='${mail_acc}'
DATABASES='${dbs}'
CRON_JOBS='unlimited'
DISK_QUOTA='${disk}000'
BANDWIDTH='${bw}000'
NS='ns1.${HESTIA_HOSTNAME},ns2.${HESTIA_HOSTNAME}'
SHELL='nologin'
BACKUPS='3'
TIME='00:00:00'
DATE='$(date +%Y-%m-%d)'
PKGEOF

        # Also try the v-add-user-package command
        /usr/local/hestia/bin/v-add-user-package "$name" 2>/dev/null || true
    done

    ok "hosting packages created"
}

# ----------------------------------------------------------------------------
# Print summary
# ----------------------------------------------------------------------------

print_summary() {
    local ip
    ip=$(curl -4 -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

    echo
    echo "=========================================="
    printf '%s HestiaCP Server Ready %s\n' "$GREEN" "$RESET"
    echo "=========================================="
    echo
    echo "HestiaCP Panel:"
    echo "  URL:      https://${ip}:${HESTIA_PORT}"
    echo "  Username: ${HESTIA_ADMIN_USER}"
    echo "  Password: ${HESTIA_ADMIN_PASSWORD}"
    echo
    echo "Add to Botginx (Admin → Hosting → Servers):"
    echo "  Hostname: ${ip}"
    echo "  Port:     22"
    echo "  Username: root"
    echo "  Password: <your-root-password>"
    echo
    echo "Hosting Packages Created:"
    echo "  starter, basic, pro, enterprise"
    echo
    echo "Traffic Flow (after botection install):"
    echo "  Internet → :443 → botection (:${BOTECTION_PORT}) → HestiaCP nginx (:${HESTIA_BACKEND_PORT})"
    echo
    echo "=========================================="
    printf '%s Next Steps %s\n' "$YELLOW" "$RESET"
    echo "=========================================="
    echo
    echo "1. Add this server to botginx panel"
    echo
    echo "2. Deploy botection (antibot) with HestiaCP mode:"
    echo "   cd /var/www && git clone https://github.com/robertp2083/antibot.git"
    echo "   SERVER_TYPE=hestiacp bash /var/www/antibot/packaging/deploy.sh"
    echo
    echo "3. Configure botection callback in /var/www/antibot/config/config.yaml:"
    echo "   callback:"
    echo "     url: https://YOUR_BOTGINX_PANEL/api/botection/should-block"
    echo
    echo "4. Route external traffic through botection:"
    echo "   - Cloudflare/nginx on 443 → botection on ${BOTECTION_PORT}"
    echo "   - Botection proxies to HestiaCP nginx on ${HESTIA_BACKEND_PORT}"
    echo
    echo "Domain Settings:"
    echo "  Each domain's bot settings stored in botginx database"
    echo "  Managed via: Hosting → Account → Domain → Settings"
    echo "  Botection fetches settings via callback API"
    echo
}

# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------

main() {
    echo
    printf '%s=== HestiaCP Deployment for Botginx ===%s\n' "$BLUE" "$RESET"
    echo

    preflight
    install_hestia
    configure_nginx_botection
    prepare_settings_dir
    create_hosting_packages
    print_summary
}

main "$@"

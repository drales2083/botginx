#!/usr/bin/env bash
#
# CloudPanel Branding Customization Script
#
# Customizes CloudPanel with:
# - GaurdBotPanel logo
# - Purple theme (#6c5ce7)
# - Dark mode as default (via JS, uses native toggle)
#
# Usage:
#   scp scripts/cloudpanel-branding.sh root@SERVER:/opt/
#   scp assets/logos/guardbotpanel-logo.png root@SERVER:/opt/cloudpanel-branding/logos/
#   ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
#
# Re-run after CloudPanel updates to restore customizations.

set -euo pipefail

# ----------------------------------------------------------------------------
# Configuration
# ----------------------------------------------------------------------------

PANEL_NAME="${PANEL_NAME:-GaurdBotPanel}"
PRIMARY_COLOR="${PRIMARY_COLOR:-#6c5ce7}"  # Purple to match logo

# ----------------------------------------------------------------------------
# Output helpers
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
# Paths
# ----------------------------------------------------------------------------

CLOUDPANEL_DIR="/home/clp/htdocs/app"
PUBLIC_DIR="$CLOUDPANEL_DIR/files/public"
BRANDING_DIR="/opt/cloudpanel-branding"

# ----------------------------------------------------------------------------
# Preflight
# ----------------------------------------------------------------------------

preflight() {
    info "Preflight checks"
    [[ $EUID -eq 0 ]] || die "Must run as root"
    [[ -d "$CLOUDPANEL_DIR" ]] || die "CloudPanel not found at $CLOUDPANEL_DIR"
    ok "CloudPanel found"
}

# ----------------------------------------------------------------------------
# Setup directories
# ----------------------------------------------------------------------------

setup_dirs() {
    info "Setting up branding directory"
    mkdir -p "$BRANDING_DIR"/{css,js,logos}
    mkdir -p "$PUBLIC_DIR"/{css,js,images}
    ok "directories ready"
}

# ----------------------------------------------------------------------------
# Create custom CSS (branding only, no dark mode forcing)
# ----------------------------------------------------------------------------

create_css() {
    info "Creating custom CSS"

    cat > "$BRANDING_DIR/css/custom-branding.css" << 'EOF'
/*
 * CloudPanel Custom Branding - GaurdBotPanel
 * Purple theme + logo only - dark mode handled by JS
 */

/* Primary color - Purple */
:root {
    --bs-primary: #6c5ce7 !important;
    --bs-primary-rgb: 108, 92, 231 !important;
}

/* Button colors */
.btn-primary {
    background-color: #6c5ce7 !important;
    border-color: #6c5ce7 !important;
}

.btn-primary:hover,
.btn-primary:focus {
    background-color: #5b4cdb !important;
    border-color: #5b4cdb !important;
}

.btn-outline-primary {
    color: #6c5ce7 !important;
    border-color: #6c5ce7 !important;
}

.btn-outline-primary:hover {
    background-color: #6c5ce7 !important;
    color: #fff !important;
}

/* Link colors */
a:not(.btn) {
    color: #6c5ce7;
}

a:not(.btn):hover {
    color: #5b4cdb;
}

/* Sidebar active */
.nav-link.active {
    background-color: #6c5ce7 !important;
}

/* Progress bars */
.progress-bar {
    background-color: #6c5ce7 !important;
}

/* Replace CloudPanel logo with GaurdBotPanel logo */
.navbar-brand img,
.login-logo img,
img[alt*="CloudPanel"],
img[src*="cloudpanel"],
img[src*="logo"] {
    content: url("/images/guardbotpanel-logo.png") !important;
    max-height: 21px !important;
    width: auto !important;
}

/* Login page logo larger */
.login-logo img,
.text-center img[src*="logo"] {
    content: url("/images/guardbotpanel-logo.png") !important;
    max-height: 30px !important;
    width: auto !important;
}

/* Hide CloudPanel text */
.navbar-brand span,
.sidebar-brand span {
    display: none !important;
}

/* Hide theme toggle - force dark mode only */
.theme-switcher,
#theme-switch,
li.theme-switcher {
    display: none !important;
}

/* ===== FOOTER CLEANUP ===== */

/* Hide Blog, Docs, Issues, Contact links */
footer ul li a[href*="cloudpanel.io/blog"],
footer ul li a[href*="cloudpanel.io/docs"],
footer ul li a[href*="github.com/cloudpanel"],
footer ul li a[href*="cloudpanel.io/contact"],
footer ul li a[title="Blog"],
footer ul li a[title="Docs"],
footer ul li a[title="Issues"],
footer ul li a[title="Contact"] {
    display: none !important;
}

/* Hide the li containing those links */
footer ul li:has(a[href*="cloudpanel.io/blog"]),
footer ul li:has(a[href*="cloudpanel.io/docs"]),
footer ul li:has(a[href*="github.com/cloudpanel"]),
footer ul li:has(a[href*="cloudpanel.io/contact"]) {
    display: none !important;
}

/* Hide the CloudPanel copyright and update button */
footer ul li a[href="https://www.cloudpanel.io"],
footer ul li #update-available-button {
    display: none !important;
}

/* Replace copyright text */
footer ul li:last-child {
    font-size: 0;
}

footer ul li:last-child::before {
    content: "© 2026 All Rights Reserved";
    font-size: 0.875rem;
}

/* Append " | Bullet Proof Hosting" to page title */
.page-title h1::after {
    content: " | Bullet Proof Hosting";
}
EOF

    cp "$BRANDING_DIR/css/custom-branding.css" "$PUBLIC_DIR/css/"
    ok "CSS created"
}

# ----------------------------------------------------------------------------
# Update translations (Dashboard -> Bullet Proof Hosting)
# ----------------------------------------------------------------------------

update_translations() {
    info "Updating translations"
    # No translation changes needed - using CSS for nav link only
    ok "translations unchanged"
}

# ----------------------------------------------------------------------------
# Create JavaScript for dark mode
# ----------------------------------------------------------------------------

create_js() {
    info "Creating dark mode JavaScript"

    mkdir -p "$BRANDING_DIR/js"
    cat > "$BRANDING_DIR/js/custom-branding.js" << 'EOF'
// GaurdBotPanel - Force dark mode (toggle hidden via CSS)
(function() {
    // Always set dark theme cookie
    document.cookie = "theme=dark; expires=" + new Date(Date.now() + 180*24*60*60*1000).toUTCString() + "; path=/; secure";

    // Apply dark theme: <html id="html" data-bs-theme="dark" class="dark">
    document.documentElement.id = 'html';
    document.documentElement.setAttribute('data-bs-theme', 'dark');
    document.documentElement.classList.add('dark');
})();
EOF

    mkdir -p "$PUBLIC_DIR/js"
    cp "$BRANDING_DIR/js/custom-branding.js" "$PUBLIC_DIR/js/"
    ok "JavaScript created"
}

# ----------------------------------------------------------------------------
# Copy logo
# ----------------------------------------------------------------------------

copy_logo() {
    info "Setting up logo"

    # Check if logo exists in branding dir
    if [[ -f "$BRANDING_DIR/logos/guardbotpanel-logo.png" ]]; then
        cp "$BRANDING_DIR/logos/guardbotpanel-logo.png" "$PUBLIC_DIR/images/"
        ok "logo copied to public"
    else
        warn "Logo not found at $BRANDING_DIR/logos/guardbotpanel-logo.png"
        log "Upload logo: scp guardbotpanel-logo.png root@SERVER:$BRANDING_DIR/logos/"
    fi
}

# ----------------------------------------------------------------------------
# Inject CSS and JS into CloudPanel templates
# ----------------------------------------------------------------------------

inject_assets() {
    info "Injecting assets into CloudPanel templates"

    local layouts=(
        "$CLOUDPANEL_DIR/files/templates/Frontend/layout.html.twig"
        "$CLOUDPANEL_DIR/files/templates/Frontend/Login/layout.html.twig"
        "$CLOUDPANEL_DIR/files/templates/Admin/layout.html.twig"
    )

    local css_link='<link rel="stylesheet" href="/css/custom-branding.css">'
    local js_link='<script src="/js/custom-branding.js"></script>'

    for layout in "${layouts[@]}"; do
        if [[ -f "$layout" ]]; then
            # Inject CSS if not present
            if ! grep -q "custom-branding.css" "$layout" 2>/dev/null; then
                sed -i "s|</head>|    ${css_link}\n    </head>|" "$layout"
                ok "CSS injected: $(basename "$layout")"
            else
                log "CSS already in: $(basename "$layout")"
            fi

            # Inject JS if not present
            if ! grep -q "custom-branding.js" "$layout" 2>/dev/null; then
                sed -i "s|</head>|    ${js_link}\n    </head>|" "$layout"
                ok "JS injected: $(basename "$layout")"
            else
                log "JS already in: $(basename "$layout")"
            fi
        fi
    done
}

# ----------------------------------------------------------------------------
# Create update hook (cron)
# ----------------------------------------------------------------------------

create_hook() {
    info "Creating update hook"

    cat > /etc/cron.daily/cloudpanel-branding << 'EOF'
#!/bin/bash
# Re-apply CloudPanel branding after updates
CSS_SRC="/opt/cloudpanel-branding/css/custom-branding.css"
CSS_DST="/home/clp/htdocs/app/files/public/css/custom-branding.css"

if [[ -f "$CSS_SRC" ]]; then
    if [[ ! -f "$CSS_DST" ]] || ! cmp -s "$CSS_SRC" "$CSS_DST"; then
        bash /opt/cloudpanel-branding.sh 2>/dev/null
    fi
fi
EOF
    chmod +x /etc/cron.daily/cloudpanel-branding

    # Copy this script
    cp "$0" "$BRANDING_DIR/cloudpanel-branding.sh" 2>/dev/null || true

    ok "update hook created"
}

# ----------------------------------------------------------------------------
# Clear cache
# ----------------------------------------------------------------------------

clear_cache() {
    info "Clearing cache"
    rm -rf "$CLOUDPANEL_DIR/files/var/cache/"* 2>/dev/null || true
    ok "cache cleared"
}

# ----------------------------------------------------------------------------
# Summary
# ----------------------------------------------------------------------------

print_summary() {
    echo
    echo "=========================================="
    printf '%s GaurdBotPanel Branding Applied %s\n' "$GREEN" "$RESET"
    echo "=========================================="
    echo
    echo "Panel Name:  GaurdBotPanel"
    echo "Theme Color: Purple (#6c5ce7)"
    echo "Dark Mode:   Forced (toggle hidden)"
    echo "Nav Link:    Bullet Proof Hosting (page title stays Dashboard)"
    echo
    echo "Files:"
    echo "  Logo: $PUBLIC_DIR/images/guardbotpanel-logo.png"
    echo "  CSS:  $PUBLIC_DIR/css/custom-branding.css"
    echo "  JS:   $PUBLIC_DIR/js/custom-branding.js"
    echo
    echo "Re-run after CloudPanel updates:"
    echo "  bash /opt/cloudpanel-branding.sh"
    echo
}

# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------

main() {
    echo
    printf '%s=== GaurdBotPanel Branding ===%s\n' "$BLUE" "$RESET"
    echo

    preflight
    setup_dirs
    create_css
    create_js
    copy_logo
    update_translations
    inject_assets
    create_hook
    clear_cache
    print_summary
}

main "$@"

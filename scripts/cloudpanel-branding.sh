#!/usr/bin/env bash
#
# CloudPanel Branding Customization Script
#
# Customizes CloudPanel with:
# - Custom logo
# - Custom panel name
# - Dark theme as default
# - Custom CSS overrides
#
# Usage:
#   # Upload and run on CloudPanel server
#   scp cloudpanel-branding.sh root@SERVER:/opt/
#   ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
#
#   # Or directly
#   ssh root@SERVER 'bash -s' < scripts/cloudpanel-branding.sh
#
# Re-run after CloudPanel updates to restore customizations.

set -euo pipefail

# ----------------------------------------------------------------------------
# Configuration - CUSTOMIZE THESE
# ----------------------------------------------------------------------------

# Panel name (replaces "CloudPanel" in UI)
PANEL_NAME="${PANEL_NAME:-GuardBotPanel}"

# Primary color (hex, used for buttons, links, accents)
PRIMARY_COLOR="${PRIMARY_COLOR:-#dc3545}"  # Red to match botginx theme

# Logo URLs (optional - leave empty to skip)
# Can be local path or URL
LOGO_LIGHT="${LOGO_LIGHT:-}"  # Logo for light theme
LOGO_DARK="${LOGO_DARK:-}"    # Logo for dark theme
LOGO_FAVICON="${LOGO_FAVICON:-}"  # Favicon

# Force dark theme for all users
FORCE_DARK_THEME="${FORCE_DARK_THEME:-true}"

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

    if [[ ! -d "$CLOUDPANEL_DIR" ]]; then
        die "CloudPanel not found at $CLOUDPANEL_DIR"
    fi

    ok "CloudPanel found"
}

# ----------------------------------------------------------------------------
# Create branding directory
# ----------------------------------------------------------------------------

setup_branding_dir() {
    info "Setting up branding directory"

    mkdir -p "$BRANDING_DIR"/{css,logos}

    ok "branding directory: $BRANDING_DIR"
}

# ----------------------------------------------------------------------------
# Create custom CSS
# ----------------------------------------------------------------------------

create_custom_css() {
    info "Creating custom CSS"

    cat > "$BRANDING_DIR/css/custom-branding.css" <<EOF
/*
 * CloudPanel Custom Branding - ${PANEL_NAME}
 * Generated: $(date)
 */

/* Primary color - Red theme */
:root {
    --bs-primary: ${PRIMARY_COLOR} !important;
    --bs-primary-rgb: 220, 53, 69 !important;
}

/* Button colors */
.btn-primary {
    background-color: ${PRIMARY_COLOR} !important;
    border-color: ${PRIMARY_COLOR} !important;
}

.btn-primary:hover,
.btn-primary:focus {
    background-color: #bb2d3b !important;
    border-color: #b02a37 !important;
}

.btn-outline-primary {
    color: ${PRIMARY_COLOR} !important;
    border-color: ${PRIMARY_COLOR} !important;
}

.btn-outline-primary:hover {
    background-color: ${PRIMARY_COLOR} !important;
    color: #fff !important;
}

/* Link colors */
a:not(.btn) {
    color: ${PRIMARY_COLOR};
}

a:not(.btn):hover {
    color: #bb2d3b;
}

/* Sidebar active */
.nav-link.active {
    background-color: ${PRIMARY_COLOR} !important;
}

/* Progress bars */
.progress-bar {
    background-color: ${PRIMARY_COLOR} !important;
}

/* Replace CloudPanel branding with ${PANEL_NAME} */
.navbar-brand img,
.login-logo img,
img[alt*="CloudPanel"],
img[src*="cloudpanel"] {
    content: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 200 40'%3E%3Ctext x='10' y='28' font-family='Arial,sans-serif' font-size='20' font-weight='bold' fill='%23dc3545'%3E${PANEL_NAME}%3C/text%3E%3C/svg%3E") !important;
    height: 32px !important;
}

/* Hide CloudPanel text in navbar */
.navbar-brand span,
.sidebar-brand span {
    font-size: 0 !important;
}

.navbar-brand span::after,
.sidebar-brand span::after {
    content: "${PANEL_NAME}" !important;
    font-size: 1.25rem !important;
    font-weight: 600 !important;
    color: ${PRIMARY_COLOR} !important;
}
EOF

    ok "custom CSS created"
}

# ----------------------------------------------------------------------------
# Create dark theme default CSS
# ----------------------------------------------------------------------------

create_dark_theme_css() {
    if [[ "$FORCE_DARK_THEME" != "true" ]]; then
        log "dark theme forcing disabled"
        return
    fi

    info "Creating dark theme default"

    cat >> "$BRANDING_DIR/css/custom-branding.css" <<'EOF'

/* Dark theme as default */
@media (prefers-color-scheme: light) {
    :root {
        color-scheme: dark;
    }
}

/* Force dark theme */
html:not([data-bs-theme="light"]) {
    --bs-body-bg: #212529;
    --bs-body-color: #dee2e6;
}

body {
    background-color: var(--bs-body-bg) !important;
    color: var(--bs-body-color) !important;
}

/* Dark sidebar */
.sidebar {
    background-color: #1a1d21 !important;
}

/* Dark cards */
.card {
    background-color: #2b3035 !important;
    border-color: #373b3e !important;
}

/* Dark tables */
.table {
    --bs-table-bg: #2b3035;
    --bs-table-color: #dee2e6;
    --bs-table-border-color: #373b3e;
}

/* Dark inputs */
.form-control,
.form-select {
    background-color: #2b3035 !important;
    border-color: #495057 !important;
    color: #dee2e6 !important;
}

/* Dark dropdowns */
.dropdown-menu {
    background-color: #2b3035 !important;
    border-color: #495057 !important;
}

.dropdown-item {
    color: #dee2e6 !important;
}

.dropdown-item:hover {
    background-color: #373b3e !important;
}

/* Dark modals */
.modal-content {
    background-color: #2b3035 !important;
    border-color: #495057 !important;
}

.modal-header,
.modal-footer {
    border-color: #495057 !important;
}
EOF

    ok "dark theme default CSS added"
}

# ----------------------------------------------------------------------------
# Inject CSS into CloudPanel
# ----------------------------------------------------------------------------

inject_css() {
    info "Injecting custom CSS into CloudPanel"

    # Copy CSS to public directory
    mkdir -p "$PUBLIC_DIR/css"
    cp "$BRANDING_DIR/css/custom-branding.css" "$PUBLIC_DIR/css/"

    # CloudPanel v2 layout files
    local layouts=(
        "$CLOUDPANEL_DIR/files/templates/Frontend/layout.html.twig"
        "$CLOUDPANEL_DIR/files/templates/Frontend/Login/layout.html.twig"
        "$CLOUDPANEL_DIR/files/templates/Admin/layout.html.twig"
    )

    local css_link='<link rel="stylesheet" href="/css/custom-branding.css">'
    local injected=false

    for layout in "${layouts[@]}"; do
        if [[ -f "$layout" ]]; then
            # Check if already injected
            if grep -q "custom-branding.css" "$layout" 2>/dev/null; then
                log "CSS already in $layout"
                injected=true
                continue
            fi

            # Inject before </head>
            if grep -q "</head>" "$layout"; then
                sed -i "s|</head>|    ${css_link}\n    </head>|" "$layout"
                ok "CSS injected into $layout"
                injected=true
            fi
        fi
    done

    if [[ "$injected" == "false" ]]; then
        warn "Could not find layout files to inject CSS"
        log "Manual injection may be needed"
        log "Add this before </head>: $css_link"
    fi
}

# ----------------------------------------------------------------------------
# Handle logos
# ----------------------------------------------------------------------------

setup_logos() {
    if [[ -z "$LOGO_LIGHT" && -z "$LOGO_DARK" && -z "$LOGO_FAVICON" ]]; then
        log "no custom logos configured"
        return
    fi

    info "Setting up custom logos"

    # Find logo locations in CloudPanel
    local logo_dirs=(
        "$PUBLIC_DIR/images"
        "$PUBLIC_DIR/img"
        "$PUBLIC_DIR/assets/images"
    )

    for dir in "${logo_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            # Backup originals
            shopt -s nullglob
            for logo in "$dir"/logo*.png "$dir"/logo*.svg "$dir"/cloudpanel*.png "$dir"/cloudpanel*.svg; do
                if [[ -f "$logo" && ! -f "${logo}.original" ]]; then
                    cp "$logo" "${logo}.original"
                    log "backed up: $logo"
                fi
            done
            shopt -u nullglob
        fi
    done

    # Copy custom logos if provided
    if [[ -n "$LOGO_LIGHT" && -f "$LOGO_LIGHT" ]]; then
        cp "$LOGO_LIGHT" "$BRANDING_DIR/logos/logo-light.png"
        ok "light logo saved"
    fi

    if [[ -n "$LOGO_DARK" && -f "$LOGO_DARK" ]]; then
        cp "$LOGO_DARK" "$BRANDING_DIR/logos/logo-dark.png"
        ok "dark logo saved"
    fi

    if [[ -n "$LOGO_FAVICON" && -f "$LOGO_FAVICON" ]]; then
        cp "$LOGO_FAVICON" "$BRANDING_DIR/logos/favicon.ico"
        # Copy to public dir
        cp "$BRANDING_DIR/logos/favicon.ico" "$PUBLIC_DIR/favicon.ico" 2>/dev/null || true
        ok "favicon updated"
    fi
}

# ----------------------------------------------------------------------------
# Set dark theme in database (for new users)
# ----------------------------------------------------------------------------

set_dark_theme_default() {
    if [[ "$FORCE_DARK_THEME" != "true" ]]; then
        return
    fi

    info "Setting dark theme as default in database"

    # CloudPanel uses SQLite
    local db_file="$CLOUDPANEL_DIR/files/db/cloudpanel.db"

    if [[ -f "$db_file" ]]; then
        # Update existing users to dark theme
        sqlite3 "$db_file" "UPDATE user SET theme = 'dark' WHERE theme IS NULL OR theme = 'light';" 2>/dev/null && \
            ok "existing users set to dark theme" || \
            warn "could not update user themes (may need different column name)"

        # Try to set default in settings if table exists
        sqlite3 "$db_file" "INSERT OR REPLACE INTO setting (name, value) VALUES ('default_theme', 'dark');" 2>/dev/null || true
    else
        warn "database not found at $db_file"
    fi
}

# ----------------------------------------------------------------------------
# Create update hook
# ----------------------------------------------------------------------------

create_update_hook() {
    info "Creating CloudPanel update hook"

    # Create a script that re-applies branding after CloudPanel updates
    cat > /etc/cron.daily/cloudpanel-branding <<'EOF'
#!/bin/bash
# Re-apply CloudPanel branding after updates
# Checks if CSS injection is still present

BRANDING_CSS="/home/clp/htdocs/app/files/public/css/custom-branding.css"
SOURCE_CSS="/opt/cloudpanel-branding/css/custom-branding.css"

if [[ -f "$SOURCE_CSS" ]]; then
    # Check if CSS file is missing or outdated
    if [[ ! -f "$BRANDING_CSS" ]] || ! cmp -s "$SOURCE_CSS" "$BRANDING_CSS"; then
        bash /opt/cloudpanel-branding.sh 2>/dev/null
    fi
fi
EOF
    chmod +x /etc/cron.daily/cloudpanel-branding

    # Also copy this script to branding dir for manual re-runs
    cp "$0" "$BRANDING_DIR/cloudpanel-branding.sh" 2>/dev/null || true

    ok "update hook created at /etc/cron.daily/cloudpanel-branding"
}

# ----------------------------------------------------------------------------
# Clear CloudPanel cache
# ----------------------------------------------------------------------------

clear_cache() {
    info "Clearing CloudPanel cache"

    # Clear Symfony cache
    rm -rf "$CLOUDPANEL_DIR/files/var/cache/"* 2>/dev/null || true

    # Clear any compiled assets
    rm -rf "$PUBLIC_DIR/build/"*.js.map 2>/dev/null || true

    ok "cache cleared"
}

# ----------------------------------------------------------------------------
# Print summary
# ----------------------------------------------------------------------------

print_summary() {
    echo
    echo "=========================================="
    printf '%s CloudPanel Branding Applied %s\n' "$GREEN" "$RESET"
    echo "=========================================="
    echo
    echo "Panel Name:    ${PANEL_NAME}"
    echo "Primary Color: ${PRIMARY_COLOR}"
    echo "Dark Theme:    ${FORCE_DARK_THEME}"
    echo
    echo "Files:"
    echo "  CSS:     $PUBLIC_DIR/css/custom-branding.css"
    echo "  Source:  $BRANDING_DIR/"
    echo
    echo "To customize further, edit:"
    echo "  $BRANDING_DIR/css/custom-branding.css"
    echo
    echo "Then re-run this script or copy CSS to public dir."
    echo
    echo "The branding will be re-applied daily via cron"
    echo "in case CloudPanel updates overwrite changes."
    echo
}

# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------

main() {
    echo
    printf '%s=== CloudPanel Branding ===%s\n' "$BLUE" "$RESET"
    echo

    preflight
    setup_branding_dir
    create_custom_css
    create_dark_theme_css
    inject_css
    setup_logos
    set_dark_theme_default
    create_update_hook
    clear_cache
    print_summary
}

main "$@"

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
PANEL_NAME="${PANEL_NAME:-GuardHost}"

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
 * CloudPanel Custom Branding
 * Generated: $(date)
 * Panel Name: ${PANEL_NAME}
 */

/* Primary color overrides */
:root {
    --clp-primary: ${PRIMARY_COLOR} !important;
    --clp-primary-hover: ${PRIMARY_COLOR}dd !important;
    --bs-primary: ${PRIMARY_COLOR} !important;
    --bs-primary-rgb: $(echo "${PRIMARY_COLOR}" | sed 's/#//' | sed 's/\(..\)\(..\)\(..\)/\1, \2, \3/' | xargs -I{} printf "%d, %d, %d" 0x{}) !important;
}

/* Button colors */
.btn-primary {
    background-color: ${PRIMARY_COLOR} !important;
    border-color: ${PRIMARY_COLOR} !important;
}

.btn-primary:hover,
.btn-primary:focus {
    background-color: ${PRIMARY_COLOR}dd !important;
    border-color: ${PRIMARY_COLOR}dd !important;
}

/* Link colors */
a {
    color: ${PRIMARY_COLOR};
}

a:hover {
    color: ${PRIMARY_COLOR}dd;
}

/* Sidebar active item */
.sidebar .nav-link.active {
    background-color: ${PRIMARY_COLOR} !important;
}

/* Progress bars */
.progress-bar {
    background-color: ${PRIMARY_COLOR} !important;
}

/* Replace CloudPanel text with custom name */
.navbar-brand,
.login-logo,
.sidebar-brand {
    font-size: 0 !important;
}

.navbar-brand::after,
.login-logo::after,
.sidebar-brand::after {
    content: "${PANEL_NAME}" !important;
    font-size: 1.25rem !important;
    font-weight: 600 !important;
}

/* Hide original logo text if needed */
.logo-text {
    visibility: hidden;
    position: relative;
}

.logo-text::after {
    content: "${PANEL_NAME}";
    visibility: visible;
    position: absolute;
    left: 0;
}

/* Footer branding */
.footer-copyright {
    visibility: hidden;
    position: relative;
}

.footer-copyright::after {
    content: "Powered by ${PANEL_NAME}";
    visibility: visible;
    position: absolute;
    left: 0;
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

/* Force dark theme as default */
@media (prefers-color-scheme: light) {
    :root {
        color-scheme: dark !important;
    }
}

/* Dark theme colors (applied always) */
html:not([data-bs-theme="light"]) body,
html[data-bs-theme="dark"] body,
body {
    --bs-body-bg: #1a1d21 !important;
    --bs-body-color: #e9ecef !important;
}

/* Ensure dark theme is default */
html {
    data-bs-theme: dark;
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
    cp "$BRANDING_DIR/css/custom-branding.css" "$PUBLIC_DIR/css/" 2>/dev/null || {
        mkdir -p "$PUBLIC_DIR/css"
        cp "$BRANDING_DIR/css/custom-branding.css" "$PUBLIC_DIR/css/"
    }

    # Find the main layout file(s) and inject CSS link
    local layouts=(
        "$CLOUDPANEL_DIR/files/templates/layout/app.html.twig"
        "$CLOUDPANEL_DIR/files/templates/layout/base.html.twig"
        "$CLOUDPANEL_DIR/files/templates/layout.html.twig"
        "$CLOUDPANEL_DIR/templates/layout/app.html.twig"
        "$CLOUDPANEL_DIR/templates/base.html.twig"
    )

    local css_link='<link rel="stylesheet" href="/css/custom-branding.css?v='$(date +%s)'">'
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
                sed -i "s|</head>|    ${css_link}\n</head>|" "$layout"
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
            for logo in "$dir"/logo*.{png,svg} "$dir"/cloudpanel*.{png,svg} 2>/dev/null; do
                if [[ -f "$logo" && ! -f "${logo}.original" ]]; then
                    cp "$logo" "${logo}.original"
                    log "backed up: $logo"
                fi
            done
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

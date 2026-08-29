#!/bin/bash
#
# Botginx — Auto Update (GitHub Releases, runs via cron)
#
# Checks the latest release every few minutes. If its version differs from the
# installed binary, downloads the new asset, verifies its checksum, swaps it in
# atomically, and restarts the service. No building — binaries are prebuilt.
#
# Cron setup (every 2 minutes):
#   */2 * * * * /opt/botginx/auto-deploy.sh >> /opt/botginx/logs/auto-deploy.log 2>&1
#
# Version: 1.3

LOCK_FILE="/tmp/botginx-deploy.lock"

# Production paths (set via env or use defaults)
APP_DIR="${APP_DIR:-/opt/botginx}"
BINARY="${BINARY:-$APP_DIR/bin/botginx}"
CONFIG_DIR="${CONFIG_DIR:-/etc/botginx}"
REPO="${BOTGINX_REPO:-robertp2083/botginx}"

mkdir -p "$APP_DIR/logs"

# Cron has no environment — load GITHUB_TOKEN from env file.
# shellcheck disable=SC1091
[ -f "$CONFIG_DIR/botginx.env" ] && . "$CONFIG_DIR/botginx.env"

# Detect architecture
case "$(uname -m)" in
    x86_64|amd64) ASSET="botginx-linux-amd64" ;;
    aarch64|arm64) ASSET="botginx-linux-arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)"; exit 0 ;;
esac

# Single-instance lock (prevent overlapping runs)
exec 200>"$LOCK_FILE"
flock -n 200 || exit 0

cd "$APP_DIR" || exit 1

# ─── Self-Update Script ───────────────────────────
# Download latest auto-deploy.sh from GitHub (like antibot's git pull)
SCRIPT_URL="https://raw.githubusercontent.com/${REPO}/main/auto-deploy.sh"
if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL -H "Authorization: token $GITHUB_TOKEN" "$SCRIPT_URL" -o "$APP_DIR/auto-deploy.sh.new" 2>/dev/null
else
    curl -fsSL "$SCRIPT_URL" -o "$APP_DIR/auto-deploy.sh.new" 2>/dev/null
fi
if [ -f "$APP_DIR/auto-deploy.sh.new" ] && [ -s "$APP_DIR/auto-deploy.sh.new" ]; then
    if ! cmp -s "$APP_DIR/auto-deploy.sh" "$APP_DIR/auto-deploy.sh.new"; then
        mv -f "$APP_DIR/auto-deploy.sh.new" "$APP_DIR/auto-deploy.sh"
        chmod 755 "$APP_DIR/auto-deploy.sh"
        echo "=== Script updated $(date '+%Y-%m-%d %H:%M:%S') ==="
    else
        rm -f "$APP_DIR/auto-deploy.sh.new"
    fi
else
    rm -f "$APP_DIR/auto-deploy.sh.new" 2>/dev/null
fi

# ─── GitHub API Helper ────────────────────────────
gh_api() {
    curl -fsSL ${GITHUB_TOKEN:+-H "Authorization: token $GITHUB_TOKEN"} \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${REPO}$1"
}

# Normalize versions: extract first word, strip "v" prefix
norm() { echo "$1" | awk '{print $1}' | sed 's/^v//' | tr -d '[:space:]'; }

# ─── Check for New Release ────────────────────────
LATEST_TAG=$(gh_api "/releases/latest" | jq -r '.tag_name // empty')
[ -n "$LATEST_TAG" ] || exit 0

INSTALLED=""
[ -x "$BINARY" ] && INSTALLED=$("$BINARY" -version 2>/dev/null)

if [ "$(norm "$INSTALLED")" = "$(norm "$LATEST_TAG")" ]; then
    exit 0   # Already current
fi

echo ""
echo "=== Botginx update $(date '+%Y-%m-%d %H:%M:%S') : ${INSTALLED:-none} -> ${LATEST_TAG} ==="

# ─── Download New Binary ──────────────────────────
rel=$(gh_api "/releases/tags/${LATEST_TAG}")

if [ -n "${GITHUB_TOKEN:-}" ]; then
    # Private repo: use API URL with auth
    url=$(echo "$rel" | jq -r ".assets[] | select(.name==\"${ASSET}\") | .url // empty")
    [ -n "$url" ] || { echo "  Asset ${ASSET} not found in ${LATEST_TAG}"; exit 1; }
    curl -fsSL -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/octet-stream" -o "$BINARY.new" "$url"

    sums_url=$(echo "$rel" | jq -r '.assets[] | select(.name=="SHA256SUMS") | .url // empty')
    [ -n "$sums_url" ] && curl -fsSL -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/octet-stream" -o /tmp/botginx.sums "$sums_url" || true
else
    # Public repo: use browser download URL
    url=$(echo "$rel" | jq -r ".assets[] | select(.name==\"${ASSET}\") | .browser_download_url // empty")
    [ -n "$url" ] || { echo "  Asset ${ASSET} not found in ${LATEST_TAG}"; exit 1; }
    curl -fsSL -o "$BINARY.new" "$url"

    sums_url=$(echo "$rel" | jq -r '.assets[] | select(.name=="SHA256SUMS") | .browser_download_url // empty')
    [ -n "$sums_url" ] && curl -fsSL -o /tmp/botginx.sums "$sums_url" || true
fi

# ─── Verify Checksum ──────────────────────────────
if [ -f /tmp/botginx.sums ]; then
    want=$(grep " ${ASSET}\$" /tmp/botginx.sums | awk '{print $1}')
    got=$(sha256sum "$BINARY.new" | awk '{print $1}')
    rm -f /tmp/botginx.sums

    if [ -n "$want" ] && [ "$want" != "$got" ]; then
        echo "  Checksum mismatch — aborting update"
        echo "  Expected: $want"
        echo "  Got:      $got"
        rm -f "$BINARY.new"
        exit 1
    fi
    echo "  Checksum verified"
fi

# ─── Verify Binary Executes ───────────────────────
chmod +x "$BINARY.new"
if ! "$BINARY.new" -version >/dev/null 2>&1; then
    echo "  New binary won't execute — aborting update"
    rm -f "$BINARY.new"
    exit 1
fi

# ─── Atomic Swap & Restart ────────────────────────
systemctl stop botginx
mv -f "$BINARY.new" "$BINARY"
chown botginx:botginx "$BINARY" 2>/dev/null || true
systemctl start botginx

sleep 2
if systemctl is-active --quiet botginx; then
    NEW_VER=$("$BINARY" -version 2>/dev/null)
    echo "  Updated OK -> ${NEW_VER}"
    echo "=== Update complete $(date '+%Y-%m-%d %H:%M:%S') ==="
else
    echo "  Service FAILED after update"
    journalctl -u botginx --no-pager -n 20 || true
    exit 1
fi

#!/bin/bash
#
# Botginx — Auto Update (GitHub Releases, runs via cron)
#
# Checks the latest release every few minutes. If its version differs from the
# installed binary, downloads the new asset, verifies its checksum, swaps it in
# atomically, and restarts the service. No building — binaries are prebuilt.
#
# Also pulls git for script/config updates (preserves .env and data/).
#
# Cron setup (every 2 minutes):
#   */2 * * * * DEPLOY_BRANCH="main" /var/www/botginx/auto-deploy.sh >> /var/www/botginx/logs/auto-deploy.log 2>&1
#

LOCK_FILE="/tmp/botginx-deploy.lock"

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$PROJECT_DIR/botginx"
REPO="${BOTGINX_REPO:-robertp2083/botginx}"
mkdir -p "$PROJECT_DIR/logs"

# Cron has no environment — load GITHUB_TOKEN from .env
# shellcheck disable=SC1091
[ -f "$PROJECT_DIR/.env" ] && . "$PROJECT_DIR/.env"

# Detect architecture
case "$(uname -m)" in
    x86_64|amd64) ASSET="botginx-linux-amd64" ;;
    aarch64|arm64) ASSET="botginx-linux-arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)"; exit 0 ;;
esac

# Single-instance lock (prevent overlapping runs)
exec 200>"$LOCK_FILE"
flock -n 200 || exit 0

cd "$PROJECT_DIR" || exit 1

# ─── Git Pull (scripts/templates update) ─────────
# IMPORTANT: Never overwrite .env or data/ — they are local state
if [ -d .git ]; then
    if [ -f /root/.ssh/github_deploy ]; then
        export GIT_SSH_COMMAND="ssh -i /root/.ssh/github_deploy -o StrictHostKeyChecking=no"
    fi

    # Backup local state before git operations
    [ -f .env ] && cp .env /tmp/botginx-env-backup
    [ -f "$BINARY" ] && cp "$BINARY" /tmp/botginx-binary-backup
    [ -d data ] && cp -r data /tmp/botginx-data-backup

    # Pull latest code
    BRANCH="${DEPLOY_BRANCH:-main}"
    git fetch origin "$BRANCH" --quiet 2>/dev/null && \
    git reset --hard "origin/$BRANCH" --quiet 2>/dev/null || true

    # Restore local state after git reset
    [ -f /tmp/botginx-env-backup ] && mv /tmp/botginx-env-backup .env
    [ -f /tmp/botginx-binary-backup ] && mv /tmp/botginx-binary-backup "$BINARY" && chmod +x "$BINARY"
    [ -d /tmp/botginx-data-backup ] && cp -r /tmp/botginx-data-backup/* data/ 2>/dev/null && rm -rf /tmp/botginx-data-backup

    # Ensure scripts are executable after git pull
    chmod 755 "$PROJECT_DIR"/*.sh 2>/dev/null || true
fi

# ─── GitHub API Helper ────────────────────────────
gh_api() {
    curl -fsSL ${GITHUB_TOKEN:+-H "Authorization: token $GITHUB_TOKEN"} \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${REPO}$1"
}

# Normalize versions: strip "v" prefix and "botginx " prefix
norm() { echo "$1" | sed -E 's/^botginx //; s/^v//' | tr -d '[:space:]'; }

# ─── Check for New Release ────────────────────────
LATEST_TAG=$(gh_api "/releases/latest" | jq -r '.tag_name // empty')
[ -n "$LATEST_TAG" ] || exit 0

INSTALLED=""
if [ -x "$BINARY" ]; then
    INSTALLED=$("$BINARY" -version 2>/dev/null || "$BINARY" version 2>/dev/null || echo "")
fi

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
if ! "$BINARY.new" -version >/dev/null 2>&1 && ! "$BINARY.new" version >/dev/null 2>&1; then
    echo "  New binary won't execute — aborting update"
    rm -f "$BINARY.new"
    exit 1
fi

# ─── Atomic Swap & Restart ────────────────────────
mv -f "$BINARY.new" "$BINARY"
systemctl restart botginx

sleep 2
if systemctl is-active --quiet botginx; then
    NEW_VER=$("$BINARY" -version 2>/dev/null || "$BINARY" version 2>/dev/null || echo "unknown")
    echo "  Updated OK -> ${NEW_VER}"
    echo "=== Update complete $(date '+%Y-%m-%d %H:%M:%S') ==="
else
    echo "  Service FAILED after update"
    journalctl -u botginx --no-pager -n 20 || true
    exit 1
fi

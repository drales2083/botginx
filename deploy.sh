#!/usr/bin/env bash
#
# Deploy botginx to a VPS.
#
# Builds the binary locally for linux/amd64, ships it over SSH, and swaps it
# behind systemd. Nothing is compiled on the server, so a broken build fails
# here rather than on the box serving traffic.
#
# Usage:
#   ./deploy.sh                 deploy the binary
#   ./deploy.sh --setup         first run: install systemd unit, nginx, TLS
#   ./deploy.sh --dry-run       show what would happen, change nothing
#   ./deploy.sh --rollback      restore the previous binary
#   ./deploy.sh --status        show service status and recent logs
#   ./deploy.sh --logs          follow the live log
#
# Configuration lives in deploy.env next to this script (see deploy.env.example).
# Nothing server-specific is hardcoded below.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# ----------------------------------------------------------------------------
# Configuration
# ----------------------------------------------------------------------------

CONFIG_FILE="${DEPLOY_CONFIG:-$SCRIPT_DIR/deploy.env}"

if [[ -f "$CONFIG_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$CONFIG_FILE"
fi

SSH_HOST="${SSH_HOST:-}"
SSH_USER="${SSH_USER:-root}"
SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-}"

APP_NAME="${APP_NAME:-botginx}"
APP_DIR="${APP_DIR:-/opt/botginx}"
CONFIG_DIR="${CONFIG_DIR:-/etc/botginx}"
SERVICE_NAME="${SERVICE_NAME:-botginx}"
RUN_USER="${RUN_USER:-botginx}"

APP_PORT="${APP_PORT:-3001}"
PANEL_DOMAIN="${PANEL_DOMAIN:-}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"

BUILD_OS="${BUILD_OS:-linux}"
BUILD_ARCH="${BUILD_ARCH:-amd64}"

HEALTH_PATH="${HEALTH_PATH:-/health}"
HEALTH_RETRIES="${HEALTH_RETRIES:-10}"
HEALTH_DELAY="${HEALTH_DELAY:-2}"

DRY_RUN=false

# ----------------------------------------------------------------------------
# Output
# ----------------------------------------------------------------------------

if [[ -t 1 ]]; then
    RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[0;33m'
    BLUE=$'\033[0;34m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; BLUE=''; DIM=''; RESET=''
fi

STEP_NUM=0
STEP_TOTAL=0

log()   { printf '%s[%s]%s %s\n' "$DIM" "$(date '+%H:%M:%S')" "$RESET" "$*"; }
info()  { printf '%s==>%s %s\n' "$BLUE" "$RESET" "$*"; }
ok()    { printf '%s  ok%s %s\n' "$GREEN" "$RESET" "$*"; }
warn()  { printf '%swarn%s %s\n' "$YELLOW" "$RESET" "$*" >&2; }
die()   { printf '%sfail%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }

step() {
    STEP_NUM=$((STEP_NUM + 1))
    printf '\n%s[%d/%d]%s %s\n' "$BLUE" "$STEP_NUM" "$STEP_TOTAL" "$RESET" "$*"
}

# ----------------------------------------------------------------------------
# SSH helpers
# ----------------------------------------------------------------------------

ssh_args() {
    local args=(-p "$SSH_PORT" -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new)
    [[ -n "$SSH_KEY" ]] && args+=(-i "$SSH_KEY")
    printf '%s\n' "${args[@]}"
}

remote() {
    local args=()
    mapfile -t args < <(ssh_args)

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would run on %s:%s %s\n' "$DIM" "$SSH_HOST" "$RESET" "$*"
        return 0
    fi
    ssh "${args[@]}" "${SSH_USER}@${SSH_HOST}" "$@"
}

# remote_sudo runs a command as root. When deploying as a non-root user it goes
# through sudo; as root it runs directly, since many minimal images ship without
# sudo installed at all.
remote_sudo() {
    if [[ "$SSH_USER" == "root" ]]; then
        remote "$@"
    else
        remote "sudo $*"
    fi
}

upload() {
    local src="$1" dest="$2"
    local args=()
    mapfile -t args < <(ssh_args)

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would upload %s -> %s:%s%s\n' "$DIM" "$src" "$SSH_HOST" "$dest" "$RESET"
        return 0
    fi
    scp -P "$SSH_PORT" ${SSH_KEY:+-i "$SSH_KEY"} \
        -o StrictHostKeyChecking=accept-new \
        "$src" "${SSH_USER}@${SSH_HOST}:${dest}"
}

# ----------------------------------------------------------------------------
# Preflight
# ----------------------------------------------------------------------------

preflight() {
    step "Preflight checks"

    [[ -n "$SSH_HOST" ]] || die "SSH_HOST is not set. Copy deploy.env.example to deploy.env and fill it in."
    command -v go  >/dev/null 2>&1 || die "go is not installed locally"
    command -v ssh >/dev/null 2>&1 || die "ssh is not installed locally"
    command -v scp >/dev/null 2>&1 || die "scp is not installed locally"

    [[ -f go.mod ]] || die "go.mod not found -- run this from the repo root"

    log "checking ssh to ${SSH_USER}@${SSH_HOST}:${SSH_PORT}"
    if [[ "$DRY_RUN" != true ]]; then
        local args=()
        mapfile -t args < <(ssh_args)
        ssh "${args[@]}" -o BatchMode=yes "${SSH_USER}@${SSH_HOST}" true 2>/dev/null \
            || die "cannot ssh to ${SSH_USER}@${SSH_HOST}:${SSH_PORT} -- check SSH_HOST, SSH_USER, SSH_PORT, SSH_KEY"
    fi

    ok "preflight passed"
}

# ----------------------------------------------------------------------------
# Build
# ----------------------------------------------------------------------------

build() {
    step "Building ${APP_NAME} for ${BUILD_OS}/${BUILD_ARCH}"

    local out="dist/${APP_NAME}"
    mkdir -p dist

    # Version comes from the nearest tag, so tagging v2.34.3 is what sets it.
    # Build is the commit count: monotonic, and maps back to an exact commit.
    local version build
    version="$(git describe --tags --abbrev=0 2>/dev/null || true)"
    build="$(git rev-list --count HEAD 2>/dev/null || true)"

    if [[ -n "$(git status --porcelain 2>/dev/null || true)" && -n "$build" ]]; then
        build="${build}-dirty"
    fi

    log "version ${version:-unset} build ${build:-unset}"

    local pkg="github.com/botginx/botginx/pkg/buildinfo"
    local ldflags="-s -w"
    [[ -n "$version" ]] && ldflags+=" -X ${pkg}.version=${version}"
    [[ -n "$build" ]]   && ldflags+=" -X ${pkg}.build=${build}"

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would build %s%s\n' "$DIM" "$out" "$RESET"
        return 0
    fi

    # CGO off so the binary is static and does not depend on the server's libc.
    CGO_ENABLED=0 GOOS="$BUILD_OS" GOARCH="$BUILD_ARCH" \
        go build -trimpath \
        -ldflags "$ldflags" \
        -o "$out" ./cmd/server \
        || die "build failed -- nothing was sent to the server"

    ok "built $(du -h "$out" | cut -f1) -> ${out}"
}

# ----------------------------------------------------------------------------
# Deploy
# ----------------------------------------------------------------------------

deploy_binary() {
    step "Uploading binary"

    upload "dist/${APP_NAME}" "/tmp/${APP_NAME}.new"
    remote_sudo "chmod +x /tmp/${APP_NAME}.new"

    ok "uploaded"

    step "Swapping binary and restarting ${SERVICE_NAME}"

    # Keep the outgoing binary so --rollback has something to restore.
    remote_sudo "mkdir -p ${APP_DIR}/bin"
    remote_sudo "if [ -f ${APP_DIR}/bin/${APP_NAME} ]; then cp ${APP_DIR}/bin/${APP_NAME} ${APP_DIR}/bin/${APP_NAME}.prev; fi"

    remote_sudo "systemctl stop ${SERVICE_NAME} || true"
    remote_sudo "mv /tmp/${APP_NAME}.new ${APP_DIR}/bin/${APP_NAME}"
    remote_sudo "chown ${RUN_USER}:${RUN_USER} ${APP_DIR}/bin/${APP_NAME}"
    remote_sudo "systemctl start ${SERVICE_NAME}"

    ok "service restarted"
}

deploy_config() {
    step "Syncing config"

    if [[ -f config.yaml ]]; then
        upload config.yaml "/tmp/config.yaml"
        remote_sudo "mkdir -p ${CONFIG_DIR}"
        remote_sudo "mv /tmp/config.yaml ${CONFIG_DIR}/config.yaml"
        ok "config.yaml synced to ${CONFIG_DIR}"
    else
        warn "no local config.yaml -- leaving the server's copy alone"
    fi

    # Secrets live only in the server's env file and are never shipped from here.
    remote_sudo "test -f ${CONFIG_DIR}/${APP_NAME}.env" \
        || warn "${CONFIG_DIR}/${APP_NAME}.env missing -- run --setup, then set DATABASE_URL there"
}

health_check() {
    step "Health check"

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would poll http://127.0.0.1:%s%s%s\n' "$DIM" "$APP_PORT" "$HEALTH_PATH" "$RESET"
        return 0
    fi

    local i
    for ((i = 1; i <= HEALTH_RETRIES; i++)); do
        if remote "curl -fsS -m 5 http://127.0.0.1:${APP_PORT}${HEALTH_PATH}" >/dev/null 2>&1; then
            ok "healthy after ${i} attempt(s)"
            return 0
        fi
        log "not ready yet (${i}/${HEALTH_RETRIES})"
        sleep "$HEALTH_DELAY"
    done

    warn "health check failed after ${HEALTH_RETRIES} attempts"
    printf '\n%srecent logs:%s\n' "$YELLOW" "$RESET"
    remote_sudo "journalctl -u ${SERVICE_NAME} -n 40 --no-pager" || true
    die "deploy failed health check -- run './deploy.sh --rollback' to restore the previous binary"
}

# ----------------------------------------------------------------------------
# First-time setup
# ----------------------------------------------------------------------------

setup() {
    STEP_TOTAL=6
    preflight

    step "Creating service user and directories"
    remote_sudo "id -u ${RUN_USER} >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin ${RUN_USER}"
    remote_sudo "mkdir -p ${APP_DIR}/bin ${CONFIG_DIR}"
    remote_sudo "chown -R ${RUN_USER}:${RUN_USER} ${APP_DIR}"
    ok "user ${RUN_USER} and ${APP_DIR} ready"

    step "Installing systemd unit"
    install_systemd_unit
    ok "unit installed and enabled"

    step "Creating environment file"
    # Written only if absent, so a redeploy never clobbers live secrets.
    remote_sudo "test -f ${CONFIG_DIR}/${APP_NAME}.env || cat > ${CONFIG_DIR}/${APP_NAME}.env <<'ENVEOF'
# botginx environment. Secrets belong here, not in config.yaml or git.
DATABASE_URL=postgres://botginx:CHANGE_ME@127.0.0.1:5432/botginx?sslmode=disable
ANTIBOT_WEBHOOK_SECRET=
DEFAULT_LANG=en
ENVEOF"
    remote_sudo "chown root:${RUN_USER} ${CONFIG_DIR}/${APP_NAME}.env"
    remote_sudo "chmod 640 ${CONFIG_DIR}/${APP_NAME}.env"
    ok "env file at ${CONFIG_DIR}/${APP_NAME}.env"

    if [[ -n "$PANEL_DOMAIN" ]]; then
        step "Configuring nginx for ${PANEL_DOMAIN}"
        install_nginx
        ok "nginx configured"

        step "Requesting TLS certificate"
        request_certificate
    else
        step "Skipping nginx"
        warn "PANEL_DOMAIN not set -- no reverse proxy configured"
        step "Skipping TLS"
    fi

    printf '\n%sSetup complete.%s\n\n' "$GREEN" "$RESET"
    printf 'Next:\n'
    printf '  1. Set DATABASE_URL in %s:%s/%s.env\n' "$SSH_HOST" "$CONFIG_DIR" "$APP_NAME"
    printf '  2. Create the database and role on the server\n'
    printf '  3. ./deploy.sh          to ship the first build\n\n'
}

install_systemd_unit() {
    local unit
    unit="$(cat <<UNITEOF
[Unit]
Description=${APP_NAME} -- VPS management and redirect links
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=${RUN_USER}
Group=${RUN_USER}
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/bin/${APP_NAME} -config ${CONFIG_DIR}/config.yaml
EnvironmentFile=${CONFIG_DIR}/${APP_NAME}.env

Restart=always
RestartSec=5

# The app needs nothing outside its own directory.
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${APP_DIR}

StandardOutput=journal
StandardError=journal
SyslogIdentifier=${APP_NAME}

[Install]
WantedBy=multi-user.target
UNITEOF
)"

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would install /etc/systemd/system/%s.service%s\n' "$DIM" "$SERVICE_NAME" "$RESET"
        return 0
    fi

    remote_sudo "cat > /etc/systemd/system/${SERVICE_NAME}.service <<'UNITEOF'
${unit}
UNITEOF"
    remote_sudo "systemctl daemon-reload"
    remote_sudo "systemctl enable ${SERVICE_NAME}"
}

install_nginx() {
    local conf
    conf="$(cat <<NGINXEOF
server {
    listen 80;
    listen [::]:80;
    server_name ${PANEL_DOMAIN};

    # certbot rewrites this block to add the 443 listener and redirect here.

    location / {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;

        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # The terminal and log views stream, so no buffering and a long timeout.
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_buffering off;
        proxy_read_timeout 3600s;
    }

    client_max_body_size 16m;
}
NGINXEOF
)"

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would install nginx site for %s%s\n' "$DIM" "$PANEL_DOMAIN" "$RESET"
        return 0
    fi

    remote_sudo "command -v nginx >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq nginx)"
    remote_sudo "cat > /etc/nginx/sites-available/${APP_NAME} <<'NGINXEOF'
${conf}
NGINXEOF"
    remote_sudo "ln -sf /etc/nginx/sites-available/${APP_NAME} /etc/nginx/sites-enabled/${APP_NAME}"
    remote_sudo "rm -f /etc/nginx/sites-enabled/default"
    remote_sudo "nginx -t" || die "nginx config test failed -- not reloading"
    remote_sudo "systemctl reload nginx"
}

request_certificate() {
    if [[ -z "$CERTBOT_EMAIL" ]]; then
        warn "CERTBOT_EMAIL not set -- skipping TLS."
        warn "run manually: certbot --nginx -d ${PANEL_DOMAIN}"
        return 0
    fi

    if [[ "$DRY_RUN" == true ]]; then
        printf '%s  would request a certificate for %s%s\n' "$DIM" "$PANEL_DOMAIN" "$RESET"
        return 0
    fi

    remote_sudo "command -v certbot >/dev/null 2>&1 || (apt-get update -qq && apt-get install -y -qq certbot python3-certbot-nginx)"

    # DNS has to resolve to this box before this can succeed.
    if remote_sudo "certbot --nginx -d ${PANEL_DOMAIN} --non-interactive --agree-tos -m ${CERTBOT_EMAIL} --redirect"; then
        ok "certificate issued for ${PANEL_DOMAIN}"
    else
        warn "certbot failed -- usually DNS for ${PANEL_DOMAIN} not pointing here yet"
        warn "fix DNS, then run: ssh ${SSH_USER}@${SSH_HOST} 'certbot --nginx -d ${PANEL_DOMAIN}'"
    fi
}

# ----------------------------------------------------------------------------
# Operations
# ----------------------------------------------------------------------------

rollback() {
    STEP_TOTAL=3
    preflight

    step "Checking for a previous binary"
    remote_sudo "test -f ${APP_DIR}/bin/${APP_NAME}.prev" \
        || die "no previous binary at ${APP_DIR}/bin/${APP_NAME}.prev -- nothing to roll back to"
    ok "found previous binary"

    step "Restoring"
    remote_sudo "systemctl stop ${SERVICE_NAME} || true"
    remote_sudo "mv ${APP_DIR}/bin/${APP_NAME}.prev ${APP_DIR}/bin/${APP_NAME}"
    remote_sudo "chown ${RUN_USER}:${RUN_USER} ${APP_DIR}/bin/${APP_NAME}"
    remote_sudo "systemctl start ${SERVICE_NAME}"
    ok "restored"

    health_check
    printf '\n%sRolled back.%s\n' "$GREEN" "$RESET"
}

status() {
    remote_sudo "systemctl status ${SERVICE_NAME} --no-pager" || true
    printf '\n%srecent logs:%s\n' "$BLUE" "$RESET"
    remote_sudo "journalctl -u ${SERVICE_NAME} -n 30 --no-pager" || true
}

follow_logs() {
    remote_sudo "journalctl -u ${SERVICE_NAME} -f"
}

deploy() {
    STEP_TOTAL=6
    local started
    started=$(date +%s)

    preflight
    build
    deploy_config
    deploy_binary
    health_check

    step "Done"
    ok "deployed in $(( $(date +%s) - started ))s"

    if [[ -n "$PANEL_DOMAIN" ]]; then
        printf '\n  https://%s\n\n' "$PANEL_DOMAIN"
    else
        printf '\n  http://%s:%s\n\n' "$SSH_HOST" "$APP_PORT"
    fi
}

usage() {
    sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

main() {
    local action=deploy

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --setup)    action=setup ;;
            --rollback) action=rollback ;;
            --status)   action=status ;;
            --logs)     action=logs ;;
            --dry-run)  DRY_RUN=true ;;
            -h|--help)  usage; exit 0 ;;
            *)          die "unknown option: $1 (try --help)" ;;
        esac
        shift
    done

    [[ "$DRY_RUN" == true ]] && warn "dry run -- nothing will be changed"

    case "$action" in
        deploy)   deploy ;;
        setup)    setup ;;
        rollback) rollback ;;
        status)   status ;;
        logs)     follow_logs ;;
    esac
}

main "$@"

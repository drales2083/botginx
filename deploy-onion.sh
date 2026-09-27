#!/bin/bash
#
# Botginx — Tor-Only Server Setup Script
#
# Deploys botginx (Go redirect link panel) to a VPS with:
# - Tor hidden service (.onion address)
# - PostgreSQL database
# - Nginx reverse proxy (HTTP only, Tor handles encryption)
# - Systemd service
# - Auto-deploy cron
#
# Traffic flow:
#   Tor → :80 (nginx) → :3001 (botginx)
#
# Usage:
#   DEPLOY_BRANCH="main" ssh root@SERVER 'bash -s' < deploy-onion.sh
#
# With antibot integration (botection on same VPS):
#   Traffic: Tor → :80 (tor) → :8080 (botection) → :3001 (botginx)
#   Set ANTIBOT_MODE=true to configure nginx for this flow
#

set -e

# DEPLOY_BRANCH is required
if [ -z "${DEPLOY_BRANCH:-}" ]; then
  echo ""
  echo "============================================"
  echo "  ERROR: DEPLOY_BRANCH is required"
  echo "============================================"
  echo ""
  echo "  Usage: DEPLOY_BRANCH=\"main\" bash deploy-onion.sh"
  echo ""
  exit 1
fi

APP_DIR="${APP_DIR:-/var/www/botginx}"
DB_NAME="botginx_db"
DB_USER="botginx"
DB_PASS="${DB_PASS:-$(openssl rand -hex 16)}"
SESSION_SECRET="${SESSION_SECRET:-$(openssl rand -hex 32)}"
ANTIBOT_WEBHOOK_SECRET="${ANTIBOT_WEBHOOK_SECRET:-$(openssl rand -hex 32)}"
REPO_SSH="git@github.com:drales2083/botginx.git"
REPO_SLUG="drales2083/botginx"
ANTIBOT_MODE="${ANTIBOT_MODE:-false}"

echo ""
echo "============================================"
echo "  Botginx Deployment — TOR ONLY MODE"
echo "============================================"
echo ""

# ─── 0. Install Tor & Generate .onion Address ─────────
echo "[0/12] Installing Tor and generating .onion address..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq tor

# Determine Tor user
TOR_USER="debian-tor"
if ! id "$TOR_USER" &>/dev/null; then
  if id "tor" &>/dev/null; then
    TOR_USER="tor"
  else
    TOR_USER="root"
  fi
fi

# Setup Tor hidden service directory
HS_DIR="/var/lib/tor/botginx"
mkdir -p "$HS_DIR"
chown -R "${TOR_USER}:${TOR_USER}" "$HS_DIR"
chmod 700 "$HS_DIR"

# Add hidden service to torrc
# Routes to port 8080 (antibot) if ANTIBOT_MODE, else directly to 3001 (botginx)
if [ "$ANTIBOT_MODE" = "true" ]; then
  TOR_TARGET_PORT="8080"
else
  TOR_TARGET_PORT="80"
fi

if ! grep -q "HiddenServiceDir ${HS_DIR}" /etc/tor/torrc 2>/dev/null; then
  cat >> /etc/tor/torrc <<EOF

# Botginx Hidden Service
HiddenServiceDir ${HS_DIR}/
HiddenServiceVersion 3
HiddenServicePort 80 127.0.0.1:${TOR_TARGET_PORT}
HiddenServiceNumIntroductionPoints 6
EOF
fi

systemctl enable tor
systemctl restart tor

# Wait for .onion address generation
echo "  Waiting for .onion address..."
RETRY=0
while [ ! -f "${HS_DIR}/hostname" ] && [ $RETRY -lt 15 ]; do
  sleep 1
  RETRY=$((RETRY + 1))
done

if [ ! -f "${HS_DIR}/hostname" ]; then
  echo "  ERROR: Failed to generate .onion address"
  exit 1
fi

ONION_ADDRESS=$(cat "${HS_DIR}/hostname" | tr -d '\r\n')
DOMAIN="${ONION_ADDRESS}"
SERVER_IP=$(curl -s ifconfig.me || hostname -I | awk '{print $1}')

echo "  .onion address: ${ONION_ADDRESS}"
echo "  Server IP: ${SERVER_IP}"

# ─── 1. System Update ──────────────────────────────
echo "[1/12] Updating system..."
apt-get upgrade -y -qq

# ─── 2. Install Dependencies ──────────────────────
echo "[2/12] Installing dependencies..."
apt-get install -y -qq curl wget git build-essential gnupg cron jq

# Add PostgreSQL repository
CODENAME=$(lsb_release -cs 2>/dev/null || echo "jammy")
sh -c "echo \"deb http://apt.postgresql.org/pub/repos/apt ${CODENAME}-pgdg main\" > /etc/apt/sources.list.d/pgdg.list"
wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
apt-get update -qq

apt-get install -y -qq postgresql-16 postgresql-contrib-16 postgresql-client-16
apt-get install -y -qq nginx

systemctl enable cron
systemctl start cron

# ─── 3. Install Go ────────────────────────────────
echo "[3/12] Installing Go..."
GO_VERSION="1.23.0"
if ! go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
fi

export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
echo "  Go: $(go version)"

# ─── 4. Setup Swap ────────────────────────────────
echo "[4/12] Setting up swap..."
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile || echo "  WARNING: swapon failed (LXC container?)"
  if swapon --show | grep -q "/swapfile"; then
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    echo "  2GB swap created"
  fi
else
  echo "  Swap already exists"
fi

# ─── 5. Setup PostgreSQL ──────────────────────────
echo "[5/12] Setting up PostgreSQL..."
systemctl enable postgresql
systemctl start postgresql

echo "  Waiting for PostgreSQL..."
for i in {1..10}; do
  if pg_isready -h localhost -p 5432 >/dev/null 2>&1; then
    echo "  PostgreSQL ready"
    break
  fi
  sleep 2
done

sudo -u postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1 || \
  sudo -u postgres psql -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASS}';"
sudo -u postgres psql -tc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1 || \
  sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};" 2>/dev/null || true

# ─── 6. Setup GitHub Deploy Key ───────────────────
echo "[6/12] Setting up GitHub deploy key & cloning app..."
mkdir -p /root/.ssh
chmod 700 /root/.ssh

if [ -n "${SSH_DEPLOY_KEY:-}" ]; then
  echo "  Using SSH deploy key from env var..."
  echo "$SSH_DEPLOY_KEY" | base64 -d > /root/.ssh/github_deploy
  chmod 600 /root/.ssh/github_deploy
elif [ -f /root/.ssh/github_deploy ]; then
  echo "  Reusing existing deploy key"
else
  echo "  Generating new deploy key..."
  ssh-keygen -t ed25519 -f /root/.ssh/github_deploy -N "" -C "botginx-deploy@onion" >/dev/null
  chmod 600 /root/.ssh/github_deploy
fi

if [ ! -f /root/.ssh/github_deploy.pub ]; then
  ssh-keygen -y -f /root/.ssh/github_deploy > /root/.ssh/github_deploy.pub
fi

grep -q 'github_deploy' /root/.ssh/config 2>/dev/null || cat >> /root/.ssh/config << 'SSHEOF'

Host github.com
  HostName github.com
  User git
  IdentityFile /root/.ssh/github_deploy
  IdentitiesOnly yes
SSHEOF
chmod 600 /root/.ssh/config

ssh-keyscan -H github.com >> /root/.ssh/known_hosts 2>/dev/null
sort -u /root/.ssh/known_hosts -o /root/.ssh/known_hosts

REPO_URL="$REPO_SSH"

github_ssh_ok() {
  git ls-remote "$REPO_SSH" HEAD >/dev/null 2>&1
}

if github_ssh_ok; then
  echo "  GitHub SSH auth OK"
else
  while true; do
    echo ""
    echo "  =========================================================="
    echo "  ACTION REQUIRED — deploy key not authorized on GitHub"
    echo "  =========================================================="
    echo "  Add this PUBLIC key at:"
    echo "    https://github.com/${REPO_SLUG}/settings/keys/new"
    echo ""
    echo "  --------- BEGIN PUBLIC KEY ---------"
    cat /root/.ssh/github_deploy.pub
    echo "  ---------  END PUBLIC KEY  ---------"
    echo ""

    if [ ! -r /dev/tty ]; then
      echo "  Non-interactive — add the key, then re-run this script"
      exit 1
    fi

    printf "  Added the key? Press [y] to continue: "
    read -r reply </dev/tty
    case "$reply" in
      y|Y)
        if github_ssh_ok; then
          echo "  GitHub SSH auth OK"
          break
        fi
        echo "  Still not authorized — try again"
        ;;
      *)
        echo "  Aborting"
        exit 1
        ;;
    esac
  done
fi

if [ -d "$APP_DIR/.git" ]; then
  cd $APP_DIR
  git remote set-url origin "$REPO_URL"
  git fetch origin "$DEPLOY_BRANCH"
  git checkout "$DEPLOY_BRANCH" 2>/dev/null || git checkout -b "$DEPLOY_BRANCH" "origin/$DEPLOY_BRANCH"
  git reset --hard "origin/$DEPLOY_BRANCH"
else
  rm -rf $APP_DIR
  git clone -b "$DEPLOY_BRANCH" $REPO_URL $APP_DIR
fi
cd $APP_DIR

# ─── 7. Create .env ───────────────────────────────
echo "[7/12] Creating environment file..."
mkdir -p "$APP_DIR/logs"

if [ -f "$APP_DIR/.env" ]; then
  echo "  .env exists — preserving secrets"
  # Extract existing secrets
  SESSION_SECRET=$(grep "^SESSION_SECRET=" "$APP_DIR/.env" | cut -d= -f2- || echo "$SESSION_SECRET")
  ANTIBOT_WEBHOOK_SECRET=$(grep "^ANTIBOT_WEBHOOK_SECRET=" "$APP_DIR/.env" | cut -d= -f2- || echo "$ANTIBOT_WEBHOOK_SECRET")
  DB_PASS=$(grep "^DB_PASS=" "$APP_DIR/.env" | cut -d= -f2- || echo "$DB_PASS")
fi

cat > $APP_DIR/.env << ENVEOF
# Botginx Configuration
# Generated by deploy-onion.sh

# Server
PORT=3001
HOST=127.0.0.1
BASE_URL=http://${DOMAIN}

# Database
DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@localhost:5432/${DB_NAME}?sslmode=disable
DB_PASS=${DB_PASS}

# Session
SESSION_SECRET=${SESSION_SECRET}

# Antibot Integration
ANTIBOT_WEBHOOK_SECRET=${ANTIBOT_WEBHOOK_SECRET}

# Tor Mode
TOR_MODE=true
ONION_ADDRESS=${ONION_ADDRESS}
SERVER_IP=${SERVER_IP}

# Admin (set after first deploy)
ADMIN_EMAIL=admin@localhost
ENVEOF
chmod 600 $APP_DIR/.env

# Save credentials
cat > /root/botginx-credentials.txt << CREDEOF
DB_USER=${DB_USER}
DB_PASS=${DB_PASS}
DB_NAME=${DB_NAME}
SESSION_SECRET=${SESSION_SECRET}
ANTIBOT_WEBHOOK_SECRET=${ANTIBOT_WEBHOOK_SECRET}
ONION_ADDRESS=${ONION_ADDRESS}
SERVER_IP=${SERVER_IP}
CREDEOF
chmod 600 /root/botginx-credentials.txt

# ─── 8. Build Application ─────────────────────────
echo "[8/12] Building botginx..."
cd $APP_DIR
export PATH=$PATH:/usr/local/go/bin

# Build the binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o botginx ./cmd/server

if [ ! -x "$APP_DIR/botginx" ]; then
  echo "  ERROR: Build failed"
  exit 1
fi

echo "  Built: $($APP_DIR/botginx -version 2>/dev/null || echo 'OK')"

# ─── 9. Setup Systemd Service ─────────────────────
echo "[9/12] Setting up systemd service..."
cat > /etc/systemd/system/botginx.service << SERVICEEOF
[Unit]
Description=Botginx Redirect Link Panel
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=${APP_DIR}
EnvironmentFile=${APP_DIR}/.env
ExecStart=${APP_DIR}/botginx
Restart=always
RestartSec=5
StandardOutput=append:${APP_DIR}/logs/botginx.log
StandardError=append:${APP_DIR}/logs/botginx.log

[Install]
WantedBy=multi-user.target
SERVICEEOF

systemctl daemon-reload
systemctl enable botginx
systemctl restart botginx

sleep 2
if systemctl is-active --quiet botginx; then
  echo "  Service running"
else
  echo "  WARNING: Service not running"
  journalctl -u botginx --no-pager -n 10
fi

# ─── 10. Setup Auto-Deploy Cron ───────────────────
echo "[10/12] Setting up auto-deploy cron..."
chmod +x "${APP_DIR}/auto-deploy.sh" 2>/dev/null || true

CRON_JOB="*/2 * * * * DEPLOY_BRANCH=\"${DEPLOY_BRANCH}\" ${APP_DIR}/auto-deploy.sh >> ${APP_DIR}/logs/auto-deploy.log 2>&1"
(crontab -l 2>/dev/null | grep -v "${APP_DIR}/auto-deploy.sh"; echo "$CRON_JOB") | crontab -
echo "  Auto-deploy cron: every 2 minutes"

# ─── 11. Setup Nginx ──────────────────────────────
echo "[11/12] Configuring Nginx..."

# Ensure hash bucket size for .onion addresses
if ! grep -q "server_names_hash_bucket_size" /etc/nginx/nginx.conf; then
  sed -i '/http {/a \    server_names_hash_bucket_size 128;' /etc/nginx/nginx.conf
fi

if [ "$ANTIBOT_MODE" = "true" ]; then
  # Antibot mode: nginx listens on 80, but Tor routes to 8080 (botection)
  # Botection then proxies to nginx on 80, which proxies to botginx on 3001
  # This config is for direct access (not via Tor)
  echo "  Antibot mode: Tor → :8080 (botection) → :3001 (botginx)"

  cat > /etc/nginx/sites-available/botginx << NGINXEOF
server {
    listen 80;
    server_name ${DOMAIN} localhost;

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
        proxy_cache_bypass \$http_upgrade;
        client_max_body_size 10M;
    }
}
NGINXEOF
else
  # Direct mode: Tor → nginx :80 → botginx :3001
  cat > /etc/nginx/sites-available/botginx << NGINXEOF
server {
    listen 80;
    server_name ${DOMAIN} localhost;

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
        proxy_cache_bypass \$http_upgrade;
        client_max_body_size 10M;
    }
}
NGINXEOF
fi

ln -sf /etc/nginx/sites-available/botginx /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl restart nginx

# ─── 12. Firewall ─────────────────────────────────
echo "[12/12] Configuring firewall..."
ufw allow 22/tcp
ufw allow 80/tcp
ufw --force enable

# ─── Done ─────────────────────────────────────────
echo ""
echo "============================================"
echo "  BOTGINX DEPLOYMENT COMPLETE!"
echo "============================================"
echo ""
echo "  .onion URL:  http://${ONION_ADDRESS}"
echo "  Server IP:   ${SERVER_IP}"
echo ""
echo "  Access via Tor Browser: http://${ONION_ADDRESS}"
echo ""
echo "  Commands:"
echo "    systemctl status botginx   — check service"
echo "    journalctl -u botginx -f   — live logs"
echo "    systemctl restart botginx  — restart"
echo ""
echo "  Files:"
echo "    App:         ${APP_DIR}"
echo "    Binary:      ${APP_DIR}/botginx"
echo "    Env:         ${APP_DIR}/.env"
echo "    Logs:        ${APP_DIR}/logs/"
echo "    Credentials: /root/botginx-credentials.txt"
echo "    Tor keys:    ${HS_DIR}/ (BACKUP THIS!)"
echo ""
if [ "$ANTIBOT_MODE" = "true" ]; then
  echo "  Traffic flow (antibot mode):"
  echo "    Tor → :8080 (botection) → :3001 (botginx)"
  echo ""
  echo "  NEXT: Install botection on this server"
else
  echo "  Traffic flow:"
  echo "    Tor → :80 (nginx) → :3001 (botginx)"
fi
echo ""
echo "============================================"

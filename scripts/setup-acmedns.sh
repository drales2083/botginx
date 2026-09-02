#!/bin/bash
# Setup acme-dns server on Deploy VPS for redirect link SSL generation
# Run this on the Deploy VPS (from servers table, NOT hosting_servers)

set -e

ACME_DNS_VERSION="1.0"
ACME_DOMAIN="${ACME_DOMAIN:-acme.guardbot.sbs}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/acme-dns"
DATA_DIR="/var/lib/acme-dns"

echo "=== Setting up acme-dns server ==="
echo "Domain: $ACME_DOMAIN"

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

# Create directories
mkdir -p "$CONFIG_DIR" "$DATA_DIR"

# Download acme-dns if not exists
if [ ! -f "$INSTALL_DIR/acme-dns" ]; then
    echo "Downloading acme-dns v${ACME_DNS_VERSION}..."
    cd /tmp
    wget -q "https://github.com/joohoi/acme-dns/releases/download/v${ACME_DNS_VERSION}/acme-dns_${ACME_DNS_VERSION}_linux_amd64.tar.gz"
    tar xzf "acme-dns_${ACME_DNS_VERSION}_linux_amd64.tar.gz"
    mv acme-dns "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/acme-dns"
    rm -f "acme-dns_${ACME_DNS_VERSION}_linux_amd64.tar.gz"
    echo "acme-dns installed to $INSTALL_DIR/acme-dns"
else
    echo "acme-dns already installed"
fi

# Create config file
cat > "$CONFIG_DIR/config.cfg" << EOF
[general]
listen = "0.0.0.0:53"
protocol = "both"
domain = "$ACME_DOMAIN"
nsname = "$ACME_DOMAIN"
nsadmin = "admin.guardbot.sbs"
debug = false

[database]
engine = "sqlite3"
connection = "$DATA_DIR/acme-dns.db"

[api]
ip = "127.0.0.1"
port = "8053"
tls = "none"
disable_registration = false

[logconfig]
loglevel = "info"
logtype = "stdout"
logformat = "text"
EOF

echo "Config written to $CONFIG_DIR/config.cfg"

# Create systemd service
cat > /etc/systemd/system/acme-dns.service << EOF
[Unit]
Description=acme-dns server for ACME DNS-01 challenges
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/acme-dns -c $CONFIG_DIR/config.cfg
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

echo "Systemd service created"

# Open firewall ports
if command -v ufw &> /dev/null; then
    ufw allow 53/udp
    ufw allow 53/tcp
    echo "Firewall ports opened (ufw)"
elif command -v firewall-cmd &> /dev/null; then
    firewall-cmd --permanent --add-port=53/udp
    firewall-cmd --permanent --add-port=53/tcp
    firewall-cmd --reload
    echo "Firewall ports opened (firewalld)"
fi

# Stop any existing DNS service on port 53
systemctl stop systemd-resolved 2>/dev/null || true
systemctl disable systemd-resolved 2>/dev/null || true

# Enable and start acme-dns
systemctl daemon-reload
systemctl enable acme-dns
systemctl restart acme-dns

sleep 2

# Check status
if systemctl is-active --quiet acme-dns; then
    echo ""
    echo "=== acme-dns is running ==="
    echo ""
    echo "API endpoint: http://127.0.0.1:8053"
    echo ""
    echo "Test registration:"
    echo "  curl -X POST http://127.0.0.1:8053/register"
    echo ""
    echo "IMPORTANT: Add these DNS records to guardbot.sbs:"
    echo "  $ACME_DOMAIN    A     $(hostname -I | awk '{print $1}')"
    echo "  $ACME_DOMAIN    NS    $ACME_DOMAIN"
    echo ""
else
    echo "ERROR: acme-dns failed to start"
    systemctl status acme-dns
    exit 1
fi

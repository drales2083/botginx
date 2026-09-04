package acmedns

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/botginx/botginx/pkg/sshexec"
)

// Registration contains the acme-dns registration response
type Registration struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Subdomain  string `json:"subdomain"`
	Fulldomain string `json:"fulldomain"`
}

// Client manages acme-dns operations via SSH to Deploy VPS
type Client struct {
	sshClient *sshexec.Client
	apiURL    string
}

// NewClient creates a new acme-dns client connected via SSH
func NewClient(ip string, port int, user, password string) (*Client, error) {
	portStr := fmt.Sprintf("%d", port)
	if port == 0 {
		portStr = "22"
	}

	client, err := sshexec.NewClient(ip, portStr, user, password)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	return &Client{
		sshClient: client,
		apiURL:    "http://127.0.0.1:8053",
	}, nil
}

// Close closes the SSH connection
func (c *Client) Close() {
	if c.sshClient != nil {
		c.sshClient.Close()
	}
}

// IsInstalled checks if acme-dns is installed and running on the server
func (c *Client) IsInstalled() bool {
	output, err := c.sshClient.Run("systemctl is-active acme-dns 2>/dev/null || echo inactive")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "active"
}

// Register creates a new subdomain on acme-dns for a domain
func (c *Client) Register() (*Registration, error) {
	cmd := fmt.Sprintf(`curl -s -X POST %s/register`, c.apiURL)

	output, err := c.sshClient.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("empty response from acme-dns")
	}

	var reg Registration
	if err := json.Unmarshal([]byte(output), &reg); err != nil {
		return nil, fmt.Errorf("failed to parse registration: %w (response: %s)", err, output)
	}

	if reg.Subdomain == "" {
		return nil, fmt.Errorf("invalid registration response: %s", output)
	}

	return &reg, nil
}

// UpdateTXT sets the TXT record value for a subdomain
func (c *Client) UpdateTXT(subdomain, password, txtValue string) error {
	payload := fmt.Sprintf(`{"subdomain":"%s","txt":"%s"}`, subdomain, txtValue)

	cmd := fmt.Sprintf(`curl -s -X POST %s/update \
		-H "X-Api-User: %s" \
		-H "X-Api-Key: %s" \
		-H "Content-Type: application/json" \
		-d '%s'`, c.apiURL, subdomain, password, payload)

	output, err := c.sshClient.Run(cmd)
	if err != nil {
		return fmt.Errorf("TXT update failed: %w", err)
	}

	output = strings.TrimSpace(output)

	// Check for error response
	if strings.Contains(output, "error") {
		return fmt.Errorf("acme-dns error: %s", output)
	}

	return nil
}

// GetTXT retrieves the current TXT value for a subdomain (for debugging)
func (c *Client) GetTXT(fulldomain string) (string, error) {
	cmd := fmt.Sprintf(`dig @127.0.0.1 %s TXT +short 2>/dev/null | tr -d '"'`, fulldomain)

	output, err := c.sshClient.Run(cmd)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

// Setup installs acme-dns on the server if not already installed
func (c *Client) Setup(acmeDomain string) error {
	// Check if already installed
	if c.IsInstalled() {
		return nil
	}

	// Download and run setup script
	setupScript := fmt.Sprintf(`
ACME_DOMAIN="%s"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/acme-dns"
DATA_DIR="/var/lib/acme-dns"

mkdir -p "$CONFIG_DIR" "$DATA_DIR"

# Download acme-dns
if [ ! -f "$INSTALL_DIR/acme-dns" ]; then
    cd /tmp
    wget -q "https://github.com/joohoi/acme-dns/releases/download/v1.0/acme-dns_1.0_linux_amd64.tar.gz"
    tar xzf acme-dns_1.0_linux_amd64.tar.gz
    mv acme-dns "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/acme-dns"
    rm -f acme-dns_1.0_linux_amd64.tar.gz
fi

# Create config
cat > "$CONFIG_DIR/config.cfg" << CFGEOF
[general]
listen = "0.0.0.0:53"
protocol = "both"
domain = "$ACME_DOMAIN"
nsname = "$ACME_DOMAIN"
nsadmin = "admin.$ACME_DOMAIN"
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
CFGEOF

# Create systemd service
cat > /etc/systemd/system/acme-dns.service << SVCEOF
[Unit]
Description=acme-dns server
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/acme-dns -c $CONFIG_DIR/config.cfg
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SVCEOF

# Open firewall
ufw allow 53/udp 2>/dev/null || true
ufw allow 53/tcp 2>/dev/null || true

# Stop systemd-resolved if using port 53
systemctl stop systemd-resolved 2>/dev/null || true
systemctl disable systemd-resolved 2>/dev/null || true

# Start service
systemctl daemon-reload
systemctl enable acme-dns
systemctl restart acme-dns

sleep 2
systemctl is-active acme-dns
`, acmeDomain)

	output, err := c.sshClient.Run(setupScript)
	if err != nil {
		return fmt.Errorf("setup failed: %w (output: %s)", err, output)
	}

	if !strings.Contains(output, "active") {
		return fmt.Errorf("acme-dns failed to start after setup")
	}

	return nil
}

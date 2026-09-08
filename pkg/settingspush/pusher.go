package settingspush

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/botginx/botginx/pkg/ssh"
)

// LinkSettings matches what botection expects
type LinkSettings struct {
	LinkID           string   `json:"link_id"`
	UserID           string   `json:"user_id"`
	Host             string   `json:"host"`
	Template         string   `json:"template,omitempty"` // Challenge template: cloudflare, humancheck, humansecurity
	BlockBots        bool     `json:"block_bots"`
	BlockTor         bool     `json:"block_tor"`
	BlockProxy       bool     `json:"block_proxy"`
	BlockDatacenter  bool     `json:"block_datacenter"`
	BlockHeadless    bool     `json:"block_headless"`
	CountryMode      string   `json:"country_mode"`
	CountryList      []string `json:"country_list"`
	ASNMode          string   `json:"asn_mode"`
	ASNList          []string `json:"asn_list"`
	DeviceMode       string   `json:"device_mode"`
	DeviceList       []string `json:"device_list"`
	MinBehaviorScore int      `json:"min_behavior_score"`
	RedirectOnBlock  string   `json:"redirect_on_block"`
	UpdatedAt        string   `json:"updated_at"`
}

// ServerInfo contains SSH connection details
type ServerInfo struct {
	IP       string
	Port     int
	User     string
	Password string
}

// Pusher pushes settings files to deploy VPS
type Pusher struct{}

// New creates a new settings pusher
func New() *Pusher {
	return &Pusher{}
}

// Push sends a settings file to the VPS
func (p *Pusher) Push(server ServerInfo, linkID string, settings LinkSettings) error {
	// Set updated timestamp
	settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Generate JSON
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Create SSH client
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.User,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	// Connect
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", server.IP, err)
	}

	// Remote path
	remotePath := fmt.Sprintf("/etc/botection/links/%s.json", linkID)

	// Ensure directory exists
	mkdirCmd := "mkdir -p /etc/botection/links"
	if _, err := client.Exec(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := client.WriteFile(remotePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// Delete removes a settings file from the VPS
func (p *Pusher) Delete(server ServerInfo, linkID string) error {
	// Create SSH client
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.User,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	// Connect
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", server.IP, err)
	}

	// Delete file
	remotePath := fmt.Sprintf("/etc/botection/links/%s.json", linkID)
	command := fmt.Sprintf("rm -f %s", remotePath)

	if _, err := client.Exec(command); err != nil {
		return fmt.Errorf("failed to delete settings file: %w", err)
	}

	return nil
}

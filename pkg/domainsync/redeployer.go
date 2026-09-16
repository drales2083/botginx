package domainsync

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/botginx/botginx/pkg/ssh"
	"github.com/jmoiron/sqlx"
)

// LinkSettings matches what botection expects
type LinkSettings struct {
	LinkID           string   `json:"link_id"`
	UserID           string   `json:"user_id"`
	Host             string   `json:"host"`
	Template         string   `json:"template"`
	PuzzleMode       string   `json:"puzzle_mode"`
	ThemeMode        string   `json:"theme_mode"`
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

// RedirectLinkInfo contains basic redirect link info for redeployment
type RedirectLinkInfo struct {
	ID         string `db:"id"`
	UserID     string `db:"user_id"`
	Subdomain  string `db:"subdomain"`
	DomainName string `db:"domain_name"`
}

// LinkSettingsRow represents protection settings from link_settings table
type LinkSettingsRow struct {
	Template         *string `db:"template"`
	ThemeMode        *string `db:"theme_mode"`
	BlockBots        bool    `db:"block_bots"`
	BlockTor         bool    `db:"block_tor"`
	BlockProxy       bool    `db:"block_proxy"`
	BlockDatacenter  bool    `db:"block_datacenter"`
	BlockHeadless    bool    `db:"block_headless"`
	CountryMode      *string `db:"country_mode"`
	CountryList      *string `db:"country_list"`
	ASNMode          *string `db:"asn_mode"`
	ASNList          *string `db:"asn_list"`
	DeviceMode       *string `db:"device_mode"`
	DeviceList       *string `db:"device_list"`
	MinBehaviorScore int     `db:"min_behavior_score"`
	RedirectOnBlock  *string `db:"redirect_on_block"`
}

// ShortLinkInfo contains basic short link info for redeployment
type ShortLinkInfo struct {
	ID         string `db:"id"`
	UserID     string `db:"user_id"`
	Path       string `db:"path"`
	DomainName string `db:"domain_name"`
}

// DefaultRedeployer implements LinkRedeployer using the database
type DefaultRedeployer struct {
	db *sqlx.DB
}

// NewDefaultRedeployer creates a new redeployer
func NewDefaultRedeployer(db *sqlx.DB) *DefaultRedeployer {
	return &DefaultRedeployer{db: db}
}

// RedeployRedirectLink deploys a redirect link to a server
func (r *DefaultRedeployer) RedeployRedirectLink(linkID string, server Server) error {
	// Fetch link info with domain name
	var link RedirectLinkInfo
	err := r.db.Get(&link, `
		SELECT r.id, r.user_id, r.subdomain, d.name as domain_name
		FROM redirect_links r
		JOIN domains d ON d.id = r.domain_id
		WHERE r.id = $1
	`, linkID)
	if err != nil {
		return fmt.Errorf("failed to fetch link: %w", err)
	}

	// Compute host from subdomain + domain
	baseDomain := link.DomainName
	if strings.HasPrefix(baseDomain, "*.") {
		baseDomain = baseDomain[2:]
	}
	host := link.Subdomain + "." + baseDomain

	// Fetch protection settings
	var settingsRow LinkSettingsRow
	err = r.db.Get(&settingsRow, `
		SELECT template, theme_mode, block_bots, block_tor, block_proxy,
		       block_datacenter, block_headless, country_mode, country_list,
		       asn_mode, asn_list, device_mode, device_list,
		       min_behavior_score, redirect_on_block
		FROM link_settings WHERE link_id = $1
	`, linkID)
	// If no settings exist, use defaults
	if err != nil {
		settingsRow = LinkSettingsRow{
			BlockBots: true,
		}
	}

	// Build settings
	settings := LinkSettings{
		LinkID:           link.ID,
		UserID:           link.UserID,
		Host:             host,
		BlockBots:        settingsRow.BlockBots,
		BlockTor:         settingsRow.BlockTor,
		BlockProxy:       settingsRow.BlockProxy,
		BlockDatacenter:  settingsRow.BlockDatacenter,
		BlockHeadless:    settingsRow.BlockHeadless,
		MinBehaviorScore: settingsRow.MinBehaviorScore,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	if settingsRow.Template != nil {
		settings.Template = *settingsRow.Template
	} else {
		settings.Template = "cloudflare"
	}
	if settingsRow.ThemeMode != nil {
		settings.ThemeMode = *settingsRow.ThemeMode
	}
	if settingsRow.CountryMode != nil {
		settings.CountryMode = *settingsRow.CountryMode
	}
	if settingsRow.CountryList != nil {
		json.Unmarshal([]byte(*settingsRow.CountryList), &settings.CountryList)
	}
	if settingsRow.ASNMode != nil {
		settings.ASNMode = *settingsRow.ASNMode
	}
	if settingsRow.ASNList != nil {
		json.Unmarshal([]byte(*settingsRow.ASNList), &settings.ASNList)
	}
	if settingsRow.DeviceMode != nil {
		settings.DeviceMode = *settingsRow.DeviceMode
	}
	if settingsRow.DeviceList != nil {
		json.Unmarshal([]byte(*settingsRow.DeviceList), &settings.DeviceList)
	}
	if settingsRow.RedirectOnBlock != nil {
		settings.RedirectOnBlock = *settingsRow.RedirectOnBlock
	}

	// Push to server
	return r.pushSettings(server, linkID, settings)
}

// RedeployShortLink deploys a short link to a server
func (r *DefaultRedeployer) RedeployShortLink(linkID string, server Server) error {
	// Fetch link info with domain name
	var link ShortLinkInfo
	err := r.db.Get(&link, `
		SELECT s.id, s.user_id, s.path, d.name as domain_name
		FROM short_links s
		JOIN domains d ON d.id = s.domain_id
		WHERE s.id = $1
	`, linkID)
	if err != nil {
		return fmt.Errorf("failed to fetch link: %w", err)
	}

	// Compute host from domain + path
	baseDomain := link.DomainName
	if strings.HasPrefix(baseDomain, "*.") {
		baseDomain = baseDomain[2:]
	}
	host := baseDomain + "/" + link.Path

	// Fetch protection settings (short links use same link_settings table)
	var settingsRow LinkSettingsRow
	err = r.db.Get(&settingsRow, `
		SELECT template, theme_mode, block_bots, block_tor, block_proxy,
		       block_datacenter, block_headless, country_mode, country_list,
		       asn_mode, asn_list, device_mode, device_list,
		       min_behavior_score, redirect_on_block
		FROM link_settings WHERE link_id = $1
	`, linkID)
	if err != nil {
		settingsRow = LinkSettingsRow{
			BlockBots: true,
		}
	}

	// Build settings
	settings := LinkSettings{
		LinkID:           link.ID,
		UserID:           link.UserID,
		Host:             host,
		BlockBots:        settingsRow.BlockBots,
		BlockTor:         settingsRow.BlockTor,
		BlockProxy:       settingsRow.BlockProxy,
		BlockDatacenter:  settingsRow.BlockDatacenter,
		BlockHeadless:    settingsRow.BlockHeadless,
		MinBehaviorScore: settingsRow.MinBehaviorScore,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	if settingsRow.Template != nil {
		settings.Template = *settingsRow.Template
	} else {
		settings.Template = "cloudflare"
	}
	if settingsRow.ThemeMode != nil {
		settings.ThemeMode = *settingsRow.ThemeMode
	}
	if settingsRow.CountryMode != nil {
		settings.CountryMode = *settingsRow.CountryMode
	}
	if settingsRow.CountryList != nil {
		json.Unmarshal([]byte(*settingsRow.CountryList), &settings.CountryList)
	}
	if settingsRow.RedirectOnBlock != nil {
		settings.RedirectOnBlock = *settingsRow.RedirectOnBlock
	}

	// Push to server
	return r.pushSettings(server, linkID, settings)
}

// pushSettings pushes settings to the server via SSH
func (r *DefaultRedeployer) pushSettings(server Server, linkID string, settings LinkSettings) error {
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SSH connect failed: %w", err)
	}

	// Create directory
	if _, err := client.Exec("mkdir -p /etc/botection/links"); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write file
	remotePath := fmt.Sprintf("/etc/botection/links/%s.json", linkID)
	if err := client.WriteFile(remotePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

package domainsync

import (
	"encoding/json"
	"fmt"
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

// RedirectLinkFull contains full redirect link data for redeployment
type RedirectLinkFull struct {
	ID               string  `db:"id"`
	UserID           string  `db:"user_id"`
	Host             string  `db:"host"`
	Template         string  `db:"template"`
	PuzzleMode       *string `db:"puzzle_mode"`
	ThemeMode        *string `db:"theme_mode"`
	BlockBots        bool    `db:"block_bots"`
	BlockTor         bool    `db:"block_tor"`
	BlockProxy       bool    `db:"block_proxy"`
	BlockDatacenter  bool    `db:"block_datacenter"`
	BlockHeadless    bool    `db:"block_headless"`
	CountryMode      string  `db:"country_mode"`
	CountryList      string  `db:"country_list"` // JSON array
	ASNMode          *string `db:"asn_mode"`
	ASNList          *string `db:"asn_list"` // JSON array
	DeviceMode       *string `db:"device_mode"`
	DeviceList       *string `db:"device_list"` // JSON array
	MinBehaviorScore int     `db:"min_behavior_score"`
	RedirectOnBlock  *string `db:"redirect_on_block"`
}

// ShortLinkFull contains full short link data for redeployment
type ShortLinkFull struct {
	ID               string  `db:"id"`
	UserID           string  `db:"user_id"`
	Host             string  `db:"host"`
	Path             string  `db:"path"`
	Template         *string `db:"template"`
	PuzzleMode       *string `db:"puzzle_mode"`
	ThemeMode        *string `db:"theme_mode"`
	BlockBots        bool    `db:"block_bots"`
	BlockTor         bool    `db:"block_tor"`
	BlockProxy       bool    `db:"block_proxy"`
	BlockDatacenter  bool    `db:"block_datacenter"`
	BlockHeadless    bool    `db:"block_headless"`
	CountryMode      *string `db:"country_mode"`
	CountryList      *string `db:"country_list"` // JSON array
	MinBehaviorScore *int    `db:"min_behavior_score"`
	RedirectOnBlock  *string `db:"redirect_on_block"`
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
	// Fetch full link data
	var link RedirectLinkFull
	err := r.db.Get(&link, `
		SELECT id, user_id, host, template, puzzle_mode, theme_mode,
		       block_bots, block_tor, block_proxy, block_datacenter, block_headless,
		       country_mode, COALESCE(country_list, '[]') as country_list,
		       asn_mode, asn_list, device_mode, device_list,
		       min_behavior_score, redirect_on_block
		FROM redirect_links WHERE id = $1
	`, linkID)
	if err != nil {
		return fmt.Errorf("failed to fetch link: %w", err)
	}

	// Build settings
	settings := LinkSettings{
		LinkID:           link.ID,
		UserID:           link.UserID,
		Host:             link.Host,
		Template:         link.Template,
		BlockBots:        link.BlockBots,
		BlockTor:         link.BlockTor,
		BlockProxy:       link.BlockProxy,
		BlockDatacenter:  link.BlockDatacenter,
		BlockHeadless:    link.BlockHeadless,
		CountryMode:      link.CountryMode,
		MinBehaviorScore: link.MinBehaviorScore,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	if link.PuzzleMode != nil {
		settings.PuzzleMode = *link.PuzzleMode
	}
	if link.ThemeMode != nil {
		settings.ThemeMode = *link.ThemeMode
	}
	if link.RedirectOnBlock != nil {
		settings.RedirectOnBlock = *link.RedirectOnBlock
	}

	// Parse JSON arrays
	json.Unmarshal([]byte(link.CountryList), &settings.CountryList)
	if link.ASNMode != nil {
		settings.ASNMode = *link.ASNMode
	}
	if link.ASNList != nil {
		json.Unmarshal([]byte(*link.ASNList), &settings.ASNList)
	}
	if link.DeviceMode != nil {
		settings.DeviceMode = *link.DeviceMode
	}
	if link.DeviceList != nil {
		json.Unmarshal([]byte(*link.DeviceList), &settings.DeviceList)
	}

	// Push to server
	return r.pushSettings(server, linkID, settings)
}

// RedeployShortLink deploys a short link to a server
func (r *DefaultRedeployer) RedeployShortLink(linkID string, server Server) error {
	// Fetch full link data
	var link ShortLinkFull
	err := r.db.Get(&link, `
		SELECT id, user_id, host, path, template, puzzle_mode, theme_mode,
		       block_bots, block_tor, block_proxy, block_datacenter, block_headless,
		       country_mode, country_list, min_behavior_score, redirect_on_block
		FROM short_links WHERE id = $1
	`, linkID)
	if err != nil {
		return fmt.Errorf("failed to fetch link: %w", err)
	}

	// Build settings
	settings := LinkSettings{
		LinkID:        link.ID,
		UserID:        link.UserID,
		Host:          link.Host + link.Path, // Short links include path
		BlockBots:     link.BlockBots,
		BlockTor:      link.BlockTor,
		BlockProxy:    link.BlockProxy,
		BlockDatacenter: link.BlockDatacenter,
		BlockHeadless: link.BlockHeadless,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if link.Template != nil {
		settings.Template = *link.Template
	} else {
		settings.Template = "cloudflare"
	}
	if link.PuzzleMode != nil {
		settings.PuzzleMode = *link.PuzzleMode
	}
	if link.ThemeMode != nil {
		settings.ThemeMode = *link.ThemeMode
	}
	if link.CountryMode != nil {
		settings.CountryMode = *link.CountryMode
	}
	if link.CountryList != nil {
		json.Unmarshal([]byte(*link.CountryList), &settings.CountryList)
	}
	if link.MinBehaviorScore != nil {
		settings.MinBehaviorScore = *link.MinBehaviorScore
	}
	if link.RedirectOnBlock != nil {
		settings.RedirectOnBlock = *link.RedirectOnBlock
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

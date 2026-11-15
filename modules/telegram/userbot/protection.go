package userbot

import (
	"fmt"
	"log"
	"strings"

	"github.com/botginx/botginx/pkg/protection"
	"github.com/jmoiron/sqlx"
)

// Protection callback data (after the "prot:" prefix):
//
//	tpl:<name>     set template
//	block:<key>    toggle block setting
const (
	protTemplate = "tpl:"
	protBlock    = "block:"
)

var templateOptions = []string{"cloudflare", "humancheck", "humansecurity", "slidepuzzle", "smartpuzzle", "simple", "lobby"}

var templateLabels = map[string]string{
	"cloudflare":    "Cloudflare",
	"humancheck":    "Human Check",
	"humansecurity": "Human Security",
	"slidepuzzle":   "Slide Puzzle",
	"smartpuzzle":   "Smart Puzzle",
	"simple":        "Simple",
	"lobby":         "Lobby",
}

var blockColumns = map[string]string{
	"bots":       "BlockBots",
	"headless":   "BlockHeadless",
	"tor":        "BlockTor",
	"proxy":      "BlockProxy",
	"datacenter": "BlockDatacenter",
}

var blockLabels = map[string]string{
	"bots":       "Known Bots",
	"headless":   "Headless Browsers",
	"tor":        "Tor Exit Nodes",
	"proxy":      "Proxy/VPN",
	"datacenter": "Datacenter IPs",
}

// ProtectionHandler renders and mutates a user's default antibot settings.
type ProtectionHandler struct {
	db *sqlx.DB
}

func NewProtectionHandler(db *sqlx.DB) *ProtectionHandler {
	return &ProtectionHandler{db: db}
}

func (p *ProtectionHandler) getSettings(userID string) *protection.Settings {
	var settings protection.Settings
	err := p.db.Get(&settings, `SELECT settings FROM user_antibot_defaults WHERE user_id = $1`, userID)
	if err != nil {
		return &protection.Settings{
			Template:        "cloudflare",
			BlockBots:       true,
			BlockHeadless:   true,
			BlockTor:        false,
			BlockProxy:      false,
			BlockDatacenter: false,
		}
	}
	return &settings
}

func (p *ProtectionHandler) saveSettings(userID string, settings *protection.Settings) error {
	_, err := p.db.Exec(`
		INSERT INTO user_antibot_defaults (user_id, settings)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			settings = EXCLUDED.settings,
			updated_at = NOW()
	`, userID, settings)
	return err
}

// Render builds the protection settings screen.
func (p *ProtectionHandler) Render(userID string) (string, InlineKeyboardMarkup) {
	settings := p.getSettings(userID)

	tplLabel := templateLabels[settings.Template]
	if tplLabel == "" {
		tplLabel = settings.Template
	}

	text := fmt.Sprintf("🛡️ <b>Bot Protection</b>\n\nDefault settings for new links.\n\n"+
		"📋 Template: <b>%s</b>", tplLabel)

	// Template row (show 4 most common, rest in "More...")
	var tplRow []InlineButton
	commonTemplates := []string{"cloudflare", "humancheck", "simple", "lobby"}
	for _, t := range commonTemplates {
		btn := InlineButton{CallbackData: "prot:" + protTemplate + t}
		if t == settings.Template {
			btn.Text = "✅ " + templateLabels[t]
			btn.Style = "success"
		} else {
			btn.Text = templateLabels[t]
		}
		tplRow = append(tplRow, btn)
	}

	// Block toggles
	blockOrder := []string{"bots", "headless", "tor", "proxy", "datacenter"}
	var blockRows [][]InlineButton
	for _, key := range blockOrder {
		var on bool
		switch key {
		case "bots":
			on = settings.BlockBots
		case "headless":
			on = settings.BlockHeadless
		case "tor":
			on = settings.BlockTor
		case "proxy":
			on = settings.BlockProxy
		case "datacenter":
			on = settings.BlockDatacenter
		}
		btn := InlineButton{CallbackData: "prot:" + protBlock + key}
		if on {
			btn.Text = "✅ " + blockLabels[key]
			btn.Style = "success"
		} else {
			btn.Text = "❌ " + blockLabels[key]
		}
		blockRows = append(blockRows, []InlineButton{btn})
	}

	rows := [][]InlineButton{tplRow}
	rows = append(rows, blockRows...)
	rows = append(rows, backRow())

	return text, InlineKeyboardMarkup{rows}
}

// Callback applies a "prot:*" action and returns the refreshed screen plus a toast.
func (p *ProtectionHandler) Callback(userID, data string) (string, InlineKeyboardMarkup, string) {
	toast := ""
	switch {
	case strings.HasPrefix(data, protTemplate):
		toast = p.setTemplate(userID, strings.TrimPrefix(data, protTemplate))
	case strings.HasPrefix(data, protBlock):
		toast = p.toggleBlock(userID, strings.TrimPrefix(data, protBlock))
	default:
		toast = "Unknown action"
	}
	text, kb := p.Render(userID)
	return text, kb, toast
}

func (p *ProtectionHandler) setTemplate(userID, template string) string {
	// Validate template
	valid := false
	for _, t := range templateOptions {
		if t == template {
			valid = true
			break
		}
	}
	if !valid {
		return "Invalid template"
	}

	settings := p.getSettings(userID)
	settings.Template = template
	if err := p.saveSettings(userID, settings); err != nil {
		log.Printf("[userbot] set template for user %s: %v", userID, err)
		return "⚠️ Could not update"
	}
	return "✅ Template updated"
}

func (p *ProtectionHandler) toggleBlock(userID, key string) string {
	if _, ok := blockColumns[key]; !ok {
		return "Unknown setting"
	}

	settings := p.getSettings(userID)
	var newVal bool
	switch key {
	case "bots":
		settings.BlockBots = !settings.BlockBots
		newVal = settings.BlockBots
	case "headless":
		settings.BlockHeadless = !settings.BlockHeadless
		newVal = settings.BlockHeadless
	case "tor":
		settings.BlockTor = !settings.BlockTor
		newVal = settings.BlockTor
	case "proxy":
		settings.BlockProxy = !settings.BlockProxy
		newVal = settings.BlockProxy
	case "datacenter":
		settings.BlockDatacenter = !settings.BlockDatacenter
		newVal = settings.BlockDatacenter
	}

	if err := p.saveSettings(userID, settings); err != nil {
		log.Printf("[userbot] toggle %s for user %s: %v", key, userID, err)
		return "⚠️ Could not update"
	}

	if newVal {
		return "✅ " + blockLabels[key] + " blocked"
	}
	return "❌ " + blockLabels[key] + " allowed"
}

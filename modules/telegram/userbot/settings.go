package userbot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/botginx/botginx/modules/auth/models"
)

// Settings callback data (after the "set:" prefix handled by the handler):
//
//	toggle:<key>   flip a notification preference
//	thr:<n>        set the traffic milestone threshold
const (
	setToggle    = "toggle:"
	setThreshold = "thr:"
)

var thresholdOptions = []int{50, 100, 500, 1000}

// toggleColumns maps callback keys to whitelisted DB columns.
var toggleColumns = map[string]string{
	"bot_attacks": "telegram_notify_bot_attacks",
	"traffic":     "telegram_notify_traffic_milestones",
	"ssl":         "telegram_notify_ssl_expiring",
	"balance":     "telegram_notify_balance_low",
}

// SettingsHandler renders and mutates a user's Telegram notification settings.
type SettingsHandler struct {
	db *sqlx.DB
}

func NewSettingsHandler(db *sqlx.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func toggleButton(label string, on bool, key string) InlineButton {
	b := InlineButton{CallbackData: "set:" + setToggle + key}
	if on {
		b.Text = "✅ " + label
		b.Style = "success"
	} else {
		b.Text = "❌ " + label
	}
	return b
}

// Render builds the settings screen for the user's current state.
func (s *SettingsHandler) Render(user *models.User) (string, InlineKeyboardMarkup) {
	threshold := user.TelegramTrafficThreshold
	if threshold <= 0 {
		threshold = 100
	}

	text := fmt.Sprintf("🔔 <b>Notifications</b>\n\nTap a setting to turn it on or off.\n\n"+
		"📊 Traffic threshold: <b>%d</b> visits", threshold)

	var thr []InlineButton
	for _, n := range thresholdOptions {
		label := strconv.Itoa(n)
		b := InlineButton{CallbackData: "set:" + setThreshold + label}
		if n == threshold {
			b.Text = "✅ " + label
			b.Style = "success"
		} else {
			b.Text = label
		}
		thr = append(thr, b)
	}

	return text, InlineKeyboardMarkup{[][]InlineButton{
		{toggleButton("Bot attack alerts", user.TelegramNotifyBotAttacks, "bot_attacks")},
		{toggleButton("Traffic milestones", user.TelegramNotifyTrafficMilestones, "traffic")},
		{toggleButton("SSL expiring", user.TelegramNotifySSLExpiring, "ssl")},
		{toggleButton("Low balance", user.TelegramNotifyBalanceLow, "balance")},
		thr,
		backRow(),
	}}
}

// Callback applies a "set:*" action and returns the refreshed screen plus a toast.
func (s *SettingsHandler) Callback(user *models.User, data string) (string, InlineKeyboardMarkup, string) {
	toast := ""
	switch {
	case strings.HasPrefix(data, setToggle):
		toast = s.toggle(user, strings.TrimPrefix(data, setToggle))
	case strings.HasPrefix(data, setThreshold):
		toast = s.setThreshold(user, strings.TrimPrefix(data, setThreshold))
	default:
		toast = "Unknown action"
	}
	text, kb := s.Render(user)
	return text, kb, toast
}

func (s *SettingsHandler) toggle(user *models.User, key string) string {
	col, ok := toggleColumns[key]
	if !ok {
		return "Unknown setting"
	}
	var val bool
	// col comes from the whitelist above, never from user input.
	q := fmt.Sprintf(`UPDATE users SET %[1]s = NOT COALESCE(%[1]s, FALSE) WHERE id = $1 RETURNING %[1]s`, col)
	if err := s.db.Get(&val, q, user.ID); err != nil {
		log.Printf("[userbot] toggle %s for user %s: %v", key, user.ID, err)
		return "⚠️ Could not update"
	}
	switch key {
	case "bot_attacks":
		user.TelegramNotifyBotAttacks = val
	case "traffic":
		user.TelegramNotifyTrafficMilestones = val
	case "ssl":
		user.TelegramNotifySSLExpiring = val
	case "balance":
		user.TelegramNotifyBalanceLow = val
	}
	return "✅ Updated"
}

func (s *SettingsHandler) setThreshold(user *models.User, raw string) string {
	n, err := strconv.Atoi(raw)
	valid := false
	for _, o := range thresholdOptions {
		if o == n {
			valid = true
		}
	}
	if err != nil || !valid {
		return "Invalid threshold"
	}
	if _, err := s.db.Exec(`UPDATE users SET telegram_traffic_threshold = $1 WHERE id = $2`, n, user.ID); err != nil {
		log.Printf("[userbot] set threshold for user %s: %v", user.ID, err)
		return "⚠️ Could not update"
	}
	user.TelegramTrafficThreshold = n
	return fmt.Sprintf("Threshold set to %d", n)
}

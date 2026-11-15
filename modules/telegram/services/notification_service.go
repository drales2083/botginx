// Package services contains Telegram user-bot notification logic.
package services

import (
	"fmt"
	"html"
	"sync"
	"time"

	"github.com/botginx/botginx/modules/telegram/userbot"
	"github.com/jmoiron/sqlx"
)

const (
	// LowBalanceThreshold is the balance (USD) below which a warning is sent.
	LowBalanceThreshold = 5.0
	// BotAttackThreshold is the number of blocked bots per window that triggers an alert.
	BotAttackThreshold = 25
	attackWindow       = time.Hour
	attackCooldown     = time.Hour
)

type prefs struct {
	BotAttacks        bool `db:"telegram_notify_bot_attacks"`
	TrafficMilestones bool `db:"telegram_notify_traffic_milestones"`
	SSLExpiring       bool `db:"telegram_notify_ssl_expiring"`
	BalanceLow        bool `db:"telegram_notify_balance_low"`
	Connected         bool `db:"connected"`
}

// NotificationService sends event-driven alerts through a user's Telegram bot.
type NotificationService struct {
	db  *sqlx.DB
	bot *userbot.UserBotService

	mu       sync.Mutex
	lastSent map[string]time.Time // key: userID|linkID (bot attack rate limit)
}

func NewNotificationService(db *sqlx.DB, bot *userbot.UserBotService) *NotificationService {
	return &NotificationService{db: db, bot: bot, lastSent: map[string]time.Time{}}
}

func (s *NotificationService) prefs(userID string) (*prefs, error) {
	var p prefs
	err := s.db.Get(&p, `SELECT telegram_notify_bot_attacks, telegram_notify_traffic_milestones,
		telegram_notify_ssl_expiring, telegram_notify_balance_low,
		(telegram_bot_token_encrypted IS NOT NULL AND telegram_bot_token_encrypted <> ''
			AND telegram_chat_id IS NOT NULL) AS connected
		FROM users WHERE id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *NotificationService) linkLabel(linkID string) string {
	var label string
	err := s.db.Get(&label, `SELECT d.name || rl.path FROM redirect_links rl
		INNER JOIN domains d ON d.id = rl.domain_id WHERE rl.id = $1`, linkID)
	if err != nil || label == "" {
		return linkID
	}
	return label
}

func (s *NotificationService) send(userID, text string, rows ...[]userbot.InlineButton) error {
	_, err := s.bot.SendNewMessage(userID, text, userbot.InlineKeyboardMarkup{InlineKeyboard: rows})
	return err
}

func btn(text, data string) userbot.InlineButton {
	return userbot.InlineButton{Text: text, CallbackData: data}
}

// NotifyBotAttack alerts the user that count bots were blocked on a link in the last hour.
// Rate limited to one notification per link per hour.
func (s *NotificationService) NotifyBotAttack(userID, linkID string, count int) error {
	p, err := s.prefs(userID)
	if err != nil || !p.Connected || !p.BotAttacks {
		return err
	}
	key := userID + "|" + linkID
	s.mu.Lock()
	if t, ok := s.lastSent[key]; ok && time.Since(t) < attackCooldown {
		s.mu.Unlock()
		return nil
	}
	s.lastSent[key] = time.Now()
	s.mu.Unlock()

	text := fmt.Sprintf("🚨 <b>Bot Attack Detected</b>\n\nLink: %s\nDetected: %d bots in last hour",
		html.EscapeString(s.linkLabel(linkID)), count)
	if err := s.send(userID, text, []userbot.InlineButton{
		btn("View Details", userbot.ScreenStats), btn("Dismiss", "dismiss"),
	}); err != nil {
		s.mu.Lock()
		delete(s.lastSent, key) // allow retry after a failed send
		s.mu.Unlock()
		return err
	}
	return nil
}

// NotifyTrafficMilestone congratulates the user when a link reaches a visit milestone.
func (s *NotificationService) NotifyTrafficMilestone(userID, linkID string, visits int) error {
	p, err := s.prefs(userID)
	if err != nil || !p.Connected || !p.TrafficMilestones {
		return err
	}
	var threshold int
	if err := s.db.Get(&threshold, `SELECT telegram_traffic_threshold FROM users WHERE id = $1`, userID); err != nil {
		return err
	}
	if visits < threshold {
		return nil
	}
	text := fmt.Sprintf("🎉 <b>Traffic Milestone!</b>\n\nYour link hit %s visits!\n%s",
		formatInt(visits), html.EscapeString(s.linkLabel(linkID)))
	return s.send(userID, text, []userbot.InlineButton{btn("View Stats", userbot.ScreenStats)})
}

// NotifySSLExpiring warns that a domain's certificate expires in daysLeft days.
func (s *NotificationService) NotifySSLExpiring(userID, domainName string, daysLeft int) error {
	p, err := s.prefs(userID)
	if err != nil || !p.Connected || !p.SSLExpiring {
		return err
	}
	unit := "days"
	if daysLeft == 1 {
		unit = "day"
	}
	text := fmt.Sprintf("⚠️ <b>SSL Certificate Expiring</b>\n\nDomain: %s\nExpires: %d %s",
		html.EscapeString(domainName), daysLeft, unit)
	return s.send(userID, text, []userbot.InlineButton{
		btn("Renew Now", userbot.ScreenLinks), btn("Settings", userbot.ScreenSettings),
	})
}

// NotifyBalanceLow warns when balance is below LowBalanceThreshold.
func (s *NotificationService) NotifyBalanceLow(userID string, balance float64) error {
	if balance >= LowBalanceThreshold {
		return nil
	}
	p, err := s.prefs(userID)
	if err != nil || !p.Connected || !p.BalanceLow {
		return err
	}
	text := fmt.Sprintf("💰 <b>Low Balance Warning</b>\n\nYour balance is $%.2f\nAdd funds to continue protection.", balance)
	return s.send(userID, text, []userbot.InlineButton{btn("Add Funds", userbot.ScreenBalance)})
}

// CheckBotAttack counts blocked bot visits on a link in the last hour and notifies
// if the threshold is reached. Intended to be run asynchronously.
func (s *NotificationService) CheckBotAttack(userID, linkID string) error {
	var n int
	if err := s.db.Get(&n, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_bot = TRUE
		AND created_at > NOW() - $2::interval`, linkID, fmt.Sprintf("%d seconds", int(attackWindow.Seconds()))); err != nil {
		return err
	}
	if n < BotAttackThreshold {
		return nil
	}
	return s.NotifyBotAttack(userID, linkID, n)
}

func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

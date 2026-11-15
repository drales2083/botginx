package models

import (
	"time"

	"github.com/lib/pq"
)

type User struct {
	ID               string         `db:"id" json:"id"`
	Email            string         `db:"email" json:"email"`
	PasswordHash     string         `db:"password_hash" json:"-"`
	Name             string         `db:"name" json:"name"`
	Role             string         `db:"role" json:"role"` // "user" or "admin"
	IsActive         bool           `db:"is_active" json:"isActive"`
	IsResellerClient bool           `db:"is_reseller_client" json:"isResellerClient"`
	Balance          float64        `db:"balance" json:"balance"`
	ReferralCode     *string        `db:"referral_code" json:"referralCode,omitempty"`
	ReferredByID     *string        `db:"referred_by_id" json:"referredById,omitempty"`
	BotsDetected     int64          `db:"bots_detected" json:"botsDetected"`
	HumansVerified   int64          `db:"humans_verified" json:"humansVerified"`
	TOTPSecret       *string        `db:"totp_secret" json:"-"`
	TOTPEnabled      bool           `db:"totp_enabled" json:"totpEnabled"`
	TOTPBackupCodes  pq.StringArray `db:"totp_backup_codes" json:"-"`
	TOTPEnabledAt    *time.Time     `db:"totp_enabled_at" json:"totpEnabledAt,omitempty"`
	CreatedAt        time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time      `db:"updated_at" json:"updatedAt"`

	// NameSilo integration
	NameSiloAPIKeyEncrypted *string    `db:"namesilo_api_key_encrypted" json:"-"`
	NameSiloConnectedAt     *time.Time `db:"namesilo_connected_at" json:"namesiloConnectedAt,omitempty"`

	// Cloudflare integration
	CloudflareEmail            *string    `db:"cloudflare_email" json:"-"`
	CloudflareAPIKeyEncrypted  *string    `db:"cloudflare_api_key_encrypted" json:"-"`
	CloudflareConnectedAt      *time.Time `db:"cloudflare_connected_at" json:"cloudflareConnectedAt,omitempty"`

	// GoDaddy integration
	GoDaddyAPIKeyEncrypted    *string    `db:"godaddy_api_key_encrypted" json:"-"`
	GoDaddyAPISecretEncrypted *string    `db:"godaddy_api_secret_encrypted" json:"-"`
	GoDaddyConnectedAt        *time.Time `db:"godaddy_connected_at" json:"godaddyConnectedAt,omitempty"`

	// Telegram user bot integration
	TelegramBotTokenEncrypted       *string    `db:"telegram_bot_token_encrypted" json:"-"`
	TelegramBotUsername             *string    `db:"telegram_bot_username" json:"telegramBotUsername,omitempty"`
	TelegramChatID                  *int64     `db:"telegram_chat_id" json:"-"`
	TelegramWebhookSecret           *string    `db:"telegram_webhook_secret" json:"-"`
	TelegramConnectedAt             *time.Time `db:"telegram_connected_at" json:"telegramConnectedAt,omitempty"`
	TelegramLastMessageID           *int64     `db:"telegram_last_message_id" json:"-"`
	TelegramNotifyBotAttacks        bool       `db:"telegram_notify_bot_attacks" json:"telegramNotifyBotAttacks"`
	TelegramNotifyTrafficMilestones bool       `db:"telegram_notify_traffic_milestones" json:"telegramNotifyTrafficMilestones"`
	TelegramNotifySSLExpiring       bool       `db:"telegram_notify_ssl_expiring" json:"telegramNotifySSLExpiring"`
	TelegramNotifyBalanceLow        bool       `db:"telegram_notify_balance_low" json:"telegramNotifyBalanceLow"`
	TelegramTrafficThreshold        int        `db:"telegram_traffic_threshold" json:"telegramTrafficThreshold"`
	TelegramAlertDismissed          bool       `db:"telegram_alert_dismissed" json:"-"`
}

type SignupInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type Session struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	Token     string    `db:"token" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

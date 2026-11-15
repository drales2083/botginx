// Package userbot manages per-user Telegram bots (connect, messaging, webhook lookup).
package userbot

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/botginx/botginx/modules/auth/models"
	"github.com/botginx/botginx/modules/telegram/service"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotConnected = errors.New("telegram bot not connected")
	ErrNoChat       = errors.New("telegram chat not linked yet")
)

type UserBotService struct {
	db      *sqlx.DB
	tgSvc   *service.Service
	baseURL string
}

type ConnectResult struct {
	BotUsername string
	WebhookURL  string
}

func NewUserBotService(db *sqlx.DB, tgSvc *service.Service, baseURL string) *UserBotService {
	return &UserBotService{db: db, tgSvc: tgSvc, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Connect validates the token, registers the webhook and stores the encrypted token.
func (s *UserBotService) Connect(userID, botToken string) (*ConnectResult, error) {
	me, err := s.tgSvc.GetMe(botToken)
	if err != nil {
		return nil, fmt.Errorf("invalid bot token: %w", err)
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(secretBytes)

	encrypted, err := crypto.Encrypt(botToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}

	webhookURL := s.baseURL + "/telegram/user-webhook/" + secret
	if err := s.tgSvc.SetWebhook(botToken, webhookURL); err != nil {
		return nil, fmt.Errorf("set webhook: %w", err)
	}

	_, err = s.db.Exec(`UPDATE users SET
		telegram_bot_token_encrypted = $1,
		telegram_bot_username = $2,
		telegram_webhook_secret = $3,
		telegram_chat_id = NULL,
		telegram_last_message_id = NULL,
		telegram_connected_at = NOW()
		WHERE id = $4`, encrypted, me.Username, secret, userID)
	if err != nil {
		_ = s.tgSvc.DeleteWebhook(botToken) // don't leave an orphaned webhook
		return nil, fmt.Errorf("save connection: %w", err)
	}

	return &ConnectResult{BotUsername: me.Username, WebhookURL: webhookURL}, nil
}

// Disconnect removes the webhook and clears all telegram connection fields.
func (s *UserBotService) Disconnect(userID string) error {
	// Best effort: the token may already be revoked; still clear local state.
	if token, err := s.GetBotToken(userID); err == nil {
		_ = s.tgSvc.DeleteWebhook(token)
	}
	_, err := s.db.Exec(`UPDATE users SET
		telegram_bot_token_encrypted = NULL,
		telegram_bot_username = NULL,
		telegram_chat_id = NULL,
		telegram_webhook_secret = NULL,
		telegram_connected_at = NULL,
		telegram_last_message_id = NULL
		WHERE id = $1`, userID)
	return err
}

// GetBotToken returns the decrypted bot token for a user.
func (s *UserBotService) GetBotToken(userID string) (string, error) {
	var enc *string
	err := s.db.Get(&enc, `SELECT telegram_bot_token_encrypted FROM users WHERE id = $1`, userID)
	if err != nil {
		return "", err
	}
	if enc == nil || *enc == "" {
		return "", ErrNotConnected
	}
	return crypto.Decrypt(*enc)
}

// GetUserByWebhookSecret finds the user owning an incoming webhook secret.
func (s *UserBotService) GetUserByWebhookSecret(secret string) (*models.User, error) {
	if secret == "" {
		return nil, sql.ErrNoRows
	}
	var u models.User
	if err := s.db.Get(&u, `SELECT * FROM users WHERE telegram_webhook_secret = $1`, secret); err != nil {
		return nil, err
	}
	return &u, nil
}

// target loads the token, chat ID (as string) and last message ID for a user.
func (s *UserBotService) target(userID string) (token, chatID string, lastMsg *int64, err error) {
	var row struct {
		Enc  *string `db:"telegram_bot_token_encrypted"`
		Chat *int64  `db:"telegram_chat_id"`
		Last *int64  `db:"telegram_last_message_id"`
	}
	if err = s.db.Get(&row, `SELECT telegram_bot_token_encrypted, telegram_chat_id, telegram_last_message_id
		FROM users WHERE id = $1`, userID); err != nil {
		return
	}
	if row.Enc == nil || *row.Enc == "" {
		err = ErrNotConnected
		return
	}
	if row.Chat == nil {
		err = ErrNoChat
		return
	}
	if token, err = crypto.Decrypt(*row.Enc); err != nil {
		return
	}
	return token, strconv.FormatInt(*row.Chat, 10), row.Last, nil
}

// SendOrEditMessage edits the last bot message if one exists, else sends a new one.
// If the edit fails (e.g. message deleted), it falls back to sending a new message.
func (s *UserBotService) SendOrEditMessage(userID, text string, replyMarkup interface{}) (int64, error) {
	token, chatID, last, err := s.target(userID)
	if err != nil {
		return 0, err
	}
	if last != nil && *last != 0 {
		if _, err := s.tgSvc.EditMessageText(token, chatID, *last, text, replyMarkup); err == nil {
			return *last, nil
		} else if strings.Contains(err.Error(), "message is not modified") {
			return *last, nil
		}
	}
	msg, err := s.tgSvc.SendMessage(token, chatID, text, replyMarkup)
	if err != nil {
		return 0, err
	}
	if err := s.SaveLastMessageID(userID, msg.MessageID); err != nil {
		return msg.MessageID, err
	}
	return msg.MessageID, nil
}

// EditMessage edits a specific message in the user's chat.
func (s *UserBotService) EditMessage(userID string, messageID int64, text string, replyMarkup interface{}) error {
	token, chatID, _, err := s.target(userID)
	if err != nil {
		return err
	}
	_, err = s.tgSvc.EditMessageText(token, chatID, messageID, text, replyMarkup)
	return err
}

// DeleteMessage deletes a single message in the user's chat.
func (s *UserBotService) DeleteMessage(userID string, messageID int64) error {
	token, chatID, _, err := s.target(userID)
	if err != nil {
		return err
	}
	return s.tgSvc.DeleteMessage(token, chatID, messageID)
}

// DeleteMessages deletes messages in batches of up to 100 (Telegram limit).
func (s *UserBotService) DeleteMessages(userID string, messageIDs []int64) error {
	if len(messageIDs) == 0 {
		return nil
	}
	token, chatID, _, err := s.target(userID)
	if err != nil {
		return err
	}
	for i := 0; i < len(messageIDs); i += 100 {
		end := i + 100
		if end > len(messageIDs) {
			end = len(messageIDs)
		}
		if err := s.tgSvc.DeleteMessages(token, chatID, messageIDs[i:end]); err != nil {
			return err
		}
	}
	return nil
}

// SaveLastMessageID stores the last bot message ID for later editing.
func (s *UserBotService) SaveLastMessageID(userID string, messageID int64) error {
	_, err := s.db.Exec(`UPDATE users SET telegram_last_message_id = $1 WHERE id = $2`, messageID, userID)
	return err
}

// AnswerCallback answers a callback query with optional alert popup.
func (s *UserBotService) AnswerCallback(token, callbackID, text string, showAlert bool) error {
	return s.tgSvc.AnswerCallbackQueryAlert(token, callbackID, text, showAlert)
}

// SaveChatID links the user's Telegram chat (only if none is linked yet).
// Returns true when the chat was newly linked.
func (s *UserBotService) SaveChatID(userID string, chatID int64) (bool, error) {
	res, err := s.db.Exec(`UPDATE users SET telegram_chat_id = $1 WHERE id = $2 AND telegram_chat_id IS NULL`, chatID, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SendNewMessage always sends a fresh message (never edits the app screen) and
// does not touch the stored last-message ID. Used for async event notifications.
func (s *UserBotService) SendNewMessage(userID, text string, replyMarkup interface{}) (int64, error) {
	token, chatID, _, err := s.target(userID)
	if err != nil {
		return 0, err
	}
	msg, err := s.tgSvc.SendMessage(token, chatID, text, replyMarkup)
	if err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

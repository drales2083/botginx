package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

type Service struct {
	db     *sqlx.DB
	client *http.Client
}

type Bot struct {
	ID            string    `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	BotToken      string    `db:"bot_token" json:"bot_token"`
	BotUsername   *string   `db:"bot_username" json:"bot_username"`
	ChatID        *string   `db:"chat_id" json:"chat_id"`
	WebhookSecret *string   `db:"webhook_secret" json:"webhook_secret,omitempty"`
	Enabled       bool      `db:"enabled" json:"enabled"`
	UseForSupport bool      `db:"use_for_support" json:"use_for_support"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

type PendingReply struct {
	ID             string    `db:"id"`
	BotID          string    `db:"bot_id"`
	TelegramUserID int64     `db:"telegram_user_id"`
	TelegramChatID int64     `db:"telegram_chat_id"`
	TicketID       string    `db:"ticket_id"`
	CreatedAt      time.Time `db:"created_at"`
	ExpiresAt      time.Time `db:"expires_at"`
}

func NewService(db *sqlx.DB) *Service {
	return &Service{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Bot CRUD

func (s *Service) ListBots() ([]Bot, error) {
	var bots []Bot
	err := s.db.Select(&bots, `
		SELECT id, name, bot_token, bot_username, chat_id, webhook_secret,
		       enabled, use_for_support, created_at, updated_at
		FROM telegram_bots
		ORDER BY created_at DESC
	`)
	return bots, err
}

func (s *Service) GetBot(id string) (*Bot, error) {
	var bot Bot
	err := s.db.Get(&bot, `
		SELECT id, name, bot_token, bot_username, chat_id, webhook_secret,
		       enabled, use_for_support, created_at, updated_at
		FROM telegram_bots WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	return &bot, nil
}

func (s *Service) GetBotByToken(token string) (*Bot, error) {
	var bot Bot
	err := s.db.Get(&bot, `
		SELECT id, name, bot_token, bot_username, chat_id, webhook_secret,
		       enabled, use_for_support, created_at, updated_at
		FROM telegram_bots WHERE bot_token = $1
	`, token)
	if err != nil {
		return nil, err
	}
	return &bot, nil
}

func (s *Service) GetSupportBots() ([]Bot, error) {
	var bots []Bot
	err := s.db.Select(&bots, `
		SELECT id, name, bot_token, bot_username, chat_id, webhook_secret,
		       enabled, use_for_support, created_at, updated_at
		FROM telegram_bots
		WHERE enabled = TRUE AND use_for_support = TRUE AND chat_id IS NOT NULL
	`)
	return bots, err
}

func (s *Service) CreateBot(name, token string) (*Bot, error) {
	// Generate webhook secret
	secretBytes := make([]byte, 16)
	rand.Read(secretBytes)
	secret := hex.EncodeToString(secretBytes)

	var bot Bot
	err := s.db.Get(&bot, `
		INSERT INTO telegram_bots (name, bot_token, webhook_secret)
		VALUES ($1, $2, $3)
		RETURNING id, name, bot_token, bot_username, chat_id, webhook_secret,
		          enabled, use_for_support, created_at, updated_at
	`, name, token, secret)
	return &bot, err
}

func (s *Service) UpdateBot(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	query := "UPDATE telegram_bots SET "
	args := []interface{}{}
	i := 1
	for key, val := range updates {
		if i > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", key, i)
		args = append(args, val)
		i++
	}
	query += fmt.Sprintf(" WHERE id = $%d", i)
	args = append(args, id)

	_, err := s.db.Exec(query, args...)
	return err
}

func (s *Service) DeleteBot(id string) error {
	_, err := s.db.Exec("DELETE FROM telegram_bots WHERE id = $1", id)
	return err
}

// Telegram API

type TelegramResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type TelegramChat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

type TelegramMessage struct {
	MessageID int64         `json:"message_id"`
	Chat      *TelegramChat `json:"chat"`
	Text      string        `json:"text,omitempty"`
}

func (s *Service) apiCall(token, method string, payload interface{}) (*TelegramResponse, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)

	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Service) GetMe(token string) (*TelegramUser, error) {
	resp, err := s.apiCall(token, "getMe", nil)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram API error: %s", resp.Description)
	}

	var user TelegramUser
	if err := json.Unmarshal(resp.Result, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) GetUpdates(token string) ([]TelegramChat, error) {
	resp, err := s.apiCall(token, "getUpdates", map[string]interface{}{
		"limit":  100,
		"offset": -100,
	})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram API error: %s", resp.Description)
	}

	var updates []struct {
		Message *struct {
			Chat TelegramChat `json:"chat"`
		} `json:"message,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &updates); err != nil {
		return nil, err
	}

	// Deduplicate chats
	seen := make(map[int64]bool)
	var chats []TelegramChat
	for _, u := range updates {
		if u.Message != nil && !seen[u.Message.Chat.ID] {
			seen[u.Message.Chat.ID] = true
			chats = append(chats, u.Message.Chat)
		}
	}
	return chats, nil
}

func (s *Service) SetWebhook(token, url string) error {
	resp, err := s.apiCall(token, "setWebhook", map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("telegram API error: %s", resp.Description)
	}
	return nil
}

func (s *Service) DeleteWebhook(token string) error {
	resp, err := s.apiCall(token, "deleteWebhook", nil)
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("telegram API error: %s", resp.Description)
	}
	return nil
}

func (s *Service) SendMessage(token, chatID, text string, replyMarkup interface{}) (*TelegramMessage, error) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	resp, err := s.apiCall(token, "sendMessage", payload)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram API error: %s", resp.Description)
	}

	var msg TelegramMessage
	if err := json.Unmarshal(resp.Result, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Service) EditMessageReplyMarkup(token, chatID string, messageID int64, replyMarkup interface{}) error {
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"message_id":   messageID,
		"reply_markup": replyMarkup,
	}

	resp, err := s.apiCall(token, "editMessageReplyMarkup", payload)
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("telegram API error: %s", resp.Description)
	}
	return nil
}

func (s *Service) AnswerCallbackQuery(token, callbackID, text string) error {
	resp, err := s.apiCall(token, "answerCallbackQuery", map[string]interface{}{
		"callback_query_id": callbackID,
		"text":              text,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("telegram API error: %s", resp.Description)
	}
	return nil
}

// Pending Replies

func (s *Service) CreatePendingReply(botID string, userID, chatID int64, ticketID string) error {
	// Delete any existing pending reply for this user
	s.db.Exec("DELETE FROM telegram_pending_replies WHERE telegram_user_id = $1 AND bot_id = $2", userID, botID)

	_, err := s.db.Exec(`
		INSERT INTO telegram_pending_replies (bot_id, telegram_user_id, telegram_chat_id, ticket_id)
		VALUES ($1, $2, $3, $4)
	`, botID, userID, chatID, ticketID)
	return err
}

func (s *Service) GetPendingReply(botID string, userID int64) (*PendingReply, error) {
	var pending PendingReply
	err := s.db.Get(&pending, `
		SELECT id, bot_id, telegram_user_id, telegram_chat_id, ticket_id, created_at, expires_at
		FROM telegram_pending_replies
		WHERE bot_id = $1 AND telegram_user_id = $2 AND expires_at > NOW()
	`, botID, userID)
	if err != nil {
		return nil, err
	}
	return &pending, nil
}

func (s *Service) DeletePendingReply(id string) error {
	_, err := s.db.Exec("DELETE FROM telegram_pending_replies WHERE id = $1", id)
	return err
}

func (s *Service) CleanupExpiredReplies() {
	s.db.Exec("DELETE FROM telegram_pending_replies WHERE expires_at < NOW()")
}

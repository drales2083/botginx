package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/botginx/botginx/modules/telegram/service"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	db        *sqlx.DB
	templates *module.TemplateEngine
	service   *service.Service
}

func NewHandler(db *sqlx.DB, templates *module.TemplateEngine, svc *service.Service) *Handler {
	return &Handler{db: db, templates: templates, service: svc}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, msg string, status int) {
	h.json(w, status, map[string]interface{}{"error": msg})
}

func (h *Handler) jsonOK(w http.ResponseWriter, data interface{}) {
	h.json(w, http.StatusOK, data)
}

// Admin Settings Page

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	bots, _ := h.service.ListBots()

	module.Render(w, r, h.templates, "telegram:settings.html", map[string]interface{}{
		"Title": "Telegram Settings",
		"Bots":  bots,
	})
}

// Admin API

func (h *Handler) APIListBots(w http.ResponseWriter, r *http.Request) {
	bots, err := h.service.ListBots()
	if err != nil {
		h.jsonError(w, "Failed to fetch bots", http.StatusInternalServerError)
		return
	}

	// Hide tokens in response (show only last 8 chars)
	for i := range bots {
		if len(bots[i].BotToken) > 8 {
			bots[i].BotToken = "..." + bots[i].BotToken[len(bots[i].BotToken)-8:]
		}
	}

	h.jsonOK(w, map[string]interface{}{"bots": bots})
}

func (h *Handler) APICreateBot(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		BotToken string `json:"bot_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Name == "" || input.BotToken == "" {
		h.jsonError(w, "Name and bot token are required", http.StatusBadRequest)
		return
	}

	// Verify token by calling getMe
	user, err := h.service.GetMe(input.BotToken)
	if err != nil {
		h.jsonError(w, "Invalid bot token: "+err.Error(), http.StatusBadRequest)
		return
	}

	bot, err := h.service.CreateBot(input.Name, input.BotToken)
	if err != nil {
		h.jsonError(w, "Failed to create bot", http.StatusInternalServerError)
		return
	}

	// Update with bot username
	h.service.UpdateBot(bot.ID, map[string]interface{}{
		"bot_username": user.Username,
	})
	bot.BotUsername = &user.Username

	h.jsonOK(w, map[string]interface{}{
		"success": true,
		"bot":     bot,
	})
}

func (h *Handler) APIUpdateBot(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "id")

	var input struct {
		Name          *string `json:"name"`
		ChatID        *string `json:"chat_id"`
		Enabled       *bool   `json:"enabled"`
		UseForSupport *bool   `json:"use_for_support"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.ChatID != nil {
		updates["chat_id"] = *input.ChatID
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}
	if input.UseForSupport != nil {
		updates["use_for_support"] = *input.UseForSupport
	}

	if len(updates) == 0 {
		h.jsonError(w, "No updates provided", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateBot(botID, updates); err != nil {
		h.jsonError(w, "Failed to update bot", http.StatusInternalServerError)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIDeleteBot(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "id")

	// Get bot to delete webhook
	bot, err := h.service.GetBot(botID)
	if err == nil && bot != nil {
		h.service.DeleteWebhook(bot.BotToken)
	}

	if err := h.service.DeleteBot(botID); err != nil {
		h.jsonError(w, "Failed to delete bot", http.StatusInternalServerError)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APITestBot(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "id")

	bot, err := h.service.GetBot(botID)
	if err != nil {
		h.jsonError(w, "Bot not found", http.StatusNotFound)
		return
	}

	// Test by calling getMe
	user, err := h.service.GetMe(bot.BotToken)
	if err != nil {
		h.jsonError(w, "Connection failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// If chat_id is set, send test message
	if bot.ChatID != nil && *bot.ChatID != "" {
		_, err := h.service.SendMessage(bot.BotToken, *bot.ChatID, "🧪 Test message from GuardBot panel", nil)
		if err != nil {
			h.jsonOK(w, map[string]interface{}{
				"success":      true,
				"bot_username": user.Username,
				"message_sent": false,
				"error":        "Connected to bot, but failed to send message: " + err.Error(),
			})
			return
		}
		h.jsonOK(w, map[string]interface{}{
			"success":      true,
			"bot_username": user.Username,
			"message_sent": true,
		})
		return
	}

	h.jsonOK(w, map[string]interface{}{
		"success":      true,
		"bot_username": user.Username,
		"message_sent": false,
	})
}

func (h *Handler) APISetupWebhook(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "id")

	var input struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	bot, err := h.service.GetBot(botID)
	if err != nil {
		h.jsonError(w, "Bot not found", http.StatusNotFound)
		return
	}

	// Build webhook URL
	webhookURL := strings.TrimSuffix(input.BaseURL, "/") + "/telegram/webhook/" + bot.ID + "/" + *bot.WebhookSecret

	if err := h.service.SetWebhook(bot.BotToken, webhookURL); err != nil {
		h.jsonError(w, "Failed to set webhook: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{
		"success":     true,
		"webhook_url": webhookURL,
	})
}

func (h *Handler) APIGetChats(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "id")

	bot, err := h.service.GetBot(botID)
	if err != nil {
		h.jsonError(w, "Bot not found", http.StatusNotFound)
		return
	}

	chats, err := h.service.GetUpdates(bot.BotToken)
	if err != nil {
		h.jsonError(w, "Failed to fetch chats: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"chats": chats})
}

// Webhook Handler

type WebhookUpdate struct {
	UpdateID      int64                   `json:"update_id"`
	Message       *WebhookMessage         `json:"message,omitempty"`
	CallbackQuery *WebhookCallbackQuery   `json:"callback_query,omitempty"`
}

type WebhookMessage struct {
	MessageID int64        `json:"message_id"`
	From      *TelegramUser `json:"from"`
	Chat      *TelegramChat `json:"chat"`
	Text      string       `json:"text"`
	Date      int64        `json:"date"`
}

type WebhookCallbackQuery struct {
	ID      string           `json:"id"`
	From    *TelegramUser    `json:"from"`
	Message *WebhookMessage  `json:"message,omitempty"`
	Data    string           `json:"data"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type TelegramChat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botID")
	secret := chi.URLParam(r, "secret")

	// Validate bot and secret
	bot, err := h.service.GetBot(botID)
	if err != nil || bot == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if bot.WebhookSecret == nil || *bot.WebhookSecret != secret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var update WebhookUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Handle callback query (button click)
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(bot, update.CallbackQuery)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Handle message (text reply)
	if update.Message != nil && update.Message.Text != "" {
		h.handleMessage(bot, update.Message)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleCallbackQuery(bot *service.Bot, query *WebhookCallbackQuery) {
	// Parse callback data: "action:ticket_id"
	parts := strings.SplitN(query.Data, ":", 2)
	if len(parts) != 2 {
		h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Invalid action")
		return
	}

	action := parts[0]
	ticketID := parts[1]

	switch action {
	case "reply":
		h.handleReplyButton(bot, query, ticketID)
	case "close":
		h.handleCloseButton(bot, query, ticketID)
	case "view":
		h.handleViewButton(bot, query, ticketID)
	default:
		h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Unknown action")
	}
}

func (h *Handler) handleReplyButton(bot *service.Bot, query *WebhookCallbackQuery, ticketID string) {
	// Get ticket info
	var ticket struct {
		Subject   string `db:"subject"`
		UserEmail string `db:"user_email"`
	}
	err := h.db.Get(&ticket, `
		SELECT t.subject, u.email as user_email
		FROM support_tickets t
		JOIN users u ON u.id = t.user_id
		WHERE t.id = $1
	`, ticketID)
	if err != nil {
		h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Ticket not found")
		return
	}

	// Create pending reply
	chatID := query.Message.Chat.ID
	userID := query.From.ID
	if err := h.service.CreatePendingReply(bot.ID, userID, chatID, ticketID); err != nil {
		h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Error creating reply session")
		return
	}

	// Send prompt message with ForceReply to ensure bot receives the response
	text := "📝 <b>Reply to:</b> " + ticket.Subject + "\n\n" +
		"Ticket from <code>" + ticket.UserEmail + "</code>\n\n" +
		"⬇️ <b>Reply to THIS message</b> with your response, or send /cancel"

	// ForceReply prompts user to reply directly to this message
	forceReply := map[string]interface{}{
		"force_reply":             true,
		"input_field_placeholder": "Type your reply here...",
		"selective":               true,
	}

	h.service.SendMessage(bot.BotToken, *bot.ChatID, text, forceReply)
	h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Send your reply message")
}

func (h *Handler) handleCloseButton(bot *service.Bot, query *WebhookCallbackQuery, ticketID string) {
	// Close the ticket
	result, err := h.db.Exec(`
		UPDATE support_tickets SET status = 'closed', updated_at = NOW()
		WHERE id = $1 AND status != 'closed'
	`, ticketID)
	if err != nil {
		h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Error closing ticket")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Ticket already closed")
		return
	}

	// Update the message to remove buttons
	h.service.EditMessageReplyMarkup(bot.BotToken, *bot.ChatID, query.Message.MessageID, nil)
	h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "✅ Ticket closed")
}

func (h *Handler) handleViewButton(bot *service.Bot, query *WebhookCallbackQuery, ticketID string) {
	// Just acknowledge - user can click the link in the original message
	h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Open panel to view ticket")
}

func (h *Handler) handleMessage(bot *service.Bot, msg *WebhookMessage) {
	// Check for /cancel command
	if msg.Text == "/cancel" {
		h.db.Exec("DELETE FROM telegram_pending_replies WHERE telegram_user_id = $1 AND bot_id = $2",
			msg.From.ID, bot.ID)
		h.service.SendMessage(bot.BotToken, *bot.ChatID, "❌ Reply cancelled", nil)
		return
	}

	// Check for pending reply
	pending, err := h.service.GetPendingReply(bot.ID, msg.From.ID)
	if err != nil {
		// No pending reply - ignore message
		return
	}

	// Process the reply
	h.processTicketReply(bot, pending, msg.Text)
}

func (h *Handler) processTicketReply(bot *service.Bot, pending *service.PendingReply, replyText string) {
	// Validate reply length
	if len(replyText) < 10 {
		h.service.SendMessage(bot.BotToken, *bot.ChatID, "⚠️ Reply too short (min 10 chars)", nil)
		return
	}
	if len(replyText) > 2000 {
		replyText = replyText[:2000]
	}

	// Generate message ID
	idBytes := make([]byte, 12)
	rand.Read(idBytes)
	messageID := hex.EncodeToString(idBytes)

	// Create the ticket reply
	_, err := h.db.Exec(`
		INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at, telegram_bot_id)
		VALUES ($1, $2, NULL, TRUE, $3, NOW(), $4)
	`, messageID, pending.TicketID, replyText, bot.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create ticket reply")
		h.service.SendMessage(bot.BotToken, *bot.ChatID, "❌ Failed to save reply", nil)
		return
	}

	// Update ticket status to answered
	h.db.Exec(`UPDATE support_tickets SET status = 'answered', updated_at = NOW() WHERE id = $1`, pending.TicketID)

	// Delete pending reply
	h.service.DeletePendingReply(pending.ID)

	// Get ticket info for confirmation
	var subject string
	h.db.Get(&subject, "SELECT subject FROM support_tickets WHERE id = $1", pending.TicketID)

	// Send confirmation
	h.service.SendMessage(bot.BotToken, *bot.ChatID,
		"✅ Reply sent to ticket:\n<b>"+subject+"</b>", nil)

	// Notify other support bots that this ticket was replied
	h.notifyOtherBots(bot.ID, pending.TicketID, "replied")
}

func (h *Handler) notifyOtherBots(excludeBotID, ticketID, action string) {
	// Get other support bots
	var bots []service.Bot
	h.db.Select(&bots, `
		SELECT id, bot_token, chat_id
		FROM telegram_bots
		WHERE id != $1 AND enabled = TRUE AND use_for_support = TRUE AND chat_id IS NOT NULL
	`, excludeBotID)

	var subject string
	h.db.Get(&subject, "SELECT subject FROM support_tickets WHERE id = $1", ticketID)

	for _, bot := range bots {
		text := "ℹ️ Ticket <b>" + subject + "</b> was " + action + " via another channel"
		h.service.SendMessage(bot.BotToken, *bot.ChatID, text, nil)
	}
}

// NotifyNewTicket sends notification to all support bots
func (h *Handler) NotifyNewTicket(ticketID, email, category, subject, message string) {
	bots, err := h.service.GetSupportBots()
	if err != nil || len(bots) == 0 {
		return
	}

	// Truncate message preview
	preview := message
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	text := "🎫 <b>New Support Ticket</b>\n\n" +
		"<b>From:</b> <code>" + email + "</code>\n" +
		"<b>Category:</b> " + category + "\n" +
		"<b>Subject:</b> " + subject + "\n\n" +
		preview

	// Inline keyboard
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": "💬 Reply", "callback_data": "reply:" + ticketID},
				{"text": "✅ Close", "callback_data": "close:" + ticketID},
			},
		},
	}

	for _, bot := range bots {
		msg, err := h.service.SendMessage(bot.BotToken, *bot.ChatID, text, keyboard)
		if err != nil {
			log.Error().Err(err).Str("bot", bot.Name).Msg("Failed to send ticket notification")
			continue
		}

		// Store message ID for later editing
		h.db.Exec(`
			UPDATE support_tickets
			SET telegram_message_id = $1, telegram_bot_id = $2, telegram_chat_id = $3
			WHERE id = $4
		`, msg.MessageID, bot.ID, bot.ChatID, ticketID)
	}
}

// NotifyTicketReply sends notification when user replies
func (h *Handler) NotifyTicketReply(ticketID, email, subject, message string) {
	bots, err := h.service.GetSupportBots()
	if err != nil || len(bots) == 0 {
		return
	}

	// Check if this reply was made via Telegram (don't notify back)
	var telegramBotID sql.NullString
	h.db.Get(&telegramBotID, `
		SELECT telegram_bot_id FROM ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, ticketID)

	// Truncate message preview
	preview := message
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	text := "💬 <b>Ticket Reply</b>\n\n" +
		"<b>From:</b> <code>" + email + "</code>\n" +
		"<b>Re:</b> " + subject + "\n\n" +
		preview

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": "💬 Reply", "callback_data": "reply:" + ticketID},
				{"text": "✅ Close", "callback_data": "close:" + ticketID},
			},
		},
	}

	for _, bot := range bots {
		// Skip if reply came from this bot
		if telegramBotID.Valid && telegramBotID.String == bot.ID {
			continue
		}

		h.service.SendMessage(bot.BotToken, *bot.ChatID, text, keyboard)
	}
}

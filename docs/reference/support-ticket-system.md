# Support Ticket System with Telegram Bot Integration

A complete reference for implementing a support ticket system with Telegram bot notifications and two-way reply capability. Written for AI agents to implement on any platform.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              USER FLOW                                   │
├─────────────────────────────────────────────────────────────────────────┤
│  User creates ticket → Stored in DB → Telegram notification to admin   │
│  User replies        → Stored in DB → Telegram notification to admin   │
│  User closes ticket  → Status updated                                  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                             ADMIN FLOW                                   │
├─────────────────────────────────────────────────────────────────────────┤
│  Admin clicks "Reply" button in Telegram → Creates pending reply state │
│  Admin sends message → Message saved to ticket → User sees in panel    │
│  Admin clicks "Close" button → Ticket closed                           │
│  Admin replies via web panel → Same result, notifies other bots        │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                          STATUS TRANSITIONS                              │
├─────────────────────────────────────────────────────────────────────────┤
│  open ──(admin replies)──→ answered ──(user replies)──→ open           │
│    │                           │                           │            │
│    └───────(close)─────────────┴───────(close)─────────────┘            │
│                                ↓                                        │
│                             closed ──(reopen)──→ open                   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

### Core Tables

```sql
-- Support tickets table
CREATE TABLE IF NOT EXISTS support_tickets (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    subject TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',  -- open, answered, closed
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    -- Telegram tracking (for editing notifications)
    telegram_message_id BIGINT,
    telegram_bot_id TEXT,
    telegram_chat_id BIGINT
);

-- Ticket messages (conversation thread)
CREATE TABLE IF NOT EXISTS ticket_messages (
    id TEXT PRIMARY KEY,
    ticket_id TEXT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    user_id TEXT,                    -- NULL for admin messages
    is_admin BOOLEAN DEFAULT FALSE,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    -- Track which Telegram bot sent this reply (prevents notification loops)
    telegram_bot_id TEXT
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON support_tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON support_tickets(status);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_ticket_id ON ticket_messages(ticket_id);
```

### Telegram Bot Tables

```sql
-- Telegram bot configurations
CREATE TABLE IF NOT EXISTS telegram_bots (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name TEXT NOT NULL,
    bot_token TEXT NOT NULL,
    bot_username TEXT,
    chat_id TEXT,                    -- Admin chat/group to send notifications
    webhook_secret TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    use_for_support BOOLEAN DEFAULT FALSE,  -- Enable for support notifications
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Pending replies (tracks "admin clicked Reply, waiting for message")
CREATE TABLE IF NOT EXISTS telegram_pending_replies (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    bot_id TEXT REFERENCES telegram_bots(id) ON DELETE CASCADE,
    telegram_user_id BIGINT NOT NULL,
    telegram_chat_id BIGINT NOT NULL,
    ticket_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP DEFAULT NOW() + INTERVAL '5 minutes'
);

CREATE INDEX IF NOT EXISTS idx_pending_replies_user ON telegram_pending_replies(telegram_user_id, bot_id);
CREATE INDEX IF NOT EXISTS idx_pending_replies_expires ON telegram_pending_replies(expires_at);
```

---

## API Endpoints

### User Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /user/support | List user's tickets |
| GET | /user/support/new | New ticket form |
| GET | /user/support/{id} | View ticket conversation |
| POST | /user/support/api | Create new ticket |
| POST | /user/support/api/{id}/reply | Reply to ticket |
| POST | /user/support/api/{id}/close | Close ticket |

### Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /admin/support | List all tickets (with filters) |
| GET | /admin/support/{id} | View ticket as admin |
| POST | /admin/support/api/{id}/reply | Reply as admin |
| POST | /admin/support/api/{id}/close | Close ticket |
| POST | /admin/support/api/{id}/reopen | Reopen ticket |

### Telegram Webhook

| Method | Path | Description |
|--------|------|-------------|
| POST | /telegram/webhook/{bot_id} | Receive Telegram updates |

---

## Implementation Guide

### Step 1: Notifier Interface

Define an interface that decouples the support module from Telegram:

```go
// Notifier interface for sending ticket notifications
type Notifier interface {
    NotifyNewTicket(ticketID, email, category, subject, message string)
    NotifyTicketReply(ticketID, email, subject, message string)
}
```

The support handler holds a reference to the notifier:

```go
type Handler struct {
    db       *sqlx.DB
    notifier Notifier
}

func (h *Handler) SetNotifier(n Notifier) {
    h.notifier = n
}
```

### Step 2: Create Ticket Handler

```go
func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
    userID := ctx.GetUserID(r)

    var input struct {
        Category string `json:"category"`
        Subject  string `json:"subject"`
        Message  string `json:"message"`
    }
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        h.jsonError(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Validate
    if len(input.Subject) < 5 || len(input.Subject) > 100 {
        h.jsonError(w, "Subject must be 5-100 characters", http.StatusBadRequest)
        return
    }
    if len(input.Message) < 10 || len(input.Message) > 2000 {
        h.jsonError(w, "Message must be 10-2000 characters", http.StatusBadRequest)
        return
    }

    // Create ticket
    ticketID := generateID()
    _, err := h.db.Exec(`
        INSERT INTO support_tickets (id, user_id, category, subject, status, created_at, updated_at)
        VALUES ($1, $2, $3, $4, 'open', NOW(), NOW())
    `, ticketID, userID, input.Category, input.Subject)
    if err != nil {
        h.jsonError(w, "Failed to create ticket", http.StatusInternalServerError)
        return
    }

    // Create first message
    messageID := generateID()
    h.db.Exec(`
        INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at)
        VALUES ($1, $2, $3, FALSE, $4, NOW())
    `, messageID, ticketID, userID, input.Message)

    // Notify admin via Telegram
    if h.notifier != nil {
        var email string
        h.db.Get(&email, `SELECT email FROM users WHERE id = $1`, userID)
        h.notifier.NotifyNewTicket(ticketID, email, input.Category, input.Subject, input.Message)
    }

    h.jsonOK(w, map[string]interface{}{
        "success":   true,
        "ticket_id": ticketID,
        "redirect":  "/user/support/" + ticketID,
    })
}
```

### Step 3: Telegram Notification with Inline Buttons

```go
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

    // Inline keyboard with Reply and Close buttons
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
```

### Step 4: Handle Telegram Button Clicks (Callback Queries)

```go
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

    // Create pending reply (tracks that admin is about to send a message)
    chatID := query.Message.Chat.ID
    userID := query.From.ID
    h.service.CreatePendingReply(bot.ID, userID, chatID, ticketID)

    // Prompt admin to send reply message
    text := "📝 <b>Reply to:</b> " + ticket.Subject + "\n\n" +
        "Ticket from <code>" + ticket.UserEmail + "</code>\n\n" +
        "⬇️ <b>Reply to THIS message</b> with your response, or send /cancel"

    // ForceReply prompts user to reply directly
    forceReply := map[string]interface{}{
        "force_reply":             true,
        "input_field_placeholder": "Type your reply here...",
        "selective":               true,
    }

    h.service.SendMessage(bot.BotToken, *bot.ChatID, text, forceReply)
    h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "Send your reply message")
}

func (h *Handler) handleCloseButton(bot *service.Bot, query *WebhookCallbackQuery, ticketID string) {
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

    // Remove buttons from notification
    h.service.EditMessageReplyMarkup(bot.BotToken, *bot.ChatID, query.Message.MessageID, nil)
    h.service.AnswerCallbackQuery(bot.BotToken, query.ID, "✅ Ticket closed")
}
```

### Step 5: Handle Admin Reply Messages

```go
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

    messageID := generateID()

    // Create the ticket reply (mark which bot sent it to prevent notification loops)
    _, err := h.db.Exec(`
        INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at, telegram_bot_id)
        VALUES ($1, $2, NULL, TRUE, $3, NOW(), $4)
    `, messageID, pending.TicketID, replyText, bot.ID)
    if err != nil {
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
}
```

### Step 6: Telegram API Helpers

```go
type TelegramResponse struct {
    OK          bool            `json:"ok"`
    Result      json.RawMessage `json:"result,omitempty"`
    Description string          `json:"description,omitempty"`
}

type TelegramMessage struct {
    MessageID int64 `json:"message_id"`
}

func (s *Service) SendMessage(token, chatID, text string, replyMarkup interface{}) (*TelegramMessage, error) {
    body := map[string]interface{}{
        "chat_id":    chatID,
        "text":       text,
        "parse_mode": "HTML",
    }
    if replyMarkup != nil {
        body["reply_markup"] = replyMarkup
    }

    jsonBody, _ := json.Marshal(body)
    resp, err := s.client.Post(
        "https://api.telegram.org/bot"+token+"/sendMessage",
        "application/json",
        bytes.NewReader(jsonBody),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result TelegramResponse
    json.NewDecoder(resp.Body).Decode(&result)
    if !result.OK {
        return nil, fmt.Errorf("telegram: %s", result.Description)
    }

    var msg TelegramMessage
    json.Unmarshal(result.Result, &msg)
    return &msg, nil
}

func (s *Service) AnswerCallbackQuery(token, queryID, text string) error {
    body := map[string]interface{}{
        "callback_query_id": queryID,
        "text":              text,
    }
    jsonBody, _ := json.Marshal(body)
    resp, err := s.client.Post(
        "https://api.telegram.org/bot"+token+"/answerCallbackQuery",
        "application/json",
        bytes.NewReader(jsonBody),
    )
    if err != nil {
        return err
    }
    resp.Body.Close()
    return nil
}

func (s *Service) EditMessageReplyMarkup(token, chatID string, messageID int64, markup interface{}) error {
    body := map[string]interface{}{
        "chat_id":    chatID,
        "message_id": messageID,
    }
    if markup != nil {
        body["reply_markup"] = markup
    }
    jsonBody, _ := json.Marshal(body)
    resp, err := s.client.Post(
        "https://api.telegram.org/bot"+token+"/editMessageReplyMarkup",
        "application/json",
        bytes.NewReader(jsonBody),
    )
    if err != nil {
        return err
    }
    resp.Body.Close()
    return nil
}
```

### Step 7: Pending Reply Management

```go
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

// Call this periodically to clean up expired pending replies
func (s *Service) CleanupExpiredReplies() {
    s.db.Exec("DELETE FROM telegram_pending_replies WHERE expires_at < NOW()")
}
```

### Step 8: Webhook Setup

```go
func (s *Service) SetWebhook(token, webhookURL, secret string) error {
    body := map[string]interface{}{
        "url":          webhookURL,
        "secret_token": secret,
    }
    jsonBody, _ := json.Marshal(body)
    resp, err := s.client.Post(
        "https://api.telegram.org/bot"+token+"/setWebhook",
        "application/json",
        bytes.NewReader(jsonBody),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var result TelegramResponse
    json.NewDecoder(resp.Body).Decode(&result)
    if !result.OK {
        return fmt.Errorf("telegram: %s", result.Description)
    }
    return nil
}
```

### Step 9: Webhook Handler

```go
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
    botID := chi.URLParam(r, "bot_id")
    
    bot, err := h.service.GetBot(botID)
    if err != nil || !bot.Enabled {
        w.WriteHeader(http.StatusNotFound)
        return
    }

    // Verify webhook secret
    secretHeader := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
    if bot.WebhookSecret != nil && secretHeader != *bot.WebhookSecret {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    var update WebhookUpdate
    if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    // Handle callback query (button click)
    if update.CallbackQuery != nil {
        h.handleCallbackQuery(bot, update.CallbackQuery)
    }

    // Handle message
    if update.Message != nil {
        h.handleMessage(bot, update.Message)
    }

    w.WriteHeader(http.StatusOK)
}
```

---

## Unread Badge for Users

Show users how many tickets have admin replies they haven't seen:

```go
// Package supportbadge provides middleware for showing unread ticket count.
package supportbadge

import (
    "context"
    "net/http"
    "github.com/jmoiron/sqlx"
)

type contextKey int
const badgeKey contextKey = 1

type Badge struct {
    Count int
}

func (b Badge) HasUnread() bool {
    return b.Count > 0
}

// Middleware attaches support badge count to the request context.
func Middleware(db *sqlx.DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := GetUser(r)  // Your auth context helper
            if user == nil {
                next.ServeHTTP(w, r)
                return
            }

            // Count tickets with status 'answered' (admin replied, user hasn't responded)
            var count int
            db.Get(&count, `
                SELECT COUNT(*) FROM support_tickets
                WHERE user_id = $1 AND status = 'answered'
            `, user.ID)

            badge := Badge{Count: count}
            r = r.WithContext(context.WithValue(r.Context(), badgeKey, badge))
            next.ServeHTTP(w, r)
        })
    }
}

func FromRequest(r *http.Request) Badge {
    if badge, ok := r.Context().Value(badgeKey).(Badge); ok {
        return badge
    }
    return Badge{}
}
```

Use in templates:
```html
<a href="/user/support">
    Support
    {{if .SupportBadge.HasUnread}}
        <span class="badge bg-danger">{{.SupportBadge.Count}}</span>
    {{end}}
</a>
```

---

## Wiring It All Together

In your main application setup:

```go
func main() {
    db := connectDB()
    
    // Create modules
    supportModule := support.New()
    telegramModule := telegram.New()
    
    // Initialize
    supportModule.Init(db)
    telegramModule.Init(db)
    
    // Connect: support tickets notify telegram
    supportModule.SetNotifier(telegramModule.Handler)
    
    // Register middleware
    r.Use(supportbadge.Middleware(db))
    
    // Mount routes
    r.Mount("/user/support", authMiddleware(supportModule.Routes()))
    r.Mount("/admin/support", adminMiddleware(supportModule.AdminRoutes()))
    r.Mount("/telegram", telegramModule.Routes())
}
```

---

## Key Design Decisions

1. **Decoupled via Interface**: Support module doesn't import Telegram. It calls `Notifier` interface methods. This allows swapping notification backends (email, Slack, etc.).

2. **Pending Reply State**: When admin clicks "Reply", we store a pending reply row. The next message from that Telegram user is captured as the reply. Expires after 5 minutes.

3. **Status Flow**: `open` → `answered` (admin replied) → `open` (user replied). This drives the unread badge and makes it clear who needs to respond.

4. **Telegram Bot ID Tracking**: We store which bot sent a reply in `ticket_messages.telegram_bot_id`. This prevents notification loops when multiple bots are configured.

5. **Inline Keyboard**: Reply/Close buttons are inline keyboards, not custom keyboards. They attach to the notification message itself.

6. **ForceReply**: After clicking Reply, we send a message with `force_reply: true`. This opens the reply UI in Telegram, guiding the admin to respond.

---

## Pseudo-code Summary for AI Agents

```
MODULE: Support Tickets
  TABLES: support_tickets, ticket_messages
  STATUSES: open, answered, closed
  
  ON user_creates_ticket:
    INSERT ticket with status='open'
    INSERT first message
    CALL notifier.NotifyNewTicket()
  
  ON user_replies:
    INSERT message with is_admin=false
    UPDATE ticket status='open'
    CALL notifier.NotifyTicketReply()
  
  ON admin_replies (web):
    INSERT message with is_admin=true
    UPDATE ticket status='answered'

MODULE: Telegram Bot
  TABLES: telegram_bots, telegram_pending_replies
  
  IMPLEMENTS Notifier:
    NotifyNewTicket(): send message with Reply/Close buttons
    NotifyTicketReply(): send message with Reply/Close buttons
  
  ON webhook_callback_query (button click):
    PARSE action:ticket_id from callback_data
    IF action == "reply":
      CREATE pending_reply row
      SEND "type your reply" with force_reply
    IF action == "close":
      UPDATE ticket status='closed'
      REMOVE buttons from message
  
  ON webhook_message:
    IF /cancel:
      DELETE pending_reply
      RETURN
    LOOKUP pending_reply for this user
    IF found:
      INSERT ticket_message with is_admin=true, telegram_bot_id=bot.id
      UPDATE ticket status='answered'
      DELETE pending_reply
      SEND confirmation

MODULE: Support Badge
  MIDDLEWARE that counts tickets where status='answered' for current user
  PASS count to templates for display
```

---

## Testing Checklist

1. [ ] User can create ticket → appears in admin list
2. [ ] Admin receives Telegram notification with Reply/Close buttons
3. [ ] Admin clicks Reply → receives prompt
4. [ ] Admin sends reply text → ticket updated, user sees reply
5. [ ] Admin clicks Close → ticket closed, buttons removed
6. [ ] User replies → status changes to open, admin notified
7. [ ] /cancel works during pending reply
8. [ ] Badge shows correct unread count
9. [ ] Multiple bots: reply from one doesn't notify that same bot
10. [ ] Pending replies expire after 5 minutes

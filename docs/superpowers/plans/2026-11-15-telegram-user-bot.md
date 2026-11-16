# Telegram User Bot Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow each user to connect their own Telegram bot (created via @BotFather) to manage links, view stats, and receive notifications — with an app-like experience that edits messages in place and keeps the chat clean.

**Architecture:** Each user creates a bot via @BotFather and enters the token in Settings. GuardBot encrypts the token, sets a webhook pointing to `/api/telegram/user/:user_id/:secret`, and handles incoming commands. The bot uses `editMessageText` and `deleteMessage` to update UI in place rather than flooding the chat with new messages. Inline keyboards with styled buttons provide the primary navigation.

**Tech Stack:** Go, Telegram Bot API (editMessageText, deleteMessage, answerCallbackQuery, InlineKeyboardMarkup with styles), AES-256-GCM encryption (existing `pkg/crypto`), existing redirect/analytics services

**Spec:** Brainstorming conversation in session (no separate spec file)

## Global Constraints

- Bot tokens encrypted with `pkg/crypto.Encrypt()` before storage
- Webhook secrets are 32-byte hex strings generated with `crypto/rand`
- All Telegram API calls use existing `modules/telegram/service` patterns
- Follow existing module structure: handlers, services, migrations, templates
- Use existing `RedirectLinkService.Create()` for link creation
- Use existing `AnalyticsService.GetUserStats()` for stats
- AdminLTE v4.9.1 components for settings UI
- **App-like UX:** Always `editMessageText` to update existing message instead of sending new ones. Delete user's command messages to keep chat clean. Use `answerCallbackQuery` immediately on every button press. Use button styles: `primary`, `success`, `danger`
- **Settings URL pattern:** All settings pages use `?page=` query param, not hash fragments (e.g., `/user/settings?page=telegram`)

## Review Focus

1. **Invalid bot token format** — User enters malformed token; should fail gracefully with clear error before attempting Telegram API call
2. **Webhook secret collision** — Two users with same secret; secrets must be unique per user
3. **Unauthorized chat messages** — Someone other than the linked user messages the bot; should be ignored or rejected
4. **Concurrent bot operations** — User spams connect/disconnect; should handle race conditions
5. **URL parsing edge cases** — Forwarded message with no URL, multiple URLs, or malformed URL; should provide clear feedback

---

### Task 1: Database Migration — Add Telegram Fields to Users

**Files:**
- Create: `modules/auth/migrations/004_telegram_user_bot.sql`

**Interfaces:**
- Consumes: Nothing
- Produces: Database columns `telegram_bot_token_encrypted`, `telegram_bot_username`, `telegram_chat_id`, `telegram_webhook_secret`, `telegram_connected_at`, `telegram_last_message_id`, `telegram_notify_bot_attacks`, `telegram_notify_traffic_milestones`, `telegram_notify_ssl_expiring`, `telegram_notify_balance_low`, `telegram_traffic_threshold`

- [ ] **Step 1: Create migration file**

```sql
-- modules/auth/migrations/004_telegram_user_bot.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_bot_token_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_bot_username TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_chat_id BIGINT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_webhook_secret TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_connected_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_last_message_id BIGINT;

-- Notification preferences (all default to true except traffic which needs threshold)
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_bot_attacks BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_traffic_milestones BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_ssl_expiring BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_balance_low BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_traffic_threshold INTEGER DEFAULT 100;

-- Index for webhook lookups
CREATE INDEX IF NOT EXISTS idx_users_telegram_webhook_secret ON users(telegram_webhook_secret) WHERE telegram_webhook_secret IS NOT NULL;
```

- [ ] **Step 2: Verify migration applies**

Run: `go run ./cmd/server` (migrations run on startup)
Expected: Server starts without migration errors, new columns exist in users table

- [ ] **Step 3: Commit**

```bash
git add modules/auth/migrations/004_telegram_user_bot.sql
git commit -m "feat(telegram): add user bot fields to users table"
```

---

### Task 2: User Model — Add Telegram Fields

**Files:**
- Modify: `modules/auth/models/user.go`

**Interfaces:**
- Consumes: Migration from Task 1
- Produces: `User` struct with fields `TelegramBotTokenEncrypted *string`, `TelegramBotUsername *string`, `TelegramChatID *int64`, `TelegramWebhookSecret *string`, `TelegramConnectedAt *time.Time`, `TelegramLastMessageID *int64`, `TelegramNotifyBotAttacks bool`, `TelegramNotifyTrafficMilestones bool`, `TelegramNotifySSLExpiring bool`, `TelegramNotifyBalanceLow bool`, `TelegramTrafficThreshold int`

- [ ] **Step 1: Add Telegram fields to User struct**

Add after the GoDaddy integration block in `modules/auth/models/user.go`:

```go
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
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles without errors

- [ ] **Step 3: Commit**

```bash
git add modules/auth/models/user.go
git commit -m "feat(telegram): add user bot fields to User struct"
```

---

### Task 3: User Bot Service — Core Operations with App-Like Messaging

**Files:**
- Create: `modules/telegram/services/user_bot_service.go`

**Interfaces:**
- Consumes: `pkg/crypto.Encrypt()`, `pkg/crypto.Decrypt()`, existing `service.Service` for Telegram API calls
- Produces: `UserBotService` with methods:
  - `Connect(userID, botToken string) (*ConnectResult, error)` — Validates token, sets webhook, returns bot username
  - `Disconnect(userID string) error` — Removes webhook, clears user fields
  - `GetBotToken(userID string) (string, error)` — Decrypts and returns token
  - `GetUserByWebhookSecret(secret string) (*models.User, error)` — Looks up user for incoming webhook
  - `SendOrEditMessage(userID string, text string, replyMarkup interface{}) (int64, error)` — Sends new message or edits last one, returns message ID
  - `EditMessage(userID string, messageID int64, text string, replyMarkup interface{}) error` — Edits specific message
  - `DeleteMessage(userID string, messageID int64) error` — Deletes a message
  - `DeleteMessages(userID string, messageIDs []int64) error` — Deletes multiple messages (up to 100)
  - `SaveLastMessageID(userID string, messageID int64) error` — Stores last bot message ID for editing
  - `AnswerCallback(token, callbackID, text string, showAlert bool) error` — Answers callback query

- [ ] **Step 1: Create user bot service file**

Create `modules/telegram/services/user_bot_service.go`:

```go
package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/botginx/botginx/modules/auth/models"
	"github.com/botginx/botginx/modules/telegram/service"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/jmoiron/sqlx"
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
	return &UserBotService{db: db, tgSvc: tgSvc, baseURL: baseURL}
}
```

- [ ] **Step 2: Implement `Connect(userID, botToken string) (*ConnectResult, error)`**

Validates token via getMe, generates webhook secret, encrypts token, sets webhook, updates user record including bot_username. Returns bot username and webhook URL.

- [ ] **Step 3: Implement `Disconnect(userID string) error`**

Decrypts token, calls deleteWebhook, clears all telegram_* fields on user.

- [ ] **Step 4: Implement `GetBotToken(userID string) (string, error)`**

Fetches encrypted token from DB, decrypts with `crypto.Decrypt()`.

- [ ] **Step 5: Implement `GetUserByWebhookSecret(secret string) (*models.User, error)`**

Query: `SELECT * FROM users WHERE telegram_webhook_secret = $1`

- [ ] **Step 6: Implement `SendOrEditMessage(userID string, text string, replyMarkup interface{}) (int64, error)`**

If user has `telegram_last_message_id`, call `editMessageText`. Otherwise call `sendMessage`. Returns message ID. Updates `telegram_last_message_id`.

- [ ] **Step 7: Implement `EditMessage(userID string, messageID int64, text string, replyMarkup interface{}) error`**

Direct wrapper around Telegram `editMessageText` API. Uses chat_id from user record.

- [ ] **Step 8: Implement `DeleteMessage(userID string, messageID int64) error`**

Wrapper around Telegram `deleteMessage` API.

- [ ] **Step 9: Implement `DeleteMessages(userID string, messageIDs []int64) error`**

Wrapper around Telegram `deleteMessages` API (batch delete up to 100).

- [ ] **Step 10: Implement `SaveLastMessageID(userID string, messageID int64) error`**

Update: `UPDATE users SET telegram_last_message_id = $1 WHERE id = $2`

- [ ] **Step 11: Implement `AnswerCallback(token, callbackID, text string, showAlert bool) error`**

Wrapper around `answerCallbackQuery` with text and show_alert parameters.

- [ ] **Step 12: Verify compilation**

Run: `go build ./...`
Expected: Compiles without errors

- [ ] **Step 13: Commit**

```bash
git add modules/telegram/services/user_bot_service.go
git commit -m "feat(telegram): add user bot service with app-like message management"
```

---

### Task 4: User Webhook Handler — App-Like Command Processing

**Files:**
- Create: `modules/telegram/handlers/user_bot_handler.go`

**Interfaces:**
- Consumes: `UserBotService`, `RedirectLinkService`, `AnalyticsService`, `DomainService`
- Produces: HTTP handler `UserBotWebhook(w, r)` at `POST /api/telegram/user/:user_id/:secret`

**App-like UX principles:**
- Delete user's command message immediately after processing
- Edit last bot message instead of sending new one (keeps chat with single "app screen")
- Use styled inline keyboards for all navigation
- Always answer callback queries immediately
- Show typing indicator for operations taking >1 second

- [ ] **Step 1: Create handler file with struct and constants**

Create `modules/telegram/handlers/user_bot_handler.go` with `UserBotHandler` struct and screen constants:

```go
const (
	ScreenMain     = "main"
	ScreenStats    = "stats"
	ScreenLinks    = "links"
	ScreenNewLink  = "newlink"
	ScreenSettings = "settings"
	ScreenBalance  = "balance"
	ScreenHelp     = "help"
)
```

- [ ] **Step 2: Implement webhook authentication**

Extract `user_id` and `secret` from URL params. Call `userBotSvc.GetUserByWebhookSecret(secret)`. Verify user ID matches. Return 401 if mismatch.

- [ ] **Step 3: Implement message cleanup**

After receiving any text message (command or URL), immediately call `DeleteMessage()` to remove user's message. This keeps chat clean with only bot's "app screen".

- [ ] **Step 4: Implement screen renderer `renderScreen(user, screen string, data interface{}) (text string, keyboard InlineKeyboardMarkup)`**

Central function that renders each screen with formatted text and inline keyboard. All screens include a "back to main" button.

- [ ] **Step 5: Implement main menu screen**

Shows welcome text and navigation buttons:
```
📊 Stats | 🔗 Links | ➕ New Link
⚙️ Settings | 💰 Balance | ❓ Help
```
Use `style: "primary"` for Stats and Links, default for others.

- [ ] **Step 6: Implement `/start` command**

If `telegram_chat_id` is NULL, save the chat ID. Send main menu screen via `SendOrEditMessage()`.

- [ ] **Step 7: Implement stats screen**

Call `analyticsService.GetUserStats(userID)`. Format with:
- Today's clicks (with trend arrow if available)
- Total bots blocked
- Humans verified
- Active links count
Back button returns to main.

- [ ] **Step 8: Implement links screen with pagination**

Call `redirectLinkService.List(userID)`. Show 5 links per page with:
- Link URL (truncated)
- Click count badge
- Inline buttons: ◀️ Prev | Page X/Y | Next ▶️
Each link is clickable to show details.

- [ ] **Step 9: Implement link detail screen**

Shows full link info:
- Full URL (monospace)
- Destination URL
- Total clicks, bots blocked
- Created date
Buttons: 📊 Stats | 📋 Copy | 🗑️ Delete | ⬅️ Back

- [ ] **Step 10: Implement new link screen**

If no URL provided, show prompt: "Send me a URL to protect"
If URL provided via `/newlink <url>` or plain URL message:
- Get user's first domain
- Create link via `redirectLinkService.Create()`
- Show success screen with protected URL and Copy button

- [ ] **Step 11: Implement balance screen**

Fetch user balance. Format with currency symbol. Show "Add Funds" button linking to dashboard.

- [ ] **Step 12: Implement help screen**

Show all commands and features. Button to open full docs in browser.

- [ ] **Step 13: Implement settings screen (calls to Task 5)**

Delegate to settings handler. Shows notification toggles as inline keyboard.

- [ ] **Step 14: Implement command router**

Parse incoming update. Route based on message text:
- `/start` → main menu
- `/stats` → stats screen
- `/links` → links screen
- `/newlink` or `/newlink <url>` → new link screen
- `/balance` → balance screen
- `/settings` → settings screen
- `/help` → help screen
- Plain URL → new link with that URL
- Any other text → main menu with "Unknown command" toast via `answerCallbackQuery` equivalent

- [ ] **Step 15: Verify compilation**

Run: `go build ./...`
Expected: Compiles without errors

- [ ] **Step 16: Commit**

```bash
git add modules/telegram/handlers/user_bot_handler.go
git commit -m "feat(telegram): add app-like webhook handler with screen navigation"
```

---

### Task 5: Settings Screen — Advanced Inline Keyboard UI

**Files:**
- Modify: `modules/telegram/handlers/user_bot_handler.go`

**Interfaces:**
- Consumes: Callback queries from Telegram inline keyboards
- Produces: Full settings management via inline keyboard with instant visual feedback

**Advanced UX features:**
- Toggle buttons show current state with checkmark (✅ Enabled / ❌ Disabled)
- Use button styles: `success` for enabled toggles, `danger` for destructive actions
- Instantly edit message to reflect new state (no loading, no new messages)
- Threshold selection via expandable sub-menu
- All callbacks answered immediately with brief confirmation

- [ ] **Step 1: Implement settings screen renderer**

Render settings as inline keyboard:
```
🔔 Notifications
────────────────
✅ Bot attack alerts     [style: success]
❌ Traffic milestones    [style: default]
✅ SSL expiring          [style: success]
✅ Low balance           [style: success]

📊 Traffic threshold: 100
[50] [100] [500] [1000]

⬅️ Back to Menu
```

- [ ] **Step 2: Add callback query routing**

In the webhook handler, check for `callback_query` in the update. Parse callback data format: `action:param` (e.g., `toggle:bot_attacks`, `threshold:500`, `nav:main`).

- [ ] **Step 3: Implement notification toggle callbacks**

Callback data: `toggle:bot_attacks`, `toggle:traffic`, `toggle:ssl`, `toggle:balance`

On callback:
1. Immediately call `AnswerCallback()` with "✅ Updated"
2. Toggle the corresponding `telegram_notify_*` field in DB
3. Call `EditMessage()` to refresh settings screen with new state

- [ ] **Step 4: Implement traffic threshold callbacks**

Callback data: `threshold:50`, `threshold:100`, `threshold:500`, `threshold:1000`

On callback:
1. Answer with "Threshold set to X"
2. Update `telegram_traffic_threshold` in DB
3. Edit message to show selected threshold highlighted

- [ ] **Step 5: Implement navigation callbacks**

Callback data: `nav:main`, `nav:stats`, `nav:links`, etc.

On callback:
1. Answer callback (empty text for instant response)
2. Render target screen
3. Edit message with new screen content

- [ ] **Step 6: Implement screen-specific callbacks**

Handle callbacks from other screens:
- `link:detail:<id>` — Show link detail
- `link:delete:<id>` — Confirm and delete link
- `link:copy:<id>` — Copy link to clipboard (show as alert)
- `links:page:<n>` — Navigate to page N
- `links:back` — Return to links list

- [ ] **Step 7: Add callback answer patterns**

```go
// Immediate feedback for all callbacks
func (h *UserBotHandler) handleCallback(user *models.User, query *CallbackQuery) {
    // Always answer callback first (prevents loading spinner)
    h.userBotSvc.AnswerCallback(token, query.ID, "", false)
    
    // Then process and edit message
    // ...
}
```

- [ ] **Step 8: Verify compilation**

Run: `go build ./...`
Expected: Compiles without errors

- [ ] **Step 9: Commit**

```bash
git add modules/telegram/handlers/user_bot_handler.go
git commit -m "feat(telegram): add advanced settings UI with instant toggle feedback"
```

---

### Task 6: Settings UI Refactor — Query Param Navigation

**Files:**
- Modify: `modules/auth/templates/settings.html`
- Modify: `modules/auth/handlers/handler.go`
- Modify: `pkg/i18n/locales/en.json`, `pkg/i18n/locales/ru.json`

**Interfaces:**
- Consumes: `UserBotService.Connect()`, `UserBotService.Disconnect()`
- Produces: Settings page with `?page=` query param navigation and Telegram section

**URL pattern change:** All settings pages now use `/user/settings?page=profile`, `/user/settings?page=security`, `/user/settings?page=telegram`, etc. Hash-based navigation is removed.

- [ ] **Step 1: Update settings handler to read `page` query param**

In `modules/auth/handlers/handler.go`, modify settings handler:
```go
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
    page := r.URL.Query().Get("page")
    if page == "" {
        page = "profile" // default
    }
    // Pass activePage to template
}
```

- [ ] **Step 2: Update settings nav to use query params**

Change `settings.html` nav links from `href="#profile"` to `href="?page=profile"`:

```html
<div class="list-group list-group-flush nav nav-pills flex-column" id="settings-nav" role="tablist">
    <a href="?page=profile" class="list-group-item list-group-item-action {{if eq .ActivePage "profile"}}active{{end}}">
        <i class="bi bi-person me-2"></i>Profile
    </a>
    <a href="?page=security" class="list-group-item list-group-item-action {{if eq .ActivePage "security"}}active{{end}}">
        <i class="bi bi-shield-lock me-2"></i>Security
    </a>
    <a href="?page=account" class="list-group-item list-group-item-action {{if eq .ActivePage "account"}}active{{end}}">
        <i class="bi bi-info-circle me-2"></i>Account
    </a>
    {{if .Subscription.Active}}
    <a href="?page=whitelist" class="list-group-item list-group-item-action {{if eq .ActivePage "whitelist"}}active{{end}}">
        <i class="bi bi-shield-check me-2"></i>Whitelisted IPs
    </a>
    {{end}}
    <a href="?page=telegram" class="list-group-item list-group-item-action {{if eq .ActivePage "telegram"}}active{{end}}">
        <i class="bi bi-telegram me-2"></i>Telegram
    </a>
</div>
```

- [ ] **Step 3: Convert tab panes to conditional rendering**

Replace Bootstrap tabs with Go template conditionals:
```html
{{if eq .ActivePage "profile"}}
<!-- Profile content -->
{{end}}

{{if eq .ActivePage "security"}}
<!-- Security content -->
{{end}}

{{if eq .ActivePage "telegram"}}
<!-- Telegram content -->
{{end}}
```

- [ ] **Step 4: Create Telegram settings section**

Add Telegram section content:
- Connection status card (connected/not connected)
- If not connected: instructions to create bot via @BotFather, input for bot token, connect button
- If connected: bot username display (@username), disconnect button
- Notification preferences toggles (mirror the Telegram bot settings)
- Link to open bot in Telegram

- [ ] **Step 5: Add connect/disconnect API endpoints**

In auth handlers, add:
- `POST /api/user/telegram/connect` — Accepts `{botToken}`, calls `UserBotService.Connect()`
- `POST /api/user/telegram/disconnect` — Calls `UserBotService.Disconnect()`
- `PUT /api/user/telegram/notifications` — Updates notification preferences

- [ ] **Step 6: Add JavaScript for form handling**

Handle connect form submission, disconnect button, notification toggles with fetch API and toast notifications.

- [ ] **Step 7: Add translations**

Add to `en.json` and `ru.json`:
- `telegram.title`, `telegram.connect`, `telegram.disconnect`, `telegram.bot_token`, `telegram.bot_token_hint`
- `telegram.connected_as`, `telegram.not_connected`
- `telegram.create_bot_instructions`
- `telegram.notifications`, `telegram.notify_bot_attacks`, `telegram.notify_traffic`, `telegram.notify_ssl`, `telegram.notify_balance`
- `telegram.traffic_threshold`, `telegram.open_bot`

- [ ] **Step 8: Test in browser**

Start server, navigate to `/user/settings?page=telegram`. Verify:
- URL shows `?page=telegram`
- Telegram nav item is active
- Connect form appears when not connected
- Connect with valid token shows success and bot username
- Disconnect works
- Notification toggles save
- Navigating between pages updates URL

- [ ] **Step 9: Commit**

```bash
git add modules/auth/templates/settings.html modules/auth/handlers/handler.go pkg/i18n/locales/*.json
git commit -m "feat(settings): refactor to query param navigation, add Telegram section"
```

---

### Task 7: Module Registration — Wire Routes

**Files:**
- Modify: `modules/telegram/module.go`

**Interfaces:**
- Consumes: `UserBotHandler`, `UserBotService`
- Produces: Registered route `POST /api/telegram/user/{userID}/{secret}`

- [ ] **Step 1: Initialize UserBotService in module**

In `Module.Init()`, create `UserBotService` with DB, existing telegram service, and base URL from env.

- [ ] **Step 2: Initialize UserBotHandler**

Create handler with all required service dependencies (UserBotService, RedirectLinkService, AnalyticsService, DomainService).

- [ ] **Step 3: Register webhook route**

In `Module.Routes()`, add:
```go
r.Post("/api/telegram/user/{userID}/{secret}", userBotHandler.Webhook)
```

- [ ] **Step 4: Verify routes registered**

Run: `go run ./cmd/server`
Expected: Server starts, no errors

- [ ] **Step 5: Commit**

```bash
git add modules/telegram/module.go
git commit -m "feat(telegram): register user bot webhook route"
```

---

### Task 8: Notification Service — Event-Driven Alerts

**Files:**
- Create: `modules/telegram/services/notification_service.go`
- Modify: `modules/analytics/services/analytics_service.go` (add hook)

**Interfaces:**
- Consumes: `UserBotService.SendOrEditMessage()`, user notification preferences
- Produces: `NotificationService` with methods:
  - `NotifyBotAttack(userID, linkID string, count int) error`
  - `NotifyTrafficMilestone(userID, linkID string, visits int) error`
  - `NotifySSLExpiring(userID, domainName string, daysLeft int) error`
  - `NotifyBalanceLow(userID string, balance float64) error`

**Note:** Notifications are sent as new messages (not editing the app screen) since they're async events. Include inline button to view details.

- [ ] **Step 1: Create notification service**

Create `modules/telegram/services/notification_service.go` with `NotificationService` struct.

- [ ] **Step 2: Implement `NotifyBotAttack()`**

Check if user has `telegram_notify_bot_attacks` enabled and is connected. Format message:
```
🚨 Bot Attack Detected

Link: example.com/offer
Blocked: 47 bots in last hour

[View Details] [Dismiss]
```
Send as new message (don't edit app screen).

- [ ] **Step 3: Implement `NotifyTrafficMilestone()`**

Check if enabled and visits >= threshold. Format:
```
🎉 Traffic Milestone!

Your link hit 1,000 visits!
example.com/offer

[View Stats]
```

- [ ] **Step 4: Implement `NotifySSLExpiring()`**

Check if enabled. Format:
```
⚠️ SSL Certificate Expiring

Domain: example.com
Expires: 7 days

[Renew Now] [Settings]
```

- [ ] **Step 5: Implement `NotifyBalanceLow()`**

Check if enabled and balance < threshold ($5). Format:
```
💰 Low Balance Warning

Your balance is $3.50
Add funds to continue protection.

[Add Funds]
```

- [ ] **Step 6: Hook into analytics**

In `RecordVisit()`, after recording a bot visit, check if threshold reached and call `NotifyBotAttack()`. Use rate limiting (max 1 notification per link per hour).

- [ ] **Step 7: Verify compilation**

Run: `go build ./...`
Expected: Compiles without errors

- [ ] **Step 8: Commit**

```bash
git add modules/telegram/services/notification_service.go modules/analytics/services/analytics_service.go
git commit -m "feat(telegram): add notification service for user alerts"
```

---

### Task 9: Update Documentation

**Files:**
- Modify: `docs-site/src/content/docs/integrations/telegram.md`

**Interfaces:**
- Consumes: Implemented features
- Produces: Accurate documentation matching actual functionality

- [ ] **Step 1: Rewrite Connect Your Account section**

Document that users create their own bot via @BotFather:
1. Open @BotFather in Telegram
2. Send `/newbot` and follow prompts
3. Copy the bot token
4. Go to Settings → Telegram in GuardBot
5. Paste token and click Connect
6. Open chat with your new bot and send `/start`

- [ ] **Step 2: Update Available Commands section**

Document the app-like experience:
- Commands show as screens, not message floods
- Navigation via inline buttons
- List all commands: `/start`, `/stats`, `/links`, `/newlink`, `/balance`, `/settings`, `/help`

- [ ] **Step 3: Add Quick Links section**

Document that pasting/forwarding a URL creates a protected link instantly.

- [ ] **Step 4: Update Configure Notifications section**

Document both methods:
- In Telegram: use `/settings` command
- In Dashboard: go to Settings → Telegram

- [ ] **Step 5: Update Troubleshooting section**

Add:
- Invalid token: check you copied the full token from @BotFather
- Bot not responding: make sure you sent `/start` after connecting
- Commands not working: verify bot is connected in Settings

- [ ] **Step 6: Commit**

```bash
git add docs-site/src/content/docs/integrations/telegram.md
git commit -m "docs(telegram): update integration guide to match implementation"
```

---

### Task 10: Dashboard Menu Item — Account Section

**Files:**
- Modify: `modules/dashboard/templates/index.html`
- Create: `web/static/icons/guardbot/telegram-bot.svg`
- Modify: `pkg/i18n/locales/en.json`, `pkg/i18n/locales/ru.json`

**Interfaces:**
- Consumes: Settings route `/user/settings?page=telegram`
- Produces: Dashboard menu item "Connect Telegram" in Account section with icon

- [ ] **Step 1: Create Telegram icon**

Create `web/static/icons/guardbot/telegram-bot.svg` — Telegram paper plane icon with subtle bot indicator.

- [ ] **Step 2: Add menu item to Account section**

In `modules/dashboard/templates/index.html`, find the Account tools section and add:
```javascript
{ label: 'Connect Telegram', icon: 'gb:telegram-bot', href: '/user/settings?page=telegram' },
```

- [ ] **Step 3: Add translation mapping**

Add `'Connect Telegram': '{{t "menu.connect_telegram"}}'` to the translation map.

- [ ] **Step 4: Add translations**

Add to `en.json`:
```json
"menu.connect_telegram": "Connect Telegram"
```

Add to `ru.json`:
```json
"menu.connect_telegram": "Подключить Telegram"
```

- [ ] **Step 5: Verify in browser**

Start server, check dashboard shows "Connect Telegram" in Account section. Click should navigate to `/user/settings?page=telegram`.

- [ ] **Step 6: Commit**

```bash
git add modules/dashboard/templates/index.html web/static/icons/guardbot/telegram-bot.svg pkg/i18n/locales/*.json
git commit -m "feat(telegram): add Connect Telegram to dashboard Account section"
```

---

### Task 11: Telegram Service Extensions — Edit and Delete Methods

**Files:**
- Modify: `modules/telegram/service/service.go`

**Interfaces:**
- Consumes: Telegram Bot API
- Produces: New methods on existing `Service`:
  - `EditMessageText(token string, chatID string, messageID int64, text string, replyMarkup interface{}) error`
  - `DeleteMessage(token string, chatID string, messageID int64) error`
  - `DeleteMessages(token string, chatID string, messageIDs []int64) error`

- [ ] **Step 1: Implement `EditMessageText()`**

Add to `service.go`:
```go
func (s *Service) EditMessageText(token string, chatID string, messageID int64, text string, replyMarkup interface{}) (*TelegramMessage, error) {
    payload := map[string]interface{}{
        "chat_id":    chatID,
        "message_id": messageID,
        "text":       text,
        "parse_mode": "HTML",
    }
    if replyMarkup != nil {
        payload["reply_markup"] = replyMarkup
    }
    // ... API call
}
```

- [ ] **Step 2: Implement `DeleteMessage()`**

Add method that calls Telegram `deleteMessage` API.

- [ ] **Step 3: Implement `DeleteMessages()`**

Add method that calls Telegram `deleteMessages` API for batch deletion (up to 100 messages).

- [ ] **Step 4: Verify compilation**

Run: `go build ./...`
Expected: Compiles without errors

- [ ] **Step 5: Commit**

```bash
git add modules/telegram/service/service.go
git commit -m "feat(telegram): add editMessageText and deleteMessage methods"
```

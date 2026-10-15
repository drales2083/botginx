# Telegram Bot Integration Plan

## Overview

Enable admins to manage support tickets directly from Telegram using inline buttons and replies.

## Current State

- Hardcoded bot tokens in `deploy.sh` and `/opt/botginx/.env`
- One-way notifications only (panel → Telegram)
- No ability to reply from Telegram

## Target State

- Bot settings stored in database, configurable via admin panel
- Two-way communication: reply to tickets from Telegram
- Inline buttons for quick actions (Reply, Close, View)
- Duplicate prevention across multiple bots/channels

---

## Database Schema

### New Tables

```sql
-- Telegram bot configurations (replaces hardcoded env vars)
CREATE TABLE telegram_bots (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,                    -- "Support", "Deploy", etc.
    bot_token TEXT NOT NULL,
    bot_username TEXT,                     -- fetched from getMe API
    chat_id TEXT,                          -- target chat/group for notifications
    webhook_secret TEXT,                   -- random secret for webhook URL
    enabled BOOLEAN DEFAULT TRUE,
    use_for_support BOOLEAN DEFAULT FALSE, -- receives ticket notifications
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Pending replies (admin clicked Reply button, waiting for their message)
CREATE TABLE telegram_pending_replies (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id TEXT REFERENCES telegram_bots(id) ON DELETE CASCADE,
    telegram_user_id BIGINT NOT NULL,      -- Telegram user who clicked Reply
    telegram_chat_id BIGINT NOT NULL,      -- Chat where interaction happened
    ticket_id TEXT REFERENCES support_tickets(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP DEFAULT NOW() + INTERVAL '5 minutes'
);

-- Index for fast lookup
CREATE INDEX idx_pending_replies_user ON telegram_pending_replies(telegram_user_id, bot_id);
```

### Schema Modifications

```sql
-- Track Telegram message IDs to prevent duplicate processing
ALTER TABLE ticket_messages ADD COLUMN telegram_message_id BIGINT;
ALTER TABLE ticket_messages ADD COLUMN telegram_bot_id TEXT;

-- Track notification status per ticket
ALTER TABLE support_tickets ADD COLUMN telegram_notified_at TIMESTAMP;
ALTER TABLE support_tickets ADD COLUMN telegram_message_id BIGINT;
ALTER TABLE support_tickets ADD COLUMN telegram_bot_id TEXT;
```

---

## User Flows

### Flow 1: New Ticket Notification

```
User creates ticket in panel
    │
    ▼
System sends Telegram notification to all enabled support bots:
    ┌─────────────────────────────────────┐
    │ 🎫 New Support Ticket               │
    │                                     │
    │ From: user@example.com              │
    │ Category: Domain                    │
    │ Subject: SSL not working            │
    │                                     │
    │ My domain example.com shows...      │
    │                                     │
    │ [💬 Reply] [✅ Close] [🔗 View]     │
    └─────────────────────────────────────┘
    │
    ▼
Store message_id in support_tickets table
```

### Flow 2: Admin Replies via Telegram

```
Admin clicks [💬 Reply] button
    │
    ▼
Bot receives callback_query: "reply:TICKET_ID"
    │
    ▼
Bot sends message:
    "📝 Reply to: SSL not working
     
     Send your message (or /cancel):"
    │
    ▼
Create pending_reply record:
    {bot_id, telegram_user_id, ticket_id, expires_at}
    │
    ▼
Admin types reply message
    │
    ▼
Bot receives message, checks pending_replies
    │
    ▼
Match found → Save reply to ticket_messages (is_admin=TRUE)
    │
    ▼
Bot sends confirmation:
    "✅ Reply sent to ticket #ABC123"
    │
    ▼
Delete pending_reply record
    │
    ▼
Update original ticket message to show "Replied ✓"
    │
    ▼
Notify OTHER bots (if any) that ticket was replied
```

### Flow 3: Admin Closes Ticket

```
Admin clicks [✅ Close] button
    │
    ▼
Bot receives callback_query: "close:TICKET_ID"
    │
    ▼
Update ticket status to 'closed'
    │
    ▼
Bot edits original message to show "Closed ✓"
    │
    ▼
Notify other bots that ticket was closed
```

### Flow 4: User Replies to Ticket

```
User replies in panel
    │
    ▼
System sends notification:
    ┌─────────────────────────────────────┐
    │ 💬 Ticket Reply                     │
    │                                     │
    │ From: user@example.com              │
    │ Subject: SSL not working            │
    │                                     │
    │ Thanks, but I still see the...      │
    │                                     │
    │ [💬 Reply] [✅ Close] [🔗 View]     │
    └─────────────────────────────────────┘
```

---

## API Endpoints

### Webhook Endpoint

```
POST /api/telegram/webhook/{bot_id}/{webhook_secret}

Handles:
1. callback_query - Button clicks
   - data: "reply:{ticket_id}"  → Start reply flow
   - data: "close:{ticket_id}"  → Close ticket
   - data: "view:{ticket_id}"   → Send panel link

2. message - Text messages
   - Check telegram_pending_replies for sender
   - If pending: Process as ticket reply
   - If "/cancel": Clear pending reply
```

### Admin API Endpoints

```
GET  /admin/api/telegram/bots          - List all bots
POST /admin/api/telegram/bots          - Add new bot
PUT  /admin/api/telegram/bots/{id}     - Update bot
DELETE /admin/api/telegram/bots/{id}   - Delete bot

POST /admin/api/telegram/bots/{id}/test     - Test bot connection
POST /admin/api/telegram/bots/{id}/webhook  - Setup webhook
GET  /admin/api/telegram/bots/{id}/chats    - Fetch available chats
```

---

## Implementation Phases

### Phase 1: Database & Migration
- [ ] Create telegram_bots table
- [ ] Create telegram_pending_replies table
- [ ] Add columns to ticket_messages
- [ ] Add columns to support_tickets
- [ ] Migrate existing env vars to database (one-time)

### Phase 2: Admin Settings UI
- [ ] Create `/admin/settings/telegram` page
- [ ] List configured bots
- [ ] Add/Edit bot form:
  - Bot token input
  - "Verify" button → calls getMe, fetches bot_username
  - "Fetch Chats" → calls getUpdates, lists available chats
  - Chat ID selector
  - Enable/disable toggle
  - "Use for Support" toggle
- [ ] Delete bot with confirmation
- [ ] Test connection button

### Phase 3: Webhook Handler
- [ ] Create webhook endpoint
- [ ] Handle callback_query (button clicks)
- [ ] Handle message (text replies)
- [ ] Pending reply management (create, lookup, expire, delete)
- [ ] Security: Validate webhook_secret

### Phase 4: Notification Updates
- [ ] Update NotifyNewTicket to use database bots
- [ ] Include inline keyboard buttons
- [ ] Store message_id for later editing
- [ ] Update NotifyTicketReply similarly

### Phase 5: Reply Processing
- [ ] Parse incoming reply
- [ ] Create ticket_message record
- [ ] Update ticket status to 'answered'
- [ ] Edit original Telegram message to show "Replied"
- [ ] Notify other bots

### Phase 6: Cleanup & Polish
- [ ] Expire old pending_replies (cron or on-access)
- [ ] Handle errors gracefully (bot blocked, chat deleted, etc.)
- [ ] Remove hardcoded env vars from deploy.sh
- [ ] Update documentation

---

## File Structure

```
modules/telegram/
├── module.go              # Module definition, migrations
├── handlers/
│   ├── admin.go           # Admin settings handlers
│   ├── webhook.go         # Telegram webhook handler
│   └── handler.go         # Shared utilities
├── templates/
│   └── settings.html      # Admin settings page
├── migrations/
│   └── 001_create_tables.sql
└── service/
    ├── bot.go             # Bot API wrapper
    ├── notify.go          # Send notifications
    └── reply.go           # Process replies
```

---

## Security Considerations

1. **Webhook Secret**: Random string in URL prevents unauthorized calls
2. **Bot Token Storage**: Encrypted in database (use existing HOSTING_ENCRYPTION_KEY)
3. **User Verification**: Only process replies from users in the chat
4. **Rate Limiting**: Prevent spam via pending_reply expiration
5. **Input Sanitization**: Escape HTML in ticket content before sending

---

## Callback Data Format

Button callback_data is limited to 64 bytes. Use compact format:

```
reply:ABC123      → Reply to ticket ABC123
close:ABC123      → Close ticket ABC123
view:ABC123       → View ticket ABC123
```

---

## Message Templates

### New Ticket
```
🎫 <b>New Support Ticket</b>

<b>From:</b> <code>{email}</code>
<b>Category:</b> {category}
<b>Subject:</b> {subject}

{message_preview}
```

### Ticket Reply (from user)
```
💬 <b>Ticket Reply</b>

<b>From:</b> <code>{email}</code>
<b>Re:</b> {subject}

{message_preview}
```

### Reply Prompt
```
📝 <b>Reply to:</b> {subject}

Ticket from <code>{email}</code>

Send your reply message, or /cancel to abort.
```

### Confirmation
```
✅ Reply sent to ticket #{ticket_id}
```

---

## Migration from Env Vars

On first startup after upgrade:
1. Check if telegram_bots table is empty
2. If TG_SUPPORT_BOT_TOKEN env var exists:
   - Create "Support" bot entry
   - Set use_for_support = TRUE
3. If TG_DEPLOY_BOT_TOKEN env var exists:
   - Create "Deploy" bot entry
4. Log migration complete

---

## Timeline Estimate

| Phase | Effort |
|-------|--------|
| Phase 1: Database | 1 hour |
| Phase 2: Admin UI | 2 hours |
| Phase 3: Webhook | 2 hours |
| Phase 4: Notifications | 1 hour |
| Phase 5: Reply Processing | 2 hours |
| Phase 6: Cleanup | 1 hour |
| **Total** | **~9 hours** |

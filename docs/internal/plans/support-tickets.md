# Support Ticket System - Implementation Plan

## Overview

A two-way support ticket system where users can submit tickets and have threaded conversations with admins.

---

## Database Schema

### Table: `support_tickets`

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY KEY | Unique ticket ID (e.g., `ticket_abc123`) |
| user_id | TEXT NOT NULL | References users(id) |
| category | TEXT NOT NULL | Ticket category |
| subject | TEXT NOT NULL | Brief description (5-100 chars) |
| status | TEXT NOT NULL | `open`, `answered`, `closed` |
| created_at | TIMESTAMP | When ticket was created |
| updated_at | TIMESTAMP | Last activity timestamp |

### Table: `ticket_messages`

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY KEY | Unique message ID |
| ticket_id | TEXT NOT NULL | References support_tickets(id) |
| user_id | TEXT | Who sent the message (NULL if system) |
| is_admin | BOOLEAN | TRUE if sent by admin |
| message | TEXT NOT NULL | Message content (10-2000 chars) |
| created_at | TIMESTAMP | When message was sent |

### Indexes

- `idx_tickets_user_id` on support_tickets(user_id)
- `idx_tickets_status` on support_tickets(status)
- `idx_ticket_messages_ticket_id` on ticket_messages(ticket_id)

---

## Categories

| Value | Display Label |
|-------|---------------|
| Domain | Domain related |
| Redirect Link | Redirect link issue |
| Short Link | Short link issue |
| Payment / Invoice | Payment / Invoice issue |
| Subscription | Subscription / Plan |
| Antibot | Antibot protection |
| Account / Login | Account / Login |
| Bug Report | Bug report |
| Other | Other |

---

## Status Flow

```
open → answered → closed
  ↑       ↓
  └───────┘ (user replies reopens to 'open')
```

| Status | Description | Who Sets It |
|--------|-------------|-------------|
| open | New ticket or user replied | System (on create/user reply) |
| answered | Admin has replied | System (on admin reply) |
| closed | Resolved | Admin or User |

---

## Routes

### User Routes (`/user/support`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | /user/support | List | Show user's tickets |
| GET | /user/support/new | NewForm | Show create ticket form |
| POST | /user/support | Create | Submit new ticket |
| GET | /user/support/:id | Show | View ticket conversation |
| POST | /user/support/:id/reply | Reply | User replies to ticket |
| POST | /user/support/:id/close | Close | User closes ticket |

### Admin Routes (`/admin/support`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | /admin/support | List | Show all tickets (filterable) |
| GET | /admin/support/:id | Show | View ticket conversation |
| POST | /admin/support/:id/reply | Reply | Admin replies to ticket |
| POST | /admin/support/:id/close | Close | Admin closes ticket |
| POST | /admin/support/:id/reopen | Reopen | Admin reopens ticket |

---

## Templates

### User Templates

1. **list.html** - User's ticket list
   - Table: Subject, Category, Status, Created, Last Update
   - Status badges: open (warning), answered (info), closed (secondary)
   - "New Ticket" button

2. **new.html** - Create ticket form
   - Category dropdown (required)
   - Subject input (5-100 chars, required)
   - Message textarea (10-2000 chars, required)
   - Submit button

3. **show.html** - View ticket conversation
   - Ticket header: subject, category, status, created date
   - Message thread (chronological)
     - User messages: aligned right, primary color
     - Admin messages: aligned left, secondary color
   - Reply form (if not closed)
   - Close button (if open/answered)

### Admin Templates

1. **list.html** - All tickets list
   - Filters: status, category
   - Table: ID, User, Subject, Category, Status, Created, Last Update
   - Click row to view

2. **show.html** - View ticket conversation
   - Same as user view, plus:
   - User info header (email, subscription status)
   - Close/Reopen buttons
   - Reply form

---

## Sidebar Menu

### User Sidebar
```
Support (icon: bi-headset)
└── links to /user/support
```

### Admin Sidebar
```
Support (icon: bi-headset) [badge: open count]
└── links to /admin/support
```

---

## Module Structure

```
modules/support/
├── module.go           # Module registration
├── handlers/
│   └── handler.go      # All route handlers
├── templates/
│   ├── list.html       # User ticket list
│   ├── new.html        # Create ticket form
│   ├── show.html       # User view ticket
│   ├── admin_list.html # Admin ticket list
│   └── admin_show.html # Admin view ticket
└── migrations/
    └── 001_create_tickets.sql
```

---

## Implementation Order

1. [ ] Database migration
2. [ ] Module registration (module.go)
3. [ ] Handlers (handler.go)
4. [ ] User templates (list, new, show)
5. [ ] Admin templates (admin_list, admin_show)
6. [ ] Add to user sidebar
7. [ ] Add to admin sidebar with badge
8. [ ] Test full flow

---

## Form Validation

### Create Ticket
- Category: required, must be valid option
- Subject: required, 5-100 characters
- Message: required, 10-2000 characters

### Reply
- Message: required, 10-2000 characters

---

## UI Components

### Status Badges
- `open` → `<span class="badge bg-warning">Open</span>`
- `answered` → `<span class="badge bg-info">Answered</span>`
- `closed` → `<span class="badge bg-secondary">Closed</span>`

### Message Bubbles
- User messages: right-aligned, bg-primary text-white
- Admin messages: left-aligned, bg-light border

---

## Future Enhancements (Not in v1)

- Email notifications on reply
- File attachments
- Ticket assignment to specific admins
- Priority levels
- Canned responses for admins
- Search tickets

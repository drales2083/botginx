# Plan: Dashboard Redesign

## Overview

Redesign the user dashboard with KPI cards, news feed, account overview, and subscription alerts. Modern card-based layout with actionable stats.

## Target Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│ [Alert Banner] Subscription expired / expiring soon (conditional)   │
├─────────────────────────────────────────────────────────────────────┤
│ [Get Started CTA] Buy your first domain (conditional, no domains)   │
├─────────────────────────────────────────────────────────────────────┤
│ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐            │
│ │ Domains   │ │ Bots      │ │ Humans    │ │ Sub Days  │            │
│ │ Owned     │ │ Detected  │ │ Verified  │ │ Left      │            │
│ │    5      │ │   1,234 🗑│ │   5,678 🗑│ │    30     │            │
│ └───────────┘ └───────────┘ └───────────┘ └───────────┘            │
├─────────────────────────────────────────┬───────────────────────────┤
│                                         │                           │
│  What's New (news feed)                 │  Top Users (future)       │
│  ┌─────────────────────────────────┐    │  ┌─────────────────────┐  │
│  │ System Update 🔥 [NEW]  Aug 27  │    │  │ # User    Rank Usage│  │
│  │ Dashboard Update...              │    │  │ 1 winx**  LEGEND 4M │  │
│  └─────────────────────────────────┘    │  │ 2 phil**  SUPREME 3M│  │
│  ┌─────────────────────────────────┐    │  └─────────────────────┘  │
│  │ Update              Aug 17      │    │                           │
│  │ Redirect links fixed...         │    │  Account Overview         │
│  └─────────────────────────────────┘    │  ┌─────────────────────┐  │
│  ...scrollable                          │  │ Plan: Standard      │  │
│                                         │  │ Balance: $50.00     │  │
│                                         │  │ Referral: $10.00    │  │
│                                         │  │ [Copy ref link]     │  │
│                                         │  └─────────────────────┘  │
└─────────────────────────────────────────┴───────────────────────────┘
```

## Implementation Phases

### Phase 1: KPI Stats Infrastructure

**Database changes:**
- Add `bots_detected` and `humans_verified` columns to `users` table (or separate `user_stats` table)
- These are running totals that can be reset by the user

**Files to modify:**
- `modules/analytics/services/analytics_service.go` - Add methods to increment bot/human counts
- `modules/analytics/models/` - Add UserStats model if separate table

**Schema:**
```sql
ALTER TABLE users ADD COLUMN bots_detected BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN humans_verified BIGINT DEFAULT 0;
```

### Phase 2: News/Announcements Module

**New module:** `modules/news/`

**Database schema:**
```sql
CREATE TABLE announcements (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL,           -- HTML content
    badge TEXT,                   -- Optional badge like "NEW", "HOT"
    is_latest BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_announcements_published ON announcements(published_at DESC);
```

**Structure:**
```
modules/news/
├── module.go
├── models/
│   └── announcement.go
├── services/
│   └── news_service.go
├── handlers/
│   └── handler.go           -- Admin CRUD
├── templates/
│   ├── admin_list.html      -- Admin: manage announcements
│   └── admin_form.html      -- Admin: create/edit
└── migrations/
    └── 001_create_tables.sql
```

**Features:**
- Admin can create/edit/delete announcements
- Rich text editor for body (or simple HTML)
- Mark one as "latest" (shows NEW badge)
- Auto-sort by published_at DESC

### Phase 3: Dashboard Handler & Template

**New dashboard route:** `/user/dashboard` (or update existing `/user/` home)

**Handler data:**
```go
type DashboardData struct {
    // Subscription alert
    SubscriptionExpired  bool
    SubscriptionExpiring bool      // < 7 days
    DaysLeft             int

    // Get started CTA
    ShowGetStarted bool             // true if 0 domains

    // KPI cards
    DomainsOwned    int
    BotsDetected    int64
    HumansVerified  int64

    // News feed
    Announcements []models.Announcement

    // Account overview
    Plan        string
    Balance     float64
    ReferralURL string
    Earned      float64

    // Future: Top users
    // TopUsers []LeaderboardEntry
}
```

**Files:**
- `modules/dashboard/` - New module or extend existing
- `modules/dashboard/templates/index.html` - Main dashboard template

### Phase 4: Custom CSS

**Add to theme or dashboard template:**

```css
/* KPI Cards */
.ex-kpi-card {
    transition: transform 0.15s, box-shadow 0.15s;
}
.ex-kpi-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}
.ex-kpi-clear {
    background: none;
    border: none;
    color: var(--bs-secondary);
    opacity: 0;
    transition: opacity 0.2s;
    cursor: pointer;
    padding: 4px;
}
.ex-kpi-card:hover .ex-kpi-clear {
    opacity: 0.6;
}
.ex-kpi-clear:hover {
    opacity: 1;
    color: var(--bs-danger);
}

/* News Cards */
.news-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
}
.news-card {
    background: var(--bs-tertiary-bg);
    border-radius: 8px;
    padding: 12px 14px;
    border-left: 3px solid transparent;
}
.news-card.is-latest {
    border-left-color: var(--bs-primary);
}
.news-card-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    font-weight: 600;
    font-size: 13px;
}
.news-card-time {
    font-size: 11px;
    color: var(--bs-secondary);
}
.news-card-body {
    font-size: 12.5px;
    line-height: 1.5;
}
.news-block {
    margin-bottom: 10px;
}
.news-block-head {
    font-weight: 600;
    margin-bottom: 4px;
}
.news-block-list {
    margin: 0;
    padding-left: 18px;
}
.badge.update-new {
    background: var(--bs-primary);
    font-size: 9px;
    padding: 2px 6px;
    vertical-align: middle;
}

/* Avatar for KPI icons */
.avatar-sm {
    width: 40px;
    height: 40px;
}
.avatar-title {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
}
```

### Phase 5: API Endpoints

**KPI reset endpoint:**
```
POST /user/dashboard/api/reset-kpi
Body: { "metric": "bots" | "humans" }
Response: { "ok": true }
```

**Implementation:**
- Reset `bots_detected` or `humans_verified` to 0 for current user
- Return success/error

### Phase 6: Top Users Leaderboard (Future)

**Database query concept:**
```sql
SELECT 
    u.username,
    u.subscription_tier,
    COALESCE(u.bots_detected, 0) + COALESCE(u.humans_verified, 0) AS usage
FROM users u
WHERE u.subscription_tier IS NOT NULL
ORDER BY usage DESC
LIMIT 10;
```

**Rank tiers (based on usage):**
| Usage | Rank | Icon | Color |
|-------|------|------|-------|
| 1M+ | LEGEND | fire | #E74C3C |
| 500K+ | SUPREME | crown | #9B59B6 |
| 100K+ | CHAMPION | trophy | #3498DB |
| 50K+ | ELITE | star-circle | #FFD700 |
| 10K+ | EXPERT | account-star | #C0C0C0 |
| < 10K | MEMBER | account | #6c757d |

**Privacy:**
- Username masked: show first 5 chars + `**`
- Opt-out setting for users who don't want to appear

**Files (when implementing):**
- `modules/dashboard/services/leaderboard.go`
- Add `show_on_leaderboard` column to users (default true)

## Icon Mapping (MDI → Bootstrap Icons)

| MDI | Bootstrap Icon |
|-----|----------------|
| mdi-web | bi-globe |
| mdi-robot-outline | bi-robot |
| mdi-account-check-outline | bi-person-check |
| mdi-calendar-clock | bi-calendar-clock |
| mdi-rocket-outline | bi-rocket |
| mdi-bell-outline | bi-bell |
| mdi-trophy-outline | bi-trophy |
| mdi-shield-check-outline | bi-shield-check |
| mdi-wallet-outline | bi-wallet2 |
| mdi-gift-outline | bi-gift |
| mdi-trash-can-outline | bi-trash |
| mdi-content-copy | bi-clipboard |
| mdi-fire | bi-fire |
| mdi-crown | bi-award |
| mdi-star-circle | bi-star-fill |

## File Summary

| File | Action |
|------|--------|
| `modules/news/` | New module |
| `modules/dashboard/` | New or extend |
| `web/templates/base.html` | Add dashboard CSS (or separate file) |
| `cmd/server/main.go` | Mount news routes (admin) |
| DB migration | Add user stats columns + announcements table |

## Rollout Order

1. **Stats columns** - Add bots_detected/humans_verified to users
2. **News module** - Admin can add announcements
3. **Dashboard template** - New layout with KPI cards + news
4. **KPI reset API** - Clear counters functionality
5. **Account overview** - Subscription/balance/referral display
6. *(Future)* **Leaderboard** - Top users when user base grows

## Testing Checklist

- [ ] KPI cards show correct counts
- [ ] Reset buttons clear counts with confirmation
- [ ] News feed scrolls, latest has badge
- [ ] Subscription alert shows when expired/expiring
- [ ] Get Started CTA hidden when user has domains
- [ ] Mobile responsive (cards stack on small screens)
- [ ] Dark mode styling correct

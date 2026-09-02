# Domain Marketplace Module Plan

## Overview

A marketplace where admins list domains for sale and users browse/purchase them. Initial version uses manual Telegram contact for purchases, designed to be extensible for future payment integration.

---

## 1. Database Schema

### Table: `domain_listings`

```sql
CREATE TABLE IF NOT EXISTS domain_listings (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL UNIQUE,
    tld TEXT NOT NULL,                        -- extracted TLD (.com, .io, etc.)
    price DECIMAL(10,2) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    description TEXT,
    category TEXT NOT NULL DEFAULT 'standard', -- premium, standard, budget
    status TEXT NOT NULL DEFAULT 'available',  -- available, reserved, sold, hidden
    featured BOOLEAN NOT NULL DEFAULT false,
    
    -- Ownership
    created_by TEXT NOT NULL REFERENCES users(id),
    reserved_by TEXT REFERENCES users(id),
    reserved_at TIMESTAMP,
    purchased_by TEXT REFERENCES users(id),
    purchased_at TIMESTAMP,
    
    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_domain_listings_status ON domain_listings(status);
CREATE INDEX idx_domain_listings_featured ON domain_listings(featured) WHERE featured = true;
CREATE INDEX idx_domain_listings_category ON domain_listings(category);
CREATE INDEX idx_domain_listings_tld ON domain_listings(tld);
CREATE INDEX idx_domain_listings_price ON domain_listings(price);
```

---

## 2. Module Structure

```
modules/marketplace/
├── module.go                        # Module registration, routes, menu
├── handlers/
│   └── handler.go                   # HTTP handlers (user + admin)
├── services/
│   └── marketplace_service.go       # Business logic
├── models/
│   └── domain_listing.go            # Model, constants, validation
├── templates/
│   ├── browse.html                  # User marketplace page
│   ├── admin.html                   # Admin management page
│   └── widget.html                  # Dashboard widget partial
└── migrations/
    └── 001_create_domain_listings.sql
```

---

## 3. Model Definition

### `models/domain_listing.go`

```go
package models

import "time"

type DomainListing struct {
    ID          string     `db:"id" json:"id"`
    Domain      string     `db:"domain" json:"domain"`
    TLD         string     `db:"tld" json:"tld"`
    Price       float64    `db:"price" json:"price"`
    Currency    string     `db:"currency" json:"currency"`
    Description string     `db:"description" json:"description"`
    Category    string     `db:"category" json:"category"`
    Status      string     `db:"status" json:"status"`
    Featured    bool       `db:"featured" json:"featured"`
    
    CreatedBy   string     `db:"created_by" json:"createdBy"`
    ReservedBy  *string    `db:"reserved_by" json:"reservedBy,omitempty"`
    ReservedAt  *time.Time `db:"reserved_at" json:"reservedAt,omitempty"`
    PurchasedBy *string    `db:"purchased_by" json:"purchasedBy,omitempty"`
    PurchasedAt *time.Time `db:"purchased_at" json:"purchasedAt,omitempty"`
    
    CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
    UpdatedAt   time.Time  `db:"updated_at" json:"updatedAt"`
}

// Status constants
const (
    ListingStatusAvailable = "available"
    ListingStatusReserved  = "reserved"
    ListingStatusSold      = "sold"
    ListingStatusHidden    = "hidden"
)

// Category constants
const (
    ListingCategoryPremium  = "premium"
    ListingCategoryStandard = "standard"
    ListingCategoryBudget   = "budget"
)

// For template/UI display
type ListingStats struct {
    Available int `json:"available"`
    Reserved  int `json:"reserved"`
    Sold      int `json:"sold"`
    Total     int `json:"total"`
}
```

---

## 4. Service Layer

### `services/marketplace_service.go`

| Method | Signature | Description |
|--------|-----------|-------------|
| Create | `Create(listing *DomainListing) error` | Add new listing |
| Update | `Update(id string, listing *DomainListing) error` | Update listing |
| Delete | `Delete(id string) error` | Hard delete listing |
| GetByID | `GetByID(id string) (*DomainListing, error)` | Get single listing |
| ListAvailable | `ListAvailable(filters ListingFilters) ([]DomainListing, int, error)` | For users (status=available only) |
| ListAll | `ListAll(filters ListingFilters) ([]DomainListing, int, error)` | For admin (all statuses) |
| ListFeatured | `ListFeatured(limit int) ([]DomainListing, error)` | Featured for widget |
| UpdateStatus | `UpdateStatus(id, status string, userID *string) error` | Change status |
| ToggleFeatured | `ToggleFeatured(id string) error` | Toggle featured flag |
| GetStats | `GetStats() (*ListingStats, error)` | Counts by status |
| GetTLDs | `GetTLDs() ([]string, error)` | Unique TLDs for filter |

### Filter Struct

```go
type ListingFilters struct {
    Search   string   // domain name search
    Category string   // premium, standard, budget
    TLD      string   // .com, .io, etc.
    MinPrice float64
    MaxPrice float64
    Status   string   // for admin
    Featured *bool
    Page     int
    PerPage  int
    SortBy   string   // price, domain, created_at
    SortDir  string   // asc, desc
}
```

---

## 5. Routes

### User Routes (`/user/marketplace`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | `BrowsePage` | Marketplace browse page |
| GET | `/api/listings` | `APIListAvailable` | List available domains |
| GET | `/api/listings/{id}` | `APIGetListing` | Get single listing |
| GET | `/api/filters` | `APIGetFilters` | Get filter options (TLDs, categories) |

### Admin Routes (`/admin/marketplace`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | `AdminPage` | Admin management page |
| GET | `/api/listings` | `APIAdminList` | List all domains |
| POST | `/api/listings` | `APICreate` | Create new listing |
| GET | `/api/listings/{id}` | `APIAdminGet` | Get listing details |
| PUT | `/api/listings/{id}` | `APIUpdate` | Update listing |
| DELETE | `/api/listings/{id}` | `APIDelete` | Delete listing |
| PUT | `/api/listings/{id}/status` | `APIUpdateStatus` | Change status |
| PUT | `/api/listings/{id}/featured` | `APIToggleFeatured` | Toggle featured |
| GET | `/api/stats` | `APIGetStats` | Get statistics |

### Dashboard Widget (in dashboard module)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/featured-domains` | `APIFeaturedDomains` | Featured for widget |

---

## 6. Menu Items

### User Menu

```go
{
    Title:   "Marketplace",
    Icon:    "bi-shop",
    Path:    "/user/marketplace",
    Order:   50,
    Section: module.MenuSectionUser,
}
```

### Admin Menu

```go
{
    Title:   "Marketplace",
    Icon:    "bi-shop",
    Path:    "/admin/marketplace",
    Order:   40,
    Section: module.MenuSectionAdmin,
}
```

---

## 7. Templates

### `browse.html` (User Page)

```
┌─────────────────────────────────────────────────────────────────┐
│ Domain Marketplace                                              │
├─────────────────────────────────────────────────────────────────┤
│ Filters:                                                        │
│ [Search domain...] [Category ▼] [TLD ▼] [Price: $0 - $10000]   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│  │ ⭐ FEATURED      │  │                  │  │                  │
│  │ premium.com      │  │ startup.io       │  │ budget.net       │
│  │ ─────────────── │  │ ─────────────── │  │ ─────────────── │
│  │ $2,500 USD       │  │ $599 USD         │  │ $99 USD          │
│  │ [PREMIUM]        │  │ [STANDARD]       │  │ [BUDGET]         │
│  │                  │  │                  │  │                  │
│  │ Great for brands │  │ Perfect for...   │  │ Affordable...    │
│  │                  │  │                  │  │                  │
│  │ [📱 Contact to   │  │ [📱 Contact to   │  │ [📱 Contact to   │
│  │     Purchase]    │  │     Purchase]    │  │     Purchase]    │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘
│                                                                 │
│                    [← Prev] 1 2 3 [Next →]                      │
└─────────────────────────────────────────────────────────────────┘
```

**Features:**
- Responsive grid (3 cols desktop, 2 tablet, 1 mobile)
- Featured badge on featured domains
- Category badges (color coded)
- Price prominently displayed
- "Contact to Purchase" button → Telegram link
- Filters: search, category dropdown, TLD dropdown, price range
- Pagination
- Sort by: price, newest
- Empty state when no results

### `admin.html` (Admin Page)

```
┌─────────────────────────────────────────────────────────────────┐
│ Marketplace Management                        [+ Add Domain]   │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                │
│ │   12    │ │    3    │ │    5    │ │   20    │                │
│ │Available│ │Reserved │ │  Sold   │ │  Total  │                │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘                │
├─────────────────────────────────────────────────────────────────┤
│ [Search...] [Status ▼] [Category ▼]                            │
├─────────────────────────────────────────────────────────────────┤
│ Domain          │ Price    │ Category │ Status    │ Featured │ Actions │
│ ────────────────┼──────────┼──────────┼───────────┼──────────┼─────────│
│ premium.com     │ $2,500   │ Premium  │ Available │ ⭐       │ ✏️ 🗑️  │
│ startup.io      │ $599     │ Standard │ Reserved  │          │ ✏️ 🗑️  │
│ sold-domain.net │ $199     │ Budget   │ Sold      │          │ ✏️ 🗑️  │
├─────────────────────────────────────────────────────────────────┤
│                         Page 1 of 3                             │
└─────────────────────────────────────────────────────────────────┘
```

**Add/Edit Modal:**
```
┌─────────────────────────────────────┐
│ Add Domain Listing              [X] │
├─────────────────────────────────────┤
│ Domain: [example.com           ]    │
│ Price:  [$] [500.00            ]    │
│ Category: [Standard ▼]              │
│ Description:                        │
│ [                              ]    │
│ [                              ]    │
│ [ ] Featured                        │
├─────────────────────────────────────┤
│              [Cancel] [Save]        │
└─────────────────────────────────────┘
```

### `widget.html` (Dashboard Widget)

```
┌─────────────────────────────────────────┐
│ 🏪 Domain Marketplace            [→]   │
├─────────────────────────────────────────┤
│ premium-brand.com     $2,500  ⭐PREMIUM │
│ cool-startup.io       $599    STANDARD │
│ budget-site.net       $99     BUDGET   │
├─────────────────────────────────────────┤
│         [Browse All Domains]            │
└─────────────────────────────────────────┘
```

**Widget placement:** Below existing dashboard widgets (analytics, recent links, etc.)

---

## 8. Dashboard Integration

### Changes to `modules/dashboard/`

1. **Handler:** Add `marketplaceService` dependency
2. **Template:** Include widget partial after existing content
3. **Data:** Pass `FeaturedDomains` to template

```go
// In dashboard handler
featuredDomains, _ := h.marketplace.ListFeatured(5)

data := map[string]interface{}{
    // ... existing data
    "FeaturedDomains": featuredDomains,
}
```

---

## 9. Extensibility Design

### Payment Provider Interface (Future)

```go
// pkg/payment/provider.go (future)
type PaymentProvider interface {
    Name() string
    CreateCheckout(listing *DomainListing, buyer *User) (*Checkout, error)
    VerifyPayment(checkoutID string) (*PaymentResult, error)
    HandleWebhook(r *http.Request) error
}

type Checkout struct {
    ID          string
    URL         string
    ExpiresAt   time.Time
}

type PaymentResult struct {
    Paid      bool
    Amount    float64
    Currency  string
    Reference string
}
```

### Manual Provider (Current)

```go
// Current implementation - just returns Telegram link
type ManualPaymentProvider struct {
    TelegramHandle string // e.g., "@robertp2083"
}

func (p *ManualPaymentProvider) GetContactURL(listing *DomainListing) string {
    msg := fmt.Sprintf("Hi, I'm interested in purchasing %s for $%.2f", 
        listing.Domain, listing.Price)
    return fmt.Sprintf("https://t.me/%s?text=%s", 
        p.TelegramHandle, url.QueryEscape(msg))
}
```

### Event Hooks (Future Notifications)

```go
type MarketplaceEvent string

const (
    EventListingCreated    MarketplaceEvent = "listing.created"
    EventListingUpdated    MarketplaceEvent = "listing.updated"
    EventInterestExpressed MarketplaceEvent = "interest.expressed"
    EventListingReserved   MarketplaceEvent = "listing.reserved"
    EventListingSold       MarketplaceEvent = "listing.sold"
)

// Hook interface for future use
type MarketplaceHook interface {
    OnEvent(event MarketplaceEvent, listing *DomainListing, user *User) error
}
```

---

## 10. Implementation Checklist

| Step | Task | Files | Status |
|------|------|-------|--------|
| 1 | Create migration | `modules/marketplace/migrations/001_create_domain_listings.sql` | ⬜ |
| 2 | Create models | `modules/marketplace/models/domain_listing.go` | ⬜ |
| 3 | Create service | `modules/marketplace/services/marketplace_service.go` | ⬜ |
| 4 | Create handlers | `modules/marketplace/handlers/handler.go` | ⬜ |
| 5 | Create module | `modules/marketplace/module.go` | ⬜ |
| 6 | Create browse template | `modules/marketplace/templates/browse.html` | ⬜ |
| 7 | Create admin template | `modules/marketplace/templates/admin.html` | ⬜ |
| 8 | Create widget template | `modules/marketplace/templates/widget.html` | ⬜ |
| 9 | Integrate widget into dashboard | `modules/dashboard/` | ⬜ |
| 10 | Register module in main.go | `cmd/server/main.go` | ⬜ |
| 11 | Run migration on server | Deploy | ⬜ |
| 12 | Test all features | Manual | ⬜ |
| 13 | Commit & release | Git | ⬜ |

---

## 11. Configuration

### Environment Variables (Future)

```env
# Marketplace settings
MARKETPLACE_TELEGRAM_CONTACT=robertp2083
MARKETPLACE_DEFAULT_CURRENCY=USD
MARKETPLACE_FEATURED_LIMIT=5
```

### Current: Hardcoded in handler

```go
const (
    telegramContact = "robertp2083"
    defaultCurrency = "USD"
    featuredLimit   = 5
)
```

---

## 12. Security Considerations

1. **Admin-only creation:** Only admins can add/edit/delete listings
2. **Input validation:** Domain format, price > 0, valid category/status
3. **XSS prevention:** All user-visible text escaped in templates
4. **CSRF:** All POST/PUT/DELETE use standard middleware
5. **Rate limiting:** Consider for "express interest" endpoint (future)

---

## 13. Future Enhancements

1. **Payment Integration:** Stripe, crypto payments
2. **Auto-transfer:** Integrate with domain registrar APIs
3. **Bidding/Offers:** Allow users to make offers
4. **Watchlist:** Users can save domains they're interested in
5. **Notifications:** Email/Telegram when price drops or domain sells
6. **Analytics:** Views, interests expressed per domain
7. **Bulk import:** CSV upload for admins
8. **Domain appraisal:** AI-suggested pricing

---

## Summary

This plan creates a self-contained marketplace module with:
- Clean separation of user/admin functionality
- Extensible architecture for future payments
- Dashboard widget integration
- Full CRUD for admin management
- Browse/filter/search for users
- Manual Telegram contact for purchases (MVP)

Ready to implement when approved.

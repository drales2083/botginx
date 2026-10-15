# Domain Marketplace Module Plan

## Overview

A marketplace where admins list **pre-configured domains** for sale. Users purchase with balance and **instantly use them for redirect links** - no setup required since domains are already deployed to VPS with all necessary files.

## Key Concept

```
Admin adds domain EXACTLY like a user would
    ↓
Domain gets fully configured on deploy VPS (files, SSL, DNS)
    ↓
Admin sets price and lists in marketplace
    ↓
User purchases with balance
    ↓
Domain ownership transfers to user
    ↓
User uses immediately - NO SETUP NEEDED
```

## Why This Works

When admin adds a domain:
1. Uses **100% same code** as user domain setup
2. Domain verified via DNS TXT record
3. SSL certificate issued
4. **Files deployed to VPS** (nginx config, site directory, etc.)
5. Domain marked as "active"

When user purchases:
- Just change `user_id` in database
- Files already on VPS - nothing to deploy
- User can create redirect links immediately

---

## Database Schema

### Add columns to existing `domains` table

```sql
-- Add marketplace columns to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS is_marketplace BOOLEAN DEFAULT FALSE;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_price DECIMAL(10,2);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_description TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_listed_at TIMESTAMP;
```

### Marketplace tracking table

```sql
CREATE TABLE IF NOT EXISTS marketplace_sales (
    id VARCHAR(26) PRIMARY KEY,
    domain_id VARCHAR(26) NOT NULL REFERENCES domains(id),
    seller_user_id VARCHAR(36) NOT NULL,  -- Admin who listed
    buyer_user_id VARCHAR(36) NOT NULL,   -- User who purchased
    price DECIMAL(10,2) NOT NULL,
    purchased_at TIMESTAMP DEFAULT NOW()
);
```

---

## Module Structure

```
modules/marketplace/
├── handlers/
│   └── handler.go           # Admin + User handlers
├── migrations/
│   └── 001_create_tables.sql
├── models/
│   └── marketplace.go       # Structs for marketplace data
├── services/
│   └── marketplace_service.go
├── templates/
│   ├── admin_list.html      # Admin: list domains for sale
│   ├── user_browse.html     # User: browse & buy domains
└── module.go
```

---

## Admin Flow

### Add Domain for Sale

1. Go to **Admin → Marketplace**
2. Click **"Add Domain for Sale"**
3. **Reuse existing domain setup flow:**
   - Enter domain name (e.g., `*.premium.com`)
   - System shows DNS records to add
   - Admin adds DNS records
   - Click "Verify" → DNS verified
   - Click "Enable SSL" → SSL certificate issued
   - **Domain deployed to VPS automatically**
4. Domain status becomes "active"
5. **Set price** (e.g., $50.00)
6. Domain appears in marketplace

### Manage Listings

- View all marketplace domains
- Edit price/description
- Remove from marketplace (keeps domain, just not for sale)
- See purchase history

---

## User Flow

### Browse & Purchase

1. Go to **"Buy Domain"** in sidebar
2. See grid of available domains with:
   - Domain name (e.g., `*.premium.com`)
   - Price
   - Status badge: "Ready to use"
3. Click **"Purchase"**
4. Confirmation modal shows:
   - Domain name
   - Price
   - Current balance
   - Balance after purchase
5. Click **"Confirm Purchase"**
6. Balance deducted
7. Domain transferred to user
8. Redirect to domain list

### After Purchase

- Domain appears in user's **"My Domains"** list
- Status: **Active** (already configured!)
- User can immediately create redirect links
- No DNS setup, no SSL setup - already done

---

## Routes

### Admin Routes (`/admin/marketplace`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | AdminList | List all marketplace domains |
| GET | `/add` | AdminAddPage | Add domain page (reuses domain setup) |
| POST | `/api/domains` | APIAddDomain | Add domain to marketplace |
| PUT | `/api/domains/{id}/price` | APISetPrice | Set/update price |
| PUT | `/api/domains/{id}` | APIUpdateListing | Update listing |
| DELETE | `/api/domains/{id}` | APIRemoveFromSale | Remove from marketplace |
| GET | `/api/sales` | APISalesHistory | Purchase history |

### User Routes (`/user/marketplace`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | UserBrowse | Browse available domains |
| GET | `/api/domains` | APIListAvailable | List available domains (JSON) |
| POST | `/api/purchase/{domainId}` | APIPurchase | Purchase domain |

---

## Menu Items

### Admin Sidebar
```
Sell Domain (icon: bi-tag)
```

### User Sidebar
```
Buy Domain (icon: bi-bag)
```

---

## Implementation Details

### Reusing Domain Module Code

```go
// Admin handler - add domain for marketplace
func (h *Handler) APIAddDomain(w http.ResponseWriter, r *http.Request) {
    // 1. Use existing domain service to create domain
    domain, err := h.domainService.Create(adminUserID, input)
    
    // 2. Mark as marketplace domain
    h.service.MarkForSale(domain.ID, price, description)
}
```

### Domain Setup (100% Reuse)

Admin uses exact same pages as users:
- `modules/domains/templates/setup.html` - DNS verification UI
- `modules/domains/services/verification_service.go` - DNS check
- `modules/domains/services/domain_service.go` - SSL setup, deploy

The only difference:
- `is_marketplace = TRUE`
- `marketplace_price` set
- `user_id` = admin's ID (temporary)

### Purchase Logic

```go
func (s *MarketplaceService) Purchase(domainID, buyerUserID string) error {
    // 1. Get domain
    domain, _ := s.domainService.Get(domainID)
    
    // 2. Verify it's for sale
    if !domain.IsMarketplace {
        return errors.New("domain not for sale")
    }
    
    // 3. Check buyer balance
    balance := s.GetUserBalance(buyerUserID)
    if balance < domain.MarketplacePrice {
        return errors.New("insufficient balance")
    }
    
    // Transaction:
    // 4. Deduct balance
    s.DeductBalance(buyerUserID, domain.MarketplacePrice)
    
    // 5. Transfer ownership
    s.db.Exec(`
        UPDATE domains SET 
            user_id = $1, 
            is_marketplace = FALSE,
            marketplace_price = NULL
        WHERE id = $2
    `, buyerUserID, domainID)
    
    // 6. Record sale
    s.RecordSale(domainID, sellerID, buyerUserID, price)
    
    return nil
}
```

### Why No Re-Deploy Needed

When domain is first added:
1. Files are written to VPS in `/var/www/sites/{domain}/`
2. Nginx config created
3. SSL cert installed

These files don't reference the owner. They just serve the domain.

When ownership transfers:
- Only database `user_id` changes
- VPS files stay exactly the same
- Domain works immediately for new owner

---

## UI Templates

### Admin: List Marketplace Domains

```
┌─────────────────────────────────────────────────────────────┐
│ Marketplace                              [+ Add Domain]     │
├─────────────────────────────────────────────────────────────┤
│ Domain              │ Price   │ Status  │ Listed    │ Actions
│ ────────────────────┼─────────┼─────────┼───────────┼────────
│ *.premium.com       │ $50.00  │ Active  │ 2 days ago│ ✏️ 🗑️
│ *.starter.io        │ $25.00  │ Active  │ 1 week ago│ ✏️ 🗑️
│ *.budget.net        │ $10.00  │ Pending │ Just now  │ ✏️ 🗑️
└─────────────────────────────────────────────────────────────┘
```

### User: Browse Domains

```
┌─────────────────────────────────────────────────────────────┐
│ Buy Domain                                                  │
│                                                             │
│ Your Balance: $75.00                                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  │ *.premium.com   │  │ *.starter.io    │  │ *.budget.net    │
│  │                 │  │                 │  │                 │
│  │   $50.00        │  │   $25.00        │  │   $10.00        │
│  │                 │  │                 │  │                 │
│  │ ✓ Ready to use  │  │ ✓ Ready to use  │  │ ✓ Ready to use  │
│  │                 │  │                 │  │                 │
│  │ [Purchase]      │  │ [Purchase]      │  │ [Purchase]      │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘
└─────────────────────────────────────────────────────────────┘
```

### Purchase Modal

```
┌─────────────────────────────────────────┐
│ Confirm Purchase                    [X] │
├─────────────────────────────────────────┤
│                                         │
│   Domain: *.premium.com                 │
│   Price: $50.00                         │
│                                         │
│   Your Balance: $75.00                  │
│   After Purchase: $25.00                │
│                                         │
├─────────────────────────────────────────┤
│            [Cancel]  [Confirm]          │
└─────────────────────────────────────────┘
```

---

## Implementation Checklist

| # | Task | Status |
|---|------|--------|
| 1 | Create migration (marketplace columns + sales table) | ⬜ |
| 2 | Create models (MarketplaceDomain, Sale) | ⬜ |
| 3 | Create service (list, purchase, mark for sale) | ⬜ |
| 4 | Create admin handlers | ⬜ |
| 5 | Create user handlers | ⬜ |
| 6 | Create admin list template | ⬜ |
| 7 | Create user browse template | ⬜ |
| 8 | Create module.go with routes | ⬜ |
| 9 | Register module in main.go | ⬜ |
| 10 | Test full flow | ⬜ |
| 11 | Commit & deploy | ⬜ |

---

## Key Points

1. **100% code reuse** for domain setup - admin uses same flow as users
2. **Instant activation** - files already on VPS, just transfer ownership
3. **Balance system** - reuse existing user balance from hosting module
4. **Wildcard domains** - `*.example.com` so users can create any subdomain
5. **Simple database** - just add columns to existing domains table

---

## Questions Resolved

1. **Edit price after listing?** → Yes, admin can update price
2. **Featured domains?** → Not in v1, can add later
3. **Purchase history?** → Yes, marketplace_sales table
4. **Refunds?** → Not in v1, manual process if needed

---

Ready to implement.

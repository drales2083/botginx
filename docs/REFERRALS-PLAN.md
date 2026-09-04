# Referrals Module Plan

Multi-level referral system with admin-configurable commission rates.

## Overview

Users earn commission when their referrals pay for subscriptions:
- **Level 1 (Direct)**: 25% — someone you referred directly
- **Level 2 (Indirect)**: 15% — someone your L1 referral brought in
- **Level 3 (Network)**: 10% — someone your L2 referral brought in

Earnings are credited to user balance immediately on payment.

## Database Schema

### Users Table Additions

```sql
ALTER TABLE users ADD COLUMN referral_code VARCHAR(8) UNIQUE;
ALTER TABLE users ADD COLUMN referred_by_id UUID REFERENCES users(id);

-- Generate referral codes for existing users
UPDATE users SET referral_code = UPPER(SUBSTR(MD5(RANDOM()::TEXT), 1, 8)) 
WHERE referral_code IS NULL;
```

### Referral Earnings Table

```sql
CREATE TABLE referral_earnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    from_user_id UUID NOT NULL REFERENCES users(id),
    level INT NOT NULL CHECK (level BETWEEN 1 AND 3),
    payment_type VARCHAR(20) NOT NULL, -- 'subscription', 'renewal'
    payment_amount DECIMAL(10,2) NOT NULL,
    commission_rate INT NOT NULL,
    commission_amount DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_referral_earnings_user ON referral_earnings(user_id);
CREATE INDEX idx_referral_earnings_from ON referral_earnings(from_user_id);
```

### Referral Settings Table

```sql
CREATE TABLE referral_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    level1_rate INT NOT NULL DEFAULT 25,
    level2_rate INT NOT NULL DEFAULT 15,
    level3_rate INT NOT NULL DEFAULT 10,
    enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Seed default settings
INSERT INTO referral_settings (level1_rate, level2_rate, level3_rate, enabled)
VALUES (25, 15, 10, true);
```

## Module Structure

```
modules/referrals/
├── module.go
├── models/
│   └── models.go
├── services/
│   └── service.go
├── handlers/
│   └── handler.go
└── templates/
    ├── index.html        # User dashboard
    └── settings.html     # Admin settings
```

## Routes

### User Routes (`/user/referrals`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | index | Referral dashboard |
| GET | `/earnings` | earnings | Earnings history (JSON) |

### Admin Routes (`/admin/referrals`)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/settings` | settings | Settings page |
| POST | `/settings` | updateSettings | Save rates |
| GET | `/stats` | stats | Platform-wide referral stats |

## Service Methods

```go
type Service struct {
    db *sqlx.DB
}

// GetSettings returns current commission rates
func (s *Service) GetSettings() (*Settings, error)

// UpdateSettings saves new rates (admin)
func (s *Service) UpdateSettings(l1, l2, l3 int, enabled bool) error

// ProcessPayment calculates and credits commissions for a payment
// Called by subscription module after successful payment
func (s *Service) ProcessPayment(userID string, amount float64, paymentType string) error

// GetReferralChain returns up to 3 levels of referrers for a user
func (s *Service) GetReferralChain(userID string) ([]ChainLink, error)

// GetUserStats returns referral counts and earnings for a user
func (s *Service) GetUserStats(userID string) (*UserStats, error)

// GetReferrals returns paginated list of user's referrals by level
func (s *Service) GetReferrals(userID string, level int) ([]Referral, error)

// GetEarnings returns paginated earnings history
func (s *Service) GetEarnings(userID string, limit, offset int) ([]Earning, error)

// GenerateCode creates a unique referral code for a user
func (s *Service) GenerateCode(userID string) (string, error)
```

## Integration Points

### 1. Auth Module (Registration)

In `modules/auth/handlers/handler.go`, capture referral code:

```go
func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
    // ... existing registration logic ...
    
    // Capture referral code from query param
    refCode := r.URL.Query().Get("ref")
    if refCode != "" {
        var referrerID string
        h.db.Get(&referrerID, 
            "SELECT id FROM users WHERE referral_code = $1", refCode)
        if referrerID != "" {
            // Set referred_by_id on the new user
            h.db.Exec(
                "UPDATE users SET referred_by_id = $1 WHERE id = $2",
                referrerID, newUserID)
        }
    }
    
    // Generate referral code for new user
    h.referrals.GenerateCode(newUserID)
}
```

### 2. Subscription Module (Payment)

In subscription payment handler, trigger commission processing:

```go
func (s *Service) ProcessSubscriptionPayment(userID string, amount float64) error {
    // ... existing payment logic ...
    
    // Process referral commissions
    if err := s.referrals.ProcessPayment(userID, amount, "subscription"); err != nil {
        log.Warn().Err(err).Msg("referral commission failed")
        // Don't fail the payment, just log
    }
    
    return nil
}
```

### 3. Commission Calculation

```go
func (s *Service) ProcessPayment(userID string, amount float64, paymentType string) error {
    settings, _ := s.GetSettings()
    if !settings.Enabled {
        return nil
    }
    
    chain, _ := s.GetReferralChain(userID)
    
    rates := []int{settings.Level1Rate, settings.Level2Rate, settings.Level3Rate}
    
    for i, link := range chain {
        if i >= 3 {
            break
        }
        
        rate := rates[i]
        commission := amount * float64(rate) / 100
        
        // Record earning
        s.db.Exec(`
            INSERT INTO referral_earnings 
            (user_id, from_user_id, level, payment_type, payment_amount, commission_rate, commission_amount)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
        `, link.UserID, userID, i+1, paymentType, amount, rate, commission)
        
        // Credit balance
        s.db.Exec(`
            UPDATE users SET balance = balance + $1 WHERE id = $2
        `, commission, link.UserID)
    }
    
    return nil
}
```

## User Interface

### User Dashboard (`/user/referrals`)

```
┌─────────────────────────────────────────────────────────────┐
│ Your Referral Link                                          │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ https://guardbot.sbs/auth/register?ref=ABC123    [Copy] │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘

┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ Level 1      │ │ Level 2      │ │ Level 3      │ │ Total Earned │
│ 12 referrals │ │ 5 referrals  │ │ 2 referrals  │ │ $245.50      │
│ 25% rate     │ │ 15% rate     │ │ 10% rate     │ │              │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Recent Earnings                                             │
├─────────────────────────────────────────────────────────────┤
│ Date       │ From      │ Level │ Payment │ Rate │ Earned   │
│ 2024-09-04 │ user@...  │ L1    │ $100    │ 25%  │ $25.00   │
│ 2024-09-03 │ anon@...  │ L2    │ $100    │ 15%  │ $15.00   │
│ 2024-09-01 │ test@...  │ L1    │ $100    │ 25%  │ $25.00   │
└─────────────────────────────────────────────────────────────┘
```

### Admin Settings (`/admin/referrals/settings`)

```
┌─────────────────────────────────────────────────────────────┐
│ Referral Commission Rates                                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Level 1 (Direct Referral)     [====|====] 25%              │
│ Level 2 (Indirect Referral)   [===|=====] 15%              │
│ Level 3 (Network Referral)    [==|======] 10%              │
│                                                             │
│ Total max commission per payment: 50%                       │
│                                                             │
│ [x] Enable referral system                                  │
│                                                             │
│                                        [Save Settings]      │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Platform Stats                                              │
├─────────────────────────────────────────────────────────────┤
│ Total Referrals: 156                                        │
│ Total Commissions Paid: $1,245.50                          │
│ Active Referrers: 23                                        │
└─────────────────────────────────────────────────────────────┘
```

## FAQ Content (Seed Data)

Add to help module FAQ:

```go
var referralFAQs = []FAQ{
    {
        Category: "Referrals",
        Question: "How does the referral program work?",
        Answer:   "Share your unique referral link with others. When they register and pay for a subscription, you earn commission. You can earn up to 3 levels deep: 25% from direct referrals (Level 1), 15% from their referrals (Level 2), and 10% from the next level (Level 3).",
        Order:    1,
    },
    {
        Category: "Referrals",
        Question: "Where do I find my referral link?",
        Answer:   "Go to the Referrals page in your dashboard. Your unique referral link is displayed at the top. Click the copy button to copy it to your clipboard.",
        Order:    2,
    },
    {
        Category: "Referrals",
        Question: "When do I receive my referral earnings?",
        Answer:   "Referral commissions are credited to your balance immediately when your referral makes a payment. You can use this balance for your own subscription or request a withdrawal.",
        Order:    3,
    },
    {
        Category: "Referrals",
        Question: "Do I earn from subscription renewals?",
        Answer:   "Yes! You earn commission every time your referrals renew their subscription, not just on their first payment. This includes all 3 levels of referrals.",
        Order:    4,
    },
    {
        Category: "Referrals",
        Question: "What are Level 1, Level 2, and Level 3 referrals?",
        Answer:   "Level 1 are users who registered using YOUR link (you earn 25%). Level 2 are users who registered using your Level 1 referrals' links (you earn 15%). Level 3 are users who registered using your Level 2 referrals' links (you earn 10%).",
        Order:    5,
    },
    {
        Category: "Referrals",
        Question: "Is there a limit to how many people I can refer?",
        Answer:   "No limit! Refer as many people as you want. The more active referrals you have, the more you earn from the 3-level commission structure.",
        Order:    6,
    },
    {
        Category: "Referrals",
        Question: "Can I refer myself or create fake accounts?",
        Answer:   "No. Self-referrals and fraudulent accounts are prohibited and will result in forfeiture of all referral earnings and possible account termination.",
        Order:    7,
    },
    {
        Category: "Referrals",
        Question: "How do I withdraw my referral earnings?",
        Answer:   "Your referral earnings are added to your account balance. You can use this balance to pay for your own subscription, or request a cryptocurrency withdrawal from the Payments section.",
        Order:    8,
    },
}
```

## Menu Item

```go
module.MenuItem{
    Title:   "Referrals",
    Icon:    "bi-people",
    Path:    "/user/referrals",
    Order:   50,
    Section: module.MenuSectionUser,
}

// Admin menu
module.MenuItem{
    Title:   "Referrals",
    Icon:    "bi-diagram-3",
    Path:    "/admin/referrals/settings",
    Order:   60,
    Section: module.MenuSectionAdmin,
}
```

## Implementation Checklist

- [ ] Create `modules/referrals/` directory structure
- [ ] Write migration (users columns + tables)
- [ ] Implement models (Settings, Earning, Referral, UserStats)
- [ ] Implement service (all methods listed above)
- [ ] Implement handlers (user dashboard, admin settings)
- [ ] Create templates (user index, admin settings)
- [ ] Register module in main.go
- [ ] Integrate with auth module (capture ref param)
- [ ] Integrate with subscription module (trigger commissions)
- [ ] Seed FAQ entries
- [ ] Generate referral codes for existing users (migration)
- [ ] Test: registration with ref code
- [ ] Test: commission calculation (all 3 levels)
- [ ] Test: balance credit
- [ ] Test: admin rate changes

## Security Considerations

1. **Self-referral prevention**: Check that referred_by_id != user's own ID
2. **Code uniqueness**: Enforce unique constraint, retry on collision
3. **Rate limits**: Sensible defaults, admin can adjust
4. **Fraud detection**: Log IP on registration, flag suspicious patterns (future)
5. **Atomic transactions**: Commission credit must be atomic with earning record

## Future Enhancements

- Referral leaderboard
- Time-limited bonus rates (campaigns)
- Custom referral codes (premium feature)
- Referral link click tracking
- Email notifications on earnings
- Minimum withdrawal threshold

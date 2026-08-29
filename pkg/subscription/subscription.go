// Package subscription gates write access behind an admin-granted subscription.
//
// Access is derived from an expiry timestamp on every check rather than cached
// in a status column, so a lapsed subscription stops working the moment it
// expires without anything having to sweep the table.
package subscription

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jmoiron/sqlx"
)

// ExpiringWindow is how long before expiry a subscription starts warning the
// user. Access is unaffected; this only drives the banner.
const ExpiringWindow = 7 * 24 * time.Hour

// Status describes a user's access at a point in time.
type Status string

const (
	StatusNone     Status = "none"     // never subscribed
	StatusActive   Status = "active"   // good
	StatusExpiring Status = "expiring" // active, but within ExpiringWindow
	StatusExpired  Status = "expired"  // lapsed
)

type Subscription struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	Plan      string    `db:"plan" json:"plan"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	Notes     string    `db:"notes" json:"notes"`
	GrantedBy string    `db:"granted_by" json:"grantedBy"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// State is the derived view the UI and middleware use.
type State struct {
	Status    Status    `json:"status"`
	Active    bool      `json:"active"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	DaysLeft  int       `json:"daysLeft"`
	Plan      string    `json:"plan,omitempty"`
}

type Service struct {
	db *sqlx.DB
}

func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

func generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Get returns the raw subscription row, or nil when the user has never had one.
func (s *Service) Get(userID string) (*Subscription, error) {
	var sub Subscription
	err := s.db.Get(&sub, `
		SELECT id, user_id, plan, expires_at, notes, granted_by, created_at, updated_at
		FROM subscriptions WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// StateFor derives the current access state for a user.
func (s *Service) StateFor(userID string) State {
	if userID == "" {
		return State{Status: StatusNone}
	}

	sub, err := s.Get(userID)
	if err != nil || sub == nil {
		return State{Status: StatusNone}
	}

	now := time.Now()
	remaining := sub.ExpiresAt.Sub(now)

	if remaining <= 0 {
		return State{
			Status:    StatusExpired,
			ExpiresAt: sub.ExpiresAt,
			Plan:      sub.Plan,
		}
	}

	status := StatusActive
	if remaining <= ExpiringWindow {
		status = StatusExpiring
	}

	return State{
		Status:    status,
		Active:    true,
		ExpiresAt: sub.ExpiresAt,
		// Rounded up, so a subscription with any time left never reads "0 days".
		DaysLeft: int(remaining.Hours()/24) + 1,
		Plan:     sub.Plan,
	}
}

// IsActive reports whether the user may perform write actions.
func (s *Service) IsActive(userID string) bool {
	return s.StateFor(userID).Active
}

// Grant gives the user access until expiresAt, replacing any existing row.
func (s *Service) Grant(userID, plan string, expiresAt time.Time, grantedBy, notes string) error {
	if plan == "" {
		plan = "standard"
	}

	_, err := s.db.Exec(`
		INSERT INTO subscriptions (id, user_id, plan, expires_at, notes, granted_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			plan = EXCLUDED.plan,
			expires_at = EXCLUDED.expires_at,
			notes = EXCLUDED.notes,
			granted_by = EXCLUDED.granted_by,
			updated_at = NOW()
	`, generateID(), userID, plan, expiresAt.UTC(), notes, grantedBy)

	return err
}

// Extend adds days to a subscription. An expired or missing subscription is
// extended from now rather than from its old expiry, so a user who lapsed for
// months does not get a grant that is already spent.
func (s *Service) Extend(userID string, days int, grantedBy string) error {
	from := time.Now()
	if sub, err := s.Get(userID); err == nil && sub != nil && sub.ExpiresAt.After(from) {
		from = sub.ExpiresAt
	}

	plan := "standard"
	if sub, err := s.Get(userID); err == nil && sub != nil && sub.Plan != "" {
		plan = sub.Plan
	}

	return s.Grant(userID, plan, from.AddDate(0, 0, days), grantedBy, "")
}

// Revoke ends access immediately by expiring the subscription rather than
// deleting it, so the grant history survives.
func (s *Service) Revoke(userID string) error {
	_, err := s.db.Exec(`
		UPDATE subscriptions SET expires_at = NOW(), updated_at = NOW()
		WHERE user_id = $1
	`, userID)
	return err
}

// StatesFor returns states for many users in one query, for the admin list.
func (s *Service) StatesFor(userIDs []string) map[string]State {
	states := make(map[string]State, len(userIDs))
	for _, id := range userIDs {
		states[id] = State{Status: StatusNone}
	}
	if len(userIDs) == 0 {
		return states
	}

	query, args, err := sqlx.In(`
		SELECT id, user_id, plan, expires_at, notes, granted_by, created_at, updated_at
		FROM subscriptions WHERE user_id IN (?)
	`, userIDs)
	if err != nil {
		return states
	}

	var subs []Subscription
	if err := s.db.Select(&subs, s.db.Rebind(query), args...); err != nil {
		return states
	}

	now := time.Now()
	for _, sub := range subs {
		remaining := sub.ExpiresAt.Sub(now)
		if remaining <= 0 {
			states[sub.UserID] = State{Status: StatusExpired, ExpiresAt: sub.ExpiresAt, Plan: sub.Plan}
			continue
		}

		status := StatusActive
		if remaining <= ExpiringWindow {
			status = StatusExpiring
		}
		states[sub.UserID] = State{
			Status:    status,
			Active:    true,
			ExpiresAt: sub.ExpiresAt,
			DaysLeft:  int(remaining.Hours()/24) + 1,
			Plan:      sub.Plan,
		}
	}

	return states
}

package banner

import (
	"database/sql"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Banner represents the system-wide notification banner
type Banner struct {
	ID          int            `db:"id" json:"id"`
	Type        string         `db:"type" json:"type"`
	Title       string         `db:"title" json:"title"`
	Message     string         `db:"message" json:"message"`
	ButtonText  string         `db:"button_text" json:"button_text"`
	ButtonURL   string         `db:"button_url" json:"button_url"`
	Enabled     bool           `db:"enabled" json:"enabled"`
	DismissedBy pq.StringArray `db:"dismissed_by" json:"-"`
	UpdatedAt   time.Time      `db:"updated_at" json:"updated_at"`
}

// Service handles system banner operations
type Service struct {
	db    *sqlx.DB
	mu    sync.RWMutex
	cache *Banner
}

// NewService creates a new banner service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// EnsureTable creates the system_banner table if it doesn't exist
func (s *Service) EnsureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS system_banner (
			id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			type VARCHAR(20) DEFAULT 'info',
			title VARCHAR(200) DEFAULT '',
			message TEXT DEFAULT '',
			button_text VARCHAR(50) DEFAULT '',
			button_url VARCHAR(500) DEFAULT '',
			enabled BOOLEAN DEFAULT false,
			dismissed_by TEXT[] DEFAULT '{}',
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	// Add dismissed_by column if not exists (for existing installations)
	s.db.Exec(`ALTER TABLE system_banner ADD COLUMN IF NOT EXISTS dismissed_by TEXT[] DEFAULT '{}'`)

	// Insert default row if not exists
	_, err = s.db.Exec(`
		INSERT INTO system_banner (id, enabled)
		VALUES (1, false)
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

// Get returns the current banner configuration
func (s *Service) Get() (*Banner, error) {
	s.mu.RLock()
	if s.cache != nil {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	var banner Banner
	err := s.db.Get(&banner, "SELECT * FROM system_banner WHERE id = 1")
	if err != nil {
		if err == sql.ErrNoRows {
			return &Banner{Type: "info", Enabled: false}, nil
		}
		return nil, err
	}

	s.mu.Lock()
	s.cache = &banner
	s.mu.Unlock()

	return &banner, nil
}

// Update updates the banner configuration and clears dismissed list
func (s *Service) Update(b *Banner) error {
	_, err := s.db.Exec(`
		INSERT INTO system_banner (id, type, title, message, button_text, button_url, enabled, dismissed_by, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, '{}', NOW())
		ON CONFLICT (id) DO UPDATE SET
			type = $1,
			title = $2,
			message = $3,
			button_text = $4,
			button_url = $5,
			enabled = $6,
			dismissed_by = '{}',
			updated_at = NOW()
	`, b.Type, b.Title, b.Message, b.ButtonText, b.ButtonURL, b.Enabled)

	if err != nil {
		return err
	}

	// Clear cache
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()

	return nil
}

// ClearCache clears the cached banner
func (s *Service) ClearCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// GetEnabled returns the banner only if it's enabled
func (s *Service) GetEnabled() *Banner {
	b, err := s.Get()
	if err != nil || !b.Enabled {
		return nil
	}
	return b
}

// GetEnabledForUser returns the banner if enabled and not dismissed by this user
func (s *Service) GetEnabledForUser(userID string) *Banner {
	b, err := s.Get()
	if err != nil || !b.Enabled || userID == "" {
		return nil
	}
	// Check if user has dismissed
	for _, id := range b.DismissedBy {
		if id == userID {
			return nil
		}
	}
	return b
}

// Dismiss marks the banner as dismissed for a user
func (s *Service) Dismiss(userID string) error {
	if userID == "" {
		return nil
	}
	_, err := s.db.Exec(`
		UPDATE system_banner
		SET dismissed_by = array_append(dismissed_by, $1)
		WHERE id = 1 AND NOT ($1 = ANY(dismissed_by))
	`, userID)
	if err != nil {
		return err
	}
	// Clear cache
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
	return nil
}

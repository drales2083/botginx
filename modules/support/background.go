package support

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// BackgroundJobs handles scheduled tasks for support module
type BackgroundJobs struct {
	db     *sqlx.DB
	stopCh chan struct{}
}

// NewBackgroundJobs creates a new background jobs handler
func NewBackgroundJobs(db *sqlx.DB) *BackgroundJobs {
	return &BackgroundJobs{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start begins the background jobs
func (b *BackgroundJobs) Start() {
	go b.runAutoClose()
	log.Printf("[support] Background jobs started")
}

// Stop halts all background jobs
func (b *BackgroundJobs) Stop() {
	close(b.stopCh)
	log.Printf("[support] Background jobs stopped")
}

// runAutoClose checks for stale tickets every hour and closes them after 48h of inactivity
func (b *BackgroundJobs) runAutoClose() {
	// Initial delay
	time.Sleep(1 * time.Minute)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	b.closeStaleTickets()

	for {
		select {
		case <-ticker.C:
			b.closeStaleTickets()
		case <-b.stopCh:
			return
		}
	}
}

// closeStaleTickets closes tickets with no activity for 48 hours
func (b *BackgroundJobs) closeStaleTickets() {
	// Find tickets where:
	// 1. Status is not 'closed'
	// 2. Last message is older than 48 hours
	result, err := b.db.Exec(`
		UPDATE support_tickets
		SET status = 'closed', updated_at = NOW()
		WHERE status != 'closed'
		AND id IN (
			SELECT t.id
			FROM support_tickets t
			LEFT JOIN ticket_messages tm ON tm.ticket_id = t.id
			WHERE t.status != 'closed'
			GROUP BY t.id
			HAVING COALESCE(MAX(tm.created_at), t.created_at) < NOW() - INTERVAL '48 hours'
		)
	`)
	if err != nil {
		log.Printf("[support] Failed to auto-close stale tickets: %v", err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[support] Auto-closed %d stale tickets (48h inactivity)", rows)
	}
}

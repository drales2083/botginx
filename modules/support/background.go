package support

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// AdminNotifier sends notifications to admin
type AdminNotifier interface {
	NotifyTicketClosed(ticketID, email, subject string)
}

// UserNotifier sends notifications to users
type UserNotifier interface {
	NotifySupportReply(userID, ticketID, subject, message string) error
}

// BackgroundJobs handles scheduled tasks for support module
type BackgroundJobs struct {
	db           *sqlx.DB
	stopCh       chan struct{}
	adminNotify  AdminNotifier
	userNotify   UserNotifier
}

// NewBackgroundJobs creates a new background jobs handler
func NewBackgroundJobs(db *sqlx.DB) *BackgroundJobs {
	return &BackgroundJobs{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// SetAdminNotifier sets the admin notification handler
func (b *BackgroundJobs) SetAdminNotifier(n AdminNotifier) {
	b.adminNotify = n
}

// SetUserNotifier sets the user notification handler
func (b *BackgroundJobs) SetUserNotifier(n UserNotifier) {
	b.userNotify = n
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

// staleTicket holds info about a ticket to be closed
type staleTicket struct {
	ID      string `db:"id"`
	UserID  string `db:"user_id"`
	Subject string `db:"subject"`
	Email   string `db:"email"`
}

// closeStaleTickets closes tickets with no activity for 48 hours
func (b *BackgroundJobs) closeStaleTickets() {
	// Find tickets where:
	// 1. Status is not 'closed'
	// 2. Last message is older than 48 hours
	var tickets []staleTicket
	err := b.db.Select(&tickets, `
		SELECT t.id, t.user_id, t.subject, u.email
		FROM support_tickets t
		JOIN users u ON u.id = t.user_id
		LEFT JOIN ticket_messages tm ON tm.ticket_id = t.id
		WHERE t.status != 'closed'
		GROUP BY t.id, t.user_id, t.subject, u.email
		HAVING COALESCE(MAX(tm.created_at), t.created_at) < NOW() - INTERVAL '48 hours'
	`)
	if err != nil {
		log.Printf("[support] Failed to find stale tickets: %v", err)
		return
	}

	if len(tickets) == 0 {
		return
	}

	// Close the tickets and send notifications
	for _, ticket := range tickets {
		_, err := b.db.Exec(`
			UPDATE support_tickets
			SET status = 'closed', updated_at = NOW()
			WHERE id = $1
		`, ticket.ID)
		if err != nil {
			log.Printf("[support] Failed to auto-close ticket %s: %v", ticket.ID, err)
			continue
		}

		// Notify admin
		if b.adminNotify != nil {
			b.adminNotify.NotifyTicketClosed(ticket.ID, ticket.Email, ticket.Subject)
		}

		// Notify user if they have Telegram connected
		if b.userNotify != nil {
			b.userNotify.NotifySupportReply(
				ticket.UserID,
				ticket.ID,
				ticket.Subject,
				"This ticket has been closed due to inactivity. If you need further assistance, please open a new ticket.",
			)
		}
	}

	log.Printf("[support] Auto-closed %d stale tickets (48h inactivity)", len(tickets))
}

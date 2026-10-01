package adminlog

import (
	"encoding/json"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

// Action types
const (
	// User actions
	ActionUserSubscriptionGrant  = "user.subscription.grant"
	ActionUserSubscriptionRevoke = "user.subscription.revoke"
	ActionUserBalanceTopup       = "user.balance.topup"
	ActionUserBalanceDeduct      = "user.balance.deduct"
	ActionUserEnable             = "user.enable"
	ActionUserDisable            = "user.disable"
	ActionUserRoleChange         = "user.role.change"
	ActionUser2FAReset           = "user.2fa.reset"

	// Payment actions
	ActionPaymentResync     = "payment.resync"
	ActionPaymentAdjust     = "payment.adjust"
	ActionPaymentRefund     = "payment.refund"

	// Domain actions
	ActionDomainAdd      = "domain.add"
	ActionDomainDelete   = "domain.delete"
	ActionDomainTransfer = "domain.transfer"

	// Hosting actions
	ActionHostingCreate   = "hosting.create"
	ActionHostingSuspend  = "hosting.suspend"
	ActionHostingUnsuspend = "hosting.unsuspend"
	ActionHostingDelete   = "hosting.delete"

	// Support actions
	ActionTicketReply  = "ticket.reply"
	ActionTicketClose  = "ticket.close"
	ActionTicketReopen = "ticket.reopen"

	// System actions
	ActionAnnouncementCreate = "announcement.create"
	ActionAnnouncementUpdate = "announcement.update"
	ActionAnnouncementDelete = "announcement.delete"
	ActionSettingsUpdate     = "settings.update"

	// Auth actions
	ActionAdminLogin       = "admin.login"
	ActionAdminLoginFailed = "admin.login.failed"
)

// Target types
const (
	TargetUser         = "user"
	TargetSubscription = "subscription"
	TargetDomain       = "domain"
	TargetHosting      = "hosting"
	TargetTicket       = "ticket"
	TargetAnnouncement = "announcement"
	TargetSettings     = "settings"
	TargetPayment      = "payment"
)

// ActivityLog represents an admin activity log entry
type ActivityLog struct {
	ID         string    `db:"id" json:"id"`
	AdminID    string    `db:"admin_id" json:"adminId"`
	AdminEmail string    `db:"admin_email" json:"adminEmail"`
	Action     string    `db:"action" json:"action"`
	TargetType string    `db:"target_type" json:"targetType"`
	TargetID   string    `db:"target_id" json:"targetId"`
	TargetName string    `db:"target_name" json:"targetName"`
	Details    string    `db:"details" json:"details"`
	IPAddress  string    `db:"ip_address" json:"ipAddress"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
}

// Logger handles admin activity logging
type Logger struct {
	db *sqlx.DB
}

// NewLogger creates a new admin activity logger
func NewLogger(db *sqlx.DB) *Logger {
	return &Logger{db: db}
}

func generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Log records an admin activity
func (l *Logger) Log(adminID, adminEmail, action, targetType, targetID, targetName string, details map[string]interface{}, r *http.Request) error {
	detailsJSON := "{}"
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = string(b)
		}
	}

	ipAddress := ""
	if r != nil {
		ipAddress = r.Header.Get("CF-Connecting-IP")
		if ipAddress == "" {
			ipAddress = r.Header.Get("X-Forwarded-For")
		}
		if ipAddress == "" {
			ipAddress = r.RemoteAddr
		}
	}

	_, err := l.db.Exec(`
		INSERT INTO admin_activity_logs (id, admin_id, admin_email, action, target_type, target_id, target_name, details, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, generateID(), adminID, adminEmail, action, targetType, targetID, targetName, detailsJSON, ipAddress, time.Now())

	return err
}

// GetLogs retrieves activity logs with optional filters
func (l *Logger) GetLogs(limit int, adminEmail, action, targetType string) ([]ActivityLog, error) {
	query := `SELECT * FROM admin_activity_logs WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if adminEmail != "" {
		query += ` AND admin_email = $` + string(rune('0'+argNum))
		args = append(args, adminEmail)
		argNum++
	}
	if action != "" {
		query += ` AND action LIKE $` + string(rune('0'+argNum))
		args = append(args, action+"%")
		argNum++
	}
	if targetType != "" {
		query += ` AND target_type = $` + string(rune('0'+argNum))
		args = append(args, targetType)
		argNum++
	}

	query += ` ORDER BY created_at DESC LIMIT $` + string(rune('0'+argNum))
	args = append(args, limit)

	var logs []ActivityLog
	err := l.db.Select(&logs, query, args...)
	return logs, err
}

// GetRecentLogs gets the most recent logs
func (l *Logger) GetRecentLogs(limit int) ([]ActivityLog, error) {
	var logs []ActivityLog
	err := l.db.Select(&logs, `
		SELECT * FROM admin_activity_logs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	return logs, err
}

// GetLogsByAdmin gets logs for a specific admin
func (l *Logger) GetLogsByAdmin(adminEmail string, limit int) ([]ActivityLog, error) {
	var logs []ActivityLog
	err := l.db.Select(&logs, `
		SELECT * FROM admin_activity_logs
		WHERE admin_email = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, adminEmail, limit)
	return logs, err
}

// GetLogsByTarget gets logs for a specific target
func (l *Logger) GetLogsByTarget(targetType, targetID string, limit int) ([]ActivityLog, error) {
	var logs []ActivityLog
	err := l.db.Select(&logs, `
		SELECT * FROM admin_activity_logs
		WHERE target_type = $1 AND target_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, targetType, targetID, limit)
	return logs, err
}

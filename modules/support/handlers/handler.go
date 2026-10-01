package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/botginx/botginx/pkg/adminlog"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Notifier interface for sending ticket notifications
type Notifier interface {
	NotifyNewTicket(ticketID, email, category, subject, message string)
	NotifyTicketReply(ticketID, email, subject, message string)
}

type Handler struct {
	db          *sqlx.DB
	templates   *module.TemplateEngine
	notifier    Notifier
	adminLogger *adminlog.Logger
}

func NewHandler(db *sqlx.DB, templates *module.TemplateEngine) *Handler {
	return &Handler{db: db, templates: templates, adminLogger: adminlog.NewLogger(db)}
}

func (h *Handler) SetNotifier(n Notifier) {
	h.notifier = n
}

// Models

type Ticket struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Category  string    `db:"category"`
	Subject   string    `db:"subject"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	// Joined fields
	UserEmail string `db:"user_email"`
}

type Message struct {
	ID        string    `db:"id"`
	TicketID  string    `db:"ticket_id"`
	UserID    *string   `db:"user_id"`
	IsAdmin   bool      `db:"is_admin"`
	Message   string    `db:"message"`
	CreatedAt time.Time `db:"created_at"`
}

// Helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

func (h *Handler) jsonOK(w http.ResponseWriter, data interface{}) {
	h.json(w, http.StatusOK, data)
}

func generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Categories for tickets
var Categories = []string{
	"Domain",
	"Redirect Link",
	"Short Link",
	"Payment / Invoice",
	"Subscription",
	"Antibot",
	"Account / Login",
	"Bug Report",
	"Other",
}

// User Handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var tickets []Ticket
	h.db.Select(&tickets, `
		SELECT id, user_id, category, subject, status, created_at, updated_at
		FROM support_tickets
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)

	module.RenderUserSection(w, r, h.templates, "support:list.html", map[string]interface{}{
		"Title":   "Support",
		"Tickets": tickets,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, h.templates, "support:new.html", map[string]interface{}{
		"Title":      "New Ticket",
		"Categories": Categories,
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	ticketID := chi.URLParam(r, "id")

	var ticket Ticket
	err := h.db.Get(&ticket, `
		SELECT id, user_id, category, subject, status, created_at, updated_at
		FROM support_tickets
		WHERE id = $1 AND user_id = $2
	`, ticketID, userID)
	if err != nil {
		http.Redirect(w, r, "/user/support", http.StatusFound)
		return
	}

	var messages []Message
	h.db.Select(&messages, `
		SELECT id, ticket_id, user_id, is_admin, message, created_at
		FROM ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at ASC
	`, ticketID)

	module.RenderUserSection(w, r, h.templates, "support:show.html", map[string]interface{}{
		"Title":    ticket.Subject,
		"Ticket":   ticket,
		"Messages": messages,
	})
}

// User API

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		Category string `json:"category"`
		Subject  string `json:"subject"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate
	if input.Category == "" || input.Subject == "" || input.Message == "" {
		h.jsonError(w, "All fields are required", http.StatusBadRequest)
		return
	}
	if len(input.Subject) < 5 || len(input.Subject) > 100 {
		h.jsonError(w, "Subject must be 5-100 characters", http.StatusBadRequest)
		return
	}
	if len(input.Message) < 10 || len(input.Message) > 2000 {
		h.jsonError(w, "Message must be 10-2000 characters", http.StatusBadRequest)
		return
	}

	// Create ticket
	ticketID := generateID()
	_, err := h.db.Exec(`
		INSERT INTO support_tickets (id, user_id, category, subject, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'open', NOW(), NOW())
	`, ticketID, userID, input.Category, input.Subject)
	if err != nil {
		h.jsonError(w, "Failed to create ticket", http.StatusInternalServerError)
		return
	}

	// Create first message
	messageID := generateID()
	_, err = h.db.Exec(`
		INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at)
		VALUES ($1, $2, $3, FALSE, $4, NOW())
	`, messageID, ticketID, userID, input.Message)
	if err != nil {
		h.jsonError(w, "Failed to create message", http.StatusInternalServerError)
		return
	}

	// Send Telegram notification to admin
	h.notifyAdminNewTicket(ticketID, userID, input.Category, input.Subject, input.Message)

	h.jsonOK(w, map[string]interface{}{
		"success":   true,
		"ticket_id": ticketID,
		"redirect":  "/user/support/" + ticketID,
	})
}

func (h *Handler) APIReply(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	ticketID := chi.URLParam(r, "id")

	// Verify ticket belongs to user and is not closed
	var ticket Ticket
	err := h.db.Get(&ticket, `
		SELECT id, user_id, status FROM support_tickets
		WHERE id = $1 AND user_id = $2
	`, ticketID, userID)
	if err != nil {
		h.jsonError(w, "Ticket not found", http.StatusNotFound)
		return
	}
	if ticket.Status == "closed" {
		h.jsonError(w, "Cannot reply to closed ticket", http.StatusBadRequest)
		return
	}

	var input struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if len(input.Message) < 10 || len(input.Message) > 2000 {
		h.jsonError(w, "Message must be 10-2000 characters", http.StatusBadRequest)
		return
	}

	// Create message
	messageID := generateID()
	_, err = h.db.Exec(`
		INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at)
		VALUES ($1, $2, $3, FALSE, $4, NOW())
	`, messageID, ticketID, userID, input.Message)
	if err != nil {
		h.jsonError(w, "Failed to send reply", http.StatusInternalServerError)
		return
	}

	// Update ticket status to open (user replied)
	h.db.Exec(`UPDATE support_tickets SET status = 'open', updated_at = NOW() WHERE id = $1`, ticketID)

	// TODO: Send Telegram notification to admin
	h.notifyAdminReply(ticketID, userID, input.Message)

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIClose(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	ticketID := chi.URLParam(r, "id")

	result, err := h.db.Exec(`
		UPDATE support_tickets SET status = 'closed', updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`, ticketID, userID)
	if err != nil {
		h.jsonError(w, "Failed to close ticket", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		h.jsonError(w, "Ticket not found", http.StatusNotFound)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

// Admin Handlers

func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	category := r.URL.Query().Get("category")

	query := `
		SELECT t.id, t.user_id, t.category, t.subject, t.status, t.created_at, t.updated_at,
		       u.email as user_email
		FROM support_tickets t
		JOIN users u ON u.id = t.user_id
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if status != "" {
		query += " AND t.status = $" + string(rune('0'+argNum))
		args = append(args, status)
		argNum++
	}
	if category != "" {
		query += " AND t.category = $" + string(rune('0'+argNum))
		args = append(args, category)
		argNum++
	}

	query += " ORDER BY t.updated_at DESC"

	var tickets []Ticket
	h.db.Select(&tickets, query, args...)

	// Count open tickets
	var openCount int
	h.db.Get(&openCount, `SELECT COUNT(*) FROM support_tickets WHERE status = 'open'`)

	module.Render(w, r, h.templates, "support:admin_list.html", map[string]interface{}{
		"Title":          "Support Tickets",
		"Tickets":        tickets,
		"OpenCount":      openCount,
		"Categories":     Categories,
		"FilterStatus":   status,
		"FilterCategory": category,
	})
}

func (h *Handler) AdminShow(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	var ticket Ticket
	err := h.db.Get(&ticket, `
		SELECT t.id, t.user_id, t.category, t.subject, t.status, t.created_at, t.updated_at,
		       u.email as user_email
		FROM support_tickets t
		JOIN users u ON u.id = t.user_id
		WHERE t.id = $1
	`, ticketID)
	if err != nil {
		http.Redirect(w, r, "/admin/support", http.StatusFound)
		return
	}

	var messages []Message
	h.db.Select(&messages, `
		SELECT id, ticket_id, user_id, is_admin, message, created_at
		FROM ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at ASC
	`, ticketID)

	module.Render(w, r, h.templates, "support:admin_show.html", map[string]interface{}{
		"Title":    ticket.Subject,
		"Ticket":   ticket,
		"Messages": messages,
	})
}

// Admin API

func (h *Handler) APIAdminReply(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	// Verify ticket exists
	var ticket Ticket
	err := h.db.Get(&ticket, `SELECT id, status FROM support_tickets WHERE id = $1`, ticketID)
	if err == sql.ErrNoRows {
		h.jsonError(w, "Ticket not found", http.StatusNotFound)
		return
	}

	var input struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if len(input.Message) < 10 || len(input.Message) > 2000 {
		h.jsonError(w, "Message must be 10-2000 characters", http.StatusBadRequest)
		return
	}

	// Create admin message
	messageID := generateID()
	_, err = h.db.Exec(`
		INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at)
		VALUES ($1, $2, NULL, TRUE, $3, NOW())
	`, messageID, ticketID, input.Message)
	if err != nil {
		h.jsonError(w, "Failed to send reply", http.StatusInternalServerError)
		return
	}

	// Update ticket status to answered
	h.db.Exec(`UPDATE support_tickets SET status = 'answered', updated_at = NOW() WHERE id = $1`, ticketID)

	// Log activity
	if adminUser := ctx.GetUser(r); adminUser != nil {
		h.adminLogger.Log(adminUser.ID, adminUser.Email, adminlog.ActionTicketReply,
			adminlog.TargetTicket, ticketID, ticket.Subject,
			map[string]interface{}{"messageLength": len(input.Message)}, r)
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIAdminClose(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	// Get ticket info for logging
	var ticket Ticket
	h.db.Get(&ticket, `SELECT id, subject FROM support_tickets WHERE id = $1`, ticketID)

	result, err := h.db.Exec(`
		UPDATE support_tickets SET status = 'closed', updated_at = NOW()
		WHERE id = $1
	`, ticketID)
	if err != nil {
		h.jsonError(w, "Failed to close ticket", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		h.jsonError(w, "Ticket not found", http.StatusNotFound)
		return
	}

	// Log activity
	if adminUser := ctx.GetUser(r); adminUser != nil {
		h.adminLogger.Log(adminUser.ID, adminUser.Email, adminlog.ActionTicketClose,
			adminlog.TargetTicket, ticketID, ticket.Subject, nil, r)
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIAdminReopen(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	// Get ticket info for logging
	var ticket Ticket
	h.db.Get(&ticket, `SELECT id, subject FROM support_tickets WHERE id = $1`, ticketID)

	result, err := h.db.Exec(`
		UPDATE support_tickets SET status = 'open', updated_at = NOW()
		WHERE id = $1
	`, ticketID)
	if err != nil {
		h.jsonError(w, "Failed to reopen ticket", http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		h.jsonError(w, "Ticket not found", http.StatusNotFound)
		return
	}

	// Log activity
	if adminUser := ctx.GetUser(r); adminUser != nil {
		h.adminLogger.Log(adminUser.ID, adminUser.Email, adminlog.ActionTicketReopen,
			adminlog.TargetTicket, ticketID, ticket.Subject, nil, r)
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

// Telegram notifications

func (h *Handler) notifyAdminNewTicket(ticketID, userID, category, subject, message string) {
	if h.notifier == nil {
		return
	}
	var email string
	h.db.Get(&email, `SELECT email FROM users WHERE id = $1`, userID)
	h.notifier.NotifyNewTicket(ticketID, email, category, subject, message)
}

func (h *Handler) notifyAdminReply(ticketID, userID, message string) {
	if h.notifier == nil {
		return
	}
	var email string
	h.db.Get(&email, `SELECT email FROM users WHERE id = $1`, userID)

	// Get ticket subject for context
	var subject string
	h.db.Get(&subject, `SELECT subject FROM support_tickets WHERE id = $1`, ticketID)

	h.notifier.NotifyTicketReply(ticketID, email, subject, message)
}

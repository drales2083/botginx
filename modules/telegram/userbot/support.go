package userbot

import (
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// SupportHandler handles viewing and replying to support tickets.
type SupportHandler struct {
	db *sqlx.DB
}

func NewSupportHandler(db *sqlx.DB) *SupportHandler {
	return &SupportHandler{db: db}
}

type ticketRow struct {
	ID        string    `db:"id"`
	Subject   string    `db:"subject"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
}

type messageRow struct {
	Message   string    `db:"message"`
	IsAdmin   bool      `db:"is_admin"`
	CreatedAt time.Time `db:"created_at"`
}

// RenderTicket shows a ticket's conversation.
func (s *SupportHandler) RenderTicket(userID, ticketID string) (string, InlineKeyboardMarkup, bool) {
	var ticket ticketRow
	err := s.db.Get(&ticket, `
		SELECT id, subject, status, created_at
		FROM support_tickets
		WHERE id = $1 AND user_id = $2
	`, ticketID, userID)
	if err != nil {
		return "⚠️ Ticket not found.", InlineKeyboardMarkup{[][]InlineButton{backRow()}}, false
	}

	// Get last 5 messages
	var messages []messageRow
	s.db.Select(&messages, `
		SELECT message, is_admin, created_at
		FROM ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at DESC
		LIMIT 5
	`, ticketID)

	// Build conversation text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🎫 <b>%s</b>\n", html.EscapeString(ticket.Subject)))
	sb.WriteString(fmt.Sprintf("Status: %s\n\n", formatStatus(ticket.Status)))

	// Show messages in chronological order (reverse the slice)
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		prefix := "👤 You"
		if m.IsAdmin {
			prefix = "👨‍💼 Support"
		}
		preview := m.Message
		if len(preview) > 150 {
			preview = preview[:150] + "..."
		}
		sb.WriteString(fmt.Sprintf("<b>%s</b> <i>%s</i>\n%s\n\n",
			prefix,
			m.CreatedAt.Format("Jan 2, 15:04"),
			html.EscapeString(preview)))
	}

	if len(messages) == 0 {
		sb.WriteString("<i>No messages yet.</i>\n\n")
	}

	var rows [][]InlineButton
	if ticket.Status != "closed" {
		sb.WriteString("💬 <i>Send a message to reply...</i>")
		rows = append(rows, []InlineButton{
			{Text: "✉️ Reply", CallbackData: "support:reply:" + ticketID},
		})
	}
	rows = append(rows, backRow())

	return sb.String(), InlineKeyboardMarkup{rows}, ticket.Status != "closed"
}

// RenderReplyPrompt shows the reply prompt.
func (s *SupportHandler) RenderReplyPrompt(ticketID string) (string, InlineKeyboardMarkup) {
	text := "💬 <b>Reply to Ticket</b>\n\nType your message and send it. I'll add it to the ticket."
	return text, InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "← Cancel", CallbackData: "support:" + ticketID}},
	}}
}

// AddReply adds a user reply to a ticket.
func (s *SupportHandler) AddReply(userID, ticketID, message string) (string, error) {
	// Verify ticket belongs to user and is open
	var status string
	err := s.db.Get(&status, `
		SELECT status FROM support_tickets
		WHERE id = $1 AND user_id = $2
	`, ticketID, userID)
	if err != nil {
		return "", fmt.Errorf("ticket not found")
	}
	if status == "closed" {
		return "", fmt.Errorf("ticket is closed")
	}

	// Add the reply
	_, err = s.db.Exec(`
		INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at)
		VALUES (gen_random_uuid(), $1, $2, FALSE, $3, NOW())
	`, ticketID, userID, message)
	if err != nil {
		log.Printf("[userbot] add reply for ticket %s: %v", ticketID, err)
		return "", fmt.Errorf("could not save reply")
	}

	// Update ticket status to open if it was answered
	s.db.Exec(`UPDATE support_tickets SET status = 'open', updated_at = NOW() WHERE id = $1 AND status = 'answered'`, ticketID)

	return "✅ Reply sent", nil
}

func formatStatus(status string) string {
	switch status {
	case "open":
		return "🟢 Open"
	case "answered":
		return "🔵 Answered"
	case "closed":
		return "⚫ Closed"
	default:
		return status
	}
}

// RenderSupportMenu shows the support main menu.
func (s *SupportHandler) RenderSupportMenu(userID string) (string, InlineKeyboardMarkup) {
	// Count open tickets
	var openCount int
	s.db.Get(&openCount, `SELECT COUNT(*) FROM support_tickets WHERE user_id = $1 AND status != 'closed'`, userID)

	text := "🎫 <b>Support</b>\n\nNeed help? Create a ticket or check your existing ones."
	if openCount > 0 {
		text += fmt.Sprintf("\n\n📬 You have <b>%d</b> open ticket(s).", openCount)
	}

	return text, InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "➕ New Ticket", CallbackData: "support:new"}},
		{{Text: "📋 My Tickets", CallbackData: "support:list"}},
		backRow(),
	}}
}

// RenderTicketList shows the user's tickets.
func (s *SupportHandler) RenderTicketList(userID string) (string, InlineKeyboardMarkup) {
	var tickets []ticketRow
	s.db.Select(&tickets, `
		SELECT id, subject, status, created_at
		FROM support_tickets
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT 10
	`, userID)

	if len(tickets) == 0 {
		return "📋 <b>My Tickets</b>\n\n<i>No tickets yet.</i>", InlineKeyboardMarkup{[][]InlineButton{
			{{Text: "➕ Create Ticket", CallbackData: "support:new"}},
			{{Text: "← Back", CallbackData: ScreenSupport}},
		}}
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>My Tickets</b>\n\n")

	var rows [][]InlineButton
	for _, t := range tickets {
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n<i>%s</i>\n\n",
			formatStatus(t.Status),
			html.EscapeString(truncate(t.Subject, 40)),
			t.CreatedAt.Format("Jan 2, 2006")))
		rows = append(rows, []InlineButton{
			{Text: truncate(t.Subject, 30), CallbackData: "support:" + t.ID},
		})
	}

	rows = append(rows, []InlineButton{{Text: "← Back", CallbackData: ScreenSupport}})
	return sb.String(), InlineKeyboardMarkup{rows}
}

// RenderNewTicketSubject prompts for ticket subject.
func (s *SupportHandler) RenderNewTicketSubject() (string, InlineKeyboardMarkup) {
	text := "➕ <b>New Ticket</b>\n\nWhat's the issue about?\n\n<i>Send a short subject (5-100 characters).</i>"
	return text, InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "← Cancel", CallbackData: ScreenSupport}},
	}}
}

// RenderNewTicketMessage prompts for ticket message.
func (s *SupportHandler) RenderNewTicketMessage(subject string) (string, InlineKeyboardMarkup) {
	text := fmt.Sprintf("➕ <b>New Ticket</b>\n\nSubject: <b>%s</b>\n\nNow describe your issue in detail.\n\n<i>Send your message (10-2000 characters).</i>",
		html.EscapeString(subject))
	return text, InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "← Cancel", CallbackData: ScreenSupport}},
	}}
}

// CreateTicket creates a new support ticket.
func (s *SupportHandler) CreateTicket(userID, subject, message string) (string, error) {
	// Validate
	if len(subject) < 5 || len(subject) > 100 {
		return "", fmt.Errorf("subject must be 5-100 characters")
	}
	if len(message) < 10 || len(message) > 2000 {
		return "", fmt.Errorf("message must be 10-2000 characters")
	}

	// Create ticket
	var ticketID string
	err := s.db.Get(&ticketID, `
		INSERT INTO support_tickets (id, user_id, category, subject, status, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'general', $2, 'open', NOW(), NOW())
		RETURNING id
	`, userID, subject)
	if err != nil {
		log.Printf("[userbot] create ticket: %v", err)
		return "", fmt.Errorf("could not create ticket")
	}

	// Create first message
	_, err = s.db.Exec(`
		INSERT INTO ticket_messages (id, ticket_id, user_id, is_admin, message, created_at)
		VALUES (gen_random_uuid(), $1, $2, FALSE, $3, NOW())
	`, ticketID, userID, message)
	if err != nil {
		log.Printf("[userbot] create ticket message: %v", err)
		return "", fmt.Errorf("could not save message")
	}

	return ticketID, nil
}

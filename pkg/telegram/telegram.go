package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	botToken string
	chatID   string
	client   = &http.Client{Timeout: 10 * time.Second}
)

func init() {
	botToken = os.Getenv("TG_DEPLOY_BOT_TOKEN")
	chatID = os.Getenv("TG_DEPLOY_CHAT_ID")
}

// IsConfigured returns true if Telegram notifications are configured
func IsConfigured() bool {
	return botToken != "" && chatID != ""
}

// SendMessage sends a message to the configured Telegram chat
func SendMessage(text string) error {
	if !IsConfigured() {
		return nil // Silently skip if not configured
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// SendMessageAsync sends a message without blocking
func SendMessageAsync(text string) {
	go func() {
		_ = SendMessage(text)
	}()
}

// NotifyNewTicket sends notification for a new support ticket
func NotifyNewTicket(email, category, subject, message string) {
	if !IsConfigured() {
		return
	}

	// Truncate message if too long
	preview := message
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	text := fmt.Sprintf(
		"🎫 <b>New Support Ticket</b>\n\n"+
			"<b>From:</b> <code>%s</code>\n"+
			"<b>Category:</b> %s\n"+
			"<b>Subject:</b> %s\n\n"+
			"%s",
		email, category, subject, preview,
	)

	SendMessageAsync(text)
}

// NotifyTicketReply sends notification when user replies to a ticket
func NotifyTicketReply(ticketID, email, subject, message string) {
	if !IsConfigured() {
		return
	}

	// Truncate message if too long
	preview := message
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}

	text := fmt.Sprintf(
		"💬 <b>Ticket Reply</b>\n\n"+
			"<b>From:</b> <code>%s</code>\n"+
			"<b>Subject:</b> %s\n\n"+
			"%s",
		email, subject, preview,
	)

	SendMessageAsync(text)
}

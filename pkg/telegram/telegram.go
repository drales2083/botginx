package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Channel represents a Telegram bot + chat destination
type Channel struct {
	BotToken string
	ChatID   string
}

var (
	// DeployChannel for deployment notifications
	DeployChannel Channel
	// SupportChannel for support ticket notifications
	SupportChannel Channel
	client         = &http.Client{Timeout: 10 * time.Second}
)

func init() {
	DeployChannel = Channel{
		BotToken: os.Getenv("TG_DEPLOY_BOT_TOKEN"),
		ChatID:   os.Getenv("TG_DEPLOY_CHAT_ID"),
	}
	SupportChannel = Channel{
		BotToken: os.Getenv("TG_SUPPORT_BOT_TOKEN"),
		ChatID:   os.Getenv("TG_SUPPORT_CHAT_ID"),
	}
}

// IsConfigured returns true if the channel has both token and chat ID
func (c Channel) IsConfigured() bool {
	return c.BotToken != "" && c.ChatID != ""
}

// SendMessage sends a message to this channel
func (c Channel) SendMessage(text string) error {
	if !c.IsConfigured() {
		return nil // Silently skip if not configured
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.BotToken)

	payload := map[string]interface{}{
		"chat_id":    c.ChatID,
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
func (c Channel) SendMessageAsync(text string) {
	go func() {
		_ = c.SendMessage(text)
	}()
}

// NotifyNewTicket sends notification for a new support ticket
func NotifyNewTicket(email, category, subject, message string) {
	if !SupportChannel.IsConfigured() {
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

	SupportChannel.SendMessageAsync(text)
}

// NotifyTicketReply sends notification when user replies to a ticket
func NotifyTicketReply(ticketID, email, subject, message string) {
	if !SupportChannel.IsConfigured() {
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

	SupportChannel.SendMessageAsync(text)
}

// Legacy compatibility - send to deploy channel
func IsConfigured() bool {
	return DeployChannel.IsConfigured()
}

func SendMessage(text string) error {
	return DeployChannel.SendMessage(text)
}

func SendMessageAsync(text string) {
	DeployChannel.SendMessageAsync(text)
}

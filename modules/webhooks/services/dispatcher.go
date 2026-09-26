package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/botginx/botginx/modules/webhooks/models"
	"github.com/jmoiron/sqlx"
)

type Event struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type Dispatcher struct {
	db      *sqlx.DB
	client  *http.Client
	service *WebhooksService
}

func NewDispatcher(db *sqlx.DB, service *WebhooksService) *Dispatcher {
	return &Dispatcher{
		db:      db,
		service: service,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (d *Dispatcher) Dispatch(userID string, eventType string, data interface{}) {
	webhooks, err := d.service.GetWebhooksForEvent(userID, eventType)
	if err != nil || len(webhooks) == 0 {
		return
	}

	idBytes := make([]byte, 16)
	rand.Read(idBytes)
	eventID := hex.EncodeToString(idBytes)

	event := &Event{
		ID:        eventID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	for _, webhook := range webhooks {
		go d.deliver(&webhook, event)
	}
}

func (d *Dispatcher) deliver(webhook *models.Webhook, event *Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	var signature string
	secretHash, _ := d.service.GetSecretHash(webhook.ID)
	if secretHash != nil && *secretHash != "" {
		signature = d.signPayload(payload, *secretHash)
	}

	var responseStatus *int
	var responseBody *string
	success := false

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}

		status, body, err := d.sendRequest(webhook.URL, payload, signature)
		responseStatus = &status
		if body != "" {
			responseBody = &body
		}

		if err == nil && status >= 200 && status < 300 {
			success = true
			break
		}
	}

	deliveryID := make([]byte, 8)
	rand.Read(deliveryID)

	d.db.Exec(`
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, response_status, response_body, delivered_at, success)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
	`, hex.EncodeToString(deliveryID), webhook.ID, event.Type, string(payload), responseStatus, responseBody, success)

	if success {
		d.db.Exec(`UPDATE webhooks SET last_triggered_at = NOW(), failure_count = 0 WHERE id = $1`, webhook.ID)
	} else {
		d.db.Exec(`UPDATE webhooks SET failure_count = failure_count + 1 WHERE id = $1`, webhook.ID)
	}
}

func (d *Dispatcher) sendRequest(url string, payload []byte, signature string) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Botginx-Webhook/1.0")
	if signature != "" {
		req.Header.Set("X-Webhook-Signature", signature)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(bodyBytes), nil
}

func (d *Dispatcher) signPayload(payload []byte, secretHash string) string {
	mac := hmac.New(sha256.New, []byte(secretHash))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (d *Dispatcher) SendTestEvent(webhook *models.Webhook) error {
	idBytes := make([]byte, 16)
	rand.Read(idBytes)

	event := &Event{
		ID:        hex.EncodeToString(idBytes),
		Type:      "test",
		Timestamp: time.Now().UTC(),
		Data: map[string]interface{}{
			"message": "This is a test webhook event",
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	var signature string
	secretHash, _ := d.service.GetSecretHash(webhook.ID)
	if secretHash != nil && *secretHash != "" {
		signature = d.signPayload(payload, *secretHash)
	}

	status, body, err := d.sendRequest(webhook.URL, payload, signature)

	deliveryID := make([]byte, 8)
	rand.Read(deliveryID)

	success := err == nil && status >= 200 && status < 300

	d.db.Exec(`
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, response_status, response_body, delivered_at, success)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
	`, hex.EncodeToString(deliveryID), webhook.ID, "test", string(payload), &status, &body, success)

	if !success {
		if err != nil {
			return fmt.Errorf("delivery failed: %w", err)
		}
		return fmt.Errorf("delivery failed with status %d", status)
	}

	return nil
}

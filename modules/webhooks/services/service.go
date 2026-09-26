package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/botginx/botginx/modules/webhooks/models"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var ValidEvents = []string{
	"link.created",
	"link.updated",
	"link.deleted",
	"link.visited",
	"bot.blocked",
	"domain.verified",
	"domain.ssl_ready",
}

type WebhooksService struct {
	db *sqlx.DB
}

func NewWebhooksService(db *sqlx.DB) *WebhooksService {
	return &WebhooksService{db: db}
}

func (s *WebhooksService) List(userID string) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := s.db.Select(&webhooks, `
		SELECT id, user_id, url, events, secret_hash, is_active,
		       last_triggered_at, failure_count, created_at
		FROM webhooks
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	return webhooks, err
}

func (s *WebhooksService) Get(userID, webhookID string) (*models.Webhook, error) {
	var webhook models.Webhook
	err := s.db.Get(&webhook, `
		SELECT id, user_id, url, events, secret_hash, is_active,
		       last_triggered_at, failure_count, created_at
		FROM webhooks
		WHERE id = $1 AND user_id = $2
	`, webhookID, userID)
	if err != nil {
		return nil, err
	}
	return &webhook, nil
}

func (s *WebhooksService) Create(userID string, req *models.CreateWebhookRequest) (*models.Webhook, error) {
	if req.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if len(req.Events) == 0 {
		return nil, fmt.Errorf("at least one event is required")
	}

	if err := s.ValidateEvents(req.Events); err != nil {
		return nil, err
	}

	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)

	var secretHash *string
	if req.Secret != "" {
		hash := sha256.Sum256([]byte(req.Secret))
		h := hex.EncodeToString(hash[:])
		secretHash = &h
	}

	webhook := &models.Webhook{
		ID:       id,
		UserID:   userID,
		URL:      req.URL,
		Events:   req.Events,
		IsActive: true,
	}

	_, err := s.db.Exec(`
		INSERT INTO webhooks (id, user_id, url, events, secret_hash, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, webhook.ID, webhook.UserID, webhook.URL, pq.Array(webhook.Events), secretHash, webhook.IsActive)

	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return webhook, nil
}

func (s *WebhooksService) Update(userID, webhookID string, req *models.UpdateWebhookRequest) (*models.Webhook, error) {
	webhook, err := s.Get(userID, webhookID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found")
	}

	if req.URL != nil {
		webhook.URL = *req.URL
	}
	if req.Events != nil {
		if err := s.ValidateEvents(req.Events); err != nil {
			return nil, err
		}
		webhook.Events = req.Events
	}
	if req.IsActive != nil {
		webhook.IsActive = *req.IsActive
	}

	query := `UPDATE webhooks SET url = $1, events = $2, is_active = $3`
	args := []interface{}{webhook.URL, pq.Array(webhook.Events), webhook.IsActive}

	if req.Secret != nil {
		if *req.Secret == "" {
			query += `, secret_hash = NULL`
		} else {
			hash := sha256.Sum256([]byte(*req.Secret))
			h := hex.EncodeToString(hash[:])
			query += fmt.Sprintf(`, secret_hash = $%d`, len(args)+1)
			args = append(args, h)
		}
	}

	query += fmt.Sprintf(` WHERE id = $%d AND user_id = $%d`, len(args)+1, len(args)+2)
	args = append(args, webhookID, userID)

	_, err = s.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}

	return s.Get(userID, webhookID)
}

func (s *WebhooksService) Delete(userID, webhookID string) error {
	result, err := s.db.Exec(`
		DELETE FROM webhooks WHERE id = $1 AND user_id = $2
	`, webhookID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

func (s *WebhooksService) ValidateEvents(events []string) error {
	validSet := make(map[string]bool)
	for _, e := range ValidEvents {
		validSet[e] = true
	}
	for _, e := range events {
		if !validSet[e] {
			return fmt.Errorf("invalid event: %s", e)
		}
	}
	return nil
}

func (s *WebhooksService) ListDeliveries(userID, webhookID string, limit int) ([]models.WebhookDelivery, error) {
	_, err := s.Get(userID, webhookID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found")
	}

	if limit <= 0 {
		limit = 20
	}

	var deliveries []models.WebhookDelivery
	err = s.db.Select(&deliveries, `
		SELECT id, webhook_id, event_type, payload, response_status,
		       response_body, delivered_at, success
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY delivered_at DESC
		LIMIT $2
	`, webhookID, limit)

	return deliveries, err
}

func (s *WebhooksService) GetByID(webhookID string) (*models.Webhook, error) {
	var webhook models.Webhook
	err := s.db.Get(&webhook, `
		SELECT id, user_id, url, events, secret_hash, is_active,
		       last_triggered_at, failure_count, created_at
		FROM webhooks
		WHERE id = $1
	`, webhookID)
	if err != nil {
		return nil, err
	}
	return &webhook, nil
}

func (s *WebhooksService) GetWebhooksForEvent(userID, eventType string) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := s.db.Select(&webhooks, `
		SELECT id, user_id, url, events, secret_hash, is_active,
		       last_triggered_at, failure_count, created_at
		FROM webhooks
		WHERE user_id = $1 AND is_active = true AND $2 = ANY(events)
	`, userID, eventType)
	return webhooks, err
}

func (s *WebhooksService) GetSecretHash(webhookID string) (*string, error) {
	var secretHash *string
	err := s.db.Get(&secretHash, `SELECT secret_hash FROM webhooks WHERE id = $1`, webhookID)
	return secretHash, err
}

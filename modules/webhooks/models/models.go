package models

import (
	"time"

	"github.com/lib/pq"
)

type Webhook struct {
	ID              string         `db:"id" json:"id"`
	UserID          string         `db:"user_id" json:"-"`
	URL             string         `db:"url" json:"url"`
	Events          pq.StringArray `db:"events" json:"events"`
	SecretHash      *string        `db:"secret_hash" json:"-"`
	IsActive        bool           `db:"is_active" json:"isActive"`
	LastTriggeredAt *time.Time     `db:"last_triggered_at" json:"lastTriggeredAt"`
	FailureCount    int            `db:"failure_count" json:"failureCount"`
	CreatedAt       time.Time      `db:"created_at" json:"createdAt"`
}

type WebhookDelivery struct {
	ID             string    `db:"id" json:"id"`
	WebhookID      string    `db:"webhook_id" json:"webhookId"`
	EventType      string    `db:"event_type" json:"eventType"`
	Payload        string    `db:"payload" json:"payload"`
	ResponseStatus *int      `db:"response_status" json:"responseStatus"`
	ResponseBody   *string   `db:"response_body" json:"responseBody"`
	DeliveredAt    time.Time `db:"delivered_at" json:"deliveredAt"`
	Success        bool      `db:"success" json:"success"`
}

type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret,omitempty"`
}

type UpdateWebhookRequest struct {
	URL      *string  `json:"url,omitempty"`
	Events   []string `json:"events,omitempty"`
	IsActive *bool    `json:"isActive,omitempty"`
	Secret   *string  `json:"secret,omitempty"`
}

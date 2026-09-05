package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/botginx/botginx/pkg/protection"
)

// JSONStringArray for storing string arrays in PostgreSQL JSONB
type JSONStringArray []string

func (j JSONStringArray) Value() (driver.Value, error) {
	if j == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(j)
}

func (j *JSONStringArray) Scan(value interface{}) error {
	if value == nil {
		*j = []string{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	}
	return json.Unmarshal(data, j)
}

// ShortLink represents a shortened URL
type ShortLink struct {
	ID                 string              `db:"id" json:"id"`
	UserID             string              `db:"user_id" json:"userId"`
	DomainID           string              `db:"domain_id" json:"domainId"`
	DomainName         string              `db:"domain_name" json:"domainName"` // joined from domains
	Path               string              `db:"path" json:"path"`
	Destinations       JSONStringArray     `db:"destinations" json:"destinations"`
	RotationMode       string              `db:"rotation_mode" json:"rotationMode"` // random, sequential
	BotError           int                 `db:"bot_error" json:"botError"`
	QREnabled          bool                `db:"qr_enabled" json:"qrEnabled"`
	ProtectionSettings protection.Settings `db:"protection_settings" json:"protectionSettings"`
	ClickCount         int                 `db:"click_count" json:"clickCount"`
	HumanCount         int                 `db:"human_count" json:"humanCount"`
	BotCount           int                 `db:"bot_count" json:"botCount"`
	LastClickAt        *time.Time          `db:"last_click_at" json:"lastClickAt"`
	DeployStatus       string              `db:"deploy_status" json:"deployStatus"`
	DeployedURL        *string             `db:"deployed_url" json:"deployedUrl"`
	DeployError        *string             `db:"deploy_error" json:"deployError"`
	IsActive           bool                `db:"is_active" json:"isActive"`
	CreatedAt          time.Time           `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time           `db:"updated_at" json:"updatedAt"`
}

// FullURL returns the complete short link URL
func (s *ShortLink) FullURL() string {
	if s.DomainName != "" {
		return "https://" + s.DomainName + "/" + s.Path
	}
	return ""
}

// ShortLinkClick represents a single click/visit
type ShortLinkClick struct {
	ID              string    `db:"id" json:"id"`
	LinkID          string    `db:"link_id" json:"linkId"`
	VisitorIPHash   string    `db:"visitor_ip_hash" json:"visitorIpHash"`
	Country         string    `db:"country" json:"country"`
	City            string    `db:"city" json:"city"`
	Device          string    `db:"device" json:"device"`
	Browser         string    `db:"browser" json:"browser"`
	OS              string    `db:"os" json:"os"`
	IsBot           bool      `db:"is_bot" json:"isBot"`
	BotType         string    `db:"bot_type" json:"botType"`
	BotReason       string    `db:"bot_reason" json:"botReason"`
	DestinationUsed string    `db:"destination_used" json:"destinationUsed"`
	Referer         string    `db:"referer" json:"referer"`
	UserAgent       string    `db:"user_agent" json:"userAgent"`
	CreatedAt       time.Time `db:"created_at" json:"createdAt"`
}

// CreateShortLinkInput for creating new short links
type CreateShortLinkInput struct {
	DomainID           string              `json:"domainId"`
	Path               string              `json:"path"`
	Destinations       []string            `json:"destinations"`
	RotationMode       string              `json:"rotationMode"`
	BotError           int                 `json:"botError"`
	QREnabled          bool                `json:"qrEnabled"`
	ProtectionSettings protection.Settings `json:"protectionSettings"`
}

// UpdateShortLinkInput for updating short links
type UpdateShortLinkInput struct {
	Destinations       []string             `json:"destinations,omitempty"`
	RotationMode       *string              `json:"rotationMode,omitempty"`
	BotError           *int                 `json:"botError,omitempty"`
	QREnabled          *bool                `json:"qrEnabled,omitempty"`
	ProtectionSettings *protection.Settings `json:"protectionSettings,omitempty"`
}

// ShortLinkStats for analytics
type ShortLinkStats struct {
	TotalClicks int            `json:"totalClicks"`
	HumanClicks int            `json:"humanClicks"`
	BotClicks   int            `json:"botClicks"`
	ByCountry   map[string]int `json:"byCountry"`
	ByDevice    map[string]int `json:"byDevice"`
	ByDay       []DayStats     `json:"byDay"`
}

type DayStats struct {
	Date   string `json:"date"`
	Humans int    `json:"humans"`
	Bots   int    `json:"bots"`
}

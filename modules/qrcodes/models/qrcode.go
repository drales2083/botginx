package models

import (
	"database/sql"
	"time"
)

type QRCode struct {
	ID             string         `db:"id" json:"id"`
	UserID         string         `db:"user_id" json:"user_id"`
	Title          string         `db:"title" json:"title"`
	URL            string         `db:"url" json:"url"`
	RedirectLinkID sql.NullString `db:"redirect_link_id" json:"redirect_link_id,omitempty"`
	FGColor        string         `db:"fg_color" json:"fg_color"`
	BGColor        string         `db:"bg_color" json:"bg_color"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at"`
}

type CreateQRCodeRequest struct {
	Title          string `json:"title"`
	URL            string `json:"url"`
	RedirectLinkID string `json:"redirectLinkId,omitempty"`
	FGColor        string `json:"fgColor"`
	BGColor        string `json:"bgColor"`
}

type UpdateQRCodeRequest struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	FGColor string `json:"fgColor"`
	BGColor string `json:"bgColor"`
}

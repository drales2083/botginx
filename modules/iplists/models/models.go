package models

import "time"

type IPWhitelist struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	IP        string    `db:"ip" json:"ip"`
	Note      *string   `db:"note" json:"note,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type IPBlocklist struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	IP        string    `db:"ip" json:"ip"`
	Note      *string   `db:"note" json:"note,omitempty"`
	Source    string    `db:"source" json:"source"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

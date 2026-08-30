package models

import "time"

type GlobalWhitelist struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	IP        string    `db:"ip" json:"ip"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

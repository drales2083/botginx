package models

import "time"

type User struct {
	ID              string    `db:"id" json:"id"`
	Email           string    `db:"email" json:"email"`
	PasswordHash    string    `db:"password_hash" json:"-"`
	Name            string    `db:"name" json:"name"`
	Role            string    `db:"role" json:"role"` // "user" or "admin"
	IsActive        bool      `db:"is_active" json:"isActive"`
	Balance         float64   `db:"balance" json:"balance"`
	ReferralCode    *string   `db:"referral_code" json:"referralCode,omitempty"`
	ReferredByID    *string   `db:"referred_by_id" json:"referredById,omitempty"`
	BotsDetected    int64     `db:"bots_detected" json:"botsDetected"`
	HumansVerified  int64     `db:"humans_verified" json:"humansVerified"`
	CreatedAt       time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `db:"updated_at" json:"updatedAt"`
}

type SignupInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type Session struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	Token     string    `db:"token" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

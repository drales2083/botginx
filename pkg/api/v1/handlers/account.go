package handlers

import (
	"net/http"
	"time"

	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/jmoiron/sqlx"
)

type AccountHandler struct {
	db *sqlx.DB
}

func NewAccountHandler(db *sqlx.DB) *AccountHandler {
	return &AccountHandler{db: db}
}

// AccountInfo represents account information
type AccountInfo struct {
	ID                string  `json:"id"`
	Email             string  `json:"email"`
	Name              string  `json:"name,omitempty"`
	SubscriptionEnds  *string `json:"subscriptionEnds,omitempty"`
	SubscriptionValid bool    `json:"subscriptionValid"`
	CreatedAt         string  `json:"createdAt"`
}

// AccountUsage represents API usage statistics
type AccountUsage struct {
	APIKeyID       string `json:"apiKeyId"`
	KeyName        string `json:"keyName"`
	RequestsCount  int64  `json:"requestsCount"`
	RateLimit      int    `json:"rateLimit"`
	LastUsedAt     string `json:"lastUsedAt,omitempty"`
}

func (h *AccountHandler) Info(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("account:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: account:read", http.StatusForbidden)
		return
	}

	var user struct {
		ID               string     `db:"id"`
		Email            string     `db:"email"`
		Name             *string    `db:"name"`
		SubscriptionEnds *time.Time `db:"subscription_ends"`
		CreatedAt        time.Time  `db:"created_at"`
	}

	err := h.db.Get(&user, `
		SELECT u.id, u.email, u.name, s.expires_at AS subscription_ends, u.created_at
		FROM users u
		LEFT JOIN subscriptions s ON s.user_id = u.id
		WHERE u.id = $1
		ORDER BY s.expires_at DESC NULLS LAST
		LIMIT 1
	`, key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Account not found", http.StatusNotFound)
		return
	}

	info := AccountInfo{
		ID:                user.ID,
		Email:             user.Email,
		SubscriptionValid: user.SubscriptionEnds != nil && user.SubscriptionEnds.After(time.Now()),
		CreatedAt:         user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if user.Name != nil {
		info.Name = *user.Name
	}

	if user.SubscriptionEnds != nil {
		ends := user.SubscriptionEnds.Format("2006-01-02T15:04:05Z")
		info.SubscriptionEnds = &ends
	}

	apiauth.Success(w, info)
}

func (h *AccountHandler) Usage(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("account:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: account:read", http.StatusForbidden)
		return
	}

	var keys []struct {
		ID            string     `db:"id"`
		Name          string     `db:"name"`
		RequestsCount int64      `db:"requests_count"`
		RateLimit     int        `db:"rate_limit"`
		LastUsedAt    *time.Time `db:"last_used_at"`
	}

	err := h.db.Select(&keys, `
		SELECT id, name, requests_count, rate_limit, last_used_at
		FROM api_keys
		WHERE user_id = $1 AND is_active = true
		ORDER BY requests_count DESC
	`, key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch usage data", http.StatusInternalServerError)
		return
	}

	result := make([]AccountUsage, 0, len(keys))
	for _, k := range keys {
		usage := AccountUsage{
			APIKeyID:      k.ID,
			KeyName:       k.Name,
			RequestsCount: k.RequestsCount,
			RateLimit:     k.RateLimit,
		}
		if k.LastUsedAt != nil {
			usage.LastUsedAt = k.LastUsedAt.Format("2006-01-02T15:04:05Z")
		}
		result = append(result, usage)
	}

	// Calculate totals
	var totalRequests int64
	for _, u := range result {
		totalRequests += u.RequestsCount
	}

	apiauth.Success(w, map[string]interface{}{
		"totalRequests": totalRequests,
		"activeKeys":    len(result),
		"keys":          result,
	})
}

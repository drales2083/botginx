package apiauth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Middleware handles API key authentication
type Middleware struct {
	db *sqlx.DB
}

// apiKeyRow represents the database row for API keys
type apiKeyRow struct {
	ID        string         `db:"id"`
	UserID    string         `db:"user_id"`
	Name      string         `db:"name"`
	Scopes    pq.StringArray `db:"scopes"`
	ExpiresAt *time.Time     `db:"expires_at"`
	RateLimit int            `db:"rate_limit"`
	IsTest    bool           `db:"is_test"`
	IsActive  bool           `db:"is_active"`
}

// NewMiddleware creates a new API authentication middleware
func NewMiddleware(db *sqlx.DB) *Middleware {
	return &Middleware{db: db}
}

// Authenticate is middleware that validates API keys
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			ErrorResp(w, "missing_api_key", "API key is required. Use Authorization: Bearer <key> or X-API-Key header.", http.StatusUnauthorized)
			return
		}

		// Validate key format
		if !strings.HasPrefix(token, "bg_live_") && !strings.HasPrefix(token, "bg_test_") {
			ErrorResp(w, "invalid_api_key", "Invalid API key format.", http.StatusUnauthorized)
			return
		}

		// Hash the token for lookup
		hash := sha256.Sum256([]byte(token))
		keyHash := hex.EncodeToString(hash[:])

		// Look up the key
		var key apiKeyRow
		err := m.db.Get(&key, `
			SELECT id, user_id, name, scopes, expires_at, rate_limit, is_test, is_active
			FROM api_keys
			WHERE key_hash = $1
		`, keyHash)

		if err != nil {
			ErrorResp(w, "invalid_api_key", "Invalid or unknown API key.", http.StatusUnauthorized)
			return
		}

		// Check if active
		if !key.IsActive {
			ErrorResp(w, "key_revoked", "This API key has been revoked.", http.StatusUnauthorized)
			return
		}

		// Check expiration
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			ErrorResp(w, "key_expired", "This API key has expired.", http.StatusUnauthorized)
			return
		}

		// Update last_used_at and requests_count asynchronously
		go m.updateUsage(key.ID)

		// Attach key info to context
		keyCtx := &APIKeyContext{
			KeyID:     key.ID,
			UserID:    key.UserID,
			Name:      key.Name,
			Scopes:    key.Scopes,
			RateLimit: key.RateLimit,
			IsTest:    key.IsTest,
		}

		next.ServeHTTP(w, SetAPIKey(r, keyCtx))
	})
}

// RequireScope returns middleware that checks for a specific scope
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetAPIKey(r)
			if key == nil {
				ErrorResp(w, "unauthorized", "Authentication required.", http.StatusUnauthorized)
				return
			}

			if !key.HasScope(scope) {
				ErrorResp(w, "insufficient_scope", "This API key does not have the '"+scope+"' scope.", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractToken gets the API token from the request
func extractToken(r *http.Request) string {
	// Check Authorization header first
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check X-API-Key header
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	return ""
}

// updateUsage updates last_used_at and increments requests_count
func (m *Middleware) updateUsage(keyID string) {
	m.db.Exec(`
		UPDATE api_keys
		SET last_used_at = NOW(), requests_count = requests_count + 1
		WHERE id = $1
	`, keyID)
}

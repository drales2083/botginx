package apiauth

import (
	"context"
	"net/http"

	"github.com/lib/pq"
)

type contextKey string

const apiKeyContextKey contextKey = "api_key"

// APIKeyContext holds the authenticated API key information
type APIKeyContext struct {
	KeyID     string
	UserID    string
	Name      string
	Scopes    pq.StringArray
	RateLimit int
	IsTest    bool
}

// GetAPIKey retrieves the API key context from the request
func GetAPIKey(r *http.Request) *APIKeyContext {
	if key, ok := r.Context().Value(apiKeyContextKey).(*APIKeyContext); ok {
		return key
	}
	return nil
}

// SetAPIKey adds the API key context to the request
func SetAPIKey(r *http.Request, key *APIKeyContext) *http.Request {
	ctx := context.WithValue(r.Context(), apiKeyContextKey, key)
	return r.WithContext(ctx)
}

// HasScope checks if the API key has a specific scope
func (k *APIKeyContext) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

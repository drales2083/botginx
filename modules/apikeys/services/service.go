package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type APIKeysService struct {
	db *sqlx.DB
}

func NewAPIKeysService(db *sqlx.DB) *APIKeysService {
	return &APIKeysService{db: db}
}

type APIKey struct {
	ID            string         `db:"id" json:"id"`
	UserID        string         `db:"user_id" json:"-"`
	Name          string         `db:"name" json:"name"`
	KeyPrefix     string         `db:"key_prefix" json:"keyPrefix"`
	Scopes        pq.StringArray `db:"scopes" json:"scopes"`
	ExpiresAt     *time.Time     `db:"expires_at" json:"expiresAt,omitempty"`
	RateLimit     int            `db:"rate_limit" json:"rateLimit"`
	IsTest        bool           `db:"is_test" json:"isTest"`
	IsActive      bool           `db:"is_active" json:"isActive"`
	LastUsedAt    *time.Time     `db:"last_used_at" json:"lastUsedAt,omitempty"`
	RequestsCount int64          `db:"requests_count" json:"requestsCount"`
	CreatedAt     time.Time      `db:"created_at" json:"createdAt"`
}

type CreateKeyRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expiresAt,omitempty"`
	RateLimit int      `json:"rateLimit"`
	IsTest    bool     `json:"isTest"`
}

type KeyWithSecret struct {
	APIKey
	SecretKey string `json:"secretKey"`
}

var ValidScopes = map[string]string{
	"links:read":      "Read redirect links",
	"links:write":     "Create and update redirect links",
	"links:delete":    "Delete redirect links",
	"domains:read":    "Read domains",
	"domains:manage":  "Add, remove, and configure domains",
	"analytics:read":  "Read analytics data",
	"qrcodes:read":    "Read QR codes",
	"qrcodes:write":   "Create and update QR codes",
	"shortener:read":  "Read short links",
	"shortener:write": "Create and update short links",
	"iplists:read":    "Read IP whitelist/blocklist",
	"iplists:write":   "Manage IP whitelist/blocklist",
	"webhooks:manage": "Manage webhook endpoints",
	"account:read":    "Read account information",
}

var ScopePresets = map[string][]string{
	"readonly": {"links:read", "domains:read", "analytics:read", "qrcodes:read", "shortener:read", "iplists:read", "account:read"},
	"standard": {"links:read", "links:write", "domains:read", "analytics:read", "qrcodes:read", "qrcodes:write", "shortener:read", "shortener:write", "iplists:read", "iplists:write", "account:read"},
	"full":     {"links:read", "links:write", "links:delete", "domains:read", "domains:manage", "analytics:read", "qrcodes:read", "qrcodes:write", "shortener:read", "shortener:write", "iplists:read", "iplists:write", "webhooks:manage", "account:read"},
}

func (s *APIKeysService) List(userID string) ([]APIKey, error) {
	var keys []APIKey
	err := s.db.Select(&keys, `
		SELECT id, user_id, name, key_prefix, scopes, expires_at, rate_limit,
		       is_test, is_active, last_used_at, requests_count, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	return keys, err
}

func (s *APIKeysService) Create(userID string, req *CreateKeyRequest) (*KeyWithSecret, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(req.Scopes) == 0 {
		return nil, fmt.Errorf("at least one scope is required")
	}

	// Validate scopes
	for _, scope := range req.Scopes {
		if _, ok := ValidScopes[scope]; !ok {
			return nil, fmt.Errorf("invalid scope: %s", scope)
		}
	}

	// Generate key
	prefix := "bg_live_"
	if req.IsTest {
		prefix = "bg_test_"
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	randomPart := hex.EncodeToString(randomBytes)
	fullKey := prefix + randomPart

	// Hash the key for storage
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	// Key prefix for display (prefix + last 4 chars)
	keyPrefix := prefix + "****" + randomPart[len(randomPart)-4:]

	// Default rate limit
	if req.RateLimit <= 0 {
		req.RateLimit = 1000
	}

	// Parse expiration
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse("2006-01-02", *req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration date format")
		}
		expiresAt = &t
	}

	// Generate ID
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)

	key := &APIKey{
		ID:        id,
		UserID:    userID,
		Name:      req.Name,
		KeyPrefix: keyPrefix,
		Scopes:    req.Scopes,
		ExpiresAt: expiresAt,
		RateLimit: req.RateLimit,
		IsTest:    req.IsTest,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, scopes, expires_at, rate_limit, is_test, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, key.ID, key.UserID, key.Name, keyHash, key.KeyPrefix, pq.Array(key.Scopes),
		key.ExpiresAt, key.RateLimit, key.IsTest, key.IsActive, key.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	return &KeyWithSecret{
		APIKey:    *key,
		SecretKey: fullKey,
	}, nil
}

func (s *APIKeysService) Get(userID, keyID string) (*APIKey, error) {
	var key APIKey
	err := s.db.Get(&key, `
		SELECT id, user_id, name, key_prefix, scopes, expires_at, rate_limit,
		       is_test, is_active, last_used_at, requests_count, created_at
		FROM api_keys
		WHERE id = $1 AND user_id = $2
	`, keyID, userID)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *APIKeysService) Revoke(userID, keyID string) error {
	result, err := s.db.Exec(`
		UPDATE api_keys SET is_active = false WHERE id = $1 AND user_id = $2
	`, keyID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("key not found")
	}
	return nil
}

func (s *APIKeysService) Delete(userID, keyID string) error {
	result, err := s.db.Exec(`
		DELETE FROM api_keys WHERE id = $1 AND user_id = $2
	`, keyID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("key not found")
	}
	return nil
}

func (s *APIKeysService) Rotate(userID, keyID string) (*KeyWithSecret, error) {
	// Get existing key
	existing, err := s.Get(userID, keyID)
	if err != nil {
		return nil, fmt.Errorf("key not found")
	}

	// Create new key with same settings
	expStr := ""
	if existing.ExpiresAt != nil {
		expStr = existing.ExpiresAt.Format("2006-01-02")
	}

	newKey, err := s.Create(userID, &CreateKeyRequest{
		Name:      existing.Name + " (rotated)",
		Scopes:    existing.Scopes,
		ExpiresAt: &expStr,
		RateLimit: existing.RateLimit,
		IsTest:    existing.IsTest,
	})
	if err != nil {
		return nil, err
	}

	// Revoke old key
	s.Revoke(userID, keyID)

	return newKey, nil
}

func (s *APIKeysService) ValidateKey(apiKey string) (*APIKey, error) {
	// Check format
	if !strings.HasPrefix(apiKey, "bg_live_") && !strings.HasPrefix(apiKey, "bg_test_") {
		return nil, fmt.Errorf("invalid key format")
	}

	// Hash the provided key
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])

	var key APIKey
	err := s.db.Get(&key, `
		SELECT id, user_id, name, key_prefix, scopes, expires_at, rate_limit,
		       is_test, is_active, last_used_at, requests_count, created_at
		FROM api_keys
		WHERE key_hash = $1 AND is_active = true
	`, keyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid or inactive key")
	}

	// Check expiration
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("key has expired")
	}

	// Update last used and request count
	s.db.Exec(`
		UPDATE api_keys SET last_used_at = NOW(), requests_count = requests_count + 1
		WHERE id = $1
	`, key.ID)

	return &key, nil
}

func (s *APIKeysService) HasScope(key *APIKey, scope string) bool {
	for _, s := range key.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

package signedurl

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	keyOnce    sync.Once
	keyError   error
)

// loadKeys loads the Ed25519 keypair from environment or generates a new one
func loadKeys() {
	keyOnce.Do(func() {
		privKeyB64 := os.Getenv("SIGNED_URL_PRIVATE_KEY")
		if privKeyB64 != "" {
			privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
			if err != nil {
				log.Error().Err(err).Msg("failed to decode SIGNED_URL_PRIVATE_KEY")
				keyError = err
				return
			}
			if len(privKeyBytes) != ed25519.PrivateKeySize {
				log.Error().Int("got", len(privKeyBytes)).Int("want", ed25519.PrivateKeySize).Msg("invalid SIGNED_URL_PRIVATE_KEY size")
				keyError = err
				return
			}
			privateKey = ed25519.PrivateKey(privKeyBytes)
			publicKey = privateKey.Public().(ed25519.PublicKey)
			log.Info().Str("publicKey", base64.StdEncoding.EncodeToString(publicKey)).Msg("loaded signed URL keypair from environment")
		} else {
			// Generate new keypair for development
			var err error
			publicKey, privateKey, err = ed25519.GenerateKey(nil)
			if err != nil {
				log.Error().Err(err).Msg("failed to generate signed URL keypair")
				keyError = err
				return
			}
			log.Warn().
				Str("privateKey", base64.StdEncoding.EncodeToString(privateKey)).
				Str("publicKey", base64.StdEncoding.EncodeToString(publicKey)).
				Msg("generated new signed URL keypair (set SIGNED_URL_PRIVATE_KEY in production)")
		}
	})
}

// GetPublicKey returns the public key in base64 format
func GetPublicKey() string {
	loadKeys()
	if publicKey == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(publicKey)
}

// GetPublicKeyBytes returns the raw public key bytes
func GetPublicKeyBytes() []byte {
	loadKeys()
	return publicKey
}

// IsConfigured returns true if signing is properly configured
func IsConfigured() bool {
	loadKeys()
	return privateKey != nil && keyError == nil
}

// GetPrivateKeyBase64 returns the private key as base64 for embedding in PHP.
// Returns empty string if signing is not configured.
func GetPrivateKeyBase64() string {
	loadKeys()
	if privateKey == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(privateKey)
}

// SignURL appends verification signature to a destination URL
// Parameters:
//   - destURL: the destination URL to sign
//   - linkID: the redirect link ID (for user reference)
//   - expirySeconds: how long the signature is valid (default 300)
//
// Returns the signed URL with _bl, _bt, and _bs parameters
func SignURL(destURL string, linkID string, expirySeconds int) (string, error) {
	loadKeys()
	if privateKey == nil {
		return destURL, keyError
	}

	if expirySeconds <= 0 {
		expirySeconds = 300
	}

	// Parse destination URL
	u, err := url.Parse(destURL)
	if err != nil {
		return destURL, err
	}

	// Get existing query params
	q := u.Query()

	// Add link ID and timestamp
	q.Set("_bl", linkID)
	q.Set("_bt", strconv.FormatInt(time.Now().Unix(), 10))

	// Sort and rebuild query string (for deterministic signing)
	u.RawQuery = sortedQueryString(q)

	// Sign the URL (without _bs)
	message := []byte(u.String())
	signature := ed25519.Sign(privateKey, message)

	// Append signature (base64url encoding, no padding)
	q.Set("_bs", base64.RawURLEncoding.EncodeToString(signature))
	u.RawQuery = sortedQueryString(q)

	return u.String(), nil
}

// VerifyURL verifies a signed URL (for testing/internal use)
// In production, this verification happens on the user's server
func VerifyURL(signedURL string, maxAgeSeconds int) bool {
	loadKeys()
	if publicKey == nil {
		return false
	}

	if maxAgeSeconds <= 0 {
		maxAgeSeconds = 300
	}

	// Parse URL
	u, err := url.Parse(signedURL)
	if err != nil {
		return false
	}

	q := u.Query()

	// Extract signature
	sigB64 := q.Get("_bs")
	if sigB64 == "" {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}

	// Check timestamp
	timestampStr := q.Get("_bt")
	if timestampStr == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-timestamp > int64(maxAgeSeconds) {
		return false // Expired
	}

	// Rebuild URL without signature
	q.Del("_bs")
	u.RawQuery = sortedQueryString(q)
	message := []byte(u.String())

	// Verify signature
	return ed25519.Verify(publicKey, message, signature)
}

// sortedQueryString returns a query string with parameters sorted alphabetically
// This ensures deterministic signing regardless of original parameter order
func sortedQueryString(q url.Values) string {
	if len(q) == 0 {
		return ""
	}

	// Get sorted keys
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var parts []string
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

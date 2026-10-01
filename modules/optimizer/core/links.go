package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"time"
)

var (
	// ErrTokenInvalid means the token was malformed or its signature failed.
	ErrTokenInvalid = errors.New("token invalid")
	// ErrTokenExpired means the token's embedded expiry has passed.
	ErrTokenExpired = errors.New("token expired")
)

// SignToken encodes rawURL and an expiry into a URL-safe, HMAC-signed token.
// It refuses URLs that are not http/https.
func SignToken(key []byte, rawURL string, ttl time.Duration) (string, error) {
	if len(key) == 0 {
		return "", ErrTokenInvalid
	}
	rawURL = strings.TrimSpace(rawURL)
	if !IsHTTPURL(rawURL) {
		return "", ErrTokenInvalid
	}
	exp := time.Now().Add(ttl).Unix()
	payload := make([]byte, 8+len(rawURL))
	binary.BigEndian.PutUint64(payload[:8], uint64(exp))
	copy(payload[8:], rawURL)

	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	sig := mac.Sum(nil)

	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(sig), nil
}

// VerifyToken checks the signature and expiry and returns the embedded URL.
func VerifyToken(key []byte, token string) (string, error) {
	if len(key) == 0 {
		return "", ErrTokenInvalid
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", ErrTokenInvalid
	}
	enc := base64.RawURLEncoding
	payload, err := enc.DecodeString(parts[0])
	if err != nil || len(payload) < 8 {
		return "", ErrTokenInvalid
	}
	sig, err := enc.DecodeString(parts[1])
	if err != nil {
		return "", ErrTokenInvalid
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", ErrTokenInvalid
	}
	exp := int64(binary.BigEndian.Uint64(payload[:8]))
	if time.Now().Unix() > exp {
		return "", ErrTokenExpired
	}
	url := string(payload[8:])
	if !IsHTTPURL(url) {
		return "", ErrTokenInvalid
	}
	return url, nil
}

// IsHTTPURL reports whether raw is an http or https URL.
func IsHTTPURL(raw string) bool {
	l := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// NeutralizeURL returns a non-navigable form of raw. http/https are defanged to
// hxxp/hxxps; any other scheme becomes "#".
func NeutralizeURL(raw string) string {
	t := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(strings.ToLower(t), "https://"):
		return "hxxps://" + t[len("https://"):]
	case strings.HasPrefix(strings.ToLower(t), "http://"):
		return "hxxp://" + t[len("http://"):]
	default:
		return "#"
	}
}

// WrapURL joins the redirector base and a token.
func WrapURL(base, token string) string {
	return strings.TrimRight(base, "/") + "/" + token
}

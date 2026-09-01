// Package crypto provides AES-256-GCM encryption for securely storing passwords.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"sync"
)

var (
	encryptionKey []byte
	keyOnce       sync.Once
)

// initKey loads the encryption key from environment on first use
func initKey() {
	keyOnce.Do(func() {
		key := os.Getenv("HOSTING_ENCRYPTION_KEY")
		if key != "" {
			encryptionKey = []byte(key)
		}
	})
}

// SetKey allows setting the encryption key programmatically (for testing)
func SetKey(key []byte) {
	encryptionKey = key
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64-encoded ciphertext
func Encrypt(plaintext string) (string, error) {
	initKey()
	if len(encryptionKey) != 32 {
		return "", errors.New("HOSTING_ENCRYPTION_KEY must be 32 bytes")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM
func Decrypt(ciphertext string) (string, error) {
	initKey()
	if len(encryptionKey) != 32 {
		return "", errors.New("HOSTING_ENCRYPTION_KEY must be 32 bytes")
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GeneratePassword creates a secure random password of the specified length.
// Uses alphanumeric characters plus symbols: !@#$%^&*
func GeneratePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// GenerateUsername creates a username with the given prefix followed by 5 random alphanumeric chars.
// Example: GenerateUsername("bp_") might return "bp_x7k2m"
func GenerateUsername(prefix string) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return prefix + string(b)
}

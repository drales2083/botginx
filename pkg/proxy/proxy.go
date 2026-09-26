package proxy

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// Config holds the proxy configuration
type Config struct {
	ID             int       `db:"id" json:"id"`
	Enabled        bool      `db:"enabled" json:"enabled"`
	Username       string    `db:"username" json:"username"`
	Password       string    `db:"password" json:"-"`
	Host           string    `db:"host" json:"host"`
	Port           int       `db:"port" json:"port"`
	Zone           string    `db:"zone" json:"zone"`
	SessionMinutes int       `db:"session_minutes" json:"session_minutes"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// Service handles proxy configuration
type Service struct {
	db    *sqlx.DB
	mu    sync.RWMutex
	cache *Config
}

// NewService creates a new proxy service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// EnsureTable creates the proxy_config table if it doesn't exist
func (s *Service) EnsureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS proxy_config (
			id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			enabled BOOLEAN DEFAULT false,
			username VARCHAR(100),
			password VARCHAR(100),
			host VARCHAR(255) DEFAULT 'global.rotgb.711proxy.com',
			port INTEGER DEFAULT 10000,
			zone VARCHAR(50) DEFAULT 'custom',
			session_minutes INTEGER DEFAULT 10,
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	// Insert default row if not exists
	_, err = s.db.Exec(`
		INSERT INTO proxy_config (id, enabled)
		VALUES (1, false)
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

// Get returns the current proxy configuration
func (s *Service) Get() (*Config, error) {
	s.mu.RLock()
	if s.cache != nil {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	var config Config
	err := s.db.Get(&config, "SELECT * FROM proxy_config WHERE id = 1")
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache = &config
	s.mu.Unlock()

	return &config, nil
}

// Update updates the proxy configuration
func (s *Service) Update(config *Config) error {
	_, err := s.db.Exec(`
		INSERT INTO proxy_config (id, enabled, username, password, host, port, zone, session_minutes, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (id) DO UPDATE SET
			enabled = $1,
			username = $2,
			password = $3,
			host = $4,
			port = $5,
			zone = $6,
			session_minutes = $7,
			updated_at = NOW()
	`, config.Enabled, config.Username, config.Password, config.Host, config.Port, config.Zone, config.SessionMinutes)

	if err != nil {
		return err
	}

	// Clear cache
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()

	return nil
}

// generateSessionID generates a random session ID for sticky sessions
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// BuildProxyURL builds the full proxy URL with session
// Format: http://USER-zone-custom-region-us-session-{random}-sessTime-10:password@host:port
func (c *Config) BuildProxyURL() string {
	if c.Username == "" || c.Password == "" || c.Host == "" {
		return ""
	}

	sessionID := generateSessionID()
	// Always use US region to avoid geo-blocks
	user := fmt.Sprintf("%s-zone-%s-region-us-session-%s-sessTime-%d",
		c.Username, c.Zone, sessionID, c.SessionMinutes)

	return fmt.Sprintf("http://%s:%s@%s:%d", user, c.Password, c.Host, c.Port)
}

// GetTransport returns an HTTP transport configured with proxy if enabled
func (s *Service) GetTransport(skipTLS bool) (*http.Transport, error) {
	config, err := s.Get()
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{}

	if skipTLS {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	if config.Enabled && config.Username != "" {
		proxyURL := config.BuildProxyURL()
		if proxyURL != "" {
			parsedURL, err := url.Parse(proxyURL)
			if err != nil {
				return nil, fmt.Errorf("invalid proxy URL: %w", err)
			}
			transport.Proxy = http.ProxyURL(parsedURL)
		}
	}

	return transport, nil
}

// IsEnabled returns true if proxy is enabled and configured
func (s *Service) IsEnabled() bool {
	config, err := s.Get()
	if err != nil {
		return false
	}
	return config.Enabled && config.Username != ""
}

// TestConnection tests the proxy by making a request
func (s *Service) TestConnection() error {
	config, err := s.Get()
	if err != nil {
		return err
	}

	if config.Username == "" || config.Password == "" {
		return fmt.Errorf("proxy credentials not configured")
	}

	// Build transport directly for testing (ignore enabled flag)
	proxyURL := config.BuildProxyURL()
	if proxyURL == "" {
		return fmt.Errorf("invalid proxy configuration")
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(parsedURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	// Test by fetching httpbin
	resp, err := client.Get("https://httpbin.org/ip")
	if err != nil {
		return fmt.Errorf("proxy connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("proxy test returned status %d", resp.StatusCode)
	}

	return nil
}

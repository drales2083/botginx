package porkbun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://api.porkbun.com/api/json/v3"
	minRequestInterval = 100 * time.Millisecond // ~10 req/sec (API allows ~20/2s)
)

// Client is the Porkbun API client
type Client struct {
	apiKey     string
	secretKey  string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// APIResponse is the wrapper for Porkbun API responses
type APIResponse struct {
	Status  string `json:"status"` // SUCCESS or ERROR
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// APIError represents a Porkbun API error
type APIError struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("porkbun: %s (code: %s)", e.Message, e.Code)
	}
	return fmt.Sprintf("porkbun: %s", e.Message)
}

// Error code checks
func (e *APIError) IsCloseoutUnavailable() bool { return e.Code == "CLOSEOUT_UNAVAILABLE" }
func (e *APIError) IsCostMismatch() bool        { return e.Code == "COST_MISMATCH" }
func (e *APIError) IsInsufficientFunds() bool   { return e.Code == "INSUFFICIENT_FUNDS" }
func (e *APIError) IsRateLimited() bool         { return e.Code == "RATE_LIMIT_EXCEEDED" }
func (e *APIError) IsInvalidCredentials() bool {
	return e.Code == "INVALID_API_KEYS_001" || e.Code == "API_KEY_REQUIRED"
}

// NewClient creates a new Porkbun API client
func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		apiKey:    apiKey,
		secretKey: secretKey,
		baseURL:   defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing)
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// rateLimit enforces the minimum request interval
func (c *Client) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// doGet makes a GET request with header authentication
// Used for read operations like /closeout/search
func (c *Client) doGet(ctx context.Context, path string) ([]byte, error) {
	c.rateLimit()

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Header authentication for GET requests (per Porkbun API docs)
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Secret-API-Key", c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

// doPost makes a POST request with body authentication
// Used for write operations like /ping, /closeout/buy, /domain/updateNs
func (c *Client) doPost(ctx context.Context, path string, data map[string]interface{}) ([]byte, error) {
	c.rateLimit()

	reqURL := c.baseURL + path

	// Add credentials to body
	if data == nil {
		data = make(map[string]interface{})
	}
	data["apikey"] = c.apiKey
	data["secretapikey"] = c.secretKey

	jsonBody, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

// checkResponse parses the response and checks for API errors
func checkResponse(body []byte) (*APIResponse, error) {
	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// If we can't parse as JSON, it's likely an HTML error page
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("invalid response (not JSON): %s", preview)
	}

	if resp.Status != "SUCCESS" {
		return &resp, &APIError{
			Status:  resp.Status,
			Message: resp.Message,
			Code:    resp.Code,
		}
	}

	return &resp, nil
}

// VerifyCredentials checks if the API credentials are valid using /ping
func (c *Client) VerifyCredentials(ctx context.Context) error {
	body, err := c.doPost(ctx, "/ping", nil)
	if err != nil {
		return err
	}
	_, err = checkResponse(body)
	return err
}

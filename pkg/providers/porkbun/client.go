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
	minRequestInterval = 100 * time.Millisecond // Conservative: 10 req/sec
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

// Credentials is the auth payload for Porkbun API requests
type Credentials struct {
	APIKey       string `json:"apikey"`
	SecretAPIKey string `json:"secretapikey"`
}

// APIResponse is the wrapper for Porkbun API responses
type APIResponse struct {
	Status  string          `json:"status"` // SUCCESS or ERROR
	Message string          `json:"message,omitempty"`
	Code    string          `json:"code,omitempty"`
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

// IsCloseoutUnavailable returns true if the closeout is no longer available
func (e *APIError) IsCloseoutUnavailable() bool {
	return e.Code == "CLOSEOUT_UNAVAILABLE"
}

// IsCostMismatch returns true if the cost doesn't match current price
func (e *APIError) IsCostMismatch() bool {
	return e.Code == "COST_MISMATCH"
}

// IsInsufficientFunds returns true if account has insufficient funds
func (e *APIError) IsInsufficientFunds() bool {
	return e.Code == "INSUFFICIENT_FUNDS"
}

// IsRateLimited returns true if rate limited
func (e *APIError) IsRateLimited() bool {
	return e.Code == "RATE_LIMIT_EXCEEDED"
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

// doRequest makes a rate-limited request to the Porkbun API
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	// Build URL
	reqURL := c.baseURL + path

	// Create request body with credentials
	var reqBody io.Reader
	if body != nil {
		// If body is a map, add credentials
		if m, ok := body.(map[string]interface{}); ok {
			m["apikey"] = c.apiKey
			m["secretapikey"] = c.secretKey
			jsonBody, err := json.Marshal(m)
			if err != nil {
				return nil, fmt.Errorf("marshaling request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonBody)
		} else {
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshaling request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonBody)
		}
	} else {
		// POST with just credentials
		creds := Credentials{
			APIKey:       c.apiKey,
			SecretAPIKey: c.secretKey,
		}
		jsonBody, _ := json.Marshal(creds)
		reqBody = bytes.NewReader(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "BotginxPanel/1.0")

	// Execute
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return respBody, nil
}

// doGet makes a POST request with credentials in body (Porkbun requires POST for all endpoints)
// The path may contain query params for filtering (e.g. ?tld=com&limit=100)
func (c *Client) doGet(ctx context.Context, path string) ([]byte, error) {
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	reqURL := c.baseURL + path

	// Porkbun requires credentials in POST body for ALL endpoints
	creds := Credentials{
		APIKey:       c.apiKey,
		SecretAPIKey: c.secretKey,
	}
	jsonBody, _ := json.Marshal(creds)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "BotginxPanel/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// checkResponse parses the response and checks for API errors
func checkResponse(body []byte) (*APIResponse, error) {
	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w (body: %s)", err, string(body))
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

// VerifyCredentials checks if the API credentials are valid
func (c *Client) VerifyCredentials(ctx context.Context) error {
	body, err := c.doRequest(ctx, "POST", "/ping", nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

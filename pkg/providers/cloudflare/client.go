package cloudflare

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
	defaultBaseURL     = "https://api.cloudflare.com/client/v4"
	minRequestInterval = 250 * time.Millisecond // 1200 req/5min = ~4 req/sec
)

// Client is the Cloudflare API client
type Client struct {
	email      string
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates a new Cloudflare API client
func NewClient(email, apiKey string) *Client {
	return &Client{
		email:   email,
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing)
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// APIResponse is the wrapper for all Cloudflare API responses
type APIResponse struct {
	Success  bool            `json:"success"`
	Errors   []APIError      `json:"errors"`
	Messages []string        `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

// APIError represents a Cloudflare API error
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("cloudflare: %s (code %d)", e.Message, e.Code)
}

// IsInvalidAuth returns true if the error indicates invalid credentials
func (e *APIError) IsInvalidAuth() bool {
	return e.Code == 9103 || e.Code == 10000
}

// doRequest makes a rate-limited request to the Cloudflare API
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

	// Create request body if provided
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	req.Header.Set("X-Auth-Email", c.email)
	req.Header.Set("X-Auth-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

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

// checkResponse parses the response and checks for API errors
func checkResponse(body []byte) (*APIResponse, error) {
	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if !resp.Success && len(resp.Errors) > 0 {
		return &resp, &resp.Errors[0]
	}

	return &resp, nil
}

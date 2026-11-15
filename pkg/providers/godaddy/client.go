package godaddy

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
	defaultBaseURL     = "https://api.godaddy.com"
	minRequestInterval = 1000 * time.Millisecond // 60 req/min = 1 req/sec
)

// Client is the GoDaddy API client
type Client struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates a new GoDaddy API client
func NewClient(apiKey, apiSecret string) *Client {
	return &Client{
		apiKey:    apiKey,
		apiSecret: apiSecret,
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

// APIError represents a GoDaddy API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fields  []struct {
		Path    string `json:"path"`
		Message string `json:"message"`
	} `json:"fields,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("godaddy: %s (%s)", e.Message, e.Code)
}

// doRequest makes a rate-limited request to the GoDaddy API
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

	// Set headers - GoDaddy uses sso-key auth
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.apiKey, c.apiSecret))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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

	// Check for error status codes
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("godaddy: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

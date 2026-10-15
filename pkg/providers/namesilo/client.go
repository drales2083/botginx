package namesilo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://www.namesilo.com/api"
	apiVersion     = "1"
	responseType   = "json"

	// Rate limiting: 1 request per 2 seconds
	minRequestInterval = 2 * time.Second
)

// Client is the NameSilo API client
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates a new NameSilo API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing/sandbox)
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// apiResponse is the wrapper for all NameSilo API responses
type apiResponse struct {
	Reply reply `json:"reply"`
}

type reply struct {
	Code   int    `json:"code"`
	Detail string `json:"detail"`
	// Additional fields populated per-operation
}

// doRequest makes a rate-limited request to the NameSilo API
func (c *Client) doRequest(ctx context.Context, operation string, params url.Values) ([]byte, error) {
	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	// Build URL
	if params == nil {
		params = url.Values{}
	}
	params.Set("version", apiVersion)
	params.Set("type", responseType)
	params.Set("key", c.apiKey)

	reqURL := fmt.Sprintf("%s/%s?%s", c.baseURL, operation, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Execute
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
func checkResponse(body []byte) error {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	// NameSilo success codes: 300 = success, 301 = success (already exists)
	if resp.Reply.Code != 300 && resp.Reply.Code != 301 {
		return &APIError{
			Code:    resp.Reply.Code,
			Message: resp.Reply.Detail,
		}
	}

	return nil
}

// APIError represents a NameSilo API error
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("namesilo: %s (code %d)", e.Message, e.Code)
}

// IsInvalidKey returns true if the error indicates an invalid API key
func (e *APIError) IsInvalidKey() bool {
	return e.Code == 110
}

// IsDomainNotFound returns true if the domain doesn't exist in the account
func (e *APIError) IsDomainNotFound() bool {
	return e.Code == 200
}

// IsRecordExists returns true if the DNS record already exists
func (e *APIError) IsRecordExists() bool {
	return e.Code == 261
}

// Common error codes:
// 110 = Invalid API key
// 200 = Domain not found in account
// 250 = Domain not active
// 261 = DNS record already exists
// 262 = Invalid IP address
// 263 = Invalid record type
// 280 = DNS record not found

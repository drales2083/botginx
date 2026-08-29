package antibot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckRequest analyzes a request through the antibot system
type CheckRequest struct {
	IP        string            `json:"ip"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Host      string            `json:"host"`
	UserAgent string            `json:"user_agent"`
	Headers   map[string]string `json:"headers,omitempty"`
	Referer   string            `json:"referer,omitempty"`
}

// CheckResponse contains the antibot decision with all analytics fields
type CheckResponse struct {
	// Core fields
	Action     string  `json:"action"` // allow, block, challenge
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	Module     string  `json:"module"`
	DurationUS int64   `json:"duration_us"`

	// Geo
	Country   string  `json:"country,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	ASN       int     `json:"asn,omitempty"`
	ASNOrg    string  `json:"asn_org,omitempty"`

	// Analytics (v3.13.0+)
	SessionID        string `json:"session_id,omitempty"`
	ReferrerDomain   string `json:"referrer_domain,omitempty"`
	UTMSource        string `json:"utm_source,omitempty"`
	UTMMedium        string `json:"utm_medium,omitempty"`
	UTMCampaign      string `json:"utm_campaign,omitempty"`
	UTMTerm          string `json:"utm_term,omitempty"`
	UTMContent       string `json:"utm_content,omitempty"`
	ScreenResolution string `json:"screen_resolution,omitempty"`
	Language         string `json:"language,omitempty"`
	Timezone         string `json:"timezone,omitempty"`

	// Bot Detection
	IsBot          bool   `json:"is_bot,omitempty"`
	BotType        string `json:"bot_type,omitempty"`
	IsHeadless     bool   `json:"is_headless,omitempty"`
	IsTor          bool   `json:"is_tor,omitempty"`
	IsProxy        bool   `json:"is_proxy,omitempty"`
	IsDatacenter   bool   `json:"is_datacenter,omitempty"`
	AutomationTool string `json:"automation_tool,omitempty"`
	BehaviorScore  int    `json:"behavior_score,omitempty"`
	CookiesEnabled bool   `json:"cookies_enabled,omitempty"`
	JSEnabled      bool   `json:"js_enabled,omitempty"`
	Fingerprint    string `json:"fingerprint,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

// Check sends a request to the antibot for analysis
func (c *Client) Check(req *CheckRequest) (*CheckResponse, error) {
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/check", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("antibot API returned %d", resp.StatusCode)
	}

	var result struct {
		Status string         `json:"status"`
		Data   *CheckResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return &CheckResponse{Action: "allow"}, nil
	}

	return result.Data, nil
}

// WebhookPayload is the structure sent by antibot webhooks
type WebhookPayload struct {
	Event      string    `json:"event"`
	Timestamp  string    `json:"timestamp"`
	InstanceID string    `json:"instance_id"`
	Data       any       `json:"data"`
}

// RequestEventData from antibot webhooks
type RequestEventData struct {
	IP         string   `json:"ip"`
	Path       string   `json:"path"`
	Host       string   `json:"host"`
	Method     string   `json:"method"`
	UserAgent  string   `json:"user_agent"`
	Referer    string   `json:"referer"`
	Score      float64  `json:"score"`
	Action     string   `json:"action"`
	Reason     string   `json:"reason"`
	Module     string   `json:"module"`
	DurationUS int64    `json:"duration_us"`

	Country     string   `json:"country,omitempty"`
	ASN         int      `json:"asn,omitempty"`
	ASNOrg      string   `json:"asn_org,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`

	// Analytics fields (v3.13.0+)
	SessionID        string `json:"session_id,omitempty"`
	ReferrerDomain   string `json:"referrer_domain,omitempty"`
	UTMSource        string `json:"utm_source,omitempty"`
	UTMMedium        string `json:"utm_medium,omitempty"`
	UTMCampaign      string `json:"utm_campaign,omitempty"`
	UTMTerm          string `json:"utm_term,omitempty"`
	UTMContent       string `json:"utm_content,omitempty"`
	ScreenResolution string `json:"screen_resolution,omitempty"`
	Language         string `json:"language,omitempty"`
	Timezone         string `json:"timezone,omitempty"`
	IsHeadless       bool   `json:"is_headless,omitempty"`
	IsTor            bool   `json:"is_tor,omitempty"`
	IsProxy          bool   `json:"is_proxy,omitempty"`
	IsDatacenter     bool   `json:"is_datacenter,omitempty"`
	AutomationTool   string `json:"automation_tool,omitempty"`
	BehaviorScore    int    `json:"behavior_score,omitempty"`
	CookiesEnabled   bool   `json:"cookies_enabled,omitempty"`
	JSEnabled        bool   `json:"js_enabled,omitempty"`
}

// SessionEventData from antibot webhooks
type SessionEventData struct {
	SessionID   string `json:"session_id"`
	IP          string `json:"ip"`
	Host        string `json:"host"`
	UserAgent   string `json:"user_agent"`
	Country     string `json:"country,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	StartedAt   int64  `json:"started_at"`
	EndedAt     int64  `json:"ended_at,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	PageViews   int    `json:"page_views,omitempty"`
}

// ConversionEventData from antibot webhooks
type ConversionEventData struct {
	SessionID        string `json:"session_id"`
	IP               string `json:"ip"`
	Host             string `json:"host"`
	Path             string `json:"path"`
	UserAgent        string `json:"user_agent"`
	Fingerprint      string `json:"fingerprint,omitempty"`
	Country          string `json:"country,omitempty"`
	TimeToConvert    int    `json:"time_to_convert,omitempty"`
	ChallengeType    string `json:"challenge_type,omitempty"`
	ReferrerDomain   string `json:"referrer_domain,omitempty"`
	UTMSource        string `json:"utm_source,omitempty"`
	UTMMedium        string `json:"utm_medium,omitempty"`
	UTMCampaign      string `json:"utm_campaign,omitempty"`
	BehaviorScore    int    `json:"behavior_score,omitempty"`
	TrustTokenSolves int    `json:"trust_token_solves,omitempty"`
}

// Stats from the antibot system
type Stats struct {
	TotalRequests   int              `json:"total_requests"`
	BlockedRequests int              `json:"blocked_requests"`
	AllowedRequests int              `json:"allowed_requests"`
	Challenges      int              `json:"challenges"`
	BlockRate       float64          `json:"block_rate"`
	RequestsPerMin  []int            `json:"requests_per_min"`
	BlocksPerMin    []int            `json:"blocks_per_min"`
	LatencyPerMin   []float64        `json:"latency_per_min"`
	ModuleBlocks    map[string]int   `json:"module_blocks"`
	RecentBlocks    []BlockEvent     `json:"recent_blocks"`
	UptimeSeconds   int              `json:"uptime_seconds"`
}

type BlockEvent struct {
	Time   string  `json:"time"`
	IP     string  `json:"ip"`
	Path   string  `json:"path"`
	Reason string  `json:"reason"`
	Module string  `json:"module"`
	Score  float64 `json:"score"`
}

// GetStats retrieves antibot statistics
func (c *Client) GetStats() (*Stats, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/stats", nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
		Data   *Stats `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// LogEntry from the antibot system
type LogEntry struct {
	ID        int      `json:"id"`
	IP        string   `json:"ip"`
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	UserAgent string   `json:"user_agent"`
	Action    string   `json:"action"`
	Score     float64  `json:"score"`
	Reason    string   `json:"reason"`
	Module    string   `json:"module"`
	Duration  int      `json:"duration_us"`
	CreatedAt string   `json:"created_at"`
	Tags      []string `json:"tags"`
	Country   string   `json:"country,omitempty"`
	Latitude  float64  `json:"latitude,omitempty"`
	Longitude float64  `json:"longitude,omitempty"`
}

type LogsResponse struct {
	Items []LogEntry `json:"items"`
	Total int        `json:"total"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
}

// GetLogs retrieves antibot logs
func (c *Client) GetLogs(page, limit int, filters map[string]string) (*LogsResponse, error) {
	url := fmt.Sprintf("%s/api/logs?page=%d&limit=%d", c.baseURL, page, limit)
	for k, v := range filters {
		url += fmt.Sprintf("&%s=%s", k, v)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result LogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

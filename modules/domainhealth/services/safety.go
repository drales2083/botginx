package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SafetyStatus represents the overall safety status of a domain
type SafetyStatus string

const (
	SafetyClean   SafetyStatus = "clean"
	SafetyFlagged SafetyStatus = "flagged"
	SafetyUnknown SafetyStatus = "unknown"
)

// Threat represents a detected threat from one of the APIs
type Threat struct {
	Source      string `json:"source"`       // google, urlhaus, phishtank
	ThreatType  string `json:"threat_type"`  // malware, phishing, unwanted, etc.
	Description string `json:"description"`  // Human-readable description
	URL         string `json:"url,omitempty"` // Reference URL if available
	DetectedAt  string `json:"detected_at"`  // ISO timestamp
}

// SafetyResult holds the combined result of all safety checks
type SafetyResult struct {
	Status    SafetyStatus `json:"status"`
	Threats   []Threat     `json:"threats"`
	CheckedAt time.Time    `json:"checked_at"`
}

// SafetyChecker checks domains against multiple threat databases
type SafetyChecker struct {
	googleAPIKey string
	httpClient   *http.Client
}

// NewSafetyChecker creates a new safety checker
func NewSafetyChecker(googleAPIKey string) *SafetyChecker {
	return &SafetyChecker{
		googleAPIKey: googleAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckDomain checks a domain against all threat databases
func (c *SafetyChecker) CheckDomain(domain string) SafetyResult {
	result := SafetyResult{
		Status:    SafetyClean,
		Threats:   []Threat{},
		CheckedAt: time.Now(),
	}

	// Strip wildcard prefix
	if strings.HasPrefix(domain, "*.") {
		domain = domain[2:]
	}

	// Check all sources in parallel
	type checkResult struct {
		threats []Threat
		err     error
	}

	googleCh := make(chan checkResult, 1)
	urlhausCh := make(chan checkResult, 1)
	phishtankCh := make(chan checkResult, 1)

	go func() {
		threats, err := c.checkGoogleSafeBrowsing(domain)
		googleCh <- checkResult{threats, err}
	}()

	go func() {
		threats, err := c.checkURLhaus(domain)
		urlhausCh <- checkResult{threats, err}
	}()

	go func() {
		threats, err := c.checkPhishTank(domain)
		phishtankCh <- checkResult{threats, err}
	}()

	// Collect results
	for i := 0; i < 3; i++ {
		select {
		case r := <-googleCh:
			if r.err == nil && len(r.threats) > 0 {
				result.Threats = append(result.Threats, r.threats...)
			}
		case r := <-urlhausCh:
			if r.err == nil && len(r.threats) > 0 {
				result.Threats = append(result.Threats, r.threats...)
			}
		case r := <-phishtankCh:
			if r.err == nil && len(r.threats) > 0 {
				result.Threats = append(result.Threats, r.threats...)
			}
		case <-time.After(15 * time.Second):
			// Timeout waiting for checks
		}
	}

	if len(result.Threats) > 0 {
		result.Status = SafetyFlagged
	}

	return result
}

// checkGoogleSafeBrowsing checks domain against Google Safe Browsing API
func (c *SafetyChecker) checkGoogleSafeBrowsing(domain string) ([]Threat, error) {
	if c.googleAPIKey == "" {
		return nil, nil // Skip if no API key configured
	}

	apiURL := fmt.Sprintf("https://safebrowsing.googleapis.com/v4/threatMatches:find?key=%s", c.googleAPIKey)

	reqBody := map[string]interface{}{
		"client": map[string]string{
			"clientId":      "guardbot",
			"clientVersion": "1.0.0",
		},
		"threatInfo": map[string]interface{}{
			"threatTypes": []string{
				"MALWARE",
				"SOCIAL_ENGINEERING",
				"UNWANTED_SOFTWARE",
				"POTENTIALLY_HARMFUL_APPLICATION",
			},
			"platformTypes": []string{"ANY_PLATFORM"},
			"threatEntryTypes": []string{"URL"},
			"threatEntries": []map[string]string{
				{"url": "http://" + domain + "/"},
				{"url": "https://" + domain + "/"},
			},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	resp, err := c.httpClient.Post(apiURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Matches []struct {
			ThreatType string `json:"threatType"`
			Threat     struct {
				URL string `json:"url"`
			} `json:"threat"`
		} `json:"matches"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var threats []Threat
	for _, match := range result.Matches {
		threats = append(threats, Threat{
			Source:      "Google Safe Browsing",
			ThreatType:  formatThreatType(match.ThreatType),
			Description: fmt.Sprintf("Flagged as %s", formatThreatType(match.ThreatType)),
			DetectedAt:  time.Now().Format(time.RFC3339),
		})
	}

	return threats, nil
}

// checkURLhaus checks domain against URLhaus malware database
func (c *SafetyChecker) checkURLhaus(domain string) ([]Threat, error) {
	apiURL := "https://urlhaus-api.abuse.ch/v1/host/"

	form := url.Values{}
	form.Add("host", domain)

	resp, err := c.httpClient.PostForm(apiURL, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		QueryStatus string `json:"query_status"`
		URLCount    int    `json:"url_count"`
		URLs        []struct {
			URL       string `json:"url"`
			URLStatus string `json:"url_status"`
			Threat    string `json:"threat"`
			DateAdded string `json:"date_added"`
		} `json:"urls"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.QueryStatus != "ok" || result.URLCount == 0 {
		return nil, nil // No threats found
	}

	var threats []Threat
	seen := make(map[string]bool)
	for _, u := range result.URLs {
		if u.URLStatus == "online" && !seen[u.Threat] {
			seen[u.Threat] = true
			threats = append(threats, Threat{
				Source:      "URLhaus",
				ThreatType:  "Malware",
				Description: fmt.Sprintf("Malware distribution: %s", u.Threat),
				URL:         "https://urlhaus.abuse.ch/host/" + domain + "/",
				DetectedAt:  u.DateAdded,
			})
		}
	}

	return threats, nil
}

// checkPhishTank checks domain against PhishTank database
func (c *SafetyChecker) checkPhishTank(domain string) ([]Threat, error) {
	// PhishTank API - check if domain appears in phishing URLs
	// Using their URL check endpoint
	apiURL := "https://checkurl.phishtank.com/checkurl/"

	form := url.Values{}
	form.Add("url", "http://"+domain+"/")
	form.Add("format", "json")

	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "guardbot/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Results struct {
			InDatabase bool `json:"in_database"`
			Valid      bool `json:"valid"`
			Verified   bool `json:"verified"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Results.InDatabase && result.Results.Valid {
		return []Threat{{
			Source:      "PhishTank",
			ThreatType:  "Phishing",
			Description: "Domain found in PhishTank phishing database",
			URL:         "https://phishtank.org/",
			DetectedAt:  time.Now().Format(time.RFC3339),
		}}, nil
	}

	return nil, nil
}

func formatThreatType(t string) string {
	switch t {
	case "MALWARE":
		return "Malware"
	case "SOCIAL_ENGINEERING":
		return "Phishing"
	case "UNWANTED_SOFTWARE":
		return "Unwanted Software"
	case "POTENTIALLY_HARMFUL_APPLICATION":
		return "Potentially Harmful"
	default:
		return t
	}
}

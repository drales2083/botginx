package cpanel

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client handles cPanel API communication
type Client struct {
	config Config
	http   *http.Client
}

// NewClient creates a new cPanel API client
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	var transport *http.Transport

	// Try to get proxy transport if proxy is enabled
	if config.UseProxy && config.ProxyService != nil && config.ProxyService.IsEnabled() {
		var err error
		transport, err = config.ProxyService.GetTransport(config.SkipTLS)
		if err != nil {
			// Fallback to direct connection if proxy fails
			transport = &http.Transport{}
			if config.SkipTLS {
				transport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			}
		}
	} else {
		transport = &http.Transport{}
		if config.SkipTLS {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
	}

	return &Client{
		config: config,
		http: &http.Client{
			Timeout:   time.Duration(config.Timeout) * time.Second,
			Transport: transport,
		},
	}
}

// NewClientSimple creates a client with minimal config (no proxy)
func NewClientSimple(host, username, apiToken string) *Client {
	return NewClient(Config{
		Host:     host,
		Username: username,
		APIToken: apiToken,
		SkipTLS:  true, // cPanel often uses self-signed certs
	})
}

// NewClientWithProxy creates a client that uses proxy if enabled
func NewClientWithProxy(host, username, apiToken string, proxyService ProxyTransportProvider) *Client {
	return NewClient(Config{
		Host:         host,
		Username:     username,
		APIToken:     apiToken,
		SkipTLS:      true,
		UseProxy:     true,
		ProxyService: proxyService,
	})
}

// baseURL returns the cPanel API base URL
func (c *Client) baseURL() string {
	host := c.config.Host
	if !strings.Contains(host, ":") {
		host = host + ":2083"
	}
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return host
}

// authHeader returns the Authorization header value
func (c *Client) authHeader() string {
	return fmt.Sprintf("cpanel %s:%s", c.config.Username, c.config.APIToken)
}

// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(method, endpoint string, params url.Values) ([]byte, error) {
	var reqURL string
	var body io.Reader

	if method == http.MethodGet {
		reqURL = fmt.Sprintf("%s%s?%s", c.baseURL(), endpoint, params.Encode())
	} else {
		reqURL = fmt.Sprintf("%s%s", c.baseURL(), endpoint)
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, NewAPIError("request", "failed to create request", err)
	}

	req.Header.Set("Authorization", c.authHeader())
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, NewAPIError("connection", "failed to connect to cPanel", ErrConnectionFailed)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAPIError("response", "failed to read response", err)
	}

	// Check HTTP status
	switch resp.StatusCode {
	case http.StatusOK:
		return respBody, nil
	case http.StatusUnauthorized:
		return nil, NewAPIError("auth", "invalid credentials", ErrUnauthorized)
	case http.StatusForbidden:
		return nil, NewAPIError("auth", "insufficient permissions", ErrForbidden)
	default:
		return nil, NewAPIError("http", fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(respBody)), nil)
	}
}

// callUAPI calls a UAPI endpoint
func (c *Client) callUAPI(module, function string, params url.Values) ([]byte, error) {
	endpoint := fmt.Sprintf("/execute/%s/%s", module, function)
	return c.doRequest(http.MethodGet, endpoint, params)
}

// TestConnection verifies the connection and credentials
func (c *Client) TestConnection() error {
	_, err := c.GetDomains()
	if err != nil {
		return err
	}
	return nil
}

// GetDomains returns all domains on this cPanel account
func (c *Client) GetDomains() (*DomainInfo, error) {
	body, err := c.callUAPI("DomainInfo", "list_domains", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status   int      `json:"status"`
		Errors   []string `json:"errors,omitempty"`
		Messages []string `json:"messages,omitempty"`
		Data     struct {
			MainDomain    string   `json:"main_domain"`
			AddonDomains  []string `json:"addon_domains"`
			SubDomains    []string `json:"sub_domains"`
			ParkedDomains []string `json:"parked_domains"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, NewAPIError("parse", "failed to parse domains response", err)
	}

	if resp.Status != 1 {
		errMsg := "unknown error"
		if len(resp.Errors) > 0 {
			errMsg = strings.Join(resp.Errors, "; ")
		}
		return nil, NewAPIError("list_domains", errMsg, nil)
	}

	return &DomainInfo{
		MainDomain:    resp.Data.MainDomain,
		AddonDomains:  resp.Data.AddonDomains,
		SubDomains:    resp.Data.SubDomains,
		ParkedDomains: resp.Data.ParkedDomains,
	}, nil
}

// AllDomains returns a flat list of all domains (main + addon + sub + parked)
func (c *Client) AllDomains() ([]string, error) {
	info, err := c.GetDomains()
	if err != nil {
		return nil, err
	}

	domains := make([]string, 0)
	if info.MainDomain != "" {
		domains = append(domains, info.MainDomain)
	}
	domains = append(domains, info.AddonDomains...)
	domains = append(domains, info.SubDomains...)
	domains = append(domains, info.ParkedDomains...)

	return domains, nil
}

// HasDomain checks if a domain exists on this cPanel account
func (c *Client) HasDomain(domain string) (bool, error) {
	domains, err := c.AllDomains()
	if err != nil {
		return false, err
	}

	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, d := range domains {
		if strings.ToLower(d) == domain {
			return true, nil
		}
	}

	// Also check if it's a subdomain of any domain
	for _, d := range domains {
		if strings.HasSuffix(domain, "."+strings.ToLower(d)) {
			return true, nil
		}
	}

	return false, nil
}

// GetBaseDomain finds the base domain for a given hostname
// e.g., "promo.example.com" -> "example.com"
func (c *Client) GetBaseDomain(hostname string) (string, error) {
	domains, err := c.AllDomains()
	if err != nil {
		return "", err
	}

	hostname = strings.ToLower(strings.TrimSpace(hostname))

	// Check if hostname is exactly a known domain
	for _, d := range domains {
		if strings.ToLower(d) == hostname {
			return d, nil
		}
	}

	// Check if hostname is a subdomain of a known domain
	for _, d := range domains {
		if strings.HasSuffix(hostname, "."+strings.ToLower(d)) {
			return d, nil
		}
	}

	return "", ErrDomainNotFound
}

package cpanel

import "net/http"

// API response wrappers

// UAPIResponse is the standard UAPI response format
type UAPIResponse struct {
	Status   int         `json:"status"`
	Errors   []string    `json:"errors,omitempty"`
	Messages []string    `json:"messages,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

// Domain types

// DomainInfo from DomainInfo::list_domains
type DomainInfo struct {
	MainDomain    string   `json:"main_domain"`
	AddonDomains  []string `json:"addon_domains"`
	SubDomains    []string `json:"sub_domains"`
	ParkedDomains []string `json:"parked_domains"`
}

// DNS record types

// ZoneRecord represents a DNS record in a zone
type ZoneRecord struct {
	Line     int    `json:"line,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	TTL      int    `json:"ttl,omitempty"`
	Class    string `json:"class,omitempty"`
	Address  string `json:"address,omitempty"`  // A record
	CName    string `json:"cname,omitempty"`    // CNAME record
	TXTData  string `json:"txtdata,omitempty"`  // TXT record
	Exchange string `json:"exchange,omitempty"` // MX record
	Priority int    `json:"preference,omitempty"`
}

// TXTRecord is a simplified TXT record
type TXTRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Line  int    `json:"line,omitempty"`
}

// ARecord is a simplified A record
type ARecord struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Line    int    `json:"line,omitempty"`
}

// AddRecordResult from ZoneEdit::add_zone_record
type AddRecordResult struct {
	Result    int    `json:"result"`
	StatusMsg string `json:"statusmsg,omitempty"`
	NewSerial int    `json:"newserial,omitempty"`
}

// RemoveRecordResult from ZoneEdit::remove_zone_record
type RemoveRecordResult struct {
	Result    int    `json:"result"`
	StatusMsg string `json:"statusmsg,omitempty"`
}

// Connection config

// Config holds cPanel connection settings
type Config struct {
	Host         string // hostname:port (e.g., "example.com:2083")
	Username     string
	APIToken     string
	Timeout      int  // seconds, default 30
	SkipTLS      bool // allow self-signed certs
	UseProxy     bool // use proxy if configured
	ProxyService ProxyTransportProvider
}

// ProxyTransportProvider gets an HTTP transport with optional proxy
type ProxyTransportProvider interface {
	GetTransport(skipTLS bool) (*http.Transport, error)
	IsEnabled() bool
}

package geoip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type IPInfo struct {
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	City        string  `json:"city"`
	Region      string  `json:"region"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	ASN         string  `json:"asn"`
}

type cacheEntry struct {
	info      *IPInfo
	expiresAt time.Time
}

var (
	cache    = make(map[string]cacheEntry)
	cacheMu  sync.RWMutex
	cacheTTL = 24 * time.Hour
	client   = &http.Client{Timeout: 5 * time.Second}
)

func Lookup(ip string) (*IPInfo, error) {
	if ip == "" || ip == "127.0.0.1" || ip == "::1" {
		return &IPInfo{IP: ip, Country: "Local"}, nil
	}

	cacheMu.RLock()
	if entry, ok := cache[ip]; ok && time.Now().Before(entry.expiresAt) {
		cacheMu.RUnlock()
		return entry.info, nil
	}
	cacheMu.RUnlock()

	info, err := lookupAPI(ip)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	cache[ip] = cacheEntry{info: info, expiresAt: time.Now().Add(cacheTTL)}
	cacheMu.Unlock()

	return info, nil
}

func lookupAPI(ip string) (*IPInfo, error) {
	url := fmt.Sprintf("https://json.geoiplookup.io/%s", ip)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("geoip request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geoip returned %d", resp.StatusCode)
	}

	var result struct {
		IP            string  `json:"ip"`
		Country       string  `json:"country_name"`
		CountryCode   string  `json:"country_code"`
		City          string  `json:"city"`
		Region        string  `json:"region"`
		Latitude      float64 `json:"latitude"`
		Longitude     float64 `json:"longitude"`
		Timezone      string  `json:"timezone_name"`
		ISP           string  `json:"isp"`
		Org           string  `json:"org"`
		ASNNumber     int     `json:"asn_number"`
		ASNOrg        string  `json:"asn_org"`
		Success       bool    `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("geoip decode failed: %w", err)
	}

	return &IPInfo{
		IP:          result.IP,
		Country:     result.Country,
		CountryCode: result.CountryCode,
		City:        result.City,
		Region:      result.Region,
		Latitude:    result.Latitude,
		Longitude:   result.Longitude,
		Timezone:    result.Timezone,
		ISP:         result.ISP,
		Org:         result.Org,
		ASN:         fmt.Sprintf("AS%d", result.ASNNumber),
	}, nil
}

func GetCountry(ip string) string {
	info, err := Lookup(ip)
	if err != nil || info == nil {
		return ""
	}
	return info.Country
}

func GetCountryCode(ip string) string {
	info, err := Lookup(ip)
	if err != nil || info == nil {
		return ""
	}
	return info.CountryCode
}

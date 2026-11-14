package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/botginx/botginx/pkg/buildinfo"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/jmoiron/sqlx"
)

type Handler struct {
	db *sqlx.DB
}

func NewHandler(db *sqlx.DB) *Handler {
	return &Handler{db: db}
}

// LLMsTxt serves /llms.txt - a brief description for LLMs
func (h *Handler) LLMsTxt(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = "guardbot.sbs"
	}

	content := fmt.Sprintf(`# GuardBot

> Bot protection and link management platform

GuardBot helps you create redirect links with bot filtering, host domains with automatic SSL, and track visitor analytics.

## Guides

- [Introduction](https://docs.guardbot.sbs/guides/introduction/)
- [Quick Start](https://docs.guardbot.sbs/guides/quick-start/)
- [Pass URL Parameters](https://docs.guardbot.sbs/guides/pass-url-parameters/)

## Features

- [Redirect Links](https://docs.guardbot.sbs/features/redirect-links/)
- [Bot Protection](https://docs.guardbot.sbs/features/bot-protection/)
- [Analytics](https://docs.guardbot.sbs/features/analytics/)
- [Custom Domains](https://docs.guardbot.sbs/features/hosting/)
- [QR Codes](https://docs.guardbot.sbs/features/qr-codes/)

## API Reference

- [API Overview](https://docs.guardbot.sbs/api/overview/)
- [Authentication](https://docs.guardbot.sbs/api/authentication/)
- [Links API](https://docs.guardbot.sbs/api/links/)
- [Domains API](https://docs.guardbot.sbs/api/domains/)
- [Analytics API](https://docs.guardbot.sbs/api/analytics/)

## Integrations

- [cPanel Integration](https://docs.guardbot.sbs/integrations/cpanel/)
- [Telegram Bot](https://docs.guardbot.sbs/integrations/telegram/)
- [Signed URL Verification](https://docs.guardbot.sbs/integrations/signed-url-verification/)

## Resources

- [Full LLM Guide](https://%s/llms-full.txt)
- [ARD Manifest](https://%s/.well-known/ard.json)
- [Support](https://%s/user/support)
`, host, host, host)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(content))
}

// LLMsFullTxt serves /llms-full.txt - detailed description for LLMs
func (h *Handler) LLMsFullTxt(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = "guardbot.sbs"
	}

	content := fmt.Sprintf(`# GuardBot - Complete Guide for AI Agents

> Version: %s
> Last Updated: %s

## Overview

GuardBot is a link management and bot protection platform. Users create redirect links that filter bot traffic before sending visitors to their destination URLs.

## Core Concepts

### Redirect Links
A redirect link is a URL on your domain that forwards visitors somewhere else. When a visitor clicks:
1. GuardBot receives the request
2. Bot protection rules are evaluated
3. If the visitor passes, they're redirected to the destination
4. The visit is logged with analytics data

### Bot Protection
Each link can have protection settings:
- Block known bots (by user agent signatures)
- Block datacenter IPs
- Block suspicious patterns
- Require JavaScript execution
- CAPTCHA challenges

### Domains
Users add custom domains that point to GuardBot servers. Each domain gets:
- Automatic SSL certificate
- Wildcard subdomain support
- DNS verification

### Analytics
Every visit is tracked with:
- Timestamp
- Country (via IP geolocation)
- Device type (desktop, mobile, tablet)
- Browser and OS
- Referrer
- Bot classification (human, bot, suspicious)

## API Endpoints

Base URL: https://%s/api/v1/

### Authentication
All API requests require a Bearer token:
Authorization: Bearer bg_live_xxxxxxxxxxxx

### Links API
- GET /links - List all redirect links
- POST /links - Create a new link
- GET /links/{id} - Get link details
- PUT /links/{id} - Update a link
- DELETE /links/{id} - Delete a link

### Domains API
- GET /domains - List all domains
- POST /domains - Add a new domain
- DELETE /domains/{id} - Remove a domain

### Analytics API
- GET /analytics/link/{id} - Get analytics for a link
- GET /analytics/overview - Get overview statistics

## WebMCP Integration

This site registers WebMCP tools in the browser for AI agents:

### Available Tools

1. **searchLinks**
   - Description: Search your redirect links by keyword
   - Parameters: q (string), limit (number, default 10)
   - Returns: Array of matching links

2. **getDomains**
   - Description: List your verified domains
   - Parameters: none
   - Returns: Array of domains with status

3. **getLinkStats**
   - Description: Get analytics for a specific link
   - Parameters: link_id (string)
   - Returns: Click counts, top countries, devices

### Using WebMCP Tools

AI agents can call these tools via:
- navigator.modelContext.tools (Chrome 130+)
- window.WebMCP.tools
- Document event: 'webmcp:ready'

## Documentation

### Guides
- [Introduction](https://docs.guardbot.sbs/guides/introduction/)
- [Quick Start](https://docs.guardbot.sbs/guides/quick-start/)
- [Pass URL Parameters](https://docs.guardbot.sbs/guides/pass-url-parameters/)

### Features
- [Redirect Links](https://docs.guardbot.sbs/features/redirect-links/)
- [Bot Protection](https://docs.guardbot.sbs/features/bot-protection/)
- [Analytics](https://docs.guardbot.sbs/features/analytics/)
- [Custom Domains](https://docs.guardbot.sbs/features/hosting/)
- [QR Codes](https://docs.guardbot.sbs/features/qr-codes/)

### API Reference
- [API Overview](https://docs.guardbot.sbs/api/overview/)
- [Authentication](https://docs.guardbot.sbs/api/authentication/)
- [Links API](https://docs.guardbot.sbs/api/links/)
- [Domains API](https://docs.guardbot.sbs/api/domains/)
- [Analytics API](https://docs.guardbot.sbs/api/analytics/)
- [IP Lists API](https://docs.guardbot.sbs/api/iplists/)
- [QR Codes API](https://docs.guardbot.sbs/api/qrcodes/)
- [Shortener API](https://docs.guardbot.sbs/api/shortener/)
- [Webhooks](https://docs.guardbot.sbs/api/webhooks/)
- [Rate Limiting](https://docs.guardbot.sbs/api/rate-limiting/)
- [Errors](https://docs.guardbot.sbs/api/errors/)

### Integrations
- [cPanel Integration](https://docs.guardbot.sbs/integrations/cpanel/)
- [Namesilo](https://docs.guardbot.sbs/integrations/namesilo/)
- [Telegram Bot](https://docs.guardbot.sbs/integrations/telegram/)
- [Signed URL Verification](https://docs.guardbot.sbs/integrations/signed-url-verification/)
- [Verify Signed URLs](https://docs.guardbot.sbs/integrations/verify-signed-urls/)
- [Payments](https://docs.guardbot.sbs/integrations/payments/)

## Discovery Files

- /llms.txt - Brief site description
- /llms-full.txt - This complete guide
- /ard.json - Agentic Resource Discovery manifest
- /.well-known/ard.json - Same as above

## Rate Limits

- API: 1000 requests per hour per API key
- WebMCP: 60 requests per minute per IP

## Support

Create a support ticket: https://%s/user/support
`, buildinfo.Version(), time.Now().Format("2006-01-02"), host, host)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(content))
}

// ARDJson serves the Agentic Resource Discovery manifest
func (h *Handler) ARDJson(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = "guardbot.sbs"
	}
	scheme := "https"
	if r.TLS == nil && !strings.HasPrefix(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "http"
	}
	baseURL := scheme + "://" + host

	manifest := map[string]interface{}{
		"schema_version": "1.0",
		"name":           "GuardBot",
		"description":    "Bot protection and link management platform",
		"url":            baseURL,
		"logo":           baseURL + "/img/logo.png",
		"entries": []map[string]interface{}{
			{
				"identifier":            "llms-txt",
				"displayName":           "LLMs.txt",
				"type":                  "text/markdown",
				"url":                   baseURL + "/llms.txt",
				"description":           "LLM-readable site description",
				"representativeQueries": []string{"what is guardbot", "bot protection", "link management"},
			},
			{
				"identifier":            "llms-full-txt",
				"displayName":           "Full LLM Guide",
				"type":                  "text/markdown",
				"url":                   baseURL + "/llms-full.txt",
				"description":           "Complete guide for AI agents",
				"representativeQueries": []string{"guardbot api", "webmcp tools", "redirect links"},
			},
			{
				"identifier":            "rest-api",
				"displayName":           "REST API",
				"type":                  "application/json",
				"url":                   baseURL + "/api/v1",
				"description":           "REST API for link management",
				"representativeQueries": []string{"create link", "list domains", "analytics api"},
			},
			{
				"identifier":            "webmcp-api",
				"displayName":           "WebMCP Tools",
				"type":                  "application/json",
				"url":                   baseURL + "/webmcp",
				"description":           "WebMCP tools for AI agents",
				"representativeQueries": []string{"search links", "get domains", "link stats"},
			},
			{
				"identifier":            "documentation",
				"displayName":           "Documentation",
				"type":                  "text/html",
				"url":                   "https://docs.guardbot.sbs/",
				"description":           "User documentation",
				"representativeQueries": []string{"guardbot docs", "how to use", "getting started"},
			},
		},
		"contact": map[string]string{
			"support": baseURL + "/user/support",
			"docs":    "https://docs.guardbot.sbs/",
		},
		"authentication": map[string]interface{}{
			"type":        "bearer",
			"header":      "Authorization",
			"prefix":      "Bearer",
			"description": "API key from account settings",
		},
		"webmcp": map[string]interface{}{
			"enabled":  true,
			"endpoint": baseURL + "/webmcp",
			"tools": []map[string]interface{}{
				{
					"name":        "searchLinks",
					"description": "Search redirect links by keyword",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"q":     map[string]string{"type": "string", "description": "Search query"},
							"limit": map[string]interface{}{"type": "number", "description": "Max results (1-25)", "default": 10},
						},
						"required": []string{"q"},
					},
				},
				{
					"name":        "getDomains",
					"description": "List your verified domains",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "getLinkStats",
					"description": "Get analytics for a specific link",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"link_id": map[string]string{"type": "string", "description": "Link ID"},
						},
						"required": []string{"link_id"},
					},
				},
			},
		},
		"api": map[string]interface{}{
			"base_url": baseURL + "/api/v1",
			"docs":     "https://docs.guardbot.sbs/api/overview/",
			"rate_limit": map[string]interface{}{
				"requests_per_hour": 1000,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(manifest)
}

// WebMCP API Handlers

type webmcpResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(webmcpResponse{Success: true, Data: data})
}

func (h *Handler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(webmcpResponse{Success: false, Error: msg})
}

// WebMCPDomains returns user's domains
func (h *Handler) WebMCPDomains(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	type domain struct {
		ID          string `db:"id" json:"id"`
		Name        string `db:"name" json:"name"`
		DNSVerified bool   `db:"dns_verified" json:"dns_verified"`
		SSLEnabled  bool   `db:"ssl_enabled" json:"ssl_enabled"`
	}

	var domains []domain
	h.db.Select(&domains, `
		SELECT id, name, dns_verified, ssl_enabled
		FROM domains
		WHERE user_id = $1
		ORDER BY name ASC
	`, userID)

	h.jsonResponse(w, domains)
}

// WebMCPLinks returns user's redirect links
func (h *Handler) WebMCPLinks(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	type link struct {
		ID         string `db:"id" json:"id"`
		Subdomain  string `db:"subdomain" json:"subdomain"`
		BaseDomain string `db:"base_domain" json:"base_domain"`
		Path       string `db:"path" json:"path"`
		Type       string `db:"type" json:"type"`
		ViewCount  int    `db:"view_count" json:"view_count"`
	}

	var links []link
	h.db.Select(&links, `
		SELECT id, subdomain, base_domain, path, type, view_count
		FROM redirect_links
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)

	h.jsonResponse(w, links)
}

// WebMCPLinkStats returns analytics for a specific link
func (h *Handler) WebMCPLinkStats(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	linkID := r.URL.Query().Get("link_id")
	if linkID == "" {
		h.jsonError(w, "link_id parameter required", http.StatusBadRequest)
		return
	}

	// Verify link belongs to user
	var count int
	h.db.Get(&count, `SELECT COUNT(*) FROM redirect_links WHERE id = $1 AND user_id = $2`, linkID, userID)
	if count == 0 {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}

	// Get basic stats
	type stats struct {
		TotalClicks  int `db:"total_clicks" json:"total_clicks"`
		HumanClicks  int `db:"human_clicks" json:"human_clicks"`
		BotClicks    int `db:"bot_clicks" json:"bot_clicks"`
		UniqueIPs    int `db:"unique_ips" json:"unique_ips"`
		MobileClicks int `db:"mobile_clicks" json:"mobile_clicks"`
	}

	var s stats
	h.db.Get(&s, `
		SELECT
			COUNT(*) as total_clicks,
			COUNT(*) FILTER (WHERE is_bot = false) as human_clicks,
			COUNT(*) FILTER (WHERE is_bot = true) as bot_clicks,
			COUNT(DISTINCT ip_hash) as unique_ips,
			COUNT(*) FILTER (WHERE device_type = 'mobile') as mobile_clicks
		FROM link_analytics
		WHERE link_id = $1
	`, linkID)

	// Get top countries
	type country struct {
		Country string `db:"country" json:"country"`
		Count   int    `db:"count" json:"count"`
	}
	var countries []country
	h.db.Select(&countries, `
		SELECT country, COUNT(*) as count
		FROM link_analytics
		WHERE link_id = $1 AND country IS NOT NULL
		GROUP BY country
		ORDER BY count DESC
		LIMIT 5
	`, linkID)

	h.jsonResponse(w, map[string]interface{}{
		"stats":         s,
		"top_countries": countries,
	})
}

// WebMCPSearch searches redirect links
func (h *Handler) WebMCPSearch(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		h.jsonError(w, "q parameter required", http.StatusBadRequest)
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 25 {
			limit = parsed
		}
	}

	type link struct {
		ID         string `db:"id" json:"id"`
		Subdomain  string `db:"subdomain" json:"subdomain"`
		BaseDomain string `db:"base_domain" json:"base_domain"`
		Path       string `db:"path" json:"path"`
		Type       string `db:"type" json:"type"`
		ViewCount  int    `db:"view_count" json:"view_count"`
	}

	searchPattern := "%" + strings.ToLower(q) + "%"
	var links []link
	h.db.Select(&links, `
		SELECT id, subdomain, base_domain, path, type, view_count
		FROM redirect_links
		WHERE user_id = $1
		  AND (LOWER(subdomain) LIKE $2 OR LOWER(path) LIKE $2 OR LOWER(base_domain) LIKE $2)
		ORDER BY view_count DESC
		LIMIT $3
	`, userID, searchPattern, limit)

	h.jsonResponse(w, links)
}

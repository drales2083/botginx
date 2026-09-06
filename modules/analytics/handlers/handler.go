package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/botginx/botginx/modules/analytics/models"
	"github.com/botginx/botginx/modules/analytics/services"
	"github.com/botginx/botginx/pkg/antibot"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/geoip"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/settingspush"
	"github.com/go-chi/chi/v5"
)

// LinkResolver attributes inbound traffic to a redirect link and its owner.
type LinkResolver interface {
	ResolveByHost(host string) (linkID, userID string, err error)
	OwnerOf(linkID string) (userID string, err error)
}

// LinkDetails provides link information for settings push
type LinkDetails interface {
	GetLinkHost(linkID string) (host string, domainID string, err error)
}

// ShortLinkResolver resolves short links by host and path
type ShortLinkResolver interface {
	ResolveByHostPath(host, path string) (linkID, userID string, destinations []string, err error)
	GetLinkHost(linkID string) (host, path, domainID string, err error)
	OwnerOf(linkID string) (userID string, err error)
	RecordClick(linkID, userID string, isBot bool, country, device, ip, userAgent string) error
}

// ServerProvider provides server SSH details for settings push
type ServerProvider interface {
	GetServerForDomain(domainID string) (ip string, port int, user, password string, err error)
}

// HostingSettings represents antibot settings for a hosting domain
type HostingSettings struct {
	CountryMode      string
	CountryList      []string
	DeviceMode       string
	DeviceList       []string
	BlockBots        bool
	BlockTor         bool
	BlockProxy       bool
	BlockDatacenter  bool
	BlockHeadless    bool
	MinBehaviorScore int
	RedirectOnBlock  string
}

// HostingSettingsProvider provides antibot settings lookup for hosting domains
type HostingSettingsProvider interface {
	GetDomainSettingsByHost(host string) (*HostingSettings, error)
}

// HostingDomainInfo contains domain and account IDs for visit recording
type HostingDomainInfo struct {
	DomainID  string
	AccountID string
	UserID    string
}

// HostingVisitRecorder records visits to hosting domains
type HostingVisitRecorder interface {
	GetDomainByHost(host string) (*HostingDomainInfo, error)
	RecordHostingVisit(visit *HostingVisit) error
}

// HostingVisit represents a visit to a hosting domain
type HostingVisit struct {
	ID        string
	DomainID  string
	AccountID string

	IP       string
	Path     string
	Method   string
	Country  string
	City     string
	ASN      int
	ASNOrg   string

	Device           string
	Browser          string
	OS               string
	UserAgent        string
	Language         string
	Timezone         string
	ScreenResolution string

	Referrer       string
	ReferrerDomain string

	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	UTMTerm     string
	UTMContent  string

	IsBot          bool
	BotScore       float64
	BehaviorScore  int
	AutomationTool string
	IsHeadless     bool
	IsTor          bool
	IsProxy        bool
	IsDatacenter   bool
	Fingerprint    string

	Action      string
	Blocked     bool
	BlockReason string

	SessionID string
	CreatedAt time.Time
}

type Handler struct {
	service        *services.AnalyticsService
	templates      *module.TemplateEngine
	links          LinkResolver
	linkInfo       LinkDetails
	servers        ServerProvider
	hosting        HostingSettingsProvider
	hostingVisits  HostingVisitRecorder
	shortLinks     ShortLinkResolver
	pusher         *settingspush.Pusher
}

func NewHandler(
	service *services.AnalyticsService,
	templates *module.TemplateEngine,
	links LinkResolver,
) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
		links:     links,
		pusher:    settingspush.New(),
	}
}

// SetLinkDetails sets the link details provider (called after init to avoid circular deps)
func (h *Handler) SetLinkDetails(linkInfo LinkDetails) {
	h.linkInfo = linkInfo
}

// SetServerProvider sets the server provider (called after init to avoid circular deps)
func (h *Handler) SetServerProvider(servers ServerProvider) {
	h.servers = servers
}

// SetHostingSettingsProvider sets the hosting settings provider (called after init to avoid circular deps)
func (h *Handler) SetHostingSettingsProvider(hosting HostingSettingsProvider) {
	h.hosting = hosting
}

// SetShortLinkResolver sets the short link resolver (called after init to avoid circular deps)
func (h *Handler) SetShortLinkResolver(shortLinks ShortLinkResolver) {
	h.shortLinks = shortLinks
}

// SetHostingVisitRecorder sets the hosting visit recorder (called after init to avoid circular deps)
func (h *Handler) SetHostingVisitRecorder(recorder HostingVisitRecorder) {
	h.hostingVisits = recorder
}

// Pages

// Overview shows general analytics for all user's links
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	stats, _ := h.service.GetUserStats(userID)
	timeline, _ := h.service.GetUserTimeline(userID, r.URL.Query().Get("period"))
	topLinks, _ := h.service.GetTopLinks(userID, 10)

	module.RenderUserSection(w, r, h.templates, "analytics:overview.html", map[string]interface{}{
		"Title":    "Analytics Overview",
		"Stats":    stats,
		"Timeline": timeline,
		"TopLinks": topLinks,
	})
}

// LinkAnalytics shows analytics for a specific link
func (h *Handler) LinkAnalytics(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "hourly" // Default to 24h view
	}

	stats, _ := h.service.GetLinkStats(linkID)
	timeline, _ := h.service.GetTimeline(linkID, period)
	countries, _ := h.service.GetCountryStats(linkID, 10)
	devices, _ := h.service.GetDeviceStats(linkID)
	browsers, _ := h.service.GetBrowserStats(linkID, 10)
	osStats, _ := h.service.GetOSStats(linkID, 10)
	referrers, _ := h.service.GetReferrerStats(linkID, 10)
	recentVisits, _ := h.service.GetRecentVisits(linkID, 20)
	settings, _ := h.service.GetLinkSettings(linkID)

	module.RenderUserSection(w, r, h.templates, "analytics:link.html", map[string]interface{}{
		"Title":        "Link Analytics",
		"LinkID":       linkID,
		"Period":       period,
		"Stats":        stats,
		"Timeline":     timeline,
		"Countries":    countries,
		"Devices":      devices,
		"Browsers":     browsers,
		"OSStats":      osStats,
		"Referrers":    referrers,
		"RecentVisits": recentVisits,
		"Settings":     settings,
	})
}

// LinkSettings shows/edits settings for a link
func (h *Handler) LinkSettings(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	settings, _ := h.service.GetLinkSettings(linkID)

	module.RenderUserSection(w, r, h.templates, "analytics:settings.html", map[string]interface{}{
		"Title":    "Link Settings",
		"LinkID":   linkID,
		"Settings": settings,
	})
}

// API handlers

func (h *Handler) APIGetStats(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	stats, err := h.service.GetLinkStats(linkID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIGetTimeline(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	period := r.URL.Query().Get("period")
	timeline, err := h.service.GetTimeline(linkID, period)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"timeline": timeline})
}

func (h *Handler) APIGetCountries(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	countries, _ := h.service.GetCountryStats(linkID, 50)
	h.json(w, http.StatusOK, map[string]interface{}{"countries": countries})
}

func (h *Handler) APIGetDevices(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	devices, _ := h.service.GetDeviceStats(linkID)
	h.json(w, http.StatusOK, map[string]interface{}{"devices": devices})
}

func (h *Handler) APIGetBrowsers(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	browsers, _ := h.service.GetBrowserStats(linkID, 20)
	h.json(w, http.StatusOK, map[string]interface{}{"browsers": browsers})
}

func (h *Handler) APIGetOS(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	osStats, _ := h.service.GetOSStats(linkID, 20)
	h.json(w, http.StatusOK, map[string]interface{}{"os": osStats})
}

func (h *Handler) APIGetReferrers(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	referrers, _ := h.service.GetReferrerStats(linkID, 20)
	h.json(w, http.StatusOK, map[string]interface{}{"referrers": referrers})
}

func (h *Handler) APIGetRecentVisits(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	visits, _ := h.service.GetRecentVisits(linkID, 50)
	h.json(w, http.StatusOK, map[string]interface{}{"visits": visits})
}

func (h *Handler) APIGetSettings(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")
	settings, _ := h.service.GetLinkSettings(linkID)
	h.json(w, http.StatusOK, settings)
}

func (h *Handler) APIUpdateSettings(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "linkId")

	var input struct {
		CountryMode      string   `json:"countryMode"`
		CountryList      []string `json:"countryList"`
		ASNMode          string   `json:"asnMode"`
		ASNList          []string `json:"asnList"`
		DeviceMode       string   `json:"deviceMode"`
		DeviceList       []string `json:"deviceList"`
		BlockBots        bool     `json:"blockBots"`
		BlockTor         bool     `json:"blockTor"`
		BlockProxy       bool     `json:"blockProxy"`
		BlockDatacenter  bool     `json:"blockDatacenter"`
		BlockHeadless    bool     `json:"blockHeadless"`
		MinBehaviorScore int      `json:"minBehaviorScore"`
		RedirectOnBlock  string   `json:"redirectOnBlock"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	settings := &models.LinkSettings{
		LinkID:           linkID,
		CountryMode:      input.CountryMode,
		CountryList:      input.CountryList,
		ASNMode:          input.ASNMode,
		ASNList:          input.ASNList,
		DeviceMode:       input.DeviceMode,
		DeviceList:       input.DeviceList,
		BlockBots:        input.BlockBots,
		BlockTor:         input.BlockTor,
		BlockProxy:       input.BlockProxy,
		BlockDatacenter:  input.BlockDatacenter,
		BlockHeadless:    input.BlockHeadless,
		MinBehaviorScore: input.MinBehaviorScore,
		RedirectOnBlock:  input.RedirectOnBlock,
	}

	// Check if exists
	existing, _ := h.service.GetLinkSettings(linkID)
	if existing.ID != "" {
		settings.ID = existing.ID
	}

	if err := h.service.SaveLinkSettings(settings); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Push settings to deploy VPS (async, don't block response)
	go h.pushSettingsToVPS(linkID, settings)

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// pushSettingsToVPS pushes the settings file to the VPS where the link is deployed
func (h *Handler) pushSettingsToVPS(linkID string, settings *models.LinkSettings) {
	if h.linkInfo == nil || h.servers == nil {
		return // Dependencies not set
	}

	// Get link host and domain
	host, domainID, err := h.linkInfo.GetLinkHost(linkID)
	if err != nil || domainID == "" {
		log.Printf("Settings push: link %s not found or not deployed", linkID)
		return
	}

	// Get user ID for IP list lookups
	userID := ""
	if h.links != nil {
		userID, _ = h.links.OwnerOf(linkID)
	}

	// Get server SSH details
	ip, port, user, password, err := h.servers.GetServerForDomain(domainID)
	if err != nil || ip == "" {
		log.Printf("Settings push: no server for domain %s", domainID)
		return
	}

	// Build settings for botection
	pushSettings := settingspush.LinkSettings{
		LinkID:           linkID,
		UserID:           userID,
		Host:             host,
		BlockBots:        settings.BlockBots,
		BlockTor:         settings.BlockTor,
		BlockProxy:       settings.BlockProxy,
		BlockDatacenter:  settings.BlockDatacenter,
		BlockHeadless:    settings.BlockHeadless,
		CountryMode:      settings.CountryMode,
		CountryList:      settings.CountryList,
		ASNMode:          settings.ASNMode,
		ASNList:          settings.ASNList,
		DeviceMode:       settings.DeviceMode,
		DeviceList:       settings.DeviceList,
		MinBehaviorScore: settings.MinBehaviorScore,
		RedirectOnBlock:  settings.RedirectOnBlock,
	}

	server := settingspush.ServerInfo{
		IP:       ip,
		Port:     port,
		User:     user,
		Password: password,
	}

	if err := h.pusher.Push(server, linkID, pushSettings); err != nil {
		log.Printf("Settings push failed for %s: %v", linkID, err)
	} else {
		log.Printf("Settings pushed for %s to %s", linkID, ip)
	}
}

// APIRecordVisit is called by the redirect link endpoint to track visits
func (h *Handler) APIRecordVisit(w http.ResponseWriter, r *http.Request) {
	var visit models.Visit
	if err := json.NewDecoder(r.Body).Decode(&visit); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if visit.LinkID == "" {
		h.jsonError(w, "linkId is required", http.StatusBadRequest)
		return
	}

	// Ownership comes from the link, never from the request body: analytics
	// queries filter on user_id, so a caller-supplied value would let one
	// account write rows into another's reports.
	owner, err := h.links.OwnerOf(visit.LinkID)
	if err != nil {
		h.jsonError(w, "Unknown link", http.StatusNotFound)
		return
	}
	visit.UserID = owner

	// GeoIP lookup if country not provided
	if visit.Country == "" && visit.IP != "" {
		if ipInfo, err := geoip.Lookup(visit.IP); err == nil && ipInfo != nil {
			visit.Country = ipInfo.CountryCode
			visit.City = ipInfo.City
			visit.Latitude = ipInfo.Latitude
			visit.Longitude = ipInfo.Longitude
		}
	}

	// Check if should be blocked
	blocked, reason := h.service.ShouldBlock(
		visit.LinkID, visit.Country, visit.Device,
		visit.IsBot, visit.IsTor, visit.IsProxy, visit.IsDatacenter, visit.IsHeadless,
		visit.BehaviorScore,
	)
	visit.Blocked = blocked
	visit.BlockReason = reason

	if err := h.service.RecordVisit(&visit); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"blocked": blocked,
		"reason":  reason,
	})
}

// Overview API
func (h *Handler) APIGetOverview(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	period := r.URL.Query().Get("period")

	stats, _ := h.service.GetUserStats(userID)
	timeline, _ := h.service.GetUserTimeline(userID, period)
	topLinks, _ := h.service.GetTopLinks(userID, 10)

	h.json(w, http.StatusOK, map[string]interface{}{
		"stats":    stats,
		"timeline": timeline,
		"topLinks": topLinks,
	})
}

// Callback handler for botection decision-making

// ShouldBlockCallback is called by botection BEFORE taking action.
// It receives visitor context and returns a block/allow decision based on per-link settings.
// This is different from the webhook which fires AFTER the action for logging.
func (h *Handler) ShouldBlockCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host         string `json:"host"`
		Path         string `json:"path"`
		IP           string `json:"ip"`
		Country      string `json:"country"`
		IsTor        bool   `json:"is_tor"`
		IsProxy      bool   `json:"is_proxy"`
		IsDatacenter bool   `json:"is_datacenter"`
		IsHeadless   bool   `json:"is_headless"`
		UserAgent    string `json:"user_agent"`
		Score        int    `json:"score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// First try: resolve by subdomain (redirect links)
	linkID, _, err := h.links.ResolveByHost(req.Host)
	if err != nil || linkID == "" {
		// Second try: resolve by host+path (short links)
		if h.shortLinks != nil && req.Path != "" {
			shortLinkID, _, destinations, err := h.shortLinks.ResolveByHostPath(req.Host, req.Path)
			if err == nil && shortLinkID != "" && len(destinations) > 0 {
				// Found short link - handle it
				h.handleShortLinkCallback(w, r, req, shortLinkID, destinations)
				return
			}
		}

		// Not a redirect link or short link - check if it's a hosting domain
		if h.hosting != nil {
			hostingSettings, err := h.hosting.GetDomainSettingsByHost(req.Host)
			if err == nil && hostingSettings != nil {
				// Found hosting domain settings - apply blocking rules
				h.handleHostingDomainBlock(w, req, hostingSettings)
				return
			}
		}

		// Unknown host - tell botection to use its local rules
		h.json(w, http.StatusOK, map[string]interface{}{
			"block":          false,
			"defer_to_local": true,
		})
		return
	}

	// Load link settings
	settings, err := h.service.GetLinkSettings(linkID)
	if err != nil || settings.ID == "" {
		// No settings for this link - defer to local rules
		h.json(w, http.StatusOK, map[string]interface{}{
			"block":          false,
			"defer_to_local": true,
		})
		return
	}

	// Detect device from user agent for device filtering
	device := h.detectDevice(req.UserAgent)

	// Apply blocking rules
	// Note: isBot comes from botection's score threshold, we treat score >= 70 as bot
	isBot := req.Score >= 70

	block, reason := h.service.ShouldBlock(
		linkID,
		req.Country,
		device,
		isBot,
		req.IsTor,
		req.IsProxy,
		req.IsDatacenter,
		req.IsHeadless,
		req.Score, // behavior score
	)

	resp := map[string]interface{}{
		"block": block,
	}
	if block {
		resp["reason"] = reason
		if settings.RedirectOnBlock != "" {
			resp["redirect"] = settings.RedirectOnBlock
		}
	}

	h.json(w, http.StatusOK, resp)
}

// handleShortLinkCallback handles the callback for short links (path-based routing).
// It looks up link settings and returns redirect URL if allowed.
func (h *Handler) handleShortLinkCallback(w http.ResponseWriter, r *http.Request, req struct {
	Host         string `json:"host"`
	Path         string `json:"path"`
	IP           string `json:"ip"`
	Country      string `json:"country"`
	IsTor        bool   `json:"is_tor"`
	IsProxy      bool   `json:"is_proxy"`
	IsDatacenter bool   `json:"is_datacenter"`
	IsHeadless   bool   `json:"is_headless"`
	UserAgent    string `json:"user_agent"`
	Score        int    `json:"score"`
}, linkID string, destinations []string) {
	// Load link settings from analytics (shared protection settings)
	settings, err := h.service.GetLinkSettings(linkID)
	if err != nil || settings.ID == "" {
		// No settings - allow with redirect to first destination
		h.json(w, http.StatusOK, map[string]interface{}{
			"block":    false,
			"redirect": destinations[0],
		})
		return
	}

	// Detect device from user agent
	device := h.detectDevice(req.UserAgent)
	isBot := req.Score >= 70

	// Apply blocking rules
	block, reason := h.service.ShouldBlock(
		linkID, req.Country, device,
		isBot, req.IsTor, req.IsProxy, req.IsDatacenter, req.IsHeadless,
		req.Score,
	)

	resp := map[string]interface{}{
		"block": block,
	}
	if block {
		resp["reason"] = reason
		if settings.RedirectOnBlock != "" {
			resp["redirect"] = settings.RedirectOnBlock
		}
	} else {
		// Not blocked - redirect to a destination (random rotation for multiple)
		dest := destinations[0]
		if len(destinations) > 1 {
			// Simple random selection
			dest = destinations[int(time.Now().UnixNano())%len(destinations)]
		}
		resp["redirect"] = dest
	}

	h.json(w, http.StatusOK, resp)
}

// handleHostingDomainBlock applies blocking rules for a hosting domain.
// This is similar to the redirect link blocking but uses hosting domain settings.
func (h *Handler) handleHostingDomainBlock(w http.ResponseWriter, req struct {
	Host         string `json:"host"`
	Path         string `json:"path"`
	IP           string `json:"ip"`
	Country      string `json:"country"`
	IsTor        bool   `json:"is_tor"`
	IsProxy      bool   `json:"is_proxy"`
	IsDatacenter bool   `json:"is_datacenter"`
	IsHeadless   bool   `json:"is_headless"`
	UserAgent    string `json:"user_agent"`
	Score        int    `json:"score"`
}, settings *HostingSettings) {
	// Detect device from user agent
	device := h.detectDevice(req.UserAgent)

	// Note: isBot comes from botection's score threshold, we treat score >= 70 as bot
	isBot := req.Score >= 70

	// Apply blocking rules
	block, reason := h.shouldBlockWithHostingSettings(
		settings,
		req.Country,
		device,
		isBot,
		req.IsTor,
		req.IsProxy,
		req.IsDatacenter,
		req.IsHeadless,
		req.Score,
	)

	resp := map[string]interface{}{
		"block": block,
	}
	if block {
		resp["reason"] = reason
		if settings.RedirectOnBlock != "" {
			resp["redirect"] = settings.RedirectOnBlock
		}
	}

	h.json(w, http.StatusOK, resp)
}

// shouldBlockWithHostingSettings checks if a visit should be blocked based on hosting domain settings.
// This mirrors the logic in AnalyticsService.ShouldBlock but uses HostingSettings.
func (h *Handler) shouldBlockWithHostingSettings(
	settings *HostingSettings,
	country, device string,
	isBot, isTor, isProxy, isDatacenter, isHeadless bool,
	behaviorScore int,
) (bool, string) {
	if settings.BlockBots && isBot {
		return true, "bot_blocked"
	}
	if settings.BlockTor && isTor {
		return true, "tor_blocked"
	}
	if settings.BlockProxy && isProxy {
		return true, "proxy_blocked"
	}
	if settings.BlockDatacenter && isDatacenter {
		return true, "datacenter_blocked"
	}
	if settings.BlockHeadless && isHeadless {
		return true, "headless_blocked"
	}
	// Only check behavior score if we actually have one (> 0 means data was provided)
	if settings.MinBehaviorScore > 0 && behaviorScore > 0 && behaviorScore < settings.MinBehaviorScore {
		return true, "low_behavior_score"
	}

	// Country filtering
	if settings.CountryMode == "whitelist" && len(settings.CountryList) > 0 && country != "" {
		if !containsIgnoreCase(settings.CountryList, country) {
			return true, "country_not_whitelisted"
		}
	}
	if settings.CountryMode == "blacklist" && len(settings.CountryList) > 0 && country != "" {
		if containsIgnoreCase(settings.CountryList, country) {
			return true, "country_blacklisted"
		}
	}

	// Device filtering
	if settings.DeviceMode == "whitelist" && len(settings.DeviceList) > 0 {
		if !containsIgnoreCase(settings.DeviceList, device) {
			return true, "device_not_whitelisted"
		}
	}
	if settings.DeviceMode == "blacklist" && len(settings.DeviceList) > 0 {
		if containsIgnoreCase(settings.DeviceList, device) {
			return true, "device_blacklisted"
		}
	}

	return false, ""
}

// containsIgnoreCase checks if slice contains item, case-insensitive.
func containsIgnoreCase(slice []string, item string) bool {
	for _, s := range slice {
		if len(s) == len(item) {
			match := true
			for i := 0; i < len(s); i++ {
				c1, c2 := s[i], item[i]
				if c1 >= 'A' && c1 <= 'Z' {
					c1 += 'a' - 'A'
				}
				if c2 >= 'A' && c2 <= 'Z' {
					c2 += 'a' - 'A'
				}
				if c1 != c2 {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// Webhook handlers for antibot events

// WebhookHandler receives events from the antibot system
func (h *Handler) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.jsonError(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	log.Printf("Webhook received: %d bytes", len(body))

	// Verify webhook signature if secret is configured
	secret := os.Getenv("ANTIBOT_WEBHOOK_SECRET")
	if secret != "" {
		signature := r.Header.Get("X-Webhook-Signature")
		if !h.verifySignature(body, signature, secret) {
			h.jsonError(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Try to parse as batched format first (antibot sends {"events": [...]})
	var batch antibot.WebhookBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		h.jsonError(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// If events array exists, process each event
	if len(batch.Events) > 0 {
		for _, payload := range batch.Events {
			h.processWebhookEvent(payload)
		}
	} else {
		// Fallback: try single event format for backwards compatibility
		var payload antibot.WebhookPayload
		if err := json.Unmarshal(body, &payload); err == nil && payload.Event != "" {
			h.processWebhookEvent(payload)
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{"received": true})
}

// processWebhookEvent handles a single webhook event
func (h *Handler) processWebhookEvent(payload antibot.WebhookPayload) {
	// Handle different event types - antibot sends specific event names like "request.blocked"
	switch payload.Event {
	case "request", "request.blocked", "request.challenged", "request.challenge_passed", "request.challenge_failed", "request.allowed":
		h.handleRequestEvent(payload.Data)
	case "session.start":
		h.handleSessionStartEvent(payload.Data)
	case "session.end":
		h.handleSessionEndEvent(payload.Data)
	case "page.view", "page_view":
		h.handlePageViewEvent(payload.Data)
	case "conversion":
		h.handleConversionEvent(payload.Data)
	default:
		log.Printf("Webhook: unknown event type: %s", payload.Event)
	}
}

func (h *Handler) verifySignature(body []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

func (h *Handler) handleRequestEvent(data any) {
	eventData, ok := data.(map[string]interface{})
	if !ok {
		log.Printf("Webhook: invalid event data type: %T", data)
		return
	}

	linkID, userID := h.resolveLink(eventData)
	if linkID == "" {
		// Not a redirect link - try short link
		host := getString(eventData, "host")
		path := getString(eventData, "path")
		if h.tryRecordShortLinkVisit(eventData, host, path) {
			return // Successfully recorded as short link visit
		}

		// Not a short link - try hosting domain
		if h.hostingVisits != nil {
			if h.tryRecordHostingVisit(eventData, host) {
				return // Successfully recorded as hosting visit
			}
		}
		log.Printf("Webhook: could not resolve link or hosting domain for host=%s path=%s", host, path)
		return
	}

	visit := &models.Visit{
		LinkID:          linkID,
		UserID:          userID,
		SessionID:       getString(eventData, "session_id"),
		IP:              getString(eventData, "ip"),
		Country:         getString(eventData, "country"),
		UserAgent:       getString(eventData, "user_agent"),
		Referrer:        getString(eventData, "referer"),
		ReferrerDomain:  getString(eventData, "referrer_domain"),
		Language:        getString(eventData, "language"),
		Timezone:        getString(eventData, "timezone"),
		ScreenRes:       getString(eventData, "screen_resolution"),
		UTMSource:       getString(eventData, "utm_source"),
		UTMMedium:       getString(eventData, "utm_medium"),
		UTMCampaign:     getString(eventData, "utm_campaign"),
		UTMTerm:         getString(eventData, "utm_term"),
		UTMContent:      getString(eventData, "utm_content"),
		BotScore:        getFloat(eventData, "score"),
		IsBot:           getFloat(eventData, "score") >= 70,
		BotModule:       getString(eventData, "module"),
		BehaviorScore:   getInt(eventData, "behavior_score"),
		AutomationTool:  getString(eventData, "automation_tool"),
		IsHeadless:      getBool(eventData, "is_headless"),
		IsTor:           getBool(eventData, "is_tor"),
		IsProxy:         getBool(eventData, "is_proxy"),
		IsDatacenter:    getBool(eventData, "is_datacenter"),
		CookiesEnabled:  getBool(eventData, "cookies_enabled"),
		JSEnabled:       getBool(eventData, "js_enabled"),
		Fingerprint:     getString(eventData, "fingerprint"),
		Action:          getString(eventData, "action"),
		DurationUS:      int64(getInt(eventData, "duration_us")),
		CreatedAt:       time.Now(),
	}

	// Determine device type from user agent
	visit.Device = h.detectDevice(visit.UserAgent)
	visit.Browser = h.detectBrowser(visit.UserAgent)
	visit.OS = h.detectOS(visit.UserAgent)

	// GeoIP lookup if country not provided by webhook
	if visit.Country == "" && visit.IP != "" {
		if ipInfo, err := geoip.Lookup(visit.IP); err == nil && ipInfo != nil {
			visit.Country = ipInfo.CountryCode
			visit.City = ipInfo.City
			visit.Latitude = ipInfo.Latitude
			visit.Longitude = ipInfo.Longitude
		}
	}

	// Use botection's actual decision from the action field
	// action = "allow", "block", or "challenge"
	action := getString(eventData, "action")
	visit.Blocked = (action == "block")
	if visit.Blocked {
		// Use the actual reason from botection if available
		reason := getString(eventData, "reason")
		if reason != "" {
			visit.BlockReason = reason
		} else {
			visit.BlockReason = "blocked_by_antibot"
		}
	}

	if err := h.service.RecordVisit(visit); err != nil {
		log.Printf("Webhook: failed to record visit for link=%s: %v", linkID, err)
	} else {
		log.Printf("Webhook: recorded visit link=%s ip=%s blocked=%v reason=%s",
			linkID, visit.IP, visit.Blocked, visit.BlockReason)
	}
}

func (h *Handler) handleSessionStartEvent(data any) {
	eventData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	linkID, userID := h.resolveLink(eventData)
	if linkID == "" {
		return
	}

	session := &models.Session{
		SessionID:      getString(eventData, "session_id"),
		LinkID:         linkID,
		UserID:         userID,
		IP:             getString(eventData, "ip"),
		Country:        getString(eventData, "country"),
		UserAgent:      getString(eventData, "user_agent"),
		Fingerprint:    getString(eventData, "fingerprint"),
		ReferrerDomain: getString(eventData, "referrer_domain"),
		UTMSource:      getString(eventData, "utm_source"),
		UTMMedium:      getString(eventData, "utm_medium"),
		UTMCampaign:    getString(eventData, "utm_campaign"),
		StartedAt:      time.Now(),
	}

	h.service.RecordSession(session)
}

func (h *Handler) handleSessionEndEvent(data any) {
	eventData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	sessionID := getString(eventData, "session_id")
	duration := getInt(eventData, "duration")
	pageViews := getInt(eventData, "page_views")

	h.service.UpdateSession(sessionID, duration, pageViews)
}

func (h *Handler) handlePageViewEvent(data any) {
	eventData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	sessionID := getString(eventData, "session_id")
	h.service.IncrementPageViews(sessionID)
}

func (h *Handler) handleConversionEvent(data any) {
	eventData, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	linkID, userID := h.resolveLink(eventData)
	if linkID == "" {
		return
	}

	conversion := &models.Conversion{
		SessionID:        getString(eventData, "session_id"),
		LinkID:           linkID,
		UserID:           userID,
		IP:               getString(eventData, "ip"),
		Country:          getString(eventData, "country"),
		Path:             getString(eventData, "path"),
		TimeToConvert:    getInt(eventData, "time_to_convert"),
		ChallengeType:    getString(eventData, "challenge_type"),
		BehaviorScore:    getInt(eventData, "behavior_score"),
		TrustTokenSolves: getInt(eventData, "trust_token_solves"),
		ReferrerDomain:   getString(eventData, "referrer_domain"),
		UTMSource:        getString(eventData, "utm_source"),
		UTMMedium:        getString(eventData, "utm_medium"),
		UTMCampaign:      getString(eventData, "utm_campaign"),
		CreatedAt:        time.Now(),
	}

	h.service.RecordConversion(conversion)
}

// resolveLink identifies the link an event belongs to, and the account that
// owns it.
//
// The owner matters as much as the link: every user-facing analytics query
// filters on user_id, so a row recorded without one is stored but invisible to
// the person whose traffic it is.
//
// Events normally identify the request by hostname. A link_id is accepted too,
// which is what the internal record endpoint sends.
func (h *Handler) resolveLink(data map[string]interface{}) (linkID, userID string) {
	if id := getString(data, "link_id"); id != "" {
		owner, err := h.links.OwnerOf(id)
		if err != nil {
			// Unknown link: recording it would produce rows nothing can query.
			return "", ""
		}
		return id, owner
	}

	host := getString(data, "host")
	if host == "" {
		return "", ""
	}

	linkID, userID, err := h.links.ResolveByHost(host)
	if err != nil {
		// Traffic to a host we do not serve, or a link deleted since. Dropping
		// it is correct; there is no link to attribute it to.
		return "", ""
	}

	return linkID, userID
}

// tryRecordShortLinkVisit attempts to record a visit for a short link.
// Returns true if the host+path was a short link and the visit was recorded.
func (h *Handler) tryRecordShortLinkVisit(eventData map[string]interface{}, host, path string) bool {
	if h.shortLinks == nil {
		return false
	}

	linkID, userID, _, err := h.shortLinks.ResolveByHostPath(host, path)
	if err != nil || linkID == "" {
		return false
	}

	// Record the click
	isBot := getFloat(eventData, "score") >= 70
	country := getString(eventData, "country")
	device := getString(eventData, "device")
	ip := getString(eventData, "ip")
	userAgent := getString(eventData, "user_agent")

	if err := h.shortLinks.RecordClick(linkID, userID, isBot, country, device, ip, userAgent); err != nil {
		log.Printf("Short link click record failed for %s: %v", linkID, err)
	}

	return true
}

// tryRecordHostingVisit attempts to record a visit for a hosting domain.
// Returns true if the host was a hosting domain and the visit was recorded.
func (h *Handler) tryRecordHostingVisit(eventData map[string]interface{}, host string) bool {
	if h.hostingVisits == nil {
		return false
	}

	domainInfo, err := h.hostingVisits.GetDomainByHost(host)
	if err != nil || domainInfo == nil {
		return false
	}

	// Build hosting visit from event data
	action := getString(eventData, "action")
	visit := &HostingVisit{
		DomainID:  domainInfo.DomainID,
		AccountID: domainInfo.AccountID,

		IP:       getString(eventData, "ip"),
		Path:     getString(eventData, "path"),
		Method:   getString(eventData, "method"),
		Country:  getString(eventData, "country"),
		ASN:      getInt(eventData, "asn"),
		ASNOrg:   getString(eventData, "asn_org"),

		UserAgent:        getString(eventData, "user_agent"),
		Language:         getString(eventData, "language"),
		Timezone:         getString(eventData, "timezone"),
		ScreenResolution: getString(eventData, "screen_resolution"),

		Referrer:       getString(eventData, "referer"),
		ReferrerDomain: getString(eventData, "referrer_domain"),

		UTMSource:   getString(eventData, "utm_source"),
		UTMMedium:   getString(eventData, "utm_medium"),
		UTMCampaign: getString(eventData, "utm_campaign"),
		UTMTerm:     getString(eventData, "utm_term"),
		UTMContent:  getString(eventData, "utm_content"),

		BotScore:       getFloat(eventData, "score"),
		IsBot:          getFloat(eventData, "score") >= 70,
		BehaviorScore:  getInt(eventData, "behavior_score"),
		AutomationTool: getString(eventData, "automation_tool"),
		IsHeadless:     getBool(eventData, "is_headless"),
		IsTor:          getBool(eventData, "is_tor"),
		IsProxy:        getBool(eventData, "is_proxy"),
		IsDatacenter:   getBool(eventData, "is_datacenter"),
		Fingerprint:    getString(eventData, "fingerprint"),

		Action:      action,
		Blocked:     action == "block",
		BlockReason: getString(eventData, "reason"),

		SessionID: getString(eventData, "session_id"),
		CreatedAt: time.Now(),
	}

	// Detect device type from user agent
	visit.Device = h.detectDevice(visit.UserAgent)
	visit.Browser = h.detectBrowser(visit.UserAgent)
	visit.OS = h.detectOS(visit.UserAgent)

	if err := h.hostingVisits.RecordHostingVisit(visit); err != nil {
		log.Printf("Webhook: failed to record hosting visit for domain=%s: %v", host, err)
		return false
	}

	log.Printf("Webhook: recorded hosting visit for domain=%s ip=%s action=%s", host, visit.IP, action)
	return true
}

func (h *Handler) detectDevice(userAgent string) string {
	ua := userAgent
	if ua == "" {
		return "desktop"
	}
	if containsAny(ua, "Mobile", "Android", "iPhone", "iPad") {
		if containsAny(ua, "iPad", "Tablet") {
			return "tablet"
		}
		return "mobile"
	}
	return "desktop"
}

func (h *Handler) detectBrowser(userAgent string) string {
	ua := userAgent
	switch {
	case containsAny(ua, "Chrome"):
		return "Chrome"
	case containsAny(ua, "Firefox"):
		return "Firefox"
	case containsAny(ua, "Safari"):
		return "Safari"
	case containsAny(ua, "Edge"):
		return "Edge"
	case containsAny(ua, "Opera"):
		return "Opera"
	default:
		return "Other"
	}
}

func (h *Handler) detectOS(userAgent string) string {
	ua := userAgent
	switch {
	case containsAny(ua, "Windows"):
		return "Windows"
	case containsAny(ua, "Mac OS", "Macintosh"):
		return "macOS"
	case containsAny(ua, "Linux"):
		return "Linux"
	case containsAny(ua, "Android"):
		return "Android"
	case containsAny(ua, "iPhone", "iPad", "iOS"):
		return "iOS"
	default:
		return "Other"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// TrackingPixel handles client-side tracking from redirect pages
// No auth required - link ownership is validated via linkID lookup
func (h *Handler) TrackingPixel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LinkID    string `json:"linkId"`
		IP        string `json:"ip"`
		UserAgent string `json:"userAgent"`
		Referrer  string `json:"referrer"`
		Language  string `json:"language"`
		ScreenRes string `json:"screenRes"`
		Timezone  string `json:"timezone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.LinkID == "" {
		h.jsonError(w, "linkId required", http.StatusBadRequest)
		return
	}

	// Validate link exists and get owner
	owner, err := h.links.OwnerOf(req.LinkID)
	if err != nil || owner == "" {
		h.jsonError(w, "Unknown link", http.StatusNotFound)
		return
	}

	// Get real IP - prioritize Cloudflare's header
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" && req.IP != "" {
		ip = req.IP
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	// Strip port from RemoteAddr if present
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}

	// Get user agent from request header if not in body
	ua := req.UserAgent
	if ua == "" {
		ua = r.Header.Get("User-Agent")
	}

	// Lookup country from IP
	ipInfo, _ := geoip.Lookup(ip)
	var country, city string
	var lat, lon float64
	if ipInfo != nil {
		country = ipInfo.Country
		city = ipInfo.City
		lat = ipInfo.Latitude
		lon = ipInfo.Longitude
	}

	visit := &models.Visit{
		LinkID:    req.LinkID,
		UserID:    owner,
		IP:        ip,
		Country:   country,
		City:      city,
		Latitude:  lat,
		Longitude: lon,
		UserAgent: ua,
		Referrer:  req.Referrer,
		Language:  req.Language,
		ScreenRes: req.ScreenRes,
		Timezone:  req.Timezone,
		Device:    h.detectDevice(ua),
		Browser:   h.detectBrowser(ua),
		OS:        h.detectOS(ua),
		CreatedAt: time.Now(),
	}

	// TrackingPixel is client-side JS - if it fires, visitor reached the page.
	// blocked=false always for client-side tracking (proof of passage).
	visit.Blocked = false
	visit.BlockReason = ""

	if err := h.service.RecordVisit(visit); err != nil {
		log.Printf("Tracking pixel error: %v", err)
		h.jsonError(w, "Failed to record", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"blocked": false,
	})
}

// Helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

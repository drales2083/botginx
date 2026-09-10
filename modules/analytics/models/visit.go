package models

import "time"

type Visit struct {
	ID            string    `db:"id" json:"id"`
	LinkID        string    `db:"link_id" json:"linkId"`
	UserID        string    `db:"user_id" json:"userId"`
	SessionID     string    `db:"session_id" json:"sessionId"`
	IP            string    `db:"ip" json:"ip"`
	Country       string    `db:"country" json:"country"`
	City          string    `db:"city" json:"city"`
	Latitude      float64   `db:"latitude" json:"latitude"`
	Longitude     float64   `db:"longitude" json:"longitude"`
	ASN           int       `db:"asn" json:"asn"`
	ASNOrg        string    `db:"asn_org" json:"asnOrg"`
	Device        string    `db:"device" json:"device"`
	Browser       string    `db:"browser" json:"browser"`
	OS            string    `db:"os" json:"os"`
	Referrer      string    `db:"referrer" json:"referrer"`
	ReferrerDomain string   `db:"referrer_domain" json:"referrerDomain"`
	UserAgent     string    `db:"user_agent" json:"userAgent"`
	Language      string    `db:"language" json:"language"`
	Timezone      string    `db:"timezone" json:"timezone"`
	ScreenRes     string    `db:"screen_resolution" json:"screenResolution"`

	// UTM Tracking
	UTMSource   string `db:"utm_source" json:"utmSource"`
	UTMMedium   string `db:"utm_medium" json:"utmMedium"`
	UTMCampaign string `db:"utm_campaign" json:"utmCampaign"`
	UTMTerm     string `db:"utm_term" json:"utmTerm"`
	UTMContent  string `db:"utm_content" json:"utmContent"`

	// Bot Detection
	IsBot          bool    `db:"is_bot" json:"isBot"`
	BotType        string  `db:"bot_type" json:"botType"`
	BotScore       float64 `db:"bot_score" json:"botScore"`
	BotModule      string  `db:"bot_module" json:"botModule"`
	BehaviorScore  int     `db:"behavior_score" json:"behaviorScore"`
	AutomationTool string  `db:"automation_tool" json:"automationTool"`
	IsHeadless     bool    `db:"is_headless" json:"isHeadless"`
	IsTor          bool    `db:"is_tor" json:"isTor"`
	IsProxy        bool    `db:"is_proxy" json:"isProxy"`
	IsDatacenter   bool    `db:"is_datacenter" json:"isDatacenter"`
	CookiesEnabled bool    `db:"cookies_enabled" json:"cookiesEnabled"`
	JSEnabled      bool    `db:"js_enabled" json:"jsEnabled"`
	Fingerprint    string  `db:"fingerprint" json:"fingerprint"`

	// Action
	Action      string `db:"action" json:"action"` // allow, block, challenge
	IsUnique    bool   `db:"is_unique" json:"isUnique"`
	Blocked     bool   `db:"blocked" json:"blocked"`
	BlockReason string `db:"block_reason" json:"blockReason"`
	DurationUS  int64  `db:"duration_us" json:"durationUs"`

	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type Session struct {
	ID          string    `db:"id" json:"id"`
	SessionID   string    `db:"session_id" json:"sessionId"`
	LinkID      string    `db:"link_id" json:"linkId"`
	UserID      string    `db:"user_id" json:"userId"`
	IP          string    `db:"ip" json:"ip"`
	Country     string    `db:"country" json:"country"`
	UserAgent   string    `db:"user_agent" json:"userAgent"`
	Fingerprint string    `db:"fingerprint" json:"fingerprint"`
	StartedAt   time.Time `db:"started_at" json:"startedAt"`
	EndedAt     time.Time `db:"ended_at" json:"endedAt"`
	Duration    int       `db:"duration" json:"duration"`
	PageViews   int       `db:"page_views" json:"pageViews"`

	// Attribution
	ReferrerDomain string `db:"referrer_domain" json:"referrerDomain"`
	UTMSource      string `db:"utm_source" json:"utmSource"`
	UTMMedium      string `db:"utm_medium" json:"utmMedium"`
	UTMCampaign    string `db:"utm_campaign" json:"utmCampaign"`
}

type Conversion struct {
	ID              string    `db:"id" json:"id"`
	SessionID       string    `db:"session_id" json:"sessionId"`
	LinkID          string    `db:"link_id" json:"linkId"`
	UserID          string    `db:"user_id" json:"userId"`
	IP              string    `db:"ip" json:"ip"`
	Country         string    `db:"country" json:"country"`
	Path            string    `db:"path" json:"path"`
	TimeToConvert   int       `db:"time_to_convert" json:"timeToConvert"`
	ChallengeType   string    `db:"challenge_type" json:"challengeType"`
	BehaviorScore   int       `db:"behavior_score" json:"behaviorScore"`
	TrustTokenSolves int      `db:"trust_token_solves" json:"trustTokenSolves"`

	// Attribution
	ReferrerDomain string `db:"referrer_domain" json:"referrerDomain"`
	UTMSource      string `db:"utm_source" json:"utmSource"`
	UTMMedium      string `db:"utm_medium" json:"utmMedium"`
	UTMCampaign    string `db:"utm_campaign" json:"utmCampaign"`

	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type LinkSettings struct {
	ID                 string    `db:"id" json:"id"`
	LinkID             string    `db:"link_id" json:"linkId"`
	Template           string    `db:"template" json:"template"`     // Challenge template: cloudflare, humancheck, humansecurity, slidepuzzle, smartpuzzle, simple
	ThemeMode          string    `db:"theme_mode" json:"themeMode"`  // Theme: auto, light, dark
	CountryMode        string    `db:"country_mode" json:"countryMode"`
	CountryList        []string  `json:"countryList"`
	CountryListRaw     string    `db:"country_list" json:"-"`
	ASNMode            string    `db:"asn_mode" json:"asnMode"`
	ASNList            []string  `json:"asnList"`
	ASNListRaw         string    `db:"asn_list" json:"-"`
	DeviceMode         string    `db:"device_mode" json:"deviceMode"`
	DeviceList         []string  `json:"deviceList"`
	DeviceListRaw      string    `db:"device_list" json:"-"`
	BlockBots          bool      `db:"block_bots" json:"blockBots"`
	BlockTor           bool      `db:"block_tor" json:"blockTor"`
	BlockProxy         bool      `db:"block_proxy" json:"blockProxy"`
	BlockDatacenter    bool      `db:"block_datacenter" json:"blockDatacenter"`
	BlockHeadless      bool      `db:"block_headless" json:"blockHeadless"`
	MinBehaviorScore   int       `db:"min_behavior_score" json:"minBehaviorScore"`
	RedirectOnBlock    string    `db:"redirect_on_block" json:"redirectOnBlock"`
	UpdatedAt          time.Time `db:"updated_at" json:"updatedAt"`
}

type LinkStats struct {
	LinkID        string `json:"linkId"`
	TotalVisits   int    `json:"totalVisits"`
	UniqueVisits  int    `json:"uniqueVisits"`
	BotVisits     int    `json:"botVisits"`
	BlockedVisits int    `json:"blockedVisits"`
	TodayVisits   int    `json:"todayVisits"`
	Conversions   int    `json:"conversions"`
	ConversionRate float64 `json:"conversionRate"`
	AvgBehaviorScore float64 `json:"avgBehaviorScore"`
	TorVisits     int    `json:"torVisits"`
	ProxyVisits   int    `json:"proxyVisits"`
}

type CountryStats struct {
	Country string `db:"country" json:"country"`
	Count   int    `db:"count" json:"count"`
}

type DeviceStats struct {
	Device string `db:"device" json:"device"`
	Count  int    `db:"count" json:"count"`
}

type BrowserStats struct {
	Browser string `db:"browser" json:"browser"`
	Count   int    `db:"count" json:"count"`
}

type OSStats struct {
	OS    string `db:"os" json:"os"`
	Count int    `db:"count" json:"count"`
}

type TimelinePoint struct {
	Time  string `db:"time_bucket" json:"time"`
	Count int    `db:"count" json:"count"`
}

type ReferrerStats struct {
	Referrer string `db:"referrer" json:"referrer"`
	Count    int    `db:"count" json:"count"`
}

type UTMStats struct {
	Source   string `db:"utm_source" json:"source"`
	Medium   string `db:"utm_medium" json:"medium"`
	Campaign string `db:"utm_campaign" json:"campaign"`
	Count    int    `db:"count" json:"count"`
}

type BehaviorScoreDistribution struct {
	Range string `db:"score_range" json:"range"`
	Count int    `db:"count" json:"count"`
}

type AutomationStats struct {
	Tool  string `db:"automation_tool" json:"tool"`
	Count int    `db:"count" json:"count"`
}

type ThreatStats struct {
	TorCount       int `json:"torCount"`
	ProxyCount     int `json:"proxyCount"`
	DatacenterCount int `json:"datacenterCount"`
	HeadlessCount  int `json:"headlessCount"`
	AutomationCount int `json:"automationCount"`
	LowBehaviorCount int `json:"lowBehaviorCount"`
}

type TimelineMultiPoint struct {
	Time    string `db:"time_bucket" json:"time"`
	Total   int    `db:"total" json:"total"`
	Blocked int    `db:"blocked" json:"blocked"`
	Unique  int    `db:"unique_count" json:"unique"`
}

type VisitorPoint struct {
	Lat     float64   `db:"lat" json:"lat"`
	Lng     float64   `db:"lng" json:"lng"`
	Country string    `db:"country" json:"country"`
	City    string    `db:"city" json:"city"`
	Time    time.Time `db:"time" json:"time"`
	Blocked bool      `db:"blocked" json:"blocked"`
}

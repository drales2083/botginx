package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type LinkType string
type DeployStatus string

const (
	LinkTypeRedirect LinkType = "REDIRECT"
	LinkTypeHTML     LinkType = "HTML"

	DeployStatusPending  DeployStatus = "pending"
	DeployStatusDeployed DeployStatus = "deployed"
	DeployStatusFailed   DeployStatus = "failed"
)

// JSONArray for storing string arrays in JSONB
type JSONArray []string

func (j JSONArray) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	return json.Unmarshal(value.([]byte), j)
}

// JSONMap for storing objects in JSONB
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	return json.Unmarshal(value.([]byte), j)
}

// RedirectLink represents a redirect link configuration
type RedirectLink struct {
	ID                 string       `db:"id" json:"id"`
	UserID             string       `db:"user_id" json:"userId"`
	DomainID           string       `db:"domain_id" json:"domainId"`
	Subdomain          string       `db:"subdomain" json:"subdomain"`
	Path               string       `db:"path" json:"path"`
	Type               LinkType     `db:"type" json:"type"`
	DestinationURLs    JSONArray    `db:"destination_urls" json:"destinationUrls"`
	HTMLContent        *string      `db:"html_content" json:"htmlContent,omitempty"`
	Customization      JSONMap      `db:"customization" json:"customization,omitempty"`
	AnimationDuration  int          `db:"animation_duration" json:"animationDuration"`
	TurnstileEnabled   bool         `db:"turnstile_enabled" json:"turnstileEnabled"`
	TurnstileSiteKey   *string      `db:"turnstile_site_key" json:"turnstileSiteKey,omitempty"`
	TurnstileSecretKey *string      `db:"turnstile_secret_key" json:"-"`
	BotProtection      bool         `db:"bot_protection" json:"botProtection"`
	PassParams         bool         `db:"pass_params" json:"passParams"`
	DeployStatus       DeployStatus `db:"deploy_status" json:"deployStatus"`
	DeployError        *string      `db:"deploy_error" json:"deployError,omitempty"`
	DeployedURL        *string      `db:"deployed_url" json:"deployedUrl,omitempty"`
	IsActive           bool         `db:"is_active" json:"isActive"`
	CreatedAt          time.Time    `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time    `db:"updated_at" json:"updatedAt"`

	// Joined fields
	DomainName string `db:"domain_name" json:"domainName,omitempty"`
	ViewCount  int    `db:"view_count" json:"viewCount,omitempty"`
}

// BaseDomain returns the domain name without wildcard prefix
func (r *RedirectLink) BaseDomain() string {
	if len(r.DomainName) > 2 && r.DomainName[:2] == "*." {
		return r.DomainName[2:]
	}
	return r.DomainName
}

// FullURL returns the deployed URL
func (r *RedirectLink) FullURL() string {
	if r.DeployedURL != nil {
		return *r.DeployedURL
	}
	if r.DomainName != "" {
		return "https://" + r.Subdomain + "." + r.BaseDomain() + "/" + r.Path
	}
	return ""
}

type CreateRedirectLinkInput struct {
	DomainID         string   `json:"domainId" validate:"required"`
	Subdomain        string   `json:"subdomain" validate:"required"`
	Path             string   `json:"path" validate:"required"`
	Type             string   `json:"type" validate:"required"`
	DestinationURLs  []string `json:"destinationUrls"`
	HTMLContent      string   `json:"htmlContent"`
	TurnstileEnabled bool     `json:"turnstileEnabled"`
	BotProtection    bool     `json:"botProtection"`
	PassParams       bool     `json:"passParams"`
}

type UpdateRedirectLinkInput struct {
	DestinationURLs  []string `json:"destinationUrls"`
	HTMLContent      string   `json:"htmlContent"`
	TurnstileEnabled *bool    `json:"turnstileEnabled"`
	BotProtection    *bool    `json:"botProtection"`
	PassParams       *bool    `json:"passParams"`
}

// Customization settings for the redirect splash page
type Customization struct {
	// Background
	BgColor          string `json:"bgColor"`
	GradientEnabled  bool   `json:"gradientEnabled"`
	BgColorSecondary string `json:"bgColorSecondary"`
	Pattern          string `json:"pattern"`
	PatternColor     string `json:"patternColor"`

	// Loader
	Loader               string `json:"loader"`
	LoaderColorPrimary   string `json:"loaderColorPrimary"`
	LoaderColorSecondary string `json:"loaderColorSecondary"`

	// Text
	Heading           string `json:"heading"`
	HeadingVisible    bool   `json:"headingVisible"`
	Subheading        string `json:"subheading"`
	SubheadingVisible bool   `json:"subheadingVisible"`
	Font              string `json:"font"`
	FontWeight        int    `json:"fontWeight"`
	TextColor         string `json:"textColor"`
	TextSize          int    `json:"textSize"`
	TextShadow        bool   `json:"textShadow"`

	// Image
	ImageMode    string `json:"imageMode"`
	ImageDataURL string `json:"imageDataUrl"`
	ImageSize    int    `json:"imageSize"`
	ImageOverlay int    `json:"imageOverlay"`

	// Layout
	Layout    string `json:"layout"`
	Alignment string `json:"alignment"`
	VPos      int    `json:"vPos"`
	Gap       int    `json:"gap"`
	PageTitle string `json:"pageTitle"`
}

var DefaultCustomization = Customization{
	BgColor:              "#0a0a0a",
	GradientEnabled:      false,
	BgColorSecondary:     "#1a1a1a",
	Pattern:              "none",
	PatternColor:         "#333333",
	Loader:               "dots-bounce",
	LoaderColorPrimary:   "#3b82f6",
	LoaderColorSecondary: "#1e3a5f",
	Heading:              "Please Wait",
	HeadingVisible:       true,
	Subheading:           "Redirecting...",
	SubheadingVisible:    true,
	Font:                 "inter",
	FontWeight:           600,
	TextColor:            "#ffffff",
	TextSize:             32,
	TextShadow:           false,
	ImageMode:            "none",
	ImageSize:            150,
	ImageOverlay:         90,
	Layout:               "loader-text",
	Alignment:            "center",
	VPos:                 0,
	Gap:                  24,
	PageTitle:            "Redirecting",
}

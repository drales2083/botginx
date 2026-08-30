package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/redirectlinks/models"
	servermodels "github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/modules/redirectlinks/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/namegen"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/go-chi/chi/v5"
)

// DomainProvider supplies the domains a user may deploy to.
type DomainProvider interface {
	ListAvailable(userID string) ([]domainmodels.Domain, error)
}

// ServerPool hands out a deploy target chosen by the platform.
type ServerPool interface {
	PickRandom() (*servermodels.Server, error)
}

// ServerProvider returns SSH credentials for a domain's server
type ServerProvider interface {
	GetServerForDomain(domainID string) (ip string, port int, user, password string, err error)
}

type Handler struct {
	service        *services.RedirectLinkService
	templates      *module.TemplateEngine
	domains        DomainProvider
	servers        ServerPool
	serverProvider ServerProvider
}

func NewHandler(
	service *services.RedirectLinkService,
	templates *module.TemplateEngine,
	domains DomainProvider,
	servers ServerPool,
) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
		domains:   domains,
		servers:   servers,
	}
}

// SetServerProvider sets the server provider for VPS cleanup (called after init)
func (h *Handler) SetServerProvider(sp ServerProvider) {
	h.serverProvider = sp
}

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	links, _ := h.service.List(userID)

	module.RenderUserSection(w, r, h.templates, "redirectlinks:list.html", map[string]interface{}{
		"Title": "Redirect Links",
		"Links": links,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domains, _ := h.domains.ListAvailable(userID)

	module.RenderUserSection(w, r, h.templates, "redirectlinks:new.html", map[string]interface{}{
		"Title":              "Create Redirect Link",
		"Domains":            domains,
		"SuggestedSubdomain": namegen.Subdomain(),
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Redirect link not found", http.StatusNotFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "redirectlinks:show.html", map[string]interface{}{
		"Title": link.Subdomain,
		"Link":  link,
	})
}

func (h *Handler) Customize(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Redirect link not found", http.StatusNotFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "redirectlinks:customize.html", map[string]interface{}{
		"Title": "Customize Redirect",
		"Link":  link,
	})
}

// API handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	links, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"links": links})
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CreateRedirectLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	link, err := h.service.Create(userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-deploy the link immediately after creation
	go h.autoDeploy(link.ID)

	h.json(w, http.StatusCreated, map[string]interface{}{"link": link})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Redirect link not found", http.StatusNotFound)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"link": link})
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.UpdateRedirectLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	link, err := h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"link": link})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get link info before deleting for VPS cleanup
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}

	// Cleanup VPS files (async, don't block response)
	go h.cleanupVPS(link)

	// Delete from database
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

// cleanupVPS removes deployed files from the VPS
func (h *Handler) cleanupVPS(link *models.RedirectLink) {
	if h.serverProvider == nil {
		return
	}

	// Get server SSH details
	ip, port, user, password, err := h.serverProvider.GetServerForDomain(link.DomainID)
	if err != nil || ip == "" {
		return
	}

	// Connect to server
	portStr := fmt.Sprintf("%d", port)
	if port == 0 {
		portStr = "22"
	}

	client, err := sshexec.NewClient(ip, portStr, user, password)
	if err != nil {
		return
	}
	defer client.Close()

	// Delete botection settings file
	settingsPath := fmt.Sprintf("/etc/botection/links/%s.json", link.ID)
	client.Run(fmt.Sprintf("rm -f %s", settingsPath))

	// Delete site directory
	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", link.DomainName, link.Subdomain)
	client.Run(fmt.Sprintf("rm -rf %s", siteDir))
}

func (h *Handler) APIUpdateCustomization(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var customization models.JSONMap
	if err := json.NewDecoder(r.Body).Decode(&customization); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateCustomization(id, customization); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIUpdateURLs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input struct {
		URLs []string `json:"urls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(input.URLs) == 0 {
		h.jsonError(w, "At least one destination URL is required", http.StatusBadRequest)
		return
	}

	// Update the destination URLs
	link, err := h.service.UpdateDestinationURLs(id, input.URLs)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-redeploy with new URLs
	go h.autoDeploy(link.ID)

	h.json(w, http.StatusOK, map[string]interface{}{"success": true, "urls": link.DestinationURLs})
}

// APIRandomSubdomain suggests a readable subdomain like "amber-canyon". The
// form fills the field with it; the user is free to edit or ignore it.
func (h *Handler) APIRandomSubdomain(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]interface{}{
		"subdomain": namegen.Subdomain(),
	})
}

func (h *Handler) APIDeploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Redirect link not found", http.StatusNotFound)
		return
	}

	// The user never picks a server; the platform assigns one from the admin's
	// pool at deploy time.
	server, err := h.servers.PickRandom()
	if err != nil {
		h.jsonError(w, "No servers available. Contact an administrator.", http.StatusServiceUnavailable)
		return
	}

	// Build the full hostname
	fullHost := link.Subdomain + "." + link.DomainName
	deployedURL := "https://" + fullHost + "/" + link.Path

	// Connect to server via SSH
	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.SSHUser, server.SSHPassword)
	if err != nil {
		errMsg := "SSH connection failed: " + err.Error()
		h.service.SetDeployStatus(id, models.DeployStatusFailed, nil, &errMsg)
		h.jsonError(w, errMsg, http.StatusInternalServerError)
		return
	}
	defer client.Close()

	// Create site directory: /var/www/sites/{domain}/{subdomain}/
	// This matches the nginx upstream config which parses host as subdomain.domain
	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", link.DomainName, link.Subdomain)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", siteDir)
	client.Run(mkdirCmd)

	// Generate and write the redirect HTML page
	redirectHTML := generateRedirectHTML(link)
	htmlPath := fmt.Sprintf("%s/index.html", siteDir)
	writeHTMLCmd := fmt.Sprintf("cat > %s << 'HTMLEOF'\n%s\nHTMLEOF", htmlPath, redirectHTML)
	if _, err := client.Run(writeHTMLCmd); err != nil {
		errMsg := "Failed to write redirect page: " + err.Error()
		h.service.SetDeployStatus(id, models.DeployStatusFailed, nil, &errMsg)
		h.jsonError(w, errMsg, http.StatusInternalServerError)
		return
	}

	// Update deploy status
	h.service.SetDeployStatus(id, models.DeployStatusDeployed, &deployedURL, nil)

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"deployedUrl": deployedURL,
		"server":      server.Name,
	})
}

// autoDeploy deploys a link in the background (called after create)
func (h *Handler) autoDeploy(linkID string) {
	link, err := h.service.Get(linkID)
	if err != nil {
		return
	}

	server, err := h.servers.PickRandom()
	if err != nil {
		errMsg := "No servers available"
		h.service.SetDeployStatus(linkID, models.DeployStatusFailed, nil, &errMsg)
		return
	}

	fullHost := link.Subdomain + "." + link.DomainName
	deployedURL := "https://" + fullHost + "/" + link.Path

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.SSHUser, server.SSHPassword)
	if err != nil {
		errMsg := "SSH connection failed: " + err.Error()
		h.service.SetDeployStatus(linkID, models.DeployStatusFailed, nil, &errMsg)
		return
	}
	defer client.Close()

	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", link.DomainName, link.Subdomain)
	client.Run(fmt.Sprintf("mkdir -p %s", siteDir))

	redirectHTML := generateRedirectHTML(link)
	htmlPath := fmt.Sprintf("%s/index.html", siteDir)
	writeHTMLCmd := fmt.Sprintf("cat > %s << 'HTMLEOF'\n%s\nHTMLEOF", htmlPath, redirectHTML)
	if _, err := client.Run(writeHTMLCmd); err != nil {
		errMsg := "Failed to write redirect page: " + err.Error()
		h.service.SetDeployStatus(linkID, models.DeployStatusFailed, nil, &errMsg)
		return
	}

	h.service.SetDeployStatus(linkID, models.DeployStatusDeployed, &deployedURL, nil)
}

// generateRedirectHTML creates the redirect splash page HTML using customization settings
func generateRedirectHTML(link *models.RedirectLink) string {
	// Get first destination URL
	destURL := "https://example.com"
	if len(link.DestinationURLs) > 0 {
		destURL = link.DestinationURLs[0]
	}

	// Animation duration in seconds
	duration := link.AnimationDuration
	if duration <= 0 {
		duration = 3
	}

	// Extract customization with defaults
	bgColor := "#0a0a0a"
	bgColorSecondary := "#1a1a1a"
	gradientEnabled := false
	textColor := "#ffffff"
	textSize := 32
	loaderColor := "#3b82f6"
	loaderType := "spinner"
	heading := "Please Wait"
	subheading := "Redirecting..."
	pageTitle := "Redirecting"

	if link.Customization != nil {
		if v, ok := link.Customization["bgColor"].(string); ok && v != "" {
			bgColor = v
		}
		if v, ok := link.Customization["bgColorSecondary"].(string); ok && v != "" {
			bgColorSecondary = v
		}
		if v, ok := link.Customization["gradientEnabled"].(bool); ok {
			gradientEnabled = v
		}
		if v, ok := link.Customization["textColor"].(string); ok && v != "" {
			textColor = v
		}
		if v, ok := link.Customization["textSize"].(float64); ok && v > 0 {
			textSize = int(v)
		}
		if v, ok := link.Customization["loaderColorPrimary"].(string); ok && v != "" {
			loaderColor = v
		}
		if v, ok := link.Customization["loader"].(string); ok && v != "" {
			loaderType = v
		}
		if v, ok := link.Customization["heading"].(string); ok {
			heading = v
		}
		if v, ok := link.Customization["subheading"].(string); ok {
			subheading = v
		}
		if v, ok := link.Customization["pageTitle"].(string); ok && v != "" {
			pageTitle = v
		}
	}

	// Build background style
	bgStyle := bgColor
	if gradientEnabled {
		bgStyle = fmt.Sprintf("linear-gradient(135deg, %s 0%%, %s 100%%)", bgColor, bgColorSecondary)
	}

	// Build loader HTML based on type
	loaderHTML := ""
	loaderCSS := ""
	switch loaderType {
	case "dots-bounce":
		loaderHTML = `<div class="loader-dots"><span></span><span></span><span></span></div>`
		loaderCSS = fmt.Sprintf(`
        .loader-dots span {
            display: inline-block;
            width: 12px;
            height: 12px;
            margin: 0 4px;
            background: %s;
            border-radius: 50%%;
            animation: bounce 1.4s ease-in-out infinite both;
        }
        .loader-dots span:nth-child(1) { animation-delay: -0.32s; }
        .loader-dots span:nth-child(2) { animation-delay: -0.16s; }
        @keyframes bounce { 0%%, 80%%, 100%% { transform: scale(0); } 40%% { transform: scale(1); } }`, loaderColor)
	case "pulse":
		loaderHTML = `<div class="loader-pulse"></div>`
		loaderCSS = fmt.Sprintf(`
        .loader-pulse {
            width: 48px;
            height: 48px;
            background: %s;
            border-radius: 50%%;
            margin: 0 auto;
            animation: pulse 1.5s ease-in-out infinite;
        }
        @keyframes pulse { 0%%, 100%% { transform: scale(0.8); opacity: 0.5; } 50%% { transform: scale(1); opacity: 1; } }`, loaderColor)
	case "bars":
		loaderHTML = `<div class="loader-bars"><span></span><span></span><span></span><span></span></div>`
		loaderCSS = fmt.Sprintf(`
        .loader-bars span {
            display: inline-block;
            width: 6px;
            height: 32px;
            margin: 0 3px;
            background: %s;
            animation: bars 1.2s ease-in-out infinite;
        }
        .loader-bars span:nth-child(1) { animation-delay: 0s; }
        .loader-bars span:nth-child(2) { animation-delay: 0.1s; }
        .loader-bars span:nth-child(3) { animation-delay: 0.2s; }
        .loader-bars span:nth-child(4) { animation-delay: 0.3s; }
        @keyframes bars { 0%%, 40%%, 100%% { transform: scaleY(0.4); } 20%% { transform: scaleY(1); } }`, loaderColor)
	default: // spinner
		loaderHTML = `<div class="loader-spinner"></div>`
		loaderCSS = fmt.Sprintf(`
        .loader-spinner {
            width: 48px;
            height: 48px;
            border: 4px solid rgba(255,255,255,0.2);
            border-top-color: %s;
            border-radius: 50%%;
            margin: 0 auto;
            animation: spin 1s linear infinite;
        }
        @keyframes spin { to { transform: rotate(360deg); } }`, loaderColor)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: %s;
            color: %s;
            font-family: system-ui, -apple-system, sans-serif;
        }
        .container { text-align: center; padding: 2rem; }
        h1 { font-size: %dpx; margin-bottom: 1rem; font-weight: 600; }
        p { opacity: 0.7; font-size: 1rem; margin-top: 1rem; }
        .loader { margin: 1.5rem 0; }
        %s
    </style>
</head>
<body>
    <div class="container">
        <h1>%s</h1>
        <div class="loader">%s</div>
        <p>%s</p>
    </div>
    <script>
        setTimeout(function() {
            window.location.href = %q;
        }, %d000);
    </script>
</body>
</html>`, pageTitle, bgStyle, textColor, textSize, loaderCSS, heading, loaderHTML, subheading, destURL, duration)
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

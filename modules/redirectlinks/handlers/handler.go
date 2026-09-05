package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/redirectlinks/models"
	servermodels "github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/modules/redirectlinks/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/customizer"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/namegen"
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
		"SuggestedPath":      namegen.Path(),
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
		"Title":    "Customize Redirect",
		"Link":     link,
		"Loaders":  customizer.GetLoaders(),
		"Patterns": customizer.GetPatterns(),
		"Fonts":    customizer.GetFonts(),
	})
}

func (h *Handler) EditHTML(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Redirect link not found", http.StatusNotFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "redirectlinks:edit_html.html", map[string]interface{}{
		"Title": "Edit HTML",
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

// cleanupVPS removes deployed files from the Deploy VPS
func (h *Handler) cleanupVPS(link *models.RedirectLink) {
	if h.serverProvider == nil {
		return
	}

	// Get server SSH details
	ip, port, user, password, err := h.serverProvider.GetServerForDomain(link.DomainID)
	if err != nil || ip == "" {
		log.Printf("[cleanup] no server found for domain %s", link.DomainID)
		return
	}

	// Connect to server
	portStr := fmt.Sprintf("%d", port)
	if port == 0 {
		portStr = "22"
	}

	client, err := sshexec.NewClient(ip, portStr, user, password)
	if err != nil {
		log.Printf("[cleanup] failed to connect to %s: %v", ip, err)
		return
	}
	defer client.Close()

	baseDomain := link.BaseDomain()
	subdomain := link.Subdomain

	// Delete botection settings file
	settingsPath := fmt.Sprintf("/etc/botection/links/%s.json", link.ID)
	if _, err := client.Run(fmt.Sprintf("rm -f %s", settingsPath)); err == nil {
		log.Printf("[cleanup] deleted botection settings: %s", settingsPath)
	}

	// Delete site directory for this link
	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", baseDomain, subdomain)
	if _, err := client.Run(fmt.Sprintf("rm -rf %s", siteDir)); err == nil {
		log.Printf("[cleanup] deleted site directory: %s", siteDir)
	}

	// Clean up empty parent directory if no other subdomains exist
	parentDir := fmt.Sprintf("/var/www/sites/%s", baseDomain)
	client.Run(fmt.Sprintf("rmdir %s 2>/dev/null || true", parentDir))
}

// APIBulkDelete deletes multiple redirect links at once
func (h *Handler) APIBulkDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(input.IDs) == 0 {
		h.jsonError(w, "No IDs provided", http.StatusBadRequest)
		return
	}

	if len(input.IDs) > 100 {
		h.jsonError(w, "Maximum 100 items at once", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("userID").(string)
	deleted := 0
	failed := 0

	for _, id := range input.IDs {
		// Get link and verify ownership
		link, err := h.service.Get(id)
		if err != nil {
			failed++
			continue
		}

		// Verify ownership
		if link.UserID != userID {
			failed++
			continue
		}

		// Cleanup VPS files (async per link)
		go h.cleanupVPS(link)

		// Delete from database
		if err := h.service.Delete(id); err != nil {
			failed++
			continue
		}
		deleted++
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"failed":  failed,
		"total":   len(input.IDs),
	})
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

	// Auto-redeploy to VPS so visitors see changes immediately
	go h.autoDeploy(id)

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

// APIRandomPath suggests a single random word for use as a URL path.
func (h *Handler) APIRandomPath(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]interface{}{
		"path": namegen.Path(),
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

	// Build the full hostname (strip wildcard prefix for URL)
	fullHost := link.Subdomain + "." + link.BaseDomain()
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
	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", link.BaseDomain(), link.Subdomain)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", siteDir)
	client.Run(mkdirCmd)

	// Get HTML content based on link type
	deployHTML := getDeployHTML(link)
	htmlPath := fmt.Sprintf("%s/index.html", siteDir)
	writeHTMLCmd := fmt.Sprintf("cat > %s << 'HTMLEOF'\n%s\nHTMLEOF", htmlPath, deployHTML)
	if _, err := client.Run(writeHTMLCmd); err != nil {
		errMsg := "Failed to write page: " + err.Error()
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

	fullHost := link.Subdomain + "." + link.BaseDomain()
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

	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", link.BaseDomain(), link.Subdomain)
	client.Run(fmt.Sprintf("mkdir -p %s", siteDir))

	// Get HTML content based on link type
	deployHTML := getDeployHTML(link)
	htmlPath := fmt.Sprintf("%s/index.html", siteDir)
	writeHTMLCmd := fmt.Sprintf("cat > %s << 'HTMLEOF'\n%s\nHTMLEOF", htmlPath, deployHTML)
	if _, err := client.Run(writeHTMLCmd); err != nil {
		errMsg := "Failed to write page: " + err.Error()
		h.service.SetDeployStatus(linkID, models.DeployStatusFailed, nil, &errMsg)
		return
	}

	h.service.SetDeployStatus(linkID, models.DeployStatusDeployed, &deployedURL, nil)
}

// getDeployHTML returns the appropriate HTML based on link type
func getDeployHTML(link *models.RedirectLink) string {
	if link.Type == models.LinkTypeHTML && link.HTMLContent != nil && *link.HTMLContent != "" {
		html := *link.HTMLContent
		// Inject pass params helper if enabled
		if link.PassParams {
			helper := `<script>
window.getQueryParams = function() { return window.location.search; };
window.appendParams = function(url) {
    if (!window.location.search) return url;
    var sep = url.indexOf('?') >= 0 ? '&' : '?';
    return url + sep + window.location.search.substring(1);
};
</script>`
			// Insert before </body> or at end
			if idx := strings.Index(strings.ToLower(html), "</body>"); idx >= 0 {
				html = html[:idx] + helper + html[idx:]
			} else {
				html = html + helper
			}
		}
		return html
	}
	return generateRedirectHTML(link)
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

	// Convert JSONMap to customizer.Customization
	c := jsonMapToCustomization(link.Customization)

	// Generate HTML using the customizer package
	return customizer.GenerateHTML(customizer.GenerateOptions{
		Customization: c,
		RedirectURL:   destURL,
		Delay:         duration,
	})
}

// jsonMapToCustomization converts the stored JSONMap to a Customization struct
func jsonMapToCustomization(m models.JSONMap) customizer.Customization {
	// Start with defaults
	c := customizer.GetDefaultCustomization()

	if m == nil {
		return c
	}

	// Background
	if v, ok := m["bgColor"].(string); ok && v != "" {
		c.BgColor = v
	}
	if v, ok := m["bgColorSecondary"].(string); ok && v != "" {
		c.BgColorSecondary = v
	}
	if v, ok := m["gradientEnabled"].(bool); ok {
		c.GradientEnabled = v
	}
	if v, ok := m["pattern"].(string); ok && v != "" {
		c.Pattern = v
	}
	if v, ok := m["patternColor"].(string); ok && v != "" {
		c.PatternColor = v
	}
	if v, ok := m["patternOpacity"].(float64); ok {
		c.PatternOpacity = int(v)
	}

	// Loader
	if v, ok := m["loader"].(string); ok && v != "" {
		c.Loader = v
	}
	if v, ok := m["loaderColorPrimary"].(string); ok && v != "" {
		c.LoaderColorPrimary = v
	}
	if v, ok := m["loaderColorSecondary"].(string); ok && v != "" {
		c.LoaderColorSecondary = v
	}

	// Text
	if v, ok := m["heading"].(string); ok {
		c.Heading = v
	}
	if v, ok := m["headingVisible"].(bool); ok {
		c.HeadingVisible = v
	}
	if v, ok := m["subheading"].(string); ok {
		c.Subheading = v
	}
	if v, ok := m["subheadingVisible"].(bool); ok {
		c.SubheadingVisible = v
	}
	if v, ok := m["font"].(string); ok && v != "" {
		c.Font = v
	}
	if v, ok := m["fontWeight"].(float64); ok && v > 0 {
		c.FontWeight = int(v)
	}
	if v, ok := m["textColor"].(string); ok && v != "" {
		c.TextColor = v
	}
	if v, ok := m["textSize"].(float64); ok && v > 0 {
		c.TextSize = int(v)
	}
	if v, ok := m["textShadow"].(bool); ok {
		c.TextShadow = v
	}

	// Image
	if v, ok := m["imageMode"].(string); ok {
		c.ImageMode = v
	}
	if v, ok := m["imageDataUrl"].(string); ok {
		c.ImageDataURL = v
	}
	if v, ok := m["imageSize"].(float64); ok && v > 0 {
		c.ImageSize = int(v)
	}
	if v, ok := m["imageOverlay"].(float64); ok {
		c.ImageOverlay = int(v)
	}

	// Layout
	if v, ok := m["contentOrder"].(string); ok && v != "" {
		c.ContentOrder = v
	}
	if v, ok := m["textAlign"].(string); ok && v != "" {
		c.TextAlign = v
	}
	if v, ok := m["vPos"].(float64); ok {
		c.VPos = int(v)
	}
	if v, ok := m["gap"].(float64); ok && v > 0 {
		c.Gap = int(v)
	}
	if v, ok := m["pageTitle"].(string); ok && v != "" {
		c.PageTitle = v
	}

	return c
}

// getPassParamsJS returns JavaScript to append query params to destination if enabled
func getPassParamsJS(enabled bool) string {
	if !enabled {
		return ""
	}
	return `if (window.location.search) {
                var sep = dest.indexOf('?') >= 0 ? '&' : '?';
                dest = dest + sep + window.location.search.substring(1);
            }`
}

// APICustomizerPreview generates a preview of the customized page
func (h *Handler) APICustomizerPreview(w http.ResponseWriter, r *http.Request) {
	var c customizer.Customization
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate preview HTML (no redirect)
	html := customizer.GeneratePreviewHTML(c)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
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

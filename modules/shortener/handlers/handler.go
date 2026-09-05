package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	servermodels "github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/modules/shortener/models"
	"github.com/botginx/botginx/modules/shortener/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/protection"
	"github.com/botginx/botginx/pkg/settingspush"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
)

type DomainProvider interface {
	ListAvailable(userID string) ([]domainmodels.Domain, error)
}

// ServerPool hands out a deploy target chosen by the platform.
type ServerPool interface {
	PickRandom() (*servermodels.Server, error)
}

type Handler struct {
	service   *services.ShortenerService
	templates *module.TemplateEngine
	domains   DomainProvider
	servers   ServerPool
	pusher    *settingspush.Pusher
}

func NewHandler(service *services.ShortenerService, templates *module.TemplateEngine, domains DomainProvider, servers ServerPool) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
		domains:   domains,
		servers:   servers,
		pusher:    settingspush.New(),
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]string{"error": message})
}

// Page Handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	links, _ := h.service.List(userID)

	module.RenderUserSection(w, r, h.templates, "shortener:list.html", map[string]interface{}{
		"Title": "Short Links",
		"Links": links,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domains, _ := h.domains.ListAvailable(userID)

	module.RenderUserSection(w, r, h.templates, "shortener:new.html", map[string]interface{}{
		"Title":             "Create Short Link",
		"Domains":           domains,
		"DefaultProtection": protection.GetDefaultSettings(),
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	stats, _ := h.service.GetStats(id, 30)

	module.RenderUserSection(w, r, h.templates, "shortener:show.html", map[string]interface{}{
		"Title":    "Short Link",
		"Link":     link,
		"Stats":    stats,
		"Settings": link.ProtectionSettings,
	})
}

func (h *Handler) QRCode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	size := 256
	if s := r.URL.Query().Get("size"); s == "512" {
		size = 512
	}

	png, err := qrcode.Encode(link.FullURL(), qrcode.Medium, size)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "attachment; filename=\"qr-"+link.Path+".png\"")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}

// API Handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	links, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, links)
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}

	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.json(w, http.StatusOK, link)
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CreateShortLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate
	if input.DomainID == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}
	if input.Path == "" {
		h.jsonError(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Auto-generate subdomain if not provided
	if input.Subdomain == "" {
		input.Subdomain = h.service.GenerateSubdomain()
	}
	if len(input.Destinations) == 0 {
		h.jsonError(w, "At least one destination URL is required", http.StatusBadRequest)
		return
	}

	// ensureUniqueSubdomain in Create() handles collisions
	link, err := h.service.Create(userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-deploy the link immediately after creation
	go h.autoDeploy(link.ID)

	h.json(w, http.StatusCreated, link)
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input models.UpdateShortLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	updated, err := h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-redeploy to VPS so visitors see changes immediately
	go h.autoDeploy(id)

	h.json(w, http.StatusOK, updated)
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) APIRandomPath(w http.ResponseWriter, r *http.Request) {
	path := h.service.GenerateRandomPath(6)
	h.json(w, http.StatusOK, map[string]string{"path": path})
}

func (h *Handler) APIRandomSubdomain(w http.ResponseWriter, r *http.Request) {
	subdomain := h.service.GenerateSubdomain()
	h.json(w, http.StatusOK, map[string]string{"subdomain": subdomain})
}

func (h *Handler) APIStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	stats, err := h.service.GetStats(id, 30)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APICheckPath(w http.ResponseWriter, r *http.Request) {
	domainID := r.URL.Query().Get("domainId")
	subdomain := r.URL.Query().Get("subdomain")
	path := r.URL.Query().Get("path")

	if domainID == "" || subdomain == "" || path == "" {
		h.jsonError(w, "domainId, subdomain and path required", http.StatusBadRequest)
		return
	}

	available, _ := h.service.CheckPathAvailable(domainID, subdomain, path)
	h.json(w, http.StatusOK, map[string]bool{"available": available})
}

// autoDeploy deploys a short link HTML file to the Deploy VPS.
// The HTML instantly redirects to the destination URL.
func (h *Handler) autoDeploy(linkID string) {
	link, err := h.service.Get(linkID)
	if err != nil {
		return
	}

	server, err := h.servers.PickRandom()
	if err != nil {
		errMsg := "No servers available"
		h.service.SetDeployStatus(linkID, "failed", nil, &errMsg)
		return
	}

	// For short links: subdomain.basedomain/path -> /var/www/sites/{baseDomain}/{subdomain}/
	baseDomain := link.BaseDomain()
	if link.Subdomain == "" {
		errMsg := "Subdomain required for short links"
		h.service.SetDeployStatus(linkID, "failed", nil, &errMsg)
		return
	}
	fullHost := link.Subdomain + "." + baseDomain
	deployedURL := "https://" + fullHost + "/" + link.Path

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.SSHUser, server.SSHPassword)
	if err != nil {
		errMsg := "SSH connection failed: " + err.Error()
		h.service.SetDeployStatus(linkID, "failed", nil, &errMsg)
		return
	}
	defer client.Close()

	// Create directory: /var/www/sites/{baseDomain}/{subdomain}/
	siteDir := fmt.Sprintf("/var/www/sites/%s/%s", baseDomain, link.Subdomain)
	client.Run(fmt.Sprintf("mkdir -p %s", siteDir))

	// Generate redirect HTML
	deployHTML := generateShortLinkHTML(link)
	htmlPath := fmt.Sprintf("%s/index.html", siteDir)
	writeHTMLCmd := fmt.Sprintf("cat > %s << 'HTMLEOF'\n%s\nHTMLEOF", htmlPath, deployHTML)
	if _, err := client.Run(writeHTMLCmd); err != nil {
		errMsg := "Failed to write page: " + err.Error()
		h.service.SetDeployStatus(linkID, "failed", nil, &errMsg)
		return
	}

	// Push protection settings to botection
	h.pushSettings(link, server, fullHost)

	h.service.SetDeployStatus(linkID, "deployed", &deployedURL, nil)
}

// pushSettings sends protection settings to the deploy VPS
func (h *Handler) pushSettings(link *models.ShortLink, server *servermodels.Server, host string) {
	settings := link.ProtectionSettings

	// Get first destination as redirect on block
	redirectOnBlock := ""
	if len(link.Destinations) > 0 {
		redirectOnBlock = link.Destinations[0]
	}

	pushSettings := settingspush.LinkSettings{
		LinkID:           link.ID,
		UserID:           link.UserID,
		Host:             host + "/" + link.Path,
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
		RedirectOnBlock:  redirectOnBlock,
	}

	port := server.Port
	if port == 0 {
		port = 22
	}

	serverInfo := settingspush.ServerInfo{
		IP:       server.IP,
		Port:     port,
		User:     server.SSHUser,
		Password: server.SSHPassword,
	}

	if err := h.pusher.Push(serverInfo, link.ID, pushSettings); err != nil {
		log.Printf("Settings push failed for short link %s: %v", link.ID, err)
	} else {
		log.Printf("Settings pushed for short link %s to %s", link.ID, server.IP)
	}
}

// generateShortLinkHTML creates instant redirect HTML for a short link
func generateShortLinkHTML(link *models.ShortLink) string {
	if len(link.Destinations) == 0 {
		return `<!DOCTYPE html><html><body>No destination configured</body></html>`
	}

	// Single destination: simple redirect
	if len(link.Destinations) == 1 {
		destURL := link.Destinations[0]
		return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="refresh" content="0;url=%s">
<script>window.location.replace("%s");</script>
</head>
<body></body>
</html>`, destURL, destURL)
	}

	// Multiple destinations: JavaScript random selection
	// Build JSON array of destinations
	destJSON, _ := json.Marshal(link.Destinations)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
var d=%s;
var u=d[Math.floor(Math.random()*d.length)];
window.location.replace(u);
</script>
</head>
<body></body>
</html>`, string(destJSON))
}


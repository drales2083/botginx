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

type Handler struct {
	service   *services.RedirectLinkService
	templates *module.TemplateEngine
	domains   DomainProvider
	servers   ServerPool
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
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"deleted": true})
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

	// Generate nginx config for the redirect
	nginxConfig := generateRedirectNginxConfig(link)

	// Create site directory
	mkdirCmd := fmt.Sprintf("mkdir -p /var/www/sites/%s", fullHost)
	client.Run(mkdirCmd)

	// Write nginx config
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", fullHost)
	writeCmd := fmt.Sprintf("cat > %s << 'NGINXEOF'\n%s\nNGINXEOF", configPath, nginxConfig)
	if _, err := client.Run(writeCmd); err != nil {
		errMsg := "Failed to write nginx config: " + err.Error()
		h.service.SetDeployStatus(id, models.DeployStatusFailed, nil, &errMsg)
		h.jsonError(w, errMsg, http.StatusInternalServerError)
		return
	}

	// Enable site and reload nginx
	enableCmd := fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/ && nginx -t && systemctl reload nginx", configPath)
	if _, err := client.Run(enableCmd); err != nil {
		errMsg := "Failed to enable site: " + err.Error()
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

// generateRedirectNginxConfig creates nginx config for a redirect link
func generateRedirectNginxConfig(link *models.RedirectLink) string {
	fullHost := link.Subdomain + "." + link.DomainName

	// For a simple redirect, we proxy to botection which handles the logic
	// The redirect splash page is served by botection based on link settings
	return fmt.Sprintf(`server {
    listen 80;
    listen 443 ssl;
    server_name %s;

    ssl_certificate /etc/nginx/ssl/%s.pem;
    ssl_certificate_key /etc/nginx/ssl/%s.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, fullHost, link.DomainName, link.DomainName)
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

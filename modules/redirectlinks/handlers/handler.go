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

	// Get customization with defaults
	c := models.DefaultCustomization
	if link.Customization != nil {
		if v, ok := link.Customization["bgColor"].(string); ok && v != "" {
			c.BgColor = v
		}
		if v, ok := link.Customization["heading"].(string); ok {
			c.Heading = v
		}
		if v, ok := link.Customization["subheading"].(string); ok {
			c.Subheading = v
		}
		if v, ok := link.Customization["textColor"].(string); ok && v != "" {
			c.TextColor = v
		}
		if v, ok := link.Customization["loaderColorPrimary"].(string); ok && v != "" {
			c.LoaderColorPrimary = v
		}
		if v, ok := link.Customization["pageTitle"].(string); ok && v != "" {
			c.PageTitle = v
		}
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
        .container { text-align: center; }
        h1 { font-size: 2rem; margin-bottom: 1rem; font-weight: 600; }
        p { opacity: 0.7; font-size: 1rem; }
        .loader {
            width: 48px;
            height: 48px;
            border: 4px solid rgba(255,255,255,0.2);
            border-top-color: %s;
            border-radius: 50%%;
            margin: 1.5rem auto;
            animation: spin 1s linear infinite;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s</h1>
        <div class="loader"></div>
        <p>%s</p>
    </div>
    <script>
        setTimeout(function() {
            window.location.href = %q;
        }, %d000);
    </script>
</body>
</html>`, c.PageTitle, c.BgColor, c.TextColor, c.LoaderColorPrimary, c.Heading, c.Subheading, destURL, duration)
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

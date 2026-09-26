package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service       *services.DomainService
	cpanelService *services.CpanelService
	verification  *services.VerificationService
	templates     *module.TemplateEngine
}

// canAccessDomain checks if the user owns a domain or is admin accessing a shared domain
func (h *Handler) canAccessDomain(r *http.Request, domain *models.Domain) bool {
	user := ctx.GetUser(r)
	if user == nil {
		return false
	}
	// User owns the domain
	if domain.UserID == user.ID {
		return true
	}
	// Admin can access shared domains
	if user.IsAdmin() && domain.IsShared {
		return true
	}
	return false
}

func NewHandler(service *services.DomainService, cpanelService *services.CpanelService, templates *module.TemplateEngine) *Handler {
	vs := services.NewVerificationService()

	// Set server provider to get credentials from database
	vs.SetServerProvider(func() (*services.ServerInfo, error) {
		ip, port, user, pass, err := service.GetDeployServer()
		if err != nil {
			return nil, err
		}
		return &services.ServerInfo{IP: ip, Port: port, User: user, Password: pass}, nil
	})

	return &Handler{
		service:       service,
		cpanelService: cpanelService,
		verification:  vs,
		templates:     templates,
	}
}

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domains, _ := h.service.List(userID)

	module.RenderUserSection(w, r, h.templates, "domains:list.html", map[string]interface{}{
		"Title":   "Domains",
		"Domains": domains,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, h.templates, "domains:new.html", map[string]interface{}{
		"Title": "Add Domain",
	})
}

func (h *Handler) CpanelConnect(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, h.templates, "domains:cpanel_connect.html", map[string]interface{}{
		"Title": "Connect cPanel",
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership - users can only view their own domains
	if !h.canAccessDomain(r, domain) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Get deploy server IP for DNS instructions
	deployIP := h.service.GetDeployIP()

	data := map[string]interface{}{
		"Title":    domain.Name,
		"Domain":   domain,
		"ServerIP": deployIP,
	}

	// If admin, pass users list for assign feature
	user := ctx.GetUser(r)
	if user != nil && user.IsAdmin() {
		users, _ := h.service.ListAllUsers()
		data["Users"] = users
	}

	module.RenderUserSection(w, r, h.templates, "domains:show.html", data)
}

// Settings renders the domain settings page (Turnstile credentials, etc.)
func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	if !h.canAccessDomain(r, domain) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Get base domain for Turnstile setup instructions
	baseDomain := domain.Name
	if domain.IsWildcard {
		baseDomain = strings.TrimPrefix(domain.Name, "*.")
	}

	data := map[string]interface{}{
		"Title":      domain.Name + " Settings",
		"Domain":     domain,
		"BaseDomain": baseDomain,
	}

	module.RenderUserSection(w, r, h.templates, "domains:settings.html", data)
}

// Admin page handlers -- the shared platform pool

func (h *Handler) SharedList(w http.ResponseWriter, r *http.Request) {
	domains, _ := h.service.ListShared()

	module.Render(w, r, h.templates, "domains:shared_list.html", map[string]interface{}{
		"Title":   "Shared Domains",
		"Domains": domains,
		"Enabled": h.service.SharedDomainsFlag(),
		"Active":  h.service.SharedDomainsEnabled(),
	})
}

func (h *Handler) SharedNew(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "domains:shared_new.html", map[string]interface{}{
		"Title": "Add Shared Domain",
	})
}

// Admin API handlers

func (h *Handler) APISharedList(w http.ResponseWriter, r *http.Request) {
	domains, err := h.service.ListShared()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{
		"domains": domains,
		"enabled": h.service.SharedDomainsFlag(),
		"active":  h.service.SharedDomainsEnabled(),
	})
}

func (h *Handler) APISharedCreate(w http.ResponseWriter, r *http.Request) {
	adminID := ctx.GetUserID(r)

	var input models.CreateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		h.jsonError(w, "Domain name is required", http.StatusBadRequest)
		return
	}

	domain, err := h.service.CreateShared(adminID, input)
	if err != nil {
		if err == services.ErrDomainExists {
			h.jsonError(w, "Domain already registered", http.StatusConflict)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"domain": domain})
}

func (h *Handler) APIToggleShared(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.SetSharedDomainsEnabled(input.Enabled); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"enabled": input.Enabled,
		"active":  h.service.SharedDomainsEnabled(),
	})
}

// SharedSetup shows the setup wizard for shared domains (admin version)
func (h *Handler) SharedSetup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Verify it's a shared domain
	if !domain.IsShared {
		http.Error(w, "Not a shared domain", http.StatusBadRequest)
		return
	}

	// Get deploy server IP
	deployIP := h.service.GetDeployIP()

	// Build setup info
	baseDomain := services.GetBaseDomain(domain.Name)
	isWildcard := domain.IsWildcard || len(domain.Name) > 2 && domain.Name[:2] == "*."

	setupInfo := models.ExternalSetupInfo{
		Domain:        domain.Name,
		BaseDomain:    baseDomain,
		IsWildcard:    isWildcard,
		ServerIP:      deployIP,
		VerifyToken:   domain.VerifyToken,
		SetupStep:     domain.SetupStep,
		VerifyTXTName: "_guardbot-verify." + baseDomain,
		AcmeTXTName:   "_acme-challenge." + baseDomain,
	}

	// Check for acme-dns registration (all domains use wildcard SSL)
	if domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
		setupInfo.AcmeCnameTarget = *domain.AcmeFulldomain
		setupInfo.AcmeCnameVerified = domain.AcmeCnameVerified
		setupInfo.UseAcmeDns = true

		if !domain.AcmeCnameVerified {
			if h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain) {
				verified := true
				h.service.Update(domain.ID, models.UpdateDomainInput{AcmeCnameVerified: &verified})
				setupInfo.AcmeCnameVerified = true
			}
		}
	} else {
		// All domains need acme-dns for wildcard SSL support
		go h.registerAcmeDnsBackground(domain.ID)
	}

	// Legacy ACME token
	if domain.AcmeToken != nil && *domain.AcmeToken != "" {
		setupInfo.AcmeToken = *domain.AcmeToken
		setupInfo.AcmeTokenReady = true
	}

	module.Render(w, r, h.templates, "domains:shared_setup.html", map[string]interface{}{
		"Title":     "Setup " + domain.Name,
		"Domain":    domain,
		"SetupInfo": setupInfo,
	})
}

// AdminAssign shows the admin page to assign domains to users
func (h *Handler) AdminAssign(w http.ResponseWriter, r *http.Request) {
	// Get all users for dropdown
	users, _ := h.service.ListAllUsers()

	module.Render(w, r, h.templates, "domains:admin_assign.html", map[string]interface{}{
		"Title": "Assign Domain to User",
		"Users": users,
	})
}

// APIAdminAssign creates a domain and assigns it to a specific user
func (h *Handler) APIAdminAssign(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name   string `json:"name"`
		UserID string `json:"userId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate
	if input.Name == "" {
		h.jsonError(w, "Domain name required", http.StatusBadRequest)
		return
	}
	if input.UserID == "" {
		h.jsonError(w, "User ID required", http.StatusBadRequest)
		return
	}

	// Clean the domain name
	input.Name = strings.ToLower(strings.TrimSpace(input.Name))
	input.Name = strings.TrimPrefix(input.Name, "http://")
	input.Name = strings.TrimPrefix(input.Name, "https://")
	input.Name = strings.TrimSuffix(input.Name, "/")

	// Create the domain for the specified user
	domain, err := h.service.CreateForUser(input.UserID, models.CreateDomainInput{
		Name: input.Name,
	})
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"domain":  domain,
	})
}

// APITransferOwnership transfers a domain to another user (admin only)
func (h *Handler) APITransferOwnership(w http.ResponseWriter, r *http.Request) {
	// Verify admin
	user := ctx.GetUser(r)
	if user == nil || !user.IsAdmin() {
		h.jsonError(w, "Admin access required", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	var input struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.UserID == "" {
		h.jsonError(w, "User ID required", http.StatusBadRequest)
		return
	}

	// Transfer ownership
	if err := h.service.TransferOwnership(id, input.UserID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Domain transferred successfully",
	})
}

// API handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domains, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"domains": domains})
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CreateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		h.jsonError(w, "Domain name is required", http.StatusBadRequest)
		return
	}

	domain, err := h.service.Create(userID, input)
	if err != nil {
		if err == services.ErrDomainExists {
			h.jsonError(w, "Domain already registered", http.StatusConflict)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If cPanel connection provided, auto-add verification TXT record
	cpanelAutoDNS := false
	if domain.CpanelConnectionID != nil && *domain.CpanelConnectionID != "" && h.cpanelService != nil {
		if h.cpanelService.TriggerAutoDNS(domain.ID, domain.Name, domain.VerifyToken, *domain.CpanelConnectionID) {
			cpanelAutoDNS = true
			log.Printf("[domains] cPanel auto-DNS triggered for %s", domain.Name)
		}
	}

	// All domains use DNS-01 challenge via acme-dns for wildcard SSL
	// This ensures subdomains work for all domain types (cPanel-style)
	needsSetupWizard := true
	var cnameTarget string

	// Update domain to mark as external setup pending
	setupType := models.SetupTypeExternal
	setupStep := models.SetupStepDNSWaiting
	h.service.Update(domain.ID, models.UpdateDomainInput{
		SetupType: &setupType,
		SetupStep: &setupStep,
	})

	// Auto-register with acme-dns for wildcard SSL support
	reg, err := h.service.RegisterWithAcmeDNS(domain.ID)
	if err != nil {
		log.Printf("[domains] auto acme-dns registration failed for %s: %v", domain.Name, err)
	} else {
		cnameTarget = reg.Fulldomain
		log.Printf("[domains] auto-registered %s with acme-dns: %s", domain.Name, cnameTarget)

		// For cPanel domains, auto-add the CNAME record
		if cpanelAutoDNS && domain.CpanelConnectionID != nil && h.cpanelService != nil {
			if err := h.cpanelService.AddAcmeCNAME(*domain.CpanelConnectionID, domain.Name, cnameTarget); err != nil {
				log.Printf("[domains] cPanel auto-CNAME failed for %s: %v", domain.Name, err)
			} else {
				log.Printf("[domains] cPanel auto-CNAME added for %s → %s", domain.Name, cnameTarget)
				// Mark CNAME as verified since we added it ourselves
				verified := true
				h.service.Update(domain.ID, models.UpdateDomainInput{
					AcmeCnameVerified: &verified,
				})
				needsSetupWizard = false // All DNS is auto-configured
			}
		}
	}

	resp := map[string]interface{}{
		"domain":           domain,
		"needsSetupWizard": needsSetupWizard,
		"cpanelAutoDNS":    cpanelAutoDNS,
	}
	if cnameTarget != "" {
		resp["cnameTarget"] = cnameTarget
	}
	h.json(w, http.StatusCreated, resp)
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership - users can only view their own domains
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// If check param is set, include link count for delete confirmation
	if r.URL.Query().Get("check") == "true" {
		linkCount, _ := h.service.CountRedirectLinks(id)
		h.json(w, http.StatusOK, map[string]interface{}{
			"domain":     domain,
			"link_count": linkCount,
		})
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"domain": domain})
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check ownership first
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	var input models.UpdateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	domain, err = h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"domain": domain})
}

// APIUpdateSettings updates domain settings (Turnstile credentials)
func (h *Handler) APIUpdateSettings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	var input struct {
		TurnstileSiteKey   string `json:"turnstile_site_key"`
		TurnstileSecretKey string `json:"turnstile_secret_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateTurnstile(id, input.TurnstileSiteKey, input.TurnstileSecretKey); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIGetTurnstileStatus returns whether a domain has Turnstile keys configured
func (h *Handler) APIGetTurnstileStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	hasKeys := domain.TurnstileSiteKey != nil && *domain.TurnstileSiteKey != "" &&
		domain.TurnstileSecretKey != nil && *domain.TurnstileSecretKey != ""

	h.json(w, http.StatusOK, map[string]interface{}{"hasKeys": hasKeys})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get domain first to have the name for cleanup
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership - users can only delete their own domains
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Check if force delete is requested
	force := r.URL.Query().Get("force") == "true"

	// Count redirect links using this domain
	linkCount, _ := h.service.CountRedirectLinks(id)

	// If there are links and not force, return warning
	if linkCount > 0 && !force {
		h.json(w, http.StatusConflict, map[string]interface{}{
			"warning":    true,
			"link_count": linkCount,
			"message":    fmt.Sprintf("This domain has %d redirect link(s). Deleting will permanently remove all links and their analytics data.", linkCount),
			"domain":     domain.Name,
		})
		return
	}

	// Cleanup VPS files and cPanel DNS (async to not block response)
	go func() {
		// Clean up domain files: nginx config, SSL certs, site directories
		if cleanupErr := h.verification.CleanupDomain(domain.Name); cleanupErr != nil {
			log.Printf("[domains] cleanup failed for %s (continuing with DB delete): %v", domain.Name, cleanupErr)
		} else {
			log.Printf("[domains] cleaned up VPS files for %s", domain.Name)
		}

		// Also clean up redirect link botection settings by link ID
		// (CleanupDomain handles site dirs, but botection settings are by link ID)
		if linkCount > 0 {
			if linkIDs, err := h.service.GetRedirectLinkIDs(id); err == nil {
				h.verification.CleanupRedirectLinkSettings(linkIDs)
			}
		}

		// Clean up cPanel DNS records if this domain was added via cPanel
		if domain.CpanelAutoDNS && domain.CpanelConnectionID != nil && h.cpanelService != nil {
			log.Printf("[domains] cleaning up cPanel DNS for %s", domain.Name)
			h.cpanelService.CleanupDomainDNS(*domain.CpanelConnectionID, domain.Name)
		}
	}()

	// Delete from database (cascade removes redirect_links)
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"deleted":       true,
		"links_deleted": linkCount,
	})
}

func (h *Handler) APIVerifyDNS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Verify via TXT record lookup (works with Cloudflare proxy)
	verified, verifyErr := h.verification.VerifyDNS(domain.Name, domain.VerifyToken)

	// Update the domain record
	_, err = h.service.Update(id, models.UpdateDomainInput{
		DNSVerified: &verified,
	})
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := map[string]interface{}{
		"verified": verified,
		"domain":   domain.Name,
	}
	if verifyErr != nil && !verified {
		result["message"] = verifyErr.Error()
	}

	// Auto-setup SSL and nginx after successful verification
	if verified && !domain.SSLEnabled {
		go func() {
			// Generate SSL
			if err := h.verification.GenerateSSL(domain.Name); err != nil {
				return
			}
			// Setup nginx
			if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
				return
			}
			// Mark SSL enabled and assign server
			t := true
			serverID := h.service.GetDeployServerID()
			h.service.Update(id, models.UpdateDomainInput{SSLEnabled: &t, ServerID: &serverID})
		}()
		result["setup_started"] = true
	}

	h.json(w, http.StatusOK, result)
}

// APICheckSSL checks if SSL certificate exists for the domain
func (h *Handler) APICheckSSL(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	status, err := h.verification.CheckSSL(domain.Name)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update SSL status in database and assign server
	if status.Exists && status.IsWildcard {
		sslEnabled := true
		serverID := h.service.GetDeployServerID()
		h.service.Update(id, models.UpdateDomainInput{
			SSLEnabled: &sslEnabled,
			ServerID:   &serverID,
		})
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"domain": domain.Name,
		"ssl":    status,
	})
}

// APISetupDomain generates SSL and creates nginx config for the domain on the VPS
func (h *Handler) APISetupDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Generate SSL certificate first (self-signed for Cloudflare, Let's Encrypt for others)
	if err := h.verification.GenerateSSL(domain.Name); err != nil {
		h.jsonError(w, "SSL generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Then setup nginx config
	if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Mark SSL enabled and assign server
	t := true
	serverID := h.service.GetDeployServerID()
	h.service.Update(id, models.UpdateDomainInput{SSLEnabled: &t, ServerID: &serverID})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"domain":  domain.Name,
		"message": "Domain configured successfully",
	})
}

// APIGetWildcardSSLInstructions returns instructions for setting up wildcard SSL
func (h *Handler) APIGetWildcardSSLInstructions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Get email from query or use default
	email := r.URL.Query().Get("email")

	instructions, err := h.verification.GenerateWildcardSSL(domain.Name, email)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, instructions)
}

// ExternalSetup shows the setup wizard for external/cPanel domains
func (h *Handler) ExternalSetup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Get deploy server IP
	deployIP := h.service.GetDeployIP()

	// Build setup info
	baseDomain := services.GetBaseDomain(domain.Name)
	isWildcard := domain.IsWildcard || len(domain.Name) > 2 && domain.Name[:2] == "*."

	setupInfo := models.ExternalSetupInfo{
		Domain:        domain.Name,
		BaseDomain:    baseDomain,
		IsWildcard:    isWildcard,
		ServerIP:      deployIP,
		VerifyToken:   domain.VerifyToken,
		SetupStep:     domain.SetupStep,
		VerifyTXTName: "_guardbot-verify." + baseDomain,
		AcmeTXTName:   "_acme-challenge." + baseDomain,
	}

	// Check for acme-dns registration (all domains use wildcard SSL)
	if domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
		setupInfo.AcmeCnameTarget = *domain.AcmeFulldomain
		setupInfo.AcmeCnameVerified = domain.AcmeCnameVerified
		setupInfo.UseAcmeDns = true

		// Verify CNAME if not yet verified
		if !domain.AcmeCnameVerified {
			if h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain) {
				// CNAME verified - update database
				verified := true
				h.service.Update(domain.ID, models.UpdateDomainInput{AcmeCnameVerified: &verified})
				setupInfo.AcmeCnameVerified = true
			}
		}
	} else {
		// All domains need acme-dns for wildcard SSL support
		go h.registerAcmeDnsBackground(domain.ID)
	}

	// Legacy ACME token (fallback if acme-dns not available)
	if domain.AcmeToken != nil && *domain.AcmeToken != "" {
		setupInfo.AcmeToken = *domain.AcmeToken
		setupInfo.AcmeTokenReady = true
	}

	module.RenderUserSection(w, r, h.templates, "domains:external_setup.html", map[string]interface{}{
		"Title":     "Setup " + domain.Name,
		"Domain":    domain,
		"SetupInfo": setupInfo,
	})
}

// generateAcmeTokenBackground generates ACME token in background
func (h *Handler) generateAcmeTokenBackground(domainID, baseDomain string) {
	token, err := h.verification.PreGenerateAcmeToken(baseDomain)
	if err != nil || token == "" {
		return
	}

	// If cert already exists, skip to completion
	if token == "CERT_EXISTS" {
		domain, _ := h.service.Get(domainID)
		if domain != nil {
			h.completeExternalSetup(domain)
		}
		return
	}

	// Save token to database
	h.service.Update(domainID, models.UpdateDomainInput{
		AcmeToken: &token,
	})
}

// registerAcmeDnsBackground registers a domain with acme-dns in the background
func (h *Handler) registerAcmeDnsBackground(domainID string) {
	reg, err := h.service.RegisterWithAcmeDNS(domainID)
	if err != nil {
		log.Printf("[domains] acme-dns registration failed for %s: %v", domainID, err)
		return
	}
	log.Printf("[domains] acme-dns registered for %s: %s", domainID, reg.Fulldomain)
}

// APIRegisterAcmeDNS manually triggers acme-dns registration
func (h *Handler) APIRegisterAcmeDNS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Already registered?
	if domain.AcmeSubdomain != nil && *domain.AcmeSubdomain != "" {
		h.json(w, http.StatusOK, map[string]interface{}{
			"registered":  true,
			"fulldomain":  domain.AcmeFulldomain,
			"cnameTarget": domain.AcmeFulldomain,
		})
		return
	}

	// Register
	reg, err := h.service.RegisterWithAcmeDNS(id)
	if err != nil {
		h.jsonError(w, "Registration failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"registered":  true,
		"subdomain":   reg.Subdomain,
		"fulldomain":  reg.Fulldomain,
		"cnameTarget": reg.Fulldomain,
	})
}

// APICheckAcmeCname checks if the CNAME record is correctly configured
func (h *Handler) APICheckAcmeCname(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	if domain.AcmeFulldomain == nil || *domain.AcmeFulldomain == "" {
		h.jsonError(w, "Domain not registered with acme-dns", http.StatusBadRequest)
		return
	}

	verified := h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain)

	if verified && !domain.AcmeCnameVerified {
		// Update database
		v := true
		h.service.Update(id, models.UpdateDomainInput{AcmeCnameVerified: &v})
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"verified":     verified,
		"cnameTarget":  domain.AcmeFulldomain,
		"expectedName": "_acme-challenge." + services.GetBaseDomain(domain.Name),
	})
}

// APIGenerateSSLAcmeDNS generates SSL certificate using acme-dns CNAME delegation
func (h *Handler) APIGenerateSSLAcmeDNS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Check acme-dns registration
	if domain.AcmeSubdomain == nil || *domain.AcmeSubdomain == "" {
		h.jsonError(w, "Domain not registered with acme-dns", http.StatusBadRequest)
		return
	}

	// Check CNAME verification
	if !domain.AcmeCnameVerified {
		h.jsonError(w, "CNAME record not verified", http.StatusBadRequest)
		return
	}

	// Generate SSL via acme-dns
	if err := h.verification.GenerateWildcardSSLWithAcmeDNS(
		domain.Name,
		*domain.AcmeSubdomain,
		*domain.AcmeUsername,
		*domain.AcmePassword,
	); err != nil {
		h.jsonError(w, "SSL generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Setup nginx and enable SSL
	if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
		h.jsonError(w, "Nginx setup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update database and assign server
	t := true
	serverID := h.service.GetDeployServerID()
	h.service.Update(id, models.UpdateDomainInput{SSLEnabled: &t, ServerID: &serverID})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "SSL certificate generated and enabled",
	})
}

// APIGetSetupStatus returns the current DNS setup status for polling
func (h *Handler) APIGetSetupStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	deployIP := h.service.GetDeployIP()
	baseDomain := services.GetBaseDomain(domain.Name)

	status := models.SetupStatus{
		SetupStep: domain.SetupStep,
		CheckedAt: time.Now().Unix(),
	}

	// Check A record with rich status
	status.ARecordFound, status.ARecordIP = h.verification.CheckARecord(domain.Name, deployIP)
	status.ARecord = models.RecordStatus{
		Expected: deployIP,
		Found:    status.ARecordIP,
	}
	if status.ARecordFound {
		status.ARecord.Status = "verified"
		status.ARecord.Message = "A record correctly points to server"
	} else if status.ARecordIP != "" {
		status.ARecord.Status = "mismatch"
		status.ARecord.Message = fmt.Sprintf("Found %s, expected %s", status.ARecordIP, deployIP)
	} else {
		status.ARecord.Status = "not_found"
		status.ARecord.Message = "No A record found"
	}

	// Check verify TXT with rich status
	verified, verifyErr := h.verification.VerifyDNS(baseDomain, domain.VerifyToken)
	status.VerifyTXTFound = verified
	status.VerifyTXT = models.RecordStatus{
		Expected: domain.VerifyToken,
	}
	if verified {
		status.VerifyTXT.Status = "verified"
		status.VerifyTXT.Found = domain.VerifyToken
		status.VerifyTXT.Message = "Ownership verified"
	} else if verifyErr != nil {
		status.VerifyTXT.Status = "not_found"
		status.VerifyTXT.Message = fmt.Sprintf("TXT record not found at _guardbot-verify.%s", baseDomain)
		status.ErrorMessage = status.VerifyTXT.Message
	} else {
		status.VerifyTXT.Status = "not_found"
		status.VerifyTXT.Message = "Waiting for TXT record"
	}

	// Check SSL readiness - all domains use DNS-01 via acme-dns for wildcard SSL
	// Prefer acme-dns CNAME delegation (100% reliable)
	if domain.AcmeSubdomain != nil && domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
		// Using acme-dns - check CNAME instead of ACME TXT
		status.AcmeTXTFound = domain.AcmeCnameVerified
		status.CnameOrTXT = models.RecordStatus{
			Expected: *domain.AcmeFulldomain,
		}
		if !domain.AcmeCnameVerified {
			// Check if CNAME is now configured
			if h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain) {
				status.AcmeTXTFound = true
				verified := true
				h.service.Update(id, models.UpdateDomainInput{AcmeCnameVerified: &verified})
				status.CnameOrTXT.Status = "verified"
				status.CnameOrTXT.Found = *domain.AcmeFulldomain
				status.CnameOrTXT.Message = "CNAME verified - SSL renewals will be automatic"
			} else {
				status.CnameOrTXT.Status = "not_found"
				status.CnameOrTXT.Message = fmt.Sprintf("Add CNAME: _acme-challenge.%s → %s", baseDomain, *domain.AcmeFulldomain)
			}
		} else {
			status.CnameOrTXT.Status = "verified"
			status.CnameOrTXT.Found = *domain.AcmeFulldomain
			status.CnameOrTXT.Message = "CNAME verified - SSL renewals will be automatic"
		}
		// Clear old ACME token from response (not needed with acme-dns)
		status.AcmeToken = ""
	} else if domain.AcmeToken != nil && *domain.AcmeToken != "" {
		// Legacy: using ACME TXT (fallback)
		status.AcmeToken = *domain.AcmeToken
		status.AcmeTXTFound = h.verification.CheckAcmeTXT(domain.Name, *domain.AcmeToken)
		status.CnameOrTXT = models.RecordStatus{
			Expected: *domain.AcmeToken,
		}
		if status.AcmeTXTFound {
			status.CnameOrTXT.Status = "verified"
			status.CnameOrTXT.Found = *domain.AcmeToken
			status.CnameOrTXT.Message = "TXT record verified"
		} else {
			// Check for stale ACME record that needs to be deleted
			existingValue := h.verification.GetAcmeTXTValue(domain.Name)
			if existingValue != "" && existingValue != *domain.AcmeToken {
				status.AcmeTXTStale = true
				status.AcmeTXTStaleValue = existingValue
				status.CnameOrTXT.Status = "mismatch"
				status.CnameOrTXT.Found = existingValue
				status.CnameOrTXT.Message = fmt.Sprintf("Wrong value found - delete old TXT and add: %s", *domain.AcmeToken)
			} else {
				status.CnameOrTXT.Status = "not_found"
				status.CnameOrTXT.Message = "Waiting for TXT record"
			}
		}
	} else {
		// No acme-dns registration yet
		status.CnameOrTXT = models.RecordStatus{
			Status:  "not_found",
			Message: "Registering with acme-dns...",
		}
	}

	// Update verification status in DB
	if verified && !domain.DNSVerified {
		h.service.Update(id, models.UpdateDomainInput{DNSVerified: &verified})
	}

	// Check if all records found
	// For non-wildcard: just A + verify TXT
	// For wildcard: A + verify TXT + ACME TXT
	status.AllRecordsFound = status.ARecordFound && status.VerifyTXTFound && status.AcmeTXTFound

	// All domains use DNS-01 challenge via acme-dns for wildcard SSL
	// Auto-trigger SSL generation once ACME CNAME is verified
	// Also retry if stuck at ssl_generating (previous attempt may have failed)
	// Add cooldown: only retry every 5 minutes to avoid rate limiting
	sslCooldown := 5 * time.Minute
	canRetrySSL := time.Since(domain.UpdatedAt) > sslCooldown

	if status.AllRecordsFound && domain.AcmeCnameVerified && !domain.SSLEnabled &&
		(domain.SetupStep == models.SetupStepDNSWaiting || (domain.SetupStep == models.SetupStepSSLGenerating && canRetrySSL)) {
		// All domains: auto-generate wildcard SSL with DNS-01 via acme-dns
		freshDomain, _ := h.service.Get(id)
		if freshDomain != nil {
			go h.completeExternalSetup(freshDomain)
		}
		if domain.SetupStep == models.SetupStepDNSWaiting {
			step := models.SetupStepSSLGenerating
			h.service.Update(id, models.UpdateDomainInput{SetupStep: &step})
			status.SetupStep = models.SetupStepSSLGenerating
		}
	}

	// Check if SSL is ready
	if domain.SSLEnabled {
		status.SSLReady = true
		status.SetupStep = models.SetupStepComplete
	}

	// Include SSL error if present
	// But suppress old ACME TXT errors when using acme-dns (they're no longer relevant)
	if domain.SSLError != nil && *domain.SSLError != "" {
		// When using acme-dns, clear old errors mentioning ACME TXT
		if domain.AcmeSubdomain != nil && strings.Contains(*domain.SSLError, "_acme-challenge") {
			// Clear the stale error from database
			emptyErr := ""
			h.service.Update(id, models.UpdateDomainInput{SSLError: &emptyErr})
		} else {
			status.ErrorMessage = *domain.SSLError
		}
	}

	h.json(w, http.StatusOK, status)
}

// completeExternalSetup finishes SSL setup for external domain
func (h *Handler) completeExternalSetup(domain *models.Domain) {
	log.Printf("[domains] completing SSL setup for %s (wildcard=%v, acmeDns=%v)", domain.Name, domain.IsWildcard, domain.AcmeSubdomain != nil)

	// Clear any previous error
	emptyErr := ""
	h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &emptyErr})

	// All domains use wildcard SSL via acme-dns for subdomain support (cPanel-style)
	var sslErr error
	if domain.AcmeSubdomain != nil && *domain.AcmeSubdomain != "" &&
		domain.AcmeUsername != nil && *domain.AcmeUsername != "" &&
		domain.AcmePassword != nil && *domain.AcmePassword != "" &&
		domain.AcmeCnameVerified {
		// Use acme-dns - automatic SSL renewals will work
		log.Printf("[domains] using acme-dns for %s", domain.Name)
		sslErr = h.verification.GenerateWildcardSSLWithAcmeDNS(
			domain.Name,
			*domain.AcmeSubdomain,
			*domain.AcmeUsername,
			*domain.AcmePassword,
		)
	} else if domain.AcmeSubdomain != nil && *domain.AcmeSubdomain != "" && !domain.AcmeCnameVerified {
		// acme-dns registered but CNAME not verified - cannot proceed
		log.Printf("[domains] %s has acme-dns but CNAME not verified, cannot generate SSL", domain.Name)
		errMsg := "CNAME delegation to acme-dns not verified. Add CNAME record and verify first."
		h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &errMsg})
		return
	} else {
		// No acme-dns registration - this shouldn't happen for new domains
		log.Printf("[domains] %s missing acme-dns registration, cannot generate wildcard SSL", domain.Name)
		errMsg := "Domain not registered with acme-dns. Please re-add the domain."
		h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &errMsg})
		return
	}

	if sslErr != nil {
		log.Printf("[domains] SSL generation failed for %s: %v", domain.Name, sslErr)
		errMsg := sslErr.Error()
		h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &errMsg})
		return
	}

	// Verify SSL certificate actually exists on VPS before marking enabled
	sslStatus, err := h.verification.CheckSSL(domain.Name)
	if err != nil || !sslStatus.Exists {
		log.Printf("[domains] SSL verification failed for %s: cert not found on VPS", domain.Name)
		errMsg := "SSL certificate not found on server after generation"
		h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &errMsg})
		return
	}

	// Setup nginx
	if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
		log.Printf("[domains] nginx setup failed for %s: %v", domain.Name, err)
		errMsg := "nginx configuration failed: " + err.Error()
		h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &errMsg})
		return
	}

	// Get deploy server ID and assign to domain
	serverID := h.service.GetDeployServerID()

	// Mark as complete with server assignment
	step := models.SetupStepComplete
	sslEnabled := true
	dnsVerified := true
	h.service.Update(domain.ID, models.UpdateDomainInput{
		SetupStep:   &step,
		SSLEnabled:  &sslEnabled,
		DNSVerified: &dnsVerified,
		ServerID:    &serverID,
	})

	log.Printf("[domains] SSL setup complete for %s", domain.Name)
}

// APIRefreshAcmeToken generates a new ACME token (if expired)
func (h *Handler) APIRefreshAcmeToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	baseDomain := services.GetBaseDomain(domain.Name)
	token, err := h.verification.PreGenerateAcmeToken(baseDomain)
	if err != nil {
		h.jsonError(w, "Failed to generate token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Save new token
	h.service.Update(id, models.UpdateDomainInput{AcmeToken: &token})

	h.json(w, http.StatusOK, map[string]interface{}{
		"token":   token,
		"txtName": "_acme-challenge." + baseDomain,
	})
}

// APIRetrySSL forces a retry of SSL generation (with rate limiting)
func (h *Handler) APIRetrySSL(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Prevent spam - if already generating and updated within last 2 minutes, reject
	if domain.SetupStep == models.SetupStepSSLGenerating {
		if time.Since(domain.UpdatedAt) < 2*time.Minute {
			h.jsonError(w, "SSL generation already in progress - please wait 2 minutes before retrying", http.StatusTooManyRequests)
			return
		}
	}

	// All domains use DNS-01 via acme-dns
	// Cancel any existing lego challenge
	h.verification.LegoCancelChallenge(domain.Name)

	// Clear old token and reset step
	emptyToken := ""
	step := models.SetupStepDNSWaiting
	h.service.Update(id, models.UpdateDomainInput{
		AcmeToken: &emptyToken,
		SetupStep: &step,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Challenge reset. Verify CNAME record and SSL will generate automatically.",
	})
}

// APIStartSSLChallenge starts the ACME DNS-01 challenge for wildcard SSL (Phase 1)
// Returns the TXT record value that user needs to add to their DNS
func (h *Handler) APIStartSSLChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Check if there's already a pending challenge
	existingToken, _ := h.verification.LegoGetPendingToken(domain.Name)
	if existingToken != "" {
		baseDomain := services.GetBaseDomain(domain.Name)
		h.json(w, http.StatusOK, map[string]interface{}{
			"status":     "pending",
			"token":      existingToken,
			"txt_record": "_acme-challenge." + baseDomain,
			"message":    "Challenge already in progress. Add the TXT record and click 'Complete SSL'.",
		})
		return
	}

	// Start new challenge
	challenge, err := h.verification.LegoStartChallenge(domain.Name, "")
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If cert already exists, auto-complete
	if challenge.Token == "CERT_EXISTS" {
		// Setup nginx
		if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
			h.jsonError(w, "Certificate exists but nginx setup failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Mark as complete
		stepComplete := models.SetupStepComplete
		sslEnabled := true
		dnsVerified := true
		serverID := h.service.GetDeployServerID()
		h.service.Update(id, models.UpdateDomainInput{
			SetupStep:   &stepComplete,
			SSLEnabled:  &sslEnabled,
			DNSVerified: &dnsVerified,
			ServerID:    &serverID,
		})

		h.json(w, http.StatusOK, map[string]interface{}{
			"status":  "complete",
			"message": "SSL certificate already exists and is now active!",
		})
		return
	}

	// Save token to database
	h.service.Update(id, models.UpdateDomainInput{AcmeToken: &challenge.Token})

	// Update step
	step := models.SetupStepSSLWaiting
	h.service.Update(id, models.UpdateDomainInput{SetupStep: &step})

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":     "started",
		"token":      challenge.Token,
		"txt_record": challenge.TXTRecord,
		"message":    "Add this TXT record to your DNS, then click 'Complete SSL'.",
	})
}

// APICompleteSSLChallenge completes the ACME challenge after DNS is configured (Phase 2)
func (h *Handler) APICompleteSSLChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Check if there's a pending challenge
	token, _ := h.verification.LegoGetPendingToken(domain.Name)
	if token == "" {
		h.jsonError(w, "No pending SSL challenge. Click 'Get SSL Token' first.", http.StatusBadRequest)
		return
	}

	// Verify DNS is set before completing
	baseDomain := services.GetBaseDomain(domain.Name)
	if !h.verification.CheckAcmeTXT(domain.Name, token) {
		h.json(w, http.StatusOK, map[string]interface{}{
			"status":     "dns_not_ready",
			"token":      token,
			"txt_record": "_acme-challenge." + baseDomain,
			"message":    "TXT record not found yet. Please add it and wait for DNS propagation.",
		})
		return
	}

	// Update step
	step := models.SetupStepSSLGenerating
	h.service.Update(id, models.UpdateDomainInput{SetupStep: &step})

	// Complete the challenge
	err = h.verification.LegoCompleteChallenge(domain.Name)
	if err != nil {
		errMsg := err.Error()
		h.service.Update(id, models.UpdateDomainInput{SSLError: &errMsg})
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Setup nginx
	if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
		h.jsonError(w, "Certificate installed but nginx setup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Mark as complete
	stepComplete := models.SetupStepComplete
	sslEnabled := true
	dnsVerified := true
	serverID := h.service.GetDeployServerID()
	h.service.Update(id, models.UpdateDomainInput{
		SetupStep:   &stepComplete,
		SSLEnabled:  &sslEnabled,
		DNSVerified: &dnsVerified,
		ServerID:    &serverID,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":  "complete",
		"message": "SSL certificate installed successfully!",
	})
}

// APIGetSSLToken returns the pending ACME token for a domain
func (h *Handler) APIGetSSLToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	token, _ := h.verification.LegoGetPendingToken(domain.Name)
	baseDomain := services.GetBaseDomain(domain.Name)

	if token == "" {
		h.json(w, http.StatusOK, map[string]interface{}{
			"status":  "none",
			"message": "No pending SSL challenge",
		})
		return
	}

	// Check if DNS is ready
	dnsReady := h.verification.CheckAcmeTXT(domain.Name, token)

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":     "pending",
		"token":      token,
		"txt_record": "_acme-challenge." + baseDomain,
		"dns_ready":  dnsReady,
	})
}

// APICancelSSLChallenge cancels a pending SSL challenge
func (h *Handler) APICancelSSLChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	if err := h.verification.LegoCancelChallenge(domain.Name); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear token from database
	emptyToken := ""
	step := models.SetupStepDNSWaiting
	h.service.Update(id, models.UpdateDomainInput{
		AcmeToken: &emptyToken,
		SetupStep: &step,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":  "cancelled",
		"message": "SSL challenge cancelled",
	})
}

// Helpers

// APIDeleteCpanelConnection deletes a cPanel connection and all associated data
// This includes: domains, redirect links, analytics, VPS files, DNS records
func (h *Handler) APIDeleteCpanelConnection(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	connID := chi.URLParam(r, "id")

	// Get all domains using this connection
	var domains []models.Domain
	h.service.DB().Select(&domains, `
		SELECT * FROM domains WHERE cpanel_connection_id = $1 AND user_id = $2
	`, connID, userID)

	log.Printf("[cpanel] Deleting connection %s with %d domains", connID, len(domains))

	// Clean up each domain (VPS files, DNS, redirect links)
	go func() {
		for _, domain := range domains {
			log.Printf("[cpanel] Cleaning up domain %s", domain.Name)

			// Clean up VPS files
			if cleanupErr := h.verification.CleanupDomain(domain.Name); cleanupErr != nil {
				log.Printf("[cpanel] VPS cleanup failed for %s: %v", domain.Name, cleanupErr)
			}

			// Clean up redirect link botection settings
			if linkIDs, err := h.service.GetRedirectLinkIDs(domain.ID); err == nil && len(linkIDs) > 0 {
				h.verification.CleanupRedirectLinkSettings(linkIDs)
			}
		}

		// Clean up DNS records via cPanel
		if len(domains) > 0 && h.cpanelService != nil {
			for _, domain := range domains {
				h.cpanelService.CleanupDomainDNS(connID, domain.Name)
			}
		}
	}()

	// Delete domains (cascade deletes redirect_links and analytics)
	for _, domain := range domains {
		h.service.Delete(domain.ID)
	}

	// Delete the cPanel connection itself
	if err := h.cpanelService.DeleteConnection(userID, connID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"deleted":         true,
		"domains_deleted": len(domains),
	})
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

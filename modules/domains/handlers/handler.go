package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service      *services.DomainService
	verification *services.VerificationService
	templates    *module.TemplateEngine
}

func NewHandler(service *services.DomainService, templates *module.TemplateEngine) *Handler {
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
		service:      service,
		verification: vs,
		templates:    templates,
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

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Get deploy server IP for DNS instructions
	deployIP := h.service.GetDeployIP()

	module.RenderUserSection(w, r, h.templates, "domains:show.html", map[string]interface{}{
		"Title":    domain.Name,
		"Domain":   domain,
		"ServerIP": deployIP,
	})
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

	// Detect if this domain needs external setup wizard
	// Wildcard domains always need DNS-01 challenge (external setup)
	needsSetupWizard := false
	if domain.IsWildcard {
		needsSetupWizard = true
		// Update domain to mark as external setup pending
		setupType := models.SetupTypeExternal
		setupStep := models.SetupStepDNSWaiting
		h.service.Update(domain.ID, models.UpdateDomainInput{
			SetupType: &setupType,
			SetupStep: &setupStep,
		})
		// Start generating ACME token in background
		go h.generateAcmeTokenBackground(domain.ID, services.GetBaseDomain(domain.Name))
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"domain":            domain,
		"needsSetupWizard": needsSetupWizard,
	})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
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

	var input models.UpdateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	domain, err := h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"domain": domain})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get domain first to have the name for cleanup
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
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

	// Cleanup VPS: nginx config, SSL certs, site directories
	if cleanupErr := h.verification.CleanupDomain(domain.Name); cleanupErr != nil {
		// Log but don't fail - VPS might be unreachable
		// We still want to remove from our database
	}

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
			// Mark SSL enabled
			t := true
			h.service.Update(id, models.UpdateDomainInput{SSLEnabled: &t})
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

	status, err := h.verification.CheckSSL(domain.Name)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update SSL status in database
	if status.Exists && status.IsWildcard {
		sslEnabled := true
		h.service.Update(id, models.UpdateDomainInput{
			SSLEnabled: &sslEnabled,
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

	// Mark SSL enabled
	t := true
	h.service.Update(id, models.UpdateDomainInput{SSLEnabled: &t})

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

	// Get ACME token if available
	if domain.AcmeToken != nil && *domain.AcmeToken != "" {
		setupInfo.AcmeToken = *domain.AcmeToken
		setupInfo.AcmeTokenReady = true
	} else if isWildcard || domain.SetupType == models.SetupTypeExternal {
		// Try to generate ACME token if not ready
		go h.generateAcmeTokenBackground(domain.ID, baseDomain)
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

// APIGetSetupStatus returns the current DNS setup status for polling
func (h *Handler) APIGetSetupStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	deployIP := h.service.GetDeployIP()
	baseDomain := services.GetBaseDomain(domain.Name)

	status := models.SetupStatus{
		SetupStep: domain.SetupStep,
	}

	// Check A record
	status.ARecordFound, status.ARecordIP = h.verification.CheckARecord(domain.Name, deployIP)

	// Check verify TXT
	verified, verifyErr := h.verification.VerifyDNS(baseDomain, domain.VerifyToken)
	status.VerifyTXTFound = verified
	if verifyErr != nil {
		status.ErrorMessage = fmt.Sprintf("Verify check: %s (looking for %s at _guardbot-verify.%s)", verifyErr.Error(), domain.VerifyToken, baseDomain)
	}

	// Check ACME TXT if we have a token
	if domain.AcmeToken != nil && *domain.AcmeToken != "" {
		status.AcmeToken = *domain.AcmeToken
		status.AcmeTXTFound = h.verification.CheckAcmeTXT(domain.Name, *domain.AcmeToken)

		// Check for stale ACME record that needs to be deleted
		if !status.AcmeTXTFound {
			existingValue := h.verification.GetAcmeTXTValue(domain.Name)
			if existingValue != "" && existingValue != *domain.AcmeToken {
				status.AcmeTXTStale = true
				status.AcmeTXTStaleValue = existingValue
			}
		}
	}

	// Update verification status in DB
	if verified && !domain.DNSVerified {
		h.service.Update(id, models.UpdateDomainInput{DNSVerified: &verified})
	}

	// Check if all records found
	status.AllRecordsFound = status.ARecordFound && status.VerifyTXTFound && status.AcmeTXTFound

	// If all records found and not yet complete, trigger SSL generation
	if status.AllRecordsFound && domain.SetupStep == models.SetupStepDNSWaiting {
		go h.completeExternalSetup(domain)
		step := models.SetupStepSSLGenerating
		h.service.Update(id, models.UpdateDomainInput{SetupStep: &step})
		status.SetupStep = models.SetupStepSSLGenerating
	}

	// Check if SSL is ready
	if domain.SSLEnabled {
		status.SSLReady = true
		status.SetupStep = models.SetupStepComplete
	}

	h.json(w, http.StatusOK, status)
}

// completeExternalSetup finishes SSL setup for external domain
func (h *Handler) completeExternalSetup(domain *models.Domain) {
	// Complete the wildcard SSL generation
	if err := h.verification.CompleteWildcardSSL(domain.Name); err != nil {
		return
	}

	// Setup nginx
	if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
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
}

// APIRefreshAcmeToken generates a new ACME token (if expired)
func (h *Handler) APIRefreshAcmeToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
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

// Helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

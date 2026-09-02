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

	// Check for acme-dns registration (preferred method for wildcard)
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
	} else if isWildcard {
		// Register with acme-dns if not yet registered
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

	// Check SSL readiness for wildcard domains
	if domain.IsWildcard {
		// Prefer acme-dns CNAME delegation (100% reliable)
		if domain.AcmeSubdomain != nil && domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
			// Using acme-dns - check CNAME instead of ACME TXT
			status.AcmeTXTFound = domain.AcmeCnameVerified
			if !domain.AcmeCnameVerified {
				// Check if CNAME is now configured
				if h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain) {
					status.AcmeTXTFound = true
					verified := true
					h.service.Update(id, models.UpdateDomainInput{AcmeCnameVerified: &verified})
				}
			}
			// Clear old ACME token from response (not needed with acme-dns)
			status.AcmeToken = ""
		} else if domain.AcmeToken != nil && *domain.AcmeToken != "" {
			// Legacy: using ACME TXT (fallback)
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
	} else {
		// Non-wildcard: ACME TXT not needed (HTTP-01 challenge)
		status.AcmeTXTFound = true
	}

	// Update verification status in DB
	if verified && !domain.DNSVerified {
		h.service.Update(id, models.UpdateDomainInput{DNSVerified: &verified})
	}

	// Check if all records found
	// For non-wildcard: just A + verify TXT
	// For wildcard: A + verify TXT + ACME TXT
	status.AllRecordsFound = status.ARecordFound && status.VerifyTXTFound && status.AcmeTXTFound

	// If all records found and not yet complete, trigger SSL generation
	// Also retry if stuck at ssl_generating (previous attempt may have failed)
	// Add cooldown: only retry every 5 minutes to avoid rate limiting
	sslCooldown := 5 * time.Minute
	canRetrySSL := time.Since(domain.UpdatedAt) > sslCooldown

	if status.AllRecordsFound && !domain.SSLEnabled &&
		(domain.SetupStep == models.SetupStepDNSWaiting || (domain.SetupStep == models.SetupStepSSLGenerating && canRetrySSL)) {
		// Refresh domain from DB to get updated CNAME verification status
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

	var sslErr error
	if domain.IsWildcard {
		// Wildcard domains need DNS-01 challenge
		// Prefer acme-dns if available (100% reliable)
		if domain.AcmeSubdomain != nil && domain.AcmePassword != nil && domain.AcmeCnameVerified {
			log.Printf("[domains] using acme-dns for %s", domain.Name)
			sslErr = h.verification.GenerateWildcardSSLWithAcmeDNS(domain.Name, *domain.AcmeSubdomain, *domain.AcmePassword)
		} else {
			// Fallback to old method (less reliable)
			log.Printf("[domains] using legacy DNS-01 for %s (acme-dns not configured)", domain.Name)
			acmeToken := ""
			if domain.AcmeToken != nil {
				acmeToken = *domain.AcmeToken
			}
			sslErr = h.verification.CompleteWildcardSSL(domain.Name, acmeToken)
		}
	} else {
		// Non-wildcard domains use HTTP-01 challenge (simpler, no ACME TXT needed)
		sslErr = h.verification.GenerateHTTPSSL(domain.Name)
	}

	if sslErr != nil {
		log.Printf("[domains] SSL generation failed for %s: %v", domain.Name, sslErr)
		errMsg := sslErr.Error()
		h.service.Update(domain.ID, models.UpdateDomainInput{SSLError: &errMsg})
		return
	}

	// Setup nginx
	if err := h.verification.SetupDomainNginx(domain.Name); err != nil {
		log.Printf("[domains] nginx setup failed for %s: %v", domain.Name, err)
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

	// Prevent spam - if already generating and updated within last 2 minutes, reject
	if domain.SetupStep == models.SetupStepSSLGenerating {
		if time.Since(domain.UpdatedAt) < 2*time.Minute {
			h.jsonError(w, "SSL generation already in progress - please wait 2 minutes before retrying", http.StatusTooManyRequests)
			return
		}
	}

	// Check DNS records
	deployIP := h.service.GetDeployIP()
	baseDomain := services.GetBaseDomain(domain.Name)

	aRecordFound, _ := h.verification.CheckARecord(domain.Name, deployIP)
	verifyFound, _ := h.verification.VerifyDNS(baseDomain, domain.VerifyToken)

	// ACME TXT only needed for wildcard domains (non-wildcard uses HTTP-01)
	acmeFound := true
	if domain.IsWildcard {
		acmeFound = false
		if domain.AcmeToken != nil && *domain.AcmeToken != "" {
			acmeFound = h.verification.CheckAcmeTXT(domain.Name, *domain.AcmeToken)
		}
	}

	missing := []string{}
	if !aRecordFound {
		missing = append(missing, "A record")
	}
	if !verifyFound {
		missing = append(missing, "verify TXT")
	}
	if domain.IsWildcard && !acmeFound {
		missing = append(missing, "ACME TXT")
	}

	if len(missing) > 0 {
		h.jsonError(w, "Missing DNS records: "+strings.Join(missing, ", "), http.StatusBadRequest)
		return
	}

	// Update step to ssl_generating (this also updates updated_at)
	step := models.SetupStepSSLGenerating
	h.service.Update(id, models.UpdateDomainInput{SetupStep: &step})

	// Trigger SSL generation in background
	go h.completeExternalSetup(domain)

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "SSL generation started - this may take up to 60 seconds",
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

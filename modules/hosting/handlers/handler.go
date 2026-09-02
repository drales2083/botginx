package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/modules/hosting/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for hosting operations
type Handler struct {
	service      *services.HostingService
	billing      *services.BillingService
	provisioning *services.ProvisioningService
	templates    *module.TemplateEngine
}

// NewHandler creates a new hosting handler instance
func NewHandler(service *services.HostingService, billing *services.BillingService, provisioning *services.ProvisioningService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:      service,
		billing:      billing,
		provisioning: provisioning,
		templates:    templates,
	}
}

// requireAccountOwner verifies the current user owns the account, returns the account or writes an error
func (h *Handler) requireAccountOwner(w http.ResponseWriter, r *http.Request) (*models.HostingAccount, bool) {
	accountID := chi.URLParam(r, "accountID")
	userID := ctx.GetUserID(r)

	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return nil, false
	}

	if account.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}

	return account, true
}

// requireAccountOwnerJSON is like requireAccountOwner but returns JSON errors for API endpoints
func (h *Handler) requireAccountOwnerJSON(w http.ResponseWriter, r *http.Request) (*models.HostingAccount, bool) {
	accountID := chi.URLParam(r, "accountID")
	userID := ctx.GetUserID(r)

	account, err := h.service.GetAccount(accountID)
	if err != nil {
		h.jsonError(w, "Account not found", http.StatusNotFound)
		return nil, false
	}

	if account.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}

	return account, true
}

// ========== User Page Handlers ==========

// UserIndex lists the user's hosting accounts
func (h *Handler) UserIndex(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accounts, _ := h.service.ListAccountsByUser(userID)
	balance := h.service.GetUserBalance(userID)

	module.RenderUserSection(w, r, h.templates, "hosting:index.html", map[string]interface{}{
		"Title":    "Hosting",
		"Accounts": accounts,
		"Balance":  balance,
	})
}

// UserPurchase shows the purchase form with available packages
func (h *Handler) UserPurchase(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	packages, _ := h.service.ListActivePackages()
	balance := h.service.GetUserBalance(userID)

	module.RenderUserSection(w, r, h.templates, "hosting:buy.html", map[string]interface{}{
		"Title":    "Purchase Hosting",
		"Packages": packages,
		"Balance":  balance,
	})
}

// UserDoPurchase handles the purchase POST request
func (h *Handler) UserDoPurchase(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		PackageID string `json:"packageId"`
		Domain    string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Domain == "" {
		h.jsonError(w, "Domain name is required", http.StatusBadRequest)
		return
	}

	account, err := h.service.PurchaseHosting(userID, input.PackageID, input.Domain)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Trigger automatic provisioning in background
	h.provisioning.ProvisionAccountAsync(account.ID)

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"account": account,
		"message": "Your hosting account is being set up automatically. This usually takes less than a minute.",
	})
}

// UserOverview shows the account overview with control panel access
func (h *Handler) UserOverview(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwner(w, r)
	if !ok {
		return
	}

	// Get provisioning error if any
	_, _, provisioningError, _ := h.provisioning.GetProvisioningStatus(account.ID)

	// Fix: If account has a server assigned, it should be active (not pending)
	// This handles cases where status update failed during provisioning
	statusString := string(account.Status)
	if account.ServerID != nil && account.Status == models.AccountStatusPending {
		statusString = string(models.AccountStatusActive)
		// Also fix the database
		go h.service.FixAccountStatus(account.ID, models.AccountStatusActive)
	}

	module.RenderUserSection(w, r, h.templates, "hosting:overview.html", map[string]interface{}{
		"Title":             account.PackageName + " Hosting",
		"Account":           account,
		"StatusString":      statusString,
		"ProvisioningError": provisioningError,
	})
}

// UserDomains lists domains for antibot management
func (h *Handler) UserDomains(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwner(w, r)
	if !ok {
		return
	}

	domains, _ := h.service.ListDomains(account.ID)

	module.RenderUserSection(w, r, h.templates, "hosting:domains.html", map[string]interface{}{
		"Title":   "Domain Protection",
		"Account": account,
		"Domains": domains,
	})
}

// UserDomainSettings shows antibot settings for a specific domain
func (h *Handler) UserDomainSettings(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwner(w, r)
	if !ok {
		return
	}

	domainID := chi.URLParam(r, "domainID")
	settings, err := h.service.GetDomainSettings(domainID)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "hosting:domain_settings.html", map[string]interface{}{
		"Title":    "Protection Settings",
		"Account":  account,
		"Settings": settings,
		"DomainID": domainID,
	})
}

// ========== Admin Page Handlers ==========

// AdminServers shows the server management page
func (h *Handler) AdminServers(w http.ResponseWriter, r *http.Request) {
	servers, _ := h.service.ListServers()

	module.Render(w, r, h.templates, "hosting:admin_servers.html", map[string]interface{}{
		"Title":   "Hosting Servers",
		"Servers": servers,
	})
}

// AdminPackages shows the package management page
func (h *Handler) AdminPackages(w http.ResponseWriter, r *http.Request) {
	packages, _ := h.service.ListPackages()

	module.Render(w, r, h.templates, "hosting:admin_packages.html", map[string]interface{}{
		"Title":    "Hosting Packages",
		"Packages": packages,
	})
}

// AdminAccounts shows all hosting accounts
func (h *Handler) AdminAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.service.ListAllAccounts()
	servers, _ := h.service.ListServers()

	module.Render(w, r, h.templates, "hosting:admin_accounts.html", map[string]interface{}{
		"Title":    "Hosting Accounts",
		"Accounts": accounts,
		"Servers":  servers,
	})
}

// ========== User API Handlers ==========

// APIGetProvisioningStatus returns the current provisioning status for an account
func (h *Handler) APIGetProvisioningStatus(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	status, serverIP, provisioningError, err := h.provisioning.GetProvisioningStatus(account.ID)
	if err != nil {
		h.jsonError(w, "Failed to get status", http.StatusInternalServerError)
		return
	}

	// Get domain for this account
	domains, _ := h.service.ListDomains(account.ID)
	var domain string
	if len(domains) > 0 {
		domain = domains[0].Domain
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"status":   status,
		"serverIP": serverIP,
		"domain":   domain,
		"ready":    status == string(models.AccountStatusActive),
		"error":    provisioningError,
	})
}

// APIRetryProvisioning retries auto-provisioning for a failed account
func (h *Handler) APIRetryProvisioning(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	// Only allow retry for pending accounts
	if account.Status != models.AccountStatusPending {
		h.jsonError(w, "Account is not in pending state", http.StatusBadRequest)
		return
	}

	// Clear previous error and retry
	h.service.ClearProvisioningError(account.ID)
	h.provisioning.ProvisionAccountAsync(account.ID)

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Retrying provisioning...",
	})
}

// APIUpdateDomainSettings updates antibot settings for a domain
func (h *Handler) APIUpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	domainID := chi.URLParam(r, "domainID")
	var input models.UpdateDomainSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify domain belongs to account
	domains, _ := h.service.ListDomains(account.ID)
	found := false
	for _, d := range domains {
		if d.ID == domainID {
			found = true
			break
		}
	}
	if !found {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	if err := h.service.UpdateDomainSettings(domainID, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIReactivate handles account reactivation request
func (h *Handler) APIReactivate(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	if account.Status != models.AccountStatusSuspended {
		h.jsonError(w, "Account is not suspended", http.StatusBadRequest)
		return
	}

	// Get package price
	pkg, err := h.service.GetPackage(*account.PackageID)
	if err != nil {
		h.jsonError(w, "Package not found", http.StatusInternalServerError)
		return
	}

	price := pkg.PriceMonthly
	if account.CustomPrice != nil {
		price = *account.CustomPrice
	}

	// Check balance
	balance := h.service.GetUserBalance(account.UserID)
	if balance < price {
		h.jsonError(w, "Insufficient balance", http.StatusPaymentRequired)
		return
	}

	// Unsuspend
	if err := h.service.UnsuspendAccount(account.ID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Charge
	h.service.DeductBalance(account.UserID, price, "Hosting reactivation: "+pkg.Name)

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIGetPanelCredentials returns panel login credentials for the user
func (h *Handler) APIGetPanelCredentials(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	if account.Status == models.AccountStatusPending {
		h.jsonError(w, "Account is not yet set up", http.StatusBadRequest)
		return
	}

	username, password, err := h.service.GetPanelCredentials(account.ID)
	if err != nil {
		h.jsonError(w, "Unable to retrieve credentials", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"username": username,
		"password": password,
		"panelUrl": account.PanelURL,
	})
}

// ========== Admin API Handlers ==========

// APICreateServer creates a new hosting server
func (h *Handler) APICreateServer(w http.ResponseWriter, r *http.Request) {
	var input models.CreateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	server, err := h.service.CreateServer(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"success": true, "server": server})
}

// APIUpdateServer updates an existing hosting server
func (h *Handler) APIUpdateServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.UpdateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateServer(id, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIToggleServer enables or disables a server
func (h *Handler) APIToggleServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.ToggleServer(id, input.Active); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIDeleteServer removes a server
func (h *Handler) APIDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteServer(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APICreatePackage creates a new hosting package
func (h *Handler) APICreatePackage(w http.ResponseWriter, r *http.Request) {
	var input models.CreatePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	pkg, err := h.service.CreatePackage(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"success": true, "package": pkg})
}

// APIUpdatePackage updates an existing package
func (h *Handler) APIUpdatePackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input models.CreatePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdatePackage(id, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APITogglePackage enables or disables a package
func (h *Handler) APITogglePackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.TogglePackage(id, input.Active); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APISuspendAccount suspends a hosting account
func (h *Handler) APISuspendAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.SuspendAccount(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIUnsuspendAccount unsuspends a hosting account
func (h *Handler) APIUnsuspendAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.UnsuspendAccount(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIDeleteAccount deletes a hosting account and its domains (admin action)
func (h *Handler) APIDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteAccount(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APILinkAccount links a pending account to a server and activates it (admin action)
func (h *Handler) APILinkAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.LinkAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.LinkAccount(id, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIGetAccountCredentials returns credentials for admin to create panel account
func (h *Handler) APIGetAccountCredentials(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	account, password, err := h.service.GetAccountWithCredentials(id)
	if err != nil {
		h.jsonError(w, "Account not found", http.StatusNotFound)
		return
	}

	// Get domain for this account
	domains, _ := h.service.ListDomains(id)
	domain := ""
	if len(domains) > 0 {
		domain = domains[0].Domain
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"username": account.PanelUsername,
		"password": password,
		"domain":   domain,
	})
}

// APIAddDomain adds a domain to an account (admin action)
func (h *Handler) APIAddDomain(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")

	var input models.AddDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	domain, err := h.service.AddDomain(accountID, input.Domain)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"success": true, "domain": domain})
}

// APIDeleteDomain removes a domain from an account (admin action)
func (h *Handler) APIDeleteDomain(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.DeleteDomain(accountID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APITopUpBalance adds funds to a user's balance
func (h *Handler) APITopUpBalance(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserEmail   string  `json:"user_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Amount <= 0 {
		h.jsonError(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	// Look up user by email
	userID, err := h.service.GetUserIDByEmail(input.UserEmail)
	if err != nil {
		h.jsonError(w, "User not found", http.StatusNotFound)
		return
	}

	desc := input.Description
	if desc == "" {
		desc = "Admin top-up"
	}

	if err := h.service.TopUpBalance(userID, input.Amount, desc); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// RunBilling runs the billing cron job
func (h *Handler) RunBilling(w http.ResponseWriter, r *http.Request) {
	h.billing.ProcessMonthlyBilling()
	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ========== Analytics API (for Antibot Dashboard) ==========

// APIAnalyticsSummary returns summary stats for the antibot dashboard
func (h *Handler) APIAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	summary := h.service.GetAnalyticsSummary(account.ID)
	h.json(w, http.StatusOK, summary)
}

// APIAnalytics returns detailed analytics for the antibot dashboard
func (h *Handler) APIAnalytics(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwnerJSON(w, r)
	if !ok {
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	analytics := h.service.GetAnalytics(account.ID, period)
	h.json(w, http.StatusOK, analytics)
}

// APIGetDomainStatus returns DNS/SSL status for a domain
func (h *Handler) APIGetDomainStatus(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwner(w, r)
	if !ok {
		return
	}

	domainID := chi.URLParam(r, "domainID")
	domain, err := h.service.GetDomain(domainID)
	if err != nil || domain.AccountID != account.ID {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Get server IP for DNS check
	serverIP := account.ServerHostname

	// Check DNS by resolving the domain
	ips, err := net.LookupHost(domain.Domain)
	var currentIP string
	dnsResolved := false
	if err == nil && len(ips) > 0 {
		currentIP = ips[0]
		dnsResolved = true
	}

	directMatch := dnsResolved && currentIP == serverIP

	// HTTP verification for proxied domains
	httpReachable := false
	if dnsResolved {
		httpReachable = h.checkHTTPReachable(domain.Domain)
	}

	dnsOK := directMatch || httpReachable

	h.json(w, http.StatusOK, map[string]interface{}{
		"domain":      domain.Domain,
		"dnsVerified": domain.DNSVerified,
		"dnsOK":       dnsOK,
		"directMatch": directMatch,
		"httpOK":      httpReachable,
		"isProxied":   dnsResolved && !directMatch && httpReachable,
		"currentIP":   currentIP,
		"expectedIP":  serverIP,
		"sslEnabled":  domain.SSLEnabled,
		"setupStatus": domain.SetupStatus,
		"sslError":    domain.SSLError,
	})
}

// APICheckDNS checks DNS for a domain and updates status
func (h *Handler) APICheckDNS(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwner(w, r)
	if !ok {
		return
	}

	domainID := chi.URLParam(r, "domainID")
	domain, err := h.service.GetDomain(domainID)
	if err != nil || domain.AccountID != account.ID {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	serverIP := account.ServerHostname

	// Check DNS by resolving the domain
	ips, err := net.LookupHost(domain.Domain)
	var currentIP string
	dnsResolved := false
	if err == nil && len(ips) > 0 {
		currentIP = ips[0]
		dnsResolved = true
	}

	// DNS is "OK" if it resolves AND matches our server
	// For proxied domains (Cloudflare, etc.), IP won't match but HTTP verification will work
	directMatch := dnsResolved && currentIP == serverIP

	// HTTP verification: try to reach the domain and see if it connects
	// This works for both direct and proxied domains
	httpReachable := false
	if dnsResolved {
		httpReachable = h.checkHTTPReachable(domain.Domain)
	}

	// DNS is considered OK if either:
	// 1. IP directly matches our server, OR
	// 2. Domain is reachable via HTTP (works for CDN/proxy)
	dnsOK := directMatch || httpReachable

	// Update domain status
	if dnsOK && !domain.DNSVerified {
		h.service.UpdateDomainStatus(domainID, true, models.DomainStatusPendingDNS, nil)
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"dnsOK":       dnsOK,
		"directMatch": directMatch,
		"httpOK":      httpReachable,
		"currentIP":   currentIP,
		"expectedIP":  serverIP,
		"isProxied":   dnsResolved && !directMatch && httpReachable,
	})
}

// checkHTTPReachable checks if a domain is reachable via HTTP and returns a valid response
// This works regardless of whether the domain is behind a CDN/proxy
func (h *Handler) checkHTTPReachable(domain string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Try HTTP first (most sites will redirect to HTTPS but that's fine)
	resp, err := client.Get("http://" + domain)
	if err != nil {
		// Try HTTPS if HTTP fails
		resp, err = client.Get("https://" + domain)
		if err != nil {
			return false
		}
	}
	defer resp.Body.Close()

	// Any valid HTTP response (even 404) means the domain is reachable
	return resp.StatusCode > 0
}

// APIEnableSSL enables SSL for a domain
func (h *Handler) APIEnableSSL(w http.ResponseWriter, r *http.Request) {
	account, ok := h.requireAccountOwner(w, r)
	if !ok {
		return
	}

	domainID := chi.URLParam(r, "domainID")
	domain, err := h.service.GetDomain(domainID)
	if err != nil || domain.AccountID != account.ID {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	if domain.SSLEnabled {
		h.json(w, http.StatusOK, map[string]interface{}{"message": "SSL already enabled"})
		return
	}

	// Check DNS first
	serverIP := account.ServerHostname
	ips, _ := net.LookupHost(domain.Domain)
	if len(ips) == 0 || ips[0] != serverIP {
		h.jsonError(w, "DNS not configured. Point your domain to "+serverIP+" first.", http.StatusBadRequest)
		return
	}

	// Update status to generating
	h.service.UpdateDomainStatus(domainID, true, models.DomainStatusSSLGenerating, nil)

	// Start SSL generation in background
	go h.generateSSL(account, domain)

	h.json(w, http.StatusOK, map[string]interface{}{
		"message": "SSL generation started",
		"status":  models.DomainStatusSSLGenerating,
	})
}

// generateSSL runs SSL generation in background
func (h *Handler) generateSSL(account *models.HostingAccount, domain *models.HostingDomain) {
	// Get server credentials and run Let's Encrypt
	err := h.service.GenerateSSLForDomain(account, domain)
	if err != nil {
		errMsg := err.Error()
		h.service.UpdateDomainStatus(domain.ID, true, models.DomainStatusPendingDNS, &errMsg)
	} else {
		h.service.UpdateDomainStatus(domain.ID, true, models.DomainStatusActive, nil)
		h.service.SetDomainSSLEnabled(domain.ID, true)
	}
}

// JSON helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

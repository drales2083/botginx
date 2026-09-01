package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/modules/hosting/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for hosting operations
type Handler struct {
	service   *services.HostingService
	templates *module.TemplateEngine
}

// NewHandler creates a new hosting handler instance
func NewHandler(service *services.HostingService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
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
		PackageID string `json:"package_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	account, err := h.service.PurchaseHosting(userID, input.PackageID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"account": account})
}

// UserOverview shows the account overview/dashboard
func (h *Handler) UserOverview(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	domains, _ := h.service.ListDomains(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:overview.html", map[string]interface{}{
		"Title":     "Account Overview",
		"Account":   account,
		"Domains":   domains,
		"NavActive": "overview",
	})
}

// UserDomains shows the domains management page
func (h *Handler) UserDomains(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	domains, _ := h.service.ListDomains(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:domains.html", map[string]interface{}{
		"Title":     "Domains",
		"Account":   account,
		"Domains":   domains,
		"NavActive": "domains",
	})
}

// UserDomainSettings shows antibot settings for a domain
func (h *Handler) UserDomainSettings(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	domainID := chi.URLParam(r, "domainID")

	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	settings, err := h.service.GetDomainSettings(domainID)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "hosting:domain_settings.html", map[string]interface{}{
		"Title":     "Domain Settings",
		"Account":   account,
		"Settings":  settings,
		"NavActive": "domains",
	})
}

// UserEmails shows the email accounts management page
func (h *Handler) UserEmails(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	emails, _ := h.service.ListEmails(accountID)
	domains, _ := h.service.ListDomains(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:emails.html", map[string]interface{}{
		"Title":     "Email Accounts",
		"Account":   account,
		"Emails":    emails,
		"Domains":   domains,
		"NavActive": "emails",
	})
}

// UserDatabases shows the databases management page
func (h *Handler) UserDatabases(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	databases, _ := h.service.ListDatabases(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:databases.html", map[string]interface{}{
		"Title":     "Databases",
		"Account":   account,
		"Databases": databases,
		"NavActive": "databases",
	})
}

// UserFTP shows the FTP accounts management page
func (h *Handler) UserFTP(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	account, err := h.service.GetAccount(accountID)
	if err != nil {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	ftpAccounts, _ := h.service.ListFTP(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:ftp.html", map[string]interface{}{
		"Title":       "FTP Accounts",
		"Account":     account,
		"FTPAccounts": ftpAccounts,
		"NavActive":   "ftp",
	})
}

// ========== Admin Page Handlers ==========

// AdminServers shows the servers management page
func (h *Handler) AdminServers(w http.ResponseWriter, r *http.Request) {
	servers, _ := h.service.ListServers()

	module.Render(w, r, h.templates, "hosting:admin_servers.html", map[string]interface{}{
		"Title":     "Hosting Servers",
		"Servers":   servers,
		"NavActive": "servers",
	})
}

// AdminPackages shows the packages management page
func (h *Handler) AdminPackages(w http.ResponseWriter, r *http.Request) {
	packages, _ := h.service.ListPackages()

	module.Render(w, r, h.templates, "hosting:admin_packages.html", map[string]interface{}{
		"Title":     "Hosting Packages",
		"Packages":  packages,
		"NavActive": "packages",
	})
}

// AdminAccounts shows all hosting accounts for all users
func (h *Handler) AdminAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.service.ListAllAccounts()

	module.Render(w, r, h.templates, "hosting:admin_accounts.html", map[string]interface{}{
		"Title":     "All Hosting Accounts",
		"Accounts":  accounts,
		"NavActive": "accounts",
	})
}

// ========== User API Handlers ==========

// APIAddDomain adds a domain to an account
func (h *Handler) APIAddDomain(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var input struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	domain, err := h.service.AddDomain(accountID, input.Domain)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"domain": domain})
}

// APIDeleteDomain removes a domain from an account
func (h *Handler) APIDeleteDomain(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.DeleteDomain(accountID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIEnableSSL enables SSL for a domain
func (h *Handler) APIEnableSSL(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.EnableSSL(accountID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIUpdateDomainSettings updates antibot settings for a domain
func (h *Handler) APIUpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")

	var input models.UpdateDomainSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateDomainSettings(domainID, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIAddEmail creates an email account
func (h *Handler) APIAddEmail(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var input models.AddEmailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email, err := h.service.AddEmail(accountID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"email": email})
}

// APIDeleteEmail removes an email account
func (h *Handler) APIDeleteEmail(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	emailID := chi.URLParam(r, "emailID")

	if err := h.service.DeleteEmail(accountID, emailID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIAddDatabase creates a database
func (h *Handler) APIAddDatabase(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var input models.AddDatabaseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	database, err := h.service.AddDatabase(accountID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"database": database})
}

// APIDeleteDatabase removes a database
func (h *Handler) APIDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	dbID := chi.URLParam(r, "dbID")

	if err := h.service.DeleteDatabase(accountID, dbID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIAddFTP creates an FTP account
func (h *Handler) APIAddFTP(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")

	var input models.AddFTPInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ftp, err := h.service.AddFTP(accountID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"ftp": ftp})
}

// APIDeleteFTP removes an FTP account
func (h *Handler) APIDeleteFTP(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	ftpID := chi.URLParam(r, "ftpID")

	if err := h.service.DeleteFTP(accountID, ftpID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIReactivate reactivates a suspended account by paying the balance
func (h *Handler) APIReactivate(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	userID := ctx.GetUserID(r)

	account, err := h.service.GetAccount(accountID)
	if err != nil {
		h.jsonError(w, "Account not found", http.StatusNotFound)
		return
	}

	// Get package price
	pkg, err := h.service.GetPackage(*account.PackageID)
	if err != nil {
		h.jsonError(w, "Package not found", http.StatusInternalServerError)
		return
	}

	// Check balance
	balance := h.service.GetUserBalance(userID)
	if balance < pkg.PriceMonthly {
		h.jsonError(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Unsuspend and deduct
	if err := h.service.UnsuspendAccount(accountID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.service.DeductBalance(userID, pkg.PriceMonthly, "Hosting reactivation: "+pkg.Name)

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ========== Admin API Handlers ==========

// APICreateServer creates a new hosting server
func (h *Handler) APICreateServer(w http.ResponseWriter, r *http.Request) {
	var input models.CreateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	server, err := h.service.CreateServer(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"server": server})
}

// APITestServer tests connection to a server
func (h *Handler) APITestServer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	if err := h.service.TestServerConnection(serverID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIToggleServer enables or disables a server
func (h *Handler) APIToggleServer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	var input struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.ToggleServer(serverID, input.Active); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIDeleteServer removes a server
func (h *Handler) APIDeleteServer(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	if err := h.service.DeleteServer(serverID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APICreatePackage creates a new hosting package
func (h *Handler) APICreatePackage(w http.ResponseWriter, r *http.Request) {
	var input models.CreatePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pkg, err := h.service.CreatePackage(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"package": pkg})
}

// APIUpdatePackage updates an existing hosting package
func (h *Handler) APIUpdatePackage(w http.ResponseWriter, r *http.Request) {
	packageID := chi.URLParam(r, "id")

	var input models.CreatePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdatePackage(packageID, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APITogglePackage enables or disables a package for new purchases
func (h *Handler) APITogglePackage(w http.ResponseWriter, r *http.Request) {
	packageID := chi.URLParam(r, "id")

	var input struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.TogglePackage(packageID, input.Active); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APISuspendAccount suspends a hosting account
func (h *Handler) APISuspendAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")

	if err := h.service.SuspendAccount(accountID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIUnsuspendAccount unsuspends a hosting account
func (h *Handler) APIUnsuspendAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")

	if err := h.service.UnsuspendAccount(accountID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APITopUpBalance adds funds to a user's balance
func (h *Handler) APITopUpBalance(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID      string  `json:"user_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.TopUpBalance(input.UserID, input.Amount, input.Description); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ========== Helpers ==========

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

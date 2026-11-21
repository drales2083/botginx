package handlers

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	ageddomainsmodels "github.com/botginx/botginx/modules/ageddomains/models"
	ageddomainssvc "github.com/botginx/botginx/modules/ageddomains/services"
	healthservices "github.com/botginx/botginx/modules/domainhealth/services"
	"github.com/botginx/botginx/modules/marketplace/models"
	"github.com/botginx/botginx/modules/marketplace/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/domainmask"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service       *services.MarketplaceService
	templates     *module.TemplateEngine
	safetyChecker *healthservices.SafetyChecker

	// Aged domains services (optional, set via SetAgedDomainsServices)
	agedBalanceService   *ageddomainssvc.BalanceService
	agedInventoryService *ageddomainssvc.InventoryService
	agedPurchaseService  *ageddomainssvc.PurchaseService
	agedFavoritesService *ageddomainssvc.FavoritesService
}

func NewHandler(service *services.MarketplaceService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:       service,
		templates:     templates,
		safetyChecker: healthservices.NewSafetyChecker(""),
	}
}

// SetAgedDomainsServices wires up the aged domains services for the unified marketplace
func (h *Handler) SetAgedDomainsServices(bs *ageddomainssvc.BalanceService, is *ageddomainssvc.InventoryService, ps *ageddomainssvc.PurchaseService, fs *ageddomainssvc.FavoritesService) {
	h.agedBalanceService = bs
	h.agedInventoryService = is
	h.agedPurchaseService = ps
	h.agedFavoritesService = fs
}

// SetGoogleAPIKey sets the Google Safe Browsing API key for safety checks
func (h *Handler) SetGoogleAPIKey(apiKey string) {
	h.safetyChecker = healthservices.NewSafetyChecker(apiKey)
}

// ============ Admin Handlers ============

// AdminList shows all marketplace domains
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	domains, _ := h.service.ListForSale()
	sales, _ := h.service.GetSalesHistory()
	sellable, _ := h.service.ListSellable()

	// Calculate total revenue
	var totalRevenue float64
	for _, sale := range sales {
		totalRevenue += sale.Price
	}

	module.Render(w, r, h.templates, "marketplace:admin_list.html", map[string]interface{}{
		"Title":        "Sell Domain",
		"Domains":      domains,
		"Sales":        sales,
		"Sellable":     sellable,
		"TotalRevenue": totalRevenue,
	})
}

// APIListForSale returns marketplace domains as JSON
func (h *Handler) APIListForSale(w http.ResponseWriter, r *http.Request) {
	domains, err := h.service.ListForSale()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, domains)
}

// APIMarkForSale adds a domain to marketplace
func (h *Handler) APIMarkForSale(w http.ResponseWriter, r *http.Request) {
	var input models.ListForSaleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.DomainID == "" {
		h.jsonError(w, "Domain ID required", http.StatusBadRequest)
		return
	}
	if input.Price <= 0 {
		h.jsonError(w, "Price must be greater than 0", http.StatusBadRequest)
		return
	}

	if err := h.service.MarkForSale(input.DomainID, input.Price, input.Description, input.DomainType); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIUpdateListing updates a marketplace listing
func (h *Handler) APIUpdateListing(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")

	var input models.UpdateListingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateListing(domainID, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIRemoveFromSale removes domain from marketplace
func (h *Handler) APIRemoveFromSale(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.RemoveFromSale(domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIGetSales returns sales history
func (h *Handler) APIGetSales(w http.ResponseWriter, r *http.Request) {
	sales, err := h.service.GetSalesHistory()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, sales)
}

// ============ User Handlers ============

// UserBrowse shows available domains for purchase
func (h *Handler) UserBrowse(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	user := ctx.GetUser(r)
	domains, _ := h.service.ListAvailable()
	balance := h.service.GetUserBalance(userID)

	// Mask domain names for display (hide full name until purchase)
	for i := range domains {
		domains[i].Name = domainmask.Mask(domains[i].Name)
	}

	// Get aged domains data if services are wired up
	var agedBalance *ageddomainsmodels.BalanceInfo
	var agedTLDs []string
	var agedFavCount int
	if h.agedBalanceService != nil && h.agedInventoryService != nil && h.agedFavoritesService != nil {
		agedBalance, _ = h.agedBalanceService.GetAvailableBalance(r.Context(), user.ID)
		agedTLDs, _ = h.agedInventoryService.GetAvailableTLDs(r.Context())
		agedFavCount, _ = h.agedFavoritesService.Count(r.Context(), user.ID)
	}
	if agedBalance == nil {
		agedBalance = &ageddomainsmodels.BalanceInfo{Available: 0, Locked: 0, Total: 0}
	}

	module.RenderUserSection(w, r, h.templates, "marketplace:user_browse.html", map[string]interface{}{
		"Title":         "Buy Domain",
		"Domains":       domains,
		"Balance":       balance,
		"AgedBalance":   agedBalance,
		"AvailableTLDs": agedTLDs,
		"FavoriteCount": agedFavCount,
	})
}

// APIListAvailable returns available domains as JSON
func (h *Handler) APIListAvailable(w http.ResponseWriter, r *http.Request) {
	domains, err := h.service.ListAvailable()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Mask domain names for display (hide full name until purchase)
	for i := range domains {
		domains[i].Name = domainmask.Mask(domains[i].Name)
	}

	h.json(w, http.StatusOK, domains)
}

// APIPurchase handles domain purchase
func (h *Handler) APIPurchase(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.Purchase(domainID, userID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Domain purchased successfully",
	})
}

// APICheckDomain performs real-time health checks on a marketplace domain
func (h *Handler) APICheckDomain(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")

	// Get domain info
	domain, err := h.service.GetDomain(domainID)
	if err != nil || !domain.IsMarketplace {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Perform checks
	checks := []map[string]interface{}{}

	// 1. DNS Check
	dnsOK, dnsMsg := h.checkDNS(domain.Name)
	checks = append(checks, map[string]interface{}{
		"name":   "DNS Resolution",
		"status": dnsOK,
		"detail": dnsMsg,
	})

	// 2. SSL Check
	sslOK, sslMsg := h.checkSSL(domain.Name)
	checks = append(checks, map[string]interface{}{
		"name":   "SSL Certificate",
		"status": sslOK,
		"detail": sslMsg,
	})

	// 3. Safety Check (blacklists)
	safetyResult := h.safetyChecker.CheckDomain(domain.Name)
	safetyOK := safetyResult.Status == healthservices.SafetyClean
	safetyMsg := "Clean - No threats detected"
	if !safetyOK {
		safetyMsg = "Flagged - Threats detected"
	}
	checks = append(checks, map[string]interface{}{
		"name":   "Google Safe Browsing",
		"status": safetyOK,
		"detail": safetyMsg,
	})

	// 4. Spam blacklist check (from safety checker)
	checks = append(checks, map[string]interface{}{
		"name":   "Spam Blacklists",
		"status": safetyOK,
		"detail": safetyMsg,
	})

	// 5. Ready status
	readyOK := dnsOK && sslOK && safetyOK
	readyMsg := "Ready for immediate use"
	if !readyOK {
		readyMsg = "Some checks failed"
	}
	checks = append(checks, map[string]interface{}{
		"name":   "Ready Status",
		"status": readyOK,
		"detail": readyMsg,
	})

	h.json(w, http.StatusOK, map[string]interface{}{
		"domain": domainmask.Mask(domain.Name),
		"checks": checks,
	})
}

// checkDNS verifies the domain resolves
func (h *Handler) checkDNS(domain string) (bool, string) {
	_, err := net.LookupHost(domain)
	if err != nil {
		return false, "DNS resolution failed"
	}
	return true, "Resolves correctly"
}

// checkSSL verifies SSL certificate is valid
func (h *Handler) checkSSL(domain string) (bool, string) {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp",
		domain+":443",
		&tls.Config{InsecureSkipVerify: false},
	)
	if err != nil {
		// Try with InsecureSkipVerify to check if cert exists but is self-signed
		conn2, err2 := tls.DialWithDialer(
			&net.Dialer{Timeout: 5 * time.Second},
			"tcp",
			domain+":443",
			&tls.Config{InsecureSkipVerify: true},
		)
		if err2 != nil {
			return false, "SSL connection failed"
		}
		conn2.Close()
		return true, "Certificate present (self-signed)"
	}
	defer conn.Close()

	// Check expiry
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) > 0 {
		expiry := certs[0].NotAfter
		daysLeft := int(time.Until(expiry).Hours() / 24)
		if daysLeft < 0 {
			return false, "Certificate expired"
		}
		if daysLeft < 14 {
			return true, "Valid (expiring soon)"
		}
		return true, "Valid certificate"
	}
	return true, "Certificate present"
}

// ============ Aged Domains API Handlers ============

// APIAgedDomainsList returns available aged domains with advanced filters
func (h *Handler) APIAgedDomainsList(w http.ResponseWriter, r *http.Request) {
	if h.agedInventoryService == nil || h.agedFavoritesService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	user := ctx.GetUser(r)

	filters := ageddomainsmodels.ListFilters{
		TLD:           r.URL.Query().Get("tld"),
		Category:      r.URL.Query().Get("category"),
		SafetyStatus:  r.URL.Query().Get("safety"),
		Search:        r.URL.Query().Get("search"),
		SortBy:        r.URL.Query().Get("sort_by"),
		SortDir:       r.URL.Query().Get("sort_dir"),
		FavoritesOnly: r.URL.Query().Get("favorites") == "true",
	}

	if v := r.URL.Query().Get("min_price"); v != "" {
		filters.MinPrice, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("max_price"); v != "" {
		filters.MaxPrice, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("min_age"); v != "" {
		filters.MinAge, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("max_age"); v != "" {
		filters.MaxAge, _ = strconv.Atoi(v)
	}
	filters.Page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	filters.PerPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))

	domains, total, err := h.agedInventoryService.ListWithFilters(r.Context(), filters, user.ID)
	if err != nil {
		h.jsonError(w, "Failed to list domains", http.StatusInternalServerError)
		return
	}

	favoriteIDs, _ := h.agedFavoritesService.GetUserFavoriteIDs(r.Context(), user.ID)

	domainsWithFavorites := make([]ageddomainsmodels.DomainWithFavorite, len(domains))
	for i, d := range domains {
		domainsWithFavorites[i] = ageddomainsmodels.DomainWithFavorite{
			DomainInventory: d,
			IsFavorite:      favoriteIDs[d.ID],
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"domains": domainsWithFavorites,
		"total":   total,
		"page":    filters.Page,
		"perPage": filters.PerPage,
	})
}

// APIAgedDomainsCheckSafety performs an on-demand safety check for an aged domain
func (h *Handler) APIAgedDomainsCheckSafety(w http.ResponseWriter, r *http.Request) {
	if h.agedInventoryService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")

	result, err := h.agedInventoryService.CheckSafety(r.Context(), id)
	if err != nil {
		if err == ageddomainssvc.ErrDomainNotFound {
			h.jsonError(w, "Domain not found", http.StatusNotFound)
			return
		}
		h.jsonError(w, "Safety check failed", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, result)
}

// APIAgedDomainsToggleFavorite adds or removes an aged domain from favorites
func (h *Handler) APIAgedDomainsToggleFavorite(w http.ResponseWriter, r *http.Request) {
	if h.agedFavoritesService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	user := ctx.GetUser(r)
	inventoryID := chi.URLParam(r, "id")

	isFavorite, err := h.agedFavoritesService.Toggle(r.Context(), user.ID, inventoryID)
	if err != nil {
		h.jsonError(w, "Failed to update favorite", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"isFavorite": isFavorite,
	})
}

// APIAgedDomainsGetTLDs returns available TLDs for filter dropdown
func (h *Handler) APIAgedDomainsGetTLDs(w http.ResponseWriter, r *http.Request) {
	if h.agedInventoryService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	tlds, err := h.agedInventoryService.GetAvailableTLDs(r.Context())
	if err != nil {
		h.jsonError(w, "Failed to get TLDs", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"tlds": tlds,
	})
}

// APIAgedDomainsPurchase initiates an aged domain purchase
func (h *Handler) APIAgedDomainsPurchase(w http.ResponseWriter, r *http.Request) {
	if h.agedPurchaseService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	user := ctx.GetUser(r)
	inventoryID := chi.URLParam(r, "id")

	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Confirm {
		h.jsonError(w, "Purchase must be confirmed", http.StatusBadRequest)
		return
	}

	purchase, err := h.agedPurchaseService.PurchaseDomain(r.Context(), user.ID, inventoryID)
	if err != nil {
		if err == ageddomainssvc.ErrInsufficientBalance {
			h.jsonError(w, "Insufficient balance", http.StatusPaymentRequired)
			return
		}
		if err == ageddomainssvc.ErrDomainUnavailable {
			h.jsonError(w, "Domain no longer available", http.StatusConflict)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"purchaseId": purchase.ID,
		"status":     purchase.Status,
		"domain":     purchase.DomainName,
		"priceCents": purchase.SalePrice,
	})
}

// APIAgedDomainsGetStatus returns the current status of an aged domain purchase
func (h *Handler) APIAgedDomainsGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.agedPurchaseService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	user := ctx.GetUser(r)
	purchaseID := chi.URLParam(r, "id")

	status, err := h.agedPurchaseService.GetStatus(r.Context(), purchaseID)
	if err != nil {
		h.jsonError(w, "Purchase not found", http.StatusNotFound)
		return
	}

	purchase, _ := h.agedPurchaseService.Get(r.Context(), purchaseID)
	if purchase != nil && purchase.UserID != user.ID {
		h.jsonError(w, "Access denied", http.StatusForbidden)
		return
	}

	h.json(w, http.StatusOK, status)
}

// APIAgedDomainsGetBalance returns the user's available balance for aged domains
func (h *Handler) APIAgedDomainsGetBalance(w http.ResponseWriter, r *http.Request) {
	if h.agedBalanceService == nil {
		h.jsonError(w, "Aged domains not configured", http.StatusServiceUnavailable)
		return
	}

	user := ctx.GetUser(r)

	balance, err := h.agedBalanceService.GetAvailableBalance(r.Context(), user.ID)
	if err != nil {
		h.jsonError(w, "Failed to get balance", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, balance)
}

// ============ Helpers ============

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

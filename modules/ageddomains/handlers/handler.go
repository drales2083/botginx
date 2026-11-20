package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/ageddomains/models"
	"github.com/botginx/botginx/modules/ageddomains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	balanceService   *services.BalanceService
	inventoryService *services.InventoryService
	purchaseService  *services.PurchaseService
	favoritesService *services.FavoritesService
	templates        *module.TemplateEngine
}

func NewHandler(bs *services.BalanceService, is *services.InventoryService, ps *services.PurchaseService, fs *services.FavoritesService, t *module.TemplateEngine) *Handler {
	return &Handler{
		balanceService:   bs,
		inventoryService: is,
		purchaseService:  ps,
		favoritesService: fs,
		templates:        t,
	}
}

// Browse renders the marketplace browsing page
func (h *Handler) Browse(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	balance, _ := h.balanceService.GetAvailableBalance(r.Context(), user.ID)
	tlds, _ := h.inventoryService.GetAvailableTLDs(r.Context())
	favCount, _ := h.favoritesService.Count(r.Context(), user.ID)

	module.Render(w, r, h.templates, "ageddomains:browse.html", map[string]interface{}{
		"Title":         "Domain Marketplace",
		"Balance":       balance,
		"AvailableTLDs": tlds,
		"FavoriteCount": favCount,
	})
}

// APIListDomains returns available domains with advanced filters
func (h *Handler) APIListDomains(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	// Parse all filter params
	filters := models.ListFilters{
		TLD:           r.URL.Query().Get("tld"),
		Category:      r.URL.Query().Get("category"),
		SafetyStatus:  r.URL.Query().Get("safety"),
		Search:        r.URL.Query().Get("search"),
		SortBy:        r.URL.Query().Get("sort_by"),
		SortDir:       r.URL.Query().Get("sort_dir"),
		FavoritesOnly: r.URL.Query().Get("favorites") == "true",
	}

	// Parse numeric filters
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

	// Get domains
	domains, total, err := h.inventoryService.ListWithFilters(r.Context(), filters, user.ID)
	if err != nil {
		h.jsonError(w, "Failed to list domains", http.StatusInternalServerError)
		return
	}

	// Get user's favorites to mark domains
	favoriteIDs, _ := h.favoritesService.GetUserFavoriteIDs(r.Context(), user.ID)

	// Build response with favorite status
	domainsWithFavorites := make([]models.DomainWithFavorite, len(domains))
	for i, d := range domains {
		domainsWithFavorites[i] = models.DomainWithFavorite{
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

// APICheckSafety performs an on-demand safety check for a domain
func (h *Handler) APICheckSafety(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	result, err := h.inventoryService.CheckSafety(r.Context(), id)
	if err != nil {
		if err == services.ErrDomainNotFound {
			h.jsonError(w, "Domain not found", http.StatusNotFound)
			return
		}
		h.jsonError(w, "Safety check failed", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, result)
}

// APIPurchase initiates a domain purchase
func (h *Handler) APIPurchase(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	inventoryID := chi.URLParam(r, "id")

	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Confirm {
		h.jsonError(w, "Purchase must be confirmed", http.StatusBadRequest)
		return
	}

	purchase, err := h.purchaseService.PurchaseDomain(r.Context(), user.ID, inventoryID)
	if err != nil {
		if err == services.ErrInsufficientBalance {
			h.jsonError(w, "Insufficient balance", http.StatusPaymentRequired)
			return
		}
		if err == services.ErrDomainUnavailable {
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

// APIGetStatus returns the current status of a purchase
func (h *Handler) APIGetStatus(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	purchaseID := chi.URLParam(r, "id")

	status, err := h.purchaseService.GetStatus(r.Context(), purchaseID)
	if err != nil {
		h.jsonError(w, "Purchase not found", http.StatusNotFound)
		return
	}

	purchase, _ := h.purchaseService.Get(r.Context(), purchaseID)
	if purchase != nil && purchase.UserID != user.ID {
		h.jsonError(w, "Access denied", http.StatusForbidden)
		return
	}

	h.json(w, http.StatusOK, status)
}

// APIGetBalance returns the user's available balance
func (h *Handler) APIGetBalance(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	balance, err := h.balanceService.GetAvailableBalance(r.Context(), user.ID)
	if err != nil {
		h.jsonError(w, "Failed to get balance", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, balance)
}

// APIToggleFavorite adds or removes a domain from favorites
func (h *Handler) APIToggleFavorite(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	inventoryID := chi.URLParam(r, "id")

	isFavorite, err := h.favoritesService.Toggle(r.Context(), user.ID, inventoryID)
	if err != nil {
		h.jsonError(w, "Failed to update favorite", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"isFavorite": isFavorite,
	})
}

// APIGetTLDs returns available TLDs for filter dropdown
func (h *Handler) APIGetTLDs(w http.ResponseWriter, r *http.Request) {
	tlds, err := h.inventoryService.GetAvailableTLDs(r.Context())
	if err != nil {
		h.jsonError(w, "Failed to get TLDs", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"tlds": tlds,
	})
}

// Helper methods
func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

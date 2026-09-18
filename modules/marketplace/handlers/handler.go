package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/marketplace/models"
	"github.com/botginx/botginx/modules/marketplace/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.MarketplaceService
	templates *module.TemplateEngine
}

func NewHandler(service *services.MarketplaceService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
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

	if err := h.service.MarkForSale(input.DomainID, input.Price, input.Description); err != nil {
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
	domains, _ := h.service.ListAvailable()
	balance := h.service.GetUserBalance(userID)

	module.RenderUserSection(w, r, h.templates, "marketplace:user_browse.html", map[string]interface{}{
		"Title":   "Buy Domain",
		"Domains": domains,
		"Balance": balance,
	})
}

// APIListAvailable returns available domains as JSON
func (h *Handler) APIListAvailable(w http.ResponseWriter, r *http.Request) {
	domains, err := h.service.ListAvailable()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
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

// ============ Helpers ============

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

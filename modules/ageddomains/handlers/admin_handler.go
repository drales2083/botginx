package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/botginx/botginx/modules/ageddomains/models"
	"github.com/botginx/botginx/modules/ageddomains/services"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	providerRegistry *services.ProviderRegistry
	inventoryService *services.InventoryService
	purchaseService  *services.PurchaseService
	templates        *module.TemplateEngine
	background       *services.BackgroundJobs
}

func NewAdminHandler(pr *services.ProviderRegistry, is *services.InventoryService, ps *services.PurchaseService, t *module.TemplateEngine) *AdminHandler {
	return &AdminHandler{
		providerRegistry: pr,
		inventoryService: is,
		purchaseService:  ps,
		templates:        t,
	}
}

// SetBackgroundJobs sets the background jobs reference for manual refresh
func (h *AdminHandler) SetBackgroundJobs(bg *services.BackgroundJobs) {
	h.background = bg
}

// Dashboard renders the admin dashboard
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, _ := h.inventoryService.GetStats(r.Context())
	providers, _ := h.providerRegistry.List(r.Context())

	module.Render(w, r, h.templates, "ageddomains:admin_dashboard.html", map[string]interface{}{
		"Title":     "Aged Domains Admin",
		"Stats":     stats,
		"Providers": providers,
	})
}

// Providers renders the provider management page
func (h *AdminHandler) Providers(w http.ResponseWriter, r *http.Request) {
	providers, _ := h.providerRegistry.List(r.Context())

	module.Render(w, r, h.templates, "ageddomains:admin_providers.html", map[string]interface{}{
		"Title":     "Domain Providers",
		"Providers": providers,
	})
}

// Inventory renders the inventory management page
func (h *AdminHandler) Inventory(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "ageddomains:admin_inventory.html", map[string]interface{}{
		"Title": "Domain Inventory",
	})
}

// Purchases renders the purchases page
func (h *AdminHandler) Purchases(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "ageddomains:admin_purchases.html", map[string]interface{}{
		"Title": "Domain Purchases",
	})
}

// API Handlers

func (h *AdminHandler) APIListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.providerRegistry.List(r.Context())
	if err != nil {
		h.jsonError(w, "Failed to list providers", http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, providers)
}

func (h *AdminHandler) APICreateProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string  `json:"name"`
		DisplayName   string  `json:"displayName"`
		ProviderType  string  `json:"providerType"`
		APIKey        string  `json:"apiKey"`
		APISecret     string  `json:"apiSecret"`
		IsActive      bool    `json:"isActive"`
		MarkupPercent float64 `json:"markupPercent"`
		MinAgeYears   int     `json:"minAgeYears"`
		MaxPriceCents int     `json:"maxPriceCents"`
		AllowedTLDs   string  `json:"allowedTlds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Encrypt API credentials if encryption key is configured, otherwise store plaintext
	apiKey := req.APIKey
	apiSecret := req.APISecret
	if encryptedKey, err := crypto.Encrypt(req.APIKey); err == nil {
		apiKey = encryptedKey
	}
	if encryptedSecret, err := crypto.Encrypt(req.APISecret); err == nil {
		apiSecret = encryptedSecret
	}

	provider := &models.Provider{
		ID:            uuid.New().String(),
		Name:          req.Name,
		DisplayName:   req.DisplayName,
		ProviderType:  req.ProviderType,
		APIKey:        apiKey,
		APISecret:     apiSecret,
		IsActive:      req.IsActive,
		MarkupPercent: req.MarkupPercent,
		MinAgeYears:   req.MinAgeYears,
		MaxPriceCents: req.MaxPriceCents,
		AllowedTLDs:   req.AllowedTLDs,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := h.providerRegistry.Create(r.Context(), provider); err != nil {
		h.jsonError(w, "Failed to create provider", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, provider)
}

func (h *AdminHandler) APIUpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.providerRegistry.Get(r.Context(), id)
	if err != nil {
		h.jsonError(w, "Provider not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name          string  `json:"name"`
		DisplayName   string  `json:"displayName"`
		ProviderType  string  `json:"providerType"`
		APIKey        string  `json:"apiKey"`
		APISecret     string  `json:"apiSecret"`
		IsActive      bool    `json:"isActive"`
		MarkupPercent float64 `json:"markupPercent"`
		MinAgeYears   int     `json:"minAgeYears"`
		MaxPriceCents int     `json:"maxPriceCents"`
		AllowedTLDs   string  `json:"allowedTlds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	existing.Name = req.Name
	existing.DisplayName = req.DisplayName
	existing.ProviderType = req.ProviderType
	if req.APIKey != "" {
		// Encrypt if key configured, otherwise store plaintext
		if encryptedKey, err := crypto.Encrypt(req.APIKey); err == nil {
			existing.APIKey = encryptedKey
		} else {
			existing.APIKey = req.APIKey
		}
	}
	if req.APISecret != "" {
		if encryptedSecret, err := crypto.Encrypt(req.APISecret); err == nil {
			existing.APISecret = encryptedSecret
		} else {
			existing.APISecret = req.APISecret
		}
	}
	existing.IsActive = req.IsActive
	existing.MarkupPercent = req.MarkupPercent
	existing.MinAgeYears = req.MinAgeYears
	existing.MaxPriceCents = req.MaxPriceCents
	existing.AllowedTLDs = req.AllowedTLDs
	existing.UpdatedAt = time.Now()

	if err := h.providerRegistry.Update(r.Context(), existing); err != nil {
		h.jsonError(w, "Failed to update provider", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, existing)
}

func (h *AdminHandler) APIDeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.providerRegistry.Delete(r.Context(), id); err != nil {
		h.jsonError(w, "Failed to delete provider", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *AdminHandler) APITestProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.providerRegistry.TestProvider(r.Context(), id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *AdminHandler) APIListInventory(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	domains, total, err := h.inventoryService.ListAvailable(r.Context(), "", 0, "", page, perPage)
	if err != nil {
		h.jsonError(w, "Failed to list inventory", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"domains": domains,
		"total":   total,
	})
}

func (h *AdminHandler) APIRefreshInventory(w http.ResponseWriter, r *http.Request) {
	if h.background == nil {
		h.jsonError(w, "Background jobs not initialized", http.StatusInternalServerError)
		return
	}
	h.background.RefreshInventoryNow()
	h.json(w, http.StatusOK, map[string]bool{"started": true})
}

func (h *AdminHandler) APIListPurchases(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	purchases, total, err := h.purchaseService.ListAll(r.Context(), page, perPage)
	if err != nil {
		h.jsonError(w, "Failed to list purchases", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"purchases": purchases,
		"total":     total,
	})
}

// Helper methods
func (h *AdminHandler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AdminHandler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

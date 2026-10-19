package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/promocodes/models"
	"github.com/botginx/botginx/modules/promocodes/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// Handler handles promo code routes
type Handler struct {
	service   *services.PromoService
	templates *module.TemplateEngine
}

// NewHandler creates a new promo handler
func NewHandler(service *services.PromoService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]string{"error": message})
}

// ============== Admin Page Handlers ==============

// AdminList shows all promo codes
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	promos, _ := h.service.List()

	module.Render(w, r, h.templates, "promocodes:admin_list.html", map[string]interface{}{
		"Title":  "Promo Codes",
		"Promos": promos,
	})
}

// AdminNew shows the create form
func (h *Handler) AdminNew(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "promocodes:admin_form.html", map[string]interface{}{
		"Title":              "Create Promo Code",
		"Promo":              nil,
		"EligibilityOptions": models.EligibilityOptions(),
		"IsNew":              true,
	})
}

// AdminCreate handles promo creation
func (h *Handler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	input := models.CreatePromoInput{
		Code:          r.FormValue("code"),
		Name:          r.FormValue("name"),
		Description:   r.FormValue("description"),
		DiscountType:  r.FormValue("discount_type"),
		Eligibility:   r.FormValue("eligibility"),
		IsActive:      r.FormValue("is_active") == "on",
	}

	// Parse discount value
	if v, err := strconv.ParseFloat(r.FormValue("discount_value"), 64); err == nil {
		input.DiscountValue = v
	}

	// Parse max uses
	if v := r.FormValue("max_uses"); v != "" {
		if maxUses, err := strconv.Atoi(v); err == nil {
			input.MaxUses = &maxUses
		}
	}

	// Parse max uses per user
	if v := r.FormValue("max_uses_per_user"); v != "" {
		if maxPerUser, err := strconv.Atoi(v); err == nil {
			input.MaxUsesPerUser = &maxPerUser
		}
	} else {
		// Default to 1 (one-time use per user)
		one := 1
		input.MaxUsesPerUser = &one
	}

	// Parse dates
	if v := r.FormValue("valid_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			input.ValidFrom = &t
		}
	}
	if v := r.FormValue("valid_until"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			// Set to end of day
			endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			input.ValidUntil = &endOfDay
		}
	}

	userID := ctx.GetUserID(r)
	_, err := h.service.Create(input, userID)
	if err != nil {
		module.Render(w, r, h.templates, "promocodes:admin_form.html", map[string]interface{}{
			"Title":              "Create Promo Code",
			"Promo":              nil,
			"EligibilityOptions": models.EligibilityOptions(),
			"IsNew":              true,
			"Error":              err.Error(),
			"Input":              input,
		})
		return
	}

	http.Redirect(w, r, "/admin/promocodes?created=1", http.StatusSeeOther)
}

// AdminEdit shows the edit form
func (h *Handler) AdminEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	promo, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Promo code not found", http.StatusNotFound)
		return
	}

	module.Render(w, r, h.templates, "promocodes:admin_form.html", map[string]interface{}{
		"Title":              "Edit Promo Code",
		"Promo":              promo,
		"EligibilityOptions": models.EligibilityOptions(),
		"IsNew":              false,
	})
}

// AdminUpdate handles promo updates
func (h *Handler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	discountType := r.FormValue("discount_type")
	eligibility := r.FormValue("eligibility")
	isActive := r.FormValue("is_active") == "on"

	input := models.UpdatePromoInput{
		Name:        &name,
		Description: &description,
		DiscountType: &discountType,
		Eligibility: &eligibility,
		IsActive:    &isActive,
	}

	// Parse discount value
	if v, err := strconv.ParseFloat(r.FormValue("discount_value"), 64); err == nil {
		input.DiscountValue = &v
	}

	// Parse max uses
	if v := r.FormValue("max_uses"); v != "" {
		if maxUses, err := strconv.Atoi(v); err == nil {
			input.MaxUses = &maxUses
		}
	}

	// Parse max uses per user
	if v := r.FormValue("max_uses_per_user"); v != "" {
		if maxPerUser, err := strconv.Atoi(v); err == nil {
			input.MaxUsesPerUser = &maxPerUser
		}
	}

	// Parse dates
	if v := r.FormValue("valid_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			input.ValidFrom = &t
		}
	}
	if v := r.FormValue("valid_until"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			input.ValidUntil = &endOfDay
		}
	}

	_, err := h.service.Update(id, input)
	if err != nil {
		promo, _ := h.service.Get(id)
		module.Render(w, r, h.templates, "promocodes:admin_form.html", map[string]interface{}{
			"Title":              "Edit Promo Code",
			"Promo":              promo,
			"EligibilityOptions": models.EligibilityOptions(),
			"IsNew":              false,
			"Error":              err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/admin/promocodes?updated=1", http.StatusSeeOther)
}

// AdminDelete handles promo deletion
func (h *Handler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]bool{"success": true})
}

// AdminToggle enables/disables a promo code
func (h *Handler) AdminToggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	promo, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Promo not found", http.StatusNotFound)
		return
	}

	newStatus := !promo.IsActive
	if err := h.service.SetActive(id, newStatus); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"isActive": newStatus,
	})
}

// AdminUsage shows redemption history
func (h *Handler) AdminUsage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	promo, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Promo code not found", http.StatusNotFound)
		return
	}

	redemptions, _ := h.service.GetRedemptions(id)
	stats, _ := h.service.GetStats(id)

	module.Render(w, r, h.templates, "promocodes:admin_usage.html", map[string]interface{}{
		"Title":       promo.Code + " - Usage History",
		"Promo":       promo,
		"Redemptions": redemptions,
		"Stats":       stats,
	})
}

// ============== User API Handlers ==============

// APIValidate validates a promo code for the current user
func (h *Handler) APIValidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string  `json:"code"`
		Amount float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	userID := ctx.GetUserID(r)
	if userID == "" {
		h.jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	result := h.service.ValidateCode(req.Code, userID, req.Amount)
	h.json(w, http.StatusOK, result)
}

// GetService returns the promo service for external use
func (h *Handler) GetService() *services.PromoService {
	return h.service
}

// ValidateForUser validates a code for a specific user (for internal use)
func (h *Handler) ValidateForUser(code, userID string, amount float64) *models.ValidateResult {
	return h.service.ValidateCode(code, userID, amount)
}

// RedeemForUser redeems a code for a specific user (for internal use)
func (h *Handler) RedeemForUser(promoID, userID, subscriptionID string, original, discount, final float64) (*models.PromoRedemption, error) {
	return h.service.RedeemCode(promoID, userID, subscriptionID, original, discount, final)
}

// FormatDiscount returns a human-readable discount string
func FormatDiscount(promo *models.PromoCode) string {
	if promo.DiscountType == models.DiscountTypePercentage {
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(promo.DiscountValue, 'f', 2, 64), "0"), ".") + "% off"
	}
	return "$" + strings.TrimRight(strings.TrimRight(strconv.FormatFloat(promo.DiscountValue, 'f', 2, 64), "0"), ".") + " off"
}

// FormatUsage returns usage as "X/Y" or "X/∞"
func FormatUsage(promo *models.PromoCode) string {
	if promo.MaxUses == nil {
		return strconv.Itoa(promo.CurrentUses) + "/∞"
	}
	return strconv.Itoa(promo.CurrentUses) + "/" + strconv.Itoa(*promo.MaxUses)
}

// FormatExpiry returns expiry date or "Never"
func FormatExpiry(promo *models.PromoCode) string {
	if promo.ValidUntil == nil {
		return "Never"
	}
	return promo.ValidUntil.Format("Jan 2, 2006")
}

// EligibilityLabel returns human-readable eligibility
func EligibilityLabel(eligibility string) string {
	for _, opt := range models.EligibilityOptions() {
		if opt.Value == eligibility {
			return opt.Label
		}
	}
	return eligibility
}

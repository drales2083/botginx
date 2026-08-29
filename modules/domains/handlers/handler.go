package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.DomainService
	templates *module.TemplateEngine
}

func NewHandler(service *services.DomainService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
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

	module.RenderUserSection(w, r, h.templates, "domains:show.html", map[string]interface{}{
		"Title":  domain.Name,
		"Domain": domain,
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

	h.json(w, http.StatusCreated, map[string]interface{}{"domain": domain})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
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
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func (h *Handler) APIVerifyDNS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// DNS verification logic would go here
	// For now, just mark as verified
	verified := true
	_, err := h.service.Update(id, models.UpdateDomainInput{
		DNSVerified: &verified,
	})
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"verified": true})
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

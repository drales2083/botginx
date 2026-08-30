package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/botginx/botginx/modules/iplists/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.IPListService
	templates *module.TemplateEngine
}

func NewHandler(service *services.IPListService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

// Pages

func (h *Handler) WhitelistPage(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	entries, _ := h.service.ListWhitelist(userID)

	module.RenderUserSection(w, r, h.templates, "iplists:whitelist.html", map[string]interface{}{
		"Title":     "IP Whitelist",
		"Whitelist": entries,
		"MaxIPs":    services.MaxWhitelistIPs,
	})
}

func (h *Handler) BlocklistPage(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	entries, _ := h.service.ListBlocklist(userID)

	module.RenderUserSection(w, r, h.templates, "iplists:blocklist.html", map[string]interface{}{
		"Title":     "IP Blocklist",
		"Blocklist": entries,
		"MaxIPs":    services.MaxBlocklistIPs,
	})
}

// Whitelist API

func (h *Handler) APIListWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	entries, err := h.service.ListWhitelist(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{
		"whitelist": entries,
		"maxIps":    services.MaxWhitelistIPs,
	})
}

func (h *Handler) APIAddWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		IP   string `json:"ip"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	input.IP = strings.TrimSpace(input.IP)
	if input.IP == "" {
		h.jsonError(w, "IP address is required", http.StatusBadRequest)
		return
	}

	entry, err := h.service.AddWhitelist(userID, input.IP, strings.TrimSpace(input.Note))
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"entry":   entry,
	})
}

func (h *Handler) APIRemoveWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	if err := h.service.RemoveWhitelist(userID, id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Blocklist API

func (h *Handler) APIListBlocklist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	entries, err := h.service.ListBlocklist(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{
		"blocklist": entries,
		"maxIps":    services.MaxBlocklistIPs,
	})
}

func (h *Handler) APIAddBlocklist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		IP   string `json:"ip"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	input.IP = strings.TrimSpace(input.IP)
	if input.IP == "" {
		h.jsonError(w, "IP address is required", http.StatusBadRequest)
		return
	}

	entry, err := h.service.AddBlocklist(userID, input.IP, strings.TrimSpace(input.Note), "manual")
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"entry":   entry,
	})
}

func (h *Handler) APIRemoveBlocklist(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	if err := h.service.RemoveBlocklist(userID, id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Quick block/unblock from analytics

func (h *Handler) APIQuickBlock(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	ip := chi.URLParam(r, "ip")

	if ip == "" {
		h.jsonError(w, "IP is required", http.StatusBadRequest)
		return
	}

	_, err := h.service.AddBlocklist(userID, ip, "Blocked from analytics", "analytics")
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"blocked": true,
	})
}

func (h *Handler) APIQuickUnblock(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	ip := chi.URLParam(r, "ip")

	if ip == "" {
		h.jsonError(w, "IP is required", http.StatusBadRequest)
		return
	}

	if err := h.service.RemoveBlocklistByIP(userID, ip); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"blocked": false,
	})
}

func (h *Handler) APICheckBlocked(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	ip := chi.URLParam(r, "ip")

	blocked := h.service.IsBlocked(userID, ip)

	h.json(w, http.StatusOK, map[string]interface{}{
		"ip":      ip,
		"blocked": blocked,
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

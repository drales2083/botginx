package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/webhooks/models"
	"github.com/botginx/botginx/modules/webhooks/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service    *services.WebhooksService
	dispatcher *services.Dispatcher
	templates  *module.TemplateEngine
}

func NewHandler(service *services.WebhooksService, dispatcher *services.Dispatcher, tmpl *module.TemplateEngine) *Handler {
	return &Handler{
		service:    service,
		dispatcher: dispatcher,
		templates:  tmpl,
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, msg string, status int) {
	h.json(w, status, map[string]string{"error": msg})
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	webhooks, err := h.service.List(userID)
	if err != nil {
		webhooks = []models.Webhook{}
	}

	module.RenderUserSection(w, r, h.templates, "webhooks:index.html", map[string]interface{}{
		"Title":       "Webhooks",
		"Webhooks":    webhooks,
		"ValidEvents": services.ValidEvents,
	})
}

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	webhooks, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, "Failed to list webhooks", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, webhooks)
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var req models.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	webhook, err := h.service.Create(userID, &req)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, webhook)
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	webhookID := chi.URLParam(r, "id")

	webhook, err := h.service.Get(userID, webhookID)
	if err != nil {
		h.jsonError(w, "Webhook not found", http.StatusNotFound)
		return
	}

	h.json(w, http.StatusOK, webhook)
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	webhookID := chi.URLParam(r, "id")

	var req models.UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	webhook, err := h.service.Update(userID, webhookID, &req)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, webhook)
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	webhookID := chi.URLParam(r, "id")

	if err := h.service.Delete(userID, webhookID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Webhook deleted",
	})
}

func (h *Handler) APIDeliveries(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	webhookID := chi.URLParam(r, "id")

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	deliveries, err := h.service.ListDeliveries(userID, webhookID, limit)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, deliveries)
}

func (h *Handler) APITest(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	webhookID := chi.URLParam(r, "id")

	webhook, err := h.service.Get(userID, webhookID)
	if err != nil {
		h.jsonError(w, "Webhook not found", http.StatusNotFound)
		return
	}

	if err := h.dispatcher.SendTestEvent(webhook); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Test event sent successfully",
	})
}

func (h *Handler) APIEvents(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]interface{}{
		"events": services.ValidEvents,
	})
}

func (h *Handler) Dispatcher() *services.Dispatcher {
	return h.dispatcher
}

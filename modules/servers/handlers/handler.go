package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/modules/servers/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.ServerService
	templates *module.TemplateEngine
}

func NewHandler(service *services.ServerService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	servers, _ := h.service.List(userID)

	module.Render(w, r, h.templates, "servers:list.html", map[string]interface{}{
		"Title":   "Servers",
		"Servers": servers,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "servers:new.html", map[string]interface{}{
		"Title": "Add Server",
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.service.Get(id)
	if err != nil {
		// Render a proper 404 page instead of plain text
		module.Render(w, r, h.templates, "errors:404.html", map[string]interface{}{
			"Title":   "Server Not Found",
			"Message": "The server you're looking for doesn't exist or has been deleted.",
		})
		return
	}

	pubKey, _ := h.service.GetPublicKey(id)

	module.Render(w, r, h.templates, "servers:show.html", map[string]interface{}{
		"Title":     server.Name,
		"Server":    server,
		"PublicKey": pubKey,
	})
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	logs, _ := h.service.GetLogs(id, 100)

	module.Render(w, r, h.templates, "servers:logs.html", map[string]interface{}{
		"Title":  server.Name + " - Logs",
		"Server": server,
		"Logs":   logs,
	})
}

func (h *Handler) Terminal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	module.Render(w, r, h.templates, "servers:terminal.html", map[string]interface{}{
		"Title":  server.Name + " - Terminal",
		"Server": server,
	})
}

// API handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	servers, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"servers": servers})
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CreateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	server, err := h.service.Create(userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Verify the server was actually created
	_, verifyErr := h.service.Get(server.ID)
	if verifyErr != nil {
		h.jsonError(w, "Server created but verification failed: "+verifyErr.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"server": server})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Server not found", http.StatusNotFound)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"server": server})
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.UpdateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	server, err := h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"server": server})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func (h *Handler) APITestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.TestConnection(id); err != nil {
		h.json(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIExec(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.ExecInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	output, err := h.service.Exec(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, output)
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

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/modules/servers/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// ProvisionHandler handles provision-related HTTP requests
type ProvisionHandler struct {
	provisionService *services.ProvisionService
	serverService    *services.ServerService
	templates        *module.TemplateEngine
}

// NewProvisionHandler creates a new provision handler
func NewProvisionHandler(provisionService *services.ProvisionService, serverService *services.ServerService, templates *module.TemplateEngine) *ProvisionHandler {
	return &ProvisionHandler{
		provisionService: provisionService,
		serverService:    serverService,
		templates:        templates,
	}
}

// APIStartProvision starts provisioning for a server
func (h *ProvisionHandler) APIStartProvision(w http.ResponseWriter, r *http.Request) {
	var input models.CreateProvisionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.ServerID == "" {
		h.jsonError(w, "serverId is required", http.StatusBadRequest)
		return
	}
	if input.AcmeDomain == "" {
		h.jsonError(w, "acmeDomain is required", http.StatusBadRequest)
		return
	}

	// Check server exists
	server, err := h.serverService.Get(input.ServerID)
	if err != nil {
		h.jsonError(w, "Server not found", http.StatusNotFound)
		return
	}

	// Check for existing active provision
	existing, _ := h.provisionService.GetByServerID(input.ServerID)
	if existing != nil && existing.Status == models.ProvisionStatusRunning {
		h.jsonError(w, "Server already has an active provision", http.StatusConflict)
		return
	}

	// Create provision record
	provision, err := h.provisionService.Create(input)
	if err != nil {
		h.jsonError(w, fmt.Sprintf("Failed to create provision: %v", err), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"provisionId": provision.ID,
		"serverId":    server.ID,
		"serverName":  server.Name,
	})
}

// APIGetProvision returns provision status
func (h *ProvisionHandler) APIGetProvision(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	provision, err := h.provisionService.Get(id)
	if err != nil {
		h.jsonError(w, "Provision not found", http.StatusNotFound)
		return
	}
	h.json(w, http.StatusOK, provision)
}

// APIStreamProvision streams provision progress via SSE
func (h *ProvisionHandler) APIStreamProvision(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	provision, err := h.provisionService.Get(id)
	if err != nil {
		http.Error(w, "Provision not found", http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// If already completed or failed, send final state and close
	if provision.Status == models.ProvisionStatusCompleted || provision.Status == models.ProvisionStatusFailed {
		h.sendSSE(w, flusher, "step", map[string]interface{}{
			"step":  provision.CurrentStep,
			"total": provision.TotalSteps,
			"name":  provision.StepName,
		})
		if provision.Error != "" {
			h.sendSSE(w, flusher, "error", map[string]interface{}{
				"message": provision.Error,
			})
		}
		h.sendSSE(w, flusher, "complete", map[string]interface{}{
			"success": provision.Status == models.ProvisionStatusCompleted,
		})
		return
	}

	// Create event channel
	eventChan := make(chan services.ProvisionEvent, 100)

	// Create cancellable context
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start provisioning in background if pending
	if provision.Status == models.ProvisionStatusPending {
		go func() {
			h.provisionService.RunProvision(ctx, id, eventChan)
		}()
	} else if provision.Status == models.ProvisionStatusRunning {
		// Already running, just stream current state
		h.sendSSE(w, flusher, "step", map[string]interface{}{
			"step":  provision.CurrentStep,
			"total": provision.TotalSteps,
			"name":  provision.StepName,
		})
	}

	// Stream events to client
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, provisioning complete
				return
			}
			h.sendSSE(w, flusher, event.Type, event.Payload)
		}
	}
}

// sendSSE sends a Server-Sent Event
func (h *ProvisionHandler) sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	flusher.Flush()
}

// ProvisionPage renders the provision progress page
func (h *ProvisionHandler) ProvisionPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	provision, err := h.provisionService.Get(id)
	if err != nil {
		log.Printf("[Provision] Get provision %s failed: %v", id, err)
		module.Render(w, r, h.templates, "errors:404.html", map[string]interface{}{
			"Title":   "Provision Not Found",
			"Message": "The provision job you're looking for doesn't exist.",
		})
		return
	}

	server, err := h.serverService.Get(provision.ServerID)
	if err != nil {
		log.Printf("[Provision] Get server %s for provision %s failed: %v", provision.ServerID, id, err)
		module.Render(w, r, h.templates, "errors:404.html", map[string]interface{}{
			"Title":   "Server Not Found",
			"Message": "The server for this provision doesn't exist.",
		})
		return
	}

	module.Render(w, r, h.templates, "servers:provision.html", map[string]interface{}{
		"Title":     "Provisioning " + server.Name,
		"Server":    server,
		"Provision": provision,
		"Steps":     models.ProvisionSteps,
	})
}

// Helpers

func (h *ProvisionHandler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ProvisionHandler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

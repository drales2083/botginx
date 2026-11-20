package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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

// APIStreamProvision streams provision progress via SSE (polling-based)
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

	// If pending or running, ensure the job is started
	if provision.Status == models.ProvisionStatusPending || provision.Status == models.ProvisionStatusRunning {
		if err := h.provisionService.StartProvision(id); err != nil {
			log.Printf("[Provision] StartProvision error: %v", err)
		}
	}

	// Track what we've already sent
	lastStep := -1
	lastOutputLen := 0
	lastStatus := ""

	// Poll for updates
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Send initial state immediately
	h.sendCurrentState(w, flusher, id, &lastStep, &lastOutputLen, &lastStatus)

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected - job continues in background
			return
		case <-ticker.C:
			done := h.sendCurrentState(w, flusher, id, &lastStep, &lastOutputLen, &lastStatus)
			if done {
				return
			}
		}
	}
}

// sendCurrentState polls DB and sends any new state via SSE
// Returns true if the provision is complete (no more updates expected)
func (h *ProvisionHandler) sendCurrentState(w http.ResponseWriter, flusher http.Flusher, id string, lastStep *int, lastOutputLen *int, lastStatus *string) bool {
	provision, err := h.provisionService.Get(id)
	if err != nil {
		return true // Can't get provision, stop
	}

	// Send step update if changed
	if provision.CurrentStep != *lastStep {
		*lastStep = provision.CurrentStep
		stepName := ""
		if provision.StepName != nil {
			stepName = *provision.StepName
		}
		h.sendSSE(w, flusher, "step", map[string]interface{}{
			"step":  provision.CurrentStep,
			"total": provision.TotalSteps,
			"name":  stepName,
		})
	}

	// Send new output lines
	output, _ := h.provisionService.GetOutput(id)
	if len(output) > *lastOutputLen {
		newOutput := output[*lastOutputLen:]
		*lastOutputLen = len(output)

		// Split into lines and send each
		lines := strings.Split(newOutput, "\n")
		for _, line := range lines {
			if line != "" {
				h.sendSSE(w, flusher, "output", map[string]interface{}{
					"line": line,
				})
			}
		}
	}

	// Check for status changes
	status := string(provision.Status)
	if status != *lastStatus {
		*lastStatus = status

		if provision.Status == models.ProvisionStatusCompleted {
			h.sendSSE(w, flusher, "complete", map[string]interface{}{
				"success": true,
			})
			return true
		}

		if provision.Status == models.ProvisionStatusFailed {
			errMsg := ""
			if provision.Error != nil {
				errMsg = *provision.Error
			}
			h.sendSSE(w, flusher, "error", map[string]interface{}{
				"message": errMsg,
			})
			h.sendSSE(w, flusher, "complete", map[string]interface{}{
				"success": false,
			})
			return true
		}
	}

	return false
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

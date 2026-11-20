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

// ReconfigureHandler handles reconfiguration-related HTTP requests
type ReconfigureHandler struct {
	reconfigureService *services.ReconfigureService
	serverService      *services.ServerService
	templates          *module.TemplateEngine
}

// NewReconfigureHandler creates a new reconfigure handler
func NewReconfigureHandler(reconfigureService *services.ReconfigureService, serverService *services.ServerService, templates *module.TemplateEngine) *ReconfigureHandler {
	return &ReconfigureHandler{
		reconfigureService: reconfigureService,
		serverService:      serverService,
		templates:          templates,
	}
}

// APIStartReconfigure starts reconfiguration for a server
func (h *ReconfigureHandler) APIStartReconfigure(w http.ResponseWriter, r *http.Request) {
	var input models.CreateReconfigureInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.ServerID == "" {
		h.jsonError(w, "serverId is required", http.StatusBadRequest)
		return
	}

	server, err := h.serverService.Get(input.ServerID)
	if err != nil {
		h.jsonError(w, "Server not found", http.StatusNotFound)
		return
	}

	existing, _ := h.reconfigureService.GetByServerID(input.ServerID)
	if existing != nil && existing.Status == models.ReconfigureStatusRunning {
		h.jsonError(w, "Server already has an active reconfiguration", http.StatusConflict)
		return
	}

	reconfig, err := h.reconfigureService.Create(input)
	if err != nil {
		h.jsonError(w, fmt.Sprintf("Failed to create reconfiguration: %v", err), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"reconfigureId": reconfig.ID,
		"serverId":      server.ID,
		"serverName":    server.Name,
	})
}

// APIGetReconfigure returns reconfigure status
func (h *ReconfigureHandler) APIGetReconfigure(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	reconfig, err := h.reconfigureService.Get(id)
	if err != nil {
		h.jsonError(w, "Reconfiguration not found", http.StatusNotFound)
		return
	}
	h.json(w, http.StatusOK, reconfig)
}

// APIStreamReconfigure streams reconfiguration progress via SSE
func (h *ReconfigureHandler) APIStreamReconfigure(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	reconfig, err := h.reconfigureService.Get(id)
	if err != nil {
		http.Error(w, "Reconfiguration not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	if reconfig.Status == models.ReconfigureStatusPending || reconfig.Status == models.ReconfigureStatusRunning {
		if err := h.reconfigureService.StartReconfigure(id); err != nil {
			log.Printf("[Reconfigure] StartReconfigure error: %v", err)
		}
	}

	lastStep := -1
	lastOutputLen := 0
	lastStatus := ""

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	h.sendCurrentState(w, flusher, id, &lastStep, &lastOutputLen, &lastStatus)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			done := h.sendCurrentState(w, flusher, id, &lastStep, &lastOutputLen, &lastStatus)
			if done {
				return
			}
		}
	}
}

func (h *ReconfigureHandler) sendCurrentState(w http.ResponseWriter, flusher http.Flusher, id string, lastStep *int, lastOutputLen *int, lastStatus *string) bool {
	reconfig, err := h.reconfigureService.Get(id)
	if err != nil {
		return true
	}

	if reconfig.CurrentStep != *lastStep {
		*lastStep = reconfig.CurrentStep
		stepName := ""
		if reconfig.StepName != nil {
			stepName = *reconfig.StepName
		}
		h.sendSSE(w, flusher, "step", map[string]interface{}{
			"step":  reconfig.CurrentStep,
			"total": reconfig.TotalSteps,
			"name":  stepName,
		})
	}

	output, _ := h.reconfigureService.GetOutput(id)
	if len(output) > *lastOutputLen {
		newOutput := output[*lastOutputLen:]
		*lastOutputLen = len(output)

		lines := strings.Split(newOutput, "\n")
		for _, line := range lines {
			if line != "" {
				h.sendSSE(w, flusher, "output", map[string]interface{}{
					"line": line,
				})
			}
		}
	}

	status := string(reconfig.Status)
	if status != *lastStatus {
		*lastStatus = status

		if reconfig.Status == models.ReconfigureStatusCompleted {
			h.sendSSE(w, flusher, "complete", map[string]interface{}{
				"success": true,
			})
			return true
		}

		if reconfig.Status == models.ReconfigureStatusFailed {
			errMsg := ""
			if reconfig.Error != nil {
				errMsg = *reconfig.Error
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

func (h *ReconfigureHandler) sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	flusher.Flush()
}

// ReconfigurePage renders the reconfiguration progress page
func (h *ReconfigureHandler) ReconfigurePage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	reconfig, err := h.reconfigureService.Get(id)
	if err != nil {
		log.Printf("[Reconfigure] Get reconfigure %s failed: %v", id, err)
		module.Render(w, r, h.templates, "errors:404.html", map[string]interface{}{
			"Title":   "Reconfiguration Not Found",
			"Message": "The reconfiguration job you're looking for doesn't exist.",
		})
		return
	}

	server, err := h.serverService.Get(reconfig.ServerID)
	if err != nil {
		log.Printf("[Reconfigure] Get server %s for reconfigure %s failed: %v", reconfig.ServerID, id, err)
		module.Render(w, r, h.templates, "errors:404.html", map[string]interface{}{
			"Title":   "Server Not Found",
			"Message": "The server for this reconfiguration doesn't exist.",
		})
		return
	}

	module.Render(w, r, h.templates, "servers:reconfigure.html", map[string]interface{}{
		"Title":       "DNS Reconfiguration - " + server.Name,
		"Server":      server,
		"Reconfigure": reconfig,
		"Steps":       models.ReconfigureSteps,
	})
}

func (h *ReconfigureHandler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ReconfigureHandler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

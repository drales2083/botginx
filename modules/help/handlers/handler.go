package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/help/models"
	"github.com/botginx/botginx/modules/help/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.HelpService
	templates *module.TemplateEngine
}

func NewHandler(service *services.HelpService, templates *module.TemplateEngine) *Handler {
	return &Handler{service: service, templates: templates}
}

// User pages

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.service.GetCategoriesWithItems()

	module.RenderUserSection(w, r, h.templates, "help:index.html", map[string]interface{}{
		"Title":      "Help & FAQ",
		"Categories": categories,
	})
}

// Admin pages

func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.service.ListCategories()
	items, _ := h.service.ListItems()

	module.Render(w, r, h.templates, "help:admin_list.html", map[string]interface{}{
		"Title":      "Manage FAQs",
		"Categories": categories,
		"Items":      items,
	})
}

// API handlers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

// Categories API

func (h *Handler) APIListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.ListCategories()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"categories": categories})
}

func (h *Handler) APICreateCategory(w http.ResponseWriter, r *http.Request) {
	var input models.CreateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		h.jsonError(w, "Name is required", http.StatusBadRequest)
		return
	}

	cat, err := h.service.CreateCategory(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"category": cat})
}

func (h *Handler) APIUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.UpdateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	cat, err := h.service.UpdateCategory(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"category": cat})
}

func (h *Handler) APIDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteCategory(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Items API

func (h *Handler) APIListItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListItems()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (h *Handler) APICreateItem(w http.ResponseWriter, r *http.Request) {
	var input models.CreateFAQInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.CategoryID == "" || input.Question == "" || input.Answer == "" {
		h.jsonError(w, "Category, question and answer are required", http.StatusBadRequest)
		return
	}

	item, err := h.service.CreateItem(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"item": item})
}

func (h *Handler) APIGetItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	item, err := h.service.GetItem(id)
	if err != nil {
		h.jsonError(w, "Item not found", http.StatusNotFound)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"item": item})
}

func (h *Handler) APIUpdateItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input models.UpdateFAQInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	item, err := h.service.UpdateItem(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"item": item})
}

func (h *Handler) APIDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteItem(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

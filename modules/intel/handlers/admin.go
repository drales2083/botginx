package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminHandler struct {
	articles   *services.ArticleService
	categories *services.CategoryService
	tags       *services.TagService
	markdown   *services.MarkdownService
	templates  *module.TemplateEngine
}

func NewAdminHandler(
	articles *services.ArticleService,
	categories *services.CategoryService,
	tags *services.TagService,
	markdown *services.MarkdownService,
	templates *module.TemplateEngine,
) *AdminHandler {
	return &AdminHandler{
		articles:   articles,
		categories: categories,
		tags:       tags,
		markdown:   markdown,
		templates:  templates,
	}
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.listPage)
	r.Get("/new", h.editPage)
	r.Get("/{id}/edit", h.editPage)
	r.Get("/categories", h.categoriesPage)
	r.Get("/tags", h.tagsPage)

	r.Route("/api", func(r chi.Router) {
		r.Get("/articles", h.apiListArticles)
		r.Post("/articles", h.apiCreateArticle)
		r.Put("/articles/{id}", h.apiUpdateArticle)
		r.Delete("/articles/{id}", h.apiDeleteArticle)

		r.Get("/categories", h.apiListCategories)
		r.Post("/categories", h.apiCreateCategory)
		r.Put("/categories/{id}", h.apiUpdateCategory)
		r.Delete("/categories/{id}", h.apiDeleteCategory)

		r.Get("/tags", h.apiListTags)
		r.Post("/tags", h.apiCreateTag)
		r.Delete("/tags/{id}", h.apiDeleteTag)

		r.Post("/upload", h.apiUpload)
		r.Post("/preview", h.apiPreview)
	})

	return r
}

func (h *AdminHandler) json(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": data})
}

func (h *AdminHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": msg})
}

func (h *AdminHandler) listPage(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.categories.List()
	module.Render(w, r, h.templates, "intel:admin/list.html", map[string]interface{}{
		"Title":      "Intel",
		"Categories": categories,
	})
}

func (h *AdminHandler) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	categories, _ := h.categories.List()
	tags, _ := h.tags.List()

	data := map[string]interface{}{
		"Title":      "New Article",
		"Categories": categories,
		"Tags":       tags,
		"Article":    nil,
	}

	if id != "" {
		article, err := h.articles.GetByID(id)
		if err == nil {
			data["Title"] = "Edit Article"
			data["Article"] = article
		}
	}

	module.Render(w, r, h.templates, "intel:admin/edit.html", data)
}

func (h *AdminHandler) categoriesPage(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.categories.List()
	module.Render(w, r, h.templates, "intel:admin/categories.html", map[string]interface{}{
		"Title":      "Categories",
		"Categories": categories,
	})
}

func (h *AdminHandler) tagsPage(w http.ResponseWriter, r *http.Request) {
	tags, _ := h.tags.List()
	module.Render(w, r, h.templates, "intel:admin/tags.html", map[string]interface{}{
		"Title": "Tags",
		"Tags":  tags,
	})
}

func (h *AdminHandler) apiListArticles(w http.ResponseWriter, r *http.Request) {
	opts := services.ListOptions{
		Status:     r.URL.Query().Get("status"),
		CategoryID: r.URL.Query().Get("category"),
		Search:     r.URL.Query().Get("search"),
		Page:       1,
		PerPage:    50,
	}
	result, err := h.articles.List(opts)
	if err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, result)
}

func (h *AdminHandler) apiCreateArticle(w http.ResponseWriter, r *http.Request) {
	var input services.ArticleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid JSON", 400)
		return
	}
	article, err := h.articles.Create(input)
	if err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, article)
}

func (h *AdminHandler) apiUpdateArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input services.ArticleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid JSON", 400)
		return
	}
	if err := h.articles.Update(id, input); err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, map[string]string{"id": id})
}

func (h *AdminHandler) apiDeleteArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.articles.Delete(id); err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, map[string]bool{"deleted": true})
}

func (h *AdminHandler) apiListCategories(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.categories.List()
	h.json(w, categories)
}

func (h *AdminHandler) apiCreateCategory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&input)
	cat, err := h.categories.Create(input.Name, input.Description)
	if err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, cat)
}

func (h *AdminHandler) apiUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SortOrder   int    `json:"sortOrder"`
	}
	json.NewDecoder(r.Body).Decode(&input)
	if err := h.categories.Update(id, input.Name, input.Description, input.SortOrder); err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, map[string]string{"id": id})
}

func (h *AdminHandler) apiDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.categories.Delete(id); err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, map[string]bool{"deleted": true})
}

func (h *AdminHandler) apiListTags(w http.ResponseWriter, r *http.Request) {
	tags, _ := h.tags.List()
	h.json(w, tags)
}

func (h *AdminHandler) apiCreateTag(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&input)
	tag, err := h.tags.Create(input.Name)
	if err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, tag)
}

func (h *AdminHandler) apiDeleteTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.tags.Delete(id); err != nil {
		h.jsonError(w, err.Error(), 500)
		return
	}
	h.json(w, map[string]bool{"deleted": true})
}

func (h *AdminHandler) apiUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)
	file, header, err := r.FormFile("image")
	if err != nil {
		h.jsonError(w, "No file uploaded", 400)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		h.jsonError(w, "Invalid file type", 400)
		return
	}

	now := time.Now()
	dir := fmt.Sprintf("static/uploads/intel/%d/%02d", now.Year(), now.Month())
	os.MkdirAll(dir, 0755)

	filename := uuid.New().String() + ext
	path := filepath.Join(dir, filename)

	dst, err := os.Create(path)
	if err != nil {
		h.jsonError(w, "Failed to save file", 500)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	h.json(w, map[string]string{"url": "/" + path})
}

func (h *AdminHandler) apiPreview(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Markdown string `json:"markdown"`
	}
	json.NewDecoder(r.Body).Decode(&input)
	html := h.markdown.Render(input.Markdown)
	h.json(w, map[string]string{"html": html})
}

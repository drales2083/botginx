package handlers

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// renderPublic is a helper to render public templates
func renderPublic(w http.ResponseWriter, r *http.Request, templates *module.TemplateEngine, name string, data map[string]interface{}) {
	module.RenderPublic(w, r, templates, name, data)
}

type PublicHandler struct {
	content    *services.ContentService
	markdown   *services.MarkdownService
	templates  *module.TemplateEngine
}

func NewPublicHandler(content *services.ContentService, markdown *services.MarkdownService, templates *module.TemplateEngine) *PublicHandler {
	return &PublicHandler{
		content:   content,
		markdown:  markdown,
		templates: templates,
	}
}

func (h *PublicHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{slug}", h.Show)
	r.Get("/category/{slug}", h.Category)
	r.Get("/tag/{tag}", h.Tag)
	return r
}

func (h *PublicHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage := 9
	articles, total := h.content.GetArticles(page, perPage)
	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"Title":       "Intel",
		"Articles":    articles,
		"Categories":  h.content.GetCategories(),
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"BaseURL":     getBaseURL(r),
	}

	renderPublic(w, r, h.templates, "intel:public/list.html", data)
}

func (h *PublicHandler) Show(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	article, found := h.content.GetArticle(slug)
	if !found {
		http.NotFound(w, r)
		return
	}

	// Render markdown to HTML
	htmlContent := h.markdown.Render(article.Content)
	toc := h.content.ExtractTOC(article.Content)

	// Get category
	var category *services.Category
	if article.Category != "" {
		if cat, ok := h.content.GetCategory(article.Category); ok {
			category = &cat
		}
	}

	// Get related articles
	related := h.content.GetRelatedArticles(slug, 3)

	data := map[string]interface{}{
		"Title":       article.Title,
		"Article":     articleToMap(article),
		"Content":     template.HTML(htmlContent),
		"TOC":         toc,
		"Category":    category,
		"CoverImage":  article.CoverImage,
		"PublishedAt": article.Date,
		"Related":     articles2Maps(related),
		"BaseURL":     getBaseURL(r),
	}

	renderPublic(w, r, h.templates, "intel:public/show.html", data)
}

func (h *PublicHandler) Category(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	category, found := h.content.GetCategory(slug)
	if !found {
		http.NotFound(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage := 9
	articles, total := h.content.GetArticlesByCategory(slug, page, perPage)
	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"Title":       category.Name,
		"Category":    category,
		"Articles":    articles,
		"Categories":  h.content.GetCategories(),
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"BaseURL":     getBaseURL(r),
	}

	renderPublic(w, r, h.templates, "intel:public/category.html", data)
}

func (h *PublicHandler) Tag(w http.ResponseWriter, r *http.Request) {
	tag := chi.URLParam(r, "tag")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage := 9
	articles, total := h.content.GetArticlesByTag(tag, page, perPage)
	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"Title":       "Tag: " + tag,
		"Tag":         tag,
		"Articles":    articles,
		"Categories":  h.content.GetCategories(),
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"HasPrev":     page > 1,
		"HasNext":     page < totalPages,
		"BaseURL":     getBaseURL(r),
	}

	renderPublic(w, r, h.templates, "intel:public/tag.html", data)
}

func getBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		}
	}
	return scheme + "://" + r.Host
}

// articleToMap converts Article to map for template compatibility
func articleToMap(a services.Article) map[string]interface{} {
	return map[string]interface{}{
		"Title":        a.Title,
		"Slug":         a.Slug,
		"Excerpt":      a.Excerpt,
		"AuthorName":   a.Author,
		"ReadingTime":  a.ReadingTime,
		"CoverImage":   a.CoverImage,
		"Featured":     a.Featured,
		"CategoryName": a.CategoryName,
		"Tags":         tagsToMaps(a.Tags),
		"UpdatedAt":    a.Date,
	}
}

func tagsToMaps(tags []string) []map[string]string {
	result := make([]map[string]string, len(tags))
	for i, t := range tags {
		result[i] = map[string]string{
			"Name": t,
			"Slug": t,
		}
	}
	return result
}

func articles2Maps(articles []services.Article) []map[string]interface{} {
	result := make([]map[string]interface{}, len(articles))
	for i, a := range articles {
		result[i] = map[string]interface{}{
			"Title":       a.Title,
			"Slug":        a.Slug,
			"Excerpt":     a.Excerpt,
			"CoverImage":  a.CoverImage,
			"ReadingTime": a.ReadingTime,
		}
	}
	return result
}

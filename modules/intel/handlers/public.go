package handlers

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/botginx/botginx/modules/intel/models"
	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type PublicHandler struct {
	articles   *services.ArticleService
	categories *services.CategoryService
	tags       *services.TagService
	markdown   *services.MarkdownService
	templates  *module.TemplateEngine
}

func NewPublicHandler(
	articles *services.ArticleService,
	categories *services.CategoryService,
	tags *services.TagService,
	markdown *services.MarkdownService,
	templates *module.TemplateEngine,
) *PublicHandler {
	return &PublicHandler{
		articles:   articles,
		categories: categories,
		tags:       tags,
		markdown:   markdown,
		templates:  templates,
	}
}

func (h *PublicHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.listPage)
	r.Get("/sitemap.xml", h.sitemap)
	r.Get("/rss", h.rssFeed)
	r.Get("/category/{slug}", h.categoryPage)
	r.Get("/tag/{slug}", h.tagPage)
	r.Get("/{slug}", h.showPage)

	return r
}

func pageParam(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	return page
}

func baseURL(r *http.Request) string {
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (h *PublicHandler) listPage(w http.ResponseWriter, r *http.Request) {
	page := pageParam(r)

	categories, _ := h.categories.List()

	result, err := h.articles.List(services.ListOptions{
		Status:  "published",
		Page:    page,
		PerPage: 9,
	})
	if err != nil {
		http.Error(w, "Failed to load articles", 500)
		return
	}

	module.RenderPublic(w, r, h.templates, "intel:public/list.html", map[string]interface{}{
		"Title":       "Intel",
		"Articles":    result.Articles,
		"Categories":  categories,
		"CurrentPage": page,
		"TotalPages":  result.TotalPages,
		"HasNext":     page < result.TotalPages,
		"HasPrev":     page > 1,
	})
}

func (h *PublicHandler) showPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	article, err := h.articles.GetBySlug(slug)
	if err != nil || article.Status != models.StatusPublished {
		http.NotFound(w, r)
		return
	}

	// Markdown output is sanitized by MarkdownService's policy.
	html := template.HTML(h.markdown.Render(article.Content))
	toc := h.markdown.ExtractTOC(article.Content)

	categoryID := ""
	if article.CategoryID.Valid {
		categoryID = article.CategoryID.String
	}

	related, _ := h.articles.GetRelated(article.ID, categoryID, 3)

	var category *models.Category
	if categoryID != "" {
		category, _ = h.categories.GetByID(categoryID)
	}

	var publishedAt *time.Time
	if article.PublishedAt.Valid {
		publishedAt = &article.PublishedAt.Time
	}

	module.RenderPublic(w, r, h.templates, "intel:public/show.html", map[string]interface{}{
		"Title":       article.Title,
		"Article":     article,
		"Content":     html,
		"TOC":         toc,
		"Related":     related,
		"Category":    category,
		"MetaTitle":   article.MetaTitle,
		"MetaDesc":    article.MetaDescription,
		"CoverImage":  article.CoverImage,
		"PublishedAt": publishedAt,
		"BaseURL":     baseURL(r),
	})
}

func (h *PublicHandler) categoryPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page := pageParam(r)

	category, err := h.categories.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	categories, _ := h.categories.List()

	result, err := h.articles.List(services.ListOptions{
		Status:     "published",
		CategoryID: category.ID,
		Page:       page,
		PerPage:    9,
	})
	if err != nil {
		http.Error(w, "Failed to load articles", 500)
		return
	}

	module.RenderPublic(w, r, h.templates, "intel:public/category.html", map[string]interface{}{
		"Title":       category.Name,
		"Category":    category,
		"Categories":  categories,
		"Articles":    result.Articles,
		"CurrentPage": page,
		"TotalPages":  result.TotalPages,
		"HasNext":     page < result.TotalPages,
		"HasPrev":     page > 1,
	})
}

func (h *PublicHandler) tagPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page := pageParam(r)

	tag, err := h.tags.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	categories, _ := h.categories.List()

	result, err := h.articles.List(services.ListOptions{
		Status:  "published",
		TagID:   tag.ID,
		Page:    page,
		PerPage: 9,
	})
	if err != nil {
		http.Error(w, "Failed to load articles", 500)
		return
	}

	module.RenderPublic(w, r, h.templates, "intel:public/tag.html", map[string]interface{}{
		"Title":       tag.Name,
		"Tag":         tag,
		"Categories":  categories,
		"Articles":    result.Articles,
		"CurrentPage": page,
		"TotalPages":  result.TotalPages,
		"HasNext":     page < result.TotalPages,
		"HasPrev":     page > 1,
	})
}

func (h *PublicHandler) sitemap(w http.ResponseWriter, r *http.Request) {
	type URL struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod,omitempty"`
		ChangeFreq string `xml:"changefreq,omitempty"`
		Priority   string `xml:"priority,omitempty"`
	}
	type URLSet struct {
		XMLName xml.Name `xml:"urlset"`
		XMLNS   string   `xml:"xmlns,attr"`
		URLs    []URL    `xml:"url"`
	}

	base := baseURL(r)

	urls := []URL{
		{Loc: base + "/intel", ChangeFreq: "daily", Priority: "1.0"},
	}

	result, err := h.articles.List(services.ListOptions{
		Status:  "published",
		Page:    1,
		PerPage: 1000,
	})
	if err == nil {
		for _, a := range result.Articles {
			urls = append(urls, URL{
				Loc:        base + "/intel/" + a.Slug,
				LastMod:    a.UpdatedAt.Format("2006-01-02"),
				ChangeFreq: "weekly",
				Priority:   "0.8",
			})
		}
	}

	categories, _ := h.categories.List()
	for _, c := range categories {
		urls = append(urls, URL{
			Loc:        base + "/intel/category/" + c.Slug,
			ChangeFreq: "weekly",
			Priority:   "0.6",
		})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(URLSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}

func (h *PublicHandler) rssFeed(w http.ResponseWriter, r *http.Request) {
	type Item struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		PubDate     string `xml:"pubDate"`
		GUID        string `xml:"guid"`
	}
	type Channel struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		Items       []Item `xml:"item"`
	}
	type RSS struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Channel Channel  `xml:"channel"`
	}

	base := baseURL(r)

	result, err := h.articles.List(services.ListOptions{
		Status:  "published",
		Page:    1,
		PerPage: 20,
	})
	if err != nil {
		http.Error(w, "Failed to load articles", 500)
		return
	}

	items := make([]Item, 0, len(result.Articles))
	for _, a := range result.Articles {
		pubDate := a.CreatedAt
		if a.PublishedAt.Valid {
			pubDate = a.PublishedAt.Time
		}
		items = append(items, Item{
			Title:       a.Title,
			Link:        base + "/intel/" + a.Slug,
			Description: a.Excerpt,
			PubDate:     pubDate.Format(time.RFC1123Z),
			GUID:        base + "/intel/" + a.Slug,
		})
	}

	w.Header().Set("Content-Type", "application/rss+xml")
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(RSS{
		Version: "2.0",
		Channel: Channel{
			Title:       "Intel",
			Link:        base + "/intel",
			Description: "Latest articles from Intel",
			Items:       items,
		},
	})
}

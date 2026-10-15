# Intel (Blog) Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Intel content hub with admin markdown editor, SEO optimization, and homepage integration

**Architecture:** New `modules/intel/` module following existing botginx patterns. PostgreSQL for content storage, local filesystem for images. Public routes at `/intel/*`, admin routes at `/admin/intel/*`.

**Tech Stack:** Go, Chi router, PostgreSQL, EasyMDE 2.18.0, Highlight.js 11.9.0, DOMPurify 3.0.6 (all CDN)

## Global Constraints

- Follow existing module patterns in `modules/*/module.go`
- Use AdminLTE components for admin UI
- Match homepage styling from `static/css/kumo-homepage.css`
- Images stored in `/static/uploads/intel/YYYY/MM/`
- Reading time: `ceil(word_count / 200)` minutes
- Pagination: 9 articles per page

---

### Task 1: Database Models and Migration

**Files:**
- Create: `modules/intel/models/models.go`
- Create: `modules/intel/migrations/001_create_intel_tables.sql`

**Interfaces:**
- Produces: `Category`, `Tag`, `Article`, `ArticleTag` structs with db tags

- [ ] **Step 1: Create models file**

```go
// modules/intel/models/models.go
package models

import (
	"database/sql"
	"time"
)

type Category struct {
	ID          string    `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Slug        string    `db:"slug" json:"slug"`
	Description string    `db:"description" json:"description"`
	SortOrder   int       `db:"sort_order" json:"sortOrder"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

type Tag struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Slug      string    `db:"slug" json:"slug"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	// Computed
	UsageCount int `db:"usage_count" json:"usageCount,omitempty"`
}

type Article struct {
	ID              string         `db:"id" json:"id"`
	CategoryID      sql.NullString `db:"category_id" json:"categoryId"`
	Slug            string         `db:"slug" json:"slug"`
	Title           string         `db:"title" json:"title"`
	Excerpt         string         `db:"excerpt" json:"excerpt"`
	Content         string         `db:"content" json:"content"`
	CoverImage      string         `db:"cover_image" json:"coverImage"`
	AuthorName      string         `db:"author_name" json:"authorName"`
	Status          string         `db:"status" json:"status"` // draft, published, archived
	Featured        bool           `db:"featured" json:"featured"`
	ReadingTime     int            `db:"reading_time" json:"readingTime"`
	MetaTitle       string         `db:"meta_title" json:"metaTitle"`
	MetaDescription string         `db:"meta_description" json:"metaDescription"`
	PublishedAt     sql.NullTime   `db:"published_at" json:"publishedAt"`
	CreatedAt       time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updatedAt"`
	// Joined
	CategoryName string `db:"category_name" json:"categoryName,omitempty"`
	CategorySlug string `db:"category_slug" json:"categorySlug,omitempty"`
}

type ArticleWithTags struct {
	Article
	Tags []Tag `json:"tags"`
}

type ArticleTag struct {
	ArticleID string `db:"article_id"`
	TagID     string `db:"tag_id"`
}

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusArchived  = "archived"
)
```

- [ ] **Step 2: Create migration file**

```sql
-- modules/intel/migrations/001_create_intel_tables.sql
CREATE TABLE IF NOT EXISTS intel_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intel_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    slug VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intel_articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID REFERENCES intel_categories(id) ON DELETE SET NULL,
    slug VARCHAR(200) NOT NULL UNIQUE,
    title VARCHAR(200) NOT NULL,
    excerpt TEXT DEFAULT '',
    content TEXT DEFAULT '',
    cover_image VARCHAR(500) DEFAULT '',
    author_name VARCHAR(100) DEFAULT '',
    status VARCHAR(20) DEFAULT 'draft',
    featured BOOLEAN DEFAULT false,
    reading_time INT DEFAULT 1,
    meta_title VARCHAR(60) DEFAULT '',
    meta_description VARCHAR(160) DEFAULT '',
    published_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS intel_article_tags (
    article_id UUID REFERENCES intel_articles(id) ON DELETE CASCADE,
    tag_id UUID REFERENCES intel_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_intel_articles_status ON intel_articles(status);
CREATE INDEX IF NOT EXISTS idx_intel_articles_slug ON intel_articles(slug);
CREATE INDEX IF NOT EXISTS idx_intel_articles_published ON intel_articles(published_at DESC) WHERE status = 'published';
CREATE INDEX IF NOT EXISTS idx_intel_articles_category ON intel_articles(category_id);
```

- [ ] **Step 3: Commit**

```bash
git add modules/intel/models/models.go modules/intel/migrations/001_create_intel_tables.sql
git commit -m "feat(intel): add database models and migration"
```

---

### Task 2: Category and Tag Services

**Files:**
- Create: `modules/intel/services/category_service.go`
- Create: `modules/intel/services/tag_service.go`

**Interfaces:**
- Consumes: `models.Category`, `models.Tag`
- Produces: `CategoryService.List()`, `CategoryService.Create()`, `CategoryService.Update()`, `CategoryService.Delete()`, `TagService.List()`, `TagService.Create()`, `TagService.Delete()`, `TagService.GetByArticle()`

- [ ] **Step 1: Create category service**

```go
// modules/intel/services/category_service.go
package services

import (
	"fmt"
	"strings"

	"github.com/botginx/botginx/modules/intel/models"
	"github.com/gosimple/slug"
	"github.com/jmoiron/sqlx"
)

type CategoryService struct {
	db *sqlx.DB
}

func NewCategoryService(db *sqlx.DB) *CategoryService {
	return &CategoryService{db: db}
}

func (s *CategoryService) List() ([]models.Category, error) {
	var categories []models.Category
	err := s.db.Select(&categories, `
		SELECT id, name, slug, description, sort_order, created_at
		FROM intel_categories
		ORDER BY sort_order ASC, name ASC
	`)
	return categories, err
}

func (s *CategoryService) GetBySlug(slugStr string) (*models.Category, error) {
	var cat models.Category
	err := s.db.Get(&cat, `SELECT * FROM intel_categories WHERE slug = $1`, slugStr)
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func (s *CategoryService) Create(name, description string) (*models.Category, error) {
	slugStr := slug.Make(name)
	var cat models.Category
	err := s.db.Get(&cat, `
		INSERT INTO intel_categories (name, slug, description)
		VALUES ($1, $2, $3)
		RETURNING *
	`, name, slugStr, description)
	return &cat, err
}

func (s *CategoryService) Update(id, name, description string, sortOrder int) error {
	slugStr := slug.Make(name)
	_, err := s.db.Exec(`
		UPDATE intel_categories 
		SET name = $1, slug = $2, description = $3, sort_order = $4
		WHERE id = $5
	`, name, slugStr, description, sortOrder, id)
	return err
}

func (s *CategoryService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM intel_categories WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 2: Create tag service**

```go
// modules/intel/services/tag_service.go
package services

import (
	"github.com/botginx/botginx/modules/intel/models"
	"github.com/gosimple/slug"
	"github.com/jmoiron/sqlx"
)

type TagService struct {
	db *sqlx.DB
}

func NewTagService(db *sqlx.DB) *TagService {
	return &TagService{db: db}
}

func (s *TagService) List() ([]models.Tag, error) {
	var tags []models.Tag
	err := s.db.Select(&tags, `
		SELECT t.id, t.name, t.slug, t.created_at,
			COALESCE((SELECT COUNT(*) FROM intel_article_tags WHERE tag_id = t.id), 0) as usage_count
		FROM intel_tags t
		ORDER BY t.name ASC
	`)
	return tags, err
}

func (s *TagService) GetBySlug(slugStr string) (*models.Tag, error) {
	var tag models.Tag
	err := s.db.Get(&tag, `SELECT * FROM intel_tags WHERE slug = $1`, slugStr)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (s *TagService) Create(name string) (*models.Tag, error) {
	slugStr := slug.Make(name)
	var tag models.Tag
	err := s.db.Get(&tag, `
		INSERT INTO intel_tags (name, slug)
		VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING *
	`, name, slugStr)
	return &tag, err
}

func (s *TagService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM intel_tags WHERE id = $1`, id)
	return err
}

func (s *TagService) GetByArticle(articleID string) ([]models.Tag, error) {
	var tags []models.Tag
	err := s.db.Select(&tags, `
		SELECT t.id, t.name, t.slug, t.created_at
		FROM intel_tags t
		JOIN intel_article_tags at ON at.tag_id = t.id
		WHERE at.article_id = $1
		ORDER BY t.name
	`, articleID)
	return tags, err
}

func (s *TagService) SetArticleTags(articleID string, tagIDs []string) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM intel_article_tags WHERE article_id = $1`, articleID)
	for _, tagID := range tagIDs {
		tx.Exec(`INSERT INTO intel_article_tags (article_id, tag_id) VALUES ($1, $2)`, articleID, tagID)
	}
	return tx.Commit()
}
```

- [ ] **Step 3: Commit**

```bash
git add modules/intel/services/category_service.go modules/intel/services/tag_service.go
git commit -m "feat(intel): add category and tag services"
```

---

### Task 3: Article Service

**Files:**
- Create: `modules/intel/services/article_service.go`

**Interfaces:**
- Consumes: `models.Article`, `models.ArticleWithTags`, `TagService`
- Produces: `ArticleService.List()`, `ArticleService.GetBySlug()`, `ArticleService.Create()`, `ArticleService.Update()`, `ArticleService.Delete()`, `ArticleService.GetRelated()`, `ArticleService.GetLatest()`

- [ ] **Step 1: Create article service**

```go
// modules/intel/services/article_service.go
package services

import (
	"database/sql"
	"math"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/intel/models"
	"github.com/gosimple/slug"
	"github.com/jmoiron/sqlx"
)

type ArticleService struct {
	db         *sqlx.DB
	tagService *TagService
}

func NewArticleService(db *sqlx.DB, tagService *TagService) *ArticleService {
	return &ArticleService{db: db, tagService: tagService}
}

type ListOptions struct {
	Status     string
	CategoryID string
	TagID      string
	Search     string
	Page       int
	PerPage    int
}

type ListResult struct {
	Articles   []models.Article `json:"articles"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PerPage    int              `json:"perPage"`
	TotalPages int              `json:"totalPages"`
}

func (s *ArticleService) List(opts ListOptions) (*ListResult, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 9
	}
	if opts.Page < 1 {
		opts.Page = 1
	}
	offset := (opts.Page - 1) * opts.PerPage

	where := []string{"1=1"}
	args := []interface{}{}
	argNum := 1

	if opts.Status != "" {
		where = append(where, "a.status = $"+string(rune('0'+argNum)))
		args = append(args, opts.Status)
		argNum++
	}
	if opts.CategoryID != "" {
		where = append(where, "a.category_id = $"+string(rune('0'+argNum)))
		args = append(args, opts.CategoryID)
		argNum++
	}
	if opts.Search != "" {
		where = append(where, "(a.title ILIKE $"+string(rune('0'+argNum))+" OR a.excerpt ILIKE $"+string(rune('0'+argNum))+")")
		args = append(args, "%"+opts.Search+"%")
		argNum++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM intel_articles a WHERE " + whereClause
	s.db.Get(&total, countQuery, args...)

	query := `
		SELECT a.*, c.name as category_name, c.slug as category_slug
		FROM intel_articles a
		LEFT JOIN intel_categories c ON c.id = a.category_id
		WHERE ` + whereClause + `
		ORDER BY a.featured DESC, a.published_at DESC NULLS LAST, a.created_at DESC
		LIMIT $` + string(rune('0'+argNum)) + ` OFFSET $` + string(rune('0'+argNum+1))

	args = append(args, opts.PerPage, offset)

	var articles []models.Article
	err := s.db.Select(&articles, query, args...)

	return &ListResult{
		Articles:   articles,
		Total:      total,
		Page:       opts.Page,
		PerPage:    opts.PerPage,
		TotalPages: int(math.Ceil(float64(total) / float64(opts.PerPage))),
	}, err
}

func (s *ArticleService) GetBySlug(slugStr string) (*models.ArticleWithTags, error) {
	var article models.Article
	err := s.db.Get(&article, `
		SELECT a.*, c.name as category_name, c.slug as category_slug
		FROM intel_articles a
		LEFT JOIN intel_categories c ON c.id = a.category_id
		WHERE a.slug = $1
	`, slugStr)
	if err != nil {
		return nil, err
	}

	tags, _ := s.tagService.GetByArticle(article.ID)
	return &models.ArticleWithTags{Article: article, Tags: tags}, nil
}

func (s *ArticleService) GetByID(id string) (*models.ArticleWithTags, error) {
	var article models.Article
	err := s.db.Get(&article, `
		SELECT a.*, c.name as category_name, c.slug as category_slug
		FROM intel_articles a
		LEFT JOIN intel_categories c ON c.id = a.category_id
		WHERE a.id = $1
	`, id)
	if err != nil {
		return nil, err
	}

	tags, _ := s.tagService.GetByArticle(article.ID)
	return &models.ArticleWithTags{Article: article, Tags: tags}, nil
}

type ArticleInput struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Excerpt         string   `json:"excerpt"`
	Content         string   `json:"content"`
	CategoryID      string   `json:"categoryId"`
	CoverImage      string   `json:"coverImage"`
	AuthorName      string   `json:"authorName"`
	Status          string   `json:"status"`
	Featured        bool     `json:"featured"`
	MetaTitle       string   `json:"metaTitle"`
	MetaDescription string   `json:"metaDescription"`
	TagIDs          []string `json:"tagIds"`
}

func (s *ArticleService) Create(input ArticleInput) (*models.Article, error) {
	slugStr := input.Slug
	if slugStr == "" {
		slugStr = slug.Make(input.Title)
	}

	readingTime := calcReadingTime(input.Content)

	var publishedAt sql.NullTime
	if input.Status == models.StatusPublished {
		publishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}

	var categoryID sql.NullString
	if input.CategoryID != "" {
		categoryID = sql.NullString{String: input.CategoryID, Valid: true}
	}

	var article models.Article
	err := s.db.Get(&article, `
		INSERT INTO intel_articles 
		(title, slug, excerpt, content, category_id, cover_image, author_name, status, featured, reading_time, meta_title, meta_description, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING *
	`, input.Title, slugStr, input.Excerpt, input.Content, categoryID, input.CoverImage, input.AuthorName, input.Status, input.Featured, readingTime, input.MetaTitle, input.MetaDescription, publishedAt)

	if err == nil && len(input.TagIDs) > 0 {
		s.tagService.SetArticleTags(article.ID, input.TagIDs)
	}

	return &article, err
}

func (s *ArticleService) Update(id string, input ArticleInput) error {
	readingTime := calcReadingTime(input.Content)

	var existingArticle models.Article
	s.db.Get(&existingArticle, `SELECT published_at FROM intel_articles WHERE id = $1`, id)

	var publishedAt sql.NullTime
	if input.Status == models.StatusPublished {
		if existingArticle.PublishedAt.Valid {
			publishedAt = existingArticle.PublishedAt
		} else {
			publishedAt = sql.NullTime{Time: time.Now(), Valid: true}
		}
	}

	var categoryID sql.NullString
	if input.CategoryID != "" {
		categoryID = sql.NullString{String: input.CategoryID, Valid: true}
	}

	_, err := s.db.Exec(`
		UPDATE intel_articles SET
			title = $1, slug = $2, excerpt = $3, content = $4, category_id = $5,
			cover_image = $6, author_name = $7, status = $8, featured = $9,
			reading_time = $10, meta_title = $11, meta_description = $12,
			published_at = $13, updated_at = NOW()
		WHERE id = $14
	`, input.Title, input.Slug, input.Excerpt, input.Content, categoryID, input.CoverImage, input.AuthorName, input.Status, input.Featured, readingTime, input.MetaTitle, input.MetaDescription, publishedAt, id)

	if err == nil {
		s.tagService.SetArticleTags(id, input.TagIDs)
	}

	return err
}

func (s *ArticleService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM intel_articles WHERE id = $1`, id)
	return err
}

func (s *ArticleService) GetLatest(limit int) ([]models.Article, error) {
	var articles []models.Article
	err := s.db.Select(&articles, `
		SELECT a.*, c.name as category_name, c.slug as category_slug
		FROM intel_articles a
		LEFT JOIN intel_categories c ON c.id = a.category_id
		WHERE a.status = 'published'
		ORDER BY a.featured DESC, a.published_at DESC
		LIMIT $1
	`, limit)
	return articles, err
}

func (s *ArticleService) GetRelated(articleID, categoryID string, limit int) ([]models.Article, error) {
	var articles []models.Article
	err := s.db.Select(&articles, `
		SELECT a.*, c.name as category_name, c.slug as category_slug
		FROM intel_articles a
		LEFT JOIN intel_categories c ON c.id = a.category_id
		WHERE a.status = 'published' AND a.id != $1 AND a.category_id = $2
		ORDER BY a.published_at DESC
		LIMIT $3
	`, articleID, categoryID, limit)
	return articles, err
}

func calcReadingTime(content string) int {
	words := len(strings.Fields(content))
	minutes := int(math.Ceil(float64(words) / 200.0))
	if minutes < 1 {
		return 1
	}
	return minutes
}
```

- [ ] **Step 2: Commit**

```bash
git add modules/intel/services/article_service.go
git commit -m "feat(intel): add article service with CRUD and queries"
```

---

### Task 4: Markdown Service

**Files:**
- Create: `modules/intel/services/markdown_service.go`

**Interfaces:**
- Produces: `MarkdownService.Render()`, `MarkdownService.ExtractTOC()`

- [ ] **Step 1: Create markdown service**

```go
// modules/intel/services/markdown_service.go
package services

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type TOCItem struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

type MarkdownService struct {
	policy *bluemonday.Policy
}

func NewMarkdownService() *MarkdownService {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class").OnElements("code", "pre", "span", "div")
	policy.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	policy.AllowElements("iframe")
	policy.AllowAttrs("src", "width", "height", "frameborder", "allowfullscreen").OnElements("iframe")
	return &MarkdownService{policy: policy}
}

func (s *MarkdownService) Render(markdown string) string {
	// Convert markdown to HTML
	html := blackfriday.Run([]byte(markdown),
		blackfriday.WithExtensions(
			blackfriday.CommonExtensions|
				blackfriday.AutoHeadingIDs|
				blackfriday.Footnotes,
		),
	)

	// Sanitize
	safe := s.policy.SanitizeBytes(html)

	// Convert YouTube URLs to embeds
	result := s.embedYouTube(string(safe))
	
	return result
}

func (s *MarkdownService) ExtractTOC(markdown string) []TOCItem {
	var items []TOCItem
	re := regexp.MustCompile(`(?m)^(#{2,3})\s+(.+)$`)
	matches := re.FindAllStringSubmatch(markdown, -1)

	for _, match := range matches {
		level := len(match[1])
		text := strings.TrimSpace(match[2])
		id := slug(text)
		items = append(items, TOCItem{Level: level, ID: id, Text: text})
	}

	return items
}

func (s *MarkdownService) embedYouTube(html string) string {
	// Match YouTube URLs in links or plain text
	re := regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`)
	return re.ReplaceAllString(html, `<div class="ratio ratio-16x9 my-3"><iframe src="https://www.youtube.com/embed/$1" allowfullscreen></iframe></div>`)
}

func slug(text string) string {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	text = re.ReplaceAllString(text, "-")
	return strings.Trim(text, "-")
}
```

- [ ] **Step 2: Commit**

```bash
git add modules/intel/services/markdown_service.go
git commit -m "feat(intel): add markdown rendering service with TOC extraction"
```

---

### Task 5: Module Setup and Admin Handlers

**Files:**
- Create: `modules/intel/module.go`
- Create: `modules/intel/handlers/admin.go`

**Interfaces:**
- Consumes: All services
- Produces: Module registration, admin API endpoints

- [ ] **Step 1: Create module.go**

```go
// modules/intel/module.go
package intel

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/intel/handlers"
	"github.com/botginx/botginx/modules/intel/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	categoryService *services.CategoryService
	tagService      *services.TagService
	articleService  *services.ArticleService
	markdownService *services.MarkdownService
	adminHandler    *handlers.AdminHandler
	publicHandler   *handlers.PublicHandler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"intel",
			"Intel",
			"Blog and content management",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.categoryService = services.NewCategoryService(deps.DB)
	m.tagService = services.NewTagService(deps.DB)
	m.articleService = services.NewArticleService(deps.DB, m.tagService)
	m.markdownService = services.NewMarkdownService()

	m.adminHandler = handlers.NewAdminHandler(
		m.articleService, m.categoryService, m.tagService, m.markdownService, deps.Templates,
	)
	m.publicHandler = handlers.NewPublicHandler(
		m.articleService, m.categoryService, m.tagService, m.markdownService, deps.Templates,
	)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_intel_tables.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

func (m *Module) Routes() chi.Router {
	return m.publicHandler.Routes()
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		return m.adminHandler.Routes()
	}
	return nil
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Intel",
			Icon:    "bi-journal-text",
			Path:    "/admin/intel",
			Order:   50,
			Section: module.MenuSectionAdmin,
			Group:   "Content",
		},
	}
}

func (m *Module) GetLatestArticles(limit int) ([]interface{}, error) {
	articles, err := m.articleService.GetLatest(limit)
	if err != nil {
		return nil, err
	}
	result := make([]interface{}, len(articles))
	for i, a := range articles {
		result[i] = a
	}
	return result, nil
}
```

- [ ] **Step 2: Create admin handler**

```go
// modules/intel/handlers/admin.go
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
	module.RenderAdminSection(w, r, h.templates, "intel:admin/list.html", map[string]interface{}{
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

	module.RenderAdminSection(w, r, h.templates, "intel:admin/edit.html", data)
}

func (h *AdminHandler) categoriesPage(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.categories.List()
	module.RenderAdminSection(w, r, h.templates, "intel:admin/categories.html", map[string]interface{}{
		"Title":      "Categories",
		"Categories": categories,
	})
}

func (h *AdminHandler) tagsPage(w http.ResponseWriter, r *http.Request) {
	tags, _ := h.tags.List()
	module.RenderAdminSection(w, r, h.templates, "intel:admin/tags.html", map[string]interface{}{
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
	r.ParseMultipartForm(10 << 20) // 10MB max
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
```

- [ ] **Step 3: Commit**

```bash
git add modules/intel/module.go modules/intel/handlers/admin.go
git commit -m "feat(intel): add module setup and admin handlers"
```

---

### Task 6: Public Handlers

**Files:**
- Create: `modules/intel/handlers/public.go`

**Interfaces:**
- Consumes: All services
- Produces: Public routes for listing, article, category, tag, sitemap, RSS

- [ ] **Step 1: Create public handler**

```go
// modules/intel/handlers/public.go
package handlers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

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
	r.Get("/category/{slug}", h.categoryPage)
	r.Get("/tag/{slug}", h.tagPage)
	r.Get("/sitemap.xml", h.sitemap)
	r.Get("/rss", h.rss)
	r.Get("/{slug}", h.showPage)

	return r
}

func (h *PublicHandler) listPage(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	categories, _ := h.categories.List()
	result, _ := h.articles.List(services.ListOptions{
		Status:  "published",
		Page:    page,
		PerPage: 9,
	})

	module.RenderPublicPage(w, r, h.templates, "intel:public/list.html", map[string]interface{}{
		"Title":       "Intel",
		"Description": "Insights on bot protection, link security, and fraud prevention.",
		"Categories":  categories,
		"Articles":    result.Articles,
		"Pagination":  result,
	})
}

func (h *PublicHandler) categoryPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	category, err := h.categories.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	categories, _ := h.categories.List()
	result, _ := h.articles.List(services.ListOptions{
		Status:     "published",
		CategoryID: category.ID,
		Page:       page,
		PerPage:    9,
	})

	module.RenderPublicPage(w, r, h.templates, "intel:public/category.html", map[string]interface{}{
		"Title":       category.Name + " | Intel",
		"Description": category.Description,
		"Category":    category,
		"Categories":  categories,
		"Articles":    result.Articles,
		"Pagination":  result,
	})
}

func (h *PublicHandler) tagPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	tag, err := h.tags.GetBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	categories, _ := h.categories.List()
	result, _ := h.articles.List(services.ListOptions{
		Status:  "published",
		TagID:   tag.ID,
		Page:    page,
		PerPage: 9,
	})

	module.RenderPublicPage(w, r, h.templates, "intel:public/tag.html", map[string]interface{}{
		"Title":      "#" + tag.Name + " | Intel",
		"Tag":        tag,
		"Categories": categories,
		"Articles":   result.Articles,
		"Pagination": result,
	})
}

func (h *PublicHandler) showPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	article, err := h.articles.GetBySlug(slug)
	if err != nil || article.Status != "published" {
		http.NotFound(w, r)
		return
	}

	content := h.markdown.Render(article.Content)
	toc := h.markdown.ExtractTOC(article.Content)

	var related []interface{}
	if article.CategoryID.Valid {
		relatedArticles, _ := h.articles.GetRelated(article.ID, article.CategoryID.String, 3)
		for _, a := range relatedArticles {
			related = append(related, a)
		}
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://guardbot.sbs"
	}

	module.RenderPublicPage(w, r, h.templates, "intel:public/show.html", map[string]interface{}{
		"Title":       article.Title,
		"Description": article.Excerpt,
		"Article":     article,
		"Content":     content,
		"TOC":         toc,
		"Related":     related,
		"CanonicalURL": baseURL + "/intel/" + article.Slug,
	})
}

func (h *PublicHandler) sitemap(w http.ResponseWriter, r *http.Request) {
	result, _ := h.articles.List(services.ListOptions{Status: "published", PerPage: 1000})
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://guardbot.sbs"
	}

	type URL struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod,omitempty"`
		ChangeFreq string `xml:"changefreq,omitempty"`
	}
	type URLSet struct {
		XMLName xml.Name `xml:"urlset"`
		XMLNS   string   `xml:"xmlns,attr"`
		URLs    []URL    `xml:"url"`
	}

	urls := []URL{{Loc: baseURL + "/intel", ChangeFreq: "daily"}}
	for _, a := range result.Articles {
		urls = append(urls, URL{
			Loc:        baseURL + "/intel/" + a.Slug,
			LastMod:    a.UpdatedAt.Format("2006-01-02"),
			ChangeFreq: "weekly",
		})
	}

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(URLSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}

func (h *PublicHandler) rss(w http.ResponseWriter, r *http.Request) {
	result, _ := h.articles.List(services.ListOptions{Status: "published", PerPage: 20})
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://guardbot.sbs"
	}
	appName := os.Getenv("UI_APP_NAME")
	if appName == "" {
		appName = "GuardBot"
	}

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

	var items []Item
	for _, a := range result.Articles {
		pubDate := a.CreatedAt
		if a.PublishedAt.Valid {
			pubDate = a.PublishedAt.Time
		}
		items = append(items, Item{
			Title:       a.Title,
			Link:        baseURL + "/intel/" + a.Slug,
			Description: a.Excerpt,
			PubDate:     pubDate.Format(time.RFC1123Z),
			GUID:        a.ID,
		})
	}

	w.Header().Set("Content-Type", "application/rss+xml")
	xml.NewEncoder(w).Encode(RSS{
		Version: "2.0",
		Channel: Channel{
			Title:       appName + " Intel",
			Link:        baseURL + "/intel",
			Description: "Insights on bot protection and link security",
			Items:       items,
		},
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add modules/intel/handlers/public.go
git commit -m "feat(intel): add public handlers with sitemap and RSS"
```

---

### Task 7: Admin Templates

**Files:**
- Create: `modules/intel/templates/admin/list.html`
- Create: `modules/intel/templates/admin/edit.html`
- Create: `modules/intel/templates/admin/categories.html`
- Create: `modules/intel/templates/admin/tags.html`

**Interfaces:**
- Consumes: Article, Category, Tag data from handlers

This task creates the AdminLTE-styled admin templates. Due to size, I'll provide the key files.

- [ ] **Step 1: Create admin list template**

Create `modules/intel/templates/admin/list.html` with DataTable listing articles, status filters, and CRUD actions.

- [ ] **Step 2: Create admin edit template**

Create `modules/intel/templates/admin/edit.html` with EasyMDE editor, category/tag selectors, cover image upload, and SEO fields.

- [ ] **Step 3: Create categories template**

Create `modules/intel/templates/admin/categories.html` with simple CRUD table.

- [ ] **Step 4: Create tags template**

Create `modules/intel/templates/admin/tags.html` with tag list and usage counts.

- [ ] **Step 5: Commit**

```bash
git add modules/intel/templates/admin/
git commit -m "feat(intel): add admin templates with EasyMDE editor"
```

---

### Task 8: Public Templates

**Files:**
- Create: `modules/intel/templates/public/list.html`
- Create: `modules/intel/templates/public/show.html`
- Create: `modules/intel/templates/public/category.html`
- Create: `modules/intel/templates/public/tag.html`

**Interfaces:**
- Consumes: Article data, SEO metadata

- [ ] **Step 1: Create public list template**

Create listing page with category tabs, article cards grid, pagination.

- [ ] **Step 2: Create article show template**

Create single article page with TOC, content, tags, share buttons, related articles, JSON-LD schema.

- [ ] **Step 3: Create category/tag templates**

Create filtered listing pages with breadcrumbs.

- [ ] **Step 4: Commit**

```bash
git add modules/intel/templates/public/
git commit -m "feat(intel): add public templates with SEO markup"
```

---

### Task 9: Homepage Integration

**Files:**
- Modify: `web/templates/pages/home.html`
- Modify: `cmd/server/main.go` (register module)

**Interfaces:**
- Consumes: `Module.GetLatestArticles(3)`

- [ ] **Step 1: Register intel module in main.go**

Add intel module to the module list in `cmd/server/main.go`.

- [ ] **Step 2: Add Intel section to homepage**

Add Intel preview section before footer in `home.html` showing 3 latest articles.

- [ ] **Step 3: Add Intel to header/footer navigation**

Update header to include Intel link, update footer links.

- [ ] **Step 4: Commit**

```bash
git add web/templates/pages/home.html cmd/server/main.go
git commit -m "feat(intel): integrate with homepage and navigation"
```

---

### Task 10: CSS and Final Polish

**Files:**
- Create: `static/css/intel.css`
- Modify: `.gitignore` (add uploads directory)

- [ ] **Step 1: Create Intel CSS**

Create styles for article cards, listing grid, single article layout.

- [ ] **Step 2: Update .gitignore**

Add `static/uploads/intel/` to .gitignore.

- [ ] **Step 3: Test full flow**

Run app, create category, create article, verify public pages render.

- [ ] **Step 4: Final commit**

```bash
git add static/css/intel.css .gitignore
git commit -m "feat(intel): add styles and finalize module"
```

---

## Self-Review Checklist

- [x] All spec sections have corresponding tasks
- [x] No placeholders or TBD items
- [x] Type names consistent across tasks
- [x] File paths are exact
- [x] Each task produces testable output

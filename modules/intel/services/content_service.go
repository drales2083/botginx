package services

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Article represents a blog post loaded from markdown
type Article struct {
	Slug         string    `yaml:"slug"`
	Title        string    `yaml:"title"`
	Excerpt      string    `yaml:"excerpt"`
	Content      string    `yaml:"-"` // Markdown content (not from frontmatter)
	Author       string    `yaml:"author"`
	Category     string    `yaml:"category"`
	Tags         []string  `yaml:"tags"`
	Date         time.Time `yaml:"date"`
	Featured     bool      `yaml:"featured"`
	CoverImage   string    `yaml:"cover_image"`
	ReadingTime  int       `yaml:"-"` // Calculated
	CategoryName string    `yaml:"-"` // Populated from config
}

// Category represents a content category
type Category struct {
	Slug        string `yaml:"slug"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Config holds the intel content configuration
type Config struct {
	Categories []Category `yaml:"categories"`
}

// ContentService reads articles from markdown files
type ContentService struct {
	contentDir string
	config     Config
	articles   []Article
	categories map[string]Category
}

// NewContentService creates a new file-based content service
func NewContentService(contentDir string) (*ContentService, error) {
	s := &ContentService{
		contentDir: contentDir,
		categories: make(map[string]Category),
	}

	if err := s.loadConfig(); err != nil {
		return nil, err
	}

	if err := s.loadArticles(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *ContentService) loadConfig() error {
	configPath := filepath.Join(s.contentDir, "_config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config is OK
		}
		return err
	}

	if err := yaml.Unmarshal(data, &s.config); err != nil {
		return err
	}

	for _, cat := range s.config.Categories {
		s.categories[cat.Slug] = cat
	}

	return nil
}

func (s *ContentService) loadArticles() error {
	files, err := filepath.Glob(filepath.Join(s.contentDir, "*.md"))
	if err != nil {
		return err
	}

	s.articles = nil
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), "_") {
			continue // Skip files starting with _
		}

		article, err := s.loadArticle(file)
		if err != nil {
			continue // Skip invalid files
		}

		s.articles = append(s.articles, article)
	}

	// Sort by date descending
	sort.Slice(s.articles, func(i, j int) bool {
		return s.articles[i].Date.After(s.articles[j].Date)
	})

	return nil
}

func (s *ContentService) loadArticle(path string) (Article, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Article{}, err
	}

	content := string(data)

	// Parse frontmatter
	var article Article
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			if err := yaml.Unmarshal([]byte(parts[1]), &article); err != nil {
				return Article{}, err
			}
			article.Content = strings.TrimSpace(parts[2])
		}
	}

	// Default slug from filename
	if article.Slug == "" {
		article.Slug = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	// Calculate reading time (avg 200 words per minute)
	words := len(strings.Fields(article.Content))
	article.ReadingTime = int(math.Ceil(float64(words) / 200.0))
	if article.ReadingTime < 1 {
		article.ReadingTime = 1
	}

	// Set category name from config
	if cat, ok := s.categories[article.Category]; ok {
		article.CategoryName = cat.Name
	}

	return article, nil
}

// Reload reloads all content from disk
func (s *ContentService) Reload() error {
	if err := s.loadConfig(); err != nil {
		return err
	}
	return s.loadArticles()
}

// GetArticles returns paginated articles
func (s *ContentService) GetArticles(page, perPage int) ([]Article, int) {
	if perPage <= 0 {
		perPage = 9
	}
	if page < 1 {
		page = 1
	}

	total := len(s.articles)
	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return nil, total
	}
	if end > total {
		end = total
	}

	return s.articles[start:end], total
}

// GetArticlesByCategory returns articles in a category
func (s *ContentService) GetArticlesByCategory(categorySlug string, page, perPage int) ([]Article, int) {
	var filtered []Article
	for _, a := range s.articles {
		if a.Category == categorySlug {
			filtered = append(filtered, a)
		}
	}

	if perPage <= 0 {
		perPage = 9
	}
	if page < 1 {
		page = 1
	}

	total := len(filtered)
	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return nil, total
	}
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

// GetArticlesByTag returns articles with a tag
func (s *ContentService) GetArticlesByTag(tag string, page, perPage int) ([]Article, int) {
	var filtered []Article
	for _, a := range s.articles {
		for _, t := range a.Tags {
			if t == tag {
				filtered = append(filtered, a)
				break
			}
		}
	}

	if perPage <= 0 {
		perPage = 9
	}
	if page < 1 {
		page = 1
	}

	total := len(filtered)
	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		return nil, total
	}
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

// GetArticle returns a single article by slug
func (s *ContentService) GetArticle(slug string) (Article, bool) {
	for _, a := range s.articles {
		if a.Slug == slug {
			return a, true
		}
	}
	return Article{}, false
}

// GetLatestArticles returns the N most recent articles
func (s *ContentService) GetLatestArticles(limit int) []Article {
	if limit > len(s.articles) {
		limit = len(s.articles)
	}
	return s.articles[:limit]
}

// GetFeaturedArticles returns featured articles
func (s *ContentService) GetFeaturedArticles(limit int) []Article {
	var featured []Article
	for _, a := range s.articles {
		if a.Featured {
			featured = append(featured, a)
			if len(featured) >= limit {
				break
			}
		}
	}
	return featured
}

// GetRelatedArticles returns articles in the same category
func (s *ContentService) GetRelatedArticles(slug string, limit int) []Article {
	article, ok := s.GetArticle(slug)
	if !ok {
		return nil
	}

	var related []Article
	for _, a := range s.articles {
		if a.Slug != slug && a.Category == article.Category {
			related = append(related, a)
			if len(related) >= limit {
				break
			}
		}
	}
	return related
}

// GetCategories returns all categories
func (s *ContentService) GetCategories() []Category {
	return s.config.Categories
}

// GetCategory returns a category by slug
func (s *ContentService) GetCategory(slug string) (Category, bool) {
	cat, ok := s.categories[slug]
	return cat, ok
}

// GetTags returns all unique tags
func (s *ContentService) GetTags() []string {
	tagSet := make(map[string]bool)
	for _, a := range s.articles {
		for _, t := range a.Tags {
			tagSet[t] = true
		}
	}

	var tags []string
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

// ExtractTOC extracts table of contents from markdown
func (s *ContentService) ExtractTOC(markdown string) []TOCItem {
	var items []TOCItem
	re := regexp.MustCompile(`(?m)^(#{2,3})\s+(.+)$`)
	matches := re.FindAllStringSubmatch(markdown, -1)

	for _, match := range matches {
		level := len(match[1])
		text := strings.TrimSpace(match[2])
		id := slugify(text)
		items = append(items, TOCItem{Level: level, ID: id, Text: text})
	}

	return items
}

// Search searches articles by query
func (s *ContentService) Search(query string, limit int) []Article {
	query = strings.ToLower(query)
	var results []Article

	for _, a := range s.articles {
		if strings.Contains(strings.ToLower(a.Title), query) ||
			strings.Contains(strings.ToLower(a.Excerpt), query) ||
			strings.Contains(strings.ToLower(a.Content), query) {
			results = append(results, a)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results
}

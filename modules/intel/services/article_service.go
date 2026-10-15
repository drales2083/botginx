package services

import (
	"database/sql"
	"fmt"
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
		where = append(where, fmt.Sprintf("a.status = $%d", argNum))
		args = append(args, opts.Status)
		argNum++
	}
	if opts.CategoryID != "" {
		where = append(where, fmt.Sprintf("a.category_id = $%d", argNum))
		args = append(args, opts.CategoryID)
		argNum++
	}
	if opts.TagID != "" {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM intel_article_tags t WHERE t.article_id = a.id AND t.tag_id = $%d)", argNum))
		args = append(args, opts.TagID)
		argNum++
	}
	if opts.Search != "" {
		where = append(where, fmt.Sprintf("(a.title ILIKE $%d OR a.excerpt ILIKE $%d)", argNum, argNum))
		args = append(args, "%"+opts.Search+"%")
		argNum++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM intel_articles a WHERE " + whereClause
	s.db.Get(&total, countQuery, args...)

	query := fmt.Sprintf(`
		SELECT a.*, c.name as category_name, c.slug as category_slug
		FROM intel_articles a
		LEFT JOIN intel_categories c ON c.id = a.category_id
		WHERE %s
		ORDER BY a.featured DESC, a.published_at DESC NULLS LAST, a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argNum, argNum+1)

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

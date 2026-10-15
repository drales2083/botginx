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
	ID         string    `db:"id" json:"id"`
	Name       string    `db:"name" json:"name"`
	Slug       string    `db:"slug" json:"slug"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
	UsageCount int       `db:"usage_count" json:"usageCount,omitempty"`
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
	Status          string         `db:"status" json:"status"`
	Featured        bool           `db:"featured" json:"featured"`
	ReadingTime     int            `db:"reading_time" json:"readingTime"`
	MetaTitle       string         `db:"meta_title" json:"metaTitle"`
	MetaDescription string         `db:"meta_description" json:"metaDescription"`
	PublishedAt     sql.NullTime   `db:"published_at" json:"publishedAt"`
	CreatedAt       time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updatedAt"`
	CategoryName    string         `db:"category_name" json:"categoryName,omitempty"`
	CategorySlug    string         `db:"category_slug" json:"categorySlug,omitempty"`
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

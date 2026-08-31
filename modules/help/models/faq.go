package models

import "time"

type FAQCategory struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Icon      string    `db:"icon" json:"icon"`
	SortOrder int       `db:"sort_order" json:"sortOrder"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`

	// Joined
	Items []FAQItem `json:"items,omitempty"`
}

type FAQItem struct {
	ID         string    `db:"id" json:"id"`
	CategoryID string    `db:"category_id" json:"categoryId"`
	Question   string    `db:"question" json:"question"`
	Answer     string    `db:"answer" json:"answer"`
	SortOrder  int       `db:"sort_order" json:"sortOrder"`
	IsActive   bool      `db:"is_active" json:"isActive"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt  time.Time `db:"updated_at" json:"updatedAt"`

	// Joined
	CategoryName string `db:"category_name" json:"categoryName,omitempty"`
}

type CreateCategoryInput struct {
	Name      string `json:"name" validate:"required"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sortOrder"`
}

type UpdateCategoryInput struct {
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder *int   `json:"sortOrder"`
}

type CreateFAQInput struct {
	CategoryID string `json:"categoryId" validate:"required"`
	Question   string `json:"question" validate:"required"`
	Answer     string `json:"answer" validate:"required"`
	SortOrder  int    `json:"sortOrder"`
}

type UpdateFAQInput struct {
	CategoryID string `json:"categoryId"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	SortOrder  *int   `json:"sortOrder"`
	IsActive   *bool  `json:"isActive"`
}

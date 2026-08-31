package services

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/botginx/botginx/modules/help/models"
	"github.com/jmoiron/sqlx"
)

type HelpService struct {
	db *sqlx.DB
}

func NewHelpService(db *sqlx.DB) *HelpService {
	return &HelpService{db: db}
}

func (s *HelpService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Categories

func (s *HelpService) ListCategories() ([]models.FAQCategory, error) {
	var categories []models.FAQCategory
	err := s.db.Select(&categories, `
		SELECT * FROM faq_categories ORDER BY sort_order, name
	`)
	return categories, err
}

func (s *HelpService) GetCategory(id string) (*models.FAQCategory, error) {
	var cat models.FAQCategory
	err := s.db.Get(&cat, `SELECT * FROM faq_categories WHERE id = $1`, id)
	return &cat, err
}

func (s *HelpService) CreateCategory(input models.CreateCategoryInput) (*models.FAQCategory, error) {
	cat := &models.FAQCategory{
		ID:        s.generateID(),
		Name:      input.Name,
		Icon:      input.Icon,
		SortOrder: input.SortOrder,
		CreatedAt: time.Now(),
	}
	if cat.Icon == "" {
		cat.Icon = "bi-question-circle"
	}

	_, err := s.db.NamedExec(`
		INSERT INTO faq_categories (id, name, icon, sort_order, created_at)
		VALUES (:id, :name, :icon, :sort_order, :created_at)
	`, cat)
	return cat, err
}

func (s *HelpService) UpdateCategory(id string, input models.UpdateCategoryInput) (*models.FAQCategory, error) {
	cat, err := s.GetCategory(id)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		cat.Name = input.Name
	}
	if input.Icon != "" {
		cat.Icon = input.Icon
	}
	if input.SortOrder != nil {
		cat.SortOrder = *input.SortOrder
	}

	_, err = s.db.NamedExec(`
		UPDATE faq_categories SET name = :name, icon = :icon, sort_order = :sort_order
		WHERE id = :id
	`, cat)
	return cat, err
}

func (s *HelpService) DeleteCategory(id string) error {
	// Delete items first
	s.db.Exec(`DELETE FROM faq_items WHERE category_id = $1`, id)
	_, err := s.db.Exec(`DELETE FROM faq_categories WHERE id = $1`, id)
	return err
}

// FAQ Items

func (s *HelpService) ListItems() ([]models.FAQItem, error) {
	var items []models.FAQItem
	err := s.db.Select(&items, `
		SELECT i.*, c.name as category_name
		FROM faq_items i
		JOIN faq_categories c ON c.id = i.category_id
		ORDER BY c.sort_order, i.sort_order
	`)
	return items, err
}

func (s *HelpService) ListItemsByCategory(categoryID string) ([]models.FAQItem, error) {
	var items []models.FAQItem
	err := s.db.Select(&items, `
		SELECT * FROM faq_items WHERE category_id = $1 ORDER BY sort_order
	`, categoryID)
	return items, err
}

func (s *HelpService) ListActiveItems() ([]models.FAQItem, error) {
	var items []models.FAQItem
	err := s.db.Select(&items, `
		SELECT i.*, c.name as category_name
		FROM faq_items i
		JOIN faq_categories c ON c.id = i.category_id
		WHERE i.is_active = true
		ORDER BY c.sort_order, i.sort_order
	`)
	return items, err
}

func (s *HelpService) GetItem(id string) (*models.FAQItem, error) {
	var item models.FAQItem
	err := s.db.Get(&item, `SELECT * FROM faq_items WHERE id = $1`, id)
	return &item, err
}

func (s *HelpService) CreateItem(input models.CreateFAQInput) (*models.FAQItem, error) {
	item := &models.FAQItem{
		ID:         s.generateID(),
		CategoryID: input.CategoryID,
		Question:   input.Question,
		Answer:     input.Answer,
		SortOrder:  input.SortOrder,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err := s.db.NamedExec(`
		INSERT INTO faq_items (id, category_id, question, answer, sort_order, is_active, created_at, updated_at)
		VALUES (:id, :category_id, :question, :answer, :sort_order, :is_active, :created_at, :updated_at)
	`, item)
	return item, err
}

func (s *HelpService) UpdateItem(id string, input models.UpdateFAQInput) (*models.FAQItem, error) {
	item, err := s.GetItem(id)
	if err != nil {
		return nil, err
	}

	if input.CategoryID != "" {
		item.CategoryID = input.CategoryID
	}
	if input.Question != "" {
		item.Question = input.Question
	}
	if input.Answer != "" {
		item.Answer = input.Answer
	}
	if input.SortOrder != nil {
		item.SortOrder = *input.SortOrder
	}
	if input.IsActive != nil {
		item.IsActive = *input.IsActive
	}
	item.UpdatedAt = time.Now()

	_, err = s.db.NamedExec(`
		UPDATE faq_items SET category_id = :category_id, question = :question, answer = :answer,
		sort_order = :sort_order, is_active = :is_active, updated_at = :updated_at
		WHERE id = :id
	`, item)
	return item, err
}

func (s *HelpService) DeleteItem(id string) error {
	_, err := s.db.Exec(`DELETE FROM faq_items WHERE id = $1`, id)
	return err
}

// GetCategoriesWithItems returns all categories with their active items
func (s *HelpService) GetCategoriesWithItems() ([]models.FAQCategory, error) {
	categories, err := s.ListCategories()
	if err != nil {
		return nil, err
	}

	for i := range categories {
		var items []models.FAQItem
		s.db.Select(&items, `
			SELECT * FROM faq_items
			WHERE category_id = $1 AND is_active = true
			ORDER BY sort_order
		`, categories[i].ID)
		categories[i].Items = items
	}

	return categories, nil
}

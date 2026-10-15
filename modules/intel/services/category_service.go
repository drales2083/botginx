package services

import (
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

func (s *CategoryService) GetByID(id string) (*models.Category, error) {
	var cat models.Category
	err := s.db.Get(&cat, `SELECT * FROM intel_categories WHERE id = $1`, id)
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

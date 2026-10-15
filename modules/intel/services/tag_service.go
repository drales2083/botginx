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

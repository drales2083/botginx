package services

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrAlreadyFavorited = errors.New("domain already in favorites")

type FavoritesService struct {
	db *sqlx.DB
}

func NewFavoritesService(db *sqlx.DB) *FavoritesService {
	return &FavoritesService{db: db}
}

// Add adds a domain to user's favorites
func (s *FavoritesService) Add(ctx context.Context, userID, inventoryID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO aged_domain_favorites (id, user_id, inventory_id, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, inventory_id) DO NOTHING
	`, uuid.New().String(), userID, inventoryID)
	return err
}

// Remove removes a domain from user's favorites
func (s *FavoritesService) Remove(ctx context.Context, userID, inventoryID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM aged_domain_favorites
		WHERE user_id = $1 AND inventory_id = $2
	`, userID, inventoryID)
	return err
}

// Toggle adds or removes a favorite, returns new state
func (s *FavoritesService) Toggle(ctx context.Context, userID, inventoryID string) (bool, error) {
	// Check if exists
	var exists bool
	err := s.db.GetContext(ctx, &exists, `
		SELECT EXISTS(SELECT 1 FROM aged_domain_favorites WHERE user_id = $1 AND inventory_id = $2)
	`, userID, inventoryID)
	if err != nil {
		return false, err
	}

	if exists {
		err = s.Remove(ctx, userID, inventoryID)
		return false, err
	}

	err = s.Add(ctx, userID, inventoryID)
	return true, err
}

// IsFavorite checks if a domain is in user's favorites
func (s *FavoritesService) IsFavorite(ctx context.Context, userID, inventoryID string) (bool, error) {
	var exists bool
	err := s.db.GetContext(ctx, &exists, `
		SELECT EXISTS(SELECT 1 FROM aged_domain_favorites WHERE user_id = $1 AND inventory_id = $2)
	`, userID, inventoryID)
	return exists, err
}

// GetUserFavoriteIDs returns all inventory IDs that are favorites for a user
func (s *FavoritesService) GetUserFavoriteIDs(ctx context.Context, userID string) (map[string]bool, error) {
	var ids []string
	err := s.db.SelectContext(ctx, &ids, `
		SELECT inventory_id FROM aged_domain_favorites WHERE user_id = $1
	`, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	result := make(map[string]bool)
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// Count returns the number of favorites for a user
func (s *FavoritesService) Count(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM aged_domain_favorites WHERE user_id = $1
	`, userID)
	return count, err
}

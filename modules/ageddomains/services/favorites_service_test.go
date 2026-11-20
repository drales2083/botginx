package services

import (
	"testing"
)

func TestNewFavoritesService(t *testing.T) {
	svc := NewFavoritesService(nil)
	if svc == nil {
		t.Fatal("NewFavoritesService returned nil")
	}
}

func TestFavoritesServiceErrors(t *testing.T) {
	if ErrAlreadyFavorited == nil {
		t.Error("ErrAlreadyFavorited should not be nil")
	}
	if ErrAlreadyFavorited.Error() == "" {
		t.Error("ErrAlreadyFavorited should have an error message")
	}
}

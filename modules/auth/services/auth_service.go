package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/botginx/botginx/modules/auth/models"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailExists      = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound     = errors.New("user not found")
	ErrSessionExpired   = errors.New("session expired")
)

type AuthService struct {
	db *sqlx.DB
}

func NewAuthService(db *sqlx.DB) *AuthService {
	return &AuthService{db: db}
}

func (s *AuthService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *AuthService) generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *AuthService) Signup(input models.SignupInput) (*models.User, error) {
	// Check if email exists
	var count int
	s.db.Get(&count, "SELECT COUNT(*) FROM users WHERE email = $1", input.Email)
	if count > 0 {
		return nil, ErrEmailExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:           s.generateID(),
		Email:        input.Email,
		PasswordHash: string(hash),
		Name:         input.Name,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES (:id, :email, :password_hash, :name, :is_active, :created_at, :updated_at)
	`, user)

	return user, err
}

func (s *AuthService) Login(input models.LoginInput) (*models.User, string, error) {
	var user models.User
	err := s.db.Get(&user, "SELECT * FROM users WHERE email = $1 AND is_active = true", input.Email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Create session
	session := &models.Session{
		ID:        s.generateID(),
		UserID:    user.ID,
		Token:     s.generateToken(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
		CreatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES (:id, :user_id, :token, :expires_at, :created_at)
	`, session)
	if err != nil {
		return nil, "", err
	}

	return &user, session.Token, nil
}

func (s *AuthService) ValidateSession(token string) (*models.User, error) {
	var session models.Session
	err := s.db.Get(&session, "SELECT * FROM sessions WHERE token = $1", token)
	if err != nil {
		return nil, ErrSessionExpired
	}

	if time.Now().After(session.ExpiresAt) {
		s.db.Exec("DELETE FROM sessions WHERE id = $1", session.ID)
		return nil, ErrSessionExpired
	}

	var user models.User
	err = s.db.Get(&user, "SELECT * FROM users WHERE id = $1 AND is_active = true", session.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return &user, nil
}

func (s *AuthService) Logout(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = $1", token)
	return err
}

// UpdateProfile changes the user's own name and email.
func (s *AuthService) UpdateProfile(userID, name, email string) error {
	// Email identifies the account, so it must stay unique across users.
	var count int
	s.db.Get(&count, `SELECT COUNT(*) FROM users WHERE email = $1 AND id <> $2`, email, userID)
	if count > 0 {
		return ErrEmailExists
	}

	_, err := s.db.Exec(`
		UPDATE users SET name = $2, email = $3, updated_at = NOW() WHERE id = $1
	`, userID, name, email)
	return err
}

// ChangePassword verifies the current password before setting a new one, so a
// borrowed session cannot be used to lock the real owner out.
func (s *AuthService) ChangePassword(userID, currentPassword, newPassword string) error {
	var user models.User
	if err := s.db.Get(&user, `SELECT * FROM users WHERE id = $1`, userID); err != nil {
		return ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1
	`, userID, string(hash)); err != nil {
		return err
	}

	// Every other session was authenticated with the old password; drop them so
	// a changed password actually revokes access elsewhere.
	_, err = s.db.Exec(`DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

func (s *AuthService) GetUser(id string) (*models.User, error) {
	var user models.User
	err := s.db.Get(&user, "SELECT * FROM users WHERE id = $1", id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

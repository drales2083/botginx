package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
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
	return s.SignupWithReferral(input, "")
}

func (s *AuthService) SignupWithReferral(input models.SignupInput, refCode string) (*models.User, error) {
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

	// Generate referral code for new user
	newRefCode := s.generateID()[:8]

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
		INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at, referral_code)
		VALUES (:id, :email, :password_hash, :name, :is_active, :created_at, :updated_at, '`+newRefCode+`')
	`, user)
	if err != nil {
		return nil, err
	}

	// Set referrer if valid code provided
	if refCode != "" {
		var referrerID string
		err := s.db.Get(&referrerID, `SELECT id FROM users WHERE referral_code = $1`, refCode)
		if err == nil && referrerID != user.ID {
			s.db.Exec(`UPDATE users SET referred_by_id = $1 WHERE id = $2`, referrerID, user.ID)
		}
	}

	return user, nil
}

func (s *AuthService) Login(input models.LoginInput) (*models.User, string, error) {
	var user models.User
	err := s.db.Get(&user, "SELECT * FROM users WHERE email = $1 AND is_active = true", input.Email)
	if err != nil {
		log.Printf("[AUTH DEBUG] User lookup failed for email=%q: %v", input.Email, err)
		return nil, "", ErrInvalidCredentials
	}
	log.Printf("[AUTH DEBUG] User found: id=%s, email=%s, hash_len=%d, hash_prefix=%s", user.ID, user.Email, len(user.PasswordHash), user.PasswordHash[:10])

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		log.Printf("[AUTH DEBUG] Password mismatch for user=%s: %v", user.Email, err)
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

// CreateImpersonationSession creates a session for the target user (admin impersonation)
// Returns the new session token for the target user
func (s *AuthService) CreateImpersonationSession(targetUserID string) (string, error) {
	// Verify target user exists
	var user models.User
	err := s.db.Get(&user, "SELECT * FROM users WHERE id = $1", targetUserID)
	if err != nil {
		return "", ErrUserNotFound
	}

	// Create session for target user
	session := &models.Session{
		ID:        s.generateID(),
		UserID:    user.ID,
		Token:     s.generateToken(),
		ExpiresAt: time.Now().Add(4 * time.Hour), // Shorter expiry for impersonation
		CreatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES (:id, :user_id, :token, :expires_at, :created_at)
	`, session)
	if err != nil {
		return "", err
	}

	return session.Token, nil
}

package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/botginx/botginx/modules/users/models"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailExists  = errors.New("email already registered")
	ErrUserNotFound = errors.New("user not found")
)

type UserService struct {
	db *sqlx.DB
}

func NewUserService(db *sqlx.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *UserService) List() ([]models.User, error) {
	var users []models.User
	err := s.db.Select(&users, `SELECT id, email, password_hash, name, role, is_active, COALESCE(balance, 0) as balance, created_at, updated_at FROM users ORDER BY created_at DESC`)
	return users, err
}

func (s *UserService) Get(id string) (*models.User, error) {
	var user models.User
	err := s.db.Get(&user, `SELECT * FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

func (s *UserService) Create(input models.CreateUserInput) (*models.User, error) {
	// Check if email exists
	var count int
	s.db.Get(&count, "SELECT COUNT(*) FROM users WHERE email = $1", input.Email)
	if count > 0 {
		return nil, ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := models.RoleUser
	if input.Role == "admin" {
		role = models.RoleAdmin
	}

	user := &models.User{
		ID:           s.generateID(),
		Email:        input.Email,
		PasswordHash: string(hash),
		Name:         input.Name,
		Role:         role,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO users (id, email, password_hash, name, role, is_active, created_at, updated_at)
		VALUES (:id, :email, :password_hash, :name, :role, :is_active, :created_at, :updated_at)
	`, user)

	return user, err
}

func (s *UserService) Update(id string, input models.UpdateUserInput) (*models.User, error) {
	user, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if input.Email != "" {
		user.Email = input.Email
	}
	if input.Name != "" {
		user.Name = input.Name
	}
	if input.Role != "" {
		user.Role = models.UserRole(input.Role)
	}
	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}
	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = string(hash)
	}
	user.UpdatedAt = time.Now()

	_, err = s.db.NamedExec(`
		UPDATE users SET
			email = :email,
			password_hash = :password_hash,
			name = :name,
			role = :role,
			is_active = :is_active,
			updated_at = :updated_at
		WHERE id = :id
	`, user)

	return user, err
}

func (s *UserService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = $1`, id)
	return err
}

func (s *UserService) Count() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM users`)
	return count, err
}

func (s *UserService) CountActive() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM users WHERE is_active = true`)
	return count, err
}

// GetBalance returns a user's current balance
func (s *UserService) GetBalance(userID string) (float64, error) {
	var balance float64
	err := s.db.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)
	return balance, err
}

// TopUpBalance adds funds to a user's balance
func (s *UserService) TopUpBalance(userID string, amount float64, description string) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE users SET balance = COALESCE(balance, 0) + $1 WHERE id = $2`, amount, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO balance_transactions (id, user_id, amount, type, description, created_at)
		VALUES ($1, $2, $3, 'topup', $4, $5)
	`, s.generateID(), userID, amount, description, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

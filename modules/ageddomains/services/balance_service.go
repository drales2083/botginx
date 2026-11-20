package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/botginx/botginx/modules/ageddomains/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrLockNotFound        = errors.New("balance lock not found")
	ErrLockAlreadyResolved = errors.New("balance lock already resolved")
	ErrDuplicateOperation  = errors.New("duplicate operation detected")
)

const (
	LockDuration = 10 * time.Minute
)

type BalanceService struct {
	db *sqlx.DB
}

func NewBalanceService(db *sqlx.DB) *BalanceService {
	return &BalanceService{db: db}
}

// GenerateIdempotencyKey creates a unique key for a purchase operation
func GenerateIdempotencyKey(userID, domainName string) string {
	data := fmt.Sprintf("%s:%s:%s", userID, domainName, time.Now().Format("2006-01-02"))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// LockFunds atomically reserves funds for a purchase
// Uses SERIALIZABLE isolation to prevent race conditions
func (s *BalanceService) LockFunds(ctx context.Context, userID string, amountCents int, idempotencyKey, purpose string) (*models.BalanceLock, error) {
	// Check for existing lock with same idempotency key (idempotent replay)
	var existing models.BalanceLock
	err := s.db.GetContext(ctx, &existing,
		`SELECT * FROM balance_locks WHERE idempotency_key = $1`, idempotencyKey)
	if err == nil {
		if existing.Status == models.LockStatusPending {
			return &existing, nil // Idempotent: return existing pending lock
		}
		return nil, ErrDuplicateOperation // Already committed/released
	}

	// Start serializable transaction
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get user's current balance with row lock
	var currentBalance int
	err = tx.GetContext(ctx, &currentBalance,
		`SELECT COALESCE(balance, 0) FROM users WHERE id = $1 FOR UPDATE`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInsufficientBalance
		}
		return nil, fmt.Errorf("get balance: %w", err)
	}

	// Calculate total pending locks for this user
	var pendingLocks int
	err = tx.GetContext(ctx, &pendingLocks,
		`SELECT COALESCE(SUM(amount), 0) FROM balance_locks
		 WHERE user_id = $1 AND status = 'pending'`, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get pending locks: %w", err)
	}

	// Check available balance
	availableBalance := currentBalance - pendingLocks
	if availableBalance < amountCents {
		return nil, ErrInsufficientBalance
	}

	// Create lock record
	lock := &models.BalanceLock{
		ID:             uuid.New().String(),
		UserID:         userID,
		Amount:         amountCents,
		IdempotencyKey: idempotencyKey,
		Purpose:        purpose,
		Status:         models.LockStatusPending,
		LockedAt:       time.Now(),
		ExpiresAt:      time.Now().Add(LockDuration),
	}

	_, err = tx.NamedExecContext(ctx, `
		INSERT INTO balance_locks (id, user_id, amount, idempotency_key, purpose, status, locked_at, expires_at)
		VALUES (:id, :user_id, :amount, :idempotency_key, :purpose, :status, :locked_at, :expires_at)
	`, lock)
	if err != nil {
		return nil, fmt.Errorf("create lock: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return lock, nil
}

// CommitLock converts a pending lock to a permanent balance debit
// This is the point of no return - only call after provider purchase succeeds
func (s *BalanceService) CommitLock(ctx context.Context, lockID string) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get lock with row lock
	var lock models.BalanceLock
	err = tx.GetContext(ctx, &lock,
		`SELECT * FROM balance_locks WHERE id = $1 FOR UPDATE`, lockID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLockNotFound
		}
		return fmt.Errorf("get lock: %w", err)
	}

	if lock.Status != models.LockStatusPending {
		return ErrLockAlreadyResolved
	}

	// Debit user balance
	result, err := tx.ExecContext(ctx,
		`UPDATE users SET balance = balance - $1 WHERE id = $2 AND balance >= $1`,
		lock.Amount, lock.UserID)
	if err != nil {
		return fmt.Errorf("debit balance: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInsufficientBalance // Balance changed since lock
	}

	// Record balance transaction with idempotency
	_, err = tx.ExecContext(ctx, `
		INSERT INTO balance_transactions (id, user_id, amount, type, description, idempotency_key, lock_id, created_at)
		VALUES ($1, $2, $3, 'purchase', $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
	`, uuid.New().String(), lock.UserID, -lock.Amount, lock.Purpose, lock.IdempotencyKey, lockID, time.Now())
	if err != nil {
		return fmt.Errorf("record transaction: %w", err)
	}

	// Update lock status
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE balance_locks SET status = 'committed', resolved_at = $1 WHERE id = $2`,
		now, lockID)
	if err != nil {
		return fmt.Errorf("update lock: %w", err)
	}

	return tx.Commit()
}

// ReleaseLock releases funds back (purchase failed before commit)
func (s *BalanceService) ReleaseLock(ctx context.Context, lockID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE balance_locks
		SET status = 'released', resolved_at = $1
		WHERE id = $2 AND status = 'pending'
	`, time.Now(), lockID)
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrLockAlreadyResolved
	}
	return nil
}

// GetAvailableBalance returns balance minus pending locks
func (s *BalanceService) GetAvailableBalance(ctx context.Context, userID string) (*models.BalanceInfo, error) {
	var result struct {
		Balance int `db:"balance"`
		Locked  int `db:"locked"`
	}
	err := s.db.GetContext(ctx, &result, `
		SELECT
			COALESCE(u.balance, 0) as balance,
			COALESCE(l.total, 0) as locked
		FROM users u
		LEFT JOIN (
			SELECT user_id, SUM(amount) as total
			FROM balance_locks
			WHERE status = 'pending'
			GROUP BY user_id
		) l ON l.user_id = u.id
		WHERE u.id = $1
	`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &models.BalanceInfo{}, nil
		}
		return nil, err
	}
	return &models.BalanceInfo{
		Available: result.Balance - result.Locked,
		Locked:    result.Locked,
		Total:     result.Balance,
	}, nil
}

// GetLock retrieves a lock by ID
func (s *BalanceService) GetLock(ctx context.Context, lockID string) (*models.BalanceLock, error) {
	var lock models.BalanceLock
	err := s.db.GetContext(ctx, &lock, `SELECT * FROM balance_locks WHERE id = $1`, lockID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLockNotFound
		}
		return nil, err
	}
	return &lock, nil
}

// GetPendingLocks returns all pending locks for a user
func (s *BalanceService) GetPendingLocks(ctx context.Context, userID string) ([]models.BalanceLock, error) {
	var locks []models.BalanceLock
	err := s.db.SelectContext(ctx, &locks, `
		SELECT * FROM balance_locks
		WHERE user_id = $1 AND status = 'pending'
		ORDER BY locked_at DESC
	`, userID)
	return locks, err
}

// CleanupExpiredLocks releases locks past their expiration (background job)
func (s *BalanceService) CleanupExpiredLocks(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE balance_locks
		SET status = 'released', resolved_at = $1
		WHERE status = 'pending' AND expires_at < $1
	`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

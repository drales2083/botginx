package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Pixel is one tracking pixel row.
type Pixel struct {
	ID         string       `db:"id"`
	UserID     string       `db:"user_id"`
	Label      string       `db:"label"`
	Domain     string       `db:"domain"`
	Format     string       `db:"format"`
	Token      string       `db:"token"`
	Verified   bool         `db:"verified"`
	LastOpenAt sql.NullTime `db:"last_open_at"`
	CreatedAt  time.Time    `db:"created_at"`
	Opens      int          `db:"opens"` // populated by ListByUser join
}

// Store wraps DB access for the module.
type Store struct{ DB *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{DB: db} }

// CountByUser counts a user's pixels (for the free-tier check).
func (s *Store) CountByUser(userID string) (int, error) {
	var n int
	err := s.DB.Get(&n, `SELECT count(*) FROM tracking_pixels WHERE user_id=$1`, userID)
	return n, err
}

// InsertPixelTx inserts a pixel inside an existing transaction.
func InsertPixelTx(tx *sqlx.Tx, p *Pixel) error {
	p.ID = uuid.New().String()
	_, err := tx.Exec(`INSERT INTO tracking_pixels (id,user_id,label,domain,format,token)
		VALUES ($1,$2,$3,$4,$5,$6)`, p.ID, p.UserID, p.Label, p.Domain, p.Format, p.Token)
	return err
}

// ListByUser returns a user's pixels with open counts, newest first.
func (s *Store) ListByUser(userID string) ([]Pixel, error) {
	var out []Pixel
	err := s.DB.Select(&out, `
		SELECT p.*, COALESCE(o.c,0) AS opens
		FROM tracking_pixels p
		LEFT JOIN (SELECT pixel_id, count(*) c FROM tracking_pixel_opens GROUP BY pixel_id) o
		  ON o.pixel_id = p.id
		WHERE p.user_id=$1 ORDER BY p.created_at DESC`, userID)
	return out, err
}

// GetByToken finds a pixel by its public token.
func (s *Store) GetByToken(token string) (*Pixel, error) {
	var p Pixel
	err := s.DB.Get(&p, `SELECT * FROM tracking_pixels WHERE token=$1`, token)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeleteOwned deletes a pixel only if it belongs to userID.
func (s *Store) DeleteOwned(id, userID string) error {
	_, err := s.DB.Exec(`DELETE FROM tracking_pixels WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

// RecordOpen inserts an open and bumps the pixel; sets verified on first human.
func (s *Store) RecordOpen(pixelID, ipHash, ua, classification string) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO tracking_pixel_opens (id,pixel_id,ip_hash,user_agent,classification)
		VALUES ($1,$2,$3,$4,$5)`, uuid.New().String(), pixelID, ipHash, ua, classification); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE tracking_pixels SET last_open_at=NOW(),
		verified = verified OR ($2='human') WHERE id=$1`, pixelID, classification); err != nil {
		return err
	}
	return tx.Commit()
}

// PurgeOlderThan deletes opens older than the retention cutoff.
func (s *Store) PurgeOlderThan(d time.Duration) error {
	_, err := s.DB.Exec(`DELETE FROM tracking_pixel_opens WHERE opened_at < NOW() - make_interval(secs => $1)`,
		d.Seconds())
	return err
}

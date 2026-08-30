package services

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/botginx/botginx/modules/auth/models"
	"github.com/jmoiron/sqlx"
)

const (
	MaxGlobalWhitelistIPs = 5
	supabaseTimeout       = 10 * time.Second
)

type GlobalWhitelistService struct {
	db *sqlx.DB
}

func NewGlobalWhitelistService(db *sqlx.DB) *GlobalWhitelistService {
	return &GlobalWhitelistService{db: db}
}

func (s *GlobalWhitelistService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *GlobalWhitelistService) List(userID string) ([]models.GlobalWhitelist, error) {
	var entries []models.GlobalWhitelist
	err := s.db.Select(&entries, `
		SELECT id, user_id, ip, created_at
		FROM user_global_whitelists
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []models.GlobalWhitelist{}
	}
	return entries, nil
}

func (s *GlobalWhitelistService) Add(userID, ip string) (*models.GlobalWhitelist, error) {
	if !isValidIPv4(ip) {
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	count, err := s.Count(userID)
	if err != nil {
		return nil, err
	}
	if count >= MaxGlobalWhitelistIPs {
		return nil, fmt.Errorf("maximum %d IPs allowed", MaxGlobalWhitelistIPs)
	}

	exists, err := s.Exists(userID, ip)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("IP already whitelisted")
	}

	entry := &models.GlobalWhitelist{
		ID:        s.generateID(),
		UserID:    userID,
		IP:        ip,
		CreatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO user_global_whitelists (id, user_id, ip, created_at)
		VALUES (:id, :user_id, :ip, :created_at)
	`, entry)
	if err != nil {
		return nil, err
	}

	go s.syncToSupabase(ip, userID)

	return entry, nil
}

func (s *GlobalWhitelistService) Remove(userID, id string) error {
	var ip string
	err := s.db.Get(&ip, `SELECT ip FROM user_global_whitelists WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("whitelist entry not found")
	}

	_, err = s.db.Exec(`DELETE FROM user_global_whitelists WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}

	go s.removeFromSupabase(ip)

	return nil
}

func (s *GlobalWhitelistService) Count(userID string) (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM user_global_whitelists WHERE user_id = $1`, userID)
	return count, err
}

func (s *GlobalWhitelistService) Exists(userID, ip string) (bool, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM user_global_whitelists WHERE user_id = $1 AND ip = $2`, userID, ip)
	return count > 0, err
}

func isValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() != nil
}

func (s *GlobalWhitelistService) syncToSupabase(ip, userID string) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		return
	}

	sourceHost := os.Getenv("PANEL_URL")
	if sourceHost == "" {
		sourceHost = "guardbot.sbs"
	}

	payload := map[string]interface{}{
		"ip":          ip,
		"user_id":     userID,
		"source_host": sourceHost,
		"updated_at":  time.Now().Format(time.RFC3339),
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", supabaseURL+"/rest/v1/global_whitelisted_ips", bytes.NewReader(body))
	if err != nil {
		return
	}

	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates")

	client := &http.Client{Timeout: supabaseTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (s *GlobalWhitelistService) removeFromSupabase(ip string) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		return
	}

	url := fmt.Sprintf("%s/rest/v1/global_whitelisted_ips?ip=eq.%s", supabaseURL, ip)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return
	}

	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)

	client := &http.Client{Timeout: supabaseTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

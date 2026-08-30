package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/botginx/botginx/modules/iplists/models"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

const (
	MaxWhitelistIPs = 10
	MaxBlocklistIPs = 50
)

type ServerInfo struct {
	IP       string
	Port     int
	User     string
	Password string
}

type ServerProvider interface {
	GetAllDeployServers() ([]ServerInfo, error)
}

type IPListService struct {
	db      *sqlx.DB
	servers ServerProvider
}

func NewIPListService(db *sqlx.DB) *IPListService {
	return &IPListService{db: db}
}

func (s *IPListService) SetServerProvider(sp ServerProvider) {
	s.servers = sp
}

func (s *IPListService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func isValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() != nil
}

// Whitelist operations

func (s *IPListService) ListWhitelist(userID string) ([]models.IPWhitelist, error) {
	var entries []models.IPWhitelist
	err := s.db.Select(&entries, `
		SELECT id, user_id, ip, note, created_at
		FROM ip_whitelists
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []models.IPWhitelist{}
	}
	return entries, nil
}

func (s *IPListService) AddWhitelist(userID, ip, note string) (*models.IPWhitelist, error) {
	if !isValidIPv4(ip) {
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	count, _ := s.CountWhitelist(userID)
	if count >= MaxWhitelistIPs {
		return nil, fmt.Errorf("maximum %d IPs allowed", MaxWhitelistIPs)
	}

	exists, _ := s.WhitelistExists(userID, ip)
	if exists {
		return nil, fmt.Errorf("IP already whitelisted")
	}

	// Check if IP is in blocklist
	blocked, _ := s.BlocklistExists(userID, ip)
	if blocked {
		return nil, fmt.Errorf("IP is in blocklist, remove it first")
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	entry := &models.IPWhitelist{
		ID:        s.generateID(),
		UserID:    userID,
		IP:        ip,
		Note:      notePtr,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO ip_whitelists (id, user_id, ip, note, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, entry.ID, entry.UserID, entry.IP, entry.Note, entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	go s.pushWhitelistToVPS(userID)

	return entry, nil
}

func (s *IPListService) RemoveWhitelist(userID, id string) error {
	result, err := s.db.Exec(`DELETE FROM ip_whitelists WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("entry not found")
	}

	go s.pushWhitelistToVPS(userID)

	return nil
}

func (s *IPListService) CountWhitelist(userID string) (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM ip_whitelists WHERE user_id = $1`, userID)
	return count, err
}

func (s *IPListService) WhitelistExists(userID, ip string) (bool, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM ip_whitelists WHERE user_id = $1 AND ip = $2`, userID, ip)
	return count > 0, err
}

// Blocklist operations

func (s *IPListService) ListBlocklist(userID string) ([]models.IPBlocklist, error) {
	var entries []models.IPBlocklist
	err := s.db.Select(&entries, `
		SELECT id, user_id, ip, note, source, created_at
		FROM ip_blocklists
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []models.IPBlocklist{}
	}
	return entries, nil
}

func (s *IPListService) AddBlocklist(userID, ip, note, source string) (*models.IPBlocklist, error) {
	if !isValidIPv4(ip) {
		return nil, fmt.Errorf("invalid IPv4 address")
	}

	count, _ := s.CountBlocklist(userID)
	if count >= MaxBlocklistIPs {
		return nil, fmt.Errorf("maximum %d IPs allowed", MaxBlocklistIPs)
	}

	exists, _ := s.BlocklistExists(userID, ip)
	if exists {
		return nil, fmt.Errorf("IP already blocked")
	}

	// Check if IP is in whitelist
	whitelisted, _ := s.WhitelistExists(userID, ip)
	if whitelisted {
		return nil, fmt.Errorf("IP is whitelisted, remove it first")
	}

	if source == "" {
		source = "manual"
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	entry := &models.IPBlocklist{
		ID:        s.generateID(),
		UserID:    userID,
		IP:        ip,
		Note:      notePtr,
		Source:    source,
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(`
		INSERT INTO ip_blocklists (id, user_id, ip, note, source, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, entry.ID, entry.UserID, entry.IP, entry.Note, entry.Source, entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	go s.pushBlocklistToVPS(userID)

	return entry, nil
}

func (s *IPListService) RemoveBlocklist(userID, id string) error {
	result, err := s.db.Exec(`DELETE FROM ip_blocklists WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("entry not found")
	}

	go s.pushBlocklistToVPS(userID)

	return nil
}

func (s *IPListService) RemoveBlocklistByIP(userID, ip string) error {
	result, err := s.db.Exec(`DELETE FROM ip_blocklists WHERE ip = $1 AND user_id = $2`, ip, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("IP not in blocklist")
	}

	go s.pushBlocklistToVPS(userID)

	return nil
}

func (s *IPListService) CountBlocklist(userID string) (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM ip_blocklists WHERE user_id = $1`, userID)
	return count, err
}

func (s *IPListService) BlocklistExists(userID, ip string) (bool, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM ip_blocklists WHERE user_id = $1 AND ip = $2`, userID, ip)
	return count > 0, err
}

func (s *IPListService) IsBlocked(userID, ip string) bool {
	exists, _ := s.BlocklistExists(userID, ip)
	return exists
}

// VPS Push operations

func (s *IPListService) pushWhitelistToVPS(userID string) {
	if s.servers == nil {
		return
	}

	entries, err := s.ListWhitelist(userID)
	if err != nil {
		log.Printf("Failed to get whitelist for push: %v", err)
		return
	}

	ips := make([]string, len(entries))
	for i, e := range entries {
		ips[i] = e.IP
	}

	data := map[string]interface{}{
		"user_id":    userID,
		"ips":        ips,
		"updated_at": time.Now().Format(time.RFC3339),
	}
	content, _ := json.MarshalIndent(data, "", "  ")

	servers, err := s.servers.GetAllDeployServers()
	if err != nil {
		log.Printf("Failed to get servers for whitelist push: %v", err)
		return
	}

	for _, server := range servers {
		s.pushFileToServer(server, fmt.Sprintf("/etc/botection/whitelists/%s.json", userID), content)
	}
}

func (s *IPListService) pushBlocklistToVPS(userID string) {
	if s.servers == nil {
		return
	}

	entries, err := s.ListBlocklist(userID)
	if err != nil {
		log.Printf("Failed to get blocklist for push: %v", err)
		return
	}

	ips := make([]string, len(entries))
	for i, e := range entries {
		ips[i] = e.IP
	}

	data := map[string]interface{}{
		"user_id":    userID,
		"ips":        ips,
		"updated_at": time.Now().Format(time.RFC3339),
	}
	content, _ := json.MarshalIndent(data, "", "  ")

	servers, err := s.servers.GetAllDeployServers()
	if err != nil {
		log.Printf("Failed to get servers for blocklist push: %v", err)
		return
	}

	for _, server := range servers {
		s.pushFileToServer(server, fmt.Sprintf("/etc/botection/blocklists/%s.json", userID), content)
	}
}

func (s *IPListService) pushFileToServer(server ServerInfo, remotePath string, content []byte) {
	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		log.Printf("SSH connect failed for IP list push to %s: %v", server.IP, err)
		return
	}
	defer client.Close()

	// Ensure directories exist
	client.Run("mkdir -p /etc/botection/whitelists /etc/botection/blocklists")

	// Write file
	cmd := fmt.Sprintf("cat > %s << 'IPLISTEOF'\n%s\nIPLISTEOF", remotePath, string(content))
	if _, err := client.Run(cmd); err != nil {
		log.Printf("Failed to write IP list to %s: %v", server.IP, err)
	}
}

package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/pkg/ssh"
	"github.com/jmoiron/sqlx"
)

var ErrNoServersAvailable = errors.New("no servers available")

type ServerService struct {
	db       *sqlx.DB
	keysPath string
}

func NewServerService(db *sqlx.DB) *ServerService {
	return &ServerService{
		db:       db,
		keysPath: "data/keys",
	}
}

func (s *ServerService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *ServerService) List(userID string) ([]models.Server, error) {
	var servers []models.Server
	err := s.db.Select(&servers,
		`SELECT * FROM servers WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	return servers, err
}

func (s *ServerService) Get(id string) (*models.Server, error) {
	var server models.Server
	err := s.db.Get(&server, `SELECT * FROM servers WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (s *ServerService) Create(userID string, input models.CreateServerInput) (*models.Server, error) {
	id := s.generateID()

	// Determine auth method
	authMethod := input.AuthMethod
	if authMethod == "" {
		if input.SSHPassword != "" {
			authMethod = "password"
		} else {
			authMethod = "key"
		}
	}

	// Generate SSH keypair only if using key auth
	if authMethod == "key" {
		if err := s.generateKeyPair(id); err != nil {
			return nil, fmt.Errorf("generate keypair: %w", err)
		}
	}

	var provider *string
	if input.Provider != "" {
		provider = &input.Provider
	}

	server := &models.Server{
		ID:          id,
		UserID:      userID,
		Name:        input.Name,
		IP:          input.IP,
		Port:        input.Port,
		SSHUser:     input.SSHUser,
		SSHPassword: input.SSHPassword,
		AuthMethod:  authMethod,
		Status:      models.ServerStatusPending,
		Provider:    provider,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := s.db.NamedExec(`
		INSERT INTO servers (id, user_id, name, ip, port, ssh_user, ssh_password, auth_method, status, provider, created_at, updated_at)
		VALUES (:id, :user_id, :name, :ip, :port, :ssh_user, :ssh_password, :auth_method, :status, :provider, :created_at, :updated_at)
	`, server)
	if err != nil {
		return nil, fmt.Errorf("insert server: %w", err)
	}

	return server, nil
}

func (s *ServerService) Update(id string, input models.UpdateServerInput) (*models.Server, error) {
	_, err := s.db.Exec(`
		UPDATE servers SET
			name = COALESCE(NULLIF($2, ''), name),
			ip = COALESCE(NULLIF($3, ''), ip),
			port = COALESCE(NULLIF($4, 0), port),
			ssh_user = COALESCE(NULLIF($5, ''), ssh_user),
			updated_at = NOW()
		WHERE id = $1
	`, id, input.Name, input.IP, input.Port, input.SSHUser)

	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *ServerService) Delete(id string) error {
	// Delete keypair if exists
	os.Remove(s.privateKeyPath(id))
	os.Remove(s.publicKeyPath(id))

	_, err := s.db.Exec(`DELETE FROM servers WHERE id = $1`, id)
	return err
}

func (s *ServerService) SetStatus(id string, status models.ServerStatus) error {
	_, err := s.db.Exec(`UPDATE servers SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	return err
}

// SSH operations

func (s *ServerService) privateKeyPath(id string) string {
	return filepath.Join(s.keysPath, id)
}

func (s *ServerService) publicKeyPath(id string) string {
	return filepath.Join(s.keysPath, id+".pub")
}

func (s *ServerService) generateKeyPair(id string) error {
	os.MkdirAll(s.keysPath, 0700)

	privPath := s.privateKeyPath(id)

	// Use ssh-keygen to generate ed25519 keypair
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", privPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-keygen: %w", err)
	}

	return nil
}

func (s *ServerService) GetPublicKey(id string) (string, error) {
	data, err := os.ReadFile(s.publicKeyPath(id))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *ServerService) GetSSHClient(server *models.Server) (*ssh.Client, error) {
	cfg := ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		TrustOnFirst: true, // TOFU for new servers
	}

	// Use password or key auth based on server config
	if server.AuthMethod == "password" && server.SSHPassword != "" {
		cfg.Password = server.SSHPassword
	} else {
		privateKey, err := os.ReadFile(s.privateKeyPath(server.ID))
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		cfg.PrivateKey = privateKey
	}

	return ssh.NewClient(cfg), nil
}

func (s *ServerService) TestConnection(id string) error {
	server, err := s.Get(id)
	if err != nil {
		return err
	}

	client, err := s.GetSSHClient(server)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.TestConnection(); err != nil {
		s.SetStatus(id, models.ServerStatusDisconnected)
		return err
	}

	s.SetStatus(id, models.ServerStatusReady)
	return nil
}

func (s *ServerService) Exec(id string, input models.ExecInput) (*models.ExecOutput, error) {
	server, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := s.GetSSHClient(server)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Wrap command if running as different user
	command := input.Command
	if input.User != "" && input.User != server.SSHUser {
		command = fmt.Sprintf("sudo -u %s bash -c %s", input.User, ssh.ShellEscape(command))
	}

	result, err := client.Exec(command)
	if err != nil {
		return nil, err
	}

	output := &models.ExecOutput{
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}
	if result.Error != nil {
		output.Error = result.Error.Error()
	}

	// Log the command
	s.Log(id, "exec", fmt.Sprintf("$ %s\n%s", input.Command, result.Output))

	return output, nil
}

// Logging

func (s *ServerService) Log(serverID, logType, content string) error {
	id := s.generateID()
	_, err := s.db.Exec(`
		INSERT INTO server_logs (id, server_id, type, content, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, id, serverID, logType, content)
	return err
}

type LogEntry struct {
	Timestamp time.Time `db:"created_at" json:"timestamp"`
	Level     string    `db:"type" json:"level"`
	Message   string    `db:"content" json:"message"`
}

func (s *ServerService) GetLogs(serverID string, limit int) ([]LogEntry, error) {
	if limit == 0 {
		limit = 50
	}
	var logs []LogEntry
	err := s.db.Select(&logs,
		`SELECT created_at, type, content FROM server_logs WHERE server_id = $1 ORDER BY created_at DESC LIMIT $2`,
		serverID, limit,
	)
	return logs, err
}

// PickRandom returns a random active server from the pool.
// Used when deploying: users never choose a server, the platform does.
func (s *ServerService) PickRandom() (*models.Server, error) {
	var server models.Server
	err := s.db.Get(&server, `
		SELECT * FROM servers
		WHERE status = 'ready'
		ORDER BY RANDOM()
		LIMIT 1
	`)
	if err != nil {
		return nil, ErrNoServersAvailable
	}
	return &server, nil
}

// CountAvailable returns how many servers are ready to deploy to.
func (s *ServerService) CountAvailable() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM servers WHERE status = 'ready'`)
	return count, err
}

// GetDeployIP returns the IP of the first available server for DNS instructions.
func (s *ServerService) GetDeployIP() string {
	var ip string
	err := s.db.Get(&ip, `SELECT ip FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1`)
	if err != nil {
		return ""
	}
	return ip
}

// GetServerForDomain returns SSH connection details for the server a domain is deployed to.
// Used by analytics to push settings files to the VPS.
func (s *ServerService) GetServerForDomain(domainID string) (ip string, port int, user, password string, err error) {
	var server models.Server
	err = s.db.Get(&server, `
		SELECT s.* FROM servers s
		JOIN domains d ON d.server_id = s.id
		WHERE d.id = $1
	`, domainID)
	if err != nil {
		return "", 0, "", "", err
	}
	return server.IP, server.Port, server.SSHUser, server.SSHPassword, nil
}

package services

import (
	"bufio"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/pkg/ssh"
	"github.com/jmoiron/sqlx"
)

//go:embed scripts/deploy-vps-setup.sh
var deployVPSSetupScript string

// ProvisionService handles server provisioning operations
type ProvisionService struct {
	db            *sqlx.DB
	serverService *ServerService
}

// NewProvisionService creates a new provision service
func NewProvisionService(db *sqlx.DB, serverService *ServerService) *ProvisionService {
	return &ProvisionService{
		db:            db,
		serverService: serverService,
	}
}

func (s *ProvisionService) generateID() string {
	b := make([]byte, 18)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Create creates a new provision record
func (s *ProvisionService) Create(input models.CreateProvisionInput) (*models.Provision, error) {
	id := s.generateID()
	now := time.Now()

	provision := &models.Provision{
		ID:          id,
		ServerID:    input.ServerID,
		Status:      models.ProvisionStatusPending,
		CurrentStep: 0,
		TotalSteps:  5,
		StepName:    "",
		AcmeDomain:  input.AcmeDomain,
		CreatedAt:   now,
	}

	_, err := s.db.NamedExec(`
		INSERT INTO server_provisions (id, server_id, status, current_step, total_steps, step_name, acme_domain, created_at)
		VALUES (:id, :server_id, :status, :current_step, :total_steps, :step_name, :acme_domain, :created_at)
	`, provision)
	if err != nil {
		return nil, fmt.Errorf("insert provision: %w", err)
	}

	// Link provision to server
	_, err = s.db.Exec(`UPDATE servers SET provision_id = $1 WHERE id = $2`, id, input.ServerID)
	if err != nil {
		return nil, fmt.Errorf("link provision to server: %w", err)
	}

	return provision, nil
}

// Get fetches a provision by ID
func (s *ProvisionService) Get(id string) (*models.Provision, error) {
	var provision models.Provision
	err := s.db.Get(&provision, `SELECT * FROM server_provisions WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &provision, nil
}

// GetByServerID fetches the active/latest provision for a server
func (s *ProvisionService) GetByServerID(serverID string) (*models.Provision, error) {
	var provision models.Provision
	err := s.db.Get(&provision, `
		SELECT * FROM server_provisions
		WHERE server_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, serverID)
	if err != nil {
		return nil, err
	}
	return &provision, nil
}

// UpdateStep updates the current step and step name
func (s *ProvisionService) UpdateStep(id string, step int, name string) error {
	_, err := s.db.Exec(`
		UPDATE server_provisions
		SET current_step = $2, step_name = $3
		WHERE id = $1
	`, id, step, name)
	return err
}

// SetStatus updates the provision status
func (s *ProvisionService) SetStatus(id string, status models.ProvisionStatus) error {
	query := `UPDATE server_provisions SET status = $2 WHERE id = $1`
	if status == models.ProvisionStatusRunning {
		query = `UPDATE server_provisions SET status = $2, started_at = NOW() WHERE id = $1`
	} else if status == models.ProvisionStatusCompleted || status == models.ProvisionStatusFailed {
		query = `UPDATE server_provisions SET status = $2, completed_at = NOW() WHERE id = $1`
	}
	_, err := s.db.Exec(query, id, status)
	return err
}

// SetError sets the error message
func (s *ProvisionService) SetError(id string, errorMsg string) error {
	_, err := s.db.Exec(`UPDATE server_provisions SET error = $2 WHERE id = $1`, id, errorMsg)
	return err
}

// AppendOutput appends a line to the output
func (s *ProvisionService) AppendOutput(id string, line string) error {
	_, err := s.db.Exec(`
		UPDATE server_provisions
		SET output = COALESCE(output, '') || $2 || E'\n'
		WHERE id = $1
	`, id, line)
	return err
}

// ProvisionEvent represents an event sent during provisioning
type ProvisionEvent struct {
	Type    string      `json:"type"` // step, output, error, complete
	Payload interface{} `json:"payload"`
}

// RunProvision executes the provisioning steps
func (s *ProvisionService) RunProvision(ctx context.Context, provisionID string, eventChan chan<- ProvisionEvent) error {
	defer close(eventChan)

	// Fetch provision
	provision, err := s.Get(provisionID)
	if err != nil {
		return fmt.Errorf("get provision: %w", err)
	}

	// Fetch server
	server, err := s.serverService.Get(provision.ServerID)
	if err != nil {
		return fmt.Errorf("get server: %w", err)
	}

	// Mark as running
	s.SetStatus(provisionID, models.ProvisionStatusRunning)

	// Get SSH client
	client, err := s.serverService.GetSSHClient(server)
	if err != nil {
		s.failProvision(provisionID, eventChan, fmt.Sprintf("SSH client error: %v", err))
		return err
	}
	defer client.Close()

	// Execute steps
	steps := models.ProvisionSteps
	for _, step := range steps {
		select {
		case <-ctx.Done():
			s.failProvision(provisionID, eventChan, "Provisioning cancelled")
			return ctx.Err()
		default:
		}

		// Update step in DB and notify
		s.UpdateStep(provisionID, step.Number, step.Name)
		eventChan <- ProvisionEvent{
			Type: "step",
			Payload: map[string]interface{}{
				"step":  step.Number,
				"total": len(steps),
				"name":  step.Name,
			},
		}

		var stepErr error
		switch step.Number {
		case 1:
			stepErr = s.executeStep1(client, provisionID, eventChan)
		case 2:
			stepErr = s.executeStep2(client, provisionID, provision.AcmeDomain, eventChan)
		case 3:
			stepErr = s.executeStep3(client, provisionID, eventChan)
		case 4:
			stepErr = s.executeStep4(client, provisionID, eventChan)
		case 5:
			stepErr = s.executeStep5(client, provisionID, eventChan)
		}

		if stepErr != nil {
			s.failProvision(provisionID, eventChan, fmt.Sprintf("Step %d failed: %v", step.Number, stepErr))
			return stepErr
		}
	}

	// Mark as completed
	s.SetStatus(provisionID, models.ProvisionStatusCompleted)
	s.serverService.SetStatus(server.ID, models.ServerStatusReady)

	eventChan <- ProvisionEvent{
		Type: "complete",
		Payload: map[string]interface{}{
			"success": true,
		},
	}

	log.Printf("[provision] Server %s provisioned successfully", server.Name)
	return nil
}

func (s *ProvisionService) failProvision(id string, eventChan chan<- ProvisionEvent, errMsg string) {
	s.SetStatus(id, models.ProvisionStatusFailed)
	s.SetError(id, errMsg)
	eventChan <- ProvisionEvent{
		Type: "error",
		Payload: map[string]interface{}{
			"message": errMsg,
		},
	}
	eventChan <- ProvisionEvent{
		Type: "complete",
		Payload: map[string]interface{}{
			"success": false,
		},
	}
}

func (s *ProvisionService) sendOutput(provisionID string, eventChan chan<- ProvisionEvent, line string) {
	// Sanitize credentials from output
	if strings.Contains(line, "ghp_") || strings.Contains(line, "GITHUB_TOKEN") {
		line = "[credential redacted]"
	}
	s.AppendOutput(provisionID, line)
	eventChan <- ProvisionEvent{
		Type: "output",
		Payload: map[string]interface{}{
			"line": line,
		},
	}
}

// Step 1: Test SSH connection
func (s *ProvisionService) executeStep1(client *ssh.Client, provisionID string, eventChan chan<- ProvisionEvent) error {
	s.sendOutput(provisionID, eventChan, "Testing SSH connection...")
	if err := client.TestConnection(); err != nil {
		return fmt.Errorf("SSH connection test failed: %w", err)
	}
	s.sendOutput(provisionID, eventChan, "SSH connection successful")
	return nil
}

// Step 2: Run deploy-vps-setup.sh
func (s *ProvisionService) executeStep2(client *ssh.Client, provisionID string, acmeDomain string, eventChan chan<- ProvisionEvent) error {
	s.sendOutput(provisionID, eventChan, "Running base infrastructure setup...")
	s.sendOutput(provisionID, eventChan, fmt.Sprintf("ACME_DOMAIN=%s", acmeDomain))

	cmd := fmt.Sprintf("ACME_DOMAIN=%s bash -s", acmeDomain)
	result, err := client.ExecWithStdin(cmd, deployVPSSetupScript)
	if err != nil {
		return fmt.Errorf("deploy-vps-setup.sh failed: %w", err)
	}

	// Stream output lines
	scanner := bufio.NewScanner(strings.NewReader(result.Output))
	for scanner.Scan() {
		s.sendOutput(provisionID, eventChan, scanner.Text())
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("deploy-vps-setup.sh exited with code %d", result.ExitCode)
	}

	s.sendOutput(provisionID, eventChan, "Base infrastructure setup complete")
	return nil
}

// Step 3: Clone antibot repository
func (s *ProvisionService) executeStep3(client *ssh.Client, provisionID string, eventChan chan<- ProvisionEvent) error {
	s.sendOutput(provisionID, eventChan, "Cloning antibot repository...")

	// The deploy.sh has the GitHub token hardcoded, so we just clone with HTTPS
	// First check if already exists
	checkCmd := "test -d /var/www/antibot && echo 'exists' || echo 'not_exists'"
	checkResult, _ := client.Exec(checkCmd)
	if strings.TrimSpace(checkResult.Output) == "exists" {
		s.sendOutput(provisionID, eventChan, "Antibot directory already exists, pulling latest...")
		pullCmd := "cd /var/www/antibot && git pull"
		result, err := client.Exec(pullCmd)
		if err != nil {
			return fmt.Errorf("git pull failed: %w", err)
		}
		s.sendOutput(provisionID, eventChan, result.Output)
	} else {
		// Clone fresh using ANTIBOT_GITHUB_TOKEN env var
		token := os.Getenv("ANTIBOT_GITHUB_TOKEN")
		if token == "" {
			return fmt.Errorf("ANTIBOT_GITHUB_TOKEN environment variable not set")
		}
		cloneCmd := fmt.Sprintf("git clone https://%s@github.com/drales2083/antibot.git /var/www/antibot", token)
		result, err := client.Exec(cloneCmd)
		if err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
		// Don't output the clone command (has token), just the result
		if result.ExitCode != 0 {
			return fmt.Errorf("git clone exited with code %d: %s", result.ExitCode, result.Output)
		}
		s.sendOutput(provisionID, eventChan, "Repository cloned successfully")
	}

	return nil
}

// Step 4: Run botection deploy.sh
func (s *ProvisionService) executeStep4(client *ssh.Client, provisionID string, eventChan chan<- ProvisionEvent) error {
	s.sendOutput(provisionID, eventChan, "Installing botection...")

	cmd := "cd /var/www/antibot && SERVER_TYPE=templates bash packaging/deploy.sh"
	result, err := client.Exec(cmd)
	if err != nil {
		return fmt.Errorf("botection deploy failed: %w", err)
	}

	// Stream output lines
	scanner := bufio.NewScanner(strings.NewReader(result.Output))
	for scanner.Scan() {
		s.sendOutput(provisionID, eventChan, scanner.Text())
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("botection deploy exited with code %d", result.ExitCode)
	}

	s.sendOutput(provisionID, eventChan, "Botection installation complete")
	return nil
}

// Step 5: Verify services
func (s *ProvisionService) executeStep5(client *ssh.Client, provisionID string, eventChan chan<- ProvisionEvent) error {
	s.sendOutput(provisionID, eventChan, "Verifying services...")

	services := []string{"nginx", "redis-server", "botection"}
	for _, svc := range services {
		cmd := fmt.Sprintf("systemctl is-active %s", svc)
		result, err := client.Exec(cmd)
		if err != nil || strings.TrimSpace(result.Output) != "active" {
			return fmt.Errorf("service %s is not active", svc)
		}
		s.sendOutput(provisionID, eventChan, fmt.Sprintf("  %s: active", svc))
	}

	s.sendOutput(provisionID, eventChan, "All services verified")
	return nil
}


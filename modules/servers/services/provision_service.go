package services

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/pkg/ssh"
	"github.com/jmoiron/sqlx"
)

//go:embed scripts/deploy-vps-setup.sh
var deployVPSSetupScript string

// ptrStr returns a pointer to the given string
func ptrStr(s string) *string {
	return &s
}

// ProvisionService handles server provisioning operations
type ProvisionService struct {
	db            *sqlx.DB
	serverService *ServerService

	// Global registry of active provisions
	mu     sync.RWMutex
	active map[string]context.CancelFunc // provisionID -> cancel function
}

// NewProvisionService creates a new provision service
func NewProvisionService(db *sqlx.DB, serverService *ServerService) *ProvisionService {
	return &ProvisionService{
		db:            db,
		serverService: serverService,
		active:        make(map[string]context.CancelFunc),
	}
}

func (s *ProvisionService) generateID() string {
	b := make([]byte, 18)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// IsActive checks if a provision is currently being executed
func (s *ProvisionService) IsActive(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.active[id]
	return exists
}

// registerActive marks a provision as actively running
func (s *ProvisionService) registerActive(id string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[id] = cancel
}

// unregisterActive removes a provision from the active registry
func (s *ProvisionService) unregisterActive(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, id)
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
		StepName:    ptrStr(""),
		AcmeDomain:  ptrStr(input.AcmeDomain),
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

// GetOutput fetches only the output field (for streaming)
func (s *ProvisionService) GetOutput(id string) (string, error) {
	var output *string
	err := s.db.Get(&output, `SELECT output FROM server_provisions WHERE id = $1`, id)
	if err != nil {
		return "", err
	}
	if output == nil {
		return "", nil
	}
	return *output, nil
}

// StartProvision starts provisioning in a background goroutine.
// Returns immediately. The job runs independently of HTTP connections.
// Safe to call multiple times - only starts if not already running.
func (s *ProvisionService) StartProvision(provisionID string) error {
	// Check if already actively running (in memory)
	if s.IsActive(provisionID) {
		log.Printf("[Provision] %s already active in memory, skipping start", provisionID)
		return nil
	}

	// Get provision to check status
	provision, err := s.Get(provisionID)
	if err != nil {
		return fmt.Errorf("get provision: %w", err)
	}

	// If completed or failed, don't restart
	if provision.Status == models.ProvisionStatusCompleted || provision.Status == models.ProvisionStatusFailed {
		return nil
	}

	// If status is "running" but not in active map, it's stuck from a previous server instance
	if provision.Status == models.ProvisionStatusRunning {
		// Check if started more than 2 minutes ago (stuck)
		if provision.StartedAt != nil {
			stuckThreshold := time.Now().Add(-2 * time.Minute)
			if provision.StartedAt.Before(stuckThreshold) {
				log.Printf("[Provision] %s is stuck (started %v ago), resetting", provisionID, time.Since(*provision.StartedAt))
				s.SetStatus(provisionID, models.ProvisionStatusPending)
				s.UpdateStep(provisionID, 0, "")
				s.AppendOutput(provisionID, "\n[System] Detected stuck provision, restarting...")
			} else {
				// Started recently but not in memory - race condition or immediate restart
				// Wait a bit and check again
				log.Printf("[Provision] %s is running but not in memory, may be starting up", provisionID)
				return nil
			}
		}
	}

	// Create a background context (not tied to any HTTP request)
	ctx, cancel := context.WithCancel(context.Background())

	// Register as active
	s.registerActive(provisionID, cancel)

	// Start the actual work in a goroutine
	go func() {
		defer s.unregisterActive(provisionID)
		s.runProvisionInternal(ctx, provisionID)
	}()

	log.Printf("[Provision] Started background job for %s", provisionID)
	return nil
}

// runProvisionInternal executes the provisioning steps
func (s *ProvisionService) runProvisionInternal(ctx context.Context, provisionID string) {
	// Fetch provision
	provision, err := s.Get(provisionID)
	if err != nil {
		log.Printf("[Provision] Get provision %s failed: %v", provisionID, err)
		s.failProvision(provisionID, fmt.Sprintf("Get provision failed: %v", err))
		return
	}

	// Fetch server
	server, err := s.serverService.Get(provision.ServerID)
	if err != nil {
		log.Printf("[Provision] Get server for provision %s failed: %v", provisionID, err)
		s.failProvision(provisionID, fmt.Sprintf("Get server failed: %v", err))
		return
	}

	// Mark as running
	s.SetStatus(provisionID, models.ProvisionStatusRunning)
	s.AppendOutput(provisionID, fmt.Sprintf("Starting provisioning for %s (%s)...", server.Name, server.IP))

	// Get SSH client
	client, err := s.serverService.GetSSHClient(server)
	if err != nil {
		s.failProvision(provisionID, fmt.Sprintf("SSH connection failed: %v", err))
		return
	}
	defer client.Close()

	// Execute steps
	steps := models.ProvisionSteps
	for _, step := range steps {
		select {
		case <-ctx.Done():
			s.failProvision(provisionID, "Provisioning cancelled")
			return
		default:
		}

		// Update step in DB
		s.UpdateStep(provisionID, step.Number, step.Name)
		s.AppendOutput(provisionID, fmt.Sprintf("\n=== Step %d/%d: %s ===", step.Number, len(steps), step.Name))

		var stepErr error
		acmeDomain := ""
		if provision.AcmeDomain != nil {
			acmeDomain = *provision.AcmeDomain
		}
		switch step.Number {
		case 1:
			stepErr = s.executeStep1(client, provisionID)
		case 2:
			stepErr = s.executeStep2(client, provisionID, acmeDomain)
		case 3:
			stepErr = s.executeStep3(client, provisionID)
		case 4:
			stepErr = s.executeStep4(client, provisionID)
		case 5:
			stepErr = s.executeStep5(client, provisionID)
		}

		if stepErr != nil {
			s.failProvision(provisionID, fmt.Sprintf("Step %d (%s) failed: %v", step.Number, step.Name, stepErr))
			return
		}

		s.AppendOutput(provisionID, fmt.Sprintf("Step %d completed successfully", step.Number))
	}

	// Mark as completed
	s.SetStatus(provisionID, models.ProvisionStatusCompleted)
	s.serverService.SetStatus(server.ID, models.ServerStatusReady)
	s.AppendOutput(provisionID, "\n=== Provisioning completed successfully! ===")

	log.Printf("[Provision] Server %s provisioned successfully", server.Name)
}

func (s *ProvisionService) failProvision(id string, errMsg string) {
	s.SetStatus(id, models.ProvisionStatusFailed)
	s.SetError(id, errMsg)
	s.AppendOutput(id, fmt.Sprintf("\n[ERROR] %s", errMsg))
	log.Printf("[Provision] %s failed: %s", id, errMsg)
}

func (s *ProvisionService) sendOutput(provisionID string, line string) {
	// Sanitize credentials from output
	if strings.Contains(line, "ghp_") || strings.Contains(line, "GITHUB_TOKEN") {
		line = "[credential redacted]"
	}
	s.AppendOutput(provisionID, line)
}

// streamWriter implements io.Writer and streams output lines to DB
type streamWriter struct {
	service     *ProvisionService
	provisionID string
	buffer      []byte
}

func (w *streamWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	// Process complete lines
	for {
		idx := strings.Index(string(w.buffer), "\n")
		if idx < 0 {
			break
		}
		line := string(w.buffer[:idx])
		w.buffer = w.buffer[idx+1:]
		if line != "" {
			w.service.sendOutput(w.provisionID, line)
		}
	}
	return len(p), nil
}

func (w *streamWriter) Flush() {
	// Flush any remaining content in buffer
	if len(w.buffer) > 0 {
		line := string(w.buffer)
		if line != "" {
			w.service.sendOutput(w.provisionID, line)
		}
		w.buffer = nil
	}
}

// Step 1: Test SSH connection
func (s *ProvisionService) executeStep1(client *ssh.Client, provisionID string) error {
	s.sendOutput(provisionID, "Testing SSH connection...")
	if err := client.TestConnection(); err != nil {
		return fmt.Errorf("SSH connection test failed: %w", err)
	}
	s.sendOutput(provisionID, "SSH connection successful")
	return nil
}

// Step 2: Run deploy-vps-setup.sh
func (s *ProvisionService) executeStep2(client *ssh.Client, provisionID string, acmeDomain string) error {
	s.sendOutput(provisionID, "Running base infrastructure setup...")
	s.sendOutput(provisionID, fmt.Sprintf("ACME_DOMAIN=%s", acmeDomain))

	// Upload script to server first, then execute with streaming
	scriptPath := "/tmp/deploy-vps-setup.sh"
	if err := client.WriteFile(scriptPath, []byte(deployVPSSetupScript), 0755); err != nil {
		return fmt.Errorf("upload setup script: %w", err)
	}

	// Execute with streaming output
	writer := &streamWriter{
		service:     s,
		provisionID: provisionID,
	}
	cmd := fmt.Sprintf("ACME_DOMAIN=%s bash %s 2>&1", acmeDomain, scriptPath)
	err := client.ExecStream(cmd, writer, writer)
	writer.Flush()

	// Cleanup
	client.Exec("rm -f " + scriptPath)

	if err != nil {
		return fmt.Errorf("deploy-vps-setup.sh failed: %w", err)
	}

	s.sendOutput(provisionID, "Base infrastructure setup complete")
	return nil
}

// Step 3: Clone antibot repository
func (s *ProvisionService) executeStep3(client *ssh.Client, provisionID string) error {
	s.sendOutput(provisionID, "Cloning antibot repository...")

	// The deploy.sh has the GitHub token hardcoded, so we just clone with HTTPS
	// First check if already exists
	checkCmd := "test -d /var/www/antibot && echo 'exists' || echo 'not_exists'"
	checkResult, _ := client.Exec(checkCmd)
	if strings.TrimSpace(checkResult.Output) == "exists" {
		s.sendOutput(provisionID, "Antibot directory already exists, pulling latest...")
		pullCmd := "cd /var/www/antibot && git pull"
		result, err := client.Exec(pullCmd)
		if err != nil {
			return fmt.Errorf("git pull failed: %w", err)
		}
		s.sendOutput(provisionID, result.Output)
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
		s.sendOutput(provisionID, "Repository cloned successfully")
	}

	return nil
}

// Step 4: Run botection deploy.sh
func (s *ProvisionService) executeStep4(client *ssh.Client, provisionID string) error {
	s.sendOutput(provisionID, "Installing botection...")

	// Execute with streaming output
	writer := &streamWriter{
		service:     s,
		provisionID: provisionID,
	}
	cmd := "cd /var/www/antibot && SERVER_TYPE=templates bash packaging/deploy.sh 2>&1"
	err := client.ExecStream(cmd, writer, writer)
	writer.Flush()

	if err != nil {
		return fmt.Errorf("botection deploy failed: %w", err)
	}

	s.sendOutput(provisionID, "Botection installation complete")
	return nil
}

// Step 5: Verify services
func (s *ProvisionService) executeStep5(client *ssh.Client, provisionID string) error {
	s.sendOutput(provisionID, "Verifying services...")

	services := []string{"nginx", "redis-server", "botection"}
	for _, svc := range services {
		cmd := fmt.Sprintf("systemctl is-active %s", svc)
		result, err := client.Exec(cmd)
		if err != nil || strings.TrimSpace(result.Output) != "active" {
			return fmt.Errorf("service %s is not active", svc)
		}
		s.sendOutput(provisionID, fmt.Sprintf("  %s: active", svc))
	}

	s.sendOutput(provisionID, "All services verified")
	return nil
}

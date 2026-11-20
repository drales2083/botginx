package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/botginx/botginx/modules/servers/models"
	"github.com/jmoiron/sqlx"
)

// ReconfigureResult holds the result of a DNS update for a single domain
type ReconfigureResult struct {
	Domain  string `json:"domain"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ReconfigureService handles server DNS reconfiguration operations
type ReconfigureService struct {
	db            *sqlx.DB
	serverService *ServerService

	// Callback to domains module for the actual reconfiguration
	onReconfigureAll func(serverIP string, progressFn func(provider string, results []ReconfigureResult)) error

	// Callback to domains module for assigning server to all domains
	onAssignServer func(serverID string) (int, error)

	// Callback to domains module for resetting all domains
	onResetDomains func() (int, error)

	// Global registry of active reconfigurations
	mu     sync.RWMutex
	active map[string]context.CancelFunc
}

// NewReconfigureService creates a new reconfigure service
func NewReconfigureService(db *sqlx.DB, serverService *ServerService) *ReconfigureService {
	return &ReconfigureService{
		db:            db,
		serverService: serverService,
		active:        make(map[string]context.CancelFunc),
	}
}

// SetOnReconfigureAll registers the callback for DNS reconfiguration
func (s *ReconfigureService) SetOnReconfigureAll(fn func(serverIP string, progressFn func(provider string, results []ReconfigureResult)) error) {
	s.onReconfigureAll = fn
}

// SetOnAssignServer registers the callback for assigning server to all domains
func (s *ReconfigureService) SetOnAssignServer(fn func(serverID string) (int, error)) {
	s.onAssignServer = fn
}

// SetOnResetDomains registers the callback for resetting all domains
func (s *ReconfigureService) SetOnResetDomains(fn func() (int, error)) {
	s.onResetDomains = fn
}

func (s *ReconfigureService) generateID() string {
	b := make([]byte, 18)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// IsActive checks if a reconfiguration is currently being executed
func (s *ReconfigureService) IsActive(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.active[id]
	return exists
}

func (s *ReconfigureService) registerActive(id string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[id] = cancel
}

func (s *ReconfigureService) unregisterActive(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, id)
}

// Create creates a new reconfigure record
func (s *ReconfigureService) Create(input models.CreateReconfigureInput) (*models.Reconfigure, error) {
	id := s.generateID()
	now := time.Now()

	stepName := ""
	reconfig := &models.Reconfigure{
		ID:          id,
		ServerID:    input.ServerID,
		Status:      models.ReconfigureStatusPending,
		CurrentStep: 0,
		TotalSteps:  4,
		StepName:    &stepName,
		CreatedAt:   now,
	}

	_, err := s.db.NamedExec(`
		INSERT INTO server_reconfigurations (id, server_id, status, current_step, total_steps, step_name, created_at)
		VALUES (:id, :server_id, :status, :current_step, :total_steps, :step_name, :created_at)
	`, reconfig)
	if err != nil {
		return nil, fmt.Errorf("insert reconfiguration: %w", err)
	}

	return reconfig, nil
}

// Get fetches a reconfigure job by ID
func (s *ReconfigureService) Get(id string) (*models.Reconfigure, error) {
	var reconfig models.Reconfigure
	err := s.db.Get(&reconfig, `SELECT * FROM server_reconfigurations WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &reconfig, nil
}

// GetByServerID fetches the latest reconfigure for a server
func (s *ReconfigureService) GetByServerID(serverID string) (*models.Reconfigure, error) {
	var reconfig models.Reconfigure
	err := s.db.Get(&reconfig, `
		SELECT * FROM server_reconfigurations
		WHERE server_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, serverID)
	if err != nil {
		return nil, err
	}
	return &reconfig, nil
}

// UpdateStep updates the current step and step name
func (s *ReconfigureService) UpdateStep(id string, step int, name string) error {
	_, err := s.db.Exec(`
		UPDATE server_reconfigurations
		SET current_step = $2, step_name = $3
		WHERE id = $1
	`, id, step, name)
	return err
}

// SetStatus updates the reconfigure status
func (s *ReconfigureService) SetStatus(id string, status models.ReconfigureStatus) error {
	query := `UPDATE server_reconfigurations SET status = $2 WHERE id = $1`
	if status == models.ReconfigureStatusRunning {
		query = `UPDATE server_reconfigurations SET status = $2, started_at = NOW() WHERE id = $1`
	} else if status == models.ReconfigureStatusCompleted || status == models.ReconfigureStatusFailed {
		query = `UPDATE server_reconfigurations SET status = $2, completed_at = NOW() WHERE id = $1`
	}
	_, err := s.db.Exec(query, id, status)
	return err
}

// SetError sets the error message
func (s *ReconfigureService) SetError(id string, errorMsg string) error {
	_, err := s.db.Exec(`UPDATE server_reconfigurations SET error = $2 WHERE id = $1`, id, errorMsg)
	return err
}

// AppendOutput appends a line to the output
func (s *ReconfigureService) AppendOutput(id string, line string) error {
	_, err := s.db.Exec(`
		UPDATE server_reconfigurations
		SET output = COALESCE(output, '') || $2 || E'\n'
		WHERE id = $1
	`, id, line)
	return err
}

// GetOutput fetches only the output field (for streaming)
func (s *ReconfigureService) GetOutput(id string) (string, error) {
	var output *string
	err := s.db.Get(&output, `SELECT output FROM server_reconfigurations WHERE id = $1`, id)
	if err != nil {
		return "", err
	}
	if output == nil {
		return "", nil
	}
	return *output, nil
}

// StartReconfigure starts reconfiguration in a background goroutine.
// Returns immediately. The job runs independently of HTTP connections.
func (s *ReconfigureService) StartReconfigure(reconfigID string) error {
	if s.IsActive(reconfigID) {
		log.Printf("[Reconfigure] %s already active in memory, skipping start", reconfigID)
		return nil
	}

	reconfig, err := s.Get(reconfigID)
	if err != nil {
		return fmt.Errorf("get reconfigure record: %w", err)
	}

	if reconfig.Status == models.ReconfigureStatusCompleted || reconfig.Status == models.ReconfigureStatusFailed {
		log.Printf("[Reconfigure] %s already finished (status=%s), skipping start", reconfigID, reconfig.Status)
		return nil
	}

	server, err := s.serverService.Get(reconfig.ServerID)
	if err != nil {
		return fmt.Errorf("get server: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.registerActive(reconfigID, cancel)

	go func() {
		defer s.unregisterActive(reconfigID)
		s.runReconfigure(ctx, reconfigID, server)
	}()

	return nil
}

// runReconfigure performs the actual reconfiguration
func (s *ReconfigureService) runReconfigure(ctx context.Context, reconfigID string, server *models.Server) {
	log.Printf("[Reconfigure] Starting reconfiguration for server %s (IP: %s)", server.Name, server.IP)

	s.SetStatus(reconfigID, models.ReconfigureStatusRunning)
	s.AppendOutput(reconfigID, fmt.Sprintf("> Starting DNS reconfiguration for server: %s", server.Name))
	s.AppendOutput(reconfigID, fmt.Sprintf("> Target IP: %s", server.IP))
	s.AppendOutput(reconfigID, "")

	// Step 1: Reset all domains
	s.UpdateStep(reconfigID, 1, "Reset All Domains")
	s.AppendOutput(reconfigID, "[Step 1/4] Resetting all domains...")

	if s.onResetDomains == nil {
		s.failReconfigure(reconfigID, "Reset domains callback not configured")
		return
	}

	resetCount, err := s.onResetDomains()
	if err != nil {
		s.failReconfigure(reconfigID, fmt.Sprintf("Failed to reset domains: %v", err))
		return
	}
	s.AppendOutput(reconfigID, fmt.Sprintf("  ✓ Reset %d domains (dns_verified=false, ssl_enabled=false, sync_status=pending, setup_step=dns)", resetCount))
	s.AppendOutput(reconfigID, "")

	// Step 2: Assign server to all domains
	s.UpdateStep(reconfigID, 2, "Assign Server to All Domains")
	s.AppendOutput(reconfigID, "[Step 2/4] Assigning server to all domains...")

	if s.onAssignServer == nil {
		s.failReconfigure(reconfigID, "Assign server callback not configured")
		return
	}

	assignCount, err := s.onAssignServer(server.ID)
	if err != nil {
		s.failReconfigure(reconfigID, fmt.Sprintf("Failed to assign server: %v", err))
		return
	}
	s.AppendOutput(reconfigID, fmt.Sprintf("  ✓ Assigned server %s to %d domains", server.ID[:12]+"...", assignCount))
	s.AppendOutput(reconfigID, "")

	// Step 3: Update DNS for API-connected domains
	s.UpdateStep(reconfigID, 3, "Update DNS (API Providers)")
	s.AppendOutput(reconfigID, "[Step 3/4] Updating DNS for API-connected domains...")

	if s.onReconfigureAll == nil {
		s.failReconfigure(reconfigID, "Reconfigure callback not configured")
		return
	}

	var totalUpdated, totalFailed int
	var manualDomains []string

	progressFn := func(provider string, results []ReconfigureResult) {
		for _, r := range results {
			if r.Success {
				totalUpdated++
				s.AppendOutput(reconfigID, fmt.Sprintf("  ✓ [%s] %s", provider, r.Domain))
			} else {
				totalFailed++
				s.AppendOutput(reconfigID, fmt.Sprintf("  ✗ [%s] %s: %s", provider, r.Domain, r.Error))
			}
		}
	}

	err = s.onReconfigureAll(server.IP, progressFn)
	if err != nil {
		s.AppendOutput(reconfigID, fmt.Sprintf("  ⚠ Some providers had errors: %v", err))
	}

	s.AppendOutput(reconfigID, "")
	s.AppendOutput(reconfigID, fmt.Sprintf("  API-updated: %d domains", totalUpdated))
	if totalFailed > 0 {
		s.AppendOutput(reconfigID, fmt.Sprintf("  Failed: %d domains", totalFailed))
	}
	s.AppendOutput(reconfigID, "")

	// Step 4: Complete
	s.UpdateStep(reconfigID, 4, "Complete")
	s.AppendOutput(reconfigID, "[Step 4/4] Summary")
	s.AppendOutput(reconfigID, "")
	s.AppendOutput(reconfigID, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	s.AppendOutput(reconfigID, fmt.Sprintf("Total domains reset: %d", resetCount))
	s.AppendOutput(reconfigID, fmt.Sprintf("Server assigned to: %d domains", assignCount))
	s.AppendOutput(reconfigID, fmt.Sprintf("DNS auto-updated: %d domains", totalUpdated))
	if len(manualDomains) > 0 {
		s.AppendOutput(reconfigID, "")
		s.AppendOutput(reconfigID, "Manual/external domains (need manual DNS update):")
		for _, d := range manualDomains {
			s.AppendOutput(reconfigID, fmt.Sprintf("  - %s", d))
		}
	}
	s.AppendOutput(reconfigID, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	s.AppendOutput(reconfigID, "")
	s.AppendOutput(reconfigID, "✓ DomainSync will verify DNS and generate SSL for all domains in the background.")

	s.SetStatus(reconfigID, models.ReconfigureStatusCompleted)
	log.Printf("[Reconfigure] Completed reconfiguration for server %s", server.Name)
}

func (s *ReconfigureService) failReconfigure(id string, errMsg string) {
	log.Printf("[Reconfigure] Failed: %s", errMsg)
	s.AppendOutput(id, "")
	s.AppendOutput(id, fmt.Sprintf("✗ Error: %s", errMsg))
	s.SetError(id, errMsg)
	s.SetStatus(id, models.ReconfigureStatusFailed)
}

package services

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/botginx/botginx/modules/modules/models"
	"github.com/botginx/botginx/pkg/module"
	"github.com/jmoiron/sqlx"
)

// Core modules that cannot be disabled. Users is core because disabling it
// would remove the only way to manage accounts and subscriptions -- including
// the admin's own.
var coreModules = map[string]bool{
	"auth":      true,
	"dashboard": true,
	"modules":   true,
	"users":     true,
}

type ModuleService struct {
	db       *sqlx.DB
	registry *module.Registry
}

func NewModuleService(db *sqlx.DB, registry *module.Registry) *ModuleService {
	return &ModuleService{
		db:       db,
		registry: registry,
	}
}

func (s *ModuleService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SyncModules ensures all registered modules are tracked in database
func (s *ModuleService) SyncModules() error {
	for _, m := range s.registry.All() {
		var count int
		s.db.Get(&count, "SELECT COUNT(*) FROM module_states WHERE module_id = $1", m.ID())

		if count == 0 {
			state := &models.ModuleState{
				ID:          s.generateID(),
				ModuleID:    m.ID(),
				Name:        m.Name(),
				Description: m.Description(),
				Version:     "1.0.0",
				IsEnabled:   true,
				IsCore:      coreModules[m.ID()],
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			s.db.NamedExec(`
				INSERT INTO module_states (id, module_id, name, description, version, is_enabled, is_core, settings, created_at, updated_at)
				VALUES (:id, :module_id, :name, :description, :version, :is_enabled, :is_core, '', :created_at, :updated_at)
			`, state)
			continue
		}

		// Refresh the fields that come from code rather than from an admin's
		// choices, so changing which modules are core takes effect on restart
		// instead of only for installs that have never run before.
		s.db.Exec(`
			UPDATE module_states
			SET name = $2, description = $3, is_core = $4, updated_at = NOW()
			WHERE module_id = $1
		`, m.ID(), m.Name(), m.Description(), coreModules[m.ID()])
	}
	return nil
}

func (s *ModuleService) List() ([]models.ModuleInfo, error) {
	// settings and description are nullable and are not set on insert, so they
	// are coalesced -- scanning NULL into a string field fails the whole query.
	var states []models.ModuleState
	err := s.db.Select(&states, `
		SELECT id, module_id, name,
		       COALESCE(description, '') AS description,
		       COALESCE(version, '')     AS version,
		       is_enabled, is_core,
		       COALESCE(settings, '')    AS settings,
		       created_at, updated_at
		FROM module_states
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}

	var modules []models.ModuleInfo
	for _, state := range states {
		info := models.ModuleInfo{
			ID:          state.ModuleID,
			Name:        state.Name,
			Description: state.Description,
			Version:     state.Version,
			IsEnabled:   state.IsEnabled,
			IsCore:      state.IsCore,
		}

		// Get additional info from registry
		if m, ok := s.registry.Get(state.ModuleID); ok {
			info.HasRoutes = m.Routes() != nil
			info.HasWidgets = len(m.Widgets()) > 0
			if items := m.MenuItems(); len(items) > 0 {
				info.MenuSection = string(items[0].Section)
			}
		}

		modules = append(modules, info)
	}

	return modules, nil
}

func (s *ModuleService) Get(moduleID string) (*models.ModuleState, error) {
	var state models.ModuleState
	err := s.db.Get(&state, `
		SELECT id, module_id, name,
		       COALESCE(description, '') AS description,
		       COALESCE(version, '')     AS version,
		       is_enabled, is_core,
		       COALESCE(settings, '')    AS settings,
		       created_at, updated_at
		FROM module_states
		WHERE module_id = $1
	`, moduleID)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *ModuleService) Enable(moduleID string) error {
	_, err := s.db.Exec(`
		UPDATE module_states SET is_enabled = true, updated_at = NOW() WHERE module_id = $1
	`, moduleID)
	return err
}

func (s *ModuleService) Disable(moduleID string) error {
	// Check if core module
	var isCore bool
	s.db.Get(&isCore, "SELECT is_core FROM module_states WHERE module_id = $1", moduleID)
	if isCore {
		return nil // Silently ignore
	}

	_, err := s.db.Exec(`
		UPDATE module_states SET is_enabled = false, updated_at = NOW() WHERE module_id = $1
	`, moduleID)
	return err
}

func (s *ModuleService) IsEnabled(moduleID string) bool {
	var enabled bool
	err := s.db.Get(&enabled, "SELECT is_enabled FROM module_states WHERE module_id = $1", moduleID)
	if err != nil {
		return true // Default to enabled if not found
	}
	return enabled
}

func (s *ModuleService) UpdateSettings(moduleID, settings string) error {
	_, err := s.db.Exec(`
		UPDATE module_states SET settings = $2, updated_at = NOW() WHERE module_id = $1
	`, moduleID, settings)
	return err
}

func (s *ModuleService) Count() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM module_states`)
	return count, err
}

func (s *ModuleService) CountEnabled() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM module_states WHERE is_enabled = true`)
	return count, err
}

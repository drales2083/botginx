package backup

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/botginx/botginx/modules/backup/handlers"
	"github.com/botginx/botginx/modules/backup/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service  *services.BackupService
	handler  *handlers.Handler
	stopChan chan struct{}
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"backup",
			"Backup",
			"Telegram backup system",
		),
		stopChan: make(chan struct{}),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	appDir := os.Getenv("APP_DIR")
	if appDir == "" {
		appDir = "/opt/botginx"
	}

	m.service = services.NewBackupService(deps.DB, appDir)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	go m.startScheduler()

	return nil
}

func (m *Module) Migrate() error {
	migrations := []string{
		"migrations/001_create_tables.sql",
		"migrations/002_add_backup_name.sql",
	}
	for _, file := range migrations {
		sql, err := migrationsFS.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := m.DB().Exec(string(sql)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()

		r.Get("/", m.handler.Settings)
		r.Get("/history", m.handler.History)
		r.Get("/restore", m.handler.Restore)

		r.Route("/api", func(r chi.Router) {
			r.Post("/settings", m.handler.APIUpdateSettings)
			r.Post("/run", m.handler.APIRunBackup)
			r.Get("/history", m.handler.APIGetHistory)
			r.Post("/restore/validate", m.handler.APIValidateManifest)
			r.Post("/restore/run", m.handler.APIRunRestore)
		})

		return r
	}
	return chi.NewRouter()
}

func (m *Module) Routes() chi.Router {
	return chi.NewRouter()
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Backup",
			Icon:    "bi-cloud-arrow-up",
			Path:    "/admin/backup",
			Order:   98, // Before Settings
			Section: module.MenuSectionAdmin,
		},
	}
}

func (m *Module) startScheduler() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndRunBackup()
		case <-m.stopChan:
			return
		}
	}
}

func (m *Module) checkAndRunBackup() {
	settings, err := m.service.GetSettings()
	if err != nil || !settings.Enabled {
		return
	}

	history, err := m.service.GetHistory(1)
	if err != nil {
		return
	}

	interval := time.Duration(settings.IntervalHours) * time.Hour

	if len(history) == 0 || time.Since(history[0].StartedAt) >= interval {
		log.Println("[backup] Starting scheduled backup...")
		if _, err := m.service.RunBackup(context.Background()); err != nil {
			log.Printf("[backup] Scheduled backup failed: %v", err)
		} else {
			log.Println("[backup] Scheduled backup completed")
		}
	}
}

func (m *Module) Stop() {
	close(m.stopChan)
}

func (m *Module) Service() *services.BackupService {
	return m.service
}

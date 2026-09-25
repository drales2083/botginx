package twofactor

import (
	"crypto/rand"
	"embed"
	"encoding/base32"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/pquerna/otp/totp"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	templates *module.TemplateEngine
	appName   string
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"twofactor",
			"Two-Factor Auth",
			"Two-factor authentication with TOTP",
		),
		appName: "GuardBot",
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)
	m.templates = deps.Templates

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_add_2fa_columns.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", m.handlePage)
	r.Post("/api/setup", m.apiSetup)
	r.Post("/api/enable", m.apiEnable)
	r.Post("/api/disable", m.apiDisable)
	r.Post("/api/regenerate-backup", m.apiRegenerateBackup)
	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Two-Factor Auth",
			Icon:    "bi-shield-lock",
			Path:    "/user/twofactor",
			Order:   25,
			Section: module.MenuSectionUser,
			Group:   "Account",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

type twoFactorStatus struct {
	Enabled   bool      `db:"totp_enabled"`
	Secret    *string   `db:"totp_secret"`
	EnabledAt *time.Time `db:"totp_enabled_at"`
}

func (m *Module) handlePage(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	var status twoFactorStatus
	m.DB().Get(&status, `SELECT totp_enabled, totp_secret, totp_enabled_at FROM users WHERE id = $1`, user.ID)

	module.RenderUserSection(w, r, m.templates, "twofactor:index.html", map[string]interface{}{
		"Title":     "Two-Factor Authentication",
		"Enabled":   status.Enabled,
		"EnabledAt": status.EnabledAt,
	})
}

func (m *Module) apiSetup(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	// Check if already enabled
	var enabled bool
	m.DB().Get(&enabled, `SELECT COALESCE(totp_enabled, false) FROM users WHERE id = $1`, user.ID)
	if enabled {
		http.Error(w, `{"error":"2FA is already enabled"}`, http.StatusBadRequest)
		return
	}

	// Generate new secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      m.appName,
		AccountName: user.Email,
	})
	if err != nil {
		http.Error(w, `{"error":"Failed to generate secret"}`, http.StatusInternalServerError)
		return
	}

	// Store secret (not enabled yet)
	_, err = m.DB().Exec(`UPDATE users SET totp_secret = $1 WHERE id = $2`, key.Secret(), user.ID)
	if err != nil {
		http.Error(w, `{"error":"Failed to save secret"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"secret":  key.Secret(),
		"qr_url":  key.URL(),
		"issuer":  m.appName,
		"account": user.Email,
	})
}

func (m *Module) apiEnable(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Get stored secret
	var secret string
	err := m.DB().Get(&secret, `SELECT COALESCE(totp_secret, '') FROM users WHERE id = $1`, user.ID)
	if err != nil || secret == "" {
		http.Error(w, `{"error":"Please set up 2FA first"}`, http.StatusBadRequest)
		return
	}

	// Verify code
	if !totp.Validate(req.Code, secret) {
		http.Error(w, `{"error":"Invalid code. Please try again."}`, http.StatusBadRequest)
		return
	}

	// Generate backup codes
	backupCodes := m.generateBackupCodes(8)

	// Enable 2FA
	_, err = m.DB().Exec(`
		UPDATE users
		SET totp_enabled = true, totp_enabled_at = NOW(), totp_backup_codes = $1
		WHERE id = $2
	`, backupCodes, user.ID)
	if err != nil {
		http.Error(w, `{"error":"Failed to enable 2FA"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"backup_codes": backupCodes,
	})
}

func (m *Module) apiDisable(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Get stored secret
	var secret string
	err := m.DB().Get(&secret, `SELECT COALESCE(totp_secret, '') FROM users WHERE id = $1`, user.ID)
	if err != nil || secret == "" {
		http.Error(w, `{"error":"2FA is not enabled"}`, http.StatusBadRequest)
		return
	}

	// Verify code
	if !totp.Validate(req.Code, secret) {
		http.Error(w, `{"error":"Invalid code"}`, http.StatusBadRequest)
		return
	}

	// Disable 2FA
	_, err = m.DB().Exec(`
		UPDATE users
		SET totp_enabled = false, totp_secret = NULL, totp_backup_codes = NULL, totp_enabled_at = NULL
		WHERE id = $1
	`, user.ID)
	if err != nil {
		http.Error(w, `{"error":"Failed to disable 2FA"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (m *Module) apiRegenerateBackup(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Get stored secret
	var secret string
	err := m.DB().Get(&secret, `SELECT COALESCE(totp_secret, '') FROM users WHERE id = $1`, user.ID)
	if err != nil || secret == "" {
		http.Error(w, `{"error":"2FA is not enabled"}`, http.StatusBadRequest)
		return
	}

	// Verify code
	if !totp.Validate(req.Code, secret) {
		http.Error(w, `{"error":"Invalid code"}`, http.StatusBadRequest)
		return
	}

	// Generate new backup codes
	backupCodes := m.generateBackupCodes(8)

	_, err = m.DB().Exec(`UPDATE users SET totp_backup_codes = $1 WHERE id = $2`, backupCodes, user.ID)
	if err != nil {
		http.Error(w, `{"error":"Failed to regenerate backup codes"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"backup_codes": backupCodes,
	})
}

func (m *Module) generateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 5)
		rand.Read(b)
		code := strings.ToUpper(base32.StdEncoding.EncodeToString(b))[:8]
		codes[i] = code[:4] + "-" + code[4:]
	}
	return codes
}

// VerifyTOTP verifies a TOTP code for a user (used during login)
func (m *Module) VerifyTOTP(userID, code string) bool {
	var secret string
	err := m.DB().Get(&secret, `SELECT COALESCE(totp_secret, '') FROM users WHERE id = $1 AND totp_enabled = true`, userID)
	if err != nil || secret == "" {
		return false
	}
	return totp.Validate(code, secret)
}

// IsTOTPEnabled checks if 2FA is enabled for a user
func (m *Module) IsTOTPEnabled(userID string) bool {
	var enabled bool
	m.DB().Get(&enabled, `SELECT COALESCE(totp_enabled, false) FROM users WHERE id = $1`, userID)
	return enabled
}

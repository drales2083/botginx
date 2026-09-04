package referrals

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	templates *module.TemplateEngine
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"referrals",
			"Referrals",
			"Multi-level referral commission system",
		),
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
	migrations := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(8) UNIQUE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by_id UUID REFERENCES users(id)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS balance DECIMAL(10,2) DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS referral_earnings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id),
			from_user_id UUID NOT NULL REFERENCES users(id),
			level INT NOT NULL CHECK (level BETWEEN 1 AND 3),
			payment_type VARCHAR(20) NOT NULL,
			payment_amount DECIMAL(10,2) NOT NULL,
			commission_rate INT NOT NULL,
			commission_amount DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_referral_earnings_user ON referral_earnings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_referral_earnings_from ON referral_earnings(from_user_id)`,
		`CREATE TABLE IF NOT EXISTS referral_settings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			level1_rate INT NOT NULL DEFAULT 25,
			level2_rate INT NOT NULL DEFAULT 15,
			level3_rate INT NOT NULL DEFAULT 10,
			enabled BOOLEAN NOT NULL DEFAULT true,
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
	}

	for _, q := range migrations {
		if _, err := m.DB().Exec(q); err != nil {
			return err
		}
	}

	// Seed default settings if none exist
	var count int
	m.DB().Get(&count, `SELECT COUNT(*) FROM referral_settings`)
	if count == 0 {
		m.DB().Exec(`INSERT INTO referral_settings (level1_rate, level2_rate, level3_rate, enabled) VALUES (25, 15, 10, true)`)
	}

	// Generate referral codes for existing users without one
	m.DB().Exec(`UPDATE users SET referral_code = UPPER(SUBSTR(MD5(RANDOM()::TEXT), 1, 8)) WHERE referral_code IS NULL`)

	// Seed FAQ category and items for referrals
	m.DB().Exec(`INSERT INTO faq_categories (id, name, icon, sort_order) VALUES
		('cat_referrals', 'Referrals', 'bi-people', 7)
		ON CONFLICT (id) DO NOTHING`)

	m.DB().Exec(`INSERT INTO faq_items (id, category_id, question, answer, sort_order) VALUES
		('faq_ref_1', 'cat_referrals', 'How does the referral program work?',
		 'Share your unique referral link with others. When they register and pay for a subscription, you earn commission. You can earn up to 3 levels deep: 25%% from direct referrals (Level 1), 15%% from their referrals (Level 2), and 10%% from the next level (Level 3).',
		 1),
		('faq_ref_2', 'cat_referrals', 'Where do I find my referral link?',
		 'Go to the <a href="/user/referrals">Referrals</a> page in your dashboard. Your unique referral link is displayed at the top. Click the copy button to copy it to your clipboard.',
		 2),
		('faq_ref_3', 'cat_referrals', 'When do I receive my referral earnings?',
		 'Referral commissions are credited to your balance immediately when your referral makes a payment. You can use this balance for your own subscription or request a withdrawal.',
		 3),
		('faq_ref_4', 'cat_referrals', 'Do I earn from subscription renewals?',
		 'Yes! You earn commission every time your referrals renew their subscription, not just on their first payment. This includes all 3 levels of referrals.',
		 4),
		('faq_ref_5', 'cat_referrals', 'What are Level 1, Level 2, and Level 3 referrals?',
		 'Level 1 are users who registered using YOUR link (you earn 25%%). Level 2 are users who registered using your Level 1 referrals'' links (you earn 15%%). Level 3 are users who registered using your Level 2 referrals'' links (you earn 10%%).',
		 5),
		('faq_ref_6', 'cat_referrals', 'Is there a limit to how many people I can refer?',
		 'No limit! Refer as many people as you want. The more active referrals you have, the more you earn from the 3-level commission structure.',
		 6),
		('faq_ref_7', 'cat_referrals', 'Can I refer myself or create fake accounts?',
		 'No. Self-referrals and fraudulent accounts are prohibited and will result in forfeiture of all referral earnings and possible account termination.',
		 7),
		('faq_ref_8', 'cat_referrals', 'How do I withdraw my referral earnings?',
		 'Your referral earnings are added to your account balance. You can use this balance to pay for your own subscription, or request a cryptocurrency withdrawal from the Payments section.',
		 8)
		ON CONFLICT (id) DO NOTHING`)

	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", m.handleIndex)
	r.Get("/api/earnings", m.apiEarnings)
	r.Get("/api/referrals", m.apiReferrals)
	return r
}

func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/settings", m.handleAdminSettings)
	r.Post("/settings", m.handleSaveSettings)
	r.Get("/api/stats", m.apiAdminStats)
	return r
}

type Settings struct {
	ID         string `db:"id"`
	Level1Rate int    `db:"level1_rate"`
	Level2Rate int    `db:"level2_rate"`
	Level3Rate int    `db:"level3_rate"`
	Enabled    bool   `db:"enabled"`
}

func (m *Module) GetSettings() (*Settings, error) {
	var s Settings
	err := m.DB().Get(&s, `SELECT * FROM referral_settings LIMIT 1`)
	return &s, err
}

type UserStats struct {
	ReferralCode  string  `db:"referral_code"`
	Level1Count   int     `db:"level1_count"`
	Level2Count   int     `db:"level2_count"`
	Level3Count   int     `db:"level3_count"`
	TotalEarnings float64 `db:"total_earnings"`
	Balance       float64 `db:"balance"`
}

func (m *Module) GetUserStats(userID string) (*UserStats, error) {
	var stats UserStats

	// Get referral code and balance
	m.DB().Get(&stats, `SELECT COALESCE(referral_code, '') as referral_code, COALESCE(balance, 0) as balance FROM users WHERE id = $1`, userID)

	// Generate code if missing
	if stats.ReferralCode == "" {
		code := m.generateCode()
		m.DB().Exec(`UPDATE users SET referral_code = $1 WHERE id = $2`, code, userID)
		stats.ReferralCode = code
	}

	// Count referrals by level
	m.DB().Get(&stats.Level1Count, `SELECT COUNT(*) FROM users WHERE referred_by_id = $1`, userID)

	// Level 2: users referred by my level 1s
	m.DB().Get(&stats.Level2Count, `
		SELECT COUNT(*) FROM users u2
		WHERE u2.referred_by_id IN (SELECT id FROM users WHERE referred_by_id = $1)
	`, userID)

	// Level 3: users referred by my level 2s
	m.DB().Get(&stats.Level3Count, `
		SELECT COUNT(*) FROM users u3
		WHERE u3.referred_by_id IN (
			SELECT u2.id FROM users u2
			WHERE u2.referred_by_id IN (SELECT id FROM users WHERE referred_by_id = $1)
		)
	`, userID)

	// Total earnings
	m.DB().Get(&stats.TotalEarnings, `SELECT COALESCE(SUM(commission_amount), 0) FROM referral_earnings WHERE user_id = $1`, userID)

	return &stats, nil
}

func (m *Module) generateCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type Earning struct {
	ID               string  `db:"id" json:"id"`
	FromEmail        string  `db:"from_email" json:"from_email"`
	Level            int     `db:"level" json:"level"`
	PaymentType      string  `db:"payment_type" json:"payment_type"`
	PaymentAmount    float64 `db:"payment_amount" json:"payment_amount"`
	CommissionRate   int     `db:"commission_rate" json:"commission_rate"`
	CommissionAmount float64 `db:"commission_amount" json:"commission_amount"`
	CreatedAt        string  `db:"created_at" json:"created_at"`
}

func (m *Module) GetEarnings(userID string, limit, offset int) ([]Earning, error) {
	var earnings []Earning
	err := m.DB().Select(&earnings, `
		SELECT e.id, COALESCE(u.email, 'Unknown') as from_email, e.level, e.payment_type,
			e.payment_amount, e.commission_rate, e.commission_amount,
			TO_CHAR(e.created_at, 'YYYY-MM-DD HH24:MI') as created_at
		FROM referral_earnings e
		LEFT JOIN users u ON u.id = e.from_user_id
		WHERE e.user_id = $1
		ORDER BY e.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	return earnings, err
}

type Referral struct {
	ID        string `db:"id" json:"id"`
	Email     string `db:"email" json:"email"`
	Level     int    `json:"level"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

func (m *Module) GetReferrals(userID string, level int) ([]Referral, error) {
	var referrals []Referral
	var err error

	switch level {
	case 1:
		err = m.DB().Select(&referrals, `
			SELECT id, email, TO_CHAR(created_at, 'YYYY-MM-DD') as created_at
			FROM users WHERE referred_by_id = $1
			ORDER BY created_at DESC
		`, userID)
	case 2:
		err = m.DB().Select(&referrals, `
			SELECT id, email, TO_CHAR(created_at, 'YYYY-MM-DD') as created_at
			FROM users WHERE referred_by_id IN (SELECT id FROM users WHERE referred_by_id = $1)
			ORDER BY created_at DESC
		`, userID)
	case 3:
		err = m.DB().Select(&referrals, `
			SELECT id, email, TO_CHAR(created_at, 'YYYY-MM-DD') as created_at
			FROM users WHERE referred_by_id IN (
				SELECT u2.id FROM users u2
				WHERE u2.referred_by_id IN (SELECT id FROM users WHERE referred_by_id = $1)
			)
			ORDER BY created_at DESC
		`, userID)
	}

	for i := range referrals {
		referrals[i].Level = level
	}
	return referrals, err
}

// ProcessPayment calculates and credits commissions for a payment
func (m *Module) ProcessPayment(db *sqlx.DB, userID string, amount float64, paymentType string) error {
	settings, err := m.GetSettings()
	if err != nil || !settings.Enabled {
		return nil
	}

	chain, err := m.GetReferralChain(db, userID)
	if err != nil {
		return err
	}

	rates := []int{settings.Level1Rate, settings.Level2Rate, settings.Level3Rate}

	for i, referrerID := range chain {
		if i >= 3 {
			break
		}

		rate := rates[i]
		commission := amount * float64(rate) / 100

		// Record earning
		_, err := db.Exec(`
			INSERT INTO referral_earnings
			(user_id, from_user_id, level, payment_type, payment_amount, commission_rate, commission_amount)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, referrerID, userID, i+1, paymentType, amount, rate, commission)
		if err != nil {
			return err
		}

		// Credit balance
		_, err = db.Exec(`UPDATE users SET balance = COALESCE(balance, 0) + $1 WHERE id = $2`, commission, referrerID)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetReferralChain returns up to 3 levels of referrers for a user
func (m *Module) GetReferralChain(db *sqlx.DB, userID string) ([]string, error) {
	var chain []string

	currentID := userID
	for i := 0; i < 3; i++ {
		var referrerID *string
		err := db.Get(&referrerID, `SELECT referred_by_id FROM users WHERE id = $1`, currentID)
		if err != nil || referrerID == nil {
			break
		}
		chain = append(chain, *referrerID)
		currentID = *referrerID
	}

	return chain, nil
}

// SetReferrer sets the referred_by_id for a user (called during registration)
func (m *Module) SetReferrer(db *sqlx.DB, userID, refCode string) error {
	if refCode == "" {
		return nil
	}

	var referrerID string
	err := db.Get(&referrerID, `SELECT id FROM users WHERE referral_code = $1`, refCode)
	if err != nil {
		return nil // Invalid code, silently ignore
	}

	// Prevent self-referral
	if referrerID == userID {
		return nil
	}

	_, err = db.Exec(`UPDATE users SET referred_by_id = $1 WHERE id = $2 AND referred_by_id IS NULL`, referrerID, userID)
	return err
}

// Handlers

func (m *Module) handleIndex(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	stats, _ := m.GetUserStats(userID)
	settings, _ := m.GetSettings()

	module.RenderUserSection(w, r, m.templates, "referrals:index.html", map[string]interface{}{
		"Title":    "Referrals",
		"Stats":    stats,
		"Settings": settings,
	})
}

func (m *Module) apiEarnings(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit == 0 {
		limit = 20
	}

	earnings, _ := m.GetEarnings(userID, limit, offset)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(earnings)
}

func (m *Module) apiReferrals(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	level, _ := strconv.Atoi(r.URL.Query().Get("level"))
	if level < 1 || level > 3 {
		level = 1
	}

	referrals, _ := m.GetReferrals(userID, level)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(referrals)
}

func (m *Module) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, _ := m.GetSettings()

	// Platform stats
	var totalReferrals, activeReferrers int
	var totalCommissions float64
	m.DB().Get(&totalReferrals, `SELECT COUNT(*) FROM users WHERE referred_by_id IS NOT NULL`)
	m.DB().Get(&totalCommissions, `SELECT COALESCE(SUM(commission_amount), 0) FROM referral_earnings`)
	m.DB().Get(&activeReferrers, `SELECT COUNT(DISTINCT user_id) FROM referral_earnings`)

	module.Render(w, r, m.templates, "referrals:settings.html", map[string]interface{}{
		"Title":            "Referral Settings",
		"Settings":         settings,
		"TotalReferrals":   totalReferrals,
		"TotalCommissions": totalCommissions,
		"ActiveReferrers":  activeReferrers,
	})
}

func (m *Module) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	level1, _ := strconv.Atoi(r.FormValue("level1_rate"))
	level2, _ := strconv.Atoi(r.FormValue("level2_rate"))
	level3, _ := strconv.Atoi(r.FormValue("level3_rate"))
	enabled := r.FormValue("enabled") == "on"

	// Validate rates
	if level1 < 0 || level1 > 100 || level2 < 0 || level2 > 100 || level3 < 0 || level3 > 100 {
		http.Error(w, "Invalid rate values", http.StatusBadRequest)
		return
	}

	_, err := m.DB().Exec(`
		UPDATE referral_settings
		SET level1_rate = $1, level2_rate = $2, level3_rate = $3, enabled = $4, updated_at = NOW()
	`, level1, level2, level3, enabled)

	if err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/referrals/settings?saved=1", http.StatusSeeOther)
}

func (m *Module) apiAdminStats(w http.ResponseWriter, r *http.Request) {
	var stats struct {
		TotalReferrals   int     `json:"total_referrals"`
		TotalCommissions float64 `json:"total_commissions"`
		ActiveReferrers  int     `json:"active_referrers"`
	}

	m.DB().Get(&stats.TotalReferrals, `SELECT COUNT(*) FROM users WHERE referred_by_id IS NOT NULL`)
	m.DB().Get(&stats.TotalCommissions, `SELECT COALESCE(SUM(commission_amount), 0) FROM referral_earnings`)
	m.DB().Get(&stats.ActiveReferrers, `SELECT COUNT(DISTINCT user_id) FROM referral_earnings`)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Referrals",
			Icon:    "bi-people",
			Path:    "/user/referrals",
			Order:   50,
			Section: module.MenuSectionUser,
		},
		{
			Title:   "Referrals",
			Icon:    "bi-diagram-3",
			Path:    "/admin/referrals/settings",
			Order:   60,
			Section: module.MenuSectionAdmin,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

package services

import (
	"github.com/botginx/botginx/pkg/protection"
	"github.com/jmoiron/sqlx"
)

type AntibotControlService struct {
	db *sqlx.DB
}

func NewAntibotControlService(db *sqlx.DB) *AntibotControlService {
	return &AntibotControlService{db: db}
}

type UserDefaults struct {
	UserID   string              `db:"user_id" json:"userId"`
	Settings protection.Settings `db:"settings" json:"settings"`
}

func (s *AntibotControlService) GetDefaults(userID string) (*protection.Settings, error) {
	var settings protection.Settings

	err := s.db.Get(&settings, `
		SELECT settings FROM user_antibot_defaults WHERE user_id = $1
	`, userID)

	if err != nil {
		// Return empty defaults if none set
		return &protection.Settings{
			Template:         "cloudflare",
			ThemeMode:        "auto",
			CountryMode:      "",
			CountryList:      []string{},
			ASNMode:          "",
			ASNList:          []string{},
			DeviceMode:       "",
			DeviceList:       []string{},
			BlockBots:        true,
			BlockTor:         false,
			BlockProxy:       false,
			BlockDatacenter:  false,
			BlockHeadless:    true,
			MinBehaviorScore: 0,
			RedirectOnBlock:  "",
		}, nil
	}

	return &settings, nil
}

func (s *AntibotControlService) SaveDefaults(userID string, settings *protection.Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}

	_, err := s.db.Exec(`
		INSERT INTO user_antibot_defaults (user_id, settings)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			settings = EXCLUDED.settings,
			updated_at = NOW()
	`, userID, settings)

	return err
}

func (s *AntibotControlService) ResetDefaults(userID string) error {
	_, err := s.db.Exec(`
		DELETE FROM user_antibot_defaults WHERE user_id = $1
	`, userID)
	return err
}

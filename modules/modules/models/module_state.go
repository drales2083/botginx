package models

import "time"

type ModuleState struct {
	ID          string    `db:"id" json:"id"`
	ModuleID    string    `db:"module_id" json:"moduleId"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Version     string    `db:"version" json:"version"`
	IsEnabled   bool      `db:"is_enabled" json:"isEnabled"`
	IsCore      bool      `db:"is_core" json:"isCore"` // Core modules can't be disabled
	Settings    string    `db:"settings" json:"settings,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type ModuleInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	IsEnabled   bool   `json:"isEnabled"`
	IsCore      bool   `json:"isCore"`
	HasRoutes   bool   `json:"hasRoutes"`
	HasWidgets  bool   `json:"hasWidgets"`
	MenuSection string `json:"menuSection"`
}

type UpdateModuleInput struct {
	IsEnabled *bool  `json:"isEnabled"`
	Settings  string `json:"settings"`
}

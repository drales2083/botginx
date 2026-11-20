package models

import "time"

type ProvisionStatus string

const (
	ProvisionStatusPending   ProvisionStatus = "pending"
	ProvisionStatusRunning   ProvisionStatus = "running"
	ProvisionStatusCompleted ProvisionStatus = "completed"
	ProvisionStatusFailed    ProvisionStatus = "failed"
)

type Provision struct {
	ID          string          `db:"id" json:"id"`
	ServerID    string          `db:"server_id" json:"serverId"`
	Status      ProvisionStatus `db:"status" json:"status"`
	CurrentStep int             `db:"current_step" json:"currentStep"`
	TotalSteps  int             `db:"total_steps" json:"totalSteps"`
	StepName    *string         `db:"step_name" json:"stepName,omitempty"`
	Output      *string         `db:"output" json:"output,omitempty"`
	Error       *string         `db:"error" json:"error,omitempty"`
	AcmeDomain  *string         `db:"acme_domain" json:"acmeDomain,omitempty"`
	StartedAt   *time.Time      `db:"started_at" json:"startedAt,omitempty"`
	CompletedAt *time.Time      `db:"completed_at" json:"completedAt,omitempty"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
}

type CreateProvisionInput struct {
	ServerID   string `json:"serverId" validate:"required"`
	AcmeDomain string `json:"acmeDomain" validate:"required"`
}

// ProvisionStep defines a provisioning step
type ProvisionStep struct {
	Number int
	Name   string
}

var ProvisionSteps = []ProvisionStep{
	{1, "Connecting via SSH"},
	{2, "Installing base packages"},
	{3, "Cloning antibot repository"},
	{4, "Installing botection"},
	{5, "Verifying services"},
}

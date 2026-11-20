package models

import "time"

type ReconfigureStatus string

const (
	ReconfigureStatusPending   ReconfigureStatus = "pending"
	ReconfigureStatusRunning   ReconfigureStatus = "running"
	ReconfigureStatusCompleted ReconfigureStatus = "completed"
	ReconfigureStatusFailed    ReconfigureStatus = "failed"
)

type Reconfigure struct {
	ID          string            `db:"id" json:"id"`
	ServerID    string            `db:"server_id" json:"serverId"`
	Status      ReconfigureStatus `db:"status" json:"status"`
	CurrentStep int               `db:"current_step" json:"currentStep"`
	TotalSteps  int               `db:"total_steps" json:"totalSteps"`
	StepName    *string           `db:"step_name" json:"stepName,omitempty"`
	Output      *string           `db:"output" json:"output,omitempty"`
	Error       *string           `db:"error" json:"error,omitempty"`
	StartedAt   *time.Time        `db:"started_at" json:"startedAt,omitempty"`
	CompletedAt *time.Time        `db:"completed_at" json:"completedAt,omitempty"`
	CreatedAt   time.Time         `db:"created_at" json:"createdAt"`
}

type CreateReconfigureInput struct {
	ServerID string `json:"serverId" validate:"required"`
}

// ReconfigureStep defines a reconfiguration step
type ReconfigureStep struct {
	Number int
	Name   string
}

var ReconfigureSteps = []ReconfigureStep{
	{1, "Reset All Domains"},
	{2, "Assign Server to All Domains"},
	{3, "Update DNS (API Providers)"},
	{4, "Complete"},
}

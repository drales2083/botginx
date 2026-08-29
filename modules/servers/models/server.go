package models

import (
	"time"
)

type ServerStatus string

const (
	ServerStatusPending      ServerStatus = "pending"
	ServerStatusConnecting   ServerStatus = "connecting"
	ServerStatusReady        ServerStatus = "ready"
	ServerStatusDisconnected ServerStatus = "disconnected"
	ServerStatusFailed       ServerStatus = "failed"
)

type Server struct {
	ID          string       `db:"id" json:"id"`
	UserID      string       `db:"user_id" json:"userId"`
	Name        string       `db:"name" json:"name"`
	IP          string       `db:"ip" json:"ip"`
	Port        int          `db:"port" json:"port"`
	SSHUser     string       `db:"ssh_user" json:"sshUser"`
	SSHPassword string       `db:"ssh_password" json:"-"` // Never expose in JSON
	AuthMethod  string       `db:"auth_method" json:"authMethod"` // "password" or "key"
	Status      ServerStatus `db:"status" json:"status"`
	Provider    *string      `db:"provider" json:"provider,omitempty"`
	OS          *string      `db:"os" json:"os,omitempty"`
	CreatedAt   time.Time    `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time    `db:"updated_at" json:"updatedAt"`
}

func (s *Server) User() string {
	return s.SSHUser
}

type ServerLog struct {
	ID        string    `db:"id" json:"id"`
	ServerID  string    `db:"server_id" json:"serverId"`
	Type      string    `db:"type" json:"type"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type CreateServerInput struct {
	Name        string `json:"name" validate:"required"`
	IP          string `json:"ip" validate:"required,ip"`
	Port        int    `json:"port" validate:"required,min=1,max=65535"`
	SSHUser     string `json:"sshUser" validate:"required"`
	SSHPassword string `json:"sshPassword"` // Optional - use password auth
	AuthMethod  string `json:"authMethod"`  // "password" or "key" (default: key if no password)
	Provider    string `json:"provider"`
}

type UpdateServerInput struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	SSHUser string `json:"sshUser"`
}

type ExecInput struct {
	Command string `json:"command" validate:"required"`
	User    string `json:"user"`
}

type ExecOutput struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

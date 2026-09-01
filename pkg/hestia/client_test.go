package hestia

import (
	"testing"
)

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\"'\"'quote'"},
		{"multi'quo'tes", "'multi'\"'\"'quo'\"'\"'tes'"},
		{"", "''"},
		{"user@domain.com", "'user@domain.com'"},
		{"pass;rm -rf /", "'pass;rm -rf /'"},
		{"$(whoami)", "'$(whoami)'"},
		{"`whoami`", "'`whoami`'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := shellEscape(tt.input)
			if result != tt.expected {
				t.Errorf("shellEscape(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("example.com", 22, "root", "password")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.host != "example.com:22" {
		t.Errorf("host = %q, want %q", client.host, "example.com:22")
	}
	if client.IsConnected() {
		t.Error("new client should not be connected")
	}
}

func TestUserStats(t *testing.T) {
	// Test that UserStats struct can be instantiated
	stats := UserStats{
		Username:  "testuser",
		DiskUsed:  "100",
		DiskQuota: "1000",
		BWUsed:    "500",
		BWQuota:   "10000",
		Domains:   "5",
		Mail:      "10",
		Databases: "3",
		Suspended: "no",
	}
	if stats.Username != "testuser" {
		t.Errorf("Username = %q, want %q", stats.Username, "testuser")
	}
}

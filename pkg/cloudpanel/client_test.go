package cloudpanel

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
		{"multiple''quotes", "'multiple'\"'\"''\"'\"'quotes'"},
		{"special$chars", "'special$chars'"},
		{"", "''"},
	}

	for _, tt := range tests {
		result := shellEscape(tt.input)
		if result != tt.expected {
			t.Errorf("shellEscape(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("192.168.1.100", 22, "root", "password123")

	if client == nil {
		t.Error("NewClient returned nil")
	}

	if client.host != "192.168.1.100:22" {
		t.Errorf("host = %q; want %q", client.host, "192.168.1.100:22")
	}

	if client.IsConnected() {
		t.Error("new client should not be connected")
	}
}

func TestPHPVersions(t *testing.T) {
	versions := []PHPVersion{PHP74, PHP80, PHP81, PHP82, PHP83}
	expected := []string{"7.4", "8.0", "8.1", "8.2", "8.3"}

	for i, v := range versions {
		if string(v) != expected[i] {
			t.Errorf("PHP version %d = %q; want %q", i, v, expected[i])
		}
	}
}

package status

import "testing"

func TestSetupStep_Valid(t *testing.T) {
	tests := []struct {
		step  SetupStep
		valid bool
	}{
		{SetupStepPending, true},
		{SetupStepDNSWaiting, true},
		{SetupStepSSLWaiting, true},
		{SetupStepSSLGenerating, true},
		{SetupStepComplete, true},
		{SetupStep("invalid"), false},
		{SetupStep(""), false},
	}

	for _, tt := range tests {
		if got := tt.step.Valid(); got != tt.valid {
			t.Errorf("SetupStep(%q).Valid() = %v, want %v", tt.step, got, tt.valid)
		}
	}
}

func TestSetupStep_Color(t *testing.T) {
	tests := []struct {
		step  SetupStep
		color string
	}{
		{SetupStepComplete, "success"},
		{SetupStepSSLGenerating, "info"},
		{SetupStepDNSWaiting, "warning"},
		{SetupStepSSLWaiting, "warning"},
		{SetupStepPending, "secondary"},
		{SetupStep("unknown"), "secondary"},
	}

	for _, tt := range tests {
		if got := tt.step.Color(); got != tt.color {
			t.Errorf("SetupStep(%q).Color() = %q, want %q", tt.step, got, tt.color)
		}
	}
}

func TestSetupStep_Label(t *testing.T) {
	if got := SetupStepComplete.Label(); got != "Complete" {
		t.Errorf("SetupStepComplete.Label() = %q, want %q", got, "Complete")
	}
	if got := SetupStep("unknown").Label(); got != "unknown" {
		t.Errorf("unknown step should return itself as label")
	}
}

func TestSetupStep_Icon(t *testing.T) {
	if got := SetupStepComplete.Icon(); got != "bi-check-circle-fill" {
		t.Errorf("SetupStepComplete.Icon() = %q, want bi-check-circle-fill", got)
	}
}

func TestSetupType_Valid(t *testing.T) {
	if !SetupTypeDirect.Valid() {
		t.Error("SetupTypeDirect should be valid")
	}
	if !SetupTypeExternal.Valid() {
		t.Error("SetupTypeExternal should be valid")
	}
	if SetupType("invalid").Valid() {
		t.Error("invalid type should not be valid")
	}
}

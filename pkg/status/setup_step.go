// pkg/status/setup_step.go
package status

// SetupStep represents domain setup wizard progress
type SetupStep string

const (
	SetupStepPending       SetupStep = "pending"
	SetupStepDNSWaiting    SetupStep = "dns_waiting"
	SetupStepSSLWaiting    SetupStep = "ssl_waiting"
	SetupStepSSLGenerating SetupStep = "ssl_generating"
	SetupStepComplete      SetupStep = "complete"
)

// Valid returns true if the step is a known value
func (s SetupStep) Valid() bool {
	switch s {
	case SetupStepPending, SetupStepDNSWaiting, SetupStepSSLWaiting,
		SetupStepSSLGenerating, SetupStepComplete:
		return true
	}
	return false
}

// Color returns Bootstrap color class for UI badges
func (s SetupStep) Color() string {
	switch s {
	case SetupStepComplete:
		return "success"
	case SetupStepSSLGenerating:
		return "info"
	case SetupStepDNSWaiting, SetupStepSSLWaiting:
		return "warning"
	case SetupStepPending:
		return "secondary"
	default:
		return "secondary"
	}
}

// Label returns human-readable text for UI
func (s SetupStep) Label() string {
	switch s {
	case SetupStepPending:
		return "Detecting..."
	case SetupStepDNSWaiting:
		return "Waiting for DNS"
	case SetupStepSSLWaiting:
		return "Waiting for SSL verification"
	case SetupStepSSLGenerating:
		return "Generating SSL"
	case SetupStepComplete:
		return "Complete"
	default:
		return string(s)
	}
}

// Icon returns Bootstrap icon class
func (s SetupStep) Icon() string {
	switch s {
	case SetupStepComplete:
		return "bi-check-circle-fill"
	case SetupStepSSLGenerating:
		return "bi-arrow-repeat"
	case SetupStepDNSWaiting, SetupStepSSLWaiting:
		return "bi-hourglass-split"
	case SetupStepPending:
		return "bi-clock"
	default:
		return "bi-question-circle"
	}
}

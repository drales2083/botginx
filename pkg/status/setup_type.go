// pkg/status/setup_type.go
package status

// SetupType indicates how domain DNS is managed
type SetupType string

const (
	SetupTypeDirect   SetupType = "direct"   // A record points to our server
	SetupTypeExternal SetupType = "external" // cPanel/Cloudflare/other external DNS
)

// Valid returns true if the type is a known value
func (t SetupType) Valid() bool {
	return t == SetupTypeDirect || t == SetupTypeExternal
}

// Label returns human-readable text for UI
func (t SetupType) Label() string {
	switch t {
	case SetupTypeDirect:
		return "Direct DNS"
	case SetupTypeExternal:
		return "External DNS"
	default:
		return string(t)
	}
}

# Status Enums & Action Classes Design

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve backend code organization with typed status enums and single-purpose action classes.

**Architecture:** Extract status constants into typed enums with display helpers. Extract complex domain operations into reusable action classes.

**Tech Stack:** Go, PostgreSQL (no new dependencies)

## Global Constraints

- No breaking changes to existing behavior
- Database schema unchanged (enums use same string values)
- Templates continue working with existing status values
- All changes are internal refactoring

---

## Part 1: Status Enums

### Location

```
pkg/status/
├── setup_step.go    # SetupStep enum
├── setup_type.go    # SetupType enum
└── status.go        # Package doc, shared helpers
```

### SetupStep Enum

```go
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
```

### SetupType Enum

```go
package status

// SetupType indicates how domain DNS is managed
type SetupType string

const (
    SetupTypeDirect   SetupType = "direct"   // A record points to our server
    SetupTypeExternal SetupType = "external" // cPanel/Cloudflare/other external DNS
)

func (t SetupType) Valid() bool {
    return t == SetupTypeDirect || t == SetupTypeExternal
}

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
```

### Model Update

Update `modules/domains/models/domain.go`:

```go
import "github.com/botginx/botginx/pkg/status"

// Remove old constants (SetupTypeDirect, SetupStepPending, etc.)

type Domain struct {
    // ... existing fields ...
    SetupType status.SetupType `db:"setup_type" json:"setupType"`
    SetupStep status.SetupStep `db:"setup_step" json:"setupStep"`
    // ... rest unchanged ...
}
```

### Template Function

Add to template funcs so templates can call display methods:

```go
// In pkg/module/templates.go or similar
"statusColor": func(s string) string {
    return status.SetupStep(s).Color()
},
"statusLabel": func(s string) string {
    return status.SetupStep(s).Label()
},
"statusIcon": func(s string) string {
    return status.SetupStep(s).Icon()
},
```

---

## Part 2: Action Classes

### Location

```
pkg/actions/
├── action.go           # Base types and helpers
├── create_domain.go    # CreateDomainAction
├── verify_dns.go       # VerifyDNSAction
└── provision_ssl.go    # ProvisionSSLAction
```

### Base Types

```go
package actions

import (
    "context"
    "database/sql"
)

// Result wraps an action result with optional metadata
type Result[T any] struct {
    Data    T
    Message string // Optional success message
}

// Deps contains shared dependencies for actions
type Deps struct {
    DB *sql.DB
}
```

### CreateDomainAction

```go
package actions

import (
    "context"
    "errors"
    "strings"
    
    "github.com/botginx/botginx/modules/domains/models"
    "github.com/botginx/botginx/pkg/status"
)

// CreateDomainInput contains validated input for domain creation
type CreateDomainInput struct {
    UserID     string
    DomainName string
    SetupType  status.SetupType
    IsWildcard bool
}

// Validate checks input is valid
func (i CreateDomainInput) Validate() error {
    if i.UserID == "" {
        return errors.New("user ID required")
    }
    if i.DomainName == "" {
        return errors.New("domain name required")
    }
    if !i.SetupType.Valid() {
        return errors.New("invalid setup type")
    }
    return nil
}

// CreateDomainAction handles domain creation logic
type CreateDomainAction struct {
    deps  *Deps
    input CreateDomainInput
}

// NewCreateDomain creates a new action instance
func NewCreateDomain(deps *Deps, input CreateDomainInput) *CreateDomainAction {
    return &CreateDomainAction{deps: deps, input: input}
}

// Execute runs the domain creation
func (a *CreateDomainAction) Execute(ctx context.Context) (*models.Domain, error) {
    // Validate input
    if err := a.input.Validate(); err != nil {
        return nil, err
    }
    
    // Normalize domain name
    name := strings.ToLower(strings.TrimSpace(a.input.DomainName))
    
    // Check for duplicates
    // ... (extracted from handler)
    
    // Generate verification token
    // ... (extracted from handler)
    
    // Create domain record
    // ... (extracted from handler)
    
    // Return created domain
    return domain, nil
}
```

### VerifyDNSAction

```go
package actions

// VerifyDNSInput contains input for DNS verification
type VerifyDNSInput struct {
    DomainID string
}

// VerifyDNSAction checks DNS records for a domain
type VerifyDNSAction struct {
    deps  *Deps
    input VerifyDNSInput
}

// NewVerifyDNS creates a new action instance
func NewVerifyDNS(deps *Deps, input VerifyDNSInput) *VerifyDNSAction {
    return &VerifyDNSAction{deps: deps, input: input}
}

// VerifyDNSResult contains verification outcome
type VerifyDNSResult struct {
    Verified bool
    Records  []string // Found DNS records
    Error    string   // Human-readable error if not verified
}

// Execute runs DNS verification
func (a *VerifyDNSAction) Execute(ctx context.Context) (*VerifyDNSResult, error) {
    // Load domain from DB
    // Perform DNS lookup
    // Check for expected records
    // Update domain status if verified
    // Return result
}
```

### ProvisionSSLAction

```go
package actions

// ProvisionSSLInput contains input for SSL provisioning
type ProvisionSSLInput struct {
    DomainID string
    Force    bool // Force renewal even if cert exists
}

// ProvisionSSLAction handles ACME certificate generation
type ProvisionSSLAction struct {
    deps  *Deps
    input ProvisionSSLInput
}

// NewProvisionSSL creates a new action instance
func NewProvisionSSL(deps *Deps, input ProvisionSSLInput) *ProvisionSSLAction {
    return &ProvisionSSLAction{deps: deps, input: input}
}

// ProvisionSSLResult contains provisioning outcome
type ProvisionSSLResult struct {
    Success   bool
    ExpiresAt time.Time
    Error     string
}

// Execute runs SSL provisioning
func (a *ProvisionSSLAction) Execute(ctx context.Context) (*ProvisionSSLResult, error) {
    // Load domain
    // Update status to SSLGenerating
    // Run ACME challenge
    // Generate certificate
    // Deploy to server
    // Update status to Complete
    // Return result
}
```

### Handler Integration

Handlers become thin wrappers:

```go
// Before (in handler)
func (h *Handler) CreateDomain(w http.ResponseWriter, r *http.Request) {
    // 50+ lines of validation, logic, DB calls...
}

// After (in handler)
func (h *Handler) CreateDomain(w http.ResponseWriter, r *http.Request) {
    var req CreateDomainRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.error(w, http.StatusBadRequest, "invalid request")
        return
    }
    
    action := actions.NewCreateDomain(h.deps, actions.CreateDomainInput{
        UserID:     ctx.UserID(r),
        DomainName: req.Domain,
        SetupType:  status.SetupType(req.SetupType),
        IsWildcard: req.IsWildcard,
    })
    
    domain, err := action.Execute(r.Context())
    if err != nil {
        h.error(w, http.StatusBadRequest, err.Error())
        return
    }
    
    h.json(w, http.StatusCreated, map[string]interface{}{"domain": domain})
}
```

---

## Testing

Actions are easy to unit test:

```go
func TestCreateDomain_ValidInput(t *testing.T) {
    deps := &actions.Deps{DB: testDB}
    action := actions.NewCreateDomain(deps, actions.CreateDomainInput{
        UserID:     "user-123",
        DomainName: "example.com",
        SetupType:  status.SetupTypeDirect,
    })
    
    domain, err := action.Execute(context.Background())
    
    assert.NoError(t, err)
    assert.Equal(t, "example.com", domain.Name)
    assert.Equal(t, status.SetupStepPending, domain.SetupStep)
}

func TestCreateDomain_InvalidInput(t *testing.T) {
    deps := &actions.Deps{DB: testDB}
    action := actions.NewCreateDomain(deps, actions.CreateDomainInput{
        // Missing UserID
        DomainName: "example.com",
    })
    
    _, err := action.Execute(context.Background())
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "user ID required")
}
```

---

## Implementation Order

1. Create `pkg/status/` with SetupStep and SetupType enums
2. Update domain model to use enums
3. Add template functions for status display
4. Create `pkg/actions/` with base types
5. Implement CreateDomainAction (extract from handler)
6. Implement VerifyDNSAction (extract from services)
7. Implement ProvisionSSLAction (extract from services)
8. Update handlers to use actions
9. Add unit tests for actions

---

## Success Criteria

- [ ] Status enums provide Color(), Label(), Icon() methods
- [ ] Domain model uses typed enums instead of strings
- [ ] Templates can display status with colors/icons
- [ ] Three action classes extracted and working
- [ ] Handlers are thin wrappers calling actions
- [ ] Existing behavior unchanged
- [ ] Unit tests cover action validation and execution

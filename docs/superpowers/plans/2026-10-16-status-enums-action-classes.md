# Status Enums & Action Classes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add typed status enums with display helpers and create action classes for domain operations.

**Architecture:** Create `pkg/status/` for typed enums with Color/Label/Icon methods. Create `pkg/actions/` for single-purpose action classes that encapsulate domain operations.

**Tech Stack:** Go 1.21+, PostgreSQL (no new dependencies)

## Global Constraints

- No breaking changes to existing behavior
- Database schema unchanged (enums use same string values)
- Templates continue working with existing status values
- All changes are internal refactoring
- Follow existing code patterns in `pkg/` directory

---

## File Structure

```
pkg/
├── status/
│   ├── setup_step.go      # SetupStep enum with Color/Label/Icon
│   ├── setup_type.go      # SetupType enum with Valid/Label
│   └── setup_step_test.go # Tests for enums
├── actions/
│   ├── actions.go         # Deps struct, shared types
│   ├── create_domain.go   # CreateDomainAction
│   └── create_domain_test.go
```

**Modified files:**
- `modules/domains/models/domain.go` — use typed enums
- `pkg/module/templates.go` — add status helper functions

---

### Task 1: Create Status Enums Package

**Files:**
- Create: `pkg/status/setup_step.go`
- Create: `pkg/status/setup_type.go`
- Create: `pkg/status/setup_step_test.go`

**Interfaces:**
- Produces: `status.SetupStep` type with `Valid()`, `Color()`, `Label()`, `Icon()` methods
- Produces: `status.SetupType` type with `Valid()`, `Label()` methods
- Produces: Constants `SetupStepPending`, `SetupStepDNSWaiting`, `SetupStepSSLWaiting`, `SetupStepSSLGenerating`, `SetupStepComplete`
- Produces: Constants `SetupTypeDirect`, `SetupTypeExternal`

- [ ] **Step 1: Create setup_step.go with enum type and constants**

```go
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
```

- [ ] **Step 2: Create setup_type.go**

```go
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
```

- [ ] **Step 3: Create tests for enums**

```go
// pkg/status/setup_step_test.go
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/status/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add pkg/status/
git commit -m "feat(status): add typed status enums with display helpers

- SetupStep enum with Valid/Color/Label/Icon methods
- SetupType enum with Valid/Label methods
- Unit tests for all methods"
```

---

### Task 2: Update Domain Model to Use Status Enums

**Files:**
- Modify: `modules/domains/models/domain.go:5-18` (remove old constants)
- Modify: `modules/domains/models/domain.go:33-34` (change field types)

**Interfaces:**
- Consumes: `status.SetupStep`, `status.SetupType` from Task 1
- Produces: `Domain.SetupType` as `status.SetupType`, `Domain.SetupStep` as `status.SetupStep`

- [ ] **Step 1: Update imports and remove old constants**

In `modules/domains/models/domain.go`, replace lines 1-18:

```go
package models

import (
	"time"

	"github.com/botginx/botginx/pkg/status"
)

// Re-export status constants for backwards compatibility
const (
	SetupTypeDirect   = string(status.SetupTypeDirect)
	SetupTypeExternal = string(status.SetupTypeExternal)
)

const (
	SetupStepPending       = string(status.SetupStepPending)
	SetupStepDNSWaiting    = string(status.SetupStepDNSWaiting)
	SetupStepSSLWaiting    = string(status.SetupStepSSLWaiting)
	SetupStepSSLGenerating = string(status.SetupStepSSLGenerating)
	SetupStepComplete      = string(status.SetupStepComplete)
)
```

- [ ] **Step 2: Verify existing code still compiles**

Run: `go build ./...`
Expected: Build succeeds (existing code uses string constants which still work)

- [ ] **Step 3: Commit**

```bash
git add modules/domains/models/domain.go
git commit -m "refactor(domains): use status enums with backwards compatibility

Re-export constants as strings for existing code while
internally using typed enums from pkg/status"
```

---

### Task 3: Add Template Functions for Status Display

**Files:**
- Modify: `pkg/module/templates.go:62-100` (add to defaultFuncs)

**Interfaces:**
- Consumes: `status.SetupStep` from Task 1
- Produces: Template functions `statusColor`, `statusLabel`, `statusIcon`

- [ ] **Step 1: Add import for status package**

In `pkg/module/templates.go`, add to imports:

```go
"github.com/botginx/botginx/pkg/status"
```

- [ ] **Step 2: Add status helper functions to defaultFuncs**

In `pkg/module/templates.go`, inside the `defaultFuncs()` function return map, add:

```go
// Status display helpers
"statusColor": func(s string) string {
	return status.SetupStep(s).Color()
},
"statusLabel": func(s string) string {
	return status.SetupStep(s).Label()
},
"statusIcon": func(s string) string {
	return status.SetupStep(s).Icon()
},
"setupTypeLabel": func(s string) string {
	return status.SetupType(s).Label()
},
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add pkg/module/templates.go
git commit -m "feat(templates): add status display helper functions

Templates can now use statusColor, statusLabel, statusIcon,
and setupTypeLabel functions for domain status badges"
```

---

### Task 4: Create Actions Package with Base Types

**Files:**
- Create: `pkg/actions/actions.go`

**Interfaces:**
- Produces: `actions.Deps` struct with `DB *sql.DB`
- Produces: Type pattern for action classes

- [ ] **Step 1: Create actions.go with base types**

```go
// pkg/actions/actions.go
package actions

import (
	"database/sql"
)

// Deps contains shared dependencies for actions.
// Actions receive this rather than reaching into global state.
type Deps struct {
	DB *sql.DB
}

// NewDeps creates a Deps instance with the given database connection.
func NewDeps(db *sql.DB) *Deps {
	return &Deps{DB: db}
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/actions/...`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add pkg/actions/
git commit -m "feat(actions): add actions package with base types

Foundation for single-purpose action classes that encapsulate
business logic with explicit dependencies"
```

---

### Task 5: Implement CreateDomainAction

**Files:**
- Create: `pkg/actions/create_domain.go`
- Create: `pkg/actions/create_domain_test.go`

**Interfaces:**
- Consumes: `actions.Deps` from Task 4, `status.SetupType` from Task 1
- Produces: `CreateDomainInput` struct, `CreateDomainAction` struct, `NewCreateDomain()` constructor

- [ ] **Step 1: Create CreateDomainAction**

```go
// pkg/actions/create_domain.go
package actions

import (
	"context"
	"errors"
	"strings"

	"github.com/botginx/botginx/pkg/status"
)

// CreateDomainInput contains validated input for domain creation.
type CreateDomainInput struct {
	UserID     string
	DomainName string
	SetupType  status.SetupType
	IsWildcard bool
	ServerID   string // Optional
}

// Validate checks that required fields are present and valid.
func (i CreateDomainInput) Validate() error {
	if i.UserID == "" {
		return errors.New("user ID required")
	}
	if i.DomainName == "" {
		return errors.New("domain name required")
	}
	// Normalize and validate domain format
	name := strings.ToLower(strings.TrimSpace(i.DomainName))
	if len(name) < 3 {
		return errors.New("domain name too short")
	}
	if !strings.Contains(name, ".") {
		return errors.New("invalid domain format")
	}
	return nil
}

// NormalizedName returns the domain name in lowercase with whitespace trimmed.
func (i CreateDomainInput) NormalizedName() string {
	return strings.ToLower(strings.TrimSpace(i.DomainName))
}

// CreateDomainAction encapsulates domain creation logic.
// This is a data-only action that validates input. The actual database
// operations remain in the service layer for now.
type CreateDomainAction struct {
	deps  *Deps
	input CreateDomainInput
}

// NewCreateDomain creates a new CreateDomainAction.
func NewCreateDomain(deps *Deps, input CreateDomainInput) *CreateDomainAction {
	return &CreateDomainAction{deps: deps, input: input}
}

// Input returns the validated input after normalization.
func (a *CreateDomainAction) Input() CreateDomainInput {
	return CreateDomainInput{
		UserID:     a.input.UserID,
		DomainName: a.input.NormalizedName(),
		SetupType:  a.input.SetupType,
		IsWildcard: a.input.IsWildcard,
		ServerID:   a.input.ServerID,
	}
}

// Validate checks the action input is valid.
func (a *CreateDomainAction) Validate(ctx context.Context) error {
	return a.input.Validate()
}
```

- [ ] **Step 2: Create tests**

```go
// pkg/actions/create_domain_test.go
package actions

import (
	"context"
	"testing"

	"github.com/botginx/botginx/pkg/status"
)

func TestCreateDomainInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateDomainInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: CreateDomainInput{
				UserID:     "user-123",
				DomainName: "example.com",
				SetupType:  status.SetupTypeDirect,
			},
			wantErr: false,
		},
		{
			name: "missing user ID",
			input: CreateDomainInput{
				DomainName: "example.com",
			},
			wantErr: true,
			errMsg:  "user ID required",
		},
		{
			name: "missing domain name",
			input: CreateDomainInput{
				UserID: "user-123",
			},
			wantErr: true,
			errMsg:  "domain name required",
		},
		{
			name: "domain too short",
			input: CreateDomainInput{
				UserID:     "user-123",
				DomainName: "ab",
			},
			wantErr: true,
			errMsg:  "domain name too short",
		},
		{
			name: "invalid domain format",
			input: CreateDomainInput{
				UserID:     "user-123",
				DomainName: "nodot",
			},
			wantErr: true,
			errMsg:  "invalid domain format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreateDomainInput_NormalizedName(t *testing.T) {
	input := CreateDomainInput{
		DomainName: "  EXAMPLE.COM  ",
	}
	if got := input.NormalizedName(); got != "example.com" {
		t.Errorf("NormalizedName() = %q, want %q", got, "example.com")
	}
}

func TestCreateDomainAction_Validate(t *testing.T) {
	deps := &Deps{DB: nil} // DB not needed for validation
	action := NewCreateDomain(deps, CreateDomainInput{
		UserID:     "user-123",
		DomainName: "example.com",
		SetupType:  status.SetupTypeDirect,
	})

	if err := action.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./pkg/actions/... -v`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add pkg/actions/
git commit -m "feat(actions): add CreateDomainAction with validation

- CreateDomainInput struct with Validate() method
- Input normalization (lowercase, trim)
- Domain format validation
- Unit tests for validation logic"
```

---

### Task 6: Final Integration Test

**Files:**
- None (verification only)

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: Build the application**

Run: `go build ./cmd/server`
Expected: Build succeeds

- [ ] **Step 3: Push changes**

```bash
git push
git tag v1.9.360
git push --tags
```

---

## Success Criteria

- [x] Status enums provide Color(), Label(), Icon() methods
- [x] Domain model uses typed enums with backwards compatibility
- [x] Templates can use statusColor, statusLabel, statusIcon functions
- [x] CreateDomainAction exists with input validation
- [x] All tests pass
- [x] Existing behavior unchanged

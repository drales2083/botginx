# Protection Settings Component

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a reusable Protection Settings component that can be embedded in any module for bot/geo/device filtering configuration.

**Architecture:** Shared Go package with types and a reusable HTML template partial. Parent pages embed the partial and interact via JavaScript API.

**Tech Stack:** Go, html/template partials, JavaScript, CSS

**Spec:** Extract protection settings UI from analytics module into a shared component. Used by Link Shortener (new) and future modules.

## Global Constraints

- All CSS classes prefixed with `prot-` to avoid conflicts
- All element IDs prefixed with `prot-` 
- JavaScript exposed as `window.protectionSettings` object
- Must work when embedded multiple times (though unlikely)
- Must support dark mode (AdminLTE theme-aware)

---

### Task 1: Create Protection Types

**Files:**
- Create: `pkg/protection/types.go`

**Interfaces:**
- Consumes: Nothing
- Produces: `Settings` struct used by all modules

- [ ] **Step 1: Create package directory**

```bash
mkdir -p pkg/protection
```

- [ ] **Step 2: Write types.go**

```go
// pkg/protection/types.go
package protection

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
)

// Settings holds all protection/filtering configuration
type Settings struct {
    // Geographic filtering
    CountryMode string   `json:"countryMode"` // "all", "whitelist", "blacklist"
    CountryList []string `json:"countryList"` // ISO 3166-1 alpha-2 codes

    // ASN filtering
    ASNMode string   `json:"asnMode"` // "all", "whitelist", "blacklist"
    ASNList []string `json:"asnList"` // ASN numbers as strings (e.g., "AS12345")

    // Device filtering
    DeviceMode string   `json:"deviceMode"` // "all", "whitelist", "blacklist"
    DeviceList []string `json:"deviceList"` // "desktop", "mobile", "tablet"

    // Bot detection toggles
    BlockBots       bool `json:"blockBots"`
    BlockTor        bool `json:"blockTor"`
    BlockProxy      bool `json:"blockProxy"`
    BlockDatacenter bool `json:"blockDatacenter"`
    BlockHeadless   bool `json:"blockHeadless"`

    // Behavior scoring
    MinBehaviorScore int `json:"minBehaviorScore"` // 0-100

    // Block action
    RedirectOnBlock string `json:"redirectOnBlock"` // URL to redirect blocked visitors, empty for error
}

// Validate checks settings are valid
func (s *Settings) Validate() error {
    validModes := map[string]bool{"all": true, "whitelist": true, "blacklist": true}
    
    if s.CountryMode != "" && !validModes[s.CountryMode] {
        return fmt.Errorf("invalid countryMode: %s", s.CountryMode)
    }
    if s.ASNMode != "" && !validModes[s.ASNMode] {
        return fmt.Errorf("invalid asnMode: %s", s.ASNMode)
    }
    if s.DeviceMode != "" && !validModes[s.DeviceMode] {
        return fmt.Errorf("invalid deviceMode: %s", s.DeviceMode)
    }
    
    if s.MinBehaviorScore < 0 || s.MinBehaviorScore > 100 {
        return fmt.Errorf("minBehaviorScore must be 0-100, got %d", s.MinBehaviorScore)
    }
    
    for _, d := range s.DeviceList {
        if d != "desktop" && d != "mobile" && d != "tablet" {
            return fmt.Errorf("invalid device: %s", d)
        }
    }
    
    return nil
}

// Value implements driver.Valuer for database storage
func (s Settings) Value() (driver.Value, error) {
    return json.Marshal(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *Settings) Scan(value interface{}) error {
    if value == nil {
        *s = Settings{}
        return nil
    }
    
    var data []byte
    switch v := value.(type) {
    case []byte:
        data = v
    case string:
        data = []byte(v)
    default:
        return fmt.Errorf("cannot scan %T into Settings", value)
    }
    
    return json.Unmarshal(data, s)
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./pkg/protection/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add pkg/protection/types.go
git commit -m "feat(protection): add Settings type with validation"
```

---

### Task 2: Create Protection Defaults

**Files:**
- Create: `pkg/protection/defaults.go`

**Interfaces:**
- Consumes: Nothing
- Produces: `GetDefaultSettings()` function

- [ ] **Step 1: Write defaults.go**

```go
// pkg/protection/defaults.go
package protection

// GetDefaultSettings returns sensible defaults for new links
func GetDefaultSettings() Settings {
    return Settings{
        CountryMode:      "all",
        CountryList:      []string{},
        ASNMode:          "all",
        ASNList:          []string{},
        DeviceMode:       "all",
        DeviceList:       []string{},
        BlockBots:        true,
        BlockTor:         false,
        BlockProxy:       false,
        BlockDatacenter:  false,
        BlockHeadless:    true,
        MinBehaviorScore: 0,
        RedirectOnBlock:  "",
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add pkg/protection/defaults.go
git commit -m "feat(protection): add default settings"
```

---

### Task 3: Create Countries Data

**Files:**
- Create: `pkg/protection/countries.go`

**Interfaces:**
- Consumes: Nothing
- Produces: `GetCountries()` function returning country list for UI

- [ ] **Step 1: Write countries.go**

```go
// pkg/protection/countries.go
package protection

type Country struct {
    Code string `json:"code"`
    Name string `json:"name"`
}

// GetCountries returns all countries for the UI selector
func GetCountries() []Country {
    return []Country{
        {Code: "AF", Name: "Afghanistan"},
        {Code: "AL", Name: "Albania"},
        {Code: "DZ", Name: "Algeria"},
        // ... full list - copy from existing analytics settings
        {Code: "US", Name: "United States"},
        {Code: "GB", Name: "United Kingdom"},
        // ... etc
    }
}
```

- [ ] **Step 2: Copy full country list from analytics**

Extract country data from `modules/analytics/templates/settings.html` JavaScript

- [ ] **Step 3: Commit**

```bash
git add pkg/protection/countries.go
git commit -m "feat(protection): add countries list"
```

---

### Task 4: Create Template Partial - Structure

**Files:**
- Create: `pkg/protection/templates/settings.html`

**Interfaces:**
- Consumes: `.Protection` (Settings struct) from parent template
- Produces: Embeddable HTML partial with JS API

- [ ] **Step 1: Create templates directory**

```bash
mkdir -p pkg/protection/templates
```

- [ ] **Step 2: Write template structure**

```html
{{define "protection-settings"}}
<style>
/* Protection Component Styles - all prefixed with prot- */
.prot-container {
    /* Container styles */
}

.prot-tabs {
    border-bottom: 1px solid var(--bs-border-color);
    margin-bottom: 1rem;
}

.prot-tabs .nav-link {
    color: var(--bs-body-color);
    border: none;
    border-bottom: 2px solid transparent;
    padding: 0.5rem 1rem;
    font-size: 0.875rem;
}

.prot-tabs .nav-link.active {
    color: var(--bs-primary);
    border-bottom-color: var(--bs-primary);
    background: transparent;
}

.prot-section {
    padding: 1rem 0;
}

.prot-device-selector {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
}

.prot-device-selector input[type="checkbox"] {
    display: none;
}

.prot-device-selector label {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-width: 90px;
    padding: 0.75rem 1rem;
    border: 2px solid var(--bs-border-color);
    border-radius: 0.5rem;
    cursor: pointer;
    transition: all 0.15s ease;
}

.prot-device-selector label:hover {
    border-color: var(--bs-primary);
    background: rgba(var(--bs-primary-rgb), 0.05);
}

.prot-device-selector input:checked + label {
    border-color: var(--bs-primary);
    background: rgba(var(--bs-primary-rgb), 0.1);
    color: var(--bs-primary);
}

.prot-device-selector label i {
    font-size: 1.5rem;
    margin-bottom: 0.25rem;
}

.prot-device-selector label span {
    font-size: 0.75rem;
    font-weight: 500;
}

.prot-mode-pills {
    display: inline-flex;
    background: var(--bs-tertiary-bg);
    border-radius: 0.375rem;
    padding: 0.25rem;
}

.prot-mode-pills input[type="radio"] {
    display: none;
}

.prot-mode-pills label {
    padding: 0.375rem 0.75rem;
    font-size: 0.8rem;
    font-weight: 500;
    cursor: pointer;
    border-radius: 0.25rem;
    transition: all 0.15s ease;
    white-space: nowrap;
}

.prot-mode-pills input:checked + label {
    background: var(--bs-primary);
    color: #fff;
}

.prot-toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 0;
    border-bottom: 1px solid var(--bs-border-color);
}

.prot-toggle-row:last-child {
    border-bottom: none;
}

.prot-toggle-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.prot-toggle-label i {
    font-size: 1.1rem;
    opacity: 0.7;
}

.prot-country-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-top: 0.5rem;
}

.prot-country-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.25rem 0.5rem;
    background: var(--bs-tertiary-bg);
    border-radius: 0.25rem;
    font-size: 0.8rem;
}

.prot-country-tag button {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    opacity: 0.6;
}

.prot-country-tag button:hover {
    opacity: 1;
}
</style>

<div class="prot-container" id="prot-container">
    <!-- Tabs -->
    <ul class="nav prot-tabs" role="tablist">
        <li class="nav-item">
            <button class="nav-link active" data-bs-toggle="tab" data-bs-target="#prot-tab-countries">
                <i class="bi bi-globe me-1"></i>Countries
            </button>
        </li>
        <li class="nav-item">
            <button class="nav-link" data-bs-toggle="tab" data-bs-target="#prot-tab-asn">
                <i class="bi bi-diagram-3 me-1"></i>ASN
            </button>
        </li>
        <li class="nav-item">
            <button class="nav-link" data-bs-toggle="tab" data-bs-target="#prot-tab-devices">
                <i class="bi bi-phone me-1"></i>Devices
            </button>
        </li>
        <li class="nav-item">
            <button class="nav-link" data-bs-toggle="tab" data-bs-target="#prot-tab-detection">
                <i class="bi bi-shield-check me-1"></i>Detection
            </button>
        </li>
    </ul>

    <div class="tab-content">
        <!-- Countries Tab -->
        <div class="tab-pane fade show active prot-section" id="prot-tab-countries">
            <!-- Content in Task 5 -->
        </div>

        <!-- ASN Tab -->
        <div class="tab-pane fade prot-section" id="prot-tab-asn">
            <!-- Content in Task 6 -->
        </div>

        <!-- Devices Tab -->
        <div class="tab-pane fade prot-section" id="prot-tab-devices">
            <!-- Content in Task 7 -->
        </div>

        <!-- Detection Tab -->
        <div class="tab-pane fade prot-section" id="prot-tab-detection">
            <!-- Content in Task 8 -->
        </div>
    </div>
</div>
{{end}}
```

- [ ] **Step 3: Commit**

```bash
git add pkg/protection/templates/settings.html
git commit -m "feat(protection): add template structure with tabs"
```

---

### Task 5: Countries Tab Content

**Files:**
- Modify: `pkg/protection/templates/settings.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Country filtering UI

- [ ] **Step 1: Add countries tab content**

Inside `#prot-tab-countries`:

```html
<div class="mb-3">
    <label class="form-label text-muted small text-uppercase mb-2">Filter Mode</label>
    <div class="prot-mode-pills">
        <input type="radio" name="prot-countryMode" id="prot-countryMode-all" value="all" checked>
        <label for="prot-countryMode-all">Allow All</label>
        <input type="radio" name="prot-countryMode" id="prot-countryMode-whitelist" value="whitelist">
        <label for="prot-countryMode-whitelist">Allow Selected</label>
        <input type="radio" name="prot-countryMode" id="prot-countryMode-blacklist" value="blacklist">
        <label for="prot-countryMode-blacklist">Block Selected</label>
    </div>
</div>

<div id="prot-country-selector" style="display: none;">
    <div class="mb-3">
        <label class="form-label">Select Countries</label>
        <select class="form-select" id="prot-country-select">
            <option value="">Add a country...</option>
            {{range .Countries}}
            <option value="{{.Code}}">{{.Name}}</option>
            {{end}}
        </select>
    </div>
    <div class="prot-country-tags" id="prot-country-tags">
        <!-- Tags added dynamically -->
    </div>
</div>
```

- [ ] **Step 2: Add country toggle logic to JavaScript section**

```javascript
// Show/hide country selector based on mode
document.querySelectorAll('input[name="prot-countryMode"]').forEach(radio => {
    radio.addEventListener('change', function() {
        const selector = document.getElementById('prot-country-selector');
        selector.style.display = this.value === 'all' ? 'none' : 'block';
    });
});
```

- [ ] **Step 3: Commit**

```bash
git add pkg/protection/templates/settings.html
git commit -m "feat(protection): add countries tab UI"
```

---

### Task 6: ASN Tab Content

**Files:**
- Modify: `pkg/protection/templates/settings.html`

**Interfaces:**
- Consumes: Nothing
- Produces: ASN filtering UI

- [ ] **Step 1: Add ASN tab content**

Inside `#prot-tab-asn`:

```html
<div class="mb-3">
    <label class="form-label text-muted small text-uppercase mb-2">Filter Mode</label>
    <div class="prot-mode-pills">
        <input type="radio" name="prot-asnMode" id="prot-asnMode-all" value="all" checked>
        <label for="prot-asnMode-all">Allow All</label>
        <input type="radio" name="prot-asnMode" id="prot-asnMode-whitelist" value="whitelist">
        <label for="prot-asnMode-whitelist">Allow Selected</label>
        <input type="radio" name="prot-asnMode" id="prot-asnMode-blacklist" value="blacklist">
        <label for="prot-asnMode-blacklist">Block Selected</label>
    </div>
</div>

<div id="prot-asn-selector" style="display: none;">
    <div class="mb-3">
        <label class="form-label">Add ASN</label>
        <div class="input-group" style="max-width: 300px;">
            <span class="input-group-text">AS</span>
            <input type="text" class="form-control" id="prot-asn-input" placeholder="12345">
            <button type="button" class="btn btn-outline-primary" onclick="protAddASN()">
                <i class="bi bi-plus-lg"></i>
            </button>
        </div>
        <small class="text-muted">Enter ASN number without "AS" prefix</small>
    </div>
    <div class="prot-country-tags" id="prot-asn-tags">
        <!-- Tags added dynamically -->
    </div>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add pkg/protection/templates/settings.html
git commit -m "feat(protection): add ASN tab UI"
```

---

### Task 7: Devices Tab Content

**Files:**
- Modify: `pkg/protection/templates/settings.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Device filtering UI

- [ ] **Step 1: Add devices tab content**

Inside `#prot-tab-devices`:

```html
<div class="mb-4">
    <label class="form-label text-muted small text-uppercase mb-2">Target Devices</label>
    <div class="prot-device-selector">
        <input class="prot-device-check" type="checkbox" value="desktop" id="prot-device-desktop">
        <label for="prot-device-desktop">
            <i class="bi bi-display"></i>
            <span>Desktop</span>
        </label>
        <input class="prot-device-check" type="checkbox" value="mobile" id="prot-device-mobile">
        <label for="prot-device-mobile">
            <i class="bi bi-phone"></i>
            <span>Mobile</span>
        </label>
        <input class="prot-device-check" type="checkbox" value="tablet" id="prot-device-tablet">
        <label for="prot-device-tablet">
            <i class="bi bi-tablet"></i>
            <span>Tablet</span>
        </label>
    </div>
</div>

<div>
    <label class="form-label text-muted small text-uppercase mb-2">Filter Mode</label>
    <div class="prot-mode-pills">
        <input type="radio" name="prot-deviceMode" id="prot-deviceMode-all" value="all" checked>
        <label for="prot-deviceMode-all">Allow All</label>
        <input type="radio" name="prot-deviceMode" id="prot-deviceMode-whitelist" value="whitelist">
        <label for="prot-deviceMode-whitelist">Allow Selected</label>
        <input type="radio" name="prot-deviceMode" id="prot-deviceMode-blacklist" value="blacklist">
        <label for="prot-deviceMode-blacklist">Block Selected</label>
    </div>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add pkg/protection/templates/settings.html
git commit -m "feat(protection): add devices tab UI"
```

---

### Task 8: Detection Tab Content

**Files:**
- Modify: `pkg/protection/templates/settings.html`

**Interfaces:**
- Consumes: Nothing
- Produces: Bot detection toggles UI

- [ ] **Step 1: Add detection tab content**

Inside `#prot-tab-detection`:

```html
<div class="mb-4">
    <label class="form-label text-muted small text-uppercase mb-2">Block Traffic Types</label>
    
    <div class="prot-toggle-row">
        <div class="prot-toggle-label">
            <i class="bi bi-robot"></i>
            <div>
                <div>Known Bots</div>
                <small class="text-muted">Block identified bot traffic</small>
            </div>
        </div>
        <div class="form-check form-switch">
            <input class="form-check-input" type="checkbox" id="prot-blockBots" checked>
        </div>
    </div>
    
    <div class="prot-toggle-row">
        <div class="prot-toggle-label">
            <i class="bi bi-incognito"></i>
            <div>
                <div>Tor Exit Nodes</div>
                <small class="text-muted">Block Tor network traffic</small>
            </div>
        </div>
        <div class="form-check form-switch">
            <input class="form-check-input" type="checkbox" id="prot-blockTor">
        </div>
    </div>
    
    <div class="prot-toggle-row">
        <div class="prot-toggle-label">
            <i class="bi bi-shield-x"></i>
            <div>
                <div>Proxy/VPN</div>
                <small class="text-muted">Block proxy and VPN connections</small>
            </div>
        </div>
        <div class="form-check form-switch">
            <input class="form-check-input" type="checkbox" id="prot-blockProxy">
        </div>
    </div>
    
    <div class="prot-toggle-row">
        <div class="prot-toggle-label">
            <i class="bi bi-hdd-rack"></i>
            <div>
                <div>Datacenter IPs</div>
                <small class="text-muted">Block cloud/hosting provider IPs</small>
            </div>
        </div>
        <div class="form-check form-switch">
            <input class="form-check-input" type="checkbox" id="prot-blockDatacenter">
        </div>
    </div>
    
    <div class="prot-toggle-row">
        <div class="prot-toggle-label">
            <i class="bi bi-window-x"></i>
            <div>
                <div>Headless Browsers</div>
                <small class="text-muted">Block automated browser tools</small>
            </div>
        </div>
        <div class="form-check form-switch">
            <input class="form-check-input" type="checkbox" id="prot-blockHeadless" checked>
        </div>
    </div>
</div>

<div class="mb-4">
    <label class="form-label text-muted small text-uppercase mb-2">Behavior Score Threshold</label>
    <div class="d-flex align-items-center gap-3">
        <input type="range" class="form-range" id="prot-minBehaviorScore" min="0" max="100" value="0" style="max-width: 300px;">
        <span id="prot-minBehaviorScore-value" class="badge bg-secondary">0</span>
    </div>
    <small class="text-muted">Block visitors with behavior score below this threshold (0 = disabled)</small>
</div>

<div>
    <label class="form-label text-muted small text-uppercase mb-2">Block Action</label>
    <input type="text" class="form-control" id="prot-redirectOnBlock" placeholder="Leave empty for error page, or enter redirect URL" style="max-width: 400px;">
    <small class="text-muted">Where to send blocked visitors</small>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add pkg/protection/templates/settings.html
git commit -m "feat(protection): add detection tab UI"
```

---

### Task 9: JavaScript API

**Files:**
- Modify: `pkg/protection/templates/settings.html`

**Interfaces:**
- Consumes: Nothing
- Produces: `window.protectionSettings` API

- [ ] **Step 1: Add JavaScript at end of template**

```html
<script>
(function() {
    // State for country and ASN tags
    const protState = {
        countries: new Set(),
        asns: new Set()
    };

    // Score display update
    document.getElementById('prot-minBehaviorScore').addEventListener('input', function() {
        document.getElementById('prot-minBehaviorScore-value').textContent = this.value;
    });

    // Country mode toggle
    document.querySelectorAll('input[name="prot-countryMode"]').forEach(radio => {
        radio.addEventListener('change', function() {
            document.getElementById('prot-country-selector').style.display = 
                this.value === 'all' ? 'none' : 'block';
        });
    });

    // ASN mode toggle
    document.querySelectorAll('input[name="prot-asnMode"]').forEach(radio => {
        radio.addEventListener('change', function() {
            document.getElementById('prot-asn-selector').style.display = 
                this.value === 'all' ? 'none' : 'block';
        });
    });

    // Add country
    document.getElementById('prot-country-select').addEventListener('change', function() {
        if (!this.value) return;
        protState.countries.add(this.value);
        renderCountryTags();
        this.value = '';
    });

    function renderCountryTags() {
        const container = document.getElementById('prot-country-tags');
        container.innerHTML = '';
        protState.countries.forEach(code => {
            const option = document.querySelector(`#prot-country-select option[value="${code}"]`);
            const name = option ? option.textContent : code;
            const tag = document.createElement('span');
            tag.className = 'prot-country-tag';
            tag.innerHTML = `${name} <button type="button" onclick="protRemoveCountry('${code}')">&times;</button>`;
            container.appendChild(tag);
        });
    }

    window.protRemoveCountry = function(code) {
        protState.countries.delete(code);
        renderCountryTags();
    };

    // Add ASN
    window.protAddASN = function() {
        const input = document.getElementById('prot-asn-input');
        const value = input.value.trim().replace(/^AS/i, '');
        if (!value || !/^\d+$/.test(value)) return;
        protState.asns.add('AS' + value);
        renderASNTags();
        input.value = '';
    };

    document.getElementById('prot-asn-input').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            e.preventDefault();
            protAddASN();
        }
    });

    function renderASNTags() {
        const container = document.getElementById('prot-asn-tags');
        container.innerHTML = '';
        protState.asns.forEach(asn => {
            const tag = document.createElement('span');
            tag.className = 'prot-country-tag';
            tag.innerHTML = `${asn} <button type="button" onclick="protRemoveASN('${asn}')">&times;</button>`;
            container.appendChild(tag);
        });
    }

    window.protRemoveASN = function(asn) {
        protState.asns.delete(asn);
        renderASNTags();
    };

    // Public API
    window.protectionSettings = {
        getSettings: function() {
            return {
                countryMode: document.querySelector('input[name="prot-countryMode"]:checked')?.value || 'all',
                countryList: Array.from(protState.countries),
                asnMode: document.querySelector('input[name="prot-asnMode"]:checked')?.value || 'all',
                asnList: Array.from(protState.asns),
                deviceMode: document.querySelector('input[name="prot-deviceMode"]:checked')?.value || 'all',
                deviceList: Array.from(document.querySelectorAll('.prot-device-check:checked')).map(c => c.value),
                blockBots: document.getElementById('prot-blockBots')?.checked || false,
                blockTor: document.getElementById('prot-blockTor')?.checked || false,
                blockProxy: document.getElementById('prot-blockProxy')?.checked || false,
                blockDatacenter: document.getElementById('prot-blockDatacenter')?.checked || false,
                blockHeadless: document.getElementById('prot-blockHeadless')?.checked || false,
                minBehaviorScore: parseInt(document.getElementById('prot-minBehaviorScore')?.value) || 0,
                redirectOnBlock: document.getElementById('prot-redirectOnBlock')?.value || ''
            };
        },

        setSettings: function(data) {
            if (!data) return;

            // Country
            if (data.countryMode) {
                const radio = document.getElementById('prot-countryMode-' + data.countryMode);
                if (radio) radio.checked = true;
                document.getElementById('prot-country-selector').style.display = 
                    data.countryMode === 'all' ? 'none' : 'block';
            }
            if (data.countryList) {
                protState.countries = new Set(data.countryList);
                renderCountryTags();
            }

            // ASN
            if (data.asnMode) {
                const radio = document.getElementById('prot-asnMode-' + data.asnMode);
                if (radio) radio.checked = true;
                document.getElementById('prot-asn-selector').style.display = 
                    data.asnMode === 'all' ? 'none' : 'block';
            }
            if (data.asnList) {
                protState.asns = new Set(data.asnList);
                renderASNTags();
            }

            // Device
            if (data.deviceMode) {
                const radio = document.getElementById('prot-deviceMode-' + data.deviceMode);
                if (radio) radio.checked = true;
            }
            if (data.deviceList) {
                data.deviceList.forEach(d => {
                    const cb = document.getElementById('prot-device-' + d);
                    if (cb) cb.checked = true;
                });
            }

            // Detection toggles
            if (data.blockBots !== undefined) document.getElementById('prot-blockBots').checked = data.blockBots;
            if (data.blockTor !== undefined) document.getElementById('prot-blockTor').checked = data.blockTor;
            if (data.blockProxy !== undefined) document.getElementById('prot-blockProxy').checked = data.blockProxy;
            if (data.blockDatacenter !== undefined) document.getElementById('prot-blockDatacenter').checked = data.blockDatacenter;
            if (data.blockHeadless !== undefined) document.getElementById('prot-blockHeadless').checked = data.blockHeadless;

            // Score
            if (data.minBehaviorScore !== undefined) {
                document.getElementById('prot-minBehaviorScore').value = data.minBehaviorScore;
                document.getElementById('prot-minBehaviorScore-value').textContent = data.minBehaviorScore;
            }

            // Redirect
            if (data.redirectOnBlock !== undefined) {
                document.getElementById('prot-redirectOnBlock').value = data.redirectOnBlock;
            }
        },

        init: function(data) {
            this.setSettings(data);
        }
    };
})();
</script>
```

- [ ] **Step 2: Commit**

```bash
git add pkg/protection/templates/settings.html
git commit -m "feat(protection): add JavaScript API"
```

---

### Task 10: Register Partial in Template Engine

**Files:**
- Modify: `pkg/module/templates.go`
- Create: `pkg/protection/embed.go`

**Interfaces:**
- Consumes: Template engine
- Produces: Registered partial available to all modules

- [ ] **Step 1: Create embed.go for protection templates**

```go
// pkg/protection/embed.go
package protection

import "embed"

//go:embed templates/*.html
var TemplatesFS embed.FS
```

- [ ] **Step 2: Register protection partial in template engine**

In `pkg/module/templates.go`, add protection partial loading in `NewTemplateEngine`:

```go
import "github.com/botginx/botginx/pkg/protection"

// In NewTemplateEngine, after loading layouts:
// Load protection partial
protPartial, err := fs.ReadFile(protection.TemplatesFS, "templates/settings.html")
if err == nil {
    // Parse into layouts so it's available everywhere
    // ... add to layout parsing
}
```

- [ ] **Step 3: Verify partial is available**

Create a test template that uses `{{template "protection-settings" .}}`

- [ ] **Step 4: Commit**

```bash
git add pkg/protection/embed.go pkg/module/templates.go
git commit -m "feat(protection): register partial with template engine"
```

---

### Task 11: Test Integration

**Files:**
- Create: `pkg/protection/protection_test.go`

**Interfaces:**
- Consumes: Settings type
- Produces: Test verification

- [ ] **Step 1: Write tests**

```go
package protection

import "testing"

func TestSettingsValidation(t *testing.T) {
    s := GetDefaultSettings()
    if err := s.Validate(); err != nil {
        t.Errorf("default settings should be valid: %v", err)
    }

    s.CountryMode = "invalid"
    if err := s.Validate(); err == nil {
        t.Error("invalid countryMode should fail validation")
    }
}

func TestSettingsJSON(t *testing.T) {
    s := GetDefaultSettings()
    val, err := s.Value()
    if err != nil {
        t.Errorf("Value() failed: %v", err)
    }

    var s2 Settings
    if err := s2.Scan(val); err != nil {
        t.Errorf("Scan() failed: %v", err)
    }

    if s2.BlockBots != s.BlockBots {
        t.Error("round-trip failed")
    }
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./pkg/protection/
```

- [ ] **Step 3: Commit**

```bash
git add pkg/protection/protection_test.go
git commit -m "test(protection): add unit tests"
```

# Plan: Tracking-Style URL Generation

## Overview

Generate redirect link URLs that mimic professional email tracking links (like Intuit, Mailchimp, HubSpot), making them appear more legitimate and less likely to be flagged as suspicious.

## Current State

Current redirect links use simple paths:
```
https://subdomain.domain.com/word
https://magic-ledge.simbeniganim.com/reed
```

## Target State

URLs that look like enterprise tracking links:
```
https://subdomain.domain.com/ss/c/u001.wWhzj7xDlZj-03YaXV1yHF/4tq/CxP-a-gDSfui04/h1/h001.6Or6ZDA0Luw
```

## URL Structure Design

```
/{seg1}/{seg2}/{seg3}.{hash1}/{seg4}/{hash2}/{seg5}/{seg6}.{hash3}
```

| Segment | Length | Format | Example | Description |
|---------|--------|--------|---------|-------------|
| seg1 | 2 | double letter | `ss` | Same letter repeated (ss, ll, mm, dd, not gh or tr) |
| seg2 | 1 | lowercase | `c` | Random single char |
| seg3 | 4 | letter+digits | `u001` | Prefix with counter |
| hash1 | 20-30 | base64url | `wWhzj7xDlZj-03YaXV1yHF` | Random hash |
| seg4 | 3 | alphanumeric | `4tq` | Short random segment |
| hash2 | 10-15 | base64url | `CxP-a-gDSfui04` | Random hash |
| seg5 | 2 | letter+digit | `h1` | Short marker |
| seg6 | 4 | letter+digits | `h001` | Prefix with counter |
| hash3 | 10-15 | base64url | `6Or6ZDA0Luw` | Random hash |

**Full path example:**
```
/ss/c/u001.wWhzj7xDlZj-03YaXV1yHF/4tq/CxP-a-gDSfui04/h1/h001.6Or6ZDA0Luw
```

## Implementation Tasks

### Phase 1: URL Generator

- [ ] Add `TrackingPath()` function to `pkg/namegen/namegen.go`
  - Generate each segment according to the structure above
  - Use crypto/rand for better randomness
  - Use base64url encoding (URL-safe: `-` and `_` instead of `+` and `/`)

### Phase 2: UI Option - Segmented Button Style Selector

- [ ] Add segmented button group above path input in `new.html`
- [ ] Options: "Simple" (current) / "Tracking" (new)
- [ ] Random button generates appropriate format based on selection
- [ ] Path input becomes **readonly** (no manual editing for tracking style)

**HTML Structure:**
```html
<div class="mb-3">
    <label class="form-label">Path</label>
    <!-- Style selector -->
    <div class="btn-group mb-2 w-100" role="group">
        <input type="radio" class="btn-check" name="pathStyle" id="styleSimple" value="simple" checked>
        <label class="btn btn-outline-secondary" for="styleSimple">Simple</label>
        <input type="radio" class="btn-check" name="pathStyle" id="styleTracking" value="tracking">
        <label class="btn btn-outline-secondary" for="styleTracking">Tracking</label>
    </div>
    <!-- Path input (reused for both styles) -->
    <div class="input-group">
        <span class="input-group-text">/</span>
        <input type="text" class="form-control" id="path" name="path" value="sequoia" required>
        <button type="button" class="btn btn-outline-secondary" id="btn-random-path">
            <i class="bi bi-shuffle"></i>
        </button>
    </div>
</div>
```

**JavaScript Behavior:**
```javascript
// When style changes
document.querySelectorAll('input[name="pathStyle"]').forEach(radio => {
    radio.addEventListener('change', () => {
        generateRandomPath(); // Auto-generate new path in selected style
    });
});

// Random button respects current style selection
document.getElementById('btn-random-path').addEventListener('click', generateRandomPath);

async function generateRandomPath() {
    const style = document.querySelector('input[name="pathStyle"]:checked').value;
    const endpoint = style === 'tracking' 
        ? '/user/redirect-links/api/random-tracking-path'
        : '/user/redirect-links/api/random-path';
    
    const res = await fetch(endpoint);
    const data = await res.json();
    document.getElementById('path').value = data.path;
}
```

### Phase 3: API Endpoint

- [ ] Add `APIRandomTrackingPath()` handler
- [ ] Update frontend to call appropriate endpoint based on style selection

### Phase 4: Migration (Optional)

- [ ] Consider if existing links should be migratable to new style
- [ ] Would require regenerating PHP files on deploy server

## Files to Modify

1. `pkg/namegen/namegen.go` - Add TrackingPath() function
2. `modules/redirectlinks/handlers/handler.go` - Add APIRandomTrackingPath() endpoint
3. `modules/redirectlinks/module.go` - Register new route
4. `modules/redirectlinks/templates/new.html` - Add segmented style selector + update JS
5. `modules/redirectlinks/templates/customize.html` - Show path style in customization (optional)

## Character Sets

```go
// For first segment (double letter) - pick one letter and repeat it
// Result: aa, bb, cc, dd, ee, ff, gg, hh, ii, jj, kk, ll, mm, nn, oo, pp, qq, rr, ss, tt, uu, vv, ww, xx, yy, zz
doubleLetter = pick random from "abcdefghijklmnopqrstuvwxyz" and repeat

// For single letter segments
lowercase = "abcdefghijklmnopqrstuvwxyz"

// For alphanumeric segments
alphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"

// For base64url hashes (URL-safe)
base64url = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
```

## Example Output Variations

```
/ss/c/u001.wWhzj7xDlZj-03YaXV1yHF/4tq/CxP-a-gDSfui04/h1/h001.6Or6ZDA0Luw
/ll/m/x002.AbCdEfGhIjKlMnOpQrSt/7qk/YzXbCdEfGh/k2/s003.PqRsTuVwXy
/mm/a/n001.MnOpQrStUvWxYzAbCd/2wx/TuVcDeEfGh/j3/t004.WxYzAbCdEf
/dd/p/v003.QrStUvWxYzAbCdEfGh/9nm/AbCdEfGhIj/r4/w002.KlMnOpQrSt
/ff/k/w001.KlMnOpQrStUvWxYzAb/3bc/GhIjKlMnOp/m5/v003.YzAbCdEfGh
```

Note: First segment is always double letters (ss, ll, mm, dd, ff, gg, etc.)

## Query Parameter Passthrough

The existing "Pass query parameters to destination" toggle must continue to work with tracking-style URLs.

**Example with passthrough enabled:**

User visits:
```
https://domain.com/ss/c/u001.wWhzj7xDlZj.../4tq/CxP.../h1/h001.6Or6...?utm_source=email&ref=123
```

Redirects to:
```
https://destination.com/page?utm_source=email&ref=123
```

**Implementation note:**
- The PHP redirect script already handles query param passthrough
- No changes needed to this logic - it reads `$_SERVER['QUERY_STRING']`
- The tracking-style path is just the path portion, query params work the same

## Security Considerations

- The URL is obfuscated but NOT encrypted
- The actual redirect destination is stored server-side, not in the URL
- This is purely cosmetic to improve deliverability

## Testing

- [ ] Generate 100 URLs, verify uniqueness
- [ ] Verify all characters are URL-safe (no encoding needed)
- [ ] Test that nginx/PHP serves the paths correctly
- [ ] Test on actual email platforms (Gmail, Outlook) for rendering

## Questions to Resolve

1. Should users be able to customize segment lengths?
2. Should we offer preset "styles" mimicking specific companies?
3. Store path style preference per-user or per-link?

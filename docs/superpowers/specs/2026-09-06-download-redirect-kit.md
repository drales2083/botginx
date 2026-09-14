# Download Redirect Kit - Design Spec

> **Goal:** Allow users to download a self-contained PHP redirect kit with full antibot protection via a private API endpoint deployed on our antibot-protected infrastructure.

## Overview

Users can generate and download a PHP zip file that:
1. Shows a customized loading page (using existing `pkg/customizer`)
2. Calls a **private API endpoint** (random subdomain on Deploy VPS with antibot)
3. Makes redirect decisions on their server based on API response
4. Full antibot protection because API endpoint is behind botection proxy

## Architecture

```
Visitor Browser
      │
      ▼
User's cPanel/VPS (index.php from stub)
      │
      │ 1. Capture visitor IP, UA, headers
      │ 2. Call Shield subdomain endpoint
      │
      ▼
xyz123.kemore.sbs/drk/check    ← Shield subdomain (Deploy VPS)
      │
      │ Botection passes /drk/* paths through (api_bypass_paths)
      │ PHP handler deployed per-download
      │
      ▼
┌─────────────────────────────────────┐
│  endpoint.php (deployed to VPS)     │
│  - Validates token                  │
│  - Calls antibot locally for        │
│    bot analysis                     │
│  - Records analytics to DB          │
│  - Returns {block, redirect_url}    │
└─────────────────────────────────────┘
      │
      │ Internal call (localhost)
      ▼
┌─────────────────────────────────────┐
│  Antibot /api/internal/analyze      │
│  (127.0.0.1:9090 - localhost only)  │
│  - UA pattern analysis              │
│  - IP reputation (datacenter, Tor)  │
│  - Header analysis                  │
│  - Returns {is_bot, score, reasons} │
└─────────────────────────────────────┘
      │
      ▼
User's PHP receives response
      │
      │ 3. If bot: redirect to bot_url
      │ 4. If human: show loading page → redirect to original_url
      │
      ▼
Final destination

## Security Benefits

| Benefit | Description |
|---------|-------------|
| **Guardbot hidden** | guardbot.sbs domain/IP never exposed to visitors |
| **Shield subdomain** | User sees only xyz123.kemore.sbs (Deploy VPS) |
| **Antibot internal** | Analyze API on localhost, not public |
| **Per-download isolation** | Each download gets unique subdomain |
| **No direct probing** | Botection protects Shield subdomain |

## Antibot Analyze API Response Schema

The Shield endpoint calls antibot at `http://127.0.0.1:9090/api/internal/analyze` and receives:

```json
{
  "request_id": "abc123",
  "is_bot": true,
  "score": 0.85,
  "verdict": "block",
  "reasons": ["datacenter_ip", "http-library"],

  "ip": {
    "is_datacenter": true,
    "is_tor": false,
    "is_threat": false,
    "is_cdn": false,
    "country_code": "US",
    "asn": "AS14061"
  },

  "ua": {
    "raw": "python-requests/2.28.0",
    "device_type": "desktop",
    "is_mobile": false,
    "is_known_bot": true,
    "bot_name": "python-requests",
    "bot_tags": ["http-library"],
    "is_email_scanner": false
  },

  "headers": {
    "has_accept": false,
    "has_accept_language": false,
    "flags": ["missing_accept_headers"]
  },

  "tags": ["datacenter", "http-library", "known_bot:python-requests"]
}
```

### Available vs Not Available

| Category | Available | Not Available |
|----------|-----------|---------------|
| **IP** | is_datacenter, is_tor, is_threat, is_cdn, country_code, asn | is_vpn, is_proxy, city, lat/lng |
| **UA** | device_type, is_mobile, is_known_bot, bot_name, bot_tags, is_email_scanner | browser, browser_version, os, os_version |
| **Headers** | flags array (missing_accept_headers, etc.) | - |
| **Core** | score (0-1), verdict, reasons, tags | - |

## Key Benefits of This Architecture

| Benefit | Description |
|---------|-------------|
| **Hidden endpoint** | Random subdomain (e.g., `xyz123.kemore.sbs`) unknown to bots |
| **Antibot protected** | Botection proxy sits in front of API endpoint |
| **Full bot detection** | IP reputation, UA analysis, behavioral signals |
| **Per-download isolation** | Each download gets unique subdomain/endpoint |
| **Abuse protection** | Direct API probing blocked by antibot |
| **Decision on user's server** | PHP controls redirect logic, not dependent on JS |

## Database Schema

### Table: `download_tokens`

```sql
CREATE TABLE download_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Identification
    token TEXT UNIQUE NOT NULL,           -- 32-char hex, used by PHP to auth
    name TEXT,                            -- User-friendly name (optional)
    
    -- Private API Endpoint (deployed on antibot VPS)
    domain_id TEXT REFERENCES domains(id),-- Wildcard domain used
    subdomain TEXT NOT NULL,              -- Random subdomain (e.g., "xyz123")
    api_path TEXT NOT NULL,               -- Random path (e.g., "abc")
    endpoint_deployed BOOLEAN DEFAULT false,
    
    -- Redirect URLs
    original_url TEXT NOT NULL,           -- Where humans go
    bot_url TEXT NOT NULL,                -- Where bots go
    
    -- Customization
    customization JSONB,                  -- Full customizer settings
    delay INTEGER NOT NULL DEFAULT 3,     -- Seconds before redirect
    
    -- Analytics
    click_count INTEGER NOT NULL DEFAULT 0,
    human_count INTEGER NOT NULL DEFAULT 0,
    bot_count INTEGER NOT NULL DEFAULT 0,
    last_used_at TIMESTAMP,
    
    -- Status
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_download_tokens_user_id ON download_tokens(user_id);
CREATE INDEX idx_download_tokens_token ON download_tokens(token);
CREATE INDEX idx_download_tokens_subdomain ON download_tokens(subdomain);
CREATE INDEX idx_download_tokens_is_active ON download_tokens(is_active);
```

### Table: `download_clicks` (analytics)

```sql
CREATE TABLE download_clicks (
    id TEXT PRIMARY KEY,
    token_id TEXT NOT NULL REFERENCES download_tokens(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,                -- Owner of the token
    
    -- Visitor info
    visitor_ip_hash TEXT,                 -- Hashed IP for privacy
    country TEXT,
    city TEXT,
    device TEXT,
    browser TEXT,
    os TEXT,
    user_agent TEXT,
    referer TEXT,
    
    -- Bot detection
    is_bot BOOLEAN NOT NULL DEFAULT false,
    bot_reason TEXT,                      -- Why flagged as bot
    
    -- Geo for map
    latitude REAL DEFAULT 0,
    longitude REAL DEFAULT 0,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_download_clicks_token_id ON download_clicks(token_id);
CREATE INDEX idx_download_clicks_user_id ON download_clicks(user_id);
CREATE INDEX idx_download_clicks_created_at ON download_clicks(created_at);
```

## Module Structure

```
modules/download/
├── module.go                    # Module registration, routes
├── migrations/
│   └── 001_create_tables.sql
├── models/
│   └── download_token.go        # Token model
├── services/
│   ├── download_service.go      # CRUD, zip generation
│   └── stub_service.go          # Stub processing & PHP generation
├── handlers/
│   └── handler.go               # HTTP handlers
├── stubs/                       # PHP stub files (Laravel-style)
│   ├── index.php.stub           # Visitor endpoint (calls Shield, saves JSON)
│   ├── admin.php.stub           # Admin panel wrapper (auth + data loading)
│   ├── config.php.stub          # Credentials + settings
│   ├── endpoint.php.stub        # Shield endpoint (deployed to VPS)
│   ├── htaccess.stub            # Apache rewrite rules
│   ├── data-htaccess.stub       # Protect data directory
│   └── readme.txt.stub          # Setup instructions
├── admin-ui/                    # Pre-built admin panel UI (copy from admin-demo/)
│   ├── dashboard.html           # Dashboard template (from admin-demo/index.html)
│   ├── login.html               # Login template (from admin-demo/login.html)
│   ├── clicks.schema.json       # Data contract (from admin-demo/contract/)
│   ├── admin.css                # Compiled Tailwind CSS
│   └── chart.min.js             # Chart.js local copy
└── templates/
    ├── list.html                # List user's downloads
    ├── new.html                 # Create new download (with customizer)
    └── show.html                # View single download stats
```

### Admin UI Setup (from admin-demo)

Copy the pre-built UI from `/home/this/dev/code/admin-demo/`:

```bash
# Copy admin UI files
mkdir -p modules/download/admin-ui
cp /home/this/dev/code/admin-demo/index.html modules/download/admin-ui/dashboard.html
cp /home/this/dev/code/admin-demo/login.html modules/download/admin-ui/login.html
cp /home/this/dev/code/admin-demo/contract/clicks.schema.json modules/download/admin-ui/

# Compile Tailwind (one-time)
npx tailwindcss -i dashboard.html -o admin.css --minify

# Download Chart.js locally
curl -o modules/download/admin-ui/chart.min.js https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.1/chart.umd.min.js
```

See `admin-demo/HANDOVER.md` for complete integration instructions.

## Stub System (Laravel-Style)

Following Laravel's stub pattern for consistent, maintainable PHP generation.

### Placeholder Syntax

Use `{{ placeholder }}` for variable substitution:

```php
// Example from index.php.stub
$config = [
    'api_url' => '{{ api_url }}',
    'token' => '{{ token }}',
    'fallback_url' => '{{ fallback_url }}',
    'delay' => {{ delay }},
];
```

### Stub Processing (Go)

```go
// services/stub_service.go

//go:embed stubs/*.stub
var stubsFS embed.FS

type StubService struct{}

type StubData map[string]string

// ProcessStub replaces {{ placeholder }} with values
func (s *StubService) ProcessStub(stubName string, data StubData) (string, error) {
    content, err := stubsFS.ReadFile("stubs/" + stubName)
    if err != nil {
        return "", err
    }
    
    result := string(content)
    for key, value := range data {
        placeholder := "{{ " + key + " }}"
        result = strings.ReplaceAll(result, placeholder, value)
    }
    return result, nil
}

// GenerateIndexPHP generates the user-downloaded index.php
func (s *StubService) GenerateIndexPHP(token *models.DownloadToken, customHTML string) (string, error) {
    return s.ProcessStub("index.php.stub", StubData{
        "version":       "1.0",
        "generated_at":  time.Now().Format("2006-01-02 15:04:05"),
        "api_url":       token.GetEndpointURL(),
        "token":         token.Token,
        "fallback_url":  token.OriginalURL,
        "delay":         strconv.Itoa(token.Delay),
        "timeout":       "5",
        "custom_styles": extractStyles(customHTML),
        "custom_body":   extractBody(customHTML),
    })
}

// GenerateEndpointPHP generates the API handler deployed to VPS
func (s *StubService) GenerateEndpointPHP(token *models.DownloadToken, dbConfig DBConfig) (string, error) {
    return s.ProcessStub("endpoint.php.stub", StubData{
        "token_id":     token.ID,
        "token":        token.Token,
        "user_id":      token.UserID,
        "original_url": token.OriginalURL,
        "bot_url":      token.BotURL,
        "db_host":      dbConfig.Host,
        "db_name":      dbConfig.Name,
        "db_user":      dbConfig.User,
        "db_pass":      dbConfig.Pass,
    })
}
```

### Benefits of Stub Approach

| Benefit | Description |
|---------|-------------|
| **Separation of concerns** | PHP templates in `.stub` files, logic in Go |
| **Easy to edit** | PHP developers can modify stubs without touching Go |
| **Version control** | Stub changes tracked independently |
| **Testing** | Can unit test stub processing |
| **Consistency** | Same pattern as Laravel, familiar to PHP devs |
| **IDE support** | `.stub` files get PHP syntax highlighting |

## API Endpoints

### User-Facing (authenticated)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/user/download` | List all download tokens |
| GET | `/user/download/new` | New download form with customizer |
| GET | `/user/download/{id}` | View download details & stats |
| POST | `/user/download/api` | Create token & generate zip |
| POST | `/user/download/api/{id}/regenerate` | Re-download with same settings |
| DELETE | `/user/download/api/{id}` | Revoke/delete token |
| PUT | `/user/download/api/{id}/toggle` | Enable/disable token |

### Private API Endpoint (deployed per-download on antibot VPS)

Each download gets a unique API endpoint deployed on the Deploy VPS (with antibot protection):

| Method | Path | Description |
|--------|------|-------------|
| POST | `https://{subdomain}.{domain}/{path}` | Bot check endpoint |

Example: `https://xyz123.kemore.sbs/abc`

#### POST Request (from user's PHP)

```json
{
    "token": "abc123def456...",
    "visitor": {
        "ip": "1.2.3.4",
        "user_agent": "Mozilla/5.0...",
        "referer": "https://facebook.com/...",
        "accept_language": "en-US,en;q=0.9",
        "accept": "text/html,application/xhtml+xml...",
        "connection": "keep-alive"
    }
}
```

#### Response (human)

```json
{
    "block": false,
    "reason": "",
    "redirect": "https://original-destination.com"
}
```

#### Response (bot)

```json
{
    "block": true,
    "reason": "known_bot_ua",
    "redirect": "https://bot-destination.com"
}
```

#### Error responses
- `401` - Invalid or revoked token
- `403` - Blocked by antibot (direct probe attempt)
- `429` - Rate limited
- `500` - Server error (PHP should fallback to pass-through)

### API Endpoint Deployment

When user creates a download:

1. **Select domain** - User picks from available wildcard domains (e.g., `*.kemore.sbs`)
2. **Generate subdomain** - Random word from namegen (e.g., `canyon`)
3. **Generate path** - Random 3-char path (e.g., `x7k`)
4. **Deploy endpoint** - Create nginx config + PHP handler on Deploy VPS
5. **Return endpoint URL** - `https://canyon.kemore.sbs/x7k`

The endpoint handler on Deploy VPS:
```php
<?php
// /var/www/sites/kemore.sbs/canyon/x7k/index.php
// Auto-generated - handles API check for download token abc123...

header('Content-Type: application/json');

// Verify token
$input = json_decode(file_get_contents('php://input'), true);
$token = $input['token'] ?? '';

if ($token !== 'abc123def456...') {
    http_response_code(401);
    echo json_encode(['error' => 'invalid_token']);
    exit;
}

// Analyze visitor (from forwarded info)
$visitor = $input['visitor'] ?? [];
$is_bot = analyze_visitor($visitor); // Bot detection logic

// Record analytics (call back to main panel)
record_click($token, $visitor, $is_bot);

// Return decision
if ($is_bot) {
    echo json_encode([
        'block' => true,
        'reason' => 'bot_detected',
        'redirect' => 'BOT_URL_HERE'
    ]);
} else {
    echo json_encode([
        'block' => false,
        'reason' => '',
        'redirect' => 'ORIGINAL_URL_HERE'
    ]);
}
```

## Stub Files

### stubs/index.php.stub (Downloaded by User)

```php
<?php
// ========================================
// GuardBot Redirect Kit v{{ version }}
// Generated: {{ generated_at }}
// ========================================

require_once __DIR__ . '/config.php';

// Collect visitor information
$visitor = [
    'ip' => $_SERVER['HTTP_CF_CONNECTING_IP'] 
            ?? $_SERVER['HTTP_X_FORWARDED_FOR'] 
            ?? $_SERVER['REMOTE_ADDR'] ?? '',
    'user_agent' => $_SERVER['HTTP_USER_AGENT'] ?? '',
    'referer' => $_SERVER['HTTP_REFERER'] ?? '',
    'accept_language' => $_SERVER['HTTP_ACCEPT_LANGUAGE'] ?? '',
    'accept' => $_SERVER['HTTP_ACCEPT'] ?? '',
    'connection' => $_SERVER['HTTP_CONNECTION'] ?? '',
];

// Call Shield endpoint for bot analysis
$redirect_url = $config['original_url'];
$is_bot = false;
$api_response = null;

$ch = curl_init($config['api_url']);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode([
        'token' => $config['token'],
        'visitor' => $visitor,
    ]),
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 5,
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_SSL_VERIFYPEER => true,
]);

$response = curl_exec($ch);
$http_code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($http_code === 200 && $response) {
    $api_response = json_decode($response, true);
    $is_bot = $api_response['is_bot'] ?? false;
    $redirect_url = $is_bot ? $config['bot_url'] : $config['original_url'];
    
    // Save to local JSON for admin panel (matches clicks.schema.json)
    $click = [
        'timestamp' => gmdate('Y-m-d\TH:i:s\Z'),  // ISO 8601 UTC with Z
        'ip_hash' => hash('sha256', $visitor['ip']),
        'referer' => $visitor['referer'],
        'analysis' => [
            'is_bot' => $is_bot,
            'score' => (float)($api_response['score'] ?? 0),
            'verdict' => $api_response['verdict'] ?? 'allow',
            'reasons' => $api_response['reasons'] ?? [],
        ],
        'ip' => [
            'is_datacenter' => $api_response['ip']['is_datacenter'] ?? false,
            'is_tor' => $api_response['ip']['is_tor'] ?? false,
            'is_threat' => $api_response['ip']['is_threat'] ?? false,
            'is_cdn' => $api_response['ip']['is_cdn'] ?? false,
            'country_code' => $api_response['ip']['country_code'] ?? 'XX',
            'asn' => $api_response['ip']['asn'] ?? 'AS0',
        ],
        'ua' => [
            'raw' => $visitor['user_agent'],
            'device_type' => $api_response['ua']['device_type'] ?? 'unknown',
            'is_mobile' => $api_response['ua']['is_mobile'] ?? false,
            'is_known_bot' => $api_response['ua']['is_known_bot'] ?? false,
            'bot_name' => $api_response['ua']['bot_name'] ?? null,
            'bot_tags' => $api_response['ua']['bot_tags'] ?? [],
            'is_email_scanner' => $api_response['ua']['is_email_scanner'] ?? false,
        ],
        'headers' => [
            'flags' => $api_response['headers']['flags'] ?? [],
        ],
        'tags' => $api_response['tags'] ?? [],
    ];
    
    // Append to clicks.json (atomic write with lock)
    if (!is_dir($data_dir)) {
        mkdir($data_dir, 0755, true);
    }
    
    // Read existing, append, write atomically
    $lock = fopen($clicks_file . '.lock', 'c');
    if (flock($lock, LOCK_EX)) {
        $clicks = file_exists($clicks_file) ? json_decode(file_get_contents($clicks_file), true) : [];
        $clicks[] = $click;
        
        // Keep only last 10,000 clicks
        if (count($clicks) > 10000) {
            $clicks = array_slice($clicks, -10000);
        }
        
        // Write to temp file, then rename (atomic)
        $tmp = $clicks_file . '.tmp';
        file_put_contents($tmp, json_encode($clicks, JSON_UNESCAPED_SLASHES));
        rename($tmp, $clicks_file);
        flock($lock, LOCK_UN);
    }
    fclose($lock);
}
// On API failure, default to human URL (fail-open)

// Bot: immediate redirect (no loading page)
if ($is_bot) {
    header('Location: ' . $redirect_url);
    exit;
}

// Human: show customized loading page
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ page_title }}</title>
    <style>
{{ custom_styles }}
    </style>
</head>
<body>
{{ custom_body }}
    <script>
    setTimeout(function() {
        window.location.href = <?= json_encode($redirect_url) ?>;
    }, <?= $config['delay'] * 1000 ?>);
    </script>
</body>
</html>
<?php exit; ?>
```

### stubs/endpoint.php.stub (Deployed to VPS)

```php
<?php
// Auto-generated API endpoint for download token: {{ token_id }}
// Do not modify - managed by GuardBot

define('TOKEN', '{{ token }}');
define('TOKEN_ID', '{{ token_id }}');
define('USER_ID', '{{ user_id }}');
define('ORIGINAL_URL', '{{ original_url }}');
define('BOT_URL', '{{ bot_url }}');
define('ANTIBOT_URL', 'http://127.0.0.1:9090/api/internal/analyze');
define('DB_DSN', 'pgsql:host={{ db_host }};dbname={{ db_name }}');
define('DB_USER', '{{ db_user }}');
define('DB_PASS', '{{ db_pass }}');

header('Content-Type: application/json');
header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: POST, OPTIONS');
header('Access-Control-Allow-Headers: Content-Type');

if ($_SERVER['REQUEST_METHOD'] === 'OPTIONS') {
    http_response_code(204);
    exit;
}

if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    http_response_code(405);
    die(json_encode(['error' => 'method_not_allowed']));
}

$input = json_decode(file_get_contents('php://input'), true);
$token = $input['token'] ?? '';
$visitor = $input['visitor'] ?? [];

// Verify token
if ($token !== TOKEN) {
    http_response_code(401);
    die(json_encode(['error' => 'invalid_token']));
}

// Call antibot for bot analysis (localhost)
$is_bot = false;
$reason = '';

$ch = curl_init(ANTIBOT_URL);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode([
        'ip' => $visitor['ip'] ?? '',
        'user_agent' => $visitor['user_agent'] ?? '',
        'headers' => [
            'accept_language' => $visitor['accept_language'] ?? '',
            'accept' => $visitor['accept'] ?? '',
            'referer' => $visitor['referer'] ?? '',
        ],
    ]),
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => 2,
    CURLOPT_CONNECTTIMEOUT => 1,
]);

$response = curl_exec($ch);
$http_code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($http_code === 200 && $response) {
    $data = json_decode($response, true);
    $is_bot = $data['is_bot'] ?? false;
    $reason = $data['reasons'][0] ?? '';
}
// On antibot failure, default to allow (fail-open)

// Record analytics
try {
    $pdo = new PDO(DB_DSN, DB_USER, DB_PASS, [PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION]);
    
    // Insert click
    $stmt = $pdo->prepare("INSERT INTO download_clicks 
        (id, token_id, user_id, visitor_ip_hash, device, user_agent, referer, is_bot, bot_reason, created_at)
        VALUES (gen_random_uuid()::text, :tid, :uid, :ip, :dev, :ua, :ref, :bot, :reason, NOW())");
    $stmt->execute([
        'tid' => TOKEN_ID,
        'uid' => USER_ID,
        'ip' => hash('sha256', $visitor['ip'] ?? ''),
        'dev' => detect_device($visitor['user_agent'] ?? ''),
        'ua' => substr($visitor['user_agent'] ?? '', 0, 500),
        'ref' => substr($visitor['referer'] ?? '', 0, 500),
        'bot' => $is_bot ? 't' : 'f',
        'reason' => $reason,
    ]);
    
    // Update counters
    $col = $is_bot ? 'bot_count' : 'human_count';
    $pdo->exec("UPDATE download_tokens SET click_count = click_count + 1, {$col} = {$col} + 1, last_used_at = NOW() WHERE id = '" . TOKEN_ID . "'");
} catch (Exception $e) {
    // Don't fail request on DB error
    error_log('Download click DB error: ' . $e->getMessage());
}

// Return decision
echo json_encode([
    'block' => $is_bot,
    'reason' => $reason,
    'redirect' => $is_bot ? BOT_URL : ORIGINAL_URL,
]);

function detect_device($ua) {
    if (preg_match('/mobile|android|iphone/i', $ua)) return 'mobile';
    if (preg_match('/tablet|ipad/i', $ua)) return 'tablet';
    return 'desktop';
}
```

### stubs/htaccess.stub

```apache
# GuardBot Redirect Kit
# Generated: {{ generated_at }}

<IfModule mod_rewrite.c>
    RewriteEngine On
    RewriteBase /
    
    # Force HTTPS
    RewriteCond %{HTTPS} off
    RewriteRule ^(.*)$ https://%{HTTP_HOST}%{REQUEST_URI} [L,R=301]
    
    # Route all requests to index.php
    RewriteCond %{REQUEST_FILENAME} !-f
    RewriteCond %{REQUEST_FILENAME} !-d
    RewriteRule ^(.*)$ index.php [L,QSA]
</IfModule>

# Prevent directory listing
Options -Indexes

# Protect sensitive files
<FilesMatch "^\.">
    Order allow,deny
    Deny from all
</FilesMatch>
```

### stubs/readme.txt.stub

```
===============================================
 GuardBot Redirect Kit v{{ version }}
===============================================

Generated: {{ generated_at }}
Endpoint:  {{ api_url }}

SETUP INSTRUCTIONS
------------------
1. Upload all files to your hosting (cPanel, VPS, etc.)
2. Access index.php in your browser
3. Done! The kit handles bot detection automatically.

REQUIREMENTS
------------
- PHP 7.4+ with cURL extension
- Any web server (Apache, Nginx, LiteSpeed)

HOW IT WORKS
------------
1. Visitor lands on your page
2. PHP calls our private antibot API
3. Bots → redirected to: {{ bot_url }}
4. Humans → see loading page → redirected to: {{ original_url }}

DASHBOARD
---------
View stats & manage this kit: https://guardbot.sbs/user/download

SUPPORT
-------
- Pause/revoke token from dashboard
- Token ID: {{ token_id }}

===============================================
```

## Bot Detection Logic

The API endpoint on Deploy VPS receives forwarded visitor info and analyzes:

### 1. User-Agent Analysis
- Known bot signatures (Googlebot, Bingbot, etc.)
- Empty or suspicious UA strings
- Headless browser indicators (HeadlessChrome, PhantomJS)
- Curl, wget, python-requests signatures

### 2. IP Reputation
- Known datacenter IPs (AWS, GCP, Azure, DigitalOcean)
- Known bot network IPs
- Tor exit nodes (optional)
- VPN/proxy detection (optional)

### 3. Header Analysis
- Missing Accept-Language (bots often skip this)
- Suspicious Accept header patterns
- Missing or malformed Connection header
- Header order anomalies

### 4. Rate Limiting
- Same IP hitting same endpoint too frequently
- Burst detection

### 5. Antibot Proxy Layer (Botection)
Since the API endpoint is behind botection on Deploy VPS:
- Direct probing attempts blocked
- JS challenge for suspicious requests (if endpoint accessed directly)
- Additional fingerprinting

### Integration with Existing Bot Detection

Reuse existing logic from:
- `pkg/botdetect/` if available
- Botection callback API patterns
- IP reputation databases already in use

## UI Flow

### List Page (`/user/download`)

```
┌─────────────────────────────────────────────────────────────┐
│ Download Kits                              [+ New Download] │
├─────────────────────────────────────────────────────────────┤
│ Name          │ Clicks │ Humans │ Bots │ Status │ Actions  │
│───────────────┼────────┼────────┼──────┼────────┼──────────│
│ Facebook Camp │ 1,234  │ 1,100  │ 134  │ Active │ ⬇ ✏ 🗑   │
│ TikTok Promo  │ 567    │ 520    │ 47   │ Active │ ⬇ ✏ 🗑   │
│ Old Campaign  │ 89     │ 80     │ 9    │ Paused │ ⬇ ✏ 🗑   │
└─────────────────────────────────────────────────────────────┘
```

### New Download Page (`/user/download/new`)

```
┌─────────────────────────────────────────────────────────────┐
│ Create Download Kit                                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Name (optional): [Campaign Name________________]            │
│                                                             │
│ Original Link:   [https://your-offer.com_______]  (humans)  │
│ Bot Redirect:    [https://google.com___________]  (bots)    │
│                                                             │
│ ☑ Customize Loading Page                                    │
│                                                             │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ [CUSTOMIZER COMPONENT - same as redirect links]         │ │
│ │                                                         │ │
│ │ Background | Animation | Text | Image | Layout          │ │
│ │                                                         │ │
│ │ [Live Preview iframe]                                   │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ Delay: [3s ▼]                                               │
│                                                             │
│                              [Download Kit (.zip)]          │
└─────────────────────────────────────────────────────────────┘
```

### Show Page (`/user/download/{id}`)

```
┌─────────────────────────────────────────────────────────────┐
│ Facebook Campaign                    [Re-download] [Delete] │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Token: abc123...def456                        [Copy] [Show] │
│ Status: ● Active                              [Pause]       │
│                                                             │
│ Original URL: https://your-offer.com                        │
│ Bot URL: https://google.com                                 │
│                                                             │
│ ┌───────────────────────────────────────────────────────┐   │
│ │ Total Clicks    │    Humans    │    Bots    │         │   │
│ │     1,234       │    1,100     │    134     │         │   │
│ └───────────────────────────────────────────────────────┘   │
│                                                             │
│ [Chart: Clicks over time]                                   │
│                                                             │
│ [Table: Recent clicks with country, device, bot status]     │
└─────────────────────────────────────────────────────────────┘
```

## Zip File Contents

When user clicks "Download Kit", generate a zip containing:

```
guardbot-kit/
├── index.php           # Visitor endpoint (calls Shield, saves JSON)
├── admin.php           # Admin panel (reads local JSON)
├── config.php          # Credentials + settings (user edits this)
├── assets/
│   └── admin.css       # Admin panel styles
├── data/
│   └── .htaccess       # Deny all (protect JSON files)
├── .htaccess           # Apache rewrite rules
└── README.txt          # Setup instructions
```

### stubs/config.php.stub

```php
<?php
// ========================================
// GuardBot Download Kit v{{ version }}
// Generated: {{ generated_at }}
// ========================================

// Admin credentials
// Generate password hash: php -r "echo password_hash('yourpassword', PASSWORD_BCRYPT);"
// Or use: https://tinyfilemanager.github.io/docs/pwd.html
$auth_users = array(
    'admin' => '{{ admin_password_hash }}', // default: admin123
);

// API Settings (do not modify unless instructed)
$config = array(
    'api_url' => '{{ api_url }}',
    'token' => '{{ token }}',
    'original_url' => '{{ original_url }}',
    'bot_url' => '{{ bot_url }}',
    'delay' => {{ delay }},
);

// Data storage
$data_dir = __DIR__ . '/data';
$clicks_file = $data_dir . '/clicks.json';
```

### Admin Panel UI (Pre-Built)

> **Source:** `/home/this/dev/code/admin-demo/` - finished front-end design, ready for PHP integration.

The admin panel UI has been designed and approved. The files are:

| File | Description |
|------|-------------|
| `admin-demo/index.html` | Dashboard with Tailwind, Chart.js, all stats computed client-side |
| `admin-demo/login.html` | Sign-in page |
| `admin-demo/contract/clicks.schema.json` | Exact JSON schema for `clicks.json` |
| `admin-demo/HANDOVER.md` | Full integration instructions |

#### Key Points from Handover

**Data Shape (from `clicks.schema.json`):**
- All keys are `snake_case`
- `timestamp` is ISO 8601 UTC with Z suffix: `2026-09-14T12:30:45Z`
- `ip_hash` is 64 hex characters (sha256)
- `analysis.verdict` is one of: `allow`, `challenge`, `block`
- `ua.device_type` is one of: `desktop`, `mobile`, `tablet`, `bot`, `unknown`

**Dashboard reads these 13 paths:**
`timestamp`, `ip_hash`, `referer`, `analysis.is_bot`, `analysis.verdict`, `analysis.reasons`, `ip.country_code`, `ip.is_datacenter`, `ip.is_tor`, `ip.is_threat`, `ua.device_type`, `ua.is_known_bot`, `ua.is_email_scanner`

**How to wire data:**
```php
// Option 1: Inline from PHP
<script>renderDashboard(<?= json_encode($clicks, JSON_UNESCAPED_SLASHES) ?>);</script>

// Option 2: Fetch
fetch('clicks.json').then(r => r.json()).then(renderDashboard);
```

**What to change in login.html:**
- Remove demo script that intercepts submit
- Unhide `#login-error` box on failed attempt
- Add CSRF token and rate limiting

**CDN to local (for offline use):**
1. Run Tailwind CLI over both HTML files → single CSS file
2. Download `chart.umd.min.js` and serve locally
3. Self-host Manrope font or use system font stack

### stubs/admin.php.stub

This stub wraps the pre-built UI from `admin-demo/index.html`:

```php
<?php
// ========================================
// GuardBot Download Kit - Admin Panel
// Based on: admin-demo/HANDOVER.md
// ========================================

require_once __DIR__ . '/config.php';
session_start();

// CSRF token
if (!isset($_SESSION['csrf_token'])) {
    $_SESSION['csrf_token'] = bin2hex(random_bytes(32));
}

// Rate limiting for login
$rate_file = __DIR__ . '/data/.login_attempts';
function check_rate_limit() {
    global $rate_file;
    $attempts = @json_decode(@file_get_contents($rate_file), true) ?: [];
    $ip = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
    $now = time();
    // Clean old attempts (older than 15 minutes)
    $attempts = array_filter($attempts, fn($t) => $now - $t < 900);
    $ip_attempts = array_filter($attempts, fn($t, $k) => str_starts_with($k, $ip), ARRAY_FILTER_USE_BOTH);
    return count($ip_attempts) < 5; // Max 5 attempts per 15 min
}
function record_attempt() {
    global $rate_file;
    $attempts = @json_decode(@file_get_contents($rate_file), true) ?: [];
    $ip = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
    $attempts[$ip . '_' . time()] = time();
    @file_put_contents($rate_file, json_encode($attempts));
}

// Authentication
if (!isset($_SESSION['logged_in'])) {
    $error = '';
    if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['login'])) {
        // CSRF check
        if (!hash_equals($_SESSION['csrf_token'], $_POST['csrf_token'] ?? '')) {
            $error = 'Invalid request';
        } elseif (!check_rate_limit()) {
            $error = 'Too many attempts. Try again in 15 minutes.';
        } else {
            $user = $_POST['username'] ?? '';
            $pass = $_POST['password'] ?? '';
            
            if (isset($auth_users[$user]) && password_verify($pass, $auth_users[$user])) {
                $_SESSION['logged_in'] = true;
                $_SESSION['username'] = $user;
                header('Location: admin.php');
                exit;
            } else {
                record_attempt();
                $error = 'Invalid username or password';
            }
        }
    }
    
    // Show login form (from admin-demo/login.html, with PHP wiring)
    include __DIR__ . '/templates/login.php';
    exit;
}

// Logout
if (isset($_GET['logout'])) {
    session_destroy();
    header('Location: admin.php');
    exit;
}

// Load clicks data
$clicks = [];
if (file_exists($clicks_file)) {
    $content = @file_get_contents($clicks_file);
    $clicks = json_decode($content, true) ?: [];
}

// Serve dashboard (from admin-demo/index.html, with data injected)
?>
<!-- Dashboard HTML from admin-demo/index.html -->
<!-- Delete the <script id="demo-data"> block -->
<!-- Replace renderDashboard call with: -->
<script>renderDashboard(<?= json_encode($clicks, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE) ?>);</script>
```

### stubs/data-htaccess.stub

```apache
# Deny all access to data directory
Order deny,allow
Deny from all
```

### README.txt

```
GuardBot Redirect Kit
=====================

Setup Instructions:
1. Upload all files to your hosting (cPanel, VPS, etc.)
2. Edit config.php to change admin password (see below)
3. Access index.php - visitors land here
4. Access admin.php - your analytics dashboard

Changing Admin Password:
1. Generate hash: php -r "echo password_hash('yourpassword', PASSWORD_BCRYPT);"
2. Or use: https://tinyfilemanager.github.io/docs/pwd.html
3. Edit config.php and replace the hash in $auth_users

Default Login:
- Username: admin
- Password: admin123 (CHANGE THIS!)

Requirements:
- PHP 7.4+ with cURL extension
- Any web server (Apache, Nginx, LiteSpeed)

Generated: {{ generated_at }}
Token ID: {{ token_id }}
```

## Security Considerations

### Token Security
- Tokens are 32-char hex (128-bit entropy)
- Tokens can be revoked from dashboard
- Rate limiting per token (100 req/min default)
- Tokens tied to user account

### API Security
- HTTPS only
- IP logging (hashed for privacy)
- No sensitive data in responses
- Timeout handling in PHP

### Abuse Prevention
- Max downloads per user (configurable)
- Max active tokens per user
- Token expiration (optional, e.g., 90 days)
- Usage monitoring in dashboard

## Dashboard Integration

Download clicks should appear in:
- User dashboard charts (UNION with visits + short_link_clicks)
- Countries list
- Map visualization

Add to `modules/dashboard/module.go` UNION queries:
```sql
UNION ALL
SELECT ... FROM download_clicks WHERE user_id = $1
```

## Decisions (Finalized)

| Question | Decision |
|----------|----------|
| Token expiration | Never expire, user can revoke manually |
| Max downloads per user | 10 active downloads (can delete old ones) |
| Domain selection | Same wildcard domains as redirect links |
| API fallback | Pass-through to human URL (fail-open) |
| Analytics sync | Direct DB insert (Deploy VPS has PostgreSQL access) |
| Re-download | Yes, same token/endpoint, regenerate PHP file |
| Menu location | New "Download Kit" item under Tools section |
| Subdomain pool | Same `pkg/namegen` pool as short links |
| Bot detection | Full checks (UA, IP reputation, headers, rate limiting) |
| GeoIP | MaxMind DB on Deploy VPS for country/coordinates |

## Implementation Order

### Phase 1: Stubs & Module Setup
1. Create stub files (`index.php.stub`, `endpoint.php.stub`, `htaccess.stub`, `readme.txt.stub`)
2. Stub service (`stub_service.go`) with `{{ placeholder }}` processing
3. Database migration (download_tokens, download_clicks)
4. Module scaffold (routes, handlers, templates)
5. Token CRUD service
6. Domain selection logic (pick from wildcard domains)

### Phase 2: API Endpoint Deployment
7. Endpoint deployment service (SSH to Deploy VPS)
8. Process `endpoint.php.stub` → deploy via SCP
9. Test bot detection logic in deployed endpoint
10. Verify analytics recording (DB insert)

### Phase 3: Download Generation
11. Process `index.php.stub` with customizer HTML injection
12. Process `htaccess.stub` and `readme.txt.stub`
13. Zip file packaging (all processed stubs)
14. Download handler (serve zip)

### Phase 4: UI & Integration
15. List page (user's downloads)
16. New download form (with customizer)
17. Show page (stats, re-download, revoke)
18. Dashboard integration (UNION queries for analytics)

## Endpoint Deployment on Deploy VPS

When a download is created, we deploy a PHP handler on Deploy VPS:

### Directory Structure
```
/var/www/sites/{baseDomain}/{subdomain}/{path}/
└── index.php    # API endpoint handler (from endpoint.php.stub)
```

Example: `/var/www/sites/kemore.sbs/canyon/x7k/index.php`

### Nginx Configuration
The wildcard domain nginx config already routes `*.kemore.sbs` to the sites directory.
No additional nginx config needed per-download.

### Endpoint Handler
Uses `stubs/endpoint.php.stub` (see Stub Files section above).

### Deployment via SSH
Reuse existing `pkg/sshexec/` package to:
1. Create directory on Deploy VPS
2. Process `endpoint.php.stub` with token data
3. Write PHP handler file via SCP
4. Set permissions (644)

### Cleanup on Delete
When user deletes a download:
1. Remove directory from Deploy VPS
2. Delete database records

## Success Criteria

- [ ] User can create download with customized loading page
- [ ] Private API endpoint deploys to Deploy VPS with antibot protection
- [ ] Downloaded PHP works on standard cPanel hosting (PHP 7.4+)
- [ ] Bot detection works via private API endpoint
- [ ] Bots get immediate redirect (no loading page)
- [ ] Humans see customized loading page
- [ ] Analytics appear in user dashboard
- [ ] Tokens can be paused/revoked from dashboard
- [ ] Re-download works without changing endpoint URL
- [ ] Endpoint cleanup on delete

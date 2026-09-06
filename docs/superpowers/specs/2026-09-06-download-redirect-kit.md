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
User's cPanel/VPS (index.php)
      │
      │ 1. Capture visitor IP, UA, headers
      │ 2. Call private API endpoint
      │
      ▼
xyz123.kemore.sbs/api/check    ← Private subdomain (Deploy VPS)
      │
      │ Antibot proxy (botection) protects this endpoint
      │ - Blocks direct abuse/probing
      │ - Rate limiting
      │ - Unknown to bots (random subdomain)
      │
      ▼
┌─────────────────────────────────────┐
│  API Handler                        │
│  - Validates token                  │
│  - Analyzes forwarded visitor info  │
│  - IP reputation, UA analysis       │
│  - Returns {block, redirect_url}    │
│  - Records analytics                │
└─────────────────────────────────────┘
      │
      ▼
User's PHP receives response
      │
      │ 3. If bot: redirect to bot_url (or show block page)
      │ 4. If human: show loading page → redirect to original_url
      │
      ▼
Final destination

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
│   └── download_token.go        # Token model, click model
├── services/
│   └── download_service.go      # CRUD, PHP generation, analytics
├── handlers/
│   └── handler.go               # HTTP handlers
└── templates/
    ├── list.html                # List user's downloads
    ├── new.html                 # Create new download (with customizer)
    └── show.html                # View single download stats
```

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

## PHP Template (Downloaded by User)

The generated `index.php` structure:

```php
<?php
// ========================================
// GuardBot Redirect Kit v1.0
// Generated: 2026-09-06 12:00:00
// Endpoint: https://canyon.kemore.sbs/x7k
// ========================================

$config = [
    // Private API endpoint (antibot-protected, unique to this download)
    'api_url' => 'https://canyon.kemore.sbs/x7k',
    'token' => 'abc123def456...',
    
    // Fallback URLs (used if API fails)
    'fallback_url' => 'https://human-destination.com',
    
    // Settings
    'delay' => 3,
    'timeout' => 5,
];

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

// Call private API endpoint
$redirect_url = $config['fallback_url'];
$is_bot = false;
$show_loader = true;

$ch = curl_init($config['api_url']);
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_POSTFIELDS => json_encode([
        'token' => $config['token'],
        'visitor' => $visitor,
    ]),
    CURLOPT_HTTPHEADER => ['Content-Type: application/json'],
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_TIMEOUT => $config['timeout'],
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_SSL_VERIFYPEER => true,
]);

$response = curl_exec($ch);
$http_code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
$curl_error = curl_error($ch);
curl_close($ch);

if ($http_code === 200 && $response) {
    $data = json_decode($response, true);
    if (isset($data['redirect'])) {
        $redirect_url = $data['redirect'];
    }
    $is_bot = $data['block'] ?? false;
}
// On API failure, fallback to human URL (pass-through)

// Bot detected: immediate server-side redirect (no loading page)
if ($is_bot) {
    header('Location: ' . $redirect_url);
    exit;
}

// Human: show customized loading page, then redirect
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Loading...</title>
    <!-- CUSTOMIZED STYLES HERE -->
</head>
<body>
    <!-- CUSTOMIZED LOADING PAGE CONTENT HERE -->
    
    <script>
    setTimeout(function() {
        window.location.href = <?= json_encode($redirect_url) ?>;
    }, <?= $config['delay'] * 1000 ?>);
    </script>
</body>
</html>
<?php exit; ?>

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
├── index.php           # Main redirect script
├── .htaccess           # Apache rewrite rules (optional)
└── README.txt          # Setup instructions
```

### README.txt

```
GuardBot Redirect Kit
=====================

Setup Instructions:
1. Upload all files to your hosting (cPanel, VPS, etc.)
2. Access index.php in your browser
3. That's it! The kit is ready to use.

Requirements:
- PHP 7.4+ with cURL extension
- Any web server (Apache, Nginx, LiteSpeed)

Support:
- Dashboard: https://guardbot.sbs/user/download
- Token can be paused/revoked from dashboard

Generated: 2026-09-06 12:00:00
Token ID: abc123
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

### Phase 1: Database & Module Setup
1. Database migration (download_tokens, download_clicks)
2. Module scaffold (routes, handlers, templates)
3. Token CRUD service
4. Domain selection logic (pick from wildcard domains)

### Phase 2: API Endpoint Deployment
5. Endpoint deployment service (SSH to Deploy VPS)
6. Generate nginx config + PHP handler on Deploy VPS
7. Bot detection logic in endpoint handler
8. Analytics recording (DB insert or callback)

### Phase 3: Download Generation
9. PHP template generation with customizer
10. Zip file packaging
11. Download handler

### Phase 4: UI & Integration
12. List page (user's downloads)
13. New download form (with customizer)
14. Show page (stats, re-download, revoke)
15. Dashboard integration (UNION queries for analytics)

## Endpoint Deployment on Deploy VPS

When a download is created, we deploy a PHP handler on Deploy VPS:

### Directory Structure
```
/var/www/sites/{baseDomain}/{subdomain}/{path}/
└── index.php    # API endpoint handler
```

Example: `/var/www/sites/kemore.sbs/canyon/x7k/index.php`

### Nginx Configuration
The wildcard domain nginx config already routes `*.kemore.sbs` to the sites directory.
No additional nginx config needed per-download.

### Endpoint Handler Template
```php
<?php
// Auto-generated endpoint for download token: {token_id}
// Do not modify - managed by GuardBot

define('TOKEN', '{token}');
define('ORIGINAL_URL', '{original_url}');
define('BOT_URL', '{bot_url}');
define('DB_HOST', '{db_host}');
define('DB_NAME', '{db_name}');
define('DB_USER', '{db_user}');
define('DB_PASS', '{db_pass}');

header('Content-Type: application/json');
header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: POST');
header('Access-Control-Allow-Headers: Content-Type');

if ($_SERVER['REQUEST_METHOD'] === 'OPTIONS') {
    exit(0);
}

if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    http_response_code(405);
    echo json_encode(['error' => 'method_not_allowed']);
    exit;
}

$input = json_decode(file_get_contents('php://input'), true);
$token = $input['token'] ?? '';
$visitor = $input['visitor'] ?? [];

// Verify token
if ($token !== TOKEN) {
    http_response_code(401);
    echo json_encode(['error' => 'invalid_token']);
    exit;
}

// Bot detection
$is_bot = false;
$reason = '';

// Check User-Agent
$ua = strtolower($visitor['user_agent'] ?? '');
$bot_signatures = ['bot', 'crawler', 'spider', 'curl', 'wget', 'python', 'headless', 'phantom'];
foreach ($bot_signatures as $sig) {
    if (strpos($ua, $sig) !== false) {
        $is_bot = true;
        $reason = 'bot_ua';
        break;
    }
}

// Check for missing headers (bots often skip these)
if (!$is_bot && empty($visitor['accept_language'])) {
    $is_bot = true;
    $reason = 'missing_headers';
}

// TODO: Add IP reputation check, rate limiting

// Record analytics
try {
    $pdo = new PDO("pgsql:host=".DB_HOST.";dbname=".DB_NAME, DB_USER, DB_PASS);
    $stmt = $pdo->prepare("INSERT INTO download_clicks 
        (id, token_id, user_id, visitor_ip_hash, country, device, user_agent, is_bot, bot_reason, created_at)
        VALUES (gen_random_uuid()::text, :token_id, :user_id, :ip, :country, :device, :ua, :is_bot, :reason, NOW())");
    $stmt->execute([
        'token_id' => '{token_id}',
        'user_id' => '{user_id}',
        'ip' => hash('sha256', $visitor['ip'] ?? ''),
        'country' => '', // TODO: GeoIP lookup
        'device' => detect_device($visitor['user_agent'] ?? ''),
        'ua' => substr($visitor['user_agent'] ?? '', 0, 500),
        'is_bot' => $is_bot ? 't' : 'f',
        'reason' => $reason,
    ]);
    
    // Update counters
    $counter = $is_bot ? 'bot_count' : 'human_count';
    $pdo->exec("UPDATE download_tokens SET click_count = click_count + 1, {$counter} = {$counter} + 1, last_used_at = NOW() WHERE id = '{token_id}'");
} catch (Exception $e) {
    // Log error but don't fail the request
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

### Deployment via SSH
Reuse existing `pkg/sshexec/` package to:
1. Create directory on Deploy VPS
2. Write PHP handler file
3. Set permissions (644)

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

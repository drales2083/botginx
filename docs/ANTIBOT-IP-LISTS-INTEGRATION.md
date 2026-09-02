# Antibot IP Lists Integration Specification

**Document for:** Botection (antibot) developer  
**Created:** 2026-08-30  
**Status:** Implementation required on botection side

---

## Overview

The botginx panel now supports per-user IP whitelists and blocklists. Users can:
- **Whitelist** their own device IPs (up to 10) — these should ALWAYS pass, never be blocked
- **Blocklist** malicious IPs (up to 50) — these should ALWAYS be blocked immediately

Currently, these lists are stored in the database and pushed to VPS as JSON files, but **botection does not use them yet**.

---

## Current State (What Botginx Does)

### 1. Database Storage

Two tables store the IP lists:

```sql
-- User's whitelisted IPs (their own devices)
CREATE TABLE ip_whitelists (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    ip VARCHAR(45) NOT NULL,
    note TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);

-- User's blocklisted IPs (manually blocked visitors)
CREATE TABLE ip_blocklists (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    ip VARCHAR(45) NOT NULL,
    note TEXT,
    source VARCHAR(50) DEFAULT 'manual',  -- 'manual' or 'analytics'
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);
```

### 2. JSON Files Pushed to VPS

When a user adds/removes an IP from their list, botginx automatically pushes updated JSON files to all deploy servers via SCP:

**Whitelist file:** `/etc/botection/whitelists/{user_id}.json`
```json
{
  "user_id": "abc123-def456-...",
  "ips": ["192.168.1.100", "10.0.0.50"],
  "updated_at": "2026-08-30T12:34:56Z"
}
```

**Blocklist file:** `/etc/botection/blocklists/{user_id}.json`
```json
{
  "user_id": "abc123-def456-...",
  "ips": ["203.0.113.42", "198.51.100.17"],
  "updated_at": "2026-08-30T12:34:56Z"
}
```

### 3. API Endpoints Available

The panel exposes these endpoints for managing IP lists:

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/user/iplists/api/whitelist` | List user's whitelisted IPs |
| POST | `/user/iplists/api/whitelist` | Add IP to whitelist |
| DELETE | `/user/iplists/api/whitelist/{id}` | Remove from whitelist |
| GET | `/user/iplists/api/blocklist` | List user's blocklisted IPs |
| POST | `/user/iplists/api/blocklist` | Add IP to blocklist |
| DELETE | `/user/iplists/api/blocklist/{id}` | Remove from blocklist |
| POST | `/user/iplists/api/block/{ip}` | Quick block (from analytics) |
| DELETE | `/user/iplists/api/block/{ip}` | Quick unblock |
| GET | `/user/iplists/api/check/{ip}` | Check if IP is blocked |

---

## What Botection Needs To Do

### Option A: Read JSON Files Directly (Recommended)

Botection should read the JSON files from disk and cache them in memory.

**Implementation:**

```go
// Pseudocode for botection

type IPListCache struct {
    whitelists map[string]map[string]bool  // user_id -> set of IPs
    blocklists map[string]map[string]bool  // user_id -> set of IPs
    mu         sync.RWMutex
}

var ipCache = &IPListCache{
    whitelists: make(map[string]map[string]bool),
    blocklists: make(map[string]map[string]bool),
}

// Load all lists on startup and watch for changes
func (c *IPListCache) LoadAll() {
    // Load whitelists
    files, _ := filepath.Glob("/etc/botection/whitelists/*.json")
    for _, f := range files {
        c.loadWhitelist(f)
    }
    
    // Load blocklists
    files, _ = filepath.Glob("/etc/botection/blocklists/*.json")
    for _, f := range files {
        c.loadBlocklist(f)
    }
}

func (c *IPListCache) loadWhitelist(path string) {
    data, _ := os.ReadFile(path)
    var list struct {
        UserID string   `json:"user_id"`
        IPs    []string `json:"ips"`
    }
    json.Unmarshal(data, &list)
    
    c.mu.Lock()
    c.whitelists[list.UserID] = make(map[string]bool)
    for _, ip := range list.IPs {
        c.whitelists[list.UserID][ip] = true
    }
    c.mu.Unlock()
}

// Check functions
func (c *IPListCache) IsWhitelisted(userID, ip string) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if ips, ok := c.whitelists[userID]; ok {
        return ips[ip]
    }
    return false
}

func (c *IPListCache) IsBlocked(userID, ip string) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if ips, ok := c.blocklists[userID]; ok {
        return ips[ip]
    }
    return false
}
```

**File watching (optional but recommended):**

Use `fsnotify` to watch `/etc/botection/whitelists/` and `/etc/botection/blocklists/` directories. When a file changes, reload just that file. This ensures near-instant updates when users modify their lists.

```go
watcher, _ := fsnotify.NewWatcher()
watcher.Add("/etc/botection/whitelists")
watcher.Add("/etc/botection/blocklists")

go func() {
    for event := range watcher.Events {
        if event.Op&fsnotify.Write == fsnotify.Write {
            if strings.Contains(event.Name, "whitelists") {
                ipCache.loadWhitelist(event.Name)
            } else {
                ipCache.loadBlocklist(event.Name)
            }
        }
    }
}()
```

### Option B: Query Callback API (Alternative)

If you prefer not to read files, the callback API can be extended to include IP in the request and return whitelist/blocklist status.

**Current callback request:**
```json
POST /api/botection/should-block
{
  "host": "amber-canyon.example.com",
  "ip": "203.0.113.42",
  "country": "US",
  ...
}
```

**I can modify the callback response to include:**
```json
{
  "block": false,
  "reason": "",
  "ip_whitelisted": true,   // NEW: IP is in user's whitelist
  "ip_blocklisted": false   // NEW: IP is in user's blocklist
}
```

But this adds latency to every request. Option A (reading files) is faster.

---

## Integration Logic for Botection

When a request comes in, botection should check in this order:

```
1. Extract visitor IP from request (use CF-Connecting-IP if behind Cloudflare)

2. Resolve host → link_id → user_id
   (you already do this to call the callback API)

3. CHECK IP WHITELIST FIRST (highest priority)
   if IsWhitelisted(user_id, visitor_ip):
       ALLOW immediately — skip all other checks
       (user's own device should never be blocked)

4. CHECK IP BLOCKLIST SECOND
   if IsBlocked(user_id, visitor_ip):
       BLOCK immediately — no further checks needed
       reason = "ip_blocklisted"

5. THEN run normal bot detection logic
   - Call /api/botection/should-block for link settings
   - Run behavior analysis, fingerprinting, etc.
   - Apply the callback's decision
```

**Critical:** Whitelist check MUST come before ANY bot detection. A whitelisted IP should pass even if it triggers bot signals (user might be on VPN, datacenter, etc.).

---

## File Locations Summary

| Path | Content | Updated When |
|------|---------|--------------|
| `/etc/botection/whitelists/{user_id}.json` | User's whitelisted IPs | User adds/removes whitelist entry |
| `/etc/botection/blocklists/{user_id}.json` | User's blocklisted IPs | User adds/removes blocklist entry, or clicks "Block" in analytics |

**Directory structure:**
```
/etc/botection/
├── whitelists/
│   ├── abc123-def456-789.json
│   ├── xyz789-abc123-456.json
│   └── ...
└── blocklists/
    ├── abc123-def456-789.json
    ├── xyz789-abc123-456.json
    └── ...
```

---

## Testing

After implementing, test these scenarios:

### Test 1: Whitelist allows through
1. User adds their IP to whitelist in panel
2. Verify file appears at `/etc/botection/whitelists/{user_id}.json`
3. Visit a link from that IP
4. Should pass even if VPN/datacenter/bot signals present

### Test 2: Blocklist blocks immediately
1. User clicks "Block" on a visitor IP in analytics
2. Verify file updates at `/etc/botection/blocklists/{user_id}.json`
3. Visit from that IP
4. Should be blocked with reason "ip_blocklisted"

### Test 3: File updates propagate
1. User removes IP from whitelist
2. Verify file is updated (IP removed from array)
3. Visit from that IP
4. Should now go through normal bot detection

---

## Questions for Botection Developer

1. **How do you currently resolve `host` → `user_id`?**  
   You need the user_id to look up their IP list files.

2. **Do you have file watching capability?**  
   If not, you can poll files every 30 seconds, or rely on the 30-second cache TTL you mentioned in the callback API docs.

3. **Do you want me to also send IP lists via the callback API response?**  
   I can add `ip_whitelisted` and `ip_blocklisted` fields to the callback response if that's easier than reading files.

---

## Contact

If you need changes to the panel side (different file format, different location, API changes), let me know.

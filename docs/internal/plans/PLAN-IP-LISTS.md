# IP Whitelist & Blocklist Feature Plan

## Overview

Two separate IP management features:

| Feature | Purpose | Sync Method | Location |
|---------|---------|-------------|----------|
| **Global Whitelist** | User's own device IPs for testing | Supabase (central) | User Settings |
| **Per-Link Whitelist** | Visitor IPs to allow | SCP to VPS files | IP Lists module |
| **Per-Link Blocklist** | Visitor IPs to block | SCP to VPS files | IP Lists module |

---

## Part 1: Global Whitelist (User Settings)

User's own device IPs that bypass antibot when testing their links. Broadcasts to ALL antibot servers via Supabase central server.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          PANEL (guardbot.sbs)                          │
│                                                                         │
│  User Settings → Add IP → Save to PostgreSQL → Sync to Supabase        │
│                                                                         │
└────────────────────────────────────────────│────────────────────────────┘
                                             │
                                             ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      SUPABASE (Central Server)                         │
│                                                                         │
│  Table: global_whitelisted_ips                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ ip          │ user_id  │ source_host      │ updated_at         │   │
│  │ 203.45.67.89│ user123  │ guardbot.sbs     │ 2026-08-30 12:00   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└────────────────────────────────────────────│────────────────────────────┘
                                             │
                         ┌───────────────────┼───────────────────┐
                         ▼                   ▼                   ▼
              ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
              │ Antibot Server 1 │ │ Antibot Server 2 │ │ Antibot Server N │
              │ (kemore.sbs)     │ │ (other VPS)      │ │ (future VPS)     │
              │                  │ │                  │ │                  │
              │ Reads Supabase   │ │ Reads Supabase   │ │ Reads Supabase   │
              │ on each request  │ │ on each request  │ │ on each request  │
              └──────────────────┘ └──────────────────┘ └──────────────────┘
```

### Database Schema

```sql
-- Panel PostgreSQL (local copy)
CREATE TABLE IF NOT EXISTS user_global_whitelists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_user_global_whitelists_user ON user_global_whitelists(user_id);
```

```sql
-- Supabase (central, shared across all servers)
-- Table: global_whitelisted_ips
-- Columns: ip, user_id, source_host, updated_at
```

### Sync to Supabase

```go
// Same pattern as custom-mail-client
func syncWhitelistToSupabase(ip, userID string) error {
    supabaseURL := os.Getenv("SUPABASE_URL")
    supabaseKey := os.Getenv("SUPABASE_KEY")
    
    body := map[string]interface{}{
        "ip":          ip,
        "user_id":     userID,
        "source_host": "guardbot.sbs",
        "updated_at":  time.Now().Format(time.RFC3339),
    }
    
    req, _ := http.NewRequest("POST", supabaseURL+"/rest/v1/global_whitelisted_ips", ...)
    req.Header.Set("apikey", supabaseKey)
    req.Header.Set("Prefer", "resolution=merge-duplicates")
    
    return http.DefaultClient.Do(req)
}
```

### UI in User Settings (`/user/settings`)

```
┌─────────────────────────────────────────────────────────────────┐
│ Settings                                                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ My Whitelisted IPs                                              │
│ ─────────────────                                               │
│ These IPs bypass bot protection when you test your links.      │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ IP Address        │ Added        │ Actions                 │ │
│ ├───────────────────┼──────────────┼─────────────────────────┤ │
│ │ 203.45.67.89      │ Today        │ [Remove]                │ │
│ │ 192.168.1.100     │ 3 days ago   │ [Remove]                │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ┌─────────────────────────────────┐ ┌─────────────────┐        │
│ │ Enter IP address...             │ │ Add to Whitelist│        │
│ └─────────────────────────────────┘ └─────────────────┘        │
│                                                                 │
│ [+ Add my current IP: 203.45.67.89]                            │
│                                                                 │
│ 2 of 5 IPs used                                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/user/settings/api/global-whitelist` | Get user's global whitelist |
| POST | `/user/settings/api/global-whitelist` | Add IP (syncs to Supabase) |
| DELETE | `/user/settings/api/global-whitelist/{id}` | Remove IP (removes from Supabase) |

### Validation Rules

- Maximum 5 IPs per user
- IPv4 only
- No duplicates
- User can remove any IP to add a new one

### Antibot Integration

Antibot servers read from Supabase to check if IP is whitelisted:

```go
// In antibot/botection
func isGlobalWhitelisted(ip string) bool {
    // Query Supabase: SELECT 1 FROM global_whitelisted_ips WHERE ip = ?
    // Cache result for 5 minutes (Redis)
    return cached || querySupabase(ip)
}

func handleRequest(r *Request) Decision {
    // Check global whitelist FIRST (user's own testing IPs)
    if isGlobalWhitelisted(r.IP) {
        return Allow("global_whitelist")
    }
    
    // Continue normal bot detection...
}
```

---

## Part 2: Per-Link IP Lists (IP Lists Module)

Whitelists and blocklists for **visitor** IPs. Pushed to VPS files via SCP.

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          PANEL (guardbot.sbs)                          │
│                                                                         │
│  IP Lists Module → Add/Remove IP → Save to PostgreSQL → Push via SCP   │
│                                                                         │
└────────────────────────────────────────────│────────────────────────────┘
                                             │ SCP Push
                                             ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      ANTIBOT VPS (kemore.sbs)                          │
│                                                                         │
│  /etc/botection/whitelists/{userId}.json  ← visitor IPs to allow       │
│  /etc/botection/blocklists/{userId}.json  ← visitor IPs to block       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Database Schema

```sql
-- Visitor IP Whitelists (bypass antibot for these visitors)
CREATE TABLE IF NOT EXISTS ip_whitelists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    note TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_ip_whitelists_user ON ip_whitelists(user_id);

-- Visitor IP Blocklists (always block these visitors)
CREATE TABLE IF NOT EXISTS ip_blocklists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    note TEXT,
    source TEXT DEFAULT 'manual',  -- 'manual' or 'analytics'
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_ip_blocklists_user ON ip_blocklists(user_id);
```

### Module Structure

```
modules/iplists/
├── module.go
├── migrations/
│   └── 001_create_ip_lists.sql
├── models/
│   └── models.go
├── services/
│   └── service.go
├── handlers/
│   └── handler.go
└── templates/
    ├── whitelist.html
    └── blocklist.html
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/user/iplists/whitelist` | Page: whitelist management |
| GET | `/user/iplists/blocklist` | Page: blocklist management |
| POST | `/user/iplists/api/whitelist` | Add visitor IP to whitelist |
| DELETE | `/user/iplists/api/whitelist/{id}` | Remove from whitelist |
| POST | `/user/iplists/api/blocklist` | Add visitor IP to blocklist |
| DELETE | `/user/iplists/api/blocklist/{id}` | Remove from blocklist |

### Analytics Quick Actions

| Method | Path | Description |
|--------|------|-------------|
| POST | `/user/analytics/api/visitor/{ip}/block` | Block visitor IP |
| DELETE | `/user/analytics/api/visitor/{ip}/block` | Unblock visitor IP |

### Sync to VPS (SCP)

```go
// Push when user adds/removes visitor IP
func (p *Pusher) PushWhitelist(userID string, ips []string) error {
    data := map[string]interface{}{
        "user_id": userID,
        "ips":     ips,
        "updated_at": time.Now().Format(time.RFC3339),
    }
    content, _ := json.Marshal(data)
    remotePath := fmt.Sprintf("/etc/botection/whitelists/%s.json", userID)
    return p.SCPWrite(server, remotePath, content)
}
```

### Validation Rules

| List | Max IPs | Notes |
|------|---------|-------|
| Per-Link Whitelist | 10 | Visitor IPs |
| Per-Link Blocklist | 50 | Visitor IPs |

---

## Feature Comparison

| Aspect | Global Whitelist | Per-Link Whitelist | Per-Link Blocklist |
|--------|------------------|--------------------|--------------------|
| **Purpose** | User's own testing IPs | Visitor IPs to allow | Visitor IPs to block |
| **Location** | User Settings | IP Lists module | IP Lists module |
| **Max IPs** | 5 | 10 | 50 |
| **Sync** | Supabase (central) | SCP to VPS files | SCP to VPS files |
| **Scope** | All antibot servers | Per-VPS | Per-VPS |

---

## Implementation Phases

### Phase 1: Global Whitelist
- [ ] Add `user_global_whitelists` table
- [ ] Configure Supabase connection (env vars)
- [ ] Add sync functions (add/remove to Supabase)
- [ ] Add UI in user settings page
- [ ] "Add my current IP" button
- [ ] Update antibot to check Supabase global whitelist

### Phase 2: Per-Link Database & Models
- [ ] Create `modules/iplists/` directory
- [ ] Add migrations for `ip_whitelists` and `ip_blocklists`
- [ ] Define Go models

### Phase 3: Per-Link Service & API
- [ ] CRUD operations
- [ ] Validation logic
- [ ] SCP push to VPS

### Phase 4: Per-Link UI
- [ ] Whitelist page
- [ ] Blocklist page
- [ ] Add/remove forms

### Phase 5: Analytics Integration
- [ ] Block/unblock buttons on visitor rows
- [ ] Quick action API endpoints

### Phase 6: Server Migration Handling
- [ ] Admin "Sync IP Lists" button
- [ ] Auto-sync on new server deploy

---

## Testing Checklist

### Global Whitelist
- [ ] Add IP → appears in settings
- [ ] Add IP → synced to Supabase
- [ ] Visit link from whitelisted IP → no bot challenge
- [ ] Remove IP → removed from Supabase
- [ ] Add 6th IP → error (max 5)
- [ ] "Add my current IP" → works

### Per-Link Lists
- [ ] Add visitor IP to whitelist → syncs to VPS
- [ ] Add visitor IP to blocklist → syncs to VPS
- [ ] Block from analytics → appears in blocklist
- [ ] Unblock from analytics → removed

### Server Migration
- [ ] New VPS → admin syncs → per-link lists restored
- [ ] Global whitelist works immediately (Supabase)

---

## Environment Variables

```bash
# Supabase (for global whitelist)
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_KEY=eyJ...

# VPS SSH (for per-link lists)
DEPLOY_VPS_IP=194.147.217.45
DEPLOY_VPS_USER=root
DEPLOY_VPS_PASSWORD=xxx
```

# Server Auto-Provisioning Design

**Date:** 2026-11-20  
**Status:** Draft

## Intent

Automate the deployment of new VPS servers through the GuardBot panel. When an admin adds a new server, the system should automatically provision it with the base infrastructure (nginx, redis, PHP, acme-dns) and botection antibot proxy, streaming progress in real-time.

## Why

- **Eliminate manual SSH work** - No more copy-pasting scripts and watching terminals
- **Consistency** - Every server gets the exact same setup
- **Visibility** - See exactly what's happening, catch errors immediately
- **Speed** - Add servers in one click, not 20 minutes of manual work

## Goals

1. One-click provisioning from `/admin/servers/new`
2. Real-time progress streaming (SSE-based)
3. Clear error reporting with step identification
4. Secure credential handling (encrypted GitHub token)
5. Idempotent scripts (safe to re-run if something fails)

## Non-Goals

- Interactive terminal during provisioning (not needed)
- Cancellation mid-provision (complexity not worth it)
- Multi-server parallel provisioning (do one at a time)
- Custom script upload (use the standard scripts)

## Anti-Goals

- Exposing SSH credentials in logs or UI
- Storing GitHub token in plaintext
- Running provisioning without user confirmation

## Constraints

- Antibot repo is private - GitHub token already embedded in deploy.sh
- deploy-vps-setup.sh requires `ACME_DOMAIN` environment variable (only user input needed)
- Scripts take 5-10 minutes to complete
- Need to handle SSH connection failures gracefully

## Approaches Considered

### A: WebSocket Streaming
- Full bidirectional communication
- Rejected: Overkill for one-way progress streaming

### B: Server-Sent Events (SSE) - **Selected**
- Real-time one-way streaming over HTTP
- Simple to implement, good browser support
- Works with existing handler patterns

### C: Polling
- Background job + status polling
- Rejected: Not truly real-time, more API calls

## Design

### Database Schema

```sql
-- New table for provisioning jobs
CREATE TABLE IF NOT EXISTS server_provisions (
    id VARCHAR(36) PRIMARY KEY,
    server_id VARCHAR(36) REFERENCES servers(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed
    current_step INT DEFAULT 0,
    total_steps INT DEFAULT 5,
    step_name VARCHAR(100),
    output TEXT,
    error TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add provision_id to servers for tracking
ALTER TABLE servers ADD COLUMN IF NOT EXISTS provision_id VARCHAR(36);
```

### Provisioning Steps

| Step | Name | Script/Action |
|------|------|---------------|
| 1 | Connecting | Test SSH connection |
| 2 | Base Setup | Run deploy-vps-setup.sh (nginx, redis, PHP, acme-dns) |
| 3 | Clone Antibot | Clone private repo with GitHub token |
| 4 | Install Botection | Run packaging/deploy.sh |
| 5 | Verify Services | Check nginx, redis, botection are running |

### API Endpoints

```
POST /admin/servers/api/provision
  Body: { serverId, acmeDomain }
  Response: { provisionId }
  Starts provisioning job

GET /admin/servers/api/provision/{id}/stream
  Response: SSE stream
  Content-Type: text/event-stream
  Events: step, output, error, complete
```

### SSE Event Format

```
event: step
data: {"step": 2, "total": 5, "name": "Installing base packages..."}

event: output
data: {"line": "[1/5] Installing system packages..."}

event: error
data: {"step": 2, "message": "apt-get failed: unable to locate package"}

event: complete
data: {"success": true, "duration": "5m32s"}
```

### UI Flow

```
/admin/servers/new
┌─────────────────────────────────────────────────┐
│  Add New Server                                 │
│                                                 │
│  Name: [My Deploy VPS        ]                  │
│  IP:   [23.175.50.131        ]                  │
│  Port: [22                   ]                  │
│  User: [root                 ]                  │
│  Password: [••••••••••       ]                  │
│                                                 │
│  ACME Domain: [acme.pamach.xyz]                 │
│  (For wildcard SSL via DNS-01)                  │
│                                                 │
│  [ ] Provision automatically after adding       │
│                                                 │
│  [Add Server]  [Add & Provision]                │
└─────────────────────────────────────────────────┘

After clicking "Add & Provision":
┌─────────────────────────────────────────────────┐
│  Provisioning: My Deploy VPS                    │
│                                                 │
│  ✓ Step 1/5: Connected via SSH                  │
│  ✓ Step 2/5: Base setup complete                │
│  ⟳ Step 3/5: Installing botection...            │
│    Step 4/5: Configuring services               │
│    Step 5/5: Verifying installation             │
│                                                 │
│  ┌─────────────────────────────────────────┐    │
│  │ [2/9] Downloading binary...             │    │
│  │   drales2083/antibot v3.63.0            │    │
│  │   Checksum OK                           │    │
│  │ [3/9] Creating config...                │    │
│  │   Admin token generated -> .env.local   │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  [Cancel] (disabled during provision)           │
└─────────────────────────────────────────────────┘
```

### Environment Variables

Add to `.env`:
```
# Auto-provisioning
DEFAULT_ACME_DOMAIN=acme.pamach.xyz
```

**Note:** GitHub token, MaxMind key, Supabase keys, and Botginx panel URL are already hardcoded in the antibot deploy.sh with working defaults. We don't need to store or pass them - the script handles it.

### File Structure

```
modules/servers/
├── services/
│   ├── server_service.go      # Existing
│   └── provision_service.go   # NEW: Provisioning logic
├── handlers/
│   ├── handler.go             # Existing
│   └── provision_handler.go   # NEW: SSE streaming endpoint
├── models/
│   ├── server.go              # Existing
│   └── provision.go           # NEW: Provision model
├── migrations/
│   └── 002_add_provisions.sql # NEW
└── templates/
    ├── new.html               # Update: Add provision option
    └── provision.html         # NEW: Provision progress page
```

### Security

1. **GitHub Token**: Already in antibot deploy.sh - no need to store separately
2. **SSH Password**: Already stored with json:"-", never in API responses
3. **Provision Logs**: Sanitize any credentials before storing (grep out token lines)
4. **SSE Auth**: Require admin session for provision stream

### Error Handling

| Error | Recovery |
|-------|----------|
| SSH connection fails | Mark server as "failed", show error |
| Script step fails | Save output, mark provision as "failed", allow retry |
| GitHub auth fails | Clear error message about token |
| Service verification fails | Show which service, suggest manual check |

### Builder's Discretion

- Exact log line buffering strategy (line-by-line vs chunked)
- Toast notifications vs inline alerts
- Whether to auto-scroll the output panel
- Exact timeout values for SSH operations

## Implementation Order

1. Database migration for provisions table
2. Provision model and service
3. SSE streaming handler
4. Update new.html template with provision option
5. Create provision.html progress page
6. Add GitHub token to encrypted config
7. Test end-to-end on a real VPS

# Proxy Links Module — Port Plan

Complete specification for porting the `redirect-generator` / `proxy-links` module from `msf-tokens/custom-mail-client` to a new standalone application.

---

## Table of Contents

1. [What It Does](#1-what-it-does)
2. [Architecture Overview](#2-architecture-overview)
3. [Database Schema](#3-database-schema)
4. [API Endpoints](#4-api-endpoints)
5. [UI Components](#5-ui-components)
6. [Cloudflare Integration](#6-cloudflare-integration)
7. [Bot Protection Layers](#7-bot-protection-layers)
8. [Worker Code Generation](#8-worker-code-generation)
9. [Splash Page Renderer](#9-splash-page-renderer)
10. [Turnstile Integration](#10-turnstile-integration)
11. [Shield Domain System](#11-shield-domain-system)
12. [Deployment Flow](#12-deployment-flow)
13. [Dependencies](#13-dependencies)
14. [Implementation Checklist](#14-implementation-checklist)

---

## 1. What It Does

The **Proxy Links** module lets users create custom redirect links with:

1. **Splash pages** — animated loading screens before redirect (loader, text, background patterns)
2. **URL rotation** — randomly picks from multiple destination URLs
3. **Bot protection** — multi-layer defense to block scrapers/crawlers
4. **Cloudflare Turnstile** — optional human verification widget
5. **Server-side redirects** — destination URLs never exposed in HTML source
6. **Custom HTML mode** — serve arbitrary HTML instead of splash page

Each "campaign" deploys as a **Cloudflare Pages** project with a **Worker** that handles all logic. The URL format is:

```
https://{subdomain}.pages.dev/{path}/
```

Two campaign types:
- **REDIRECT** — shows splash page, then redirects to a random URL from the pool
- **HTML** — serves user-provided HTML content (landing pages, etc.)

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Your Application                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐          │
│  │   Dashboard UI  │───▶│   API Routes    │───▶│  CF Deploy      │          │
│  │  (React/Next)   │    │  /api/proxy-*   │    │  Service        │          │
│  └─────────────────┘    └─────────────────┘    └────────┬────────┘          │
│                                                          │                   │
│  ┌─────────────────┐    ┌─────────────────┐              │                   │
│  │ Splash Renderer │    │ Worker Template │◀─────────────┘                   │
│  │  (HTML+CSS+JS)  │    │   Generator     │                                  │
│  └─────────────────┘    └─────────────────┘                                  │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      │ Wrangler CLI
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                         Cloudflare Account (User's)                          │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐          │
│  │  Pages Project  │    │  Turnstile      │    │  (Optional)     │          │
│  │  {subdomain}    │    │  Widget         │    │  Shield Domain  │          │
│  │  _worker.js     │    │  sitekey+secret │    │  for bot checks │          │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘          │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      │ HTTPS
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                              Visitor Browser                                 │
│  1. Hits {subdomain}.pages.dev/{path}/                                       │
│  2. Worker serves splash HTML (with bot-mask, loader, optional Turnstile)    │
│  3. Client JS calls /_go endpoint after delay (or after Turnstile pass)      │
│  4. Worker responds with 302 redirect to random URL from pool                │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Database Schema

### RedirectCampaign (main table)

```sql
CREATE TABLE redirect_campaigns (
  id                    VARCHAR(25) PRIMARY KEY,  -- cuid
  user_id               VARCHAR(25) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  
  -- Identity
  subdomain             VARCHAR(63) NOT NULL,     -- CF Pages project name
  domain                VARCHAR(100) DEFAULT 'pages.dev',
  path                  VARCHAR(128) NOT NULL,    -- URL path segment
  type                  VARCHAR(20) DEFAULT 'REDIRECT', -- 'REDIRECT' | 'HTML'
  
  -- Content
  human_urls            JSONB DEFAULT '[]',       -- string[] of destination URLs
  html_content          TEXT,                     -- for type=HTML
  customization         JSONB,                    -- splash editor settings
  animation_duration_sec INT DEFAULT 3,           -- 3-20 seconds
  
  -- Turnstile
  turnstile_enabled     BOOLEAN DEFAULT FALSE,
  turnstile_widget_id   VARCHAR(100),
  turnstile_site_key    VARCHAR(100),
  turnstile_secret_key  TEXT,                     -- ENCRYPTED
  turnstile_settings    JSONB,                    -- {mode, theme, size, position...}
  
  -- CF Deploy State
  cf_worker_name        VARCHAR(100),
  cf_worker_url         VARCHAR(255),
  cf_route_id           VARCHAR(100),
  last_deployed_at      TIMESTAMP,
  deploy_status         VARCHAR(20) DEFAULT 'pending', -- 'pending'|'deployed'|'failed'
  deploy_error          TEXT,
  
  -- Bot Protection
  bot_protection        BOOLEAN DEFAULT FALSE,
  shield_domain_id      VARCHAR(25) REFERENCES shield_domains(id) ON DELETE SET NULL,
  
  -- Meta
  is_active             BOOLEAN DEFAULT TRUE,
  created_at            TIMESTAMP DEFAULT NOW(),
  updated_at            TIMESTAMP DEFAULT NOW(),
  
  UNIQUE(subdomain, domain, path)
);

CREATE INDEX idx_campaigns_user ON redirect_campaigns(user_id, is_active);
CREATE INDEX idx_campaigns_created ON redirect_campaigns(user_id, created_at DESC);
```

### ShieldDomain (optional bot protection proxy)

```sql
CREATE TABLE shield_domains (
  id                VARCHAR(25) PRIMARY KEY,
  user_id           VARCHAR(25) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  custom_domain_id  VARCHAR(25) NOT NULL REFERENCES custom_domains(id) ON DELETE CASCADE,
  subdomain         VARCHAR(63) NOT NULL,
  full_url          VARCHAR(255) NOT NULL,     -- e.g. "shield.userdomain.com"
  secret_key        VARCHAR(100),              -- X-Shield-Key header secret
  status            VARCHAR(20) DEFAULT 'pending',
  deploy_error      TEXT,
  is_healthy        BOOLEAN DEFAULT FALSE,
  last_health_check TIMESTAMP,
  health_fail_count INT DEFAULT 0,
  created_at        TIMESTAMP DEFAULT NOW(),
  updated_at        TIMESTAMP DEFAULT NOW(),
  
  UNIQUE(custom_domain_id, subdomain)
);
```

### User Cloudflare credentials

```sql
-- Add to users table
ALTER TABLE users ADD COLUMN cloudflare_email VARCHAR(255);
ALTER TABLE users ADD COLUMN cloudflare_api_token TEXT;      -- ENCRYPTED
ALTER TABLE users ADD COLUMN cloudflare_account_id VARCHAR(50);
```

---

## 4. API Endpoints

### `GET /api/proxy-links`

List user's campaigns.

**Response:**
```json
{
  "campaigns": [
    {
      "id": "clx...",
      "subdomain": "mylink",
      "domain": "pages.dev",
      "path": "go",
      "type": "REDIRECT",
      "humanUrls": ["https://example1.com", "https://example2.com"],
      "cfWorkerUrl": "https://mylink.pages.dev/go/",
      "turnstileEnabled": false,
      "botProtection": false,
      "deployStatus": "deployed",
      "createdAt": "2026-08-29T..."
    }
  ]
}
```

### `POST /api/proxy-links`

Create and deploy a new campaign.

**Request:**
```json
{
  "subdomain": "mylink",
  "path": "go",
  "type": "REDIRECT",
  "humanUrls": ["https://example1.com", "https://example2.com"],
  "turnstileEnabled": false,
  "botProtection": false,
  "shieldDomainId": null
}
```

**Flow:**
1. Validate inputs (subdomain regex, path regex, URL schemes)
2. Decrypt user's CF credentials
3. Create DB row with `deployStatus=pending`
4. If Turnstile enabled, create CF Turnstile widget via API
5. Generate Worker code (standalone or shielded)
6. Deploy to CF Pages via wrangler CLI
7. Poll deployment status until live
8. Update DB row with `deployStatus=deployed` + URL

**Response:**
```json
{
  "campaign": { ... },
  "url": "https://mylink.pages.dev/go/",
  "turnstileError": null
}
```

### `GET /api/proxy-links/[id]`

Get single campaign details.

### `PUT /api/proxy-links/[id]`

Update campaign settings and redeploy.

### `DELETE /api/proxy-links/[id]`

Delete campaign and CF Pages project.

### `GET /api/proxy-links/cf-zones`

Check if user has Cloudflare configured.

**Response:**
```json
{ "configured": true }
```

### `PUT /api/proxy-links/[id]/customization`

Update splash page customization and redeploy.

---

## 5. UI Components

### Feature Entry Point

`/src/features/proxy-links/index.tsx`

```
ProxyLinksFeature
├── ProxyLinksProvider (context for dialogs/state)
├── ProxyLinksTable (data table with columns)
├── ProxyLinksPrimaryButtons (Create button)
├── ProxyLinksCreateDrawer (multi-step form)
├── ProxyLinksDeleteDialog (confirmation)
├── ProxyLinksEditHtmlDialog (HTML editor for type=HTML)
└── ProxyLinksRowActions (copy URL, edit, delete per row)
```

### Table Columns

1. **URL** — clickable link to `{subdomain}.pages.dev/{path}/`
2. **Type** — REDIRECT or HTML badge
3. **Destinations** — count of URLs in pool
4. **Turnstile** — enabled/disabled badge
5. **Bot Protection** — enabled/disabled badge
6. **Status** — deployed/pending/failed with error tooltip
7. **Created** — relative timestamp
8. **Actions** — copy, customize, delete

### Create Drawer Steps

1. **Basic** — subdomain, path, type selection
2. **Destinations** — URL list (for REDIRECT) or HTML editor (for HTML)
3. **Protection** — Turnstile toggle, Bot Protection + Shield Domain select
4. **Review** — summary before deploy

### Customize Page

Full splash editor at `/dashboard/proxy-links/customize?id=xxx`:
- Live preview iframe
- Background (color, gradient, pattern)
- Loader animation picker (100+ variants)
- Text content (heading, subheading, font, size, color)
- Image/logo upload (base64 embedded)
- Layout options
- Turnstile position selector

---

## 6. Cloudflare Integration

### Required User Credentials

User must configure in settings:
- `cloudflareEmail` — account email
- `cloudflareApiToken` — API token with permissions:
  - `Account → Cloudflare Pages → Edit`
  - `Account → Turnstile → Edit` (if using Turnstile)
- `cloudflareAccountId` — account ID

### Deployment Method: Cloudflare Pages

NOT Workers directly — **Pages** gives free `*.pages.dev` subdomain without zone setup.

**Deploy flow:**
```
1. Create temp directory
2. Write _worker.js (generated Worker code)
3. Write _routes.json (route ALL traffic to Worker)
4. Write index.html (fallback)
5. Run: npx wrangler pages deploy {dir} --project-name={subdomain}
6. Poll CF API until deployment is "live"
7. Clean up temp directory
```

### Project Lifecycle

- **Create**: Delete existing project if any → Create fresh → Deploy
- **Update**: Delete project → Recreate → Deploy (ensures clean state)
- **Delete**: Best-effort project deletion via CF API

### Wrangler Environment Isolation

Critical: strip all `CLOUDFLARE_*`, `CF_*`, `WRANGLER_*` from env before running wrangler, then inject user's credentials:

```javascript
env.CLOUDFLARE_EMAIL = cfEmail;
env.CLOUDFLARE_API_KEY = cfApiKey;
env.CLOUDFLARE_ACCOUNT_ID = accountId;
```

---

## 7. Bot Protection Layers

When `botProtection=true`, the Worker runs a multi-layer defense pipeline:

### Layer A: Rate Limiting

```
Key: IP + short UA hash
Limit: 30 requests/minute per key
Storage: Redis or CF KV
```

### Layer B: Header Sanity

Checks for suspicious header patterns:
- Missing `Accept-Language`
- Missing `Sec-Fetch-*` headers
- UA claims Chrome but missing Chrome headers
- Impossible header combinations

### Layer C: IP/ASN Check

External API lookup (IPHub, IP-API, etc.):
- Datacenter/VPN/proxy detection
- Known bot ASN blocklists
- Threat score threshold

### Layer D: JS Challenge

Cookie-based proof that browser executed JavaScript:
```javascript
1. Worker checks for signed cookie
2. If missing, serve challenge HTML with inline JS
3. JS runs canvas/WebGL fingerprinting
4. If passes, sets signed cookie + reloads
5. Worker sees cookie → serves real content
```

### Layer E: Turnstile (Optional)

Cloudflare's invisible/managed CAPTCHA widget.

---

## 8. Worker Code Generation

Three Worker modes based on settings:

### Mode 1: Standalone (no bot protection)

```javascript
// _worker.js structure
var _W = {
  h: "...HTML...",           // splash page
  u: ["url1", "url2"],       // redirect URLs
  g: "/_go",                 // server redirect path
  ts: "...",                 // turnstile secret (optional)
  vp: "/__verify"            // turnstile verify path
};

export default {
  async fetch(request) {
    const url = new URL(request.url);
    
    // Health check
    if (url.pathname === "/.health") {
      return new Response("ok");
    }
    
    // Turnstile verification (if enabled)
    if (url.pathname === _W.vp && request.method === "POST") {
      // Verify token with CF API
      // Return {success: true/false}
    }
    
    // Server-side redirect
    if (url.pathname === _W.g) {
      const target = _W.u[Math.floor(Math.random() * _W.u.length)];
      return Response.redirect(target, 302);
    }
    
    // Serve splash page
    return new Response(_W.h, {
      headers: { "Content-Type": "text/html" }
    });
  }
};
```

### Mode 2: Protected (with Shield Domain)

Adds bot protection via backend API calls through a shield domain that hides the real backend:

```javascript
// Additional checks before serving content:
// 1. JS challenge cookie verification
// 2. CF threat score check (cf.threat_score)
// 3. Datacenter ASN detection
// 4. Header sanity validation
// 5. TLS/protocol fingerprinting
// 6. Backend signature verification via shield domain
```

### Mode 3: Full Protection (direct backend calls)

Same as Mode 2 but calls the platform backend directly (exposes backend domain).

### Code Obfuscation

Before deployment, Worker code runs through an obfuscator:
```javascript
import { obfuscateCode } from "@/lib/obfuscation";
const finalCode = await obfuscateCode(workerCode, "worker.js", { worker: true });
```

### Security: Leak Prevention

Before every deploy, scan Worker code for forbidden substrings:
- API tokens
- Database URLs
- Internal domains
- Secrets

```javascript
import { assertNoLeak } from "@/lib/cf-protected-worker";
assertNoLeak(workerCode); // throws WorkerCodeLeakError if found
```

---

## 9. Splash Page Renderer

The `renderSplashHtml()` function generates a self-contained HTML page.

### Input Interface

```typescript
interface SplashInput {
  redirectUrls: string[];
  customization?: Partial<RedirectCustomization>;
  animationDurationSec: number;
  turnstileEnabled: boolean;
  turnstileSiteKey: string | null;
  turnstileSettings?: Partial<TurnstileSettings>;
  verifyPath: string;
  previewMode?: boolean;
  botProtection?: boolean;
  serverSideRedirect?: boolean;
  goPath?: string;
}
```

### Customization Options

```typescript
interface RedirectCustomization {
  // Background
  bgColor: string;              // "#ffffff"
  gradientEnabled: boolean;
  bgColorSecondary: string;
  pattern: PatternKey;          // "none" | "topography" | "dots" | ...
  patternColor: string;
  
  // Loader
  loader: LoaderKey;            // "dots-bounce" | "ring" | "pulse" | ...
  loaderColorPrimary: string;
  loaderColorSecondary: string;
  
  // Text
  heading: string;              // "Please Wait"
  headingVisible: boolean;
  subheading: string;           // "We're redirecting you..."
  subheadingVisible: boolean;
  font: FontKey;                // "montserrat" | "poppins" | ...
  fontWeight: 400 | 600 | 700 | 800;
  textColor: string;
  textSize: number;             // px
  textShadow: boolean;
  
  // Image/Logo
  imageMode: "none" | "background" | "logo";
  imageDataUrl: string | null;  // base64 data URL
  imageSize: number;            // px
  imageOverlay: number;         // 0-100 opacity
  
  // Layout
  layout: LayoutKey;            // "image-loader-text" | "loader-text" | ...
  alignment: "center" | "left" | "right";
  vPos: number;                 // vertical offset px
  gap: number;                  // gap between elements px
  pageTitle: string;
}
```

### External Resources

- **Fonts**: Google Fonts CDN (`fonts.googleapis.com`)
- **Turnstile**: Cloudflare CDN (`challenges.cloudflare.com`)
- **Everything else**: Inline (patterns as data URLs, no external assets)

### Bot Mask Snippet

Injected into all splash pages:
```html
<style>
  html.bot-mask main, html.bot-mask .stack { display: none !important; }
  html.bot-mask .render-skel { display: flex !important; }
  .render-skel { /* loading skeleton */ }
</style>
<script>
  document.documentElement.classList.add("bot-mask");
  // Canvas/WebGL fingerprinting
  // If passes, remove bot-mask class after delay
  // Bots never see real content
</script>
```

### Client-Side Redirect Logic

```javascript
// Server-side redirect mode (URLs hidden):
function go() {
  location.href = "/_go";  // Worker handles actual redirect
}

// If Turnstile enabled:
window.onTurnstileSuccess = function(token) {
  fetch("/__verify", {
    method: "POST",
    body: JSON.stringify({ token })
  })
  .then(r => r.ok && setTimeout(go, duration * 1000));
};

// If no Turnstile:
setTimeout(go, duration * 1000);
```

---

## 10. Turnstile Integration

### Widget Creation

When user enables Turnstile for a campaign:

```javascript
const result = await createTurnstileWidget({
  cfEmail,
  cfApiKey,
  accountId,
  campaignName: `${subdomain}-${path}`,
  domains: [`${subdomain}.pages.dev`],
  mode: "managed"  // or "non-interactive" or "invisible"
});

// Returns: { widgetId, siteKey, secret } or { detail: "error" }
```

### Worker Verification

Worker handles `POST /__verify`:

```javascript
const verifyRes = await fetch(
  "https://challenges.cloudflare.com/turnstile/v0/siteverify",
  {
    method: "POST",
    body: new URLSearchParams({
      secret: TURNSTILE_SECRET,
      response: token,
      remoteip: request.headers.get("CF-Connecting-IP")
    })
  }
);
const result = await verifyRes.json();
return new Response(
  JSON.stringify({ success: result.success }),
  { status: result.success ? 200 : 403 }
);
```

### Settings

```typescript
interface TurnstileSettings {
  mode: "managed" | "non-interactive" | "invisible";
  theme: "auto" | "light" | "dark";
  size: "normal" | "compact" | "flexible";
  position: "above-heading" | "below-heading" | "below-subheading" | "below-loader" | "at-bottom";
  action: string;
  cData: string;
  fallback: "retry" | "error-page";
}
```

---

## 11. Shield Domain System

Optional layer that hides your backend domain from the Worker code.

### How It Works

1. User sets up a custom domain (e.g., `userdomain.com`)
2. Creates a ShieldDomain subdomain (e.g., `shield.userdomain.com`)
3. Configures nginx/reverse proxy on that domain to forward to your backend
4. Worker calls `shield.userdomain.com` instead of `yourplatform.com`

### Shield Domain Config

```nginx
# nginx config for shield.userdomain.com
location /api/bot-check {
    # Verify X-Shield-Key header matches secret
    if ($http_x_shield_key != "secret-key-here") {
        return 403;
    }
    proxy_pass https://yourplatform.com;
    proxy_set_header Host yourplatform.com;
}
```

### Worker Integration

```javascript
// Shielded worker calls shield domain
const checkRes = await fetch(
  `https://${SHIELD_DOMAIN}/api/bot-check`,
  {
    headers: {
      "X-Shield-Key": SHIELD_SECRET,
      "X-Visitor-IP": request.headers.get("CF-Connecting-IP")
    }
  }
);
```

---

## 12. Deployment Flow

### Full Create Flow

```
User clicks "Create" in UI
        │
        ▼
┌─────────────────────────────────────┐
│ POST /api/proxy-links               │
│   • Validate subdomain/path regex   │
│   • Validate URL schemes (https)    │
│   • Check CF credentials exist      │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│ Create DB row (status=pending)      │
│   • Auto-suffix subdomain on        │
│     conflict (mylink → mylink3x2k)  │
└─────────────────────────────────────┘
        │
        ▼ (if Turnstile enabled)
┌─────────────────────────────────────┐
│ Create Turnstile Widget             │
│   • POST to CF Turnstile API        │
│   • Save siteKey + encrypted secret │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│ Render Splash HTML                  │
│   • Apply customization settings    │
│   • Inject bot-mask snippet         │
│   • Inject Turnstile widget if on   │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│ Generate Worker Code                │
│   • Select mode (standalone/shield) │
│   • Embed splash HTML               │
│   • Embed redirect URLs             │
│   • Embed secrets                   │
│   • Obfuscate                       │
│   • Assert no leaks                 │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│ Deploy to CF Pages                  │
│   • Delete existing project         │
│   • Create fresh project            │
│   • Write _worker.js + _routes.json │
│   • Run wrangler pages deploy       │
│   • Poll until "live" status        │
└─────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────┐
│ Update DB row                       │
│   • status=deployed                 │
│   • cfWorkerUrl=https://x.pages.dev │
│   • lastDeployedAt=now()            │
└─────────────────────────────────────┘
        │
        ▼
Return { campaign, url }
```

---

## 13. Dependencies

### NPM Packages

```json
{
  "dependencies": {
    // Core
    "next": "^14.x",
    "react": "^18.x",
    "prisma": "^5.x",
    "@prisma/client": "^5.x",
    
    // UI
    "@radix-ui/react-*": "various",
    "sonner": "^1.x",
    "lucide-react": "^0.x",
    
    // Validation
    "zod": "^3.x",
    
    // Encryption
    "crypto": "builtin",
    
    // CF Deploy
    "wrangler": "^4.87.0"  // installed via npx, not dependency
  }
}
```

### External Services

- **Cloudflare** — Pages, Turnstile, API
- **Database** — PostgreSQL/MySQL with JSON support
- **Redis** — Rate limiting (optional)
- **Object Storage** — For user CF credentials encryption keys

### Environment Variables

```env
# Database
DATABASE_URL=postgresql://...

# Encryption key for CF tokens
ENCRYPTION_KEY=32-byte-hex-key

# Your platform domain (for shield mode)
NEXT_PUBLIC_PLATFORM_DOMAIN=yourplatform.com
NEXTAUTH_URL=https://yourplatform.com

# Rate limiting (optional)
REDIS_URL=redis://...
```

---

## 14. Implementation Checklist

### Phase 1: Core Infrastructure

- [ ] Database migrations (RedirectCampaign, ShieldDomain)
- [ ] Encryption utilities for CF tokens
- [ ] User Cloudflare settings page
- [ ] CF credentials validation endpoint

### Phase 2: Worker Generation

- [ ] Splash HTML renderer with all customization options
- [ ] Standalone Worker template
- [ ] Code obfuscation utility
- [ ] Leak detection utility
- [ ] Wrangler deploy wrapper

### Phase 3: API Endpoints

- [ ] `GET /api/proxy-links` — list campaigns
- [ ] `POST /api/proxy-links` — create + deploy
- [ ] `GET /api/proxy-links/[id]` — get campaign
- [ ] `PUT /api/proxy-links/[id]` — update + redeploy
- [ ] `DELETE /api/proxy-links/[id]` — delete campaign + CF project
- [ ] `GET /api/proxy-links/cf-zones` — check CF config

### Phase 4: Turnstile Integration

- [ ] Turnstile widget creation via CF API
- [ ] Turnstile verification in Worker
- [ ] Settings UI for mode/theme/position

### Phase 5: UI Components

- [ ] Campaign list table
- [ ] Create drawer (multi-step form)
- [ ] Customize page (splash editor)
- [ ] Live preview iframe
- [ ] Row actions (copy, edit, delete)

### Phase 6: Bot Protection (Optional)

- [ ] Shield domain setup flow
- [ ] Protected Worker template
- [ ] JS challenge cookie system
- [ ] Rate limiting middleware
- [ ] Header sanity checks

### Phase 7: Polish

- [ ] Deployment status polling with progress
- [ ] Error handling and user feedback
- [ ] Subdomain conflict auto-resolution
- [ ] Campaign analytics (clicks, countries)

---

## Source Files Reference

From `msf-tokens/custom-mail-client`:

| File | Purpose |
|------|---------|
| `src/features/proxy-links/index.tsx` | Main feature component |
| `src/features/proxy-links/data/schema.ts` | Zod schemas |
| `src/features/proxy-links/components/*` | UI components |
| `src/modules/redirect-generator/api/campaigns.ts` | API route (create/list) |
| `src/modules/redirect-generator/api/campaigns-id.ts` | API route (CRUD) |
| `src/modules/redirect-generator/cf-deploy.ts` | Wrangler deployment |
| `src/modules/redirect-generator/cf-worker-template.ts` | Worker code generator |
| `src/modules/redirect-generator/splash-renderer.ts` | HTML splash generator |
| `src/modules/redirect-generator/cf-turnstile.ts` | Turnstile widget API |
| `src/modules/redirect-generator/types.ts` | TypeScript types |
| `src/lib/cf-protected-worker.ts` | Bot protection builder |
| `src/lib/obfuscation.ts` | Code obfuscation |
| `src/proxy.ts` | Middleware bot detection |
| `src/lib/proxy-service.ts` | HTTP proxy for requests |

---

*Document generated: 2026-08-29*
*Source: msf-tokens/custom-mail-client reverse-engineering*

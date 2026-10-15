# Diataxis Documentation Audit

**Audit Date:** 2026-10-15  
**Framework:** [Diataxis](https://diataxis.fr) by Daniele Procida

## Executive Summary

This audit analyzed 66 documentation files across the botginx project. The documentation has grown organically and mixes internal planning documents with operational guides and API references. The main issues are:

1. **No clear organization by user need** - docs are organized by location/date, not purpose
2. **Type mixing within documents** - many docs blend how-to, reference, and explanation
3. **PLAN files dominate** - 15+ planning documents that should be in a separate category
4. **Missing tutorials** - no guided learning paths for new users
5. **No documentation index** - users can't find what they need

---

## Diataxis Quick Reference

| Type | Purpose | Oriented To | User Mode |
|------|---------|-------------|-----------|
| **Tutorial** | Teach by doing | Learning | Study |
| **How-to Guide** | Accomplish a goal | Tasks | Work |
| **Reference** | Describe the machinery | Information | Work |
| **Explanation** | Deepen understanding | Concepts | Study |

---

## Document Inventory & Classification

### Root Level Files

| File | Current Type | Should Be | Issues |
|------|--------------|-----------|--------|
| `CLAUDE.md` | Reference | Reference | OK - project configuration |
| `ENFORCEMENT-PLAN.md` | Mixed | Explanation | Mixes architecture + implementation steps |
| `PLAN-API-KEYS.md` | Explanation | Plan (internal) | Planning doc, not user-facing |
| `PLAN-CPANEL-DNS.md` | Explanation | Plan (internal) | Planning doc |
| `PLAN-HOMEPAGE.md` | Explanation | Plan (internal) | Planning doc |
| `PLAN-KUMO-THEME.md` | Explanation | Plan (internal) | Planning doc |
| `PLAN-PDF-GENERATOR.md` | Explanation | Plan (internal) | Planning doc |
| `PLAN-PUBLIC-API.md` | Explanation | Plan (internal) | Planning doc |
| `PLAN-QRCODES.md` | Explanation | Plan (internal) | Planning doc |
| `PROXY-LINKS-PORT-PLAN.md` | Explanation | Plan (internal) | Planning doc |
| `VPS-REMOTE-EXECUTION-BLUEPRINT.md` | Explanation | Explanation | Good - architecture concepts |

### docs/ Directory

| File | Current Type | Should Be | Issues |
|------|--------------|-----------|--------|
| `DEPLOYMENT.md` | How-to | How-to | Good structure, needs clearer section headers |
| `BOTECTION-CALLBACK-API.md` | Reference | Reference | Excellent - follows API spec pattern |
| `MODULE-INTEGRATION.md` | Reference | Reference | Good - interface specification |
| `CLOUDPANEL-HOSTING.md` | Mixed | Split | Mixes how-to, reference, and explanation |
| `CLOUDPANEL-QUICK-SETUP.md` | How-to | How-to | Good - task-oriented |
| `CPANEL-DOMAIN-FLOW.md` | Mixed | Explanation + How-to | UX flow + implementation mixed |
| `COMING-SOON-FEATURES.md` | Reference | Reference | Feature inventory |
| `TELEGRAM-INTEGRATION-PLAN.md` | Explanation | Plan (internal) | Planning doc |
| `TELEGRAM_BACKUP_RESTORE.md` | Mixed | How-to | Mixes how-to with troubleshooting reference |
| `NAMESILO-INTEGRATION-PLAN.md` | Explanation | Plan (internal) | Planning doc |
| `MARKETPLACE-PLAN.md` | Explanation | Plan (internal) | Planning doc |
| `ANTIBOT-IP-LISTS-INTEGRATION.md` | Reference | Reference | Spec for external dev - good |
| `PLAN-IP-LISTS.md` | Explanation | Plan (internal) | Planning doc |
| `REFERRALS-PLAN.md` | Explanation | Plan (internal) | Planning doc |
| `ACME-DNS-DELEGATION-PLAN.md` | Mixed | Explanation | Architecture doc with implementation |
| `BITGO-PAYMENT-GATEWAY-PLAN.md` | Explanation | Plan (internal) | Planning doc |

### docs/plans/ Directory (All Planning Documents)

| File | Type | Notes |
|------|------|-------|
| `cpanel-domain-integration.md` | Plan | Implementation plan |
| `dashboard-redesign.md` | Plan | Implementation plan |
| `marketplace-domain-masking.md` | Plan | Implementation plan |
| `support-tickets.md` | Plan | Implementation plan |
| `telegram-backup-system.md` | Plan | Implementation plan |
| `tracking-style-urls.md` | Plan | Implementation plan |

### docs/superpowers/ (Agent-Generated Plans)

All files in `docs/superpowers/plans/` and `docs/superpowers/specs/` are agent-generated planning and specification documents. These are internal artifacts, not user documentation.

### docs-site/ (Public Documentation)

| File | Current Type | Should Be | Issues |
|------|--------------|-----------|--------|
| **Guides** | | | |
| `guides/introduction.md` | Explanation | Explanation | Good - explains the product |
| `guides/quick-start.md` | Tutorial | Tutorial | Good - guided learning |
| **API** | | | |
| `api/overview.md` | Reference | Reference | Good |
| `api/authentication.md` | Reference | Reference | Good |
| `api/links.md` | Reference | Reference | Good |
| `api/domains.md` | Reference | Reference | Good |
| `api/analytics.md` | Reference | Reference | Good |
| `api/qrcodes.md` | Reference | Reference | Good |
| `api/shortener.md` | Reference | Reference | Good |
| `api/iplists.md` | Reference | Reference | Good |
| `api/account.md` | Reference | Reference | Good |
| `api/webhooks.md` | Reference | Reference | Good |
| `api/errors.md` | Reference | Reference | Good |
| `api/rate-limiting.md` | Reference | Reference | Good |
| **Features** | | | |
| `features/redirect-links.md` | Mixed | Split | Explanation + How-to mixed |
| `features/bot-protection.md` | Mixed | Split | Explanation + How-to mixed |
| `features/analytics.md` | Mixed | How-to | Feature description |
| `features/qr-codes.md` | Mixed | How-to | Feature description |
| `features/hosting.md` | Mixed | How-to | Feature description |
| **Integrations** | | | |
| `integrations/cpanel.md` | How-to | How-to | Good - task-oriented |
| `integrations/namesilo.md` | How-to | How-to | Good - task-oriented |
| `integrations/payments.md` | How-to | How-to | Good - task-oriented |
| `integrations/telegram.md` | How-to | How-to | Good - task-oriented |

---

## Key Issues Identified

### 1. Mixed Document Types (HIGH PRIORITY)

Several documents blend multiple Diataxis types, making them harder to use:

**CLOUDPANEL-HOSTING.md** contains:
- Architecture explanation (how the system works)
- Setup how-to (deploy steps)
- Reference tables (settings, metrics)
- Troubleshooting guide

**Recommendation:** Split into:
- `explanation/cloudpanel-architecture.md` - How the system works
- `how-to/setup-cloudpanel.md` - Step-by-step setup
- `reference/cloudpanel-settings.md` - Settings and metrics reference

**TELEGRAM_BACKUP_RESTORE.md** contains:
- Backup how-to
- Restore how-to
- Manifest format reference
- Troubleshooting reference

**Recommendation:** Split into:
- `how-to/backup-to-telegram.md`
- `how-to/restore-from-telegram.md`
- `reference/backup-manifest.md`

### 2. Planning Documents Mixed with Operational Docs (HIGH PRIORITY)

15+ files are internal planning documents (`PLAN-*.md`, `*-PLAN.md`). These:
- Are not useful to end users
- Clutter the docs directory
- Make it hard to find operational documentation

**Recommendation:** Move all planning docs to `docs/internal/plans/`. Create a clear separation:
```
docs/
├── README.md              # Public documentation index
├── how-to/               # Operational guides
├── reference/            # API specs, settings
├── explanation/          # Architecture, concepts
└── internal/             # Not for users
    └── plans/            # Planning documents
```

### 3. No Documentation Index (HIGH PRIORITY)

Users cannot quickly find what they need. There's no `docs/README.md` showing available documentation.

**Recommendation:** Create `docs/README.md` organized by Diataxis quadrants.

### 4. Missing Tutorials (MEDIUM PRIORITY)

No guided learning paths exist for:
- Setting up your first redirect link
- Configuring bot protection
- Integrating with external systems

**Recommendation:** Create tutorials:
- `tutorials/your-first-redirect-link.md`
- `tutorials/configure-bot-protection.md`

### 5. How-to Titles Don't Follow "How to..." Pattern (MEDIUM PRIORITY)

Many how-to guides don't clearly state what task they help accomplish.

| Current Title | Recommended Title |
|---------------|-------------------|
| `DEPLOYMENT.md` | `how-to-deploy-botginx.md` |
| `CLOUDPANEL-QUICK-SETUP.md` | `how-to-setup-cloudpanel-server.md` |
| `MODULE-INTEGRATION.md` | `how-to-integrate-a-module.md` |

### 6. Feature Docs Mix Explanation and How-to (MEDIUM PRIORITY)

The `docs-site/features/` directory mixes "what is it" (explanation) with "how to use it" (how-to).

**Recommendation:** Each feature should have:
- Explanation section: What it is and why
- How-to section: How to configure and use it

---

## Restructuring Recommendations

### Proposed Directory Structure

```
docs/
├── README.md                      # Documentation index (Diataxis organized)
│
├── tutorials/                     # Learning-oriented (NEW)
│   ├── first-redirect-link.md
│   └── bot-protection-basics.md
│
├── how-to/                        # Task-oriented
│   ├── deploy-botginx.md          # (moved from DEPLOYMENT.md)
│   ├── setup-cloudpanel-server.md # (moved from CLOUDPANEL-QUICK-SETUP.md)
│   ├── backup-to-telegram.md      # (split from TELEGRAM_BACKUP_RESTORE.md)
│   ├── restore-from-telegram.md   # (split from TELEGRAM_BACKUP_RESTORE.md)
│   └── integrate-module.md        # (moved from MODULE-INTEGRATION.md)
│
├── reference/                     # Information-oriented
│   ├── botection-callback-api.md  # (moved from BOTECTION-CALLBACK-API.md)
│   ├── module-interface.md        # (extracted from MODULE-INTEGRATION.md)
│   ├── environment-variables.md   # (extracted from CLAUDE.md)
│   ├── backup-manifest-format.md  # (split from TELEGRAM_BACKUP_RESTORE.md)
│   ├── coming-soon-features.md    # (kept)
│   └── antibot-ip-lists-spec.md   # (moved from ANTIBOT-IP-LISTS-INTEGRATION.md)
│
├── explanation/                   # Understanding-oriented
│   ├── enforcement-architecture.md # (moved from ENFORCEMENT-PLAN.md)
│   ├── cloudpanel-architecture.md # (split from CLOUDPANEL-HOSTING.md)
│   ├── acme-dns-delegation.md     # (moved from ACME-DNS-DELEGATION-PLAN.md)
│   └── vps-remote-execution.md    # (moved from VPS-REMOTE-EXECUTION-BLUEPRINT.md)
│
└── internal/                      # NOT for end users
    └── plans/                     # All PLAN-*.md files moved here
        ├── api-keys.md
        ├── cpanel-dns.md
        ├── marketplace.md
        ├── namesilo-integration.md
        ├── referrals.md
        ├── telegram-integration.md
        └── ...
```

### docs-site/ (Public Docs) Restructuring

The public documentation is better organized but still has mixed types in features:

```
docs-site/src/content/docs/
├── guides/
│   ├── introduction.md           # Explanation (keep)
│   └── quick-start.md            # Tutorial (keep)
│
├── tutorials/                    # NEW - guided learning
│   ├── create-first-link.md
│   ├── enable-bot-protection.md
│   └── setup-custom-domain.md
│
├── how-to/                       # NEW - task-oriented
│   ├── rotate-destinations.md
│   ├── configure-country-blocking.md
│   └── export-analytics-data.md
│
├── features/                     # Explanation (what/why)
│   ├── redirect-links.md         # Keep, but move "how to" sections
│   ├── bot-protection.md         # Keep, but move "how to" sections
│   ├── analytics.md
│   ├── qr-codes.md
│   └── hosting.md
│
├── api/                          # Reference (keep as-is, well structured)
│   ├── overview.md
│   ├── authentication.md
│   ├── links.md
│   └── ...
│
└── integrations/                 # How-to (keep as-is)
    ├── cpanel.md
    ├── namesilo.md
    ├── payments.md
    └── telegram.md
```

---

## Action Items

### Immediate (Do Now)

1. [x] Create `docs/README.md` as documentation index
2. [ ] Move all `PLAN-*.md` files to `docs/internal/plans/`
3. [ ] Rename key how-to docs with "How to..." titles

### Short-term (This Week)

4. [ ] Split `CLOUDPANEL-HOSTING.md` into explanation/how-to/reference
5. [ ] Split `TELEGRAM_BACKUP_RESTORE.md` into how-to + reference
6. [ ] Create `docs/tutorials/` directory with first tutorial
7. [ ] Update docs-site navigation to reflect Diataxis structure

### Medium-term (This Month)

8. [ ] Create missing tutorials for key user journeys
9. [ ] Extract "how to" sections from feature docs into dedicated guides
10. [ ] Add cross-links between related docs (tutorial → how-to → reference)
11. [ ] Review and update all docs for language patterns per Diataxis

---

## Diataxis Language Patterns

For reference when restructuring:

### Tutorial Language
- "We will..." / "In this tutorial, we will..."
- "First, do X. Now, do Y."
- "Notice that..."
- "You have successfully..."

### How-to Language
- "This guide shows you how to..."
- "If you want X, do Y."
- "Refer to the X reference for full options."
- Conditional imperatives, not teaching

### Reference Language
- "X is available as..."
- Lists: "Sub-commands are: a, b, c"
- Warnings: "You must use X. Never Y."
- State facts, no opinions

### Explanation Language
- "The reason for X is..."
- "X is better than Y because..."
- "An X in system Y is analogous to..."
- Discuss alternatives and trade-offs

---

## Summary Statistics

| Category | Count | Well-Structured | Needs Work |
|----------|-------|-----------------|------------|
| Tutorials | 1 | 1 | 0 |
| How-to Guides | 8 | 4 | 4 |
| Reference | 18 | 15 | 3 |
| Explanation | 4 | 2 | 2 |
| Plans (internal) | 20+ | N/A | N/A |
| Mixed Type | 8 | 0 | 8 |

**Overall Assessment:** The public API docs are well-structured as reference material. The main project docs need restructuring to separate planning documents from operational documentation, and to split mixed-type documents into their proper Diataxis categories.

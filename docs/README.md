# Botginx Documentation

Documentation for the botginx redirect generator panel, organized by what you need.

## Getting Started

New to botginx? Start here:

- **[Quick Start](../docs-site/src/content/docs/guides/quick-start.md)** — Create your first protected redirect link
- **[Introduction](../docs-site/src/content/docs/guides/introduction.md)** — What botginx does and why

---

## Tutorials

Learn by doing. Step-by-step lessons that build skills.

*Coming soon — see [DIATAXIS-AUDIT.md](DIATAXIS-AUDIT.md) for planned tutorials.*

---

## How-to Guides

Accomplish specific tasks. Use these when you know what you want to do.

### Deployment

| Guide | Description |
|-------|-------------|
| [Deploy Botginx](DEPLOYMENT.md) | Deploy to a VPS with Tor hidden service and auto-updates |
| [Setup CloudPanel Server](CLOUDPANEL-QUICK-SETUP.md) | Provision a CloudPanel hosting VPS |

### Integration

| Guide | Description |
|-------|-------------|
| [CloudPanel Hosting](CLOUDPANEL-HOSTING.md) | Full guide for CloudPanel servers with antibot |
| [Telegram Backup & Restore](TELEGRAM_BACKUP_RESTORE.md) | Backup and restore via Telegram |
| [cPanel Domain Setup](CPANEL-DOMAIN-FLOW.md) | External domain setup flow for cPanel users |

### Development

| Guide | Description |
|-------|-------------|
| [Module Integration](MODULE-INTEGRATION.md) | Integrate third-party Go modules |

---

## Reference

Technical specifications and API documentation. Look things up.

### API Specifications

| Reference | Description |
|-----------|-------------|
| [Botection Callback API](BOTECTION-CALLBACK-API.md) | API for per-link blocking decisions |
| [Antibot IP Lists Integration](ANTIBOT-IP-LISTS-INTEGRATION.md) | IP whitelist/blocklist spec for botection |

### Configuration

| Reference | Description |
|-----------|-------------|
| [Project Configuration](../CLAUDE.md) | Environment variables, project structure |
| [Coming Soon Features](COMING-SOON-FEATURES.md) | Planned features with route and implementation notes |

### Public API

The public API reference is in the [docs-site](../docs-site/src/content/docs/api/):

- [API Overview](../docs-site/src/content/docs/api/overview.md)
- [Authentication](../docs-site/src/content/docs/api/authentication.md)
- [Links API](../docs-site/src/content/docs/api/links.md)
- [Domains API](../docs-site/src/content/docs/api/domains.md)
- [Analytics API](../docs-site/src/content/docs/api/analytics.md)

---

## Explanation

Understand how things work. Background, architecture, design decisions.

| Document | Description |
|----------|-------------|
| [Bot Protection Enforcement](../ENFORCEMENT-PLAN.md) | How settings push to VPS and enforcement works |
| [VPS Remote Execution](../VPS-REMOTE-EXECUTION-BLUEPRINT.md) | SSH execution architecture based on Vito |
| [ACME-DNS Delegation](ACME-DNS-DELEGATION-PLAN.md) | Wildcard SSL via DNS delegation |

---

## Internal Documentation

Planning and specification documents for development. Not end-user documentation.

### Plans

Located in `docs/plans/` and various `PLAN-*.md` files:

- Feature implementation plans
- Architecture proposals
- Integration specifications

### Agent-Generated Specs

Located in `docs/superpowers/`:

- Design specifications
- Implementation briefs
- Task reports

---

## Documentation Standards

This documentation follows the [Diataxis framework](https://diataxis.fr):

- **Tutorials** — Learning-oriented lessons (teach by doing)
- **How-to Guides** — Task-oriented steps (accomplish goals)
- **Reference** — Information-oriented descriptions (look things up)
- **Explanation** — Understanding-oriented discussion (concepts and why)

See [DIATAXIS-AUDIT.md](DIATAXIS-AUDIT.md) for the documentation audit and restructuring plan.

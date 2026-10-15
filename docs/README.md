# Botginx Documentation

Documentation for the botginx redirect generator panel, organized by what you need ([Diataxis](https://diataxis.fr)).

## Tutorials

Learn by doing.

| Tutorial | Description |
|----------|-------------|
| [Your First Redirect Link](tutorials/first-redirect-link.md) | Create a link, visit it, see it in analytics |
| [Bot Protection Basics](tutorials/configure-bot-protection.md) | Turn on protection, block a bot, read the threat log |

The public docs site also has a [Quick Start](../docs-site/src/content/docs/guides/quick-start.md) and an [Introduction](../docs-site/src/content/docs/guides/introduction.md).

## How-to Guides

Accomplish a specific task.

| Guide | Description |
|-------|-------------|
| [Deploy Botginx](how-to/deploy-botginx.md) | Deploy to a VPS with Tor hidden service and auto-updates |
| [Set Up a CloudPanel Server (quick)](how-to/setup-cloudpanel-server.md) | Provision a CloudPanel hosting VPS |
| [Set Up CloudPanel Hosting](how-to/setup-cloudpanel-hosting.md) | Connect a CloudPanel server to botginx with botection |
| [Customize CloudPanel Branding](how-to/customize-cloudpanel-branding.md) | Apply your own name, colors and logos |
| [Back Up to Telegram](how-to/backup-to-telegram.md) | Back up database and config to a Telegram chat |
| [Restore from Telegram](how-to/restore-from-telegram.md) | Restore onto a fresh server from a backup |
| [Integrate a Module](how-to/integrate-module.md) | Integrate third-party Go modules |

## Reference

Look things up.

| Reference | Description |
|-----------|-------------|
| [Botection Callback API](reference/botection-callback-api.md) | API for per-link blocking decisions |
| [Antibot IP Lists Spec](reference/antibot-ip-lists-spec.md) | IP whitelist/blocklist spec for botection |
| [CloudPanel Settings](reference/cloudpanel-settings.md) | Settings file, options, metrics, stats API, directory layout |
| [Backup Manifest Format](reference/backup-manifest-format.md) | Telegram backup manifest, chunking, API methods |
| [Coming Soon Features](reference/coming-soon-features.md) | Planned features with route and implementation notes |
| [OpenAPI spec](openapi.yaml) | Machine-readable API description |
| [Project Configuration](../CLAUDE.md) | Environment variables, project structure |

Public API reference lives in [docs-site](../docs-site/src/content/docs/api/overview.md).

## Explanation

Understand how and why.

| Document | Description |
|----------|-------------|
| [Bot Protection Enforcement](explanation/enforcement-architecture.md) | How settings push to VPS and enforcement works |
| [VPS Remote Execution](explanation/vps-remote-execution.md) | SSH execution architecture based on Vito |
| [CloudPanel Architecture](explanation/cloudpanel-architecture.md) | How panel, botection and CloudPanel fit together |
| [ACME-DNS Delegation](explanation/acme-dns-delegation.md) | Wildcard SSL via DNS delegation |
| [cPanel Domain Flow](explanation/cpanel-domain-flow.md) | Why and how external domains work for cPanel users |

## Internal

Not end-user documentation.

- [`internal/plans/`](internal/plans/) — feature and architecture plans (`PLAN-*`, `*-PLAN`, design proposals)
- [`superpowers/`](superpowers/) — agent-generated plans and specs

## Audit

See [DIATAXIS-AUDIT.md](DIATAXIS-AUDIT.md) for the audit behind this structure.

# GuardBot Brand Guidelines

Used by the `banner-design` skill (and anyone making graphics) to fill brand tokens.

## Logo

- Wordmark: `web/static/img/guardbot-logo.svg`: "Guard" + "Bot" inside an outlined rounded box. Use white on colored backgrounds.
- Icon: `web/static/img/logo-icon.svg`: shield with a check mark.

## Colors

| Token | Value | Use |
|---|---|---|
| `--brand` | `#e63946` | primary red (also the active tab color in `intel.css`) |
| `--brand-hi` | `#d42a38` | light end of banner gradient |
| `--brand-lo` | `#a3131f` | dark end of banner gradient |
| `--brand-deep` | `#7a0c16` | solid pill fill |
| `--accent-dark` | `#2a070b` | dark accent headline line on red |
| `--ink` | `#1b1d22` | text on white |
| `--muted` | `#7a7f88` | secondary text on white |
| `--ok` | `#1f9d55` | success, "human / passed" |

## Fonts

- Headlines: Lexend 700-800
- UI text: Inter 400-700
- Codes, paths, IPs inside mockups: JetBrains Mono

## Banner style

- Intel blog banners: 1200x630, flat red marketing style (see the banner-design skill's house style). No dark/neon "hacker" look.
- Product vocabulary for mockups: Domains (Verified badge), Redirect Links → New Link (Domain, Path, Destination), Block Known Bots, Threat Log. Block reasons: "Datacenter IP", "Bot signature detected", "Headless browser".
- Approved examples: `web/static/uploads/intel/2026/11/antibot-protection-banner-alt.png`, `create-first-redirect-link-banner.png`, `create-first-redirect-link-protection-active.png`.

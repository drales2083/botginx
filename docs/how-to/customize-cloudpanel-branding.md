# How to Customize CloudPanel Branding

This guide shows you how to apply your own name, colors and logos to a CloudPanel server.

## Quick Branding Setup

```bash
# Upload branding script
scp scripts/cloudpanel-branding.sh root@SERVER:/opt/

# Run with defaults (GuardHost name, red theme, dark mode)
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'

# Or with custom settings
ssh root@SERVER 'PANEL_NAME=MyHost PRIMARY_COLOR=#007bff bash /opt/cloudpanel-branding.sh'
```

## Branding Options

| Variable | Default | Description |
|----------|---------|-------------|
| PANEL_NAME | GuardHost | Custom panel name (replaces "CloudPanel") |
| PRIMARY_COLOR | #dc3545 | Theme color (hex) for buttons, links |
| FORCE_DARK_THEME | true | Set dark mode as default |
| LOGO_LIGHT | - | Path to light theme logo |
| LOGO_DARK | - | Path to dark theme logo |
| LOGO_FAVICON | - | Path to favicon |

## What Gets Customized

1. **Panel Name** - "CloudPanel" text replaced throughout UI
2. **Primary Color** - Buttons, links, accents use your color
3. **Dark Theme** - Set as default for all users
4. **Logos** - Custom logos and favicon (optional)

## Re-applying After Updates

The script creates a daily cron job that checks if branding needs to be re-applied after CloudPanel updates. You can also manually re-run:

```bash
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
```

## Branding Files Location

```
/opt/cloudpanel-branding/
├── css/
│   └── custom-branding.css   # CSS overrides
├── logos/
│   ├── logo-light.png        # Light theme logo
│   ├── logo-dark.png         # Dark theme logo
│   └── favicon.ico           # Favicon
└── cloudpanel-branding.sh    # Script copy
```

---
title: Connect Claude Desktop to GuardBot with MCP
slug: connect-claude-desktop-to-guardbot-with-mcp
date: 2026-11-14
author: GuardBot Team
category: tutorials
tags:
  - mcp
  - claude
  - ai-tools
  - integrations
excerpt: Manage your redirect links, domains, and analytics directly from Claude Desktop. No dashboard needed.
featured: true
cover_image: /static/uploads/intel/2026/11/mcp-server-banner.png
---

What if you could ask Claude to create a redirect link for you? Or check how many clicks your Black Friday campaign got? Or delete that old promo link cluttering your dashboard?

You can. GuardBot's MCP Server connects Claude Desktop directly to your account.

## What MCP Does

MCP (Model Context Protocol) lets AI assistants like Claude access external tools. Instead of copying data between apps, you talk to Claude and it handles the rest.

With GuardBot's MCP Server installed, Claude can:

- Search and list your redirect links
- Check domain status and SSL configuration
- Pull analytics for any link
- Create new links on your domains
- Update destinations
- Delete links you don't need

All through conversation. No clicking around the dashboard.

## Why This Matters

Managing redirect links usually means:

1. Open the dashboard
2. Find the link
3. Check the stats
4. Make your change
5. Repeat for the next one

With MCP, you say "show me my top 5 links this week" and Claude pulls them. You say "create a link to example.com/sale on my go.brand.co domain" and it's done.

The difference is friction. Less friction means you actually check your analytics. You actually clean up old links. You catch problems faster because looking is easy.

![Benefits of MCP Integration](/static/uploads/intel/2026/11/mcp-server-benefits.png)

## Getting Started

You'll need three things: Claude Desktop, the GuardBot MCP binary, and an API key.

### Download the Binary

Pick your platform:

| Platform | Download |
|----------|----------|
| macOS Apple Silicon | [guardbot-mcp-darwin-arm64](https://guardbot.sbs/downloads/guardbot-mcp-darwin-arm64) |
| macOS Intel | [guardbot-mcp-darwin-amd64](https://guardbot.sbs/downloads/guardbot-mcp-darwin-amd64) |
| Windows 64-bit | [guardbot-mcp-windows-amd64.exe](https://guardbot.sbs/downloads/guardbot-mcp-windows-amd64.exe) |

### Install It

On macOS:

```bash
# Download (Apple Silicon example)
curl -L -o guardbot-mcp https://guardbot.sbs/downloads/guardbot-mcp-darwin-arm64

# Make executable
chmod +x guardbot-mcp

# Move to PATH
sudo mv guardbot-mcp /usr/local/bin/

# Remove quarantine
xattr -d com.apple.quarantine /usr/local/bin/guardbot-mcp
```

On Windows, download the .exe and put it somewhere like `C:\Program Files\GuardBot\`.

### Get Your API Key

Go to [Account Settings → API Keys](https://guardbot.sbs/user/apikeys) and create one. It starts with `bg_live_`. Keep it somewhere safe.

### Configure Claude Desktop

Open the Claude Desktop config file:

- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

Add GuardBot to the mcpServers section:

```json
{
  "mcpServers": {
    "guardbot": {
      "command": "/usr/local/bin/guardbot-mcp",
      "env": {
        "GUARDBOT_API_KEY": "bg_live_your_key_here"
      }
    }
  }
}
```

Restart Claude Desktop. Done.

## What You Can Ask

Once connected, Claude has seven GuardBot tools available:

| Tool | What it does |
|------|--------------|
| Search links | Find links by keyword |
| List domains | Show all your verified domains |
| List links | Get your recent redirect links |
| Get link stats | Pull analytics for a specific link |
| Create link | Make a new redirect link |
| Update link | Change a link's destination |
| Delete link | Remove a link |

Some examples:

> "List my GuardBot domains"

> "Search for links containing 'black-friday'"

> "How many clicks did link abc123 get last week?"

> "Create a redirect from go.mybrand.co/sale to mybrand.com/holiday-sale"

> "Delete the link with ID xyz789"

Claude figures out which tool to use based on what you ask.

![Available GuardBot tools](/static/uploads/intel/2026/11/mcp-server-tools.png)

## Security

Your API key stays local. It lives in the Claude Desktop config on your machine and goes directly to GuardBot's API over HTTPS. No third parties see it.

If you ever need to revoke access, delete the key from your [account settings](https://guardbot.sbs/user/apikeys). The MCP server stops working immediately.

![Direct connection - your key stays local](/static/uploads/intel/2026/11/mcp-server-security.png)

## Common Issues

**"Command not found"** — The binary isn't in your PATH. Use the full path in your config instead.

**"GUARDBOT_API_KEY is required"** — The environment variable isn't set. Check your config file syntax.

**"401 Unauthorized"** — Your API key is wrong or expired. Generate a new one.

**macOS security warning** — Run the quarantine removal command: `xattr -d com.apple.quarantine /usr/local/bin/guardbot-mcp`

## What's Next

Once you're connected, start simple. Ask Claude to list your domains. Then your recent links. Then analytics for one of them.

The more you use it, the more natural it feels. Managing links through conversation beats clicking through a dashboard every time.

[Download the MCP Server](https://guardbot.sbs/downloads/guardbot-mcp-darwin-arm64) and [get your API key](https://guardbot.sbs/user/apikeys) to get started.

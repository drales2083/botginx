# MCP Server for Claude Desktop

Connect Claude Desktop directly to your GuardBot account using the Model Context Protocol (MCP).

> **Get your API key:** [Account Settings → API Keys](https://guardbot.sbs/user/apikeys)

## Overview

The GuardBot MCP Server exposes your redirect links, domains, and analytics to Claude Desktop. You can ask Claude to:

- Search and list your redirect links
- View domain status and SSL configuration
- Get analytics for specific links
- Create, update, and delete redirect links

## Download

| Platform | Architecture | Download |
|----------|-------------|----------|
| macOS | Apple Silicon (M1/M2/M3) | [guardbot-mcp-darwin-arm64](https://guardbot.sbs/downloads/guardbot-mcp-darwin-arm64) |
| macOS | Intel | [guardbot-mcp-darwin-amd64](https://guardbot.sbs/downloads/guardbot-mcp-darwin-amd64) |
| Windows | 64-bit | [guardbot-mcp-windows-amd64.exe](https://guardbot.sbs/downloads/guardbot-mcp-windows-amd64.exe) |

## Installation

### macOS

1. Download the binary for your Mac:
   ```bash
   # Apple Silicon (M1/M2/M3)
   curl -L -o guardbot-mcp https://guardbot.sbs/downloads/guardbot-mcp-darwin-arm64
   
   # Intel Mac
   curl -L -o guardbot-mcp https://guardbot.sbs/downloads/guardbot-mcp-darwin-amd64
   ```

2. Make it executable:
   ```bash
   chmod +x guardbot-mcp
   ```

3. Move to a permanent location:
   ```bash
   sudo mv guardbot-mcp /usr/local/bin/
   ```

4. Remove quarantine (macOS security):
   ```bash
   xattr -d com.apple.quarantine /usr/local/bin/guardbot-mcp
   ```

### Windows

1. Download `guardbot-mcp-windows-amd64.exe`
2. Move to a folder like `C:\Program Files\GuardBot\`
3. Note the full path for the config below

## Configuration

### Get Your API Key

1. Go to [Account Settings → API Keys](https://guardbot.sbs/user/apikeys)
2. Click "Create API Key"
3. Copy the key (starts with `bg_live_`)

### Configure Claude Desktop

Edit the Claude Desktop config file:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`

**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

Add the GuardBot server:

```json
{
  "mcpServers": {
    "guardbot": {
      "command": "/usr/local/bin/guardbot-mcp",
      "env": {
        "GUARDBOT_API_KEY": "bg_live_your_api_key_here"
      }
    }
  }
}
```

**Windows example:**
```json
{
  "mcpServers": {
    "guardbot": {
      "command": "C:\\Program Files\\GuardBot\\guardbot-mcp-windows-amd64.exe",
      "env": {
        "GUARDBOT_API_KEY": "bg_live_your_api_key_here"
      }
    }
  }
}
```

### Restart Claude Desktop

Quit and reopen Claude Desktop. The GuardBot tools will now be available.

## Available Tools

| Tool | Description |
|------|-------------|
| `guardbot_search_links` | Search redirect links by keyword |
| `guardbot_get_domains` | List all your verified domains |
| `guardbot_get_links` | List your most recent redirect links |
| `guardbot_get_link_stats` | Get analytics for a specific link |
| `guardbot_create_link` | Create a new redirect link |
| `guardbot_update_link` | Update a redirect link destination |
| `guardbot_delete_link` | Delete a redirect link |

## Usage Examples

Once configured, you can ask Claude:

> "List my GuardBot domains"

> "Search for my Black Friday links"

> "How many clicks did link abc123 get?"

> "Create a new redirect link to https://example.com on my domain"

> "Delete the link with ID xyz789"

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GUARDBOT_API_KEY` | Yes | Your API key from account settings |
| `GUARDBOT_API_URL` | No | API endpoint (default: `https://guardbot.sbs`) |

## Troubleshooting

### "Command not found"

Make sure the binary is in your PATH or use the full path in the config.

### "GUARDBOT_API_KEY is required"

The API key environment variable is not set. Check your config file.

### "API error: 401 Unauthorized"

Your API key is invalid or expired. Generate a new one from account settings.

### macOS: "cannot be opened because it is from an unidentified developer"

Run the quarantine removal command:
```bash
xattr -d com.apple.quarantine /usr/local/bin/guardbot-mcp
```

## Security

- Your API key is stored locally in the Claude Desktop config
- The MCP server communicates directly with GuardBot's API over HTTPS
- No data is sent to third parties
- API keys can be revoked anytime from account settings

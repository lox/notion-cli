# notion-cli

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A command-line interface for Notion using the remote MCP (Model Context Protocol).

Inspired by [linear-cli](https://github.com/schpet/linear-cli) - stay in the terminal while managing your Notion workspace.

## Installation

### From Source

```bash
go install github.com/lox/notion-cli@latest
```

### Build Locally

```bash
git clone https://github.com/lox/notion-cli
cd notion-cli
task build
```

## Quick Start

```bash
# Authenticate with Notion (opens browser for OAuth)
notion-cli config auth

# Search your workspace
notion-cli search "meeting notes"

# View a page
notion-cli page view "https://notion.so/My-Page-abc123"

# List your pages
notion-cli page list

# Create a page
notion-cli page create --title "New Page" --content "# Hello World"
```

## Commands

### Configuration

```bash
notion-cli config auth     # Run OAuth flow to authenticate
notion-cli config show     # Show current configuration
notion-cli config clear    # Clear stored credentials
```

### Pages

```bash
notion-cli page list                           # List pages
notion-cli page list --limit 50                # Limit results
notion-cli page list --json                    # Output as JSON

notion-cli page view <url>                     # View page content
notion-cli page view <url> --json              # Output as JSON

notion-cli page create --title "Title"         # Create a page
notion-cli page create --title "T" --content "Body text"
notion-cli page create --title "T" --parent <page-id>
```

### Search

```bash
notion-cli search "query"                      # Search workspace
notion-cli search "query" --limit 10           # Limit results
notion-cli search "query" --json               # Output as JSON
```

### Databases

```bash
notion-cli db list                             # List databases
notion-cli db list --json                      # Output as JSON

notion-cli db query <database-id>              # Query database
notion-cli db query <id> --json                # Output as JSON
```

### Comments

```bash
notion-cli comment list <page-id>              # List comments on a page
notion-cli comment list <page-id> --json       # Output as JSON

notion-cli comment create <page-id> --content "Comment text"
```

### Other

```bash
notion-cli version                             # Show version
notion-cli --help                              # Show help
```

## Configuration

Configuration is stored at `~/.config/notion-cli/config.json`.

The CLI uses Notion's remote MCP server with OAuth authentication. On first run, `notion-cli config auth` will open your browser to authorize the CLI with your Notion workspace.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NOTION_ACCESS_TOKEN` | Access token for CI/headless usage (skips OAuth) |

## How It Works

This CLI connects to [Notion's remote MCP server](https://developers.notion.com/guides/mcp/mcp) at `https://mcp.notion.com/mcp` using the Model Context Protocol. This provides:

- **OAuth authentication** - No API tokens to manage
- **Notion-flavoured Markdown** - Create/edit content naturally
- **Semantic search** - Search across connected apps too
- **Optimised for CLI** - Efficient responses

## Links

- [Notion MCP Documentation](https://developers.notion.com/guides/mcp/mcp)
- [Notion API Reference](https://developers.notion.com/reference/intro)
- [Model Context Protocol](https://modelcontextprotocol.io/)

## License

MIT License - see [LICENSE](LICENSE) for details.

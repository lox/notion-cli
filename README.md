# notion-cli

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.22-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A command-line interface for Notion using the remote MCP (Model Context Protocol).

Inspired by [linear-cli](https://github.com/schpet/linear-cli) - stay in the terminal while managing your Notion workspace.

**Works great with AI agents** — includes a [skill](#skills) that lets agents search, create, and manage your Notion workspace alongside your code.

## Installation

### From Source

```bash
go install github.com/lox/notion-cli@latest
```

### Build Locally

```bash
git clone https://github.com/lox/notion-cli
cd notion-cli
mise run build
```

## Quick Start

```bash
# Authenticate with Notion (opens browser for OAuth)
notion-cli auth login

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

### Authentication

```bash
notion-cli auth login      # Authenticate with Notion via OAuth
notion-cli auth refresh    # Refresh the access token
notion-cli auth status     # Show authentication status
notion-cli auth logout     # Clear stored credentials
```

### Pages

```bash
notion-cli page list                           # List pages
notion-cli page list --limit 50                # Limit results
notion-cli page list --json                    # Output as JSON

notion-cli page view <url>                     # View page content
notion-cli page view <url> --raw               # View raw Notion markup
notion-cli page view <url> --json              # Output as JSON

notion-cli page create --title "Title"         # Create a page
notion-cli page create --title "T" --content "Body text"
notion-cli page create --title "T" --parent <page-id>

# Upload a markdown file as a new page
notion-cli page upload ./document.md                        # Title from # heading or filename
notion-cli page upload ./document.md --title "Custom Title" # Explicit title
notion-cli page upload ./document.md --parent "Engineering" # Parent by name or ID
notion-cli page upload ./document.md --icon "📄"             # Set emoji icon

# Sync a markdown file (create or update)
notion-cli page sync ./document.md                          # Creates page, writes notion-id to frontmatter
notion-cli page sync ./document.md                          # Updates page using notion-id from frontmatter
notion-cli page sync ./document.md --parent "Engineering"   # Set parent on first sync

# Edit an existing page
notion-cli page edit <url> --replace "New content"                      # Replace all content
notion-cli page edit <url> --find "old text" --replace-with "new text"  # Find and replace
notion-cli page edit <url> --find "section" --append "extra content"    # Append after match
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

The CLI uses Notion's remote MCP server with OAuth authentication. On first run, `notion-cli auth login` will open your browser to authorize the CLI with your Notion workspace.

**Note:** Access tokens expire after 1 hour. The CLI automatically refreshes tokens when they expire or are about to expire, so you typically don't need to think about this. Use `notion-cli auth refresh` to manually refresh if needed.

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

## Skills

notion-cli includes a skill that helps AI agents use the CLI effectively.

### Amp / Claude Code

Install the skill using [skills.sh](https://skills.sh):

```bash
npx skills add lox/notion-cli
```

Or manually add to your Amp/Claude config:

```bash
# Amp
amp skill add https://github.com/lox/notion-cli/tree/main/skills/notion-cli

# Claude Code
claude plugin marketplace add lox/notion-cli
claude plugin install notion-cli@notion-cli
```

View the skill at: [skills/notion/SKILL.md](skills/notion/SKILL.md)

## Links

- [Notion MCP Documentation](https://developers.notion.com/guides/mcp/mcp)
- [Notion API Reference](https://developers.notion.com/reference/intro)
- [Model Context Protocol](https://modelcontextprotocol.io/)

## License

MIT License - see [LICENSE](LICENSE) for details.

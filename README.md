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

# View a page with open comments inline by default
notion-cli page view "https://notion.so/My-Page-abc123"

# Hide page and block comments when you only want the body
notion-cli page view "Meeting Notes" --no-comments

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

# Official API fallback auth for features MCP cannot handle directly
notion-cli auth api setup     # Opens the internal integrations page, prompts for token, warns if format looks wrong
notion-cli auth api status
notion-cli auth api verify
notion-cli auth api unset
```

### Pages

```bash
notion-cli page list                           # List pages
notion-cli page list --limit 50                # Limit results
notion-cli page list --json                    # Output as JSON

notion-cli page view <page>                    # View page content with comments
notion-cli page view <page> --no-comments      # Hide page and block comments
notion-cli page view <page> --raw              # View raw Notion markup
notion-cli page view <page> --json             # Output as JSON

notion-cli page create --title "Title"         # Create a page
notion-cli page create --title "T" --content "Body text"
notion-cli page create --title "T" --parent <page-id>
notion-cli page archive <page-url-or-id>       # Archive a page via the official API

# Upload a markdown file as a new page
notion-cli page upload ./document.md                        # Title from # heading or filename
notion-cli page upload ./document.md --title "Custom Title" # Explicit title
notion-cli page upload ./document.md --parent "Engineering" # Parent by name or ID
notion-cli page upload ./document.md --parent-db <db-id>    # Upload as database entry
notion-cli page upload ./document.md --icon "📄"             # Set emoji icon
notion-cli page upload ./document.md                        # Uploads standalone local images when configured
notion-cli page upload ./document.md --skip-local-images    # Strips standalone local image lines instead

# Sync a markdown file (create or update)
notion-cli page sync ./document.md                          # Creates page, writes notion-id to frontmatter
notion-cli page sync ./document.md                          # Updates page using notion-id from frontmatter
notion-cli page sync ./document.md --parent "Engineering"   # Set parent on first sync
notion-cli page sync ./document.md --parent-db <db-id>      # Sync as database entry
notion-cli page sync ./document.md                          # Uploads standalone local images when configured
notion-cli page sync ./document.md --skip-local-images      # Strips standalone local image lines instead

# Edit an existing page
notion-cli page edit <page> --replace "New content"                      # Replace all content
notion-cli page edit <page> --replace "New content" --allow-deleting-content # Allow replacing pages with child content
notion-cli page edit <page> --find "old text" --replace-with "new text"  # Find and replace
notion-cli page edit <page> --find "section" --append "extra content"    # Append after match
notion-cli page edit <page> -P "Status=Done" -P "Priority=1"             # Update page properties
```

The `<page>` argument accepts a URL, ID, or page name.

`page archive` accepts a page URL or page ID and moves that page to trash via the official API. This requires an official API token configured through `auth api setup` or `NOTION_API_TOKEN`.

`page view` shows open page-level comments and inline block discussions by default. Inline discussions are rendered in context, with the anchor text wrapped in `[[...]]` and the discussion shown immediately below it. Use `--no-comments` to suppress comments, `--raw` to inspect the original Notion markup, and `--json` to return the page plus a `Comments` array.

`page upload` and `page sync` support native local image upload for standalone markdown image lines like `![Alt](./diagram.png)`. When local images are present, `notion-cli` uploads those files through the official Notion API and keeps them in document order. This requires an official API token configured through `auth api setup` or `NOTION_API_TOKEN`. Pass `--skip-local-images` to silently remove standalone local image lines instead of uploading them. Inline or mixed-content local image syntax is rejected instead of being guessed.

### Search

```bash
notion-cli search "query"                      # Search workspace
notion-cli search "query" --limit 10           # Limit results
notion-cli search "query" --json               # Output as JSON
```

### Databases

```bash
notion-cli db list                             # List databases
notion-cli db list -q "project"                # Filter by name
notion-cli db list --json                      # Output as JSON

notion-cli db query <database-id>              # Query database
notion-cli db query <id> --json                # Output as JSON

# Create an entry in a database
notion-cli db create <database> --title "Entry Title"
notion-cli db create <database> -t "Title" --prop "Status=Not started"
notion-cli db create <database> -t "Title" --prop "Status=Done" --prop "date:Due:start=2026-03-01"
notion-cli db create <database> -t "Title" --content "Body text"
notion-cli db create <database> -t "Title" --file ./notes.md
notion-cli db create <database> -t "Title" --json
```

The `<database>` argument accepts a URL, ID, or name. Date properties use the expanded key format: `date:<Property Name>:start`, `date:<Property Name>:end`.

### Comments

```bash
notion-cli comment list <page>                 # List open page and block comments
notion-cli comment list <page> --resolved      # Include resolved discussions too
notion-cli comment list <page> --json          # Output as JSON
notion-cli comment list "Meeting Notes"        # Resolve the page by name

notion-cli comment create <page> --content "Comment text"
notion-cli comment create https://notion.so/... --content "Looks good"
```

The comment commands accept a page URL, ID, or name. `comment list` includes both page-level and block-level discussions by default and only shows open discussions unless you pass `--resolved`.

### Other

```bash
notion-cli tools                               # List available MCP tools
notion-cli tools --json                        # Output tools as JSON
notion-cli version                             # Show version
notion-cli --version                           # Alias for version
notion-cli -v                                  # Short alias for version
notion-cli --help                              # Show help
```

## Configuration

The CLI uses Notion's remote MCP server with OAuth authentication. On first run, `notion-cli auth login` will open your browser to authorize the CLI with your Notion workspace.

**Note:** Access tokens expire after 1 hour. The CLI automatically refreshes tokens when they expire or are about to expire, so you typically don't need to think about this. Use `notion-cli auth refresh` to manually refresh if needed.

### Profiles

Every command accepts `--profile <name>` (or `NOTION_CLI_PROFILE`) to scope the OAuth token and official API config to a specific Notion account, so you can keep separate logins for `work`, `home`, etc.

```bash
# Log in to a named profile
notion-cli auth login --profile work

# Use the profile for a single command
notion-cli page list --profile work

# Pin a profile for the shell session
export NOTION_CLI_PROFILE=work

# Make a profile the default for future invocations
notion-cli auth use work
```

Profile resolution, highest priority first:

1. `--profile <name>` flag
2. `NOTION_CLI_PROFILE` environment variable
3. Active profile from `notion-cli auth use <name>`
4. Implicit default profile

The default profile keeps using the existing OAuth token path, so existing single-account installs need no migration. `notion-cli auth use <name>` stores the active profile in the cross-profile config directory.

Profile names may contain letters, numbers, at signs, dots, underscores, and hyphens.

Named profiles store their credentials under `~/.config/notion-cli/profiles/<profile>/{token,config}.json`.

`notion-cli auth status` prints the selected profile and token path, and `notion-cli auth list` shows all known profiles with OAuth and API-token status.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NOTION_ACCESS_TOKEN` | Access token for CI/headless usage (skips OAuth) |
| `NOTION_API_TOKEN` | Official Notion API token used for upload fallback and verification |
| `NOTION_API_BASE_URL` | Override the official Notion API base URL |
| `NOTION_API_NOTION_VERSION` | Override the official Notion API version |
| `NOTION_CLI_PROFILE` | Default profile when `--profile` is not passed |

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

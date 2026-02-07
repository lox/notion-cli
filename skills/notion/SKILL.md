---
name: notion
description: Manage Notion pages, databases, and comments from the command line. Search, view, create, and edit content in your Notion workspace.
allowed-tools: Bash(notion:*), Bash(notion-cli:*)
---

# Notion CLI

A CLI to manage Notion from the command line, using Notion's remote MCP server.

## Prerequisites

The `notion` command must be available on PATH. To check:

```bash
notion --version
```

If not installed:

```bash
go install github.com/lox/notion-cli@latest
```

Or see: https://github.com/lox/notion-cli

## Authentication

The CLI uses OAuth authentication. On first use, it opens a browser for authorization:

```bash
notion auth login      # Authenticate with Notion
notion auth status     # Check authentication status
notion auth logout     # Clear credentials
```

For CI/headless environments, set `NOTION_ACCESS_TOKEN` environment variable.

## Available Commands

```
notion auth            # Manage authentication
notion page            # Manage pages (list, view, create, upload, edit)
notion db              # Manage databases (list, query)
notion search          # Search the workspace
notion comment         # Manage comments (list, create)
notion tools           # List available MCP tools
```

## Common Operations

### Search

```bash
notion search "meeting notes"           # Search workspace
notion search "project" --limit 5       # Limit results
notion search "query" --json            # JSON output
```

### Pages

```bash
# List pages
notion page list
notion page list --limit 10
notion page list --json

# View a page (renders as markdown in terminal)
notion page view <url-or-id>
notion page view <url> --raw            # Show raw Notion markup
notion page view <url> --json           # JSON output

# Create a page
notion page create --title "New Page"
notion page create --title "Doc" --content "# Heading\n\nContent here"
notion page create --title "Child" --parent <parent-page-id>

# Upload a markdown file as a page
notion page upload ./document.md
notion page upload ./doc.md --title "Custom Title"
notion page upload ./doc.md --parent "Parent Page Name"

# Edit a page
notion page edit <url> --replace "New content"
notion page edit <url> --find "old text" --replace-with "new text"
notion page edit <url> --find "section" --append "additional content"
```

### Databases

```bash
notion db list                          # List databases
notion db list --json

notion db query <database-url-or-id>    # Query a database
notion db query <id> --json
```

### Comments

```bash
notion comment list <page-id>           # List comments on a page
notion comment list <page-id> --json

notion comment create <page-id> --content "Great work!"
```

## Output Formats

Most commands support `--json` for machine-readable output:

```bash
notion page list --json | jq '.[0].url'
notion search "api" --json | jq '.[] | .title'
```

## Tips for Agents

1. **Search first** - Use `notion search` to find pages before operating on them
2. **Use URLs or IDs** - Both work for page/database references
3. **Check --help** - Every command has detailed help: `notion page edit --help`
4. **Raw output** - Use `--raw` with `page view` to see the original Notion markup
5. **JSON for parsing** - Use `--json` when you need to extract specific fields

---
name: notion
description: Manage Notion pages, databases, and comments from the command line. Search, view, create, and edit content in your Notion workspace.
allowed-tools: Bash(notion-cli:*)
---

# Notion CLI

A CLI to manage Notion from the command line, using Notion's remote MCP server.

## Prerequisites

The `notion-cli` command must be available on PATH. To check:

```bash
notion-cli version
```

If not installed:

```bash
go install github.com/lox/notion-cli@latest
```

Or see: https://github.com/lox/notion-cli

## Authentication

The CLI uses OAuth authentication for MCP-backed commands. On first use, it opens a browser for authorization:

```bash
notion-cli auth login      # Authenticate with Notion
notion-cli auth status     # Show active profile and OAuth state (diagnostic)
notion-cli auth refresh    # Force-refresh; commands auto-refresh on use, so rarely needed
notion-cli auth logout     # Clear credentials
```

For CI/headless environments, set `NOTION_ACCESS_TOKEN` environment variable.

Some fallback features also use the official Notion API:

```bash
notion-cli auth api setup
notion-cli auth api status
notion-cli auth api verify
notion-cli auth api unset
```

For CI/headless environments, set `NOTION_API_TOKEN`.

This is a separate credential from the OAuth one above, and one does not stand
in for the other: the OAuth token authorizes the MCP server and is rejected by
`api.notion.com` with a 401. The official API expects an internal integration
secret, in the `ntn_…` form, created at
https://www.notion.so/profile/integrations/internal.

An internal integration only reaches the pages it is explicitly connected to,
through the page's `•••` menu under Connections, and the connection is inherited
by child pages. A valid token on an unconnected page fails with
`object_not_found`, so grant the connection at the highest parent the workflow
touches.

### Multiple accounts

Every command accepts `--profile <name>` (or `NOTION_CLI_PROFILE`) to target a specific Notion account. Named profiles keep credentials isolated under `~/.config/notion-cli/profiles/<profile>/`; the implicit default profile uses the existing top-level paths.

```bash
notion-cli auth login --profile work
notion-cli page list --profile work
export NOTION_CLI_PROFILE=work  # pin for the shell session
notion-cli auth use work         # make work the default profile
```

Resolution precedence: `--profile` > `NOTION_CLI_PROFILE` > `notion-cli auth use <name>` > implicit default profile.

## Available Commands

```
notion-cli auth            # Manage authentication
notion-cli page            # Manage pages (list, view, create, upload, edit)
notion-cli db              # Manage databases (list, query, create entries)
notion-cli search          # Search the workspace
notion-cli comment         # Manage comments (list, create)
notion-cli user             # List workspace users
notion-cli tools            # List available MCP tools
```

## Common Operations

### Search

```bash
notion-cli search "meeting notes"           # Search workspace
notion-cli search "project" --limit 5       # Limit results
notion-cli search "query" --json            # JSON output
```

### Pages

All page commands accept a **URL**, **name**, or **ID** to identify pages.

```bash
# List pages
notion-cli page list
notion-cli page list --limit 10
notion-cli page list --json

# View a page (renders as markdown in terminal)
notion-cli page view <page>
notion-cli page view <page> --no-comments    # Hide page and block comments
notion-cli page view <page> --raw            # Show raw Notion markup
notion-cli page view <page> --json           # JSON output
notion-cli page view "Meeting Notes"         # By name
notion-cli page view https://notion.so/...   # By URL

# Create a page
notion-cli page create --title "New Page"
notion-cli page create --title "Doc" --content "# Heading\n\nContent here"
notion-cli page create --title "Child" --parent "Engineering"   # Parent by name
notion-cli page create --title "Child" --parent <page-id>       # Parent by ID

# Upload a markdown file as a page
notion-cli page upload ./document.md
notion-cli page upload ./doc.md --title "Custom Title"
notion-cli page upload ./doc.md --parent "Parent Page Name"
notion-cli page upload ./doc.md --parent-db <db-id>         # Upload as database entry

# Sync a markdown file (create or update)
# First run creates the page and writes notion-id to the file's frontmatter.
# Subsequent runs update the page content using the stored notion-id.
notion-cli page sync ./document.md
notion-cli page sync ./document.md --parent "Engineering"   # Set parent on first sync
notion-cli page sync ./document.md --parent-db <db-id>      # Sync as database entry
notion-cli page sync ./document.md --title "Custom Title"

# Edit a page
notion-cli page edit <page> --replace "New content"
notion-cli page edit <page> --find "old text" --replace-with "new text"
notion-cli page edit <page> --find "section" --append "additional content"
notion-cli page edit <page> -P "Status=Done" -P "Priority=1"   # Update properties

# Move a page under a new parent
notion-cli page move <page> --parent "Engineering"      # Parent page by name, URL, or ID
notion-cli page move <page> --parent-db <db-id>         # Move into a database

# Duplicate a page
notion-cli page duplicate <page>
notion-cli page duplicate <page> --json

# Archive a page
notion-cli page archive https://notion.so/...
notion-cli page archive 12345678-abcd-ef12-3456-7890abcdef12
```

For `page upload` and `page sync`, standalone local markdown image lines like `![Alt](./diagram.png)` are uploaded natively through the official API when configured. Local images must appear on their own line. Inline or mixed-content local image syntax is rejected.

`page view` shows open page-level comments and inline block discussions by default. Inline discussions are rendered beside their anchor text, with the anchor wrapped in `[[...]]` and the discussion shown immediately below it. Use `--no-comments` when you only want the page body, `--raw` to inspect the original Notion markup, and `--json` when an agent needs the page plus the `Comments` array.

`page archive` uses the official API fallback path and requires `notion-cli auth api setup` or `NOTION_API_TOKEN`. The MCP server exposes no tool that trashes a page, so this is the only way to remove one; without the integration secret, the closest an MCP-only session gets is `page move`, gathering throwaway pages under a single parent for a human to delete.

### Edit mode guardrails

`page edit` supports these mutually exclusive modes:

1. `--replace "..."` for full-page replacement.
2. `--find "..." --replace-with "..."` for targeted replacement.
3. `--find "..." --append "..."` for append-after-match.
4. `-P key=value` (repeatable) for property updates, including the title.

When a targeted edit fails (for example MCP validation errors), fall back to full replacement by fetching content, editing locally, then applying `--replace`.

`page move` takes exactly one of `--parent` or `--parent-db`. `page duplicate` copies the page, its child pages, and its explicit permissions under the same parent, so it is the way to obtain a page that keeps a permission grant the parent does not hand down.

### Users

```bash
notion-cli user list                        # List workspace users (paginated)
notion-cli user list --query alice           # Filter by name or email
notion-cli user list --limit 20              # Cap the number of users returned
notion-cli user list --json                 # Output as JSON
notion-cli user me                          # Show the user the token authenticates as
```

`user list` follows the server cursor until the workspace is exhausted, so large
workspaces return every member rather than only the first page. Use `--query` to
let the server filter instead of paging through everything.

### Databases

All database commands accept a **URL**, **name**, or **ID** to identify databases.

```bash
# List databases
notion-cli db list                          # List databases
notion-cli db list -q "project"             # Filter by name
notion-cli db list --json

# Query a database
notion-cli db query <database-url-or-id>    # Query a database
notion-cli db query <id> --json

# Create an entry in a database
notion-cli db create <database> --title "Entry Title"
notion-cli db create <database> -t "Title" --prop "Status=Not started"
notion-cli db create <database> -t "Title" --prop "date:Due:start=2026-03-01"
notion-cli db create <database> -t "Title" --content "Body text"
notion-cli db create <database> -t "Title" --file ./notes.md    # Body from file
notion-cli db create <database> -t "Title" --json
```

**Property format:** Use `--prop Key=Value` for text/status properties. Date properties use expanded keys: `--prop "date:Date Field:start=2026-01-15"`.

### Comments

```bash
notion-cli comment list <page>              # List open page and block comments
notion-cli comment list <page> --resolved   # Include resolved discussions too
notion-cli comment list <page> --json
notion-cli comment list "Meeting Notes"     # Resolve a page by name

notion-cli comment create <page> --content "Great work!"
notion-cli comment create https://notion.so/... --content "Looks good"
```

The comment commands accept a page URL, ID, or name. `comment list` includes both page-level and block-level discussions by default and only shows open discussions unless `--resolved` is passed.

## Output Formats

Most commands support `--json` for machine-readable output:

```bash
notion-cli page list --json | jq '.[0].url'
notion-cli search "api" --json | jq '.[] | .title'
```

## Tips for Agents

1. **Search first** - Use `notion-cli search` to find pages before operating on them
2. **Use URLs, names, or IDs** - All page commands and comment commands resolve pages from any of these forms
3. **Explicit parent types** - Use `--parent` for page parents, `--parent-db` for database parents on `page sync`/`page upload`
4. **Query databases first** - Use `notion-cli db query <id>` to see the schema and property types before creating entries
5. **Check --help** - Every command has detailed help: `notion-cli page edit --help`
6. **Inline comments by default** - `page view` includes open page comments and inline block discussions unless `--no-comments` is set
7. **Raw output** - Use `--raw` with `page view` to see the original Notion markup
8. **JSON for parsing** - Use `--json` when you need to extract specific fields, including the `Comments` array from `page view`
9. **No auth preflight** - Just run the command; the CLI auto-refreshes tokens on use. `notion-cli auth status` and `notion-cli auth list` are diagnostic surfaces, not health gates - do not poll them as a sanity check before each call. Only run `notion-cli auth login` if a real command returns an authentication error.
10. **API fallback preflight** - Run `notion-cli auth api verify` before workflows that need local image upload
11. **Error handling** - If a targeted `page edit` call fails, rerun with `--replace` as a safe fallback

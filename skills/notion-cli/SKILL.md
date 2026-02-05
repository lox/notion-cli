---
name: notion-cli
description: Manage Notion workspaces from the command line using notion-cli. This skill allows creating, viewing, editing pages, searching content, querying databases, and managing comments in Notion.
allowed-tools: Bash(notion-cli:*)
---

# Notion CLI

A command-line interface for Notion that uses the Model Context Protocol (MCP) to interact with Notion workspaces. Provides OAuth-based authentication and comprehensive workspace management capabilities directly from the terminal.

## Prerequisites

The `notion-cli` command must be available on PATH. To check:

```bash
notion-cli version
```

If not installed, follow the instructions at:
<https://github.com/lox/notion-cli#installation>

## Quick Start

```bash
# Authenticate with Notion (opens browser for OAuth)
notion-cli auth login

# Check authentication status
notion-cli auth status

# Search your workspace
notion-cli search "meeting notes"

# View a page
notion-cli page view "https://notion.so/My-Page-abc123"

# List your pages
notion-cli page list

# Create a page
notion-cli page create --title "New Page" --content "# Hello World"
```

## Authentication

### Initial Setup

Before using the CLI, authenticate with Notion:

```bash
notion-cli auth login
```

This will:

1. Open your browser to Notion's authorization page
2. Start a local callback server to receive the OAuth code
3. Exchange the code for access and refresh tokens
4. Store tokens at `~/.config/notion-cli/token.json`

### Token Management

Access tokens expire after **1 hour**, but the CLI automatically refreshes them when needed. You typically don't need to manually refresh tokens.

```bash
# Check authentication status
notion-cli auth status

# Manually refresh token (rarely needed)
notion-cli auth refresh

# Clear credentials (logout)
notion-cli auth logout
```

### Environment Variables

For CI/headless usage, set the access token directly:

```bash
export NOTION_ACCESS_TOKEN="your-token-here"
```

This skips the OAuth flow and uses the provided token directly.

### Troubleshooting Authentication

If you encounter authentication errors:

1. Run `notion-cli auth status` to check current status
2. If expired or invalid, run `notion-cli auth login` again
3. For persistent issues, try `notion-cli auth logout` followed by `notion-cli auth login`

## Available Commands

### Authentication Commands

#### `notion-cli auth login`

Authenticate with Notion via OAuth flow.

```bash
notion-cli auth login
```

#### `notion-cli auth status`

Show current authentication status.

```bash
# Human-readable output
notion-cli auth status

# JSON output for parsing
notion-cli auth status --json
```

#### `notion-cli auth refresh`

Manually refresh the access token.

```bash
notion-cli auth refresh
```

#### `notion-cli auth logout`

Clear stored credentials.

```bash
notion-cli auth logout
```

### Page Commands

#### `notion-cli page list`

List pages in your workspace.

```bash
# List first 20 pages
notion-cli page list

# Filter with query
notion-cli page list --query "project"
notion-cli page list -q "meeting notes"

# Increase limit
notion-cli page list --limit 50
notion-cli page list -l 100

# JSON output
notion-cli page list --json
```

**Output columns**: ID, Title, Last Edited, URL

#### `notion-cli page view <url-or-id>`

View page content as formatted markdown.

```bash
# View by URL
notion-cli page view "https://notion.so/My-Page-abc123"

# View by page ID
notion-cli page view "abc123def456"

# JSON output
notion-cli page view "abc123" --json
```

**Tips**:

- Accepts both full Notion URLs and raw page IDs
- Renders markdown beautifully in the terminal
- Use `--json` to get structured data for parsing

#### `notion-cli page create`

Create a new page with markdown content.

```bash
# Basic page creation
notion-cli page create --title "Daily Notes"

# With content
notion-cli page create --title "Todo List" --content "# Tasks\n- Item 1\n- Item 2"
notion-cli page create -t "Notes" -c "# Meeting Notes\n\nDiscussed project timeline."

# With parent page
notion-cli page create --title "Subpage" --parent "parent-page-id"

# JSON output
notion-cli page create --title "Page" --content "# Content" --json
```

**Notes**:

- Content uses **Notion-flavored markdown**
- Parent can be a page ID
- Returns the URL of the created page
- Use `\n` for line breaks in content

#### `notion-cli page upload <file>`

Upload a markdown file as a Notion page.

```bash
# Upload markdown file
notion-cli page upload notes.md

# Specify title (overrides file title)
notion-cli page upload notes.md --title "Custom Title"
notion-cli page upload notes.md -t "My Notes"

# Specify parent by name or ID
notion-cli page upload notes.md --parent "Projects"
notion-cli page upload notes.md -p "parent-page-id"

# Add emoji icon
notion-cli page upload notes.md --icon "📝"
notion-cli page upload notes.md -i "🎯"

# JSON output
notion-cli page upload notes.md --json
```

**Smart Features**:

- Auto-extracts title from first `# heading` or filename
- Auto-extracts emoji icon from title (e.g., "📝 Notes" → icon: 📝, title: Notes)
- Resolves parent by searching workspace for matching page name
- Full markdown file support

**Best Practice**: Use `page upload` for markdown files, `page create` for inline content.

#### `notion-cli page edit <page-url-or-id>`

Edit existing page content.

```bash
# Replace entire content
notion-cli page edit "abc123" --replace "# New Content\n\nCompletely new text."

# Find and replace text
notion-cli page edit "abc123" --find "old text" --replace-with "new text"

# Insert after specific text
notion-cli page edit "abc123" --find "## Section Header" --append "\n\nNew paragraph here."
```

**Edit Operations**:

- `--replace <text>`: Replace entire page content
- `--find <text> --replace-with <text>`: Find and replace specific text
- `--find <text> --append <text>`: Insert content after found text

**Notes**:

- Use `...` for ellipsis in text selections
- Cannot perform fine-grained block-level editing
- Content is replaced at the text level

### Search Command

#### `notion-cli search <query>`

Search across your entire workspace.

```bash
# Basic search
notion-cli search "meeting notes"

# Limit results
notion-cli search "project" --limit 10
notion-cli search "todo" -l 5

# JSON output
notion-cli search "docs" --json
```

**Features**:

- Searches pages, databases, and other objects
- Supports semantic search across connected apps
- Shows result type with emoji (📄 Page, 🗂️ Database, etc.)

**Output columns**: Type, ID, Title, URL

### Database Commands

#### `notion-cli db list`

List databases in your workspace.

```bash
# List all databases
notion-cli db list

# Filter with query
notion-cli db list --query "roadmap"
notion-cli db list -q "tasks"

# Limit results
notion-cli db list --limit 10

# JSON output
notion-cli db list --json
```

**Output columns**: ID, Title, Description, URL

#### `notion-cli db query <database-id-or-url>`

Query database content as markdown.

```bash
# Query by database ID
notion-cli db query "database-id-here"

# Query by URL
notion-cli db query "https://notion.so/Database-abc123"

# JSON output
notion-cli db query "database-id" --json
```

**Notes**:

- Fetches database rows and properties
- Renders as formatted markdown in terminal
- Use `--json` for structured data

### Comment Commands

#### `notion-cli comment list <page-id-or-url>`

List comments on a page.

```bash
# List by page ID
notion-cli comment list "page-id-here"

# List by URL
notion-cli comment list "https://notion.so/Page-abc123"

# JSON output
notion-cli comment list "page-id" --json
```

**Output columns**: Author ID, Created Time, Content

#### `notion-cli comment create <page-id-or-url>`

Create a comment on a page.

```bash
# Add comment
notion-cli comment create "page-id" --content "Looks good!"
notion-cli comment create "page-id" -c "Please review section 2."

# JSON output
notion-cli comment create "page-id" --content "Comment text" --json
```

### Utility Commands

#### `notion-cli tools`

List available MCP tools from the Notion server.

```bash
notion-cli tools
```

Useful for debugging and understanding what operations the MCP server supports.

#### `notion-cli version`

Show CLI version.

```bash
notion-cli version
```

## Common Workflows for AI Agents

### 1. Check Authentication First

Always verify authentication before performing operations:

```bash
# Check if authenticated
notion-cli auth status

# If not authenticated or expired, login
notion-cli auth login
```

### 2. Search Before Creating

Search to avoid duplicates and find existing content:

```bash
# Search for existing pages
notion-cli search "project plan" --limit 5

# Then create if needed
notion-cli page create --title "Project Plan Q1" --content "# Plan\n..."
```

### 3. Create Documentation Pages

For creating pages from markdown content:

```bash
# From inline content (short)
notion-cli page create --title "Quick Note" --content "# Note\n\nContent here."

# From file (long documents)
notion-cli page upload documentation.md --parent "Docs" --icon "📚"
```

### 4. Update Existing Content

Find and update specific sections:

```bash
# Get page ID from search
PAGE_ID=$(notion-cli search "Weekly Report" --json | jq -r '.results[0].id')

# Update content
notion-cli page edit "$PAGE_ID" --find "## Status" --append "\n\n### New Updates\n- Progress made"
```

### 5. Query Databases for Information

```bash
# List databases to find the right one
notion-cli db list --query "tasks"

# Query database content
notion-cli db query "database-id" --json | jq '.properties'
```

### 6. Organize with Parent Pages

```bash
# Create parent page
PARENT_URL=$(notion-cli page create --title "Projects" --json | jq -r '.url')

# Extract ID from URL (or use returned ID)
PARENT_ID=$(echo "$PARENT_URL" | grep -o '[a-f0-9]\{32\}')

# Create child pages
notion-cli page create --title "Project A" --parent "$PARENT_ID"
notion-cli page create --title "Project B" --parent "$PARENT_ID"
```

### 7. Bulk Operations with JSON Output

```bash
# Get all pages and process
notion-cli page list --limit 100 --json | jq -r '.pages[] | "\(.id): \(.title)"'

# Search and extract URLs
notion-cli search "meeting" --json | jq -r '.results[] | .url'
```

## Output Formats

### Default Table Format

Most commands output a formatted table:

```
ID           Title          Last Edited      URL
abc123...    My Page       2 days ago       https://notion.so/...
def456...    Another Page  1 week ago       https://notion.so/...
```

### JSON Format

Use `--json` or `-j` flag for structured output:

```bash
notion-cli page list --json
```

```json
{
  "pages": [
    {
      "id": "abc123...",
      "title": "My Page",
      "last_edited": "2026-02-03T10:30:00Z",
      "url": "https://notion.so/..."
    }
  ]
}
```

### Markdown Rendering

Page content is rendered as formatted markdown:

```bash
notion-cli page view "page-id"
```

Output is styled with:

- Syntax highlighting for code blocks
- Formatted headers, lists, quotes
- Color-coded callouts (ℹ️, ⚠️, 💡)
- Terminal-aware width (max 120 chars)

## Best Practices

### For AI Agents

1. **Always check authentication first**: Run `notion-cli auth status` before operations
2. **Use `--json` for parsing**: Structured output is easier to parse than tables
3. **Search before creating**: Avoid duplicates by searching first
4. **Prefer full URLs over IDs**: More readable and explicit
5. **Use `page upload` for markdown files**: Better than inline content for long documents
6. **Handle errors gracefully**: Check command exit codes and parse error messages
7. **Use parent names**: The CLI can resolve parent pages by name (searches workspace)
8. **Batch with JSON**: Process multiple items by piping JSON through jq
9. **Store page IDs**: Save IDs from create operations for later reference
10. **Respect token lifecycle**: Tokens auto-refresh, but verify auth on errors

### Content Formatting

- Use **Notion-flavored markdown** for content
- Include `\n` for line breaks in CLI arguments
- Quote multi-line content properly in shell commands
- Use emoji in titles (automatically extracted as icons)
- Callouts: Use standard emoji at start of paragraphs (ℹ️, ⚠️, 💡, etc.)

### Error Handling

```bash
# Check exit code
if ! notion-cli auth status &> /dev/null; then
    echo "Not authenticated. Please run: notion-cli auth login"
    exit 1
fi

# Parse JSON for errors
RESULT=$(notion-cli page create --title "Test" --json)
if echo "$RESULT" | jq -e '.error' &> /dev/null; then
    echo "Error: $(echo "$RESULT" | jq -r '.error.message')"
    exit 1
fi
```

## Limitations

1. **Block-level editing**: Cannot manipulate individual blocks, only replace content
2. **No file attachments**: Cannot upload images/PDFs directly (markdown only)
3. **No database creation**: Can only query existing databases
4. **No permission management**: Cannot modify sharing or access permissions
5. **Single workspace**: Uses the authenticated user's default workspace
6. **Browser required**: Initial OAuth login requires browser (unless using env var)
7. **Token expiry**: Access tokens expire after 1 hour (but auto-refresh)

## Troubleshooting

### Authentication Issues

**Problem**: "authentication required" error

**Solution**:

```bash
notion-cli auth logout
notion-cli auth login
```

### Page Not Found

**Problem**: Cannot find page by ID

**Solution**:

- Verify the page exists: `notion-cli search "page title"`
- Check permissions: ensure you have access to the page
- Use full URL instead of just ID

### Command Not Found

**Problem**: `notion: command not found`

**Solution**:

1. Verify installation: Check if `notion` is in PATH
2. Reinstall if needed: Follow installation guide
3. Check shell configuration: Ensure PATH includes installation directory

### Markdown Not Rendering

**Problem**: Page content shows raw markdown

**Solution**:

- This is expected for `--json` output
- Remove `--json` flag for rendered markdown
- Check terminal supports formatting (TTY detection)

## Advanced Usage

### Piping Content from Files

```bash
# Create page from file content
CONTENT=$(cat documentation.md)
notion-cli page create --title "Documentation" --content "$CONTENT"

# Or use upload directly
notion-cli page upload documentation.md
```

### Combining with Other Tools

```bash
# Create page from command output
notion-cli page create --title "System Info" --content "$(uname -a)"

# Search and open in browser (using jq)
URL=$(notion-cli search "project" --json | jq -r '.results[0].url')
open "$URL"

# Create daily note
DATE=$(date +%Y-%m-%d)
notion-cli page create --title "Daily Note $DATE" --content "# $DATE\n\n## Tasks\n\n## Notes"
```

### Scripting Examples

```bash
#!/bin/bash
# Create weekly report

# Check auth
if ! notion-cli auth status &> /dev/null; then
    echo "Please authenticate first: notion-cli auth login"
    exit 1
fi

# Create report
WEEK=$(date +%Y-W%V)
TITLE="Weekly Report $WEEK"

# Check if already exists
EXISTING=$(notion-cli search "$TITLE" --json | jq -r '.results | length')
if [ "$EXISTING" -gt 0 ]; then
    echo "Report already exists"
    exit 0
fi

# Create new report
CONTENT="# $TITLE\n\n## Accomplishments\n\n## Challenges\n\n## Next Week"
notion-cli page create --title "$TITLE" --content "$CONTENT"
echo "Created report: $TITLE"
```

## How It Works

The CLI connects to **Notion's remote MCP server** at `https://mcp.notion.com/mcp` using the Model Context Protocol. This provides:

- **OAuth authentication** - No API tokens to manage manually
- **Notion-flavored Markdown** - Create/edit content naturally
- **Semantic search** - Search across connected apps too
- **Optimized for CLI** - Efficient responses designed for terminal use

### MCP Server Tools

The CLI internally uses these Notion MCP server tools:

- `notion-search`: Search workspace
- `notion-fetch`: Fetch page/database content
- `notion-create-pages`: Create new pages
- `notion-update-page`: Update page content
- `notion-get-comments`: List comments
- `notion-create-comment`: Create comments

## Resources

- [Notion MCP Documentation](https://developers.notion.com/guides/mcp/mcp)
- [Notion API Reference](https://developers.notion.com/reference/intro)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [notion-cli Repository](https://github.com/lox/notion-cli)

## Examples by Use Case

### Knowledge Base Management

```bash
# Create main knowledge base page
KB_ID=$(notion-cli page create --title "📚 Knowledge Base" --json | jq -r '.id')

# Add category pages
notion-cli page create --title "Engineering" --parent "$KB_ID"
notion-cli page create --title "Product" --parent "$KB_ID"
notion-cli page create --title "Design" --parent "$KB_ID"
```

### Daily Notes Automation

```bash
# Create today's note
TODAY=$(date +%Y-%m-%d)
notion-cli page create --title "📅 $TODAY" \
  --content "# Daily Note - $TODAY\n\n## Tasks\n- [ ] \n\n## Notes\n\n## Reflections"
```

### Documentation Sync

```bash
# Upload multiple markdown files
for file in docs/*.md; do
    echo "Uploading $file..."
    notion-cli page upload "$file" --parent "Documentation" --icon "📖"
done
```

### Meeting Notes

```bash
# Create meeting note with template
MEETING_TITLE="Team Sync $(date +%Y-%m-%d)"
CONTENT="# $MEETING_TITLE\n\n## Attendees\n\n## Agenda\n\n## Notes\n\n## Action Items"
notion-cli page create --title "$MEETING_TITLE" --content "$CONTENT"
```

### Project Status Updates

```bash
# Find project page
PROJECT_ID=$(notion-cli search "Q1 Project" --json | jq -r '.results[0].id')

# Add status update
STATUS="## Status Update $(date +%Y-%m-%d)\n\n- Progress: On track\n- Blockers: None"
notion-cli page edit "$PROJECT_ID" --find "## Updates" --append "\n\n$STATUS"
```

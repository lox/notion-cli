package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type PageCmd struct {
	List   PageListCmd   `cmd:"" help:"List pages"`
	View   PageViewCmd   `cmd:"" help:"View a page"`
	Create PageCreateCmd `cmd:"" help:"Create a page"`
	Upload PageUploadCmd `cmd:"" help:"Upload a markdown file as a page"`
}

type PageListCmd struct {
	Query string `help:"Filter pages by name" short:"q"`
	Limit int    `help:"Maximum number of results" short:"l" default:"20"`
	JSON  bool   `help:"Output as JSON" short:"j"`
}

func (c *PageListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageList(ctx, c.Query, c.Limit)
}

func runPageList(ctx *Context, query string, limit int) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bgCtx := context.Background()

	searchQuery := query
	if searchQuery == "" {
		searchQuery = "*"
	}

	resp, err := client.Search(bgCtx, searchQuery)
	if err != nil {
		output.PrintError(err)
		return err
	}

	pages := filterPages(resp.Results, limit)
	return output.PrintPages(pages, ctx.JSON)
}

func filterPages(results []mcp.SearchResult, limit int) []output.Page {
	pages := make([]output.Page, 0)
	for _, r := range results {
		if r.ObjectType != "page" && r.Object != "page" {
			continue
		}
		if limit > 0 && len(pages) >= limit {
			break
		}
		pages = append(pages, output.Page{
			ID:    r.ID,
			Title: r.Title,
			URL:   r.URL,
		})
	}
	return pages
}

type PageViewCmd struct {
	URL  string `arg:"" help:"Page URL or ID"`
	JSON bool   `help:"Output as JSON" short:"j"`
}

func (c *PageViewCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageView(ctx, c.URL)
}

func runPageView(ctx *Context, url string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bgCtx := context.Background()
	result, err := client.Fetch(bgCtx, url)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if result.Content == "" {
		output.PrintWarning("No content found")
		return nil
	}

	return output.RenderMarkdown(result.Content)
}

type PageCreateCmd struct {
	Title   string `help:"Page title" short:"t" required:""`
	Parent  string `help:"Parent page ID" short:"p"`
	Content string `help:"Page content (markdown)" short:"c"`
	JSON    bool   `help:"Output as JSON" short:"j"`
}

func (c *PageCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageCreate(ctx, c.Title, c.Parent, c.Content)
}

func runPageCreate(ctx *Context, title, parent, content string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bgCtx := context.Background()
	req := mcp.CreatePageRequest{
		Title:        title,
		ParentPageID: parent,
		Content:      content,
	}

	resp, err := client.CreatePage(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		outPage := output.Page{
			ID:    resp.ID,
			URL:   resp.URL,
			Title: title,
		}
		return output.PrintPage(outPage, true)
	}

	if resp.URL != "" {
		output.PrintSuccess("Page created: " + resp.URL)
	} else {
		output.PrintSuccess("Page created")
	}
	return nil
}

type PageUploadCmd struct {
	File   string `arg:"" help:"Markdown file to upload" type:"existingfile"`
	Title  string `help:"Page title (default: filename or first heading)" short:"t"`
	Parent string `help:"Parent page name or ID" short:"p"`
	Icon   string `help:"Emoji icon for the page" short:"i"`
	JSON   bool   `help:"Output as JSON" short:"j"`
}

func (c *PageUploadCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageUpload(ctx, c.File, c.Title, c.Parent, c.Icon)
}

func runPageUpload(ctx *Context, file, title, parent, icon string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		output.PrintError(err)
		return err
	}

	markdown := string(content)

	if title == "" {
		title = extractTitleFromMarkdown(markdown)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}

	if icon == "" {
		icon, title = extractEmojiFromTitle(title)
	}

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	parentID := parent
	if parent != "" && !looksLikeID(parent) {
		resolved, err := resolvePageByName(client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		parentID = resolved
	}

	bgCtx := context.Background()
	req := mcp.CreatePageRequest{
		Title:        title,
		ParentPageID: parentID,
		Content:      markdown,
	}

	resp, err := client.CreatePage(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	displayTitle := title
	if icon != "" {
		displayTitle = icon + " " + title
	}

	if ctx.JSON {
		outPage := output.Page{
			ID:    resp.ID,
			URL:   resp.URL,
			Title: displayTitle,
			Icon:  icon,
		}
		return output.PrintPage(outPage, true)
	}

	output.PrintSuccess("Uploaded: " + displayTitle)
	if resp.URL != "" {
		output.PrintInfo(resp.URL)
	}
	return nil
}

func extractTitleFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func extractEmojiFromTitle(title string) (icon, cleanTitle string) {
	runes := []rune(title)
	if len(runes) == 0 {
		return "", title
	}

	first := runes[0]
	if isEmoji(first) {
		rest := strings.TrimSpace(string(runes[1:]))
		return string(first), rest
	}

	return "", title
}

func isEmoji(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) && !unicode.IsPunct(r) && r > 127
}

func looksLikeID(s string) bool {
	if len(s) == 32 || len(s) == 36 {
		for _, c := range s {
			if !unicode.IsDigit(c) && (c < 'a' || c > 'f') && c != '-' {
				return false
			}
		}
		return true
	}
	return false
}

func resolvePageByName(client *mcp.Client, name string) (string, error) {
	bgCtx := context.Background()
	resp, err := client.Search(bgCtx, name)
	if err != nil {
		return "", err
	}

	for _, r := range resp.Results {
		if r.ObjectType == "page" || r.Object == "page" {
			if strings.EqualFold(r.Title, name) {
				return r.ID, nil
			}
		}
	}

	for _, r := range resp.Results {
		if r.ObjectType == "page" || r.Object == "page" {
			if strings.Contains(strings.ToLower(r.Title), strings.ToLower(name)) {
				return r.ID, nil
			}
		}
	}

	return "", &output.UserError{Message: "page not found: " + name}
}

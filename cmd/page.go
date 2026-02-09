package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type PageCmd struct {
	List   PageListCmd   `cmd:"" help:"List pages"`
	View   PageViewCmd   `cmd:"" help:"View a page"`
	Create PageCreateCmd `cmd:"" help:"Create a page"`
	Upload PageUploadCmd `cmd:"" help:"Upload a markdown file as a page"`
	Edit   PageEditCmd   `cmd:"" help:"Edit a page"`
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
	defer func() { _ = client.Close() }()

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
	Page string `arg:"" help:"Page URL, name, or ID"`
	JSON bool   `help:"Output as JSON" short:"j"`
	Raw  bool   `help:"Output raw Notion response without formatting" short:"r"`
}

func (c *PageViewCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageView(ctx, c.Page, c.Raw)
}

func runPageView(ctx *Context, page string, raw bool) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	fetchID := page
	if ref.Kind == cli.RefName {
		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		fetchID = resolved
	}

	result, err := client.Fetch(bgCtx, fetchID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if result.Content == "" {
		output.PrintWarning("No content found")
		return nil
	}

	if raw {
		fmt.Println(result.Content)
		return nil
	}

	return output.RenderPage(result.Content)
}

type PageCreateCmd struct {
	Title   string `help:"Page title" short:"t" required:""`
	Parent  string `help:"Parent page URL, name, or ID" short:"p"`
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
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	parentID := parent
	if parent != "" {
		resolved, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		parentID = resolved
	}

	req := mcp.CreatePageRequest{
		Title:        title,
		ParentPageID: parentID,
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
	Parent string `help:"Parent page URL, name, or ID" short:"p"`
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
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	parentID := parent
	if parent != "" {
		resolved, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		parentID = resolved
	}

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
	if cli.IsEmoji(first) {
		rest := strings.TrimSpace(string(runes[1:]))
		return string(first), rest
	}

	return "", title
}

type PageEditCmd struct {
	Page      string `arg:"" help:"Page URL, name, or ID"`
	Replace   string `help:"Replace entire content with this text" xor:"action"`
	Find      string `help:"Text to find (use ... for ellipsis)" xor:"action"`
	ReplaceWith string `help:"Text to replace with (requires --find)" name:"replace-with"`
	Append    string `help:"Append text after selection (requires --find)" xor:"action"`
}

func (c *PageEditCmd) Run(ctx *Context) error {
	return runPageEdit(ctx, c.Page, c.Replace, c.Find, c.ReplaceWith, c.Append)
}

func runPageEdit(ctx *Context, page, replace, find, replaceWith, appendText string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	pageID := page
	if ref.Kind == cli.RefName {
		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		pageID = resolved
	} else if ref.Kind == cli.RefID {
		pageID = ref.ID
	}

	var req mcp.UpdatePageRequest
	req.PageID = pageID

	switch {
	case replace != "":
		req.Command = "replace_content"
		req.NewContent = replace
	case find != "" && replaceWith != "":
		req.Command = "replace_content_range"
		req.Selection = find
		req.NewStr = replaceWith
	case find != "" && appendText != "":
		req.Command = "insert_content_after"
		req.Selection = find
		req.NewStr = appendText
	default:
		return &output.UserError{Message: "specify --replace, or --find with --replace-with or --append"}
	}

	if err := client.UpdatePage(bgCtx, req); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Page updated")
	return nil
}

package cmd

import (
	"context"
	"encoding/json"
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
	Sync   PageSyncCmd   `cmd:"" help:"Sync a markdown file to a page (create or update)"`
	Edit   PageEditCmd   `cmd:"" help:"Edit a page"`
}

var loadPageViewCommentsFn = loadPageViewComments
var printViewedPageFn = output.PrintViewedPage
var printWarningFn = output.PrintWarning

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

	resp, err := client.Search(bgCtx, query, &mcp.SearchOptions{ContentSearchMode: "workspace_search"})
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
		if r.ObjectType != "page" && r.Object != "page" && r.Type != "page" {
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
	Page     string `arg:"" help:"Page URL, name, or ID"`
	Comments bool   `help:"Show open page and block comments" default:"true" negatable:""`
	JSON     bool   `help:"Output as JSON" short:"j"`
	Raw      bool   `help:"Output raw Notion response without formatting" short:"r"`
}

func (c *PageViewCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageView(ctx, c.Page, c.Raw, c.Comments)
}

func runPageView(ctx *Context, page string, raw, includeComments bool) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	fetchID, err := resolveFetchID(bgCtx, page, ref, client, cli.ResolvePageID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	fetchPage := client.Fetch
	if shouldLoadPageViewComments(raw, includeComments, ctx.JSON) {
		fetchPage = client.FetchWithDiscussions
	}

	result, err := fetchPage(bgCtx, fetchID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	return renderFetchedPageView(bgCtx, ctx, client, fetchID, result, raw, includeComments)
}

func renderFetchedPageView(bgCtx context.Context, ctx *Context, client *mcp.Client, fetchID string, result *mcp.FetchResult, raw, includeComments bool) error {
	comments, err := loadPageViewCommentsFn(bgCtx, client, fetchID, result.Content, raw, includeComments, ctx.JSON)
	if err != nil {
		if !ctx.JSON {
			printWarningFn("Unable to load comments: " + err.Error())
		}
		comments = nil
	}

	pageOutput := output.Page{
		ID:      fetchID,
		Title:   result.Title,
		URL:     result.URL,
		Content: result.Content,
	}

	if ctx.JSON {
		return printViewedPageFn(pageOutput, comments, true)
	}

	if raw {
		fmt.Println(result.Content)
		return nil
	}

	if result.Content == "" {
		printWarningFn("No content found")
		if len(comments) == 0 {
			return nil
		}
		fmt.Println()
	}

	return printViewedPageFn(pageOutput, comments, false)
}

func loadPageViewComments(ctx context.Context, client *mcp.Client, pageID, pageContent string, raw, includeComments, asJSON bool) ([]output.Comment, error) {
	if !shouldLoadPageViewComments(raw, includeComments, asJSON) {
		return nil, nil
	}

	mcpComments, err := loadAllComments(ctx, client, buildCommentListRequest(pageID, false))
	if err != nil {
		return nil, err
	}

	comments := convertComments(mcpComments)
	hydrateCommentContextsFromPageContent(pageContent, comments)
	hydrateCommentAuthors(ctx, client, comments)
	return comments, nil
}

func shouldLoadPageViewComments(raw, includeComments, asJSON bool) bool {
	return includeComments && (!raw || asJSON)
}

type pageIDResolver func(context.Context, *mcp.Client, string) (string, error)

func resolveFetchID(ctx context.Context, page string, ref cli.PageRef, client *mcp.Client, resolve pageIDResolver) (string, error) {
	switch ref.Kind {
	case cli.RefID:
		return ref.ID, nil
	case cli.RefName:
		if resolve == nil {
			return "", fmt.Errorf("resolver not configured for page name input")
		}
		return resolve(ctx, client, page)
	default:
		return page, nil
	}
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
	File     string `arg:"" help:"Markdown file to upload" type:"existingfile"`
	Title    string `help:"Page title (default: filename or first heading)" short:"t"`
	Parent   string `help:"Parent page URL, name, or ID" short:"p"`
	ParentDB string `help:"Parent database URL, name, or ID" name:"parent-db" short:"d"`
	Icon     string `help:"Emoji icon for the page" short:"i"`
	JSON     bool   `help:"Output as JSON" short:"j"`
}

func (c *PageUploadCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageUpload(ctx, c.File, c.Title, c.Parent, c.ParentDB, c.Icon)
}

func runPageUpload(ctx *Context, file, title, parent, parentDB, icon string) error {
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

	req := mcp.CreatePageRequest{
		Title:   title,
		Content: markdown,
	}

	if parentDB != "" {
		dbID, err := cli.ResolveDatabaseID(bgCtx, client, parentDB)
		if err != nil {
			output.PrintError(err)
			return err
		}
		dbID, err = client.ResolveDataSourceID(bgCtx, dbID)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentDatabaseID = dbID
	} else if parent != "" {
		parentID, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentPageID = parentID
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
	Page                 string   `arg:"" help:"Page URL, name, or ID"`
	Replace              string   `help:"Replace entire content with this text"`
	Find                 string   `help:"Text to find (use ... for ellipsis)"`
	ReplaceWith          string   `help:"Text to replace with (requires --find)" name:"replace-with"`
	Append               string   `help:"Append text after selection (requires --find)"`
	Prop                 []string `help:"Set page properties (key=value, repeatable)" short:"P"`
	AllowDeletingContent bool     `help:"Allow deleting child pages/databases when replacing content" name:"allow-deleting-content"`
}

func (c *PageEditCmd) Run(ctx *Context) error {
	return runPageEdit(ctx, c.Page, c.Replace, c.Find, c.ReplaceWith, c.Append, c.Prop, c.AllowDeletingContent)
}

func runPageEdit(ctx *Context, page, replace, find, replaceWith, appendText string, props []string, allowDeletingContent bool) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	pageID := page
	switch ref.Kind {
	case cli.RefName:
		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		pageID = resolved
	case cli.RefID:
		pageID = ref.ID
	}

	req, err := buildPageEditRequest(replace, find, replaceWith, appendText, props, allowDeletingContent)
	if err != nil {
		output.PrintError(err)
		return err
	}
	req.PageID = pageID

	if err := client.UpdatePage(bgCtx, req); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Page updated")
	return nil
}

func buildPageEditRequest(replace, find, replaceWith, appendText string, props []string, allowDeletingContent bool) (mcp.UpdatePageRequest, error) {
	if allowDeletingContent && replace == "" {
		return mcp.UpdatePageRequest{}, &output.UserError{Message: "--allow-deleting-content requires --replace"}
	}

	if len(props) > 0 {
		if allowDeletingContent {
			return mcp.UpdatePageRequest{}, &output.UserError{Message: "--allow-deleting-content requires --replace"}
		}
		if replace != "" || find != "" || replaceWith != "" || appendText != "" {
			return mcp.UpdatePageRequest{}, &output.UserError{Message: "--prop cannot be combined with --replace, --find, --replace-with, or --append"}
		}

		properties, err := parsePageEditProperties(props)
		if err != nil {
			return mcp.UpdatePageRequest{}, err
		}

		return mcp.UpdatePageRequest{
			Command:    "update_properties",
			Properties: properties,
		}, nil
	}

	if replace == "" && find == "" && replaceWith == "" && appendText == "" {
		return mcp.UpdatePageRequest{}, &output.UserError{Message: "specify --replace, --prop, or --find with --replace-with or --append"}
	}

	if replace != "" {
		if find != "" || replaceWith != "" || appendText != "" {
			return mcp.UpdatePageRequest{}, &output.UserError{Message: "--replace cannot be combined with --find, --replace-with, or --append"}
		}
		return mcp.UpdatePageRequest{
			Command:              "replace_content",
			NewContent:           replace,
			AllowDeletingContent: allowDeletingContent,
		}, nil
	}

	if find == "" {
		if replaceWith != "" || appendText != "" {
			return mcp.UpdatePageRequest{}, &output.UserError{Message: "--replace-with and --append require --find"}
		}
		return mcp.UpdatePageRequest{}, &output.UserError{Message: "specify --replace, --prop, or --find with --replace-with or --append"}
	}

	hasReplace := replaceWith != ""
	hasAppend := appendText != ""
	if hasReplace == hasAppend {
		return mcp.UpdatePageRequest{}, &output.UserError{Message: "with --find, specify exactly one of --replace-with or --append"}
	}

	if hasAppend {
		return mcp.UpdatePageRequest{
			Command:   "insert_content_after",
			Selection: find,
			NewStr:    appendText,
		}, nil
	}

	return mcp.UpdatePageRequest{
		Command: "update_content",
		ContentUpdates: []mcp.ContentUpdate{
			{OldStr: find, NewStr: replaceWith},
		},
	}, nil
}

func parsePageEditProperties(props []string) (map[string]any, error) {
	properties := make(map[string]any, len(props))
	for _, p := range props {
		k, v, ok := strings.Cut(p, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, &output.UserError{Message: "invalid property format (expected key=value): " + p}
		}

		k = strings.TrimSpace(k)
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			properties[k] = parsed
			continue
		}

		properties[k] = v
	}

	return properties, nil
}

type PageSyncCmd struct {
	File     string `arg:"" help:"Markdown file to sync" type:"existingfile"`
	Title    string `help:"Page title (default: filename or first heading)" short:"t"`
	Parent   string `help:"Parent page URL, name, or ID" short:"p"`
	ParentDB string `help:"Parent database URL, name, or ID" name:"parent-db" short:"d"`
	Icon     string `help:"Emoji icon for the page" short:"i"`
	JSON     bool   `help:"Output as JSON" short:"j"`
}

func (c *PageSyncCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageSync(ctx, c.File, c.Title, c.Parent, c.ParentDB, c.Icon)
}

func runPageSync(ctx *Context, file, title, parent, parentDB, icon string) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		output.PrintError(err)
		return err
	}

	content := string(raw)
	fm, body := cli.ParseFrontmatter(content)

	if title == "" {
		title = extractTitleFromMarkdown(body)
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

	if fm.NotionID != "" {
		req := mcp.UpdatePageRequest{
			PageID:     fm.NotionID,
			Command:    "replace_content",
			NewContent: body,
		}
		if err := client.UpdatePage(bgCtx, req); err != nil {
			output.PrintError(err)
			return err
		}

		displayTitle := title
		if icon != "" {
			displayTitle = icon + " " + title
		}

		if ctx.JSON {
			outPage := output.Page{
				ID:    fm.NotionID,
				Title: displayTitle,
				Icon:  icon,
			}
			return output.PrintPage(outPage, true)
		}

		output.PrintSuccess("Synced: " + displayTitle)
		return nil
	}

	req := mcp.CreatePageRequest{
		Title:   title,
		Content: body,
	}

	if parentDB != "" {
		dbID, err := cli.ResolveDatabaseID(bgCtx, client, parentDB)
		if err != nil {
			output.PrintError(err)
			return err
		}
		dbID, err = client.ResolveDataSourceID(bgCtx, dbID)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentDatabaseID = dbID
	} else if parent != "" {
		parentID, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentPageID = parentID
	}

	resp, err := client.CreatePage(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	pageID := resp.ID
	if pageID == "" && resp.URL != "" {
		pageID, _ = cli.ExtractNotionUUID(resp.URL)
	}
	if pageID == "" {
		output.PrintWarning("Page created but could not retrieve ID for frontmatter")
	} else {
		updated := cli.SetFrontmatterID(content, pageID)
		fileMode := os.FileMode(0o644)
		if info, err := os.Stat(file); err == nil {
			fileMode = info.Mode()
		}
		if err := os.WriteFile(file, []byte(updated), fileMode); err != nil {
			output.PrintError(fmt.Errorf("page created but failed to update frontmatter: %w", err))
			return err
		}
	}

	displayTitle := title
	if icon != "" {
		displayTitle = icon + " " + title
	}

	if ctx.JSON {
		outPage := output.Page{
			ID:    pageID,
			URL:   resp.URL,
			Title: displayTitle,
			Icon:  icon,
		}
		return output.PrintPage(outPage, true)
	}

	output.PrintSuccess("Created: " + displayTitle)
	if resp.URL != "" {
		output.PrintInfo(resp.URL)
	}
	return nil
}

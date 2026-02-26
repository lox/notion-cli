package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type DBCmd struct {
	List   DBListCmd   `cmd:"" help:"List databases"`
	Query  DBQueryCmd  `cmd:"" help:"Query a database"`
	Create DBCreateCmd `cmd:"" help:"Create an entry in a database"`
}

type DBListCmd struct {
	Query string `help:"Filter databases by name" short:"q"`
	Limit int    `help:"Maximum number of results" short:"l" default:"20"`
	JSON  bool   `help:"Output as JSON" short:"j"`
}

func (c *DBListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBList(ctx, c.Query, c.Limit)
}

func runDBList(ctx *Context, query string, limit int) error {
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

	resp, err := client.Search(bgCtx, searchQuery, &mcp.SearchOptions{ContentSearchMode: "workspace_search"})
	if err != nil {
		output.PrintError(err)
		return err
	}

	dbs := filterDatabases(resp.Results, limit)
	return output.PrintDatabases(dbs, ctx.JSON)
}

func filterDatabases(results []mcp.SearchResult, limit int) []output.Database {
	dbs := make([]output.Database, 0)
	for _, r := range results {
		if r.ObjectType != "database" && r.Object != "database" && r.ObjectType != "data_source" && r.Type != "database" {
			continue
		}
		if limit > 0 && len(dbs) >= limit {
			break
		}
		dbs = append(dbs, output.Database{
			ID:    r.ID,
			Title: r.Title,
			URL:   r.URL,
		})
	}
	return dbs
}

type DBQueryCmd struct {
	ID   string `arg:"" help:"Database URL or ID"`
	JSON bool   `help:"Output as JSON" short:"j"`
}

func (c *DBQueryCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBQuery(ctx, c.ID)
}

type DBCreateCmd struct {
	Database string   `arg:"" help:"Database URL, ID, or name"`
	Title    string   `help:"Entry title" short:"t" required:""`
	Prop     []string `help:"Property key=value (repeatable)" short:"P"`
	Content  string   `help:"Inline markdown body" short:"c" xor:"body"`
	File     string   `help:"Read body from markdown file" short:"f" type:"existingfile" xor:"body"`
	JSON     bool     `help:"Output as JSON" short:"j"`
}

func (c *DBCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBCreate(ctx, c.Database, c.Title, c.Prop, c.Content, c.File)
}

func runDBCreate(ctx *Context, database, title string, props []string, content, file string) error {
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			output.PrintError(err)
			return err
		}
		content = string(data)
	}

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	dbID, err := cli.ResolveDatabaseID(bgCtx, client, database)
	if err != nil {
		output.PrintError(err)
		return err
	}

	dbID, err = client.ResolveDataSourceID(bgCtx, dbID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	properties := make(map[string]string)
	for _, p := range props {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			output.PrintError(&output.UserError{Message: "invalid property format (expected key=value): " + p})
			return &output.UserError{Message: "invalid property format: " + p}
		}
		properties[k] = v
	}

	req := mcp.CreatePageRequest{
		ParentDatabaseID: dbID,
		Title:            title,
		Content:          content,
		Properties:       properties,
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
		output.PrintSuccess("Entry created: " + resp.URL)
	} else {
		output.PrintSuccess("Entry created")
	}
	return nil
}

func runDBQuery(ctx *Context, id string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()
	result, err := client.Fetch(bgCtx, id)
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

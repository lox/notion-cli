package cmd

import (
	"context"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type DBCmd struct {
	List  DBListCmd  `cmd:"" help:"List databases"`
	Query DBQueryCmd `cmd:"" help:"Query a database"`
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

	resp, err := client.Search(bgCtx, searchQuery)
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
		if r.ObjectType != "database" && r.Object != "database" {
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

package cmd

import (
	"context"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type SearchCmd struct {
	Query      string `arg:"" help:"Search query"`
	Limit      int    `help:"Maximum number of results" short:"l" default:"20"`
	JSON       bool   `help:"Output as JSON" short:"j"`
	SearchMode string `help:"Search mode: 'workspace' (default) or 'ai' (includes connected sources like Linear, Slack)" short:"m" default:"workspace" enum:"workspace,ai"`
}

func (c *SearchCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runSearch(ctx, c.Query, c.Limit, c.SearchMode)
}

func runSearch(ctx *Context, query string, limit int, searchMode string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}

	mode := "workspace_search"
	if searchMode == "ai" {
		mode = "ai_search"
	}
	opts := &mcp.SearchOptions{ContentSearchMode: mode}

	bgCtx := context.Background()
	resp, err := client.Search(bgCtx, query, opts)
	if err != nil {
		output.PrintError(err)
		return err
	}

	results := convertSearchResults(resp.Results, limit)
	return output.PrintSearchResults(results, ctx.JSON)
}

func convertSearchResults(mcpResults []mcp.SearchResult, limit int) []output.SearchResult {
	results := make([]output.SearchResult, 0, len(mcpResults))
	for i, r := range mcpResults {
		if limit > 0 && i >= limit {
			break
		}
		results = append(results, output.SearchResult{
			ID:    r.ID,
			Type:  r.ObjectType,
			Title: r.Title,
			URL:   r.URL,
		})
	}
	return results
}

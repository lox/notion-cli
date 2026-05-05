package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/output"
)

type ToolsCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

type toolSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *ToolsCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()
	tools, err := client.ListTools(bgCtx)
	if err != nil {
		output.PrintError(err)
		return err
	}

	summaries := make([]toolSummary, len(tools))
	for i, t := range tools {
		summaries[i] = toolSummary{Name: t.Name, Description: t.Description}
	}

	return printTools(os.Stdout, summaries, c.JSON)
}

func printTools(w io.Writer, tools []toolSummary, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tools)
	}

	for _, t := range tools {
		if _, err := fmt.Fprintf(w, "%s\n  %s\n\n", t.Name, t.Description); err != nil {
			return err
		}
	}

	return nil
}

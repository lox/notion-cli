package cmd

import (
	"context"
	"fmt"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/output"
)

type ToolsCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
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

	if c.JSON {
		return writeJSON(tools)
	}

	for _, t := range tools {
		fmt.Printf("%s\n  %s\n\n", t.Name, t.Description)
	}

	return nil
}

package cmd

import (
	"context"
	"fmt"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/output"
)

type ToolsCmd struct{}

func (c *ToolsCmd) Run(ctx *Context) error {
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

	for _, t := range tools {
		fmt.Printf("%s\n  %s\n\n", t.Name, t.Description)
	}

	return nil
}

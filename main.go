package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/lox/notion-cli/cmd"
	"github.com/lox/notion-cli/internal/cli"
)

var version = "dev"

func main() {
	c := &cmd.CLI{}
	ctx := kong.Parse(c,
		kong.Name("notion"),
		kong.Description("A CLI for Notion"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	cli.SetAccessToken(c.Token)
	err := ctx.Run(&cmd.Context{Token: c.Token})
	ctx.FatalIfErrorf(err)
	os.Exit(0)
}

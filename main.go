package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/lox/notion-cli/cmd"
	"github.com/lox/notion-cli/internal/cli"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			println("notion-cli version " + version)
			os.Exit(0)
		}
	}

	c := &cmd.CLI{}
	ctx := kong.Parse(c,
		kong.Name("notion-cli"),
		kong.Description("A CLI for Notion"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	cli.SetAccessToken(c.Token)
	err := ctx.Run(&cmd.Context{Token: c.Token})
	ctx.FatalIfErrorf(err)
	os.Exit(0)
}

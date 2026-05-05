package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/lox/notion-cli/cmd"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/config"
)

var version = "dev"

func main() {
	if shouldPrintVersionAndExit(os.Args[1:]) {
		println("notion-cli version " + version)
		os.Exit(0)
	}

	c := &cmd.CLI{}
	ctx := kong.Parse(c,
		kong.Name("notion-cli"),
		kong.Description("A CLI for Notion"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	profile, err := config.ResolveSelectedProfile(c.Profile)
	ctx.FatalIfErrorf(err)
	cli.SetAccessToken(c.Token)
	cli.SetProfile(profile)
	err = ctx.Run(&cmd.Context{
		Profile:          profile,
		Token:            c.Token,
		APIToken:         c.APIToken,
		APIBaseURL:       c.APIBaseURL,
		APINotionVersion: c.APINotionVersion,
	})
	ctx.FatalIfErrorf(err)
	os.Exit(0)
}

func shouldPrintVersionAndExit(args []string) bool {
	if len(args) != 1 {
		return false
	}

	return args[0] == "--version" || args[0] == "-v"
}

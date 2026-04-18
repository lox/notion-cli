package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/lox/notion-cli/cmd"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/profile"
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

	active, profileErr := profile.Resolve(c.Profile)
	if profileErr != nil {
		hasAccessToken := strings.TrimSpace(c.Token) != ""
		if !hasAccessToken || !errors.Is(profileErr, profile.ErrNoProfile) {
			if errors.Is(profileErr, profile.ErrNoProfile) {
				_, _ = fmt.Fprintln(os.Stderr, "\u2717 No profile specified. Pass --profile <name> or set NOTION_CLI_PROFILE.")
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "\u2717 %s\n", profileErr)
			}
			os.Exit(1)
		}
		// Headless path: NOTION_ACCESS_TOKEN authenticates MCP directly, so
		// the profile gate can be skipped. Profile-scoped operations (like
		// auth api setup) will still use the implicit default layout.
	} else {
		cli.SetActiveProfile(active)
	}
	cli.SetAccessToken(c.Token)

	runErr := ctx.Run(&cmd.Context{
		Token:            c.Token,
		APIToken:         c.APIToken,
		APIBaseURL:       c.APIBaseURL,
		APINotionVersion: c.APINotionVersion,
		Profile:          cli.ActiveProfile(),
	})
	ctx.FatalIfErrorf(runErr)
	os.Exit(0)
}

func shouldPrintVersionAndExit(args []string) bool {
	if len(args) != 1 {
		return false
	}

	return args[0] == "--version" || args[0] == "-v"
}

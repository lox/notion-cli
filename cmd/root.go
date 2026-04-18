package cmd

import (
	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/profile"
)

type Context struct {
	JSON             bool
	Token            string
	APIToken         string
	APIBaseURL       string
	APINotionVersion string
	Profile          profile.Profile
}

type CLI struct {
	Token            string `help:"Access token (skips OAuth)" env:"NOTION_ACCESS_TOKEN" hidden:""`
	APIToken         string `env:"NOTION_API_TOKEN" hidden:""`
	APIBaseURL       string `env:"NOTION_API_BASE_URL" hidden:""`
	APINotionVersion string `env:"NOTION_API_NOTION_VERSION" hidden:""`
	// Profile intentionally does not use Kong's env:"NOTION_CLI_PROFILE" tag.
	// profile.Resolve needs to see the flag value and env variable as
	// separate inputs so auth status can attribute the selection to
	// --profile vs NOTION_CLI_PROFILE; Kong would merge them into one value
	// and we'd always report SourceFlag.
	Profile string `help:"Notion account profile to use (also reads $NOTION_CLI_PROFILE)" name:"profile"`

	Auth    AuthCmd    `cmd:"" help:"Authentication commands"`
	Page    PageCmd    `cmd:"" help:"Page commands"`
	Search  SearchCmd  `cmd:"" help:"Search Notion"`
	DB      DBCmd      `cmd:"" name:"db" help:"Database commands"`
	Comment CommentCmd `cmd:"" help:"Comment commands"`
	Tools   ToolsCmd   `cmd:"" help:"List available MCP tools"`
	Version VersionCmd `cmd:"" help:"Show version"`
}

type VersionCmd struct {
	Version string `kong:"hidden,default='${version}'"`
}

func (c *VersionCmd) Run(ctx *Context) error {
	println("notion-cli version " + c.Version)
	return nil
}

func officialAPIOverrides(ctx *Context) config.APIOverrides {
	if ctx == nil {
		return config.APIOverrides{}
	}
	return config.APIOverrides{
		BaseURL:       ctx.APIBaseURL,
		NotionVersion: ctx.APINotionVersion,
		Token:         ctx.APIToken,
	}
}

package cmd

import "github.com/lox/notion-cli/internal/config"

type Context struct {
	JSON             bool
	Token            string
	APIToken         string
	APIBaseURL       string
	APINotionVersion string
}

type CLI struct {
	Token            string `help:"Access token (skips OAuth)" env:"NOTION_ACCESS_TOKEN" hidden:""`
	APIToken         string `env:"NOTION_API_TOKEN" hidden:""`
	APIBaseURL       string `env:"NOTION_API_BASE_URL" hidden:""`
	APINotionVersion string `env:"NOTION_API_NOTION_VERSION" hidden:""`

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

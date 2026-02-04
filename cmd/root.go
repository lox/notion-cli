package cmd

type Context struct {
	JSON bool
}

type CLI struct {
	Config  ConfigCmd  `cmd:"" help:"Configure authentication"`
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
	println("notion version " + c.Version)
	return nil
}

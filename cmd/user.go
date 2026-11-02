package cmd

import (
	"context"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type UserCmd struct {
	List UserListCmd `cmd:"" help:"List workspace users"`
	Me   UserMeCmd   `cmd:"" help:"Show the user the current token authenticates as"`
}

type UserListCmd struct {
	Query string `help:"Filter users by name or email" short:"q"`
	Limit int    `help:"Maximum number of users" short:"l"`
	JSON  bool   `help:"Output as JSON" short:"j"`
}

func (c *UserListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runUserList(ctx, c.Query, c.Limit)
}

func runUserList(ctx *Context, query string, limit int) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	users, err := client.ListUsers(context.Background(), &mcp.ListUsersOptions{Limit: limit, Query: query})
	if err != nil {
		output.PrintError(err)
		return err
	}

	return output.PrintUsers(toOutputUsers(users), ctx.JSON)
}

type UserMeCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *UserMeCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runUserMe(ctx)
}

func runUserMe(ctx *Context) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		output.PrintError(err)
		return err
	}
	if user == nil {
		err := &output.UserError{Message: "the server returned no user for this token"}
		output.PrintError(err)
		return err
	}

	return output.PrintUsers(toOutputUsers([]mcp.User{*user}), ctx.JSON)
}

func toOutputUsers(users []mcp.User) []output.User {
	out := make([]output.User, 0, len(users))
	for _, u := range users {
		item := output.User{ID: u.ID, Name: u.Name, Type: u.Type}
		if u.Person != nil {
			item.Email = u.Person.Email
		}
		out = append(out, item)
	}
	return out
}

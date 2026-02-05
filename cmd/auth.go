package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type AuthCmd struct {
	Login   AuthLoginCmd   `cmd:"" help:"Authenticate with Notion via OAuth"`
	Refresh AuthRefreshCmd `cmd:"" help:"Refresh the access token"`
	Status  AuthStatusCmd  `cmd:"" default:"withargs" help:"Show authentication status"`
	Logout  AuthLogoutCmd  `cmd:"" help:"Clear stored credentials"`
}

type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	if err := mcp.RunOAuthFlow(bgCtx, tokenStore); err != nil {
		output.PrintError(err)
		return err
	}

	return nil
}

type AuthRefreshCmd struct{}

func (c *AuthRefreshCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	token, err := tokenStore.GetToken(bgCtx)
	if err != nil {
		if err == mcp.ErrNoToken {
			output.PrintWarning("Not authenticated. Run 'notion-cli auth login' first.")
			return err
		}
		output.PrintError(err)
		return err
	}

	if token.RefreshToken == "" {
		output.PrintWarning("No refresh token available. Run 'notion-cli auth login' to re-authenticate.")
		return fmt.Errorf("no refresh token")
	}

	newToken, err := mcp.RefreshToken(bgCtx, tokenStore)
	if err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Token refreshed")
	fmt.Printf("Expires: %s\n", newToken.ExpiresAt.Format("2 Jan 2006 15:04"))
	return nil
}

type AuthStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthStatusCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	token, err := tokenStore.GetToken(context.Background())
	if err != nil {
		if err == mcp.ErrNoToken {
			fmt.Println("Not authenticated. Run 'notion-cli auth login' to authenticate.")
			return nil
		}
		output.PrintError(err)
		return err
	}

	hasValidToken := token.AccessToken != "" && !token.IsExpired()

	if ctx.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"authenticated": hasValidToken,
			"token_type":    token.TokenType,
			"has_token":     token.AccessToken != "",
			"expires_at":    token.ExpiresAt,
			"config_path":   tokenStore.Path(),
		})
	}

	labelStyle := color.New(color.Faint)

	if hasValidToken {
		output.PrintSuccess("Authenticated")
	} else {
		output.PrintWarning("Token expired or not set")
	}
	fmt.Println()

	_, _ = labelStyle.Print("Config path: ")
	fmt.Println(tokenStore.Path())

	_, _ = labelStyle.Print("Token type:  ")
	fmt.Println(token.TokenType)

	if !token.ExpiresAt.IsZero() {
		_, _ = labelStyle.Print("Expires:     ")
		fmt.Println(token.ExpiresAt.Format("2 Jan 2006 15:04"))
	}

	return nil
}

type AuthLogoutCmd struct{}

func (c *AuthLogoutCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	if err := tokenStore.Clear(); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Logged out")
	return nil
}

package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
	"golang.org/x/term"
)

type AuthCmd struct {
	Login   AuthLoginCmd   `cmd:"" help:"Authenticate with Notion via OAuth"`
	Refresh AuthRefreshCmd `cmd:"" help:"Refresh the access token"`
	Status  AuthStatusCmd  `cmd:"" default:"withargs" help:"Show authentication status"`
	Logout  AuthLogoutCmd  `cmd:"" help:"Clear stored credentials"`
	API     AuthAPICmd     `cmd:"" name:"api" help:"Official API token commands"`
}

var authAPIInput io.Reader = os.Stdin
var authAPIOutput io.Writer = os.Stdout
var authAPIError io.Writer = os.Stderr
var openOfficialAPIBrowser = mcp.OpenBrowser
var notionAPITokenPattern = regexp.MustCompile(`^ntn_[A-Za-z0-9]{20,}$`)

const officialAPIIntegrationsURL = "https://www.notion.so/profile/integrations/internal"

type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStoreForProfile(ctx.Profile)
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
	tokenStore, err := mcp.NewFileTokenStoreForProfile(ctx.Profile)
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

	tokenStore, err := mcp.NewFileTokenStoreForProfile(ctx.Profile)
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
	activeProfile := ctx.Profile

	if ctx.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"authenticated":  hasValidToken,
			"token_type":     token.TokenType,
			"has_token":      token.AccessToken != "",
			"expires_at":     token.ExpiresAt,
			"config_path":    tokenStore.Path(),
			"profile":        activeProfile.Name,
			"profile_source": string(activeProfile.Source),
		})
	}

	labelStyle := color.New(color.Faint)

	if hasValidToken {
		output.PrintSuccess("Authenticated")
	} else {
		output.PrintWarning("Token expired or not set")
	}
	fmt.Println()

	_, _ = labelStyle.Print("Profile:     ")
	fmt.Printf("%s (from %s)\n", activeProfile.Name, activeProfile.Source)

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
	tokenStore, err := mcp.NewFileTokenStoreForProfile(ctx.Profile)
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

type AuthAPICmd struct {
	Setup  AuthAPISetupCmd  `cmd:"" help:"Set up official Notion API token"`
	Status AuthAPIStatusCmd `cmd:"" help:"Show official API token status"`
	Verify AuthAPIVerifyCmd `cmd:"" help:"Verify official API token"`
	Unset  AuthAPIUnsetCmd  `cmd:"" help:"Remove saved official API token"`
}

type AuthAPISetupCmd struct{}

func (c *AuthAPISetupCmd) Run(ctx *Context) error {
	token, err := readOfficialAPIToken(authAPIInput, authAPIOutput, authAPIError)
	if err != nil {
		output.PrintError(err)
		return err
	}
	if token == "" {
		err := fmt.Errorf("official API token cannot be empty")
		output.PrintError(err)
		return err
	}
	if !looksLikeNotionAPIToken(token) {
		output.PrintWarning("Official API token does not match the expected Notion token format")
		_, _ = fmt.Fprintln(authAPIOutput, "Expected format: ntn_<letters-and-numbers>")
	}
	if err := config.SetAPITokenForProfile(ctx.Profile, token); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Official API token saved")
	_, _ = fmt.Fprintf(authAPIOutput, "Config path: %s\n", mustConfigPath(ctx))
	return nil
}

type AuthAPIStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIStatusCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	loaded, err := cli.LoadOfficialAPIConfig(ctx.Profile, officialAPIOverrides(ctx))
	if err != nil {
		output.PrintError(err)
		return err
	}
	return printAuthAPIStatus(ctx, loaded)
}

type AuthAPIVerifyCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIVerifyCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	loaded, err := cli.LoadOfficialAPIConfig(ctx.Profile, officialAPIOverrides(ctx))
	if err != nil {
		output.PrintError(err)
		return err
	}
	client, err := cli.RequireOfficialAPIClient(ctx.Profile, officialAPIOverrides(ctx))
	if err != nil {
		output.PrintError(err)
		return err
	}

	self, err := client.GetSelf(context.Background())
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		enc := json.NewEncoder(authAPIOutput)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"verified":       true,
			"token_source":   loaded.APITokenSource,
			"config_path":    loaded.ConfigPath,
			"base_url":       loaded.Config.API.BaseURL,
			"notion_version": loaded.Config.API.NotionVersion,
			"self":           self,
		})
	}

	output.PrintSuccess("Official API token verified")
	_, _ = fmt.Fprintf(authAPIOutput, "Token source:   %s\n", loaded.APITokenSource)
	_, _ = fmt.Fprintf(authAPIOutput, "Config path:    %s\n", loaded.ConfigPath)
	_, _ = fmt.Fprintf(authAPIOutput, "Base URL:       %s\n", loaded.Config.API.BaseURL)
	_, _ = fmt.Fprintf(authAPIOutput, "Notion version: %s\n", loaded.Config.API.NotionVersion)
	if self.Name != "" {
		_, _ = fmt.Fprintf(authAPIOutput, "Actor:          %s\n", self.Name)
	}
	if self.Bot != nil && self.Bot.WorkspaceName != "" {
		_, _ = fmt.Fprintf(authAPIOutput, "Workspace:      %s\n", self.Bot.WorkspaceName)
	}
	return nil
}

type AuthAPIUnsetCmd struct{}

func (c *AuthAPIUnsetCmd) Run(ctx *Context) error {
	loaded, err := cli.LoadOfficialAPIConfig(ctx.Profile, officialAPIOverrides(ctx))
	if err != nil {
		output.PrintError(err)
		return err
	}
	if !loaded.HasConfigToken {
		if loaded.APITokenSource == config.APITokenSourceEnv {
			output.PrintWarning("No saved official API token to remove")
			_, _ = fmt.Fprintln(authAPIOutput, "Effective token still comes from NOTION_API_TOKEN.")
			return nil
		}
		output.PrintWarning("No saved official API token to remove")
		_, _ = fmt.Fprintf(authAPIOutput, "Config path: %s\n", loaded.ConfigPath)
		return nil
	}

	if err := config.UnsetAPITokenForProfile(ctx.Profile); err != nil {
		output.PrintError(err)
		return err
	}
	if loaded.APITokenSource == config.APITokenSourceEnv {
		output.PrintSuccess("Saved official API token removed")
		_, _ = fmt.Fprintln(authAPIOutput, "Effective token still comes from NOTION_API_TOKEN.")
	} else {
		output.PrintSuccess("Official API token removed")
	}
	_, _ = fmt.Fprintf(authAPIOutput, "Config path: %s\n", loaded.ConfigPath)
	return nil
}

func printAuthAPIStatus(ctx *Context, loaded *cli.OfficialAPIConfig) error {
	hasToken := strings.TrimSpace(loaded.Config.API.Token) != ""
	if ctx.JSON {
		enc := json.NewEncoder(authAPIOutput)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"configured":     hasToken,
			"token_source":   loaded.APITokenSource,
			"config_path":    loaded.ConfigPath,
			"base_url":       loaded.Config.API.BaseURL,
			"notion_version": loaded.Config.API.NotionVersion,
		})
	}

	if hasToken {
		output.PrintSuccess("Official API token configured")
	} else {
		output.PrintWarning("Official API token not configured")
	}
	_, _ = fmt.Fprintln(authAPIOutput)
	_, _ = fmt.Fprintf(authAPIOutput, "Token source:   %s\n", loaded.APITokenSource)
	_, _ = fmt.Fprintf(authAPIOutput, "Config path:    %s\n", loaded.ConfigPath)
	_, _ = fmt.Fprintf(authAPIOutput, "Base URL:       %s\n", loaded.Config.API.BaseURL)
	_, _ = fmt.Fprintf(authAPIOutput, "Notion version: %s\n", loaded.Config.API.NotionVersion)
	if !hasToken {
		_, _ = fmt.Fprintln(authAPIOutput, "Run 'notion-cli auth api setup' or set NOTION_API_TOKEN.")
	}
	return nil
}

func readOfficialAPIToken(in io.Reader, out, errOut io.Writer) (string, error) {
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		printOfficialAPITokenSetupHint(errOut, true)
		_, _ = fmt.Fprint(errOut, "Official API token: ")
		secret, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(errOut)
		if err != nil {
			return "", err
		}
		token := strings.TrimSpace(string(secret))
		if token != "" {
			_, _ = fmt.Fprintln(errOut, "Token received (hidden).")
		}
		return token, nil
	}

	_, _ = fmt.Fprint(out, "Official API token: ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func looksLikeNotionAPIToken(token string) bool {
	return notionAPITokenPattern.MatchString(strings.TrimSpace(token))
}

func printOfficialAPITokenSetupHint(out io.Writer, shouldOpenBrowser bool) {
	_, _ = fmt.Fprintln(out, "Get your token from Notion internal integrations:")
	_, _ = fmt.Fprintf(out, "  %s\n", officialAPIIntegrationsURL)
	if shouldOpenBrowser {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Opening that page in your browser...")
		if err := openOfficialAPIBrowser(officialAPIIntegrationsURL); err != nil {
			_, _ = fmt.Fprintf(out, "(Could not open browser automatically: %v)\n", err)
		}
	}
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Create or select an integration, copy the token from Configuration, then paste it below.")
	_, _ = fmt.Fprintln(out, "Paste is hidden. Press Enter when done.")
	_, _ = fmt.Fprintln(out)
}

func mustConfigPath(ctx *Context) string {
	path, err := config.PathFor(ctx.Profile)
	if err != nil {
		return "<unknown>"
	}
	return path
}

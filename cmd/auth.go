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
	"time"

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
	List    AuthListCmd    `cmd:"" help:"List profiles and authentication state"`
	Use     AuthUseCmd     `cmd:"" help:"Set the active profile"`
	Logout  AuthLogoutCmd  `cmd:"" help:"Clear stored credentials"`
	API     AuthAPICmd     `cmd:"" name:"api" help:"Official API token commands"`
}

var authAPIInput io.Reader = os.Stdin
var authAPIOutput io.Writer = os.Stdout
var authAPIError io.Writer = os.Stderr
var openOfficialAPIBrowser = mcp.OpenBrowser
var notionAPITokenPattern = regexp.MustCompile(`^ntn_[A-Za-z0-9]{20,}$`)

const officialAPIIntegrationsURL = "https://www.notion.so/profile/integrations/internal"

type authProfileStatus struct {
	Profile        string     `json:"profile"`
	Active         bool       `json:"active"`
	HasOAuthToken  bool       `json:"has_oauth_token"`
	OAuthStatus    string     `json:"oauth_status"`
	OAuthExpiresAt *time.Time `json:"oauth_expires_at,omitempty"`
	HasAPIToken    bool       `json:"has_api_token"`
	TokenPath      string     `json:"token_path"`
	ConfigPath     string     `json:"config_path"`
}

type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore(ctx.Profile)
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
	tokenStore, err := mcp.NewFileTokenStore(ctx.Profile)
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

	status, err := inspectProfileStatus(ctx.Profile)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		payload := map[string]any{
			"authenticated": status.OAuthStatus == "valid",
			"profile":       status.Profile,
			"has_token":     status.HasOAuthToken,
			"token_path":    status.TokenPath,
			"oauth_status":  status.OAuthStatus,
		}
		if status.OAuthExpiresAt != nil {
			payload["expires_at"] = status.OAuthExpiresAt
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	labelStyle := color.New(color.Faint)
	switch status.OAuthStatus {
	case "valid":
		output.PrintSuccess("Authenticated")
	case "expired":
		output.PrintWarning("Token expired")
	default:
		output.PrintWarning("Not authenticated")
	}
	fmt.Println()

	_, _ = labelStyle.Print("Profile:    ")
	fmt.Println(status.Profile)
	_, _ = labelStyle.Print("Token path: ")
	fmt.Println(status.TokenPath)

	if status.OAuthExpiresAt != nil {
		_, _ = labelStyle.Print("Expires:    ")
		fmt.Println(status.OAuthExpiresAt.Format("2 Jan 2006 15:04"))
	}
	if status.OAuthStatus == "missing" {
		_, _ = fmt.Fprintln(os.Stdout, "Run 'notion-cli auth login' to authenticate this profile.")
	}

	return nil
}

type AuthListCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthListCmd) Run(ctx *Context) error {
	profiles, err := config.ListProfiles()
	if err != nil {
		output.PrintError(err)
		return err
	}

	rows := make([]authProfileStatus, 0, len(profiles))
	for _, profile := range profiles {
		row, err := inspectProfileStatus(profile)
		if err != nil {
			output.PrintError(err)
			return err
		}
		rows = append(rows, row)
	}

	if c.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	labelStyle := color.New(color.Faint)
	for i, row := range rows {
		header := row.Profile
		if row.Active {
			header += " (active)"
		}
		fmt.Println(header)
		_, _ = labelStyle.Print("  OAuth:      ")
		fmt.Println(row.OAuthStatus)
		if row.OAuthExpiresAt != nil {
			_, _ = labelStyle.Print("  Expires:    ")
			fmt.Println(row.OAuthExpiresAt.Format("2 Jan 2006 15:04"))
		}
		_, _ = labelStyle.Print("  API token:  ")
		if row.HasAPIToken {
			fmt.Println("configured")
		} else {
			fmt.Println("missing")
		}
		_, _ = labelStyle.Print("  Token path: ")
		fmt.Println(row.TokenPath)
		_, _ = labelStyle.Print("  Config path:")
		fmt.Println(" " + row.ConfigPath)
		if i < len(rows)-1 {
			fmt.Println()
		}
	}

	return nil
}

type AuthUseCmd struct {
	Profile string `arg:"" help:"Profile name to make active"`
}

func (c *AuthUseCmd) Run(ctx *Context) error {
	if err := config.SetActiveProfile(c.Profile); err != nil {
		output.PrintError(err)
		return err
	}

	status, err := inspectProfileStatus(c.Profile)
	if err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Active profile updated")
	fmt.Printf("Profile: %s\n", status.Profile)
	if status.OAuthStatus == "missing" {
		fmt.Println("Run 'notion-cli auth login' to authenticate this profile.")
	}
	if !status.HasAPIToken {
		fmt.Println("Run 'notion-cli auth api setup' if this profile needs official API features.")
	}
	return nil
}

type AuthLogoutCmd struct{}

func (c *AuthLogoutCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore(ctx.Profile)
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
	_, _ = fmt.Fprintf(authAPIOutput, "Config path: %s\n", mustConfigPath(ctx.Profile))
	return nil
}

type AuthAPIStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIStatusCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	loaded, err := cli.LoadOfficialAPIConfig(officialAPIOverrides(ctx))
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

	loaded, err := cli.LoadOfficialAPIConfig(officialAPIOverrides(ctx))
	if err != nil {
		output.PrintError(err)
		return err
	}
	client, err := cli.RequireOfficialAPIClient(officialAPIOverrides(ctx))
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
			"profile":        loaded.Profile,
			"token_source":   loaded.APITokenSource,
			"config_path":    loaded.ConfigPath,
			"base_url":       loaded.Config.API.BaseURL,
			"notion_version": loaded.Config.API.NotionVersion,
			"self":           self,
		})
	}

	output.PrintSuccess("Official API token verified")
	_, _ = fmt.Fprintf(authAPIOutput, "Profile:        %s\n", loaded.Profile)
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
	loaded, err := cli.LoadOfficialAPIConfig(officialAPIOverrides(ctx))
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
			"profile":        loaded.Profile,
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
	_, _ = fmt.Fprintf(authAPIOutput, "Profile:        %s\n", loaded.Profile)
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

func mustConfigPath(profile string) string {
	path, err := config.PathForProfile(profile)
	if err != nil {
		return "<unknown>"
	}
	return path
}

func inspectProfileStatus(profile string) (authProfileStatus, error) {
	resolvedProfile, err := config.ResolveProfile(profile)
	if err != nil {
		return authProfileStatus{}, err
	}
	active, err := config.ActiveProfile()
	if err != nil {
		return authProfileStatus{}, err
	}
	paths, err := config.PathsForProfile(resolvedProfile)
	if err != nil {
		return authProfileStatus{}, err
	}
	loaded, err := config.LoadWithMeta(config.APIOverrides{Profile: resolvedProfile})
	if err != nil {
		return authProfileStatus{}, err
	}

	status := authProfileStatus{
		Profile:     resolvedProfile,
		Active:      resolvedProfile == active,
		HasAPIToken: loaded.HasConfigToken,
		TokenPath:   paths.TokenPath,
		ConfigPath:  paths.ConfigPath,
		OAuthStatus: "missing",
	}

	tokenStore, err := mcp.NewFileTokenStore(resolvedProfile)
	if err != nil {
		return authProfileStatus{}, err
	}
	token, err := tokenStore.GetToken(context.Background())
	if err != nil {
		if err == mcp.ErrNoToken {
			return status, nil
		}
		return authProfileStatus{}, err
	}

	status.HasOAuthToken = strings.TrimSpace(token.AccessToken) != ""
	expiresAt := token.ExpiresAt
	status.OAuthExpiresAt = &expiresAt
	if status.HasOAuthToken {
		if token.IsExpired() {
			status.OAuthStatus = "expired"
		} else {
			status.OAuthStatus = "valid"
		}
	}
	return status, nil
}

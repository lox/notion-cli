package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

var accessToken string
var authRefreshNoticeWriter io.Writer = os.Stderr

func SetAccessToken(token string) {
	accessToken = token
}

func GetClient() (*mcp.Client, error) {
	ctx := context.Background()

	// Auto-refresh if token is expired or expiring soon
	if accessToken == "" {
		if err := autoRefreshIfNeeded(ctx); err != nil {
			// Non-fatal, but surface guidance to reduce auth-related command failures.
			printAuthRefreshGuidance(err)
		}
	}

	opts := []mcp.ClientOption{mcp.WithProfile(ActiveProfile())}
	if accessToken != "" {
		opts = append(opts, mcp.WithAccessToken(accessToken))
	}

	client, err := mcp.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if err := client.Start(ctx); err != nil {
		if mcp.IsAuthRequired(err) {
			output.PrintWarning("Not authenticated. Run 'notion-cli auth login' to authenticate.")
			return nil, err
		}
		return nil, fmt.Errorf("start client: %w", err)
	}

	return client, nil
}

func autoRefreshIfNeeded(ctx context.Context) error {
	tokenStore, err := mcp.NewFileTokenStoreForProfile(ActiveProfile())
	if err != nil {
		return err
	}

	token, err := tokenStore.GetToken(ctx)
	if err != nil {
		return err
	}

	// Refresh if expired or expiring within 5 minutes
	if token.ExpiresAt.Before(time.Now().Add(5 * time.Minute)) {
		if token.RefreshToken == "" {
			return fmt.Errorf("token expired and no refresh token available")
		}

		_, err := mcp.RefreshToken(ctx, tokenStore)
		if err != nil {
			return fmt.Errorf("auto-refresh failed: %w", err)
		}
	}

	return nil
}

func RequireClient() (*mcp.Client, error) {
	return GetClient()
}

func printAuthRefreshGuidance(err error) {
	if err == nil {
		return
	}

	_, _ = fmt.Fprintf(authRefreshNoticeWriter, "Note: Auth token refresh skipped: %s\n", err.Error())
	_, _ = fmt.Fprintln(authRefreshNoticeWriter, "Note: Run 'notion-cli auth status' and, if needed, 'notion-cli auth login' or 'notion-cli auth refresh'.")
}

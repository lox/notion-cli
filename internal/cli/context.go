package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

var accessToken string

func SetAccessToken(token string) {
	accessToken = token
}

func GetClient() (*mcp.Client, error) {
	ctx := context.Background()

	// Auto-refresh if token is expired or expiring soon
	if accessToken == "" {
		if err := autoRefreshIfNeeded(ctx); err != nil {
			// Non-fatal, will try to proceed anyway
			_ = err
		}
	}

	var opts []mcp.ClientOption
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
	tokenStore, err := mcp.NewFileTokenStore()
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

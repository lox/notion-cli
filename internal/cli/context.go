package cli

import (
	"context"
	"fmt"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

var accessToken string

func SetAccessToken(token string) {
	accessToken = token
}

func GetClient() (*mcp.Client, error) {
	var opts []mcp.ClientOption
	if accessToken != "" {
		opts = append(opts, mcp.WithAccessToken(accessToken))
	}

	client, err := mcp.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		if mcp.IsAuthRequired(err) {
			output.PrintWarning("Not authenticated. Run 'notion config auth' to authenticate.")
			return nil, err
		}
		return nil, fmt.Errorf("start client: %w", err)
	}

	return client, nil
}

func RequireClient() (*mcp.Client, error) {
	return GetClient()
}

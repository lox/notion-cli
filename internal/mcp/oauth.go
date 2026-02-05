package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const callbackPath = "/callback"

func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64URLEncode(h[:])
}

func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

type OAuthResult struct {
	Code  string
	State string
	Error string
}

func RunOAuthFlow(ctx context.Context, tokenStore *FileTokenStore) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start callback server: %w", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)

	oauthConfig := transport.OAuthConfig{
		RedirectURI: redirectURI,
		TokenStore:  tokenStore,
		PKCEEnabled: true,
	}

	trans, err := transport.NewStreamableHTTP(
		DefaultEndpoint,
		transport.WithHTTPOAuth(oauthConfig),
	)
	if err != nil {
		return fmt.Errorf("create transport: %w", err)
	}

	mcpClient := client.NewClient(trans)
	defer func() { _ = mcpClient.Close() }()

	if err := mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("start client: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "notion-cli",
		Version: "0.1.0",
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err == nil {
		fmt.Println("Already authenticated!")
		return nil
	}

	handler := trans.GetOAuthHandler()
	if handler == nil {
		return fmt.Errorf("initialize (no handler): %w", err)
	}

	codeVerifier, err := GenerateCodeVerifier()
	if err != nil {
		return fmt.Errorf("generate code verifier: %w", err)
	}
	codeChallenge := GenerateCodeChallenge(codeVerifier)

	state, err := GenerateState()
	if err != nil {
		return fmt.Errorf("generate state: %w", err)
	}

	if handler.GetClientID() == "" {
		if err := handler.RegisterClient(ctx, "notion-cli"); err != nil {
			return fmt.Errorf("register client: %w", err)
		}
		if err := tokenStore.SaveClientID(ctx, handler.GetClientID()); err != nil {
			return fmt.Errorf("save client ID: %w", err)
		}
	}

	authURL, err := handler.GetAuthorizationURL(ctx, state, codeChallenge)
	if err != nil {
		return fmt.Errorf("get authorization URL: %w", err)
	}

	resultChan := make(chan OAuthResult, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != callbackPath {
				http.NotFound(w, r)
				return
			}

			result := OAuthResult{
				Code:  r.URL.Query().Get("code"),
				State: r.URL.Query().Get("state"),
				Error: r.URL.Query().Get("error"),
			}

			w.Header().Set("Content-Type", "text/html")
			if result.Error != "" {
				_, _ = fmt.Fprintf(w, "<h1>Authentication failed</h1><p>%s</p>", result.Error)
			} else {
				_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><body>
<h1>Authentication successful!</h1>
<p>You can close this window and return to the terminal.</p>
<script>window.close();</script>
</body></html>`)
			}

			resultChan <- result
		}),
	}

	go func() {
		_ = server.Serve(listener)
	}()
	defer func() { _ = server.Shutdown(context.Background()) }()

	fmt.Println()
	fmt.Println("To authenticate, open this URL in your browser:")
	fmt.Println()
	fmt.Printf("  %s\n", authURL)
	fmt.Println()
	fmt.Println("Waiting for authentication...")

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("(Could not open browser automatically: %v)\n", err)
	}

	select {
	case result := <-resultChan:
		if result.Error != "" {
			return fmt.Errorf("OAuth error: %s", result.Error)
		}
		if result.State != state {
			return errors.New("state mismatch - possible CSRF attack")
		}
		if result.Code == "" {
			return errors.New("no authorization code received")
		}

		if err := handler.ProcessAuthorizationResponse(ctx, result.Code, state, codeVerifier); err != nil {
			return fmt.Errorf("exchange token: %w", err)
		}

		fmt.Println()
		fmt.Println("Authentication successful!")
		return nil

	case <-ctx.Done():
		return ctx.Err()

	case <-time.After(5 * time.Minute):
		return errors.New("authentication timeout - no response received")
	}
}

func RefreshToken(ctx context.Context, tokenStore *FileTokenStore) (*transport.Token, error) {
	token, err := tokenStore.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	if token.RefreshToken == "" {
		return nil, errors.New("no refresh token available")
	}

	clientID, err := tokenStore.GetClientID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get client ID: %w", err)
	}
	if clientID == "" {
		return nil, errors.New("no client ID stored - run 'notion-cli auth login' first")
	}

	oauthConfig := transport.OAuthConfig{
		ClientID:    clientID,
		TokenStore:  tokenStore,
		PKCEEnabled: true,
	}

	trans, err := transport.NewStreamableHTTP(
		DefaultEndpoint,
		transport.WithHTTPOAuth(oauthConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("create transport: %w", err)
	}

	mcpClient := client.NewClient(trans)
	defer func() { _ = mcpClient.Close() }()

	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("start client: %w", err)
	}

	handler := trans.GetOAuthHandler()
	if handler == nil {
		return nil, errors.New("no OAuth handler available")
	}

	newToken, err := handler.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	if err := tokenStore.SaveToken(ctx, newToken); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}

	return newToken, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

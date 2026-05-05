package mcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client/transport"
)

func isolateMCPConfig(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func TestRefreshTokenSkipsRefreshWhenAnotherCallerAlreadyRefreshed(t *testing.T) {
	isolateMCPConfig(t)

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	saveExpiredToken(t, store)

	oldRefresh := refreshOAuthToken
	t.Cleanup(func() {
		refreshOAuthToken = oldRefresh
	})

	var refreshCalls atomic.Int32
	refreshOAuthToken = func(ctx context.Context, tokenStore *FileTokenStore, token *transport.Token) (*transport.Token, error) {
		refreshCalls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return &transport.Token{
			AccessToken:  "new-access",
			TokenType:    "bearer",
			RefreshToken: "new-refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := RefreshTokenIfNeeded(context.Background(), store)
			if err != nil {
				errs <- err
				return
			}
			if token.AccessToken != "new-access" {
				errs <- fmt.Errorf("access token = %q, want new-access", token.AccessToken)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
}

func TestRefreshTokenInvalidGrantUsesNewerSavedToken(t *testing.T) {
	isolateMCPConfig(t)

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	saveExpiredToken(t, store)

	oldRefresh := refreshOAuthToken
	t.Cleanup(func() {
		refreshOAuthToken = oldRefresh
	})

	refreshOAuthToken = func(ctx context.Context, tokenStore *FileTokenStore, token *transport.Token) (*transport.Token, error) {
		if err := tokenStore.SaveToken(ctx, &transport.Token{
			AccessToken:  "winner-access",
			TokenType:    "bearer",
			RefreshToken: "winner-refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}
		return nil, fmt.Errorf("refresh token: %w", transport.OAuthError{ErrorCode: "invalid_grant"})
	}

	token, err := RefreshToken(context.Background(), store)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if token.AccessToken != "winner-access" {
		t.Fatalf("access token = %q, want winner-access", token.AccessToken)
	}
}

func TestRefreshTokenForcesRefreshWhenTokenIsFresh(t *testing.T) {
	isolateMCPConfig(t)

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	if err := store.SaveClientID(context.Background(), "client-123"); err != nil {
		t.Fatalf("SaveClientID: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		AccessToken:  "fresh-access",
		TokenType:    "bearer",
		RefreshToken: "fresh-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	oldRefresh := refreshOAuthToken
	t.Cleanup(func() {
		refreshOAuthToken = oldRefresh
	})

	var refreshCalls atomic.Int32
	refreshOAuthToken = func(context.Context, *FileTokenStore, *transport.Token) (*transport.Token, error) {
		refreshCalls.Add(1)
		return &transport.Token{
			AccessToken:  "forced-access",
			TokenType:    "bearer",
			RefreshToken: "forced-refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		}, nil
	}

	token, err := RefreshToken(context.Background(), store)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if token.AccessToken != "forced-access" {
		t.Fatalf("access token = %q, want forced-access", token.AccessToken)
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
}

func TestRefreshTokenInvalidGrantRequiresLoginWhenNoNewerTokenExists(t *testing.T) {
	isolateMCPConfig(t)

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	saveExpiredToken(t, store)

	oldRefresh := refreshOAuthToken
	t.Cleanup(func() {
		refreshOAuthToken = oldRefresh
	})

	refreshOAuthToken = func(context.Context, *FileTokenStore, *transport.Token) (*transport.Token, error) {
		return nil, fmt.Errorf("refresh token: %w", transport.OAuthError{ErrorCode: "invalid_grant"})
	}

	_, err = RefreshToken(context.Background(), store)
	if err == nil {
		t.Fatal("RefreshToken returned nil error, want browser login required")
	}
	if got := err.Error(); !strings.Contains(got, "browser login required") {
		t.Fatalf("error = %q, want browser login required", got)
	}
}

func saveExpiredToken(t *testing.T, store *FileTokenStore) {
	t.Helper()

	if err := store.SaveClientID(context.Background(), "client-123"); err != nil {
		t.Fatalf("SaveClientID: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		AccessToken:  "old-access",
		TokenType:    "bearer",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
}

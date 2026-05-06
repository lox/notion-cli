package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/mark3labs/mcp-go/client/transport"
)

func isolateAuthConfig(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func TestAuthUsePersistsActiveProfile(t *testing.T) {
	isolateAuthConfig(t)

	cmd := &AuthUseCmd{Profile: "work"}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	active, err := config.ActiveProfile()
	if err != nil {
		t.Fatalf("ActiveProfile: %v", err)
	}
	if active != "work" {
		t.Fatalf("active profile = %q, want work", active)
	}
	if !strings.Contains(stdout, "Profile: work") {
		t.Fatalf("unexpected output: %q", stdout)
	}
}

func TestAuthListJSONShowsProfilesAndActiveState(t *testing.T) {
	isolateAuthConfig(t)

	if err := config.SetActiveProfile("work"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}
	if err := config.SetAPITokenForProfile("personal", "personal-token"); err != nil {
		t.Fatalf("SetAPITokenForProfile: %v", err)
	}
	store, err := mcp.NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		AccessToken: "oauth-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	cmd := &AuthListCmd{JSON: true}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(stdout, `"profile": "work"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stdout, `"active": true`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stdout, `"oauth_status": "valid"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stdout, `"profile": "personal"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestAuthStatusJSONReportsMissingTokenForProfile(t *testing.T) {
	isolateAuthConfig(t)

	cmd := &AuthStatusCmd{JSON: true}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{Profile: "work"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(stdout, `"profile": "work"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stdout, `"oauth_status": "missing"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestAuthStatusJSONReportsValidWhenAccessExpiredButRefreshAvailable(t *testing.T) {
	isolateAuthConfig(t)

	store, err := mcp.NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		AccessToken:  "stale-access",
		TokenType:    "Bearer",
		RefreshToken: "rotating-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	cmd := &AuthStatusCmd{JSON: true}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{Profile: "work"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(stdout, `"oauth_status": "valid"`) {
		t.Fatalf("expected valid status when refresh token present, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"authenticated": true`) {
		t.Fatalf("expected authenticated true when refresh token present, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"has_oauth_token": true`) {
		t.Fatalf("expected has_oauth_token field, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"oauth_expires_at":`) {
		t.Fatalf("expected oauth_expires_at field, got: %s", stdout)
	}
}

func TestAuthStatusJSONOmitsExpiryWhenAccessTokenMissing(t *testing.T) {
	isolateAuthConfig(t)

	store, err := mcp.NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		TokenType:    "Bearer",
		RefreshToken: "leftover-refresh",
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	cmd := &AuthStatusCmd{JSON: true}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{Profile: "work"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(stdout, `"oauth_status": "missing"`) {
		t.Fatalf("expected missing status when access token empty, got: %s", stdout)
	}
	if strings.Contains(stdout, `"oauth_expires_at"`) {
		t.Fatalf("expected no oauth_expires_at when access token empty, got: %s", stdout)
	}
}

func TestAuthStatusJSONReportsLoginRequiredWithoutRefreshToken(t *testing.T) {
	isolateAuthConfig(t)

	store, err := mcp.NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}
	if err := store.SaveToken(context.Background(), &transport.Token{
		AccessToken: "stale-access",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	cmd := &AuthStatusCmd{JSON: true}
	stdout := captureStdout(t, func() {
		if err := cmd.Run(&Context{Profile: "work"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(stdout, `"oauth_status": "login_required"`) {
		t.Fatalf("expected login_required without refresh token, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"authenticated": false`) {
		t.Fatalf("expected authenticated false without refresh token, got: %s", stdout)
	}
}

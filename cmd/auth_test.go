package cmd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/mark3labs/mcp-go/client/transport"
)

func TestAuthUsePersistsActiveProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

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
	t.Setenv("HOME", t.TempDir())

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
	t.Setenv("HOME", t.TempDir())

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
